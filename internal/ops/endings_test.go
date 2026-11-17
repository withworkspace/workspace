package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

const endingBrief = "## Goal\nFind out why the total's wrong.\n\n## Acceptance\n- Product know the cause.\n- A regression test, if it's a bug.\n"

// The orchestrator suggests how a mission ends, with Z checked part by
// part; you end it; reopening takes it back.
func TestConcludeAndEnd(t *testing.T) {
	w, _ := fixture(t, endingBrief, "fix")
	as := EndAs{Ending: "answered", Text: "Not a bug: rounded as product asked", Met: []string{"1: told in the thread"}, Unmet: []string{"2: no bug, so no test"}}

	// Z checked wrong, or an ending that isn't one: refused, saying why.
	_, err := w.Conclude("cache", EndAs{Ending: "answered", Text: "x", Met: []string{"1: told"}}, AsOrchestrator)
	if err == nil || !strings.Contains(err.Error(), "part 2 of Z") {
		t.Errorf("a part of Z unchecked: %v", err)
	}
	if _, err := w.Conclude("cache", EndAs{Ending: "merged", Text: "x"}, AsOrchestrator); err == nil {
		t.Error("an ending that isn't one was taken")
	}
	if _, err := w.Conclude("cache", EndAs{Ending: "answered", Met: as.Met, Unmet: as.Unmet}, AsOrchestrator); err == nil {
		t.Error("an ending that says nothing it came to was taken")
	}
	_, err = w.Conclude("cache", EndAs{Ending: "landed", Text: "x", Met: []string{"1: a", "2: b"}}, AsOrchestrator)
	refusedWith(t, err, "not-landed") // its capsule has neither merged nor landed
	_, err = w.Conclude("cache", EndAs{Ending: "landed", Text: "x", Met: as.Met, Unmet: as.Unmet}, AsOrchestrator)
	refusedWith(t, err, "not-at-z")
	if _, err := w.Conclude("cache", EndAs{Ending: "handed-on", Text: "x", Met: as.Met, Unmet: as.Unmet}, AsOrchestrator); err == nil || !strings.Contains(err.Error(), "which missions") {
		t.Errorf("handed on to nowhere: %v", err)
	}
	if _, err := w.Conclude("cache", EndAs{Ending: "answered", Text: "x", Met: as.Met, Unmet: as.Unmet, To: []string{"cache"}}, AsOrchestrator); err == nil {
		t.Error("--to on an ending that isn't handed on was taken")
	}

	c, err := w.Conclude("cache", as, AsOrchestrator)
	if err != nil {
		t.Fatal(err)
	}
	if c.Ended || c.Conclusion.By != "orchestrator" || len(c.Conclusion.Z) != 2 || c.Conclusion.Z[1].Met {
		t.Errorf("concluded = %+v", c)
	}
	m, _ := w.Missions.Load("cache")
	if hs := mission.Resolve(mission.Snapshot{M: m, State: mission.Active}); hs[0].Kind != mission.KindWrapUp || hs[0].Action.Verb != mission.VerbEnd {
		t.Errorf("headline = %+v, want a wrap-up to end", hs[0])
	}

	// Ending it is yours.
	_, err = w.End("cache", nil, AsOrchestrator)
	refusedWith(t, err, "not-yours")
	e, err := w.End("cache", nil, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Ended || e.Conclusion.By != "you" || !strings.Contains(e.String(), "Ended, answered (early: 1 of 2 of Z met)") {
		t.Errorf("ended = %+v (%s)", e, e)
	}
	m, _ = w.Missions.Load("cache")
	if m.Ending == nil || m.Conclude != nil || mission.DeriveState(m, func(mission.Capsule) mission.CapsuleFacts { return mission.CapsuleFacts{} }) != mission.Answered {
		t.Errorf("after ending: %+v", m)
	}
	_, err = w.End("cache", &as, AsYou)
	refusedWith(t, err, "ended")
	_, err = w.SetState("cache", mission.Parked, AsYou)
	refusedWith(t, err, "ended")

	// Reopening takes the ending back.
	if _, err := w.SetState("cache", "", AsYou); err != nil {
		t.Fatal(err)
	}
	if m, _ := w.Missions.Load("cache"); m.Ending != nil {
		t.Error("reopening left the ending")
	}
	_, err = w.Unend("cache", AsYou)
	refusedWith(t, err, "not-ended")

	// Nothing suggested: you say how, or it's refused.
	_, err = w.End("cache", nil, AsYou)
	refusedWith(t, err, "nothing-suggested")
	// Dropping needn't check Z.
	if _, err := w.End("cache", &EndAs{Ending: "dropped", Text: "not worth it"}, AsYou); err != nil {
		t.Errorf("dropping: %v", err)
	}
}

// Ending a mission waits for crew at work on it.
func TestEndWaitsForCrew(t *testing.T) {
	w, _ := fixture(t, "## Goal\nx\n")
	w.Missions.Update("cache", func(m *mission.Mission) error {
		m.Crew = []mission.Crew{{ID: "c1", Started: time.Now()}}
		return nil
	})
	_, err := w.End("cache", &EndAs{Ending: "dropped", Text: "x"}, AsYou)
	refusedWith(t, err, "crew-working")
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "done")
	if _, err := w.End("cache", &EndAs{Ending: "dropped", Text: "x"}, AsYou); err != nil {
		t.Errorf("crew reported done: %v", err)
	}
}

