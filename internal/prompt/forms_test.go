package prompt

import "testing"

func TestParseDraft(t *testing.T) {
	for _, c := range []struct{ reply, what, why string }{
		{"What: Key per tenant.\nWhy: Tokens are per tenant.", "Key per tenant.", "Tokens are per tenant."},
		{"**What:** Key per tenant.\n**Why:** Tokens are\nper tenant.\n\nAnything else.", "Key per tenant.", "Tokens are per tenant."},
		{"what: No Redis.", "No Redis.", ""},
		{"Nothing decided.", "", ""},
	} {
		what, why, ok := ParseDraft(c.reply)
		if what != c.what || why != c.why || ok != (c.what != "") {
			t.Errorf("ParseDraft(%q) = %q, %q, %v", c.reply, what, why, ok)
		}
	}
}

func TestSplitSummary(t *testing.T) {
	for _, c := range []struct{ reply, body, title, summary string }{
		{"About a day.\n\nTitle: Token cache\nSummary: What it'd take.", "About a day.", "Token cache", "What it'd take."},
		{"About a day.\n---\n**Title:** Token cache\n**Summary:** What it'd take.\n", "About a day.", "Token cache", "What it'd take."},
		{"About a day.\nSummary: What it'd take.", "About a day.", "", "What it'd take."},
		{"Just this.", "Just this.", "", ""},
		{"Summary: What it'd take.\nAnd more.", "Summary: What it'd take.\nAnd more.", "", ""},
	} {
		body, title, summary := SplitSummary(c.reply)
		if body != c.body || title != c.title || summary != c.summary {
			t.Errorf("SplitSummary(%q) = %q, %q, %q", c.reply, body, title, summary)
		}
	}
}
