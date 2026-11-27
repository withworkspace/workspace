package ops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The planner, through the mission: it writes the plan at briefing, and
// comes back in the same conversation (the mission's briefing
// conversation, or one opened for it) to write more of it: at a step of
// its own, when the orchestrator says the route doesn't fit, and when you
// ask. What it writes is a revision: the steps behind the mission stay as
// they were; one that removes or moves a checkpoint, or changes Z, comes
// to you as a proposal to accept; anything else applies, and the log says
// what changed.

// AskedPlanner is the planner asked to plan.
type AskedPlanner struct {
	Slug         string
	Conversation string
}

func (a AskedPlanner) String() string {
	return "Asked the planner to revise the plan, in " + a.Conversation + "."
}

// OrchestratorReplans is how many times the orchestrator may ask the
// planner at one step: more would be the two of them going round, and the
// route not fitting is then the user's to settle.
const OrchestratorReplans = 1

// AskPlanner has the mission's planner revise its plan: the orchestrator
// saying the route doesn't fit what's happened (why), or you asking for a
// change. ws says it in the planner's conversation, and its agent (the
// planner's: briefing's harness and model) answers there. Crew say what
// doesn't fit in their handoff; the orchestrator asks.
func (w *Workspace) AskPlanner(slug, why string, by Actor) (AskedPlanner, error) {
	why = strings.TrimSpace(why)
	if why == "" {
		return AskedPlanner{}, invalid("say what to change")
	}
	if by.Kind == Crew {
		return AskedPlanner{}, refused("not-yours", "the planner is asked by the orchestrator or the user: say what doesn't fit the plan in your handoff")
	}
	m, err := w.load(slug)
	if err != nil {
		return AskedPlanner{}, err
	}
	if err := plannerMay(m); err != nil {
		return AskedPlanner{}, err
	}
	at := 0
	if cur, ok := currentStep(w.Missions, m); ok {
		at = cur.N
	}
	ask := why
	if by.Kind == Orchestrator {
		if cur, ok := currentStep(w.Missions, m); ok && cur.Step.Who == mission.WhoPlanner {
			return AskedPlanner{}, refused("planner-step", "step %d is the planner's: it's been asked to write the steps after it already. Note that it waits for the planner", cur.N)
		}
		if p := conversation.For(w.Root(), m.Slug).Planner(); p != nil && (p.Answering || p.NeedsAnswer()) {
			return AskedPlanner{}, refused("planner-busy", "the planner is at work on the plan already, in %s: ws runs you again once it's revised it", p.ID)
		}
		if n := w.replansAt(m.Slug, at); n >= OrchestratorReplans {
			return AskedPlanner{}, refused("asked-already", "you've asked the planner at step %d already, and the plan still doesn't fit: flag the user, saying what doesn't fit", at)
		}
		ask = fmt.Sprintf("The orchestrator says the plan doesn't fit what's happened, at step %d: %s\n\nLook at where the mission stands and revise what's ahead, or say why the plan fits as it is.", at, why)
	}
	c, err := w.askPlanner(m, ask, by)
	if err != nil {
		return AskedPlanner{}, err
	}
	w.Missions.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Crew: by.By(), Text: fmt.Sprintf("asked the planner at step %d, in %s: %s", at, c.ID, oneLine(why))})
	w.youActed(m.Slug, by)
	return AskedPlanner{Slug: m.Slug, Conversation: c.ID}, nil
}

// MayAskPlanner refuses asking the mission's planner when it can't be
// asked: a conversation's agent proposing a replan is told so then, not
// you on confirming it.
func (w *Workspace) MayAskPlanner(slug string) error {
	m, err := w.load(slug)
	if err != nil {
		return err
	}
	return plannerMay(m)
}

// plannerMay refuses running the planner on a mission it can't plan: one
// being briefed (its planner is briefing it), ended, or without a plan.
func plannerMay(m *mission.Mission) error {
	switch {
	case m.ToBrief():
		return refused("briefing", "still being briefed: the planner writes the plan with the brief")
	case m.Ending != nil || m.State == mission.Archived:
		return refused("ended", "the mission has ended: reopen it first (ws mission reopen)")
	case m.Plan == nil:
		return refused("no-plan", "no plan to revise: the mission works from its brief's stages (ws mission plan --set - adds one)")
	}
	return nil
}

// replansAt is how many times the orchestrator has asked the planner at
// step n, as the log says.
func (w *Workspace) replansAt(slug string, n int) int {
	events, _ := w.Missions.ReadLog(slug, 0)
	prefix := fmt.Sprintf("asked the planner at step %d,", n)
	count := 0
	for _, e := range events {
		if e.Crew == mission.RoleOrchestrator && strings.HasPrefix(e.Text, prefix) {
			count++
		}
	}
	return count
}

