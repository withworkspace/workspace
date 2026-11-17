package ops

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// A bug's plan, as the planner might write it: the fix is planned once the
// cause is found.
const replanPlan = `## Z
- Merged, with its regression test.

## Stops early
- It can't be reproduced.

## Steps
1. Reproduce it as a failing test.
   - who: crew, in ` + "`frontend/one`" + `
2. Is this the cause?
   - who: you
3. Plan the fix.
   - who: the planner
4. Review the fix.
   - who: you
`

// fixPlanned is replanPlan with the planner's steps after step 3.
var fixPlanned = strings.Replace(replanPlan, "4. Review the fix.", "4. Fix it, the test passing.\n   - who: crew, in `frontend/two`\n5. Review the fix.", 1)

// replanFixture is a mission with replanPlan, at step 3, the planner's:
// steps 1 and 2 passed.
func replanFixture(t *testing.T) (*Workspace, *fakeHost) {
	t.Helper()
	w, _ := fixture(t, "## Goal\nFix it.\n", "one", "two")
	w.Missions.WritePlan("cache", replanPlan)
	host := &fakeHost{}
	w.opts.Host = host
	harness.Register(fakeHarness{})
	w.Config().Crew.Harness = "fake-crew"
	w.Config().Orchestrator.Dispatch = "auto"
	if _, err := w.AcceptStage("cache", AsOrchestrator, "reproduced"); err != nil {
		t.Fatal(err)
	}
	a, err := w.Answer("cache", AsYou, "go on")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Next, "step 3 is the planner's: it's asked, in t1, to write the steps after it, and ws moves on once it has") {
		t.Fatalf("reaching step 3: %s", a.Next)
	}
	return w, host
}

// Reaching the planner's step runs the planner: ws asks it in its
// conversation, which its agent answers; what it writes there applies,
// passes its step, and ws moves on.
func TestPlannerAtItsStep(t *testing.T) {
	w, host := replanFixture(t)
	store := conversation.For(w.Root(), "cache")
	c := store.Planner()
	if c == nil || c.Goal != conversation.GoalBrief || !c.NeedsAnswer() || c.WaitingOnYou() {
		t.Fatalf("the planner's conversation = %+v", c)
	}
	last := c.Last()
	if last.By != conversation.WS || !last.Asks || !strings.Contains(last.Body, "The mission has reached step 3, yours: Plan the fix.") {
		t.Errorf("what ws asked = %+v", last)
	}
	if !w.IsPlannerTalk("cache", c.ID) {
		t.Error("its conversation isn't the planner's")
	}
	r, ok := w.replanning(mustLoad(t, w, "cache"))
	if !ok || r.Fixed != 3 || r.At != "step 3 of 4, yours (the planner's): Plan the fix." {
		t.Errorf("the planner's told %+v", r)
	}

	// The steps behind the mission stay as they were.
	_, err := w.RevisePlan("cache", c.ID, strings.Replace(fixPlanned, "Reproduce it as a failing test.", "Reproduce it.", 1))
	refusedWith(t, err, "steps-behind")
	// A malformed plan is refused, saying why.
	_, err = w.RevisePlan("cache", c.ID, strings.Replace(fixPlanned, "   - who: the planner\n", "", 1))
	if err == nil || !strings.Contains(err.Error(), "step 3 doesn't say what kind of step it is") {
		t.Errorf("malformed: %v", err)
	}

	// Writing the steps after it passes its step: step 4's crew start.
	rev, err := w.RevisePlan("cache", c.ID, fixPlanned)
	if err != nil {
		t.Fatal(err)
	}
	if rev.Proposed != 0 || !strings.HasPrefix(rev.String(), "Revised the plan: step 4 is step 5 now (Review the fix); step 4 added (Fix it, the test passing, crew, in `frontend/two`). Passed step 3, and started step 4: c1 in frontend/two.") {
		t.Errorf("revised: %s", rev)
	}
	if len(host.specs) != 1 || !strings.Contains(host.specs[0].Prompt, "Fix it, the test passing.") {
		t.Errorf("started %d crew", len(host.specs))
	}
	m := mustLoad(t, w, "cache")
	states, _ := m.StageProgress("")
	if len(states) != 5 || !states[2].Accepted() || states[2].Record.AcceptedBy != "planner" || !states[3].Current {
		t.Errorf("steps = %+v", states)
	}
	if !logSays(t, w, "revised the plan, in t1: step 4 is step 5 now (Review the fix); step 4 added") {
		t.Error("the log doesn't say what changed")
	}
	// Elsewhere than its conversation, it's refused.
	other, _ := store.OpenTalk(conversation.You, "what's next?", w.now())
	_, err = w.RevisePlan("cache", other.ID, fixPlanned)
	refusedWith(t, err, "plan-is-the-planners")
}

