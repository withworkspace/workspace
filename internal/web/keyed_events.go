package web

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// A topic of one thing, a mission's (mission:<slug>) or a conversation's
// (conversation:<scope>/<id>, its scope its mission's slug, or nothing for
// the workspace's), is followed by its own page, or a pane beside one. A
// write to the records that names it tells its followers at once (the
// records say what each write changed: watchRecords), and its next look
// tells them when what the records can't see has moved (stamp): a
// mission's own files or its capsules, a conversation's agent's steps as
// it answers. Nothing's looked at for a thing nobody follows.
//
// A topic's name is the records' own: what a write names is what's told.

const (
	missionTopic      = "mission:"
	conversationTopic = "conversation:"
	transcriptTopic   = "transcript:" // a crew's session: <slug>/<crew>
	worktreeTopic     = "worktree:"   // a ground's or capsule's runs, on the Files page: <repo>/<name>
	runLogTopic       = "runlog:"     // a run's output: its ref
	appTopic          = "app:"        // an app's run: <name>, or <name>@<repo>/<worktree>
	docTopic          = "doc:"        // a document being edited: <slug>/<ref>, the slug "" for the workspace's
	screenTopic       = "screen:"     // a running crew's screen: <slug>/<crew>
)

// reread is the change that says only that a thing's changed: its
// followers read it whole.
var reread any = rereadOp{}

type rereadOp struct{}

// keyedHub tells the followers of topics of one kind (prefix), each by
// its key: a mission's slug, a conversation's scope/id.
type keyedHub struct {
	prefix string
	poll   time.Duration
	stamp  func(key string) string
	// keep, if set, reads what a page is told of a thing before it's told
	// it's moved: the page's read then has it, and waits on nothing.
	keep func(key string)
	// change, if set, is how a thing has changed since was (nil: since
	// nobody followed it), for followers that hold what it shows: op, nil
	// for nothing they'd see, and now, what it is, for the next change to be
	// from. Without it, followers are told only that it's changed.
	change func(key string, was any) (op, now any)
	// gone, if set, is told a thing nobody follows any more.
	gone func(key string)
	// kick has it look now (nudge).
	kick chan struct{}

	mu   sync.Mutex
	subs map[string]map[chan told]bool // by key: each follower, and whether it's missed a change
	last map[string]string             // each followed thing's stamp, as last looked
	// changing is held while a change is worked out and told, so two
	// aren't worked out from the same state at once (the records' and a
	// look's); was is each followed thing as its followers were last told.
	changing sync.Mutex
	was      map[string]any
}

func newKeyedHub(prefix string, poll time.Duration, stamp func(string) string) *keyedHub {
	return &keyedHub{prefix: prefix, poll: poll, stamp: stamp, subs: map[string]map[chan told]bool{}, last: map[string]string{}, was: map[string]any{}, kick: make(chan struct{}, 1)}
}

// nudge has it look at the things followed now, not at its next poll: a
// look behind a page's read has found something other than the page was
// given. One that's waiting to be heard stands for those after it.
func (h *keyedHub) nudge() {
	select {
	case h.kick <- struct{}{}:
	default:
	}
}

// open is the things someone has open: each followed by a page.
func (h *keyedHub) open() map[string]bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]bool, len(h.subs))
	for key := range h.subs {
		out[key] = true
	}
	return out
}

// subscribe follows a thing: a channel of event names, and a function to
// stop.
func (h *keyedHub) subscribe(key string) (<-chan told, func()) {
	ch := make(chan told, 8)
	stamp := h.stamp(key) // the baseline the page fetches against
	h.changing.Lock()
	h.mu.Lock()
	first := len(h.subs[key]) == 0
	if first {
		h.last[key] = stamp
		h.subs[key] = map[chan told]bool{}
	}
	h.subs[key][ch] = false
	h.mu.Unlock()
	// A change is from what it was when it was first followed: a page
	// reads it after it follows, so it has that, or later.
	if first && h.change != nil {
		_, h.was[key] = h.change(key, nil)
	}

	h.changing.Unlock()
	return ch, func() {
		h.changing.Lock()
		defer h.changing.Unlock()
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs[key], ch)
		if len(h.subs[key]) == 0 {
			delete(h.subs, key)
			delete(h.last, key)
			delete(h.was, key)
			if h.gone != nil {
				h.gone(key)
			}
		}
	}
}

