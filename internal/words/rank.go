package words

import (
	"sort"
	"strings"
	"unicode"
)

// How a place is ranked. SQLite's BM25 scores a row against its length,
// so a short section that says the words once beats the section headed by
// them, whatever its columns' weights (tried: weighted ten to one, the
// heading barely counted); and a document's title, in every one of its
// sections' rows, would lead to its shortest. So BM25 scores a place's
// heading and words alone (its title weighted 0), and a heading or a title
// that holds the words adds to it:
//
//	a place's score = BM25 of its heading and words + HeadingBoost × how much of what's asked its heading holds
//	a thing's score = its best place's
//	                  + TitleBoost × (how much of what's asked its title holds)² × how nearly its title is just that
//	                  + EveryBoost, when it says every word that matters
//
// How much is counted of the words that matter (not "how", "the": small),
// and for a heading of every word as typed too. A title that holds some of
// what's asked counts for less than its share (squared: half is a
// quarter), and one that's mostly what was asked for more than a long one
// that happens to say it ("House sorting CLI" over "Added 'demo sort' to
// print a … house" for house sorting): both were found on the records'
// cases, where a title holding two words of "who coined hello world" beat
// the turn that says all three. A thing whose title holds every word that
// matters leads to its top, unless a heading holds what was typed more
// nearly. internal/space/testdata/ranking.json holds these for documents;
// internal/ops/testdata/find/ranking.json, for what ws records.
const (
	HeadingBoost = 6.0
	TitleBoost   = 8.0
	EveryBoost   = 3.0
)

// Row is a place a search found, as FTS5 gave it: the thing it's a place
// in (Key: places of one thing are ranked as one), its order there (N: of
// two that score the same, the earlier leads), its thing's title and its
// own heading, as written and as highlight() marked them, BM25's score for
// it (lower is better, as SQLite has it), and snippet()'s words with what
// it starts with, for a snippet that holds none of what's asked.
type Row struct {
	Key                    string
	N                      int
	Title, TitleMarked     string
	Heading, HeadingMarked string
	BM25                   float64
	Snippet, Lead          string
	// Every is whether its thing holds every word that matters, in any of
	// its places.
	Every bool
	// Of is the caller's, for the row: it's given back with the place
	// that's best.
	Of any
}

// Ranked is a thing found: its best place (Row), its score, its snippet,
// and whether it's the thing as a whole that's asked for (Top: its title
// holds every word that matters, and no heading holds what was typed more
// nearly), in which case it leads to its top.
type Ranked struct {
	Row
	Score   float64
	Snippet []Part
	Top     bool
}

// titleWorth is how much a title counts: titled (how much of what's
// asked it holds, 0 to 1) squared, by how nearly it's just that: from a
// half, for a title that's long beside what it holds of it, to whole, for
// a title that's nothing else.
func titleWorth(titled float64, title, marked string) float64 {
	n := countWords(title)
	if n == 0 {
		return 0
	}
	return titled * titled * (1 + min(1, float64(countMarked(marked))/float64(n))) / 2
}

func countWords(s string) int {
	return len(strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }))
}

// countMarked is how many words a highlighted run has marked.
func countMarked(s string) int {
	n := 0
	for {
		i := strings.IndexByte(s, MarkOpen)
		if i < 0 {
			return n
		}
		s = s[i+1:]
		j := strings.IndexByte(s, MarkClose)
		if j < 0 {
			j = len(s)
		}
		n += countWords(s[:j])
		if j >= len(s) {
			return n
		}
		s = s[j+1:]
	}
}

// Rank is the things rows are places in, each led to its best place, best
// first (a tie by key).
func Rank(q Query, rows []Row) []Ranked {
	type thing struct {
		best     Row
		score    float64 // its best place's
		titled   float64 // how much of what's asked its title holds, 0 to 1
		titleAll float64 // how much of every word as typed its title holds
		headAll  float64 // and its best place's heading
	}
	things := map[string]*thing{}
	var order []*thing
	for _, r := range rows {
		// How much of what's asked the heading holds: of the words that
		// matter, and of every word as typed, so "What a decision is" beats
		// "How decisions are made" for what a decision is.
		headed := (Held(r.HeadingMarked, q.Content, true) + HeldAll(r.HeadingMarked, r.Heading, q)) / 2
		score := -r.BM25 + HeadingBoost*headed
		t := things[r.Key]
		first := t == nil
		if first {
			t = &thing{titled: Held(r.TitleMarked, q.Content, true), titleAll: HeldAll(r.TitleMarked, r.Title, q)}
			things[r.Key] = t
			order = append(order, t)
		}
		if first || score > t.score || (score == t.score && r.N < t.best.N) {
			t.best, t.score = r, score
			t.headAll = HeldAll(r.HeadingMarked, r.Heading, q)
		}
	}
	out := make([]Ranked, 0, len(order))
	for _, t := range order {
		every := 0.0
		if t.best.Every {
			every = EveryBoost
		}
		snip := t.best.Snippet
		if snip == "" || !strings.ContainsRune(snip, MarkOpen) {
			snip = t.best.Lead // its words don't hold what's asked: how it starts
		}
		out = append(out, Ranked{
			Row:     t.best,
			Score:   t.score + TitleBoost*titleWorth(t.titled, t.best.Title, t.best.TitleMarked) + every,
			Snippet: Parts(snip),
			// It's the thing that's asked for, unless a heading says what
			// was asked more nearly than its title.
			Top: t.titled == 1 && t.titleAll >= t.headAll,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Key < out[j].Key
	})
	return out
}
