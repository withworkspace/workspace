package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/runs"
)

// The daemon keeps where the runs stand: a run started, or ended, is
// said as its files are written, not at the next poll, and the stamps the
// pages read are then what's kept, the same as the files say.
func TestWatchRuns_SaysARunStartedAndEnded(t *testing.T) {
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache")
	dir := filepath.Join(w.WS().RepoDir("frontend"), "cache")
	os.WriteFile(filepath.Join(dir, "ws.repo.toml"), []byte("[run.service.web]\ncommand = \"sleep 300\"\n"), 0o644)
	t.Cleanup(func() {
		list, _ := w.runStore().List()
		for _, r := range list {
			w.runStore().End(r)
		}
	})
	old := RunUpWait
	RunUpWait = 100 * time.Millisecond // sleep never listens
	t.Cleanup(func() { RunUpWait = old })

	stop := make(chan struct{})
	moved := make(chan struct{}, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.WatchRuns(stop, t.Logf, func() { moved <- struct{}{} })
	}()
	t.Cleanup(func() { close(stop); <-done })
	for deadline := time.Now().Add(5 * time.Second); !w.runsKept.Keeping(); {
		if time.Now().After(deadline) {
			t.Fatal("never kept")
		}
		time.Sleep(10 * time.Millisecond)
	}
	said := func(what string) {
		t.Helper()
		select {
		case <-moved:
		case <-time.After(3 * time.Second):
			t.Fatalf("%s, and nothing was said", what)
		}
		for { // and whatever follows it
			select {
			case <-moved:
				continue
			case <-time.After(300 * time.Millisecond):
			}
			break
		}
	}
	files := func() string { return w.runStore().StampIn("frontend", "cache") }

	r, err := w.RunRecipe("cache", "frontend", "cache", "web", AsYou)
	if err != nil {
		t.Fatal(err)
	}
	said("a run started")
	if got, want := w.runsStampIn("frontend", "cache"), files(); got != want {
		t.Errorf("kept %q, the files say %q", got, want)
	}
	if got, want := w.RunsStamp(), w.runStore().Stamp(); got != want {
		t.Errorf("all kept %q, the files say %q", got, want)
	}
	if _, ok := w.runningKept("frontend", "cache", "web"); !ok {
		t.Error("not kept as running")
	}

	if err := w.StopRun("cache", r.Ref()); err != nil {
		t.Fatal(err)
	}
	said("a run ended")
	if got := w.runsStampIn("frontend", "cache"); got != "" {
		t.Errorf("ended and cleared away, and its place still says %q", got)
	}
	if _, ok := w.runningKept("frontend", "cache", "web"); ok {
		t.Error("ended, and still kept as running")
	}
}
