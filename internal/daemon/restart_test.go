package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/store/storetest"
)

// installWS writes a ws that says its version at path, as an install does:
// beside it, then renamed over it.
func installWS(t *testing.T, path, version string) {
	t.Helper()
	script := fmt.Sprintf("#!/bin/sh\necho workspace version %s\n", version)
	if version == "" {
		script = "#!/bin/sh\nexit 1\n" // one that doesn't run
	}
	if err := os.WriteFile(path+".new", []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".new", path); err != nil {
		t.Fatal(err)
	}
}

// atWork is what a test's server is told is at work.
type atWork struct {
	mu   sync.Mutex
	crew []string
}

func (a *atWork) set(crew ...string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.crew = crew
}

func (a *atWork) get() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.crew
}

// restartingServer is a server that restarts itself, on a ws at v1, looking
// at its binary every few milliseconds.
func restartingServer(t *testing.T) (s *Server, root, exe string, work *atWork) {
	t.Helper()
	was := exeTick
	exeTick = 20 * time.Millisecond
	t.Cleanup(func() { exeTick = was })
	root = t.TempDir()
	storetest.Own(t, root)
	exe = filepath.Join(t.TempDir(), "workspace")
	installWS(t, exe, "v1")
	work = &atWork{}
	s = &Server{Root: root, Build: "v1", Exe: exe, CrewAtWork: work.get}
	if err := s.Listen(); err != nil {
		t.Fatal(err)
	}
	go s.Serve()
	t.Cleanup(func() { s.Shutdown("test over") })
	return s, root, exe, work
}

func waitDone(t *testing.T, s *Server, what string) {
	t.Helper()
	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatalf("the server never shut down: %s", what)
	}
}

func stillUp(t *testing.T, s *Server, what string) {
	t.Helper()
	select {
	case <-s.Done():
		t.Fatalf("the server shut down: %s", what)
	case <-time.After(300 * time.Millisecond):
	}
}

// A daemon says which ws it is, so a command can say when it's another
// than the one installed.
func TestRestart_HelloSaysItsBuild(t *testing.T) {
	_, root, _, _ := restartingServer(t)
	c := dialT(t, root)
	if c.Build() != "v1" {
		t.Errorf("Build() = %q, want v1", c.Build())
	}
	if next, wanted := c.RestartWanted(); wanted || next != "" {
		t.Errorf("a daemon on the ws at its path wants a restart (onto %q)", next)
	}
}

// Another ws installed over the daemon's is found, and the daemon shuts
// down to be started again on it: nothing was at work.
func TestRestart_OntoAWSInstalledInItsPlace(t *testing.T) {
	s, _, exe, _ := restartingServer(t)
	stillUp(t, s, "its binary hadn't changed")
	installWS(t, exe, "v2")
	waitDone(t, s, "its binary was replaced")
	if !s.Restarting() {
		t.Error("it shut down, but not to be started again")
	}
}

// With crew mid-turn it waits, saying which ws it found and what it's
// waiting for, and restarts once they're done.
func TestRestart_WaitsForWhatsAtWork(t *testing.T) {
	s, root, exe, work := restartingServer(t)
	work.set("c1 on fix-login, mid-turn")
	installWS(t, exe, "v2")

	// It finds the ws in its place, in its own time.
	var c *Client
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		c = dialT(t, root)
		if next, wanted := c.RestartWanted(); wanted {
			if next != "v2" {
				t.Errorf("it's to restart onto %q, want v2", next)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("it never found the ws installed in its place")
		}
	}
	stillUp(t, s, "crew were mid-turn")
	if waits, err := c.AtWork(); err != nil || len(waits) != 1 || waits[0] != "c1 on fix-login, mid-turn" {
		t.Errorf("AtWork() = %q, %v", waits, err)
	}

	work.set()
	waitDone(t, s, "its crew finished their turn")
	if !s.Restarting() {
		t.Error("it shut down, but not to be started again")
	}
}

// A ws that doesn't run isn't restarted onto: the daemon stays on the one
// that does, and takes the next that's installed.
func TestRestart_NotOntoAWSThatDoesntRun(t *testing.T) {
	s, root, exe, _ := restartingServer(t)
	installWS(t, exe, "")
	stillUp(t, s, "the ws in its place doesn't run")
	if _, wanted := dialT(t, root).RestartWanted(); wanted {
		t.Error("it wants to restart onto a ws that doesn't run")
	}
	installWS(t, exe, "v3")
	waitDone(t, s, "a ws that runs was installed")
}

// Asked to restart, it waits for what's at work and says what; asked to
// now, it doesn't.
func TestRestart_Asked(t *testing.T) {
	s, root, _, work := restartingServer(t)
	work.set("c1 on fix-login, mid-turn")
	c := dialT(t, root)
	waits, err := c.Restart(false)
	if err != nil || len(waits) != 1 {
		t.Fatalf("Restart(false) = %q, %v: want what it waits for", waits, err)
	}
	stillUp(t, s, "crew were mid-turn")
	if waits, err = c.Restart(true); err != nil || len(waits) != 0 {
		t.Fatalf("Restart(true) = %q, %v", waits, err)
	}
	waitDone(t, s, "it was asked to restart now")
	if !s.Restarting() {
		t.Error("it shut down, but not to be started again")
	}
}

// A server that wasn't started as one that restarts itself answers as one
// from a ws before it could: whoever asked stops it and starts another.
// Shut down any other way, it isn't to be started again.
func TestRestart_AServerThatDoesntRestartItself(t *testing.T) {
	s, root := startServer(t, 0)
	c := dialT(t, root)
	if _, err := c.Restart(true); !IsUnknownOp(err) {
		t.Errorf("Restart on a server with no binary of its own: %v, want an unknown op", err)
	}
	if c.Build() != "" {
		t.Errorf("Build() = %q from a server that wasn't given one", c.Build())
	}
	s.Shutdown("closed")
	if s.Restarting() {
		t.Error("a server that was closed says it's to be started again")
	}
}
