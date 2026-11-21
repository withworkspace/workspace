package run_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/testutil"
)

func runsWorkspace(t *testing.T) (*testutil.Workspace, string) {
	t.Helper()
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	t.Setenv(mission.EnvMission, "")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvRole, "")
	cli.NoRunsWatcher(t)
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "test-org", DefaultBranch: "main", Repos: []testutil.RepoOpts{{Name: "api"}}})
	testutil.Run(t, w, nil, "lift", "api", "feat")
	dir := testutil.CapsuleDir(w, "api", "feat")
	t.Setenv(cli.ShellPWD, dir)
	t.Cleanup(func() { testutil.RunCommand(t, w.Root, nil, "run", "stop", "--all") })
	return w, dir
}

func lsJSON(t *testing.T, w *testutil.Workspace) []map[string]any {
	t.Helper()
	res := testutil.RunCommand(t, w.Root, nil, "run", "ls", "--json")
	if res.Err != nil {
		t.Fatalf("ws run ls: %v\n%s", res.Err, res.Stderr)
	}
	var out []map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		t.Fatalf("%v\n%s", err, res.Stdout)
	}
	return out
}

func TestRun_StartsListsAndStops(t *testing.T) {
	w, _ := runsWorkspace(t)
	res := testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "capsule", "--", "sleep", "300")
	if res.Err != nil {
		t.Fatalf("ws run: %v\n%s", res.Err, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "name   sleep.feat.api") || !strings.Contains(res.Stdout, "when capsule feat is burned or landed, or in 4h00") {
		t.Errorf("ws run said:\n%s", res.Stdout)
	}
	// A second of the same name is the next free one, not refused.
	res = testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "none", "--ttl", "1", "--", "sleep", "300")
	if res.Err != nil || !strings.Contains(res.Stderr, "sleep is running already: this one is sleep-2") {
		t.Fatalf("second run: %v\n%s", res.Err, res.Stderr)
	}
	list := lsJSON(t, w)
	if len(list) != 2 || list[0]["ref"] != "sleep.feat.api" || list[1]["ref"] != "sleep-2.feat.api" {
		t.Fatalf("ls: %v", list)
	}
	if list[0]["state"] != "running" || list[1]["ends"] != "in 1h00" {
		t.Errorf("ls: %v", list)
	}
	if res := testutil.RunCommand(t, w.Root, nil, "run", "stop", "sleep-2"); res.Err != nil || !strings.Contains(res.Stderr, "Stopped sleep-2.feat.api") {
		t.Fatalf("stop: %v\n%s", res.Err, res.Stderr)
	}
	if list := lsJSON(t, w); len(list) != 1 {
		t.Fatalf("after stop: %v", list)
	}
}

func TestRun_CapsuleBurnedEndsItsRun(t *testing.T) {
	w, dir := runsWorkspace(t)
	if res := testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "capsule", "-n", "web", "--", "sleep", "300"); res.Err != nil {
		t.Fatalf("ws run: %v\n%s", res.Err, res.Stderr)
	}
	t.Setenv(cli.ShellPWD, w.Root)
	testutil.StubConfirm(t, true)
	if res := testutil.RunCommand(t, w.Root, nil, "burn", "api", "feat"); res.Err != nil {
		t.Fatalf("burn: %v\n%s", res.Err, res.Stderr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("the capsule's still there")
	}
	res := testutil.RunCommand(t, w.Root, nil, "run", "ls")
	if !strings.Contains(res.Stderr, "Ended web.feat.api: where it ran has gone.") {
		t.Errorf("ls said:\n%s\n%s", res.Stdout, res.Stderr)
	}
}

