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
// closes, sharing what GitHub said with the web UI. It looks every
// ops.WatchPRsEvery, and as soon as a page's fetch brings news (a merge,
// say: PRCache.News), so ws never sits on what a page already shows.
func watchPRsLoop(root string, prs *ops.PRCache, stop <-chan struct{}) {
	t := time.NewTicker(ops.WatchPRsEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		case <-prs.News():
		}
		watchPRs(root, prs)
	}
}

// watchPRs looks at the missions' PRs once, and acts on what's changed.
func watchPRs(root string, prs *ops.PRCache) {
	ctx, err := LoadOps(root)
	if err != nil || ctx.GitHub() == nil {
		return
	}
	ctx.TidyDocked() // capsules ws docked for a question, whose PR is done
	missions, err := (mission.Store{Root: root}).List()
	if err != nil {
		return
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
