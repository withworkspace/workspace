package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Ask is the agent waiting on the client: for leave to make a tool call,
// a form filled in, or (Method) a request of its own, an extension's.
type Ask struct {
	ID         string              `json:"id"` // ws's: a1, a2, … in the order asked
	At         time.Time           `json:"at"`
	Permission *PermissionRequest  `json:"permission,omitempty"`
	Form       *ElicitationRequest `json:"form,omitempty"`
	Method     string              `json:"method,omitempty"`
	Params     json.RawMessage     `json:"params,omitempty"`
}

// Answer answers an ask: the option chosen (a permission's), the form as
// filled in, or an extension's result. Cancelled answers any of them as
// not answered: the turn was interrupted, or nobody will.
type Answer struct {
	Ask       string               `json:"ask,omitempty"` // the ask's id, once given
	Option    string               `json:"option,omitempty"`
	Form      *ElicitationResponse `json:"form,omitempty"`
	Result    json.RawMessage      `json:"result,omitempty"`
	Cancelled bool                 `json:"cancelled,omitempty"`
	// By is who answered: "policy", or whatever the answerer says (a
	// person, the web UI), for the record.
	By string `json:"by,omitempty"`
}

// Policy answers an ask before anyone is asked: ok false leaves it to a
// person. A headless run's says yes to what it may do, and no to the rest.
type Policy func(Ask) (a Answer, ok bool)

// Allow is a permission's allow-once answer: the request's own option
// for it. One that offers none is cancelled rather than allowed for good.
func Allow(r PermissionRequest) Answer {
	if id := r.Option(AllowOnce); id != "" {
		return Answer{Option: id}
	}
	return Answer{Cancelled: true}
}

// Reject is a permission's reject-once answer, or cancelled if it offers
// none: a rejection never becomes a standing rule by default.
func Reject(r PermissionRequest) Answer {
	if id := r.Option(RejectOnce); id != "" {
		return Answer{Option: id}
	}
	return Answer{Cancelled: true}
}

// Asks are what the agent is waiting on the client for, each held until
// it's answered: by the policy, by someone (Answer), or cancelled.
type Asks struct {
	policy   Policy
	onChange func()
	record   func(Event)

	mu   sync.Mutex
	seq  int
	open []*pendingAsk
}

type pendingAsk struct {
	ask    Ask
	answer chan Answer // buffered: whoever answers first wins
}

// NewAsks holds asks, trying policy (nil: none) on each first. onChange
// (nil: none) is called when an ask opens or closes; record (nil: none)
// is given each ask and answer as an event.
func NewAsks(policy Policy, onChange func(), record func(Event)) *Asks {
	return &Asks{policy: policy, onChange: onChange, record: record}
}

// ErrNoAsk is answering an ask that isn't open: already answered, or
// cancelled.
var ErrNoAsk = errors.New("already answered or cancelled")

// Wait opens an ask and waits for its answer: the policy's, someone's, or
// cancelled when ctx ends (the agent gave up, or went) or all are.
func (a *Asks) Wait(ctx context.Context, ask Ask) Answer {
	a.mu.Lock()
	a.seq++
	ask.ID = fmt.Sprintf("a%d", a.seq)
	if ask.At.IsZero() {
		ask.At = time.Now()
	}
	a.mu.Unlock()
	a.event(Event{At: ask.At, Ask: &ask})

	if a.policy != nil {
		if ans, ok := a.policy(ask); ok {
			ans.Ask, ans.By = ask.ID, "policy"
			a.event(Event{Answer: &ans})
			return ans
		}
	}

	p := &pendingAsk{ask: ask, answer: make(chan Answer, 1)}
	a.mu.Lock()
	a.open = append(a.open, p)
	a.mu.Unlock()
	a.changed()

	var ans Answer
	select {
	case ans = <-p.answer:
	case <-ctx.Done():
		ans = Answer{Cancelled: true, By: "agent"}
	}
	a.mu.Lock()
	a.remove(p)
	a.mu.Unlock()
	ans.Ask = ask.ID
	a.event(Event{Answer: &ans})
	a.changed()
	return ans
}

// Open are the asks waiting, oldest first.
func (a *Asks) Open() []Ask {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Ask, len(a.open))
	for i, p := range a.open {
		out[i] = p.ask
	}
	return out
}

// Answer answers an open ask. A permission's option must be one it
// offered: a wrong guess must never approve something.
func (a *Asks) Answer(id string, ans Answer) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, p := range a.open {
		if p.ask.ID != id {
			continue
		}
		if r := p.ask.Permission; r != nil && !ans.Cancelled && !offers(r, ans.Option) {
			return fmt.Errorf("%s does not offer %q", id, ans.Option)
		}
		a.remove(p)
		p.answer <- ans
		return nil
	}
	return fmt.Errorf("%s: %w", id, ErrNoAsk)
}

func offers(r *PermissionRequest, option string) bool {
	for _, o := range r.Options {
		if o.OptionID == option {
			return true
		}
	}
	return false
}

// CancelAll answers every open ask cancelled: what interrupting a turn
// owes the agent.
func (a *Asks) CancelAll() {
	a.mu.Lock()
	open := a.open
	a.open = nil
	a.mu.Unlock()
	for _, p := range open {
		p.answer <- Answer{Cancelled: true, By: "interrupted"}
	}
}

// remove takes p off the open list (a.mu held).
func (a *Asks) remove(p *pendingAsk) {
	for i, q := range a.open {
		if q == p {
			a.open = append(a.open[:i], a.open[i+1:]...)
			return
		}
	}
}

func (a *Asks) changed() {
	if a.onChange != nil {
		a.onChange()
	}
}

func (a *Asks) event(e Event) {
	if a.record != nil {
		a.record(e)
	}
}
