package ops

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/notification"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// What an automation proposes is a card in a conversation of its own:
// ws's words first, no agent woken for them, the card the one you
// accept or decline, and you're told. A second in the run goes in the
// same conversation. Declined, it stands so, and no agent's woken, as
// there's none that proposed it.
func TestAutomationPropose(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	var woken []string
	w := New(&config.Config{}, &workspace.Workspace{Root: root}, nil, Options{Jobs: JobsOf(func(j job.Job) { woken = append(woken, j.Kind+" "+j.ID) }, job.Answer, job.Thread)})
	a := &automation.Automation{Name: "sketch-mission"}
	ref, err := w.automationPropose(a, automation.Proposing{Kind: "mission", Input: map[string]any{"title": "Build Queues", "about": []any{"sketch-3"}}, Why: "Brendan drew Queues", On: "e12: Brendan added Queues"}, "", []string{"sketch-mission"})
	if err != nil {
		t.Fatal(err)
	}
	id, _, _ := strings.Cut(ref, "#")
	second, err := w.automationPropose(a, automation.Proposing{Kind: "decision", Input: map[string]any{"what": "Queues next"}}, id, []string{"sketch-mission"})
	if err != nil || second != id+"#2" {
		t.Fatalf("second: %q, %v", second, err)
	}
	c, err := conversation.ForWorkspace(root).Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if c.Via != "automation:sketch-mission" || strings.Join(c.Chain, ">") != "sketch-mission" || c.NeedsAnswer() || !c.WaitingOnYou() || len(woken) != 0 {
		t.Errorf("conversation %+v, woken %q", c, woken)
	}
	if c.Title != "sketch-mission proposes a mission: Build Queues" {
		t.Errorf("titled %q", c.Title)
	}
	if t0 := c.Turns[0]; t0.By != conversation.WS || !strings.Contains(t0.Body, "Brendan drew Queues") || !strings.Contains(t0.Body, "on e12: Brendan added Queues") {
		t.Errorf("first turn %+v", t0)
	}
	if p := c.Proposals[0]; p.Kind != conversation.ProposeMission || p.Title != "Build Queues" || strings.Join(p.About, ",") != "sketch-3" || p.By != conversation.WS {
		t.Errorf("proposal %+v", p)
	}
	if state, _, ok := w.automationProposalState(ref); !ok || state != automation.ProposalOpen {
		t.Errorf("%s stands %q %v", ref, state, ok)
	}
	list, _ := notification.List(root, notification.Filter{})
	if len(list) != 2 || !strings.Contains(list[0].Text+list[1].Text, "sketch-mission proposes a mission: Build Queues") {
		t.Errorf("told %+v", list)
	}

	if _, _, err := w.SettleProposal("", id, 2, false, ProposalEdits{}); err != nil {
		t.Fatal(err)
	}
	if state, _, _ := w.automationProposalState(second); state != conversation.ProposalDeclined || len(woken) != 0 {
		t.Errorf("declined: %q, woken %q", state, woken)
	}
	if _, _, ok := w.automationProposalState(id + "#9"); ok {
		t.Error("a proposal there isn't stands")
	}

	_, err = w.automationPropose(a, automation.Proposing{Kind: "jira:changes", Input: map[string]any{}}, id, nil)
	if err == nil {
		t.Error("a plugin's kind with no plugin was proposed")
	}
}