// A suggestion made on a Z that's changed since isn't ended on.
func TestEndRefusesAChangedZ(t *testing.T) {
	w, _ := fixture(t, endingBrief)
	if _, err := w.Conclude("cache", EndAs{Ending: "answered", Text: "x", Met: []string{"1: a", "2: b"}}, AsOrchestrator); err != nil {
		t.Fatal(err)
	}
	w.Missions.WriteBrief("cache", "## Goal\nx\n\n## Acceptance\n- Something else.\n")
	_, err := w.End("cache", nil, AsYou)
	refusedWith(t, err, "z-changed")
}

// A capsule ends alone: handed on to a mission raised for it (which takes
// it), or dropped; it no longer holds the mission open, and handing a
// mission on reads where its capsules went.
func TestCapsulesEndAlone(t *testing.T) {
	w, _ := fixture(t, "## Goal\nBump the deps.\n", "web-bump", "api-bump")
	_, err := w.HandOnCapsule("cache", "frontend/web-bump", "web's bump breaks the build", "", AsCrew("c1"))
	refusedWith(t, err, "not-yours")
	_, err = w.HandOnCapsule("cache", "frontend/nope", "x", "", AsOrchestrator)
	refusedWith(t, err, "not-attached")

	h, err := w.HandOnCapsule("cache", "frontend/web-bump", "web's bump breaks the build", "", AsOrchestrator)
	if err != nil {
		t.Fatal(err)
	}
	if !h.Raised || h.To == "" {
		t.Fatalf("handed on = %+v", h)
	}
	m, _ := w.Missions.Load("cache")
	if m.HasCapsule("frontend", "web-bump") || len(m.EndedCapsules) != 1 || m.EndedCapsules[0].To != h.To {
		t.Errorf("this mission: capsules %+v, ended %+v", m.Capsules, m.EndedCapsules)
	}
	n, _ := w.Missions.Load(h.To)
	if !n.HasCapsule("frontend", "web-bump") || n.From != "cache" || !n.ToBrief() {
		t.Errorf("the mission raised for it: %+v", n)
	}
	if owner, ok := w.Missions.FindByCapsule("frontend", "web-bump"); !ok || owner.Slug != h.To {
		t.Errorf("web-bump's owner = %v", owner)
	}

	_, err = w.DropCapsule("cache", "frontend/api-bump", "not needed", AsOrchestrator)
	refusedWith(t, err, "not-yours")
	if _, err := w.DropCapsule("cache", "frontend/api-bump", "", AsYou); err == nil {
		t.Error("dropped with no why")
	}
	if _, err := w.DropCapsule("cache", "frontend/api-bump", "not needed", AsYou); err != nil {
		t.Fatal(err)
	}
	m, _ = w.Missions.Load("cache")
	if len(m.Capsules) != 0 || len(m.EndedCapsules) != 2 {
		t.Errorf("after dropping: capsules %+v, ended %+v", m.Capsules, m.EndedCapsules)
	}

	// The mission handed on: to where its capsules went.
	c, err := w.Conclude("cache", EndAs{Ending: "handed-on", Text: "web's is its own mission"}, AsOrchestrator)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Conclusion.To) != 1 || c.Conclusion.To[0] != h.To {
		t.Errorf("handed on to %v, want %s", c.Conclusion.To, h.To)
	}
}

// Crew at work in a capsule keep it from ending alone.
func TestCapsuleWithCrewInIt(t *testing.T) {
	w, _ := fixture(t, "## Goal\nx\n", "fix")
	w.Missions.Update("cache", func(m *mission.Mission) error {
		m.Crew = []mission.Crew{{ID: "c1", Capsule: "frontend/fix", Started: time.Now()}}
		return nil
	})
	_, err := w.DropCapsule("cache", "frontend/fix", "x", AsYou)
	refusedWith(t, err, "crew-working")
}

