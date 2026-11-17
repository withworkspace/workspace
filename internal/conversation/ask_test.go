package conversation

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
)

var asked = Anchor{Doc: "records:notes/tokens.md", Path: "notes/tokens.md", Version: "aaaaaaaaaaaa", Quote: &Quote{Exact: "per user"}, Section: "Caching"}

// An ask is a conversation about the document from the start, answered
// as talk is, but nobody's kept it: it isn't listed, logged, linked or
// found.
func TestAsk(t *testing.T) {
	root := owned(t)
	s := ForWorkspace(root)
	for _, bad := range []struct {
		a    Anchor
		body string
	}{{asked, "  "}, {Anchor{}, "Why?"}} {
		if _, err := s.OpenAsk(bad.a, You, bad.body, at); err == nil {
			t.Errorf("%+v %q: no error", bad.a, bad.body)
		}
	}
	s.OpenTalk(You, "What would a token cache take?", at)
	c, err := s.OpenAsk(asked, You, "Why per user?", at)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "w2" || !c.Ephemeral || c.Kind != Talk || c.What != WhatDoc || c.Anchor.Section != "Caching" || !c.NeedsAnswer() {
		t.Fatalf("w2 = %+v", c)
	}
	c, err = s.Say("w2", Turn{By: Agent, At: at.Add(time.Minute), Body: "Users never share tokens."})
	if err != nil || !c.Ephemeral || len(c.Turns) != 2 {
		t.Fatalf("answered: %+v, %v", c, err)
	}
	if c, _ := s.Load("w2"); !c.Ephemeral || c.Anchor.Quote.Exact != "per user" {
		t.Errorf("loaded: %+v", c)
	}
	if talks, _ := s.Talks(); len(talks) != 1 || talks[0].ID != "w1" {
		t.Errorf("listed: %+v", talks)
	}
	if on, _ := s.OnDoc(asked.Doc); len(on) != 0 {
		t.Errorf("a thread on the document: %+v", on)
	}
	if kind, _, _ := thing(t, root, s.key("w2")); kind != "" {
		t.Errorf("in the graph as %q", kind)
	}
	db, _ := store.Open(root)
	if found, _, _ := db.Find(context.Background(), store.Said{Words: "share"}); len(found) != 0 {
		t.Errorf("found: %+v", found)
	}

	// A mission's isn't in its log either.
	ms := mission.Store{Root: root}
	if _, err := ms.Create("Cache"); err != nil {
		t.Fatal(err)
	}
	before, _ := ms.ReadLog("cache", 0)
	if _, err := For(root, "cache").OpenAsk(asked, You, "Why per user?", at); err != nil {
		t.Fatal(err)
	}
	if after, _ := ms.ReadLog("cache", 0); len(after) != len(before) {
		t.Errorf("logged: %+v", after[len(before):])
	}
}

