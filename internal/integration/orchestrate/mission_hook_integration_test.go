package orchestrate_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// hook runs ws mission hook <event> as an agent's hook would: the crew's
// environment, the event's JSON on stdin.
func hook(t *testing.T, w *testutil.Workspace, event, payload string) {
	t.Helper()
	r, wr, _ := os.Pipe()
	wr.WriteString(payload)
	wr.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old; r.Close() }()
	if res := testutil.RunCommand(t, w.Root, nil, "mission", "hook", "--root", w.Root, event); res.Err != nil {
		t.Fatalf("hook %s: %v", event, res.Err)
	}
}

// What the agents report through their hooks lands on the crew's record:
// working once it's sent a message (which also answers what it said before),
// waiting with its last reply once its turn ends, permission when it asks.
func TestMissionHook_RecordsWhatCrewReport(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.StartCrewAs("cache", "cursor", "", "") // c2, no chat id yet
	store.SetCrewStatus("cache", "c2", mission.CrewDone, "stage one done")
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c2")
	crew := func() mission.Crew {
		m, _ := store.Load("cache")
		return m.Crew[1]
	}

	// Cursor's: its session id is the chat to resume.
	hook(t, w, "start", `{"conversation_id":"conv-77","session_id":"conv-77","hook_event_name":"sessionStart"}`)
	if c := crew(); c.Conversation != "conv-77" {
		t.Errorf("conversation = %q", c.Conversation)
	}
	hook(t, w, "prompt", `{"prompt":"now stage two","hook_event_name":"beforeSubmitPrompt"}`)
	if c := crew(); c.Activity != mission.ActivityWorking || c.Status != "" || c.ActivityAt == nil {
		t.Errorf("after a message: activity %q, status %q", c.Activity, c.Status)
	}
	// Cursor's final message is the last assistant entry in its transcript.
	transcript := filepath.Join(t.TempDir(), "conv-77.jsonl")
	os.WriteFile(transcript, []byte(`{"role":"user","message":{"content":[{"type":"text","text":"now stage two"}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"I'll look at the brief first."}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"Stage two is in: hello.py prints hello."},{"type":"text","text":"Details…"}]}}
{"type":"turn_ended","status":"success"}
`), 0o644)
	hook(t, w, "stop", `{"status":"completed","hook_event_name":"stop","transcript_path":"`+transcript+`"}`)
	if c := crew(); c.Activity != mission.ActivityWaiting || !strings.HasPrefix(c.LastReply, "Stage two is in") {
		t.Errorf("after its turn: activity %q, reply %q", c.Activity, c.LastReply)
	}

	// Claude Code's: the reply comes with Stop; a permission notification.
	hook(t, w, "stop", `{"last_assistant_message":"READY","hook_event_name":"Stop"}`)
	hook(t, w, "notify", `{"message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`)
	if c := crew(); c.LastReply != "READY" || c.Activity != mission.ActivityPermission || c.Notice != "Claude needs your permission to use Bash" {
		t.Errorf("claude: %+v", c)
	}
	hook(t, w, "notify", `{"message":"Claude is waiting for your input","notification_type":"idle_prompt"}`)
	if c := crew(); c.Activity != mission.ActivityPermission || c.Notice != "Claude is waiting for your input" {
		t.Errorf("an idle notice changes only the notice: %+v", c)
	}

	// pi's, from ws's extension: an extension's dialog waiting is a
	// permission; the reply comes with the stop.
	hook(t, w, "prompt", `{}`)
	hook(t, w, "notify", `{"notification_type":"ui_prompt","message":"Allow rm -rf?"}`)
	if c := crew(); c.Activity != mission.ActivityPermission || c.Notice != "Allow rm -rf?" {
		t.Errorf("pi dialog: %+v", c)
	}
	hook(t, w, "stop", `{"last_assistant_message":"Removed the build folder."}`)
	if c := crew(); c.Activity != mission.ActivityWaiting || c.LastReply != "Removed the build folder." {
		t.Errorf("pi stop: %+v", c)
	}

	// Not crew ws launched (no WS_CREW): nothing, and no error.
	t.Setenv(mission.EnvCrew, "")
	before := crew()
	hook(t, w, "prompt", `{}`)
	if c := crew(); c.Activity != before.Activity {
		t.Error("a hook outside crew changed the record")
	}
}

