package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/mission"
)

// rotePlan is workPlan with step 1 rote.
var rotePlan = strings.Replace(workPlan, "   - limit: 1h\n", "   - level: rote\n   - limit: 1h\n", 1)

// levelFixture is workFixture on rotePlan, crew on shell, whose rote is
// crew's own (shell has no small model) and normal "big".
func levelFixture(t *testing.T) (*Workspace, *[]string, string) {
	t.Helper()
	w, runs, dir := workFixture(t, "1. unmet: nothing keeps a lookup")
	w.cur.cfg.Crew.Harness = "shell"
	w.cur.cfg.Crew.Levels = config.CrewLevels{Normal: "shell:big"}
	if err := w.Missions.WritePlan("cache", rotePlan); err != nil {
		t.Fatal(err)
	}
	return w, runs, dir
}

// A rote step's checks unmet: ws starts it again a level up, fresh crew
// on the normal level's model, told what's unmet and to carry on from
// the handoff, before the orchestrator's asked.
func TestLevels_UnmetRoteStepsUp(t *testing.T) {
	w, runs, dir := levelFixture(t)
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	said := w.CrewDone("cache", "c1")
	if !strings.Contains(said, "new crew a level up (normal, on shell:big) queued in frontend/one") {
		t.Fatalf("CrewDone said %q", said)
	}
	if len(*runs) != 0 {
		t.Errorf("the orchestrator was woken: %v", *runs)
	}
	m, _ := w.Missions.Load("cache")
	d := m.Dispatch
	if d == nil || d.Stage != 1 || d.Harness != "shell" || d.Model != "big" || d.Capsule != "frontend/one" ||
		!strings.Contains(d.Text, "c1 did this step on shell (rote)") || !strings.Contains(d.Text, "you're on shell:big (normal)") ||
		!strings.Contains(d.Text, "- the lookup is cached: unmet") {
		t.Fatalf("the dispatch = %+v", d)
	}
	events, _ := w.Missions.ReadLog("cache", 0)
	if !strings.Contains(events[len(events)-1].Text, "step 1 up a level, its checks unmet: rote (shell) to normal (shell:big)") {
		t.Errorf("log = %+v", events[len(events)-1])
	}
}

// Crew on a pick that's none of the step's levels (launched on another,
// or switched), or at a level with nothing above on another model, are
// the orchestrator's as before.
func TestLevels_NoStepUp(t *testing.T) {
	for name, set := range map[string]func(w *Workspace){
		"top level":      func(w *Workspace) { w.cur.cfg.Crew.Levels = config.CrewLevels{} },
		"another pick":   func(w *Workspace) { w.Missions.SetCrewModel("cache", "c1", "other") },
		"step names on:": func(w *Workspace) { w.Missions.WritePlan("cache", workPlan) },
	} {
		t.Run(name, func(t *testing.T) {
			w, _, dir := levelFixture(t)
			set(w)
			os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
			w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
			if said := w.CrewDone("cache", "c1"); !strings.Contains(said, "the orchestrator decides what follows") {
				t.Fatalf("CrewDone said %q", said)
			}
		})
	}
}

// A step ahead set to a level by you is written in the plan, and kept as
// a decision of yours; one behind the mission can't be.
func TestSetStepLevel(t *testing.T) {
	w, _, _ := levelFixture(t)
	if _, err := w.SetStepLevel("cache", 2, "hard", AsOrchestrator); err == nil {
		t.Error("the orchestrator set a level")
	}
	set, err := w.SetStepLevel("cache", 2, "hard", AsYou)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	if m.Plan.Steps[1].Level != mission.LevelHard || set.Decision == "" {
		t.Fatalf("step 2 = %+v, set %+v", m.Plan.Steps[1], set)
	}
	if ds, _ := w.Missions.Decisions("cache"); len(ds) == 0 || !strings.Contains(ds[len(ds)-1].What, "Step 2 (test it) runs at hard, not normal.") {
		t.Errorf("decisions = %+v", ds)
	}
	_, err = w.SetStepLevel("cache", 1, "hard", AsYou)
	refusedWith(t, err, "steps-behind")
	if _, err := w.SetStepLevel("cache", 3, "hard", AsYou); err == nil {
		t.Error("a Review step was given a level")
	}
}
