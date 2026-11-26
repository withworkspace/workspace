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

func pluginProposal(title string, items ...string) Proposal {
	p := Proposal{Kind: ProposePlugin, Plugin: "jira", Proposes: "issues", Label: "Jira issues", Title: title, By: Agent, At: at}
	for _, it := range items {
		p.Items = append(p.Items, PluginItem{Title: it, Payload: []byte(`{"summary":"` + it + `"}`)})
	}
	return p
}

// A plugin's proposal is settled once none of its items is open: an item
// that wasn't done stays open with why; what's done stands, through a
// decline or a withdrawal of the rest.
func TestProposals_PluginItems(t *testing.T) {
	ws := ForWorkspace(owned(t))
	ws.OpenTalk(You, "issues for this?", at)
	if _, err := ws.Propose("w1", Proposal{Kind: ProposePlugin, Plugin: "jira", Proposes: "issues", By: Agent, At: at}); err == nil {
		t.Error("proposed a plugin's proposal with nothing in it")
	}
	ws.Propose("w1", pluginProposal("Epic and story", "Epic", "Story", "Spike"))
	c, err := ws.ItemsDone("w1", 1, []ItemOutcome{{N: 1, State: ItemMade, Made: "RAM-130"}, {N: 2, Problem: "Jira said no"}}, You, at)
	if err != nil {
		t.Fatal(err)
	}
	p := c.Proposal(1)
	if p.State != ProposalOpen || fmt.Sprint(p.OpenItems()) != "[2 3]" || p.Items[1].Problem != "Jira said no" || p.MadeByItem()[1] != "RAM-130" {
		t.Errorf("one done, one not: %+v", p)
	}
	if _, err := ws.Propose("w1", Proposal{Kind: ProposePlugin, Plugin: "jira", Proposes: "issues", Title: "X", Items: pluginProposal("", "X").Items, Revises: 1, By: Agent, At: at}); err == nil || !strings.Contains(err.Error(), "RAM-130") {
		t.Errorf("revising one with an item done: %v", err)
	}
	c, err = ws.ItemsDone("w1", 1, []ItemOutcome{{N: 2, State: ItemMade, Made: "RAM-131"}, {N: 3, State: ItemLeft}}, You, at)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Proposal(1); p.State != ProposalAccepted || p.Result != "RAM-130 RAM-131" || p.Items[2].State != ItemLeft {
		t.Errorf("settled: %+v", p)
	}

	// Declining the rest, once one's done, accepts what was.
	ws.Propose("w1", pluginProposal("Two more", "A", "B"))
	ws.ItemsDone("w1", 2, []ItemOutcome{{N: 1, State: ItemMade, Made: "RAM-132"}, {N: 2, Problem: "no"}}, You, at)
	c, _ = ws.SettleProposal("w1", 2, ProposalDeclined, "", You, at)
	if p := c.Proposal(2); p.State != ProposalAccepted || p.Result != "RAM-132" || p.Items[1].State != ItemLeft {
		t.Errorf("the rest declined: %+v", p)
	}
	// Withdrawing the rest, likewise.
	ws.Propose("w1", pluginProposal("Two again", "C", "D"))
	ws.ItemsDone("w1", 3, []ItemOutcome{{N: 1, State: ItemMade, Made: "RAM-133"}, {N: 2, Problem: "no"}}, You, at)
	c, _ = ws.Withdraw("w1", 3, "They'll do D themselves.", at)
	if p := c.Proposal(3); p.State != ProposalAccepted || p.Result != "RAM-133" || p.Because == "" {
		t.Errorf("the rest withdrawn: %+v", p)
	}
	// One of the same title again supersedes one with nothing done.
	ws.Propose("w1", pluginProposal("Bugs", "E"))
	c, _ = ws.Propose("w1", pluginProposal("bugs", "E", "F"))
	if got := c.Proposal(4).State; got != ProposalSuperseded {
		t.Errorf("the same title again: %s", got)
	}
	// Read back from its file, with what each item made.
	c, _ = ws.Load("w1")
	if p := c.Proposal(1); p.Items[0].Made != "RAM-130" || p.Items[1].Made != "RAM-131" || p.Items[2].State != ItemLeft {
		t.Errorf("read back: %+v", p.Items)
	}
}