// A turn that ends with crew saying nothing about where it is (no ask, no
// done), after a message ws sent it, is the orchestrator's to look at.
// After your own message it's answering you, so it's yours; and briefing
// crew, crew asking, and a workspace with the orchestrator off are left
// alone.
func TestMissionHook_QuietTurnGoesToTheOrchestrator(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.StartCrewAs("cache", "claude", "", "") // c2
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c2")
	turn := func() {
		t.Helper()
		hook(t, w, "prompt", `{}`)
		hook(t, w, "stop", `{"last_assistant_message":"I've added the cache. Shall I write tests too?"}`)
	}

	turn() // after ws's opening prompt
	if strings.Join(*runs, " ") != "cache/c2/quiet" {
		t.Fatalf("runs = %v", *runs)
	}

	store.TalkedTo("cache", "c2", "") // you replied
	turn()
	if len(*runs) != 1 {
		t.Errorf("your conversation went to the orchestrator: %v", *runs)
	}

	store.Nudged("cache", "c2", "write the tests", "orchestrator") // the orchestrator's nudge
	hook(t, w, "prompt", `{}`)
	store.SetCrewStatus("cache", "c2", mission.CrewAsking, "Table tests or not?")
	hook(t, w, "stop", `{"last_assistant_message":"Asked."}`)
	if len(*runs) != 1 {
		t.Errorf("crew asking went to the orchestrator: %v", *runs)
	}

	store.StartCrewAs("cache", "claude", "", mission.RoleBriefing) // c3
	t.Setenv(mission.EnvCrew, "c3")
	turn()
	if len(*runs) != 1 {
		t.Errorf("a briefing agent's turn went to the orchestrator: %v", *runs)
	}

	// A turn Claude Code crew end with a run_in_background task running is
	// held: Claude Code wakes them when it ends. The turn that wake-up
	// starts, ended with nothing running, is the orchestrator's.
	store.StartCrewAs("cache", "claude", "", "") // c4
	t.Setenv(mission.EnvCrew, "c4")
	hook(t, w, "prompt", `{"prompt":"Stage one: cache the lookup."}`)
	hook(t, w, "stop", stopWithSuiteRunning)
	if len(*runs) != 1 {
		t.Errorf("a turn waiting on its background task went to the orchestrator: %v", *runs)
	}
	if r := testutil.Run(t, w, nil, "show", "-f", "llm", "c4"); !strings.Contains(r.Stdout, "running, waiting on its own work (just test)") || !strings.Contains(r.Stdout, "Its last message (waiting on its own work)") {
		t.Errorf("ws show c4:\n%s", r.Stdout)
	}
	hook(t, w, "prompt", suiteNotification)
	hook(t, w, "stop", `{"hook_event_name":"Stop","last_assistant_message":"The suite passes. Shall I open the PR?","background_tasks":[],"session_crons":[]}`)
	if strings.Join(*runs, " ") != "cache/c2/quiet cache/c4/quiet" {
		t.Errorf("the wake-up's turn: runs = %v", *runs)
	}

	// pi crew whose only work is a process the agent detached (nohup … &),
	// found by the process check: its turn goes to the orchestrator when
	// that process ends, not before.
	store.StartCrewAs("cache", "pi", "", "") // c5
	t.Setenv(mission.EnvCrew, "c5")
	defer cli.UseProcs(crewProcs("cache", "c5", harness.Proc{PID: 5100, PPID: 1, Name: "node", Args: []string{"node", "e2e.js"}}))()
	store.SetCrewProcess("cache", "c5", 5000, "")
	hook(t, w, "prompt", `{"prompt":"Stage two: the end-to-end suite."}`)
	hook(t, w, "stop", `{"message":"Started the end-to-end suite; I'll read it when it's done."}`)
	if len(*runs) != 2 {
		t.Errorf("a turn waiting on its detached work went to the orchestrator: %v", *runs)
	}
	if r := testutil.Run(t, w, nil, "show", "-f", "llm", "c5"); !strings.Contains(r.Stdout, "waiting on its own work (node e2e.js)") {
		t.Errorf("ws show c5:\n%s", r.Stdout)
	}
	live := func(pids ...int) func(int) bool { return func(pid int) bool { return slices.Contains(pids, pid) } }
	cli.WatchHeld(w.Root, time.Now(), live(5100), nil)
	if len(*runs) != 2 {
		t.Errorf("taken while its work runs: %v", *runs)
	}
	cli.WatchHeld(w.Root, time.Now(), live(), nil)
	if strings.Join(*runs, " ") != "cache/c2/quiet cache/c4/quiet cache/c5/quiet" {
		t.Errorf("when its work ended: runs = %v", *runs)
	}
}

