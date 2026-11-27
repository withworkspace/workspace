package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Options are how to run one session of an agent.
type Options struct {
	Command Command
	// Auth is an auth method to sign in by once connected ("" for none:
	// the agent's signed in already, or signs in by itself).
	Auth    string
	Session SessionParams
	// Load is a session to pick up ("" starts a new one). It's resumed
	// where the agent can, without its history: Replay loads it with its
	// history instead, into the transcript and the record.
	Load   string
	Replay bool
	// Kept is the transcript to start from: what ws kept of the session it
	// loads (Replay of its record).
	Kept *Transcript
	// Mode and Model are a mode and a model to switch the session to, by
	// id or name (ConfigOption.Pick), "" for the agent's own. Settings are
	// any others, by category, chosen after them (a model's reasoning
	// effort, say). One the session hasn't fails Run.
	Mode, Model string
	Settings    []Setting
	// Policy answers the agent's asks before anyone's asked (nil: none).
	Policy Policy
	// Extension answers the agent's own requests (nil: none offered). It
	// may wait on someone through asks.Wait, with Method and Params set.
	Extension func(ctx context.Context, asks *Asks, method string, params json.RawMessage) (any, error)
	// Record is where the session's events are written (nil: nowhere).
	Record *Recorder
	// OnTurn is told of each turn as it starts and as it ends. It mustn't
	// wait on the agent.
	OnTurn func(Turn)

	// Dial connects to the agent instead of starting Command: one started
	// elsewhere, or a test's (Connect, over pipes).
	Dial func(ctx context.Context, cb Callbacks) (*Client, error)
}

// Setting is a session setting to choose, by its category (model,
// thought_level, …), and the value wanted, by id or name.
type Setting struct {
	Category string `json:"category"`
	Want     string `json:"want"`
}

// Turn is one prompt and the agent's work on it.
type Turn struct {
	N     int // from 1
	Sent  []ContentBlock
	Start time.Time
	// End is when it ended (zero while it runs), and Stop why: end_turn,
	// cancelled, max_tokens, max_turn_requests, refusal. Err is a turn
	// that failed instead (the agent gone, say).
	End  time.Time
	Stop string
	Err  error
	// Reply is the agent's final message, once it's ended.
	Reply string
	// Unfinished are the tool calls it began that hadn't finished when it
	// ended, as they stood then: a command the agent left running (Codex's
	// background terminal, finished by a tool_call_update between turns),
	// or one it was stopped in.
	Unfinished []ToolCall
}

// Ended is whether the turn has ended.
func (t Turn) Ended() bool { return !t.End.IsZero() }

// State is where a session stands.
type State struct {
	Session string
	Working bool  // a turn is running
	Queued  int   // messages waiting for it to end
	Asks    []Ask // what it's waiting on the client for
	Last    *Turn // the latest turn: the one running, or the last to end
	Ended   bool  // the agent's gone
	Exit    string
}

// Agent is one session of an agent: messages sent to it a turn at a time,
// queued while a turn runs, with what it asks held in Asks and what it
// does kept in a transcript (and a record) as it goes.
type Agent struct {
	o       Options
	client  *Client
	asks    *Asks
	session Session
	updates chan struct{}
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc

	mu        sync.Mutex
	tr        Transcript
	replaying bool
	queue     []*queued
	turns     int
	last      *Turn
	working   bool
	exit      string
	ended     bool
	wake      chan struct{}
}

type queued struct {
	blocks []ContentBlock
	done   chan Turn // buffered; nil when nobody's waiting
}

// ErrEnded is a message sent to an agent that's gone.
var ErrEnded = errors.New("the agent has ended")

