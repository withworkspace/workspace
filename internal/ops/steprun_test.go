package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// condPlan has a Decide that applies only sometimes between two steps
// that always run.
const condPlan = "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n" +
	"1. Work: find it.\n   - in: `frontend/one`\n" +
	"2. Decide: which fix?\n   - only if: the fix isn't obvious\n" +
	"3. Review: the fix.\n"

func condFixture(t *testing.T) (*Workspace, *[]string) {
	t.Helper()
	w, runs := fixture(t, "", "one")
	if err := w.Missions.WritePlan("cache", condPlan); err != nil {
		t.Fatal(err)
	}
	w.Missions.StartCrew("cache", "shell", "")
	w.Missions.JoinStage("cache", "c1", 1)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "found it: the rounding")
	return w, runs
}

// A step that applies only if waits at its start for the orchestrator's
// call: it can't accept it, may skip it saying why (not one that always
// runs), and the mission moves on.
func TestStepRun_SkippedByTheOrchestrator(t *testing.T) {
	w, runs := condFixture(t)
	acc, err := w.AcceptStage("cache", AsOrchestrator, "found")
	if err != nil || !strings.Contains(acc.Next, "step 2 waits for the orchestrator's call: applies only if the fix isn't obvious") {
		t.Fatalf("accept: %+v, %v", acc, err)
	}
	if strings.Join(*runs, ",") != "cache/step" {
		t.Errorf("runs = %v", *runs)
	}
	if m, _ := w.Missions.Load("cache"); len(m.Tasks) != 0 {
		t.Errorf("the user's asked already: %+v", m.Tasks)
	}
	_, err = w.AcceptStage("cache", AsOrchestrator, "skip it")
	refusedWith(t, err, "held")
	_, err = w.Skip("cache", "", AsOrchestrator)
	if err == nil {
		t.Error("skipped without saying why")
	}
	said, err := w.Skip("cache", "the fix is obvious: round once, at the end", AsOrchestrator)
	if err != nil || !strings.Contains(said, "skipped step 2, and step 3 waits for the user's") {
		t.Fatalf("skip: %q, %v", said, err)
	}
	m, _ := w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if !states[1].Skipped() || states[1].Record.Why != "the fix is obvious: round once, at the end" || states[1].Record.AcceptedBy != "orchestrator" {
		t.Errorf("step 2 = %+v", states[1].Record)
	}
	if task := m.Asking(); task == nil || task.Step != 3 || !strings.HasPrefix(task.Text, "Step 2 is skipped. Step 3 is yours") {
		t.Errorf("at step 3: %+v", task)
	}
	_, err = w.Skip("cache", "it's fine", AsOrchestrator)
	refusedWith(t, err, "always-runs")
}

// With the orchestrator off, the call is yours: go on starts it, skip it
// skips it.
func TestStepRun_YourCall(t *testing.T) {
	w, _ := condFixture(t)
	off := false
	w.Config().Orchestrator.Enabled = &off
	w.AcceptStage("cache", AsYou, "found")
	m, _ := w.Missions.Load("cache")
	task := m.Asking()
	if task == nil || task.Step != 2 || task.Verb != mission.TaskApprove || strings.Join(task.Options, "|") != "Go on|Skip it" || !strings.Contains(task.Text, "Step 2 applies only if the fix isn't obvious: which fix? Go on to start, or skip.") {
		t.Fatalf("the task = %+v", task)
	}
	a, err := w.AnswerTask("cache", task.N, AsYou, mission.CheckInGoOn)
	if err != nil || !strings.Contains(a.Next, "step 2 waits for the user's decision") {
		t.Fatalf("go on: %+v, %v", a, err)
	}
	m, _ = w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if states[1].Held() || states[1].Record.GoBy != "you" || m.Asking() == nil || m.Asking().Verb != mission.TaskAnswer {
		t.Errorf("started: %+v, task %+v", states[1].Record, m.Asking())
	}
}

