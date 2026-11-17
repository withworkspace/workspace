package conversation

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A review's guide: what the change does, where that differs from what it
// says it does, and the order to read its files in, in groups. It's what
// the guide's pass says (a conversation of kind pass, its goal "guide"),
// kept on its turn. Nothing's hidden by it: a file it doesn't place is
// still in the review.

// ReviewGuide is a guide to a change.
type ReviewGuide struct {
	// Does is what the change does, from the change itself.
	Does string `json:"does"`
	// Differs are where that differs from its description: what the
	// description says that the change doesn't do, and what the change
	// does that the description doesn't say.
	Differs []string `json:"differs,omitempty"`
	// Groups are its files in the order to read them in.
	Groups []GuideGroup `json:"groups"`
}

// GuideGroup is files to read together: what they are, why they're read
// here, and whether they needn't be read closely (Skim: mechanical).
type GuideGroup struct {
	Name  string      `json:"name"`
	Why   string      `json:"why,omitempty"`
	Skim  bool        `json:"skim,omitempty"`
	Files []GuideFile `json:"files"`
}

// GuideFile is a file in a group, and what the change does to it, in a
// line.
type GuideFile struct {
	Path string `json:"path"`
	Note string `json:"note,omitempty"`
}

// GuideGiven is what the guide's agent was given, kept while it writes,
// for the guide's view to say: the files the change touches (each one it
// may place), and those it wasn't shown, and why (a lockfile, generated
// code, binary, too large, no room).
type GuideGiven struct {
	Files []string    `json:"files"`
	Left  []GuideLeft `json:"left,omitempty"`
}

// GuideLeft is a file the guide's agent wasn't shown, and why.
type GuideLeft struct {
	Path string `json:"path"`
	Why  string `json:"why"`
}

var (
	guideHead = regexp.MustCompile(`(?i)^##\s*(read|skim|group|differs)\b\s*:?\s*(.*)$`)
	guideFile = regexp.MustCompile("^[-*]\\s+`?([^`\\s][^`]*?)`?\\s*(?::\\s+(.*)|\\s+[-–—]\\s+(.*))?$")
)

// ParseGuideSoFar reads a guide's reply as far as it's been written, as
// ParseGuide does, less the line it ends on if that's a heading still
// being written ("## rea"), which would read as words of what the change
// does. The line a file's being written on is left to in: a path not yet
// whole isn't one of the change's.
func ParseGuideSoFar(partial string, in func(path string) bool) *ReviewGuide {
	i := strings.LastIndex(partial, "\n") // -1: it's all one line
	if strings.HasPrefix(strings.TrimSpace(partial[i+1:]), "#") {
		partial = partial[:i+1]
	}
	return ParseGuide(partial, in)
}

// ParseGuide reads a guide's pass's reply:
//
//	Moves the greeting into a package, and colours it.
//
//	## differs
//	- The description doesn't say the output is now green.
//
//	## read: The greeting itself
//	Start here: it's what the rest follows from.
//	- internal/render/render.go: the greeting, moved, now through colour
//	- internal/render/render_test.go: its test
//
//	## skim: Moves
//	Nothing in these changes but where they are.
//	- docs/README.md: renamed
//
// in is whether a path is one of the change's files: a file that isn't,
// and one placed twice, is left out. A group left with no files is too.
func ParseGuide(reply string, in func(path string) bool) *ReviewGuide {
	g := &ReviewGuide{Groups: []GuideGroup{}}
	var does []string
	var cur *GuideGroup
	differs := false
	placed := map[string]bool{}
	flush := func() {
		if cur != nil && len(cur.Files) > 0 {
			cur.Why = strings.TrimSpace(cur.Why)
			g.Groups = append(g.Groups, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(reply, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		if m := guideHead.FindStringSubmatch(t); m != nil {
			flush()
			kind := strings.ToLower(m[1])
			differs = kind == "differs"
			if !differs {
				name := strings.Trim(strings.TrimSpace(m[2]), "*_`")
				if name == "" {
					name = map[bool]string{true: "To skim", false: "To read"}[kind == "skim"]
				}
				// It's a heading: it starts with a capital.
				if r, size := utf8.DecodeRuneInString(name); unicode.IsLower(r) {
					name = string(unicode.ToUpper(r)) + name[size:]
				}
				cur = &GuideGroup{Name: name, Skim: kind == "skim", Files: []GuideFile{}}
			}
			continue
		}
		switch {
		case strings.HasPrefix(t, "## "): // a heading of some other kind: what's under it isn't the guide's
			flush()
			differs = false
		case differs:
			if d := strings.TrimSpace(strings.TrimLeft(t, "-* ")); d != "" && !strings.EqualFold(strings.TrimRight(d, "."), "nothing") && !strings.EqualFold(strings.TrimRight(d, "."), "none") {
				g.Differs = append(g.Differs, d)
			}
		case cur != nil:
			if m := guideFile.FindStringSubmatch(t); m != nil {
				path := strings.TrimSpace(m[1])
				if in(path) && !placed[path] {
					placed[path] = true
					cur.Files = append(cur.Files, GuideFile{Path: path, Note: strings.TrimSpace(m[2] + m[3])})
				}
			} else if t != "" && len(cur.Files) == 0 {
				cur.Why += " " + t
			}
		case len(g.Groups) == 0:
			does = append(does, line)
		}
	}
	flush()
	g.Does = strings.TrimSpace(strings.Join(does, "\n"))
	return g
}
