package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// cursorLikeSession is session/new's answer as Cursor gives it: a mode
// and a model to choose.
var cursorLikeSession = map[string]any{
	"modes": map[string]any{"currentModeId": "agent", "availableModes": []any{
		map[string]any{"id": "agent", "name": "Agent"}, map[string]any{"id": "ask", "name": "Ask"},
	}},
	"configOptions": []any{
		map[string]any{"id": "mode", "name": "Mode", "category": "mode", "type": "select", "currentValue": "agent",
			"options": []any{map[string]any{"value": "agent", "name": "Agent"}, map[string]any{"value": "ask", "name": "Ask"}}},
		map[string]any{"id": "model", "name": "Model", "category": "model", "type": "select", "currentValue": "big[]",
			"options": []any{map[string]any{"value": "default[]", "name": "Auto"}, map[string]any{"value": "big[]", "name": "big"}}},
		map[string]any{"id": "reasoning_effort", "name": "Effort", "category": "thought_level", "type": "select", "currentValue": "max",
			"options": []any{map[string]any{"value": "low", "name": "Low"}, map[string]any{"value": "max", "name": "Max"}}},
	},
}

// runFake runs an Agent on a fake agent over pipes.
func runFake(t *testing.T, setup func(a *fakeAgent), o Options) (*fakeAgent, *Agent) {
	t.Helper()
	fa, from, to := newFakeAgent(t, setup)
	o.Dial = func(ctx context.Context, cb Callbacks) (*Client, error) { return Connect(ctx, from, to, cb) }
	ag, err := Run(ctx5(t), o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ag.Close(time.Second) })
	return fa, ag
}

// echo is a turn that says what it was sent.
func echo(ctx context.Context, a *fakeAgent, text string) string {
	a.say("you said: ")
	a.say(text)
	return "end_turn"
}

// eventually waits for cond.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// A session started in the mode and on the model asked for, a turn run,
// and the whole of it recorded.
func TestAgentRunsATurn(t *testing.T) {
	var rec bytes.Buffer
	var mu sync.Mutex
	var turns []Turn
	fa, ag := runFake(t, func(a *fakeAgent) {
		a.New = cursorLikeSession
		a.Turn = echo
	}, Options{
		Session:  SessionParams{Dir: "/w"},
		Mode:     "ask",
		Model:    "auto",
		Settings: []Setting{{Category: "thought_level", Want: "low"}},
		Record:   NewRecorder(&rec),
		OnTurn:   func(t Turn) { mu.Lock(); turns = append(turns, t); mu.Unlock() },
	})

	sets := fa.paramsOf("session/set_config_option")
	if len(sets) != 3 || !strings.Contains(string(sets[0]), `"value":"ask"`) || !strings.Contains(string(sets[1]), `"value":"default[]"`) ||
		!strings.Contains(string(sets[2]), `"configId":"reasoning_effort"`) || !strings.Contains(string(sets[2]), `"value":"low"`) {
		t.Errorf("settings chosen: %s", sets)
	}
	if tr := ag.Transcript(); tr.Mode != "ask" || len(tr.Config) != 3 {
		t.Errorf("mode %q, config %+v", tr.Mode, tr.Config)
	}

	turn, err := ag.Prompt(ctx5(t), Text("hi"))
	if err != nil {
		t.Fatal(err)
	}
	if turn.N != 1 || turn.Stop != "end_turn" || turn.Reply != "you said: hi" || !turn.Ended() {
		t.Errorf("turn = %+v", turn)
	}
	mu.Lock()
	if len(turns) != 2 || turns[0].Ended() || !turns[1].Ended() || turns[1].Reply != "you said: hi" {
		t.Errorf("told of turns %+v", turns)
	}
	mu.Unlock()
	st := ag.State()
	if st.Session != "s1" || st.Working || st.Queued != 0 || st.Last == nil || st.Last.Stop != "end_turn" || st.Ended {
		t.Errorf("state = %+v", st)
	}

	ag.Close(time.Second)
	events, err := ReadRecord(&rec)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, e := range events {
		switch {
		case e.Session != "":
			kinds = append(kinds, "session")
		case e.Sent != nil:
			kinds = append(kinds, "sent")
		case e.Update != nil:
			kinds = append(kinds, "update")
		case e.Stop != "":
			kinds = append(kinds, "stop")
		case e.Ended != "":
			kinds = append(kinds, "ended")
		}
	}
	if got := strings.Join(kinds, ","); got != "session,sent,update,update,stop,ended" {
		t.Errorf("recorded %s", got)
	}
	if replayed := Replay(nil, events); replayed.Reply() != "you said: hi" || len(replayed.Items) != 2 {
		t.Errorf("replayed %+v", replayed.Items)
	}
}

