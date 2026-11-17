package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLanding_WorkspaceThenTheRepoItselfThenPR(t *testing.T) {
	root := t.TempDir()
	ws := &Workspace{Root: root, DefaultBranch: "main", Landings: map[string]string{"records": LandDirect, "docs": LandPR}, Verifies: map[string]string{"records": "just lint"}}
	for _, repo := range []string{"docs", "notes"} {
		os.MkdirAll(ws.MainWorktree(repo), 0o755)
		os.WriteFile(filepath.Join(ws.MainWorktree(repo), "ws.repo.toml"), []byte("landing = \"direct\"\nverify = \"make check\"\n\n[capsule]\nafter_create = \"true\"\n"), 0o644)
	}

	for repo, want := range map[string]string{
		"records": LandDirect, // the workspace says so
		"docs":    LandPR,     // the workspace says otherwise than the repo
		"notes":   LandDirect, // the repo says so of itself
		"api":     LandPR,     // nobody says: a pull request
	} {
		if got := ws.Landing(repo); got != want {
			t.Errorf("Landing(%s) = %q, want %q", repo, got, want)
		}
		if got := ws.Direct(repo); got != (want == LandDirect) {
			t.Errorf("Direct(%s) = %v", repo, got)
		}
	}
	for repo, want := range map[string]string{"records": "just lint", "notes": "make check", "api": ""} {
		if got := ws.VerifyCommand(repo); got != want {
			t.Errorf("VerifyCommand(%s) = %q, want %q", repo, got, want)
		}
	}
}

func TestPushBehind_OnlyWhenTheRemoteHasMoved(t *testing.T) {
	for out, want := range map[string]bool{
		"To /remote\n!\tHEAD:refs/heads/main\t[rejected] (fetch first)\nDone\n":                            true,
		"To /remote\n!\tHEAD:refs/heads/main\t[rejected] (non-fast-forward)\nDone\n":                       true,
		"To /remote\n!\tHEAD:refs/heads/main\t[remote rejected] (protected branch hook declined)\nDone\n":  false,
		"To /remote\n!\tHEAD:refs/heads/main\t[remote rejected] (branch is currently checked out)\nDone\n": false,
		"fatal: unable to access 'https://example.com/': Could not resolve host: example.com\n":            false,
		"To /remote\n*\tHEAD:refs/heads/main\t[new branch]\nDone\n":                                        false,
	} {
		if got := pushBehind(out); got != want {
			t.Errorf("pushBehind(%q) = %v, want %v", out, got, want)
		}
	}
}
