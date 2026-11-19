package ref

import (
	"regexp"
	"strings"
)

// written finds the addresses written in a run of text: a link's, or one
// written out, up to what can't be part of one in prose or in markdown.
var written = regexp.MustCompile("https?://[^\\s<>\"'`)\\]]+")

// Addressed is what the addresses in a run of text name, in order, each
// as ParseAddress reads it: a plugin's thing where its plugin said one is
// (https://acme.atlassian.net/browse/RAM-12), a pull request or a file of
// the workspace's on GitHub. Find leaves these alone, since a name inside
// an address isn't one, and what's drawn reads a link's address by itself
// (At): this is for whoever wants what a text names, however it's named.
func (k *Known) Addressed(text string) []Ref {
	if k == nil || !strings.Contains(text, "://") {
		return nil
	}
	var out []Ref
	for _, addr := range written.FindAllString(text, -1) {
		if r, ok := ParseAddress(strings.TrimRight(addr, ".,;:!?"), k); ok {
			out = append(out, r)
		}
	}
	return out
}
