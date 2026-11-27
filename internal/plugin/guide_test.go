package plugin

import (
	"strings"
	"testing"
)

func TestGuideSummary(t *testing.T) {
	long := strings.Repeat("word ", 80) // 400 characters, no full stop
	for _, c := range []struct{ guide, want string }{
		{"Sketches are named sketch-12.", "Sketches are named sketch-12."},
		// The first paragraph alone, its lines joined.
		{"Jira issues,\nsynced.\n\nThe rest.", "Jira issues, synced."},
		// A paragraph that leads into a list ends in a full stop instead.
		{"`ws plugin-dev` is for writing ws plugins:\n\n- one", "`ws plugin-dev` is for writing ws plugins."},
		// Whole sentences up to about SummaryMax, and the first whatever its length.
		{"One. " + strings.Repeat("x", 300) + ". Three.", "One."},
		{strings.Repeat("y", 350) + ". Two.", strings.Repeat("y", 350) + "."},
		// No sentence ends: cut at a word.
		{long, strings.TrimSpace(long[:SummaryMax-1]) + "…"},
	} {
		if got := GuideSummary(c.guide); got != c.want {
			t.Errorf("GuideSummary(%.40q) = %q, want %q", c.guide, got, c.want)
		}
	}
	if GuideSummaryCut("Short.\n\nMore.") || !GuideSummaryCut("One. "+strings.Repeat("x", 300)+".") {
		t.Error("GuideSummaryCut")
	}
}
