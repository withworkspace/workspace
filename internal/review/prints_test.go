package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file's fingerprint is of its change, not of where the change sits: a
// rebase onto a base that's moved keeps it; changing the change doesn't.
func TestFingerprintsSurviveARebase(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-q", "--initial-branch=main")
	run(t, dir, "config", "user.name", "Test")
	run(t, dir, "config", "user.email", "test@example.com")
	write := func(name, text string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644)
	}
	write("a.go", "package a\n\nvar x = 1\n\nvar y = 1\n")
	write("b.go", "package a\n")
	write("old.go", "package a\n\nvar old = 1\n")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "base")
	base1 := run(t, dir, "rev-parse", "HEAD")

	run(t, dir, "checkout", "-q", "-b", "feature")
	write("a.go", "package a\n\nvar x = 2\n\nvar y = 1\n")
	write("new.go", "package a\n\nvar z = 1\n")
	run(t, dir, "mv", "old.go", "moved.go")
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "feature")
	head1 := run(t, dir, "rev-parse", "HEAD")
	was, err := Fingerprints(dir, base1, head1)
	if err != nil {
		t.Fatal(err)
	}
	if len(was) != 3 || was["a.go"] == "" || was["new.go"] == "" || was["moved.go"] == "" {
		t.Fatalf("fingerprints = %v", was)
	}

	// main moves on, in the same file but other lines; the feature is
	// rebased onto it.
	run(t, dir, "checkout", "-q", "main")
	write("a.go", "package a\n\nvar x = 1\n\nvar y = 1\n\n\n\n\n\nvar w = 9\n")
	write("b.go", "package a\n\nvar b = 1\n")
	run(t, dir, "commit", "-q", "-am", "main moves")
	base2 := run(t, dir, "rev-parse", "HEAD")
	run(t, dir, "checkout", "-q", "feature")
	run(t, dir, "rebase", "-q", "main")
	head2 := run(t, dir, "rev-parse", "HEAD")
	now, err := Fingerprints(dir, base2, head2)
	if err != nil {
		t.Fatal(err)
	}
	for path, id := range was {
		if now[path] != id {
			t.Errorf("%s changed fingerprint over a rebase: %s, was %s", path, now[path], id)
		}
	}
	if len(now) != len(was) {
		t.Errorf("after the rebase: %v", now)
	}

	// Changing a file's change changes its fingerprint, and only its.
	write("a.go", "package a\n\nvar x = 3\n\nvar y = 1\n\n\n\n\n\nvar w = 9\n")
	run(t, dir, "commit", "-q", "-am", "three")
	head3 := run(t, dir, "rev-parse", "HEAD")
	then, err := Fingerprints(dir, base2, head3)
	if err != nil {
		t.Fatal(err)
	}
	if then["a.go"] == was["a.go"] || then["new.go"] != was["new.go"] {
		t.Errorf("after changing a.go: %v, was %v", then, was)
	}
	if none, err := Fingerprints(dir, head3, head3); err != nil || len(none) != 0 {
		t.Errorf("no changes: %v, %v", none, err)
	}
}

// A pull request's diff is read between two commits in a clone, with no
// working tree to it.
func TestCompareCommits(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-q", "--initial-branch=main")
	run(t, dir, "config", "user.name", "Test")
	run(t, dir, "config", "user.email", "test@example.com")
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644)
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "base")
	base := run(t, dir, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nvar x = 1\n"), 0o644)
	run(t, dir, "commit", "-q", "-am", "one")
	head := run(t, dir, "rev-parse", "HEAD")
	bare := filepath.Join(t.TempDir(), "bare.git")
	run(t, dir, "clone", "-q", "--bare", dir, bare)

	d, err := CompareCommits(bare, "origin/main", base, head)
	if err != nil {
		t.Fatal(err)
	}
	if d.Base != base || d.Head != head || d.BaseRef != "origin/main" || len(d.Files) != 1 || len(d.Commits) != 1 || len(d.Uncommitted) != 0 || d.Files[0].Additions != 2 {
		t.Errorf("diff = %+v", d)
	}
	if _, err := CompareCommits(bare, "origin/main", base, "0123456789012345678901234567890123456789"); err == nil {
		t.Error("compared with a commit that isn't there")
	}
	if !IsAncestor(bare, base, head) || IsAncestor(bare, head, base) {
		t.Error("IsAncestor")
	}
	if got, err := MergeBase(bare, base, head); err != nil || got != base {
		t.Errorf("merge base = %s, %v", got, err)
	}
	if CommitOf(bare, "refs/heads/main") != head || CommitOf(bare, "refs/heads/nope") != "" {
		t.Error("CommitOf")
	}
}

// A store rooted in a folder keeps reviews with no mission to tell.
func TestStoreAt(t *testing.T) {
	m, d := reviewing(t)
	s := At(m.Missions.Root, filepath.Join(m.Missions.Root, ".ws", "prs", "api", "89"), "api#89")
	if _, err := s.AddNote("api#89", d, NewNote{Path: "cache.go", Side: "new", Line: 3, Kind: "question", Body: "Why?", By: "agent"}, at); err != nil {
		t.Fatal(err)
	}
	r, err := s.Submit("api#89", d, Approve, "", at)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "r1" || r.State != Submitted || len(r.Comments) != 1 || r.Comments[0].By != "agent" {
		t.Errorf("review = %+v", r)
	}
	if got, err := s.Latest("api#89"); err != nil || got == nil || got.ID != "r1" {
		t.Errorf("latest = %+v, %v", got, err)
	}
	if _, err := s.Load("r9"); err == nil || !strings.Contains(err.Error(), "api#89 has no review r9") {
		t.Errorf("a review that isn't there: %v", err)
	}
	// Resolving a kept comment needs no mission either.
	if _, err := s.Respond("r1.c1", "you", Resolve, "", "", at); err != nil {
		t.Errorf("resolve: %v", err)
	}
}
