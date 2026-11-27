package ops

import (
	"fmt"
	"strings"

	"github.com/aymanbagabas/go-udiff"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/workspace"
)

// A settled card tells the conversation's agent. What became of what it
// proposed is written into the conversation as a turn of yours that you
// didn't type, marked as ws's (Turn.Origin, "proposal:3"): the agent reads
// it as it reads what you say, and the transcript is what it read. It's
// woken to answer at once as conversation.Wakes says; otherwise the turn
// is quiet, and read on its next turn.

// tellAgent writes the turn for proposal n of conversation id once it's
// settled, and wakes the agent if settling it does. slug is the
// conversation's mission after settling (briefing may rename it, and its
// conversations move with it), and e your edits to the proposal. Nothing's
// written for one still open: accepting it stopped before it was settled.
func (w *Workspace) tellAgent(store conversation.Store, slug, id string, n int, e ProposalEdits) {
	if !store.Workspace() && slug != "" && slug != store.Slug {
		store = conversation.For(w.Root(), slug)
	}
	c, err := store.Load(id)
	if err != nil {
		return
	}
	p := c.Proposal(n)
	if p == nil || p.State == conversation.ProposalOpen {
		return
	}
	origin := conversation.ProposalOrigin(n)
	for _, t := range c.Turns {
		if t.Origin == origin {
			return // told already, settling it at once from two places
		}
	}
	// What ws proposed (an automation's propose) has no agent to tell: the
	// turn's there for the next one you talk to, as any quiet turn is.
	t := conversation.Turn{By: conversation.You, At: w.now(), Body: settledText(*p, store.Slug, id, e),
		Origin: origin, Quiet: !conversation.Wakes(p.Kind, p.State) || p.By == conversation.WS}
	// Waking it takes the conversation up again, if it was put away, as
	// saying something would; a quiet turn leaves it as it is.
	say := store.TakeUp
	if t.Quiet {
		say = store.Say
	}
	if c, err = say(id, t); err != nil {
		execlog.Note(w.Root(), "ws", "proposal %d in %s was settled, but its turn wasn't written: %v", n, id, err)
		return
	}
	kind := job.Answer
	if conversation.IsThread(c.What) {
		kind = job.Thread
	}
	w.answered(kind, doc.OfMission(store.Slug), c)
}

// settledText is what the turn for a settled proposal says: that it was
// accepted or declined, what it was, what accepting it made (the mission,
// the decision, the document, what was written where), and what you
// changed in it before accepting. Its first line says it on its own.
func settledText(p conversation.Proposal, slug, id string, e ProposalEdits) string {
	if p.State == conversation.ProposalDeclined {
		return fmt.Sprintf("Declined proposal %d, %s.", p.N, proposalWhat(p, ProposalEdits{}))
	}
	if p.Kind == conversation.ProposePlugin {
		return pluginSettledText(p)
	}
	text := fmt.Sprintf("Accepted proposal %d, %s", p.N, proposalWhat(p, e))
	if made := proposalMade(p, slug, id); made != "" {
		text += ": " + made
	}
	return text + "." + editedText(p, e)
}

// proposalWhat names a proposal, as it was accepted (with your edits).
func proposalWhat(p conversation.Proposal, e ProposalEdits) string {
	edited := func(edit, was string) string { return firstNonEmpty(strings.TrimSpace(edit), was) }
	switch p.Kind {
	case conversation.ProposeMission:
		if strings.TrimSpace(edited(e.Brief, p.Brief)) == "" {
			return "the mission idea “" + edited(e.Title, p.Title) + "”"
		}
		return "the mission “" + edited(e.Title, p.Title) + "”"
	case conversation.ProposeDecision:
		// Quoted mid-sentence, its own full stop goes.
		return "the decision “" + strings.TrimSuffix(edited(e.What, p.What), ".") + "”"
	case conversation.ProposeDocument:
		title := p.Title
		if t := workspace.DocTitle([]byte(e.Text)); strings.TrimSpace(e.Text) != "" && t != "" {
			title = t
		}
		if p.To != "" {
			return "the edit to the document “" + title + "” (" + p.To + ")"
		}
		return "the document “" + title + "”"
	case conversation.ProposeBrief:
		if p.Plan != "" {
			return "the brief and its plan"
		}
		return "the edit to the brief"
	case conversation.ProposePlan:
		if p.Was == "" {
			return "the plan"
		}
		return "the edit to the plan"
	case conversation.ProposeDispatch:
		return "crew to start: “" + clipLine(edited(e.Text, p.Text), 80) + "”"
	case conversation.ProposeReplan:
		// Quoted mid-sentence, its own full stop goes, as a decision's does.
		return "the replan “" + strings.TrimSuffix(clipLine(edited(e.Text, p.Text), 80), ".") + "”"
	case conversation.ProposeComment:
		return fmt.Sprintf("the review comment on %s %s:%d", p.PR, p.Path, p.Line)
	case conversation.ProposeArtifact:
		if p.To != "" {
			return fmt.Sprintf("a version of %s, on v%d", p.To, p.On)
		}
		return "the artifact “" + edited(e.Title, p.Title) + "”"
	case conversation.ProposePlugin:
		return fmt.Sprintf("%s:%s “%s”", p.Plugin, p.Proposes, p.Title)
	}
	return "the " + firstNonEmpty(p.Kind, "proposal") // a kind ws no longer has
}

