package mission

import (
	"slices"
	"time"
)

// Idle are the crew of the mission who've sat since before now-after with
// their turn over and nothing said of where they are, at a Work step of
// the plan that's theirs, while nothing else has the mission: crew nobody
// is looking at, and nothing will look at until something's said.
//
// They're not idle who've reported done or asked, who are answering you
// (their turn's end is yours), who left work running that ws is waiting
// on (Hold), or whose harness doesn't say when a turn ends. Nor are any
// while ws or the orchestrator has the mission in hand (checking,
// starting crew, moving it on, deciding), it waits on something outside,
// or something's asked of you or queued for you: that's who has it.
func (m *Mission) Idle(brief string, now time.Time, after time.Duration) []Crew {
	if m.Plan == nil || m.Orchestrating != nil || m.Checking != nil || m.Launching != nil || m.Moving != nil ||
		m.Waiting != nil || m.Dispatch != nil || len(m.Tasks) > 0 {
		return nil
	}
	states, _ := m.StageProgress(brief)
	cur, ok := CurrentStage(states)
	if !ok || cur.Step == nil || cur.Step.Type != StepWork || cur.Record == nil {
		return nil
	}
	var out []Crew
	for _, c := range m.Crew {
		switch {
		case c.Role != "", c.Ended != nil, c.Starting != nil, c.Status != "", c.Conversing:
			continue
		case c.Activity != ActivityWaiting, c.ActivityAt == nil, now.Sub(*c.ActivityAt) < after:
			continue
		case !slices.Contains(cur.Record.Crew, c.ID):
			continue
		}
		if held, _ := c.Hold(now); held {
			continue
		}
		out = append(out, c)
	}
	return out
}
