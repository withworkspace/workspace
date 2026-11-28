package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// workPlan is a Work step with checks, one ws runs and one the judge
// judges, and a limit; then a Work step that carries on from it; then the
// user's review.
const workPlan = "## Z\n- Cached.\n## Stops early\n- None.\n## Steps\n" +
	"1. Work: cache the lookup.\n   - in: `frontend/one`\n   - limit: 1h\n   - check: `test -f cache.go` passes\n   - check: the lookup is cached\n" +
	"2. Work: test it.\n   - carrying on from: 1\n" +
	"3. Review: the cache.\n"

// workFixture is the cache mission on workPlan, its crew c1 at step 1 in
// frontend/one, and the judge the fake converser, saying judged.
func workFixture(t *testing.T, judged string) (*Workspace, *[]string, string) {
	t.Helper()
	w, runs := fixture(t, "## Goal\nCache the lookup.\n", "one")
	w.cur.cfg.Judge.Harness = fake.Name()
	fake.reply, fake.prompts = judged, nil
	if err := w.Missions.WritePlan("cache", workPlan); err != nil {
		t.Fatal(err)
	}
	w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewProcess("cache", "c1", 0, "frontend/one")
	if _, err := w.Missions.JoinStage("cache", "c1", 1); err != nil {
		t.Fatal(err)
	}
	return w, runs, filepath.Join(w.WS().RepoDir("frontend"), "one")
}

func stepRecord(t *testing.T, w *Workspace, n int) mission.StageState {
	t.Helper()
	m, err := w.Missions.Load("cache")
	if err != nil {
		t.Fatal(err)
	}
	states, _ := m.StageProgress("")
	return states[n-1]
}

// Crew report done at a Work step with checks: ws runs the command, has
// the judge judge the rest, keeps the round, and with all met, passes the
// step; fresh crew carry on at the next from its crew's handoff.
func TestChecks_AllMetPassesTheStep(t *testing.T) {
	w, runs, dir := workFixture(t, "1. met: cache.go keeps each lookup")
	if st := stepRecord(t, w, 1); st.Record.Deadline == nil || st.Record.Deadline.Sub(*st.Record.Started) != time.Hour {
		t.Errorf("the clock isn't held from the start: %+v", st.Record)
	}
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	said := w.CrewDone("cache", "c1")
	if !strings.Contains(said, "passed step 1") {
		t.Errorf("CrewDone said %q", said)
	}
	st := stepRecord(t, w, 1)
	r, ok := st.Record.LatestChecks()
	if !st.Accepted() || st.Record.AcceptedBy != "ws" || !ok || r.N != 1 || !r.Met() || r.Crew != "c1" || r.Capsule != "frontend/one" {
		t.Fatalf("step 1 = %+v, round %+v", st.Record, r)
	}
	if r.Results[0].By != "ws" || !strings.HasPrefix(r.Results[0].Evidence, "passed in") || r.Results[1].By != "the judge (fake-answer)" || r.Results[1].Evidence != "cache.go keeps each lookup" {
		t.Errorf("results = %+v", r.Results)
	}
	if len(fake.prompts) != 1 || !strings.Contains(fake.prompts[0], "1. the lookup is cached") || !strings.Contains(fake.prompts[0], "`test -f cache.go` passes: met") {
		t.Errorf("the judge was told %q", fake.prompts)
	}
	if len(*runs) != 0 {
		t.Errorf("the orchestrator was woken: %v", *runs)
	}
	// Step 2 carries on from 1: fresh crew are queued, told to read c1's
	// handoff.
	m, _ := w.Missions.Load("cache")
	if m.Dispatch == nil || m.Dispatch.Stage != 2 || !strings.Contains(m.Dispatch.Text, "Step 1 is done, and you carry on from c1, who did it: read their handoff first (ws show -f llm mission:handoffs/c1.md)") {
		t.Errorf("step 2's crew: %+v", m.Dispatch)
	}
	// It's done once: the same done isn't checked again.
	if said, _ := w.RunChecks("cache", "c1"); said != "" {
		t.Errorf("checked again: %q", said)
	}
}

