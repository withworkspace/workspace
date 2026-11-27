package web

import (
	"context"
	"sync"
	"time"
)

// hub tells open pages when what they show has changed. While any page is
// open it checks every poll interval, and at once when it's nudged (the
// records it's built from were written); with none, it doesn't look.
type hub struct {
	poll        time.Duration
	fingerprint func() string
	// name is the event sent on a change; baseline, if set, is the
	// fingerprint a first subscriber starts from, when taking one mustn't
	// wait (on GitHub, say): the first check then follows at once.
	name     string
	baseline func() string
	kick     chan struct{}
	// stamp, if set, names what the fingerprint's built from, cheaply: a
	// tick takes the fingerprint only when the stamp has moved, or floor
	// has passed since it last did (for what a stamp can't see).
	stamp func() string
	floor time.Duration
	// version, if set, is what it's told it's now at (told.Version).
	version func() int64

	mu        sync.Mutex
	subs      map[chan told]struct{}
	last      string
	lastStamp string
	lastFull  time.Time
}

// told is what a topic's followers are told when it's changed: the event
// (the topic's name), and for a topic whose followers hold what it shows,
// how it changed (Op: a *ConversationOp), so they needn't read it again.
// With no Op, they read it again.
type told struct {
	Name string
	Op   any
	// Version is the version of what's changed it's now at, where it has
	// one (the workspace's view): 0 for none.
	Version int64
}

// The topics pages follow: the workspace (its missions, conversations,
// pages), the Reviews page's pull requests, and the Team page's looks.
const (
	topicWorkspace = "workspace"
	topicReviews   = "reviews"
	topicTeam      = "team"
	topicRuns      = "runs" // the Runs page's: every run, and the silos
	// topicAutomations is the Automations page's: each automation, and its
	// runs.
	topicAutomations = "automations"
)

func newHub(name string, poll time.Duration, fingerprint func() string) *hub {
	return &hub{poll: poll, fingerprint: fingerprint, name: name, subs: map[chan told]struct{}{}, kick: make(chan struct{}, 1)}
}

// subscribe returns a channel of event names, and a function to stop. The
// page fetches what it shows after subscribing (on the stream's hello), so
// anything that changes after this baseline reaches it.
func (h *hub) subscribe() (<-chan told, func()) {
	ch := make(chan told, 8)
	var baseline string
	first := !h.watching()
	if first {
		if h.baseline != nil {
			baseline = h.baseline()
		} else {
			stamp := h.stampNow()
			baseline = h.fingerprint()
			h.mu.Lock()
			h.lastStamp, h.lastFull = stamp, time.Now()
			h.mu.Unlock()
		}
	}
	h.mu.Lock()
	if len(h.subs) == 0 {
		h.last = baseline
	}
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	if first && h.baseline != nil {
		h.nudge()
	}
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// nudge has it look now, not at its next poll: what it follows has just
// been written. One that's waiting to be heard stands for those after it.
func (h *hub) nudge() {
	select {
	case h.kick <- struct{}{}:
	default:
	}
}

func (h *hub) watching() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs) > 0
}

func (h *hub) run(ctx context.Context) {
	t := time.NewTicker(h.poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-h.kick:
		}
		if !h.watching() {
			continue
		}
		stamp := h.stampNow()
		h.mu.Lock()
		same := h.stamp != nil && stamp == h.lastStamp && time.Since(h.lastFull) < h.floor
		h.mu.Unlock()
		if same {
			continue // nothing it's built from has moved
		}
		fp := h.fingerprint()
		var version int64
		if h.version != nil {
			version = h.version()
		}
		h.mu.Lock()
		h.lastStamp, h.lastFull = stamp, time.Now()
		changed := fp != h.last
		h.last = fp
		if changed {
			for ch := range h.subs {
				select {
				case ch <- told{Name: h.name, Version: version}:
				default: // a page that's this far behind will refetch anyway
				}
			}
		}
		h.mu.Unlock()
	}
}

// stampNow is the stamp, if there's one.
func (h *hub) stampNow() string {
	if h.stamp == nil {
		return ""
	}
	return h.stamp()
}
