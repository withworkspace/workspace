package ops

import (
	"os"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/mission"
)

// What this ws did is kept as events (internal/events). What a mission's
// log says of its missions is written there with it (mission/events.go);
// what's done outside a mission (a capsule of none landed or burned, an
// artifact added or promoted, a document added or amended) is emitted
// here, as it's done.

// emit keeps e as done by by: best said and never in the way, as what was
// done has been done whether or not it's kept.
func (w *Workspace) emit(e events.Event, by Actor) {
	e.By = byOf(by)
	if e.Cause == "" {
		e.Cause = os.Getenv(mission.EnvRun)
	}
	events.Emit(w.Root(), e)
}

// byOf is who an actor is, as an event says it.
func byOf(a Actor) string {
	switch a.Kind {
	case Crew:
		return events.ByCrew(a.ID)
	case Orchestrator:
		return events.ByOrchestrator
	case WS:
		return events.ByWS
	}
	return events.ByYou
}
