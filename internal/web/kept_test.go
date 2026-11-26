package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/review"
)

// getJSON is a signed-in GET's answer, decoded into v, and how many git
// processes answering it started.
func getJSON(t *testing.T, s *Server, path string, v any) (status int, gits int64) {
	t.Helper()
	before := git.Started()
	res := do(t, s, "GET", path, true, nil)
	defer res.Body.Close()
	gits = git.Started() - before
	if v != nil {
		reflect.ValueOf(v).Elem().SetZero() // what's left out isn't what was
		json.NewDecoder(res.Body).Decode(v)
	}
	return res.StatusCode, gits
}

// waitFor waits for ok, or fails saying what.
func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for range 400 {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("never: %s", what)
}

// lookingDone is whether no read behind is under way.
func (k *keptReads) lookingDone() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, e := range k.soons {
		if e.looking != nil {
			return false
		}
	}
	return true
}

func TestTreeAndSummaryAnswerAsKept(t *testing.T) {
	s, dir := reviewServer(t)
	const tree = "/api/missions/m/capsules/api/fix/tree"
	const summary = "/api/missions/m/capsules/api/fix/summary"

	// The first read is read (it's on screen, and nothing's kept).
	var tr struct {
		review.Tree
		Unread bool `json:"unread"`
	}
	if st, gits := getJSON(t, s, tree, &tr); st != 200 || tr.Unread || len(tr.Files) != 1 || gits == 0 {
		t.Fatalf("first tree: %d unread %v files %v (%d git)", st, tr.Unread, tr.Files, gits)
	}
	// Again, at once: from memory.
	if _, gits := getJSON(t, s, tree, &tr); gits != 0 {
		t.Errorf("a kept tree started %d git processes", gits)
	}

	// A row asks only what's kept: nothing's read for it.
	var sum struct {
		review.Summary
		Unread bool `json:"unread"`
	}
	if st, gits := getJSON(t, s, summary+"?kept=1", &sum); st != 200 || !sum.Unread || gits != 0 {
		t.Errorf("a summary never read, asked as kept: %d unread %v (%d git)", st, sum.Unread, gits)
	}
	if _, gits := getJSON(t, s, summary, &sum); sum.Unread || sum.CommitCount != 1 || gits == 0 {
		t.Errorf("the summary shown: unread %v %+v (%d git)", sum.Unread, sum.Summary, gits)
	}
	if _, gits := getJSON(t, s, summary+"?kept=1", &sum); sum.Unread || sum.CommitCount != 1 || gits != 0 {
		t.Errorf("a summary that's been read, asked as kept: unread %v (%d git)", sum.Unread, gits)
	}

	// A file made in the capsule moves nothing git keeps: past the floor,
	// the tree's answered as it was, at once, and read again behind; the
	// workspace's readsSeen moves when that finds the new file.
	defer func(f time.Duration) { readFloor = f }(readFloor)
	readFloor = 0
	seen := s.reads.Seen()
	view := func() Workspace {
		var ws Workspace
		getJSON(t, s, "/api/workspace", &ws)
		return ws
	}
	was := view().ReadsSeen
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644)
	getJSON(t, s, tree, &tr)
	if len(tr.Files) != 1 {
		t.Errorf("answered after waiting on git: %v", tr.Files)
	}
	waitFor(t, "the read behind found the new file", func() bool { return s.reads.Seen() > seen })
	if is := view().ReadsSeen; is == was {
		t.Errorf("the workspace's readsSeen stayed %q", is)
	}
	getJSON(t, s, tree, &tr)
	if len(tr.Files) != 2 {
		t.Errorf("after the read behind: %v", tr.Files)
	}
	// A read behind that finds nothing new doesn't move it.
	waitFor(t, "the read behind is done", s.reads.lookingDone)
	seen = s.reads.Seen()
	getJSON(t, s, tree, &tr)
	waitFor(t, "the read behind is done", s.reads.lookingDone)
	if s.reads.Seen() != seen {
		t.Error("readsSeen moved though nothing had changed")
	}
}

func TestATreeNeverReadSaysSo(t *testing.T) {
	s, _ := reviewServer(t)
	defer func(w time.Duration) { firstWait = w }(firstWait)
	firstWait = 0
	seen := s.reads.Seen()
	var tr struct {
		review.Tree
		Unread bool `json:"unread"`
	}
	if st, _ := getJSON(t, s, "/api/missions/m/capsules/api/fix/tree", &tr); st != 200 || !tr.Unread {
		t.Fatalf("a tree not read in time: %d unread %v", st, tr.Unread)
	}
	waitFor(t, "readsSeen moved when it was read", func() bool { return s.reads.Seen() > seen })
	if getJSON(t, s, "/api/missions/m/capsules/api/fix/tree", &tr); tr.Unread || len(tr.Files) != 1 {
		t.Errorf("once read: unread %v files %v", tr.Unread, tr.Files)
	}
}

