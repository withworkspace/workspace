package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

const flakyPlan = `## Z
- The flaky test passes a hundred runs in a row, merged.

## Stops early
- It isn't flaky, only slow: that's an answer.

## Steps
1. Find why it fails, as a failing test. "Bugs are reproduced first"
   - who: crew, in ` + "`backend/flaky-test`" + `
   - makes: the failing test, and the cause
2. Is that the cause?
   - who: you
3. Fix it, and open a pull request.
   - who: crew, in ` + "`backend/flaky-test`" + `
4. Merged.
   - who: outside: a teammate's review
`

// The planner: briefing in a conversation, its agent proposes the brief
// and then the plan, which ws reads strictly (refusing a malformed one,
// saying why) and which comes with the brief, so confirming it writes
// both and makes the mission ready. Then the plan is the mission's route:
// ws show has its steps, and the orchestrator is given it in place of
// the brief's stages.
func TestTalk_BriefingWithAPlan(t *testing.T) {
	w, store := orchestrateFixture(t)
	harness.Register(fakeConverser{})
	os.WriteFile(filepath.Join(w.Root, "PROCESS.md"), []byte("Bugs are reproduced first, then fixed.\n"), 0o644)
	t.Cleanup(cli.UseLifter(func(root, slug, repo, branch string) error { return nil }))
	m, _ := store.CreateBriefing("the flaky test")
	talks := conversation.For(w.Root, m.Slug)
	talks.OpenBriefing("the flaky test", time.Now())

	t.Setenv(conversation.EnvConversation, "t1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	t.Setenv(mission.EnvMission, m.Slug)
	withStdin(t, "## Goal\nFix the flaky test.\n\n## Acceptance\n- It passes a hundred runs in a row.\n")
	run(t, w, nil, "mission", "brief", "--propose", "--set", "-", "--title", "Fix the flaky test", "--capsule", "backend/flaky-test")

	// A malformed plan is refused, with every problem and its line.
	withStdin(t, "# Plan\n## Z\n- Fixed.\n## Steps\n1. Fix it. \"Bugs are squashed\"\n")
	r := testutil.RunCommand(t, w.Root, nil, "mission", "plan", "--propose", "--set", "-")
	if r.Err == nil {
		t.Fatal("a malformed plan was proposed")
	}
	for _, want := range []string{"the plan isn't right", "line 1: # Plan: a plan has no title", `line 5: "Bugs are squashed": isn't in the process`, "line 5: step 1 doesn't say what kind of step it is", "it has no ## Stops early"} {
		if !strings.Contains(r.Err.Error(), want) {
			t.Errorf("refusal lacks %q:\n%v", want, r.Err)
		}
	}

	withStdin(t, flakyPlan)
	r = run(t, w, nil, "mission", "plan", "--propose", "--set", "-")
	if !strings.Contains(r.Stderr, "it comes with the brief you proposed") {
		t.Errorf("proposing the plan said:\n%s", r.Stderr)
	}
	c, _ := talks.Load("t1")
	if len(c.Proposals) != 2 || c.Proposals[1].Kind != conversation.ProposeBrief || c.Proposals[1].Plan != flakyPlan || c.Proposals[1].Title != "Fix the flaky test" {
		t.Fatalf("proposals = %+v", c.Proposals)
	}
	// Proposing the brief again keeps its plan.
	withStdin(t, "## Goal\nFix the flaky test, for good.\n\n## Acceptance\n- It passes a hundred runs in a row.\n")
	run(t, w, nil, "mission", "brief", "--propose", "--set", "-", "--title", "Fix the flaky test", "--capsule", "backend/flaky-test")
	c, _ = talks.Load("t1")
	if p := c.Proposals[2]; p.Kind != conversation.ProposeBrief || p.Plan != flakyPlan {
		t.Errorf("the brief again lost its plan: %+v", p)
	}
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")

	r = run(t, w, nil, "conversation", "accept", "-m", m.Slug, "t1", "3")
	for _, want := range []string{"Wrote the new brief, and its plan (4 steps).", "It's under way: "} {
		if !strings.Contains(r.Stderr, want) {
			t.Errorf("accepting it said %q, lacking %q", r.Stderr, want)
		}
	}
	got, _ := store.Load(m.Slug)
	if got.ToBrief() || got.Plan == nil || len(got.Plan.Steps) != 4 {
		t.Fatalf("after: %+v", got)
	}
	if plan := run(t, w, nil, "mission", "plan", "-m", m.Slug).Stdout; plan != flakyPlan {
		t.Errorf("ws mission plan = %q", plan)
	}

	// ws show has the plan in place of stages.
	show := run(t, w, nil, "show", "-m", m.Slug, "-f", "llm").Stdout
	for _, want := range []string{
		"Z (where it ends: each has to be true, and each is checked by its number when it ends):\n  1. The flaky test passes a hundred runs in a row, merged.",
		"At: Step 1 of 4: Find why it fails, as a failing test",
		"Stops early:\n  - It isn't flaky, only slow: that's an answer.",
		"Steps (the plan's",
		"1. [current] Find why it fails, as a failing test. (Work, crew, in `backend/flaky-test`; makes: the failing test, and the cause)",
		"2. [to do] Is that the cause? (Review, you)",
		"4. [to do] Merged. (Wait, outside: a teammate's review)",
	} {
		if !strings.Contains(show, want) {
			t.Errorf("show lacks %q:\n%s", want, show)
		}
	}
	step := run(t, w, nil, "show", "-m", m.Slug, "-f", "llm", "s2").Stdout
	for _, want := range []string{"Step s2 of 4 (to do): Is that the cause", "Who: you. It's a checkpoint: only the user passes it.", "s1", "the step before"} {
		if !strings.Contains(step, want) {
			t.Errorf("ws show s2 lacks %q:\n%s", want, step)
		}
	}

	// The orchestrator is given the plan, and can't change it.
	prompt := run(t, w, nil, "mission", "orchestrate", "-m", m.Slug, "--print-prompt").Stdout
	for _, want := range []string{"Move the mission along its plan", "## The plan", "1. The flaky test passes a hundred runs in a row, merged.", "1. [current] Find why it fails", "(Review, you)"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("orchestrator prompt lacks %q", want)
		}
	}
	if strings.Contains(prompt, "## Stage progress") {
		t.Error("the orchestrator was given stages")
	}
	t.Setenv(mission.EnvMission, m.Slug)
	t.Setenv("WS_ROLE", "orchestrator")
	withStdin(t, flakyPlan)
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "plan", "--set", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "never changes it") {
		t.Errorf("the orchestrator changed the plan: %v", r.Err)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "dispatch", "--stage", "2", "look at it"); r.Err == nil || !strings.Contains(r.Err.Error(), "step 2 of the plan is a Review step, the user's, not crew's") {
		t.Errorf("the orchestrator sent crew to a checkpoint: %v", r.Err)
	}
}

