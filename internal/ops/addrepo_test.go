package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// lister is GitHub as adding a repo asks it: whose repos there are, and
// each one's. asked counts the looks.
type lister struct {
	*github.Stub
	owners []github.Owner
	repos  map[string][]github.OwnedRepo
	asked  int
}

func (l *lister) Owners() ([]github.Owner, error) {
	l.asked++
	return l.owners, nil
}

func (l *lister) ReposOf(owner string) ([]github.OwnedRepo, error) {
	l.asked++
	r, ok := l.repos[owner]
	if !ok {
		return nil, errors.New("Could not resolve to a RepositoryOwner")
	}
	return r, nil
}

// source makes a repo with one commit on main, and has a clone of
// github.com/<at>.git come from it.
func source(t *testing.T, at ...string) {
	t.Helper()
	for i, slug := range at {
		dir := t.TempDir()
		for _, args := range [][]string{{"init", "-q", "--initial-branch=main"}, {"-c", "user.name=T", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "first"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
		}
		t.Setenv(fmt.Sprintf("GIT_CONFIG_KEY_%d", i), "url."+dir+".insteadOf")
		t.Setenv(fmt.Sprintf("GIT_CONFIG_VALUE_%d", i), "https://github.com/"+slug+".git")
	}
	t.Setenv("GIT_CONFIG_COUNT", fmt.Sprint(len(at)))
}

func addWorld(t *testing.T) (*Workspace, *lister, string) {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\ndefault_branch = \"main\"\n\n[repos.api]\n"), 0o644)
	gh := &lister{Stub: &github.Stub{},
		owners: []github.Owner{{Login: "ana", You: true}, {Login: "acme"}},
		repos: map[string][]github.OwnedRepo{
			"acme": {{Name: "web", PushedAt: "2026-11-20T10:00:00Z"}, {Name: "api"}},
			"ana":  {{Name: "notes", Description: "mine"}},
		}}
	return Open(root, Options{GitHub: gh}), gh, root
}

// waitSeen waits for a look behind a read, or a clone, to finish.
func waitSeen(t *testing.T, w *Workspace, from string) {
	t.Helper()
	for range 200 {
		if w.GitHubSeen() != from {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("nothing moved")
}

// Whose repos and an owner's repos answer at once with what GitHub last
// said (nothing, the first time) and look behind; asked again soon after,
// they answer from what was said without asking. A repo that's here is
// said by its name here.
func TestGitHubListings_AnswerAtOnceAndLookBehind(t *testing.T) {
	w, gh, _ := addWorld(t)
	seen := w.GitHubSeen()
	if got := w.GitHubOwners(false); !got.Looking || len(got.Owners) != 0 {
		t.Fatalf("first: %+v, want nothing yet and a look", got)
	}
	waitSeen(t, w, seen)
	if got := w.GitHubOwners(false); got.Looking || len(got.Owners) != 2 || !got.Owners[0].You || got.At.IsZero() {
		t.Errorf("then: %+v", got)
	}
	if gh.asked != 1 {
		t.Errorf("asked %d times, want once", gh.asked)
	}

	seen = w.GitHubSeen()
	w.GitHubRepos("acme", false)
	waitSeen(t, w, seen)
	got := w.GitHubRepos("acme", false)
	if len(got.Repos) != 2 || got.Repos[0].Name != "web" || got.Repos[0].Here != "" || got.Repos[1].Here != "api" {
		t.Errorf("acme's: %+v", got.Repos)
	}

	seen = w.GitHubSeen()
	w.GitHubRepos("nobody", false)
	waitSeen(t, w, seen)
	if got := w.GitHubRepos("nobody", false); got.Problem == "" {
		t.Errorf("nobody's: %+v, want why not", got)
	}
}

// A repo added for you goes in ws.local.toml, said where it is when it
// isn't the org's of its name, and is cloned; one whose name is taken
// here is refused until it's given another.
func TestAddRepo_ForYouAndCloned(t *testing.T) {
	w, _, root := addWorld(t)
	source(t, "ana/notes", "other/api")

	done, err := w.AddRepo(context.Background(), AsYou, RepoAdd{Owner: "ana", Name: "notes", Wait: true})
	if err != nil {
		t.Fatal(err)
	}
	if done.Repo != "notes" || done.Team || done.Cloning {
		t.Errorf("added %+v", done)
	}
	local, _ := os.ReadFile(filepath.Join(root, "ws.local.toml"))
	if !strings.Contains(string(local), "[repos.notes]\ngithub = \"ana/notes\"") {
		t.Errorf("ws.local.toml:\n%s", local)
	}
	if _, err := os.Stat(filepath.Join(root, "repos", "notes", ".ground")); err != nil {
		t.Errorf("not cloned: %v", err)
	}
	if ws := w.WS(); !ws.Yours["notes"] {
		t.Errorf("notes isn't yours: %v", ws.Yours)
	}

	if _, err := w.AddRepo(context.Background(), AsYou, RepoAdd{Owner: "other", Name: "api"}); err == nil || !strings.Contains(err.Error(), "another name") {
		t.Errorf("api's name is taken: %v", err)
	}
	if _, err := w.AddRepo(context.Background(), AsYou, RepoAdd{Owner: "ana", Name: "notes", As: "notes2"}); err == nil || !strings.Contains(err.Error(), "already added, as notes") {
		t.Errorf("ana/notes is here: %v", err)
	}
	if _, err := w.AddRepo(context.Background(), AsCrew("c1"), RepoAdd{Owner: "other", Name: "api", As: "other-api"}); err == nil {
		t.Error("crew shouldn't add repos")
	}

	// Cloned behind the answer: said as cloning until it is.
	seen := w.GitHubSeen()
	done, err = w.AddRepo(context.Background(), AsYou, RepoAdd{Owner: "other", Name: "api", As: "other-api"})
	if err != nil || !done.Cloning {
		t.Fatalf("added %+v, %v", done, err)
	}
	waitSeen(t, w, seen)
	for _, r := range w.Repos() {
		if r.Name == "other-api" && (r.Cloning || r.Problem != "" || !r.Cloned || r.GitHub != "other/api") {
			t.Errorf("other-api after: %+v", r)
		}
	}
}

// A repo added for the team goes in ws.toml, committed and pushed: as
// the org's of its name, it says nothing of where it is.
func TestAddRepo_ForTheTeam(t *testing.T) {
	w, root, remote := sharedWorld(t)
	source(t, "acme/web")
	done, err := w.AddRepo(context.Background(), AsYou, RepoAdd{Owner: "acme", Name: "web", Team: true, Wait: true})
	if err != nil {
		t.Fatal(err)
	}
	if !done.Team || done.Commit == "" {
		t.Errorf("added %+v", done)
	}
	if pushed := shareGit(t, remote, "show", "main:ws.toml"); !strings.HasSuffix(pushed, "[repos.api]\n\n[repos.web]") {
		t.Errorf("the remote's ws.toml:\n%s", pushed)
	}
	if body := shareGit(t, remote, "log", "-1", "--format=%s%n%b", "main"); !strings.Contains(body, "Add repo web") || !strings.Contains(body, "acme/web, added by") {
		t.Errorf("the commit: %s", body)
	}
	if local, _ := os.ReadFile(filepath.Join(root, "ws.local.toml")); strings.Contains(string(local), "web") {
		t.Errorf("ws.local.toml has it:\n%s", local)
	}
}
