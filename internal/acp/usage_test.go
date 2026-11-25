package acp

import (
	"encoding/json"
	"testing"
)

// A prompt's response says what the turn spent: codex-acp's with the
// model in _meta, opencode's without; one that says nothing has none.
func TestPromptResponseUsage(t *testing.T) {
	for _, c := range []struct {
		name, body string
		want       *Usage
	}{
		{"codex-acp", `{"stopReason":"end_turn","usage":{"totalTokens":1020,"inputTokens":100,"outputTokens":20,"cachedReadTokens":900,"thoughtTokens":5},"_meta":{"quota":{"token_count":{},"model_usage":[{"model":"gpt-5.5","token_count":{}}]}}}`,
			&Usage{Model: "gpt-5.5", Input: 100, Output: 20, CacheRead: 900}},
		{"opencode", `{"stopReason":"end_turn","usage":{"totalTokens":30,"inputTokens":10,"outputTokens":20,"cachedWriteTokens":7}}`,
			&Usage{Input: 10, Output: 20, CacheWrite: 7}},
		{"cursor-agent", `{"stopReason":"end_turn"}`, nil},
	} {
		var r promptResponse
		if err := json.Unmarshal([]byte(c.body), &r); err != nil {
			t.Fatal(err)
		}
		got := r.usage()
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}
