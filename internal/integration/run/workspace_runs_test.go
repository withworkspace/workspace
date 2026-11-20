package run_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// A workspace with a recipe of its own, docs, in ws.toml: it needs web's
// folder and api's server, each of which can be pointed.
func workspaceRunsWorkspace(t *testing.T) *testutil.Workspace {
	t.Helper()
	w, _ := needsWorkspace(t)
	testutil.Run(t, w, nil, "lift", "web", "other")
	f, err := os.OpenFile(filepath.Join(w.Root, "ws.toml"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`
[run.docs]
command = "sleep 300 # {web}"

[run.docs.needs.web]
repo = "web"
env = "WEB_DIR"

[run.docs.needs.api]
run = "api/server"
`)
	f.Close()
	t.Setenv(cli.ShellPWD, w.Root)
	return w
}

func TestRun_TheWorkspacesOwnRecipeRunsAtItsRoot(t *testing.T) {
	w := workspaceRunsWorkspace(t)
	res := testutil.RunCommand(t, w.Root, nil, "run")
	if res.Err != nil || !strings.Contains(res.Stdout, "docs") || !strings.Contains(res.Stdout, "web (web's folder)") {
		t.Fatalf("ws run alone at the root: %v\n%s%s", res.Err, res.Stdout, res.Stderr)
	}
	res = testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "none", "docs")
	if res.Err != nil {
		t.Fatalf("ws run docs: %v\n%s", res.Err, res.Stderr)
	}
	ground := filepath.Join(w.Root, "repos", "web", ".ground")
	if !strings.Contains(res.Stdout, "name   docs\n") || !strings.Contains(res.Stdout, "needs  web  web/ground  "+ground+"  $WEB_DIR") {
		t.Errorf("ws run docs printed:\n%s", res.Stdout)
	}
	list := lsJSON(t, w)
	if len(list) != 2 || list[0]["ref"] != "docs" || list[1]["ref"] != "server.ground.api" {
		t.Fatalf("ls: %v", list)
	}
	if list[0]["command"] != "sleep 300 # "+ground || list[0]["dir"] != w.Root {
		t.Errorf("docs ran %v in %v", list[0]["command"], list[0]["dir"])
	}
	if _, ok := cli.RouteFile(w.Root, 47001, "shop", t.TempDir()).Routes["docs"]; !ok {
		t.Errorf("docs has no route of its own name")
	}
	// Asked for again, from anywhere outside the repos, it's the same run.
	os.MkdirAll(filepath.Join(w.Root, "notes"), 0o755)
	t.Setenv(cli.ShellPWD, filepath.Join(w.Root, "notes"))
	res = testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "none", "docs")
	if res.Err != nil || !strings.Contains(res.Stderr, "docs is running here already") {
		t.Fatalf("ws run docs again: %v\n%s", res.Err, res.Stderr)
	}
	// The repos' folder is no place to run anything.
	t.Setenv(cli.ShellPWD, filepath.Join(w.Root, "repos", "web"))
	res = testutil.RunCommand(t, w.Root, nil, "run", "--", "sleep", "300")
	if res.Err == nil || !strings.Contains(res.Err.Error(), "stand in one") {
		t.Fatalf("in a repo's folder: %v", res.Err)
	}
}

func TestRun_PointingANeedStartsTheRecipeAgainOnIt(t *testing.T) {
	w := workspaceRunsWorkspace(t)
	res := testutil.RunCommand(t, w.Root, nil, "run", "point", "docs")
	if res.Err != nil || !strings.Contains(res.Stdout, "api   api/server    ground (nearest)  ground\n") ||
		!strings.Contains(res.Stdout, "web   web (folder)  ground (nearest)  ground, feat, other\n") {
		t.Fatalf("ws run point docs: %v\n%s%s", res.Err, res.Stdout, res.Stderr)
	}
	// Pointed while it isn't running, it's started on it when it is.
	res = testutil.RunCommand(t, w.Root, nil, "run", "point", "docs", "web=other")
	if res.Err != nil || !strings.Contains(res.Stderr, "web is pointed at other.") {
		t.Fatalf("point web=other: %v\n%s", res.Err, res.Stderr)
	}
	if res := testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "none", "--ttl", "2", "docs"); res.Err != nil {
		t.Fatalf("ws run docs: %v\n%s", res.Err, res.Stderr)
	}
	if list := lsJSON(t, w); list[0]["command"] != "sleep 300 # "+testutil.CapsuleDir(w, "web", "other") {
		t.Fatalf("docs ran %v", list[0]["command"])
	}
	// Pointed while it runs, it's started again on it, for the time it had.
	res = testutil.RunCommand(t, w.Root, nil, "run", "point", "--wait", "0", "docs", "web=feat")
	if res.Err != nil || !strings.Contains(res.Stderr, "web is pointed at feat. docs was started again on it.") {
		t.Fatalf("point web=feat: %v\n%s", res.Err, res.Stderr)
	}
	list := lsJSON(t, w)
	if list[0]["command"] != "sleep 300 # "+testutil.CapsuleDir(w, "web", "feat") || list[0]["ends"] != "in 2h00" {
		t.Fatalf("docs after pointing: %v ends %v", list[0]["command"], list[0]["ends"])
	}
	testutil.Run(t, w, nil, "lift", "api", "feat") // with no recipes: its ws.repo.toml is the ground's alone
	t.Setenv(cli.ShellPWD, w.Root)
	for _, c := range []struct{ said, want string }{
		{"web=nope", "web has no nope"},
		{"api=feat", "api/feat has no recipe server"},
		{"db=ground", "docs has no need db that can be pointed (its: api, web)"},
	} {
		if res := testutil.RunCommand(t, w.Root, nil, "run", "point", "docs", c.said); res.Err == nil || !strings.Contains(res.Err.Error(), c.want) {
			t.Errorf("point %s: %v", c.said, res.Err)
		}
	}
	// It's yours to point: crew are refused.
	t.Setenv(mission.EnvCrew, "c1")
	if res := testutil.RunCommand(t, w.Root, nil, "run", "point", "docs", "web=ground"); res.Err == nil || !strings.Contains(res.Err.Error(), "yours to do") {
		t.Errorf("crew pointing: %v", res.Err)
	}
	t.Setenv(mission.EnvCrew, "")

	// The capsule it's pointed at burned, it's ended, and started again
	// on the nearest, which it says.
	testutil.StubConfirm(t, true)
	if res := testutil.RunCommand(t, w.Root, nil, "burn", "web", "feat"); res.Err != nil {
		t.Fatalf("burn: %v\n%s", res.Err, res.Stderr)
	}
	res = testutil.RunCommand(t, w.Root, nil, "run", "ls")
	if !strings.Contains(res.Stderr, "Ended docs: feat, the folder it was started on for web, has gone.") {
		t.Errorf("ls said:\n%s%s", res.Stdout, res.Stderr)
	}
	res = testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "none", "docs")
	if res.Err != nil || !strings.Contains(res.Stderr, "web was pointed at feat, which web no longer has: it's on the nearest now.") {
		t.Fatalf("docs again: %v\n%s", res.Err, res.Stderr)
	}
}
