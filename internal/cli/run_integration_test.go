package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
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
