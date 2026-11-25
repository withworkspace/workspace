package acp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

// A record is a session's events as JSON lines, one per line, written as
// they happen. It's how ws keeps an ACP session: an agent's own store
// isn't ws's to read (Cursor's is a database), and the agent is gone once
// the session ends. Another process reads it back (ReadRecordFile, Replay)
// the way it reads a session file today, following it as it grows.

// Event is one line of a record: one of its fields is set.
type Event struct {
	At time.Time `json:"at"`
	// Session is a session started or loaded: its id.
	Session string `json:"session,omitempty"`
	// Sent is a message the client sent (a prompt).
	Sent []ContentBlock `json:"sent,omitempty"`
	// Update is a session update, as the agent sent it.
	Update json.RawMessage `json:"update,omitempty"`
	// Stop is a turn's end: its stop reason. Error is a turn that failed.
	Stop  string `json:"stop,omitempty"`
	Error string `json:"error,omitempty"`
	// Usage is what the turn that stopped spent, where the agent said.
	Usage *Usage `json:"usage,omitempty"`
	// Ask is the agent asking something of the client, and Answer its
	// answer (Answer.Ask is the ask's id).
	Ask    *Ask    `json:"ask,omitempty"`
	Answer *Answer `json:"answer,omitempty"`
	// Ended is the agent gone: how ("exited", or its error).
	Ended string `json:"ended,omitempty"`
}

// Recorder writes events to a record. Its writes are whole lines, so a
// reader following the file never sees half of one as complete.
type Recorder struct {
	mu  sync.Mutex
	w   io.Writer
	c   io.Closer
	err error
}

// NewRecorder writes a record to w.
func NewRecorder(w io.Writer) *Recorder { return &Recorder{w: w} }

// AppendRecord opens the record at path to add to it (creating it).
func AppendRecord(path string) (*Recorder, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	return &Recorder{w: f, c: f}, nil
}

// Write adds an event, timed now if it isn't. After a failed write it
// writes nothing more, and Err says why.
func (r *Recorder) Write(e Event) error {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	if _, err := r.w.Write(append(data, '\n')); err != nil {
		r.err = err
	}
	return r.err
}

// Err is the first write that failed, or nil.
func (r *Recorder) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// Close closes the record's file (if it opened one).
func (r *Recorder) Close() error {
	if r.c == nil {
		return nil
	}
	return r.c.Close()
}

// ReadRecord reads a record's events. A line that isn't an event is
// skipped (one half-written as the record was read, say).
func ReadRecord(r io.Reader) ([]Event, error) {
	var out []Event
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 64<<20)
	for sc.Scan() {
		var e Event
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}

// ReadRecordFile reads the record at path from offset on: its events, and
// the offset to read from next. Only whole lines are read; a line still
// being written is left for next time. A record that isn't there yet has
// no events.
func ReadRecordFile(path string, offset int64) ([]Event, int64, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, offset, nil
	}
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, offset, err
	}
	end := bytes.LastIndexByte(data, '\n') + 1
	events, err := ReadRecord(bytes.NewReader(data[:end]))
	return events, offset + int64(end), err
}

// Replay folds events into a transcript: what was sent and what the agent
// said and did. tr may hold what earlier events made (following a record),
// or be nil for a new one.
func Replay(tr *Transcript, events []Event) *Transcript {
	if tr == nil {
		tr = &Transcript{}
	}
	for _, e := range events {
		switch {
		case e.Sent != nil:
			tr.Sent(e.At, e.Sent...)
		case e.Update != nil:
			if u, err := ParseUpdate(e.Update); err == nil {
				tr.Apply(e.At, u)
			}
		}
	}
	return tr
}
