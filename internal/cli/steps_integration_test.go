package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// typedPlan is the cache mission's plan, written in kinds of step: the
// work, then ws's pull request, then the user's decision.
const typedPlan = "## Z\n- Cached, and merged.\n## Stops early\n- None.\n## Steps\n" +
	"1. Work: cache the lookup.\n   - in: `backend`\n" +
	"2. Open Pull Request\n   - in: `backend`\n" +
	"3. Decide: does it go to prd now, or with the next release?\n"

// useFixtureGitHub has ws reach gh (a fixture) wherever it looks, as
// ws.local.toml's [github] fixture says.
func useFixtureGitHub(t *testing.T, root string, gh github.FixtureClient) {
	t.Helper()
	local := filepath.Join(root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[github]\nfixture = \""+gh.Path+"\"\n")...), 0o644)
}

// mergeAll merges every open pull request of repo on the fixture GitHub.
func mergeAll(t *testing.T, gh github.FixtureClient, repo string) {
	t.Helper()
	data, err := os.ReadFile(gh.Path)
	if err != nil {
		t.Fatal(err)
	}
	var f github.Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	r := f.Repos[repo]
	for _, pr := range r.PRs {
		pr.State, pr.MergedAt = "MERGED", time.Now().UTC().Format(time.RFC3339)
		r.Merged = append(r.Merged, pr)
	}
	r.PRs = nil
	data, _ = json.MarshalIndent(f, "", "  ")
	os.WriteFile(gh.Path, data, 0o644)
}

