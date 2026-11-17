package cli

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// showCapsuleMaking shows a capsule being made, step by step, and then its
// after_create hook's output: in a terminal, live; otherwise each step's
// result as it's done.
func showCapsuleMaking(steps []string, run func(string) error, hook string, dir func() string) (error, error) {
	var failed error
	op := func(name string) (bool, error) {
		err := run(name)
		if err != nil && failed == nil {
			failed = err
		}
		return false, err
	}
	if ui.IsInteractive() {
		m := newCapsuleCreateModel(newOperationModel(steps, nil, op, false, true), hook != "", dir, hook)
		final, err := tea.NewProgram(m, tea.WithOutput(os.Stderr)).Run()
		if err != nil {
			return nil, err
		}
		if fm, ok := final.(capsuleCreateModel); ok {
			return fm.HookErr(), failed
		}
		return nil, failed
	}
	fprintResults(ui.Stderr, runOperationSync(steps, nil, op, true))
	if failed != nil || hook == "" {
		return nil, failed
	}
	return workspace.RunHook(dir(), hook, os.Stderr, os.Stderr), nil
}

// madeCapsule says how making a capsule went, for ws lift and ws dock,
// and prints where it is for the shell wrapper to cd to.
func madeCapsule(ctx *ops.Workspace, c ops.Capsule, err error, successMsg string) error {
	if err != nil {
		return err
	}
	if c.HookErr != nil {
		fmt.Fprintf(ui.Stderr, "  %s after_create hook failed: %v\n", ui.Orange.Render("⚠"), c.HookErr)
	}
	if len(c.CopySkipped) > 0 {
		fmt.Fprintf(ui.Stderr, "  %s copy_from_ground: skipped missing files: %s\n", ui.Orange.Render("⚠"), strings.Join(c.CopySkipped, ", "))
	}
	if c.IDEErr != nil {
		fmt.Fprintf(ui.Stderr, "  %s workspace files: %v\n", ui.Orange.Render("⚠"), c.IDEErr)
	}
	fmt.Fprintf(ui.Stderr, "\n%s %s %s is ready for work.\n", successMsg, ctx.WS().FormatRepoName(c.Repo), ui.TagDim.Render(c.Name))
	switch {
	case c.AttachErr != nil:
		fmt.Fprintf(ui.Stderr, "  %s attaching to mission %s: %v\n", ui.Orange.Render("⚠"), c.Mission, c.AttachErr)
	case c.Mission != "":
		fmt.Fprintf(ui.Stderr, "  %s mission %s\n", ui.Dim.Render("→"), c.Mission)
	}
	fmt.Printf("cd %s\n", shellQuote(c.Path))
	return nil
}

// capsuleOptions are how ws lift and ws dock make a capsule: shown as it's
// made, and joined to $WS_MISSION's mission, if that's set and there.
func capsuleOptions(ctx *ops.Workspace) ops.CapsuleOptions {
	o := ops.CapsuleOptions{Show: showCapsuleMaking, By: ops.ActorFromEnv()}
	m, err := envMission(ctx)
	switch {
	case err != nil:
		fmt.Fprintf(ui.Stderr, "  %s %v; capsule not attached to a mission\n", ui.Orange.Render("⚠"), err)
	case m != nil:
		o.Mission = m.Slug
	}
	return o
}
