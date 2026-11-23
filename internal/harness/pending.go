package harness

import (
	"strings"

	"github.com/withworkspace/workspace/internal/acp"
)

// When crew end a turn, ws asks the harness's adapter one question: what
// did the agent leave running, and will the harness wake it when that
// ends? ws holds a quiet turn while the answer has work in it
// (mission.OrchestratorTakes), and never asks which harness gave it. See
// workspace-records:docs/orchestrator-nudges-crew-waiting-on-background-work.md.

// Pending is what an agent left running when its turn ended.
type Pending struct {
	Known bool   `json:"known"`          // false: the adapter couldn't tell
	Work  []Work `json:"work,omitempty"` // what the agent left running; empty: nothing
}

// Work is one thing an agent left running.
type Work struct {
	What  string `json:"what"`          // the command, as the harness or ps names it
	From  string `json:"from"`          // FromHarness (its own list) or FromProcesses
	PID   int    `json:"pid,omitempty"` // 0 when the harness doesn't say
	Wakes Wake   `json:"wakes"`
	// Call is the tool call it is in the agent's ACP session, by id, for
	// work an ACP agent left running: it ends when the call finishes.
	Call string `json:"call,omitempty"`
}

// Where work was seen.
const (
	FromHarness   = "harness"   // the harness's own list of what's running
	FromProcesses = "processes" // the user's processes
)

// Wake is whether the harness wakes the agent when a piece of work ends.
type Wake string

const (
	WakeYes     Wake = "yes"
	WakeNo      Wake = "no"
	WakeUnknown Wake = "unknown"
)

// TurnReader is a harness whose hooks' payloads say more than ws's hook
// events do on their own: what its agent left running when a turn ended,
// and whether a prompt is the harness waking its agent rather than a
// message. A harness that isn't one answers unknown, and every prompt it
// sends is a message.
type TurnReader interface {
	// PendingAtStop is what the agent left running, from the payload of
	// its stop hook.
	PendingAtStop(payload map[string]any) Pending
	// WakeUp is whether a prompt hook's payload is the harness waking
	// the agent (a background task of its own ended), not a message.
	WakeUp(payload map[string]any) bool
}

// PendingAtStop is the named harness's answer, from its stop hook's
// payload: unknown for a harness that can't tell.
func PendingAtStop(name string, payload map[string]any) Pending {
	if r, ok := registry[name].(TurnReader); ok {
		return r.PendingAtStop(payload)
	}
	return Pending{}
}

// WakeUp is whether the named harness's prompt hook payload is the
// harness waking its agent, not a message.
func WakeUp(name string, payload map[string]any) bool {
	r, ok := registry[name].(TurnReader)
	return ok && r.WakeUp(payload)
}

// ACPTurnReader is a harness whose crew run over ACP and whose turns' ends
// say what its agent left running, in ws's transcript of the session. A
// harness over ACP that isn't one answers unknown.
type ACPTurnReader interface {
	// PendingAtTurnEnd is what the agent left running when the turn ended.
	PendingAtTurnEnd(t acp.Turn) Pending
}

// PendingAtTurnEnd is the named harness's answer at the end of a turn of
// its crew over ACP: unknown for a harness that can't tell.
func PendingAtTurnEnd(name string, t acp.Turn) Pending {
	if r, ok := registry[name].(ACPTurnReader); ok {
		return r.PendingAtTurnEnd(t)
	}
	return Pending{}
}

// ACPDropReader is a harness whose agent, over ACP, can end a turn it
// couldn't finish the way it ends one it has: stopped with end_turn, the
// session still up, and only its last words to say an error cut it off
// (its connection to its model dropped). Nothing retries such a turn, and
// nothing in the protocol tells it from one the agent ended to wait.
type ACPDropReader interface {
	// TurnDropped is whether the turn ended that way, with the error it
	// ended on.
	TurnDropped(t acp.Turn) (string, bool)
}

// TurnDropped is whether a turn of the named harness's crew over ACP was
// cut off by an error of its agent's rather than ended by it, with the
// error: false for a harness that can't tell.
func TurnDropped(name string, t acp.Turn) (string, bool) {
	if r, ok := registry[name].(ACPDropReader); ok {
		return r.TurnDropped(t)
	}
	return "", false
}

// acpLeftRunning is what an ACP agent left running when a turn ended: each
// command (an execute tool call) the turn began and left in_progress at
// end_turn, which nothing wakes the agent for (an ACP agent can't start a
// turn of its own). Its end is the tool_call_update that finishes the call,
// between turns. Only the turn's own calls count: one an earlier turn left
// that never finishes (a server, a call in a session ws resumed, one its
// agent dropped) mustn't hold every turn after it. A turn that ended any
// other way (cancelled, refused, out of tokens, failed) was stopped rather
// than ended to wait: a call it left unfinished is likelier the one it was
// stopped in, so the answer is unknown. A call still pending hadn't
// started.
func acpLeftRunning(t acp.Turn) Pending {
	if t.Err != nil || t.Stop != "end_turn" {
		return Pending{}
	}
	out := Pending{Known: true}
	for _, c := range t.Unfinished {
		if c.Kind != "execute" || c.Status != acp.StatusInProgress {
			continue
		}
		// Its title names the command (Codex's without the shell it runs
		// in, Cursor's in backticks), else its input does.
		what, _ := permissionCommand(acp.SessionUpdate{Kind: c.Kind, Title: &c.Title})
		if strings.TrimSpace(what) == "" {
			what, _ = permissionCommand(acp.SessionUpdate{RawInput: c.Input})
		}
		out.Work = append(out.Work, Work{What: firstNonEmptyString(strings.TrimSpace(what), "a command"), From: FromHarness, Wakes: WakeNo, Call: c.ID})
	}
	return out
}
