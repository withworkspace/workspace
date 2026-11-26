package ops

import (
	"time"

	"github.com/withworkspace/workspace/internal/runs"
)

// The daemon keeps where every run stands (runs.Keeper): the pages'
// stamps of runs (RunsStamp, AppStamp, MissionStamp's) read it, and
// glob, stat and dial nothing.
//
// What moves it is what's written under .ws/runs (a run's record, its
// exit, a folder claimed or cleared away), which is watched, and its own
// asks of what no file says (a process gone, a port answering), as the
// keeper's Probe says they're due. Every runsLookEvery the folder's read
// again regardless, for what a watch missed.

// runsLookEvery is how often every run's files are read again, for a
// move the watch didn't see.
var runsLookEvery = 15 * time.Second

// WatchRuns keeps where every run stands until stop closes, telling moved
// (without waiting on it) when it's moved.
func (w *Workspace) WatchRuns(stop <-chan struct{}, logf func(string, ...any), moved func()) {
	k := &w.runsKept
	k.Store = w.runStore()
	k.Look(time.Now())
	written := make(chan struct{}, 1)
	go watchFolders(stop, func(format string, args ...any) { logf("runs: "+format, args...) },
		k.Folders, k.Heeds, func([]string) {
			select {
			case written <- struct{}{}:
			default: // one waiting stands for this
			}
		}, nil)
	_, next := k.Probe(time.Now())
	probe := time.NewTimer(next)
	defer probe.Stop()
	again := time.NewTicker(runsLookEvery)
	defer again.Stop()
	for {
		var changed bool
		select {
		case <-stop:
			return
		case <-written:
			changed = k.Look(time.Now())
		case <-again.C:
			changed = k.Look(time.Now())
		case <-probe.C:
			if k.Pending() {
				changed = k.Look(time.Now())
			}
			var probed bool
			probed, next = k.Probe(time.Now())
			changed = changed || probed
			probe.Reset(next)
		}
		if changed && moved != nil {
			moved()
		}
	}
}

// runsStampIn is where the runs in repo's worktree stand
// (runs.Store.StampIn): as the daemon keeps it, or read from their files.
func (w *Workspace) runsStampIn(repo, worktree string) string {
	if s, ok := w.runsKept.StampIn(repo, worktree); ok {
		return s
	}
	return w.runStore().StampIn(repo, worktree)
}

// runningKept is runs.Store.Running, as the daemon keeps it.
func (w *Workspace) runningKept(repo, worktree, recipe string) (runs.Run, bool) {
	if r, ok, kept := w.runsKept.Running(repo, worktree, recipe); kept {
		return r, ok
	}
	return w.runStore().Running(repo, worktree, recipe)
}

// startingKept is runs.Store.Starting, as the daemon keeps it.
func (w *Workspace) startingKept(repo, worktree, recipe string) (string, bool) {
	if out, ok, kept := w.runsKept.Starting(repo, worktree, recipe); kept {
		return out, ok
	}
	return w.runStore().Starting(repo, worktree, recipe)
}
