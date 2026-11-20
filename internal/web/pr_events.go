package web

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/pulls"
)

// A pull request's page follows its own topic, pr:<repo>#<number>: it's
// told when the Reviews page's list changes (as the reviews topic tells
// it), and when its page as ws keeps it has been read again and found
// different (pulls.Service.PageSeen). While it's open, its page is read
// again behind, when it's stale, at each look; nothing's looked at for a
// pull request nobody has open.

const prTopic = "pr:"

type prRef struct {
	repo   string
	number int
}

// prTopicRef reads a topic's pull request: false for a topic that isn't
// one.
func prTopicRef(topic string) (prRef, bool) {
	ref, ok := strings.CutPrefix(topic, prTopic)
	if !ok {
		return prRef{}, false
	}
	repo, n, ok := strings.Cut(ref, "#")
	number, err := strconv.Atoi(n)
	if !ok || err != nil || number < 1 || !repoName.MatchString(repo) || strings.Trim(repo, ".") == "" {
		return prRef{}, false
	}
	return prRef{repo, number}, true
}

func (r prRef) topic() string { return prTopic + pulls.Key(r.repo, r.number) }

type prHub struct {
	poll time.Duration
	// stamp is what a pull request's page is as kept, having it read again
	// behind if it's stale.
	stamp func(prRef) string

	mu   sync.Mutex
	subs map[prRef]map[chan string]struct{}
	last map[prRef]string
}

func newPRHub(poll time.Duration, stamp func(prRef) string) *prHub {
	return &prHub{poll: poll, stamp: stamp, subs: map[prRef]map[chan string]struct{}{}, last: map[prRef]string{}}
}

// subscribe follows a pull request: a channel of event names, and a
// function to stop.
func (h *prHub) subscribe(r prRef) (<-chan string, func()) {
	ch := make(chan string, 8)
	stamp := h.stamp(r) // the baseline the page fetches against
	h.mu.Lock()
	if len(h.subs[r]) == 0 {
		h.last[r] = stamp
		h.subs[r] = map[chan string]struct{}{}
	}
	h.subs[r][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[r], ch)
		if len(h.subs[r]) == 0 {
			delete(h.subs, r)
			delete(h.last, r)
		}
		h.mu.Unlock()
	}
}

func (h *prHub) followed() []prRef {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]prRef, 0, len(h.subs))
	for r := range h.subs {
		out = append(out, r)
	}
	return out
}

func (h *prHub) run(ctx context.Context) {
	t := time.NewTicker(h.poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		for _, r := range h.followed() {
			st := h.stamp(r)
			h.mu.Lock()
			if h.subs[r] != nil && st != h.last[r] {
				h.last[r] = st
				for ch := range h.subs[r] {
					select {
					case ch <- r.topic():
					default: // a page that's this far behind will refetch anyway
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// followPR is a pull request page's stream: its own topic's events, and
// the Reviews list's (which an open page keeps fresh, as the Reviews page
// does), both as its topic's.
func (s *Server) followPR(r prRef) (<-chan string, func()) {
	out := make(chan string, 8)
	own, stopOwn := s.prs.subscribe(r)
	var list <-chan string
	stopList := func() {}
	if h := s.topics[topicReviews]; h != nil {
		list, stopList = h.subscribe()
	}
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-quit:
				return
			case <-own:
			case <-list:
			}
			select {
			case out <- r.topic():
			default:
			}
		}
	}()
	return out, func() {
		close(quit)
		stopOwn()
		stopList()
	}
}
