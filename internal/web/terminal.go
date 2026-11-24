package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
)

// Crew terminals: a running crew's screen, drawn by the page as the deck
// draws it, and keys and pastes typed into it.

// Screen is a crew's screen: a line per row, with its colours as ANSI
// escapes, the cursor, and the terminal's size.
type Screen struct {
	Text          string `json:"text"`
	Width         int    `json:"w"`
	Height        int    `json:"h"`
	CursorX       int    `json:"cx"`
	CursorY       int    `json:"cy"`
	CursorVisible bool   `json:"cv"`
	Title         string `json:"title,omitempty"`
}

// TermInput is POST …/crew/{id}/input: a key, or text to paste.
type TermInput struct {
	Key   *TermKey `json:"key,omitempty"`
	Paste string   `json:"paste,omitempty"`
}

// TermKey is a key as the browser names it (KeyboardEvent.key: "Enter",
// "ArrowUp", "a", "A"), with its modifiers.
type TermKey struct {
	Key   string `json:"key"`
	Ctrl  bool   `json:"ctrl,omitempty"`
	Alt   bool   `json:"alt,omitempty"`
	Shift bool   `json:"shift,omitempty"`
}

// screenEvery caps how often a page gets a crew's screen.
const screenEvery = 33 * time.Millisecond

// crewScreen streams a running crew's screen as server-sent events: one
// now, then each change (at most one per screenEvery), and exit when it
// ends. The web UI follows a screen over its socket (followScreen); this
// is for a tab from before, still open as its daemon's ws was upgraded.
func (s *Server) crewScreen(w http.ResponseWriter, r *http.Request) {
	slug, id := r.PathValue("slug"), r.PathValue("id")
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	changes, ended, cancel, ok := s.App.Crew().Watch(slug, id)
	if !ok {
		jsonError(w, http.StatusNotFound, id+" isn't running")
		return
	}
	defer cancel()
	if s.Hold != nil {
		defer s.Hold()()
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	send := func() bool {
		got, ok := s.App.Crew().Screen(slug, id)
		sc := Screen(got)
		if !ok {
			return false
		}
		data, _ := json.Marshal(sc)
		fmt.Fprintf(w, "event: screen\ndata: %s\n\n", data)
		flusher.Flush()
		return true
	}
	fmt.Fprint(w, "retry: 2000\n")
	if !send() {
		return
	}
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ended:
			fmt.Fprint(w, "event: exit\ndata: {}\n\n")
			flusher.Flush()
			return
		case <-changes:
			if !send() {
				return
			}
			time.Sleep(screenEvery)
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// ScreenOp is a running crew's screen, told over the socket to a page that
// follows it (screen:<slug>/<crew>): the screen as it now is, each time it
// changes (at most one per screenEvery, and only the latest), or that the
// crew have ended (Exit).
type ScreenOp struct {
	Screen *Screen `json:"screen,omitempty"`
	Exit   bool    `json:"exit,omitempty"`
}

// followScreen follows a running crew's screen (key slug/crew): its
// screen now, then as it changes, then its end. false for crew that aren't
// running here. Only the latest screen waits to be sent: a page behind is
// sent the one it's behind with, not every one between.
func (s *Server) followScreen(key string) (<-chan told, func(), bool) {
	slug, id, _ := strings.Cut(key, "/")
	changes, ended, cancel, ok := s.App.Crew().Watch(slug, id)
	if !ok {
		return nil, nil, false
	}
	name := screenTopic + key
	out := make(chan told, 1)
	quit := make(chan struct{})
	latest := func(op *ScreenOp) {
		select {
		case <-out: // the one waiting is older
		default:
		}
		out <- told{Name: name, Op: op}
	}
	screen := func() bool {
		got, ok := s.App.Crew().Screen(slug, id)
		if ok {
			sc := Screen(got)
			latest(&ScreenOp{Screen: &sc})
		}
		return ok
	}
	go func() {
		if !screen() {
			latest(&ScreenOp{Exit: true})
			return
		}
		for {
			select {
			case <-quit:
				return
			case <-ended:
				latest(&ScreenOp{Exit: true})
				return
			case <-changes:
				if !screen() {
					latest(&ScreenOp{Exit: true})
					return
				}
				select {
				case <-quit:
					return
				case <-time.After(screenEvery):
				}
			}
		}
	}()
	var once sync.Once
	return out, func() { once.Do(func() { close(quit); cancel() }) }, true
}

// crewInput types a key or pastes into a running crew.
func (s *Server) crewInput(w http.ResponseWriter, r *http.Request) {
	slug, id := r.PathValue("slug"), r.PathValue("id")
	var in TermInput
	if !readBody(w, r, &in) {
		return
	}
	var err error
	switch {
	case in.Key != nil:
		err = s.App.Crew().Key(slug, id, ops.TermKey(*in.Key))
	case in.Paste != "":
		err = s.App.Crew().Paste(slug, id, in.Paste)
	default:
		jsonError(w, http.StatusBadRequest, "send a key or a paste")
		return
	}
	if err != nil {
		jsonError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct{}{})
}
