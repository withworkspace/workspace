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
	if got := (*Known)(nil).Addressed("https://acme.atlassian.net/browse/RAM-12"); got != nil {
		t.Errorf("with nothing known: %v", got)
	}
}
