package ref

import (
	"slices"
	"testing"
)

// The addresses in a text name what they're the place of: a plugin's
// thing, a pull request, a file's lines; and not what's someone else's.
func TestAddressed(t *testing.T) {
	k := &Known{Repos: map[string]Tree{"demo": nil}, Org: "acme", DefaultBranch: "main",
		Things: []Named{{Plugin: "jira", What: "issue", Prefixes: []string{"RAM-"}, Address: "https://acme.atlassian.net/browse/{ref}"}}}
	for _, c := range []struct {
		text string
		want []string
	}{
		{"See https://acme.atlassian.net/browse/RAM-12.", []string{"RAM-12"}},
		{"See [the issue](https://acme.atlassian.net/browse/RAM-12?focusedCommentId=4), and <https://acme.atlassian.net/browse/RAM-7>.", []string{"RAM-12", "RAM-7"}},
		{"It's https://github.com/acme/demo/pull/12, at https://github.com/acme/demo/blob/main/main.go#L3-L5!", []string{"demo#12", "demo/main.go:3-5"}},
		{"Not ours: https://github.com/other/demo/pull/12, https://other.example/browse/RAM-12, https://acme.atlassian.net/browse/UTF-8.", nil},
		// An issue is named as a pull request is; a commit by its sha, as written.
		{"Filed as https://github.com/acme/demo/issues/7, fixed by https://github.com/acme/demo/commit/4f2a9c1e8b7d6c5a4f3e2d1c0b9a8f7e6d5c4b3a and [this](https://github.com/acme/demo/commit/4f2a9c1).", []string{"demo#7", "demo@4f2a9c1e8b7d6c5a4f3e2d1c0b9a8f7e6d5c4b3a", "demo@4f2a9c1"}},
		{"Not ours, or not a commit: https://github.com/other/demo/issues/7, https://github.com/other/demo/commit/4f2a9c1, https://github.com/acme/demo/commit/main.", nil},
		{"RAM-12 by its name alone is Find's to find.", nil},
	} {
		var got []string
		for _, r := range k.Addressed(c.text) {
			got = append(got, r.Name())
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("Addressed(%q) = %q, want %q", c.text, got, c.want)
		}
	}
	// The issue's name is a pull request's, but what it names is the issue.
	var kinds []Kind
	for _, r := range k.Addressed("https://github.com/acme/demo/issues/7 https://github.com/acme/demo/pull/7 https://github.com/acme/demo/commit/4f2a9c1") {
		kinds = append(kinds, r.Kind)
	}
	if want := []Kind{Issue, PR, Commit}; !slices.Equal(kinds, want) {
		t.Errorf("Addressed kinds = %v, want %v", kinds, want)
	}
	if got := (*Known)(nil).Addressed("https://acme.atlassian.net/browse/RAM-12"); got != nil {
		t.Errorf("with nothing known: %v", got)
	}
}
