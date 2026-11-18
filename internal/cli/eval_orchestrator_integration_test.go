package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/testutil"
)

// ownMade has the test own the records of each workspace an eval's run is
// made in, as its daemon would: a test binary starts none.
func ownMade(t *testing.T) {
	t.Helper()
	was := *cli.EvalMade
	*cli.EvalMade = func(dir string) { storetest.Own(t, dir) }
	t.Cleanup(func() { *cli.EvalMade = was })
}

// copyWorkspace is a before hook that makes a run's workspace by copying
// the one at $SRC, less what's its daemon's (a socket can't be copied).
const copyWorkspace = `mkdir "$WS_EVAL_TMP/w" && (cd "$SRC" && tar cf - --exclude '*.sock' .) | (cd "$WS_EVAL_TMP/w" && tar xf -)`

// orchestratorFixtureAt writes an orchestrator fixture for the mission
// cache, with seed as its seed.
func orchestratorFixtureAt(t *testing.T, seed string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cache-done")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "fixture.toml"), []byte("mission = \"cache\"\ntrigger = \"done\"\ncrew = \"c1\"\nexpect = [\"note\"]\nnever = [\"accept\"]\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "seed.sh"), []byte(seed), 0o644)
	return dir
}

func evalResult(t *testing.T, r testutil.Result) ops.EvalResult {
	t.Helper()
	var res ops.EvalResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(r.Stdout)), &res); err != nil {
		t.Fatalf("not a result: %v\nstdout: %s\nstderr: %s", err, r.Stdout, r.Stderr)
	}
	return res
}

// A run is made in a workspace the before hook makes, in the folder it's
// given: the workspace's own orchestrator is off while the seed runs, the
// seed has the workspace and its ws, the orchestrator is run on the pick
// with what it dispatches queued, what it ran is scored, and the after
// hook is told what the before hook said, before the folder goes.
func TestEvalOrchestrator_RunsInAWorkspaceMadeForIt(t *testing.T) {
	w, _ := orchestrateFixture(t)
	ownMade(t)
	t.Setenv(crew.EnvEval, "1")
	mark := filepath.Join(t.TempDir(), "after")
	t.Setenv("SRC", w.Root)
	t.Setenv("MARK", mark)
	fixture := orchestratorFixtureAt(t, `set -e
grep -q 'enabled = false' ws.local.toml
! grep -q 'dispatch' ws.local.toml
test "$TRY_DIR" = "$PWD" && test "$WS_EVAL_DIR" = "$PWD" && test -x "$TRY_WS" && test "$WS_EVAL_NAME" = copy
test -z "$WS_EVAL"
echo seeded > seeded
`)
	r := testutil.RunCommand(t, w.Root, nil, "eval", "orchestrator", "--json", "--pick", "echo-orchestrator:tiny",
		"--before", copyWorkspace+` && echo "name copy" && echo "dir $WS_EVAL_TMP/w"`,
		"--after", `echo "$WS_EVAL_NAME $WS_EVAL_DIR $WS_EVAL_TMP" > "$MARK" && cp "$WS_EVAL_DIR/ws.local.toml" "$MARK.toml" && cp "$WS_EVAL_DIR/seeded" "$MARK.seeded"`,
		fixture)
	res := evalResult(t, r)
	if r.Err == nil {
		t.Error("it exited 0 with a check wrong")
	}
	// The stand-in orchestrator says a line and runs nothing: its note
	// is missing, and it never tried to accept.
	if res.Error != "" || res.Fixture != "cache-done" || res.Pick != "echo-orchestrator:tiny" || res.Right != 1 || res.Scored != 2 || !strings.Contains(res.Output, "ran as orchestrator on cache with tiny") {
		t.Fatalf("result = %+v", res)
	}
	if c := res.Checks[0]; c.Check != "ws mission note" || c.Expect != "runs" || c.Got != "didn't" || c.Score != ops.ScoreWrong {
		t.Errorf("check 1 = %+v", c)
	}
	if c := res.Checks[1]; c.Check != "ws mission accept" || c.Expect != "never runs" || c.Score != ops.ScoreRight {
		t.Errorf("check 2 = %+v", c)
	}
	said, _ := os.ReadFile(mark)
	fields := strings.Fields(string(said))
	if len(fields) != 3 || fields[0] != "copy" || filepath.Dir(fields[1]) != fields[2] {
		t.Fatalf("the after hook was told %q", said)
	}
	if _, err := os.Stat(fields[2]); err == nil {
		t.Errorf("the folder the workspace was made in was left: %s", fields[2])
	}
	local, _ := os.ReadFile(mark + ".toml")
	for _, want := range []string{`dispatch = "ask"`, "enabled = false", `harness = "echo-orchestrator"`, `model = "tiny"`} {
		if !strings.Contains(string(local), want) {
			t.Errorf("the run's workspace wasn't configured with %s:\n%s", want, local)
		}
	}
	if seeded, _ := os.ReadFile(mark + ".seeded"); string(seeded) != "seeded\n" {
		t.Errorf("the seed didn't run in the workspace: %q", seeded)
	}
	// The workspace it was run from is as it was.
	if local, _ := os.ReadFile(filepath.Join(w.Root, "ws.local.toml")); strings.Contains(string(local), "ask") {
		t.Errorf("the workspace it was run from was configured:\n%s", local)
	}
}

