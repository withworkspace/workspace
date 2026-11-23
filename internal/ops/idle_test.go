package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

// turnEnded has c1's turn end, since ago.
func turnEnded(t *testing.T, w *Workspace, ago time.Duration) {
	t.Helper()
	w.Missions.CrewHook("cache", "c1", mission.Hook{Prompted: true})
	w.Missions.CrewHook("cache", "c1", mission.Hook{Activity: mission.ActivityWaiting, Reply: "The agents I started are writing the tests."})
	if _, err := w.Missions.Update("cache", func(m *mission.Mission) error {
		at := time.Now().Add(-ago)
		m.Crew[0].ActivityAt = &at
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Crew who've sat with their turn over and nothing said are asked where
// they are, once a step: after that their silence is the orchestrator's
// to look at, as after any message of ws's.
func TestNudgeIdle_AsksOnceAStep(t *testing.T) {
	w, _, _ := workFixture(t, "")
	host := &sayingHost{}
	w.opts.Host = host

	turnEnded(t, w, 5*time.Minute)
	if said := w.NudgeIdle(time.Now()); len(said) != 0 || len(host.said) != 0 {
		t.Fatalf("after 5 minutes: %v, told %v", said, host.said)
	}
	turnEnded(t, w, 12*time.Minute)
	said := w.NudgeIdle(time.Now())
	if len(said) != 1 || !strings.Contains(said[0], "asked cache/c1, idle at step 1") {
		t.Fatalf("after 12 minutes: %v", said)
	}
	if len(host.said) != 1 || !strings.Contains(host.said[0], "Your turn ended 12m0s ago, at step 1, without your reporting done or asking anything") {
		t.Fatalf("c1 was told %q", host.said)
	}
	m, _ := w.Missions.Load("cache")
	if c := m.Crew[0]; c.Conversing {
		t.Errorf("it's yours after ws asked: %+v", c)
	}

	// Silent again: not asked twice.
	turnEnded(t, w, 12*time.Minute)
	if said := w.NudgeIdle(time.Now()); len(said) != 0 || len(host.said) != 1 {
		t.Errorf("asked again: %v, told %v", said, host.said)
	}

	// With the orchestrator off, crew who've stopped are yours as they are.
	w2, _, _ := workFixture(t, "")
	w2.opts.Host = &sayingHost{}
	off := false
	w2.cur.cfg.Orchestrator.Enabled = &off
	turnEnded(t, w2, 12*time.Minute)
	if said := w2.NudgeIdle(time.Now()); len(said) != 0 {
		t.Errorf("with the orchestrator off: %v", said)
	}
}

// Your answer to a question crew asked isn't you taking them over: it's
// kept as a decision, their question's answered, and the end of their
// next turn is still the orchestrator's to look at. Anything else you
// say to them makes it yours.
func TestSay_AnswerToTheirQuestionLeavesThemWatched(t *testing.T) {
	w, _, _ := workFixture(t, "")
	w.opts.Host = &sayingHost{}
	w.Missions.CrewHook("cache", "c1", mission.Hook{Prompted: true})
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewAsking, "Fix all of them here, or only the first?")

	if _, err := w.Say("cache", "c1", "All of them.", AsYou); err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	if c := m.Crew[0]; c.Status != "" || c.Conversing {
		t.Fatalf("after your answer: %+v", c)
	}
	if ds, _ := w.Missions.Decisions("cache"); len(ds) != 1 || ds[0].What != "All of them." {
		t.Errorf("decisions = %+v", ds)
	}
	if !logged(t, w, "All of them.") {
		t.Error("what you said isn't in the log")
	}
	w.Missions.CrewHook("cache", "c1", mission.Hook{Activity: mission.ActivityWaiting, Reply: "On it."})
	m, _ = w.Missions.Load("cache")
	if takes, why := mission.OrchestratorTakes(m, mission.OrchestratorEvent{Crew: "c1", Trigger: "quiet"}); !takes {
		t.Errorf("their quiet turn isn't the orchestrator's: %s", why)
	}

	if _, err := w.Say("cache", "c1", "Why a map?", AsYou); err != nil {
		t.Fatal(err)
	}
	if m, _ := w.Missions.Load("cache"); !m.Crew[0].Conversing {
		t.Error("talking to them didn't make their turn's end yours")
	}
}
