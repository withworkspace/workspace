package conversation

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// links says a thing's links, from it or to it, as "kind name@lines",
// sorted.
func links(t *testing.T, root string, k store.Key, from bool) string {
	t.Helper()
	db, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	db.Read(context.Background(), func(tx *store.Tx) error {
		ls, err := tx.Links(k, from)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range ls {
			other := l.Dst
			if !from {
				other = l.Src
			}
			s := l.Kind + " " + other.Scope + "/" + other.Name
			if l.Lines != "" {
				s += "@" + l.Lines
			}
			out = append(out, s)
		}
		return nil
	})
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func thing(t *testing.T, root string, k store.Key) (kind, title, state string) {
	t.Helper()
	db, _ := store.Open(root)
	db.Read(context.Background(), func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT kind, title, state FROM things WHERE scope = ? AND name = ?`, k.Scope, k.Name).Scan(&kind, &title, &state)
	})
	return
}

// A conversation is a thing in the graph, written with each line: about
// what it's anchored to (a capsule's file, and the repo's file as well),
// and mentioning what its turns named, at their lines. It goes with its
// mission when that's renamed.
func TestGraph(t *testing.T) {
	s := threads(t)
	root := s.Missions.Root
	c, err := s.OpenThread(NewThread{Kind: Ask, By: You, Body: "Why per user?",
		Anchor: Anchor{Doc: "records/tokens:proposal.md", Path: "proposal.md", Line: 3, EndLine: 5}}, at)
	if err != nil {
		t.Fatal(err)
	}
	s.Say(c.ID, Turn{By: Agent, At: at.Add(time.Minute), Body: "See demo/main.go:12, demo/main.go:12 again, and api#89.", Refs: []ref.Pinned{
		{Said: "demo/main.go:12", Ref: ref.Ref{Kind: ref.File, Repo: "demo", Worktree: ref.Ground, Path: "main.go", Line: 12}},
		{Said: "demo/main.go:12", Ref: ref.Ref{Kind: ref.File, Repo: "demo", Worktree: ref.Ground, Path: "main.go", Line: 12}},
		{Said: "api#89", Ref: ref.Ref{Kind: ref.PR, Repo: "api", Number: 89}},
	}})
	s.SetStatus(c.ID, You, Resolved, at.Add(2*time.Minute))

	me := store.Key{Scope: "cache", Name: "t1"}
	if got, want := links(t, root, me, true), "about /records/proposal.md@3-5, about /records/tokens:proposal.md@3-5, mentions /api#89, mentions /demo/main.go@12"; got != want {
		t.Errorf("t1's links:\n got %s\nwant %s", got, want)
	}
	if got := links(t, root, store.Key{Name: "demo/main.go"}, false); got != "mentions cache/t1@12" {
		t.Errorf("what's said of demo/main.go: %s", got)
	}
	if kind, title, state := thing(t, root, me); kind != WhatThreadAsk || title != "Why per user?" || state != Resolved {
		t.Errorf("t1 is %s %q %s", kind, title, state)
	}

	// The workspace's: a cell is in its page, a pass is about it, a talk
	// about a pull request is about that.
	ws := ForWorkspace(root)
	ws.OpenCell("p3", "The routes.", "", "", "", at)
	ws.OpenPass("p3", "check", at)
	ws.OpenAsk(Anchor{PR: "api#89"}, You, "Is this safe?", at)
	ws.Keep("w3", You, at)
	if got := links(t, root, store.Key{Name: "p3"}, false); got != "about /w2, in /w1" {
		t.Errorf("what's of p3: %s", got)
	}
	if got := links(t, root, store.Key{Name: "api#89"}, false); got != "about /w3, mentions cache/t1" {
		t.Errorf("what's of api#89: %s", got)
	}

	// Renamed, its mission's conversations are read there.
	if err := MoveMission(root, "cache", "token-cache"); err != nil {
		t.Fatal(err)
	}
	if got := links(t, root, store.Key{Name: "demo/main.go"}, false); got != "mentions token-cache/t1@12" {
		t.Errorf("after the rename: %s", got)
	}
	if kind, _, _ := thing(t, root, me); kind != "" {
		t.Errorf("t1 is still in cache: %s", kind)
	}
	if kind, _, state := thing(t, root, store.Key{Scope: "token-cache", Name: "t1"}); kind != WhatThreadAsk || state != Resolved {
		t.Errorf("t1 in token-cache: %s %s", kind, state)
	}
}

// A document's stored ref is the file it is, folders and all: a thread on
// records:adr/0001.md is about records/adr/0001.md, the same file as it's
// named anywhere else.
func TestDocKeys(t *testing.T) {
	for doc, want := range map[string][]store.Key{
		"records:adr/0001.md":      {{Name: "records/adr/0001.md"}},
		"records/adr/0001.md":      {{Name: "records/adr/0001.md"}},
		"records:adr/2026/0001.md": {{Name: "records/adr/2026/0001.md"}},
		"demo:README.md":           {{Name: "demo/README.md"}},
		"records/x:adr/0001.md":    {{Name: "records/x:adr/0001.md"}, {Name: "records/adr/0001.md"}},
		"mission:notes.md":         {{Scope: "cache", Name: "mission:notes.md"}},
	} {
		if got := docKeys("cache", doc); !reflect.DeepEqual(got, want) {
			t.Errorf("docKeys(%q) = %+v, want %+v", doc, got, want)
		}
	}

	s := threads(t)
	if _, err := s.OpenThread(NewThread{Kind: Ask, By: You, Body: "Why this one?",
		Anchor: Anchor{Doc: "records:adr/0001.md", Path: "adr/0001.md", Line: 3, EndLine: 5}}, at); err != nil {
		t.Fatal(err)
	}
	if got := links(t, s.Missions.Root, store.Key{Name: "records/adr/0001.md"}, false); got != "about cache/t1@3-5" {
		t.Errorf("what's said of records/adr/0001.md: %q", got)
	}
}
