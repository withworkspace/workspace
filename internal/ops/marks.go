package ops

import (
	"fmt"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
)

// The agents' court is told, not guessed: as ws hands a mission from one
// agent to the next, it marks what it's doing on the mission as it begins
// and clears the mark itself, the way the orchestrator's run is marked as
// its event is taken (Notify). Starting crew is marked from when ws
// begins until the crew are recorded, who then hold it until their first
// report (or, with no hooks of ws's in their session, until they've been
// handed their prompt); the planner's turn (briefing the mission, or coming
// back to its plan) from when it's asked for until the run that has it is
// over, its reply recorded; passing a step and reaching the next (moving)
// from before it's passed until what the next begins with is marked or
// asked. No mark lapses with time, and none is read from whether a process
// is alive.

// launching marks ws starting crew on a mission, for what (launchWhat),
// and returns what ends the mark: a no-op once crew recorded under it
// have taken it over, so it's deferred wherever a launch begins. A mark
// already standing is this one's too: a step reached and the launch it
// makes are one start.
func (w *Workspace) launching(slug, what string) func() {
	l, err := w.Missions.StartLaunching(slug, what)
	if err != nil {
		return func() {}
	}
	return func() { w.Missions.EndLaunching(slug, l) } //nolint:errcheck // a mark
}

// launchingOnConfirm marks ws starting crew on the first step of a mission
// about to be confirmed, when reaching that step starts crew (startsCrew):
// before it's confirmed, and before what comes first (naming it, lifting
// its capsules), so at no moment is it confirmed with nobody on it. What
// it returns ends the mark, under the slug the mission has by then.
func (w *Workspace) launchingOnConfirm(slug string) func(slug string) {
	m, err := w.Missions.Load(slug)
	if err != nil || !m.ToBrief() {
		return func(string) {}
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || !w.startsCrew(cur) {
		return func(string) {}
	}
	l, err := w.Missions.StartLaunching(slug, launchWhat(cur))
	if err != nil {
		return func(string) {}
	}
	return func(slug string) { w.Missions.EndLaunching(slug, l) } //nolint:errcheck // a mark
}

// moving marks ws moving the plan on from step n, and returns what ends
// the mark: held from before the step is passed until what the next step
// begins with has been marked or asked, so at no moment between is the
// mission at a step with nobody on it. Deferred wherever a step is passed
// and the next reached.
func (w *Workspace) moving(slug string, n int) func() {
	mv, err := w.Missions.StartMoving(slug, n)
	if err != nil {
		return func() {}
	}
	return func() { w.Missions.EndMoving(slug, mv) } //nolint:errcheck // a mark
}

// startsCrew reports whether reaching st starts crew at once, as reachStep
// has it: it's crew's, it doesn't wait for a call, and ws may start crew
// without you (else they're queued for you to launch).
func (w *Workspace) startsCrew(st mission.StageState) bool {
	if !w.Config().OrchestratorAutoDispatch() || w.opts.Host == nil {
		return false
	}
	if st.Step == nil {
		return true
	}
	if st.Held() {
		return false
	}
	switch st.Step.Type {
	case mission.StepReview, mission.StepDecide, mission.StepLand, mission.StepPlan, mission.StepWait, mission.StepOpenPR:
		return false
	}
	return true
}

// launchWhat is what crew being started are for, as the mark says it:
// "step 3", or "" for nothing in particular.
func launchWhat(st mission.StageState) string {
	if st.N == 0 {
		return ""
	}
	return fmt.Sprintf("%s %d", st.Noun(), st.N)
}

// markPlanning marks the planner's turn underway as it's asked for, when the
// conversation to answer is the mission's planner's (its briefing
// conversation, carried on): before the job's handed over, so the turn
// counts from the asking. The run that has it ends the mark
// (AnswerConversation).
func (w *Workspace) markPlanning(kind string, sc doc.Scope, id string) {
	if kind != job.Answer || sc.IsWorkspace() {
		return
	}
	c, err := conversation.For(w.Root(), sc.Mission).Load(id)
	if err != nil || !isPlannerTalk(c) {
		return
	}
	w.Missions.StartPlanning(sc.Mission, id) //nolint:errcheck // a mark
}

// isPlannerTalk is whether a mission's conversation is the one its planner
// answers in: briefing it, and once it's briefed, coming back to its plan.
func isPlannerTalk(c *conversation.Conversation) bool {
	return c != nil && c.Goal == conversation.GoalBrief && c.Status == conversation.Open
}

// planningIn is whether the planner's turn is marked underway in the
// mission's conversation id.
func planningIn(m *mission.Mission, id string) bool {
	return m.Planning != nil && m.Planning.In == id
}
