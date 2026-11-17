package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

const replanCLIPlan = `## Z
- The lookup is cached, with its tests.

## Stops early
- None.

## Steps
1. Measure the lookup.
   - who: crew, in ` + "`backend/cache-lookup`" + `
2. Plan the cache.
   - who: the planner
3. Is the cache right?
   - who: you
`

// Revising: you ask the planner (ws mission replan), and it comes back in
// its conversation; what it writes there (ws mission plan --propose) is a
// revision ws applies, or proposes to you when it touches a checkpoint or
// Z. The orchestrator is told to ask it when the route doesn't fit.
func TestPlanner_Revises(t *testing.T) {
	w, store := orchestrateFixture(t)
	if err := store.WritePlan("cache", replanCLIPlan); err != nil {
		t.Fatal(err)
	}

	r := run(t, w, nil, "mission", "replan", "-m", "cache", "Measure it under load too.")
	if !strings.Contains(r.Stderr, "asked the planner of cache, in t1") {
		t.Errorf("replan said %q", r.Stderr)
	}
	c, _ := conversation.For(w.Root, "cache").Load("t1")
	if c.Goal != conversation.GoalBrief || c.Last().By != conversation.You || !c.NeedsAnswer() {
		t.Errorf("the planner's conversation: %+v", c)
	}
	// Crew say what doesn't fit in their handoff.
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c1")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "replan", "it doesn't fit"); r.Err == nil || !strings.Contains(r.Err.Error(), "say what doesn't fit the plan in your handoff") {
		t.Errorf("crew asked the planner: %v", r.Err)
	}

	// The planner, in its conversation.
	t.Setenv(conversation.EnvConversation, "t1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	withStdin(t, strings.Replace(strings.Replace(replanCLIPlan, "3. Is the cache right?", "4. Is the cache right?", 1), "2. Plan the cache.", "2. Measure it under load.\n   - who: crew, in `backend/cache-lookup`\n3. Plan the cache.", 1))
	// Step 1 hasn't begun (no crew joined it), so it may go after it.
	r = testutil.RunCommand(t, w.Root, nil, "mission", "plan", "--propose", "--set", "-")
	if r.Err != nil || !strings.Contains(r.Stderr, "✓ Revised the plan: step 2 is step 3 now (Plan the cache); step 3 is step 4 now (Is the cache right?); step 2 added (Measure it under load, crew, in `backend/cache-lookup`)") {
		t.Fatalf("revising: %v\n%s", r.Err, r.Stderr)
	}
	// Z changed is the user's: proposed to them.
	plan, _ := store.ReadPlan("cache")
	withStdin(t, strings.Replace(plan, "- The lookup is cached, with its tests.", "- The lookup is cached, with its tests.\n- It's twice as fast under load.", 1))
	r = run(t, w, nil, "mission", "plan", "--propose", "--set", "-")
	if !strings.Contains(r.Stderr, "Proposed to the user (proposal 1 in t1): It changes Z, which is the user's to confirm.") {
		t.Errorf("changing Z said %q", r.Stderr)
	}
	if now, _ := store.ReadPlan("cache"); now != plan {
		t.Error("Z changed before the user confirmed it")
	}
	// In the conversation, the plan is proposed, not set.
	withStdin(t, plan)
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "plan", "--set", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "propose the plan with --propose") {
		t.Errorf("--set in the conversation: %v", r.Err)
	}
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")

	// The orchestrator asks it when the route doesn't fit.
	prompt := run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--print-prompt").Stdout
	if !strings.Contains(prompt, "ws mission replan \"<what doesn't fit, and what happened>\"") || !strings.Contains(prompt, "Plan: the planner writes the steps after it") {
		t.Error("the orchestrator isn't told to ask the planner")
	}
}

// A mission's conversation doesn't ask the planner itself: it proposes a
// replan (ws mission replan --propose), shown in the conversation for the
// user to confirm. Proposed or declined, nothing changes; confirming asks
// the planner as ws mission replan does.
func TestTalk_ProposesReplan(t *testing.T) {
	w, store := orchestrateFixture(t)
	if err := store.WritePlan("cache", replanCLIPlan); err != nil {
		t.Fatal(err)
	}
	talks := conversation.For(w.Root, "cache")
	talks.OpenTalk(conversation.You, "Keep it open until it's closed, and make the AppBar's workspace title a dropdown. Can you replan?", time.Now())
	ask := "Add a step for the AppBar's workspace title as a dropdown."

	if r := testutil.RunCommand(t, w.Root, nil, "mission", "replan", "-m", "cache", "--propose", ask); r.Err == nil || !strings.Contains(r.Err.Error(), "for a conversation's agent") {
		t.Errorf("proposing outside a conversation: %v", r.Err)
	}
	t.Setenv(conversation.EnvConversation, "t1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	t.Setenv(mission.EnvMission, "cache")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "replan", ask); r.Err == nil || !strings.Contains(r.Err.Error(), "--propose") {
		t.Errorf("asking the planner in a conversation isn't refused, saying to propose: %v", r.Err)
	}
	r := testutil.RunCommand(t, w.Root, nil, "mission", "replan", "--propose", ask)
	if r.Err != nil || !strings.Contains(r.Stderr, "proposed in t1 (proposal 1)") {
		t.Fatalf("proposing a replan: %v\n%s", r.Err, r.Stderr)
	}
	run(t, w, nil, "mission", "replan", "--propose", "Drop the checkpoint at step 3.")
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")

	c, _ := talks.Load("t1")
	if len(c.Proposals) != 2 || c.Proposals[0].Kind != conversation.ProposeReplan || c.Proposals[0].Text != ask || c.Proposals[0].State != conversation.ProposalOpen {
		t.Fatalf("proposals = %+v", c.Proposals)
	}
	if out := run(t, w, nil, "show", "-f", "llm", "t1").Stdout; !strings.Contains(out, ask+" (waiting for the user)") {
		t.Errorf("ws show t1 doesn't list the replan proposed:\n%s", out)
	}
	untouched := func(when string) {
		t.Helper()
		if p := talks.Planner(); p != nil {
			t.Errorf("%s, the planner was asked: %+v", when, p)
		}
		if plan, _ := store.ReadPlan("cache"); plan != replanCLIPlan {
			t.Errorf("%s, the plan changed", when)
		}
	}
	untouched("proposed")

	if r := run(t, w, nil, "conversation", "decline", "-m", "cache", "t1", "2"); !strings.Contains(r.Stderr, "Declined.") {
		t.Errorf("declining said %q", r.Stderr)
	}
	untouched("declined")

	if r := run(t, w, nil, "conversation", "accept", "-m", "cache", "t1", "1"); !strings.Contains(r.Stderr, "Asked the planner, in t2") {
		t.Errorf("confirming said %q", r.Stderr)
	}
	p := talks.Planner()
	if p == nil || p.ID != "t2" || p.Last().By != conversation.You || p.Last().Body != ask || !p.NeedsAnswer() {
		t.Fatalf("the planner's conversation, after confirming: %+v", p)
	}
	if out := run(t, w, nil, "show", "-f", "llm", "t1").Stdout; !strings.Contains(out, ask+" (accepted: t2)") {
		t.Errorf("ws show t1 doesn't say the replan was confirmed, and where the planner was asked:\n%s", out)
	}
}