func TestWhatsOfCommitsIsKept(t *testing.T) {
	s, dir := reviewServer(t)
	base, head := gitT(t, dir, "rev-parse", "HEAD~1"), gitT(t, dir, "rev-parse", "HEAD")
	q := url.Values{"base": {base}, "head": {head}, "path": {"a.go"}}.Encode()
	for _, path := range []string{
		"/api/missions/m/capsules/api/fix/file-diff?" + q,
		"/api/missions/m/capsules/api/fix/blob?" + url.Values{"rev": {head}, "path": {"a.go"}}.Encode(),
		"/api/missions/m/capsules/api/fix/diff",
		"/api/missions/m/capsules/api/fix/file?path=a.go",
	} {
		if st, gits := getJSON(t, s, path, nil); st != 200 || gits == 0 {
			t.Errorf("%s, first: %d (%d git)", path, st, gits)
		}
		if st, gits := getJSON(t, s, path, nil); st != 200 || gits != 0 {
			t.Errorf("%s, again: %d, %d git processes", path, st, gits)
		}
	}
	// A commit moves the capsule's stamp: its diff is read again.
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nvar x = 3\n"), 0o644)
	var v review.FileView
	if getJSON(t, s, "/api/missions/m/capsules/api/fix/file?path=a.go", &v); len(v.Lines) != 3 || v.Lines[2] != "var x = 3" {
		t.Errorf("the file, edited: %v", v.Lines)
	}
	gitT(t, dir, "commit", "-q", "-am", "three")
	var d review.Diff
	if getJSON(t, s, "/api/missions/m/capsules/api/fix/diff", &d); len(d.Commits) != 2 {
		t.Errorf("the diff after a commit has %d commits", len(d.Commits))
	}
}

func TestRefPathsFromWhatsKept(t *testing.T) {
	s, _ := reviewServer(t)
	ground := t.TempDir()
	gitT(t, ground, "init", "-q", "--initial-branch=main")
	gitT(t, ground, "config", "user.name", "Test")
	gitT(t, ground, "config", "user.email", "test@example.com")
	os.MkdirAll(filepath.Join(ground, "internal", "greet"), 0o755)
	os.WriteFile(filepath.Join(ground, "main.go"), []byte("package main\n"), 0o644)
	os.WriteFile(filepath.Join(ground, "internal", "greet", "greet.go"), []byte("package greet\n"), 0o644)
	gitT(t, ground, "add", "-A")
	gitT(t, ground, "commit", "-q", "-m", "base")
	s.Find = &find.Index{Worktrees: func() ([]find.Worktree, error) {
		return []find.Worktree{{Repo: "demo", Name: find.Ground, Dir: ground}}, nil
	}}
	ask := func() (RefPathsFound, int64) {
		t.Helper()
		data, _ := json.Marshal(RefPaths{Repos: map[string][]string{"demo": {"main.go", "internal/greet", "internal/greet/", "nope.go"}, "other": {"x.go"}}})
		before := git.Started()
		res := do(t, s, "POST", "/api/refs/paths", true, func(r *http.Request) {
			r.Body = io.NopCloser(bytes.NewReader(data))
			r.ContentLength = int64(len(data))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", s.Info().Origin())
		})
		defer res.Body.Close()
		var out RefPathsFound
		if res.StatusCode != 200 {
			t.Fatalf("refs/paths: %d", res.StatusCode)
		}
		json.NewDecoder(res.Body).Decode(&out)
		return out, git.Started() - before
	}
	seen := s.groundsSeen()
	// Never read: nothing to say of demo yet, and it's read behind.
	if got, _ := ask(); len(got.Repos) != 0 {
		t.Fatalf("answered before demo was read: %+v", got)
	}
	waitFor(t, "groundsSeen moved once demo was read", func() bool { return s.groundsSeen() != seen })
	got, gits := ask()
	demo := got.Repos["demo"]
	if demo.Head == "" || len(demo.Files) != 1 || demo.Files[0] != "main.go" || len(demo.Dirs) != 2 || len(got.Repos) != 1 {
		t.Errorf("demo: %+v (all %+v)", demo, got)
	}
	if gits != 0 {
		t.Errorf("answering from what's kept started %d git processes", gits)
	}
	// The ground moves: groundsSeen does too, before anything's read.
	seen = s.groundsSeen()
	os.WriteFile(filepath.Join(ground, "nope.go"), []byte("package main\n"), 0o644)
	gitT(t, ground, "add", "-A")
	gitT(t, ground, "commit", "-q", "-m", "nope")
	if s.groundsSeen() == seen {
		t.Error("groundsSeen didn't move with the ground's HEAD")
	}
}

// A capsule that moves (a commit) has what a page lately asked of it read
// again behind, as the daemon's told: the next ask is answered with it as
// it is, without waiting, and the page's told it changed.
func TestTreeReadAgainWhenItsCapsuleMoves(t *testing.T) {
	s, dir := reviewServer(t)
	const tree = "/api/missions/m/capsules/api/fix/tree"
	var tr struct {
		review.Tree
		Unread bool `json:"unread"`
	}
	getJSON(t, s, tree, &tr)
	waitFor(t, "the first read", s.reads.lookingDone)
	seen := s.reads.Seen()
	os.WriteFile(filepath.Join(dir, "more.txt"), []byte("more\n"), 0o644)
	if out, err := git.Combined(dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "add", "more.txt"); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	s.reads.refresh() // as the daemon does, told the capsule's moved
	waitFor(t, "read again behind", func() bool { return s.reads.Seen() > seen })
	if _, gits := getJSON(t, s, tree, &tr); gits != 0 || len(tr.Files) != 2 {
		t.Errorf("after the capsule moved: %d files, %d git on the read", len(tr.Files), gits)
	}
}
