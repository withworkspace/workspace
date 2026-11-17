package slug

import "testing"

func TestMake(t *testing.T) {
	cases := []struct {
		in       string
		max      int
		fallback string
		want     string
	}{
		{"Fix token refresh race", 40, "mission", "fix-token-refresh-race"},
		{"  --Cache: per tenant!!  ", 0, "x", "cache-per-tenant"},
		{"API v2 — rate limits (draft)", 0, "x", "api-v2-rate-limits-draft"},
		{"Évite les caches", 0, "x", "vite-les-caches"},
		{"one two three four", 12, "x", "one-two"}, // cut at a word
		{"onetwothreefour", 7, "x", "onetwot"},     // no word to cut at
		{"one two", 3, "x", "one"},                 // a hyphen right at the cut
		{"!!!", 0, "untitled", "untitled"},
		{"", 10, "someone", "someone"},
	}
	for _, c := range cases {
		if got := Make(c.in, c.max, c.fallback); got != c.want {
			t.Errorf("Make(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}
