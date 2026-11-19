package orchestrate_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/orchestrator"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

func TestOrchestrate_PromptHasTheContext(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	r := testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "done", "--print-prompt")
	for _, want := range []string{
		"orchestrator for the mission \"Cache\" (cache)",
		"Crew c1 has just reported that its part is done",
		"Start with `ws show -f llm`: the mission's brief",
		"backend/cache-lookup at " + filepath.Join(w.Root, "repos", "backend", "cache-lookup"),
		"ws mission resume-with <crew>", "ws mission conclude", "ws mission new",
		"The end of c1's output", "All tests pass.",
	} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if !strings.Contains(r.Stderr, "would write that to") || !strings.Contains(r.Stderr, ".prompt.md and run:") {
		t.Errorf("stderr: %s", r.Stderr)
	}
}

func TestOrchestrate_RunsAsOrchestratorAndLogs(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "done", "--auto")
	events, _ := store.ReadLog("cache", 0)
	var reviews []string
	for _, e := range events {
		if e.Kind == mission.EventOrchestrate || e.Kind == mission.EventOrchestrateRun {
			reviews = append(reviews, e.Text)
		}
	}
	// With no [orchestrator] model, the harness's own default; it's told
	// the mission's capsule paths (Claude Code allows git -C on exactly those).
	// The echo orchestrator runs no ws commands, so it's run once more
	// (TestOrchestrate_ARunThatDoesNothingIsCaught).
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	if len(reviews) != 4 || !strings.Contains(reviews[0], "with echo-orchestrator echo-small") || !strings.Contains(reviews[1], "ran as orchestrator on cache with echo-small in "+capsule) ||
		!strings.Contains(reviews[2], "running it again") {
		t.Fatalf("review events = %q", reviews)
	}
	// That each event is looked at once is ops.Notify's (TestNotify).
}

func TestOrchestrate_ConfigTurnsAutoOff(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	os.WriteFile(filepath.Join(w.Root, "ws.local.toml"), []byte("[orchestrator]\nenabled = false\nmodel = \"other\"\n"), 0o644)
	testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--auto")
	for _, e := range mustLog(t, store) {
		if e.Kind == mission.EventOrchestrate || e.Kind == mission.EventOrchestrateRun {
			t.Fatal("auto review ran with [orchestrator] enabled = false")
		}
	}
	// A manual review still runs, with the configured model.
	testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache")
	found := false
	for _, e := range mustLog(t, store) {
		if e.Kind == mission.EventOrchestrate && strings.Contains(e.Text, "with other") {
			found = true
		}
	}
	if !found {
		t.Error("manual review didn't use the configured model")
	}
}