// askPlanner says what's asked of the planner in its conversation (the
// mission's briefing conversation, carried on; or one opened for it), and
// has its agent answer. ws asking is a turn of ws's that it answers; you
// asking, yours.
func (w *Workspace) askPlanner(m *mission.Mission, ask string, by Actor) (*conversation.Conversation, error) {
	store := conversation.For(w.Root(), m.Slug)
	who := conversation.WS
	if by.IsYou() {
		who = conversation.You
	}
	var c *conversation.Conversation
	var err error
	switch p := store.Planner(); {
	case p == nil:
		c, err = store.OpenPlanner(who, ask, w.now())
	case who == conversation.You:
		c, err = store.TakeUp(p.ID, conversation.Turn{By: conversation.You, At: w.now(), Body: ask})
	default:
		c, err = store.AskAgent(p.ID, ask, w.now())
	}
	if err != nil {
		return nil, err
	}
	w.answer(job.Answer, doc.OfMission(m.Slug), c.ID)
	return c, nil
}

// plannerAtStep asks the planner to write the steps after its own: the
// mission has reached it. It says what it did.
func (w *Workspace) plannerAtStep(m *mission.Mission, done string, st mission.StageState) (string, error) {
	ask := fmt.Sprintf("%sThe mission has reached step %d, yours: %s\n\nWrite the steps after it, from what the steps before it made, and write the plan with them. Once you have, ws passes this step and moves on.", done, st.N, sentence(st.Plain()))
	if st.Step.Makes != "" {
		ask += fmt.Sprintf(" It makes: %s.", strings.TrimSuffix(st.Step.Makes, "."))
	}
	c, err := w.askPlanner(m, ask, AsWS)
	if err != nil {
		return "", err
	}
	w.Missions.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Crew: "ws", Text: fmt.Sprintf("asked the planner at step %d, in %s: write the steps after it", st.N, c.ID)})
	return fmt.Sprintf("step %d is the planner's: asked in %s to write the next steps, then the mission moves on", st.N, c.ID), nil
}

// Revised is a plan the planner wrote again: applied (and, at its own
// step, the step passed), or proposed to you.
type Revised struct {
	Slug     string
	Revision mission.Revision
	// Proposed is the proposal it became in the planner's conversation,
	// for you to accept; 0 when it applied.
	Proposed     int
	Conversation string
	// Next is what ws did after it applied: passed the planner's step and
	// reached the next, or woke the orchestrator.
	Next     string
	Warnings []string
	// Lifted is the capsules lifted for steps that name one the mission
	// hadn't got.
	Lifted []string
}

func (r Revised) String() string {
	if r.Proposed > 0 {
		return fmt.Sprintf("Proposed to the user (proposal %d in %s): %s They accept it, or not.", r.Proposed, r.Conversation, r.Revision.Why())
	}
	said := "Revised the plan: " + r.Revision.Summary() + "."
	if len(r.Lifted) > 0 {
		said += " " + strings.ToUpper(r.Lifted[0][:1]) + strings.Join(r.Lifted, "; ")[1:] + "."
	}
	if r.Next != "" {
		said += " " + strings.ToUpper(r.Next[:1]) + r.Next[1:] + "."
	}
	return said
}

// IsPlannerTalk reports whether a mission's conversation is its planner's,
// once it's briefed with a plan: what's written there is the planner
// revising it (RevisePlan).
func (w *Workspace) IsPlannerTalk(slug, id string) bool {
	m, err := w.load(slug)
	if err != nil || m.ToBrief() || m.Plan == nil {
		return false
	}
	c, err := conversation.For(w.Root(), m.Slug).Load(id)
	return err == nil && c.Goal == conversation.GoalBrief
}

