package spend

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

func assistant(id string, out int) string {
	return `{"type":"assistant","timestamp":"2026-11-25T10:00:00Z","message":{"id":"` + id +
		`","model":"claude-opus-5-5","usage":{"input_tokens":1,"output_tokens":` + strconv.Itoa(out) +
		`,"cache_creation_input_tokens":10,"cache_read_input_tokens":100}}}` + "\n"
}

func lines(t *testing.T, root, mission string) map[string]Line {
	t.Helper()
	got, err := OfMission(root, mission)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Line{}
	for _, l := range got {
		out[l.Kind+" "+l.Model] = l
	}
	return out
}

// A run's spend is kept with its cost, by model; keeping it again
// replaces it rather than counting it twice.
func TestKeep(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	f := For{Mission: "m", Kind: Orchestrate, Subject: "run-1"}
	at := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	usage := []harness.Usage{{Model: "claude-haiku-5-5", Input: 5, Output: 50, CacheWrite: 500, CacheRead: 5000, Cost: 0.04}}
	for range 2 {
		if err := Keep(root, "run a", f, "claude", at, usage); err != nil {
			t.Fatal(err)
		}
	}
	if err := Keep(root, "run b", f, "claude", at, []harness.Usage{{Model: "claude-haiku-5-5", Output: 1}}); err != nil {
		t.Fatal(err)
	}
	l := lines(t, root, "m")["orchestrate claude-haiku-5-5"]
	if l.Output != 51 || l.CacheRead != 5000 || l.Cost != 0.04 || l.Costed {
		t.Fatalf("got %+v (one run had no cost of its own, so it's not all costed)", l)
	}
	if got := lines(t, root, ""); len(got) != 0 {
		t.Fatalf("the workspace's own: %+v", got)
	}
}

// A crew's session is read as it grows: only what's appended since, only
// whole lines, a message carried over two reads counted once, and a file
// that shrank (replaced) read again from the start.
func TestReadSession(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	path := filepath.Join(t.TempDir(), "s.jsonl")
	f := For{Mission: "m", Kind: Crew, Subject: "c1"}
	read := func(want bool) {
		t.Helper()
		got, err := ReadSession(root, path, SessionSource("claude", "s"), f, "claude", "")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("found something new: %v, want %v", got, want)
		}
	}
	output := func() int64 { return lines(t, root, "m")["crew claude-opus-5-5"].Output }
	write := func(s string, appending bool) {
		flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		if appending {
			flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
		}
		file, err := os.OpenFile(path, flag, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		file.WriteString(s)
		file.Close()
	}

	write(assistant("m1", 10)+assistant("m1", 10)+assistant("m2", 20), false)
	read(true)
	if got := output(); got != 30 {
		t.Fatalf("output %d, want 30", got)
	}
	read(false) // nothing new

	// m2 carries on into what's appended, and a line isn't ended yet.
	half := assistant("m4", 40)
	write(assistant("m2", 20)+assistant("m3", 40)+half[:20], true)
	read(true)
	if got := output(); got != 70 {
		t.Fatalf("output %d, want 70", got)
	}
	write(half[20:], true) // a new message, ended now
	read(true)
	if got := output(); got != 110 {
		t.Fatalf("output %d, want 110", got)
	}

	// Replaced with less: read again from nothing.
	write(assistant("n1", 5), false)
	read(true)
	if got := output(); got != 5 {
		t.Fatalf("output %d, want 5", got)
	}

	// A harness that says nothing of what its crew spend isn't read.
	if got, err := ReadSession(root, path, SessionSource("cursor", "s"), f, "cursor", ""); got || err != nil {
		t.Fatalf("cursor: %v %v", got, err)
	}
}

// An ACP crew's spend is read from ws's own record of its session, on the
// model the crew was launched on where the agent doesn't say.
func TestReadSessionACPRecord(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	path := filepath.Join(t.TempDir(), "crew-c1.acp.jsonl")
	os.WriteFile(path, []byte(`{"at":"2026-11-25T10:00:00Z","sent":[{"type":"text","text":"go"}]}`+"\n"+
		`{"at":"2026-11-25T10:00:09Z","stop":"end_turn","usage":{"input":10,"output":2,"cacheRead":40}}`+"\n"), 0o644)
	f := For{Mission: "m", Kind: Crew, Subject: "c1"}
	if got, err := ReadSession(root, path, RecordSource("m", "c1"), f, "opencode", "big-pickle"); err != nil || !got {
		t.Fatalf("read: %v %v", got, err)
	}
	l := lines(t, root, "m")["crew big-pickle"]
	if l.Input != 10 || l.Output != 2 || l.CacheRead != 40 || l.Harness != "opencode" {
		t.Fatalf("got %+v", l)
	}
	// cursor-agent's ACP says nothing of what it spends: not read.
	if got, _ := ReadSession(root, path, RecordSource("m", "c2"), f, "cursor-acp", ""); got {
		t.Fatal("cursor-acp was read")
	}
}
