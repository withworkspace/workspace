//go:build darwin || linux

package harness

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// envProcessHelper makes this test binary a stand-in for detached work:
// it sleeps for the duration it's set to, then exits.
const envProcessHelper = "WS_TEST_PROCESS_HELPER"

func TestProcessHelper(t *testing.T) {
	d, err := time.ParseDuration(os.Getenv(envProcessHelper))
	if err != nil {
		t.Skip("run by TestReadProcs_FindsCrewWork")
	}
	time.Sleep(d)
}

// The process check on this machine's processes, with this test as the
// harness. A shell it ran detaches work and exits, as an agent's nohup …
// & does: the check finds the orphan by the crew's names it carries. (The
// orphan is this test binary, sleeping: macOS hides the environment of its
// own binaries, so a detached /bin/sleep would be missed, as the doc
// expects.) A command still running in a shell it ran is found too.
func TestReadProcs_FindsCrewWork(t *testing.T) {
	if os.Getenv(envProcessHelper) != "" {
		t.Skip("the helper")
	}
	crew := ProcCrew{Mission: "proc-check", Crew: "c9", Harness: os.Getpid(), Hook: os.Getpid()}
	env := append(os.Environ(), "WS_MISSION="+crew.Mission, "WS_CREW="+crew.Crew, envProcessHelper+"=20s")

	detach := exec.Command("/bin/sh", "-c", `nohup "$0" -test.run='^TestProcessHelper$' >/dev/null 2>&1 &`, os.Args[0])
	detach.Env = env
	if err := detach.Run(); err != nil {
		t.Fatal(err)
	}
	shell := exec.Command("/bin/sh", "-c", `sleep 20; true`) // two commands: the shell waits
	shell.Env = env
	if err := shell.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { shell.Process.Kill(); shell.Wait() })

	var found *Found
	var orphan, sh Running
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		procs, err := ReadProcs()
		if err != nil {
			t.Fatal(err)
		}
		if found = procs.CrewWork(crew); found == nil {
			t.Fatal("no check made: this test isn't in the table")
		}
		orphan, sh = Running{}, Running{}
		for _, r := range found.Running {
			switch {
			case !r.Shell && strings.Contains(r.What, "TestProcessHelper"):
				orphan = r
			case r.Shell && r.PID == shell.Process.Pid:
				sh = r
			}
		}
		if orphan.PID != 0 && sh.PID != 0 && sh.What == "sleep 20" {
			break
		}
	}
	if orphan.PID != 0 {
		syscall.Kill(orphan.PID, syscall.SIGKILL)
	}
	if orphan.PID == 0 || sh.PID == 0 || sh.What != "sleep 20" {
		t.Fatalf("found %+v: want the detached helper, and the shell running sleep 20 (pid %d)", found.Running, shell.Process.Pid)
	}
	if len(found.Running) != 2 {
		t.Errorf("found more than the crew's work: %+v", found.Running)
	}
}
