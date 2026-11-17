package conversation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// Passes: a pass reads the whole of a page and says things about it (what's
// open, what it assumes, what it requires). What it says is kept here,
// beside the page, and never written into it. A pass is a conversation
// (kind pass) anchored to its page, its goal the pass's name: each run is a
// turn of yours ("Run it") answered by the agent, whose reply is read into
// findings.

// Pass is a pass over a page.
const Pass = "pass"

// Finding is one thing a pass found: its kind (question, assumption,
// requirement…), the words of the page it's about (quoted exactly, to find
// them by; none for something missing), what it says, and, where it has
// some, what the words should read instead.
type Finding struct {
	// Key names it from one run to the next: a finding of the same kind on
	// the same words is the same finding, however it's worded.
	Key     string `json:"key"`
	Kind    string `json:"kind"`
	Quote   string `json:"quote,omitempty"`
	Text    string `json:"text"`
	Suggest string `json:"suggest,omitempty"`
	// A review's finding is on lines of a diff, not words of a page: a
	// path, a side ("new", or "old" for lines the change removes) and the
	// lines, with how they read (Excerpt) at Commit. With no path, it's
	// about the change as a whole.
	Path    string   `json:"path,omitempty"`
	Side    string   `json:"side,omitempty"`
	Line    int      `json:"line,omitempty"`
	EndLine int      `json:"endLine,omitempty"`
	Commit  string   `json:"commit,omitempty"`
	Excerpt []string `json:"excerpt,omitempty"`
	// Thread is the comment of yours a follow-up's finding is about (its
	// thread on GitHub): whether what's been pushed since deals with it.
	// Blocks is a finding that makes its pass no go while it's open
	// whatever its kind (a blocking comment not dealt with).
	Thread string `json:"thread,omitempty"`
	Blocks bool   `json:"blocks,omitempty"`
}

// Placed is whether a review's finding is on lines.
func (f Finding) Placed() bool { return f.Path != "" && f.Line > 0 }

// Finding states: open until you say it's done (answered, dealt with) or
// dismiss it. One you've settled stays settled when the pass is run again.
const (
	FindingOpen      = "open"
	FindingDone      = "done"
	FindingDismissed = "dismissed"
)

// Why a finding was dismissed: it's wrong (it isn't so), the page already
// says it, or it's so but not worth acting on. How often a pass's findings
// are each is how well it does (ws show passes).
const (
	DismissWrong   = "wrong"
	DismissAlready = "already"
	DismissMinor   = "minor"
)

// DismissReasons are the reasons a finding can be dismissed for.
var DismissReasons = []string{DismissWrong, DismissAlready, DismissMinor}

// findingSettled is you settling a finding, or opening it again: Why is
// why it was dismissed, if you said.
type findingSettled struct {
	Key   string    `json:"key"`
	State string    `json:"state"`
	Why   string    `json:"why,omitempty"`
	At    time.Time `json:"at"`
}

// FindingRoom is how many findings a run keeps.
const FindingRoom = 20

// ParseFindings reads a pass's reply: what it says of the page as a whole,
// then each finding under a heading that's its kind.
//
//	The page says what's wanted but not how it's checked.
//
//	## question
//	> It should be fast
//	What counts as fast: a time for the binary to start, or no new dependencies?
//
//	```suggest
//	It should start in under 50ms.
//	```
//
// A reply with no headings is all summary.
func ParseFindings(reply string) (summary string, findings []Finding) {
	summary, findings = parseFindings(reply, false)
	KeyFindings(findings)
	return summary, findings
}

// ParseReviewFindings reads a review's pass's reply: as a page's, with
// each finding's place where a page's has its quote, a line that says the
// path and lines it's on:
//
//	The token check is sound; the cache key isn't.
//
//	## blocking
//	@ internal/cache/key.go:41-44
//	The key leaves the tenant out, so one tenant reads another's entry.
//
//	```suggestion
//	key := tenant + ":" + token
//	```
//
// Lines the change removes are said so (@ old.go:12 old). A finding with
// no @ line is about the change as a whole. The findings aren't keyed:
// that's done once their lines are read (KeyFindings).
func ParseReviewFindings(reply string) (summary string, findings []Finding) {
	return parseFindings(reply, true)
}

// place reads a review's finding's place: path:line, path:line-line, with
// "old" after it for lines the change removes.
var place = regexp.MustCompile("^@\\s*`?([^`\\s:][^`:]*?)`?:(\\d+)(?:\\s*[-–]\\s*(\\d+))?`?\\s*(?:\\(?(old|removed|new)\\)?)?\\s*$")

// threadPlace reads which comment a follow-up's finding is about: @ T3.
var threadPlace = regexp.MustCompile(`^@\s*T(\d+)\.?\s*$`)

