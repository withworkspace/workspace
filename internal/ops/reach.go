package ops

import (
	"strconv"

	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
)

// Reaching the step after one you passed can take a while: crew started
// in a capsule lifted for them (a fetch, a worktree, its setup hook, a
// process), a pull request pushed and opened. In the daemon that's done
// behind your answer, by its job (job.Reach), with the mission marked
// moving on (Moving.Behind) until it's done, so it's never at a step with
// nobody on it. A daemon that goes meanwhile leaves the mark, and the next
// takes the job up again (ReachBehind is said again for it). The
// orchestrator's passing, and a command's, reach it there and then.

// movingOn is what's said of the step after, reached behind the answer.
const movingOn = "moving on to what's next"

// reachLater hands reaching what's after step n (0: the step it's at,
// nothing passed) to the daemon's job, where it's you and there's a
// daemon; it says whether it did. The moving mark standing, if any, is
// the job's to end from then on.
func (w *Workspace) reachLater(slug string, n int, by Actor) bool {
	if !by.IsYou() || w.opts.Outbox == nil || !w.takes(job.Reach) {
		return false
	}
	if _, err := w.Missions.MoveBehind(slug, n); err != nil {
		return false
	}
	return w.start(job.Job{Kind: job.Reach, Mission: slug, ID: strconv.Itoa(n)})
}

// ReachBehind is the daemon's job for a step passed behind its answer
// (reachLater): what's after step n is reached, as it would have been
// there and then, and the mark ended. Nothing, where the mark isn't for
// it (it's done), or what's next has crew on it already (a daemon went in
// the middle of reaching it). It says what it did.
func (w *Workspace) ReachBehind(slug string, n int) string {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return ""
	}
	mv := m.Moving
	if mv == nil || !mv.Behind || mv.Step != n {
		return ""
	}
	defer func() { logFailed(w.Missions.EndMoving(slug, *mv)) }()
	brief, _ := w.Missions.ReadBrief(slug)
	states, _ := m.StageProgress(brief)
	if next, ok := mission.CurrentStage(states); ok && next.Record != nil && len(next.Record.Crew) > 0 && next.N > n {
		return "step " + strconv.Itoa(next.N) + " has crew on it already"
	}
	if n == 0 {
		cur, ok := currentStep(w.Missions, m)
		if !ok {
			return ""
		}
		return w.reachStep(m, mission.StageState{}, cur)
	}
	if n > len(states) {
		return ""
	}
	return w.afterAccept(slug, states[n-1], states)
}
