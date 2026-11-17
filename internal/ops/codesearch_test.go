package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/workspace"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// codeRepo makes a repo in the workspace as ws does: a bare clone of a
// remote with files on main, origin/main fetched, and .ground on main.
func codeRepo(t *testing.T, root, name string, files map[string]string) {
	t.Helper()
	remote := filepath.Join(root, "remotes", name)
	for p, content := range files {
		writeFile(t, filepath.Join(remote, p), content)
	}
	gitIn(t, remote, "init", "-q", "--initial-branch=main")
	gitIn(t, remote, "add", "-A")
	gitIn(t, remote, "commit", "-q", "-m", "base")
	bare := filepath.Join(root, "repos", name, ".bare")
	gitIn(t, root, "clone", "-q", "--bare", remote, bare)
	gitIn(t, bare, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	gitIn(t, bare, "fetch", "-q", "origin")
	gitIn(t, bare, "worktree", "add", "-q", filepath.Join(root, "repos", name, ".ground"), "main")
}

// codeWorld is a workspace of two repos. backend has only its .ground,
// with a long file of lines a single letter matches (not needle). frontend has a
// capsule, login-fix, that modifies src/app.ts, adds src/new.ts (and
// src/wip.ts, not committed), removes src/gone.ts and leaves src/same.ts
// as main has it. Every file says "needle".
func codeWorld(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	var long strings.Builder
	for i := range 2000 {
		fmt.Fprintf(&long, "line %d of the long file, every one\n", i)
	}
	codeRepo(t, root, "backend", map[string]string{"main.go": "package main // needle\n", "long.txt": long.String()})
	codeRepo(t, root, "frontend", map[string]string{
		"src/app.ts":  "needle in ground app\n",
		"src/same.ts": "needle unchanged\n",
		"src/gone.ts": "needle gone in the capsule\n",
	})
	repo := filepath.Join(root, "repos", "frontend")
	capsule := filepath.Join(repo, "login-fix")
	gitIn(t, filepath.Join(repo, ".bare"), "worktree", "add", "-q", "-b", "login-fix", capsule, "main")
	writeFile(t, filepath.Join(capsule, "src/app.ts"), "needle in capsule app\n")
	writeFile(t, filepath.Join(capsule, "src/new.ts"), "needle new in capsule\n")
	os.Remove(filepath.Join(capsule, "src/gone.ts"))
	gitIn(t, capsule, "add", "-A")
	gitIn(t, capsule, "commit", "-q", "-m", "change")
	writeFile(t, filepath.Join(capsule, "src/wip.ts"), "needle not committed\n")

	ws := &workspace.Workspace{Root: root, RepoNames: []string{"backend", "frontend"}, DefaultBranch: "main"}
	return New(&config.Config{}, ws, nil, Options{})
}

func codeSearchAll(t *testing.T, w *Workspace, req CodeSearchRequest) ([]CodeHit, CodeSearchResult, error) {
	t.Helper()
	var hits []CodeHit
	res, err := w.CodeSearch(context.Background(), req, func(h CodeHit) error { hits = append(hits, h); return nil })
	return hits, res, err
}

func hitKeys(hits []CodeHit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, h.Repo+"/"+h.Worktree+":"+h.Path)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Across the workspace: every .ground, and of a capsule only what it
// changed, its copy in place of .ground's.
func TestCodeSearchGlobal(t *testing.T) {
	requireRg(t)
	w := codeWorld(t)
	hits, res, err := codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"needle"}, MaxMatches: CodeSearchMaxMatches, MaxBytes: CodeSearchMaxBytes})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"backend/.ground:main.go",
		"frontend/.ground:src/gone.ts", // removed in the capsule: main still has it
		"frontend/.ground:src/same.ts",
		"frontend/login-fix:src/app.ts", // in place of .ground's
		"frontend/login-fix:src/new.ts",
		"frontend/login-fix:src/wip.ts", // uncommitted counts
	}
	if got := hitKeys(hits); !reflect.DeepEqual(got, want) {
		t.Errorf("hits in\n %q\nwant\n %q", got, want)
	}
	if res.Capped != "" || res.Matches != len(hits) {
		t.Errorf("result %+v for %d hits", res, len(hits))
	}
	for _, h := range hits {
		if h.Path == "src/app.ts" && h.Text != "needle in capsule app" {
			t.Errorf("app.ts is the capsule's: %q", h.Text)
		}
	}

	t.Run("tagged to a repo", func(t *testing.T) {
		hits, _, err := codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"needle"}, Repo: "frontend"})
		if err != nil {
			t.Fatal(err)
		}
		if got := hitKeys(hits); !reflect.DeepEqual(got, want[1:]) {
			t.Errorf("hits in %q, want %q", got, want[1:])
		}
	})
	t.Run("one worktree, whole", func(t *testing.T) {
		hits, _, err := codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"needle"}, Repo: "frontend", Worktree: "login-fix"})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"frontend/login-fix:src/app.ts", "frontend/login-fix:src/new.ts", "frontend/login-fix:src/same.ts", "frontend/login-fix:src/wip.ts"}
		if got := hitKeys(hits); !reflect.DeepEqual(got, want) {
			t.Errorf("hits in %q, want %q", got, want)
		}
	})
	t.Run("not a worktree of the repo", func(t *testing.T) {
		for _, wt := range []string{".silo", ".bare", "..", "nope"} {
			if _, _, err := codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"needle"}, Repo: "frontend", Worktree: wt}); err == nil {
				t.Errorf("worktree %q searched", wt)
			}
		}
		if _, _, err := codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"needle"}, Repo: "nope"}); !errors.Is(err, ErrNotFound) {
			t.Errorf("unknown repo: %v", err)
		}
	})
	t.Run("literal by default, regex when asked", func(t *testing.T) {
		hits, _, _ := codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"need.e"}, Repo: "frontend"})
		if len(hits) != 0 {
			t.Errorf("literal need.e found %q", hitKeys(hits))
		}
		hits, _, _ = codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"need.e"}, Repo: "frontend", Regex: true})
		if len(hits) == 0 {
			t.Error("regex need.e found nothing")
		}
	})
	t.Run("several patterns, smart case", func(t *testing.T) {
		hits, _, _ := codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"PACKAGE", "uncommitted-nope", "unchanged"}})
		if got := hitKeys(hits); !reflect.DeepEqual(got, []string{"frontend/.ground:src/same.ts"}) {
			t.Errorf("hits in %q", got)
		}
		hits, _, _ = codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"PACKAGE"}, Case: "ignore"})
		if got := hitKeys(hits); !reflect.DeepEqual(got, []string{"backend/.ground:main.go"}) {
			t.Errorf("hits in %q", got)
		}
	})
	t.Run("a bad regex is ripgrep's error", func(t *testing.T) {
		_, _, err := codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"("}, Regex: true})
		var exit *RgExit
		if !errors.As(err, &exit) || exit.Code != 2 {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("no pattern, or an empty one", func(t *testing.T) {
		for _, p := range [][]string{nil, {""}} {
			var inv *Invalid
			if _, _, err := codeSearchAll(t, w, CodeSearchRequest{Patterns: p}); !errors.As(err, &inv) {
				t.Errorf("%q: %v", p, err)
			}
		}
	})
}