// KeyFindings names each finding from one run to the next. One of a page
// is its kind on the words it quotes; one of a review, its kind on the
// lines it's on as they read, in its file, wherever they've moved to (so a
// rebase doesn't make it another finding). With no words and no lines,
// it's its kind and what it says.
func KeyFindings(findings []Finding) {
	seen := map[string]int{}
	for i := range findings {
		f := &findings[i]
		var of string
		switch {
		case f.Thread != "":
			of = f.Kind + "\nthread\n" + f.Thread
		case f.Quote != "":
			of = f.Kind + "\n" + strings.ToLower(f.Quote)
		case f.Placed() && len(f.Excerpt) > 0:
			lines := make([]string, len(f.Excerpt))
			for j, l := range f.Excerpt {
				lines[j] = strings.TrimSpace(l)
			}
			of = f.Kind + "\n" + f.Path + "\n" + f.Side + "\n" + strings.Join(lines, "\n")
		default: // nothing quoted: it's what it says
			of = f.Kind + "\n\n" + strings.ToLower(f.Text)
		}
		sum := sha256.Sum256([]byte(of))
		f.Key = hex.EncodeToString(sum[:])[:10]
		if n := seen[f.Key]; n > 0 {
			f.Key += fmt.Sprintf("-%d", n+1)
		}
		seen[strings.SplitN(f.Key, "-", 2)[0]]++
	}
}

