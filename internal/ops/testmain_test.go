package ops

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/testgit"
)

// keepEnv, set where a cached task runs, has this test binary keep its
// pass when the wrapper calls it, as ws run cache keep would.
const keepEnv = "WS_OPS_TEST_KEEP"

// Tests make commits: never ask the machine's signing agent for them.
func TestMain(m *testing.M) {
	if os.Getenv(keepEnv) != "" && len(os.Args) == 3 && os.Args[1] == "after" {
		// The wrapper running what a task is run after, as ws run cache
		// after does: the workspace loaded from where it runs.
		testHooks()
		cwd, _ := os.Getwd()
		cfg, ws, gh, err := Load(cwd)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		os.Exit(New(cfg, ws, gh, Options{}).RunAfters(os.Args[2], os.Stdout))
	}
	if os.Getenv(keepEnv) != "" && len(os.Args) == 2 {
		s, err := runs.StoreOf(os.Args[1])
		if err == nil {
			err = s.KeepPass(os.Args[1], time.Now())
		}
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if os.Getenv(keepEnv) != "" {
		fmt.Println("this test binary was run as ws, by", os.Args[1:])
		os.Exit(2)
	}
	// Run by crew (a step's check, say), the tests aren't crew: what a
	// test does isn't said to be by whoever ran go test.
	for _, k := range []string{mission.EnvMission, mission.EnvCrew, mission.EnvRole, mission.EnvRun} {
		os.Unsetenv(k)
	}
	cleanup := testgit.NoSigning()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// testHooks has the tasks a test starts keep their passes, and run what
// they're run after, through this test binary.
func testHooks() {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	keepWith = func() []string { return []string{exe} }
	afterWith = func() []string { return []string{exe, "after"} }
}
