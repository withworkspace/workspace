package ops

import (
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
)

// Notify tells the orchestrator of an event on a mission, and says whether
// it'll look at it: when it's on, the event is one it looks at
// (mission.OrchestratorTakes), and it hasn't looked at that event already.
// It looks in the workspace's daemon, one run at a time per mission.
func (w *Workspace) Notify(slug string, e mission.OrchestratorEvent) bool {
	who := "orchestrator " + slug
	if !w.Config().OrchestratorEnabled() {
		execlog.Note(w.Root(), who, "not run for %s: [orchestrator] enabled = false", describe(e))
		return false
	}
	m, err := w.Missions.Load(slug)
	if err != nil {
		return false
	}
	if takes, why := mission.OrchestratorTakes(m, e); !takes {
		execlog.Note(w.Root(), who, "not run for %s: %s", describe(e), why)
		return false
	}
	if e.Crew != "" {
		if fresh, err := w.Missions.MarkOrchestrated(slug, e.Crew); err != nil || !fresh {
			execlog.Note(w.Root(), who, "not run for %s: already looked at", describe(e))
			return false
		}
	}
	w.start(job.Job{Kind: job.Orchestrate, Mission: slug, ID: e.Crew, Why: e.Trigger})
	return true
}

func describe(e mission.OrchestratorEvent) string {
	if e.Crew == "" {
		return e.Trigger
	}
	return e.Crew + " (" + e.Trigger + ")"
}
