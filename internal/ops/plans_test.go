package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

const cachePlan = `## Z
- The cache is merged, with its tests.

## Stops early
- None.

## Steps
1. Write the proposal. "Features are explored"
   - who: crew, in ` + "`frontend/one`" + `
   - makes: a proposal
2. Is the proposal right?
   - who: you
3. Build it.
   - who: crew, in ` + "`frontend/two`" + `
4. Merged.
   - who: outside: a teammate's review
`

// The plan is yours and the planner's: written whole, read strictly, its
// quotes held to the process.
func TestSetPlan(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "one", "two")
	_, err := w.SetPlan("cache", cachePlan, AsOrchestrator)
	refusedWith(t, err, "plan-is-the-planners")
	_, err = w.SetPlan("cache", cachePlan, AsCrew("c1"))
	refusedWith(t, err, "plan-is-the-planners")

	// No process to quote.
	_, err = w.SetPlan("cache", cachePlan, AsYou)
	if err == nil || !strings.Contains(err.Error(), `line 8: "Features are explored": the workspace has no process to quote`) {
		t.Fatalf("quoting no process: %v", err)
	}
	if _, ok := err.(*Invalid); !ok {
		t.Errorf("a malformed plan is %T", err)
	}
	os.WriteFile(filepath.Join(w.Root(), "PROCESS.md"), []byte("Features are *explored* and then written up.\n"), 0o644)
	_, err = w.SetPlan("cache", strings.Replace(cachePlan, "Features are explored", "Bugs are fixed", 1), AsYou)
	if err == nil || !strings.Contains(err.Error(), `"Bugs are fixed": isn't in the process`) {
		t.Errorf("quoting what the process doesn't say: %v", err)
	}
	set, err := w.SetPlan("cache", cachePlan, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if set.String() != "Wrote the plan: 4 steps." || len(set.Warnings) != 0 {
		t.Errorf("set = %s, warnings %q", set, set.Warnings)
	}
	if text, _ := w.Missions.ReadPlan("cache"); text != cachePlan {
		t.Errorf("plan.md = %q", text)
	}
	if m, _ := w.Missions.Load("cache"); m.Plan == nil || len(m.Plan.Steps) != 4 {
		t.Errorf("the mission's plan = %+v", m.Plan)
	}
	// A capsule it names that the mission hasn't is warned of.
	set, _ = w.SetPlan("cache", strings.Replace(cachePlan, "frontend/two", "frontend/three", 1), AsYou)
	if len(set.Warnings) != 1 || !strings.Contains(set.Warnings[0], "step 3 names `frontend/three`, which isn't one of the mission's capsules (yet): ws lift frontend three, or change the plan") {
		t.Errorf("warnings = %q", set.Warnings)
	}
	// Briefing, the planner may write it.
	b, _ := w.Missions.CreateBriefing("cache the user lookup")
	if _, err := w.SetPlan(b.Slug, cachePlan, AsCrew("c1")); err != nil {
		t.Errorf("the planner briefing: %v", err)
	}
}

// A mission with a plan moves along its steps: ws starts crew on crew's,
// asks you at yours (a checkpoint, which only you pass), and waits on
// outside's, which the orchestrator accepts once it's happened.
func TestPlanSteps(t *testing.T) {
	w, runs := fixture(t, "## Goal\nCache it.\n\n## Stages\n1. Not these.\n", "one", "two")
	w.Missions.WritePlan("cache", strings.Replace(cachePlan, ` "Features are explored"`, "", 1))
	host := &fakeHost{}
	w.opts.Host = host
	harness.Register(fakeHarness{})
	w.Config().Crew.Harness = "fake-crew"
	w.Config().Orchestrator.Dispatch = "auto"

	// Step 1 done: step 2 is yours.
	acc, err := w.AcceptStage("cache", AsOrchestrator, "the proposal's written")
	if err != nil || acc.String() != "Accepted step 1: step 2 is the user's, to review: they're asked, and their review approving what it looks at, or going on, passes it." {
		t.Fatalf("accept 1: %s, %v", acc, err)
	}
	m, _ := w.Missions.Load("cache")
	if m.Asking() == nil || m.Asking().Step != 2 || m.Asking().Text != "Step 1 is done. Step 2 is yours, to review: Is the proposal right? Read what the crew found (their handoffs), and go on when you're satisfied; say what's to change, and it goes back to them." {
		t.Errorf("the checkpoint's question = %+v", m.Asking())
	}
	// The orchestrator can't pass it, nor send crew to it.
	_, err = w.AcceptStage("cache", AsOrchestrator, "looks fine")
	refusedWith(t, err, "checkpoint")

	// Going on passes it, and starts step 3's crew.
	a, err := w.Answer("cache", AsYou, "go on")
	if err != nil || a.Next != "started step 3: c1 in frontend/two" {
		t.Fatalf("go on: %+v, %v", a, err)
	}
	if len(host.specs) != 1 || !strings.Contains(host.specs[0].Prompt, "You're on step 3:") || len(*runs) != 0 {
		t.Errorf("started %d, orchestrator ran %v", len(host.specs), *runs)
	}
	m, _ = w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if !states[1].Accepted() || states[1].Record.AcceptedBy != "you" || m.Asking() != nil {
		t.Errorf("step 2 = %+v, flag %+v", states[1].Record, m.Asking())
	}

	// Step 3 done: step 4 is waited for, and accepted once it's happened.
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "built it")
	acc, err = w.AcceptStage("cache", AsOrchestrator, "built")
	if err != nil || !strings.Contains(acc.Next, "step 4 is outside ws's (outside: a teammate's review): it's waited for") {
		t.Fatalf("accept 3: %s, %v", acc, err)
	}
	if m, _ := w.Missions.Load("cache"); m.Asking() != nil {
		t.Errorf("asked the user about an outside step: %+v", m.Asking())
	}
	acc, err = w.AcceptStage("cache", AsOrchestrator, "merged")
	if err != nil || !strings.Contains(acc.Next, "that was the plan's last step: the mission is at Z") {
		t.Errorf("accept 4: %s, %v", acc, err)
	}
}

// You may pass a checkpoint by accepting it yourself, and that answers
// ws's question about it.
func TestCheckpointAcceptedByYou(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "one", "two")
	w.Missions.WritePlan("cache", "## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Look at it.\n   - who: you\n2. The planner plans.\n   - who: the planner\n")
	w.Missions.AskAtStep("cache", mission.TaskApprove, "Step 1 is yours", 1)
	acc, err := w.AcceptStage("cache", AsYou, "")
	if err != nil || !strings.Contains(acc.Next, "step 2 is the planner's: it's asked, in t1, to write the steps after it") {
		t.Fatalf("accept: %s, %v", acc, err)
	}
	m, _ := w.Missions.Load("cache")
	if m.Asking() != nil {
		t.Errorf("your question about step 1 is still asked: %+v", m.Asking())
	}
	_, err = w.AcceptStage("cache", AsOrchestrator, "")
	refusedWith(t, err, "planner-step")
}
