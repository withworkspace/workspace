package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Callbacks are how a client hears from its agent. OnUpdate is called in
// order, before the next message is read on. The others may wait (on a
// person) as long as they need, and should give up when ctx ends: the
// agent gave up on the request, or the connection closed.
type Callbacks struct {
	OnUpdate func(SessionUpdate)
	// OnPermission answers the agent asking leave to make a tool call: the
	// option chosen, or ok false for cancelled. nil cancels every request.
	OnPermission func(ctx context.Context, r PermissionRequest) (optionID string, ok bool)
	// OnElicitation fills in a form. nil cancels every request.
	OnElicitation func(ctx context.Context, r ElicitationRequest) ElicitationResponse
	// OnExtension answers a method of the agent's own, one ACP doesn't
	// have (Cursor's cursor/ask_question, say), whether it was sent as a
	// request (whose answer is the result) or a notification. nil, or
	// ErrNotOffered, says the client doesn't offer it, and the agent does
	// without (Cursor falls back on a permission request per question).
	OnExtension func(ctx context.Context, method string, params json.RawMessage) (any, error)
}

// ErrNotOffered is an extension's answer for a method the client doesn't
// offer.
var ErrNotOffered = errors.New("not offered")

// Client is a running agent, spoken to over ACP.
type Client struct {
	conn  *Conn
	cmd   *exec.Cmd
	stdin io.Closer
	Info  AgentInfo

	// exited is closed once the agent's process has ended (or, over
	// streams alone, the connection has), with exitErr how.
	exited  chan struct{}
	exitErr error

	mu      sync.Mutex
	session string
}

// Command is how to start an agent: its argv, where, and its environment
// (nil: this process's).
type Command struct {
	Args   []string
	Dir    string
	Env    []string
	Stderr io.Writer // where the agent's own logging goes (nil: dropped)
	// ShareGroup runs the agent in this process's group rather than one of
	// its own: for a process that's itself ended by killing its group (a
	// headless run's, on a timeout), which must take the agent with it.
	ShareGroup bool
}