// Messages sent while a turn runs wait for it, then go in order.
func TestAgentQueues(t *testing.T) {
	release := make(chan struct{})
	var mu sync.Mutex
	var sent []string
	_, ag := runFake(t, func(a *fakeAgent) {
		a.Turn = func(ctx context.Context, a *fakeAgent, text string) string {
			mu.Lock()
			sent = append(sent, text)
			mu.Unlock()
			if text == "one" {
				<-release
			}
			a.say("ok " + text)
			return "end_turn"
		}
	}, Options{})

	ag.Say("one")
	eventually(t, "the first turn to start", func() bool { return ag.State().Working })
	ag.Say("two")
	if st := ag.State(); st.Queued != 1 || !st.Working {
		t.Errorf("state while busy = %+v", st)
	}
	close(release)
	eventually(t, "both turns", func() bool { st := ag.State(); return st.Last != nil && st.Last.N == 2 && st.Last.Ended() })
	mu.Lock()
	if strings.Join(sent, ",") != "one,two" {
		t.Errorf("sent %v", sent)
	}
	mu.Unlock()
	if tr := ag.Transcript(); tr.Reply() != "ok two" || len(tr.Items) != 4 {
		t.Errorf("transcript %+v", tr.Items)
	}
}

// What the agent asks waits for an answer, unless the policy has one.
func TestAgentAsks(t *testing.T) {
	var rec bytes.Buffer
	got := make(chan string, 2)
	fa, ag := runFake(t, func(a *fakeAgent) {
		a.Turn = func(ctx context.Context, a *fakeAgent, text string) string {
			got <- a.askPermission(ctx, "t1", text)
			return "end_turn"
		}
	}, Options{
		Record: NewRecorder(&rec),
		Policy: func(a Ask) (Answer, bool) {
			if a.Permission != nil && *a.Permission.ToolCall.Title == "`ls`" {
				return Allow(*a.Permission), true
			}
			return Answer{}, false
		},
	})
	_ = fa

	ag.Say("`ls`")
	if opt := <-got; opt != "allow-once" {
		t.Errorf("the policy answered %q", opt)
	}

	ag.Say("`rm -rf build`")
	eventually(t, "the ask", func() bool { return len(ag.State().Asks) == 1 })
	ask := ag.State().Asks[0]
	if ask.ID != "a2" || ask.Permission == nil || *ask.Permission.ToolCall.Title != "`rm -rf build`" {
		t.Fatalf("ask = %+v", ask)
	}
	if err := ag.Asks().Answer(ask.ID, Answer{Option: "allow"}); err == nil {
		t.Error("a guessed option was taken")
	}
	if err := ag.Asks().Answer(ask.ID, Answer{Option: "reject-once", By: "you"}); err != nil {
		t.Fatal(err)
	}
	if opt := <-got; opt != "reject-once" {
		t.Errorf("answered %q", opt)
	}
	eventually(t, "the turn to end", func() bool { st := ag.State(); return st.Last.N == 2 && st.Last.Ended() })
	ag.Close(time.Second)
	events, _ := ReadRecord(&rec)
	var answers []string
	for _, e := range events {
		if e.Answer != nil {
			answers = append(answers, e.Answer.Ask+":"+e.Answer.Option+":"+e.Answer.By)
		}
	}
	if strings.Join(answers, " ") != "a1:allow-once:policy a2:reject-once:you" {
		t.Errorf("recorded answers %v", answers)
	}
}

