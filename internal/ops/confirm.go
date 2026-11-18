package ops

import (
	"errors"
	"fmt"

	"github.com/withworkspace/workspace/internal/mission"
)

// A mission starts from your words, a brief you've confirmed and a plan,
// and it's under way: confirming them is the go-ahead, and ws reaches the
// plan's first step at once (its crew start, or it waits for a call). An
// idea stays an idea, to brief, until then.

// ConfirmBrief records the mission's brief (and plan) confirmed, by by
// (you, or a briefing crew you're briefing it with in its terminal), and
// starts it: the plan's first step is reached. A mission without a plan
// waits for its crew to be launched. It says what it did.
func (w *Workspace) ConfirmBrief(slug string, by Actor) (string, error) {
	// Starting its first step's crew is marked before it's confirmed: at
	// no moment is it confirmed with nobody on it.
	defer w.launchingOnConfirm(slug)(slug)
	if err := w.Missions.Confirm(slug, by.By()); err != nil {
		if errors.Is(err, mission.ErrConfirmed) {
			return "", refused("confirmed", "%v", err)
		}
		return "", wrap(err)
	}
	w.youActed(slug, by)
	m, err := w.Missions.Load(slug)
	if err != nil {
		return "", err
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok {
		return "it's ready for crew", nil
	}
	return fmt.Sprintf("it's under way: %s", w.reachStep(m, mission.StageState{}, cur)), nil
}
