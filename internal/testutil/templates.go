package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A test's repos are copies. Making one with git takes nine commands (a
// source repo with its first commit, the bare clone, the fetch, the ground
// worktree), and nearly every test asks for the same few shapes of repo.
// So each shape is made once for the test binary (Shared), as a template,
// and a test gets a copy of it, with the paths git keeps in it pointed at
// where the copy is.

// repoTemplate is the template for repos of repo's shape on defaultBranch:
// src, the source repo, and beside it .bare and .ground as a workspace
// keeps them (left out when it isn't cloned).
func repoTemplate(t *testing.T, defaultBranch string, repo RepoOpts) string {
	t.Helper()
	shape := fmt.Sprintf("repo %q direct=%v branches=%q cloned=%v", defaultBranch, repo.Landing == "direct", repo.Branches, !repo.NotCloned)
	return Shared(t, shape, func(dir string) {
		srcDir := filepath.Join(dir, "src")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		initRepo(t, srcDir, defaultBranch)
		if repo.Landing == "direct" {
			GitCmd(t, srcDir, "config", "receive.denyCurrentBranch", "updateInstead")
		}
		// Create extra branches in source
		for _, branch := range repo.Branches {
			GitCmd(t, srcDir, "checkout", "-b", branch)
			if err := os.WriteFile(filepath.Join(srcDir, branch+".txt"), []byte(branch), 0644); err != nil {
				t.Fatal(err)
			}
			GitCmd(t, srcDir, "add", ".")
			GitCmd(t, srcDir, "commit", "-m", "add "+branch)
			GitCmd(t, srcDir, "checkout", defaultBranch)
		}
		if repo.NotCloned {
			return
		}
		// Bare clone from source
		bareDir := filepath.Join(dir, ".bare")
		GitCmd(t, "", "clone", "--bare", srcDir, bareDir)
		// Configure fetch refspec (same as ws does)
		GitCmd(t, bareDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
		// Fetch to populate remote refs
		GitCmd(t, bareDir, "fetch", "--all")
		// Add .ground worktree
		GitCmd(t, bareDir, "worktree", "add", filepath.Join(dir, ".ground"), defaultBranch)
	})
}

// copyRepo makes a test's repo from a template: its source repo at srcDir
// and, when it's cloned, .bare and .ground in repoDir.
func copyRepo(t *testing.T, tmpl, srcDir, repoDir string, cloned bool) {
	t.Helper()
	copyTree(t, filepath.Join(tmpl, "src"), srcDir)
	if !cloned {
		return
	}
	bareDir, groundDir := filepath.Join(repoDir, ".bare"), filepath.Join(repoDir, ".ground")
	copyTree(t, filepath.Join(tmpl, ".bare"), bareDir)
	copyTree(t, filepath.Join(tmpl, ".ground"), groundDir)

	// git keeps four absolute paths in a clone with a worktree. Its
	// remote, as it was given to git clone:
	repoint(t, filepath.Join(bareDir, "config"), filepath.Join(tmpl, "src"), srcDir)
	repoint(t, filepath.Join(bareDir, "FETCH_HEAD"), filepath.Join(tmpl, "src"), srcDir)
	// and the worktree and the clone, each naming where the other is,
	// which git resolves before it writes them.
	real, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	admin, _ := filepath.Glob(filepath.Join(bareDir, "worktrees", "*", "gitdir"))
	if len(admin) != 1 {
		t.Fatalf("the template's clone has %d worktrees, not .ground alone", len(admin))
	}
	repoint(t, admin[0], filepath.Join(tmpl, ".ground"), filepath.Join(real, ".ground"))
	repoint(t, filepath.Join(groundDir, ".git"), filepath.Join(tmpl, ".bare"), filepath.Join(real, ".bare"))
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(to, os.DirFS(from)); err != nil {
		t.Fatal(err)
	}
}

// repoint has the file at path name the folder to where it named from.
func repoint(t *testing.T, path, from, to string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), from) {
		t.Fatalf("%s doesn't name %s:\n%s", path, from, data)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), from, to)), 0o644); err != nil {
		t.Fatal(err)
	}
}
