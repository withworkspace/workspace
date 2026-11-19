package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupWorkspace_CreatesStructure(t *testing.T) {
	w := SetupWorkspace(t, WorkspaceOpts{
		Org:           "test-org",
		DefaultBranch: "main",
		Repos: []RepoOpts{
			{Name: "repo-a"},
			{Name: "repo-b", Branches: []string{"feature-x"}},
		},
	})

	// ws.toml exists
	if _, err := os.Stat(filepath.Join(w.Root, "ws.toml")); err != nil {
		t.Errorf("ws.toml not found: %v", err)
	}

	// Bare repos exist
	for _, name := range []string{"repo-a", "repo-b"} {
		bareDir := filepath.Join(w.Root, "repos", name, ".bare")
		if _, err := os.Stat(bareDir); err != nil {
			t.Errorf("bare dir for %s not found: %v", name, err)
		}
	}

	// .ground worktrees exist
	for _, name := range []string{"repo-a", "repo-b"} {
		groundDir := filepath.Join(w.Root, "repos", name, ".ground")
		if _, err := os.Stat(groundDir); err != nil {
			t.Errorf(".ground for %s not found: %v", name, err)
		}
	}

	// Extra branch is fetchable (exists in bare repo refs)
	if _, ok := w.Sources["repo-b"]; !ok {
		t.Error("expected source repo path for repo-b")
	}
}

// A test's repos are copied from templates the tests share. A copy is a
// repo of its own: git finds its remote, its clone and its worktree where
// the copy is, nothing in it names the template, and what happens in one
// test's doesn't show in another's.
func TestSetupWorkspace_ReposAreCopiesOfTheirOwn(t *testing.T) {
	opts := WorkspaceOpts{
		Org:           "test-org",
		DefaultBranch: "main",
		Repos: []RepoOpts{
			{Name: "api", Branches: []string{"feature-x"}},
			{Name: "records", Landing: "direct"},
			{Name: "later", NotCloned: true},
		},
	}
	one, other := SetupWorkspace(t, opts), SetupWorkspace(t, opts)

	shared.Lock()
	templates := shared.dir
	shared.Unlock()
	if templates == "" {
		t.Fatal("no templates were made")
	}
	dirs := []string{one.Root}
	for _, src := range one.Sources {
		dirs = append(dirs, src)
	}
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || !d.Type().IsRegular() { // the records' socket is there too
				return err
			}
			data, err := os.ReadFile(path)
			if err == nil && strings.Contains(string(data), templates) {
				t.Errorf("%s names the template it was copied from", path)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"api", "records"} {
		repo := filepath.Join(one.Root, "repos", name)
		real, err := filepath.EvalSymlinks(repo)
		if err != nil {
			t.Fatal(err)
		}
		bare, ground := filepath.Join(repo, ".bare"), filepath.Join(repo, ".ground")
		if got := strings.TrimSpace(GitOut(t, bare, "remote", "get-url", "origin")); got != one.Sources[name] {
			t.Errorf("%s's remote is %s, not its source %s", name, got, one.Sources[name])
		}
		if got := GitOut(t, bare, "worktree", "list", "--porcelain"); !strings.Contains(got, "worktree "+filepath.Join(real, ".ground")+"\n") {
			t.Errorf("%s's clone doesn't know its ground:\n%s", name, got)
		}
		if got := strings.TrimSpace(GitOut(t, ground, "rev-parse", "--git-common-dir")); got != filepath.Join(real, ".bare") {
			t.Errorf("%s's ground is a worktree of %s", name, got)
		}
		if got := GitOut(t, ground, "status", "--porcelain"); got != "" {
			t.Errorf("%s's ground isn't clean:\n%s", name, got)
		}
		GitCmd(t, bare, "fsck", "--strict")
	}
	if got := GitOut(t, filepath.Join(one.Root, "repos", "api", ".bare"), "for-each-ref", "--format=%(refname)", "refs/remotes"); !strings.Contains(got, "refs/remotes/origin/feature-x") {
		t.Errorf("api's branch wasn't fetched:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(one.Root, "repos", "later")); !os.IsNotExist(err) {
		t.Errorf("later is cloned: %v", err)
	}
	if got := strings.TrimSpace(GitOut(t, one.Sources["later"], "log", "--format=%s")); got != "initial" {
		t.Errorf("later's source has %q", got)
	}

	// A commit on one's remote reaches one's clone, and not the other's.
	CommitIn(t, one.Sources["api"], "new.txt", "new", "Add a file")
	for _, w := range []*Workspace{one, other} {
		GitCmd(t, filepath.Join(w.Root, "repos", "api", ".bare"), "fetch", "--all")
	}
	if got := Subjects(t, filepath.Join(one.Root, "repos", "api", ".bare"), "origin/main"); got != "Add a file | initial" {
		t.Errorf("one's origin/main: %s", got)
	}
	if got := Subjects(t, filepath.Join(other.Root, "repos", "api", ".bare"), "origin/main"); got != "initial" {
		t.Errorf("the other's origin/main: %s", got)
	}
}