// crewProcs is a process table for the process check, as ws's hook command
// sees it under crew's harness (pid 5000, started an hour ago): the hook
// (this test) in a shell of the harness's, and the processes given, started
// a minute ago with the crew's names unless they say otherwise.
func crewProcs(slug, crewID string, procs ...harness.Proc) harness.Procs {
	now := time.Now()
	env := []string{"WS_MISSION=" + slug, "WS_CREW=" + crewID}
	list := []harness.Proc{
		{PID: 1, Start: now.Add(-24 * time.Hour), Name: "launchd", Args: []string{}},
		{PID: 5000, PPID: 1, Start: now.Add(-time.Hour), Name: "agent", Args: []string{"agent"}, Env: env},
		{PID: 5001, PPID: 5000, Start: now, Name: "sh", Args: []string{"/bin/sh", "-c", "ws mission hook stop"}, Env: []string{}},
		{PID: os.Getpid(), PPID: 5001, Start: now, Name: "ws", Args: []string{"ws", "mission", "hook", "stop"}, Env: env},
	}
	for _, p := range procs {
		if p.Start.IsZero() {
			p.Start = now.Add(-time.Minute)
		}
		if p.Env == nil {
			p.Env = env
		}
		list = append(list, p)
	}
	return harness.Procs{List: list}
}

