package cli

import (
	"time"

	"github.com/withworkspace/workspace/internal/runs"
)

// The daemon ends runs (internal/runs) whose owner has gone or whose time
// is up: as it starts, which takes what was due while it was down, then
// every runs.SweepEvery, and at once when it's poked (crew have ended). A
// run that ended by itself is removed an hour later, its log with it.
func sweepRunsLoop(root string, logf func(string, ...any), poke <-chan struct{}, stop <-chan struct{}) {
	sweepRuns(root, logf)
	t := time.NewTicker(runs.SweepEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			sweepRuns(root, logf)
		case <-poke:
			sweepRuns(root, logf)
		}
	}
}

// pokeRuns has the daemon's runs swept now, if a sweep isn't already due.
func pokeRuns(poke chan struct{}) {
	select {
	case poke <- struct{}{}:
	default:
	}
}

// sweepRuns sweeps once, saying in the daemon's log what it ended.
func sweepRuns(root string, logf func(string, ...any)) {
	swept, err := runs.Store{Root: root}.Sweep(newRunOwners(root), time.Now())
	for _, s := range swept {
		if s.Run.State == runs.Running {
			logf("runs: ended %s: %s", s.Run.Ref(), s.Why)
		}
	}
	if err != nil {
		logf("runs: couldn't end every one due: %v", err)
	}
}
