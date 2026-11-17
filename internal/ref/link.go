package ref

import (
	"regexp"
	"strings"
)

// Linking: in a document, a ref is a markdown link, its words its name and
// its address where it is. Agents write names; ws links them as what they
// write is put into a document.

var (
	fence   = regexp.MustCompile("^\\s{0,3}(```|~~~)")
	heading = regexp.MustCompile(`^\s{0,3}#{1,6}(\s|$)`)
	// What a line's names aren't looked for in: a link or an image whole
	// (its words and address), an autolink, a bare address, and code.
	guarded = regexp.MustCompile("!?\\[[^\\]]*\\]\\([^)]*\\)|<[a-z]+:[^>\\s]*>|https?://[^\\s)>\\]]+|`+[^`]*`+")
)

// Link is markdown with the names in it made links: [demo/main.go:5](its
// address). A name in backticks, alone, is linked with them. Not in code
// blocks, headings, HTML comments (a page's cells are marked with them),
// or what's a link already. address says where a found ref is, and
// whether to link it.
func (k *Known) Link(md string, address func(f Found, said string) (string, bool)) string {
	lines := strings.SplitAfter(md, "\n")
	inFence, inComment := false, false
	for i, line := range lines {
		switch {
		case fence.MatchString(line):
			inFence = !inFence
			continue
		case inFence || heading.MatchString(line):
			continue
		}
		// A comment's lines stay as they are.
		if inComment {
			inComment = !strings.Contains(line, "-->")
			continue
		}
		if at := strings.Index(line, "<!--"); at >= 0 {
			inComment = !strings.Contains(line[at:], "-->")
			continue
		}
		lines[i] = k.linkLine(line, address)
	}
	return strings.Join(lines, "")
}

func (k *Known) linkLine(line string, address func(Found, string) (string, bool)) string {
	var b strings.Builder
	last := 0
	words := func(s string) {
		at := 0
		for _, f := range k.Find(s) {
			said := s[f.From:f.To]
			addr, ok := address(f, said)
			if !ok {
				continue
			}
			b.WriteString(s[at:f.From])
			b.WriteString("[" + said + "](" + addr + ")")
			at = f.To
		}
		b.WriteString(s[at:])
	}
	for _, m := range guarded.FindAllStringIndex(line, -1) {
		words(line[last:m[0]])
		g := line[m[0]:m[1]]
		// A name in backticks, alone: linked with them.
		if strings.HasPrefix(g, "`") {
			inner := strings.Trim(g, "`")
			if f := k.Find(inner); len(f) == 1 && f[0].From == 0 && f[0].To == len(inner) {
				if addr, ok := address(f[0], inner); ok {
					g = "[" + g + "](" + addr + ")"
				}
			}
		}
		b.WriteString(g)
		last = m[1]
	}
	words(line[last:])
	return b.String()
}
