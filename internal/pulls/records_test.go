package pulls

import (
	"context"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// found is the things a search of the records finds, best first.
func found(t *testing.T, root, words string) []string {
	t.Helper()
	db, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := db.Find(context.Background(), store.Said{Words: words})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range got {
		out = append(out, f.Thing.Key.Name)
	}
	return out
}

// What's kept of a pull request is written only when it changes: its
// page reads it each time it opens. A change is its words again.
func TestUpdate(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	seq := func() int64 {
		n, err := db.Seq(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	title := func(r *Record) { r.Title, r.Body = "Cache tokens", "Per tenant." }
	if _, err := Update(root, "api", 89, title); err != nil {
		t.Fatal(err)
	}
	was := seq()
	if r, err := Update(root, "api", 89, title); err != nil || r.Title != "Cache tokens" || seq() != was {
		t.Errorf("nothing changed, and it was written: %+v, %v", r, err)
	}
	// One never kept is kept, even with nothing in it.
	if _, err := Update(root, "api", 90, func(*Record) {}); err != nil || seq() == was {
		t.Errorf("a new one wasn't kept: %v", err)
	}
	if _, err := Update(root, "api", 89, func(r *Record) { r.Body = "Each org's apart." }); err != nil {
		t.Fatal(err)
	}
	if got := found(t, root, "tenant"); len(got) != 0 {
		t.Errorf("its old words are still found: %v", got)
	}
	if got := found(t, root, "org"); len(got) != 1 || got[0] != "api#89" {
		t.Errorf("its new words: %v", got)
	}
	if err := (func() error { _, err := SayWords(context.Background(), root); return err })(); err != nil {
		t.Fatal(err)
	}
	if got := found(t, root, "org"); len(got) != 1 {
		t.Errorf("said again: %v", got)
	}
}

// The list is the records': a process holding an older one (the daemon,
// after a command asked GitHub) reads the newer in its place.
func TestList_ReadFromAnotherProcess(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	at := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	daemon, command := &Service{Root: root}, &Service{Root: root}
	if daemon.Peek() != nil {
		t.Fatal("a list before one was kept")
	}
	if _, err := keepList(root, &List{At: at, Sections: []Section{}, Count: 1}); err != nil {
		t.Fatal(err)
	}
	first := daemon.Peek()
	if first == nil || first.Count != 1 {
		t.Fatalf("daemon: %+v", first)
	}
	if again := daemon.Peek(); again != first {
		t.Error("the same list was read again")
	}
	if _, err := keepList(root, &List{At: at.Add(time.Minute), Sections: []Section{}, Count: 2}); err != nil {
		t.Fatal(err)
	}
	if l := daemon.Peek(); l == nil || l.Count != 2 {
		t.Errorf("the newer list wasn't read: %+v", l)
	}
	if l := command.Peek(); l == nil || l.Count != 2 {
		t.Errorf("command: %+v", l)
	}
}
