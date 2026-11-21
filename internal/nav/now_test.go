package nav

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNow_TheFilesAsTheyAre(t *testing.T) {
	dir, head := repoWith(t, map[string]string{
		"a.go":       "package a\n\nfunc F() {}\n",
		".gitignore": "out/\n",
	})
	s := New("")

	// Nothing changed: HEAD.
	if rev, err := s.Now(dir); err != nil || rev != head {
		t.Fatalf("clean: %q %v, want %s", rev, err, head)
	}

	// Edited, added (not ignored), and ignored files: a commit with the
	// first two and HEAD as its parent.
	write := func(p, text string) {
		t.Helper()
		os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, p), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.go", "package a\n\nfunc F() {}\n\nfunc G() { F() }\n")
	write("b.go", "package a\n\nvar B = G\n")
	write("out/c.go", "package out\n")
	statusBefore := run(t, dir, "status", "--porcelain")
	indexBefore, _ := os.ReadFile(filepath.Join(dir, ".git", "index"))

	rev, err := s.Now(dir)
	if err != nil || rev == head {
		t.Fatalf("changed: %q %v", rev, err)
	}
	if parent := run(t, dir, "rev-parse", rev+"^"); parent != head {
		t.Errorf("parent = %s, want %s", parent, head)
	}
	if got := run(t, dir, "show", rev+":a.go"); got != "package a\n\nfunc F() {}\n\nfunc G() { F() }" {
		t.Errorf("a.go at it = %q", got)
	}
	if got := run(t, dir, "show", rev+":b.go"); got != "package a\n\nvar B = G" {
		t.Errorf("b.go at it = %q", got)
	}
	if run(t, dir, "ls-tree", "-r", "--name-only", rev, "out") != "" {
		t.Error("an ignored file is in it")
	}

	// The worktree, its index and its branch are as they were.
	if got := run(t, dir, "status", "--porcelain"); got != statusBefore {
		t.Errorf("status = %q, was %q", got, statusBefore)
	}
	if indexAfter, _ := os.ReadFile(filepath.Join(dir, ".git", "index")); string(indexAfter) != string(indexBefore) {
		t.Error("the index changed")
	}
	if got := run(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD = %s", got)
	}

	// The same files: the same commit, kept or made again.
	if again, _ := s.Now(dir); again != rev {
		t.Errorf("again = %s, want %s", again, rev)
	}
	if fresh, _ := New("").Now(dir); fresh != rev {
		t.Errorf("made again = %s, want %s", fresh, rev)
	}

	// Its names are those of the files as they are.
	n, err := s.Names(dir, rev, "a.go")
	if err != nil || len(n.Lines[5]) == 0 {
		t.Errorf("names on line 5 = %+v %v", n, err)
	}
	a, err := s.Definition(dir, rev, "b.go", 3, 8)
	if err != nil || len(a.Targets) != 1 || a.Targets[0].Path != "a.go" || a.Targets[0].Line != 5 {
		t.Errorf("B's G = %+v %v", a, err)
	}

	// Changed again, though git lists it the same: a new commit.
	write("a.go", "package a\n\nfunc F() {}\n\nfunc G() { F(); F() }\n")
	if next, err := s.Now(dir); err != nil || next == rev {
		t.Errorf("changed again: %s %v", next, err)
	}
}

func TestNow_NoCommitYet(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-q")
	if _, err := New("").Now(dir); err == nil {
		t.Error("a repo with no commit has a now")
	}
}
