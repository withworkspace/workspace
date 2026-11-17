package cli

import (
	"slices"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// Crew that end a turn to wait on work they left running have that turn
// held (mission.Crew.Hold), and nobody else looks at it while the work
// runs. The workspace's daemon watches it: it records the work ending, and
// hands the turn to the orchestrator once it's no longer held: the work
// ended and nothing woke the crew, or it has run too long.

// watchHeldEvery is how often the daemon looks at held turns: often enough
// for the minute crew have to be woken (mission.WakeGrace).
const watchHeldEvery = 15 * time.Second

// workSeen is how the daemon sees work end: alive says whether a process
// is, and callDone whether a tool call in the session of crew over ACP the
// daemon holds has finished (nil: it holds none).
type workSeen struct {
	alive    func(pid int) bool
	callDone func(slug, crewID, call string) bool
}

// ended is whether a piece of crew's work has ended: its tool call
// finished, or its process gone. Work with neither isn't seen to end.
func (s workSeen) ended(slug, crewID string, w mission.Work) bool {
	return w.Call != "" && s.callDone != nil && s.callDone(slug, crewID, w.Call) ||
		w.PID > 0 && !s.alive(w.PID)
}

// watchHeld looks at the held turns of crew in play, at now. Work seen to
// end (seen.ended) is recorded as ended. A turn no longer held is one the
// orchestrator takes (mission.OrchestratorTakes): notify hands it on, and
// it's taken once.
func watchHeld(store mission.Store, now time.Time, seen workSeen, notify func(slug string, e mission.OrchestratorEvent)) {
	missions, err := store.InPlay()
	if err != nil {
		return
	}
	for _, m := range missions {
		if !ops.Watched(m) {
			continue
		}
		for _, c := range m.Crew {
			if c.Pending == nil || c.Ended != nil {
				continue
			}
			var gone []mission.Work
			for _, w := range c.Pending.Running() {
				if seen.ended(m.Slug, c.ID, w) {
					gone = append(gone, w)
				}
			}
			if len(gone) > 0 {
				ended, err := store.EndWork(m.Slug, c.ID, c.Pending.At, now, func(w mission.Work) bool { return slices.Contains(gone, w) })
				if err != nil || !ended {
					continue
				}
				if m, err = store.Load(m.Slug); err != nil {
					break
				}
			}
			e := mission.OrchestratorEvent{Crew: c.ID, Trigger: "quiet", At: now}
			if takes, _ := mission.OrchestratorTakes(m, e); takes {
				notify(m.Slug, e)
			}
		}
	}
}

// watchHeldLoop watches held turns from the workspace's daemon until stop
// closes. callDone is the daemon's: whether a tool call of crew over ACP
// it holds has finished.
func watchHeldLoop(root string, callDone func(slug, crewID, call string) bool, stop <-chan struct{}) {
	t := time.NewTicker(watchHeldEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		watchHeldAt(root, time.Now(), crew.ProcessAlive, callDone)
	}
}

// watchHeldAt looks at held turns once, as the daemon does each tick.
func watchHeldAt(root string, now time.Time, alive func(pid int) bool, callDone func(slug, crewID, call string) bool) {
	ctx, err := LoadOps(root)
	if err != nil {
		return
	}
	watchHeld(mission.Store{Root: root}, now, workSeen{alive: alive, callDone: callDone}, func(slug string, e mission.OrchestratorEvent) {
		if ctx.Config().OrchestratorEnabled() { // off, a turn no longer held is yours as it is
			ctx.Notify(slug, e)
		}
	})
}
