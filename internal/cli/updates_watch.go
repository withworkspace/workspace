package cli

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/ops"
)

// The daemon looks for later releases once a day: ws's own, and each
// plugin's that was installed from a repo (ops.CheckUpdates). What it
// finds is kept for the web UI to show (Settings' About, Plugins), and
// nothing's installed. [updates] check = false in ws.local.toml turns it
// off.

// updatesFirst is how long after it starts a daemon first asks whether
// it's time to look, and updatesTick how often after that: a look that's
// due is made within the hour, and one that couldn't ask (no network) is
// tried again then.
var (
	updatesFirst = time.Minute
	updatesTick  = time.Hour
)

// daemonWS gives a daemon's operations the ws it runs: what a look for
// updates measures against, and what an upgrade from its web UI
// replaces, with the ws that's waiting in its place if one is.
func daemonWS(s *daemon.Server) func(*ops.Options) {
	return func(o *ops.Options) {
		o.WS = thisWS(s.Build)
		o.WS.Exe = s.Exe // as it was when the daemon started: an install since has moved it
		o.Pending, o.AtWork = s.Pending, s.AtWork
	}
}

// checkUpdatesLoop looks for updates when it's a day since a look last
// answered, until stop closes.
func checkUpdatesLoop(w *ops.Workspace, logf func(string, ...any), stop <-chan struct{}) {
	t := time.NewTimer(updatesFirst)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		if w.UpdatesDue() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			logf("updates: %s", updatesSaid(w.CheckUpdates(ctx)))
			cancel()
		}
		t.Reset(updatesTick)
	}
}

// updatesSaid is what a look found, in a line for the daemon's log.
func updatesSaid(up ops.Updates) string {
	said := "ws " + up.WS.Version + " is the latest release"
	switch {
	case up.WS.Error != "":
		said = "couldn't look for ws's latest release: " + up.WS.Error
	case up.WS.Newer:
		said = "ws " + up.WS.Latest + " is out (this is " + up.WS.Version + ")"
	case up.WS.Latest == "":
		said = "ws has no release to compare with"
	case up.WS.Latest != up.WS.Version:
		said = "the latest release of ws is " + up.WS.Latest + " (this is " + up.WS.Version + ")"
	}
	for _, name := range slices.Sorted(maps.Keys(up.Plugins)) {
		if p := up.Plugins[name]; p.Newer {
			said += "; " + name + " " + p.Latest + " is out"
		}
	}
	return said
}