// Run starts the agent and its session: new or loaded, switched to the
// mode and model asked for. The agent is closed if any of it fails.
func Run(ctx context.Context, o Options) (*Agent, error) {
	actx, cancel := context.WithCancel(context.Background())
	a := &Agent{o: o, updates: make(chan struct{}, 1), done: make(chan struct{}), wake: make(chan struct{}, 1), ctx: actx, cancel: cancel}
	if o.Kept != nil {
		a.tr = o.Kept.Clone()
	}
	a.asks = NewAsks(o.Policy, a.changed, a.record)
	cb := Callbacks{
		OnUpdate: a.onUpdate,
		OnPermission: func(ctx context.Context, r PermissionRequest) (string, bool) {
			ans := a.asks.Wait(ctx, Ask{Permission: &r})
			return ans.Option, !ans.Cancelled && ans.Option != ""
		},
		OnElicitation: func(ctx context.Context, r ElicitationRequest) ElicitationResponse {
			ans := a.asks.Wait(ctx, Ask{Form: &r})
			if ans.Cancelled || ans.Form == nil {
				return ElicitationResponse{Action: "cancel"}
			}
			return *ans.Form
		},
	}
	if o.Extension != nil {
		cb.OnExtension = func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			return o.Extension(ctx, a.asks, method, params)
		}
	}
	dial := o.Dial
	if dial == nil {
		dial = func(ctx context.Context, cb Callbacks) (*Client, error) { return Start(ctx, o.Command, cb) }
	}
	cl, err := dial(ctx, cb)
	if err != nil {
		cancel()
		return nil, err
	}
	a.client = cl
	if err := a.open(ctx); err != nil {
		cl.Close(2 * time.Second) //nolint:errcheck // failing anyway
		cancel()
		return nil, err
	}
	go a.turnsLoop()
	go a.watch()
	return a, nil
}

// open signs in if asked, starts or loads the session, and switches its
// mode and model.
func (a *Agent) open(ctx context.Context) error {
	if a.o.Auth != "" {
		if err := a.client.Authenticate(ctx, a.o.Auth); err != nil {
			return fmt.Errorf("could not sign in (%s): %w", a.o.Auth, err)
		}
	}
	var err error
	switch {
	case a.o.Load != "" && !a.o.Replay && a.client.Info.AgentCapabilities.SessionCapabilities.Resume != nil:
		a.record(Event{Session: a.o.Load})
		a.session, err = a.client.ResumeSession(ctx, a.o.Load, a.o.Session)
	case a.o.Load != "":
		// Recorded first, so a replay that's recorded follows it.
		a.record(Event{Session: a.o.Load})
		a.mu.Lock()
		a.replaying = !a.o.Replay
		a.mu.Unlock()
		a.session, err = a.client.LoadSession(ctx, a.o.Load, a.o.Session)
		a.mu.Lock()
		a.replaying = false
		a.mu.Unlock()
	default:
		if a.session, err = a.client.NewSession(ctx, a.o.Session); err == nil {
			a.record(Event{Session: a.session.ID})
		}
	}
	if err != nil {
		return err
	}
	a.mu.Lock()
	if a.tr.Config == nil {
		a.tr.Config = a.session.Config
	}
	if a.session.Modes != nil && a.tr.Mode == "" {
		a.tr.Mode = a.session.Modes.Current
	}
	a.mu.Unlock()
	if a.o.Mode != "" {
		if err := a.setMode(ctx, a.o.Mode); err != nil {
			return err
		}
	}
	if a.o.Model != "" {
		if err := a.setOption(ctx, "model", a.o.Model); err != nil {
			return err
		}
	}
	for _, s := range a.o.Settings {
		if err := a.setOption(ctx, s.Category, s.Want); err != nil {
			return err
		}
	}
	return nil
}

// setMode switches mode: by its setting where the session has one, else
// by session/set_mode.
func (a *Agent) setMode(ctx context.Context, want string) error {
	if a.session.Option("mode") != nil {
		return a.setOption(ctx, "mode", want)
	}
	if a.session.Modes != nil {
		for _, m := range a.session.Modes.Available {
			if strings.EqualFold(m.ID, want) || strings.EqualFold(m.Name, want) {
				if err := a.client.SetMode(ctx, m.ID); err != nil {
					return err
				}
				a.mu.Lock()
				a.tr.Mode = m.ID
				a.mu.Unlock()
				return nil
			}
		}
	}
	return fmt.Errorf("the agent has no mode %q", want)
}

