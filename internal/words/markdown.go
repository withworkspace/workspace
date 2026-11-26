package words

import (
	"regexp"
	"strings"
	"unicode"
)

// Section is a heading and what's under it, up to the next heading: the
// first is what comes before any heading but the title's. A search leads
// to one.
type Section struct {
	Anchor  string // as the reader names the heading (GitHub's way); "" for the top
	Heading string
	Level   int
	Body    string // its words: markdown's marks and what links point at left out
}

var (
	atx      = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?[ \t]*#*[ \t]*$`)
	mdLink   = regexp.MustCompile(`!?\[([^\]]*)\]\(\s*<?([^)\s>]+)>?(?:\s+["'][^"']*["'])?\s*\)`)
	comment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	tag      = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	markRepl = strings.NewReplacer("**", "", "__", "", "`", "")
	emphasis = strings.NewReplacer("**", "", "__", "")
)

// Sections is markdown as the places a search leads to: a section for
// each heading (front matter left out, a fence's lines words and not
// headings), the title's own heading (title, as Title reads it, or what
// its caller calls it) not one: the top is under it. A text with nothing
// but a title has one section, so it's found by its title.
func Sections(text []byte, title string) []Section {
	s := strings.ReplaceAll(string(text), "\r\n", "\n")
	lines := strings.Split(s, "\n")
	start := 0
	if len(lines) > 0 && lines[0] == "---" { // front matter: not words to search
		for i := 1; i < len(lines); i++ {
			if lines[i] == "---" || lines[i] == "..." {
				start = i + 1
				break
			}
		}
	}
	var out []Section
	slug := Slugger()
	cur := Section{}
	var body []string
	flush := func() {
		cur.Body = Clean(strings.Join(body, "\n"))
		if cur.Heading != "" || strings.TrimSpace(cur.Body) != "" {
			out = append(out, cur)
		}
		body = nil
	}
	fenced := ""
	titled := false
	for i := start; i < len(lines); i++ {
		line := lines[i]
		t := strings.TrimSpace(line)
		if fenced != "" {
			if strings.HasPrefix(t, fenced) {
				fenced = ""
			}
			body = append(body, line)
			continue
		}
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fenced = t[:3]
			body = append(body, line)
			continue
		}
		if m := atx.FindStringSubmatch(line); m != nil {
			heading := strings.TrimSpace(markRepl.Replace(PlainLinks(m[2])))
			if !titled && heading == title {
				// The title's heading: the top section is under it.
				titled = true
				continue
			}
			flush()
			cur = Section{Anchor: slug(heading), Heading: heading, Level: len(m[1])}
			continue
		}
		body = append(body, line)
	}
	flush()
	if len(out) == 0 {
		// A title and nothing else is still found by its title.
		out = append(out, Section{})
	}
	return out
}

// Clean is markdown as words to search: what links point at, comments,
// tags and bold's marks left out.
func Clean(md string) string {
	md = comment.ReplaceAllString(md, " ")
	md = PlainLinks(md)
	md = tag.ReplaceAllString(md, " ")
	md = emphasis.Replace(md) // a snippet of it reads as words
	return strings.TrimSpace(md)
}

// PlainLinks is markdown with each link as its text alone.
func PlainLinks(md string) string {
	return mdLink.ReplaceAllString(md, "$1")
}

// Slugger names headings as the web UI's reader does (web/src/markdown/
// links.ts), which is GitHub's way: lower case, letters, digits, hyphens
// and underscores kept, spaces to hyphens, and a number after one that's
// been used (-1, -2).
func Slugger() func(string) string {
	used := map[string]bool{}
	return func(text string) string {
		var b strings.Builder
		for _, r := range strings.ToLower(strings.TrimSpace(text)) {
			switch {
			case r == ' ':
				b.WriteByte('-')
			case r == '-' || unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) || unicode.Is(unicode.Pc, r):
				b.WriteRune(r)
			}
		}
		base := b.String()
		slug := base
		for n := 1; used[slug]; n++ {
			slug = base + "-" + itoa(n)
		}
		used[slug] = true
		return slug
	}
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// Title is what a markdown document calls itself: its front matter's
// title, or its first heading (a #tag isn't one), or "".
func Title(text []byte) string {
	s := strings.ReplaceAll(string(text), "\r\n", "\n")
	if rest, ok := strings.CutPrefix(s, "---\n"); ok {
		if fm, body, ok := strings.Cut(rest, "\n---"); ok {
			for line := range strings.SplitSeq(fm, "\n") {
				if v, ok := strings.CutPrefix(line, "title:"); ok {
					if t := strings.Trim(strings.TrimSpace(v), `"'`); t != "" {
						return t
					}
				}
			}
			s = body
		}
	}
	fenced := false
	for line := range strings.SplitSeq(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fenced = !fenced
		}
		if fenced || !strings.HasPrefix(t, "#") {
			continue
		}
		h := strings.TrimLeft(t, "#")
		if h != "" && h[0] != ' ' && h[0] != '\t' {
			continue // #tag, not a heading
		}
		h = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(h), "#"))
		if h = strings.TrimSpace(markRepl.Replace(h)); h != "" {
			return h
		}
	}
	return ""
}
