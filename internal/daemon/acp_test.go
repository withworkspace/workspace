package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// envFakeAgent makes the test binary an ACP agent (fakeAgent): a turn that
// says it's starting, runs a ws command, asks to run curl, and says how
// that went.
const envFakeAgent = "WS_DECKSERVER_FAKE_ACP_AGENT"

func TestMain(m *testing.M) {
	if os.Getenv(envFakeAgent) != "" {
		fakeAgent()
		return
	}
	os.Exit(m.Run())
}

// dropsAlways: the fake agent drops every turn from now on.
var dropsAlways atomic.Bool

func fakeAgent() {
	var conn *acp.Conn
	ready := make(chan struct{})
	var cancelTurn atomic.Value
	update := func(u map[string]any) {
		conn.Notify("session/update", map[string]any{"sessionId": "fake-1", "update": u})
	}
	say := func(text string) {
		update(map[string]any{"sessionUpdate": "agent_message_chunk", "content": acp.Text(text)})
	}
	run := func(ctx context.Context, id, cmd string) string {
		update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": id, "kind": "execute", "title": cmd, "status": "pending", "rawInput": map[string]any{"command": cmd}})
		var out struct {
			Outcome struct {
				OptionID string `json:"optionId"`
			} `json:"outcome"`
		}
		conn.Call(ctx, "session/request_permission", map[string]any{
			"sessionId": "fake-1",
			"toolCall":  map[string]any{"toolCallId": id, "title": cmd, "kind": "execute", "status": "pending", "rawInput": map[string]any{"command": cmd}},
			"options": []acp.PermissionOption{
				{OptionID: "allow_once", Name: "Yes", Kind: acp.AllowOnce},
				{OptionID: "decline", Name: "No, continue", Kind: acp.RejectOnce},
			},
		}, &out)
		status := "completed"
		if out.Outcome.OptionID != "allow_once" {
			status = "failed"
		}
		update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": id, "status": status})
		return out.Outcome.OptionID
	}
	conn = acp.NewConn(os.Stdin, os.Stdout, func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		<-ready
		switch method {
		case "initialize":
			return map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{"loadSession": true}}, nil
		case "session/new":
			return map[string]any{"sessionId": "fake-1"}, nil
		case "session/cancel":
			if c, ok := cancelTurn.Load().(context.CancelFunc); ok {
				c()
			}
			return nil, nil
		case "session/prompt":
			turn, cancel := context.WithCancel(ctx)
			defer cancel()
			cancelTurn.Store(cancel)
			if strings.Contains(string(params), "again") {
				say("Again, then.")
				return map[string]any{"stopReason": "end_turn"}, nil
			}
			if text := promptText(params); strings.Contains(text, "drop") || dropsAlways.Load() {
				// As Cursor when its connection drops: the call it was in
				// left in_progress, the error its last words, and the turn
				// ended as one it finished.
				if strings.Contains(text, "drop always") {
					dropsAlways.Store(true)
				}
				update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": "d1", "kind": "execute", "title": "`just test`", "status": "in_progress",
					"rawInput": map[string]any{"command": "just test"}})
				say("\n\nError: RetriableError: the stream was closed")
				return map[string]any{"stopReason": "end_turn"}, nil
			} else if strings.Contains(text, "cut off") {
				say("Picking up where it stopped.")
				return map[string]any{"stopReason": "end_turn"}, nil
			}
			if _, after, ok := strings.Cut(promptText(params), "background "); ok {
				// As Codex's background terminal: the command's call is
				// left in_progress at the turn's end, and finished by an
				// update between turns, once the file named exists.
				update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": "bg1", "kind": "execute", "title": "just test", "status": "in_progress",
					"rawInput": map[string]any{"command": "/bin/zsh -lc 'just test'"}})
				go func() {
					for {
						if _, err := os.Stat(after); err == nil {
							break
						}
						time.Sleep(20 * time.Millisecond)
					}
					update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "bg1", "status": "completed",
						"_meta": map[string]any{"terminal_exit": map[string]any{"exit_code": 0, "signal": nil, "terminal_id": "bg1"}}})
				}()
				say("The suite's running in the background.")
				return map[string]any{"stopReason": "end_turn"}, nil
			}
			say("Starting.")
			note := run(turn, "x1", `ws mission note "on it"`)
			curl := run(turn, "x2", "curl https://example.com")
			say("Done: note " + note + ", curl " + curl + ".")
			return map[string]any{"stopReason": "end_turn"}, nil
		}
		return nil, &acp.RPCError{Code: acp.CodeMethodNotFound, Message: method}
	})
	close(ready)
	<-conn.Done()
	os.Exit(0)
}

