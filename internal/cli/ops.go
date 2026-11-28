package cli

import (
	"context"
	"fmt"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store"
)

func init() {
	// A command that reads or writes a record borrows a connection to the
	// daemon's: when there's no daemon, it's started, as for crew.
	store.Start = func(root string) error {
		c, err := daemon.Connect(root)
		if err != nil {
			return err
		}
		return c.Close()
	}
	// ws.local.toml is written by commands and the daemon both: each
	// write takes a turn of the records' writer, so the daemon orders
	// them as it does every write.
	config.Turn = func(root string, fn func() error) error {
		db, err := store.Open(root)
		if err != nil {
			return fmt.Errorf("writing %s: %w", config.LocalFileName, err)
		}
		return db.Write(context.Background(), func(*store.Tx) error { return fn() })
	}
}

// hostFor is where a workspace's crew run: the workspace's daemon, over
// its socket (started when crew are first launched). Tests replace it.
var hostFor = func(root string) crew.Host { return daemon.NewHost(root) }

// openOps is the workspace's operations for the daemon, its config read
// again when it changes, and its views showing the pull requests prs has
// (which the daemon's watcher shares).
func openOps(root string, prs *ops.PRCache, more ...func(*ops.Options)) *ops.Workspace {
	o := opsOptions(root)
	o.PRs = prs
	for _, f := range more {
		f(&o)
	}
	return ops.Open(root, o)
}

// opsOptions are what ops asks of cli: where crew run, and starting the
// orchestrator's runs and conversations' answers in the daemon.
func opsOptions(root string) ops.Options {
	return ops.Options{
		Host: hostFor(root),
		Live: liveOutput(root),
		Jobs: ops.JobsOf(func(j job.Job) {
			switch j.Kind {
			case job.Orchestrate:
				orchestrateSpawner(root, j.Mission, j.ID, j.Why)
			case job.Check:
				checkSpawner(root, j.Mission, j.ID)
			case job.PR:
				prWriterSpawner(root, j.Mission, j.ID)
			case job.Verify:
				verifySpawner(root, j.Mission, j.ID)
			case job.Thread:
				threadSpawner(root, j.Mission, j.ID)
			case job.Draft:
				draftSpawner(root, j.Mission, j.ID)
			default:
				talkSpawner(root, j.Mission, j.ID)
			}
		}),
	}
}
