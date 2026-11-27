package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/notification"
	"github.com/withworkspace/workspace/internal/spend"
	"github.com/withworkspace/workspace/internal/turn"
)

// An automation's ask and propose, as ws's own operations: an ask is one
// turn of the [automations] agent, headless with no tools and nothing
// kept of its session, what it spends kept for the automation; a
// proposal is a card in a conversation of the automation's, made as a
// conversation's agent makes one, and you're told of it.

// askTurnFor is how long an automation's ask may take.
const askTurnFor = 3 * time.Minute

// automationAsk has the [automations] agent answer prompt, for a.
func (w *Workspace) automationAsk(a *automation.Automation, prompt string) (string, error) {
	pick, h, err := Roles(w.Config()).ForAutomation()
	if err != nil {
		return "", err
	}
	run := harness.ConverseRun{Prompt: prompt, Dir: w.Root(), Model: pick.Model, Alone: true, Once: true}
	rep, err := RunHeadless(w.life(), w.Root(), "automation "+a.Name, spend.For{Kind: spend.Automation, Subject: a.Name}, crew.Env(w.Root(), "", ""), "", askTurnFor, h, run)
	if errors.Is(err, turn.ErrEnded) {
		return "", automation.ErrAskAgain
	}
	if err != nil {
		return "", err
	}
	return rep.Output, nil
}

// automationSpent is how many tokens name's asks have spent since.
func (w *Workspace) automationSpent(name string, since time.Time) (int64, error) {
	return spend.Tokens(w.Root(), spend.Automation, name, since)
}

// automationPropose puts what a proposes in a conversation of its own
// (in, one it's proposed in this run; "" for a new one, chain the
// automations that led to it), and tells you: it says the proposal's ref.
func (w *Workspace) automationPropose(a *automation.Automation, p automation.Proposing, in string, chain []string) (string, error) {
	store := conversation.ForWorkspace(w.Root())
	now := w.now()
	if in == "" {
		body := p.Why
		if body == "" {
			body = "The automation " + a.Name + " proposes this, for you to accept or decline."
		}
		if p.On != "" {
			body += "\n\n(From the automation " + a.Name + ", on " + p.On + ".)"
		}
		c, err := store.OpenTalkChained("automation:"+a.Name, chain, conversation.Turn{By: conversation.WS, At: now, Body: body})
		if err != nil {
			return "", err
		}
		in = c.ID
		// Named for what it's for, as an agent names its conversation.
		_ = store.SetSummary(in, a.Name+" proposes "+p.Says(), "", now)
	} else if p.Why != "" {
		if _, err := store.Say(in, conversation.Turn{By: conversation.WS, At: now, Body: p.Why}); err != nil {
			return "", err
		}
	}
	prop, err := w.automationProposal(in, p, now)
	if err != nil {
		return "", err
	}
	prop.By, prop.At = conversation.WS, now
	c, err := store.Propose(in, prop)
	if err != nil {
		return "", err
	}
	ref := fmt.Sprintf("%s#%d", c.ID, len(c.Proposals))
	_, _ = notification.Add(w.Root(), notification.Notification{
		Source: notification.SourceAutomations, Kind: "automation:" + a.Name, Refs: []string{c.ID}, By: events.ByWorkflow(a.Name),
		Text: a.Name + " proposes " + prop.Says() + ": yours to accept or decline",
	})
	return ref, nil
}

// automationProposal is the proposal p is: a mission or a decision as
// ws records them, or a plugin's kind, drafted by the plugin as for a
// conversation's agent.
func (w *Workspace) automationProposal(in string, p automation.Proposing, now time.Time) (conversation.Proposal, error) {
	str := func(k string) string { s, _ := p.Input[k].(string); return strings.TrimSpace(s) }
	switch p.Kind {
	case "mission":
		var about []string
		if l, ok := p.Input["about"].([]any); ok {
			for _, x := range l {
				if s, ok := x.(string); ok && s != "" {
					about = append(about, s)
				}
			}
		}
		return conversation.Proposal{Kind: conversation.ProposeMission, Title: str("title"), Brief: str("brief"), About: about}, nil
	case "decision":
		return conversation.Proposal{Kind: conversation.ProposeDecision, What: str("what"), Why: str("why")}, nil
	}
	input, err := json.Marshal(p.Input)
	if err != nil {
		return conversation.Proposal{}, err
	}
	guide, refused, prop, err := DraftPlugin(w.Root(), "", in, p.Kind, input, now)
	switch {
	case err != nil:
		return prop, err
	case refused != "":
		return prop, fmt.Errorf("%s refused it: %s", p.Kind, refused)
	case guide != "":
		return prop, fmt.Errorf("%s can't read what it was given. How one's written:\n%s", p.Kind, guide)
	}
	return prop, nil
}

// automationProposalState is how a proposal stands, by its ref (w12#1).
func (w *Workspace) automationProposalState(ref string) (state, result string, ok bool) {
	id, nth, found := strings.Cut(ref, "#")
	n, err := strconv.Atoi(nth)
	if !found || err != nil {
		return "", "", false
	}
	c, err := conversation.ForWorkspace(w.Root()).Load(id)
	if err != nil || n < 1 || n > len(c.Proposals) {
		return "", "", false
	}
	p := c.Proposals[n-1]
	return p.State, p.Result, true
}
