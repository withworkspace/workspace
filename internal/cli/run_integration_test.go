package cli_test

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
	run(t, w, nil, "lift", "api", "feat")
	dir := capsuleDir(w, "api", "feat")
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
	t.Setenv(cli.ShellPWD, filepath.Join(w.Root))
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
