package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/ops"
)

// sessionCrew is crew whose session is the file a test writes.
type sessionCrew struct {
	ops.NoCrew
	path string
}

func (c sessionCrew) Session(slug, id string) (ops.CrewSession, error) {
	if _, err := os.Stat(c.path); err != nil {
		return ops.CrewSession{Harness: "claude", Why: "it hasn't started"}, nil
	}
	return ops.CrewSession{Harness: "claude", Path: c.path}, nil
}

// A crew's page follows its session over the socket, and from what it read
// is told each change: the entries from the first that changed (a call
// answered, as well as what's said), as of how much of the file. Applied
// as the page applies them, they come to what reading it whole does, line
// after line of a real session; and a session that hadn't started when
// the page opened is told whole once it has.
func TestSocket_FollowsATranscript(t *testing.T) {
	whole, err := os.ReadFile("../harness/testdata/claude-session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(whole), "\n")
	path := filepath.Join(t.TempDir(), "session.jsonl")
	poll, look := transcriptPoll, sessionLookEvery
	transcriptPoll, sessionLookEvery = 20*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() { transcriptPoll, sessionLookEvery = poll, look })
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	s.App = &fakeApp{Workspace: s.App.(*ops.Workspace), crew: sessionCrew{path: path}}

	read := func() Transcript {
		t.Helper()
		res := do(t, s, "GET", "/api/missions/m/crew/c1/transcript", true, nil)
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("read: %d", res.StatusCode)
		}
		var tr Transcript
		json.NewDecoder(res.Body).Decode(&tr)
		return tr
	}
	conn, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, conn)
	st.ask(SocketAsk{Op: SocketFollow, Topic: "transcript:m/c1"})
	if got := st.next(3 * time.Second); got != "following transcript:m/c1" {
		t.Fatalf("following: %q", got)
	}
	held := read()
	if held.Available {
		t.Fatalf("a session that hasn't started: %+v", held)
	}

	// The page applies a change as the web UI does (useTranscript's).
	apply := func(op *TranscriptOp) {
		t.Helper()
		if op.Version < held.Version {
			return
		}
		if op.From < held.Earlier || op.From > held.Earlier+len(held.Entries) {
			t.Fatalf("a change from %d, holding %d from %d: it'd read it whole", op.From, len(held.Entries), held.Earlier)
		}
		held.Entries = append(held.Entries[:op.From-held.Earlier], op.Entries...)
		held.Version, held.Available, held.Harness = op.Version, op.Available, op.Harness
	}
	for i, line := range lines {
		if line == "" {
			continue
		}
		f, _ := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		f.WriteString(line)
		f.Close()
		want := harness.ReadTranscript("claude", []byte(strings.Join(lines[:i+1], "")))
		// A line that says nothing a page shows is told nothing.
		deadline := time.Now().Add(3 * time.Second)
		for asJSON(t, held.Entries) != asJSON(t, want) && time.Now().Before(deadline) {
			if got := st.next(time.Until(deadline)); got == "changed transcript:m/c1" {
				if st.last.Transcript == nil {
					t.Fatalf("line %d: told it changed, but not how", i+1)
				}
				apply(st.last.Transcript)
			}
		}
		if asJSON(t, held.Entries) != asJSON(t, want) {
			t.Fatalf("line %d: as told, %d entries; read whole, %d", i+1, len(held.Entries), len(want))
		}
	}
	if !held.Available || held.Version != int64(len(whole)) {
		t.Errorf("at the end: available %v, version %d of %d", held.Available, held.Version, len(whole))
	}

	// Followed, it's served as its hub last read it: the file isn't read
	// for a page's read (here, it can't be).
	os.Chmod(path, 0)
	t.Cleanup(func() { os.Chmod(path, 0o644) })
	got := read()
	if !got.Available || got.Version != held.Version || asJSON(t, got.Entries) != asJSON(t, held.Entries) {
		t.Errorf("read while followed: available %v, version %d, %d entries; want what the page holds", got.Available, got.Version, len(got.Entries))
	}
}

func asJSON(t *testing.T, v []harness.TranscriptEntry) string {
	t.Helper()
	if v == nil {
		v = []harness.TranscriptEntry{}
	}
	data, _ := json.Marshal(v)
	return string(data)
}

// ?last=n reads only a session's latest n entries, counting those before
// as left out, as the foot reads what crew is doing.
func TestTranscript_Last(t *testing.T) {
	whole, err := os.ReadFile("../harness/testdata/claude-session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, whole, 0o644); err != nil {
		t.Fatal(err)
	}
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	s.App = &fakeApp{Workspace: s.App.(*ops.Workspace), crew: sessionCrew{path: path}}
	read := func(q string) Transcript {
		t.Helper()
		res := do(t, s, "GET", "/api/missions/m/crew/c1/transcript"+q, true, nil)
		defer res.Body.Close()
		var tr Transcript
		json.NewDecoder(res.Body).Decode(&tr)
		return tr
	}
	all, last := read(""), read("?last=2")
	n := len(all.Entries)
	if n < 3 {
		t.Fatalf("the session has %d entries: too few to test", n)
	}
	if len(last.Entries) != 2 || last.Earlier != all.Earlier+n-2 || last.Version != all.Version {
		t.Fatalf("last 2 of %d: %d from %d, version %d (%d)", n, len(last.Entries), last.Earlier, last.Version, all.Version)
	}
	if last.Entries[1].Text != all.Entries[n-1].Text || last.Entries[1].By != all.Entries[n-1].By {
		t.Errorf("not the latest: %+v", last.Entries[1])
	}
}
