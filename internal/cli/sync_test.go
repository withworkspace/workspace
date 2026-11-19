package cli

import "testing"

func TestPluralize(t *testing.T) {
	tests := []struct {
		n        int
		singular string
		plural   string
		want     string
	}{
		{0, "repo", "repos", "repos"},
		{1, "repo", "repos", "repo"},
		{2, "repo", "repos", "repos"},
		{10, "item", "items", "items"},
	}

	for _, tt := range tests {
		if got := pluralize(tt.n, tt.singular, tt.plural); got != tt.want {
			t.Errorf("pluralize(%d, %q, %q) = %q, want %q", tt.n, tt.singular, tt.plural, got, tt.want)
		}
	}
}

func TestParseNameStatus(t *testing.T) {
	got := parseNameStatus([]string{
		"M\tPROCESS.md",
		"A\tdocs/new.md",
		"D\told.txt",
		"R087\tnotes.md\tdocs/notes.md",
		"T\tlink",
		"",
		"garbage",
	})
	want := []fileChange{
		{Path: "PROCESS.md", Verb: "updated"},
		{Path: "docs/new.md", Verb: "added"},
		{Path: "old.txt", Verb: "removed"},
		{Path: "docs/notes.md", From: "notes.md", Verb: "renamed"},
		{Path: "link", Verb: "updated"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d changes %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("change %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