func TestRun_ForAMissionNeedsOne(t *testing.T) {
	w, _ := runsWorkspace(t)
	res := testutil.RunCommand(t, w.Root, nil, "run", "--for", "mission", "--", "sleep", "300")
	if res.Err == nil || !strings.Contains(res.Err.Error(), "isn't a mission's capsule") {
		t.Fatalf("err = %v", res.Err)
	}
	res = testutil.RunCommand(t, w.Root, nil, "run", "--for", "capsule", "--ttl", "0", "--wait", "0", "--", "sleep", "300")
	if res.Err != nil {
		t.Fatalf("you may run with no limit: %v", res.Err)
	}
	t.Setenv(mission.EnvCrew, "c1")
	res = testutil.RunCommand(t, w.Root, nil, "run", "--for", "capsule", "--ttl", "0", "--", "sleep", "300")
	if res.Err == nil || !strings.Contains(res.Err.Error(), "crew's runs need an end") {
		t.Fatalf("crew with no limit: %v", res.Err)
	}
}

func TestRun_OutsideAWorktreeSaysWhere(t *testing.T) {
	w, _ := runsWorkspace(t)
	t.Setenv(cli.ShellPWD, filepath.Join(w.Root, "repos", "api"))
	res := testutil.RunCommand(t, w.Root, nil, "run", "--", "sleep", "300")
	if res.Err == nil || !strings.Contains(res.Err.Error(), "stand in one") {
		t.Fatalf("err = %v", res.Err)
	}
}

func TestRun_ARecipe(t *testing.T) {
	w, dir := runsWorkspace(t)
	os.WriteFile(filepath.Join(dir, "ws.repo.toml"), []byte(`
[run.web]
command = "sleep 300"
stop = "echo stopped $PORT > stopped.txt"
`), 0o644)
	res := testutil.RunCommand(t, w.Root, nil, "run")
	if res.Err != nil || !strings.Contains(res.Stdout, "web     sleep 300") {
		t.Fatalf("ws run alone: %v\n%s%s", res.Err, res.Stdout, res.Stderr)
	}
	res = testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "web")
	if res.Err != nil || !strings.Contains(res.Stdout, "name   web.feat.api") {
		t.Fatalf("ws run web: %v\n%s%s", res.Err, res.Stdout, res.Stderr)
	}
	list := lsJSON(t, w)
	if len(list) != 1 || list[0]["command"] != "sleep 300" {
		t.Fatalf("ls: %v", list)
	}
	// After --, a word is a command, recipe or not.
	res = testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--", "web")
	if res.Err != nil {
		t.Fatalf("ws run -- web: %v", res.Err)
	}
	if list := lsJSON(t, w); len(list) != 2 || list[1]["command"] != "web" {
		t.Fatalf("ls: %v", list)
	}
	if res := testutil.RunCommand(t, w.Root, nil, "run", "stop", "web"); res.Err != nil {
		t.Fatalf("stop: %v\n%s", res.Err, res.Stderr)
	}
	b, err := os.ReadFile(filepath.Join(dir, "stopped.txt"))
	if err != nil || !strings.HasPrefix(string(b), "stopped 48") {
		t.Errorf("the recipe's stop: %q %v", b, err)
	}
}

func TestRun_CrewsRunIsTheirStepsUntilItPasses(t *testing.T) {
	w, _ := runsWorkspace(t)
	missions := mission.Store{Root: w.Root}
	m, err := missions.Create("Build it")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	missions.Update(m.Slug, func(m *mission.Mission) error {
		m.Crew = append(m.Crew, mission.Crew{ID: "c1", Harness: "claude", Started: now})
		m.Stages = []mission.StageRecord{
			{Text: "Plan it", N: 1, Of: 2, Crew: []string{"c0"}, Accepted: &now},
			{Text: "Build the thing", N: 2, Of: 2, Crew: []string{"c1"}, Started: &now},
		}
		return nil
	})
	t.Setenv(mission.EnvMission, m.Slug)
	t.Setenv(mission.EnvCrew, "c1")
	res := testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "-n", "web", "--", "sleep", "300")
	if res.Err != nil || !strings.Contains(res.Stdout, "when step 2 of mission "+m.Slug+" passes, or in 4h00") {
		t.Fatalf("ws run: %v\n%s%s", res.Err, res.Stdout, res.Stderr)
	}
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvMission, "")
	if res := testutil.RunCommand(t, w.Root, nil, "run", "ls"); strings.Contains(res.Stderr, "Ended") {
		t.Fatalf("ended while its step was going:\n%s", res.Stderr)
	}
	missions.Update(m.Slug, func(m *mission.Mission) error {
		m.Stages[1].Accepted = &now
		return nil
	})
	res = testutil.RunCommand(t, w.Root, nil, "run", "ls")
	if !strings.Contains(res.Stderr, "Ended web.feat.api: step 2 of mission "+m.Slug+" has passed.") {
		t.Fatalf("ls:\n%s%s", res.Stdout, res.Stderr)
	}
}

