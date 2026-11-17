package crew

import (
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// LeftRunning is what crew left running at the end of a turn, as their
// harness's adapter said (harness.Pending), kept as the mission keeps it:
// none when there's none, or the adapter can't tell. Either way the turn
// is taken as it always was.
func LeftRunning(p harness.Pending) []mission.Work {
	if !p.Known {
		return nil
	}
	var out []mission.Work
	for _, w := range p.Work {
		wakes := mission.WakeUnknown
		switch w.Wakes {
		case harness.WakeYes:
			wakes = mission.WakeYes
		case harness.WakeNo:
			wakes = mission.WakeNo
		}
		out = append(out, mission.Work{What: w.What, From: w.From, PID: w.PID, Wakes: wakes, Call: w.Call})
	}
	return out
}
