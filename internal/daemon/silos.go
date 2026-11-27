package daemon

import (
	"encoding/json"
	"errors"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The daemon keeps the workspace's silos (workspace.SiloWatcher): each
// mirrors the capsule it's pointed at for as long as the daemon runs, with
// no terminal held open for it. ws silo status and watch ask it how they
// stand, and the web UI's Runs page shows them.

// keepSilos starts keeping the workspace's silos, and has the server
// answer for them. What it does is said in the daemon's log.
func (d *daemon) keepSilos(stop <-chan struct{}) {
	logger := d.s.Log
	sw, err := workspace.NewSiloWatcher(d.w.WS(), logger)
	if err != nil {
		logger.Printf("silos: not kept: %v", err)
		return
	}
	local, err := config.ReadLocal(d.cfg.Root)
	if err != nil {
		logger.Printf("silos: not kept: %v", err)
		return
	}
	d.silos = sw
	d.s.Silos = func() json.RawMessage {
		data, _ := json.Marshal(sw.States())
		return data
	}
	d.s.ResyncSilo = sw.Resync
	go sw.Watch(stop, local.Silo) //nolint:errcheck // it says what went wrong in the log
}

// siloStates is where each silo stands, for the web UI: none if the
// daemon isn't keeping them.
func (d *daemon) siloStates() []ops.SiloState {
	if d.silos == nil {
		return nil
	}
	return d.silos.States()
}

// resyncSilo gives a repo's silo its target's files in full.
func (d *daemon) resyncSilo(repo string) error {
	if d.silos == nil {
		return errors.New("Workspace is not keeping silos: see the log for why")
	}
	return d.silos.Resync(repo)
}
