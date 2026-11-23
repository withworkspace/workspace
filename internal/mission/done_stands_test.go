package mission

import (
	"errors"
	"testing"
	"time"
)

const checkedPlan = "## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Work: cache it.\n   - check: `true` passes\n2. Review: it.\n"

// clock is the store's clock, held by the test: times are kept to the
// second, so what happens after something needs it moved on.
func clock(t *testing.T) (advance func(time.Duration)) {
	t.Helper()
	at, old := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC), now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = old })
	return func(d time.Duration) { at = at.Add(d) }
}

// That crew reported done stands through what you say to them: a message
// answers a question, and isn't more to do. ws giving them more to do
// takes it back, as it's sent.
func TestDone_StandsThroughYourMessages(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Cache")
	s.StartCrew(m.Slug, "claude", "chat-1")
	crew := func() Crew { m, _ := s.Load(m.Slug); return m.Crew[0] }
	done := func() { s.SetCrewStatus(m.Slug, "c1", CrewDone, "cached") }

	done()
	if cleared, err := s.TalkedTo(m.Slug, "c1", "why a map?"); err != nil || cleared {
		t.Fatalf("talked to: cleared %v, %v", cleared, err)
	}
	s.CrewHook(m.Slug, "c1", Hook{Prompted: true})
	if c := crew(); c.Status != CrewDone || c.Note != "cached" || !c.Conversing || !c.HandedBack(&Mission{}) {
		t.Errorf("after your message: %+v", c)
	}
	s.CrewHook(m.Slug, "c1", Hook{Activity: ActivityWaiting, Reply: "It's keyed by org."})

	if err := s.Nudged(m.Slug, "c1", "add a test for it", "orchestrator"); err != nil {
		t.Fatal(err)
	}
	if c := crew(); c.Status != "" || c.Note != "" || c.Conversing {
		t.Errorf("after ws gave it more to do: %+v", c)
	}

	// Closed since, and opened again: by you to talk to, the done stands;
	// by ws, or by you with what ws left for them, they've more to do.
	resumed := func(by string) Crew {
		t.Helper()
		if _, err := s.ResumeCrew(m.Slug, "c1", by); err != nil {
			t.Fatal(err)
		}
		return crew()
	}
	done()
	s.EndCrew(m.Slug, "c1", "exit 0")
	if c := resumed("you"); c.Status != CrewDone {
		t.Errorf("resumed by you: status %q", c.Status)
	}
	s.EndCrew(m.Slug, "c1", "exit 0")
	s.SetResumeWithBy(m.Slug, "c1", "why a map?", "you")
	if c := resumed("you"); c.Status != CrewDone {
		t.Errorf("resumed with your message: status %q", c.Status)
	}
	s.EndCrew(m.Slug, "c1", "exit 0")
	s.SetResumeWithBy(m.Slug, "c1", "carry on at step 2", "ws")
	if c := resumed("you"); c.Status != "" {
		t.Errorf("resumed by you with what ws left: status %q", c.Status)
	}
	done()
	s.EndCrew(m.Slug, "c1", "exit 0")
	if c := resumed("orchestrator"); c.Status != "" {
		t.Errorf("resumed by the orchestrator: status %q", c.Status)
	}
}

// checkedStep is a mission at a Work step with a check, its crew c1 having
// reported done and a round of its checks kept, as result.
func checkedStep(t *testing.T, s Store, result string) string {
	t.Helper()
	m, _ := s.Create("Cache")
	if err := s.WritePlan(m.Slug, checkedPlan); err != nil {
		t.Fatal(err)
	}
	s.StartCrew(m.Slug, "shell", "")
	if _, err := s.JoinStage(m.Slug, "c1", 1); err != nil {
		t.Fatal(err)
	}
	s.SetCrewStatus(m.Slug, "c1", CrewDone, "cached")
	got, _ := s.Load(m.Slug)
	if _, err := s.RecordChecks(m.Slug, 1, CheckRound{Crew: "c1", Done: got.Crew[0].DoneKey(), Results: []CheckResult{{Check: "`true` passes", Result: result, By: "ws"}}}); err != nil {
		t.Fatal(err)
	}
	return m.Slug
}

