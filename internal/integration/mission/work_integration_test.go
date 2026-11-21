package mission_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// fakeJudge judges as Claude Code would print it: its verdict, a line a
// check.
type fakeJudge struct{}

// judged is what fakeJudge says.
var judged = "1. met: cache.go keeps each lookup"

func (fakeJudge) Name() string     { return "fake-judge" }
func (fakeJudge) Available() error { return nil }
func (fakeJudge) Start(harness.Brief) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (fakeJudge) Resume(string, string) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (fakeJudge) ConverseModel() string { return "judge-model" }
func (fakeJudge) Converse(r harness.ConverseRun) (harness.Launch, error) {
	said := strings.ReplaceAll(judged, "\n", `\n`)
	if !strings.Contains(r.Prompt, "You are judging work done on the mission") {
		said = "not a judge's prompt"
	}
	script := fmt.Sprintf(`printf '%%s\n' '{"type":"system","subtype":"init","session_id":"j1"}' '{"type":"result","subtype":"success","is_error":false,"result":"%s","session_id":"j1"}'`, said)
	return harness.Launch{Args: []string{"/bin/sh", "-c", script}, Dir: r.Dir, Stream: harness.StreamClaude}, nil
}

// workPlan's first step has checks (one ws runs, one the judge judges)
// and a limit; its second carries on from it.
const workPlan = "## Z\n- Cached.\n## Stops early\n- None.\n## Steps\n" +
	"1. Work: cache the lookup.\n   - in: `backend`\n   - limit: 1h, less if you like\n   - check: `test -f cache.go` passes\n   - check: the lookup is cached\n" +
	"2. Work: test it.\n   - carrying on from: 1\n" +
	"3. Review: the cache.\n"

// Crew report done at a Work step with checks: ws checks it, and the step
// passes when they're met, or the orchestrator's told what isn't (and
// can't pass it); what it came to is shown, with the clock ws holds.
func TestWork_ChecksWhenCrewReportDone(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	harness.Register(fakeJudge{})
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[judge]\nharness = \"fake-judge\"\n")...), 0o644)
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()
	testutil.WithStdin(t, workPlan)
	testutil.Run(t, w, nil, "mission", "plan", "-m", "cache", "--set", "-")
	store.JoinStage("cache", "c1", 1)

	out := testutil.Run(t, w, nil, "show", "-m", "cache", "-f", "llm", "s1").Stdout
	for _, want := range []string{"Its limit: 1h, less if you like.", "It's done when these are true", "1. `test -f cache.go` passes", "Its time runs out at"} {
		if !strings.Contains(out, want) {
			t.Errorf("ws show s1 lacks %q:\n%s", want, out)
		}
	}

	// Crew take less time than the step gives them.
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c1")
	if r := testutil.Run(t, w, nil, "mission", "limit", "45m"); !strings.Contains(r.Stderr, "step 1's time runs out at") {
		t.Errorf("ws mission limit: %s", r.Stderr)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "limit", "an hour"); r.Err == nil || !strings.Contains(r.Err.Error(), "say it as 45m, 1h or 1h30m") {
		t.Errorf("a limit of no length: %v", r.Err)
	}

	// Done, with cache.go missing and the judge unconvinced: the
	// orchestrator's told, and can't pass it.
	judged = "1. unmet: nothing keeps a lookup"
	r := testutil.Run(t, w, nil, "mission", "done", "Cached it", "--handoff", writeHandoff(t, "Nothing."))
	if !strings.Contains(r.Stderr, "ws runs step 1's checks") {
		t.Errorf("ws mission done: %s", r.Stderr)
	}
	if strings.Join(*runs, ",") != "cache/c1/checks" {
		t.Errorf("orchestrator runs = %v", *runs)
	}
	t.Setenv(mission.EnvCrew, "")
	prompt := testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "checks", "--print-prompt").Stdout
	for _, want := range []string{"not all are met (round 1; after round 3 it's the user's", "`test -f cache.go` passes: unmet (by ws)", "the lookup is cached: unmet (by the judge (fake-judge:judge-model)): nothing keeps a lookup", "c1's handoff"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the orchestrator's prompt lacks %q", want)
		}
	}
	t.Setenv("WS_ROLE", "orchestrator")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "accept", "-m", "cache", "done"); r.Err == nil || !strings.Contains(r.Err.Error(), "step 1 has checks, and ws passes it once they're all met (round 1 came to 0 of 2 met") {
		t.Errorf("the orchestrator passing it: %v", r.Err)
	}
	t.Setenv("WS_ROLE", "")

	// Put right, and done again: met, and passed by ws.
	testutil.CommitIn(t, testutil.CapsuleDir(w, "backend", "cache-lookup"), "cache.go", "package cache\n", "Cache the lookup")
	judged = "1. met: cache.go keeps each lookup"
	t.Setenv(mission.EnvCrew, "c1")
	if r := testutil.Run(t, w, nil, "mission", "done", "Cached it, properly", "--handoff", writeHandoff(t, "Nothing.")); !strings.Contains(r.Stderr, "ws runs step 1's checks") {
		t.Errorf("ws mission done, again: %s", r.Stderr)
	}
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvMission, "")
	m := testutil.LoadMission(t, w, "cache")
	states, _ := m.StageProgress("")
	if !states[0].Accepted() || states[0].Record.AcceptedBy != "ws" || len(states[0].Record.Checks) != 2 {
		t.Fatalf("step 1 = %+v", states[0].Record)
	}
	out = testutil.Run(t, w, nil, "show", "-m", "cache", "-f", "llm", "s1").Stdout
	for _, want := range []string{"Checks, round 1, of c1's done", "Checks, round 2, of c1's done", "2 of 2 met", "- `test -f cache.go` passes: met, by ws: passed in"} {
		if !strings.Contains(out, want) {
			t.Errorf("ws show s1 lacks %q:\n%s", want, out)
		}
	}
	if !logged(t, store, "step 1's checks, round 2 (c1's done): 2 of 2 met") {
		t.Error("the round isn't logged")
	}
	// Each check's start, end and result, and the judge's, are in the
	// step's Lately list.
	_, lately, _ := strings.Cut(testutil.Run(t, w, nil, "show", "-m", "cache", "-f", "llm", "--full", "s1").Stdout, "\nLately:\n")
	for _, want := range []string{
		// How long each took isn't said here: it's a run now, and a slow
		// machine's is a second.
		"step 1, check 1 of 2 started: `test -f cache.go`", "step 1, check 1 of 2: `test -f cache.go` failed (exit 1) in ", "s: unmet",
		"step 1, check 1 of 2: `test -f cache.go` passed in ", "s: met",
		"step 1: the judge (fake-judge:judge-model) started reading check 2", "step 1: the judge (fake-judge:judge-model) finished in ", ": check 2 met",
	} {
		if !strings.Contains(lately, want) {
			t.Errorf("s1's Lately list lacks %q:\n%s", want, lately)
		}
	}
}