// reposFixture is the cache mission with a capsule in each of two repos,
// frontend/one and backend/two, ws starting crew as fake-crew.
func reposFixture(t *testing.T, plan string) (*Workspace, *[]string, *fakeHost) {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"frontend", "backend"}}
	os.MkdirAll(filepath.Join(ws.RepoDir("frontend"), "one"), 0o755)
	os.MkdirAll(filepath.Join(ws.RepoDir("backend"), "two"), 0o755)
	var runs []string
	host := &fakeHost{}
	w := New(&config.Config{}, ws, nil, Options{Host: host, Jobs: JobsOf(func(j job.Job) { runs = append(runs, j.Mission+"/"+j.Why) }, job.Orchestrate)})
	harness.Register(fakeHarness{})
	w.Config().Crew.Harness = "fake-crew"
	m, _ := w.Missions.Create("Cache")
	w.Attach(m.Slug, "frontend", "one", false, AsYou)
	w.Attach(m.Slug, "backend", "two", false, AsYou)
	if err := w.Missions.WritePlan(m.Slug, plan); err != nil {
		t.Fatal(err)
	}
	return w, &runs, host
}

// A step that runs in each of several repos waits for the orchestrator to
// name them, runs once in each (crew in each repo's capsule), and is done
// when every run is: with checks, when each run's are met.
func TestStepRun_InEachRepo(t *testing.T) {
	plan := "## Z\n- Updated.\n## Stops early\n- None.\n## Steps\n" +
		"1. Work: update the dependencies.\n   - in each: repo of the workspace\n   - check: `test -f updated` passes\n" +
		"2. Review: the updates.\n"
	w, runs, host := reposFixture(t, plan)
	// Reaching step 1 (the plan's first) is the orchestrator's call.
	m, _ := w.Missions.Load("cache")
	cur, _ := currentStep(w.Missions, m)
	if said := w.reachStep(m, mission.StageState{}, cur); !strings.Contains(said, "waits for the orchestrator's call: runs in each repo of the workspace") || strings.Join(*runs, ",") != "cache/step" {
		t.Fatalf("reached: %q, runs %v", said, *runs)
	}
	_, err := w.StartStep("cache", nil, AsOrchestrator)
	refusedWith(t, err, "which-repos")
	said, err := w.StartStep("cache", []string{"frontend", "backend"}, AsOrchestrator)
	if err != nil || !strings.Contains(said, "started step 1: c1 in frontend/one") || !strings.Contains(said, "started step 1: c2 in backend/two") {
		t.Fatalf("started: %q, %v", said, err)
	}
	if len(host.specs) != 2 || !strings.Contains(host.specs[1].Prompt, "It runs in each of frontend, backend: you're its run in `backend`.") {
		t.Errorf("crew launched: %d", len(host.specs))
	}

	// frontend's run is done once its checks are met; the step, once
	// backend's are too.
	os.WriteFile(filepath.Join(w.WS().RepoDir("frontend"), "one", "updated"), nil, 0o644)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "updated")
	if said := w.CrewDone("cache", "c1"); !strings.Contains(said, "step 1 done in frontend, still running in backend") {
		t.Errorf("frontend done: %q", said)
	}
	_, err = w.AcceptStage("cache", AsOrchestrator, "done")
	refusedWith(t, err, "runs-on")
	os.WriteFile(filepath.Join(w.WS().RepoDir("backend"), "two", "updated"), nil, 0o644)
	w.Missions.SetCrewStatus("cache", "c2", mission.CrewDone, "updated")
	if said := w.CrewDone("cache", "c2"); !strings.Contains(said, "passed step 1") {
		t.Errorf("backend done: %q", said)
	}
	m, _ = w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if !states[0].Accepted() || len(states[0].Instances()) != 2 || states[0].Instances()[1].Capsule != "backend/two" || states[0].Instances()[1].Done == nil {
		t.Errorf("step 1 = %+v", states[0].Record)
	}
}

