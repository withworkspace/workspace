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
// it: its marks included), read as its page and ws mission show read it
// (ops.MissionSnapshot), with the planner's conversation as
// ops.DocThreadCourt gives it.

// handoffMoment is one moment of a handoff.
type handoffMoment struct {
	name string
	// ws records what ws records as this moment comes: its marks (crew
	// being started, the planner's turn), and what the mission's come to.
	ws func()
	// kind is the kind of headline the mission's is, at this moment.
	kind Kind
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
	// planning and planned are the planner's turn in its conversation, t1,
	// as ws marks it: asked for, and over with its reply recorded.
	planning := func(t *testing.T, s Store, slug string) {
		t.Helper()
		_, err := s.StartPlanning(slug, "t1")
		must(t, err)
	}
	planned := func(t *testing.T, s Store, slug string) {
		t.Helper()
		must(t, s.EndPlanning(slug, "t1"))
	}
	// starting is ws beginning to start crew, for what.
	starting := func(t *testing.T, s Store, slug, what string) {
		t.Helper()
		_, err := s.StartLaunching(slug, what)
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
				{name: "the planner's turn: it's briefing it, writing the brief and its plan", kind: KindBriefing, ws: func() {
					planning(t, s, slug)
				}},
				{name: "you accepted them: ws is starting it, lifting its capsules, before it's confirmed", kind: KindLaunching, ws: func() {
					planned(t, s, slug) // its turn was over, the brief yours to confirm
					must(t, s.WriteBrief(slug, "## Goal\nThe totals are right.\n"))
					must(t, s.WritePlan(slug, handoffBriefPlan))
					starting(t, s, slug, "step 1")
				}},
				{name: "it's confirmed: ws is starting step 1's crew", kind: KindLaunching, ws: func() {
					must(t, s.Confirm(slug, "you"))
				}},
				{name: "crew are recorded, their session starting", kind: KindLaunching, ws: func() {
					launch(t, s, slug, "c1", "Fix it, the test passing.", 1)
				}},
				{name: "the planner's told what you confirmed, and answers; crew's session still starting", kind: KindBriefing, ws: func() {
					planning(t, s, slug)
				}},
				{name: "ws has handed crew their prompt; the planner's still answering", kind: KindWorking, ws: func() {
					must(t, s.CrewStarted(slug, "c1"))
				}},
				{name: "crew's first activity, and the planner's reply recorded: crew are at work", kind: KindWorking, ws: func() {
					must(t, s.CrewHook(slug, "c1", Hook{Prompted: true}))
					planned(t, s, slug)
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
				{name: "the planner's turn, at its step", kind: KindBriefing, ws: func() {
					planning(t, s, slug)
				}},
				{name: "its revision accepted: ws has passed its step, and is starting step 3's crew", kind: KindLaunching, ws: func() {
					planned(t, s, slug) // its turn was over, the revision yours to confirm
					must(t, s.WritePlan(slug, handoffPlanned))
					_, _, err := s.AcceptStage(slug, "planner", "the user accepted the planner's revision")
					must(t, err)
					starting(t, s, slug, "step 3")
				}},
				{name: "crew are recorded, their session starting", kind: KindLaunching, ws: func() {
					launch(t, s, slug, "c2", "Fix it, the test passing.", 3)
				}},
				{name: "the planner's told what you accepted, and answers; crew's session still starting", kind: KindBriefing, ws: func() {
					planning(t, s, slug)
				}},
				{name: "crew's first activity, before ws says it's handed them their prompt", kind: KindWorking, ws: func() {
					must(t, s.CrewHook(slug, "c2", Hook{Prompted: true}))
				}},
				{name: "the planner's reply recorded: crew are at work", kind: KindWorking, ws: func() {
					planned(t, s, slug)
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
				snap := handoffSnapshot(t, s, slug)
				hs := Resolve(snap)
				h := hs[0]
				switch {
				case idleFallback(snap, hs):
					t.Errorf("%s: resolves to the idle fallback, %q (%s, offering %q); want it in the agents' court",
						mo.name, h.Text, h.Kind, h.Action.Label)
				case h.Court != CourtAgent:
					t.Errorf("%s: in the court of %q, %q (%s); want it in the agents' court", mo.name, h.Court, h.Text, h.Kind)
				case h.Kind != mo.kind:
					t.Errorf("%s: %q (%s); want %s", mo.name, h.Text, h.Kind, mo.kind)
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
			// The handoff over, ws's marks are gone: crew's own activity
			// says where they stand from here.
			m, err := s.Load(slug)
			must(t, err)
			if m.Launching != nil || m.Planning != nil {
				t.Errorf("marks left after the handoff: launching %+v, planning %+v", m.Launching, m.Planning)
			}
			for _, c := range m.Crew {
				if c.Starting != nil {
					t.Errorf("%s is still starting after its first activity", c.ID)
				}
			}
		})
	}
}

// handoffSnapshot is the mission as recorded now, as ops.MissionSnapshot
// gathers it for Resolve: nothing in it but what the records hold, with
// its planner's conversation, t1, at work while its turn is marked on the
// mission, as ops.DocThreadCourt has it.
func handoffSnapshot(t *testing.T, s Store, slug string) Snapshot {
	t.Helper()
	m, err := s.Load(slug)
	if err != nil {
		t.Fatal(err)
	}
	brief, _ := s.ReadBrief(slug)
	state := DeriveState(m, func(Capsule) CapsuleFacts { return CapsuleFacts{} })
	return Snapshot{M: m, State: state, Brief: brief, Now: time.Now(),
		Threads: DocThreads{Briefing: &BriefingTalk{ID: "t1", Answering: m.Planning != nil && m.Planning.In == "t1"}}}
}

// idleFallback reports whether the mission resolved to what Resolve says
// when nothing applies (restingHeadline): nobody's on it, and it's yours.
func idleFallback(s Snapshot, hs []Headline) bool {
	rest := s.restingHeadline()
	return len(hs) == 1 && hs[0].Kind == rest.Kind && hs[0].Text == rest.Text
}

// ws's marks are set and cleared by what ws does, and by nothing else: a
// launch's mark stands until crew are recorded under it or it's ended; the
// crew's until they've their prompt or have done anything; the planner's
// turn until it's ended in its conversation.
func TestStore_HandoffMarks(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Cache tokens")
	slug := m.Slug
	load := func() *Mission {
		t.Helper()
		m, err := s.Load(slug)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	starting := func(id string) bool {
		c, _ := load().CrewByID(id)
		return c.Starting != nil
	}

	// One launch underway stands for what begins inside it; ending it
	// ends it, once.
	first, err := s.StartLaunching(slug, "step 2")
	if err != nil {
		t.Fatal(err)
	}
	again, _ := s.StartLaunching(slug, "")
	if !again.Since.Equal(first.Since) || again.What != "step 2" {
		t.Errorf("a launch begun inside another: %+v, want %+v", again, first)
	}
	stale := first
	stale.Since = stale.Since.Add(-time.Hour)
	s.EndLaunching(slug, stale)
	if load().Launching == nil {
		t.Fatal("another launch's end cleared this one's mark")
	}
	s.EndLaunching(slug, first)
	if load().Launching != nil {
		t.Fatal("still launching after it ended")
	}

	// Crew recorded under a mark take it over; without one, they don't.
	s.StartLaunching(slug, "")
	s.StartCrew(slug, "claude", "conv")
	if load().Launching != nil || !starting("c1") {
		t.Errorf("recorded under the mark: launching %+v, c1 starting %v", load().Launching, starting("c1"))
	}
	s.StartCrew(slug, "claude", "conv")
	if starting("c2") {
		t.Error("c2 is starting, with no launch marked")
	}

	// What ends a crew's starting: ws handing them their prompt, anything
	// they report, a status, their end. Their session id isn't activity.
	for name, happen := range map[string]func(id string){
		"handed its prompt": func(id string) { s.CrewStarted(slug, id) },
		"prompted":          func(id string) { s.CrewHook(slug, id, Hook{Prompted: true}) },
		"a permission":      func(id string) { s.CrewHook(slug, id, Hook{Activity: ActivityPermission}) },
		"reported done":     func(id string) { s.SetCrewStatus(slug, id, CrewDone, "built it") },
		"ended":             func(id string) { s.EndCrew(slug, id, "exit 1") },
	} {
		s.StartLaunching(slug, "")
		c, _ := s.StartCrew(slug, "claude", "")
		s.CrewHook(slug, c.ID, Hook{Conversation: "abc"})
		if !starting(c.ID) {
			t.Errorf("%s: %s isn't starting once its session has an id", name, c.ID)
		}
		happen(c.ID)
		if starting(c.ID) {
			t.Errorf("%s: %s is still starting", name, c.ID)
		}
	}

	// Resumed under a mark, crew are starting again, whatever they last did.
	s.EndCrew(slug, "c1", "exit 0")
	s.StartLaunching(slug, "")
	if _, err := s.ResumeCrew(slug, "c1"); err != nil {
		t.Fatal(err)
	}
	if load().Launching != nil || !starting("c1") {
		t.Errorf("resumed under the mark: launching %+v, c1 starting %v", load().Launching, starting("c1"))
	}

	// The planner's turn: one underway in a conversation stands, and only
	// that conversation's end ends it. A rename ends it: its run can't
	// record a reply under the new slug.
	turn, _ := s.StartPlanning(slug, "t1")
	if same, _ := s.StartPlanning(slug, "t1"); !same.Since.Equal(turn.Since) {
		t.Errorf("asked again while underway: %+v, want %+v", same, turn)
	}
	s.EndPlanning(slug, "t2")
	if p := load().Planning; p == nil || p.In != "t1" {
		t.Fatalf("another conversation's end cleared it: %+v", p)
	}
	s.EndPlanning(slug, "t1")
	if load().Planning != nil {
		t.Error("still planning after its turn ended")
	}
	s.StartPlanning(slug, "t1")
	if err := s.Rename(slug, "cache-all-tokens"); err != nil {
		t.Fatal(err)
	}
	slug = "cache-all-tokens"
	if load().Planning != nil {
		t.Error("the planner's turn followed the mission to its new slug")
	}
}