// Interrupting cancels the turn and what it asked; what's queued goes next.
func TestAgentInterrupt(t *testing.T) {
	got := make(chan string, 1)
	_, ag := runFake(t, func(a *fakeAgent) {
		a.Turn = func(ctx context.Context, a *fakeAgent, text string) string {
			if text != "first" {
				a.say("next")
				return "end_turn"
			}
			got <- a.askPermission(context.Background(), "t1", "x")
			<-ctx.Done() // session/cancel
			return "cancelled"
		}
	}, Options{})

	ag.Say("first")
	eventually(t, "the ask", func() bool { return len(ag.State().Asks) == 1 })
	ag.Say("second")
	if err := ag.Interrupt(); err != nil {
		t.Fatal(err)
	}
	if opt := <-got; opt != "" {
		t.Errorf("the ask was answered %q, want cancelled", opt)
	}
	eventually(t, "the second turn", func() bool { st := ag.State(); return st.Last.N == 2 && st.Last.Ended() })
	if tr := ag.Transcript(); tr.Reply() != "next" {
		t.Errorf("reply = %q", tr.Reply())
	}
}

// A turn says which of the tool calls it began hadn't finished when it
// ended: a command left running (Codex's background terminal) is among
// them, until a tool_call_update finishes it between turns. A later turn
// doesn't count an earlier one's.
func TestAgentTurnLeavesACallRunning(t *testing.T) {
	var mu sync.Mutex
	var ended []Turn
	fa, ag := runFake(t, func(a *fakeAgent) {
		a.Turn = func(ctx context.Context, a *fakeAgent, text string) string {
			if text == "again" {
				a.say("Still waiting.")
				return "end_turn"
			}
			a.update(map[string]any{"sessionUpdate": UpdateToolCall, "toolCallId": "c1", "kind": "read", "title": "Read go.mod", "status": "in_progress"})
			a.update(map[string]any{"sessionUpdate": UpdateToolCallEdit, "toolCallId": "c1", "status": "completed"})
			a.update(map[string]any{"sessionUpdate": UpdateToolCall, "toolCallId": "c2", "kind": "execute", "title": "just test", "status": "in_progress",
				"rawInput": map[string]any{"command": "just test"}})
			a.say("The suite's running in the background.")
			return "end_turn"
		}
	}, Options{OnTurn: func(t Turn) {
		if t.Ended() {
			mu.Lock()
			ended = append(ended, t)
			mu.Unlock()
		}
	}})
	turns := func() []Turn { mu.Lock(); defer mu.Unlock(); return append([]Turn(nil), ended...) }

	ag.Say("run the suite")
	eventually(t, "the turn's end", func() bool { return len(turns()) == 1 })
	left := turns()[0].Unfinished
	if len(left) != 1 || left[0].ID != "c2" || left[0].Kind != "execute" || left[0].Status != StatusInProgress {
		t.Fatalf("unfinished = %+v", left)
	}
	if c, ok := ag.Call("c2"); !ok || c.Done() {
		t.Fatalf("c2 = %+v, %v: it's still running", c, ok)
	}

	ag.Say("again")
	eventually(t, "the second turn's end", func() bool { return len(turns()) == 2 })
	if left := turns()[1].Unfinished; len(left) != 0 {
		t.Errorf("the second turn left %+v: it began none", left)
	}

	// The command ends, outside any turn.
	fa.update(map[string]any{"sessionUpdate": UpdateToolCallEdit, "toolCallId": "c2", "status": "completed",
		"_meta": map[string]any{"terminal_exit": map[string]any{"exit_code": 0, "signal": nil, "terminal_id": "c2"}}})
	eventually(t, "c2 to finish", func() bool { c, ok := ag.Call("c2"); return ok && c.Done() })
	if _, ok := ag.Call("no-such-call"); ok {
		t.Error("a call the session hasn't had")
	}
}

