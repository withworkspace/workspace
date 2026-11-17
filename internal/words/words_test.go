package words

import "testing"

func TestMatch_WhatsTypedIsNeverTheQueryLanguage(t *testing.T) {
	for _, c := range []struct {
		typed, all, any string
		n, content      int
	}{
		{"token-cache", `"token-cache"`, `"token-cache"`, 1, 1},
		{`demo#12 "per`, `"demo#12" """per"`, `"demo#12" OR """per"`, 2, 2},
		{"  ", "", "", 0, 0},
		{"-- !", "", "", 0, 0},
		{"how do we key per tenant", `"how" "do" "we" "key" "per" "tenant"`, `"how" OR "do" OR "we" OR "key" OR "per" OR "tenant"`, 6, 3},
		{"what is it", `"what" "is" "it"`, `"what" OR "is" OR "it"`, 3, 1},
	} {
		if c.typed == "how do we key per tenant" {
			if m := Match(c.typed); m.ContentAny != `"key" OR "per" OR "tenant"` || len(m.Smalls) != 3 {
				t.Errorf("what's looked for: %+v", m)
			}
		}
		if c.typed == "what is it" {
			if m := Match(c.typed); m.ContentAny != "" {
				t.Errorf("all small words: look for them all: %+v", m)
			}
		}
		m := Match(c.typed)
		if m.All != c.all || m.Any != c.any || m.N != c.n || m.Content != c.content {
			t.Errorf("%q: %+v, want %s | %s | %d %d", c.typed, m, c.all, c.any, c.n, c.content)
		}
	}
}