// pluginSettledText is what the agent's told of a plugin's proposal, once
// none of its items is open: each item, and what came of it.
func pluginSettledText(p conversation.Proposal) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Accepted proposal %d, %s", p.N, proposalWhat(p, ProposalEdits{}))
	if p.SettledBy == conversation.Agent {
		b.WriteString(", what of it was done (you withdrew the rest)")
	}
	b.WriteString(".")
	if len(p.Items) == 1 && p.Items[0].State == conversation.ItemMade {
		if p.Items[0].Made != "" {
			fmt.Fprintf(&b, " It made %s.", p.Items[0].Made)
		}
		return b.String()
	}
	b.WriteString("\n\n")
	for i, it := range p.Items {
		switch {
		case it.State == conversation.ItemMade && it.Made != "":
			fmt.Fprintf(&b, "- %d, %s: made %s\n", i+1, it.Title, it.Made)
		case it.State == conversation.ItemMade:
			fmt.Fprintf(&b, "- %d, %s: done\n", i+1, it.Title)
		default:
			fmt.Fprintf(&b, "- %d, %s: not accepted\n", i+1, it.Title)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// proposalMade is what accepting a proposal made, in words: "" for
// nothing to say.
func proposalMade(p conversation.Proposal, slug, id string) string {
	switch p.Kind {
	case conversation.ProposeMission:
		return "it's mission " + p.Result + " now"
	case conversation.ProposeDecision:
		if on, d, ok := strings.Cut(p.Result, ":"); ok {
			return "recorded as " + d + " on mission " + on
		}
		return "kept with " + id + " until it becomes a mission"
	case conversation.ProposeDocument, conversation.ProposeArtifact:
		if p.Kind == conversation.ProposeDocument && p.To != "" {
			// A new capsule's, in a repo that takes pull requests.
			return "saved to " + p.Result
		}
		return "added as " + p.Result
	case conversation.ProposeBrief, conversation.ProposePlan:
		return "written to mission " + slug
	case conversation.ProposeDispatch:
		if p.Result == "queued" {
			return "queued, for the mission's page to launch"
		}
		return "started " + p.Result
	case conversation.ProposeComment:
		return "added to the draft review of " + p.PR
	case conversation.ProposeReplan:
		return "the planner's asked, in " + p.Result + ", and revises the plan there or says why not"
	}
	if p.Result != "" {
		return "made " + p.Result
	}
	return ""
}

// editedText is what you changed in a proposal before accepting it, as
// paragraphs after the first line: each part of it you changed, a line
// as it was accepted and as proposed, text of more lines as a diff from
// the proposal. "" for none.
func editedText(p conversation.Proposal, e ProposalEdits) string {
	var b strings.Builder
	for _, f := range editable(p, e) {
		accepted, proposed := strings.TrimSpace(f.accepted), strings.TrimSpace(f.proposed)
		if accepted == "" || accepted == proposed {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("\n\nEdited before accepting.")
		}
		if !strings.Contains(accepted, "\n") && !strings.Contains(proposed, "\n") {
			fmt.Fprintf(&b, "\n\n%s: “%s” (proposed: “%s”).", upperFirst(f.name), accepted, proposed)
			continue
		}
		diff := udiff.Unified("proposed", "accepted", proposed+"\n", accepted+"\n")
		fence := codeFence(diff)
		fmt.Fprintf(&b, "\n\n%s, against what was proposed:\n\n%sdiff\n%s%s", upperFirst(f.name), fence, diff, fence)
	}
	return b.String()
}

// editField is a part of a proposal you can edit before accepting it: as
// proposed, and as you had it.
type editField struct{ name, proposed, accepted string }

// editable is the parts of a proposal of its kind that accepting it takes
// your edits to.
func editable(p conversation.Proposal, e ProposalEdits) []editField {
	switch p.Kind {
	case conversation.ProposeDecision:
		return []editField{{"the decision", p.What, e.What}, {"why", p.Why, e.Why}}
	case conversation.ProposeMission, conversation.ProposeBrief:
		return []editField{{"the title", p.Title, e.Title}, {"the brief", p.Brief, e.Brief}}
	case conversation.ProposePlan:
		return []editField{{"the plan", p.Plan, e.Plan}}
	case conversation.ProposeDispatch:
		return []editField{{"what the crew's to do", p.Text, e.Text}}
	case conversation.ProposeReplan:
		return []editField{{"what to change", p.Text, e.Text}}
	case conversation.ProposeComment:
		return []editField{{"the comment", p.Text, e.Text}}
	case conversation.ProposeDocument:
		return []editField{{"the document", p.Text, e.Text}}
	case conversation.ProposeArtifact:
		return []editField{{"the title", p.Title, e.Title}, {"its note", p.Note, e.Note}}
	}
	return nil
}

// codeFence is a fence for a code block holding text: longer than any run
// of backticks in it.
func codeFence(text string) string {
	longest, run := 0, 0
	for _, r := range text {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}
