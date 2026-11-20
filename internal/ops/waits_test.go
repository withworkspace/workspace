package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// waitPlan is a plan of a Work step, a Wait with the parameters given,
// and a Work step after it.
func waitPlan(params string) string {
	return "## Z\n- Released.\n## Stops early\n- None.\n## Steps\n" +
		"1. Work: Build it.\n   - in: `frontend/one`\n" +
		"2. Wait: **Release.** The release is tagged.\n" + params +
		"3. Work: Move on to it.\n   - in: `frontend/two`\n"
}

// atWait is a mission whose plan has reached its Wait, step 2.
func atWait(t *testing.T, params string) (*Workspace, *fakeHost, *[]string) {
	t.Helper()
	w, runs := fixture(t, "## Goal\nRelease it.\n", "one", "two")
	if err := w.Missions.WritePlan("cache", waitPlan(params)); err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{}
	w.opts.Host = host
	harness.Register(fakeHarness{})
	w.Config().Crew.Harness = "fake-crew"
	w.Config().Orchestrator.Dispatch = "auto"
	if _, err := w.AcceptStage("cache", AsOrchestrator, "built"); err != nil {
		t.Fatal(err)
	}
	return w, host, runs
}

func stepOf(t *testing.T, w *Workspace, n int) mission.StageState {
	t.Helper()
	m, err := w.Missions.Load("cache")
	if err != nil {
		t.Fatal(err)
	}
	states, _ := m.StageProgress("")
	return states[n-1]
}

// A Wait on you is yours: ws asks you, the orchestrator can't pass it, and
// marking it done passes it. What you say you did is kept for the crew
// after, as a decision.
func TestWaitOnYou(t *testing.T) {
	w, host, runs := atWait(t, "   - on: you\n")
	m, _ := w.Missions.Load("cache")
	task := m.Asking()
	if task == nil || task.Verb != mission.TaskDo || task.Step != 2 || len(task.Options) != 1 || task.Options[0] != mission.WaitDone {
		t.Fatalf("asked = %+v", task)
	}
	if want := "Step 1 is done. Step 2 is yours, to do: Release. The release is tagged. ws can't see it happen, so nothing moves until you mark it done."; !strings.HasPrefix(task.Text, want) {
		t.Errorf("asked %q, want it to start %q", task.Text, want)
	}
	if st := stepOf(t, w, 2); st.Watch() == nil || st.Watch().Since.IsZero() {
		t.Errorf("since when it's waited isn't kept: %+v", st.Record)
	}
	_, err := w.AcceptStage("cache", AsOrchestrator, "it looks released")
	refusedWith(t, err, "checkpoint")

	// A question isn't done: it's a decision for the orchestrator, and the
	// step stays yours.
	a, err := w.Answer("cache", AsYou, "which version do I tag?")
	if err != nil || !strings.Contains(a.Next, "step 2 stays yours") || stepOf(t, w, 2).Accepted() {
		t.Fatalf("a question: %+v, %v", a, err)
	}
	a, err = w.Answer("cache", AsYou, "Done: tagged v0.2.0, not v0.1.0")
	if err != nil || a.Next != "started step 3: c1 in frontend/two" {
		t.Fatalf("done: %+v, %v", a, err)
	}
	st := stepOf(t, w, 2)
	if !st.Accepted() || st.Record.AcceptedBy != "you" || st.Record.Why != "the user marked it done: tagged v0.2.0, not v0.1.0" {
		t.Errorf("step 2 = %+v", st.Record)
	}
	if m, _ := w.Missions.Load("cache"); m.Asking() != nil {
		t.Errorf("still asked: %+v", m.Asking())
	}
	ds, _ := w.Missions.Decisions("cache")
	if last := ds[len(ds)-1]; last.What != "tagged v0.2.0, not v0.1.0" || !strings.Contains(last.Why, "said of step 2, a wait") {
		t.Errorf("decisions = %+v", ds)
	}
	if len(host.specs) != 1 {
		t.Errorf("started %d crew", len(host.specs))
	}
	_ = runs
}