// The plan's kinds of step, reached by ws: Work's crew, then an Open Pull
// Request that's ws's own (waiting on the user's review, where ws holds
// one, then opened as ws, and passed when it merges), then a Decide whose
// answer is the decision and passes it.
func TestSteps_OpenPullRequestAndDecide(t *testing.T) {
	w, store := orchestrateFixture(t)
	gh := fixtureGitHub(t)
	useFixtureGitHub(t, w.Root, gh)
	holdReviews(t, w.Root)
	withStdin(t, typedPlan)
	run(t, w, gh, "mission", "plan", "-m", "cache", "--set", "-")
	m := loadMission(t, w, "cache")
	if m.Plan == nil || m.Plan.Steps[1].Type != mission.StepOpenPR || m.Plan.Steps[1].Who != mission.WhoWS {
		t.Fatalf("the plan: %+v", m.Plan)
	}
	if out := run(t, w, gh, "show", "-m", "cache", "-f", "llm", "s2").Stdout; !strings.Contains(out, "An Open Pull Request step. Who: ws.") || !strings.Contains(out, "ws opens its pull request itself") {
		t.Errorf("ws show s2:\n%s", out)
	}

	// Step 1's crew commit, and are done: ws reaches step 2, and asks for
	// the user's review first, as it holds review_before_pr.
	store.JoinStage("cache", "c1", 1)
	capsule := capsuleDir(w, "backend", "cache-lookup")
	commitIn(t, capsule, "cache.go", "package cache\n", "Cache the lookup")
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	t.Setenv("WS_ROLE", "orchestrator")
	run(t, w, gh, "mission", "accept", "-m", "cache", "cached")
	m = loadMission(t, w, "cache")
	if m.Asking() == nil || m.Asking().Step != 2 || !strings.Contains(m.Asking().Text, "Step 2 opens a pull request from backend/cache-lookup, and ws holds missions.review_before_pr: review it") {
		t.Fatalf("at step 2: %+v", m.Asking())
	}
	if open, _ := gh.PRsForRepo("test-org", "backend"); len(open) != 0 {
		t.Errorf("opened before the review: %+v", open)
	}
	// The orchestrator can't pass it: it's ws's, done once merged.
	if r := testutil.RunCommand(t, w.Root, gh, "mission", "accept", "-m", "cache", "done"); r.Err == nil || !strings.Contains(r.Err.Error(), "step 2 is ws's: it opens the pull request, and the step's done when it merges") {
		t.Errorf("the orchestrator accepting step 2: %v", r.Err)
	}
	t.Setenv("WS_ROLE", "")

	// The user's review approves it: ws opens it, as ws.
	ctx, err := cli.LoadOps(w.Root)
	if err != nil {
		t.Fatal(err)
	}
	d, err := review.Compare(capsule, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ctx.SubmitReview(ops.ReviewOf{Mission: "cache", Capsule: "backend/cache-lookup"}, review.Diffed{Dir: capsule, BaseRef: "origin/main", Base: d.Base, Head: d.Head}, review.Approve, "", ops.AsYou); err != nil {
		t.Fatal(err)
	}
	open, _ := gh.PRsForRepo("test-org", "backend")
	if len(open) != 1 || open[0].Title != "Cache" || open[0].IsDraft {
		t.Fatalf("GitHub has %+v", open)
	}
	if m = loadMission(t, w, "cache"); m.Asking() != nil {
		t.Errorf("still asking: %+v", m.Asking())
	}
	if !logged(t, store, "backend#1 opened, at step 2: Cache") {
		t.Error("the opening isn't logged")
	}

	// It merges: ws passes step 2 itself, and asks the user step 3.
	mergeAll(t, gh, "backend")
	if !ctx.PassOnMerge("cache", "backend/cache-lookup") {
		t.Fatal("the merge didn't pass step 2")
	}
	m = loadMission(t, w, "cache")
	if m.Asking() == nil || m.Asking().Step != 3 || !strings.Contains(m.Asking().Text, "Step 3 is yours, to decide: does it go to prd now, or with the next release?") || strings.Join(m.Asking().Options, ",") != mission.CheckInNotYet {
		t.Fatalf("at step 3: %+v", m.Asking())
	}
	if ctx.PassOnMerge("cache", "backend/cache-lookup") {
		t.Error("a merge passed a step that isn't ws's")
	}

	// The user's answer is the decision, and passes it.
	run(t, w, gh, "mission", "answer", "-m", "cache", "With the next release")
	m = loadMission(t, w, "cache")
	states, _ := m.StageProgress("")
	if _, ok := mission.CurrentStage(states); ok || m.Asking() != nil {
		t.Errorf("after the answer: %+v, flag %+v", states, m.Asking())
	}
	decisions, _ := store.Decisions("cache")
	if len(decisions) == 0 || decisions[len(decisions)-1].What != "With the next release" {
		t.Errorf("decisions: %+v", decisions)
	}
}

// A Land step is the user's, and passes once its capsule has landed.
func TestSteps_LandPassesOnLanding(t *testing.T) {
	w := landWorkspace(t, "")
	run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nA note.\n")
	t.Setenv(mission.EnvMission, "notes")
	run(t, w, nil, "lift", "records", "note")
	t.Setenv(mission.EnvMission, "")
	withStdin(t, "## Z\n- Landed.\n## Stops early\n- None.\n## Steps\n1. Work: write a note.\n   - in: `records`\n2. Land\n   - in: `records`\n")
	run(t, w, nil, "mission", "plan", "-m", "notes", "--set", "-")
	commitIn(t, capsuleDir(w, "records", "note"), "notes/a.md", "# A\n", "docs: add note a")
	run(t, w, nil, "mission", "accept", "-m", "notes", "written")
	m := loadMission(t, w, "notes")
	if m.Asking() == nil || m.Asking().Step != 2 || !strings.Contains(m.Asking().Text, "Step 2 is yours: land records/note") {
		t.Fatalf("at step 2: %+v", m.Asking())
	}
	run(t, w, nil, "land", "records", "note")
	m = loadMission(t, w, "notes")
	states, _ := m.StageProgress("")
	if _, ok := mission.CurrentStage(states); ok || m.Asking() != nil || states[1].Record == nil || states[1].Record.AcceptedBy != "you" {
		t.Errorf("after landing: %+v, flag %+v", states, m.Asking())
	}
}

// A Work step in a repo the mission has no capsule in gets one, named for
// the mission, when the step starts; a plan naming a repo nothing will lift
// a capsule in is warned of.
func TestSteps_WorkInARepoLiftsACapsule(t *testing.T) {
	w, _ := orchestrateFixture(t)
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[orchestrator]\ndispatch = \"ask\"\n")...), 0o644) // crew queued, not launched
	withStdin(t, "## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Review: is the cache right?\n2. Work: show the cached value.\n   - in: `fe`\n")
	if r := run(t, w, nil, "mission", "plan", "-m", "cache", "--set", "-"); strings.Contains(r.Stderr, "`fe`") {
		t.Errorf("warned of what ws lifts itself:\n%s", r.Stderr)
	}
	withStdin(t, "## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Open Pull Request\n   - in: `fe`\n")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "plan", "-m", "cache", "--set", "-"); !strings.Contains(r.Stderr, "step 1 is in `fe`, where the mission has no capsule, and no Work step before it makes one") {
		t.Errorf("an Open Pull Request with nothing to open from:\n%s", r.Stderr)
	}
	withStdin(t, "## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Review: is the cache right?\n2. Work: show the cached value.\n   - in: `fe`\n")
	run(t, w, nil, "mission", "plan", "-m", "cache", "--set", "-")
	run(t, w, nil, "mission", "accept", "-m", "cache", "it's right")
	m := loadMission(t, w, "cache")
	if !m.HasCapsule("frontend", "cache") || m.Dispatch == nil || m.Dispatch.Capsule != "frontend/cache" || m.Dispatch.Stage != 2 {
		t.Errorf("capsules %+v, crew queued %+v", m.Capsules, m.Dispatch)
	}
}