// A single letter matches nearly every line: it stops at the cap on
// matches, or on bytes, and says which.
func TestCodeSearchCaps(t *testing.T) {
	requireRg(t)
	w := codeWorld(t)

	hits, res, err := codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"e"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != CodeSearchMatches || res.Matches != CodeSearchMatches || res.Capped != CappedMatches {
		t.Errorf("default cap: %d hits, %+v; want %d capped by matches", len(hits), res, CodeSearchMatches)
	}

	hits, res, err = codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"e"}, MaxMatches: 5})
	if err != nil || len(hits) != 5 || res.Capped != CappedMatches {
		t.Errorf("max 5: %d hits, %+v, %v", len(hits), res, err)
	}

	hits, res, err = codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"e"}, MaxMatches: CodeSearchMaxMatches, MaxBytes: 2000})
	if err != nil || res.Capped != CappedBytes || res.Bytes > 2000 || len(hits) == 0 || len(hits) >= CodeSearchMaxMatches {
		t.Errorf("2000 bytes: %d hits, %+v, %v", len(hits), res, err)
	}
	size := 0
	for _, h := range hits {
		size += len(h.Path) + len(h.Text)
	}
	if size != res.Bytes {
		t.Errorf("bytes %d, hits come to %d", res.Bytes, size)
	}

	// A narrow pattern isn't capped.
	_, res, _ = codeSearchAll(t, w, CodeSearchRequest{Patterns: []string{"unchanged"}})
	if res.Capped != "" || res.Matches != 1 {
		t.Errorf("narrow: %+v", res)
	}
	// Asking for more than the most gets the most.
	if capOf(1<<30, CodeSearchBytes, CodeSearchMaxBytes) != CodeSearchMaxBytes || capOf(0, 7, 9) != 7 {
		t.Error("capOf")
	}
}

