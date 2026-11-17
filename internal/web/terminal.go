package web

import (
	"encoding/json"
	"fmt"
	"net/http"
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

// crewScreen streams a running crew's screen: one now, then each change
// (at most one per screenEvery), and exit when it ends.
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
