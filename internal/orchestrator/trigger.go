package orchestrator

import (
	"time"
)

// Spawner starts the orchestrator for an event: a crew's (done, ended,
// quiet, checks) or the mission's (answer, review, pr, plan, limit,
// step).
type Spawner func(root, slug, crewID, trigger string)

// RunFor is how long an orchestrator's run counts as underway: its own
// timeout twice (a run that does nothing runs again), and time to gather
// what it's told.
func RunFor() time.Duration { return 2*Timeout + time.Minute }

// Describe is what a run looks at, in words: "c2 (done)", "your answer".
func Describe(crewID, trigger string) string {
	switch {
	case crewID != "":
		return crewID + " (" + trigger + ")"
	case trigger == "answer":
		return "the answer to its question"
	case trigger == "review":
		return "your review"
	case trigger == "pr":
		return "a change to a PR"
	case trigger == "plan":
		return "the plan, revised"
	case trigger == "limit":
		return "a step out of time"
	case trigger == "step":
		return "a step to start or skip"
	}
	return "the mission"
}