func TestRun_CrewOnNoStepOwnTheirRunsThemselves(t *testing.T) {
	w, _ := runsWorkspace(t)
	missions := mission.Store{Root: w.Root}
	m, _ := missions.Create("Brief it")
	now := time.Now()
	missions.Update(m.Slug, func(m *mission.Mission) error {
		m.Crew = append(m.Crew, mission.Crew{ID: "c1", Harness: "claude", Started: now})
		return nil
	})
	t.Setenv(mission.EnvMission, m.Slug)
	t.Setenv(mission.EnvCrew, "c1")
	res := testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "-n", "web", "--", "sleep", "300")
	if res.Err != nil || !strings.Contains(res.Stdout, "when crew c1 of mission "+m.Slug+" close") {
		t.Fatalf("ws run: %v\n%s%s", res.Err, res.Stdout, res.Stderr)
	}
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvMission, "")
	missions.Update(m.Slug, func(m *mission.Mission) error {
		m.Crew[0].Ended = &now
		return nil
	})
	res = testutil.RunCommand(t, w.Root, nil, "run", "ls")
	if !strings.Contains(res.Stderr, "crew c1 of mission "+m.Slug+" have closed") {
		t.Fatalf("ls:\n%s%s", res.Stdout, res.Stderr)
	}
}

func TestRun_AConversationsRunEndsWhenItsResolved(t *testing.T) {
	w, _ := runsWorkspace(t)
	talks := conversation.ForWorkspace(w.Root)
	c, err := talks.OpenTalk("you", "Run it for me", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(conversation.EnvConversation, c.ID)
	res := testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "-n", "web", "--", "sleep", "300")
	if res.Err != nil || !strings.Contains(res.Stdout, "when conversation "+c.ID+" is resolved") {
		t.Fatalf("ws run: %v\n%s%s", res.Err, res.Stdout, res.Stderr)
	}
	t.Setenv(conversation.EnvConversation, "")
	if _, err := talks.SetStatus(c.ID, "you", conversation.Resolved, time.Now()); err != nil {
		t.Fatal(err)
	}
	res = testutil.RunCommand(t, w.Root, nil, "run", "ls")
	if !strings.Contains(res.Stderr, "conversation "+c.ID+" was resolved") {
		t.Fatalf("ls:\n%s%s", res.Stdout, res.Stderr)
	}
}

