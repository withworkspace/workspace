package find

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Pull is an open pull request, as much of it as Find shows.
type Pull struct {
	Repo   string
	Number int
	Title  string
	Author string
	Branch string
	// State is the section it's in on the Reviews page (needs-review,
	// ready…), and Words why, in a few.
	State  string
	Words  string
	Checks string
	Draft  bool
	Mine   bool
	// Stack says where it sits in a stack: "2 of 4: Key by tenant".
	Stack string
}

// Ref is how ws names it: demo#12.
func (p Pull) Ref() string { return p.Repo + "#" + strconv.Itoa(p.Number) }

func pullResult(p Pull) Result {
	return Result{
		Kind: KindPull, Ref: p.Ref(), Title: p.Title, Repo: p.Repo, Number: p.Number,
		Author: p.Author, Branch: p.Branch, State: p.State, Words: p.Words, Checks: p.Checks,
		Draft: p.Draft, Mine: p.Mine, Stack: p.Stack,
	}
}

var (
	// pullTerm is a term that's about a number: 12, #12, demo#12, demo#.
	pullTerm = regexp.MustCompile(`^(?:([\w.-]+)?#)?(\d*)$`)
)

// Scores for what isn't a matter of letters: a number, an author.
const (
	scoreExact  = 1000.0 // the number, whole
	scoreNumber = 400.0  // the number's start
	scoreAuthor = 60.0
)

// searchPulls is the pull requests that match what's typed: by number
// (12, #12, demo#12), by the letters of its title or its branch, or by who
// it's by (sam, @sam). Every term has to match something of it. With
// nothing typed they come as the list has them, what needs you first.
func searchPulls(pulls []Pull, q string, limit int) ([]Result, int) {
	terms := strings.Fields(q)
	type scored struct {
		r     Result
		order int
	}
	var out []scored
	s := &Scratch{}
	for order, p := range pulls {
		r := pullResult(p)
		ok := true
		for _, term := range terms {
			score, at, hit := matchPull(p, term, s)
			if !hit {
				ok = false
				break
			}
			r.Score += score
			r.At = append(r.At, at...)
		}
		if !ok {
			continue
		}
		r.At = Marks(r.Title, r.At)
		out = append(out, scored{r, order})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].r.Score != out[j].r.Score {
			return out[i].r.Score > out[j].r.Score
		}
		return out[i].order < out[j].order
	})
	res := make([]Result, 0, min(limit, len(out)))
	for i := 0; i < len(out) && i < limit; i++ {
		res = append(res, out[i].r)
	}
	return res, len(out) - len(res)
}

// matchPull is how one term matches a pull request: the best of its
// number, who it's by, its title and its branch. at is where in the title.
func matchPull(p Pull, term string, s *Scratch) (score float64, at []int, ok bool) {
	lower := strings.ToLower(term)
	if m := pullTerm.FindStringSubmatch(lower); m != nil && (m[2] != "" || strings.Contains(lower, "#")) {
		if m[1] == "" || m[1] == strings.ToLower(p.Repo) {
			n := strconv.Itoa(p.Number)
			switch {
			case m[2] == n:
				score, ok = scoreExact, true
			case strings.HasPrefix(n, m[2]):
				score, ok = scoreNumber-float64(len(n)-len(m[2]))*10, true
			}
		}
		if strings.Contains(lower, "#") {
			return score, nil, ok // a number was meant, and nothing else
		}
	}
	author := strings.ToLower(p.Author)
	if who := strings.TrimPrefix(lower, "@"); who != "" && author != "" {
		switch {
		case who == author:
			if scoreAuthor > score {
				score, ok = scoreAuthor, true
			}
		case strings.HasPrefix(author, who):
			if v := scoreAuthor - 20; v > score {
				score, ok = v, true
			}
		}
		if strings.HasPrefix(lower, "@") {
			return score, nil, ok
		}
	}
	query := NewQuery(term)
	if m, hit := query.Match(p.Title, s); hit && (!ok || m.Score > score) {
		score, ok = m.Score, true
		at = append([]int(nil), m.At...)
	}
	// Its branch and its repo count for less than its title.
	if m, hit := query.Match(p.Branch, s); hit && (!ok || m.Score-4 > score) {
		score, at, ok = m.Score-4, nil, true
	}
	if m, hit := query.Match(p.Repo, s); hit && (!ok || m.Score-6 > score) {
		score, at, ok = m.Score-6, nil, true
	}
	return score, at, ok
}
