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
	ref, prop, err := w.proposeFor(a, p, in, chain)
	if err != nil {
		return "", err
	}
	w.tellOfAutomation(a, ref, a.Name+" proposes "+prop.Says()+": accept or decline")
	return ref, nil
}

// automationAct is what a's act makes of p: a proposal as propose's, then
// accepted at once by a where how says it may be (you granted it: your
// call, whatever its plugin advises); put in front of you where you
// started the run (how.Confirm: the page you started it from shows it,
// and nothing's told); else left for you to accept, as propose's is.
func (w *Workspace) automationAct(a *automation.Automation, p automation.Proposing, how automation.Acting, in string, chain []string) (string, error) {
	ref, prop, err := w.proposeFor(a, p, in, chain)
	if err != nil {
		return "", err
	}
	id, _, _ := strings.Cut(ref, "#")
	store := conversation.ForWorkspace(w.Root())
	switch {
	case how.Confirm:
		return ref, nil
	case how.Accept:
		_, err := w.acceptUnattended(a, ref)
		if err == nil {
			w.tellOfAutomation(a, ref, a.Name+" acted without asking: "+firstNonEmpty(prop.Title, prop.Says()))
			return ref, nil
		}
		_, _ = store.Say(id, conversation.Turn{By: conversation.WS, At: w.now(), Body: "Could not do without asking: " + err.Error() + ". Accept or decline."})
		w.tellOfAutomation(a, ref, a.Name+" could not do "+prop.Says()+" ("+err.Error()+"): accept or decline")
		return ref, nil
	}
	w.tellOfAutomation(a, ref, a.Name+" proposes "+prop.Says()+": accept or decline")
	return ref, nil
}

// acceptUnattended accepts the proposal ref names, all of it, as a: it
// says what was done.
func (w *Workspace) acceptUnattended(a *automation.Automation, ref string) (string, error) {
	id, nth, _ := strings.Cut(ref, "#")
	n, _ := strconv.Atoi(nth)
	store := conversation.ForWorkspace(w.Root())
	c, err := store.Load(id)
	if err != nil {
		return "", err
	}
	p := c.Proposal(n)
	if p == nil {
		return "", fmt.Errorf("%s has no proposal %d", id, n)
	}
	said, err := w.acceptPluginBy(store, c, *p, nil, conversation.ByAutomation(a.Name))
	if err != nil {
		return "", err
	}
	_, _ = store.Say(id, conversation.Turn{By: conversation.WS, At: w.now(), Body: "Done without asking, as you let " + a.Name + " do " + p.Plugin + ":" + p.Proposes + ": " + said})
	return said, nil
}

// tellOfAutomation is a notification of a's, about the conversation ref
// is in.
func (w *Workspace) tellOfAutomation(a *automation.Automation, ref, text string) {
	id, _, _ := strings.Cut(ref, "#")
	_, _ = notification.Add(w.Root(), notification.Notification{
		Source: notification.SourceAutomations, Kind: "automation:" + a.Name, Refs: []string{id}, By: events.ByWorkflow(a.Name), Text: text,
	})
}

// proposeFor makes what a proposes a proposal, in a conversation of its
// own (in, or a new one, titled for it): its ref, and the proposal.
func (w *Workspace) proposeFor(a *automation.Automation, p automation.Proposing, in string, chain []string) (string, conversation.Proposal, error) {
	store := conversation.ForWorkspace(w.Root())
	now := w.now()
	opened := in == ""
	if in == "" {
		body := p.Why
		if body == "" {
			body = "Automation " + a.Name + " proposes this."
		}
		if p.On != "" {
			body += "\n\n(From automation " + a.Name + ", on " + p.On + ".)"
		}
		c, err := store.OpenTalkChained("automation:"+a.Name, chain, conversation.Turn{By: conversation.WS, At: now, Body: body})
		if err != nil {
			return "", conversation.Proposal{}, err
		}
		in = c.ID
	} else if p.Why != "" {
		if _, err := store.Say(in, conversation.Turn{By: conversation.WS, At: now, Body: p.Why}); err != nil {
			return "", conversation.Proposal{}, err
		}
	}
	prop, err := w.automationProposal(in, p, now)
	if err != nil {
		return "", prop, err
	}
	prop.By, prop.At = conversation.WS, now
	c, err := store.Propose(in, prop)
	if err != nil {
		return "", prop, err
	}
	if opened {
		// Named for what it's for, as an agent names its conversation: a
		// plugin's by its own title for it.
		_ = store.SetSummary(in, a.Name+": "+firstNonEmpty(prop.Title, prop.Says()), "", now)
	}
	return fmt.Sprintf("%s#%d", c.ID, len(c.Proposals)), prop, nil
}

// automationProposal is the proposal p is: a mission or a decision as
// ws records them, or a plugin's kind, drafted by the plugin as for a
// conversation's agent.
func (w *Workspace) automationProposal(in string, p automation.Proposing, now time.Time) (conversation.Proposal, error) {
	fields := p.Fields()
	str := func(k string) string { s, _ := fields[k].(string); return strings.TrimSpace(s) }
	switch p.Kind {
	case "mission":
		var about []string
		if l, ok := fields["about"].([]any); ok {
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
		return prop, fmt.Errorf("%s refused: %s", p.Kind, refused)
	case guide != "":
		return prop, fmt.Errorf("%s could not read the input. Expected format:\n%s", p.Kind, guide)
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
