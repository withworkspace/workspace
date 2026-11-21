package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/workspace"
)

// appCall sends a write to an app's API as the web UI does, and reads the
// app back.
func appCall(t *testing.T, s *Server, path string, v any) (*http.Response, AppDetail) {
	t.Helper()
	data, _ := json.Marshal(v)
	res := do(t, s, "POST", path, true, func(r *http.Request) {
		r.Body = io.NopCloser(bytes.NewReader(data))
		r.ContentLength = int64(len(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	})
	var d AppDetail
	json.NewDecoder(res.Body).Decode(&d)
	return res, d
}

// An app is opened by its page, which starts its run for whoever's
// looking; it's pointed, and stopped, from there too.
func TestApps(t *testing.T) {
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	os.MkdirAll(filepath.Join(s.Root, "repos", "spec", ".ground"), 0o755)
	os.MkdirAll(filepath.Join(s.Root, "repos", "spec", "feat"), 0o755)
	os.WriteFile(filepath.Join(s.Root, "repos", "spec", "feat", ".git"), []byte("gitdir: x\n"), 0o644)
	setOps(s, testOps(s, ops.Options{}, func(c *config.Config, _ *workspace.Workspace) {
		c.Run = map[string]config.RunRecipe{
			"docs":  {Command: "sleep 300 # {spec}", Page: &config.RunPage{Title: "API docs", Icon: "book"}, Needs: map[string]config.RunNeed{"spec": {Repo: "spec"}}},
			"plain": {Command: "sleep 300"},
		}
	}))
	t.Cleanup(func() { s.App.StopAppAt(ops.AtWorkspace, "docs") })

	res := do(t, s, "GET", "/api/workspace", true, nil)
	var w Workspace
	json.NewDecoder(res.Body).Decode(&w)
	if len(w.Apps) != 1 || w.Apps[0] != (AppInfo{Name: "docs", Title: "API docs", Icon: "book"}) {
		t.Fatalf("apps = %+v", w.Apps)
	}
	if !strings.Contains(res.Header.Get("Content-Security-Policy"), "frame-src 'self' http://*.localhost:* http://127.0.0.1:*") {
		t.Errorf("the page can't frame a run: %q", res.Header.Get("Content-Security-Policy"))
	}

	// Nothing runs until it's opened.
	res = do(t, s, "GET", "/api/apps/docs", true, nil)
	var d AppDetail
	json.NewDecoder(res.Body).Decode(&d)
	if res.StatusCode != 200 || d.Run != nil || len(d.Pointers) != 1 || d.Pointers[0].At != "" || strings.Join(d.Pointers[0].Choices, ",") != "ground,feat" {
		t.Fatalf("before it's opened: %d %+v", res.StatusCode, d)
	}
	res, d = appCall(t, s, "/api/apps/docs/open", nil)
	if res.StatusCode != 200 || d.Run == nil || d.Run.State != runs.Running || d.Run.Owner != "looking" || d.Run.Ref != "docs" {
		t.Fatalf("opened: %d %+v", res.StatusCode, d.Run)
	}
	if !strings.Contains(d.Run.Ends, "15 minutes after nobody has docs open") || d.Run.Needs[0].Worktree != "ground" || !d.Run.Needs[0].Folder {
		t.Errorf("opened: ends %q, needs %+v", d.Run.Ends, d.Run.Needs)
	}
	ref, pid := d.Run.Ref, d.Run.PID
	// Opened again (the page saying it's still open), it's the same run.
	if _, d = appCall(t, s, "/api/apps/docs/open", nil); d.Run.PID != pid {
		t.Errorf("opened again: a new run %d, not %d", d.Run.PID, pid)
	}

	// Pointed, it's started again on what it's pointed at.
	res, d = appCall(t, s, "/api/apps/docs/point", AppPoint{Need: "spec", At: "feat"})
	if res.StatusCode != 200 || d.Pointers[0].At != "feat" || d.Run.PID == pid || !strings.HasSuffix(d.Run.Command, filepath.Join("spec", "feat")) || d.Run.Owner != "looking" {
		t.Fatalf("pointed: %d %+v %+v", res.StatusCode, d.Pointers, d.Run)
	}
	if !strings.Contains(d.Said, "spec is pointed at feat. docs was started again on it.") {
		t.Errorf("said %q", d.Said)
	}
	if res, _ := appCall(t, s, "/api/apps/docs/point", AppPoint{Need: "spec", At: "nope"}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("pointed at nothing: %d", res.StatusCode)
	}

	// Stopped, it's gone: a run ws ends is removed, its record with it.
	res, d = appCall(t, s, "/api/apps/docs/stop", nil)
	if res.StatusCode != 200 || d.Run != nil {
		t.Errorf("stopped %s: %d %+v", ref, res.StatusCode, d.Run)
	}
	// A recipe with no page is no app.
	if res := do(t, s, "GET", "/api/apps/plain", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("plain: %d", res.StatusCode)
	}
}

// A repo's recipe with a page is a tab of each of its worktrees: listed
// with the worktree's recipes and runs, and opened there, by whoever has
// the repo's page open.
func TestWorktreeApps(t *testing.T) {
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	feat := filepath.Join(s.Root, "repos", "bff", "feat")
	os.MkdirAll(feat, 0o755)
	os.WriteFile(filepath.Join(feat, "ws.repo.toml"), []byte(`
[run.docs]
command = "sleep 300"
page = { title = "API docs", icon = "book" }

[run.web]
command = "sleep 300"

[run.stack]
command = "sleep 300"
where = "silo"
`), 0o644)
	t.Cleanup(func() { s.App.StopAppAt(ops.AppPlace{Repo: "bff", Worktree: "feat"}, "docs") })
	base := "/api/repos/bff/worktrees/feat"

	res := do(t, s, "GET", base+"/runs", true, nil)
	var info PlaceRunsInfo
	json.NewDecoder(res.Body).Decode(&info)
	if res.StatusCode != 200 || len(info.Apps) != 1 || info.Apps[0].Title != "API docs" || len(info.Recipes) != 2 || info.Recipes[0].App == nil || info.Recipes[1].App != nil || len(info.Runs) != 0 {
		t.Fatalf("runs: %d %+v", res.StatusCode, info)
	}
	res, d := appCall(t, s, base+"/apps/docs/open", nil)
	if res.StatusCode != 200 || d.Run == nil || d.Run.Ref != "docs.feat.bff" || d.Run.Owner != "looking" || d.Run.App != "docs" || len(d.Pointers) != 0 {
		t.Fatalf("opened: %d %+v", res.StatusCode, d.Run)
	}
	if res, _ := appCall(t, s, base+"/looked", nil); res.StatusCode != 200 {
		t.Errorf("looked: %d", res.StatusCode)
	}
	res = do(t, s, "GET", base+"/runs", true, nil)
	json.NewDecoder(res.Body).Decode(&info)
	if len(info.Runs) != 1 || info.Runs[0].Ref != "docs.feat.bff" {
		t.Errorf("runs after opening: %+v", info.Runs)
	}
	// Any of its recipes starts from its page; the silo's isn't its.
	if res, d := appCall(t, s, base+"/apps/web/open", nil); res.StatusCode != 200 || d.Run == nil || d.Title != "web" {
		t.Errorf("a recipe with no page: %d %+v", res.StatusCode, d)
	}
	t.Cleanup(func() { s.App.StopAppAt(ops.AppPlace{Repo: "bff", Worktree: "feat"}, "web") })
	if res := do(t, s, "GET", "/api/repos/bff/worktrees/nope/runs", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a worktree that isn't there: %d", res.StatusCode)
	}
}