// A revision that removes or moves a checkpoint, or changes Z, comes to
// you as a proposal; accepting the one the planner wrote at its step
// passes that step.
func TestPlannerRevisionIsYours(t *testing.T) {
	w, host := replanFixture(t)
	store := conversation.For(w.Root(), "cache")
	id := store.Planner().ID
	noReview := strings.Replace(fixPlanned, "5. Review the fix.\n   - who: you\n", "", 1)
	rev, err := w.RevisePlan("cache", id, noReview)
	if err != nil {
		t.Fatal(err)
	}
	if rev.Proposed != 1 || rev.String() != "Proposed to the user (proposal 1 in t1): It removes the checkpoint at step 4 (Review the fix), which is the user's to confirm. They accept it, or not." {
		t.Errorf("revised: %s", rev)
	}
	if text, _ := w.Missions.ReadPlan("cache"); text != replanPlan {
		t.Error("the plan changed before the user accepted it")
	}
	c, _ := store.Load(id)
	p := c.Proposal(1)
	if p == nil || p.Kind != conversation.ProposePlan || p.Step != 3 || p.Was != replanPlan || !strings.HasPrefix(p.Why, "It removes the checkpoint") {
		t.Fatalf("the proposal = %+v", p)
	}
	said, err := w.SettleProposalIn(store, id, 1, true, ProposalEdits{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(said, "Revised the plan:") || !strings.Contains(said, "Passed step 3, and started step 4: c1 in frontend/two.") {
		t.Errorf("accepted: %s", said)
	}
	if len(host.specs) != 1 {
		t.Errorf("started %d crew", len(host.specs))
	}
}

// The planner's asked by the orchestrator, when the route doesn't fit
// (once at a step), and by you; not by crew.
func TestAskPlanner(t *testing.T) {
	w, _ := fixture(t, "## Goal\nFix it.\n", "one", "two")
	_, err := w.AskPlanner("cache", "the cause is elsewhere", AsOrchestrator)
	refusedWith(t, err, "no-plan")
	w.Missions.WritePlan("cache", replanPlan)
	_, err = w.AskPlanner("cache", "the cause is elsewhere", AsCrew("c1"))
	refusedWith(t, err, "not-yours")

	a, err := w.AskPlanner("cache", "the bug is in the backend, not frontend", AsOrchestrator)
	if err != nil || a.Conversation != "t1" {
		t.Fatalf("asked: %+v, %v", a, err)
	}
	store := conversation.For(w.Root(), "cache")
	c, _ := store.Load("t1")
	if c.Last().By != conversation.WS || !strings.Contains(c.Last().Body, "The orchestrator says the plan doesn't fit what's happened, at step 1: the bug is in the backend") {
		t.Errorf("asked %+v", c.Last())
	}
	// It's at work: asking again waits.
	_, err = w.AskPlanner("cache", "and another thing", AsOrchestrator)
	refusedWith(t, err, "planner-busy")
	// It answered, and the plan still doesn't fit: that's the user's now.
	store.Say("t1", conversation.Turn{By: conversation.Agent, At: w.now(), Body: "It fits: the backend's out of scope."})
	_, err = w.AskPlanner("cache", "it still doesn't fit", AsOrchestrator)
	refusedWith(t, err, "asked-already")

	// You ask in your words, in the same conversation.
	if _, err := w.AskPlanner("cache", "Add a step to tell product.", AsYou); err != nil {
		t.Fatal(err)
	}
	c, _ = store.Load("t1")
	if c.Last().By != conversation.You || c.Last().Body != "Add a step to tell product." || !c.NeedsAnswer() {
		t.Errorf("you asked %+v", c.Last())
	}

	// Briefing, the planner's briefing it.
	b, _ := w.Missions.CreateBriefing("fix the login")
	_, err = w.AskPlanner(b.Slug, "plan it", AsYou)
	refusedWith(t, err, "briefing")
}

// The planner comes back in the conversation the mission was briefed in.
func TestPlannerInTheBriefingConversation(t *testing.T) {
	w, _ := fixture(t, "## Goal\nFix it.\n", "one", "two")
	w.Missions.WritePlan("cache", replanPlan)
	store := conversation.For(w.Root(), "cache")
	b, err := store.OpenBriefing("the login's broken", w.now())
	if err != nil {
		t.Fatal(err)
	}
	store.Say(b.ID, conversation.Turn{By: conversation.Agent, At: w.now(), Body: "Proposed."})
	store.SetStatus(b.ID, conversation.You, conversation.Resolved, w.now())
	if _, err := w.AskPlanner("cache", "the route doesn't fit", AsOrchestrator); err != nil {
		t.Fatal(err)
	}
	c, _ := store.Load(b.ID)
	if c.Status != conversation.Open || !c.NeedsAnswer() || len(c.Turns) != 3 {
		t.Errorf("the briefing conversation, taken up again: %+v", c)
	}
}

// Your edits keep the steps behind the mission as they were too.
func TestSetPlan_Revising(t *testing.T) {
	w, _ := replanFixture(t)
	_, err := w.SetPlan("cache", strings.Replace(replanPlan, "Is this the cause?", "Is it?", 1), AsYou)
	refusedWith(t, err, "steps-behind")
	set, err := w.SetPlan("cache", fixPlanned, AsYou)
	if err != nil || !strings.HasPrefix(set.String(), "Revised the plan: step 4 is step 5 now") {
		t.Errorf("set: %s, %v", set, err)
	}
	if !logSays(t, w, "plan revised: step 4 is step 5 now") {
		t.Error("the log doesn't say what changed")
	}
}

func mustLoad(t *testing.T, w *Workspace, slug string) *mission.Mission {
	t.Helper()
	m, err := w.Missions.Load(slug)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// logSays reports whether a line of the mission's log has text in it.
func logSays(t *testing.T, w *Workspace, text string) bool {
	t.Helper()
	events, _ := w.Missions.ReadLog("cache", 0)
	for _, e := range events {
		if strings.Contains(e.Text, text) {
			return true
		}
	}
	return false
}
