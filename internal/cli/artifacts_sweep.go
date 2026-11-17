package cli

import (
	"strings"
	"time"
)

// The daemon's sweep of the artifacts on this machine: each expired one
// is discarded, "expired" its reason, and each folder discarded 30 days
// ago is deleted for good (ops.ExpireArtifacts). It's done when the daemon
// starts, as it takes the day's copy of the records, and once a day while
// it runs. Those in a repo wait for the user's Clear expired: ws commits
// nothing to a shared branch on its own.

// sweepArtifactsEvery is how often a daemon that keeps running sweeps
// again.
const sweepArtifactsEvery = 24 * time.Hour

func sweepArtifactsLoop(root string, logf func(string, ...any), stop <-chan struct{}) {
	sweepArtifacts(root, logf)
	t := time.NewTicker(sweepArtifactsEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			sweepArtifacts(root, logf)
		}
	}
}

// sweepArtifacts sweeps once, saying in the daemon's log what it did.
func sweepArtifacts(root string, logf func(string, ...any)) {
	ctx, err := LoadOps(root)
	if err != nil {
		logf("artifacts: couldn't sweep: %v", err)
		return
	}
	swept, err := ctx.ExpireArtifacts()
	ids := func(n int, id func(int) string) string {
		var out []string
		for i := range n {
			out = append(out, id(i))
		}
		return strings.Join(out, ", ")
	}
	if n := len(swept.Discarded); n > 0 {
		logf("artifacts: discarded %d expired (%s)", n, ids(n, func(i int) string { return swept.Discarded[i].ID }))
	}
	if n := len(swept.Erased); n > 0 {
		logf("artifacts: deleted %d discarded 30 days ago for good (%s)", n, ids(n, func(i int) string { return swept.Erased[i].ID }))
	}
	if err != nil {
		logf("artifacts: couldn't sweep: %v", err)
	}
}