// The Work step the mission's at can be started again with fresh crew,
// another attempt, within max_rounds; then it's the user's.
func TestStepRun_Repeat(t *testing.T) {
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: fix it.\n   - in: `frontend/one`\n2. Review: the fix.\n"
	w, _, host := reposFixture(t, plan)
	w.Config().Missions.MaxRounds = 2
	_, err := w.Repeat("cache", "", "", "it went wrong", AsOrchestrator)
	refusedWith(t, err, "not-begun")
	m, _ := w.Missions.Load("cache")
	cur, _ := currentStep(w.Missions, m)
	w.reachStep(m, mission.StageState{}, cur)
	if _, err := w.Repeat("cache", "", "", "", AsOrchestrator); err == nil {
		t.Error("started again without saying why")
	}
	said, err := w.Repeat("cache", "", "", "c1 fixed the symptom, not the cause", AsOrchestrator)
	if err != nil || !strings.HasPrefix(said, "attempt 2 at step 1: started step 1: c2 in frontend/one") || len(host.specs) != 2 {
		t.Fatalf("again: %q, %v", said, err)
	}
	w.Repeat("cache", "", "", "and again", AsOrchestrator)
	_, err = w.Repeat("cache", "", "", "once more", AsOrchestrator)
	refusedWith(t, err, "max-rounds")
	if _, err := w.Repeat("cache", "", "", "once more, as I say", AsYou); err != nil {
		t.Errorf("the user starting it again: %v", err)
	}
	m, _ = w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if states[0].Record.Attempts != 3 {
		t.Errorf("attempts = %d", states[0].Record.Attempts)
	}
	if !logged(t, w, "step 1 again, attempt 2 · c1 fixed the symptom, not the cause") {
		t.Error("the attempt isn't logged")
	}
}

func logged(t *testing.T, w *Workspace, text string) bool {
	t.Helper()
	events, _ := w.Missions.ReadLog("cache", 0)
	for _, e := range events {
		if strings.Contains(e.Text, text) {
			return true
		}
	}
	return false
}

// An Open Pull Request step in each of several repos: with the
// orchestrator off, you name them; it's done once every one has merged.
func TestStepRun_PullRequestInEachRepo(t *testing.T) {
	plan := "## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n" +
		"1. Open Pull Request\n   - in each: repo the work changed\n" +
		"2. Review: what merged.\n"
	w, _, _ := reposFixture(t, plan)
	off := false
	w.Config().Orchestrator.Enabled = &off
	m, _ := w.Missions.Load("cache")
	cur, _ := currentStep(w.Missions, m)
	w.reachStep(m, mission.StageState{}, cur)
	m, _ = w.Missions.Load("cache")
	task := m.Asking()
	if task == nil || !strings.Contains(task.Text, "Step 1 runs in each repo the work changed") || !strings.Contains(task.Text, "Name the repos to run in") {
		t.Fatalf("the task = %+v", task)
	}
	if a, err := w.AnswerTask("cache", task.N, AsYou, "frontend, and something"); err != nil || !strings.Contains(a.Next, "orchestrator is off ([orchestrator] enabled = false)") {
		t.Errorf("not repos: %+v, %v", a, err)
	}
	m, _ = w.Missions.Load("cache")
	if _, err := w.AnswerTask("cache", m.Asking().N, AsYou, "frontend, backend"); err != nil {
		t.Fatal(err)
	}
	m, _ = w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if ins := states[0].Instances(); len(ins) != 2 || ins[0].Capsule != "frontend/one" || ins[1].Capsule != "backend/two" {
		t.Fatalf("its runs: %+v", ins)
	}
	// ws couldn't open them here (no git), so pretend it did; each merge
	// is a run done, and the last passes the step.
	w.Missions.SeePR("cache", "frontend/one", mission.SeenPR{Number: 3})
	w.Missions.SeePR("cache", "backend/two", mission.SeenPR{Number: 4})
	if !w.PassOnMerge("cache", "frontend/one") {
		t.Error("frontend's merge wasn't the step's")
	}
	m, _ = w.Missions.Load("cache")
	if states, _ = m.StageProgress(""); states[0].Accepted() || strings.Join(states[0].InstancesLeft(), ",") != "backend" {
		t.Errorf("after frontend: %+v", states[0].Record)
	}
	w.PassOnMerge("cache", "backend/two")
	m, _ = w.Missions.Load("cache")
	if states, _ = m.StageProgress(""); !states[0].Accepted() || states[0].Record.AcceptedBy != "ws" {
		t.Errorf("after backend: %+v", states[0].Record)
	}
}

