package ops

import (
	"context"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/store"
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
		w.keepRunChanges(k.Changes())
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

// keepRunChanges keeps what's happened to the runs as events: a run
// started, a service up, a run ended and how. They're what this ws did,
// for you or for crew, and make no notification.
func (w *Workspace) keepRunChanges(changes []runs.Change) {
	if len(changes) == 0 {
		return
	}
	db, err := w.Records()
	if err != nil {
		return
	}
	logFailed(db.Write(context.Background(), func(tx *store.Tx) error {
		for _, c := range changes {
			if _, err := events.Append(tx, runEvent(c)); err != nil {
				return err
			}
		}
		return nil
	}))
}

// runEvent is a run's change as an event.
func runEvent(c runs.Change) events.Event {
	r := c.Run
	e := events.Event{At: c.At, Repo: r.Repo, Run: r.Ref(), RunN: r.N, Title: r.Name, Status: c.Status, Mission: r.Owner.Mission}
	switch c.Kind {
	case runs.Started:
		e.Kind = events.RunStarted
	case runs.Up:
		e.Kind = events.RunUp
	default:
		e.Kind = events.RunEnded
	}
	if len(r.Chain) > 0 {
		e.Chain = append([]string{}, r.Chain...)
	}
	switch {
	case strings.HasPrefix(r.By, "workflow:"):
		e.By = r.By // an automation's
	case r.By == "" || r.By == "you":
		e.By = events.ByYou
	default:
		e.By = events.ByCrew(r.By)
	}
	if r.Check != nil && e.Mission == "" {
		e.Mission = r.Check.Mission
	}
	if r.Recipe != "" {
		e.Text = r.Recipe
	}
	return e
}
