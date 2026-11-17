package ops

import "testing"

// A passage is found where it's written: as it is, or by its words with
// the markdown's marks between them; a long one by its first words and
// its last.
func TestQuoteLines(t *testing.T) {
	text := []byte("# Tokens\n\n## Caching\n\nWe cache `tokens`\n*per user*, for an\nhour. See [the ADR](adr/1.md) for why.\n\nNothing else.\n")
	for _, c := range []struct {
		exact     string
		line, end int
		ok        bool
	}{
		{"Nothing else.", 9, 9, true},
		{"We cache tokens per user", 5, 6, true},
		{"We cache tokens per user, for an hour. See the ADR for why.", 5, 7, true},
		{"cache tokens per user, for an hour. See the ADR for", 5, 7, true},
		{"per tenant", 0, 0, false},
		{"  ", 0, 0, false},
	} {
		line, end, ok := QuoteLines(text, c.exact)
		if line != c.line || end != c.end || ok != c.ok {
			t.Errorf("%q: %d–%d %v, want %d–%d %v", c.exact, line, end, ok, c.line, c.end, c.ok)
		}
	}
}
