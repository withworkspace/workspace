package cli

import (
	"encoding/json"
	"errors"

	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The daemon keeps the workspace's silos (workspace.SiloWatcher): each
// mirrors the capsule it's pointed at for as long as the daemon runs, with
// no terminal held open for it. ws silo status and watch ask it how they
// stand, and the web UI's Runs page shows them.

// errOldDaemon is a daemon from a ws before it kept silos.
var errOldDaemon = errors.New("the daemon running is from a ws before it kept silos: `ws daemon restart` starts it on this one")

// askSilos is where each silo stands, from the workspace's daemon (started
// if it isn't running).
func askSilos(root string) ([]workspace.SiloState, error) {
	c, err := daemon.Connect(root)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return silosOf(c)
}

func silosOf(c *daemon.Client) ([]workspace.SiloState, error) {
	data, err := c.Silos()
	if err != nil {
		return nil, daemonSiloError(err)
	}
	var out []workspace.SiloState
	if len(data) > 0 {
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// daemonSiloError is err, or errOldDaemon for a daemon that didn't know
// what it was asked: one from before silos answer as a crew's name does.
func daemonSiloError(err error) error {
	if daemon.IsUnknownOp(err) || (err != nil && err.Error() == "no running crew ") {
		return errOldDaemon
	}
	return err
}