// A step of yours is a task, and a question asked after it is another: it
// doesn't take the step's place. Each is answered by its number (the
// oldest by default); going on passes the step; anything else at it goes to
// the orchestrator, and the step stays yours, as not yet leaves it; and it
// can't be dismissed.
func TestTasks_AStepOfYoursIsntLostToAQuestion(t *testing.T) {
	w, store := orchestrateFixture(t)
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[orchestrator]\ndispatch = \"ask\"\n")...), 0o644) // crew queued, not launched
	withStdin(t, "## Z\n- Cached.\n## Stops early\n- None.\n## Steps\n1. Work: cache the lookup.\n   - in: `backend`\n2. Review: is the cache right?\n3. Work: wire it in.\n   - in: `backend`\n")
	run(t, w, nil, "mission", "plan", "-m", "cache", "--set", "-")
	store.JoinStage("cache", "c1", 1)
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	t.Setenv("WS_ROLE", "orchestrator")
	run(t, w, nil, "mission", "accept", "-m", "cache", "cached")
	run(t, w, nil, "mission", "flag", "-m", "cache", "CI is red on main: fix it here?", "--option", "Fix it", "--option", "Leave it")
	t.Setenv("WS_ROLE", "")

	m := loadMission(t, w, "cache")
	if len(m.Tasks) != 2 || m.Tasks[0].Step != 2 || m.Tasks[0].Verb != mission.TaskReview || !m.Tasks[1].Question() || m.Tasks[1].By != "orchestrator" {
		t.Fatalf("the queue = %+v", m.Tasks)
	}
	review, question := m.Tasks[0].N, m.Tasks[1].N
	if out := run(t, w, nil, "mission", "show", "cache", "-f", "llm").Stdout; !strings.Contains(out, fmt.Sprintf("Waits on you, task %d (review, by ws): Step 1 is done", review)) || !strings.Contains(out, fmt.Sprintf("Waits on you, task %d (answer, by orchestrator): CI is red on main", question)) {
		t.Errorf("ws mission show:\n%s", out)
	}

	// The question, by its number: a decision, and the step's still yours.
	run(t, w, nil, "mission", "answer", "-m", "cache", "--task", fmt.Sprint(question), "Leave it")
	m = loadMission(t, w, "cache")
	if len(m.Tasks) != 1 || m.Tasks[0].N != review || m.Answer == nil || m.Answer.Text != "Leave it" {
		t.Fatalf("after answering the question: %+v, answer %+v", m.Tasks, m.Answer)
	}

	// Not yet, and anything else, leave the step yours; it can't be
	// dismissed.
	if r := run(t, w, nil, "mission", "answer", "-m", "cache", "Not yet"); !strings.Contains(r.Stderr, "step 2 stays yours") {
		t.Errorf("not yet: %s", r.Stderr)
	}
	if r := run(t, w, nil, "mission", "answer", "-m", "cache", "check the TTL first"); !strings.Contains(r.Stderr, "step 2 stays yours") {
		t.Errorf("your own words: %s", r.Stderr)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "flag", "-m", "cache", "--clear"); r.Err == nil || !strings.Contains(r.Err.Error(), "step 2 is yours") {
		t.Errorf("dismissing a step of yours: %v", r.Err)
	}
	m = loadMission(t, w, "cache")
	if len(m.Tasks) != 1 || m.Tasks[0].N != review {
		t.Fatalf("the step's task: %+v", m.Tasks)
	}
	decisions, _ := store.Decisions("cache")
	var said []string
	for _, d := range decisions {
		said = append(said, d.What)
	}
	if strings.Join(said, " | ") != "Leave it | check the TTL first" {
		t.Errorf("decisions: %q", said)
	}

	// Going on, from the page, passes it: step 3's crew are queued.
	if _, err := cli.WebAct(w.Root)("cache", web.ActRequest{Verb: "answer", Text: "Go on", Task: review}); err != nil {
		t.Fatal(err)
	}
	m = loadMission(t, w, "cache")
	if len(m.Tasks) != 0 || m.Dispatch == nil || m.Dispatch.Stage != 3 {
		t.Errorf("after going on: tasks %+v, crew queued %+v", m.Tasks, m.Dispatch)
	}
}

