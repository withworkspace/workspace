package cli

import (
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// The missions' PRs change on GitHub while nobody's looking: checks go red,
// a reviewer asks for changes, someone comments, one merges. The
// workspace's daemon watches them, and a change is noticed once, whenever
// it's next looked at (mission.SeePR), and acted on (ops.WatchPRs).

// watchPRsLoop watches the missions' PRs from the workspace's daemon until stop
// closes, sharing what GitHub said with the web UI.
func watchPRsLoop(root string, prs *ops.PRCache, stop <-chan struct{}) {
	t := time.NewTicker(ops.WatchPRsEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		ctx, err := LoadOps(root)
		if err != nil || ctx.GitHub() == nil {
			continue
		}
		ctx.TidyDocked() // capsules ws docked for a question, whose PR is done
		missions, err := (mission.Store{Root: root}).List()
		if err != nil {
			continue
		}
		var live []*mission.Mission
		for _, m := range missions {
			if ops.Watched(m) {
				live = append(live, m)
			}
		}
		idx := prs.Get(ctx.WS(), ctx.GitHub(), ops.MissionRepos(live))
		ctx.WatchPRs(ops.BuildMissionViewsFrom(ctx.WS(), idx, live))
	}
}
