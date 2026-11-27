package ops

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/testgit"
)

// keepEnv, set where a cached task runs, has this test binary keep its
// pass when the wrapper calls it, as ws run cache keep would.
const keepEnv = "WS_OPS_TEST_KEEP"

// Tests make commits: never ask the machine's signing agent for them.
func TestMain(m *testing.M) {
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
	cleanup := testgit.NoSigning()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
