package ops

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/conversation"
)

// A mission's conversation proposes a replan, which changes nothing until
// the user confirms it. Confirming asks the planner as ws mission replan
// does (AskPlanner, as you: your words, in its conversation, logged), and
// the proposal's result is that conversation; declining does nothing. The
// agent's told what became of each, in the turn written for it: declined,
// woken to take it in; confirmed, quietly, where the planner was asked. A
// workspace's conversation has no plan to replan.
func TestReplanProposal(t *testing.T) {
	w, _ := fixture(t, "## Goal\nFix it.\n", "one", "two")
	w.Missions.WritePlan("cache", replanPlan)
	store := conversation.For(w.Root(), "cache")
	talk, err := store.OpenTalk(conversation.You, "Keep it open until it's closed, and make the AppBar's workspace title a dropdown. Can you replan?", w.now())
	if err != nil {
		t.Fatal(err)
	}
	propose := func(text string) int {
		t.Helper()
		c, err := store.Propose(talk.ID, conversation.Proposal{Kind: conversation.ProposeReplan, By: conversation.Agent, At: w.now(), Text: text})
		if err != nil {
			t.Fatalf("proposing a replan: %v", err)
		}
		return len(c.Proposals)
	}
	ask := "Add a step for the AppBar's workspace title as a dropdown."
	declined := propose("Drop the review of the fix.")
	accepted := propose(ask)
	untouched := func(when string) {
		t.Helper()
		if p := store.Planner(); p != nil {
			t.Errorf("%s, the planner was asked: %+v", when, p)
		}
		if logSays(t, w, "asked the planner") {
			t.Errorf("%s, the log says the planner was asked", when)
		}
		if text, _ := w.Missions.ReadPlan("cache"); text != replanPlan {
			t.Errorf("%s, the plan changed", when)
		}
	}
	untouched("proposed")

	if said, err := w.SettleProposalIn(store, talk.ID, declined, false, ProposalEdits{}); err != nil || said != "Declined." {
		t.Fatalf("declining: %q, %v", said, err)
	}
	untouched("declined")
	told := func(n int) conversation.Turn {
		t.Helper()
		c, _ := store.Load(talk.ID)
		for _, turn := range c.Turns {
			if turn.Origin == conversation.ProposalOrigin(n) {
				return turn
			}
		}
		t.Fatalf("no turn tells the agent of proposal %d: %+v", n, c.Turns)
		return conversation.Turn{}
	}
	if turn := told(declined); turn.Body != "Declined proposal 1, the replan “Drop the review of the fix”." || turn.Quiet {
		t.Errorf("the agent's told of the declined replan: %+v", turn)
	}

	said, err := w.SettleProposalIn(store, talk.ID, accepted, true, ProposalEdits{})
	if err != nil {
		t.Fatalf("confirming a replan: %v", err)
	}
	planner := store.Planner()
	if planner == nil || planner.ID == talk.ID {
		t.Fatalf("confirming didn't ask the planner, in its own conversation: %+v", planner)
	}
	if !strings.Contains(said, "Asked the planner, in "+planner.ID) {
		t.Errorf("confirming said %q", said)
	}
	if last := planner.Last(); last.By != conversation.You || last.Body != ask || !planner.NeedsAnswer() {
		t.Errorf("the planner was asked %+v, not the user's ask", last)
	}
	if !logSays(t, w, "asked the planner at step 1, in "+planner.ID+": "+ask) {
		t.Error("the log doesn't say the planner was asked, as ws mission replan says it")
	}
	c, _ := store.Load(talk.ID)
	if p := c.Proposal(accepted); p.State != conversation.ProposalAccepted || p.Result != planner.ID {
		t.Errorf("the confirmed proposal: %+v", p)
	}
	if p := c.Proposal(declined); p.State != conversation.ProposalDeclined {
		t.Errorf("the declined proposal: %+v", p)
	}
	if turn := told(accepted); turn.Body != "Accepted proposal 2, the replan “"+strings.TrimSuffix(ask, ".")+"”: the planner's asked, in "+planner.ID+", and revises the plan there or says why not." || !turn.Quiet {
		t.Errorf("the agent's told of the confirmed replan: %+v", turn)
	}

	ws := conversation.ForWorkspace(w.Root())
	wt, err := ws.OpenTalk(conversation.You, "Can you replan?", w.now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ws.Propose(wt.ID, conversation.Proposal{Kind: conversation.ProposeReplan, By: conversation.Agent, At: w.now(), Text: ask}); err == nil {
		t.Error("a workspace's conversation proposed a replan")
	}
}