// The daemon sweeps an ask an hour after its last turn, unless it's being
// answered; what its run printed goes with it, and its number isn't given
// again.
func TestSweepAsks(t *testing.T) {
	root := owned(t)
	s := ForWorkspace(root)
	now := time.Now()
	old := now.Add(-AskFor - time.Minute)
	s.OpenAsk(asked, You, "Why per user?", old)                                       // w1: due
	s.OpenAsk(asked, You, "And per tenant?", old)                                     // w2: due, but answering
	s.OpenAsk(asked, You, "What's it cost?", old)                                     // w3: said to since
	s.Say("w3", Turn{By: You, At: now.Add(-time.Minute), Body: "In memory, I mean."}) // …
	s.OpenTalk(You, "What would a token cache take?", old)                            // w4: a conversation
	s.OpenAsk(asked, You, "Who reads it?", old)                                       // w5: due, the highest
	s.StartAnswering("w2", os.Getpid())
	os.WriteFile(s.RunPath("w1"), []byte("{}\n"), 0o644)
	// Its row says when it last changed, which a sweep reads first: as if
	// they were written an hour ago.
	db, _ := store.Open(root)
	db.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE conversations SET changed = ? WHERE id IN ('w1', 'w2', 'w5')`, stampOf(old))
		return err
	})

	swept, err := SweepAsks(context.Background(), root, now)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(swept, ",") != "w1,w5" {
		t.Errorf("swept %v", swept)
	}
	if _, err := s.Load("w1"); err == nil {
		t.Error("w1 is still there")
	}
	if _, err := os.Stat(s.RunPath("w1")); !os.IsNotExist(err) {
		t.Errorf("w1's run: %v", err)
	}
	for _, id := range []string{"w2", "w3", "w4"} {
		if _, err := s.Load(id); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
	if c, _ := s.OpenTalk(You, "Next?", now); c == nil || c.ID != "w6" {
		t.Errorf("the next is %+v, not w6", c)
	}

	// Answered, it's due an hour after: nothing's swept twice.
	s.EndAnswering("w2")
	if swept, _ := SweepAsks(context.Background(), root, now); len(swept) != 0 {
		t.Errorf("swept again: %v", swept)
	}
	if swept, _ := SweepAsks(context.Background(), root, now.Add(AskFor+time.Minute)); strings.Join(swept, ",") != "w2,w3" {
		t.Errorf("an hour on, swept %v", swept)
	}
}

// An ask about a pull request, and lines of its diff, is one as an ask
// about a document is: not listed until it's kept, and then about the
// pull request.
func TestAsk_PR(t *testing.T) {
	root := owned(t)
	s := ForWorkspace(root)
	lines := Anchor{PR: "api#89", Path: "cache.go", Side: "new", Line: 12, EndLine: 13, Commit: "abcdef123456", Excerpt: []string{"mu.Lock()", "defer mu.Unlock()"}}
	c, err := s.OpenAsk(lines, You, "Why this lock?", at)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Ephemeral || c.What != WhatPR || c.Anchor.Line != 12 || len(c.Anchor.Excerpt) != 2 {
		t.Fatalf("w1 = %+v", c)
	}
	if talks, _ := s.Talks(); len(talks) != 0 {
		t.Errorf("listed: %+v", talks)
	}
	if about := s.AboutPR("api#89"); len(about) != 0 {
		t.Errorf("about api#89, unkept: %+v", about)
	}
	if got := links(t, root, store.Key{Name: "api#89"}, false); got != "" {
		t.Errorf("what's of api#89, unkept: %s", got)
	}
	if _, err := s.Keep("w1", You, at); err != nil {
		t.Fatal(err)
	}
	if kind, _, _ := thing(t, root, s.key("w1")); kind != WhatPR {
		t.Errorf("in the graph as %q", kind)
	}
	if got := links(t, root, store.Key{Name: "api#89"}, false); got != "about /w1" {
		t.Errorf("what's of api#89, kept: %s", got)
	}
	if about := s.AboutPR("api#89"); len(about) != 1 || about[0].ID != "w1" {
		t.Errorf("about api#89, kept: %+v", about)
	}
}

// Keeping an ask makes it a conversation like any: listed, linked to its
// document and found by what it said, and the sweep leaves it. Only the
// user keeps one, and keeping it again does nothing.
func TestKeep(t *testing.T) {
	root := owned(t)
	s := ForWorkspace(root)
	old := time.Now().Add(-AskFor - time.Hour)
	s.OpenAsk(asked, You, "Why per user?", old)
	s.Say("w1", Turn{By: Agent, At: old.Add(time.Minute), Body: "Users never share tokens."})
	if _, err := s.Keep("w1", Agent, at); err != ErrKeepNotYours {
		t.Errorf("the agent kept it: %v", err)
	}
	if _, err := s.Keep("w9", You, at); err == nil {
		t.Error("kept w9, which isn't there")
	}
	kept := old.Add(time.Hour)
	c, err := s.Keep("w1", You, kept)
	if err != nil || c.Ephemeral || c.Kept == nil || !c.Kept.Equal(kept) || len(c.Turns) != 2 {
		t.Fatalf("kept: %+v, %v", c, err)
	}
	if c, _ := s.Load("w1"); c.Ephemeral || c.Kept == nil || c.WaitingOnYou() {
		t.Errorf("loaded: %+v", c) // its answer was read in the card, before it was kept
	}
	if talks, _ := s.Talks(); len(talks) != 1 || talks[0].ID != "w1" {
		t.Errorf("listed: %+v", talks)
	}
	if kind, _, _ := thing(t, root, s.key("w1")); kind != WhatDoc {
		t.Errorf("in the graph as %q", kind)
	}
	if got := links(t, root, s.key("w1"), true); got != "about /records/notes/tokens.md" {
		t.Errorf("links: %s", got)
	}
	db, _ := store.Open(root)
	if found, _, _ := db.Find(context.Background(), store.Said{Words: "share"}); len(found) != 1 {
		t.Errorf("found: %+v", found)
	}
	if again, err := s.Keep("w1", You, kept.Add(time.Minute)); err != nil || !again.Kept.Equal(kept) || again.lines != c.lines {
		t.Errorf("kept again: %+v, %v", again, err)
	}
	// Answered again, it's your turn.
	if c, _ := s.Say("w1", Turn{By: You, At: kept.Add(time.Minute), Body: "And per tenant?"}); c.WaitingOnYou() {
		t.Error("waiting on you, having asked")
	}
	if c, _ := s.Say("w1", Turn{By: Agent, At: kept.Add(2 * time.Minute), Body: "Tenants share."}); !c.WaitingOnYou() {
		t.Error("answered again, and not waiting on you")
	}
	// Due as it is, but kept: the sweep leaves it.
	db.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE conversations SET changed = ?`, stampOf(old))
		return err
	})
	if swept, _ := SweepAsks(context.Background(), root, time.Now()); len(swept) != 0 {
		t.Errorf("swept %v", swept)
	}

	// A mission's is logged as it's kept.
	ms := mission.Store{Root: root}
	if _, err := ms.Create("Cache"); err != nil {
		t.Fatal(err)
	}
	m := For(root, "cache")
	m.OpenAsk(asked, You, "Why per user?", at)
	if _, err := m.Keep("t1", You, at); err != nil {
		t.Fatal(err)
	}
	if log, _ := ms.ReadLog("cache", 0); len(log) == 0 || log[len(log)-1].Text != "t1 kept: Why per user?" {
		t.Errorf("logged: %+v", log)
	}
}
