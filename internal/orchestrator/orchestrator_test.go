package orchestrator

import (
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

func TestRun_TimeoutKillsWhatItStarted(t *testing.T) {
	old := Timeout
	Timeout = 300 * time.Millisecond
	t.Cleanup(func() { Timeout = old })

	// The agent starts a long-lived child that holds its output open, as a
	// shell running `gh pr checks --watch` would.
	pidFile := filepath.Join(t.TempDir(), "child")
	l := harness.Launch{Dir: t.TempDir(), Args: []string{"/bin/sh", "-c", `sleep 30 & echo $! > ` + pidFile + `; echo started; wait`}}
	start := time.Now()
	out, err := Run(l, t.TempDir(), "m", "run-1")
	took := time.Since(start)
	t.Logf("returned after %s", took)
	if took > 5*time.Second {
		t.Fatalf("took %s: Run waited on the child", took)
	}
	if err == nil || !strings.Contains(err.Error(), "took longer than") || !strings.Contains(out.Output, "started") {
		t.Errorf("err = %v, out = %q", err, out.Output)
	}
	data, _ := os.ReadFile(pidFile)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	deadline := time.Now().Add(2 * time.Second)
	for pid != 0 && running(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 || running(pid) {
		t.Errorf("child %d outlived the timeout", pid)
	}
}

// running reports whether pid is a live process: not gone, and not a zombie
// waiting for init to reap it (which kill -0 can't tell apart).
func running(pid int) bool {
	out, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	stat := strings.TrimSpace(string(out))
	return stat != "" && !strings.HasPrefix(stat, "Z")
}

func TestClip_KeepsWholeCharactersAndLines(t *testing.T) {
	s := strings.Repeat("“quoted” ─ line…\n", 200)
	for limit := 10; limit < 300; limit += 7 {
		h, tl := clipHead(s, limit, "…"), clipTail(s, limit, "…")
		if !utf8.ValidString(h) || !utf8.ValidString(tl) {
			t.Fatalf("limit %d split a character: %q / %q", limit, h, tl)
		}
		if len(h) > limit || len(tl) > limit {
			t.Fatalf("limit %d exceeded: %d / %d", limit, len(h), len(tl))
		}
	}
	if got := clipTail("short", 100, "…"); got != "short" {
		t.Errorf("clipped what fit: %q", got)
	}
	tail := clipTail(strings.Repeat("a\n", 100)+"last line\n", 40, "…")
	if !strings.HasPrefix(tail, "…") || !strings.HasSuffix(tail, "last line\n") || strings.HasPrefix(tail, "…\n") {
		t.Errorf("tail = %q", tail)
	}
}

func TestPrompt_AsksForANameWhileTitledWithTheRequest(t *testing.T) {
	m := &mission.Mission{Slug: "the-import-job", Title: "The import job keeps hitting the rate limiter…", Request: "The import job keeps hitting the rate limiter on large orgs"}
	if p := Prompt(Input{Mission: m}); !strings.Contains(p, "give it a name with ws mission title first") || !strings.Contains(p, `ws mission title "<terse name>"`) {
		t.Errorf("prompt doesn't ask for a name:\n%s", p)
	}
	m.Title = "Rate limits for large-org imports"
	if p := Prompt(Input{Mission: m}); strings.Contains(p, "give it a name") {
		t.Error("asked to name a named mission")
	}
}

func TestWritePrompt_OneLinePointingAtTheFile(t *testing.T) {
	store := mission.Store{Root: storetest.Own(t, t.TempDir()).Root()}
	m, _ := store.Create("Cache")
	prompt := strings.Repeat("a long\nmulti-line prompt\n", 5000)
	line, err := WritePrompt(store, m.Slug, "run-1", prompt)
	if err != nil {
		t.Fatal(err)
	}
	path := PromptFile(store, m.Slug, "run-1")
	if strings.Contains(line, "\n") || len(line) > 300 || !strings.Contains(line, " "+path+" ") {
		t.Errorf("line = %q", line)
	}
	if data, _ := os.ReadFile(path); string(data) != prompt {
		t.Error("the file doesn't hold the prompt")
	}
}

func TestRun_Logged(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	os.MkdirAll(root+"/.ws", 0o755)
	Run(harness.Launch{Dir: root, Args: []string{"/bin/sh", "-c", "echo hi; exit 3"}}, root, "cache", "run-1")
	data, _ := os.ReadFile(root + "/.ws/exec.log")
	log := string(data)
	for _, want := range []string{"[orchestrator cache] start pid", "$ /bin/sh -c 'echo hi; exit 3'", "WS_ROLE=orchestrator", "[orchestrator cache] end pid", "exit status 3 · 3 bytes of output"} {
		if !strings.Contains(log, want) {
			t.Errorf("exec log missing %q:\n%s", want, log)
		}
	}
}

// Run because crew reported done, the orchestrator reads its handoff in
// full and less of its output; crew that just ended get the long tail.
func TestPrompt_HandoffAndTail(t *testing.T) {
	now := time.Now()
	m := &mission.Mission{Slug: "cache", Title: "Cache", Crew: []mission.Crew{{ID: "c1", Dispatch: "write the ADR", HandedOff: &now}}}
	output := strings.Repeat("x", 20<<10) + "\nTHE END"
	p := Prompt(Input{Mission: m, CrewID: "c1", Trigger: "done", Output: output, Handoff: "# Wrote the ADR\n\n## Learned\n- the cache key needs the org"})
	for _, want := range []string{"## c1's handoff", "the cache key needs the org", "kept as mission:handoffs/c1.md", "THE END"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if n := strings.Count(p, "x"); n > 9<<10 {
		t.Errorf("%d bytes of output with a handoff, want at most about 8 KB", n)
	}
	if n := strings.Count(Prompt(Input{Mission: m, CrewID: "c1", Trigger: "ended", Output: output}), "x"); n < 19<<10 {
		t.Errorf("only %d bytes of output without a handoff", n)
	}
}

// The orchestrator is told the stages' progress and which stage the crew
// was on, and can accept it.
func TestPrompt_Stages(t *testing.T) {
	m := &mission.Mission{Slug: "cache", Title: "Cache"}
	p := Prompt(Input{Mission: m, CrewID: "c2", Trigger: "done", CrewStage: "1 of 2: Write the proposal",
		Stages: []string{"1. [current] Write the proposal (crew c2)", "2. [to do] Implement it"}})
	for _, want := range []string{"It was working on stage 1 of 2: Write the proposal", "## Stage progress", "- 1. [current] Write the proposal (crew c2)", `ws mission accept "<why`} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(Prompt(Input{Mission: m}), "## Stage progress") {
		t.Error("stage progress shown with no stages")
	}
}

// The process is every mission's, as it's written, and what ws holds is
// said apart from it. A mission with a plan is told less of its route, as
// ws holds it.
func TestPrompt_ProcessAndThePlansRoute(t *testing.T) {
	process := "Bugs are reproduced first. Don't close a bug's mission on merge.\n"
	guarantees := []string{"ws opens a pull request only once the user's latest review approves it"}
	brief := Prompt(Input{Mission: &mission.Mission{Slug: "cache", Title: "Cache"}, Process: process, Guarantees: guarantees})
	for _, want := range []string{"## The process", "Don't close a bug's mission on merge.", "## What ws holds", "- ws opens a pull request only once the user's latest review approves it.", "Leave the mission in someone's court"} {
		if !strings.Contains(brief, want) {
			t.Errorf("a mission without a plan: prompt missing %q", want)
		}
	}
	plan, _ := mission.ParsePlan("## Z\n- x\n## Stops early\n- None.\n## Steps\n1. a\n   - who: crew\n", nil)
	planned := Prompt(Input{Mission: &mission.Mission{Slug: "cache", Title: "Cache", Plan: plan}, Process: process})
	for _, want := range []string{"## The process", "Move the mission along its plan", "ws holds the route", "## The plan"} {
		if !strings.Contains(planned, want) {
			t.Errorf("a mission with a plan: prompt missing %q", want)
		}
	}
	for _, gone := range []string{"Leave the mission in someone's court", "An artifact still a draft", "Don't dispatch the next step yourself", "## What ws holds"} {
		if strings.Contains(planned, gone) {
			t.Errorf("a mission with a plan is still told %q", gone)
		}
	}
	if len(planned) >= len(brief) {
		t.Errorf("the plan's prompt (%d) isn't shorter than the brief's (%d)", len(planned), len(brief))
	}
	if none := Prompt(Input{Mission: &mission.Mission{Slug: "cache"}}); strings.Contains(none, "## The process") || strings.Contains(none, "## What ws holds") {
		t.Error("a process shown with none")
	}
}

// A submitted review comes to the orchestrator whole, with who worked in
// the capsule, and it's told to have it addressed, not to touch it.
func TestPrompt_Review(t *testing.T) {
	m := &mission.Mission{Slug: "cache", Title: "Cache"}
	p := Prompt(Input{Mission: m, Trigger: "review", Review: "# Review r2 of api/fix: changes requested\n\n### r2.c1 · blocking · a.go:3 at abc1234\n",
		ReviewRef: "mission:reviews/r2.md", ReviewCrew: []string{"c1", "c3"}})
	for _, want := range []string{
		"submitted the review below (mission:reviews/r2.md)",
		`ws mission resume-with <crew> "Address the review in mission:reviews/r2.md: …"`,
		"--read mission:reviews/r2.md", "accept it (with the review as the reason)",
		"Never edit or answer the review yourself", "latest last): c1, c3.",
		"### r2.c1 · blocking · a.go:3",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	// With nobody to resume, it's told so rather than left to look.
	p = Prompt(Input{Mission: m, Trigger: "review", Review: "# Review r1", ReviewRef: "mission:reviews/r1.md"})
	if !strings.Contains(p, "No crew has worked in that capsule yet") || !strings.Contains(p, "dispatch fresh crew into it with --read mission:reviews/r1.md") {
		t.Error("no crew: not told to dispatch")
	}
}

// The prompt says what just happened and what's live, and points at ws
// show for the rest: the crew aren't pasted in, and the log is its latest
// lines.
func TestPrompt_Lean(t *testing.T) {
	m := &mission.Mission{Slug: "cache", Title: "Cache", Crew: []mission.Crew{{ID: "c1", Dispatch: "write the ADR"}}}
	var log []string
	for i := range 40 {
		log = append(log, "event "+strconv.Itoa(i))
	}
	in := Input{Mission: m, CrewID: "c1", Trigger: "done", Stages: []string{"1. [current] Write the proposal"},
		Log: log, Standing: "the user's court",
		Capsules: []Capsule{{Name: "backend/cache", Path: "/w/repos/backend/cache", Exists: true, Branch: "cache", DiffStat: " a.go | 2 +-"}}}
	p := Prompt(in)
	for _, want := range []string{"Start with `ws show -f llm`", "## Stage progress", "1. [current] Write the proposal", "## Where it stands", "## Capsules", " a.go | 2 +-", "event 39", "event 25"} {
		if !strings.Contains(p, want) {
			t.Errorf("lean prompt missing %q", want)
		}
	}
	for _, never := range []string{"## Crew", "event 24\n"} {
		if strings.Contains(p, never) {
			t.Errorf("lean prompt has %q", never)
		}
	}
}

// The orchestrator is told what the mission waits on the user for, so it
// doesn't ask what's asked already; with nothing, nothing.
func TestPrompt_WaitingOnTheUser(t *testing.T) {
	m := &mission.Mission{Slug: "cache", Title: "Cache", Tasks: []mission.Task{
		{N: 2, Verb: mission.TaskReview, Step: 2, Text: "Step 2 is yours:\nis it right?", By: "ws"},
		{N: 3, Verb: mission.TaskAnswer, Text: "CI is red", By: "orchestrator"},
	}}
	p := Prompt(Input{Mission: m})
	for _, want := range []string{"## Waiting on the user", "- task 2 (review, step 2, from ws): Step 2 is yours: is it right?", "- task 3 (answer, from orchestrator): CI is red", "asking again (ws mission flag) takes the place of your own open question"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(Prompt(Input{Mission: &mission.Mission{Slug: "cache"}}), "## Waiting on the user") {
		t.Error("waiting on the user with nothing open")
	}
}

// What the user said is to change at a Review step is sent back to the
// crew who made it: it isn't an answer to the orchestrator's question.
func TestPrompt_SentBackFromAReview(t *testing.T) {
	m := &mission.Mission{Slug: "cache", Title: "Cache", Answer: &mission.Answer{
		Question: "Step 2 is yours, to review: the fix.", Text: "name the key's parts", By: "you", Step: 2, Verb: mission.TaskReview}}
	p := Prompt(Input{Mission: m, Trigger: "answer"})
	for _, want := range []string{"Step 2 is the user's, to review, and they've said what's to change", "They said: name the key's parts", "send it back to the crew who made it"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "You flagged the user") {
		t.Error("told it asked")
	}
}

// A step's checks not all met, where the user decided otherwise since the
// plan was written, isn't sent back to go against them or put to the
// planner, which can't change a step that's begun: the user's flagged,
// and what they answer in their own words is acted on, passing the step
// where that's what they said.
func TestPrompt_ChecksTheUserDecidedOtherwise(t *testing.T) {
	plan, _ := mission.ParsePlan("## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Work: a\n   - check: it pins the version\n", nil)
	m := &mission.Mission{Slug: "cache", Title: "Cache", Plan: plan}
	p := Prompt(Input{Mission: m, Trigger: "checks", CrewID: "c1", Round: 1, MaxRounds: 3})
	for _, want := range []string{
		"Where a check is unmet only because the user has decided otherwise since the plan was written",
		"don't ask the planner, which revises what's ahead and can't change a step that's begun: flag the user, saying which check and which decision",
		"Your question at this step is the step's own",
		"never a step that's begun",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the checks prompt lacks %q", want)
		}
	}

	m.Answer = &mission.Answer{Question: "Check 1 asks for a pin you decided against. How should step 1 go?", Text: "accept it with the check waived", By: "you", Step: 1, Verb: mission.TaskChecks}
	p = Prompt(Input{Mission: m, Trigger: "answer"})
	for _, want := range []string{
		"Step 1's checks weren't all met, and the user was asked what's to happen",
		"what they say stands over the checks as they're written",
		"They were asked: Check 1 asks for a pin you decided against. How should step 1 go?",
		"They answered: accept it with the check waived",
		"accept the step (ws mission accept, with what they said as the reason): ws lets you, on their answer",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the answer prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "You flagged the user, and they've answered") {
		t.Error("told it as a plain question")
	}
}

// A change to a step's pull request that asks nothing of anyone by the
// time it's looked at (a check that passes now, a bot's summary) is noted
// and left: the pull request is on the user's page as waiting for review
// or for their merge, and they aren't asked to merge it. Nor is a wait
// said at a Wait step, which says it itself.
func TestPrompt_APullRequestWithNothingToDoIsLeft(t *testing.T) {
	plan, _ := mission.ParsePlan("## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Open Pull Request\n   - in: `api/fix`\n", nil)
	p := Prompt(Input{Mission: &mission.Mission{Slug: "cache", Title: "Cache", Plan: plan}, Trigger: "pr", PRStep: "1 of 1", Round: 1, MaxRounds: 3,
		PREvents: []string{"api#5's checks are failing"}})
	for _, want := range []string{
		"Where what happened asks nothing of anyone by the time you look",
		"note what you found and leave it there",
		"so don't ask them to merge it",
		"a step that isn't a Wait waits on something ws can't see happen",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the pull request prompt lacks %q", want)
		}
	}
}