func TestRun_TheRouteFileHasTheRunsAndSilos(t *testing.T) {
	w, dir := runsWorkspace(t)
	ground := filepath.Join(w.Root, "repos", "api", ".ground")
	os.WriteFile(filepath.Join(ground, "ws.repo.toml"), []byte("[run.stack]\ncommand = \"sleep 300\"\nwhere = \"silo\"\n\n[run.web]\ncommand = \"sleep 300\"\n"), 0o644)
	testutil.StubConfirm(t, true)
	if res := testutil.RunCommand(t, w.Root, nil, "silo", "point", "api", "feat"); res.Err != nil {
		t.Fatalf("silo point: %v\n%s", res.Err, res.Stderr)
	}
	if res := testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "none", "-n", "web", "--", "sleep", "300"); res.Err != nil {
		t.Fatalf("ws run: %v\n%s", res.Err, res.Stderr)
	}
	t.Setenv(cli.ShellPWD, ground)
	res := testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "none", "stack")
	if res.Err != nil || !strings.Contains(res.Stdout, "name   stack.silo.api") {
		t.Fatalf("ws run stack, a silo's recipe, from .ground: %v\n%s%s", res.Err, res.Stdout, res.Stderr)
	}
	f := cli.RouteFile(w.Root, 47001, "My Shop", t.TempDir())
	if f.Name != "my-shop" || f.Web != 47001 || f.Given != "my-shop" {
		t.Errorf("name %q given %q web %d", f.Name, f.Given, f.Web)
	}
	if f.Routes["web.feat.api"].Port == 0 || f.Routes["stack.silo.api"].Port == 0 {
		t.Errorf("routes %v", f.Routes)
	}
	if s := f.Silos["api"]; s.Target != "feat" || strings.Join(s.Recipes, ",") != "stack" {
		t.Errorf("silos %v", f.Silos)
	}
	_ = dir
}

// needsWorkspace is two repos: web, with a capsule (feat) whose web recipe
// needs api's server, and api, with its ground alone. You stand in web's
// capsule.
func needsWorkspace(t *testing.T) (*testutil.Workspace, string) {
	t.Helper()
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	t.Setenv(mission.EnvMission, "")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvRole, "")
	cli.NoRunsWatcher(t)
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "test-org", DefaultBranch: "main", Repos: []testutil.RepoOpts{{Name: "api"}, {Name: "web"}}})
	testutil.Run(t, w, nil, "lift", "web", "feat")
	dir := testutil.CapsuleDir(w, "web", "feat")
	os.WriteFile(filepath.Join(dir, "ws.repo.toml"), []byte(`
[run.web]
command = "sleep 300"

[run.web.needs.api]
run = "api/server"
env = "API_URL"
mount = "/api"

[run.docs]
command = "sleep 300"
`), 0o644)
	os.WriteFile(filepath.Join(w.Root, "repos", "api", ".ground", "ws.repo.toml"), []byte("[run.server]\ncommand = \"sleep 300\"\n"), 0o644)
	t.Setenv(cli.ShellPWD, dir)
	t.Cleanup(func() { testutil.RunCommand(t, w.Root, nil, "run", "stop", "--all") })
	return w, dir
}

