package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
)

// Transcript is GET /api/missions/{slug}/crew/{id}/transcript: a crew's
// session as its harness keeps it, the way into an agent (its view, on the
// crew page) and what's left of one that's ended.
type Transcript struct {
	Harness   string                    `json:"harness"`
	Available bool                      `json:"available"`
	Why       string                    `json:"why,omitempty"` // why there's none
	Entries   []harness.TranscriptEntry `json:"entries"`
	// Earlier is how many entries before these were left out.
	Earlier int `json:"earlier,omitempty"`
}

// CrewSession is where a crew's harness keeps its session: Path, or Why
// there's none to read. Final is that there never will be (its harness
// keeps none ws can read), so there's no looking again.
type CrewSession struct {
	Harness string
	Path    string
	Why     string
	Final   bool
}

// TranscriptShown is how many of a session's latest entries a page gets.
const TranscriptShown = 400

// LoadTranscript reads a crew's session.
func LoadTranscript(sess CrewSession) (*Transcript, error) {
	t := &Transcript{Harness: sess.Harness, Why: sess.Why, Entries: []harness.TranscriptEntry{}}
	if sess.Path == "" {
		return t, nil
	}
	data, err := os.ReadFile(sess.Path)
	if err != nil {
		return nil, err
	}
	t.Available = true
	if entries := harness.ReadTranscript(sess.Harness, data); entries != nil {
		t.Entries = entries
	}
	if n := len(t.Entries); n > TranscriptShown {
		t.Earlier, t.Entries = n-TranscriptShown, t.Entries[n-TranscriptShown:]
	}
	return t, nil
}

func (s *Server) crewSession(w http.ResponseWriter, r *http.Request) (CrewSession, bool) {
	got, err := s.App.Crew().Session(r.PathValue("slug"), r.PathValue("id"))
	sess := CrewSession(got)
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
		return CrewSession{}, false
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
		return CrewSession{}, false
	}
	return sess, true
}

func (s *Server) crewTranscript(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.crewSession(w, r)
	if !ok {
		return
	}
	t, err := LoadTranscript(sess)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// transcriptPoll is how often a stream looks at the session file. A look
// is a stat: the file's read again only when it's changed.
var transcriptPoll = 400 * time.Millisecond

// crewTranscriptStream is GET .../transcript/stream: the transcript, then
// again each time the session file changes, as "transcript" events. A
// session that hasn't started yet (no file) is looked for until it has.
func (s *Server) crewTranscriptStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	sess, ok := s.crewSession(w, r)
	if !ok {
		return
	}
	if s.Hold != nil {
		defer s.Hold()()
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	fmt.Fprint(w, "retry: 2000\n")
	slug, id := r.PathValue("slug"), r.PathValue("id")
	var size int64 = -1
	var mod time.Time
	send := func() {
		t, err := LoadTranscript(sess)
		if err != nil {
			return // half there: the next look
		}
		data, _ := json.Marshal(t)
		fmt.Fprintf(w, "event: transcript\ndata: %s\n\n", data)
		flusher.Flush()
	}
	lookedFor := time.Time{}
	look := func() {
		if sess.Path == "" {
			// Not started when the page opened: look again for it, now and
			// then (a harness that keeps none, never).
			if sess.Final || time.Since(lookedFor) < 2*time.Second {
				return
			}
			lookedFor = time.Now()
			got, err := s.App.Crew().Session(slug, id)
			next := CrewSession(got)
			if err != nil || next.Path == "" {
				return
			}
			sess = next
		}
		info, err := os.Stat(sess.Path)
		if err != nil {
			return
		}
		if info.Size() == size && info.ModTime().Equal(mod) {
			return
		}
		size, mod = info.Size(), info.ModTime()
		send()
	}
	if sess.Path == "" {
		send() // say why there's nothing yet
	}
	look()
	tick := time.NewTicker(transcriptPoll)
	defer tick.Stop()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			look()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