// SetModel switches its session to another model, and the settings that
// go with it (its effort), as starting it does, by the session's config
// options: ACP lets them change at any time, so the session goes on, and
// the agent takes them from its next turn (or the one running, as the
// agent does).
func (a *Agent) SetModel(ctx context.Context, model string, settings []Setting) error {
	if model != "" {
		if err := a.setOption(ctx, "model", model); err != nil {
			return err
		}
	}
	for _, s := range settings {
		if err := a.setOption(ctx, s.Category, s.Want); err != nil {
			return err
		}
	}
	return nil
}

// setOption sets the session's setting of category to the value want
// names.
func (a *Agent) setOption(ctx context.Context, category, want string) error {
	opt := a.session.Option(category)
	if opt == nil {
		return fmt.Errorf("the agent has no %s option", category)
	}
	v := opt.Pick(want)
	if v == "" {
		var names []string
		for i, cv := range opt.Values {
			if i == 8 {
				names = append(names, "…")
				break
			}
			names = append(names, cv.Name)
		}
		return fmt.Errorf("the agent has no %s %q (choose from %s)", category, want, strings.Join(names, ", "))
	}
	cfg, err := a.client.SetConfig(ctx, opt.ID, v)
	if err != nil {
		return fmt.Errorf("could not choose %s %s: %w", category, v, err)
	}
	a.mu.Lock()
	if len(cfg) > 0 {
		a.session.Config, a.tr.Config = cfg, cfg
	}
	if category == "mode" {
		a.tr.Mode = v
	}
	a.mu.Unlock()
	return nil
}

func (a *Agent) onUpdate(u SessionUpdate) {
	a.mu.Lock()
	if a.replaying {
		a.mu.Unlock()
		return
	}
	at := time.Now()
	a.tr.Apply(at, u)
	a.mu.Unlock()
	a.record(Event{At: at, Update: u.Raw})
	a.changed()
}

// Send queues a message: sent at once if no turn is running, else when
// the turns before it have ended.
func (a *Agent) Send(blocks ...ContentBlock) error {
	_, err := a.enqueue(blocks, false)
	return err
}

// Say sends a message of text.
func (a *Agent) Say(text string) error { return a.Send(Text(text)) }

// Prompt sends a message and waits for its turn to end. If ctx ends first
// it stops waiting; the turn goes on (Interrupt stops it).
func (a *Agent) Prompt(ctx context.Context, blocks ...ContentBlock) (Turn, error) {
	q, err := a.enqueue(blocks, true)
	if err != nil {
		return Turn{}, err
	}
	select {
	case t := <-q.done:
		return t, t.Err
	case <-ctx.Done():
		return Turn{}, ctx.Err()
	}
}

func (a *Agent) enqueue(blocks []ContentBlock, wait bool) (*queued, error) {
	q := &queued{blocks: blocks}
	if wait {
		q.done = make(chan Turn, 1)
	}
	a.mu.Lock()
	if a.ended {
		a.mu.Unlock()
		return nil, ErrEnded
	}
	a.queue = append(a.queue, q)
	a.mu.Unlock()
	select {
	case a.wake <- struct{}{}:
	default:
	}
	a.changed()
	return q, nil
}

