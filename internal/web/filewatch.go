package web

import (
	"context"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// What some topics show is a file that grows as it's written: a crew's
// session, a run's output, what a conversation's agent writes as it
// answers, a document being edited. Their hubs looked at each followed
// file every few hundred milliseconds (a stat).
//
// The daemon watches each followed file instead (fileWatch), and has its
// hub look when it's written. The file itself is watched, not its folder:
// on macOS a watched folder holds every file in it open, and a session's
// folder has hundreds. A file that isn't there yet (a session not
// started, a conversation not yet answered) can't be watched: it's
// looked at each poll, as before, until it's there. A watched one is
// still looked at every fileSweep, for what a watch missed.

var (
	// fileSettle is how long after a file's first written its hub looks:
	// what's written together is read together.
	fileSettle = 30 * time.Millisecond
	// fileGap is the least time between two looks a file's writes have a
	// hub make: a busy log is read five times a second, not at every write.
	fileGap = 200 * time.Millisecond
	// fileSweep is how often a watched file's looked at regardless.
	fileSweep = 15 * time.Second
)

// fileWatch watches the files the hubs' followers follow.
type fileWatch struct {
	fw *fsnotify.Watcher // nil: nothing's watched, and the hubs poll

	mu sync.Mutex
	// holders is each file a hub follows, and how many of its things do;
	// on, those being watched now (one removed, or replaced, isn't until
	// it's held again).
	holders map[string]map[*keyedHub]int
	on      map[string]bool
	written map[*keyedHub]bool // the hubs to look, once settled
}

func newFileWatch(logf func(string, ...any)) *fileWatch {
	w := &fileWatch{holders: map[string]map[*keyedHub]int{}, on: map[string]bool{}, written: map[*keyedHub]bool{}}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		logf("web: files followed can't be watched (%v): their pages are told at the next look", err)
		return w
	}
	w.fw = fw
	return w
}

// hold has h follow path: whether it was watched already (false: h looks
// at it this poll, as one just watched may have been written before).
func (w *fileWatch) hold(path string, h *keyedHub) bool {
	if w == nil || w.fw == nil || path == "" {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.holders[path] == nil {
		w.holders[path] = map[*keyedHub]int{}
	}
	w.holders[path][h]++
	return w.watchLocked(path)
}

// watchLocked watches path if it isn't already: whether it was. w.mu is
// held.
func (w *fileWatch) watchLocked(path string) bool {
	if w.on[path] {
		return true
	}
	if w.fw.Add(path) == nil {
		w.on[path] = true // from now: what was written before isn't said
	}
	return false
}

// watching is whether path was watched already, watching it again if it's
// held and isn't (it was replaced, or wasn't there).
func (w *fileWatch) watching(path string) bool {
	if w == nil || w.fw == nil || path == "" {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.holders[path]) == 0 {
		return false
	}
	return w.watchLocked(path)
}

// release has h follow path no more: one nobody holds isn't watched.
func (w *fileWatch) release(path string, h *keyedHub) {
	if w == nil || w.fw == nil || path == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if hs := w.holders[path]; hs != nil {
		if hs[h]--; hs[h] <= 0 {
			delete(hs, h)
		}
		if len(hs) == 0 {
			delete(w.holders, path)
			if w.on[path] {
				w.fw.Remove(path) //nolint:errcheck // gone with it
				delete(w.on, path)
			}
		}
	}
}

// run has the hubs look as their files are written, until ctx is done.
func (w *fileWatch) run(ctx context.Context) {
	if w.fw == nil {
		return
	}
	defer w.fw.Close()
	var due <-chan time.Time
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.fw.Events:
			if !ok {
				return
			}
			if ev.Op == fsnotify.Chmod {
				continue
			}
			w.mu.Lock()
			if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
				// Replaced, or gone: watched again (if it's there) at its
				// hub's next look, which is now.
				if w.on[ev.Name] {
					w.fw.Remove(ev.Name) //nolint:errcheck // gone already
					delete(w.on, ev.Name)
				}
			}
			for h := range w.holders[ev.Name] {
				w.written[h] = true
			}
			w.mu.Unlock()
			if due == nil {
				wait := fileSettle
				if since := time.Since(last); since < fileGap {
					wait = max(wait, fileGap-since)
				}
				due = time.After(wait)
			}
		case <-w.fw.Errors:
		case <-due:
			due, last = nil, time.Now()
			w.mu.Lock()
			hubs := make([]*keyedHub, 0, len(w.written))
			for h := range w.written {
				hubs = append(hubs, h)
			}
			clear(w.written)
			w.mu.Unlock()
			for _, h := range hubs {
				h.nudge()
			}
		}
	}
}