// An extension's request can wait on a person through the asks.
func TestAgentExtension(t *testing.T) {
	got := make(chan string, 1)
	_, ag := runFake(t, func(a *fakeAgent) {
		a.Turn = func(ctx context.Context, a *fakeAgent, text string) string {
			r, err := a.ask(ctx, "cursor/ask_question", map[string]any{"title": "Which?"})
			if err != nil {
				got <- err.Error()
			} else {
				got <- string(r)
			}
			return "end_turn"
		}
	}, Options{
		Extension: func(ctx context.Context, asks *Asks, method string, params json.RawMessage) (any, error) {
			if method != "cursor/ask_question" {
				return nil, ErrNotOffered
			}
			ans := asks.Wait(ctx, Ask{Method: method, Params: params})
			if ans.Cancelled {
				return map[string]any{"outcome": map[string]string{"outcome": "cancelled"}}, nil
			}
			return ans.Result, nil
		},
	})
	ag.Say("go")
	eventually(t, "the question", func() bool { return len(ag.State().Asks) == 1 })
	q := ag.State().Asks[0]
	if q.Method != "cursor/ask_question" || !strings.Contains(string(q.Params), "Which?") {
		t.Errorf("ask = %+v", q)
	}
	ag.Asks().Answer(q.ID, Answer{Result: json.RawMessage(`{"outcome":{"outcome":"answered"}}`)})
	if r := <-got; r != `{"outcome":{"outcome":"answered"}}` {
		t.Errorf("the agent got %s", r)
	}
}