func mustLog(t *testing.T, store mission.Store) []mission.Event {
	t.Helper()
	events, err := store.ReadLog("cache", 0)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func TestProposals_FlagResumeWithConclude(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv("WS_ROLE", "orchestrator")

	testutil.Run(t, w, nil, "mission", "flag", "CI is red on the main branch")
	testutil.Run(t, w, nil, "mission", "resume-with", "c1", "the error path has no test")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "conclude", "PR merged", "--as", "landed", "--met", "1: go test passes"); r.Err == nil || !strings.Contains(r.Err.Error(), "neither merged nor landed") {
		t.Errorf("landed with nothing merged: %v", r.Err)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "conclude", "PR merged", "--as", "answered"); r.Err == nil || !strings.Contains(r.Err.Error(), "part 1 of Z (tests pass) isn't checked") {
		t.Errorf("Z unchecked: %v", r.Err)
	}
	testutil.Run(t, w, nil, "mission", "conclude", "PR merged, tests pass", "--as", "answered", "--met", "1: go test passes in api/cache")

	got, _ := store.Load("cache")
	if got.Asking() == nil || got.Asking().Text != "CI is red on the main branch" || got.Asking().By != "orchestrator" {
		t.Errorf("flag = %+v", got.Asking())
	}
	if got.Crew[0].ResumeWith != "the error path has no test" {
		t.Errorf("resume-with = %q", got.Crew[0].ResumeWith)
	}
	if got.Conclude == nil || got.Conclude.By != "orchestrator" {
		t.Errorf("conclude = %+v", got.Conclude)
	}
	show := testutil.Run(t, w, nil, "mission", "show", "-f", "llm")
	for _, want := range []string{"Waits on you, task 1 (answer, by orchestrator): CI is red", "Suggested: resume c1 with: the error path",
		"Suggested ending (orchestrator, for the user to end it): answered (at Z: 1 of 1 met): PR merged", "Z 1 met: go test passes in api/cache (tests pass)"} {
		if !strings.Contains(show.Stdout, want) {
			t.Errorf("show missing %q:\n%s", want, show.Stdout)
		}
	}

	// Mistakes come back as errors the orchestrator can correct.
	for args, want := range map[string]string{
		"resume-with c9 x": "has no crew c9 (it has: c1)",
	} {
		r := testutil.RunCommand(t, w.Root, nil, append([]string{"mission"}, strings.Fields(args)...)...)
		if r.Err == nil || !strings.Contains(r.Err.Error(), want) {
			t.Errorf("%s: err = %v", args, r.Err)
		}
	}
	store.StartCrew("cache", "shell", "")
	r := testutil.RunCommand(t, w.Root, nil, "mission", "resume-with", "c2", "go on")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "recorded no conversation") {
		t.Errorf("resume-with crew without conversation: %v", r.Err)
	}
	testutil.Run(t, w, nil, "mission", "flag", "--clear")
	if got, _ := store.Load("cache"); got.Asking() != nil {
		t.Error("flag not cleared")
	}
}

// fileOrchestrator stands in for an agent that follows the one line it's
// given: it reports how many lines that was, then reads the file it names.
type fileOrchestrator struct{ testutil.EchoOrchestrator }

func (fileOrchestrator) Name() string { return "file-orchestrator" }
func (fileOrchestrator) Orchestrate(r harness.OrchestratorRun) (harness.Launch, error) {
	script := `printf 'lines=%s\n' "$(printf '%s' "$0" | wc -l | tr -d ' ')"; for w in $0; do case "$w" in /*.prompt.md) head -n 1 "$w";; esac; done`
	return harness.Launch{Args: []string{"/bin/sh", "-c", script, r.Prompt}, Dir: r.Dir}, nil
}

func TestOrchestrate_PromptGoesInAFileNotTheCommandLine(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	harness.Register(fileOrchestrator{})
	t.Setenv(harness.EnvDefault, "file-orchestrator")
	testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "done")

	data, err := os.ReadFile(latestPrompt(t, store))
	if err != nil || !strings.Contains(string(data), "Start with `ws show -f llm`") || !strings.Contains(string(data), "All tests pass.") {
		t.Fatalf("prompt file = %q, %v", data, err)
	}
	var summary string
	for _, e := range mustLog(t, store) {
		if e.Kind == mission.EventOrchestrate {
			summary = e.Text
		}
	}
	// One line on the command line, and the agent found the prompt from it.
	if !strings.Contains(summary, "lines=0") || !strings.Contains(summary, `You are the orchestrator for the mission "Cache"`) {
		t.Errorf("summary = %q", summary)
	}
}

func TestMissionErrors_NoUsageText(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	t.Setenv(mission.EnvMission, "cache")
	r := testutil.RunCommand(t, w.Root, nil, "mission", "resume-with", "c9", "x")
	if r.Err == nil || strings.Contains(r.Stderr+r.Stdout, "Usage:") {
		t.Errorf("a failed mission command printed usage:\n%s", r.Stderr)
	}
	// A mistyped flag still gets it.
	r = testutil.RunCommand(t, w.Root, nil, "mission", "resume-with", "--nope")
	if r.Err == nil || !strings.Contains(r.Stderr+r.Stdout, "Usage:") {
		t.Errorf("a mistyped flag didn't print usage:\n%s", r.Stderr)
	}
}