// tell tells a thing's followers it's changed, and how, where the hub
// works that out (change): told nothing when nothing they'd see has. A
// follower that missed a change (too far behind to be told) is told the
// next with no op, so it reads the thing again.
func (h *keyedHub) tell(key string) {
	h.changing.Lock()
	defer h.changing.Unlock()
	t := told{Name: h.prefix + key}
	if h.change != nil {
		h.mu.Lock()
		followed := len(h.subs[key]) > 0
		h.mu.Unlock()
		if !followed {
			return
		}
		op, now := h.change(key, h.was[key])
		h.was[key] = now
		switch op {
		case nil:
			return
		case reread:
		default:
			t.Op = op
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch, missed := range h.subs[key] {
		send := t
		if missed {
			send.Op = nil
		}
		select {
		case ch <- send:
			h.subs[key][ch] = false
		default: // a page that's this far behind reads it again when it's next told
			h.subs[key][ch] = true
		}
	}
}

func (h *keyedHub) followed() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.subs))
	for key := range h.subs {
		out = append(out, key)
	}
	return out
}

// run looks at the things followed every poll, and when it's nudged,
// until ctx is done.
func (h *keyedHub) run(ctx context.Context) {
	t := time.NewTicker(h.poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-h.kick:
		}
		h.look()
	}
}

// look tells the followers of each thing whose stamp has moved since it
// last looked, having read what the page will ask of it (keep).
func (h *keyedHub) look() {
	for _, key := range h.followed() {
		st := h.stamp(key)
		h.mu.Lock()
		moved := h.subs[key] != nil && st != h.last[key]
		if moved {
			h.last[key] = st
		}
		h.mu.Unlock()
		if moved {
			if h.keep != nil {
				h.keep(key)
			}
			h.tell(key)
		}
	}
}

// watchRecords hears each write to the records (a command's included), and
// says each topic it named (told) until ctx is done. With no records to
// watch, it hears nothing.
func watchRecords(ctx context.Context, db *store.DB, told func(topic string)) {
	if db == nil {
		return
	}
	ch, stop := db.Watch()
	defer stop()
	seq, _ := db.Seq(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			topics, latest, err := db.Since(ctx, seq)
			if err != nil {
				continue
			}
			seq = latest
			for _, t := range topics {
				told(t)
			}
		}
	}
}

// topicKey is a topic's thing, for a hub of prefix: false for a topic of
// another kind, or a key that isn't one (empty, or with a space; a
// conversation's is scope/id, a mission's has no slash).
func topicKey(topic, prefix string) (string, bool) {
	key, ok := strings.CutPrefix(topic, prefix)
	if !ok || key == "" || strings.Contains(key, " ") {
		return "", false
	}
	switch prefix {
	case missionTopic:
		return key, !strings.Contains(key, "/")
	case conversationTopic:
		scope, id, ok := strings.Cut(key, "/")
		return key, ok && id != "" && !strings.Contains(id, "/") && !strings.Contains(scope, "/")
	case appTopic:
		_, _, ok := appOfKey(key)
		return key, ok
	case docTopic:
		_, ref, ok := strings.Cut(key, "/")
		return key, ok && ref != ""
	case transcriptTopic, worktreeTopic, screenTopic:
		slug, id, ok := strings.Cut(key, "/")
		return key, ok && slug != "" && id != "" && !strings.Contains(id, "/")
	}
	return key, true
}