// Crew in a turn begun since they reported done hold their step for ws
// until the turn ends: what they're doing isn't in what was checked. It's
// yours to pass over them.
func TestAcceptStage_HeldWhileCrewAreInATurnSinceTheirDone(t *testing.T) {
	advance := clock(t)
	s := owned(t)
	slug := checkedStep(t, s, CheckMet)

	advance(time.Minute)
	s.TalkedTo(slug, "c1", "why a map?")
	s.CrewHook(slug, "c1", Hook{Prompted: true})
	if _, _, err := s.AcceptStage(slug, "ws", "its checks are met"); !errors.Is(err, ErrStageBusy) {
		t.Fatalf("ws passing it mid-turn: %v", err)
	}
	if _, _, err := s.AcceptStageAt(slug, 2, "ws", ""); !errors.Is(err, ErrStageMoved) {
		t.Errorf("passing a step it isn't at: %v", err)
	}

	advance(time.Minute)
	s.CrewHook(slug, "c1", Hook{Activity: ActivityWaiting, Reply: "It's keyed by org."})
	acc, _, err := s.AcceptStageAt(slug, 1, "ws", "its checks are met")
	if err != nil || acc.N != 1 || acc.Record.AcceptedBy != "ws" {
		t.Fatalf("ws passing it once the turn's over: %+v, %v", acc.Record, err)
	}

	// Mid-turn, it's still yours to pass.
	s2 := owned(t)
	slug = checkedStep(t, s2, CheckMet)
	advance(time.Minute)
	s2.CrewHook(slug, "c1", Hook{Prompted: true})
	if _, _, err := s2.AcceptStage(slug, "you", "passed"); err != nil {
		t.Errorf("you passing it mid-turn: %v", err)
	}
}

// Crew sent back after a round of checks, who've stopped without
// reporting again, don't hold their step from you: what ws asked you of
// its checks is yours to answer. They do while they're working, and from
// anyone else; and crew who never handed it over hold it from you too.
func TestAcceptStage_YoursOnceCheckedCrewHaveStopped(t *testing.T) {
	advance := clock(t)
	s := owned(t)
	slug := checkedStep(t, s, CheckUnmet)

	advance(time.Minute)
	s.Nudged(slug, "c1", "the check is unmet: put it right", "orchestrator")
	s.CrewHook(slug, "c1", Hook{Prompted: true})
	if _, _, err := s.AcceptStage(slug, "you", "passed"); !errors.Is(err, ErrStageBusy) {
		t.Fatalf("you passing it while they work: %v", err)
	}
	advance(time.Minute)
	s.CrewHook(slug, "c1", Hook{Activity: ActivityWaiting, Reply: "It was right already."})
	if _, _, err := s.AcceptStage(slug, "orchestrator", "looks done"); !errors.Is(err, ErrStageBusy) {
		t.Errorf("the orchestrator passing it: %v", err)
	}
	if _, _, err := s.AcceptStage(slug, "you", "passed"); err != nil {
		t.Errorf("you passing it once they've stopped: %v", err)
	}

	never := owned(t)
	m, _ := never.Create("Cache")
	never.WritePlan(m.Slug, checkedPlan)
	never.StartCrew(m.Slug, "shell", "")
	never.JoinStage(m.Slug, "c1", 1)
	never.CrewHook(m.Slug, "c1", Hook{Activity: ActivityWaiting})
	if _, _, err := never.AcceptStage(m.Slug, "you", "passed"); !errors.Is(err, ErrStageBusy) {
		t.Errorf("you passing a step nobody handed over: %v", err)
	}
}
