// Package words is how ws's full-text search reads what's written and
// what's typed, wherever it's kept: what's typed made a query that can't
// be read as FTS5's own language, markdown made sections (the places a
// search leads to), and how a title or a heading that holds what's asked
// adds to a place's score. The documents' search (internal/space) and the
// records' (internal/store) rank by it, so they rank alike. It imports
// nothing of ws's, and speaks no SQL.
package words

import (
	"strings"
	"unicode"
)

// Query is what's typed, as full-text queries: every word (All), any of
// them (Any), and the same of the words that matter, those that aren't the
// small words every sentence has (ContentAll, ContentAny: empty when it's
// all small words); how many words there are (N) and how many matter
// (Content, at least 1); and the small ones, as typed. Each word is
// quoted, so what's typed is never read as the query language's own:
// token-cache isn't a column, and demo#12 and a stray " aren't errors.
//
// What's looked for is the words that matter: "the" would find every row,
// and mark itself in every snippet. The small words still count for how
// nearly a heading says what was typed ("What a decision is").
type Query struct {
	All, Any               string
	ContentAll, ContentAny string
	N, Content             int
	Smalls                 []string
}

// Match is what's typed as a Query; its All is empty when nothing typed
// is a word.
func Match(typed string) Query {
	var terms, content, smalls []string
	for _, w := range strings.Fields(typed) {
		if !strings.ContainsFunc(w, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) {
			continue // nothing in it is a word
		}
		t := `"` + strings.ReplaceAll(w, `"`, `""`) + `"`
		terms = append(terms, t)
		if bare := strings.ToLower(strings.Trim(w, `"'?!.,:;()`)); small[bare] {
			smalls = append(smalls, bare)
		} else {
			content = append(content, t)
		}
	}
	if len(terms) == 0 {
		return Query{}
	}
	q := Query{All: strings.Join(terms, " "), Any: strings.Join(terms, " OR "), N: len(terms), Content: max(1, len(content)), Smalls: smalls}
	if len(content) > 0 {
		q.ContentAll, q.ContentAny = strings.Join(content, " "), strings.Join(content, " OR ")
	}
	return q
}

// Looked is what's looked for: any of the words that matter, or of every
// word when none do.
func (q Query) Looked() string {
	if q.ContentAny != "" {
		return q.ContentAny
	}
	return q.Any
}

// Every is what a row holds when it has every word that matters.
func (q Query) Every() string {
	if q.ContentAll != "" {
		return q.ContentAll
	}
	return q.All
}

// Marks are what a full-text query's highlight() and snippet() are asked
// to put either side of a word found: char(2) and char(3), which nothing
// written has.
const (
	MarkOpen  = '\x02'
	MarkClose = '\x03'
)

// Held is how much of what's asked (n words) a highlighted run holds,
// from 0 to 1: its marked words over n, the small ones left out when only
// those that matter are counted.
func Held(highlighted string, n int, mattering bool) float64 {
	if n == 0 {
		return 0
	}
	marked := 0
	for {
		i := strings.IndexByte(highlighted, MarkOpen)
		if i < 0 {
			break
		}
		rest := highlighted[i+1:]
		j := strings.IndexByte(rest, MarkClose)
		if j < 0 {
			j = len(rest)
		}
		for _, w := range strings.Fields(rest[:j]) {
			if !mattering || !small[strings.ToLower(strings.Trim(w, `"'?!.,:;()`))] {
				marked++
			}
		}
		if j >= len(rest) {
			break
		}
		highlighted = rest[j+1:]
	}
	return min(1, float64(marked)/float64(n))
}

// HeldAll is how much of every word as typed a highlighted run holds
// (plain is the run without its marks): the words that matter, as marked,
// and the small ones, as they're said in it.
func HeldAll(highlighted, plain string, q Query) float64 {
	if q.N == 0 {
		return 0
	}
	marked := Held(highlighted, q.N, false) * float64(q.N)
	if q.ContentAny != "" { // the small words weren't looked for: count them
		said := map[string]bool{}
		for _, w := range strings.FieldsFunc(strings.ToLower(plain), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '\'' }) {
			said[w] = true
		}
		for _, w := range q.Smalls {
			if said[w] {
				marked++
			}
		}
	}
	return min(1, marked/float64(q.N))
}

// small are words a question asked in a sentence has whatever it's about:
// they're searched for (BM25 makes little of them), but not counted in
// how much of what's asked a title or a heading holds.
var small = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a an the is are was were be been do does did done how what why when where who whom which
		we you i it its of to in on for by with from at as and or not this that these those can could should would will shall
		our your my me us they them their there here about into than then so if but no yes any all some has have had`) {
		small[w] = true
	}
}

// Part is a run of a snippet: marked when it's a word that was searched for.
type Part struct {
	Text string `json:"text"`
	Mark bool   `json:"mark,omitempty"`
}

// Parts is a snippet as runs, marked where SQLite marked them (MarkOpen to
// MarkClose).
func Parts(s string) []Part {
	var out []Part
	for s != "" {
		i := strings.IndexByte(s, MarkOpen)
		if i < 0 {
			out = append(out, Part{Text: s})
			break
		}
		if i > 0 {
			out = append(out, Part{Text: s[:i]})
		}
		s = s[i+1:]
		j := strings.IndexByte(s, MarkClose)
		if j < 0 {
			j = len(s)
		}
		out = append(out, Part{Text: s[:j], Mark: true})
		if j < len(s) {
			s = s[j+1:]
		} else {
			s = ""
		}
	}
	return out
}

// Text is a snippet as plain text, its marks as the given strings either
// side.
func Text(ps []Part, open, close string) string {
	var b strings.Builder
	for _, p := range ps {
		t := strings.Join(strings.Fields(p.Text), " ")
		if t == "" && p.Text != "" { // a space between words found
			b.WriteByte(' ')
			continue
		}
		if strings.HasPrefix(p.Text, " ") || strings.HasPrefix(p.Text, "\n") {
			t = " " + t
		}
		if strings.HasSuffix(p.Text, " ") || strings.HasSuffix(p.Text, "\n") {
			t += " "
		}
		if p.Mark {
			t = open + t + close
		}
		b.WriteString(t)
	}
	return strings.TrimSpace(b.String())
}
