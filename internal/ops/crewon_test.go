package ops

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
)

// A step's on: (or a dispatch's --on) is harness:model, a harness alone,
// or a model alone, on crew's own harness.
func TestCrewPick(t *testing.T) {
	w, _, _ := reposFixture(t, "## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Review: it.\n")
	for _, c := range []struct{ on, want string }{
		{"", "fake-crew"},
		{"claude:opus", "claude:opus"},
		{"claude", "claude"},
		{"big", "fake-crew:big"},
		{"pi:sonnet:high", "pi:sonnet:high"},
		{"sonnet:high", "fake-crew:sonnet:high"},
	} {
		if got := w.CrewPick(c.on).String(); got != c.want {
			t.Errorf("CrewPick(%q) = %s, want %s", c.on, got, c.want)
		}
	}
}

// What crew run on is a guarantee: a plan naming what [crew] allowed
// doesn't list is refused, saying what it does; a step starts its crew on
// its on:; and a plan from before ws checked runs on crew's own.
func TestCrewOn_HeldToAllowed(t *testing.T) {
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: fix it.\n   - in: `frontend/one`\n   - on: fake-crew:big\n2. Review: it.\n"
	w, _, host := reposFixture(t, plan)
	_, err := w.CheckPlan(plan)
	if err == nil || !strings.Contains(err.Error(), "line 6: step 1's on: fake-crew:big is not in [crew] allowed (fake-crew (default))") {
		t.Fatalf("not allowed: %v", err)
	}
	w.Config().Crew.Allowed = []string{"fake-crew:big"}
	if _, err := w.CheckPlan(plan); err != nil {
		t.Fatalf("allowed: %v", err)
	}
	m, _ := w.Missions.Load("cache")
	cur, _ := currentStep(w.Missions, m)
	if said := w.reachStep(m, mission.StageState{}, cur); !strings.Contains(said, "started step 1: c1 in frontend/one") || strings.Contains(said, "crew's own") {
		t.Fatalf("started: %q", said)
	}
	m, _ = w.Missions.Load("cache")
	if c := m.Crew[0]; c.Harness != "fake-crew" || c.Model != "big" || len(host.specs) != 1 {
		t.Errorf("crew = %+v", c)
	}

	// Not allowed any more: the plan's on: is passed over for crew's own.
	w.Config().Crew.Allowed = nil
	said, err := w.Repeat("cache", "", "", "again, as it was", AsYou)
	if err != nil || !strings.Contains(said, "(on crew default: fake-crew:big not in [crew] allowed)") {
		t.Fatalf("again: %q, %v", said, err)
	}
	if m, _ = w.Missions.Load("cache"); m.Crew[1].Model != "" {
		t.Errorf("crew on %q", m.Crew[1].Model)
	}
	// Asking for it outright is refused, whoever asks.
	_, err = w.Repeat("cache", "", "fake-crew:big", "on the big one", AsYou)
	refusedWith(t, err, "not-allowed")
	_, err = w.Launch("cache", Dispatch{Model: "big"}, AsOrchestrator)
	refusedWith(t, err, "not-allowed")
}

// The planner and the orchestrator are told what crew may run on, among
// what ws holds.
func TestCrewOn_AGuarantee(t *testing.T) {
	w, _, _ := reposFixture(t, "## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Review: it.\n")
	w.Config().Crew.Allowed = []string{"claude:sonnet"}
	var said []string
	for _, g := range w.Guarantees() {
		said = append(said, g.Key+": "+g.What)
	}
	if !strings.Contains(strings.Join(said, "\n"), "crew.allowed: crew run only on what [crew] allowed lists: fake-crew (default), claude:sonnet") {
		t.Errorf("guarantees: %v", said)
	}
}
