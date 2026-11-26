package acp

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"os"
	"sync"
	"testing"
	"time"
)

// fakeAgent is an ACP agent for tests, over pipes or (fakeAgentMain) a
// process's stdin and stdout. It answers initialize and the session
// methods itself, and a prompt with what the test's Turn does: send
// updates, ask the client things, then give a stop reason. session/cancel
// ends the turn's ctx.
type fakeAgent struct {
	conn *Conn
	// ready is closed once conn is set: its handler may run before.
	ready chan struct{}
	// Info is initialize's result; New is added to session/new's.
	Info map[string]any
	New  map[string]any
	// Load answers session/load (after replaying, with a.update), or
	// refuses it with an error. nil: no session is found.
	Load func(a *fakeAgent, id string) error
	// Turn is a prompt's turn: its stop reason. nil: end_turn at once.
	Turn func(ctx context.Context, a *fakeAgent, text string) string
	// Ext answers a request the client makes of the agent's own.
	Ext func(method string, params json.RawMessage) (any, error)
	// AuthErr refuses authenticate.
	AuthErr *RPCError

	mu         sync.Mutex
	calls      []string // what the client asked or told, in order
	params     map[string][]json.RawMessage
	cancelTurn context.CancelFunc
	session    string
	// close ends the agent's side of the pipes (nil in a process).
	close func()
}

// newFakeAgent starts a fake agent over pipes, returning the streams a
// client reads and writes.
func newFakeAgent(t *testing.T, setup func(a *fakeAgent)) (a *fakeAgent, fromAgent io.Reader, toAgent io.Writer) {
	t.Helper()
	inR, inW := io.Pipe()   // client → agent
	outR, outW := io.Pipe() // agent → client
	a = &fakeAgent{session: "s1", params: map[string][]json.RawMessage{}, ready: make(chan struct{})}
	if setup != nil {
		setup(a)
	}
	a.conn = NewConn(inR, outW, a.handle)
	close(a.ready)
	go func() {
		// Its input closed, it exits, as an ACP agent does.
		<-a.conn.Done()
		outW.Close()
	}()
	a.close = func() {
		outW.Close()
		inR.Close()
	}
	t.Cleanup(a.close)
	return a, outR, inW
}

func (a *fakeAgent) handle(ctx context.Context, method string, params json.RawMessage) (any, error) {
	<-a.ready
	a.mu.Lock()
	a.calls = append(a.calls, method)
	a.params[method] = append(a.params[method], params)
	a.mu.Unlock()
	switch method {
	case "initialize":
		if a.Info != nil {
			return a.Info, nil
		}
		return map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{"loadSession": true}}, nil
	case "session/new":
		r := map[string]any{"sessionId": a.session}
		maps.Copy(r, a.New)
		return r, nil
	case "session/load", "session/resume":
		var p struct {
			SessionID string `json:"sessionId"`
		}
		json.Unmarshal(params, &p) //nolint:errcheck // a test's
		if a.Load == nil {
			return nil, &RPCError{Code: CodeInvalidParams, Message: "Session not found"}
		}
		a.mu.Lock()
		a.session = p.SessionID
		a.mu.Unlock()
		if err := a.Load(a, p.SessionID); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	case "session/prompt":
		var p struct {
			Prompt []ContentBlock `json:"prompt"`
		}
		json.Unmarshal(params, &p) //nolint:errcheck // a test's
		text := ""
		for _, b := range p.Prompt {
			text += b.String()
		}
		turnCtx, cancel := context.WithCancel(ctx)
		a.mu.Lock()
		a.cancelTurn = cancel
		a.mu.Unlock()
		defer cancel()
		stop := "end_turn"
		if a.Turn != nil {
			stop = a.Turn(turnCtx, a, text)
		}
		return map[string]string{"stopReason": stop}, nil
	case "session/cancel":
		a.mu.Lock()
		if a.cancelTurn != nil {
			a.cancelTurn()
		}
		a.mu.Unlock()
		return nil, nil
	case "authenticate":
		if a.AuthErr != nil {
			return nil, a.AuthErr
		}
		return map[string]any{}, nil
	case "session/set_mode":
		return map[string]any{}, nil
	case "session/set_config_option":
		return map[string]any{"configOptions": []any{}}, nil
	}
	if a.Ext != nil {
		return a.Ext(method, params)
	}
	return nil, &RPCError{Code: CodeMethodNotFound, Message: method}
}