// editFixture changes the fixture GitHub's file.
func editFixture(t *testing.T, gh github.FixtureClient, fn func(f *github.Fixture)) {
	t.Helper()
	data, err := os.ReadFile(gh.Path)
	if err != nil {
		t.Fatal(err)
	}
	var f github.Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	fn(&f)
	data, _ = json.MarshalIndent(f, "", "  ")
	os.WriteFile(gh.Path, data, 0o644)
}

// An Open Pull Request step looks after its pull request: ws opens a
// draft; what happens to it (its checks passing, a comment) goes to the
// orchestrator, which marks it ready as the process says; crew's reply to
// a person waits for the user, and is posted as them.
func TestSteps_OpenPullRequestLookedAfter(t *testing.T) {
	w, store := orchestrateFixture(t)
	gh := fixtureGitHub(t)
	useFixtureGitHub(t, w.Root, gh)
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()
	withStdin(t, "## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n1. Work: cache the lookup.\n   - in: `backend`\n2. Open Pull Request\n   - in: `backend`\n   - draft: yes\n3. Decide: prd now?\n")
	run(t, w, gh, "mission", "plan", "-m", "cache", "--set", "-")
	store.JoinStage("cache", "c1", 1)
	commitIn(t, capsuleDir(w, "backend", "cache-lookup"), "cache.go", "package cache\n", "Cache the lookup")
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	t.Setenv("WS_ROLE", "orchestrator")
	run(t, w, gh, "mission", "accept", "-m", "cache", "cached")
	t.Setenv("WS_ROLE", "")
	open, _ := gh.PRsForRepo("test-org", "backend")
	if len(open) != 1 || !open[0].IsDraft {
		t.Fatalf("GitHub has %+v", open)
	}
	watch := func() {
		t.Helper()
		ctx, err := cli.LoadOps(w.Root)
		if err != nil {
			t.Fatal(err)
		}
		m := loadMission(t, w, "cache")
		ctx.WatchPRs(ops.BuildMissionViews(ctx.WS(), ctx.GitHub(), []*mission.Mission{m}))
	}
	watch() // where reading starts

	// Green: the orchestrator's to mark ready, as the process says.
	editFixture(t, gh, func(f *github.Fixture) { f.Repos["backend"].PRs[0].StatusRollup = "success" })
	watch()
	if strings.Join(*runs, ",") != "cache//pr" {
		t.Fatalf("orchestrator runs = %v", *runs)
	}
	prompt := run(t, w, gh, "mission", "orchestrate", "-m", "cache", "--trigger", "pr", "--print-prompt").Stdout
	for _, want := range []string{"The mission is at step 2 of 3, an Open Pull Request", "- backend#1's checks pass", "ws mission pr-ready"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the orchestrator's prompt lacks %q", want)
		}
	}
	t.Setenv("WS_ROLE", "orchestrator")
	if r := run(t, w, gh, "mission", "pr-ready", "-m", "cache"); !strings.Contains(r.Stderr, "marked backend#1 ready for review") {
		t.Errorf("ws mission pr-ready: %s", r.Stderr)
	}
	t.Setenv("WS_ROLE", "")
	if open, _ = gh.PRsForRepo("test-org", "backend"); open[0].IsDraft {
		t.Error("still a draft")
	}

	// A comment: news, for the orchestrator; crew reply, and it waits for
	// the user, who posts it.
	editFixture(t, gh, func(f *github.Fixture) {
		f.Repos["backend"].Threads = map[string][]github.ReviewThread{"1": {{ID: "T1", Path: "cache.go", Line: 1,
			Comments: []github.ThreadComment{{Author: "alice", Body: "Why a map?", At: "2030-01-01T00:00:00Z"}}}}}
	})
	watch()
	if len(*runs) != 2 || !logged(t, store, "alice on backend#1, `cache.go:1` (thread T1): Why a map?") {
		t.Fatalf("the comment: runs %v", *runs)
	}
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c1")
	if r := run(t, w, gh, "mission", "reply", "backend#1", "--thread", "T1", "A", "map", "is", "enough."); !strings.Contains(r.Stderr, "waits for the user's approval") {
		t.Errorf("ws mission reply: %s", r.Stderr)
	}
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvMission, "")
	run(t, w, gh, "mission", "answer", "-m", "cache", "Post it")
	threads, _ := gh.ReviewThreads("test-org", "backend", 1)
	if c := threads[0].Comments; len(c) != 2 || c[1].Author != "me" || c[1].Body != "A map is enough." {
		t.Errorf("the thread: %+v", c)
	}
	watch()
	if len(*runs) != 2 {
		t.Errorf("the user's own reply woke the orchestrator: %v", *runs)
	}
}