// RevisePlan is the planner writing the plan again, in its conversation
// (id), once the mission's briefed. ws reads it strictly, and against how
// far the mission's got: one that changes a step behind the mission is
// refused; one that removes or moves a checkpoint, or changes Z, is
// proposed to you there; anything else applies, and the log says what
// changed. Written at the planner's own step, it passes that step, and ws
// reaches the next; elsewhere, the orchestrator looks at the plan as it
// is now.
func (w *Workspace) RevisePlan(slug, id, text string) (Revised, error) {
	m, err := w.load(slug)
	if err != nil {
		return Revised{}, err
	}
	if !w.IsPlannerTalk(m.Slug, id) {
		return Revised{}, refused("plan-is-the-planners", "the plan is revised by the planner, in its conversation, and by the user")
	}
	if err := plannerMay(m); err != nil {
		return Revised{}, err
	}
	p, err := w.CheckPlan(text)
	if err != nil {
		return Revised{}, err
	}
	if err := waitsSaid(p, w.fixedSteps(m)); err != nil {
		return Revised{}, err
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	rev := mission.Revise(m.Plan, p, states)
	if prob := rev.Problem(); prob != "" {
		return Revised{}, refused("steps-behind", "%s: write them as they are in ws mission plan, and change only what's after them", prob)
	}
	cur, curOK := mission.CurrentStage(states)
	atPlanner := curOK && cur.Step != nil && cur.Step.Who == mission.WhoPlanner
	out := Revised{Slug: m.Slug, Revision: rev, Conversation: id}
	if len(rev.Yours) > 0 {
		store := conversation.For(w.Root(), m.Slug)
		was, _ := w.Missions.ReadPlan(m.Slug)
		prop := conversation.Proposal{Kind: conversation.ProposePlan, By: conversation.Agent, At: w.now(), Plan: p.Text, Was: was, Why: rev.Why()}
		if atPlanner {
			prop.Step = cur.N
		}
		c, err := store.Propose(id, prop)
		if err != nil {
			return Revised{}, err
		}
		out.Proposed = len(c.Proposals)
		w.Missions.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Crew: "planner", Text: fmt.Sprintf("proposed a revision of the plan, in %s: %s (%s)", id, strings.TrimSuffix(rev.Why(), "."), rev.Summary())})
		return out, nil
	}
	if err := w.Missions.WritePlan(m.Slug, p.Text); err != nil {
		return Revised{}, err
	}
	w.Missions.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Crew: "planner", Text: "revised the plan, in " + id + ": " + rev.Summary()})
	out.Lifted = w.liftForPlan(m.Slug, p, rev.Fixed)
	if m, err := w.Missions.Load(m.Slug); err == nil {
		out.Warnings = w.PlanWarnings(m, p)
	}
	switch {
	case atPlanner:
		out.Next = w.passPlannerStep(m.Slug, cur.N, "the planner wrote the steps after it")
	case rev.Changed():
		if w.Notify(m.Slug, mission.OrchestratorEvent{Trigger: "plan"}) {
			out.Next = "the orchestrator looks at the plan as it is now"
		}
	}
	return out, nil
}

// passPlannerStep passes the planner's step n, the planner having written
// what's after it, and moves the plan on. It says what it did.
func (w *Workspace) passPlannerStep(slug string, n int, why string) string {
	return w.passStep(slug, n, "planner", why, mission.StepPlan)
}

// replanning is the plan as the planner comes back to it, once the
// mission's briefed; false while it's being briefed (the planner's
// briefing it), or it has no plan.
func (w *Workspace) replanning(m *mission.Mission) (prompt.Replanning, bool) {
	if m == nil || m.ToBrief() {
		return prompt.Replanning{}, false
	}
	m, err := w.Missions.Load(m.Slug) // as it is now, not as the run began
	if err != nil {
		return prompt.Replanning{}, false
	}
	text, _ := w.Missions.ReadPlan(m.Slug)
	if strings.TrimSpace(text) == "" {
		return prompt.Replanning{}, false
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	r := prompt.Replanning{Plan: text, Fixed: mission.FixedSteps(states)}
	if cur, ok := mission.CurrentStage(states); ok {
		r.At = fmt.Sprintf("step %d of %d, %s: %s", cur.N, len(states), stepWhose(cur), cur.Plain())
	}
	return r, true
}

// stepWhose is whose a step is, in words.
func stepWhose(st mission.StageState) string {
	if st.Step == nil {
		return "crew's"
	}
	if st.Step.Who == mission.WhoPlanner {
		return "yours (the planner's)"
	}
	return st.Step.Whose()
}

// liftForPlan lifts the capsules the plan's steps after fixed name that the
// mission hasn't got, in repos of the workspace (or joins them, lifted
// already), as accepting a brief
// lifts its capsules: crew are started there when the plan reaches them. A
// Work step that names a repo the mission has no capsule in gets one, named
// for the mission. It says what it did, or couldn't.
func (w *Workspace) liftForPlan(slug string, p *mission.Plan, fixed int) []string {
	var said []string
	seen := map[string]bool{}
	for i, st := range p.Steps {
		if i < fixed || st.Capsule == "" || seen[st.Capsule] {
			continue
		}
		seen[st.Capsule] = true
		m, err := w.Missions.Load(slug) // as it is now: a step before may have lifted one
		if err != nil {
			return said
		}
		repo, name, named := strings.Cut(st.Capsule, "/")
		r, ok := w.WS().ResolveAlias(repo)
		if !ok {
			continue
		}
		if !named {
			if st.Type != mission.StepWork || slices.ContainsFunc(m.Capsules, func(c mission.Capsule) bool { return c.Repo == r }) {
				continue
			}
			name = m.Slug
		}
		if m.HasCapsule(r, workspace.CapsuleName(name)) {
			continue
		}
		joined, err := w.liftOrJoin(slug, r, name)
		if err != nil {
			said = append(said, fmt.Sprintf("could not lift %s/%s for step %d (%v): ws lift %s %s", r, name, st.N, err, r, name))
			continue
		}
		if joined {
			said = append(said, fmt.Sprintf("joined %s/%s for step %d", r, name, st.N))
		} else {
			said = append(said, fmt.Sprintf("lifted %s/%s for step %d", r, name, st.N))
		}
	}
	return said
}