// Unmet goes to the orchestrator, which can't pass the step, until
// max_rounds have come round; then it's yours, to pass it as it is or send
// it back with what's unmet.
func TestChecks_UnmetIsTheOrchestratorsThenYours(t *testing.T) {
	w, runs, dir := workFixture(t, "1. unmet: nothing keeps a lookup")
	w.cur.cfg.Missions.MaxRounds = 2
	done := func(note string) string {
		w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, note)
		return w.CrewDone("cache", "c1")
	}
	if said := done("cached"); !strings.Contains(said, "the orchestrator decides what follows") {
		t.Errorf("round 1: %q", said)
	}
	if len(*runs) != 1 || (*runs)[0] != "cache/checks" {
		t.Errorf("the orchestrator wasn't woken for the checks: %v", *runs)
	}
	_, err := w.AcceptStage("cache", AsOrchestrator, "looks done")
	refusedWith(t, err, "checks")
	if r, _ := stepRecord(t, w, 1).Record.LatestChecks(); r.Met() || len(r.With(mission.CheckUnmet)) != 2 {
		t.Errorf("round 1 = %+v", r)
	}

	// Round 2 is the last ws lets come round: it's yours.
	if said := done("cached, again"); !strings.Contains(said, "it's the user's") {
		t.Errorf("round 2: %q", said)
	}
	m, _ := w.Missions.Load("cache")
	task := m.Asking()
	if task == nil || task.Verb != mission.TaskChecks || task.Step != 1 || !strings.Contains(task.Text, "round 2: 0 of 2 met") || strings.Join(task.Options, "|") != "Pass it|Send it back" || len(*runs) != 1 {
		t.Fatalf("the task = %+v (runs %v)", task, *runs)
	}

	// Sent back: c1 can't be reached here, so fresh crew are queued with
	// what's unmet.
	a, err := w.AnswerTask("cache", task.N, AsYou, mission.ChecksSendBack)
	if err != nil || !strings.Contains(a.Next, "queued") {
		t.Fatalf("sending it back: %+v, %v", a, err)
	}
	m, _ = w.Missions.Load("cache")
	if m.Dispatch == nil || m.Dispatch.Stage != 1 || !strings.Contains(m.Dispatch.Text, "Step 1 isn't done yet") || !strings.Contains(m.Dispatch.Text, "nothing keeps a lookup") || len(m.Tasks) != 0 {
		t.Errorf("sent back: %+v, tasks %+v", m.Dispatch, m.Tasks)
	}

	// Round 3, still unmet, is yours again; passed as it is, the next step
	// is reached.
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	done("cached, a third time")
	m, _ = w.Missions.Load("cache")
	if a, err := w.AnswerTask("cache", m.Asking().N, AsYou, mission.ChecksPass); err != nil || a.Next == "" {
		t.Fatalf("passing it: %+v, %v", a, err)
	}
	if st := stepRecord(t, w, 1); !st.Accepted() || st.Record.AcceptedBy != "you" || len(st.Record.Checks) != 3 {
		t.Errorf("step 1 = %+v", st.Record)
	}
}

// The judge can't tell: that's yours, not the orchestrator's; and a judge
// that can't be asked is can't tell too.
func TestChecks_CantTellIsYours(t *testing.T) {
	w, runs, dir := workFixture(t, "1. can't tell: it needs production's traffic")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	w.CrewDone("cache", "c1")
	m, _ := w.Missions.Load("cache")
	if task := m.Asking(); task == nil || task.Verb != mission.TaskChecks || !strings.Contains(task.Text, "The judge could not decide on “the lookup is cached”: your call") || len(*runs) != 0 {
		t.Fatalf("the task = %+v (runs %v)", task, *runs)
	}

	w.cur.cfg.Judge.Harness = "no-such-harness"
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached, again")
	w.CrewDone("cache", "c1")
	if r, _ := stepRecord(t, w, 1).Record.LatestChecks(); r.Results[1].Result != mission.CheckCantTell || !strings.Contains(r.Results[1].Evidence, "could not ask the judge") {
		t.Errorf("with no judge: %+v", r.Results)
	}
}

