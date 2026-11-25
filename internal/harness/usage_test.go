package harness

import (
	"strconv"
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
	got, last := SessionSpend("claude", lines(
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
		{Day: "2026-11-24", Usage: Usage{Model: "claude-opus-5-5", Input: 2, Output: 10, CacheWrite: 100, CacheRead: 1000}},
		{Day: "2026-11-25", Usage: Usage{Model: "claude-opus-5-5", Input: 4, Output: 21, CacheWrite: 1, CacheRead: 2001}},
		{Day: "2026-11-25", Usage: Usage{Model: "claude-haiku-5-5", Input: 1, Output: 5}},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if got, _ := SessionSpend("pi", lines(assistant("m1", "x", "2026-11-25", 1, 1, 1, 1)), ""); got != nil {
		t.Errorf("a harness whose sessions aren't read: %+v", got)
	}
}
