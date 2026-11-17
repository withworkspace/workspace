package mission

import (
	"fmt"
	"strings"
	"time"
)

// Crew sometimes end a turn only to wait on work they left running (a test
// suite in the background). Their harness's adapter says what that work is,
// and whether the harness wakes the agent when it ends (harness.Pending,
// kept as Work by crew.LeftRunning: mission doesn't know harnesses);
// ws keeps the answer on the crew, and holds the quiet turn while the work
// runs (OrchestratorTakes). Work nothing wakes the agent for is watched
// until it ends, when the turn is taken: someone has to wake the agent,
// and now there's a result to read. A backstop takes a held turn that
// waits too long. See
// workspace-records:docs/orchestrator-nudges-crew-waiting-on-background-work.md.

// The backstop.
var (
	// WakeGrace is how long after its work ended crew whose harness said
	// it would wake them have to start a turn, before ws takes the turn
	// they ended: the harness didn't wake them.
	WakeGrace = time.Minute
	// HoldMax is how long ws holds a turn for work that's still running: a
	// server started in the background, a watcher, a test that hangs.
	HoldMax = 30 * time.Minute
)

// Pending is the work crew left running when their last turn ended, and
// what ws has seen of it since. It's cleared when they're next prompted.
type Pending struct {
	At   time.Time `json:"at"` // the turn's end
	Work []Work    `json:"work"`
}

// Work is a piece of it: a harness.Work as ws keeps it, and its end.
type Work struct {
	What  string `json:"what"`          // the command, as the harness or ps names it
	From  string `json:"from"`          // where it was seen: the harness's own list, or the processes
	PID   int    `json:"pid,omitempty"` // 0 when the harness doesn't say
	Wakes Wake   `json:"wakes"`
	// Call is the tool call it is in the session of crew over ACP, by id:
	// it ends when the call finishes. "" for work of another kind.
	Call string `json:"call,omitempty"`
	// Ended is when ws saw it end: its process gone, or its tool call
	// finished. nil while it runs, or when ws can't tell (no pid, no call).
	Ended *time.Time `json:"ended,omitempty"`
}

// Wake is whether the harness wakes the agent when a piece of work ends.
type Wake string

const (
	WakeYes     Wake = "yes"
	WakeNo      Wake = "no"
	WakeUnknown Wake = "unknown"
)

// pendingAt is what's kept of the work crew left running at the end of a
// turn at t: nothing, without any.
func pendingAt(work []Work, t time.Time) *Pending {
	if len(work) == 0 {
		return nil
	}
	return &Pending{At: t, Work: append([]Work(nil), work...)}
}

// Running is the work not seen to end.
func (p *Pending) Running() []Work {
	if p == nil {
		return nil
	}
	var out []Work
	for _, w := range p.Work {
		if w.Ended == nil {
			out = append(out, w)
		}
	}
	return out
}

// wokenFor is when the last work ended that the harness said it would
// wake the agent for, if any has.
func (p *Pending) wokenFor() (time.Time, bool) {
	var t time.Time
	for _, w := range p.Work {
		if w.Wakes == WakeYes && w.Ended != nil && w.Ended.After(t) {
			t = *w.Ended
		}
	}
	return t, !t.IsZero()
}

// wakes is whether the harness said it wakes the agent for all of ws.
func wakes(ws []Work) bool {
	for _, w := range ws {
		if w.Wakes != WakeYes {
			return false
		}
	}
	return len(ws) > 0
}

// Hold is whether crew's quiet turn waits on the work they left running, at
// now, and why: while any of it runs, until HoldMax after the turn ended;
// then, if the harness said it would wake them for any of it, until
// WakeGrace after that ended. A turn that isn't held any more is taken
// (once), as a quiet turn.
func (c Crew) Hold(now time.Time) (bool, string) {
	p := c.Pending
	if p == nil || len(p.Work) == 0 {
		return false, ""
	}
	if running := p.Running(); len(running) > 0 {
		if now.Sub(p.At) >= HoldMax {
			return false, "" // the backstop: it never ends
		}
		if wakes(running) {
			return true, "it's waiting on its own work, which its harness wakes it for: " + WorkWhat(running)
		}
		return true, "it's waiting on its own work, and ws takes its turn when that ends: " + WorkWhat(running)
	}
	if t, ok := p.wokenFor(); ok && now.Sub(t) < WakeGrace {
		return true, "its work has ended, and its harness wakes it for that: " + WorkWhat(p.Work)
	}
	return false, ""
}

// OwnWork is what crew wait on at now, when they ended a turn after ws's
// message to wait on work they left running, and the turn is held: the
// work still running, or (all ended) the work their harness is to wake
// them for. ok is false when they don't.
func (c Crew) OwnWork(now time.Time) (work []Work, ok bool) {
	if c.Ended != nil || c.Status != "" || c.Conversing || c.Activity != ActivityWaiting {
		return nil, false
	}
	if held, _ := c.Hold(now); !held {
		return nil, false
	}
	if running := c.Pending.Running(); len(running) > 0 {
		return running, true
	}
	return c.Pending.Work, true
}

// workWhatMax keeps a command to what's worth showing.
const workWhatMax = 80

// WorkWhat names work, in a line: its commands, the first few of them.
func WorkWhat(ws []Work) string {
	var names []string
	for i, w := range ws {
		if i == 2 {
			names = append(names, fmt.Sprintf("and %d more", len(ws)-2))
			break
		}
		what, _, more := strings.Cut(strings.TrimSpace(w.What), "\n")
		if r := []rune(what); len(r) > workWhatMax || more {
			what = string(r[:min(len(r), workWhatMax-1)]) + "…"
		}
		names = append(names, what)
	}
	return strings.Join(names, ", ")
}

// EndWork records that pieces of the work crew left at the end of the
// turn at turn have ended, as ws saw at t: those ended picks, among those
// still running. A later turn's work is left alone, as is crew's that
// ended. It reports whether anything changed.
func (s Store) EndWork(slug, id string, turn, t time.Time, ended func(Work) bool) (bool, error) {
	changed := false
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			c := &m.Crew[i]
			if c.ID != id || c.Ended != nil || c.Pending == nil || !c.Pending.At.Equal(turn) {
				continue
			}
			for j := range c.Pending.Work {
				w := &c.Pending.Work[j]
				if w.Ended == nil && ended(*w) {
					at := t
					w.Ended, changed = &at, true
				}
			}
		}
		return nil
	})
	return changed, err
}