// A command's check is met by how it exits, as the check says; one that
// can't run, or isn't there, is can't tell.
func TestRunCheck(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		check mission.Check
		want  string
	}{
		{mission.Check{Command: "true", Expect: mission.CheckPasses}, mission.CheckMet},
		{mission.Check{Command: "echo boom; exit 1", Expect: mission.CheckPasses}, mission.CheckUnmet},
		{mission.Check{Command: "exit 1", Expect: mission.CheckFails}, mission.CheckMet},
		{mission.Check{Command: "true", Expect: mission.CheckFails}, mission.CheckUnmet},
		{mission.Check{Command: "no-such-command-here", Expect: mission.CheckFails}, mission.CheckCantTell},
	} {
		if r := runCheckWithin(dir, c.check, os.Environ(), checkHooks{}, checkTimeout); r.Result != c.want {
			t.Errorf("%q %s: %s (%s)", c.check.Command, c.check.Expect, r.Result, r.Evidence)
		}
	}
	if r := runCheckWithin(dir, mission.Check{Command: "echo boom; exit 1", Expect: mission.CheckPasses}, os.Environ(), checkHooks{}, checkTimeout); !strings.Contains(r.Evidence, "failed (exit 1)") || !strings.HasSuffix(r.Evidence, "boom") {
		t.Errorf("evidence = %q", r.Evidence)
	}
}

// ws holds a Work step's clock: when it runs out with crew at it, it says
// so once, to the orchestrator, or with it off, to you; more time sets it
// again, and crew may only take less.
func TestLimit_OutOfTime(t *testing.T) {
	w, runs, _ := workFixture(t, "")
	clock := time.Now().Add(30 * time.Minute)
	w.opts.Now = func() time.Time { return clock }
	if said := w.CheckLimits(); len(said) != 0 {
		t.Errorf("in time: %v", said)
	}
	clock = clock.Add(time.Hour)
	if said := w.CheckLimits(); len(said) != 1 || !strings.Contains(said[0], "step 1 is out of time (at ") || !strings.Contains(said[0], "): the orchestrator decides") || len(*runs) != 1 || (*runs)[0] != "cache/limit" {
		t.Errorf("out of time: %v, runs %v", said, *runs)
	}
	if said := w.CheckLimits(); len(said) != 0 {
		t.Errorf("said twice: %v", said)
	}

	// Crew take less time; never more; and only on their own step.
	if said, err := w.MoreTime("cache", 0, 20*time.Minute, AsCrew("c1")); err != nil || !strings.Contains(said, "step 1's time runs out at") {
		t.Errorf("crew taking less: %q, %v", said, err)
	}
	_, err := w.MoreTime("cache", 0, 0, AsCrew("c2"))
	refusedWith(t, err, "not-yours")
	before := stepRecord(t, w, 1).Record.Deadline
	w.MoreTime("cache", 0, 3*time.Hour, AsCrew("c1"))
	if after := stepRecord(t, w, 1).Record.Deadline; !after.Equal(*before) {
		t.Errorf("crew gave themselves more: %v, was %v", after, before)
	}

	// The orchestrator off: it's yours, and more time sets the clock again.
	off := false
	w.Config().Orchestrator.Enabled = &off
	clock = time.Now().Add(2 * time.Hour)
	w.CheckLimits()
	m, _ := w.Missions.Load("cache")
	task := m.Asking()
	if task == nil || task.Verb != mission.TaskTime || task.Step != 1 || strings.Join(task.Options, "|") != "More time|Wrap up" || !strings.Contains(task.Text, "c1 is still working") {
		t.Fatalf("the task = %+v", task)
	}
	a, err := w.AnswerTask("cache", task.N, AsYou, mission.TimeMore)
	if err != nil || !strings.Contains(a.Next, "step 1's time runs out at") {
		t.Errorf("more time: %+v, %v", a, err)
	}
	st := stepRecord(t, w, 1)
	if m, _ = w.Missions.Load("cache"); len(m.Tasks) != 0 || st.Record.OutOfTime != nil || time.Until(*st.Record.Deadline) < 59*time.Minute {
		t.Errorf("after more time: tasks %+v, record %+v", m.Tasks, st.Record)
	}
}

