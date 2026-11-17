package mission

import "time"

// OrchestratorEvent is something that happened on a mission that the
// orchestrator may look at.
type OrchestratorEvent struct {
	// Crew is the crew it's about ("" for the mission's own: an answer, a
	// review, a pull request's change).
	Crew string
	// Trigger is what happened: done, ended or quiet (crew's), answer,
	// review, pr, restarted (the mission's).
	Trigger string
	// Stopped is crew that ended because someone stopped it, rather than
	// by itself.
	Stopped bool
	// At is when it's looked at, for a turn held on crew's own work (Crew.Hold):
	// zero is now.
	At time.Time
}

func (e OrchestratorEvent) at() time.Time {
	if e.At.IsZero() {
		return time.Now()
	}
	return e.At
}

// OrchestratorTakes is whether the orchestrator looks at an event, and why
// not when it doesn't: the one place that's decided. That the
// orchestrator's on at all is the config's to say, and an event it's
// already looked at is told apart as it's taken (MarkOrchestrated).
//
// Crew's events are its to look at when crew report done, end by
// themselves, or finish a turn without asking or reporting done after a
// message ws sent them (not yours: then they're answering you). Not
// briefing crew's, whose end is ws mission ready's, nor crew answering a
// question for you, whose answer is. A turn crew ended to wait on work
// they left running is held while it runs (Crew.Hold): it's taken when
// the work ends, unless the harness wakes them for it; the daemon looks
// again (watchHeld).
func OrchestratorTakes(m *Mission, e OrchestratorEvent) (bool, string) {
	if e.Crew == "" {
		return true, ""
	}
	c, ok := m.CrewByID(e.Crew)
	switch {
	case !ok:
		return false, "no crew " + e.Crew
	case c.Role == RoleBriefing:
		return false, "briefing crew finish with ws mission ready"
	case c.Role == RoleAnswer:
		return false, "crew answering a question finish with their answer"
	case c.Orchestrated == c.eventKey():
		return false, "already looked at"
	}
	switch e.Trigger {
	case "done":
		if c.Status != CrewDone {
			return false, "it hasn't reported done"
		}
	case "ended":
		if e.Stopped {
			return false, "someone stopped it"
		}
		if c.Status == CrewDone {
			return false, "its done was looked at"
		}
	case "quiet":
		if c.Ended != nil || c.Status != "" || c.Conversing || c.Activity != ActivityWaiting {
			return false, "it isn't waiting on ws's message"
		}
		if held, why := c.Hold(e.at()); held {
			return false, why
		}
	}
	return true, ""
}
