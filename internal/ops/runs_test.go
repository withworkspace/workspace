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
