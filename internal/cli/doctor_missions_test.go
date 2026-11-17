package cli

import (
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

func TestCheckMissions_AbsentWithoutMissions(t *testing.T) {
	w := &workspace.Workspace{Root: t.TempDir()}
	if got := checkMissions(w); len(got.Checks) != 0 {
		t.Errorf("checks = %+v, want none", got.Checks)
	}
}

func TestCheckMissions_ExcludeAndUnknownRepo(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	store := mission.Store{Root: root}
	m, err := store.Create("Doc")
	if err != nil {
		t.Fatal(err)
	}
	store.Update(m.Slug, func(m *mission.Mission) error { m.AddCapsule("gone", "x"); return nil })
	// Undo the exclude Create added, as if it had been lost.
	os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), nil, 0o644)

	w := &workspace.Workspace{Root: root, RepoNames: []string{"frontend"}}
	got := checkMissions(w)
	if len(got.Checks) != 2 {
		t.Fatalf("checks = %+v", got.Checks)
	}
	exclude, unknown := got.Checks[0], got.Checks[1]
	if exclude.Status != workspace.CheckWarn || exclude.Fix == nil {
		t.Errorf("exclude check = %+v", exclude)
	}
	if unknown.Status != workspace.CheckWarn || unknown.FixHint != "ws mission detach -m doc gone x" {
		t.Errorf("unknown repo check = %+v", unknown)
	}

	if err := exclude.Fix(); err != nil {
		t.Fatal(err)
	}
	if again := checkMissions(w); again.Checks[0].Status != workspace.CheckOK {
		t.Errorf("exclude still failing after fix: %+v", again.Checks[0])
	}
}

// With the Cursor CLI installed, doctor checks Cursor will run ws's crew
// hooks, and --fix adds the do-nothing hooks it needs.
func TestCheckMissions_CursorCrewHooks(t *testing.T) {
	t.Setenv(harness.EnvCursorBin, "/bin/sh") // stands in for an installed cursor-agent
	t.Setenv("HOME", t.TempDir())
	c, ok := cursorHooksCheck()
	if !ok || c.Status != workspace.CheckWarn || c.Fix == nil || !strings.Contains(c.Detail, "stop, postToolUse, postToolUseFailure, preToolUse missing") {
		t.Fatalf("check = %+v, %v", c, ok)
	}
	if err := c.Fix(); err != nil {
		t.Fatal(err)
	}
	if c, _ := cursorHooksCheck(); c.Status != workspace.CheckOK {
		t.Errorf("after the fix: %+v", c)
	}
	t.Setenv(harness.EnvCursorBin, "/nonexistent/cursor-agent")
	if _, ok := cursorHooksCheck(); ok {
		t.Error("checked Cursor hooks without the Cursor CLI")
	}
}
