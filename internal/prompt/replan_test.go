package prompt

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
)

// The planner, back at the plan: told the plan as it is, what's behind
// the mission, how a revision's taken, and what's asked of it (by ws, or
// by the user), since its last word.
func TestPlannerFirst(t *testing.T) {
	at := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	c := &conversation.Conversation{ID: "t1", Goal: conversation.GoalBrief, Turns: []conversation.Turn{
		{By: conversation.You, Body: "The login's broken", At: at},
		{By: conversation.Agent, Body: "Proposed the brief and plan.", At: at},
		{By: conversation.WS, Body: "The mission has reached step 3, yours: Plan the fix.", Asks: true, At: at},
		{By: conversation.You, Body: "Keep it small.", At: at},
	}}
	h := Here{Root: "/ws", Mission: &mission.Mission{Slug: "login", Title: "Fix the login"}}
	r := Replanning{Plan: "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Find it.\n   - who: crew\n", Fixed: 3, At: "step 3 of 4, yours (the planner's): Plan the fix."}
	got := PlannerFirst(h, c, r)
	for _, want := range []string{
		`You're the planner of the mission "Fix the login" (login), in conversation t1`,
		"`ws show -f llm t1` has its summary and turns so far",
		"````markdown\n## Z\n- Fixed.",
		"The mission is at step 3 of 4, yours (the planner's): Plan the fix. Steps 1 to 3 are behind it: they stay as they are, word for word",
		"`ws mission plan --propose --set - <<'EOF'`",
		"removes or moves a checkpoint (a Review, Decide or Land step: the user's), or changes Z, is the user's to confirm",
		"Written at your own step, it passes that step",
		"What's asked of you:\n\n(ws) The mission has reached step 3, yours: Plan the fix.\n\nKeep it small.\n\n",
		"Title: <a few words",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("PlannerFirst lacks %q", want)
		}
	}
	if strings.Contains(got, "The login's broken") {
		t.Error("it was told what it answered before as if new")
	}
	// Asked while it answered, after what it answered was written.
	missed := *c
	missed.Turns = append(append([]conversation.Turn{}, c.Turns...),
		conversation.Turn{By: conversation.You, Body: "Accepted proposal 2, the edit to the plan: written to mission login.", Origin: conversation.ProposalOrigin(2), At: at},
		conversation.Turn{By: conversation.Agent, Body: "Revised.", Seen: 4, At: at},
		conversation.Turn{By: conversation.WS, Body: "The mission has reached step 4.", Asks: true, At: at})
	if got := askedSince(&missed); len(got) != 2 || got[0] != "Accepted proposal 2, the edit to the plan: written to mission login." || got[1] != "(ws) The mission has reached step 4." {
		t.Errorf("asked since, with what it missed: %q", got)
	}
	next := PlannerNext(c, r)
	if !strings.Contains(next, "You're the mission's planner, back at its plan.") || !strings.Contains(next, "Keep it small.") {
		t.Errorf("PlannerNext = %q", next)
	}
}