// A quiet turn's run is told why it's running, what the crew last said, and
// where the mission stands if it does nothing; it's taught to leave the
// ball in someone's court.
func TestOrchestrate_QuietTurnPrompt(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.CrewHook("cache", "c1", mission.Hook{Activity: mission.ActivityWaiting, Reply: "Added the cache. Shall I write tests too?"})
	r := testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "quiet", "--print-prompt")
	for _, want := range []string{
		"Crew c1 has just finished a turn without reporting done or asking the user anything",
		"Its last message:\n\nAdded the cache. Shall I write tests too?",
		"## Where it stands\n\nIf this run does nothing: the user's court: c1 is waiting for you: Added the cache.",
		"Leave the mission in someone's court",
		`ws mission wait "<what it waits on>"`,
		"repos/<repo>/ itself isn't a repository",
	} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("prompt missing %q:\n%s", want, r.Stdout)
		}
	}
}

// resume-with on crew still running types it in when the daemon has
// it; without one, it's left for you to pass on, and the orchestrator's
// allowance isn't used up. ws mission wait puts the mission outside.
func TestMission_ResumeWithRunningCrewAndWait(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.CrewHook("cache", "c1", mission.Hook{Activity: mission.ActivityWaiting, Reply: "Added the cache."})
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvRole, mission.RoleOrchestrator)
	r := testutil.Run(t, w, nil, "mission", "resume-with", "c1", "write the tests")
	if !strings.Contains(r.Stderr, "couldn't type it into c1") {
		t.Errorf("stderr = %s", r.Stderr)
	}
	m, _ := store.Load("cache")
	if m.Crew[0].ResumeWith != "write the tests" || m.Dispatches != 0 {
		t.Errorf("crew %+v, dispatches %d", m.Crew[0], m.Dispatches)
	}

	t.Setenv(mission.EnvRole, "")
	show := testutil.Run(t, w, nil, "mission", "show", "-f", "llm")
	if !strings.Contains(show.Stdout, "Repos' default branches") || !strings.Contains(show.Stdout, "  backend: main at ") {
		t.Errorf("show doesn't say where the default branches are:\n%s", show.Stdout)
	}
	if !strings.Contains(show.Stdout, "Where it stands: the user's court: c1 should carry on: write the tests") {
		t.Errorf("show:\n%s", show.Stdout)
	}

	store.EndCrewStopped("cache", "c1", "hangup", true)
	store.SetResumeWith("cache", "c1", "")
	t.Setenv(mission.EnvRole, mission.RoleOrchestrator)
	testutil.Run(t, w, nil, "mission", "wait", "the platform team's deploy")
	t.Setenv(mission.EnvRole, "")
	show = testutil.Run(t, w, nil, "mission", "show", "-f", "llm")
	if !strings.Contains(show.Stdout, "Where it stands: outside (waiting on someone else: reviewers, checks, a merge, what a Wait step waits for): Waiting on the platform team's deploy") {
		t.Errorf("show:\n%s", show.Stdout)
	}
}

// resume-with on crew that have ended is left on the page for you, unless
// they'd handed back their work (closed once they reported done) and ws may
// start crew without you: then it's they who are resumed.
func TestMission_ResumeWithEndedCrew(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvRole, mission.RoleOrchestrator)

	// Stopped mid-work: yours to resume.
	store.EndCrewStopped("cache", "c1", "hangup", true)
	r := testutil.Run(t, w, nil, "mission", "resume-with", "c1", "write the tests")
	if strings.Contains(r.Stderr, "couldn't") || !strings.Contains(r.Stderr, "the mission's page offers to resume c1") {
		t.Errorf("stderr = %s", r.Stderr)
	}

	// Closed after reporting done, with crew waiting for you to start them.
	store.SetResumeWith("cache", "c1", "")
	store.Update("cache", func(m *mission.Mission) error { m.Crew[0].Status = mission.CrewDone; return nil })
	os.WriteFile(filepath.Join(w.Root, "ws.local.toml"), []byte("[orchestrator]\ndispatch = \"ask\"\n"), 0o644)
	r = testutil.Run(t, w, nil, "mission", "resume-with", "c1", "write the tests")
	if strings.Contains(r.Stderr, "couldn't") || !strings.Contains(r.Stderr, "the mission's page offers to resume c1") {
		t.Errorf("stderr = %s", r.Stderr)
	}
	if m, _ := store.Load("cache"); m.Dispatches != 0 {
		t.Errorf("dispatches %d, want none: nothing was started", m.Dispatches)
	}
}

