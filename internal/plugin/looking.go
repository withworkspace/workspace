package plugin

import (
	"sync"
	"time"

	"github.com/withworkspace/plugin/protocol"
)

// Who's looking at what plugins show in ws's web UI. ws says it to each
// running plugin when it changes (view/looking), and asks nothing by it:
// what a plugin fetches, and when, is the plugin's to decide, and this is
// what it has to decide by.

// lookLinger is how long nobody has to have been looking before a plugin
// is told so, unless the host says another. Someone who looks away for a
// moment (to another tab and back, from one page to the next) hasn't
// gone.
const lookLinger = 5 * time.Second

// Look says a page of ws's web UI is showing, until the function it
// returns is called. app is the plugin whose app the page shows: "" for a
// page that shows none. It starts no plugin: one that's running is told,
// and one that starts while someone's looking is told as it does.
func (h *Host) Look(app string) (gone func()) {
	h.mu.Lock()
	h.pages++
	if app != "" {
		if h.apps == nil {
			h.apps = map[string]int{}
		}
		h.apps[app]++
	}
	h.mu.Unlock()
	// A page's stream doesn't wait on a plugin being told.
	go h.tellLooking()
	var once sync.Once
	return func() {
		once.Do(func() {
			h.mu.Lock()
			now := time.Now()
			if h.pages--; h.pages == 0 {
				h.pagesGone = now
			}
			if app != "" {
				if h.apps[app]--; h.apps[app] == 0 {
					delete(h.apps, app)
					if h.appsGone == nil {
						h.appsGone = map[string]time.Time{}
					}
					h.appsGone[app] = now
				}
			}
			h.mu.Unlock()
			time.AfterFunc(h.linger(), h.tellLooking)
		})
	}
}

func (h *Host) linger() time.Duration {
	if h.LookLinger != 0 {
		return h.LookLinger
	}
	return lookLinger
}

// tellLooking tells each running plugin who's looking, where that isn't
// what it was last told. Someone looking is told of at once, and nobody
// looking only once that's been so for the linger. One telling at a
// time, each of who's looking as it starts, so a plugin hears changes in
// the order they happened.
func (h *Host) tellLooking() {
	h.lookMu.Lock()
	defer h.lookMu.Unlock()
	linger := h.linger()
	h.mu.Lock()
	procs := make([]*proc, 0, len(h.procs))
	for _, p := range h.procs {
		procs = append(procs, p)
	}
	// Whoever went less than the linger ago hasn't gone yet.
	ws := h.pages > 0 || time.Since(h.pagesGone) < linger
	apps := map[string]bool{}
	for name := range h.apps {
		apps[name] = true
	}
	for name, at := range h.appsGone {
		if time.Since(at) < linger {
			apps[name] = true
		}
	}
	h.mu.Unlock()
	for _, p := range procs {
		if !p.alive() {
			continue
		}
		now := protocol.ViewLookingParams{App: apps[p.name], WS: ws || apps[p.name]}
		if now == p.told {
			continue
		}
		if p.conn.Notify(protocol.MethodViewLooking, now) == nil {
			p.told = now
		}
	}
}