// A Wait with a check is watched: ws looks as its looks come due, and
// passes the step once the check is met, with nobody saying so.
func TestWaitWatched(t *testing.T) {
	w, host, _ := atWait(t, "   - on: the release workflow\n   - check: `test -f released` passes\n")
	clock := time.Now()
	w.opts.Now = func() time.Time { return clock }
	if m, _ := w.Missions.Load("cache"); m.Asking() != nil {
		t.Errorf("asked the user about a wait on someone else: %+v", m.Asking())
	}
	_, err := w.AcceptStage("cache", AsOrchestrator, "it looks released")
	refusedWith(t, err, "watched")

	// The first look, at once: not yet.
	if said := w.CheckWaits(); len(said) != 0 {
		t.Errorf("passed before it happened: %q", said)
	}
	st := stepOf(t, w, 2)
	if st.Accepted() || st.Watch().Looks != 1 || st.Watch().Summary() != "not yet" {
		t.Fatalf("after a look = %+v", st.Watch())
	}
	// It doesn't look again until the next is due.
	w.CheckWaits()
	if looks := stepOf(t, w, 2).Watch().Looks; looks != 1 {
		t.Errorf("looked %d times within a minute", looks)
	}
	if said, err := w.CheckWait("cache", AsYou); err != nil || !strings.HasPrefix(said, "Not yet: `test -f released` passes (failed (exit 1)") {
		t.Errorf("look now: %q, %v", said, err)
	}
	_, err = w.CheckWait("cache", AsOrchestrator)
	refusedWith(t, err, "not-yours")

	// It happens, and the next look passes the step and starts the next.
	os.WriteFile(filepath.Join(w.Root(), "released"), nil, 0o644)
	clock = clock.Add(2 * time.Minute)
	said := w.CheckWaits()
	if len(said) != 1 || !strings.Contains(said[0], "cache: passed step 2, and started step 3: c1 in frontend/two") {
		t.Fatalf("the look after it happened: %q", said)
	}
	st = stepOf(t, w, 2)
	if !st.Accepted() || st.Record.AcceptedBy != "ws" || st.Record.Why != "what it waits for has happened: `test -f released` is met" {
		t.Errorf("step 2 = %+v", st.Record)
	}
	if len(host.specs) != 1 {
		t.Errorf("started %d crew", len(host.specs))
	}
	_, err = w.CheckWait("cache", AsYou)
	refusedWith(t, err, "no-wait")
}

// Marking a watched Wait done has ws look first: unmet, it stays yours,
// with what the look found, and you can pass it as it is.
func TestWaitOnYouWatched(t *testing.T) {
	w, _, _ := atWait(t, "   - on: you\n   - check: `test -f released` passes\n")
	m, _ := w.Missions.Load("cache")
	if task := m.Asking(); task == nil || !strings.Contains(task.Text, "ws watches for it (`test -f released`), and passes the step once it's happened; or mark it done, and ws looks then.") {
		t.Fatalf("asked = %+v", task)
	}
	a, err := w.Answer("cache", AsYou, "Done")
	if err != nil || !strings.Contains(a.Next, "ws's look doesn't show it yet (`test -f released` passes (failed (exit 1)") || stepOf(t, w, 2).Accepted() {
		t.Fatalf("done, not happened: %+v, %v", a, err)
	}
	m, _ = w.Missions.Load("cache")
	task := m.Asking()
	if task == nil || task.Verb != mission.TaskDo || len(task.Options) != 2 || task.Options[1] != mission.ChecksPass || !strings.Contains(task.Text, "Step 2 is marked done, and ws's look doesn't show it yet: `test -f released` passes (failed (exit 1)") {
		t.Fatalf("asked again = %+v", task)
	}
	// Done again, once it has.
	os.WriteFile(filepath.Join(w.Root(), "released"), nil, 0o644)
	a, err = w.Answer("cache", AsYou, "done")
	if err != nil || a.Next != "started step 3: c1 in frontend/two" {
		t.Fatalf("done: %+v, %v", a, err)
	}
	if st := stepOf(t, w, 2); !st.Accepted() || st.Record.Why != "the user marked it done, and `test -f released` is met" {
		t.Errorf("step 2 = %+v", st.Record)
	}

	// Passed as it is, whatever the check says.
	w, _, _ = atWait(t, "   - on: you\n   - check: `test -f released` passes\n")
	if _, err := w.Answer("cache", AsYou, "Done"); err != nil {
		t.Fatal(err)
	}
	a, err = w.Answer("cache", AsYou, "Pass it")
	if err != nil || a.Next != "started step 3: c1 in frontend/two" {
		t.Fatalf("pass it: %+v, %v", a, err)
	}
	if st := stepOf(t, w, 2); !st.Accepted() || st.Record.Why != "the user passed it, its checks as they were" {
		t.Errorf("step 2 = %+v", st.Record)
	}
}

