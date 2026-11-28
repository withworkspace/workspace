package ops

import (
	"slices"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/outbox"
)

// In the daemon, a step you pass answers before what's next is reached:
// the mission's marked moving on, and the daemon's job reaches it, as it
// would have been there and then, and ends the mark. A command's passing
// reaches it at once.
func TestReachBehind(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "one", "two")
	w.Missions.WritePlan("cache", "## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Look at it.\n   - who: you\n2. Look again.\n   - who: you\n3. The planner plans.\n   - who: the planner\n")
	w.opts.Outbox = &outbox.Queue{Root: w.Root()}
	var jobs []string
	w.opts.Jobs = JobsOf(func(j job.Job) { jobs = append(jobs, j.Kind+" "+j.Mission+"/"+j.ID) }, job.Reach, job.Answer)

	acc, err := w.AcceptStage("cache", AsYou, "")
	if err != nil || acc.String() != "Accepted step 1: moving on to what's next." {
		t.Fatalf("accept: %s, %v", acc, err)
	}
	if !slices.Contains(jobs, "reach cache/1") {
		t.Errorf("jobs: %v", jobs)
	}
	m, _ := w.Missions.Load("cache")
	if m.Moving == nil || !m.Moving.Behind || m.Moving.Step != 1 {
		t.Fatalf("marked: %+v", m.Moving)
	}
	if m.Asking() != nil {
		t.Errorf("step 2 reached before the job: %+v", m.Asking())
	}
	if hs := mission.Resolve(mission.Snapshot{M: m, State: mission.Active}); !slices.ContainsFunc(hs, func(h mission.Headline) bool {
		return h.Kind == mission.KindMovingOn && h.Text == "Moving on from step 1"
	}) {
		t.Errorf("headlines: %+v", hs)
	}

	// The job.
	if said := w.ReachBehind("cache", 1); !strings.Contains(said, "step 2") {
		t.Errorf("the job said %q", said)
	}
	m, _ = w.Missions.Load("cache")
	if m.Moving != nil {
		t.Errorf("still moving: %+v", m.Moving)
	}
	if a := m.Asking(); a == nil || !strings.Contains(a.Text, "Step 2") {
		t.Errorf("step 2 not reached: %+v", a)
	}
	// Taken up again, by a daemon after: nothing to do.
	if said := w.ReachBehind("cache", 1); said != "" {
		t.Errorf("again: %q", said)
	}

	// A command (no daemon) reaches it there and then.
	w.opts.Outbox = nil
	acc, err = w.AcceptStage("cache", AsYou, "")
	if err != nil || !strings.Contains(acc.Next, "step 3 is the planner's") {
		t.Errorf("accept 2, as a command: %s, %v", acc, err)
	}
}
