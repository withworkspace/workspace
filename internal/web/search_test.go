package web

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

func requireRipgrep(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
	}
}

// searchServer serves a workspace of one repo, api, as ws lays it out: a
// bare clone with origin/main, .ground on main, and a capsule, fix, that
// changes a.go and adds new.go, and leaves b.go as main has it. long.txt
// has a thousand lines a single letter matches.
func searchServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote")
	os.MkdirAll(remote, 0o755)
	gitT(t, remote, "init", "-q", "--initial-branch=main")
	gitT(t, remote, "config", "user.name", "Test")
	gitT(t, remote, "config", "user.email", "test@example.com")
	var long strings.Builder
	for i := range 1000 {
		fmt.Fprintf(&long, "line %d, every one of them\n", i)
	}
	for name, content := range map[string]string{"a.go": "package a // needle on main\n", "b.go": "package a // needle unchanged\n", "long.txt": long.String()} {
		os.WriteFile(filepath.Join(remote, name), []byte(content), 0o644)
	}
	gitT(t, remote, "add", "-A")
	gitT(t, remote, "commit", "-q", "-m", "base")
	bare := filepath.Join(root, "repos", "api", ".bare")
	if out, err := exec.Command("git", "clone", "-q", "--bare", remote, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	gitT(t, bare, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	gitT(t, bare, "fetch", "-q", "origin")
	gitT(t, bare, "worktree", "add", "-q", filepath.Join(root, "repos", "api", ".ground"), "main")
	capsule := filepath.Join(root, "repos", "api", "fix")
	gitT(t, bare, "worktree", "add", "-q", "-b", "fix", capsule, "main")
	os.WriteFile(filepath.Join(capsule, "a.go"), []byte("package a // needle in the capsule\n"), 0o644)
	os.WriteFile(filepath.Join(capsule, "new.go"), []byte("package a // needle new\n"), 0o644)
	gitT(t, capsule, "add", "-A")
	gitT(t, capsule, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-q", "-m", "fix")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	storetest.Own(t, root)
	s := &Server{Root: root, Poll: 20 * time.Millisecond, Assets: testApp, listener: ln}
	s.App = testOps(s, ops.Options{}, func(_ *config.Config, ws *workspace.Workspace) {
		ws.RepoNames, ws.DefaultBranch = []string{"api"}, "main"
	})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

// searchLines reads what GET /api/search answered: its hits, and its last
// line.
func searchLines(t *testing.T, s *Server, query url.Values) (hits []ops.CodeHit, end map[string]any) {
	t.Helper()
	res := do(t, s, "GET", "/api/search?"+query.Encode(), true, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("search %v: %d %s", query, res.StatusCode, body(res))
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("content type %q", ct)
	}
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		if end != nil {
			t.Fatalf("a line after the end: %s", sc.Text())
		}
		var probe map[string]any
		if err := json.Unmarshal(sc.Bytes(), &probe); err != nil {
			t.Fatalf("line %q: %v", sc.Text(), err)
		}
		if probe["done"] == true {
			end = probe
			continue
		}
		var h ops.CodeHit
		json.Unmarshal(sc.Bytes(), &h)
		hits = append(hits, h)
	}
	if end == nil {
		t.Fatal("no end line")
	}
	return hits, end
}

func TestCodeSearchAPI(t *testing.T) {
	requireRipgrep(t)
	s := searchServer(t)

	if res := do(t, s, "GET", "/api/search?p=needle", false, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("not signed in: %d", res.StatusCode)
	}

	// Across the workspace: .ground, and the capsule's changed files in
	// place of .ground's copies; its unchanged b.go isn't searched.
	hits, end := searchLines(t, s, url.Values{"p": {"needle"}})
	var got []string
	for _, h := range hits {
		got = append(got, h.Worktree+":"+h.Path+": "+h.Text)
	}
	slices.Sort(got)
	want := []string{
		".ground:b.go: package a // needle unchanged",
		"fix:a.go: package a // needle in the capsule",
		"fix:new.go: package a // needle new",
	}
	if !slices.Equal(got, want) {
		t.Errorf("hits\n %q\nwant\n %q", got, want)
	}
	if end["matches"] != float64(3) || end["capped"] != nil || end["problem"] != nil {
		t.Errorf("end %v", end)
	}
	for _, h := range hits {
		if h.Repo != "api" || h.Line != 1 || h.Column != 14 || len(h.Ranges) != 1 || h.Text[h.Ranges[0][0]:h.Ranges[0][1]] != "needle" {
			t.Errorf("hit %+v", h)
		}
	}

	// Several patterns; tagged to the repo; one worktree whole.
	hits, _ = searchLines(t, s, url.Values{"p": {"unchanged", "new"}, "repo": {"api"}})
	if len(hits) != 2 {
		t.Errorf("two patterns: %+v", hits)
	}
	hits, _ = searchLines(t, s, url.Values{"p": {"needle"}, "repo": {"api"}, "worktree": {"fix"}})
	if len(hits) != 3 {
		t.Errorf("the capsule, whole: %+v", hits)
	}

	// A single letter is capped, by matches or by bytes, and says so.
	hits, end = searchLines(t, s, url.Values{"p": {"e"}})
	if len(hits) != ops.CodeSearchMatches || end["capped"] != ops.CappedMatches || end["matches"] != float64(ops.CodeSearchMatches) {
		t.Errorf("single letter: %d hits, end %v", len(hits), end)
	}
	hits, end = searchLines(t, s, url.Values{"p": {"e"}, "max": {"10"}})
	if len(hits) != 10 || end["capped"] != ops.CappedMatches {
		t.Errorf("max 10: %d hits, end %v", len(hits), end)
	}
	hits, end = searchLines(t, s, url.Values{"p": {"e"}, "bytes": {"1000"}})
	if len(hits) == 0 || len(hits) >= ops.CodeSearchMatches || end["capped"] != ops.CappedBytes || end["bytes"].(float64) > 1000 {
		t.Errorf("1000 bytes: %d hits, end %v", len(hits), end)
	}

	// Asked wrong.
	for q, status := range map[string]int{
		"":                               http.StatusUnprocessableEntity, // no pattern
		"p=x&max=lots":                   http.StatusBadRequest,
		"p=x&case=loud":                  http.StatusUnprocessableEntity,
		"p=x&repo=nope":                  http.StatusNotFound,
		"p=x&repo=api&worktree=.silo":    http.StatusNotFound,
		"p=x&worktree=fix":               http.StatusUnprocessableEntity,
		"p=(&regex=1":                    http.StatusUnprocessableEntity,
		"p=x&repo=api&worktree=..%2Fapi": http.StatusNotFound,
	} {
		res := do(t, s, "GET", "/api/search?"+q, true, nil)
		if res.StatusCode != status {
			t.Errorf("%q: %d %s, want %d", q, res.StatusCode, body(res), status)
		}
	}
}

// A request the page drops, its query changed, stops ripgrep.
func TestCodeSearchAPIDroppedStopsRg(t *testing.T) {
	s := searchServer(t)
	// A ripgrep that says its pid, finds one match, then takes its time.
	bin := t.TempDir()
	pidFile := filepath.Join(bin, "pid")
	script := `#!/bin/sh
for a; do last=$a; done
printf '{"type":"match","data":{"path":{"text":"%s/x.go"},"lines":{"text":"x\\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}\n' "$last"
echo $$ > "` + pidFile + `.tmp" && mv "` + pidFile + `.tmp" "` + pidFile + `"
exec sleep 30
`
	os.WriteFile(filepath.Join(bin, "rg"), []byte(script), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res := do(t, s, "GET", "/api/search?p=x&repo=api&worktree=fix", true, func(r *http.Request) { *r = *r.WithContext(ctx) })
	if res.StatusCode != http.StatusOK {
		t.Fatalf("search: %d %s", res.StatusCode, body(res))
	}
	sc := bufio.NewScanner(res.Body)
	if !sc.Scan() || !strings.Contains(sc.Text(), `"path":"x.go"`) {
		t.Fatalf("first hit: %q", sc.Text())
	}
	var pid int
	for deadline := time.Now().Add(5 * time.Second); pid == 0 && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if b, err := os.ReadFile(pidFile); err == nil {
			fmt.Sscan(string(b), &pid)
		}
	}
	if pid == 0 || !running(pid) {
		t.Fatalf("ripgrep (%d) wasn't running", pid)
	}

	cancel() // the query changed
	for deadline := time.Now().Add(5 * time.Second); running(pid); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("ripgrep %d still running after the request was dropped", pid)
		}
	}
}

// running is whether a process is running: one killed but not yet reaped
// isn't.
func running(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	out, _ := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(pid)).Output()
	return len(out) > 0 && !strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
}
