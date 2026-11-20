package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The Files page asks a ground for its runs and recipes, and so says it's
// open: the runs started from it are looked at while it asks.
func TestWorktreeRunsSaysTheFilesPageIsOpen(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	s.App = testOps(s, ops.Options{}, func(_ *config.Config, ws *workspace.Workspace) { ws.RepoNames = []string{"api"} })
	ground := filepath.Join(s.Root, "repos", "api", ".ground")
	os.MkdirAll(ground, 0o755)
	os.WriteFile(filepath.Join(ground, "ws.repo.toml"), []byte("[run.web]\ncommand = \"sleep 300\"\n\n[run.stack]\ncommand = \"sleep 300\"\nwhere = \"silo\"\n"), 0o644)

	var got WorktreeRunsInfo
	if st, _ := getJSON(t, s, "/api/repos/api/worktrees/.ground/runs", &got); st != 200 {
		t.Fatalf("status %d", st)
	}
	if len(got.Runs) != 0 || strings.Join(got.Recipes, ",") != "web" {
		t.Errorf("runs %v, recipes %v: want web alone, the silo's left out", got.Runs, got.Recipes)
	}
	if open := s.files.open(time.Now()); !open["api/.ground"] {
		t.Errorf("open on Files: %v", open)
	}
	if st, _ := getJSON(t, s, "/api/repos/api/worktrees/nope/runs", nil); st != 404 {
		t.Errorf("a capsule the repo hasn't: %d", st)
	}
	if res := do(t, s, "POST", "/api/repos/api/worktrees/.ground/runs", true, func(r *http.Request) {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	}); res.StatusCode != 400 {
		t.Errorf("no recipe named: %d", res.StatusCode)
	}
}

// A worktree the page stopped asking about is forgotten.
func TestFilesOpenForgetsAPageThatStoppedAsking(t *testing.T) {
	var f filesOpen
	now := time.Now()
	f.saw("api", ".ground", now)
	f.saw("api", "fix", now.Add(-3*lookedEvery))
	if open := f.open(now); !open["api/.ground"] || open["api/fix"] {
		t.Errorf("open: %v", open)
	}
	if _, kept := f.seen["api/fix"]; kept {
		t.Error("kept one nobody's asked about")
	}
}
