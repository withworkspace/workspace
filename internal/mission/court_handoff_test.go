package mission

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A handoff is ws passing the work from one agent to the next: the
// planner's turn ends, ws starts crew, crew get to work. The mission is in
// the agents' court all the way: at no moment is it nobody's, with crew
// for you to launch.
//
// Each walk goes through a handoff a moment at a time. A moment is the
// mission as ws has recorded it by then (through the Store, as ws records
// it), read as its page and ws mission show read it (ops.MissionSnapshot),
// with the planner's conversation as ops.DocThreadCourt gives it.

// handoffMoment is one moment of a handoff.
type handoffMoment struct {
	name string
	// ws records what ws records as this moment comes: nil where it
	// records nothing.
	ws func()
	// planner is whether the planner's (or, briefing, the briefing agent's)
	// turn is marked underway in its conversation, t1.
	planner bool
}

const (
	handoffBriefPlan = "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Fix it, the test passing.\n   - who: crew\n2. Is it right?\n   - who: you\n"
	// handoffPlanned is planWithPlanner with the step the planner wrote
	// after its own.
	handoffPlanned = planWithPlanner + "3. Fix it, the test passing.\n   - who: crew\n"
)

func TestResolve_HandoffIsNeverIdle(t *testing.T) {
	must := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// launch records crew as crew.Start does: the crew, what they were
	// dispatched to do, and the step they're on.
	launch := func(t *testing.T, s Store, slug, id, text string, step int) {
		t.Helper()
		c, err := s.StartCrewOn(slug, "claude", "", "", "")
		must(t, err)
		if c.ID != id {
			t.Fatalf("started %s, want %s", c.ID, id)
		}
		must(t, s.SetCrewDispatch(slug, id, "ws", text))
		_, err = s.JoinStage(slug, id, step)
		must(t, err)
	}

	walks := []struct {
		name    string
		moments func(t *testing.T, s Store) (slug string, moments []handoffMoment)
	}{
		// You confirm a brief and its plan: ws starts step 1's crew, and
		// then tells the planner what you confirmed, which it answers.
		{name: "a brief and its plan confirmed", moments: func(t *testing.T, s Store) (string, []handoffMoment) {
			m, err := s.CreateBriefing("the totals are off")
			must(t, err)
			slug := m.Slug
			return slug, []handoffMoment{
				{name: "the planner's turn: it's briefing it, writing the brief and its plan", planner: true},
				{name: "you confirmed them: ws is starting step 1's crew", ws: func() {
					must(t, s.WriteBrief(slug, "## Goal\nThe totals are right.\n"))
					must(t, s.WritePlan(slug, handoffBriefPlan))
					must(t, s.Confirm(slug, "you"))
					// Nothing says ws is starting crew: the place of ws's mark.
				}},
				{name: "crew are recorded, their session starting", ws: func() {
					launch(t, s, slug, "c1", "Fix it, the test passing.", 1)
				}},
				{name: "the planner's told what you confirmed, and answers; crew's session still starting", planner: true},
				{name: "crew's first activity: they're at work", ws: func() {
					must(t, s.CrewHook(slug, "c1", Hook{Prompted: true}))
				}},
			}
		}},
		// Under way, at the planner's step: it proposes the steps after its
		// own, you accept them, and ws passes its step and starts the
		// next one's crew.
		{name: "the planner's step, its revision accepted", moments: func(t *testing.T, s Store) (string, []handoffMoment) {
			m, err := s.Create("Fix the totals")
			must(t, err)
			slug := m.Slug
			must(t, s.WritePlan(slug, planWithPlanner))
			launch(t, s, slug, "c1", "Find it.", 1)
			must(t, s.SetCrewStatus(slug, "c1", CrewDone, "found it: the rounding"))
			must(t, s.EndCrew(slug, "c1", "exit 0"))
			_, _, err = s.AcceptStage(slug, "orchestrator", "found")
			must(t, err)
			return slug, []handoffMoment{
				{name: "the planner's turn, at its step", planner: true},
				{name: "its revision accepted: ws has passed its step, and is starting step 3's crew", ws: func() {
					must(t, s.WritePlan(slug, handoffPlanned))
					_, _, err := s.AcceptStage(slug, "planner", "the user accepted the planner's revision")
					must(t, err)
					// Nothing says ws is starting crew: the place of ws's mark.
				}},
				{name: "crew are recorded, their session starting", ws: func() {
					launch(t, s, slug, "c2", "Fix it, the test passing.", 3)
				}},
				{name: "the planner's told what you accepted, and answers; crew's session still starting", planner: true},
				{name: "crew's first activity: they're at work", ws: func() {
					must(t, s.CrewHook(slug, "c2", Hook{Prompted: true}))
				}},
			}
		}},
	}
	for _, w := range walks {
		t.Run(w.name, func(t *testing.T) {
			s := owned(t)
			slug, moments := w.moments(t, s)
			for _, mo := range moments {
				if mo.ws != nil {
					mo.ws()
				}
				snap := handoffSnapshot(t, s, slug, mo.planner)
				hs := Resolve(snap)
				h := hs[0]
				switch {
				case idleFallback(snap, hs):
					t.Errorf("%s: resolves to the idle fallback, %q (%s, offering %q); want it in the agents' court",
						mo.name, h.Text, h.Kind, h.Action.Label)
				case h.Court != CourtAgent:
					t.Errorf("%s: in the court of %q, %q (%s); want it in the agents' court", mo.name, h.Court, h.Text, h.Kind)
				}
				// Shown, not yet held to one: every agent-court headline that
				// applies comes back, and the page stacks them.
				var agents []string
				for _, h := range hs {
					if h.Court == CourtAgent {
						agents = append(agents, fmt.Sprintf("%q (%s)", h.Text, h.Kind))
					}
				}
				if len(agents) > 1 {
					t.Logf("%s: %d agent-court headlines resolve together: %s", mo.name, len(agents), strings.Join(agents, ", "))
				}
			}
		})
	}
}

// handoffSnapshot is the mission as recorded now, as ops.MissionSnapshot
// gathers it for Resolve: nothing in it but what the records hold, and its
// planner's conversation.
func handoffSnapshot(t *testing.T, s Store, slug string, planner bool) Snapshot {
	t.Helper()
	m, err := s.Load(slug)
	if err != nil {
		t.Fatal(err)
	}
	brief, _ := s.ReadBrief(slug)
	state := DeriveState(m, func(Capsule) CapsuleFacts { return CapsuleFacts{} })
	return Snapshot{M: m, State: state, Brief: brief, Now: time.Now(),
		Threads: DocThreads{Briefing: &BriefingTalk{ID: "t1", Answering: planner}}}
}

// idleFallback reports whether the mission resolved to what Resolve says
// when nothing applies (restingHeadline): nobody's on it, and it's yours.
func idleFallback(s Snapshot, hs []Headline) bool {
	rest := s.restingHeadline()
	return len(hs) == 1 && hs[0].Kind == rest.Kind && hs[0].Text == rest.Text
}
