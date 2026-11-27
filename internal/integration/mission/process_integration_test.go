package mission_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/testutil"
)

const bugsProcess = `Bugs are reproduced first, then a regression test is added, then they're fixed,
and a pull request opened. Don't close a bug's mission on merge.
`

// ws process, and ws show process, show the process as it's written,
// where it is, and the guarantees ws holds beside it. There's nothing to
// read back or confirm: the planner and the orchestrator read it as it is.
func TestProcess_AsItsWritten(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	if out := testutil.Run(t, w, nil, "process").Stdout; !strings.Contains(out, "The workspace's process: none") || !strings.Contains(out, "PROCESS.md isn't there") {
		t.Errorf("with no process:\n%s", out)
	}
	os.WriteFile(filepath.Join(w.Root, "PROCESS.md"), []byte(bugsProcess), 0o644)
	for _, args := range [][]string{{"process"}, {"show", "-f", "llm", "process"}} {
		out := testutil.Run(t, w, nil, args...).Stdout
		for _, want := range []string{"The workspace's process: PROCESS.md", "not committed", "Don't close a bug's mission on merge."} {
			if !strings.Contains(out, want) {
				t.Errorf("ws %s lacks %q:\n%s", strings.Join(args, " "), want, out)
			}
		}
		if !strings.Contains(out, "missions.max_rounds: a Work step's checks come round unmet 3 times at most") || strings.Contains(out, "review_before") {
			t.Errorf("ws %s shows guarantees other than the one always held:\n%s", strings.Join(args, " "), out)
		}
	}
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[missions]\nreview_before_pr = true\n")...), 0o644)
	if out := testutil.Run(t, w, nil, "process").Stdout; !strings.Contains(out, "missions.review_before_pr: Workspace opens a pull request") || strings.Contains(out, "review_before_land") {
		t.Errorf("with review_before_pr on:\n%s", out)
	}
	for _, gone := range []string{"read-back", "confirm", "answer", "edit"} {
		if r := testutil.RunCommand(t, w.Root, nil, "process", gone); r.Err == nil {
			t.Errorf("ws process %s still runs", gone)
		}
	}
}