// A step that applies only if waits for the orchestrator's call: it's
// told why, and skips it (or starts it) through ws, which shows the step
// skipped, with why.
func TestSteps_TheOrchestratorsCall(t *testing.T) {
	w, store := orchestrateFixture(t)
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[orchestrator]\ndispatch = \"ask\"\n")...), 0o644)
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()
	withStdin(t, "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: find it.\n   - in: `backend`\n2. Decide: which fix?\n   - only if: the fix isn't obvious\n3. Work: fix it.\n   - in: `backend`\n4. Review: the fix.\n")
	run(t, w, nil, "mission", "plan", "-m", "cache", "--set", "-")
	store.JoinStage("cache", "c1", 1)
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "found it")
	t.Setenv("WS_ROLE", "orchestrator")
	run(t, w, nil, "mission", "accept", "-m", "cache", "found")
	if strings.Join(*runs, ",") != "cache//step" {
		t.Fatalf("orchestrator runs = %v", *runs)
	}
	prompt := run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--trigger", "step", "--print-prompt").Stdout
	if !strings.Contains(prompt, "The mission has reached step 2 of 4 (Decide: which fix?), which waits for your call: it applies only if the fix isn't obvious") {
		t.Errorf("the orchestrator's prompt: %s", prompt)
	}
	if out := run(t, w, nil, "show", "-m", "cache", "-f", "llm", "s2").Stdout; !strings.Contains(out, "(current, waiting for the orchestrator's call, or the user's)") || !strings.Contains(out, "Only if the fix isn't obvious") {
		t.Errorf("ws show s2:\n%s", out)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "start", "-m", "cache", "--in", "backend"); r.Err == nil || !strings.Contains(r.Err.Error(), "step 2 runs in one place") {
		t.Errorf("starting it in a repo: %v", r.Err)
	}
	if r := run(t, w, nil, "mission", "skip", "-m", "cache", "the fix is obvious"); !strings.Contains(r.Stderr, "skipped step 2, and step 3 is queued in backend/cache-lookup") {
		t.Errorf("ws mission skip: %s", r.Stderr)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "skip", "-m", "cache", "no need"); r.Err == nil || !strings.Contains(r.Err.Error(), "step 3 has no only if: it runs") {
		t.Errorf("skipping a step that always runs: %v", r.Err)
	}
	t.Setenv("WS_ROLE", "")
	if out := run(t, w, nil, "show", "-m", "cache", "-f", "llm", "s2").Stdout; !strings.Contains(out, "(skipped)") || !strings.Contains(out, "by orchestrator: the fix is obvious") {
		t.Errorf("ws show s2, skipped:\n%s", out)
	}
}