// A PR's change is told as such, what happened since the orchestrator's
// last run listed: what's done about it is the process's to say.
func TestOrchestrate_PRChangePrompt(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.AppendLog("cache", mission.Event{Kind: mission.EventPR, Repo: "backend", Capsule: "cache-lookup", Text: "backend#11 was approved"})
	store.AppendLog("cache", mission.Event{Kind: mission.EventOrchestrate, Text: "noted it"})
	store.AppendLog("cache", mission.Event{Kind: mission.EventPR, What: mission.PREventChecksFailed, Repo: "backend", Capsule: "cache-lookup", Text: "backend#12's checks are failing"})
	r := testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--trigger", "pr", "--print-prompt")
	if strings.Contains(r.Stdout, "- backend#11 was approved") {
		t.Error("told what happened before its last run")
	}
	for _, want := range []string{"One of the mission's pull requests has changed on GitHub", "What's done about it is the process's to say", "What happened, since your last run:\n\n- backend#12's checks are failing"} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

// scriptOrchestrator runs $WS_TEST_ORCHESTRATOR in sh as the orchestrator:
// the script can append to the mission's log as its ws commands would.
type scriptOrchestrator struct{ testutil.EchoOrchestrator }

func (scriptOrchestrator) Name() string { return "script-orchestrator" }
func (scriptOrchestrator) Orchestrate(r harness.OrchestratorRun) (harness.Launch, error) {
	return harness.Launch{Args: []string{"/bin/sh", "-c", os.Getenv("WS_TEST_ORCHESTRATOR")}, Dir: r.Dir}, nil
}

// An orchestrator run that runs no ws mission command did nothing: it's
// run once more, told its shell works; if it still does nothing, or can't
// run at all, the mission is the user's, with a question and what it said.
func TestOrchestrate_ARunThatDoesNothingIsCaught(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	harness.Register(scriptOrchestrator{})
	t.Setenv(harness.EnvDefault, "script-orchestrator")
	// What its ws mission note would log, stamped with its run ($WS_RUN):
	// the test binary, as a command reaching the test's records.
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	note := fmt.Sprintf(`WS_TEST_NOTE=%q %q cache "Orchestrator: all good"`, w.Root, exe)
	orchestrate := func() {
		t.Helper()
		testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "done")
	}
	flag := func() *mission.Task { m, _ := store.Load("cache"); return m.Asking() }

	// Acts on the first run: once, and no flag.
	t.Setenv("WS_TEST_ORCHESTRATOR", note)
	orchestrate()
	if f := flag(); f != nil {
		t.Errorf("flagged a run that acted: %+v", f)
	}

	// Acts only when told again.
	marker := filepath.Join(t.TempDir(), "ran")
	t.Setenv("WS_TEST_ORCHESTRATOR", `if [ -f `+marker+` ]; then `+note+`; else touch `+marker+`; echo "**My decision:** flag it, once Bash is available."; fi`)
	orchestrate()
	prompt, _ := os.ReadFile(latestPrompt(t, store))
	if !strings.Contains(string(prompt), "## Your last run") || !strings.Contains(string(prompt), "flag it, once Bash is available") {
		t.Errorf("the second run wasn't told about the first:\n%s", prompt)
	}
	if f := flag(); f != nil {
		t.Errorf("flagged a run that acted the second time: %+v", f)
	}

	// Never acts: the user's, with what it said.
	t.Setenv("WS_TEST_ORCHESTRATOR", `echo "…"; echo; echo "**My decision:** the note is accurate; flag the commit."`)
	orchestrate()
	f := flag()
	if f == nil || f.By != "ws" || !strings.Contains(f.Text, "The orchestrator looked at c1 (done) but ran no commands, twice") ||
		!strings.Contains(f.Text, "It said: My decision: the note is accurate; flag the commit.") {
		t.Fatalf("flag = %+v", f)
	}

	// Can't run (a sign-in that's run out, say): the user's, saying so.
	store.Ask("cache", "", "ws")
	t.Setenv("WS_TEST_ORCHESTRATOR", `echo "not signed in" >&2; exit 3`)
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "done"); r.Err == nil {
		t.Error("a run that failed succeeded")
	}
	if f := flag(); f == nil || !strings.Contains(f.Text, "The orchestrator couldn't run on c1 (done)") || !strings.Contains(f.Text, "not signed in") {
		t.Errorf("flag = %+v", f)
	}
}