// The planner briefing in a terminal writes the plan before it makes the
// mission ready.
func TestBriefing_ReadyWantsThePlan(t *testing.T) {
	w := missionWorkspace(t)
	store := mission.Store{Root: w.Root}
	m, _ := store.CreateBriefing("api: cache the user lookup")
	crew, _ := store.StartCrewAs(m.Slug, "cursor", "conv-9", mission.RoleBriefing)
	t.Setenv(mission.EnvMission, m.Slug)
	t.Setenv(mission.EnvCrew, crew.ID)
	run(t, w, nil, "mission", "brief", "--set", writeFile(t, t.TempDir(), "b.md", "## Goal\nCache it."))
	run(t, w, nil, "lift", "backend", "cache-lookup")
	r := testutil.RunCommand(t, w.Root, nil, "mission", "ready")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "write the plan first") {
		t.Fatalf("ready without a plan: %v", r.Err)
	}
	plan := "## Z\n- Cached.\n## Stops early\n- None.\n## Steps\n1. Cache it.\n   - who: crew, in `backend/cache-lookup`\n"
	r = run(t, w, nil, "mission", "plan", "--set", writeFile(t, t.TempDir(), "p.md", plan))
	if !strings.Contains(r.Stderr, "plan for "+m.Slug+" written: 1 step") {
		t.Errorf("plan said:\n%s", r.Stderr)
	}
	run(t, w, nil, "mission", "ready")
	if got, _ := store.Load(m.Slug); got.ToBrief() || got.Plan == nil {
		t.Errorf("after ready: %+v", got)
	}
	// Once it's briefed, the plan isn't crew's to change.
	t.Setenv(mission.EnvCrew, "c2")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "plan", "--set", writeFile(t, t.TempDir(), "p.md", plan)); r.Err == nil || !strings.Contains(r.Err.Error(), "the plan is the planner's and the user's") {
		t.Errorf("crew changed the plan: %v", r.Err)
	}
}

// On the mission's page: the plan under the brief, edited there (refused
// with every problem when it doesn't read), and its steps in the details
// with who's on each.
func TestWeb_ThePlan(t *testing.T) {
	w, _ := orchestrateFixture(t)
	act := cli.WebAct(w.Root)
	say := func(text string) (string, error) { return act("cache", web.ActRequest{Verb: "edit-plan", Text: text}) }
	if _, err := say("## Z\n- Cached.\n## Steps\n1. Cache it.\n"); err == nil || !strings.Contains(err.Error(), "line 4: step 1 doesn't say what kind of step it is") || !strings.Contains(err.Error(), "it has no ## Stops early") {
		t.Errorf("a malformed plan: %v", err)
	}
	plan := "## Z\n- Cached.\n## Stops early\n- None.\n## Steps\n1. Cache it.\n   - who: crew, in `backend/cache-lookup`\n   - makes: a pull request\n2. Look at it.\n   - who: you\n"
	if said, err := say(plan); err != nil || said != "Wrote the plan: 2 steps." {
		t.Fatalf("edit-plan: %q, %v", said, err)
	}
	d, err := cli.WebMission(w.Root)("cache", false)
	if err != nil {
		t.Fatal(err)
	}
	if d.Plan != plan || d.PlanProblem != "" || len(d.Stages) != 2 {
		t.Fatalf("detail: plan %q, problem %q, stages %+v", d.Plan, d.PlanProblem, d.Stages)
	}
	if s := d.Stages[0]; s.Who != "crew" || s.WhoSays != "crew, in `backend/cache-lookup`" || s.Makes != "a pull request" || s.State != "current" {
		t.Errorf("step 1 = %+v", s)
	}
	if s := d.Stages[1]; s.Who != "you" || s.State != "todo" {
		t.Errorf("step 2 = %+v", s)
	}
	// A plan changed by hand that doesn't read is said, and the mission
	// works from its brief.
	os.WriteFile(mission.Store{Root: w.Root}.PlanPath("cache"), []byte("## Steps\n- oops\n"), 0o644)
	d, _ = cli.WebMission(w.Root)("cache", false)
	if !strings.Contains(d.PlanProblem, "isn't numbered") || len(d.Stages) != 0 {
		t.Errorf("a plan that doesn't read: %q, stages %+v", d.PlanProblem, d.Stages)
	}
}
