package page

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// found is the things a search of the records finds, best first, and the
// heading of the place each is led to.
func found(t *testing.T, root, words string) (names, headings []string) {
	t.Helper()
	db, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := db.Find(context.Background(), store.Said{Words: words})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		names, headings = append(names, f.Thing.Key.Name), append(headings, f.Heading)
	}
	return names, headings
}

// A page says what it reads as, a place for each section: its cells'
// words and not its questions. Saved again, it says what it says now; put
// away, nothing. A save names the page and the workspace, and one that
// changes nothing writes nothing.
func TestWords(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	s := Store{Root: root}
	text := []byte("# Caching\n\nWhy we cache.\n\n<!-- ws:cell w3 v2 | the routes -->\n| Route |\n|---|\n| /tokens |\n<!-- /ws:cell -->\n\n<!-- ws:cell w4 v0 ask | is refresh cached? -->\n<!-- /ws:cell -->\n\n## Revocation\n\nTenants apart.\n")
	p, err := s.New(text)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := s.New([]byte("# Other\n\nCaching elsewhere.\n"))
	if names, _ := found(t, root, "tokens"); !slices.Equal(names, []string{p.ID}) {
		t.Errorf("a cell's words: %v", names)
	}
	if names, _ := found(t, root, "refresh"); len(names) != 0 {
		t.Errorf("a question's: %v", names)
	}
	if names, headings := found(t, root, "tenants"); !slices.Equal(names, []string{p.ID}) || headings[0] != "Revocation" {
		t.Errorf("a section: %v %v", names, headings)
	}

	seq, _ := db.Seq(context.Background())
	again := []byte("# Caching\n\nWhy we cache.\n\n## Revocation\n\nOrgs apart.\n")
	if _, err := s.Write(p.ID, doc.Version(text), again); err != nil {
		t.Fatal(err)
	}
	topics, now, err := db.Since(context.Background(), seq)
	if err != nil || !slices.Contains(topics, "page:"+p.ID) || !slices.Contains(topics, "workspace") {
		t.Errorf("a save's topics: %v %v", topics, err)
	}
	if _, err := s.Write(p.ID, "whatever", again); err != nil {
		t.Fatal(err)
	}
	if seq, _ := db.Seq(context.Background()); seq != now {
		t.Error("saving what's there wrote something")
	}
	if names, _ := found(t, root, "tenants"); len(names) != 0 {
		t.Errorf("its old words: %v", names)
	}
	if names, _ := found(t, root, "orgs"); !slices.Equal(names, []string{p.ID}) {
		t.Errorf("its new words: %v", names)
	}

	if err := s.Remove(p.ID); err != nil {
		t.Fatal(err)
	}
	if names, _ := found(t, root, "caching"); !slices.Equal(names, []string{other.ID}) {
		t.Errorf("after %s was put away: %v", p.ID, names)
	}
	if err := s.Remove(p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("put away twice: %v", err)
	}
	if err := s.SetMeta(p.ID, Meta{Kind: "adr"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a page put away kept something: %v", err)
	}
	if n, err := SayWords(context.Background(), root); err != nil || n != 1 {
		t.Errorf("said again: %d %v", n, err)
	}
	if names, _ := found(t, root, "caching"); !slices.Equal(names, []string{other.ID}) {
		t.Errorf("said again: %v", names)
	}
}

// A page's file edited outside ws is taken in when the page is next
// listed, saved or searched for: a save from what it was before is
// refused, as it was when the check was the file's. A file that's gone
// takes the page out of the list and its words with it, and its number
// stays its; back, it's the page again.
func TestEditedByHand(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	s := Store{Root: root}
	text := []byte("# Plan\n\nFirst.\n")
	p, err := s.New(text)
	if err != nil {
		t.Fatal(err)
	}
	byHand := []byte("# Plan B\n\nSecond thoughts.\n")
	if err := os.WriteFile(s.Path(p.ID), byHand, 0o644); err != nil {
		t.Fatal(err)
	}
	if at, err := s.Write(p.ID, doc.Version(text), []byte("# Plan\n\nMine.\n")); !errors.Is(err, ErrChanged) || at != doc.Version(byHand) {
		t.Errorf("a save from before the hand's edit: %q %v", at, err)
	}
	if _, got, _ := s.Read(p.ID); got.Title != "Plan B" {
		t.Errorf("read: %q", got.Title)
	}
	if list, _ := s.List(); len(list) != 1 || list[0].Title != "Plan B" {
		t.Errorf("listed: %+v", list)
	}
	if names, _ := found(t, root, "thoughts"); !slices.Equal(names, []string{p.ID}) {
		t.Errorf("its words now: %v", names)
	}
	if names, _ := found(t, root, "first"); len(names) != 0 {
		t.Errorf("its words before: %v", names)
	}

	// Edited by hand again, and searched for before it's listed.
	os.WriteFile(s.Path(p.ID), []byte("# Plan C\n\nThird way.\n"), 0o644)
	if n, err := s.Reread(context.Background()); err != nil || n != 1 {
		t.Errorf("read again: %d %v", n, err)
	}
	if n, _ := s.Reread(context.Background()); n != 0 {
		t.Errorf("read again, when nothing's moved: %d", n)
	}
	if _, err := s.Write(p.ID, doc.Version([]byte("# Plan C\n\nThird way.\n")), []byte("# Plan C\n\nFourth.\n")); err != nil {
		t.Errorf("a save from what it is now: %v", err)
	}

	os.Rename(s.Path(p.ID), s.Path(p.ID)+".aside")
	if list, _ := s.List(); len(list) != 0 {
		t.Errorf("listed with its file gone: %+v", list)
	}
	if names, _ := found(t, root, "fourth"); len(names) != 0 {
		t.Errorf("its words with its file gone: %v", names)
	}
	if q, _ := s.New(nil); q.ID != "p2" {
		t.Errorf("its number was used again: %s", q.ID)
	}
	os.Rename(s.Path(p.ID)+".aside", s.Path(p.ID))
	if list, _ := s.List(); len(list) != 2 {
		t.Errorf("its file back: %+v", list)
	}
	if names, _ := found(t, root, "fourth"); !slices.Equal(names, []string{p.ID}) {
		t.Errorf("its words with its file back: %v", names)
	}
}

// A page is listed by when its text last changed, latest first; what's
// kept beside it doesn't move it.
func TestList_LatestChangedFirst(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	s := Store{Root: root}
	a, _ := s.New([]byte("# A\n"))
	b, _ := s.New([]byte("# B\n"))
	old := time.Now().Add(-time.Hour)
	os.Chtimes(s.Path(b.ID), old, old) // b, as if written an hour ago: taken in
	if list, _ := s.List(); len(list) != 2 || list[0].ID != a.ID {
		t.Fatalf("listed: %+v", list)
	}
	if err := s.SetMeta(b.ID, Meta{Kind: "adr"}); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(); list[0].ID != a.ID || list[1].Kind != "adr" {
		t.Errorf("after b's meta: %+v", list)
	}
	text, _, _ := s.Read(b.ID)
	s.Write(b.ID, doc.Version(text), []byte("# B\n\nMore.\n"))
	if list, _ := s.List(); list[0].ID != b.ID {
		t.Errorf("after b was saved: %+v", list)
	}
}