// promptText is session/prompt's text.
func promptText(params json.RawMessage) string {
	var p struct {
		Prompt []acp.ContentBlock `json:"prompt"`
	}
	json.Unmarshal(params, &p) //nolint:errcheck // a test's
	text := ""
	for _, b := range p.Prompt {
		text += b.String()
	}
	return text
}

// eventually waits for cond, failing with what if it doesn't come.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// Crew over ACP, held by the server: launched over the socket, its session
// recorded, what it does told to the mission (the policy letting ws out,
// curl left to a person), answered through the agent as the web UI does,
// its turn's end handed on, and its stop recorded.
func TestServer_ACPCrew(t *testing.T) {
	s, root := startServer(t, time.Minute)
	var turnsEnded atomic.Int32
	s.OnACPTurnEnded = func(slug, crewID string) { turnsEnded.Add(1) }
	spec := crewOn(t, root, "")
	exe, _ := os.Executable()
	spec.Harness, spec.Args = "codex", nil
	spec.Env = os.Environ()
	spec.ACP = &harness.ACPRun{Agent: []string{exe, "-test.run=^$"}, Env: []string{envFakeAgent + "=1"}, Dir: root,
		Allow: []string{"ws"}, Prompt: "go"}
	c := dialT(t, root)
	pid, err := c.Start(spec)
	if err != nil || pid == 0 {
		t.Fatalf("LaunchACP = %d, %v", pid, err)
	}
	store := mission.Store{Root: root}
	crew := func() mission.Crew {
		m, _ := store.Load(spec.Slug)
		return m.Crew[0]
	}

	eventually(t, "the curl to wait for a person", func() bool {
		c := crew()
		return c.Activity == mission.ActivityPermission && strings.Contains(c.Notice, "curl")
	})
	if c := crew(); c.Conversation != "fake-1" || c.Notice != spec.CrewID+" wants to run `curl https://example.com`" {
		t.Errorf("crew = %+v", c)
	}
	ag, ok := s.Agent(spec.Key())
	if !ok {
		t.Fatal("no agent to answer")
	}
	open := ag.Asks().Open()
	if len(open) != 1 || open[0].Permission == nil {
		t.Fatalf("asks = %+v", open)
	}
	if err := ag.Asks().Answer(open[0].ID, acp.Answer{Option: "decline", By: "you"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the turn to end", func() bool { return crew().Activity == mission.ActivityWaiting })
	if r := crew().LastReply; r != "Done: note allow_once, curl decline." {
		t.Errorf("reply = %q", r)
	}
	// The turn's end is told after its waiting is recorded.
	eventually(t, "the turn's end to be told", func() bool { return turnsEnded.Load() == 1 })

	data, err := os.ReadFile(store.ACPRecordPath(spec.Slug, spec.CrewID))
	if err != nil {
		t.Fatal(err)
	}
	entries := harness.ReadTranscript("codex", data)
	var got []string
	for _, e := range entries {
		got = append(got, string(e.By)+":"+e.Text+e.Input)
	}
	if strings.Join(got, "|") != `user:go|agent:Starting.|call:ws mission note "on it"|call:curl https://example.com|agent:Done: note allow_once, curl decline.` {
		t.Errorf("transcript = %q", got)
	}
	if text, _ := c.Transcript(spec.Key()); !strings.Contains(text, "Done: note allow_once") {
		t.Errorf("transcript over the socket = %q", text)
	}

	// The web UI's message goes to it as a turn.
	ag.Send(acp.Text("again"))
	eventually(t, "the second reply", func() bool { return crew().LastReply == "Again, then." })

	if err := s.Stop(spec.Key()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "its end recorded", func() bool { return crew().Ended != nil })
	if c := crew(); !c.Stopped {
		t.Errorf("ended %+v, want stopped", c)
	}
	// Let go of once its end's recorded, which comes first.
	eventually(t, "it let go of", func() bool { _, ok := s.Agent(spec.Key()); return !ok })
}

// Crew over ACP that end a turn with a command left running (Codex's
// background terminal: its call still in_progress) wait on it: the turn
// is held, with nothing to wake them. The tool_call_update that finishes
// the call, between turns, is the work's end, which the daemon's watch
// sees (ACPCallDone), and the turn is then the orchestrator's.
func TestServer_ACPCrewLeavesACommandRunning(t *testing.T) {
	s, root := startServer(t, time.Minute)
	var turnsEnded atomic.Int32
	s.OnACPTurnEnded = func(slug, crewID string) { turnsEnded.Add(1) }
	spec := crewOn(t, root, "")
	exe, _ := os.Executable()
	finished := filepath.Join(t.TempDir(), "finished")
	spec.Harness, spec.Args = "codex", nil
	spec.Env = os.Environ()
	spec.ACP = &harness.ACPRun{Agent: []string{exe, "-test.run=^$"}, Env: []string{envFakeAgent + "=1"}, Dir: root,
		Prompt: "background " + finished}
	if _, err := dialT(t, root).Start(spec); err != nil {
		t.Fatal(err)
	}
	store := mission.Store{Root: root}
	load := func() *mission.Mission { m, _ := store.Load(spec.Slug); return m }
	quiet := mission.OrchestratorEvent{Crew: spec.CrewID, Trigger: "quiet"}

	eventually(t, "the turn's end to be told", func() bool { return turnsEnded.Load() == 1 })
	c := load().Crew[0]
	want := []mission.Work{{What: "just test", From: harness.FromHarness, Wakes: mission.WakeNo, Call: "bg1"}}
	if c.Activity != mission.ActivityWaiting || c.Pending == nil || !reflect.DeepEqual(c.Pending.Work, want) {
		t.Fatalf("crew = %+v, pending %+v", c, c.Pending)
	}
	if takes, why := mission.OrchestratorTakes(load(), quiet); takes || !strings.Contains(why, "ws takes its turn when that ends: just test") {
		t.Errorf("OrchestratorTakes = %v, %q: the command's running", takes, why)
	}
	if s.ACPCallDone(spec.Slug, spec.CrewID, "bg1") {
		t.Error("the command's call is done while it runs")
	}

	if err := os.WriteFile(finished, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the call to finish", func() bool { return s.ACPCallDone(spec.Slug, spec.CrewID, "bg1") })
	// As the watch records it.
	ended, err := store.EndWork(spec.Slug, spec.CrewID, c.Pending.At, time.Now(), func(w mission.Work) bool { return w.Call == "bg1" })
	if err != nil || !ended {
		t.Fatalf("EndWork = %v, %v", ended, err)
	}
	if takes, why := mission.OrchestratorTakes(load(), quiet); !takes {
		t.Errorf("OrchestratorTakes = false (%s), once the command's ended", why)
	}
	if turnsEnded.Load() != 1 {
		t.Errorf("turns ended = %d: the call's update isn't a turn", turnsEnded.Load())
	}

	// A call of crew it doesn't hold has ended with them.
	if !s.ACPCallDone(spec.Slug, "c9", "bg1") {
		t.Error("crew it doesn't hold: call not done")
	}
}

// Work an ACP crew's agent detached (nohup … &) is found by the process
// check, under the agent's process, and held as work nothing wakes it for.
func TestServer_ACPCrewDetachesWork(t *testing.T) {
	s, root := startServer(t, time.Minute)
	var turnsEnded atomic.Int32
	s.OnACPTurnEnded = func(slug, crewID string) { turnsEnded.Add(1) }
	spec := crewOn(t, root, "")
	exe, _ := os.Executable()
	spec.Harness, spec.Args = "codex", nil
	spec.Env = os.Environ()
	spec.ACP = &harness.ACPRun{Agent: []string{exe, "-test.run=^$"}, Env: []string{envFakeAgent + "=1"}, Dir: root, Prompt: "again"}
	s.ReadProcs = func() (harness.Procs, error) {
		ag, ok := s.Agent(spec.Key())
		if !ok {
			return harness.Procs{}, nil
		}
		now, names := time.Now(), []string{"WS_MISSION=" + spec.Slug, "WS_CREW=" + spec.CrewID}
		return harness.Procs{List: []harness.Proc{
			{PID: 1, Start: now.Add(-time.Hour), Name: "launchd", Args: []string{}},
			{PID: ag.Client().Pid(), PPID: os.Getpid(), Start: now.Add(-time.Minute), Name: "codex-acp", Args: []string{"codex-acp"}, Env: names},
			{PID: 7100, PPID: 1, Start: now, Name: "node", Args: []string{"node", "e2e.js"}, Env: names},
		}}, nil
	}
	if _, err := dialT(t, root).Start(spec); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the turn's end to be told", func() bool { return turnsEnded.Load() == 1 })
	m, _ := mission.Store{Root: root}.Load(spec.Slug)
	c := m.Crew[0]
	want := []mission.Work{{What: "node e2e.js", From: harness.FromProcesses, PID: 7100, Wakes: mission.WakeNo}}
	if c.Pending == nil || !reflect.DeepEqual(c.Pending.Work, want) {
		t.Fatalf("pending %+v", c.Pending)
	}
}

// droppedCrew starts crew on cursor-acp, their agent the fake one, told
// prompt, with ws's wait before going on with a dropped turn cut short.
func droppedCrew(t *testing.T, prompt string) (*Server, Spec, mission.Store, *atomic.Int32) {
	t.Helper()
	old := droppedWait
	droppedWait = 10 * time.Millisecond
	t.Cleanup(func() { droppedWait = old })
	s, root := startServer(t, time.Minute)
	var turnsEnded atomic.Int32
	s.OnACPTurnEnded = func(slug, crewID string) { turnsEnded.Add(1) }
	spec := crewOn(t, root, "")
	exe, _ := os.Executable()
	spec.Harness, spec.Args = "cursor-acp", nil
	spec.Env = os.Environ()
	spec.ACP = &harness.ACPRun{Agent: []string{exe, "-test.run=^$"}, Env: []string{envFakeAgent + "=1"}, Dir: root, Prompt: prompt}
	if _, err := dialT(t, root).Start(spec); err != nil {
		t.Fatal(err)
	}
	return s, spec, mission.Store{Root: root}, &turnsEnded
}

// typedIn are the messages ws typed into a mission's crew, as its log has
// them.
func typedIn(store mission.Store, slug string) []string {
	var out []string
	events, _ := store.ReadLog(slug, 0)
	for _, e := range events {
		if e.Kind == mission.EventResumeWith && e.By == "ws" {
			out = append(out, e.Text)
		}
	}
	return out
}

// A turn an error of the agent's cut off (Cursor's, when its connection
// drops: ended as one it finished, the error its last words) isn't a
// turn's end to look at. ws has the crew go on from it, says so on the
// mission, and the turn that follows is the one that ends.
func TestServer_ACPCrewGoesOnFromADroppedTurn(t *testing.T) {
	_, spec, store, turnsEnded := droppedCrew(t, "drop this one")
	crew := func() mission.Crew { m, _ := store.Load(spec.Slug); return m.Crew[0] }

	eventually(t, "the turn after it", func() bool { return crew().LastReply == "Picking up where it stopped." })
	eventually(t, "that turn's end to be told", func() bool { return turnsEnded.Load() == 1 })
	said := typedIn(store, spec.Slug)
	if len(said) != 1 || !strings.Contains(said[0], "cut off by an error from your own agent (RetriableError: the stream was closed)") {
		t.Errorf("ws typed in %q", said)
	}
	// The call it was cut off in isn't work it's waiting on.
	if c := crew(); c.Pending != nil || c.Activity != mission.ActivityWaiting {
		t.Errorf("crew = %+v, pending %+v", c, c.Pending)
	}
	time.Sleep(50 * time.Millisecond)
	if n := turnsEnded.Load(); n != 1 {
		t.Errorf("turns ended = %d: the dropped one was told too", n)
	}
}

// An agent whose every turn is cut off isn't sent on for ever: after
// maxDropped in a row it's left as any turn that ended with nothing said.
func TestServer_ACPCrewDroppedTurnsAreGivenUpOn(t *testing.T) {
	_, spec, store, turnsEnded := droppedCrew(t, "drop always")
	eventually(t, "the last of them to be told", func() bool { return turnsEnded.Load() == 1 })
	if said := typedIn(store, spec.Slug); len(said) != maxDropped {
		t.Errorf("ws went on with it %d times: %q", len(said), said)
	}
}

// Crew who'd reported done before the turn was cut off have nothing to go
// on with: their work's handed over, and only their last words were lost.
func TestServer_ACPCrewDroppedAfterDoneIsLeft(t *testing.T) {
	s, spec, store, turnsEnded := droppedCrew(t, "again")
	eventually(t, "the first turn's end", func() bool { return turnsEnded.Load() == 1 })
	if err := store.SetCrewStatus(spec.Slug, spec.CrewID, mission.CrewDone, "cached"); err != nil {
		t.Fatal(err)
	}
	ag, _ := s.Agent(spec.Key())
	ag.Send(acp.Text("drop this one"))
	eventually(t, "the dropped turn's end", func() bool { return turnsEnded.Load() == 2 })
	if said := typedIn(store, spec.Slug); len(said) != 0 {
		t.Errorf("ws typed in %q", said)
	}
}
