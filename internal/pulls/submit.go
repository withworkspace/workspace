package pulls

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/review"
)

// Submitting a review posts it to GitHub, as you: the one thing here that
// leaves your machine. Posting says exactly what will go and where, for
// the dialog to show before it does.

// kindLabels are a comment's kind as Conventional Comments
// (conventionalcomments.org) writes it, at the start of the comment.
var kindLabels = map[string]string{
	"blocking":   "issue (blocking):",
	"suggestion": "suggestion:",
	"question":   "question:",
	"nit":        "nitpick:",
}

var conventional = regexp.MustCompile(`(?i)^\s*\**(praise|nitpick|nit|suggestion|issue|todo|question|thought|chore|note|typo|polish|quibble)\**\s*(\(([^)]*)\))?\s*:\**[ \t]*`)

// ParseKind reads a comment's kind back from its first words, as
// Conventional Comments writes them, and answers the rest. A comment that
// doesn't say is kind "".
func ParseKind(body string) (kind, rest string) {
	m := conventional.FindStringSubmatch(body)
	if m == nil {
		return "", body
	}
	label, decorations := strings.ToLower(m[1]), strings.ToLower(m[3])
	blocking := strings.Contains(decorations, "blocking") && !strings.Contains(decorations, "non-blocking")
	switch label {
	case "issue":
		kind = "blocking"
		if strings.Contains(decorations, "non-blocking") {
			kind = "suggestion"
		}
	case "suggestion", "todo", "chore":
		kind = "suggestion"
		if blocking {
			kind = "blocking"
		}
	case "question":
		kind = "question"
	case "nitpick", "nit", "typo", "polish", "quibble":
		kind = "nit"
	default: // praise, thought, note: something said, nothing asked
		return "", body
	}
	return kind, strings.TrimLeft(body[len(m[0]):], "\n")
}

// commentBody is a comment as it's posted: its kind first, then what it
// says, then a suggestion block, which GitHub can apply.
func commentBody(n review.Note) string {
	b := kindLabels[n.Kind]
	if n.Body != "" {
		b += " " + n.Body
	}
	if n.Suggestion != nil {
		b += "\n\n```suggestion\n" + *n.Suggestion
		if *n.Suggestion != "" && !strings.HasSuffix(*n.Suggestion, "\n") {
			b += "\n"
		}
		b += "```"
	}
	return strings.TrimSpace(b)
}

var events = map[string]string{review.Approve: github.EventApprove, review.Changes: github.EventRequestChanges, review.Comment: github.EventComment}

// Posting is what submitting a review would post, and where.
type Posting struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	// Commit is the commit the comments are made at: the one you reviewed.
	Commit string `json:"commit"`
	// Event is GitHub's name for the verdict.
	Event string `json:"event"`
	// Body is the review's body: your summary, then any comments that
	// can't go on their lines.
	Body     string          `json:"body"`
	Comments []PostedComment `json:"comments"`
	// Loose is how many comments are in the body instead of on their
	// lines, because the lines aren't in the PR's diff on GitHub.
	Loose int `json:"loose,omitempty"`
}

// PostedComment is a comment as it'll be posted on lines.
type PostedComment struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Side      string `json:"side"` // RIGHT or LEFT
	StartLine int    `json:"startLine,omitempty"`
	Line      int    `json:"line"`
	Body      string `json:"body"`
}

func (p *Posting) submission() github.ReviewSubmission {
	sub := github.ReviewSubmission{Commit: p.Commit, Event: p.Event, Body: p.Body}
	for _, c := range p.Comments {
		rc := github.ReviewComment{Path: c.Path, Body: c.Body, Line: c.Line, Side: c.Side}
		if c.StartLine != 0 && c.StartLine != c.Line {
			rc.StartLine, rc.StartSide = c.StartLine, c.Side
		}
		sub.Comments = append(sub.Comments, rc)
	}
	return sub
}

