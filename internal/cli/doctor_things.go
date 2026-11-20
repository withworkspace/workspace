package cli

import (
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/workspace"
)

// checkThings is ws doctor's word on [things.<kind>]: the places ws.toml
// names for a plugin's kind of thing to go that ws can't put one in (no
// such repo, a repo that isn't a space of documents, a path that isn't a
// folder for each one), and a kind no plugin here has.
func checkThings(ctx *ops.Workspace) workspace.CheckCategory {
	cat := workspace.CheckCategory{Name: "Things"}
	for _, problem := range ctx.ThingPlaceProblems() {
		cat.Checks = append(cat.Checks, workspace.CheckResult{
			Name:    "[things]",
			Status:  workspace.CheckFail,
			Detail:  problem,
			FixHint: "paths = [\"<repo>:sketches/{slug}/\"] is where one of the kind's things goes: each a folder of a repo that's a space of documents, ending in /, with {slug} in its last part",
		})
	}
	return cat
}
