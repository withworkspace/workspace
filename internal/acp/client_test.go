package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func ctx5(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// A session as Cursor's starts: its modes, and its settings with a model
// among them, picked by name.
func TestNewSessionAndSettings(t *testing.T) {
	a, cl := connectFake(t, func(a *fakeAgent) {
		a.Info = map[string]any{
			"protocolVersion":   1,
			"agentCapabilities": map[string]any{"loadSession": true, "sessionCapabilities": map[string]any{"list": map[string]any{}}},
			"authMethods":       []any{map[string]any{"id": "cursor_login", "name": "Cursor Login"}},
		}
		a.New = map[string]any{
			"modes": map[string]any{"currentModeId": "agent", "availableModes": []any{
				map[string]any{"id": "agent", "name": "Agent"}, map[string]any{"id": "ask", "name": "Ask"},
			}},
			"configOptions": []any{
				map[string]any{"id": "mode", "name": "Mode", "category": "mode", "type": "select", "currentValue": "agent",
					"options": []any{map[string]any{"value": "agent", "name": "Agent"}, map[string]any{"value": "ask", "name": "Ask"}}},
				// Grouped values, flattened.
				map[string]any{"id": "model", "name": "Model", "category": "model", "type": "select", "currentValue": "claude-sonnet-5-5[effort=high]",
					"options": []any{
						map[string]any{"group": "fast", "name": "Fast", "options": []any{map[string]any{"value": "default[]", "name": "Auto"}}},
						map[string]any{"group": "big", "name": "Big", "options": []any{
							map[string]any{"value": "claude-sonnet-5-5[effort=high]", "name": "claude-sonnet-5-5"},
							map[string]any{"value": "gpt-5.6-sol[fast=false]", "name": "gpt-5.6-sol"},
						}},
					}},
			},
		}
	}, Callbacks{})

	if !cl.Info.AgentCapabilities.LoadSession || cl.Info.AgentCapabilities.SessionCapabilities.List == nil {
		t.Errorf("capabilities = %+v", cl.Info.AgentCapabilities)
	}
	if cl.Info.AgentCapabilities.SessionCapabilities.Resume != nil {
		t.Error("resume offered, though the agent didn't say so")
	}
	if len(cl.Info.AuthMethods) != 1 || cl.Info.AuthMethods[0].ID != "cursor_login" {
		t.Errorf("auth methods = %+v", cl.Info.AuthMethods)
	}

	s, err := cl.NewSession(ctx5(t), SessionParams{Dir: "/w"})
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "s1" || cl.SessionID() != "s1" {
		t.Errorf("session = %q, client's %q", s.ID, cl.SessionID())
	}
	if s.Modes == nil || s.Modes.Current != "agent" || len(s.Modes.Available) != 2 {
		t.Errorf("modes = %+v", s.Modes)
	}
	model := s.Option("model")
	if model == nil || len(model.Values) != 3 || model.CurrentValue() != "claude-sonnet-5-5[effort=high]" {
		t.Fatalf("model option = %+v", model)
	}
	for want, got := range map[string]string{
		"auto":              "default[]",
		"claude-sonnet-5-5": "claude-sonnet-5-5[effort=high]",
		"GPT":               "gpt-5.6-sol[fast=false]",
		"llama":             "",
	} {
		if v := model.Pick(want); v != got {
			t.Errorf("Pick(%q) = %q, want %q", want, v, got)
		}
	}
	if s.Option("thought_level") != nil {
		t.Error("an option the session hasn't")
	}

	if _, err := cl.SetConfig(ctx5(t), "model", "default[]"); err != nil {
		t.Fatal(err)
	}
	if err := cl.SetMode(ctx5(t), "ask"); err != nil {
		t.Fatal(err)
	}
	p := a.paramsOf("session/set_config_option")
	if len(p) != 1 || !strings.Contains(string(p[0]), `"value":"default[]"`) || !strings.Contains(string(p[0]), `"sessionId":"s1"`) {
		t.Errorf("set_config_option params = %s", p)
	}
	if _, err := cl.NewSession(ctx5(t), SessionParams{Dir: "/w", Dirs: []string{"/x"}}); err == nil {
		t.Error("more folders than the agent can work in, accepted")
	}
}

// A turn's updates all reach OnUpdate, in order, before Prompt returns.
func TestPromptHearsTheTurnInOrder(t *testing.T) {
	var mu sync.Mutex
	var heard []string
	_, cl := connectFake(t, func(a *fakeAgent) {
		a.Turn = func(ctx context.Context, a *fakeAgent, text string) string {
			a.update(map[string]any{"sessionUpdate": UpdateToolCall, "toolCallId": "t1", "title": "ls", "kind": "execute", "status": "pending"})
			for i := 0; i < 200; i++ {
				a.say("x")
			}
			a.update(map[string]any{"sessionUpdate": UpdateToolCallEdit, "toolCallId": "t1", "status": "completed",
				"content": []any{map[string]any{"type": "diff", "path": "a.go", "oldText": nil, "newText": "package a"}}})
			return "end_turn"
		}
	}, Callbacks{OnUpdate: func(u SessionUpdate) {
		mu.Lock()
		heard = append(heard, u.Update)
		mu.Unlock()
		if u.Update == UpdateToolCallEdit {
			if c := u.ToolContent(); len(c) != 1 || c[0].Type != "diff" || c[0].OldText != nil || c[0].NewText != "package a" {
				t.Errorf("diff = %+v", c)
			}
			if !strings.Contains(string(u.Raw), `"toolCallId":"t1"`) {
				t.Errorf("raw = %s", u.Raw)
			}
		}
	}})
	if _, err := cl.NewSession(ctx5(t), SessionParams{Dir: "/w"}); err != nil {
		t.Fatal(err)
	}
	stop, err := cl.Prompt(ctx5(t), Text("go"))
	if err != nil || stop != "end_turn" {
		t.Fatalf("Prompt = %q, %v", stop, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(heard) != 202 || heard[0] != UpdateToolCall || heard[201] != UpdateToolCallEdit {
		t.Fatalf("heard %d updates (first %q, last %q), want all 202 before the answer", len(heard), heard[0], heard[len(heard)-1])
	}
}

// What the agent asks of the client: leave for a tool call, a form, a
// method of its own. Each is answered by its callback, or refused.
func TestAgentRequests(t *testing.T) {
	var asked PermissionRequest
	a, cl := connectFake(t, func(a *fakeAgent) {
		a.Info = map[string]any{"protocolVersion": 1}
	}, Callbacks{
		OnPermission: func(ctx context.Context, r PermissionRequest) (string, bool) {
			asked = r
			return r.Option(AllowOnce), true
		},
		OnElicitation: func(ctx context.Context, r ElicitationRequest) ElicitationResponse {
			return ElicitationResponse{Action: "accept", Content: map[string]any{"answer": r.Message + "!"}}
		},
		OnExtension: func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if method == "cursor/ask_question" {
				return map[string]any{"outcome": map[string]any{"outcome": "answered"}}, nil
			}
			return nil, ErrNotOffered
		},
	})
	ctx := ctx5(t)

	if got := a.askPermission(ctx, "t1", "`rm -rf build`"); got != "allow-once" {
		t.Errorf("permission answered %q", got)
	}
	if asked.ToolCall.ToolCallID != "t1" || asked.ToolCall.Title == nil || *asked.ToolCall.Title != "`rm -rf build`" || asked.Option(RejectAlways, RejectOnce) != "reject-once" {
		t.Errorf("asked = %+v", asked)
	}

	r, err := a.ask(ctx, "elicitation/create", map[string]any{"sessionId": "s1", "mode": "form", "message": "which", "requestedSchema": map[string]any{"type": "object"}})
	if err != nil || !strings.Contains(string(r), `"action":"accept"`) || !strings.Contains(string(r), `"which!"`) {
		t.Errorf("elicitation answered %s, %v", r, err)
	}

	r, err = a.ask(ctx, "cursor/ask_question", map[string]any{"title": "?"})
	if err != nil || !strings.Contains(string(r), "answered") {
		t.Errorf("extension answered %s, %v", r, err)
	}
	// One the client doesn't offer, and one of ACP's it doesn't: not found,
	// so the agent does without.
	for _, m := range []string{"cursor/create_plan", "fs/read_text_file"} {
		_, err = a.ask(ctx, m, map[string]any{})
		var re *RPCError
		if !errors.As(err, &re) || re.Code != CodeMethodNotFound {
			t.Errorf("%s answered %v, want method not found", m, err)
		}
	}
	_ = cl
}

// With no callbacks, permissions and forms are cancelled, not left waiting.
func TestNoCallbacksCancel(t *testing.T) {
	a, _ := connectFake(t, nil, Callbacks{})
	if got := a.askPermission(ctx5(t), "t1", "x"); got != "" {
		t.Errorf("permission answered %q, want cancelled", got)
	}
	r, err := a.ask(ctx5(t), "elicitation/create", map[string]any{"mode": "form", "message": "?"})
	if err != nil || !strings.Contains(string(r), `"cancel"`) {
		t.Errorf("elicitation answered %s, %v", r, err)
	}
}

// The agent giving up on a request ends the wait for its answer.
func TestAgentCancelsItsRequest(t *testing.T) {
	waiting := make(chan struct{})
	gaveUp := make(chan struct{})
	a, _ := connectFake(t, nil, Callbacks{
		OnPermission: func(ctx context.Context, r PermissionRequest) (string, bool) {
			close(waiting)
			<-ctx.Done()
			close(gaveUp)
			return "", false
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	go a.askPermission(ctx, "t1", "x")
	within(t, waiting, "the permission to be asked")
	cancel() // the agent's Call sends $/cancel_request
	within(t, gaveUp, "the client to stop waiting")
}

// The connection closing fails what's waiting on it, and what's asked after.
func TestConnectionClosing(t *testing.T) {
	a, cl := connectFake(t, func(a *fakeAgent) {
		a.Turn = func(ctx context.Context, a *fakeAgent, text string) string {
			<-ctx.Done()
			return "cancelled"
		}
	}, Callbacks{})
	if _, err := cl.NewSession(ctx5(t), SessionParams{Dir: "/w"}); err != nil {
		t.Fatal(err)
	}
	errc := make(chan error, 1)
	go func() {
		_, err := cl.Prompt(context.Background(), Text("go"))
		errc <- err
	}()
	time.Sleep(50 * time.Millisecond)
	a.close()
	select {
	case err := <-errc:
		if !errors.Is(err, ErrClosed) {
			t.Errorf("waiting prompt = %v, want ErrClosed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the prompt is still waiting on a closed connection")
	}
	within(t, cl.Done(), "the client to be done")
	if _, err := cl.Prompt(ctx5(t), Text("again")); !errors.Is(err, ErrClosed) {
		t.Errorf("prompt after = %v, want ErrClosed", err)
	}
}

// An agent that ends before it says what it is.
func TestConnectToAnAgentThatEnds(t *testing.T) {
	r, w := io.Pipe()
	w.Close()
	_, err := Connect(ctx5(t), r, io.Discard, Callbacks{})
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Connect = %v, want ErrClosed", err)
	}
}

// Loading a session replays it before the answer; one the agent hasn't is
// refused.
func TestLoadSession(t *testing.T) {
	var heard []string
	a, cl := connectFake(t, func(a *fakeAgent) {
		a.Load = func(a *fakeAgent, id string) error {
			if id != "kept" {
				return &RPCError{Code: CodeInvalidParams, Message: "Invalid params", Data: json.RawMessage(`{"message":"Session \"` + id + `\" not found"}`)}
			}
			a.update(map[string]any{"sessionUpdate": UpdateUserChunk, "content": Text("hi")})
			a.say("hello")
			return nil
		}
	}, Callbacks{OnUpdate: func(u SessionUpdate) { heard = append(heard, u.Block().Text) }})

	s, err := cl.LoadSession(ctx5(t), "kept", SessionParams{Dir: "/w"})
	if err != nil || s.ID != "kept" || cl.SessionID() != "kept" {
		t.Fatalf("LoadSession = %+v, %v", s, err)
	}
	if strings.Join(heard, ",") != "hi,hello" {
		t.Errorf("replayed %v", heard)
	}
	_, err = cl.LoadSession(ctx5(t), "gone", SessionParams{Dir: "/w"})
	var re *RPCError
	if !errors.As(err, &re) || re.Code != CodeInvalidParams || !strings.Contains(err.Error(), "not found") {
		t.Errorf("loading a missing session = %v", err)
	}
	if _, err := cl.ResumeSession(ctx5(t), "kept", SessionParams{Dir: "/w"}); err == nil {
		t.Error("resumed, though the agent can't")
	}
	_ = a
}

// An agent run as a process: started, prompted, and closed by its stdin.
func TestStartAndClose(t *testing.T) {
	var heard []string
	cl, err := Start(ctx5(t), fakeAgentCommand(t, "echo"), Callbacks{OnUpdate: func(u SessionUpdate) { heard = append(heard, u.Block().Text) }})
	if err != nil {
		t.Fatal(err)
	}
	if cl.Pid() == 0 {
		t.Error("no pid")
	}
	if _, err := cl.NewSession(ctx5(t), SessionParams{Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if stop, err := cl.Prompt(ctx5(t), Text("hi")); err != nil || stop != "end_turn" {
		t.Fatalf("Prompt = %q, %v", stop, err)
	}
	if strings.Join(heard, "") != "you said: hi" {
		t.Errorf("heard %q", heard)
	}
	if err := cl.Close(5 * time.Second); err != nil {
		t.Errorf("Close = %v, want a clean exit", err)
	}
	within(t, cl.Done(), "the agent to be done")
}

// An agent that won't go when asked is killed once its grace is up.
func TestCloseKillsAnAgentThatStays(t *testing.T) {
	cl, err := Start(ctx5(t), fakeAgentCommand(t, "linger"), Callbacks{})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err = cl.Close(200 * time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "killed") {
		t.Errorf("Close = %v, want killed", err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("Close took %s", took)
	}
}

// An agent sharing ws's process group (a headless run's, killed whole on a
// timeout) is still ended by Close, and alone: the group is the caller's.
func TestShareGroup(t *testing.T) {
	for _, mode := range []string{"echo", "linger"} {
		c := fakeAgentCommand(t, mode)
		c.ShareGroup = true
		cl, err := Start(ctx5(t), c, Callbacks{})
		if err != nil {
			t.Fatal(err)
		}
		grace := 5 * time.Second
		if mode == "linger" {
			grace = 200 * time.Millisecond
		}
		err = cl.Close(grace)
		switch {
		case mode == "echo" && err != nil:
			t.Errorf("%s: Close = %v, want a clean exit", mode, err)
		case mode == "linger" && (err == nil || !strings.Contains(err.Error(), "killed")):
			t.Errorf("%s: Close = %v, want killed", mode, err)
		}
	}
}
