package ops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/runs"
)

// A capsule's recipe, run from its mission's page, lasts while someone has
// the mission open; the page sees it, and Stop ends it.
func TestRunRecipe(t *testing.T) {
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache")
	dir := filepath.Join(w.WS().RepoDir("frontend"), "cache")
	os.WriteFile(filepath.Join(dir, "ws.repo.toml"), []byte("[run.web]\ncommand = \"sleep 300\"\n\n[run.api]\ncommand = \"sleep 300\"\n"), 0o644)
	t.Cleanup(func() {
		list, _ := w.runStore().List()
		for _, r := range list {
			w.runStore().End(r)
		}
	})
	old := RunUpWait
	RunUpWait = 100 * time.Millisecond // sleep never listens
	t.Cleanup(func() { RunUpWait = old })

	if _, err := w.RunRecipe("cache", "frontend", "cache", "nope", AsYou); !strings.Contains(err.Error(), "no recipe nope") {
		t.Errorf("an unknown recipe: %v", err)
	}
	if _, err := w.RunRecipe("cache", "frontend", "other", "web", AsYou); !errors.Is(err, ErrNotFound) {
		t.Errorf("a capsule not the mission's: %v", err)
	}
	r, err := w.RunRecipe("cache", "frontend", "cache", "web", AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if r.Owner.Kind != runs.ByLooking || r.Owner.Mission != "cache" || r.Ref() != "web.cache.frontend" {
		t.Fatalf("run %+v", r)
	}
	d, err := w.Mission("cache", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.CapsuleRuns["frontend/cache"]; len(got) != 1 || got[0].Name != "web" {
		t.Errorf("the page's runs: %v", got)
	}
	if got := d.Recipes["frontend/cache"]; strings.Join(got, ",") != "api,web" {
		t.Errorf("the page's recipes: %v", got)
	}
	if err := w.LookedAt(map[string]bool{"cache": true}); err != nil {
		t.Fatal(err)
	}
	if err := w.StopRun("cache", r.Ref()); err != nil {
		t.Fatal(err)
	}
	if list, _ := w.runStore().List(); len(list) != 0 {
		t.Errorf("after Stop: %v", list)
	}
}

// The Runs page: every run listed, its output read, and any one ended by
// its full ref (a name alone isn't guessed at).
func TestRunsPage(t *testing.T) {
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache")
	dir := filepath.Join(w.WS().RepoDir("frontend"), "cache")
	r, err := w.runStore().Start(runs.Start{
		Name: "web", Repo: "frontend", Worktree: "cache", Dir: dir,
		Command: "echo hello from web; sleep 300", Owner: runs.Owner{Kind: runs.ByNone}, TTL: time.Hour,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.runStore().End(r) })

	list, err := w.Runs()
	if err != nil || len(list) != 1 || list[0].Ref() != "web.cache.frontend" {
		t.Fatalf("Runs = %v, %v", list, err)
	}
	var log string
	for range 50 {
		if log, _ = w.RunLog(r.Ref(), 10); log != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(log, "hello from web") {
		t.Errorf("RunLog = %q", log)
	}
	if err := w.EndRun("web"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a name alone: %v", err)
	}
	if err := w.EndRun(r.Ref()); err != nil {
		t.Fatal(err)
	}
	if list, _ := w.Runs(); len(list) != 0 {
		t.Errorf("after EndRun: %v", list)
	}
	if _, err := w.RunLog(r.Ref(), 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("an ended run's log: %v", err)
	}
}
