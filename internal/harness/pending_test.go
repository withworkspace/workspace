package harness

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/withworkspace/workspace/internal/acp"
)

func payload(t *testing.T, s string) map[string]any {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// Claude Code's Stop lists the background tasks the agent left running:
// each still running is work it'll wake the agent for. An empty list, or
// finished tasks, is nothing running; no list at all, or one it can't read,
// is unknown. (The payloads are Claude Code 2.1.322's.)
func TestClaude_PendingAtStop(t *testing.T) {
	c := &Claude{}
	cases := []struct {
		name, payload string
		want          Pending
	}{
		{"a task running",
			`{"hook_event_name":"Stop","last_assistant_message":"Waiting on the background command.",
			 "background_tasks":[{"id":"bmfzcllkh","type":"shell","status":"running","description":"Run delayed echo in background","command":"sleep 20 && echo finished"}],
			 "session_crons":[]}`,
			Pending{Known: true, Work: []Work{{What: "sleep 20 && echo finished", From: FromHarness, Wakes: WakeYes}}}},
		{"no command: its description",
			`{"background_tasks":[{"id":"b1","type":"shell","status":"running","description":"Run the suite"},{"id":"b2","status":"running"}]}`,
			Pending{Known: true, Work: []Work{{What: "Run the suite", From: FromHarness, Wakes: WakeYes}, {What: "b2", From: FromHarness, Wakes: WakeYes}}}},
		{"an empty list", `{"hook_event_name":"Stop","background_tasks":[],"session_crons":[]}`, Pending{Known: true}},
		{"a finished task",
			`{"background_tasks":[{"id":"b1","type":"shell","status":"completed","command":"just test"}]}`,
			Pending{Known: true}},
		{"no field", `{"hook_event_name":"Stop","last_assistant_message":"READY"}`, Pending{}},
		{"null", `{"background_tasks":null}`, Pending{}},
		{"not a list", `{"background_tasks":{"b1":"running"}}`, Pending{}},
		{"a task without a status", `{"background_tasks":[{"id":"b1","command":"just test"}]}`, Pending{}},
	}
	for _, tc := range cases {
		if got := c.PendingAtStop(payload(t, tc.payload)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %+v, want %+v", tc.name, got, tc.want)
		}
	}
	// Asked by name, as ws's hook command does.
	if got := PendingAtStop("claude", payload(t, `{"background_tasks":[]}`)); !got.Known {
		t.Errorf("claude by name: %+v", got)
	}
	for _, name := range []string{"cursor", "pi", "codex", "no-such-harness"} {
		if got := PendingAtStop(name, payload(t, `{"background_tasks":[{"status":"running","command":"x"}]}`)); got.Known {
			t.Errorf("%s answered %+v: it can't tell yet", name, got)
		}
	}
}

// Claude Code wakes the agent for a finished background task with a
// prompt of its own, which isn't a message.
func TestClaude_WakeUp(t *testing.T) {
	c := &Claude{}
	notification := `{"hook_event_name":"UserPromptSubmit","prompt":"<task-notification>\n<task-id>bdq9jhgg9</task-id>\n<status>completed</status>\n<summary>Background command \"Sleep\" completed (exit code 0)</summary>\n</task-notification>"}`
	if !c.WakeUp(payload(t, notification)) || !WakeUp("claude", payload(t, notification)) {
		t.Error("a task notification is a wake-up")
	}
	for _, p := range []string{`{"prompt":"now stage two"}`, `{"prompt":"what's a <task-notification>?"}`, `{}`} {
		if c.WakeUp(payload(t, p)) {
			t.Errorf("%s is a message", p)
		}
	}
	if WakeUp("cursor", payload(t, notification)) {
		t.Error("cursor's prompts are messages, until its adapter says otherwise")
	}
}

// Crew over ACP: a turn that ends end_turn with a command it began still
// in_progress left it running (Codex's background terminal), and nothing
// wakes the agent for it. Calls that finished, aren't commands, or hadn't
// started yet aren't work. A turn that ended any other way can't tell.
func TestACP_PendingAtTurnEnd(t *testing.T) {
	call := func(id, kind, status, title string, input string) acp.ToolCall {
		c := acp.ToolCall{ID: id, Kind: kind, Status: status, Title: title}
		if input != "" {
			c.Input = json.RawMessage(input)
		}
		return c
	}
	// codex-acp 2.6.0's call for a command: its title is the command
	// without the shell Codex runs it in, which its input may keep.
	suite := call("call_7", "execute", acp.StatusInProgress, "just test", `{"command":"/bin/zsh -lc 'just test'","cwd":"/w"}`)
	cases := []struct {
		name string
		turn acp.Turn
		want Pending
	}{
		{"a command left running", acp.Turn{Stop: "end_turn", Unfinished: []acp.ToolCall{suite}},
			Pending{Known: true, Work: []Work{{What: "just test", From: FromHarness, Wakes: WakeNo, Call: "call_7"}}}},
		{"Cursor's: the command in backticks", acp.Turn{Stop: "end_turn", Unfinished: []acp.ToolCall{call("tool_1", "execute", acp.StatusInProgress, "`npm run dev`", "")}},
			Pending{Known: true, Work: []Work{{What: "npm run dev", From: FromHarness, Wakes: WakeNo, Call: "tool_1"}}}},
		{"no title: its input", acp.Turn{Stop: "end_turn", Unfinished: []acp.ToolCall{call("call_8", "execute", acp.StatusInProgress, "", `{"command":["bash","-lc","sleep 120"]}`)}},
			Pending{Known: true, Work: []Work{{What: "sleep 120", From: FromHarness, Wakes: WakeNo, Call: "call_8"}}}},
		{"nothing unfinished", acp.Turn{Stop: "end_turn"}, Pending{Known: true}},
		{"a read, and a command waiting to start", acp.Turn{Stop: "end_turn", Unfinished: []acp.ToolCall{
			call("r1", "read", acp.StatusInProgress, "Read go.mod", ""), call("x1", "execute", acp.StatusPending, "rm -rf build", "")}},
			Pending{Known: true}},
		{"cancelled", acp.Turn{Stop: "cancelled", Unfinished: []acp.ToolCall{suite}}, Pending{}},
		{"out of tokens", acp.Turn{Stop: "max_tokens", Unfinished: []acp.ToolCall{suite}}, Pending{}},
		{"failed", acp.Turn{Err: errors.New("the agent has ended"), Unfinished: []acp.ToolCall{suite}}, Pending{}},
	}
	for _, tc := range cases {
		for _, name := range []string{"codex", "cursor-acp"} {
			if got := PendingAtTurnEnd(name, tc.turn); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s, %s: %+v, want %+v", name, tc.name, got, tc.want)
			}
		}
	}
	for _, name := range []string{"claude", "cursor", "pi", "no-such-harness"} {
		if got := PendingAtTurnEnd(name, acp.Turn{Stop: "end_turn", Unfinished: []acp.ToolCall{suite}}); got.Known {
			t.Errorf("%s answered %+v: it doesn't run over ACP", name, got)
		}
	}
}