// update sends the client a session update.
func (a *fakeAgent) update(u map[string]any) {
	a.mu.Lock()
	session := a.session
	a.mu.Unlock()
	a.conn.Notify("session/update", map[string]any{"sessionId": session, "update": u}) //nolint:errcheck // a test's
}

// say sends a chunk of the agent's message.
func (a *fakeAgent) say(text string) {
	a.update(map[string]any{"sessionUpdate": UpdateAgentChunk, "content": Text(text)})
}

// ask makes a request of the client and waits for its answer.
func (a *fakeAgent) ask(ctx context.Context, method string, params any) (json.RawMessage, error) {
	var r json.RawMessage
	err := a.conn.Call(ctx, method, params, &r)
	return r, err
}

// askPermission asks leave for a tool call, offering the usual options:
// the option chosen, or "" for cancelled.
func (a *fakeAgent) askPermission(ctx context.Context, callID, title string) string {
	r, err := a.ask(ctx, "session/request_permission", map[string]any{
		"sessionId": a.session,
		"toolCall":  map[string]any{"toolCallId": callID, "title": title, "kind": "execute", "status": "pending"},
		"options": []PermissionOption{
			{OptionID: "allow-once", Name: "Allow once", Kind: AllowOnce},
			{OptionID: "allow-always", Name: "Allow always", Kind: AllowAlways},
			{OptionID: "reject-once", Name: "Reject", Kind: RejectOnce},
		},
	})
	if err != nil {
		return ""
	}
	var out struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	json.Unmarshal(r, &out) //nolint:errcheck // a test's
	return out.Outcome.OptionID
}

func (a *fakeAgent) called() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.calls...)
}

func (a *fakeAgent) paramsOf(method string) []json.RawMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]json.RawMessage(nil), a.params[method]...)
}

// envFakeAgent makes the test binary a fake agent over its stdin and
// stdout (fakeAgentMain): "echo" answers a prompt with its text; "linger"
// does too, but stays when its stdin closes, as an agent that won't exit.
const envFakeAgent = "WS_ACP_FAKE_AGENT"

func TestMain(m *testing.M) {
	if mode := os.Getenv(envFakeAgent); mode != "" {
		fakeAgentMain(mode)
		return
	}
	os.Exit(m.Run())
}

func fakeAgentMain(mode string) {
	a := &fakeAgent{session: "p1", params: map[string][]json.RawMessage{}, ready: make(chan struct{})}
	a.Turn = func(ctx context.Context, a *fakeAgent, text string) string {
		a.say("you said: " + text)
		return "end_turn"
	}
	a.conn = NewConn(os.Stdin, os.Stdout, a.handle)
	close(a.ready)
	<-a.conn.Done()
	if mode == "linger" {
		time.Sleep(time.Minute)
	}
	os.Exit(0)
}

// fakeAgentCommand runs this test binary as a fake agent.
func fakeAgentCommand(t *testing.T, mode string) Command {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Command{Args: []string{exe, "-test.run=^$"}, Env: append(os.Environ(), envFakeAgent+"="+mode), Stderr: os.Stderr}
}

// connectFake connects a client to a fake agent over pipes.
func connectFake(t *testing.T, setup func(a *fakeAgent), cb Callbacks) (*fakeAgent, *Client) {
	t.Helper()
	a, from, to := newFakeAgent(t, setup)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cl, err := Connect(ctx, from, to, cb)
	if err != nil {
		t.Fatal(err)
	}
	return a, cl
}

// within fails the test if ch isn't closed within a few seconds.
func within(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}
