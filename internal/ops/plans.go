package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/process"
	"github.com/withworkspace/workspace/internal/prompt"
)

// The plan: a mission's route from A to Z (plan.md beside the brief),
// written by the planner (briefing's agent) from the brief and the
// workspace's process, and confirmed by you with the brief. Its steps are
// the mission's route in place of the brief's stages: ws moves it along
// them (stages.go), and the orchestrator is given it.

// PlanSet is a mission's plan written.
type PlanSet struct {
	Slug     string
	Plan     *mission.Plan
	Warnings []string
	// Revision is how it changed the plan it replaced, once the mission's
	// briefed (Revised): what's behind the mission stayed as it was.
	Revised  bool
	Revision mission.Revision
}

func (p PlanSet) String() string {
	if p.Revised {
		return "Revised the plan: " + p.Revision.Summary() + "."
	}
	return fmt.Sprintf("Wrote the plan: %d %s.", len(p.Plan.Steps), plural(len(p.Plan.Steps), "step"))
}

// planQuotes holds a plan's quotes to the process, as it's written now.
func (w *Workspace) planQuotes() mission.PlanQuotes {
	text := w.ProcessText()
	if strings.TrimSpace(text) == "" {
		return func(string) string {
			return "the workspace has no process to quote (PROCESS.md): leave the quote out"
		}
	}
	plain := process.Plain(text)
	return func(quote string) string {
		if process.Says(plain, quote) {
			return ""
		}
		return "isn't in the process: quote its words as they're written there (ws show process has it), or leave the quote out"
	}
}

// CheckPlan reads a plan as it's written, against the process as it's
// written now, and refuses one that isn't right with every problem and its
// line.
func (w *Workspace) CheckPlan(text string) (*mission.Plan, error) {
	if strings.TrimSpace(text) == "" {
		return nil, invalid("the plan is empty")
	}
	p, err := mission.ParsePlan(text, w.planQuotes())
	if err != nil {
		return nil, &Invalid{Reason: err.Error()}
	}
	if err := w.planPicks(p); err != nil {
		return nil, err
	}
	return p, nil
}

// SetPlan replaces a mission's plan, refused with every problem and its
// line when it isn't right. It's yours, and the planner's while it's
// briefing the mission; crew and the orchestrator follow it, and never
// change it.
func (w *Workspace) SetPlan(slug, text string, by Actor) (PlanSet, error) {
	m, err := w.load(slug)
	if err != nil {
		return PlanSet{}, err
	}
	switch {
	case by.Kind == Orchestrator:
		return PlanSet{}, refused("plan-is-the-planners", "the orchestrator moves the mission along its plan and never changes it: when the route doesn't fit, flag the user")
	case by.Kind == Crew && !m.ToBrief():
		return PlanSet{}, refused("plan-is-the-planners", "the plan is the planner's and the user's once a mission is briefed: say what doesn't fit in your handoff, or ask the user")
	}
	p, err := w.CheckPlan(text)
	if err != nil {
		return PlanSet{}, err
	}
	logged := fmt.Sprintf("plan written: %d %s", len(p.Steps), plural(len(p.Steps), "step"))
	var rev mission.Revision
	revised := m.Plan != nil && !m.ToBrief()
	if revised {
		// A revision: what's behind the mission stays as it was.
		brief, _ := w.Missions.ReadBrief(m.Slug)
		states, _ := m.StageProgress(brief)
		rev = mission.Revise(m.Plan, p, states)
		if prob := rev.Problem(); prob != "" {
			return PlanSet{}, refused("steps-behind", "%s: change only what's after them", prob)
		}
		logged = "plan revised: " + rev.Summary()
	}
	if err := w.Missions.WritePlan(m.Slug, p.Text); err != nil {
		return PlanSet{}, err
	}
	w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventNote, Crew: by.By(), Text: logged})
	w.youActed(m.Slug, by)
	return PlanSet{Slug: m.Slug, Plan: p, Warnings: w.PlanWarnings(m, p), Revised: revised, Revision: rev}, nil
}