// Each run is kept: what it looked at, its prompt, what it said, whether it
// acted; a retry says so.
func TestOrchestrate_RunsAreKept(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "done")
	runs := orchestrator.Runs(store, "cache")
	if len(runs) != 2 || !runs[0].Retry || runs[1].Retry || runs[1].For != "c1 (done)" || runs[1].Acted || runs[1].Harness != "echo-orchestrator" {
		t.Fatalf("runs = %+v", runs)
	}
	r, _ := orchestrator.LoadRun(store, "cache", runs[0].ID)
	if !strings.Contains(r.Prompt, "## Your last run") || !strings.Contains(r.Output, "ran as orchestrator") {
		t.Errorf("retry run = %+v", r)
	}
}

// The web UI's orchestrator page: how it's set up, and its runs, each in
// full with its prompt; a mission's page lists its own.
func TestWebOrchestrator(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "done")
	info, err := testutil.WebOrchestrator(w.Root)()
	if err != nil {
		t.Fatal(err)
	}
	if !info.Enabled || info.Harness != "echo-orchestrator" || info.Dispatch != "auto" || info.MaxDispatches != 3 || len(info.Runs) != 2 {
		t.Fatalf("info = %+v", info)
	}
	r := info.Runs[1] // the first run; the retry is newest
	if r.Mission != "cache" || r.MissionTitle != "Cache" || r.For != "c1 (done)" || r.Acted || !info.Runs[0].Retry {
		t.Errorf("run = %+v", r)
	}
	d, err := testutil.WebOrchestratorRun(w.Root)("cache", r.ID)
	if err != nil || !strings.Contains(d.Prompt, "orchestrator for the mission") || !strings.Contains(d.Output, "ran as orchestrator") {
		t.Errorf("detail = %+v, %v", d, err)
	}
	if _, err := testutil.WebOrchestratorRun(w.Root)("cache", "nope"); !errors.Is(err, web.ErrNotFound) {
		t.Errorf("no such run: %v", err)
	}
	if md, _ := testutil.WebMission(w.Root)("cache", false); len(md.Runs) != 2 {
		t.Errorf("mission runs = %+v", md.Runs)
	}
}

// latestPrompt is the prompt file of the mission's latest orchestrator run.
func latestPrompt(t *testing.T, store mission.Store) string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(store.Dir(), "cache", "orchestrator", "*.prompt.md"))
	if len(files) == 0 {
		t.Fatal("no prompt file")
	}
	sort.Slice(files, func(i, j int) bool {
		a, _ := os.Stat(files[i])
		b, _ := os.Stat(files[j])
		return a.ModTime().Before(b.ModTime())
	})
	return files[len(files)-1]
}
