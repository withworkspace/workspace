package cli_test

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// Stopping or restarting the daemon, and upgrading ws, are for whoever
// runs the workspace: crew and the orchestrator are refused, and told
// what to do instead. (An agent that hits trouble with the daemon reaches
// for stopping it, which hangs every crew up.)
func TestDaemonAndUpgrade_NotForCrew(t *testing.T) {
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "acme", DefaultBranch: "main"})
	srv := &daemon.Server{Root: w.Root}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Shutdown("test over") })

	for _, as := range []struct{ env, value, who string }{
		{mission.EnvCrew, "c2", "c2"},
		{mission.EnvRole, mission.RoleOrchestrator, "orchestrator"},
	} {
		t.Setenv(as.env, as.value)
		for _, args := range [][]string{{"daemon", "stop"}, {"daemon", "restart", "--now"}, {"upgrade"}, {"upgrade", "--rollback"}} {
			r := testutil.RunCommand(t, w.Root, nil, args...)
			if r.Err == nil || !strings.Contains(r.Err.Error(), "is for whoever runs the workspace, not "+as.who) || !strings.Contains(r.Err.Error(), "ws mission ask") {
				t.Errorf("ws %s as %s: %v", strings.Join(args, " "), as.who, r.Err)
			}
		}
		t.Setenv(as.env, "")
	}
	select {
	case <-srv.Done():
		t.Fatal("the daemon was stopped")
	default:
	}
}

// ws daemon status says which ws the daemon is, and that it's another
// than this one when it is.
func TestDaemonStatus_SaysWhichWSItRuns(t *testing.T) {
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "acme", DefaultBranch: "main"})
	srv := &daemon.Server{Root: w.Root, Build: "v1.3.0", Exe: "/somewhere/else/workspace"}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Shutdown("test over") })

	out := run(t, w, nil, "daemon", "status").Stderr
	for _, want := range []string{"ws v1.3.0", "it runs the ws at /somewhere/else/workspace, not this one (test, at "} {
		if !strings.Contains(out, want) {
			t.Errorf("status doesn't say %q:\n%s", want, out)
		}
	}
}

// ws daemon restart on a daemon that can't restart itself (one from a ws
// before it could) says what restarting it takes, and leaves it up.
func TestDaemonRestart_ADaemonFromBeforeItCould(t *testing.T) {
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "acme", DefaultBranch: "main"})
	srv := &daemon.Server{Root: w.Root}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Shutdown("test over") })

	r := testutil.RunCommand(t, w.Root, nil, "daemon", "restart")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "ws daemon restart --now restarts it at once") {
		t.Errorf("ws daemon restart: %v", r.Err)
	}
	select {
	case <-srv.Done():
		t.Fatal("the daemon was stopped")
	default:
	}
}