// PlanProposal is what a conversation's agent proposing a plan proposes,
// checked against the process: with a brief it proposed still open (the
// brief a mission's briefed with, or a new goal for one under way), that
// brief again with this plan, so you confirm the plan with the brief;
// otherwise an edit to the plan.
func (w *Workspace) PlanProposal(store conversation.Store, id, text string) (conversation.Proposal, error) {
	if store.Workspace() {
		return conversation.Proposal{}, invalid("a plan is a mission's: this conversation is the workspace's")
	}
	p, err := w.CheckPlan(text)
	if err != nil {
		return conversation.Proposal{}, err
	}
	was, _ := w.Missions.ReadPlan(store.Slug)
	c, err := store.Load(id)
	if err != nil {
		return conversation.Proposal{}, err
	}
	// With a brief proposed in the conversation and still open (the brief
	// a mission's briefed with, or a new goal for one under way), the plan
	// comes with it: you confirm both together.
	if b := c.LatestOpen(conversation.ProposeBrief); b != nil {
		out := *b
		out.N, out.State, out.Result, out.SettledBy, out.SettledAt = 0, "", "", "", nil
		out.Plan, out.PlanWas = p.Text, was
		return out, nil
	}
	// A mission still to brief whose brief is written (confirmed with it
	// when it was proposed): the plan comes with that brief, as it is, so
	// confirming them starts it.
	if m, err := w.Missions.Load(store.Slug); err == nil && m.ToBrief() {
		if brief, _ := w.Missions.ReadBrief(m.Slug); !mission.IsUnwrittenBrief(brief, m.Title) {
			return conversation.Proposal{Kind: conversation.ProposeBrief, Title: m.Title, Brief: brief, Was: brief, Plan: p.Text, PlanWas: was}, nil
		}
	}
	return conversation.Proposal{Kind: conversation.ProposePlan, Plan: p.Text, Was: was}, nil
}

// WithPlan gives a brief proposed in a mission's conversation the plan
// proposed before it there and still open, so the two are confirmed
// together, and proposing the brief again keeps its plan.
func (w *Workspace) WithPlan(store conversation.Store, id string, p *conversation.Proposal) {
	if store.Workspace() {
		return
	}
	c, err := store.Load(id)
	if err != nil {
		return
	}
	if plan := c.LatestPlan(); plan != "" {
		p.Plan = plan
		p.PlanWas, _ = w.Missions.ReadPlan(store.Slug)
	}
}

// acceptPlan writes a proposed plan, if the plan is still what it was
// when it was proposed, and it reads (as you edited it, perhaps).
func (w *Workspace) acceptPlan(store conversation.Store, c *conversation.Conversation, p conversation.Proposal, edits ProposalEdits) (string, error) {
	p.Plan = firstNonEmpty(edits.Plan, p.Plan)
	m, err := w.Missions.Load(store.Slug)
	if err != nil {
		return "", err
	}
	if now, _ := w.Missions.ReadPlan(m.Slug); now != p.Was {
		return "", refused("changed", "the plan has changed since this was proposed: ask in %s for it again, from the plan as it is now", c.ID)
	}
	set, err := w.SetPlan(m.Slug, p.Plan, AsYou)
	if err != nil {
		return "", err
	}
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, "plan", conversation.You, time.Now()); err != nil {
		return "", err
	}
	said := []string{set.String()}
	warnings := set.Warnings
	// Its brief written (confirmed with it when it was proposed), the plan
	// is all a mission to brief waited for: accepting it is the go-ahead.
	start := false
	if m.ToBrief() {
		brief, _ := w.Missions.ReadBrief(m.Slug)
		start = !mission.IsUnwrittenBrief(brief, m.Title)
	}
	if start {
		// The go-ahead: ws is starting it from here, through lifting its
		// capsules.
		defer w.launchingOnConfirm(m.Slug)(m.Slug)
	}
	if !m.ToBrief() || start {
		// Accepting it, as accepting a brief, lifts the capsules it names.
		if lifted := w.liftForPlan(m.Slug, set.Plan, set.Revision.Fixed); len(lifted) > 0 {
			said = append(said, strings.ToUpper(lifted[0][:1])+strings.Join(lifted, "; ")[1:]+".")
			if m, err := w.Missions.Load(m.Slug); err == nil {
				warnings = w.PlanWarnings(m, set.Plan)
			}
		}
	}
	if p.Step > 0 {
		// The planner wrote it at its own step: it's done that step.
		if next := w.passPlannerStep(m.Slug, p.Step, "the user accepted the planner's revision"); next != "" {
			said = append(said, strings.ToUpper(next[:1])+next[1:]+".")
		}
	}
	switch {
	case start:
		started, err := w.ConfirmBrief(m.Slug, AsYou)
		if err != nil {
			return strings.Join(said, " "), err
		}
		said = append(said, upperFirst(sentence(started)))
	case m.ToBrief():
		said = append(said, "It's still being briefed: confirming its brief makes it ready.")
	}
	if len(warnings) > 0 {
		said = append(said, strings.Join(warnings, "; ")+".")
	}
	return strings.Join(said, " "), nil
}

// planning is the process the planner works from, and what ws holds
// whatever a plan says, as its prompt has them.
func (w *Workspace) planning() prompt.Planning {
	p := prompt.Planning{Text: w.ProcessText()}
	for _, g := range w.Guarantees() {
		p.Guarantees = append(p.Guarantees, g.What)
	}
	return p
}

// PlanFor is a mission's plan as written, or "".
func (w *Workspace) PlanFor(slug string) string {
	text, _ := w.Missions.ReadPlan(slug)
	return text
}