// Posting works out what submitting a PR's draft with this verdict and
// summary would post, without posting it. head is the commit reviewed.
func (s *Service) Posting(repo string, number int, head, verdict, summary string) (*Posting, error) {
	// (Found as it's known here, as Submit finds it.)
	pr, st, err := s.Known(repo, number)
	if err != nil {
		return nil, err
	}
	event := events[verdict]
	if event == "" {
		return nil, fmt.Errorf("a review ends with %s, not %q", strings.Join(review.Verdicts, ", "), verdict)
	}
	if pr.Mine && verdict != review.Comment {
		return nil, errors.New("it's your own pull request: GitHub takes comments on it, not a verdict")
	}
	l, err := s.layer(pr, st)
	if err != nil {
		return nil, err
	}
	if head == "" {
		head = l.head
	}
	draft, err := s.Store(repo, number).Draft(Key(repo, number))
	if err != nil {
		return nil, err
	}
	p := &Posting{Repo: repo, Number: number, Title: pr.Title, URL: pr.URL, Commit: head, Event: event, Comments: []PostedComment{}}
	summary = strings.TrimSpace(summary)
	if draft == nil || len(draft.Comments) == 0 {
		if verdict != review.Approve && summary == "" {
			return nil, errors.New("say what needs changing: comment on the lines, or write a summary")
		}
		p.Body = summary
		return p, nil
	}
	// GitHub takes a comment only on lines of the PR's own diff, at the
	// commit reviewed: where it left its base to that commit.
	base := l.base
	if head != l.head {
		tip := review.CommitOf(l.dir, l.baseRef)
		if b, err := review.MergeBase(l.dir, tip, head); err == nil {
			base = b
		}
	}
	hunks := map[string]*review.File{}
	var loose, general []string
	for _, n := range draft.Comments {
		// One on the change as a whole is part of what the review says.
		if n.General() {
			general = append(general, commentBody(n))
			continue
		}
		side, ok := "RIGHT", n.Commit == head
		if n.Side == "old" {
			side, ok = "LEFT", n.Commit == base
		}
		if ok {
			f, seen := hunks[n.Path]
			if !seen {
				f, _ = review.FileDiff(l.dir, base, head, n.Path)
				hunks[n.Path] = f
			}
			ok = f != nil && inHunk(f, n.Side, n.Line, n.EndLine)
		}
		if !ok {
			loose = append(loose, looseComment(n))
			continue
		}
		pc := PostedComment{ID: n.ID, Path: n.Path, Side: side, Line: n.EndLine, Body: commentBody(n)}
		if n.EndLine > n.Line {
			pc.StartLine = n.Line
		}
		p.Comments = append(p.Comments, pc)
	}
	p.Body = strings.Join(append(nonEmpty(summary), general...), "\n\n")
	if len(loose) > 0 {
		p.Loose = len(loose)
		if p.Body != "" {
			p.Body += "\n\n"
		}
		p.Body += "On lines outside this pull request's diff:\n\n" + strings.Join(loose, "\n\n")
	}
	return p, nil
}

// inHunk is whether lines of a file's side all sit in one hunk of its
// diff: GitHub takes a comment nowhere else.
func inHunk(f *review.File, side string, line, endLine int) bool {
	for _, h := range f.Hunks {
		first, last := false, false
		for _, l := range h.Lines {
			n := l.New
			if side == "old" {
				n = l.Old
				if l.Kind == "a" {
					continue
				}
			} else if l.Kind == "d" {
				continue
			}
			first = first || n == line
			last = last || n == endLine
		}
		if first && last {
			return true
		}
	}
	return false
}

// looseComment is a comment as it reads in a review's body, when it can't
// go on its lines.
func looseComment(n review.Note) string {
	where := fmt.Sprintf("line %d", n.Line)
	if n.EndLine > n.Line {
		where = fmt.Sprintf("lines %d–%d", n.Line, n.EndLine)
	}
	if n.Side == "old" {
		where = "removed " + where
	}
	// A suggestion can't be applied from a review's body: it's shown as code.
	body := commentBody(n)
	body = strings.Replace(body, "```suggestion\n", "```\n", 1)
	return fmt.Sprintf("**`%s`, %s:** %s", n.Path, where, body)
}

