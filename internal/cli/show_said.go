package cli

import (
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	wsref "github.com/withworkspace/workspace/internal/ref"
)

// What's been said of a thing beyond the view itself, from the graph: what
// an agent most needs before it changes a file is what's already been said
// of it (a comment still open on the lines it's about to touch, a decision
// about the file, a thread that asked about it), wherever that was. Each is
// a ref to follow, with the mission it's read in.

// saidHow is how a view groups what points at it, in the order an agent
// wants them: what's been asked of the thing itself first.
var saidHow = []struct{ how, title string }{
	{"on", "Comments on it"},
	{"about", "About it"},
	{"of", "Reviews of it"},
	{"in", "In it"}, // a decision made in a thread, a cell of a page, a plugin's thing that's part of another
	{"from", "Came out of it"},
	{"mentions", "Named in"},
	{"by", "By it"},
}

// showSaid is how many of each group a view lists, without --full.
const showSaid = 8

// saidSection is what the view says of what points at it: grouped, those
// that still stand first. title says where it was looked for. With line,
// only what's on those lines (or on none) is listed, and what's elsewhere
// in the file is counted.
type saidSection struct {
	title     string
	said      []ops.Said
	line, end int
	elsewhere string // the ref to show for the rest of the file
}

func (v *view) addSaid(s saidSection) {
	if len(s.said) > 0 {
		v.saids = append(v.saids, s)
	}
}

func (s saidSection) render(b *strings.Builder, full bool) {
	var on []ops.Said
	other := 0
	for _, x := range s.said {
		if ops.OnLines(x.Lines, s.line, s.end) {
			on = append(on, x)
		} else {
			other++
		}
	}
	if len(on) == 0 && other == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s (lines as they were when it was said; ws show <ref> --mission <mission> for a mission's, ws show <ref> for the workspace's, ws show <pull request> for its reviews):\n", s.title)
	if s.line > 0 {
		at := fmt.Sprint(s.line)
		if s.end > s.line {
			at += fmt.Sprintf("-%d", s.end)
		}
		fmt.Fprintf(b, "  On lines %s, or the whole of it:\n", at)
	}
	for _, g := range saidHow {
		var group []ops.Said
		for _, x := range on {
			if x.How == g.how {
				group = append(group, x)
			}
		}
		if len(group) == 0 {
			continue
		}
		standing := 0
		for _, x := range group {
			if x.Standing() {
				standing++
			}
		}
		fmt.Fprintf(b, "  %s (%d, %d still standing):\n", g.title, len(group), standing)
		shown := group
		if !full && len(shown) > showSaid {
			shown = shown[:showSaid]
		}
		width := 0
		for _, x := range shown {
			width = max(width, len(x.Ref))
		}
		for _, x := range shown {
			fmt.Fprintf(b, "    %-*s  %s\n", width, x.Ref, saidLine(x))
		}
		if cut := len(group) - len(shown); cut > 0 {
			fmt.Fprintf(b, "    … %d more: --full shows them\n", cut)
		}
	}
	if other > 0 {
		fmt.Fprintf(b, "  … %d more elsewhere in it: ws show %s\n", other, s.elsewhere)
	}
}

// saidLine is one thing that points at the view's: what it is, where it's
// read, how it stands, its lines, and its title.
func saidLine(x ops.Said) string {
	where := "the workspace's"
	switch {
	case x.Scope == "":
	case isPR(x.Scope):
		where = "pull request " + x.Scope
	default:
		where = "mission " + x.Scope
	}
	kind := saidKind(x.Kind)
	if x.Kind == "" && isFileKey(x.Ref) {
		kind = "a document" // a file has no thing of its own: one a talk proposed, added
	}
	parts := []string{kind, where}
	if x.State != "" {
		parts = append(parts, x.State)
	}
	if x.Lines != "" {
		parts = append(parts, "at "+x.Lines)
	}
	line := strings.Join(parts, ", ")
	if x.Title != "" {
		line += ": " + clipString(strings.Join(strings.Fields(x.Title), " "), 120)
	}
	return line
}

// isFileKey is whether a graph key names a file: by its name
// (records/adr/0001.md, as KeysOf keys one), as a document's ref is stored
// (records:adr/0001.md), or a capsule's (records/x:adr/0001.md). Not an
// address or a mission's file, which Parse would read as a file too.
func isFileKey(s string) bool {
	if strings.Contains(s, "://") || strings.HasPrefix(s, "mission:") {
		return false
	}
	r, ok := wsref.Parse(s)
	return ok && r.Kind == wsref.File
}

func isPR(s string) bool {
	_, _, ok := wsref.ParsePR(s)
	return ok
}

// saidKind is a thing's kind in words.
func saidKind(kind string) string {
	switch kind {
	case "":
		return "something"
	case "thread-ask", "thread-change", "thread-note":
		return "a thread (" + strings.TrimPrefix(kind, "thread-") + ")"
	case "pr":
		return "a talk about a pull request"
	case "doc":
		return "a talk about a document" // an ask, kept
	case "review-pass":
		return "a pass over a review"
	case "pass", "cell", "talk", "review", "comment", "decision", "mission", "briefing", "outline", "handoff", "document":
		return "a " + kind
	case "brief":
		return "a mission's brief"
	case "file":
		return "a file of a mission's"
	case "ask":
		return "a question in a page"
	case plugin.ThingKind:
		return "a plugin's thing"
	}
	return kind
}
