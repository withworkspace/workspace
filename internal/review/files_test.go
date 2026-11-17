package review

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func treeFile(t *Tree, path string) *TreeFile {
	for i := range t.Files {
		if t.Files[i].Path == path {
			return &t.Files[i]
		}
	}
	return nil
}

func TestReadTree(t *testing.T) {
	dir := capsule(t)
	write(t, dir, "cache.go", "package cache\n\n// edited, not committed\n")
	tr, err := ReadTree(dir, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Base == "" || tr.Head == "" || tr.Branch != "cache-tokens" {
		t.Fatalf("base %q head %q branch %q", tr.Base, tr.Head, tr.Branch)
	}
	cases := []struct {
		path, status string
		uncommitted  bool
	}{
		{"cache.go", "modified", true},
		{"long.txt", "modified", false},
		{"added.go", "added", false},
		{"gone.txt", "deleted", false},
		{"new name.md", "renamed", false},
		{"scratch.txt", "added", true}, // untracked: new
	}
	for _, c := range cases {
		f := treeFile(tr, c.path)
		if f == nil {
			t.Errorf("%s isn't in the tree", c.path)
			continue
		}
		if f.Status != c.status || f.Uncommitted != c.uncommitted {
			t.Errorf("%s: status %q uncommitted %v, want %q %v", c.path, f.Status, f.Uncommitted, c.status, c.uncommitted)
		}
	}
	if f := treeFile(tr, "new name.md"); f == nil || f.OldPath != "old name.md" {
		t.Errorf("rename: %+v", f)
	}
	if treeFile(tr, "old name.md") != nil {
		t.Error("a renamed file's old path is listed")
	}
}

func TestReadTreeWithoutBase(t *testing.T) {
	dir := capsule(t)
	tr, err := ReadTree(dir, "origin/nope")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Base != "" {
		t.Errorf("base %q for a branch that isn't there", tr.Base)
	}
	if f := treeFile(tr, "long.txt"); f == nil || f.Status != "" {
		t.Errorf("without a base nothing's changed: %+v", f)
	}
}

func TestReadFile(t *testing.T) {
	dir := capsule(t)
	v, err := ReadFile(dir, "origin/main", "long.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Lines) != 40 || len(v.Changes) != 1 {
		t.Fatalf("%d lines, changes %+v", len(v.Lines), v.Changes)
	}
	if c := v.Changes[0]; c.Kind != "modified" || c.Start != 10 || c.End != 10 {
		t.Errorf("change %+v, want line 10 modified", c)
	}

	v, err = ReadFile(dir, "origin/main", "scratch.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Changes) != 1 || v.Changes[0].Kind != "added" || v.Changes[0].End != 1 {
		t.Errorf("untracked file's changes %+v, want all added", v.Changes)
	}

	v, err = ReadFile(dir, "origin/main", "gone.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Deleted || len(v.Lines) != 1 || v.Lines[0] != "bye" {
		t.Errorf("deleted file: %+v", v)
	}

	if v, err = ReadFile(dir, "origin/main", "logo.bin"); err != nil || !v.Binary {
		t.Errorf("binary: %+v %v", v, err)
	}
}

func TestReadFileStaysInside(t *testing.T) {
	dir := capsule(t)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(outside, []byte("secret\n"), 0o644)
	os.Symlink(outside, filepath.Join(dir, "link.txt"))
	for _, p := range []string{"../secret.txt", "link.txt", "a/../../x"} {
		if _, err := ReadFile(dir, "origin/main", p); !errors.Is(err, ErrOutside) {
			t.Errorf("%s: %v, want ErrOutside", p, err)
		}
	}
}
