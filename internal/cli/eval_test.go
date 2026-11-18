package cli

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/crew"
)

// ws eval is off unless WS_EVAL is set: not listed, and refused before
// anything's read or run.
func TestEval_OffUnlessTurnedOn(t *testing.T) {
	t.Setenv(crew.EnvEval, "")
	cmd := newEvalCmd()
	if !cmd.Hidden {
		t.Error("it's listed with WS_EVAL unset")
	}
	cmd.SetArgs([]string{"judge", t.TempDir()})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "WS_EVAL=1 ws eval judge") {
		t.Errorf("err = %v", err)
	}
	t.Setenv(crew.EnvEval, "1")
	if newEvalCmd().Hidden {
		t.Error("it's hidden with WS_EVAL set")
	}
}
