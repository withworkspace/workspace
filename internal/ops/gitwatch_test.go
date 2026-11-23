package ops

import (
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// watchedGit starts the watch of a workspace's git folders, and gives
// how many times it's said one moved since last asked (waiting up to a
// second for the first), and a way to wait until n folders are watched.
func watchedGit(t *testing.T, w *Workspace) (moved func() int, watching func(n int)) {
	t.Helper()
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	said := make(chan struct{}, 64)
	folders := make(chan int, 64)
	go w.watchGit(stop, t.Logf, func() { said <- struct{}{} }, func(n int) {
		select {
		case folders <- n:
		default:
		}
	})
	moved = func() int {
		n := 0
		wait := time.After(time.Second)
		for {
			select {
			case <-said:
				n++
				wait = time.After(2 * gitGap) // any that follow it
			case <-wait:
				return n
			}
		}
	}
	watching = func(n int) {
		t.Helper()
		for deadline := time.After(5 * time.Second); ; {
			select {
			case got := <-folders:
				if got == n {
					return
				}
			case <-deadline:
				t.Fatalf("never watching %d folders", n)
			}
		}
	}
	return moved, watching
}

// The daemon watches where git writes, and says when it has: a commit in
// a capsule, something staged, a fetch. What git doesn't write to says
// nothing, and a capsule lifted later is watched once the folders are
// listed again.
func TestWatchGit_SaysWhenGitWritesInACapsuleOrItsRepo(t *testing.T) {
	was := gitFoldersEvery
	gitFoldersEvery = 50 * time.Millisecond
	t.Cleanup(func() { gitFoldersEvery = was })
	w, run := spaceFixture(t)
	repo := w.WS().RepoDir("records")

	want := []string{filepath.Join(repo, ".bare"), filepath.Join(repo, ".bare", "worktrees", "-ground"), filepath.Join(repo, ".bare", "worktrees", "note")} // git's name for .ground's
	got := gitWatchFolders(w.WS())
	for i := range got {
		got[i], _ = filepath.EvalSymlinks(got[i])
	}
	for i := range want {
		want[i], _ = filepath.EvalSymlinks(want[i])
	}
	if !slices.Equal(got, want) {
		t.Fatalf("git's folders:\n got  %q\n want %q", got, want)
	}

	moved, watching := watchedGit(t, w)
	watching(3)
	if n := moved(); n != 0 {
		t.Fatalf("said %d times that something moved, with nothing done", n)
	}

	run("note", "commit", "-q", "--allow-empty", "-m", "a commit")
	if n := moved(); n == 0 {
		t.Error("a commit in a capsule: nothing said")
	}
	writeFile(t, filepath.Join(repo, "note", "new.md"), "# New\n")
	if n := moved(); n != 0 {
		t.Errorf("a file written and not staged: said %d times, want none (git wrote nothing)", n)
	}
	run("note", "add", "new.md")
	if n := moved(); n == 0 {
		t.Error("a file staged: nothing said")
	}
	run(".bare", "fetch", "-q", ".", "main") // from itself: what a fetch writes, with no remote to ask
	if n := moved(); n == 0 {
		t.Error("a fetch: nothing said")
	}

	// A capsule lifted since: watched once the folders are listed again.
	run(".bare", "worktree", "add", "-q", "-b", "later", filepath.Join(repo, "later"), "origin/main")
	watching(4)
	moved() // what lifting it wrote
	run("later", "commit", "-q", "--allow-empty", "-m", "in the new one")
	if n := moved(); n == 0 {
		t.Error("a commit in a capsule lifted since: nothing said")
	}
}