// A Wait on someone else with no check: nobody's watching. Nothing asks
// you, nothing looks, and it's passed when you say it's happened; what
// you say of it is kept for the crew after.
func TestWaitNobodyWatches(t *testing.T) {
	w, _, runs := atWait(t, "   - on: the platform team\n")
	if m, _ := w.Missions.Load("cache"); m.Asking() != nil {
		t.Errorf("asked: %+v", m.Asking())
	}
	if said := w.CheckWaits(); len(said) != 0 || stepOf(t, w, 2).Watch().Looks != 0 {
		t.Errorf("looked at a wait with nothing to run: %q", said)
	}
	_, err := w.CheckWait("cache", AsYou)
	refusedWith(t, err, "not-watched")
	if len(*runs) != 0 {
		t.Errorf("the orchestrator ran: %v", *runs)
	}
	acc, err := w.AcceptStage("cache", AsYou, "deployed to prd at 14:02")
	if err != nil || acc.Next != "started step 3: c1 in frontend/two" {
		t.Fatalf("accept: %+v, %v", acc, err)
	}
	ds, _ := w.Missions.Decisions("cache")
	if len(ds) == 0 || ds[len(ds)-1].What != "deployed to prd at 14:02" {
		t.Errorf("decisions = %+v", ds)
	}
}

// At a Wait, the step says what the mission waits on: the orchestrator
// saying it too (ws mission wait) is refused, and where a mission says
// both anyway, the step's is the headline, with marking it done. Anywhere
// else it's the orchestrator's to say.
func TestWaitIsntSaidTwice(t *testing.T) {
	w, _, _ := atWait(t, "   - on: the platform team\n")
	err := w.SayWaiting("cache", "the platform team's deploy; the user says when it has happened", AsOrchestrator)
	refusedWith(t, err, "wait-step")
	if m, _ := w.Missions.Load("cache"); m.Waiting != nil {
		t.Errorf("waiting = %+v", m.Waiting)
	}
	// Said by hand, or kept from before ws refused it.
	if err := w.SayWaiting("cache", "the platform team's deploy", AsYou); err != nil {
		t.Fatal(err)
	}
	v, err := w.Mission("cache", false)
	if err != nil {
		t.Fatal(err)
	}
	if hs := v.Headlines; len(hs) != 1 || hs[0].Action.Verb != mission.VerbWait {
		t.Errorf("headlines = %+v; want the step's alone, to mark done", hs)
	}

	if _, err := w.AcceptStage("cache", AsYou, "deployed"); err != nil {
		t.Fatal(err)
	}
	if err := w.SayWaiting("cache", "a teammate elsewhere", AsOrchestrator); err != nil {
		t.Errorf("at a Work step: %v", err)
	}
}