// You may put an ending suggested aside: it doesn't end yet.
func TestDismissASuggestedEnding(t *testing.T) {
	w, _ := fixture(t, "## Goal\nx\n")
	w.Conclude("cache", EndAs{Ending: "dropped", Text: "x"}, AsOrchestrator)
	d, err := w.Dismiss("cache", AsYou)
	if err != nil || d.What != "suggested ending" {
		t.Fatalf("dismissed %+v, %v", d, err)
	}
	if m, _ := w.Missions.Load("cache"); m.Conclude != nil {
		t.Error("the suggestion is still there")
	}
}

// landedFixture is a mission whose one capsule has landed, at Z once both
// parts of endingBrief's are met.
func landedFixture(t *testing.T) *Workspace {
	t.Helper()
	w, _ := fixture(t, endingBrief, "fix")
	w.Missions.Update("cache", func(m *mission.Mission) error {
		m.Capsules[0].Landed = &mission.Landing{At: time.Now(), Head: "abc123"}
		return nil
	})
	return w
}

var landedAs = EndAs{Ending: "landed", Text: "Fixed the rounding", Met: []string{"1: told in the thread", "2: TestTotal"}}

// The orchestrator suggesting a mission landed, with everything merged and
// nothing else on it waiting, ends it: ws's, not yours, unless
// end_when_merged is off. Any other ending is still yours.
func TestEndWhenMerged(t *testing.T) {
	w := landedFixture(t)
	c, err := w.Conclude("cache", landedAs, AsOrchestrator)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Ended || c.Conclusion.By != "ws" || !strings.HasPrefix(c.String(), "ws ended it, landed (at Z: 2 of 2 met)") {
		t.Errorf("concluded = %+v (%s)", c, c)
	}
	m, _ := w.Missions.Load("cache")
	if m.Ending == nil || m.Ending.Ending != mission.EndLanded || m.Ending.By != "ws" || m.Conclude != nil {
		t.Errorf("after: ending %+v, conclude %+v", m.Ending, m.Conclude)
	}

	// Off, it waits for you.
	w = landedFixture(t)
	off := false
	w.Config().Missions.EndWhenMerged = &off
	if c, err := w.Conclude("cache", landedAs, AsOrchestrator); err != nil || c.Ended {
		t.Errorf("end_when_merged off: %+v, %v", c, err)
	}

	// Crew's suggestion, and an ending that isn't landed, wait for you.
	w = landedFixture(t)
	if c, err := w.Conclude("cache", landedAs, AsCrew("c1")); err != nil || c.Ended {
		t.Errorf("crew suggested it: %+v, %v", c, err)
	}
	w = landedFixture(t)
	answered := EndAs{Ending: "answered", Text: "x", Met: landedAs.Met}
	if c, err := w.Conclude("cache", answered, AsOrchestrator); err != nil || c.Ended {
		t.Errorf("answered: %+v, %v", c, err)
	}
}

// What waits on you, or crew still at it, holds a landed mission open
// until it doesn't; then the PR watcher ends it.
func TestEndWhenMergedWaits(t *testing.T) {
	for _, hold := range []struct {
		why string
		set func(m *mission.Mission)
	}{
		{"it waits on the user for something", func(m *mission.Mission) {
			m.Tasks = []mission.Task{{N: 1, Kind: "question", Verb: "answer", Text: "Ship it?"}}
		}},
		{"c1 is still on it", func(m *mission.Mission) {
			m.Crew = []mission.Crew{{ID: "c1", Started: time.Now()}}
		}},
		{"it wrote a Plan, and whether it's built is the user's to say", func(m *mission.Mission) {
			m.Artifacts = []mission.Artifact{{Ref: "records:plans/x.md", Kind: "plan"}}
		}},
		{"it's parked", func(m *mission.Mission) { m.State = mission.Parked }},
	} {
		w := landedFixture(t)
		w.Missions.Update("cache", func(m *mission.Mission) error { hold.set(m); return nil })
		c, err := w.Conclude("cache", landedAs, AsOrchestrator)
		if err != nil || c.Ended {
			t.Errorf("%s: ended anyway: %+v, %v", hold.why, c, err)
			continue
		}
		if _, why := w.endWhenMerged("cache"); why != hold.why {
			t.Errorf("held because %q, want %q", why, hold.why)
		}
	}

	w := landedFixture(t)
	w.Missions.Update("cache", func(m *mission.Mission) error {
		m.Crew = []mission.Crew{{ID: "c1", Started: time.Now()}}
		return nil
	})
	if c, _ := w.Conclude("cache", landedAs, AsOrchestrator); c.Ended {
		t.Fatal("ended with c1 still on it")
	}
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "done")
	m, _ := w.Missions.Load("cache")
	w.WatchPRs([]MissionView{{M: m, State: mission.Active}})
	if m, _ := w.Missions.Load("cache"); m.Ending == nil || m.Ending.By != "ws" {
		t.Errorf("once c1 had reported done, the watcher didn't end it: %+v", m.Ending)
	}
}
