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
	os.WriteFile(filepath.Join(dir, "ws.repo.toml"), []byte("[run.service.web]\ncommand = \"sleep 300\"\n\n[run.service.api]\ncommand = \"sleep 300\"\n"), 0o644)
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
	d, err := w.Mission("cache")
	if err != nil {
		t.Fatal(err)
	}
	if got := d.CapsuleRuns["frontend/cache"]; len(got) != 1 || got[0].Name != "web" {
		t.Errorf("the page's runs: %v", got)
	}
	if got := d.Recipes["frontend/cache"]; strings.Join(got, ",") != "api,web" {
		t.Errorf("the page's recipes: %v", got)
	}
	if err := w.LookedAt(runs.Looking{Missions: map[string]bool{"cache": true}}); err != nil {
		t.Fatal(err)
	}
	if err := w.StopRun("cache", r.Ref()); err != nil {
		t.Fatal(err)
	}
	if list, _ := w.runStore().List(); len(list) != 0 {
		t.Errorf("after Stop: %v", list)
	}
}

// A ground's or capsule's recipe, run from the Files page, lasts while
// someone has it open there: no mission's needed, and the page sees it.
func TestRunIn(t *testing.T) {
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache")
	ground := w.WS().MainWorktree("frontend")
	os.MkdirAll(ground, 0o755)
	os.WriteFile(filepath.Join(ground, "ws.repo.toml"), []byte("[run.service.web]\ncommand = \"sleep 300\"\n\n[run.service.stack]\ncommand = \"sleep 300\"\nwhere = \"silo\"\n\n[run.task.lint]\ncommand = \"true\"\n"), 0o644)
	t.Cleanup(func() {
		list, _ := w.runStore().List()
		for _, r := range list {
			w.runStore().End(r)
		}
	})
	old := RunUpWait
	RunUpWait = 100 * time.Millisecond // sleep never listens
	t.Cleanup(func() { RunUpWait = old })

	if _, err := w.RunIn("frontend", "nope", "web", false, AsYou); !errors.Is(err, ErrNotFound) {
		t.Errorf("a capsule the repo hasn't: %v", err)
	}
	if _, err := w.RunIn("nope", ".ground", "web", false, AsYou); !errors.Is(err, ErrNotFound) {
		t.Errorf("a repo the workspace hasn't: %v", err)
	}
	r, err := w.RunIn("frontend", ".ground", "web", false, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if r.Owner.Kind != runs.ByLooking || r.Owner.Mission != "" || r.Ref() != "web.ground.frontend" {
		t.Fatalf("run %+v", r)
	}
	in, err := w.RunsIn("frontend", ".ground")
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Runs) != 1 || in.Runs[0].Name != "web" {
		t.Errorf("the page's runs: %v", in.Runs)
	}
	var names, tasks []string
	for _, rc := range in.Recipes {
		names = append(names, rc.Name)
		if rc.Task {
			tasks = append(tasks, rc.Name)
		}
	}
	if strings.Join(names, ",") != "lint,web" || strings.Join(tasks, ",") != "lint" {
		t.Errorf("the page's recipes, the silo's left out, and its tasks: %v, %v", names, tasks)
	}
	if err := w.LookedAt(runs.Looking{Worktrees: map[string]bool{"frontend/.ground": true}}); err != nil {
		t.Fatal(err)
	}
	if err := w.EndRun(r.Ref()); err != nil {
		t.Fatal(err)
	}
	// A task isn't the lookers': it runs to its end, the page open or not.
	task, err := w.RunIn("frontend", ".ground", "lint", false, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if !task.IsTask() || task.Owner.Kind != runs.ByNone || task.NRef() != "lint.ground.frontend#1" {
		t.Errorf("task %+v", task.Record)
	}
	// Run again, the first is kept behind it: the page has both, what's
	// been run there lately.
	if _, done, err := w.runStore().WaitEnd(task, 10*time.Second, nil); err != nil || !done {
		t.Fatalf("lint didn't end: %v", err)
	}
	again, err := w.RunIn("frontend", ".ground", "lint", false, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if _, done, err := w.runStore().WaitEnd(again, 10*time.Second, nil); err != nil || !done {
		t.Fatalf("lint didn't end again: %v", err)
	}
	in, err = w.RunsIn("frontend", ".ground")
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Runs) != 1 || in.Runs[0].NRef() != "lint.ground.frontend#2" {
		t.Errorf("the page's runs, the latest of lint: %v", in.Runs)
	}
	if len(in.Past) != 1 || in.Past[0].NRef() != "lint.ground.frontend#1" {
		t.Errorf("the runs of lint behind it: %v", in.Past)
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
