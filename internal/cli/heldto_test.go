package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// What crew are told as their decision's recorded: what it's held to, and
// what to do if it conflicts, by its id.
func TestPrintHeldTo(t *testing.T) {
	var buf bytes.Buffer
	old := ui.Stderr
	ui.Stderr = &buf
	t.Cleanup(func() { ui.Stderr = old })
	printHeldTo("d3", ops.HeldTo{Step: 1, Checks: []string{"the lookup is cached"},
		Later: []ops.LaterChecks{{Step: 3, Checks: []string{"nothing new reads git on a request path"}}}, Yours: []string{"d1: Key the cache per tenant"}})
	out := buf.String()
	for _, want := range []string{"Step 1's checks (yours):\n    1. the lookup is cached", "Step 3's checks (later, which this decision mustn't rule out):\n    1. nothing new reads git on a request path",
		"The user's decisions (they stand over yours):\n    d1: Key the cache per tenant", "Please check that d3 doesn't conflict with any of the above", `ws mission decide --withdraw d3 "why"`, "--supersedes d3"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}
