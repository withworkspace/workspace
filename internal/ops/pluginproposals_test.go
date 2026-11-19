package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/plugin"
)

// fakeProposer is a plugin proposing issues: each made with the next key,
// one under #N with what N made, one titled "fails" not made; and what it
// was asked.
type fakeProposer struct {
	next    int
	accepts []protocol.ProposalAcceptParams
}

func (f *fakeProposer) Draft(_ context.Context, name string, in protocol.ProposalDraftParams) (plugin.Drafted, error) {
	out := plugin.Drafted{Proposes: protocol.Proposes{ID: "issues", Title: "Jira issues", Icon: "ticket"}}
	if name != "jira" || in.ID != "issues" {
		return out, fmt.Errorf("%w: %s proposes jira:issues", plugin.ErrNoProposes, name)
	}
	if len(in.Input) == 0 {
		out.Guide = `[{"summary": "…"}]`
		return out, nil
	}
	var list []struct{ Summary, Parent string }
	if err := json.Unmarshal(in.Input, &list); err != nil {
		out.Refused = "a JSON list, please"
		return out, nil
	}
	out.Proposal = &protocol.Proposal{Title: fmt.Sprintf("%d issues", len(list))}
	for _, is := range list {
		data, _ := json.Marshal(is)
		out.Proposal.Items = append(out.Proposal.Items, protocol.ProposalItem{Title: is.Summary, Payload: data, Rows: []protocol.ProposalRow{{Label: "Parent", Value: is.Parent}}})
	}
	return out, nil
}

func (f *fakeProposer) Accept(_ context.Context, name string, in protocol.ProposalAcceptParams) (protocol.ProposalAcceptResult, error) {
	f.accepts = append(f.accepts, in)
	var out protocol.ProposalAcceptResult
	made := map[int]string{}
	for n, m := range in.Made {
		made[n] = m
	}
	for _, it := range in.Items {
		var is struct{ Summary, Parent string }
		json.Unmarshal(it.Payload, &is)
		if is.Summary == "fails" {
			out.Items = append(out.Items, protocol.AcceptedItem{N: it.N, Problem: "Jira said no"})
			continue
		}
		if p, ok := strings.CutPrefix(is.Parent, "#"); ok {
			var n int
			fmt.Sscan(p, &n)
			if made[n] == "" {
				out.Items = append(out.Items, protocol.AcceptedItem{N: it.N, Problem: "its parent isn't made"})
				continue
			}
		}
		f.next++
		made[it.N] = fmt.Sprintf("RAM-%d", 100+f.next)
		out.Items = append(out.Items, protocol.AcceptedItem{N: it.N, Made: made[it.N]})
	}
	return out, nil
}

func (f *fakeProposer) Close() {}

func withProposer(t *testing.T, f *fakeProposer) {
	t.Helper()
	was := Proposers
	Proposers = func(string) (Proposer, error) { return f, nil }
	t.Cleanup(func() { Proposers = was })
}

// A plugin's proposal, drafted by the plugin from what the agent wrote,
// accepted in parts: what's picked is done, in order, told what earlier
// items made, under a key that's the same each time; what wasn't done
// stays to accept again, and once none is open, the agent's told what
// each item came to.
func TestPluginProposal_DraftAndAccept(t *testing.T) {
	f := &fakeProposer{}
	withProposer(t, f)
	w, talks, woken := settledFixture(t)

	guide, _, _, err := DraftPlugin(w.Root(), "cache", "t1", "jira:issues", nil, time.Now())
	if err != nil || !strings.Contains(guide, "summary") {
		t.Errorf("the guide: %q, %v", guide, err)
	}
	if _, refused, _, err := DraftPlugin(w.Root(), "cache", "t1", "jira:issues", []byte("not json"), time.Now()); err != nil || refused == "" {
		t.Errorf("refused: %q, %v", refused, err)
	}
	if _, _, _, err := DraftPlugin(w.Root(), "cache", "t1", "jira:bugs", []byte("[]"), time.Now()); !errors.Is(err, plugin.ErrNoProposes) {
		t.Errorf("what the plugin doesn't propose: %v", err)
	}
	_, _, p, err := DraftPlugin(w.Root(), "cache", "t1", "jira:issues", []byte(`[{"Summary":"Epic"},{"Summary":"Story","Parent":"#1"},{"Summary":"fails"},{"Summary":"Spike"}]`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.By = conversation.Agent
	if p.Label != "Jira issues" || p.Title != "4 issues" || len(p.Items) != 4 {
		t.Fatalf("drafted: %+v", p)
	}
	if _, err := talks.Propose("t1", p); err != nil {
		t.Fatal(err)
	}

	// The story alone: its parent isn't made, so it isn't, and nothing's
	// left of what wasn't picked.
	if _, _, err := w.SettleProposal("cache", "t1", 1, true, ProposalEdits{Items: []int{2}}); err == nil || !strings.Contains(err.Error(), "parent") {
		t.Errorf("the story before its epic: %v", err)
	}
	c, _ := talks.Load("t1")
	if got := fmt.Sprint(c.Proposal(1).OpenItems()); got != "[1 2 3 4]" {
		t.Errorf("open after a failure: %s", got)
	}
	// The epic, the story and the one that fails.
	_, _, err = w.SettleProposal("cache", "t1", 1, true, ProposalEdits{Items: []int{3, 1, 2}})
	if err == nil || !strings.Contains(err.Error(), "Done: RAM-101, RAM-102") || !strings.Contains(err.Error(), "Jira said no") {
		t.Errorf("two done, one not: %v", err)
	}
	if a := f.accepts[len(f.accepts)-1]; a.Key != "cache/t1/1" || a.Items[0].N != 1 || a.Conversation != "t1" {
		t.Errorf("asked: %+v", a)
	}
	c, _ = talks.Load("t1")
	if got := fmt.Sprint(c.Proposal(1).OpenItems()); got != "[3 4]" || len(c.Turns) != 2 {
		t.Errorf("open: %s; turns %d (the agent's told nothing yet)", got, len(c.Turns))
	}
	// The spike, leaving the one that fails: it's settled, and the agent's
	// told, quietly.
	said, _, err := w.SettleProposal("cache", "t1", 1, true, ProposalEdits{Items: []int{4}})
	if err != nil || said != "Done: RAM-103" {
		t.Fatalf("the spike: %q, %v", said, err)
	}
	if a := f.accepts[len(f.accepts)-1]; a.Made[1] != "RAM-101" || a.Made[2] != "RAM-102" {
		t.Errorf("not told what was made: %+v", a.Made)
	}
	c, _ = talks.Load("t1")
	p = *c.Proposal(1)
	if p.State != conversation.ProposalAccepted || p.Result != "RAM-101 RAM-102 RAM-103" || p.Items[2].State != conversation.ItemLeft {
		t.Errorf("settled: %+v", p)
	}
	turn := c.Turns[len(c.Turns)-1]
	for _, want := range []string{"Accepted proposal 1, jira:issues “4 issues”.", "- 2, Story: made RAM-102", "- 3, fails: not accepted"} {
		if !strings.Contains(turn.Body, want) {
			t.Errorf("the agent's told %q, missing %q", turn.Body, want)
		}
	}
	if !turn.Quiet || len(*woken) != 0 {
		t.Errorf("woken for it: %v, %v", turn.Quiet, *woken)
	}
}