func TestRun_ARecipeStartsWhatItNeeds(t *testing.T) {
	w, _ := needsWorkspace(t)
	res := testutil.RunCommand(t, w.Root, nil, "run")
	if res.Err != nil || !strings.Contains(res.Stdout, "NEEDS") || !strings.Contains(res.Stdout, "api (api/server)") {
		t.Fatalf("ws run alone: %v\n%s", res.Err, res.Stdout)
	}
	res = testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "capsule", "web")
	if res.Err != nil {
		t.Fatalf("ws run web: %v\n%s", res.Err, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "Started server.ground.api for api (api's ground: --with api=<capsule> runs another).") {
		t.Errorf("ws run web said:\n%s", res.Stderr)
	}
	if !strings.Contains(res.Stdout, "name   web.feat.web") || !strings.Contains(res.Stdout, "needs  api  server.ground.api  http://127.0.0.1:48") ||
		!strings.Contains(res.Stdout, "$API_URL  at /api\n") {
		t.Errorf("ws run web printed:\n%s", res.Stdout)
	}
	// The workspace has no names here, and a mount is the router's.
	if !strings.Contains(res.Stderr, "api answers at /api of web's own name only where the workspace has names") {
		t.Errorf("ws run web said nothing of the mount needing names:\n%s", res.Stderr)
	}
	list := lsJSON(t, w)
	if len(list) != 2 || list[0]["ref"] != "server.ground.api" || list[1]["ref"] != "web.feat.web" {
		t.Fatalf("ls: %v", list)
	}
	if by, _ := list[0]["needed_by"].([]any); len(by) != 1 || by[0] != "web.feat.web" || list[0]["ends"] != "when web.feat.web has ended" {
		t.Errorf("the server: needed by %v, ends %v", list[0]["needed_by"], list[0]["ends"])
	}
	// Its mount is the router's to serve, from the workspace's route file.
	f := cli.RouteFile(w.Root, 47001, "shop", t.TempDir())
	m := f.Routes["web.feat.web"].Mounts
	if len(m) != 1 || m[0].Path != "/api" || m[0].Need != "api" || m[0].Run != "server.ground.api" || m[0].Port != f.Routes["server.ground.api"].Port || m[0].Port == 0 {
		t.Errorf("mounts %+v", m)
	}

	// Asked for again, it's the same run.
	res = testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "none", "web")
	if res.Err != nil || !strings.Contains(res.Stderr, "web is running here already") || !strings.Contains(res.Stderr, "it's yours as well now") {
		t.Fatalf("ws run web again: %v\n%s", res.Err, res.Stderr)
	}
	if list := lsJSON(t, w); len(list) != 2 {
		t.Fatalf("ls after the second: %v", list)
	}
	// Stopping what something needs says so, and it's started again by
	// asking for what needs it.
	res = testutil.RunCommand(t, w.Root, nil, "run", "stop", "server.ground.api")
	if res.Err != nil || !strings.Contains(res.Stderr, "web.feat.web needed it") {
		t.Fatalf("stop the server: %v\n%s", res.Err, res.Stderr)
	}
	res = testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "web")
	if res.Err != nil || !strings.Contains(res.Stderr, "Started server.ground.api for api") {
		t.Fatalf("ws run web, its server gone: %v\n%s", res.Err, res.Stderr)
	}
	// Stopped, what was started for it goes too.
	res = testutil.RunCommand(t, w.Root, nil, "run", "stop", "web")
	if res.Err != nil || !strings.Contains(res.Stderr, "Stopped web.feat.web.") || !strings.Contains(res.Stderr, "Ended server.ground.api: nothing needs it now.") {
		t.Fatalf("stop web: %v\n%s", res.Err, res.Stderr)
	}
	if list := lsJSON(t, w); len(list) != 0 {
		t.Fatalf("ls after stop: %v", list)
	}
}

func TestRun_WithSaysWhatMeetsANeed(t *testing.T) {
	w, _ := needsWorkspace(t)
	res := testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--for", "none", "--with", "api=https://staging.example", "web")
	if res.Err != nil || !strings.Contains(res.Stdout, "needs  api  outside ws  https://staging.example  $API_URL") {
		t.Fatalf("with an address: %v\n%s%s", res.Err, res.Stdout, res.Stderr)
	}
	if list := lsJSON(t, w); len(list) != 1 {
		t.Fatalf("ls: %v", list)
	}
	f := cli.RouteFile(w.Root, 47001, "shop", t.TempDir())
	if m := f.Routes["web.feat.web"].Mounts; len(m) != 1 || m[0].URL != "https://staging.example" || m[0].Port != 0 {
		t.Errorf("mounts %+v", m)
	}
	for _, c := range []struct{ with, recipe, want string }{
		{"api", "docs", "--with is <need>=<capsule>, ground, silo or an address"},
		{"api=ground", "docs", "docs has no need called api: it needs nothing"},
	} {
		res := testutil.RunCommand(t, w.Root, nil, "run", "--wait", "0", "--with", c.with, c.recipe)
		if res.Err == nil || !strings.Contains(res.Err.Error(), c.want) {
			t.Errorf("--with %s %s: %v", c.with, c.recipe, res.Err)
		}
	}
}
