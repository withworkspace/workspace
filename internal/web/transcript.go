package web

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
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
	// Version is how much of the session file these are as of (its bytes
	// to its last whole line): a change told over the socket as of less
	// (TranscriptOp) is in it already.
	Version int64 `json:"version"`
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
	t.Version = transcriptSize(sess.Harness, data)
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
	// A session a page follows is served as its hub last read it: the page
	// follows before it reads, and is told of what's been added since.
	t := s.followedTranscript(r.PathValue("slug"), r.PathValue("id"))
	if t == nil {
		sess, ok := s.crewSession(w, r)
		if !ok {
			return
		}
		var err error
		if t, err = LoadTranscript(sess); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	// ?last=n is only its latest n entries, for what needs no more than
	// what it's doing now (the foot).
	if last, err := strconv.Atoi(r.URL.Query().Get("last")); err == nil && last > 0 && len(t.Entries) > last {
		cut := len(t.Entries) - last
		t.Earlier, t.Entries = t.Earlier+cut, t.Entries[cut:]
	}
	writeJSON(w, http.StatusOK, t)
}

// transcriptPoll is how often a followed session file, or run's log, is
// looked at. A look is a stat: the file's read only when it's changed.
var transcriptPoll = 400 * time.Millisecond

// sessionLookEvery is how often a session that hasn't started is looked
// for, while it's followed.
var sessionLookEvery = 2 * time.Second

// transcriptSize is how much of a session file is read: to its last whole
// line (one being written is read once it's ended), or all of an ACP
// record, which is read whole.
func transcriptSize(harnessName string, data []byte) int64 {
	if harness.CapsOf(harnessName).Transcript == harness.TranscriptACP {
		return int64(len(data))
	}
	return int64(bytes.LastIndexByte(data, '\n') + 1)
}

// TranscriptOp is how a crew's session a page holds has changed, told over
// the socket (SocketSaid.Transcript) so the page needn't read it again:
// its entries from From on (counted from the session's first; those
// before are as the page has them), the session as it now stands
// (Harness, Available, Why), as of Version (Transcript's). A page holding
// fewer than From, or none of those from From on (they're before the
// latest it holds), reads it whole.
type TranscriptOp struct {
	Version   int64                     `json:"version"`
	From      int                       `json:"from"`
	Entries   []harness.TranscriptEntry `json:"entries"`
	Harness   string                    `json:"harness"`
	Available bool                      `json:"available"`
	Why       string                    `json:"why,omitempty"`
}

// transcriptFollow is a crew's session someone follows: where it is
// (found once it's started), read as it grows.
type transcriptFollow struct {
	slug, id  string
	sess      CrewSession
	reader    *harness.TranscriptReader
	lookedFor time.Time

	// read is the session as last read, which a page that follows it is
	// served (followedTranscript): nil till it's been read.
	mu   sync.Mutex
	read *Transcript
}

// keep notes the session as a read found it.
func (f *transcriptFollow) keep(got harness.TranscriptRead) {
	t := &Transcript{Harness: f.sess.Harness, Available: true, Why: f.sess.Why, Entries: got.Entries, Version: got.Size}
	f.mu.Lock()
	f.read = t
	f.mu.Unlock()
}

// followedTranscript is a followed session's transcript as its hub last
// read it, its latest entries: no file's read, and nothing's parsed again.
// nil when nobody follows it, or it hasn't been read.
func (s *Server) followedTranscript(slug, id string) *Transcript {
	s.followedMu.Lock()
	f := s.followed[slug+"/"+id]
	s.followedMu.Unlock()
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.read == nil {
		return nil
	}
	t := *f.read
	if n := len(t.Entries); n > TranscriptShown {
		t.Earlier, t.Entries = n-TranscriptShown, t.Entries[n-TranscriptShown:]
	}
	return &t
}

// transcriptHub follows crew's sessions (transcript:<slug>/<crew>): each
// file's looked at every transcriptPoll while it's followed (a stat), and
// read, from where it got to, only when it's grown. A session that hasn't
// started is looked for every sessionLookEvery, until it has.
func (s *Server) transcriptHub() *keyedHub {
	var mu sync.Mutex
	paths := map[string]string{} // each followed session's file, once it's found
	h := newKeyedHub(transcriptTopic, transcriptPoll, func(key string) string {
		mu.Lock()
		path := paths[key]
		mu.Unlock()
		if path == "" {
			return fmt.Sprint(time.Now().UnixMilli() / sessionLookEvery.Milliseconds()) // a look for it, now and then
		}
		info, err := os.Stat(path)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%d %d", info.Size(), info.ModTime().UnixNano())
	})
	h.pathOf = func(key string) string {
		mu.Lock()
		defer mu.Unlock()
		return paths[key]
	}
	h.gone = func(key string) {
		mu.Lock()
		delete(paths, key)
		mu.Unlock()
		s.followedMu.Lock()
		delete(s.followed, key)
		s.followedMu.Unlock()
	}
	// find looks for where f's session is, and says whether it's found.
	find := func(key string, f *transcriptFollow) bool {
		f.lookedFor = time.Now()
		got, err := s.App.Crew().Session(f.slug, f.id)
		if err != nil {
			return false
		}
		f.sess = CrewSession(got)
		f.reader = harness.NewTranscriptReader(f.sess.Harness)
		mu.Lock()
		paths[key] = f.sess.Path
		mu.Unlock()
		return f.sess.Path != ""
	}
	h.change = func(key string, was any) (any, any) {
		f, _ := was.(*transcriptFollow)
		if f == nil {
			// Followed from now: its reader's brought up to it as it is,
			// and the page reads it after.
			slug, id, _ := strings.Cut(key, "/")
			f = &transcriptFollow{slug: slug, id: id}
			if find(key, f) {
				if got, err := f.reader.Read(f.sess.Path); err == nil {
					f.keep(got)
				} // else read again at the next change
			}
			s.followedMu.Lock()
			if s.followed == nil {
				s.followed = map[string]*transcriptFollow{}
			}
			s.followed[key] = f
			s.followedMu.Unlock()
			return nil, f
		}
		from0 := false
		if f.sess.Path == "" {
			if f.sess.Final || time.Since(f.lookedFor) < sessionLookEvery || !find(key, f) {
				return nil, f
			}
			from0 = true // started since: the page has none of it
		}
		got, err := f.reader.Read(f.sess.Path)
		if err == nil {
			f.keep(got)
		}
		switch {
		case err != nil:
			return nil, f // half there: the next look
		case got.Restarted:
			return reread, f
		case got.From == len(got.Entries) && !from0:
			return nil, f
		}
		return &TranscriptOp{Version: got.Size, From: got.From, Entries: got.Entries[got.From:],
			Harness: f.sess.Harness, Available: true, Why: f.sess.Why}, f
	}
	return h
}
