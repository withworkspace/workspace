package harness

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// A Claude Code run's result line says what it spent by model, with its
// cost; one from before modelUsage says it on no model in particular.
func TestClaudeResultUsage(t *testing.T) {
	line := `{"type":"result","total_cost_usd":0.33,"usage":{"input_tokens":10,"output_tokens":3030,"cache_creation_input_tokens":67405,"cache_read_input_tokens":306368},` +
		`"modelUsage":{"claude-sonnet-5-5":{"inputTokens":10,"outputTokens":3030,"cacheReadInputTokens":306368,"cacheCreationInputTokens":67405,"costUSD":0.33},` +
		`"claude-haiku-5-5":{"inputTokens":4,"outputTokens":20,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"costUSD":0.01}}}`
	got := claudeResultUsage([]byte(line))
	want := []Usage{
		{Model: "claude-haiku-5-5", Input: 4, Output: 20, Cost: 0.01},
		{Model: "claude-sonnet-5-5", Input: 10, Output: 3030, CacheWrite: 67405, CacheRead: 306368, Cost: 0.33},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	old := `{"type":"result","total_cost_usd":0.02,"usage":{"input_tokens":3,"output_tokens":40,"cache_creation_input_tokens":0,"cache_read_input_tokens":900}}`
	if got := claudeResultUsage([]byte(old)); len(got) != 1 || got[0] != (Usage{Input: 3, Output: 40, CacheRead: 900, Cost: 0.02}) {
		t.Fatalf("without modelUsage: got %+v", got)
	}

	// The report carries it from the stream's result line.
	rep := readClaudeStream(`{"type":"system","session_id":"s"}` + "\n" + line + "\n")
	if len(rep.Usage) != 2 {
		t.Fatalf("the report's usage: %+v", rep.Usage)
	}
}

func assistant(id, model, day string, in, out, cw, cr int) string {
	return `{"type":"assistant","timestamp":"` + day + `T10:00:00Z","message":{"id":"` + id + `","model":"` + model + `","usage":{"input_tokens":` +
		strconv.Itoa(in) + `,"output_tokens":` + strconv.Itoa(out) + `,"cache_creation_input_tokens":` + strconv.Itoa(cw) + `,"cache_read_input_tokens":` + strconv.Itoa(cr) + `}}}`
}

func lines(ss ...string) [][]byte {
	var out [][]byte
	for _, s := range ss {
		out = append(out, []byte(s))
	}
	return out
}

// A message written over several lines is counted once; one carried on
// from the last read isn't counted again; spend is by model and day; what
// Claude Code writes itself, and what you said, count for nothing.
func TestSessionSpend(t *testing.T) {
	got, last := SessionSpend("claude", "", lines(
		assistant("m0", "claude-opus-5-5", "2026-11-24", 1, 1, 1, 1), // carried on from the last read
		assistant("m1", "claude-opus-5-5", "2026-11-24", 2, 10, 100, 1000),
		assistant("m1", "claude-opus-5-5", "2026-11-24", 2, 10, 100, 1000),
		`{"type":"user","message":{"content":"carry on"}}`,
		assistant("m2", "claude-opus-5-5", "2026-11-25", 3, 20, 0, 2000),
		assistant("m3", "claude-haiku-5-5", "2026-11-25", 1, 5, 0, 0),
		assistant("m4", "<synthetic>", "2026-11-25", 0, 0, 0, 0),
		assistant("m5", "claude-opus-5-5", "2026-11-25", 1, 1, 1, 1),
		assistant("m5", "claude-opus-5-5", "2026-11-25", 1, 1, 1, 1),
	), "m0")
	if last != "m5" {
		t.Errorf("last read: %q", last)
	}
	want := []Spent{
		{Day: "2026-11-24", Model: "claude-opus-5-5", Input: 2, Output: 10, CacheWrite: 100, CacheRead: 1000},
		{Day: "2026-11-25", Model: "claude-opus-5-5", Input: 4, Output: 21, CacheWrite: 1, CacheRead: 2001},
		{Day: "2026-11-25", Model: "claude-haiku-5-5", Input: 1, Output: 5},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if got, _ := SessionSpend("cursor", "", lines(assistant("m1", "x", "2026-11-25", 1, 1, 1, 1)), ""); got != nil {
		t.Errorf("a harness whose sessions aren't read: %+v", got)
	}
}

// Cursor's run says what it spent on its result line, on the model it was
// started on (its init line's).
func TestCursorStreamUsage(t *testing.T) {
	data, err := os.ReadFile("testdata/cursor-converse.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	got := readCursorStream(string(data)).Usage
	want := Usage{Model: "Auto", Input: 9603, Output: 617, CacheRead: 29824}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// pi's run says what each of its messages spent, with its cost, as it
// ends: they're added up by model.
func TestPiStreamUsage(t *testing.T) {
	data, err := os.ReadFile("testdata/pi-converse.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	got := readPiStream(string(data)).Usage
	if len(got) != 1 || got[0].Model != "~anthropic/claude-haiku-latest" || got[0].Output == 0 || got[0].Cost == 0 {
		t.Fatalf("got %+v", got)
	}
	// Four assistant messages, each counted once.
	var out int64
	for line := range strings.SplitSeq(string(data), "\n") {
		var e struct {
			Type    string    `json:"type"`
			Message piMessage `json:"message"`
		}
		if json.Unmarshal([]byte(line), &e) == nil && e.Type == "message_end" && e.Message.Role == "assistant" && e.Message.Usage != nil {
			out += e.Message.Usage.Output
		}
	}
	if out == 0 || got[0].Output != out {
		t.Fatalf("output %d, want %d", got[0].Output, out)
	}
}

// pi's session file has each message once, with what it spent and its
// cost; what you said spent nothing.
func TestPiSessionSpend(t *testing.T) {
	got, _ := SessionSpend("pi", "", lines(
		`{"type":"message","id":"a","timestamp":"2026-11-25T10:00:00Z","message":{"role":"user","content":"hi"}}`,
		`{"type":"message","id":"b","timestamp":"2026-11-25T10:00:01Z","message":{"role":"assistant","model":"m","usage":{"input":5,"output":7,"cacheRead":11,"cacheWrite":13,"cost":{"total":0.5}}}}`,
		`{"type":"message","id":"c","timestamp":"2026-11-25T10:00:02Z","message":{"role":"assistant","model":"m","usage":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0,"cost":{"total":0.25}}}}`,
	), "")
	want := Spent{Day: "2026-11-25", Model: "m", Input: 6, Output: 8, CacheRead: 11, CacheWrite: 13, Cost: 0.75}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// An ACP agent's turn says what it spent in its prompt's response, which
// ws keeps on the line its turn stopped: a run reads it from there, and so
// does a crew's record, on the model it's on where the agent doesn't say.
func TestACPUsage(t *testing.T) {
	record := strings.Join([]string{
		`{"at":"2026-11-25T10:00:00Z","session":"s1"}`,
		`{"at":"2026-11-25T10:00:00Z","sent":[{"type":"text","text":"go"}]}`,
		`{"at":"2026-11-25T10:00:09Z","stop":"end_turn","usage":{"model":"gpt-5.5","input":100,"output":20,"cacheRead":900}}`,
		`{"at":"2026-11-25T10:01:00Z","sent":[{"type":"text","text":"again"}]}`,
		`{"at":"2026-11-25T10:01:09Z","stop":"end_turn","usage":{"input":10,"output":2}}`,
	}, "\n") + "\n"
	rep := readACPStream(record)
	want := []Usage{{Model: "gpt-5.5", Input: 100, Output: 20, CacheRead: 900}, {Input: 10, Output: 2}}
	if len(rep.Usage) != 2 || rep.Usage[0] != want[0] || rep.Usage[1] != want[1] {
		t.Fatalf("a run's: got %+v, want %+v", rep.Usage, want)
	}
	got, _ := SessionSpend("codex", "gpt-5.5", lines(strings.Split(strings.TrimSpace(record), "\n")...), "")
	if len(got) != 1 || got[0].Usage != (Usage{Model: "gpt-5.5", Input: 110, Output: 22, CacheRead: 900}) {
		t.Fatalf("a crew's: got %+v", got)
	}
}
