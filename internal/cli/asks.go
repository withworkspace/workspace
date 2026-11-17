package cli

import (
	"context"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
)

// Asks nobody kept (conversation/ask.go) are swept here, in the daemon: it
// owns the records and is up whenever a page that could ask is open (a
// page's event stream holds it up). It sweeps as it starts, which takes
// what was due while it was down, and then every SweepEvery.

// sweepAsksLoop sweeps until stop closes.
func sweepAsksLoop(root string, logf func(string, ...any), stop <-chan struct{}) {
	sweepAsks(root, time.Now(), logf)
	t := time.NewTicker(conversation.SweepEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			sweepAsks(root, time.Now(), logf)
		}
	}
}

// sweepAsks sweeps once, saying in the daemon's log what it swept.
func sweepAsks(root string, now time.Time, logf func(string, ...any)) {
	swept, err := conversation.SweepAsks(context.Background(), root, now)
	switch {
	case err != nil:
		logf("asks: couldn't sweep: %v", err)
	case len(swept) > 0:
		logf("asks: swept %d nobody kept (%s)", len(swept), strings.Join(swept, ", "))
	}
}
