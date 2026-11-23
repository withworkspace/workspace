package web

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// A mission's page follows its own topic, mission:<slug>: it's told at once
// when a write to the records names the mission (its record, its log, its
// decisions, its conversations), and at its next look when the mission's
// own files or its capsules move (Missions.MissionStamp), which the records
// can't see. Nothing's looked at for a mission nobody has open, and the
// workspace's view isn't built.
//
// The records' other topics are heard here too, and passed on (wrote):
// the workspace's, for its hub to look at once and not at its next poll.

const missionTopic = "mission:"

type missionHub struct {
	poll    time.Duration
	stamp   func(slug string) string
	records func() *store.DB // nil when there are none to watch
	// wrote, if set, is told each topic a write to the records named that
	// isn't a mission's.
	wrote func(topic string)
	// keep, if set, reads what a mission's page is told of its capsules
	// before the page is told they've moved: the page's read then has it,
	// and waits on nothing.
	keep func(slug string)
	// kick has it look now (nudge).
	kick chan struct{}

	mu   sync.Mutex
	subs map[string]map[chan string]struct{} // by slug
	last map[string]string                   // each followed mission's stamp, as last looked
}

func newMissionHub(poll time.Duration, stamp func(string) string, records func() *store.DB) *missionHub {
	return &missionHub{poll: poll, stamp: stamp, records: records, subs: map[string]map[chan string]struct{}{}, last: map[string]string{}, kick: make(chan struct{}, 1)}
}

// nudge has it look at the missions followed now, not at its next poll:
// a look behind a page's read has found a capsule other than the page
// was given. One that's waiting to be heard stands for those after it.
func (h *missionHub) nudge() {
	select {
	case h.kick <- struct{}{}:
	default:
	}
}

// open is the missions someone has open: each followed by a page.
func (h *missionHub) open() map[string]bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]bool, len(h.subs))
	for slug := range h.subs {
		out[slug] = true
	}
	return out
}

// subscribe follows a mission: a channel of event names, and a function
// to stop.
func (h *missionHub) subscribe(slug string) (<-chan string, func()) {
	ch := make(chan string, 8)
	stamp := h.stamp(slug) // the baseline the page fetches against
	h.mu.Lock()
	if len(h.subs[slug]) == 0 {
		h.last[slug] = stamp
		h.subs[slug] = map[chan string]struct{}{}
	}
	h.subs[slug][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[slug], ch)
		if len(h.subs[slug]) == 0 {
			delete(h.subs, slug)
			delete(h.last, slug)
		}
		h.mu.Unlock()
	}
}

// tell sends a mission's followers its event.
func (h *missionHub) tell(slug string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[slug] {
		select {
		case ch <- missionTopic + slug:
		default: // a page that's this far behind will refetch anyway
		}
	}
}

func (h *missionHub) followed() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.subs))
	for slug := range h.subs {
		out = append(out, slug)
	}
	return out
}

func (h *missionHub) run(ctx context.Context) {
	var told <-chan struct{}
	var seq int64
	db := h.records()
	if db != nil {
		ch, stop := db.Watch()
		defer stop()
		told = ch
		seq, _ = db.Seq(ctx)
	}
	t := time.NewTicker(h.poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-told:
			topics, latest, err := db.Since(ctx, seq)
			if err != nil {
				continue
			}
			seq = latest
			for _, topic := range topics {
				if slug, ok := strings.CutPrefix(topic, missionTopic); ok {
					h.tell(slug)
				} else if h.wrote != nil {
					h.wrote(topic)
				}
			}
		case <-t.C:
			h.look()
		case <-h.kick:
			h.look()
		}
	}
}

// look tells the pages of each followed mission whose files or capsules
// have moved since it last looked, having read what the page will ask of
// its capsules (keep).
func (h *missionHub) look() {
	for _, slug := range h.followed() {
		st := h.stamp(slug)
		h.mu.Lock()
		moved := h.subs[slug] != nil && st != h.last[slug]
		if moved {
			h.last[slug] = st
		}
		h.mu.Unlock()
		if moved {
			if h.keep != nil {
				h.keep(slug)
			}
			h.tell(slug)
		}
	}
}