// ws doesn't open a pull request that conflicts from the start: at an Open
// Pull Request step whose capsule doesn't merge cleanly onto main (main
// moved on), the capsule goes back to the crew who worked in it, to merge
// main in, naming what conflicts; when they report done, ws opens it.
func TestSteps_OpenPullRequestWaitsForAMergeWithItsBase(t *testing.T) {
	w, store := orchestrateFixture(t)
	gh := fixtureGitHub(t)
	useFixtureGitHub(t, w.Root, gh)
	// Crew are resumed when you say (no daemon here to resume them in).
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[orchestrator]\ndispatch = \"ask\"\n")...), 0o644)
	withStdin(t, typedPlan)
	run(t, w, gh, "mission", "plan", "-m", "cache", "--set", "-")

	// c1 caches the lookup; meanwhile main changes the same file.
	store.JoinStage("cache", "c1", 1)
	capsule := capsuleDir(w, "backend", "cache-lookup")
	commitIn(t, capsule, "cache.go", "package cache\n\n// Lookup caches.\n", "Cache the lookup")
	commitIn(t, w.Sources["backend"], "cache.go", "package cache\n\n// Lookup, uncached.\n", "Another lookup")
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	t.Setenv("WS_ROLE", "orchestrator")
	run(t, w, gh, "mission", "accept", "-m", "cache", "cached")
	t.Setenv("WS_ROLE", "")

	if open, _ := gh.PRsForRepo("test-org", "backend"); len(open) != 0 {
		t.Fatalf("opened one that conflicts: %+v", open)
	}
	if !logged(t, store, "didn't open backend/cache-lookup's pull request at step 2: it conflicts with main in cache.go; sent it back to c1 to merge origin/main in first (round 1)") {
		t.Error("not logged")
	}
	m := loadMission(t, w, "cache")
	c, _ := m.CrewByID("c1")
	if !strings.Contains(c.ResumeWith, "it conflicts with main in cache.go. Merge origin/main into your branch (don't rebase, don't force-push)") {
		t.Errorf("c1 is told: %q", c.ResumeWith)
	}
	if m.Asking() != nil {
		t.Errorf("asked the user: %+v", m.Asking())
	}

	// c1 merges main in, and reports done: ws opens it.
	testutil.GitCmd(t, capsule, "fetch", "-q", "origin")
	if out, err := exec.Command("git", "-C", capsule, "merge", "-q", "origin/main").CombinedOutput(); err == nil {
		t.Fatalf("main merged in cleanly: %s", out)
	}
	commitIn(t, capsule, "cache.go", "package cache\n\n// Lookup caches, as main's does.\n", "Merge main")
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "merged main in")
	ctx, err := cli.LoadOps(w.Root)
	if err != nil {
		t.Fatal(err)
	}
	if said := ctx.CrewDone("cache", "c1"); !strings.Contains(said, "it opened backend#1 from backend/cache-lookup") {
		t.Errorf("c1 done again: %q", said)
	}
	if open, _ := gh.PRsForRepo("test-org", "backend"); len(open) != 1 {
		t.Errorf("GitHub has %+v", open)
	}
}
