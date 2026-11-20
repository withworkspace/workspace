package artifact

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// Body is what a commit that keeps a version of an artifact says of where
// it came from, for its body: a conversation (named, with its title and
// the proposal), a mission, or by hand; then the version's note.
func Body(v Version) string {
	var b bytes.Buffer
	title := ""
	if v.FromTitle != "" {
		title = fmt.Sprintf(", %q", v.FromTitle)
	}
	proposal := ""
	if v.Proposal != 0 {
		proposal = fmt.Sprintf(" (proposal %d)", v.Proposal)
	}
	switch {
	case missionTalk.MatchString(v.From):
		slug, id, _ := strings.Cut(v.From, "/")
		fmt.Fprintf(&b, "Proposed in conversation %s on mission %s%s%s, and added by %s.", id, slug, title, proposal, v.By)
	case conversation.MatchString(v.From):
		fmt.Fprintf(&b, "Proposed in conversation %s%s%s, and added by %s.", v.From, title, proposal, v.By)
	case v.From != "":
		fmt.Fprintf(&b, "Made for mission %s%s, by %s.", v.From, title, v.By)
	default:
		fmt.Fprintf(&b, "Added by hand, by %s.", v.By)
	}
	if v.Note != "" {
		b.WriteString("\n\n" + v.Note)
	}
	return b.String()
}

var (
	conversation = regexp.MustCompile(`^w[1-9][0-9]*$`)
	missionTalk  = regexp.MustCompile(`^[^/\s]+/t[1-9][0-9]*$`)
)

// IsConversation reports whether a version's from is a conversation: a
// workspace's (w9) or a mission's (glow/t3).
func IsConversation(from string) bool {
	return conversation.MatchString(from) || missionTalk.MatchString(from)
}
