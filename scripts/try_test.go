package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// try.sh tears a try down once its owner has gone, and any `try.sh up` or
// `try.sh ls` sweeps every try on the machine. These are about who it
// takes to have gone: tries here are a folder and its .try/info, with no
// workspace or daemon behind them, which is all a sweep looks at.

// Whoever made the try: a terminal's shell, in its locale and time zone.
var maker = []string{"LANG=en_GB.UTF-8", "TZ=Europe/London"}

// Whoever sweeps it: crew, which the daemon starts with no LANG.
var sweeper = []string{"LC_ALL=C", "TZ=America/New_York"}

func try(t *testing.T, root string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"./try.sh"}, args...)...)
	for _, kv := range os.Environ() {
		switch k, _, _ := strings.Cut(kv, "="); {
		case k == "LANG", k == "TZ", k == "TRY_ROOT", strings.HasPrefix(k, "LC_"):
		default:
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(append(cmd.Env, "TRY_ROOT="+root), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("try.sh %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// A process to own a try, and the way to end it.
func owner(t *testing.T) (pid int, end func()) {
	t.Helper()
	cmd := exec.Command("sleep", "300")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	end = func() {
		cmd.Process.Kill()
		cmd.Wait()
	}
	t.Cleanup(end)
	return cmd.Process.Pid, end
}

// A try's folder, with what try.sh up keeps about it and these lines
// about its owner.
func makeTry(t *testing.T, root, name, ownerLines string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, ".try"), 0o755); err != nil {
		t.Fatal(err)
	}
	info := fmt.Sprintf("name=%s\nrepo=/nowhere\nbin=/nowhere/bin/workspace\ncreated=%d\nexpires=0\n%s",
		name, time.Now().Unix(), ownerLines)
	if err := os.WriteFile(filepath.Join(dir, ".try", "info"), []byte(info), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func there(dir string) bool {
	_, err := os.Stat(dir)
	return err == nil
}

func needsBash(t *testing.T) {
	t.Helper()
	for _, c := range []string{"bash", "ps"} {
		if _, err := exec.LookPath(c); err != nil {
			t.Skipf("no %s here", c)
		}
	}
}

func TestTrySweepLeavesATryWhoseOwnerIsThere(t *testing.T) {
	needsBash(t)
	root := t.TempDir()
	pid, end := owner(t)
	dir := makeTry(t, root, "theirs", try(t, root, maker, "_owner", fmt.Sprint(pid)))

	// Swept from another locale and time zone, where ps writes the
	// owner's start time differently.
	if out := try(t, root, sweeper, "ls"); !there(dir) {
		t.Fatalf("a try whose owner is running was torn down:\n%s", out)
	}

	end()
	out := try(t, root, sweeper, "ls")
	if there(dir) {
		t.Fatalf("a try whose owner has exited was left:\n%s", out)
	}
	if !strings.Contains(out, fmt.Sprintf("Tore down theirs: its owner (pid %d) has gone.", pid)) {
		t.Errorf("the sweep didn't say why:\n%s", out)
	}
}

func TestTrySweepTakesATryWhosePidIsSomeoneElses(t *testing.T) {
	needsBash(t)
	root := t.TempDir()
	pid, _ := owner(t)
	// The owner's pid, given out again: the process there now started at
	// another time.
	dir := makeTry(t, root, "reused", fmt.Sprintf("owner_pid=%d\nowner_began=Thu Jan 1 00:00:00 1970\n", pid))
	if out := try(t, root, sweeper, "ls"); there(dir) {
		t.Fatalf("a try whose owner's pid is another process's was left:\n%s", out)
	}
}

// A try made by a copy of try.sh from before, in another checkout: its
// owner's start time is in its maker's locale, so only the pid is gone by.
func TestTrySweepOfAnOlderTry(t *testing.T) {
	needsBash(t)
	root := t.TempDir()
	pid, end := owner(t)
	dir := makeTry(t, root, "older", fmt.Sprintf("owner=%d\nowner_started=Wed 18 Nov 07:30:02 2026\n", pid))

	if out := try(t, root, sweeper, "ls"); !there(dir) {
		t.Fatalf("an older try whose owner is running was torn down:\n%s", out)
	}
	end()
	if out := try(t, root, sweeper, "ls"); there(dir) {
		t.Fatalf("an older try whose owner has exited was left:\n%s", out)
	}
}