// Submitted is what submitting did.
type Submitted struct {
	Review *review.Review `json:"review"`
	URL    string         `json:"url,omitempty"` // the review on GitHub
	// Sending: it's kept here as submitted, and on its way to GitHub.
	Sending bool `json:"sending,omitempty"`
}

// Submit posts a PR's draft review to GitHub with a verdict, as you, and
// keeps it: the commit reviewed stays in the clone, with each file's
// fingerprint, for looking again.
//
// Where there's somewhere to send from (SendBehind), it's kept first, as
// submitted, and sent behind: what's sent is the posting as it was worked
// out here, whatever's written since. A stack's reviews take turns on the
// stack, in the order they were submitted (bottom first), and one GitHub
// refuses holds those behind it. One it refuses is said on the PR's page
// (Unsent), to send again or make a draft of again.
func (s *Service) Submit(repo string, number int, head, verdict, summary string) (*Submitted, error) {
	p, err := s.Posting(repo, number, head, verdict, summary)
	if err != nil {
		return nil, err
	}
	// The PR is found as it's known here, not asked for: posting a review
	// marks the list stale, so asking would have GitHub asked about every
	// repo before each review of a stack is posted.
	pr, st, err := s.Known(repo, number)
	if err != nil {
		return nil, err
	}
	l, err := s.layer(pr, st)
	if err != nil {
		return nil, err
	}
	base := l.base
	if p.Commit != l.head {
		if b, err := review.MergeBase(l.dir, review.CommitOf(l.dir, l.baseRef), p.Commit); err == nil {
			base = b
		}
	}
	if s.Outbox == nil {
		url, err := s.GitHub.SubmitReview(s.owner(repo), s.name(repo), number, p.submission())
		if err != nil {
			return nil, err
		}
		// It's posted: from here on, nothing may lose that.
		r, _, err := s.keepReview(repo, number, l, base, p, verdict, summary, url)
		if err != nil {
			return &Submitted{URL: url}, fmt.Errorf("the review is on GitHub (%s), but ws couldn't keep it: %w", url, err)
		}
		s.Stale() // what it asks of you has changed
		return &Submitted{Review: r, URL: url}, nil
	}
	r, was, err := s.keepReview(repo, number, l, base, p, verdict, summary, "")
	if err != nil {
		return nil, err
	}
	w := outgoing{Repo: repo, Number: number, Review: r.ID, Posting: p, Was: was}
	if _, err := s.Outbox.Add(reviewLane(repo, number, st), sendReview, "your review of "+Key(repo, number), w); err != nil {
		// It can't be kept to send, so it isn't submitted: a draft again.
		_ = s.takeBack(w) // the outbox's error is the one to say
		return nil, err
	}
	s.bump(repo, number)
	return &Submitted{Review: r, Sending: true}, nil
}

// keepReview keeps a PR's draft as your review of it, submitted: in its
// reviews, with the commit reviewed kept in the clone and each file's
// fingerprint, and it no longer asks for your review. url is where it is
// on GitHub, "" for one still to be sent. It's the review, and the one
// kept as yours before it.
func (s *Service) keepReview(repo string, number int, l *layer, base string, p *Posting, verdict, summary, url string) (*review.Review, *Reviewed, error) {
	at := s.now()
	r, err := s.Store(repo, number).Submit(Key(repo, number), review.Diffed{Dir: l.dir, BaseRef: l.baseRef, Base: base, Head: p.Commit}, verdict, summary, at)
	if err != nil {
		return nil, nil, err
	}
	prints, _ := review.Fingerprints(l.dir, base, p.Commit)
	release := s.lockRepo(repo)
	if err := Keep(l.dir, ReviewedRef(number, r.ID), p.Commit); err != nil {
		log.Printf("%s: couldn't keep the commit reviewed: %v", Key(repo, number), err)
	}
	release()
	var was *Reviewed
	if _, err := Update(s.Root, repo, number, func(rec *Record) {
		was = rec.Reviewed
		rec.Reviewed = &Reviewed{ID: r.ID, Verdict: verdict, Head: p.Commit, Base: base, At: at, URL: url, Prints: prints}
	}); err != nil {
		log.Printf("%s: couldn't record the review: %v", Key(repo, number), err)
	}
	s.settle(repo, number) // it doesn't ask for your review any more
	return r, was, nil
}