// The process check gives a Claude Code task its pid, from the shell
// Claude Code runs it in, so ws sees it end: Claude Code then has
// mission.WakeGrace to wake the crew. Another shell of Claude Code's (a
// Stop hook beside ws's) isn't work; a process the agent detached is, and
// nothing wakes the crew for it.
func TestMissionHook_ProcessCheckWithClaudeCode(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.StartCrewAs("cache", "claude", "", "") // c2
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c2")
	store.SetCrewProcess("cache", "c2", 5000, "")
	crew := func() mission.Crew { m, _ := store.Load("cache"); return m.Crew[1] }
	defer cli.UseProcs(crewProcs("cache", "c2",
		harness.Proc{PID: 5200, PPID: 5000, Name: "zsh", Args: []string{"/bin/zsh", "-c", "source snap.sh && eval 'just test' < /dev/null && pwd -P >| /tmp/cwd"}, Env: []string{}},
		harness.Proc{PID: 5201, PPID: 5200, Name: "just", Args: []string{"just", "test"}},
		harness.Proc{PID: 5300, PPID: 5000, Name: "sh", Args: []string{"/bin/sh", "-c", "afplay done.aiff"}, Env: []string{}},
		harness.Proc{PID: 5400, PPID: 1, Name: "node", Args: []string{"node", "server.js"}},
	))()

	hook(t, w, "prompt", `{}`)
	hook(t, w, "stop", stopWithSuiteRunning)
	c := crew()
	want := []mission.Work{
		{What: "just test", From: harness.FromHarness, PID: 5200, Wakes: mission.WakeYes},
		{What: "node server.js", From: harness.FromProcesses, PID: 5400, Wakes: mission.WakeNo},
	}
	if c.Pending == nil || !slices.Equal(c.Pending.Work, want) {
		t.Fatalf("what it left running: %+v", c.Pending)
	}
	at := c.Pending.At
	live := func(pids ...int) func(int) bool { return func(pid int) bool { return slices.Contains(pids, pid) } }
	cli.WatchHeld(w.Root, at.Add(time.Minute), live(5200, 5400), nil)
	cli.WatchHeld(w.Root, at.Add(2*time.Minute), live(5400), nil)            // the suite ends
	cli.WatchHeld(w.Root, at.Add(2*time.Minute+30*time.Second), live(), nil) // the server ends
	if len(*runs) != 0 {
		t.Errorf("taken within a minute of the suite ending: %v", *runs)
	}
	cli.WatchHeld(w.Root, at.Add(2*time.Minute+mission.WakeGrace), live(), nil)
	if strings.Join(*runs, " ") != "cache/c2/quiet" {
		t.Errorf("Claude Code didn't wake it: runs = %v", *runs)
	}
}

// Claude Code 2.1.322's Stop with a run_in_background task running, and the
// prompt it wakes the agent with when that task ends.
const (
	stopWithSuiteRunning = `{"hook_event_name":"Stop","last_assistant_message":"Rerunning the full suite; I'll commit once it passes.",
		"background_tasks":[{"id":"bdq9jhgg9","type":"shell","status":"running","description":"Run the full suite","command":"just test"}],"session_crons":[]}`
	suiteNotification = `{"hook_event_name":"UserPromptSubmit","prompt":"<task-notification>\n<task-id>bdq9jhgg9</task-id>\n<status>completed</status>\n<summary>Background command \"Run the full suite\" completed (exit code 0)</summary>\n</task-notification>"}`
)

// Crew that report done while their background task runs are still done
// once it wakes them: the wake-up isn't a message, and nobody answered
// the done. The turn it starts isn't a quiet one.
func TestMissionHook_DoneSurvivesAWakeUp(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.StartCrewAs("cache", "claude", "", "") // c2
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c2")
	crew := func() mission.Crew { m, _ := store.Load("cache"); return m.Crew[1] }

	hook(t, w, "prompt", `{}`)
	hook(t, w, "stop", stopWithSuiteRunning)
	if c := crew(); c.Pending == nil || c.Pending.Work[0].What != "just test" || c.Pending.Work[0].Wakes != mission.WakeYes {
		t.Fatalf("the task it left running: %+v", c.Pending)
	}
	store.SetCrewStatus("cache", "c2", mission.CrewDone, "Stage one is in; the suite's rerunning.")
	hook(t, w, "prompt", suiteNotification)
	if c := crew(); c.Status != mission.CrewDone || c.Activity != mission.ActivityWorking || c.Pending != nil {
		t.Errorf("after the wake-up: status %q, activity %q, pending %+v", c.Status, c.Activity, c.Pending)
	}
	hook(t, w, "stop", `{"last_assistant_message":"The suite passes.","background_tasks":[]}`)
	if c := crew(); c.Status != mission.CrewDone {
		t.Errorf("after its turn: status %q", c.Status)
	}
	if len(*runs) != 0 {
		t.Errorf("runs = %v", *runs)
	}

	// Your message is one: it clears the done, as ever.
	hook(t, w, "prompt", `{"prompt":"Also add a benchmark."}`)
	if c := crew(); c.Status != "" {
		t.Errorf("after your message: status %q", c.Status)
	}
}

