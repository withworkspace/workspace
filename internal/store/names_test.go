package store

import (
	"slices"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/ref"
)

// jira is a workspace where a plugin's issues are named (RAM-12), with a
// repo, a mission and an artifact to name besides.
func jira() *ref.Known {
	return &ref.Known{
		Repos:     map[string]ref.Tree{"demo": nil},
		Missions:  []string{"cache-per-user"},
		Artifacts: []string{"a3"},
		Things:    []ref.Named{{Plugin: "jira", What: "issue", Prefixes: []string{"RAM-"}, Address: "https://acme.example/browse/{ref}"}},
	}
}

// mentioned is what a thing points at as mentions, each as its name and
// lines, in order.
func mentioned(t testing.TB, db *DB, from Key) []string {
	t.Helper()
	var out []string
	if err := db.Read(bg, func(tx *Tx) error {
		links, err := tx.Links(from, true)
		for _, l := range links {
			if l.Kind != Mentions {
				continue
			}
			name := l.Dst.Scope + "/" + l.Dst.Name
			if l.Lines != "" {
				name += ":" + l.Lines
			}
			out = append(out, name)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

// What a place's words name is linked from the thing that says them, when
// the words are written: each thing once, a repo's bare name never, and
// nothing where nobody's said what there is to name.
func TestWords_LinkWhatTheyName(t *testing.T) {
	db := own(t)
	at := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	d1, o := Key{Scope: "cache", Name: "d1"}, Origin("decision:cache/d1")
	decide := func(tx *Tx) error {
		if err := tx.Forget(o); err != nil {
			return err
		}
		if err := tx.Words(o, Place{Thing: d1, Kind: "decision", Title: "Do RAM-12 before RAM-7", Text: "RAM-12 blocks demo#4, and the demo repo is quiet. See [the issue](https://acme.example/browse/RAM-9?focusedCommentId=4) and a3.v2.", Said: at}); err != nil {
			return err
		}
		// A second place of the same thing names nothing twice.
		return tx.Words(o, Place{Thing: d1, At: "more", Kind: "decision", Text: "RAM-12 again, with cache-per-user.", Said: at})
	}

	say(t, db, decide)
	if got := mentioned(t, db, d1); len(got) != 0 {
		t.Fatalf("with nobody saying what there is to name, it mentions %q", got)
	}

	asked := 0
	Name(db.Root(), func() *ref.Known { asked++; return jira() })
	t.Cleanup(func() { Name(db.Root(), nil) })
	say(t, db, decide)
	want := []string{"/RAM-12", "/RAM-7", "/RAM-9", "/a3", "/cache-per-user", "/demo#4"}
	if got := mentioned(t, db, d1); !slices.Equal(got, want) {
		t.Errorf("it mentions %q, want %q: its title's names and its words', what a link's address is the place of, an artifact's version as the artifact, and not the repo by its name alone", got, want)
	}
	if asked != 1 {
		t.Errorf("what there is to name was asked %d times in one transaction, want once", asked)
	}

	// Said again without the names, they're gone with what said them.
	say(t, db, func(tx *Tx) error {
		if err := tx.Forget(o); err != nil {
			return err
		}
		return tx.Words(o, Place{Thing: d1, Kind: "decision", Title: "Do the first before the second", Said: at})
	})
	if got := mentioned(t, db, d1); len(got) != 0 {
		t.Errorf("forgotten and said again without them, it still mentions %q", got)
	}
}

// A place said by something other than the thing it's in (a mission's
// brief) links what its words name from that, and its title's names not at
// all: the title is the thing's, not what says the words'. A place whose
// record pins its own names is left alone, and a thing doesn't mention
// itself.
func TestWords_LinkFromWhatSaysThem(t *testing.T) {
	db := own(t)
	Name(db.Root(), func() *ref.Known { return jira() })
	t.Cleanup(func() { Name(db.Root(), nil) })
	at := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	mission, brief, thread, issue := Key{Name: "cache-per-user"}, Key{Scope: "cache-per-user", Name: "brief"}, Key{Scope: "cache-per-user", Name: "t1"}, Key{Name: "RAM-12"}
	say(t, db, func(tx *Tx) error {
		if err := tx.Words("missionfile:cache-per-user/brief.md", Place{Thing: mission, By: brief, At: "goal", Kind: "brief", Title: "Cache per user (RAM-7)", Heading: "Goal", Text: "What RAM-12 asks for, in cache-per-user.", Said: at}); err != nil {
			return err
		}
		if err := tx.Words("conversation:cache-per-user/t1", Place{Thing: thread, At: "1", Kind: "turn", Text: "Is this RAM-12?", Said: at, Pinned: true}); err != nil {
			return err
		}
		return tx.Words("plugin:jira/issue/RAM-12", Place{Thing: issue, Kind: "thing", Title: "RAM-12 Token refresh is flaky", Text: "As RAM-12 says, after RAM-7.", Said: at})
	})
	if got, want := mentioned(t, db, brief), []string{"/RAM-12"}; !slices.Equal(got, want) {
		t.Errorf("the brief mentions %q, want %q: not the mission it's in", got, want)
	}
	if got := mentioned(t, db, mission); len(got) != 0 {
		t.Errorf("the mission itself mentions %q: its brief says its words", got)
	}
	if got := mentioned(t, db, thread); len(got) != 0 {
		t.Errorf("a place that pins its own names was read for them: %q", got)
	}
	if got, want := mentioned(t, db, issue), []string{"/RAM-7"}; !slices.Equal(got, want) {
		t.Errorf("the issue mentions %q, want %q: not itself", got, want)
	}
}

// A file's lines are where in it, on the link; the file is its key.
func TestKeyOf(t *testing.T) {
	for _, c := range []struct {
		r           ref.Ref
		name, lines string
	}{
		{ref.Ref{Kind: ref.File, Repo: "demo", Path: "main.go", Line: 12, EndLine: 20}, "demo/main.go", "12-20"},
		{ref.Ref{Kind: ref.File, Repo: "demo", Path: "main.go", Line: 12}, "demo/main.go", "12"},
		{ref.Ref{Kind: ref.Artifact, ID: "a3", Version: 2}, "a3", ""},
		{ref.Ref{Kind: ref.Thing, Plugin: "jira", What: "issue", ID: "RAM-12"}, "RAM-12", ""},
	} {
		if k, lines := KeyOf(c.r); k != (Key{Name: c.name}) || lines != c.lines {
			t.Errorf("KeyOf(%+v) = %+v, %q, want %q, %q", c.r, k, lines, c.name, c.lines)
		}
	}
}

// Whoever holds a workspace for the process's life names it for good: one
// loaded for a moment after (a command's, a job's) doesn't take its place,
// until it's taken back.
func TestKeepName_StandsOverName(t *testing.T) {
	root := t.TempDir()
	held := func() *ref.Known { return jira() }
	brief := func() *ref.Known { return &ref.Known{} }
	asked := func() func() *ref.Known {
		namingMu.Lock()
		defer namingMu.Unlock()
		return naming[root]
	}
	same := func(a, b func() *ref.Known) bool { return a != nil && b != nil && a() != nil && len(a().Things) == len(b().Things) }
	t.Cleanup(func() { Name(root, nil) })

	Name(root, brief)
	KeepName(root, held)
	Name(root, brief)
	if got := asked(); !same(got, held) {
		t.Error("a moment's name took the place of the one kept")
	}
	Name(root, nil)
	Name(root, brief)
	if got := asked(); !same(got, brief) {
		t.Error("taken back, the kept one still stood")
	}
}
