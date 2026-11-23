package ops

import (
	"os"
	"os/exec"
	"testing"

	"github.com/withworkspace/workspace/internal/git"
)

// The process page reads where the process is committed each time it's
// opened: git's asked once, and again only when the repo moves or the
// file's written.
func TestProcess_AsksGitOnlyWhenItsMoved(t *testing.T) {
	w, _ := fixture(t, "")
	root := w.WS().Root
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	file := w.WS().Process()
	os.WriteFile(file, []byte("# How we work\n"), 0o644)
	run("init", "-q", "--initial-branch=main")
	run("add", "--", file)
	run("commit", "-q", "-m", "the process")

	first, err := w.Process()
	if err != nil || first.Commit == "" || first.Uncommitted {
		t.Fatalf("committed: %+v, %v", first, err)
	}
	before := git.Started()
	for range 3 {
		if again, _ := w.Process(); again != first {
			t.Fatalf("read again %+v, was %+v", again, first)
		}
	}
	if w.ProcessText() != "# How we work\n" {
		t.Errorf("text = %q", w.ProcessText())
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("reads of a process that hasn't moved started git %d times, want none", n)
	}

	// Written, and not committed: it says so.
	os.WriteFile(file, []byte("# How we work\n\nIn small steps.\n"), 0o644)
	if v, _ := w.Process(); !v.Uncommitted || v.Commit != first.Commit {
		t.Errorf("edited: %+v", v)
	}
	// Committed: it's that commit's.
	run("commit", "-q", "-am", "small steps")
	if v, _ := w.Process(); v.Uncommitted || v.Commit == first.Commit || v.Commit == "" {
		t.Errorf("committed again: %+v, was %+v", v, first)
	}
}
