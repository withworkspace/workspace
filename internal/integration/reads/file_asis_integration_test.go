package reads_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cost"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/testutil"
)

// A file the explorer opens is read with no git when the capsule's tree,
// read before it, lists it unchanged: nothing to mark. One that's changed
// is marked as before, and one written since the tree was read is read
// with git, its changes marked.
func TestReads_AFileUnchangedIsReadAsItIs(t *testing.T) {
	var gits atomic.Int64
	stop := cost.Observe(func(l cost.Line) {
		if l.Tag == "file" {
			gits.Add(l.Costs["git"].N)
		}
	})
	t.Cleanup(stop)
	w, _ := testutil.OrchestrateFixture(t)
	dir := testutil.CapsuleDir(w, "backend", "cache-lookup")
	testutil.CommitIn(t, dir, "cache.go", "package cache\n", "add the cache")
	testutil.CommitIn(t, dir, "notes.md", "one\ntwo\n", "notes")
	s := testutil.WebServer(t, w.Root)
	base := s.Info().Origin() + "/api/repos/backend/worktrees/cache-lookup/"
	get := func(path, tag string, v any) {
		t.Helper()
		req, _ := http.NewRequest("GET", base+path, nil)
		req.Header.Set("Authorization", "Bearer "+s.Info().Token)
		req.Header.Set(cost.TagHeader, tag)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		json.NewDecoder(res.Body).Decode(v)
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		var tree struct {
			Unread bool              `json:"unread"`
			Files  []review.TreeFile `json:"files"`
		}
		get("tree", "tree", &tree)
		if !tree.Unread && len(tree.Files) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the tree was never read")
		}
	}

	var f review.FileView
	get("file?path=README.md", "file", &f)
	if len(f.Lines) != 1 || f.Lines[0] != "hello" || len(f.Changes) != 0 {
		t.Errorf("README.md: %+v", f)
	}
	if n := gits.Load(); n != 0 {
		t.Errorf("an unchanged file read with %d git", n)
	}

	f = review.FileView{}
	get("file?path=cache.go", "changed", &f)
	if len(f.Changes) != 1 || f.Changes[0].Kind != "added" {
		t.Errorf("cache.go, added since the branch point: %+v", f.Changes)
	}

	// Written since the tree was read (not committed): its changes are read.
	time.Sleep(20 * time.Millisecond)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\nworld\n"), 0o644)
	f = review.FileView{}
	get("file?path=README.md", "edited", &f)
	if len(f.Lines) != 2 || len(f.Changes) != 1 {
		t.Errorf("README.md, edited since the tree was read: %+v", f)
	}
}
