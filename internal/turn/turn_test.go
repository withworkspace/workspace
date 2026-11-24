package turn

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
)

// A turn whose life ends is ended, and what it started with it: the
// agent's whole process group, not just the agent.
func TestRun_EndedWithItsLife(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child")
	life, end := context.WithCancel(context.Background())
	// The agent starts a child that would outlive it, and waits.
	script := `sleep 60 & echo $! > ` + pidFile + `; wait`
	done := make(chan error, 1)
	go func() {
		_, err := Run(Spec{Life: life, Launch: harness.Launch{Args: []string{"/bin/sh", "-c", script}}, Timeout: time.Minute, Root: dir, Who: "test"})
		done <- err
	}()
	var child int
	for end := time.Now().Add(5 * time.Second); child == 0 && time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if data, err := os.ReadFile(pidFile); err == nil && len(data) > 1 {
			child, _ = strconv.Atoi(string(data[:len(data)-1]))
		}
	}
	if child == 0 {
		t.Fatal("the agent never started its child")
	}
	end()
	select {
	case err := <-done:
		if !errors.Is(err, ErrEnded) {
			t.Errorf("err = %v, want ErrEnded", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the turn outlived its life")
	}
	for end := time.Now().Add(2 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if syscall.Kill(child, 0) != nil {
			break
		}
		if time.Now().After(end) {
			t.Fatalf("the agent's child %d outlived it", child)
		}
	}

	// One asked for once its life's over isn't started.
	if _, err := Run(Spec{Life: life, Launch: harness.Launch{Args: []string{"/bin/true"}}, Timeout: time.Minute, Root: dir, Who: "test"}); !errors.Is(err, ErrEnded) {
		t.Errorf("after its life: err = %v, want ErrEnded", err)
	}
}
