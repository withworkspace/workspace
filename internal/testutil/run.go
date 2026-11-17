package testutil

import (
	"bytes"
	"os"
	"sync"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/ops"
)

// Result captures the output of a command execution.
type Result struct {
	Stdout string
	Stderr string
	Err    error
}

// RunCommand executes a ws CLI command against the given workspace root.
// Pass nil for gh to use a no-op stub.
func RunCommand(t *testing.T, root string, gh github.Client, args ...string) Result {
	t.Helper()

	if gh == nil {
		gh = &StubClient{}
	}

	// The fixture workspace, with gh as its GitHub: commands run in it.
	ws, err := cli.LoadOps(root, func(o *ops.Options) { o.GitHub = gh })
	if err != nil {
		t.Fatalf("LoadOps(%s) failed: %v", root, err)
	}
	cli.SetOverride(ws)
	defer cli.ClearOverride()

	// Capture os.Stdout and os.Stderr (commands write to them directly).
	// Read while the command runs: output bigger than a pipe's buffer would
	// otherwise block the command forever.
	oldStdout, oldStderr := os.Stdout, os.Stderr
	r, w, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = w, wErr
	var stdout, stderr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); stdout.ReadFrom(r) }()
	go func() { defer wg.Done(); stderr.ReadFrom(rErr) }()

	cmd := cli.NewRootCmd("test")
	cmd.SetArgs(args)

	runErr := cmd.Execute()

	w.Close()
	wErr.Close()
	wg.Wait()
	os.Stdout, os.Stderr = oldStdout, oldStderr

	return Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Err:    runErr,
	}
}