// listeningHost is crew that are running, and take what's said to them.
type listeningHost struct{ said []string }

func (h *listeningHost) Launch(s crew.Spec) (int, error) { return deadPID, nil }
func (h *listeningHost) Stop(string) error               { return nil }
func (h *listeningHost) Say(key, text string) (string, error) {
	h.said = append(h.said, key+": "+text)
	return "typed", nil
}

// A step carrying on from one whose crew are still running starts fresh
// crew in their capsule, given their handoff, rather than telling them:
// a session carried from step to step re-reads all of each step before.
// The crew before keep their done at the step they did.
func TestChecks_CarryingOnStartsFreshCrew(t *testing.T) {
	w, _, dir := workFixture(t, "1. met: cache.go keeps each lookup")
	host := &listeningHost{}
	w.opts.Host = host
	harness.Register(fakeHarness{}) // crew launched on no harness of this machine's
	w.Config().Crew.Harness = "fake-crew"
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	if said := w.CrewDone("cache", "c1"); !strings.Contains(said, "started c2 in frontend/one to carry on at step 2, with handoff") {
		t.Errorf("CrewDone said %q", said)
	}
	if len(host.said) != 0 {
		t.Errorf("c1 was told %q", host.said)
	}
	m, _ := w.Missions.Load("cache")
	c, _ := m.CrewByID("c1")
	st := stepRecord(t, w, 2)
	if c.Status != mission.CrewDone || st.Record == nil || len(st.Record.Crew) != 1 || st.Record.Crew[0] != "c2" {
		t.Errorf("c1 = %+v, step 2 = %+v", c, st.Record)
	}
	if c2, _ := m.CrewByID("c2"); c2.Capsule != "frontend/one" {
		t.Errorf("c2 = %+v", c2)
	}
}

// Crew whose step comes back for a command ws ran are given the end of
// what it printed, as it printed it, and where all of it is: not the start
// of it run onto one line.
func TestSendBackText_CommandsEnd(t *testing.T) {
	out := "=== RUN TestA\n" + strings.Repeat("ok noise\n", 400) + "--- FAIL: TestCache (0.01s)\n    cache_test.go:12: want 2, got 1\nFAIL"
	round := mission.CheckRound{N: 2, Results: []mission.CheckResult{
		{Check: "`go test ./...` passes", Result: mission.CheckMet, By: "ws", Evidence: "passed in 3s:\nok"},
		{Check: "`go test ./...` passes", Result: mission.CheckUnmet, By: "ws", Run: "check-1-1.cache.api#2", Evidence: "failed (exit 1) in 40s:\n" + out},
		{Check: "the cache is named", Result: mission.CheckUnmet, By: "the judge (claude)", Evidence: "cache.go names nothing"},
	}}
	got := sendBackText(mission.StageState{N: 1}, round)
	for _, want := range []string{
		"- `go test ./...` passes: unmet. failed (exit 1) in 40s.\n",
		"    cache_test.go:12: want 2, got 1\n  FAIL\n  ```\n",
		"All it printed: `ws run logs check-1-1.cache.api#2`.",
		"- the cache is named: unmet. cache.go names nothing\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("sent back without %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "=== RUN") || strings.Contains(got, "passed in 3s") || len(got) > sentBackOutputMax+800 {
		t.Errorf("sent back too much (%d bytes):\n%s", len(got), got)
	}
}