// turnsLoop sends queued messages a turn at a time, until the agent ends.
// Once its connection's gone it sends nothing more: watch fails what's
// left, rather than a turn taking a message it can't deliver.
func (a *Agent) turnsLoop() {
	for {
		a.mu.Lock()
		if a.ended || a.connGone() {
			a.mu.Unlock()
			return
		}
		if len(a.queue) == 0 {
			a.mu.Unlock()
			select {
			case <-a.wake:
				continue
			case <-a.ctx.Done():
				return
			}
		}
		q := a.queue[0]
		a.queue = a.queue[1:]
		a.turns++
		t := Turn{N: a.turns, Sent: q.blocks, Start: time.Now()}
		first := len(a.tr.Items) // the turn's own items start here
		a.tr.Sent(t.Start, q.blocks...)
		a.working, a.last = true, &t
		a.mu.Unlock()
		a.record(Event{At: t.Start, Sent: q.blocks})
		a.turn(t)
		a.changed()

		stop, usage, err := a.client.PromptTurn(a.ctx, q.blocks...)

		a.mu.Lock()
		t.End, t.Stop, t.Err, t.Reply = time.Now(), stop, err, a.tr.Reply()
		t.Unfinished = unfinished(a.tr.Items[first:])
		a.working, a.last = false, &t
		a.mu.Unlock()
		if err != nil {
			a.record(Event{At: t.End, Error: err.Error()})
		} else {
			a.record(Event{At: t.End, Stop: stop, Usage: usage})
		}
		if q.done != nil {
			q.done <- t
		}
		a.turn(t)
		a.changed()
	}
}

// connGone is whether the agent's connection has closed: it may not have
// ended yet (watch), but it can't be sent anything.
func (a *Agent) connGone() bool {
	select {
	case <-a.client.conn.Done():
		return true
	default:
		return false
	}
}

// watch waits for the agent to end, then fails what's queued and
// cancels what's asked.
func (a *Agent) watch() {
	<-a.client.Done()
	exit := "exited"
	if err := a.client.Err(); err != nil {
		exit = err.Error()
	}
	a.mu.Lock()
	a.ended, a.exit = true, exit
	queue := a.queue
	a.queue = nil
	a.mu.Unlock()
	a.cancel()
	a.asks.CancelAll()
	for _, q := range queue {
		if q.done != nil {
			q.done <- Turn{Sent: q.blocks, Err: ErrEnded}
		}
	}
	a.record(Event{Ended: exit})
	close(a.done)
	a.changed()
}

// Interrupt stops the turn running: the agent is told to cancel, and
// whatever it's asked is answered cancelled, as ACP requires. Messages
// queued behind the turn are sent once it's ended.
func (a *Agent) Interrupt() error {
	err := a.client.Cancel()
	a.asks.CancelAll()
	return err
}

// Asks are what the agent is waiting on the client for: Answer one there.
func (a *Agent) Asks() *Asks { return a.asks }

// Client is the agent's client, for what Agent doesn't do itself.
func (a *Agent) Client() *Client { return a.client }

// Session is the session as it started (its modes and settings then).
func (a *Agent) Session() Session { return a.session }

// State is where the session stands now.
func (a *Agent) State() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := State{Session: a.session.ID, Working: a.working, Queued: len(a.queue), Ended: a.ended, Exit: a.exit}
	if a.last != nil {
		t := *a.last
		s.Last = &t
	}
	s.Asks = a.asks.Open()
	return s
}

// Transcript is a copy of the session's transcript as it stands.
func (a *Agent) Transcript() Transcript {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tr.Clone()
}

// Call is the session's tool call of that id as it now stands (updates
// applied between turns too), if it has one.
func (a *Agent) Call(id string) (ToolCall, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c := a.tr.Call(id); c != nil {
		return *c, true
	}
	return ToolCall{}, false
}

// Updates ticks when anything changes: an update, a turn starting or
// ending, an ask opening or closing, the agent ending. Ticks don't queue
// up: one waiting stands for any number since it was last read.
func (a *Agent) Updates() <-chan struct{} { return a.updates }

// Done is closed once the agent has ended.
func (a *Agent) Done() <-chan struct{} { return a.done }

// Close ends the agent (Client.Close) and waits until its end is recorded.
func (a *Agent) Close(grace time.Duration) error {
	err := a.client.Close(grace)
	<-a.done
	return err
}

func (a *Agent) changed() {
	select {
	case a.updates <- struct{}{}:
	default:
	}
}

func (a *Agent) turn(t Turn) {
	if a.o.OnTurn != nil {
		a.o.OnTurn(t)
	}
}

func (a *Agent) record(e Event) {
	if a.o.Record != nil {
		a.o.Record.Write(e) //nolint:errcheck // Record.Err says
	}
}