// reviewLane is what a PR's review takes turns on: its stack, so a
// stack's are posted in the order they were submitted, or itself.
func reviewLane(repo string, number int, st *Stack) string {
	if st != nil && len(st.Layers) > 0 {
		return "stack " + Key(repo, st.Layers[0].Number)
	}
	return laneOf(repo, number)
}

// sendReview posts a review kept here to GitHub, and keeps where it is.
func (s *Service) postReview(w outgoing) error {
	if w.Posting == nil {
		return errors.New("what was to be posted can't be read")
	}
	if s.GitHub == nil {
		return errors.New("there's no GitHub to ask here")
	}
	url, err := s.GitHub.SubmitReview(s.owner(w.Repo), s.name(w.Repo), w.Number, w.Posting.submission())
	if err != nil {
		return err
	}
	s.reviewAt(w, url)
	return nil
}

// reviewAt keeps where a review sent behind is on GitHub.
func (s *Service) reviewAt(w outgoing, url string) {
	if _, err := Update(s.Root, w.Repo, w.Number, func(rec *Record) {
		if rec.Reviewed != nil && rec.Reviewed.ID == w.Review {
			rec.Reviewed.URL = url
		}
	}); err != nil {
		log.Printf("%s: couldn't keep where the review is: %v", Key(w.Repo, w.Number), err)
	}
}

// states are GitHub's names for a review as it stands, by the event that
// made it.
var states = map[string]string{github.EventApprove: "APPROVED", github.EventRequestChanges: "CHANGES_REQUESTED", github.EventComment: "COMMENTED"}

// madeSkew is how far GitHub's clock may be behind this one's and a
// review there still be the one sent from here.
const madeSkew = 5 * time.Minute

// reviewMade says whether a review the daemon stopped in the middle of
// sending is on GitHub: yours, of that commit, with that verdict, made no
// earlier than it was submitted here. Posting it twice would be two.
func (s *Service) reviewMade(w outgoing, queued time.Time) (bool, error) {
	if w.Posting == nil || s.GitHub == nil {
		return false, errors.New("what was to be posted can't be read")
	}
	me, err := s.GitHub.Viewer()
	if err != nil {
		return false, err
	}
	said, err := s.GitHub.Discussion(s.owner(w.Repo), s.name(w.Repo), w.Number)
	if err != nil {
		return false, err
	}
	for _, g := range slices.Backward(said) {

		if g.State != states[w.Posting.Event] || !sameLogin(g.Author, me) || g.Commit != w.Posting.Commit {
			continue
		}
		if at, err := time.Parse(time.RFC3339, g.At); err == nil && at.Before(queued.Add(-madeSkew)) {
			continue
		}
		s.reviewAt(w, g.URL)
		return true, nil
	}
	return false, nil
}

// takeBack makes a review that was to be sent your draft again, and what
// was kept as your review before it is again. It's refused while there's
// a newer draft.
func (s *Service) takeBack(w outgoing) error {
	if _, err := s.Store(w.Repo, w.Number).Unsubmit(w.Review); err != nil && !errors.Is(err, review.ErrNotFound) {
		return err
	}
	_, err := Update(s.Root, w.Repo, w.Number, func(rec *Record) {
		if rec.Reviewed != nil && rec.Reviewed.ID == w.Review {
			rec.Reviewed = w.Was
		}
	})
	s.Stale() // it asks for your review again
	return err
}

// nonEmpty is s as a list of one, or of none if there's nothing in it.
func nonEmpty(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return []string{s}
}