// Start starts the agent and opens the connection (initialize). The agent
// runs in a process group of its own (unless ShareGroup), so the commands
// it starts end with it (Close).
func Start(ctx context.Context, c Command, cb Callbacks) (*Client, error) {
	if len(c.Args) == 0 {
		return nil, errors.New("no agent command")
	}
	cmd := exec.Command(c.Args[0], c.Args[1:]...)
	cmd.Dir, cmd.Env, cmd.Stderr = c.Dir, c.Env, c.Stderr
	if !c.ShareGroup {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	cl, err := connect(ctx, out, in, cb, func(cl *Client) {
		cl.cmd, cl.stdin = cmd, in
		go func() {
			// Its output read to the end first: Wait closes the pipe.
			<-cl.conn.Done()
			cl.exitErr = cmd.Wait()
			// Anything it started that's still there goes with it, as a
			// terminal's would when it hangs up. (In a shared group, that's
			// whoever kills the group's job.)
			if !c.ShareGroup {
				syscall.Kill(-cmd.Process.Pid, syscall.SIGHUP) //nolint:errcheck // none left: fine
			}
			close(cl.exited)
		}()
	})
	if err != nil {
		killGroup(cmd)
		return nil, err
	}
	return cl, nil
}

// Connect opens the connection over streams already there (an agent
// started elsewhere, or a test's): initialize, as the editor.
// If w can be closed, Close closes it, as it would an agent's stdin.
func Connect(ctx context.Context, r io.Reader, w io.Writer, cb Callbacks) (*Client, error) {
	return connect(ctx, r, w, cb, func(cl *Client) {
		if c, ok := w.(io.Closer); ok {
			cl.stdin = c
		}
		go func() {
			<-cl.conn.Done()
			close(cl.exited)
		}()
	})
}

// connect opens the connection; watch, called before initialize, sees to
// closing exited once the agent's gone.
func connect(ctx context.Context, r io.Reader, w io.Writer, cb Callbacks, watch func(*Client)) (*Client, error) {
	cl := &Client{exited: make(chan struct{})}
	cl.conn = NewConn(r, w, cl.handler(cb))
	watch(cl)
	params := map[string]any{
		"protocolVersion": ProtocolVersion,
		"clientCapabilities": map[string]any{
			// The agent reads and writes files and runs commands itself.
			"fs":       map[string]bool{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
			// Questions may come as forms.
			"elicitation": map[string]any{"form": map[string]any{}},
		},
		"clientInfo": map[string]string{"name": "ws", "title": "ws (workspace)", "version": "1"},
	}
	// An agent that ends before it answers fails this with ErrClosed.
	if err := cl.conn.Call(ctx, "initialize", params, &cl.Info); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	return cl, nil
}

func (cl *Client) handler(cb Callbacks) Handler {
	return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		switch method {
		case "session/update":
			var n struct {
				Update json.RawMessage `json:"update"`
			}
			if json.Unmarshal(params, &n) != nil || cb.OnUpdate == nil {
				return nil, nil
			}
			if u, err := ParseUpdate(n.Update); err == nil {
				cb.OnUpdate(u)
			}
			return nil, nil
		case "session/request_permission":
			var r PermissionRequest
			if err := json.Unmarshal(params, &r); err != nil {
				return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
			}
			if cb.OnPermission == nil {
				return permissionOutcome("", false), nil
			}
			return permissionOutcome(cb.OnPermission(ctx, r)), nil
		case "elicitation/create":
			var r ElicitationRequest
			if err := json.Unmarshal(params, &r); err != nil {
				return nil, &RPCError{Code: CodeInvalidParams, Message: err.Error()}
			}
			if cb.OnElicitation == nil {
				return ElicitationResponse{Action: "cancel"}, nil
			}
			return cb.OnElicitation(ctx, r), nil
		}
		if strings.HasPrefix(method, "$/") || isACP(method) {
			return nil, &RPCError{Code: CodeMethodNotFound, Message: "ws doesn't offer " + method}
		}
		if cb.OnExtension != nil {
			res, err := cb.OnExtension(ctx, method, params)
			if !errors.Is(err, ErrNotOffered) {
				return res, err
			}
		}
		return nil, &RPCError{Code: CodeMethodNotFound, Message: "ws doesn't offer " + method}
	}
}

// isACP is whether method is one of ACP's own, for the client to offer:
// those ws doesn't (a file read, a terminal) aren't the extension's to
// answer.
func isACP(method string) bool {
	for _, p := range []string{"fs/", "terminal/", "session/", "elicitation/", "auth/"} {
		if strings.HasPrefix(method, p) {
			return true
		}
	}
	return false
}

func permissionOutcome(optionID string, ok bool) map[string]any {
	if !ok {
		return map[string]any{"outcome": map[string]string{"outcome": "cancelled"}}
	}
	return map[string]any{"outcome": map[string]string{"outcome": "selected", "optionId": optionID}}
}

// Authenticate signs in by one of the agent's methods (Info.AuthMethods),
// as its flow wants after initialize: Cursor's cursor_login uses what
// `agent login` left.
func (cl *Client) Authenticate(ctx context.Context, methodID string) error {
	return cl.conn.Call(ctx, "authenticate", map[string]string{"methodId": methodID}, nil)
}

// Done is closed once the agent's gone: its process ended (Err says how).
func (cl *Client) Done() <-chan struct{} { return cl.exited }

// Err is how the agent ended, once it has (Done): nil for exiting 0.
func (cl *Client) Err() error {
	<-cl.exited
	return cl.exitErr
}

// Pid is the agent's process id (0 for a client over streams alone).
func (cl *Client) Pid() int {
	if cl.cmd == nil || cl.cmd.Process == nil {
		return 0
	}
	return cl.cmd.Process.Pid
}

// SessionID is the session in use ("" before one's started).
func (cl *Client) SessionID() string {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.session
}

func (cl *Client) setSession(id string) {
	cl.mu.Lock()
	cl.session = id
	cl.mu.Unlock()
}

// Session is a session started or loaded: its modes and settings, where
// the agent says.
type Session struct {
	ID     string         `json:"sessionId"`
	Modes  *Modes         `json:"modes,omitempty"`
	Config []ConfigOption `json:"configOptions,omitempty"`
}

// Option is the session's setting of category (mode, model, …), or nil.
func (s Session) Option(category string) *ConfigOption {
	for i := range s.Config {
		if s.Config[i].Category == category {
			return &s.Config[i]
		}
	}
	return nil
}

// SessionParams are where a session works: Dir, and any other folders it
// may (Dirs, if the agent can: Info).
type SessionParams struct {
	Dir  string
	Dirs []string
}

func (p SessionParams) params() map[string]any {
	m := map[string]any{"cwd": p.Dir, "mcpServers": []any{}}
	if len(p.Dirs) > 0 {
		m["additionalDirectories"] = p.Dirs
	}
	return m
}

// NewSession starts a session.
func (cl *Client) NewSession(ctx context.Context, p SessionParams) (Session, error) {
	if len(p.Dirs) > 0 && cl.Info.AgentCapabilities.SessionCapabilities.AdditionalDirectories == nil {
		return Session{}, errors.New("the agent can't work in more than one folder")
	}
	var s Session
	if err := cl.conn.Call(ctx, "session/new", p.params(), &s); err != nil {
		return Session{}, err
	}
	cl.setSession(s.ID)
	return s, nil
}

// LoadSession picks up a session the agent kept (if it can: Info), which
// it replays as session updates (OnUpdate) before answering.
func (cl *Client) LoadSession(ctx context.Context, id string, p SessionParams) (Session, error) {
	if !cl.Info.AgentCapabilities.LoadSession {
		return Session{}, errors.New("the agent can't load sessions")
	}
	return cl.reopen(ctx, "session/load", id, p)
}

// ResumeSession picks up a session the agent kept (if it can: Info),
// without replaying it: for a client that kept its own record.
func (cl *Client) ResumeSession(ctx context.Context, id string, p SessionParams) (Session, error) {
	if cl.Info.AgentCapabilities.SessionCapabilities.Resume == nil {
		return Session{}, errors.New("the agent can't resume sessions")
	}
	return cl.reopen(ctx, "session/resume", id, p)
}

func (cl *Client) reopen(ctx context.Context, method, id string, p SessionParams) (Session, error) {
	params := p.params()
	params["sessionId"] = id
	var s Session
	if err := cl.conn.Call(ctx, method, params, &s); err != nil {
		return Session{}, err
	}
	s.ID = id
	cl.setSession(id)
	return s, nil
}

// Prompt sends a message and waits for the agent's turn to end: its stop
// reason (end_turn, cancelled, max_tokens, max_turn_requests, refusal).
func (cl *Client) Prompt(ctx context.Context, blocks ...ContentBlock) (string, error) {
	stop, _, err := cl.PromptTurn(ctx, blocks...)
	return stop, err
}

// PromptTurn is Prompt, with what the turn spent where the agent says
// (nil where it doesn't).
func (cl *Client) PromptTurn(ctx context.Context, blocks ...ContentBlock) (string, *Usage, error) {
	var r promptResponse
	err := cl.conn.Call(ctx, "session/prompt", map[string]any{
		"sessionId": cl.SessionID(),
		"prompt":    blocks,
	}, &r)
	return r.StopReason, r.usage(), err
}

// Cancel asks the agent to stop its turn; the prompt then ends, cancelled.
// The client must also answer any permission it was asked as cancelled
// (Asks.CancelAll does).
func (cl *Client) Cancel() error {
	return cl.conn.Notify("session/cancel", map[string]string{"sessionId": cl.SessionID()})
}

// SetMode switches the session's mode.
func (cl *Client) SetMode(ctx context.Context, modeID string) error {
	return cl.conn.Call(ctx, "session/set_mode", map[string]string{"sessionId": cl.SessionID(), "modeId": modeID}, nil)
}

// SetConfig changes one of the session's settings (a select's, to value),
// returning them all as they now are.
func (cl *Client) SetConfig(ctx context.Context, configID, value string) ([]ConfigOption, error) {
	var r struct {
		Config []ConfigOption `json:"configOptions"`
	}
	err := cl.conn.Call(ctx, "session/set_config_option", map[string]string{"sessionId": cl.SessionID(), "configId": configID, "value": value}, &r)
	return r.Config, err
}

// Close ends the agent: its stdin closed (an ACP agent's cue to exit), then
// its process group killed if it hasn't gone within grace. It returns how
// it ended.
func (cl *Client) Close(grace time.Duration) error {
	if cl.stdin != nil {
		cl.stdin.Close() //nolint:errcheck // closing to end it
	}
	select {
	case <-cl.exited:
	case <-time.After(grace):
		if cl.cmd == nil {
			return errors.New("the agent didn't end when its input closed")
		}
		killGroup(cl.cmd)
		<-cl.exited
	}
	return cl.exitErr
}

// killGroup kills the agent and whatever it started: its group, if it has
// one of its own, else the agent alone. (Start's watcher reaps it.)
func killGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid || syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) != nil {
		cmd.Process.Kill() //nolint:errcheck // it's going anyway
	}
}
