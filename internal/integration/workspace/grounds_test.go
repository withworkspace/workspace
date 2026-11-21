package workspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/testutil"
)

// followed is what following the grounds did, by repo.
func followed(t *testing.T, o *ops.Workspace) map[string]ops.GroundFollowed {
	t.Helper()
	out := map[string]ops.GroundFollowed{}
	for _, f := range o.FollowGrounds(context.Background()) {
		out[f.Repo] = f
	}
	return out
}

// The daemon keeps each repo's ground up with its remote: GitHub says in
// one request where every repo's branch is, and only a repo whose branch
// has moved there is fetched, then its ground moved forward.
func TestFollowGrounds_FetchesOnlyWhatMovedOnGitHub(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	asked := 0
	heads := map[string]string{}
	gh := &testutil.StubClient{HeadsFn: func(repos []ref.Remote, branch string) (map[ref.Remote]string, error) {
		asked++
		if branch != "main" || len(repos) != 2 || repos[0].Owner != "test-org" {
			t.Errorf("asked after %v %s", repos, branch)
		}
		out := map[ref.Remote]string{}
		for repo, oid := range heads {
			out[ref.Remote{Owner: "test-org", Name: repo}] = oid
		}
		return out, nil
	}}
	o := cli.OpenOps(w.Root, nil, func(o *ops.Options) { o.GitHub = gh })
	at := func(repo string) string {
		return strings.TrimSpace(testutil.GitOut(t, w.Sources[repo], "rev-parse", "main"))
	}

	// Nothing's moved: nothing's fetched.
	heads["api"], heads["records"] = at("api"), at("records")
	got := followed(t, o)
	if asked != 1 || len(got) != 2 || got["api"].Fetched || got["records"].Fetched || got["records"].Moved != 0 {
		t.Fatalf("asked %d times, followed %+v", asked, got)
	}

	// records moves on GitHub: it alone is fetched, and its ground follows.
	testutil.CommitIn(t, w.Sources["records"], "docs/later.md", "# Later\n", "docs: add later")
	heads["records"] = at("records")
	got = followed(t, o)
	if got["api"].Fetched || !got["records"].Fetched || got["records"].Moved != 1 || got["records"].At == "" || got["records"].Err != nil {
		t.Fatalf("followed %+v", got)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "repos", "records", ".ground", "docs", "later.md")); err != nil {
		t.Errorf("the ground after: %v", err)
	}
}

// When GitHub can't say, each repo's fetched; a ground with changes of its
// own, or on another branch, is held where it is.
func TestFollowGrounds_FetchesEachWithoutGitHubAndHoldsAGroundInUse(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	gh := &testutil.StubClient{HeadsFn: func([]ref.Remote, string) (map[ref.Remote]string, error) {
		return nil, errors.New("gh isn't signed in")
	}}
	o := cli.OpenOps(w.Root, nil, func(o *ops.Options) { o.GitHub = gh })

	testutil.CommitIn(t, w.Sources["api"], "later.txt", "later\n", "add later")
	testutil.CommitIn(t, w.Sources["records"], "later.txt", "later\n", "add later")
	apiGround := filepath.Join(w.Root, "repos", "api", ".ground")
	readme := firstTracked(t, apiGround)
	os.WriteFile(filepath.Join(apiGround, readme), []byte("mine\n"), 0o644)
	testutil.GitCmd(t, filepath.Join(w.Root, "repos", "records", ".ground"), "switch", "-c", "elsewhere")

	got := followed(t, o)
	if !got["api"].Fetched || !got["records"].Fetched {
		t.Fatalf("both should be fetched: %+v", got)
	}
	if h := got["api"].Held; h == nil || h.Code != "ground-dirty" || got["api"].Moved != 0 {
		t.Errorf("api: %+v", got["api"])
	}
	if h := got["records"].Held; h == nil || h.Code != "ground-branch" || got["records"].Moved != 0 {
		t.Errorf("records: %+v", got["records"])
	}
}

// firstTracked is a file the worktree's HEAD has.
func firstTracked(t *testing.T, dir string) string {
	t.Helper()
	first, _, _ := strings.Cut(testutil.GitOut(t, dir, "ls-files"), "\n")
	if first == "" {
		t.Fatal("nothing tracked in " + dir)
	}
	return first
}