// A step that carries on from one that ran in several repos runs in the
// same repos, fresh crew in each run's capsule told to read the handoff
// of the crew before them.
func TestStepRun_CarryingOnInEachRepo(t *testing.T) {
	plan := "## Z\n- Updated.\n## Stops early\n- None.\n## Steps\n" +
		"1. Work: update.\n   - in each: repo of the workspace\n" +
		"2. Work: test it.\n   - carrying on from: 1\n" +
		"3. Review: it.\n"
	w, _, host := reposFixture(t, plan)
	w.StartStep("cache", []string{"frontend", "backend"}, AsOrchestrator)
	for _, c := range []string{"c1", "c2"} {
		w.Missions.SetCrewStatus("cache", c, mission.CrewDone, "updated")
		w.CrewDone("cache", c)
	}
	acc, err := w.AcceptStage("cache", AsOrchestrator, "updated in both")
	if err != nil || !strings.Contains(acc.Next, "started c3 in frontend/one") || !strings.Contains(acc.Next, "started c4 in backend/two") {
		t.Fatalf("accept: %+v, %v", acc, err)
	}
	m, _ := w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if ins := states[1].Instances(); len(ins) != 2 || ins[0].Capsule != "frontend/one" || len(host.specs) != 4 || !strings.Contains(host.specs[3].Prompt, "read their handoff first (ws show -f llm mission:handoffs/c2.md)") {
		t.Errorf("step 2's runs: %+v", ins)
	}
}

// Crew waiting for you are queued one at a time: a step's runs in several
// repos queue the next as each is launched.
func TestStepRun_QueuedOneAtATime(t *testing.T) {
	plan := "## Z\n- Updated.\n## Stops early\n- None.\n## Steps\n1. Work: update.\n   - in each: repo of the workspace\n2. Review: it.\n"
	w, _, host := reposFixture(t, plan)
	w.Config().Orchestrator.Dispatch = "ask"
	said, err := w.StartStep("cache", []string{"frontend", "backend"}, AsOrchestrator)
	if err != nil || !strings.Contains(said, "queued in frontend/one, ready to launch; the rest queue one at a time") {
		t.Fatalf("started: %q, %v", said, err)
	}
	if _, err := w.LaunchQueued("cache", AsYou); err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	if m.Dispatch == nil || m.Dispatch.Capsule != "backend/two" || m.Dispatch.Stage != 1 {
		t.Fatalf("the next queued: %+v", m.Dispatch)
	}
	w.LaunchQueued("cache", AsYou)
	if m, _ = w.Missions.Load("cache"); m.Dispatch != nil || len(host.specs) != 2 {
		t.Errorf("after both: %+v, %d launched", m.Dispatch, len(host.specs))
	}
}

// Launching crew at a step that waits for a call is your call: one that
// applies only if starts; one that runs in several repos asks you which.
func TestStepRun_LaunchingIsYourCall(t *testing.T) {
	plan := "## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Work: tidy.\n   - in each: repo of the workspace\n2. Review: it.\n"
	w, _, host := reposFixture(t, plan)
	_, err := w.Launch("cache", Dispatch{}, AsYou)
	refusedWith(t, err, "which-repos")
	m, _ := w.Missions.Load("cache")
	task := m.Asking()
	if task == nil || task.Step != 1 || !strings.Contains(task.Text, "Name the repos to run in") {
		t.Fatalf("the task = %+v", task)
	}
	if _, err := w.AnswerTask("cache", task.N, AsYou, "backend"); err != nil || len(host.specs) != 1 || host.specs[0].Capsule != "backend/two" {
		t.Errorf("answered: %v, launched %d", err, len(host.specs))
	}

	w2, _, host2 := reposFixture(t, "## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Work: tidy.\n   - in: `frontend/one`\n   - only if: it's untidy\n2. Review: it.\n")
	if l, err := w2.Launch("cache", Dispatch{}, AsYou); err != nil || l.Capsule != "frontend/one" || len(host2.specs) != 1 {
		t.Fatalf("launched: %+v, %v", l, err)
	}
	m, _ = w2.Missions.Load("cache")
	if states, _ := m.StageProgress(""); states[0].Held() || states[0].Record.GoBy != "you" {
		t.Errorf("step 1 = %+v", states[0].Record)
	}
	w3, _, _ := reposFixture(t, plan)
	_, err = w3.Launch("cache", Dispatch{}, AsOrchestrator)
	refusedWith(t, err, "held")
}
