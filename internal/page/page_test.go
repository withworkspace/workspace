package page

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// Pages are numbered as they're started, named by their first words, and
// saved from the version an edit began on; one put away keeps its number.
func TestStore(t *testing.T) {
	s := Store{Root: t.TempDir()}
	storetest.Own(t, s.Root)
	if list, err := s.List(); err != nil || len(list) != 0 {
		t.Fatalf("no pages yet: %v %v", list, err)
	}
	a, err := s.New([]byte("# Token cache\n"))
	if err != nil || a.ID != "p1" || a.Title != "Token cache" {
		t.Fatalf("new: %+v %v", a, err)
	}
	b, _ := s.New(nil)
	if b.ID != "p2" || b.Title != Untitled {
		t.Errorf("an empty page: %+v", b)
	}

	text, _, err := s.Read("p1")
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Write("p1", doc.Version(text), []byte("# Token cache\n\nWhy.\n"))
	if err != nil || v != doc.Version([]byte("# Token cache\n\nWhy.\n")) {
		t.Fatalf("save: %q %v", v, err)
	}
	// From the version before: something else wrote it since.
	if at, err := s.Write("p1", doc.Version(text), []byte("# Other\n")); !errors.Is(err, ErrChanged) || at != v {
		t.Errorf("a stale save: %q %v", at, err)
	}
	// The same text again is no change, whatever it's said to be from.
	if _, err := s.Write("p1", "stale", []byte("# Token cache\n\nWhy.\n")); err != nil {
		t.Errorf("saving what's there: %v", err)
	}
	if _, _, err := s.Read("p9"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such page: %v", err)
	}
	if _, _, err := s.Read("../x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("not a page's name: %v", err)
	}

	if err := s.Remove("p2"); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(); len(list) != 1 || list[0].ID != "p1" {
		t.Errorf("after putting p2 away: %+v", list)
	}
	if _, err := os.Stat(s.Path("p2") + ".removed"); err != nil {
		t.Errorf("p2's file is kept: %v", err)
	}
	if c, _ := s.New(nil); c.ID != "p3" {
		t.Errorf("p2's number isn't used again: %s", c.ID)
	}
}

// A page's cells are read from its text as the editor reads them, and a
// cell knows when the cells above it, or the page around it, have changed.
func TestCells(t *testing.T) {
	text := []byte("# Page\n\n<!-- ws:cell w1 v2 | the routes -->\n\n| Route |\n|---|\n| /tokens |\n\n<!-- /ws:cell -->\n\nWords.\n\n<!-- ws:cell w2 v0 -->\n\n<!-- /ws:cell -->\n\n<!-- ws:cell w3 v1 | never closed -->\n\nEnd.\n")
	cells := Cells(text)
	if len(cells) != 2 || cells[0].ID != "w1" || cells[0].V != 2 || cells[0].Prompt != "the routes" || cells[0].Body != "| Route |\n|---|\n| /tokens |" || cells[1].ID != "w2" || cells[1].Body != "" {
		t.Fatalf("cells: %+v", cells)
	}
	if string(text[cells[0].Start:cells[0].End]) != "<!-- ws:cell w1 v2 | the routes -->\n\n| Route |\n|---|\n| /tokens |\n\n<!-- /ws:cell -->" {
		t.Errorf("w1 spans %q", text[cells[0].Start:cells[0].End])
	}
	// One opened inside another that isn't closed: the inner one's the cell.
	if got := Cells([]byte("<!-- ws:cell w1 v1 -->\n\n<!-- ws:cell w2 v1 -->\n\nWords.\n\n<!-- /ws:cell -->\n")); len(got) != 1 || got[0].ID != "w2" {
		t.Errorf("nested: %+v", got)
	}

	above, around := Above(text, "w2"), Around(text, "w2")
	if above == "" || around == "" || Above(text, "w9") != "" || Around(text, "w9") != "" {
		t.Fatalf("above %q, around %q", above, around)
	}
	// Words changed outside any cell: the page around w2 has changed, the
	// cells above it haven't.
	words := []byte(strings.Replace(string(text), "Words.", "Other words.", 1))
	if Above(words, "w2") != above || Around(words, "w2") == around {
		t.Errorf("after words changed: above %v, around %v", Above(words, "w2") == above, Around(words, "w2") == around)
	}
	// The cell above written again: both have.
	again := []byte(strings.Replace(string(text), "/tokens", "/tokens/refresh", 1))
	if Above(again, "w2") == above || Around(again, "w2") == around {
		t.Errorf("after w1 changed: above %v, around %v", Above(again, "w2") == above, Around(again, "w2") == around)
	}
	// w2 itself filled: neither has. And w1 has nothing above it to change.
	filled := []byte(strings.Replace(string(text), "<!-- ws:cell w2 v0 -->\n", "<!-- ws:cell w2 v1 -->\n\nFilled.\n", 1))
	if Above(filled, "w2") != above || Around(filled, "w2") != around || Above(filled, "w1") != Above(text, "w1") {
		t.Errorf("after w2 filled: above %v, around %v", Above(filled, "w2") == above, Around(filled, "w2") == around)
	}
}