// fakeRg puts a ripgrep on PATH that says its pid, prints one match under
// the last root it's given, then waits a long time: a search over a big
// tree, to stop part way.
func fakeRg(t *testing.T) (pidFile string) {
	t.Helper()
	bin := t.TempDir()
	pidFile = filepath.Join(bin, "pid")
	script := `#!/bin/sh
for a; do last=$a; done
printf '{"type":"match","data":{"path":{"text":"%s/x.go"},"lines":{"text":"x\\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}\n' "$last"
echo $$ > "` + pidFile + `.tmp" && mv "` + pidFile + `.tmp" "` + pidFile + `"
exec sleep 30
`
	if err := os.WriteFile(filepath.Join(bin, "rg"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return pidFile
}

// FakeRgPid waits for the fake ripgrep's pid.
func fakeRgPid(t *testing.T, pidFile string) int {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if b, err := os.ReadFile(pidFile); err == nil {
			var pid int
			fmt.Sscan(string(b), &pid)
			return pid
		}
	}
	t.Fatal("ripgrep never started")
	return 0
}

// alive is whether a process is still running (not yet reaped counts as
// gone: it's been killed).
func alive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	out, _ := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(pid)).Output()
	return len(out) > 0 && !strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
}

// Cancelling the search (the query changed) stops ripgrep there and then.
func TestCodeSearchCancelStopsRg(t *testing.T) {
	w := codeWorld(t)
	pidFile := fakeRg(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	first := make(chan struct{})
	go func() {
		_, err := w.CodeSearch(ctx, CodeSearchRequest{Patterns: []string{"x"}, Repo: "frontend", Worktree: "login-fix"}, func(h CodeHit) error {
			close(first)
			return nil
		})
		done <- err
	}()
	<-first
	pid := fakeRgPid(t, pidFile)
	if !alive(pid) {
		t.Fatal("ripgrep wasn't running")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("search didn't stop when cancelled")
	}
	if alive(pid) {
		t.Errorf("ripgrep %d still running", pid)
	}
}

// The roots of a search across a layout: .ground whole, a capsule's
// changed files (not removed ones), and which .ground copies they replace.
func TestCodeRoots(t *testing.T) {
	l := find.Layout{
		Grounds: []find.Worktree{{Repo: "b", Name: find.Ground, Dir: "/w/repos/b/.ground"}, {Repo: "a", Name: find.Ground, Dir: "/w/repos/a/.ground"}},
		Capsules: []find.CapsuleChanges{
			{Worktree: find.Worktree{Repo: "a", Name: "z-fix", Dir: "/w/repos/a/z-fix"}, Files: map[string]string{"x.go": find.Modified, "old.go": find.Removed, "n.go": find.Added}},
			{Worktree: find.Worktree{Repo: "a", Name: "gone-only", Dir: "/w/repos/a/gone-only"}, Files: map[string]string{"y.go": find.Removed}},
			{Worktree: find.Worktree{Repo: "a", Name: "clean", Dir: "/w/repos/a/clean"}, Files: map[string]string{}},
			{Worktree: find.Worktree{Repo: "a", Name: "b-fix", Dir: "/w/repos/a/b-fix"}, Files: map[string]string{"x.go": find.Modified}},
			{Worktree: find.Worktree{Repo: "b", Name: "fix", Dir: "/w/repos/b/fix"}, Files: map[string]string{"q.go": find.Added}},
		},
	}
	roots, replaced, err := codeRoots(l, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range roots {
		got = append(got, fmt.Sprintf("%s/%s %v", r.Repo, r.Worktree, r.Files))
	}
	want := []string{"a/.ground []", "a/b-fix [x.go]", "a/z-fix [n.go x.go]", "b/.ground []", "b/fix [q.go]"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roots\n %q\nwant\n %q", got, want)
	}
	if roots[0].Files != nil {
		t.Error(".ground is searched whole")
	}
	wantReplaced := map[string]map[string]bool{"a": {"x.go": true, "n.go": true}, "b": {"q.go": true}}
	if !reflect.DeepEqual(replaced, wantReplaced) {
		t.Errorf("replaced %v, want %v", replaced, wantReplaced)
	}

	roots, _, _ = codeRoots(l, "b")
	if len(roots) != 2 || roots[0].Repo != "b" || roots[1].Repo != "b" {
		t.Errorf("tagged b: %+v", roots)
	}
}

// Many changed files go to ripgrep in several runs, none too long.
func TestBatchRoots(t *testing.T) {
	ground := SearchRoot{SearchRoot: workspace.SearchRoot{Repo: "a", Worktree: ".ground", Path: "/r/a/.ground"}}
	capsule := SearchRoot{SearchRoot: workspace.SearchRoot{Repo: "a", Worktree: "fix", Path: "/r/a/fix"}}
	for i := range 10 {
		capsule.Files = append(capsule.Files, fmt.Sprintf("f%d.go", i))
	}
	batches := batchRoots([]SearchRoot{ground, capsule}, 60)
	var files []string
	for _, b := range batches {
		size := 0
		for _, r := range b {
			if r.Files == nil {
				size += len(r.Path) + 1
				continue
			}
			for _, f := range r.Files {
				size += len(r.Path) + len(f) + 2
				files = append(files, f)
			}
		}
		if size > 60 {
			t.Errorf("batch of %d bytes: %+v", size, b)
		}
	}
	if len(batches) < 3 || batches[0][0].Files != nil || !reflect.DeepEqual(files, capsule.Files) {
		t.Errorf("%d batches, files %q", len(batches), files)
	}
	if got := batchRoots([]SearchRoot{ground, capsule}, codeArgBytes); len(got) != 1 {
		t.Errorf("small enough for one run: %d", len(got))
	}
}

// A hit's text is valid UTF-8 with its ranges in UTF-16 units; a long
// line is cut to a window around its first match.
func TestCodeHit(t *testing.T) {
	root := workspace.SearchRoot{Repo: "a", Worktree: ".ground"}
	// "é" is 2 bytes and 1 unit; "😀" 4 bytes and 2 units; \xff isn't UTF-8.
	text := "é😀 needle \xff needle"
	h := codeHit(Match{Root: root, Path: "p.go", Line: 3, Column: 8, Text: text, Submatches: []Span{{7, 13}, {16, 22}}})
	if h.Text != "é😀 needle � needle" {
		t.Errorf("text %q", h.Text)
	}
	if want := [][2]int{{4, 10}, {13, 19}}; !reflect.DeepEqual(h.Ranges, want) {
		t.Errorf("ranges %v, want %v", h.Ranges, want)
	}
	units := []uint16{}
	for _, r := range h.Text {
		if r >= 0x10000 {
			units = append(units, 0, 0)
		} else {
			units = append(units, 0)
		}
	}
	if len(units) != 19 || h.Cut || h.Line != 3 || h.Column != 8 || h.Repo != "a" || h.Worktree != ".ground" || h.Path != "p.go" {
		t.Errorf("hit %+v (%d units)", h, len(units))
	}

	long := strings.Repeat("a", 5000) + "needle" + strings.Repeat("b", 5000) + "needle"
	h = codeHit(Match{Root: root, Text: long, Submatches: []Span{{5000, 5006}, {11006, 11012}}})
	if !h.Cut || len(h.Text) > codeLineBytes || len(h.Ranges) != 1 || h.Text[h.Ranges[0][0]:h.Ranges[0][1]] != "needle" {
		t.Errorf("cut: %d bytes, ranges %v, cut %v", len(h.Text), h.Ranges, h.Cut)
	}
	// A window that would start inside a rune starts at it.
	wide := strings.Repeat("é", 400) + "needle"
	h = codeHit(Match{Root: root, Text: wide, Submatches: []Span{{800, 806}}})
	if !strings.HasPrefix(h.Text, "é") || !strings.HasSuffix(h.Text, "needle") || len(h.Ranges) != 1 {
		t.Errorf("wide: %q %v", h.Text[:10], h.Ranges)
	}
}
