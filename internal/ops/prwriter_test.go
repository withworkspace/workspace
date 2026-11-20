package ops

import (
	"slices"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
)

// twoPRPlan opens a pull request from frontend/one twice: after step 1's
// work, then after step 3's.
const twoPRPlan = "## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n" +
	"1. Work: cache the lookup.\n   - in: `frontend/one`\n" +
	"2. Open Pull Request\n   - in: `frontend/one`\n" +
	"3. Work: expire it.\n   - in: `frontend/one`\n   - check: entries expire after a minute\n" +
	"4. Review: is it right?\n" +
	"5. Work: write the docs.\n   - in: `frontend/two`\n" +
	"6. Open Pull Request\n   - in: `frontend/one`\n"

// A pull request's task is the work in its capsule since the last pull
// request from it: not the mission's other work, nor its reviews.
func TestPRTask(t *testing.T) {
	w, _ := fixture(t, "", "one", "two")
	if err := w.Missions.WritePlan("cache", twoPRPlan); err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	steps := func(step int) []int {
		var out []int
		for _, st := range w.prTask(m, "frontend/one", step) {
			out = append(out, st.N)
		}
		return out
	}
	if got := steps(2); !slices.Equal(got, []int{1}) {
		t.Errorf("at step 2: %v", got)
	}
	if got := steps(6); !slices.Equal(got, []int{3}) {
		t.Errorf("at step 6: %v", got)
	}
	// Opened outside a step: up to where the plan is (its first step).
	if got := steps(0); !slices.Equal(got, []int{1}) {
		t.Errorf("at no step: %v", got)
	}
	tasks := w.prTask(m, "frontend/one", 6)
	if b := taskBlock(tasks[0]); !strings.Contains(b, "Step 3 (Work): expire it.") || !strings.Contains(b, "  - entries expire after a minute") {
		t.Errorf("the task block:\n%s", b)
	}
	// No plan: no steps, and the goal is the task.
	m.Plan = nil
	if got := w.prTask(m, "frontend/one", 0); got != nil {
		t.Errorf("with no plan: %v", got)
	}
}

// What the PR writer's writing is written once: ws doesn't open it again
// meanwhile, nor take it for a step whose pull request isn't open. Only
// you have one written to read first.
func TestWritingPR_HoldsItsPlace(t *testing.T) {
	w, _ := fixture(t, "", "one")
	if _, started, err := w.Missions.StartWritingPR("cache", mission.WritingPR{Capsule: "frontend/one", By: "you"}); err != nil || !started {
		t.Fatalf("started %v, %v", started, err)
	}
	if _, started, _ := w.Missions.StartWritingPR("cache", mission.WritingPR{Capsule: "frontend/one", By: "c1"}); started {
		t.Error("marked twice")
	}
	_, err := w.OpenPR("cache", "frontend/one", NewPullRequest{}, AsYou)
	refusedWith(t, err, "pr-writing")
	m, _ := w.Missions.Load("cache")
	if hs := mission.Resolve(mission.Snapshot{M: m, State: mission.Active}); !slices.ContainsFunc(hs, func(h mission.Headline) bool {
		return h.Kind == mission.KindWritingPR && strings.Contains(h.Text, "The PR writer is writing frontend/one's pull request")
	}) {
		t.Errorf("headlines: %+v", hs)
	}
	_, err = w.WritePR("cache", "frontend/one", AsCrew("c1"))
	refusedWith(t, err, "not-yours")
	w.Missions.EndWritingPR("cache", "frontend/one")
	if m, _ := w.Missions.Load("cache"); len(m.WritingPRs) != 0 {
		t.Errorf("still marked: %+v", m.WritingPRs)
	}
}