func TestTitle(t *testing.T) {
	for text, want := range map[string]string{
		"":                                    Untitled,
		"\n\n# The **token** cache\n\nWords.": "The token cache",
		"Just words to start with.\n":         "Just words to start with.",
		"<!-- ws:cell w1 v1 | routes -->\n\n| a |\n|---|\n\n<!-- /ws:cell -->\n\nAfter.": "| a |",
		"- [ ] a task": "[ ] a task",
	} {
		if got := Title([]byte(text)); got != want {
			t.Errorf("Title(%q) = %q, want %q", text, got, want)
		}
	}
}

// Filed, a page is as it reads: its cells' words without their comments,
// and no questions.
func TestAsText(t *testing.T) {
	text := "# Caching\n\nWhy we cache.\n\n<!-- ws:cell w3 v2 | a table of the routes -->\n| Route | Cached |\n|---|---|\n| /a | yes |\n<!-- /ws:cell -->\n\n<!-- ws:cell w4 v0 ask | is /b cached? -->\n<!-- /ws:cell -->\n\n## Next\n\n<!-- ws:cell w5 v0 | what's left -->\n<!-- /ws:cell -->\n\nThe end.\n\n<!-- ws:cell w6 v1 ask | anything else? -->\n<!-- /ws:cell -->\n"

	got := string(AsText([]byte(text)))

	want := "# Caching\n\nWhy we cache.\n\n| Route | Cached |\n|---|---|\n| /a | yes |\n\n## Next\n\nThe end.\n"
	if got != want {
		t.Errorf("AsText:\n%q\nwant\n%q", got, want)
	}
	if cells := Cells([]byte(text)); len(cells) != 4 || cells[0].Ask || !cells[1].Ask || cells[1].Prompt != "is /b cached?" {
		t.Errorf("Cells: %+v", cells)
	}
}

// What's kept of a page beside it is read with it, and a page with none
// has nothing.
func TestMeta(t *testing.T) {
	s := Store{Root: t.TempDir()}
	storetest.Own(t, s.Root)
	p, _ := s.New([]byte("# A proposal\n"))
	if _, got, _ := s.Read(p.ID); got.Kind != "" || got.Filed != nil {
		t.Errorf("a new page: %+v", got)
	}
	at := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	if err := s.SetMeta(p.ID, Meta{Kind: "bp", Filed: &Filed{Ref: "records:bp/x.md", Kind: "bp", At: at, Landed: true}}); err != nil {
		t.Fatal(err)
	}
	_, got, _ := s.Read(p.ID)
	if got.Kind != "bp" || got.Filed == nil || got.Filed.Ref != "records:bp/x.md" || !got.Filed.At.Equal(at) || !got.Filed.Landed {
		t.Errorf("read back: %+v", got)
	}
	if list, _ := s.List(); len(list) != 1 {
		t.Errorf("the record beside it is listed as a page: %+v", list)
	}
	if err := s.SetMeta("p9", Meta{Kind: "bp"}); err == nil {
		t.Error("kept a record for a page that isn't there")
	}
}
