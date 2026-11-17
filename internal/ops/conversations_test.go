package ops

import (
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
)

// A workspace the daemon holds reads its config again once it's changed,
// and keeps the last that read when it no longer does. One opened before
// its config reads has none until it does.
func TestOpenReadsConfigAgain(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	write := func(name, text string, at time.Time) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(path, at, at)
	}
	start := time.Now().Add(-time.Hour)
	write("ws.toml", "[workspace\n", start)
	w := Open(root, Options{})
	if w.WS().Root != root || w.WS().Org != "" {
		t.Fatalf("opened on a config that doesn't read: %+v", w.WS())
	}
	write("ws.toml", "[workspace]\norg = \"acme\"\ndefault_branch = \"main\"\n[repos]\n", start.Add(time.Second))
	if w.Config().Web.Port != 0 || w.WS().Org != "acme" {
		t.Fatalf("opened: port %d, org %q", w.Config().Web.Port, w.WS().Org)
	}
	write("ws.local.toml", "[web]\nport = 4242\n", start.Add(time.Minute))
	if got := w.Config().Web.Port; got != 4242 {
		t.Errorf("after ws.local.toml changed, port = %d, want 4242", got)
	}
	write("ws.toml", "[workspace\n", start.Add(2*time.Minute))
	if got := w.Config().Web.Port; got != 4242 || w.WS().Org != "acme" {
		t.Errorf("a config that doesn't read: port %d, org %q, want the last that did", got, w.WS().Org)
	}
}

// Threads and talk are written once, here, and each has the daemon
// answer it when there's something for its agent to answer.
func TestConversations(t *testing.T) {
	w, _ := fixture(t, "")
	var answers []string
	w.opts.Answer = func(kind, slug, id string) { answers = append(answers, kind+" "+slug+"/"+id) }
	sc := doc.OfMission("cache")
	text := []byte("# Plan\n\nCache per user.\n")
	ref := "frontend/one:plan.md"

	c, err := w.OpenThread(sc, ref, text, conversation.DocThread{Version: doc.Version(text), Line: 3, Quote: conversation.Quote{Exact: "per user"}, Kind: conversation.Ask, Body: "Why?"}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Reply(sc, c.ID, text, "And per team?", "", AsYou); err != nil {
		t.Fatal(err)
	}
	if _, err := w.StartTalk(doc.Scope{}, "What's left?", AsYou); err != nil {
		t.Fatal(err)
	}
	if _, err := w.StartTalk(doc.OfMission("nope"), "Hello?", AsYou); err == nil {
		t.Error("talk in a mission that isn't there: no error")
	}
	want := []string{job.Thread + " cache/t1", job.Thread + " cache/t1", job.Answer + " /w1"}
	if !slices.Equal(answers, want) {
		t.Errorf("answered %q, want %q", answers, want)
	}

	// Resolved, with something said worth drafting from, its agent drafts
	// what was decided; recorded, it's the mission's, and something you
	// did to it.
	conversation.For(w.Root(), "cache").Say(c.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "Tokens are per tenant."})
	answers = nil
	if _, err := w.SetThreadStatus(sc, c.ID, conversation.Resolved, AsYou); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(answers, []string{job.Draft + " cache/t1"}) {
		t.Errorf("resolving answered %q, want a draft", answers)
	}
	w.Missions.Update("cache", func(m *mission.Mission) error { m.Dispatches = 2; return nil })
	said, err := w.Decide("cache", c.ID, "Key per tenant.", "Tokens are.", []string{ref}, AsYou)
	if err != nil || said != "Recorded d1: Key per tenant." {
		t.Fatalf("decide: %q, %v", said, err)
	}
	if m, _ := w.Missions.Load("cache"); m.Dispatches != 0 {
		t.Errorf("recording a decision left the dispatch count at %d", m.Dispatches)
	}
	if _, err := w.Decide("cache", c.ID, "Again.", "", nil, AsYou); err == nil {
		t.Error("recorded twice: no error")
	}
}
