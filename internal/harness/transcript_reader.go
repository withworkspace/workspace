package harness

import (
	"bytes"
	"io"
	"os"
	"slices"
)

// TranscriptReader reads a session's transcript as its file grows: each
// read parses only the lines appended since the last, carrying on from
// where it got to (a call that was pending is answered by a later line),
// and says which entries changed. A session kept as ws's ACP record isn't
// a simple fold over its lines, so it's read whole each time.
type TranscriptReader struct {
	harness string
	t       transcript
	read    int64  // bytes of the file read so far
	rest    []byte // a last line not yet ended, held until it is
}

// NewTranscriptReader reads the transcripts of harness's sessions.
func NewTranscriptReader(harnessName string) *TranscriptReader {
	return &TranscriptReader{harness: harnessName}
}

// TranscriptRead is what a read found: every entry as it now stands, the
// first that changed since the last read (len(Entries) for none), how much
// of the file it's as of (Size), and whether it was read from the start
// again (Restarted: the file was replaced, or shrank).
type TranscriptRead struct {
	Entries   []TranscriptEntry
	From      int
	Size      int64
	Restarted bool
}

// Read reads what's been appended to the session file at path since the
// last read: the whole of it, the first time.
func (r *TranscriptReader) Read(path string) (TranscriptRead, error) {
	f, err := os.Open(path)
	if err != nil {
		return TranscriptRead{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return TranscriptRead{}, err
	}
	restarted := false
	if CapsOf(r.harness).Transcript == TranscriptACP {
		data, err := io.ReadAll(f)
		if err != nil {
			return TranscriptRead{}, err
		}
		prev := r.t.out
		r.t.out = readACPTranscript(data)
		from := 0
		for from < len(prev) && from < len(r.t.out) && sameEntry(prev[from], r.t.out[from]) {
			from++
		}
		r.read = int64(len(data))
		return TranscriptRead{Entries: slices.Clone(r.t.out), From: from, Size: r.read}, nil
	}
	if info.Size() < r.read {
		*r = TranscriptReader{harness: r.harness}
		restarted = true
	}
	if _, err := f.Seek(r.read, io.SeekStart); err != nil {
		return TranscriptRead{}, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return TranscriptRead{}, err
	}
	r.read += int64(len(data))
	data = append(r.rest, data...)
	end := bytes.LastIndexByte(data, '\n')
	lines := data[:end+1]
	r.rest = slices.Clone(data[end+1:])
	for line := range bytes.SplitSeq(lines, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		switch r.harness {
		case "claude":
			r.t.claudeLine(string(line))
		case "pi":
			r.t.piLine(string(line))
		}
	}
	return TranscriptRead{Entries: slices.Clone(r.t.out), From: r.t.changedFrom(), Size: r.read - int64(len(r.rest)), Restarted: restarted}, nil
}

// sameEntry is whether two entries say the same.
func sameEntry(a, b TranscriptEntry) bool {
	return a.By == b.By && a.Text == b.Text && a.Tool == b.Tool && a.Input == b.Input && a.Result == b.Result &&
		a.Pending == b.Pending && a.Error == b.Error && a.Refused == b.Refused && slices.Equal(a.Todos, b.Todos) && len(a.Questions) == len(b.Questions)
}