// Loading a session leaves the agent's replay out (ws has it, or a
// headless run doesn't want it), unless asked for; an agent that can
// resume one is asked to, so there's no replay to leave out.
func TestAgentLoad(t *testing.T) {
	load := func(a *fakeAgent) {
		a.Load = func(a *fakeAgent, id string) error {
			a.update(map[string]any{"sessionUpdate": UpdateUserChunk, "content": Text("before")})
			a.say("earlier")
			return nil
		}
		a.Turn = echo
	}

	kept := Replay(nil, []Event{{Sent: []ContentBlock{Text("before")}}, {Update: json.RawMessage(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"earlier"}}`)}})
	var rec bytes.Buffer
	fa, ag := runFake(t, load, Options{Load: "old", Kept: kept, Record: NewRecorder(&rec)})
	if ag.State().Session != "old" || len(fa.paramsOf("session/load")) != 1 {
		t.Fatalf("state %+v", ag.State())
	}
	if _, err := ag.Prompt(ctx5(t), Text("again")); err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, it := range ag.Transcript().Items {
		texts = append(texts, it.Text)
	}
	if strings.Join(texts, "|") != "before|earlier|again|you said: again" {
		t.Errorf("with what ws kept: %q", texts)
	}
	ag.Close(time.Second)
	if strings.Contains(rec.String(), "earlier") {
		t.Error("the replay was recorded again")
	}

	_, ag2 := runFake(t, load, Options{Load: "old", Replay: true})
	texts = nil
	for _, it := range ag2.Transcript().Items {
		texts = append(texts, it.Text)
	}
	if strings.Join(texts, "|") != "before|earlier" {
		t.Errorf("from the replay: %q", texts)
	}
}

// An agent that goes mid-turn fails its turn and what's queued, cancels
// what it asked, and takes no more.
func TestAgentEnds(t *testing.T) {
	started := make(chan struct{})
	fa, ag := runFake(t, func(a *fakeAgent) {
		a.Turn = func(ctx context.Context, a *fakeAgent, text string) string {
			close(started)
			<-ctx.Done()
			return "cancelled"
		}
	}, Options{})
	first := make(chan error, 1)
	go func() {
		_, err := ag.Prompt(context.Background(), Text("one"))
		first <- err
	}()
	<-started
	queued := make(chan error, 1)
	go func() {
		_, err := ag.Prompt(context.Background(), Text("two"))
		queued <- err
	}()
	eventually(t, "the second to queue", func() bool { return ag.State().Queued == 1 })
	fa.close()

	if err := <-first; !errors.Is(err, ErrClosed) {
		t.Errorf("the running turn = %v, want ErrClosed", err)
	}
	if err := <-queued; !errors.Is(err, ErrEnded) {
		t.Errorf("the queued one = %v, want ErrEnded", err)
	}
	within(t, ag.Done(), "the agent to be done")
	if st := ag.State(); !st.Ended || st.Exit != "exited" || st.Queued != 0 {
		t.Errorf("state = %+v", st)
	}
	if err := ag.Say("three"); !errors.Is(err, ErrEnded) {
		t.Errorf("sending after = %v", err)
	}
}

// A model the session hasn't fails Run, saying what it has.
func TestAgentModelNotThere(t *testing.T) {
	fa, from, to := newFakeAgent(t, func(a *fakeAgent) { a.New = cursorLikeSession })
	_, err := Run(ctx5(t), Options{Model: "llama", Dial: func(ctx context.Context, cb Callbacks) (*Client, error) { return Connect(ctx, from, to, cb) }})
	if err == nil || !strings.Contains(err.Error(), `no model "llama" (choose from Auto, big)`) {
		t.Errorf("Run = %v", err)
	}
	within(t, fa.conn.Done(), "the agent to be closed")
}

// The same, as a process: started, prompted, closed.
func TestAgentProcess(t *testing.T) {
	ag, err := Run(ctx5(t), Options{Command: fakeAgentCommand(t, "echo"), Session: SessionParams{Dir: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	turn, err := ag.Prompt(ctx5(t), Text("hi"))
	if err != nil || turn.Reply != "you said: hi" {
		t.Fatalf("turn = %+v, %v", turn, err)
	}
	if err := ag.Close(5 * time.Second); err != nil {
		t.Errorf("Close = %v", err)
	}
	if st := ag.State(); !st.Ended || st.Exit != "exited" {
		t.Errorf("state = %+v", st)
	}
}

func TestAgentResumes(t *testing.T) {
	fa, ag := runFake(t, func(a *fakeAgent) {
		a.Info = map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{
			"loadSession": true, "sessionCapabilities": map[string]any{"resume": map[string]any{}}}}
		a.Load = func(a *fakeAgent, id string) error { return nil }
		a.Turn = echo
	}, Options{Load: "old"})
	if got := fa.called(); len(fa.paramsOf("session/resume")) != 1 || len(fa.paramsOf("session/load")) != 0 {
		t.Errorf("asked %v, want a resume", got)
	}
	if turn, err := ag.Prompt(ctx5(t), Text("hi")); err != nil || turn.Reply != "you said: hi" || ag.State().Session != "old" {
		t.Errorf("turn %+v, %v; state %+v", turn, err, ag.State())
	}
}

// Asked to, it signs in before the session starts; an agent that refuses
// fails Run.
func TestAgentAuthenticates(t *testing.T) {
	fa, _ := runFake(t, nil, Options{Auth: "cursor_login"})
	calls := strings.Join(fa.called(), ",")
	if !strings.HasPrefix(calls, "initialize,authenticate,session/new") || !strings.Contains(string(fa.paramsOf("authenticate")[0]), `"methodId":"cursor_login"`) {
		t.Errorf("calls = %s", calls)
	}

	fb, from, to := newFakeAgent(t, func(a *fakeAgent) {
		a.AuthErr = &RPCError{Code: -32000, Message: "Authentication required"}
	})
	_, err := Run(ctx5(t), Options{Auth: "nope", Dial: func(ctx context.Context, cb Callbacks) (*Client, error) { return Connect(ctx, from, to, cb) }})
	within(t, fb.conn.Done(), "the refusing agent to be closed")
	if err == nil || !strings.Contains(err.Error(), "could not sign in (nope)") {
		t.Errorf("Run = %v", err)
	}
}