// The daemon watches held turns. Work nothing wakes the crew for gets the
// orchestrator once it ends, not before; work that never ends gets it
// after mission.HoldMax; work the harness said it would wake the crew for,
// ended with no turn started within mission.WakeGrace, gets it then. Each
// once. Crew that start a turn in between aren't handed on.
func TestWatchHeld_Backstop(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()
	t.Setenv(mission.EnvMission, "cache")
	server := mission.Work{What: "nohup node server.js", From: harness.FromProcesses, PID: 4242, Wakes: mission.WakeNo}
	shell := mission.Work{What: "npm run watch", From: harness.FromHarness, PID: 4343, Wakes: mission.WakeYes}
	held := func(id string, work ...mission.Work) time.Time {
		t.Helper()
		store.StartCrewAs("cache", "claude", "", "")
		t.Setenv(mission.EnvCrew, id)
		hook(t, w, "prompt", `{}`)
		if len(work) == 0 {
			hook(t, w, "stop", stopWithSuiteRunning)
		} else { // what stages 2 and 3's adapters will say
			store.CrewHook("cache", id, mission.Hook{Activity: mission.ActivityWaiting, Work: work})
		}
		m, _ := store.Load("cache")
		c, _ := m.CrewByID(id)
		if c.Pending == nil {
			t.Fatalf("%s: nothing held", id)
		}
		return c.Pending.At
	}
	alive := func(pids ...int) func(int) bool {
		return func(pid int) bool { return slices.Contains(pids, pid) }
	}
	var callsDone []string // the daemon's ACP sessions: the tool calls finished
	callDone := func(slug, crewID, call string) bool { return slices.Contains(callsDone, slug+"/"+crewID+"/"+call) }
	watch := func(at time.Time, live func(int) bool, want ...string) {
		t.Helper()
		cli.WatchHeld(w.Root, at, live, callDone)
		if got := strings.Join(*runs, " "); got != strings.Join(want, " ") {
			t.Errorf("runs = %v, want %v", got, want)
		}
	}

	// Work nothing wakes it for: taken when its process is gone.
	at := held("c2", server)
	watch(at.Add(time.Minute), alive(4242))
	watch(at.Add(2*time.Minute), alive(), "cache/c2/quiet")
	watch(at.Add(3*time.Minute), alive(), "cache/c2/quiet")

	// Claude Code's task, which never ends: taken after HoldMax.
	at = held("c3")
	watch(at.Add(mission.HoldMax-time.Second), alive(), "cache/c2/quiet")
	watch(at.Add(mission.HoldMax), alive(), "cache/c2/quiet", "cache/c3/quiet")
	watch(at.Add(mission.HoldMax+time.Minute), alive(), "cache/c2/quiet", "cache/c3/quiet")

	// Work the harness wakes it for ends, and nothing wakes it.
	at = held("c4", shell)
	watch(at.Add(time.Minute), alive(), "cache/c2/quiet", "cache/c3/quiet")
	watch(at.Add(time.Minute+mission.WakeGrace-time.Second), alive(), "cache/c2/quiet", "cache/c3/quiet")
	watch(at.Add(time.Minute+mission.WakeGrace), alive(), "cache/c2/quiet", "cache/c3/quiet", "cache/c4/quiet")

	// Woken in between: its turn's no longer held, and not handed on.
	at = held("c5")
	hook(t, w, "prompt", suiteNotification)
	watch(at.Add(mission.HoldMax), alive(), "cache/c2/quiet", "cache/c3/quiet", "cache/c4/quiet")

	// Codex's background terminal, a command whose call it left in_progress
	// (as the daemon reads the turn's end): taken when the call finishes.
	// Its pid isn't known, so it isn't asked about.
	terminal := mission.Work{What: "just test", From: harness.FromHarness, Wakes: mission.WakeNo, Call: "call_7"}
	at = held("c6", terminal)
	watch(at.Add(time.Minute), alive(), "cache/c2/quiet", "cache/c3/quiet", "cache/c4/quiet")
	callsDone = append(callsDone, "cache/c6/call_7")
	watch(at.Add(2*time.Minute), alive(), "cache/c2/quiet", "cache/c3/quiet", "cache/c4/quiet", "cache/c6/quiet")
	watch(at.Add(3*time.Minute), alive(), "cache/c2/quiet", "cache/c3/quiet", "cache/c4/quiet", "cache/c6/quiet")
	m, _ := store.Load("cache")
	if c, _ := m.CrewByID("c6"); c.Pending == nil || c.Pending.Work[0].Ended == nil || !c.Pending.Work[0].Ended.Equal(at.Add(2*time.Minute)) {
		t.Errorf("c6's work: %+v", c.Pending)
	}
}

