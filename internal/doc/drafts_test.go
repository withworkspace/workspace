package doc

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A document's draft is kept with its mission, one per document, until
// it's dropped.
func TestDocDrafts(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(missionDir(root, "cache"), 0o755)
	s := Drafts{Root: root, Scope: OfMission("cache")}
	ref := "backend/cache:docs/proposal.md"
	if d, err := s.Get(ref); d != nil || err != nil {
		t.Fatalf("none yet: %+v, %v", d, err)
	}
	at := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	if err := s.Save(Draft{Ref: ref, Base: "dcf9d7a5559d", Text: "# Cache\n\nPer tenant.\n", At: at}); err != nil {
		t.Fatal(err)
	}
	s.Save(Draft{Ref: "brief", Base: "000000000000", Text: "other", At: at})
	if err := s.Save(Draft{Ref: ref, Base: "dcf9d7a5559d", Text: "# Cache\n\nPer tenant, keyed by token.\n", At: at}); err != nil {
		t.Fatal(err)
	}
	d, err := s.Get(ref)
	if err != nil || d == nil || d.Text != "# Cache\n\nPer tenant, keyed by token.\n" || d.Base != "dcf9d7a5559d" || !d.At.Equal(at) {
		t.Fatalf("draft = %+v, %v", d, err)
	}
	if err := s.Drop(ref); err != nil {
		t.Fatal(err)
	}
	if d, _ := s.Get(ref); d != nil {
		t.Errorf("dropped, but still %+v", d)
	}
	if d, _ := s.Get("brief"); d == nil || d.Text != "other" {
		t.Errorf("another document's draft went too: %+v", d)
	}
	if err := s.Drop(ref); err != nil {
		t.Errorf("dropping none: %v", err)
	}
	if err := (Drafts{Root: root, Scope: OfMission("no-such-mission")}).Save(Draft{Ref: ref}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a draft for no mission: %v", err)
	}
}

// The slug is a folder's name on the way to the draft, and comes from a
// URL: one that climbs out of the missions folder is no mission, for
// reading, writing or dropping.
func TestDocDrafts_ASlugThatIsntOne(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(missionDir(root, "cache"), 0o755)
	for _, slug := range []string{"..", "../..", "cache/..", "Cache", "."} {
		if _, err := (Drafts{Root: root, Scope: OfMission(slug)}).Get("brief"); !errors.Is(err, ErrNotFound) {
			t.Errorf("reading %q: %v", slug, err)
		}
		if err := (Drafts{Root: root, Scope: OfMission(slug)}).Save(Draft{Ref: "brief", Base: "0"}); !errors.Is(err, ErrNotFound) {
			t.Errorf("saving %q: %v", slug, err)
		}
		if err := (Drafts{Root: root, Scope: OfMission(slug)}).Drop("brief"); !errors.Is(err, ErrNotFound) {
			t.Errorf("dropping %q: %v", slug, err)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(root, ".ws")); len(entries) != 1 {
		t.Errorf("something was written outside .ws/missions: %v", entries)
	}
}
