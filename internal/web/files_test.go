package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/review"
)

func TestFilesAPI(t *testing.T) {
	s, dir := reviewServer(t)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "dot.png"), []byte("\x89PNG\r\n\x1a\n"), 0o644)

	res := do(t, s, "GET", "/api/missions/m/capsules/api/fix/tree", true, nil)
	var tr review.Tree
	if err := json.NewDecoder(res.Body).Decode(&tr); err != nil || res.StatusCode != 200 {
		t.Fatalf("tree: %d %v", res.StatusCode, err)
	}
	status := map[string]string{}
	for _, f := range tr.Files {
		status[f.Path] = f.Status
	}
	if status["a.go"] != "modified" || status["new.txt"] != "added" {
		t.Errorf("tree statuses %v", status)
	}

	res = do(t, s, "GET", "/api/missions/m/capsules/api/fix/file?"+url.Values{"path": {"a.go"}}.Encode(), true, nil)
	var v review.FileView
	if json.NewDecoder(res.Body).Decode(&v); res.StatusCode != 200 || len(v.Lines) != 3 || len(v.Changes) != 1 {
		t.Errorf("file: %d %+v", res.StatusCode, v)
	}

	// Only paths in the capsule, and only images as they are.
	for _, bad := range []string{"../x", "/etc/passwd", "-x", ""} {
		res := do(t, s, "GET", "/api/missions/m/capsules/api/fix/file?"+url.Values{"path": {bad}}.Encode(), true, nil)
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("file %q: %d, want 400", bad, res.StatusCode)
		}
	}
	if res := do(t, s, "GET", "/api/missions/m/capsules/api/fix/raw?"+url.Values{"path": {"a.go"}}.Encode(), true, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("raw a.go: %d, want 400 (not an image)", res.StatusCode)
	}
	res = do(t, s, "GET", "/api/missions/m/capsules/api/fix/raw?"+url.Values{"path": {"dot.png"}}.Encode(), true, nil)
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/png" {
		t.Errorf("raw dot.png: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}

	res = do(t, s, "GET", "/api/missions/m/capsules/api/fix/summary", true, nil)
	var sum review.Summary
	if json.NewDecoder(res.Body).Decode(&sum); res.StatusCode != 200 || sum.CommitCount != 1 || sum.Files != 1 || sum.Uncommitted != 2 {
		t.Errorf("summary: %d %+v", res.StatusCode, sum)
	}
}

func TestAgentAPI(t *testing.T) {
	s, _ := reviewServer(t)
	var said, answered string
	fake(s).crew = crewFuncs{
		say: func(slug, id, text string) (string, error) {
			said = slug + "/" + id + ": " + text
			return "Sent to " + id + ".", nil
		},
		answer: func(slug, id string, a ops.CrewAnswer) (string, error) {
			if a.Option == "missing" {
				return "", ErrNotOnScreen
			}
			answered = a.Permission + a.Option
			return "Allowed it once.", nil
		},
	}
	if res, _ := write(t, s, "POST", "/api/missions/m/crew/c1/say", SayRequest{Text: "add a test"}); res.StatusCode != 200 || said != "m/c1: add a test" {
		t.Errorf("say: %d %q", res.StatusCode, said)
	}
	if res, _ := write(t, s, "POST", "/api/missions/m/crew/c1/say", SayRequest{Text: "  "}); res.StatusCode != http.StatusBadRequest {
		t.Errorf("empty say: %d", res.StatusCode)
	}
	if res, _ := write(t, s, "POST", "/api/missions/m/crew/c1/answer", AnswerRequest{Permission: "allow"}); res.StatusCode != 200 || answered != "allow" {
		t.Errorf("answer: %d %q", res.StatusCode, answered)
	}
	if res, _ := write(t, s, "POST", "/api/missions/m/crew/c1/answer", AnswerRequest{Permission: "sudo"}); res.StatusCode != http.StatusBadRequest {
		t.Errorf("bad permission: %d", res.StatusCode)
	}
	if res, _ := write(t, s, "POST", "/api/missions/m/crew/c1/answer", AnswerRequest{}); res.StatusCode != http.StatusBadRequest {
		t.Errorf("empty answer: %d", res.StatusCode)
	}
	// Not on screen: refused, as a conflict.
	if res, _ := write(t, s, "POST", "/api/missions/m/crew/c1/answer", AnswerRequest{Option: "missing"}); res.StatusCode != http.StatusConflict {
		t.Errorf("not on screen: %d", res.StatusCode)
	}
}

// The Files page reads any of the workspace's worktrees as the explorer
// reads a capsule: the ground with nothing to compare with, a capsule
// against its base. Repos and a repo's worktrees are listed.
func TestRepoFilesAPI(t *testing.T) {
	s, dir := reviewServer(t)
	fake(s).worktree = func(repo, name string) (string, string, error) {
		switch {
		case repo == "api" && name == ".ground":
			return dir, "", nil
		case repo == "api" && name == "fix":
			return dir, "origin/main", nil
		}
		return "", "", ErrNotFound
	}
	fake(s).repos = func() []ops.Repo {
		return []ops.Repo{{Name: "api", Label: "API", Color: "#5f5fff", Cloned: true, Capsules: []string{"fix"}}}
	}
	fake(s).worktrees = func(repo string) ([]ops.Worktree, error) {
		if repo != "api" {
			return nil, ErrNotFound
		}
		return []ops.Worktree{{Name: ".ground", Ground: true, Branch: "main"}, {Name: "fix", Branch: "fix", Dirty: true}}, nil
	}

	var tr review.Tree
	res := do(t, s, "GET", "/api/repos/api/worktrees/.ground/tree", true, nil)
	if json.NewDecoder(res.Body).Decode(&tr); res.StatusCode != 200 || tr.Base != "" || len(tr.Files) == 0 {
		t.Errorf("the ground's tree: %d %+v", res.StatusCode, tr)
	}
	for _, f := range tr.Files {
		if f.Status != "" {
			t.Errorf("the ground has nothing to compare with, but %s is %s", f.Path, f.Status)
		}
	}
	res = do(t, s, "GET", "/api/repos/api/worktrees/fix/file?"+url.Values{"path": {"a.go"}}.Encode(), true, nil)
	var v review.FileView
	if json.NewDecoder(res.Body).Decode(&v); res.StatusCode != 200 || len(v.Changes) != 1 {
		t.Errorf("a capsule's file: %d %+v", res.StatusCode, v)
	}
	if res := do(t, s, "GET", "/api/repos/api/worktrees/nope/tree", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("no such worktree: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/api/repos/api/worktrees/fix/file?"+url.Values{"path": {"../x"}}.Encode(), true, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("a path out of it: %d", res.StatusCode)
	}

	var repos struct{ Repos []Repo }
	if json.NewDecoder(do(t, s, "GET", "/api/repos", true, nil).Body).Decode(&repos); len(repos.Repos) != 1 || repos.Repos[0].Label != "API" {
		t.Errorf("repos: %+v", repos)
	}
	var wts struct{ Worktrees []Worktree }
	if json.NewDecoder(do(t, s, "GET", "/api/repos/api/worktrees", true, nil).Body).Decode(&wts); len(wts.Worktrees) != 2 || !wts.Worktrees[0].Ground {
		t.Errorf("worktrees: %+v", wts)
	}
	if res := do(t, s, "GET", "/api/repos/nope/worktrees", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("no such repo: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/api/repos", false, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("signed out: %d", res.StatusCode)
	}
}
