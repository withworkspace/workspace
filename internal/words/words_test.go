package words

import "testing"

func TestMatch_WhatsTypedIsNeverTheQueryLanguage(t *testing.T) {
	for _, c := range []struct {
		typed, all, any string
		n, content      int
	}{
		{"token-cache ", `"token-cache"`, `"token-cache"`, 1, 1},
		{`demo#12 "per `, `"demo#12" """per"`, `"demo#12" OR """per"`, 2, 2},
		{"  ", "", "", 0, 0},
		{"-- !", "", "", 0, 0},
		{"how do we key per tenant ", `"how" "do" "we" "key" "per" "tenant"`, `"how" OR "do" OR "we" OR "key" OR "per" OR "tenant"`, 6, 3},
		{"what is it", `"what" "is" "it"`, `"what" OR "is" OR "it"`, 3, 1},
	} {
		if c.typed == "how do we key per tenant " {
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

func TestMatch_TheLastWordMayBeBegun(t *testing.T) {
	for _, c := range []struct{ typed, all, any string }{
		{"bin", `"bin" *`, `"bin" *`},                         // as it's typed: binary
		{"bin ", `"bin"`, `"bin"`},                            // followed by a space: finished
		{"one bin", `"one" "bin" *`, `"one" OR "bin" *`},      // only the last word
		{"token-cache", `"token-cache" *`, `"token-cache" *`}, // a phrase: its last word begun
		{`"per`, `"""per" *`, `"""per" *`},                    // still never the query language
		{"x", `"x"`, `"x"`},                                   // one letter starts too many
		{"what is", `"what" "is"`, `"what" OR "is"`},          // a small word isn't begun
		{"cache?", `"cache?" *`, `"cache?" *`},
	} {
		if m := Match(c.typed); m.All != c.all || m.Any != c.any {
			t.Errorf("%q: %s | %s, want %s | %s", c.typed, m.All, m.Any, c.all, c.any)
		}
	}
	if m := Match("key per ten"); m.ContentAny != `"key" OR "per" OR "ten" *` || m.Every() != `"key" "per" "ten" *` {
		t.Errorf("what's looked for keeps it begun: %+v", m)
	}
}
