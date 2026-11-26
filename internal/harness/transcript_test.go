package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Transcripts read from real sessions (claude 2.1.319, pi 1.5.0): what the
// agent was sent, said and did, each call with its result.
func TestReadTranscript(t *testing.T) {
	data, _ := os.ReadFile("testdata/pi-session.jsonl")
	pi := ReadTranscript("pi", data)
	var got []string
	for _, e := range pi {
		got = append(got, string(e.By))
	}
	if strings.Join(got, " ") != "user call call call agent user agent user agent" || pi[1].Input != "cat note.txt" || pi[8].Text != "Hello" {
		t.Errorf("pi: %v", got)
	}
	if pi[1].Result != "hello from a file" || !pi[3].Refused {
		t.Errorf("pi calls: %+v %+v", pi[1], pi[3])
	}

	data, _ = os.ReadFile("testdata/claude-session.jsonl")
	cl := ReadTranscript("claude", data)
	if len(cl) < 3 || cl[0].By != "user" || cl[len(cl)-1].By != "agent" {
		t.Fatalf("claude: %+v", cl)
	}
	calls := 0
	for _, e := range cl {
		if e.By == "call" {
			calls++
			if e.Tool == "" || e.Input == "" {
				t.Errorf("a call without its tool or input: %+v", e)
			}
		}
	}
	if calls == 0 {
		t.Error("no calls read")
	}
	if ReadTranscript("cursor", data) != nil {
		t.Error("read a Cursor transcript")
	}
}

// Sessions are found by id in the folder each harness keeps them in.
func TestSessionFile(t *testing.T) {
	home := t.TempDir()
	old := homeDir
	homeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { homeDir = old })
	os.MkdirAll(filepath.Join(home, ".claude", "projects", "-w-repos-api-cache"), 0o755)
	os.WriteFile(filepath.Join(home, ".claude", "projects", "-w-repos-api-cache", "abc-1.jsonl"), nil, 0o644)
	os.MkdirAll(filepath.Join(home, ".pi", "agent", "sessions", "--w--"), 0o755)
	os.WriteFile(filepath.Join(home, ".pi", "agent", "sessions", "--w--", "2026-11-10T00-00-00-000Z_def-2.jsonl"), nil, 0o644)
	if p, ok := SessionFile("claude", "abc-1"); !ok || filepath.Base(p) != "abc-1.jsonl" {
		t.Errorf("claude: %q %v", p, ok)
	}
	if p, ok := SessionFile("pi", "def-2"); !ok || !strings.HasSuffix(p, "_def-2.jsonl") {
		t.Errorf("pi: %q %v", p, ok)
	}
	for _, c := range [][2]string{{"claude", "nope"}, {"cursor", "abc-1"}, {"claude", "../x"}, {"claude", ""}} {
		if _, ok := SessionFile(c[0], c[1]); ok {
			t.Errorf("%v found", c)
		}
	}
}

// A session's file is remembered once found, so it's looked for in every
// folder once; one that isn't found isn't looked for again for a while,
// and one that's gone is looked for again.
func TestSessionFile_Remembered(t *testing.T) {
	home := t.TempDir()
	old, miss := homeDir, sessionMissFor
	homeDir = func() (string, error) { return home, nil }
	sessionMissFor = time.Hour
	t.Cleanup(func() { homeDir, sessionMissFor = old, miss })
	folder := filepath.Join(home, ".claude", "projects", "-w-repos-api-cache")
	os.MkdirAll(folder, 0o755)

	if _, ok := SessionFile("claude", "late-1"); ok {
		t.Fatal("found before it's there")
	}
	os.WriteFile(filepath.Join(folder, "late-1.jsonl"), nil, 0o644)
	if _, ok := SessionFile("claude", "late-1"); ok {
		t.Error("looked for again straight after it wasn't found")
	}
	sessionMissFor = 0
	p, ok := SessionFile("claude", "late-1")
	if !ok {
		t.Fatal("not found once it's there")
	}
	// Moved to another folder: the one remembered is gone, so it's looked
	// for again, and found where it is.
	other := filepath.Join(home, ".claude", "projects", "-w-repos-api-other")
	os.MkdirAll(other, 0o755)
	os.Rename(p, filepath.Join(other, "late-1.jsonl"))
	if p, ok := SessionFile("claude", "late-1"); !ok || filepath.Dir(p) != other {
		t.Errorf("moved: %q %v", p, ok)
	}
}

// Calls the web UI draws as themselves keep what they are as data: an
// edit, the plan, a question. A call with nothing back yet is pending;
// subagents' lines (sidechains) are left to their Task call.
func TestTranscriptDetails(t *testing.T) {
	lines := strings.Join([]string{
		`{"type":"user","timestamp":"2026-11-10T10:00:00Z","message":{"role":"user","content":"Fix the cache key"}}`,
		`{"type":"assistant","timestamp":"2026-11-10T10:00:05Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"TodoWrite","input":{"todos":[{"content":"Read cache.go","status":"completed","activeForm":"Reading cache.go"},{"content":"Key by tenant","status":"in_progress","activeForm":"Keying by tenant"}]}}]}}`,
		`{"type":"user","timestamp":"2026-11-10T10:00:06Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`,
		`{"type":"assistant","timestamp":"2026-11-10T10:00:10Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t2","name":"Edit","input":{"file_path":"/w/cache.go","old_string":"return id","new_string":"return tenant + id"}}]}}`,
		`{"type":"user","timestamp":"2026-11-10T10:00:11Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"edited"}]}}`,
		`{"type":"assistant","isSidechain":true,"timestamp":"2026-11-10T10:00:12Z","message":{"role":"assistant","content":[{"type":"text","text":"a subagent's aside"}]}}`,
		`{"type":"assistant","timestamp":"2026-11-10T10:00:20Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t3","name":"AskUserQuestion","input":{"questions":[{"question":"Which key?","header":"Key","multiSelect":false,"options":[{"label":"Per tenant"},{"label":"Global"}]}]}}]}}`,
	}, "\n")
	es := ReadTranscript("claude", []byte(lines))
	if len(es) != 4 {
		t.Fatalf("%d entries: %+v", len(es), es)
	}
	if es[0].At.IsZero() || es[0].Text != "Fix the cache key" {
		t.Errorf("first: %+v", es[0])
	}
	if len(es[1].Todos) != 2 || es[1].Todos[1].Status != "in_progress" || es[1].Todos[1].Active != "Keying by tenant" || es[1].Pending {
		t.Errorf("plan: %+v", es[1])
	}
	if e := es[2].Edit; e == nil || e.Path != "/w/cache.go" || e.Old != "return id" || e.New != "return tenant + id" {
		t.Errorf("edit: %+v", es[2])
	}
	if q := es[3].Questions; len(q) != 1 || q[0].Question != "Which key?" || len(q[0].Options) != 2 || !es[3].Pending || es[3].ID != "t3" {
		t.Errorf("question: %+v", es[3])
	}
}
