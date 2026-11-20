package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// capsuleFixture is a capsule as ws makes one: a linked worktree of a bare
// clone, on a branch of its own off origin/main. It gives the capsule's
// folder, and a way to run git in any folder under the test's.
func capsuleFixture(t *testing.T) (capsule string, run func(dir string, args ...string) string) {
	t.Helper()
	root := t.TempDir()
	run = func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", filepath.Join(root, dir), "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	for _, d := range []string{"origin.git", "seed", "repo"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	run("origin.git", "init", "-q", "--bare", "--initial-branch=main")
	run("seed", "init", "-q", "--initial-branch=main")
	run("seed", "commit", "-q", "--allow-empty", "-m", "start")
	run("seed", "remote", "add", "origin", filepath.Join(root, "origin.git"))
	run("seed", "push", "-q", "origin", "main")
	run("repo", "clone", "-q", "--bare", filepath.Join(root, "origin.git"), ".bare")
	run("repo/.bare", "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	run("repo/.bare", "fetch", "-q", "origin")
	run("repo/.bare", "worktree", "add", "-q", "-b", "work", filepath.Join(root, "repo", "work"), "origin/main")
	return filepath.Join(root, "repo", "work"), run
}

// everything asks a capsule's git for all a view shows of it.
type capsuleSaid struct {
	head, branch  string
	upstream      bool
	ahead, behind int
	since         int
	dirty         bool
}

func everything(g capsuleGit, dir string) capsuleSaid {
	var s capsuleSaid
	s.head, s.branch = g.headBranch(dir)
	s.upstream = g.hasUpstream(dir)
	s.ahead, s.behind = g.aheadBehind(dir)
	s.since = g.commitsSince(dir, "origin/main")
	s.dirty = g.dirty(dir)
	return s
}

func TestKeptGit_AsksNothingOfACapsuleThatHasntMoved(t *testing.T) {
	dir, _ := capsuleFixture(t)
	g := &keptGit{}
	first := everything(g, dir)
	if want := everything(askedGit{}, dir); first != want {
		t.Fatalf("kept %+v, git says %+v", first, want)
	}
	if first.head == "" || first.branch != "work" {
		t.Fatalf("head and branch = %q, %q", first.head, first.branch)
	}
	before := git.Started()
	for range 5 {
		if again := everything(g, dir); again != first {
			t.Fatalf("read again %+v, was %+v", again, first)
		}
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("5 reads of a capsule that hasn't moved started git %d times, want none", n)
	}
}

func TestKeptGit_AsksAgainWhenTheCapsuleMoves(t *testing.T) {
	dir, run := capsuleFixture(t)
	g := &keptGit{}
	check := func(what string) capsuleSaid {
		t.Helper()
		got, want := everything(g, dir), everything(askedGit{}, dir)
		if got != want {
			t.Fatalf("after %s: kept %+v, git says %+v", what, got, want)
		}
		return got
	}
	start := check("nothing")

	// A commit: its HEAD, and how far it is from main.
	run("repo/work", "commit", "-q", "--allow-empty", "-m", "one")
	if s := check("a commit"); s.head == start.head || s.since != 1 {
		t.Errorf("after a commit: %+v, started at %+v", s, start)
	}

	// Something staged: it's dirty.
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	run("repo/work", "add", "a.txt")
	if s := check("staging a file"); !s.dirty {
		t.Errorf("after staging a file it isn't dirty: %+v", s)
	}
	run("repo/work", "commit", "-q", "-m", "two")

	// Pushed, with an upstream set: it has one, and is level with it.
	run("repo/work", "push", "-q", "-u", "origin", "work")
	if s := check("a push"); !s.upstream || s.ahead != 0 {
		t.Errorf("after a push: %+v", s)
	}

	// Its branch moves on the remote, and is fetched (in the repo, not the
	// capsule: nothing of the capsule's own is written).
	run("seed", "fetch", "-q", "origin", "work")
	run("seed", "checkout", "-q", "-b", "work", "FETCH_HEAD")
	run("seed", "commit", "-q", "--allow-empty", "-m", "theirs")
	run("seed", "push", "-q", "origin", "work")
	run("repo/.bare", "fetch", "-q", "origin")
	if s := check("a fetch"); s.behind != 1 {
		t.Errorf("after a fetch of its branch, moved on: %+v", s)
	}

	// Another branch checked out.
	run("repo/work", "checkout", "-q", "-b", "other")
	if s := check("a checkout"); s.branch != "other" || s.upstream {
		t.Errorf("after a checkout: %+v", s)
	}
}

// A file edited moves nothing git keeps: it's shown as last seen, looked
// at behind the read once that's a while ago, and the stamp a page is
// told by moves when the look finds it changed.
func TestKeptGit_LooksAgainForDirtBehindTheRead(t *testing.T) {
	dir, _ := capsuleFixture(t)
	g := &keptGit{}
	if g.dirty(dir) {
		t.Fatal("a fresh capsule is dirty")
	}
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644)

	// Within the floor: as it was, and git isn't asked.
	before := git.Started()
	if g.dirty(dir) {
		t.Error("dirty within the floor: git was asked")
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("a read within the floor started git %d times", n)
	}
	if moved := g.dirtyMoved(dir); moved != "" {
		t.Errorf("nothing's been found changed, and dirtyMoved = %q", moved)
	}

	// Past it: the read is answered as it was, and a look follows.
	was := dirtyFloor
	dirtyFloor = 0
	defer func() { dirtyFloor = was }()
	deadline := time.Now().Add(10 * time.Second)
	for !g.dirty(dir) {
		if time.Now().After(deadline) {
			t.Fatal("never found dirty")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if moved := g.dirtyMoved(dir); !strings.HasPrefix(moved, dir+" ") {
		t.Errorf("found dirty, and dirtyMoved = %q", moved)
	}
	if all := g.dirtyMoved(); all != g.dirtyMoved(dir) {
		t.Errorf("of all it keeps, dirtyMoved = %q", all)
	}
}

func TestKeptGit_ForgetsACapsuleThatsGone(t *testing.T) {
	dir, _ := capsuleFixture(t)
	g := &keptGit{}
	everything(g, dir)
	os.RemoveAll(dir)
	if head, _ := g.headBranch(dir); head != "" {
		t.Errorf("a capsule that's gone has a head: %q", head)
	}
	g.dirtyMoved()
	if len(g.of) != 0 {
		t.Errorf("still keeps %d capsules", len(g.of))
	}
}

// movedSince waits for a look behind a read to find the capsule other
// than it was (dirtyMoved moves from was).
func movedSince(t *testing.T, g *keptGit, dir, was string) string {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if now := g.dirtyMoved(dir); now != was {
			return now
		}
	}
	t.Fatalf("no look behind the read found %s changed", dir)
	return ""
}

func TestKeptGit_SoonAnswersWithoutWaitingAndSaysWhenItsLooked(t *testing.T) {
	dir, run := capsuleFixture(t)
	g := &keptGit{}
	ahead := func() (aheadAndBehind, bool) {
		return soon(g, dir, "ahead", func() answer[aheadAndBehind] { return askAheadBehind(dir) })
	}
	before := git.Started()
	if _, ok := ahead(); ok {
		t.Fatal("a capsule never asked of was said known")
	}
	if _, ok := g.dirtySoon(dir); ok {
		t.Fatal("a capsule never asked of was said known to be dirty or not")
	}
	// Both looks run behind the reads, each in its own slot: each counts
	// once it's in (a first answer is news), and they may land apart.
	moved := movedSince(t, g, dir, "")
	if moved != fmt.Sprintf("%s 2\n", dir) {
		moved = movedSince(t, g, dir, moved)
	}
	if ab, ok := ahead(); !ok || ab != (aheadAndBehind{}) {
		t.Fatalf("after the look: %+v, %v; want 0 ahead, known", ab, ok)
	}
	if dirty, ok := g.dirtySoon(dir); !ok || dirty {
		t.Fatalf("after the look: dirty %v, known %v; want clean, known", dirty, ok)
	}
	if n := git.Started() - before; n != 2 {
		t.Errorf("git started %d times, want 2 (one look for each)", n)
	}

	// A commit moves the capsule: the read's told what was, at once, and
	// the look behind it says it's moved.
	run("repo/work", "commit", "-q", "--allow-empty", "-m", "more")
	if ab, ok := ahead(); !ok || ab.ahead != 0 {
		t.Fatalf("read after a commit: %+v, %v; want what was (0 ahead), known", ab, ok)
	}
	movedSince(t, g, dir, moved)
	if ab, ok := ahead(); !ok || ab.ahead != 1 {
		t.Fatalf("after the look: %+v, %v; want 1 ahead", ab, ok)
	}
}

func TestBranchOf_ReadsHEADWithoutGit(t *testing.T) {
	dir, run := capsuleFixture(t)
	before := git.Started()
	if got := branchOf(dir); got != "work" {
		t.Errorf("branchOf = %q, want work", got)
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("branchOf started git %d times, want none", n)
	}
	run("repo/work", "checkout", "-q", "--detach")
	if got := branchOf(dir); got != "HEAD" {
		t.Errorf("detached: branchOf = %q, want HEAD", got)
	}
	if got := branchOf(t.TempDir()); got != "" {
		t.Errorf("not a worktree: branchOf = %q, want empty", got)
	}
}
