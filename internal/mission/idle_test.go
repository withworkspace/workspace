package mission

import (
	"testing"
	"time"
)

// Crew are idle whose turn ended a while ago at a Work step of theirs with
// nothing said, while nobody else has the mission. Not those who've said
// where they are, are answering you, or left work running; nor any while
// ws has the mission in hand or something's asked of you.
func TestIdle(t *testing.T) {
	advance := clock(t)
	s := owned(t)
	m, _ := s.Create("Cache")
	slug := m.Slug
	if err := s.WritePlan(slug, checkedPlan); err != nil {
		t.Fatal(err)
	}
	s.StartCrew(slug, "claude", "chat-1")
	s.JoinStage(slug, "c1", 1)
	s.CrewHook(slug, "c1", Hook{Prompted: true})
	s.CrewHook(slug, "c1", Hook{Activity: ActivityWaiting, Reply: "The agents I started are writing the tests."})
	idle := func() []string {
		t.Helper()
		m, err := s.Load(slug)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, c := range m.Idle("", now(), 10*time.Minute) {
			ids = append(ids, c.ID)
		}
		return ids
	}

	advance(9 * time.Minute)
	if got := idle(); len(got) != 0 {
		t.Errorf("after 9 minutes: %v", got)
	}
	advance(2 * time.Minute)
	if got := idle(); len(got) != 1 || got[0] != "c1" {
		t.Fatalf("after 11 minutes: %v", got)
	}

	for name, moment := range map[string]struct{ set, unset func() }{
		"you're talking to them":   {func() { s.TalkedTo(slug, "c1", "") }, func() { s.Nudged(slug, "c1", "carry on", "ws") }},
		"they've asked":            {func() { s.SetCrewStatus(slug, "c1", CrewAsking, "Which?") }, func() { s.ClearStatus(slug, "c1") }},
		"they've reported done":    {func() { s.SetCrewStatus(slug, "c1", CrewDone, "cached") }, func() { s.ClearStatus(slug, "c1") }},
		"something's asked of you": {func() { s.Ask(slug, "Which cache?", "orchestrator") }, func() { s.CloseTask(slug, 1, "you", "") }},
		"it waits on something":    {func() { s.SetWaiting(slug, "a deploy", "orchestrator") }, func() { s.SetWaiting(slug, "", "orchestrator") }},
		"ws is moving it on": {func() { s.StartMoving(slug, 1) }, func() {
			m, _ := s.Load(slug)
			s.EndMoving(slug, *m.Moving)
		}},
	} {
		moment.set()
		if got := idle(); len(got) != 0 {
			t.Errorf("%s: idle %v", name, got)
		}
		moment.unset()
		if got := idle(); len(got) != 1 {
			t.Errorf("after %s: idle %v, want c1 again", name, got)
		}
	}

	// Work they left running, which ws is waiting on with them.
	s.CrewHook(slug, "c1", Hook{Prompted: true})
	s.CrewHook(slug, "c1", Hook{Activity: ActivityWaiting, Work: []Work{{What: "just test", From: "harness", Wakes: WakeNo, PID: 4242}}})
	advance(11 * time.Minute)
	if got := idle(); len(got) != 0 {
		t.Errorf("with work running: %v", got)
	}
}
