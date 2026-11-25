package ops

import (
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// keptFixture is spaceFixture's workspace with its worktrees kept, as the
// daemon keeps them, and looked at often enough for a test.
func keptFixture(t *testing.T) (*Workspace, func(dir string, args ...string)) {
	t.Helper()
	for _, set := range []struct {
		v  *time.Duration
		to time.Duration
	}{{&gitFoldersEvery, 50 * time.Millisecond}, {&dirtyFloor, 100 * time.Millisecond}, {&capsuleSweepEvery, time.Hour}} {
		was := *set.v
		*set.v = set.to
		t.Cleanup(func() { *set.v = was })
	}
	w, run := spaceFixture(t)
	stop, done := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		close(stop)
		<-done
	})
	go func() {
		defer close(done)
		w.WatchGit(stop, t.Logf)
	}()
	waitFor(t, "every worktree read", func() bool {
		wts, err := w.Worktrees("records")
		if err != nil || len(wts) != 2 || !w.caps.keeping.Load() {
			return false
		}
		for _, wt := range wts {
			if wt.Unread {
				return false
			}
		}
		return true
	})
	return w, run
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("never: %s", what)
}

func worktreeNamed(t *testing.T, w *Workspace, name string) (Worktree, bool) {
	t.Helper()
	wts, err := w.Worktrees("records")
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range wts {
		if wt.Name == name {
			return wt, true
		}
	}
	return Worktree{}, false
}

// A page's read of the repos and their worktrees is a lookup of what the
// daemon keeps: it starts no git, however often it's asked.
func TestCapsuleKeeper_ReadsStartNoGit(t *testing.T) {
	w, _ := keptFixture(t)
	before := git.Started()
	for range 5 {
		w.Repos()
		w.Worktrees("records")
		w.RefsKnown(t.Context(), false)
		w.Stamp()
	}
	if n := git.Started() - before; n != 0 {
		t.Fatalf("reads started %d git processes, want none", n)
	}
	repos := w.Repos()
	if len(repos) != 1 || !repos[0].Cloned || !slices.Equal(repos[0].Capsules, []string{"note"}) || repos[0].Branches["note"] != "note" {
		t.Fatalf("repos = %+v", repos)
	}
	ground, _ := worktreeNamed(t, w, ".ground")
	if !ground.Ground || ground.Branch != "main" {
		t.Fatalf("ground = %+v", ground)
	}
}

// What git writes in a capsule moves what's kept of it, and the pages
// are told (Seen); a file edited moves nothing git keeps, but a capsule a
// page has shown is asked again whether it's dirty.
func TestCapsuleKeeper_FollowsGit(t *testing.T) {
	w, run := keptFixture(t)
	seen := w.Seen()
	drain := func() {
		for {
			select {
			case <-seen:
			default:
				return
			}
		}
	}
	drain()
	stamp := w.Stamp()

	run("note", "commit", "-q", "--allow-empty", "-m", "a commit")
	waitFor(t, "the commit kept", func() bool {
		wt, _ := worktreeNamed(t, w, "note")
		return wt.Ahead == 1 && !wt.Unread
	})
	select {
	case <-seen:
	default:
		t.Error("the pages weren't told")
	}
	if w.Stamp() == stamp {
		t.Error("the workspace's stamp didn't move")
	}

	writeFile(t, filepath.Join(w.WS().RepoDir("records"), "note", "draft.md"), "# Draft\n")
	waitFor(t, "an edit found, the capsule being looked at", func() bool {
		wt, _ := worktreeNamed(t, w, "note")
		return wt.Dirty
	})
}

// A capsule lifted is kept once the repos' folders are listed again, and
// one burned is gone.
func TestCapsuleKeeper_CapsulesLiftedAndBurned(t *testing.T) {
	w, run := keptFixture(t)
	repo := w.WS().RepoDir("records")
	run(".bare", "worktree", "add", "-q", "-b", "later", filepath.Join(repo, "later"), "origin/main")
	waitFor(t, "the capsule lifted kept, and read", func() bool {
		wt, ok := worktreeNamed(t, w, "later")
		return ok && wt.Branch == "later" && !wt.Unread
	})
	if got := w.Repos()[0].Capsules; !slices.Equal(got, []string{"later", "note"}) {
		t.Fatalf("capsules = %q", got)
	}
	run(".bare", "worktree", "remove", "--force", filepath.Join(repo, "later"))
	os.RemoveAll(filepath.Join(repo, "later"))
	waitFor(t, "the capsule burned gone", func() bool {
		_, ok := worktreeNamed(t, w, "later")
		return !ok
	})
}

// The find index reads a ground's HEAD from what's kept: looking at the
// grounds again, as it does every couple of seconds while they're asked
// for, starts no git when they haven't moved.
func TestCapsuleKeeper_GroundsLookedAtWithoutGit(t *testing.T) {
	w, _ := keptFixture(t)
	ix := w.FindIndex()
	var now atomic.Int64 // the index reads it behind, as it looks
	now.Store(time.Now().UnixNano())
	ix.Now = func() time.Time { return time.Unix(0, now.Load()) }
	ix.GroundsKept()
	waitFor(t, "the ground read", func() bool { return len(ix.GroundsKept()) == 1 })
	before := git.Started()
	for range 3 {
		now.Add(int64(time.Minute)) // every look is due
		ix.GroundsKept()
		time.Sleep(20 * time.Millisecond) // the look, behind
	}
	if n := git.Started() - before; n != 0 {
		t.Fatalf("looking at the grounds again started %d git processes, want none", n)
	}
}