// A question crew put with Claude Code's question tool is a question, with
// its options, not "Claude needs your permission"; answering it puts the
// crew back to work.
func TestMissionHook_CrewQuestionIsAQuestion(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.StartCrewAs("cache", "claude", "", "") // c2
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c2")
	crew := func() mission.Crew { m, _ := store.Load("cache"); return m.Crew[1] }

	hook(t, w, "prompt", `{}`)
	hook(t, w, "question", `{"tool_name":"AskUserQuestion","tool_input":{"questions":[{"question":"Per-org or global cache?","header":"Scope","multiSelect":false,"options":[{"label":"Per-org","description":"one per org"},{"label":"Global","description":"one for all"}]}]}}`)
	hook(t, w, "notify", `{"message":"Claude needs your permission","notification_type":"permission_prompt"}`)
	c := crew()
	if c.Activity != mission.ActivityQuestion || c.Notice != "Per-org or global cache?" || strings.Join(c.Options, "|") != "Per-org|Global" {
		t.Fatalf("after its question: %+v", c)
	}
	m, _ := store.Load("cache")
	h := mission.Resolve(mission.Snapshot{M: m, State: mission.Active})[0]
	if h.Kind != mission.KindCrewAsks || h.Text != "c2 asks: Per-org or global cache?" || len(h.Action.Options) != 2 {
		t.Errorf("headline = %+v", h)
	}

	hook(t, w, "answered", `{"tool_name":"AskUserQuestion"}`)
	if c := crew(); c.Activity != mission.ActivityWorking || c.Options != nil {
		t.Errorf("after you answered: %+v", c)
	}
}

// Cursor's question tool asks its question as a prompt under a title, with
// labelled options; its input may come as JSON text. Skipping it (the
// tool failing) answers it too.
func TestMissionHook_CursorQuestion(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.StartCrewAs("cache", "cursor", "", "") // c2
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c2")
	crew := func() mission.Crew { m, _ := store.Load("cache"); return m.Crew[1] }
	for _, payload := range []string{
		`{"tool_name":"AskQuestion","tool_input":{"title":"Setup","questions":[{"id":"q1","prompt":"Which language?","options":[{"id":"a","label":"Go"},{"id":"b","label":"Python"}],"allowMultiple":false}]}}`,
		`{"tool_name":"AskQuestion","tool_input":"{\"title\":\"Setup\",\"questions\":[{\"id\":\"q1\",\"prompt\":\"Which language?\",\"options\":[{\"id\":\"a\",\"label\":\"Go\"},{\"id\":\"b\",\"label\":\"Python\"}]}]}"}`,
	} {
		hook(t, w, "prompt", `{}`)
		hook(t, w, "question", payload)
		if c := crew(); c.Activity != mission.ActivityQuestion || c.Notice != "Which language?" || strings.Join(c.Options, "|") != "Go|Python" {
			t.Errorf("after its question: %+v", c)
		}
		hook(t, w, "answered", `{"tool_name":"AskQuestion","error_message":"skipped"}`)
		if c := crew(); c.Activity != mission.ActivityWorking {
			t.Errorf("after skipping it: %+v", c)
		}
	}
}
