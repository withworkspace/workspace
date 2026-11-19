package conversation

import (
	"fmt"
	"strings"
	"testing"
)

func proposalStates(c *Conversation) string {
	var out []string
	for _, p := range c.Proposals {
		out = append(out, strings.TrimSpace(fmt.Sprintf("%d %s %s", p.N, p.State, p.Result)))
	}
	return strings.Join(out, ", ")
}

// A proposal revised by number is superseded by its revision, whatever its
// title; one that isn't open, or isn't of what the revision is, can't be.
func TestProposals_Revises(t *testing.T) {
	ws := ForWorkspace(owned(t))
	ws.OpenTalk(You, "A token cache?", at)
	ws.Propose("w1", Proposal{Kind: ProposeMission, Title: "Token cache", By: Agent, At: at})
	ws.Propose("w1", Proposal{Kind: ProposeDecision, What: "Per tenant.", By: Agent, At: at})
	c, err := ws.Propose("w1", Proposal{Kind: ProposeMission, Title: "Cache tokens per tenant", Revises: 1, By: Agent, At: at})
	if err != nil {
		t.Fatal(err)
	}
	if got := proposalStates(c); got != "1 superseded 3, 2 open, 3 open" {
		t.Errorf("revised by number: %s", got)
	}
	if c.Proposal(3).Revises != 1 {
		t.Errorf("the revision doesn't say what it revises: %+v", c.Proposal(3))
	}

	refused := []struct {
		name string
		p    Proposal
		want string
	}{
		{"none such", Proposal{Kind: ProposeMission, Title: "X", Revises: 9}, "no proposal 9"},
		{"revised already", Proposal{Kind: ProposeMission, Title: "X", Revises: 1}, "as proposal 3"},
		{"another kind", Proposal{Kind: ProposeMission, Title: "X", Revises: 2}, "same kind"},
	}
	for _, r := range refused {
		r.p.By, r.p.At = Agent, at
		if _, err := ws.Propose("w1", r.p); err == nil || !strings.Contains(err.Error(), r.want) {
			t.Errorf("%s: %v, want it refused saying %q", r.name, err, r.want)
		}
	}
	ws.SettleProposal("w1", 3, ProposalAccepted, "token-cache", You, at)
	if _, err := ws.Propose("w1", Proposal{Kind: ProposeMission, Title: "X", Revises: 3, By: Agent, At: at}); err == nil || !strings.Contains(err.Error(), "token-cache") {
		t.Errorf("revising one accepted: %v, want it refused, naming what it made", err)
	}
	c, _ = ws.Load("w1")
	if len(c.Proposals) != 3 {
		t.Errorf("a refused revision was kept: %s", proposalStates(c))
	}

	// A brief with its plan revises the plan.
	s := threads(t)
	m, _ := s.OpenTalk(You, "plan it", at)
	s.Propose(m.ID, Proposal{Kind: ProposePlan, Plan: "## Z\n- A.\n", By: Agent, At: at})
	if c, err = s.Propose(m.ID, Proposal{Kind: ProposeBrief, Brief: "## Goal\nB.\n", Plan: "## Z\n- B.\n", Revises: 1, By: Agent, At: at}); err != nil {
		t.Fatal(err)
	}
	if got := proposalStates(c); got != "1 superseded 2, 2 open" {
		t.Errorf("a brief with its plan revising the plan: %s", got)
	}
}

// The agent withdraws its own proposal, still open, saying why; one
// that's settled, or revised, it can't.
func TestProposals_Withdraw(t *testing.T) {
	ws := ForWorkspace(owned(t))
	ws.OpenTalk(You, "A token cache?", at)
	ws.Propose("w1", Proposal{Kind: ProposeMission, Title: "Token cache", By: Agent, At: at})
	ws.Propose("w1", Proposal{Kind: ProposeDecision, What: "Per tenant.", By: Agent, At: at})
	ws.Propose("w1", Proposal{Kind: ProposeMission, Title: "Session cache", Revises: 1, By: Agent, At: at})
	if _, err := ws.Withdraw("w1", 2, "", at); err == nil {
		t.Error("withdrawn without saying why")
	}
	c, err := ws.Withdraw("w1", 2, "They want it shared across tenants.", at)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Proposal(2); p.State != ProposalWithdrawn || p.Because != "They want it shared across tenants." || p.SettledBy != Agent {
		t.Errorf("withdrawn: %+v", p)
	}
	if got := len(c.Open()); got != 1 {
		t.Errorf("open after withdrawing: %d, want 1", got)
	}
	for n, want := range map[int]string{1: "as proposal 3", 2: "already", 7: "no proposal"} {
		if _, err := ws.Withdraw("w1", n, "no", at); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("withdrawing %d: %v, want %q", n, err, want)
		}
	}
	// Read back from its file.
	c, _ = ws.Load("w1")
	if p := c.Proposal(2); p.State != ProposalWithdrawn || p.Because == "" {
		t.Errorf("withdrawn, read back: %+v", p)
	}
}
