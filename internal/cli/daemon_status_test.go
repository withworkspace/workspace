package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/testutil"
)

// ws daemon status says what's true of the daemon it reaches: one with no
// idle timeout (the default) stays open until it's closed, and one with
// a timeout set exits after it.
func TestDaemonStatus_SaysWhenTheDaemonExits(t *testing.T) {
	for _, tc := range []struct {
		idle      time.Duration
		want, not string
	}{
		{0, "it stays open until it's closed (ws daemon stop)", "exits after"},
		{5 * time.Minute, "it exits after 5 minutes idle", "stays open"},
	} {
		w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "acme", DefaultBranch: "main"})
		srv := &daemon.Server{Root: w.Root, IdleTimeout: tc.idle}
		if err := srv.Listen(); err != nil {
			t.Fatal(err)
		}
		go srv.Serve()
		t.Cleanup(func() { srv.Shutdown("test over") })

		out := run(t, w, nil, "daemon", "status").Stderr
		if !strings.Contains(out, tc.want) || strings.Contains(out, tc.not) {
			t.Errorf("IdleTimeout %v: status says\n%s\nwant %q", tc.idle, out, tc.want)
		}
		if !strings.Contains(out, "no crew") {
			t.Errorf("IdleTimeout %v: status doesn't say it has no crew:\n%s", tc.idle, out)
		}
	}
}
