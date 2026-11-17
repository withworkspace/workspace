package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

// checkMissions only runs once missions exist, so workspaces that never use
// them see no change in doctor output.
func checkMissions(w *workspace.Workspace) workspace.CheckCategory {
	store := mission.Store{Root: w.Root}
	if _, err := os.Stat(store.Dir()); err != nil {
		return workspace.CheckCategory{Name: "Missions"}
	}

	excluded := workspace.CheckResult{Name: ".ws/ ignored by config repo", Status: workspace.CheckOK}
	if !mission.IsExcluded(w.Root) {
		excluded.Status = workspace.CheckWarn
		excluded.Detail = "mission files would show up in the config repo's git status"
		root := w.Root
		excluded.Fix = func() error {
			mission.EnsureExcluded(root)
			if !mission.IsExcluded(root) {
				return fmt.Errorf("could not add .ws/ to info/exclude")
			}
			return nil
		}
	}
	checks := []workspace.CheckResult{excluded}
	if c, ok := cursorHooksCheck(); ok {
		checks = append(checks, c)
	}

	missions, _ := store.InPlay()
	for _, m := range missions {
		for _, c := range m.Capsules {
			if !slices.Contains(w.RepoNames, c.Repo) {
				checks = append(checks, workspace.CheckResult{
					Name:    fmt.Sprintf("%s: %s/%s", m.Slug, c.Repo, c.Name),
					Status:  workspace.CheckWarn,
					Detail:  "repo is not in ws.toml",
					FixHint: fmt.Sprintf("ws mission detach -m %s %s %s", m.Slug, c.Repo, c.Name),
				})
			}
		}
	}
	return workspace.CheckCategory{Name: "Missions", Checks: checks}
}

// cursorHooksCheck checks Cursor will run ws's per-crew hooks, which tell
// ws when Cursor crew are working or waiting. Only with the Cursor CLI
// installed.
func cursorHooksCheck() (workspace.CheckResult, bool) {
	h, err := harness.Get("cursor")
	if err != nil || h.Available() != nil {
		return workspace.CheckResult{}, false
	}
	c := workspace.CheckResult{Name: "Cursor crew hooks", Status: workspace.CheckOK}
	missing, err := harness.CursorHooksMissing()
	switch {
	case err != nil:
		c.Status, c.Detail = workspace.CheckWarn, err.Error()
	case len(missing) > 0:
		c.Status = workspace.CheckWarn
		c.Detail = "Cursor runs ws's hooks for its crew only for events ~/.cursor/hooks.json has a hook for too (" + strings.Join(missing, ", ") + " missing), so ws guesses from the screen whether Cursor crew are working, and doesn't see the questions they ask with their question tool; the fix adds hooks there that do nothing (/usr/bin/true), the question tool's matched to it alone"
		c.Fix = harness.EnableCursorHooks
	}
	return c, true
}