func parseFindings(reply string, places bool) (summary string, findings []Finding) {
	var head []string
	var cur *Finding
	var body, quote, suggest []string
	inSuggest, fence := false, ""
	flush := func() {
		if cur == nil {
			return
		}
		cur.Quote = strings.Join(strings.Fields(strings.Join(quote, " ")), " ")
		cur.Text = strings.TrimSpace(strings.Join(body, "\n"))
		cur.Suggest = strings.TrimSpace(strings.Join(suggest, "\n"))
		if places {
			// What a suggestion puts in place of its lines is as it's
			// written, indented as it is.
			cur.Suggest = strings.Trim(strings.Join(suggest, "\n"), "\n")
		}
		if (cur.Text != "" || cur.Quote != "" || cur.Thread != "" || (cur.Placed() && cur.Suggest != "")) && len(findings) < FindingRoom {
			findings = append(findings, *cur)
		}
		cur, body, quote, suggest = nil, nil, nil, nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(reply, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		if inSuggest {
			if t == fence {
				inSuggest = false
				continue
			}
			suggest = append(suggest, line)
			continue
		}
		switch {
		case strings.HasPrefix(t, "## "):
			flush()
			cur = &Finding{Kind: findingKind(t[3:])}
		case cur == nil:
			head = append(head, line)
		case strings.HasPrefix(t, "```suggest") || strings.HasPrefix(t, "~~~suggest"):
			inSuggest, fence = true, t[:3]
		case places && len(body) == 0 && cur.Path == "" && place.MatchString(t):
			m := place.FindStringSubmatch(t)
			cur.Path, cur.Side = strings.TrimSpace(m[1]), "new"
			cur.Line, _ = strconv.Atoi(m[2])
			cur.EndLine = cur.Line
			if m[3] != "" {
				if end, _ := strconv.Atoi(m[3]); end > cur.Line {
					cur.EndLine = end
				}
			}
			if m[4] == "old" || m[4] == "removed" {
				cur.Side = "old"
			}
		case places && len(body) == 0 && cur.Path == "" && cur.Thread == "" && threadPlace.MatchString(t):
			// A follow-up's finding is about a comment, named as it was listed.
			cur.Thread = "T" + threadPlace.FindStringSubmatch(t)[1]
		case places && len(body) == 0 && cur.Path == "" && strings.HasPrefix(t, "@"):
			// A place that can't be read is no place: what's said still
			// stands, about the change as a whole.
		case places && len(body) == 0 && t == "":
		case !places && strings.HasPrefix(t, ">") && len(body) == 0:
			quote = append(quote, strings.TrimSpace(strings.TrimPrefix(t, ">")))
		default:
			body = append(body, line)
		}
	}
	flush()
	return strings.TrimSpace(strings.Join(head, "\n")), findings
}

// findingKind is a finding's kind from its heading: its first words, up
// to anything said after them (a colon, a dash), lowercased and joined by
// hyphens: "question", "to-write", "ground" (of "ground: false"). A heading
// that runs on is its first word.
func findingKind(heading string) string {
	kind := strings.ToLower(strings.TrimSpace(heading))
	if i := strings.IndexAny(kind, ":—–(|"); i >= 0 {
		kind = kind[:i]
	}
	words := strings.Fields(strings.Trim(kind, "*_`:. -"))
	if len(words) > 3 {
		words = words[:1]
	}
	for i, w := range words {
		words[i] = strings.Trim(w, "*_`:.,")
	}
	return strings.Join(words, "-")
}

// OpenPass starts a pass over a page: its first run.
func (s Store) OpenPass(page, name string, at time.Time) (*Conversation, error) {
	if !s.Workspace() {
		return nil, errors.New("a page's passes are the workspace's")
	}
	if !cellGoal.MatchString(name) || name == "" {
		return nil, fmt.Errorf("a pass is named by a word, not %q", name)
	}
	return s.open(&Conversation{Kind: Pass, Goal: name, Anchor: Anchor{Page: page}, By: You, At: at, Status: Open}, Turn{By: You, At: at, Body: "Run it."})
}

// Reviewed names what a review's pass is over: a pull request (api#89),
// or a mission's capsule (cache-tokens:api/tokens). "" for an anchor
// that's neither.
func (a Anchor) Reviewed() string {
	switch {
	case a.PR != "":
		return a.PR
	case a.Capsule != "":
		return a.Mission + ":" + a.Capsule
	}
	return ""
}

// ReviewedCapsule is the anchor of a pass over a mission's capsule
// (repo/name).
func ReviewedCapsule(mission, capsule string) Anchor {
	return Anchor{Mission: mission, Capsule: capsule}
}

// OpenReviewPass starts a pass over a review of a pull request (api#89):
// its first run.
func (s Store) OpenReviewPass(pr, name string, at time.Time) (*Conversation, error) {
	return s.OpenPassOver(Anchor{PR: pr}, name, at)
}

// OpenPassOver starts a pass over a review: of a pull request, or of a
// mission's capsule. Its first run.
func (s Store) OpenPassOver(over Anchor, name string, at time.Time) (*Conversation, error) {
	if !s.Workspace() {
		return nil, errors.New("a review's passes are the workspace's")
	}
	if !cellGoal.MatchString(name) || name == "" {
		return nil, fmt.Errorf("a pass is named by a word, not %q", name)
	}
	if over.Reviewed() == "" {
		return nil, errors.New("a review's pass is over a pull request, or a mission's capsule")
	}
	return s.open(&Conversation{Kind: Pass, Goal: name, Anchor: Anchor{PR: over.PR, Mission: over.Mission, Capsule: over.Capsule}, By: You, At: at, Status: Open}, Turn{By: You, At: at, Body: "Run it."})
}

// ReviewPassOn is the pass named over a pull request's review, or nil if
// it's never been run.
func (s Store) ReviewPassOn(pr, name string) *Conversation { return s.PassOver(Anchor{PR: pr}, name) }

// PassOver is the pass named over a review (a pull request's, or a
// mission's capsule's), or nil if it's never been run.
func (s Store) PassOver(over Anchor, name string) *Conversation {
	of := over.Reviewed()
	if of == "" {
		return nil
	}
	return latestOf(s.list(`c.reviewed = ? AND c.kind = ? AND c.goal = ?`, of, Pass, name))
}

// PassOn is the pass named over a page, or nil if it's never been run.
func (s Store) PassOn(page, name string) *Conversation {
	return latestOf(s.list(`c.page = ? AND c.kind = ? AND c.goal = ?`, page, Pass, name))
}

// PassesOn are the passes that have been run over a page.
func (s Store) PassesOn(page string) []*Conversation {
	out, _ := s.list(`c.page = ? AND c.kind = ?`, page, Pass)
	return out
}

// Passes are the passes that have been run, over pages and reviews.
func (s Store) Passes() ([]*Conversation, error) { return s.list(`c.kind = ?`, Pass) }

// PassesOverPRs are the passes that have been run over pull requests'
// reviews.
func (s Store) PassesOverPRs() ([]*Conversation, error) {
	return s.list(`c.pr != '' AND c.kind = ?`, Pass)
}

// latestOf is the last of a list, or nil.
func latestOf(all []*Conversation, _ error) *Conversation {
	if len(all) == 0 {
		return nil
	}
	return all[len(all)-1]
}

// SettleFinding marks a pass's finding done or dismissed (why: one of
// DismissReasons, or "" unsaid), or opens it again. It's kept by the
// finding's key, so it holds when the pass is run again.
func (s Store) SettleFinding(id, key, state, why string, at time.Time) (*Conversation, error) {
	if state != FindingOpen && state != FindingDone && state != FindingDismissed {
		return nil, fmt.Errorf("a finding is open, done or dismissed, not %q", state)
	}
	if why != "" && (state != FindingDismissed || !slices.Contains(DismissReasons, why)) {
		return nil, fmt.Errorf("a finding is dismissed as %s, and only dismissed: not %q", strings.Join(DismissReasons, ", "), why)
	}
	return s.update(id, func(tx *store.Tx, c *Conversation) error {
		if c.Kind != Pass {
			return fmt.Errorf("%w: %s isn't a pass", ErrNotFound, id)
		}
		return s.add(tx, c, line{Settle: &findingSettled{Key: key, State: state, Why: why, At: at}})
	})
}

func (c *Conversation) settle(key, state, why string) {
	delete(c.Dismissed, key)
	if state == FindingOpen {
		delete(c.Settled, key)
		return
	}
	if c.Settled == nil {
		c.Settled = map[string]string{}
	}
	c.Settled[key] = state
	if why != "" {
		if c.Dismissed == nil {
			c.Dismissed = map[string]string{}
		}
		c.Dismissed[key] = why
	}
}

// DismissedAs is why a dismissed finding was dismissed, if you said.
func (c *Conversation) DismissedAs(key string) string { return c.Dismissed[key] }

// FindingState is where a finding stands: open, unless you've settled it.
func (c *Conversation) FindingState(key string) string {
	if st := c.Settled[key]; st != "" {
		return st
	}
	return FindingOpen
}