// What can't be run is said, and nothing's run in a workspace that wasn't
// made for it: no hook, a hook that fails or doesn't say where, a
// workspace that was there already, a seed that fails. The after hook is
// run once the before hook has been.
func TestEvalOrchestrator_Refuses(t *testing.T) {
	w, _ := orchestrateFixture(t)
	ownMade(t)
	fixture := orchestratorFixtureAt(t, "exit 4\n")
	if r := testutil.RunCommand(t, w.Root, nil, "eval", "orchestrator", "--before", "true", fixture); r.Err == nil || !strings.Contains(r.Err.Error(), "WS_EVAL=1 ws eval orchestrator") {
		t.Errorf("with WS_EVAL unset: %v", r.Err)
	}
	t.Setenv(crew.EnvEval, "1")
	if r := testutil.RunCommand(t, w.Root, nil, "eval", "orchestrator", fixture); r.Err == nil || !strings.Contains(r.Err.Error(), "--before is what makes the workspace") {
		t.Errorf("with no --before: %v", r.Err)
	}
	mark := filepath.Join(t.TempDir(), "after")
	t.Setenv("SRC", w.Root)
	t.Setenv("MARK", mark)
	after := `echo ran >> "$MARK"`
	for name, c := range map[string]struct {
		before, want string
		after        bool
	}{
		"a hook that fails":    {`echo "no room" >&2; exit 3`, "--before failed: exit status 3: no room", false},
		"no dir":               {`echo "name x"`, "didn't say where the workspace is", true},
		"a workspace as it is": {`echo "dir $SRC"`, "isn't in the folder it was given to make it in", true},
		"a seed that fails":    {copyWorkspace + ` && echo "dir $WS_EVAL_TMP/w"`, "its seed failed: exit status 4", true},
	} {
		os.Remove(mark)
		r := testutil.RunCommand(t, w.Root, nil, "eval", "orchestrator", "--json", "--before", c.before, "--after", after, fixture)
		res := evalResult(t, r)
		if r.Err == nil || !strings.Contains(res.Error, c.want) || len(res.Checks) != 0 {
			t.Errorf("%s: err %v, result %+v", name, r.Err, res)
		}
		if _, err := os.Stat(mark); (err == nil) != c.after {
			t.Errorf("%s: the after hook ran: %v", name, err == nil)
		}
	}
	// Nothing was run in the workspace that was there already.
	if runs, _ := filepath.Glob(filepath.Join(w.Root, ".ws", "missions", "cache", "orchestrator", "*.json")); len(runs) != 0 {
		t.Errorf("the orchestrator ran in the workspace it was run from: %v", runs)
	}
}
