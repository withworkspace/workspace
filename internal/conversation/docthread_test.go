package conversation

import (
	"errors"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/doc"
)

func TestOpenOnDoc(t *testing.T) {
	s := threads(t)
	text := []byte("# Tokens\n\nCache per user.\nCache per user.\nExpire hourly.\n")
	v := doc.Version(text)
	ref := "records/tokens:proposal.md"
	str := func(s string) *string { return &s }

	if _, err := s.OpenOnDoc(ref, text, DocThread{Version: "old", Line: 3, Kind: Ask, Body: "x"}, You, at); !errors.Is(err, ErrStale) {
		t.Errorf("another version: %v, want ErrStale", err)
	}
	if _, err := s.OpenOnDoc(ref, text, DocThread{Version: v, Line: 6, Kind: Ask, Body: "x"}, You, at); err == nil {
		t.Error("a line past the end: no error")
	}

	// A suggestion whose text is there once is an edit to make.
	c, err := s.OpenOnDoc(ref, text, DocThread{Version: v, Line: 5, Quote: Quote{Exact: "hourly", Prefix: strings.Repeat("p", 100)}, Kind: Ask, Suggestion: str("daily")}, You, at)
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != Change || c.Pending() == nil || c.Pending().From != "hourly" || c.Pending().To != "daily" {
		t.Errorf("an edit: kind %s, suggestions %+v", c.Kind, c.Suggestions)
	}
	if got := len([]rune(c.Anchor.Quote.Prefix)); got != contextMax {
		t.Errorf("the quote's prefix is kept to %d runes, want %d", got, contextMax)
	}
	if c.Anchor.Path != "proposal.md" || len(c.Anchor.Excerpt) != 1 || c.Anchor.Excerpt[0] != "Expire hourly." {
		t.Errorf("anchor = %+v", c.Anchor)
	}
	if _, err := s.VersionText(v); err != nil {
		t.Errorf("the version it's on isn't kept: %v", err)
	}

	// One whose text is there twice is said, for the agent to make.
	c, err = s.OpenOnDoc(ref, text, DocThread{Version: v, Line: 3, Quote: Quote{Exact: "Cache per user."}, Body: "Shared.", Suggestion: str("Cache per team.")}, You, at)
	if err != nil {
		t.Fatal(err)
	}
	if c.Pending() != nil || !strings.HasSuffix(c.Turns[0].Body, "It should read: Cache per team.") {
		t.Errorf("said: suggestions %+v, body %q", c.Suggestions, c.Turns[0].Body)
	}
}

func TestTakeUp(t *testing.T) {
	s := ForWorkspace(owned(t))
	c, err := s.OpenTalk(You, "What's left?", at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetStatus(c.ID, You, Resolved, at); err != nil {
		t.Fatal(err)
	}
	c, err = s.TakeUp(c.ID, Turn{By: You, At: at, Body: "One more thing."})
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != Open || len(c.Turns) != 2 {
		t.Errorf("taken up: status %s, %d turns", c.Status, len(c.Turns))
	}
}
