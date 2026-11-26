// Package find is the find window's side of ws (the web UI's ⌘K, and what
// it asks the server): an index of what the workspace has that a page
// doesn't already hold (every repo's files, what each capsule has
// changed, the open pull requests), kept in memory, and looked through as
// you type.
package find

import (
	"strings"
	"unicode/utf8"
)

// Matching is fuzzy, as a finder wants it: the letters of what's typed, in
// order, anywhere in the text; better when they sit together, start words
// or path parts, and fall in a path's last part (its file name). Space
// separates terms that must all match. It's the web UI's matcher
// (web/src/ui/fuzzy.ts) in Go, and both are tested against
// testdata/fuzzy.json, so the page and the server rank alike.
//
// Case is folded for ASCII only: paths are, nearly always.

// Match is how well a text matched, and where.
type Match struct {
	Score float64
	// At are the bytes of the text that matched, in the order the query's
	// letters came.
	At []int
}

// Query is what's typed, ready to match many texts against.
type Query struct {
	terms []string
	mask  uint64
}

// NewQuery reads what's typed: its terms, lowercased.
func NewQuery(q string) Query {
	out := Query{terms: strings.Fields(strings.ToLower(q))}
	for _, t := range out.terms {
		out.mask |= Mask(t)
	}
	return out
}

// Empty is a query with nothing in it: everything matches, equally.
func (q Query) Empty() bool { return len(q.terms) == 0 }

// Mask is which letters and digits a text has, as bits: a text can only
// match a query whose mask it covers, which is one comparison to tell.
func Mask(s string) uint64 {
	var m uint64
	for i := 0; i < len(s); i++ {
		m |= maskOf(s[i])
	}
	return m
}

func maskOf(c byte) uint64 {
	switch {
	case c >= 'a' && c <= 'z':
		return 1 << (c - 'a')
	case c >= 'A' && c <= 'Z':
		return 1 << (c - 'A')
	case c >= '0' && c <= '9':
		return 1 << (26 + c - '0')
	case c == '/':
		return 1 << 36
	case c == '.':
		return 1 << 37
	case c == '_':
		return 1 << 38
	case c == '-':
		return 1 << 39
	}
	return 1 << 40 // anything else
}

// Covers is whether a text with this mask could match the query.
func (q Query) Covers(mask uint64) bool { return mask&q.mask == q.mask }

func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// text is what's matched against: a string, or bytes put together for
// the purpose (a repo's name and a path), which needn't become a string
// unless they match.
type text interface{ ~string | ~[]byte }

func lastSlash[T text](t T) int {
	for i := len(t) - 1; i >= 0; i-- {
		if t[i] == '/' {
			return i
		}
	}
	return -1
}

func boundary[T text](text T, i int) bool {
	if i == 0 {
		return true
	}
	p := text[i-1]
	if p == '/' || p == '.' || p == '_' || p == '-' || p == ' ' {
		return true
	}
	return p >= 'a' && p <= 'z' && text[i] >= 'A' && text[i] <= 'Z'
}

func indexFrom(lower []byte, c byte, from int) int {
	for i := from; i < len(lower); i++ {
		if lower[i] == c {
			return i
		}
	}
	return -1
}

// Scratch is room a caller keeps between matches, so matching many texts
// allocates only for those that match.
type Scratch struct {
	lower []byte
	at    []int
	best  []int
	out   []int
	// Text is the caller's, to put a text together in.
	Text []byte
}

// matchTerm finds term in text, trying a few starts and keeping the best.
func matchTerm[T text](text T, lower []byte, term string, s *Scratch) (float64, bool) {
	base := lastSlash(text) + 1
	found := false
	var bestScore float64
	start := indexFrom(lower, term[0], 0)
	for tries := 0; start >= 0 && tries < 12; tries, start = tries+1, indexFrom(lower, term[0], start+1) {
		s.at = s.at[:0]
		score := 0.0
		i := start
		run := 0
		ok := true
		for k := 0; k < len(term); k++ {
			j := indexFrom(lower, term[k], i)
			if j < 0 {
				ok = false
				break
			}
			if n := len(s.at); n > 0 && j == s.at[n-1]+1 {
				run++
			} else {
				run = 0
			}
			score += 1 + float64(run)*4
			if boundary(text, j) {
				score += 6
			}
			if j >= base {
				score += 2
			}
			gap := min(j-i, 8)
			score -= float64(gap) * 0.3
			s.at = append(s.at, j)
			i = j + 1
		}
		if ok && (!found || score > bestScore) {
			found, bestScore = true, score
			s.best = append(s.best[:0], s.at...)
		}
	}
	return bestScore, found
}

// Match is how text matches the query, if it does. A nil scratch is fine
// for a one-off. With one, the match's At is the scratch's, and holds
// only until the next match: copy it to keep it.
func (q Query) Match(text string, s *Scratch) (Match, bool) { return match(q, text, s) }

// MatchBytes is Match for a text put together as bytes.
func (q Query) MatchBytes(text []byte, s *Scratch) (Match, bool) { return match(q, text, s) }

func match[T text](q Query, text T, s *Scratch) (Match, bool) {
	if q.Empty() {
		return Match{}, true
	}
	if s == nil {
		s = &Scratch{}
	}
	s.lower = s.lower[:0]
	for i := 0; i < len(text); i++ {
		s.lower = append(s.lower, lowerByte(text[i]))
	}
	var m Match
	s.out = s.out[:0]
	for _, term := range q.terms {
		score, ok := matchTerm(text, s.lower, term, s)
		if !ok {
			return Match{}, false
		}
		m.Score += score
		s.out = append(s.out, s.best...)
	}
	m.At = s.out
	// Shorter paths win ties: src/a.go over src/deep/nested/a.go.
	m.Score -= float64(len(text)) * 0.02
	return m, true
}

// Marks turns a match's bytes into places in the text as a browser counts
// them (UTF-16 units), which is what the page marks letters by.
func Marks(text string, at []int) []int {
	ascii := true
	for i := 0; i < len(text); i++ {
		if text[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii || len(at) == 0 {
		return at
	}
	units := make([]int, len(text)+1)
	n := 0
	for i, r := range text {
		for k := i; k < i+utf8.RuneLen(r) && k < len(text); k++ {
			units[k] = n
		}
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	out := make([]int, len(at))
	for i, b := range at {
		out[i] = units[b]
	}
	return out
}
