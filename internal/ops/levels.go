package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// Crew levels (mission/levels.go): a Work step runs on its level's model,
// and one whose checks come round unmet is started again a level up, by
// ws, before the orchestrator's asked: fresh crew on the next level's
// model, told what's unmet and to carry on from the handoff of the crew
// before them. A rote step, run on a small model, goes to the normal one;
// a normal one to hard, where [crew.levels] gives hard a model of its
// own. It's still a round: max_rounds holds as ever.

// crewLevel is the level crew c ran step st at: the first from the
// step's level up whose pick is what c ran on. "" when it's none of them
// (crew launched on another pick, or switched while running): then ws
// leaves the model as it is.
func crewLevel(roles harness.Roles, st *mission.Step, c mission.Crew) string {
	ran := harness.Pick{Harness: c.Harness, Model: c.Model}
	for l := st.LevelOf(); l != ""; l = mission.Above(l) {
		if roles.ForLevel(l) == ran {
			return l
		}
	}
	return ""
}

// levelUp is the next level above at whose model runs differently from
// what's run at at, and that pick; "" when no level above runs on another.
func levelUp(roles harness.Roles, at string) (string, harness.Pick) {
	now := roles.ForLevel(at)
	for l := mission.Above(at); l != ""; l = mission.Above(l) {
		if p := roles.ForLevel(l); p != now {
			return l, p
		}
	}
	return "", harness.Pick{}
}

// stepUp starts step n again a level up, its round of checks unmet, when
// it's a Work step that names no model of its own (on:), runs in one
// repo, and its crew ran at a level with one above it on another model.
// It reports whether it did, and what it did.
func (w *Workspace) stepUp(slug string, n int, round mission.CheckRound) (string, bool) {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return "", false
	}
	st, ok := currentStep(w.Missions, m)
	if !ok || st.N != n || st.Step == nil || st.Step.Type != mission.StepWork || st.Step.Model != "" || len(st.Instances()) > 0 {
		return "", false
	}
	c, ok := m.CrewByID(round.Crew)
	if !ok || c.Role != "" {
		return "", false
	}
	roles := Roles(w.Config())
	at := crewLevel(roles, st.Step, c)
	if at == "" {
		return "", false
	}
	up, pick := levelUp(roles, at)
	if up == "" {
		return "", false
	}
	capsule := c.Capsule
	if capsule == "" || capsule == RootPlace {
		if cap, ok := w.stepCapsule(m, st); ok {
			capsule = cap
		}
	}
	ran := harness.Pick{Harness: c.Harness, Model: c.Model}
	var b strings.Builder
	b.WriteString(strings.Join(strings.Fields(st.Whole()), " "))
	fmt.Fprintf(&b, "\n\n%s did this step on %s (%s), and its checks came round unmet, so ws has started it again a level up: you're on %s (%s). Read their handoff first (ws show -f llm %s), and carry on from what they did in the capsule rather than starting over.\n\n", c.ID, ran, at, pick, up, mission.HandoffRef(c.ID))
	b.WriteString(sendBackText(st, round))
	text := b.String()
	note := fmt.Sprintf("step %d up a level, its checks unmet: %s (%s) to %s (%s)", n, at, ran, up, pick)
	if w.opts.Host == nil || !w.Config().OrchestratorAutoDispatch() {
		if err := w.Missions.QueueDispatch(slug, &mission.Dispatch{Text: text, Capsule: capsule, By: "ws", Stage: n, Harness: pick.Harness, Model: pick.Model}); err != nil {
			return "", false
		}
		logFailed(w.Missions.AppendLog(slug, mission.Event{Kind: mission.EventNote, Crew: "ws", Text: note}))
		return fmt.Sprintf("step %d's checks aren't all met (%s): new crew a level up (%s, on %s) queued in %s, ready to launch", n, round.Summary(), up, pick, capsule), true
	}
	l, err := w.Launch(slug, Dispatch{Capsule: capsule, Shared: true, Text: text, Stage: n, Harness: pick.Harness, Model: pick.Model}, AsWS)
	var r *Refused
	switch {
	case errors.As(err, &r) && r.Code == "dispatch-limit":
		return "", false
	case err != nil:
		return "", false
	}
	logFailed(w.Missions.AppendLog(slug, mission.Event{Kind: mission.EventNote, Crew: "ws", Text: note}))
	if c.Ended == nil {
		// One crew at the step: the ones before hand over.
		_, _ = w.StopCrew(slug, c.ID, AsWS)
	}
	return fmt.Sprintf("step %d's checks aren't all met (%s): started %s a level up (%s, on %s) in %s, carrying on from %s's handoff", n, round.Summary(), l.ID, up, pick, l.Capsule, c.ID), true
}

// LevelSet is a step's level changed.
type LevelSet struct {
	Slug     string
	N        int
	Level    string
	Decision string // the decision it's recorded as
}

func (l LevelSet) String() string {
	return fmt.Sprintf("Step %d is %s now (%s).", l.N, l.Level, l.Decision)
}

// SetStepLevel puts step n of a mission's plan at level, yours: its
// level: written, any on: taken out, and kept as a decision of yours, so
// the planner, revising the plan, leaves it as you set it. Only a Work
// step not yet begun; rote only with a check ws runs.
func (w *Workspace) SetStepLevel(slug string, n int, level string, by Actor) (LevelSet, error) {
	if !by.IsYou() {
		return LevelSet{}, refused("not-yours", "a step's level is the planner's, in the plan, or the user's: revise the plan instead")
	}
	level = strings.ToLower(strings.TrimSpace(level))
	if !mission.IsLevel(level) {
		return LevelSet{}, invalid("%q is not a level: rote, normal or hard", level)
	}
	m, err := w.load(slug)
	if err != nil {
		return LevelSet{}, err
	}
	if m.Plan == nil {
		return LevelSet{}, refused("no-plan", "the mission has no plan")
	}
	if n < 1 || n > len(m.Plan.Steps) {
		return LevelSet{}, invalid("the plan has no step %d", n)
	}
	if fixed := w.fixedSteps(m); n <= fixed {
		return LevelSet{}, refused("steps-behind", "step %d is behind the mission (begun or passed): its crew run on what they run on. Switch their model on their page", n)
	}
	was := m.Plan.Steps[n-1].LevelOf()
	text, err := mission.WithLevel(m.Plan.Text, n, level)
	if err != nil {
		return LevelSet{}, invalid("%v", err)
	}
	if _, err := w.setPlan(slug, text, by, &m.Plan.Text); err != nil {
		return LevelSet{}, err
	}
	name := clipString(oneLine(m.Plan.Steps[n-1].Text), 60)
	brief, _ := w.Missions.ReadBrief(slug)
	if states, _ := m.StageProgress(brief); n <= len(states) {
		name = states[n-1].Short()
	}
	d, err := w.RecordDecision(slug, Deciding{
		What: fmt.Sprintf("Step %d (%s) runs at %s, not %s.", n, name, level, was),
		Why:  "Set by the user on the plan: keep it at this level when revising the plan.",
	}, by)
	if err != nil {
		return LevelSet{Slug: slug, N: n, Level: level}, nil
	}
	return LevelSet{Slug: slug, N: n, Level: level, Decision: d.Decision.ID}, nil
}
