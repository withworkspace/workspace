package web

import (
	"fmt"
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
	os.WriteFile(filepath.Join(ground, "ws.repo.toml"), []byte(`[run.service.web]
command = "sleep 300"
display_name = "Storefront"

[run.service.docs]
command = "sleep 300"
kind = "tool"
render = "tab"
page = { title = "API docs" }

[run.task.test]
command = "true"

[run.service.stack]
command = "sleep 300"
where = "silo"
`), 0o644)

	var got WorktreeRunsInfo
	if st, _ := getJSON(t, s, "/api/repos/api/worktrees/.ground/runs", &got); st != 200 {
		t.Fatalf("status %d", st)
	}
	var said []string
	for _, rc := range got.Recipes {
		said = append(said, fmt.Sprintf("%s %s %s %s %q page=%v", rc.Name, rc.Type, rc.Kind, rc.Render, rc.Title, rc.Page))
	}
	want := []string{
		`docs service tool tab "API docs" page=true`,
		`test task   "" page=false`,
		`web service app inline "Storefront" page=false`,
	}
	if len(got.Runs) != 0 || strings.Join(said, "\n") != strings.Join(want, "\n") {
		t.Errorf("runs %v, recipes:\n%s\nwant (the silo's left out):\n%s", got.Runs, strings.Join(said, "\n"), strings.Join(want, "\n"))
	}
	if len(got.Apps) != 1 || got.Apps[0].Title != "API docs" || got.Apps[0].Render != RenderTab {
		t.Errorf("apps %+v: want API docs, opened in a tab", got.Apps)
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