// What the orchestrator asked at a Wait (the run failed: again, or wait?)
// is settled when the step passes, marked done or seen by ws's look: it
// isn't left to lead the next step's page.
func TestWaitPassedSettlesWhatTheOrchestratorAskedAtIt(t *testing.T) {
	asked := func(t *testing.T, w *Workspace) {
		t.Helper()
		if _, err := w.Flag("cache", "The release's last run failed on a flaky upload. Run it again, or wait for the nightly?", AsOrchestrator, "Run it again", "Wait"); err != nil {
			t.Fatal(err)
		}
	}
	questions := func(w *Workspace) (left []string) {
		m, _ := w.Missions.Load("cache")
		for _, task := range m.Tasks {
			if task.Question() {
				left = append(left, task.Text)
			}
		}
		return left
	}

	w, _, _ := atWait(t, "   - on: the platform team\n")
	asked(t, w)
	if _, err := w.AcceptStage("cache", AsYou, "deployed"); err != nil {
		t.Fatal(err)
	}
	if left := questions(w); len(left) != 0 || !stepOf(t, w, 2).Accepted() {
		t.Errorf("marked done: still asked %q", left)
	}

	w, _, _ = atWait(t, "   - on: CI\n   - in: `frontend`\n   - check: `test -f here` passes\n")
	asked(t, w)
	if said, err := w.CheckWait("cache", AsYou); err != nil || !strings.HasPrefix(said, "Not yet") || len(questions(w)) != 1 {
		t.Fatalf("a look that finds nothing: %q, %v, asked %q", said, err, questions(w))
	}
	os.WriteFile(filepath.Join(w.WS().RepoDir("frontend"), "one", "here"), nil, 0o644)
	if _, err := w.CheckWait("cache", AsYou); err != nil {
		t.Fatal(err)
	}
	if left := questions(w); len(left) != 0 || !stepOf(t, w, 2).Accepted() {
		t.Errorf("seen to have happened: still asked %q", left)
	}
}

// A Wait's checks run in the repo it names: the mission's capsule there,
// or the repo's ground.
func TestWaitChecksRunInItsRepo(t *testing.T) {
	w, _, _ := atWait(t, "   - on: CI\n   - in: `frontend`\n   - check: `test -f here` passes\n")
	os.WriteFile(filepath.Join(w.WS().RepoDir("frontend"), "one", "here"), nil, 0o644)
	if said := w.CheckWaits(); len(said) != 1 {
		t.Errorf("a check in the mission's capsule: %q, %+v", said, stepOf(t, w, 2).Watch())
	}
}

// A plan written now says who each Wait ahead is on; one behind the
// mission, written before a Wait said, stays as it was.
func TestPlanWaitsSayWhoTheyAreOn(t *testing.T) {
	w, _ := fixture(t, "## Goal\nRelease it.\n", "one", "two")
	_, err := w.SetPlan("cache", waitPlan(""), AsYou)
	if err == nil || !strings.Contains(err.Error(), "line 8: step 2 is a Wait that doesn't say who it's on: add - on: you") {
		t.Fatalf("a wait that doesn't say: %v", err)
	}
	if _, err := w.SetPlan("cache", waitPlan("   - on: you\n"), AsYou); err != nil {
		t.Fatal(err)
	}
	// Behind the mission, it's left as it is.
	w.Missions.WritePlan("cache", waitPlan(""))
	w.Missions.Confirm("cache", "you")
	w.AcceptStage("cache", AsYou, "")
	w.AcceptStage("cache", AsYou, "")
	if st := stepOf(t, w, 2); !st.Accepted() {
		t.Fatalf("step 2 = %+v", st.Record)
	}
	if _, err := w.SetPlan("cache", waitPlan("")+"4. Wait: **Again.** Another release.\n   - on: you\n", AsYou); err != nil {
		t.Errorf("a revision that leaves a passed wait as it was: %v", err)
	}
	_, err = w.SetPlan("cache", waitPlan("")+"4. Wait: **Again.** Another release.\n", AsYou)
	if err == nil || !strings.Contains(err.Error(), "step 4 is a Wait that doesn't say who it's on") || strings.Contains(err.Error(), "step 2") {
		t.Errorf("a new wait that doesn't say: %v", err)
	}
}
