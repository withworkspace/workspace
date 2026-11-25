package pulls

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/outbox"
	"github.com/withworkspace/workspace/internal/review"
)

// What's said from a pull request's page (a reply in a thread, resolving
// or reopening one, a comment on the pull request, taking your name off
// its reviewers) is shown at once and sent to GitHub behind, where the
// service has somewhere to send from (SendBehind: the daemon's outbox).
// GitHub's slow: the write, then reading what it changed, is a second or
// more, which the page needn't wait on.
//
// While a write's waiting, what's kept of the page is shown with it on:
// the reply in its thread, the thread resolved, the comment under the
// others, each marked as on its way. Once GitHub has it, what it changed
// is read again and takes the place of what was shown. One GitHub refuses
// is kept, with why, and said on the page (Unsent), to send again or let
// go of.
//
// A service with nowhere to send from (a command's) posts at once.

// The kinds of write a pull request's page makes.
const (
	sendThread  = "pr-thread"  // a reply in a thread, or resolving or reopening it
	sendComment = "pr-comment" // a comment on the pull request itself
	sendDecline = "pr-decline" // your name taken off its reviewers
)

// outgoing is one of them, as it's kept: which pull request, and what's
// said.
type outgoing struct {
	Repo   string          `json:"repo"`
	Number int             `json:"number"`
	Thread string          `json:"thread,omitempty"`
	What   review.Response `json:"what,omitempty"`
	Body   string          `json:"body,omitempty"`
}

func outgoingOf(it outbox.Item) (outgoing, bool) {
	var w outgoing
	return w, json.Unmarshal(it.Op, &w) == nil && w.Repo != "" && w.Number > 0
}

// laneOf is what a pull request's writes take turns on: itself.
func laneOf(repo string, number int) string { return "pr " + Key(repo, number) }

// SendBehind has what's said from a pull request's page sent from q,
// behind the answer.
func (s *Service) SendBehind(q *outbox.Queue) {
	s.Outbox = q
	q.Handle(sendThread, outbox.Kind{
		Send: s.send,
		// Resolving twice is resolving once; replying twice is two replies.
		Again: func(it outbox.Item) bool {
			w, ok := outgoingOf(it)
			return ok && w.What != review.Answer
		},
		Settled: s.sentBehind(func(repo string, number int, sent []outbox.Item) error {
			if _, err := s.ThreadsNow(repo, number); err != nil {
				s.keepShown(repo, number, sent)
				return err
			}
			return nil
		}),
	})
	q.Handle(sendComment, outbox.Kind{
		Send: s.send,
		Settled: s.sentBehind(func(repo string, number int, sent []outbox.Item) error {
			if _, err := s.DiscussionNow(repo, number); err != nil {
				s.keepShown(repo, number, sent)
				return err
			}
			return nil
		}),
	})
	q.Handle(sendDecline, outbox.Kind{
		Send:  s.send,
		Again: func(outbox.Item) bool { return true },
		Settled: s.sentBehind(func(string, int, []outbox.Item) error {
			s.Stale() // what it asks of you has changed
			return nil
		}),
	})
}

// send makes a write on GitHub.
func (s *Service) send(it outbox.Item) error {
	w, ok := outgoingOf(it)
	if !ok {
		return errors.New("what was to be sent can't be read")
	}
	return s.post(it.Kind, w)
}

// post makes a write on GitHub, now.
func (s *Service) post(kind string, w outgoing) error {
	if s.GitHub == nil {
		return errors.New("there's no GitHub to ask here")
	}
	owner, name := s.owner(w.Repo), s.name(w.Repo)
	switch kind {
	case sendThread:
		if w.What == review.Answer {
			return s.GitHub.ReplyToThread(owner, name, w.Number, w.Thread, w.Body)
		}
		return s.GitHub.ResolveThread(owner, name, w.Number, w.Thread, w.What == review.Resolve)
	case sendComment:
		return s.GitHub.Comment(owner, name, w.Number, w.Body)
	case sendDecline:
		me, err := s.GitHub.Viewer()
		if err != nil {
			return fmt.Errorf("asking GitHub who you are: %w", err)
		}
		return s.GitHub.DropReviewRequest(owner, name, w.Number, me)
	}
	return fmt.Errorf("ws doesn't know how to send %s", kind)
}

// sentBehind is what a kind is told once a pull request's writes of it have
// been sent: what those made changed is read again (read), and the page
// looks again, at that and at what failed.
func (s *Service) sentBehind(read func(repo string, number int, sent []outbox.Item) error) func(string, []outbox.Item, []outbox.Item) {
	return func(lane string, sent, failed []outbox.Item) {
		w, ok := outgoing{}, false
		for _, it := range append(slices.Clone(sent), failed...) {
			if w, ok = outgoingOf(it); ok {
				break
			}
		}
		if !ok {
			return
		}
		if len(sent) > 0 {
			if err := read(w.Repo, w.Number, sent); err != nil {
				// GitHub has them: what's kept shows them as made, until
				// it can be read again.
				s.mu.Lock()
				if s.pageDue == nil {
					s.pageDue = map[string]bool{}
				}
				s.pageDue[Key(w.Repo, w.Number)] = true
				s.mu.Unlock()
			}
		}
		s.bump(w.Repo, w.Number)
	}
}

// keepShown keeps a page with writes GitHub has taken shown on it, as
// they were while they waited: for when what they changed can't be read
// again just now.
func (s *Service) keepShown(repo string, number int, sent []outbox.Item) {
	pg := s.keptPage(repo, number)
	if pg == nil {
		return
	}
	s.keepSaid(repo, number, pg, func(x *page) {
		if x.Threads != nil {
			x.Threads = withThreadWrites(x.Threads, sent)
		}
		if x.Discussion != nil {
			x.Discussion = withComments(x.Discussion, sent, s.me())
		}
	})
}

// bump has a PR's open page look again: what's waiting for it, or what
// failed, has changed.
func (s *Service) bump(repo string, number int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pageSeen == nil {
		s.pageSeen = map[string]int64{}
	}
	s.pageSeen[Key(repo, number)]++
}

// say makes a write from a pull request's page: kept and sent behind,
// where there's somewhere to send from (and then it says true), or posted
// now.
func (s *Service) say(kind, doing string, w outgoing) (behind bool, err error) {
	if s.Outbox == nil {
		// Asked for, as a command finds it: GitHub is waited on anyway.
		// (Declining one doesn't ask: the list is asked for again after.)
		if kind != sendDecline {
			if _, _, err := s.Open(w.Repo, w.Number); err != nil {
				return false, err
			}
		}
		return false, s.post(kind, w)
	}
	// Found as it's known here: asking would be waiting on GitHub.
	if _, _, err := s.Known(w.Repo, w.Number); err != nil {
		return false, err
	}
	if _, err := s.Outbox.Add(laneOf(w.Repo, w.Number), kind, doing, w); err != nil {
		return false, err
	}
	s.bump(w.Repo, w.Number)
	return true, nil
}

// where is a thread's place in words (cache.go:3), as the page as kept
// has it: the pull request, for one it hasn't.
func (s *Service) where(repo string, number int, thread string) string {
	if pg := s.keptPage(repo, number); pg != nil && pg.Threads != nil {
		for _, n := range pg.Threads.Comments {
			if n.ID == thread && n.Path != "" {
				return fmt.Sprintf("%s:%d", n.Path, n.EndLine)
			}
		}
	}
	return Key(repo, number)
}

// waiting is what's kept to send for a PR, in the order it was made:
// nothing, where nothing's sent behind.
func (s *Service) waiting(repo string, number int) []outbox.Item {
	if s.Outbox == nil {
		return nil
	}
	items, _ := outbox.In(s.Root, laneOf(repo, number))
	return items
}

// withThreadWrites is a PR's threads with writes shown on them: a reply in
// its thread, a thread resolved or open again. One that failed isn't
// shown (the thread is as GitHub has it); one still on its way is marked
// so. One GitHub has taken, which the threads may have been read with
// since, isn't shown twice.
func withThreadWrites(t *review.Review, items []outbox.Item) *review.Review {
	out, copied := t, false
	for _, it := range items {
		w, ok := outgoingOf(it)
		if !ok || it.Kind != sendThread || it.State == outbox.Failed {
			continue
		}
		i := slices.IndexFunc(out.Comments, func(n review.Note) bool { return n.ID == w.Thread })
		if i < 0 {
			continue
		}
		if !copied {
			x := *t
			x.Comments = slices.Clone(t.Comments)
			out, copied = &x, true
		}
		n := out.Comments[i]
		switch w.What {
		case review.Answer:
			if it.State == outbox.Sent && slices.ContainsFunc(n.Thread, func(r review.Reply) bool { return r.By == "you" && strings.TrimSpace(r.Body) == w.Body }) {
				continue
			}
			n.Thread = append(slices.Clone(n.Thread), review.Reply{By: "you", At: it.Queued, Body: w.Body, Sending: it.Unsent()})
		case review.Resolve:
			n.Status, n.Sending = review.Resolved, it.Unsent()
		case review.Reopen:
			n.Status, n.Sending = review.Open, it.Unsent()
		}
		out.Comments[i] = n
	}
	return out
}

// withComments is what's been said on a PR with comments made from here
// under it, as withThreadWrites shows a thread's.
func withComments(d *Discussion, items []outbox.Item, me string) *Discussion {
	out, copied := d, false
	for _, it := range items {
		w, ok := outgoingOf(it)
		if !ok || it.Kind != sendComment || it.State == outbox.Failed {
			continue
		}
		body := Readable(w.Body)
		if it.State == outbox.Sent && slices.ContainsFunc(out.Items, func(x Said) bool { return x.Mine && x.Kind == SaidComment && x.Body == body }) {
			continue
		}
		if !copied {
			x := *d
			x.Items = slices.Clone(d.Items)
			out, copied = &x, true
		}
		out.Items = append(out.Items, Said{ID: fmt.Sprintf("unsent-%d", it.ID), By: me, Mine: true, Kind: SaidComment, Body: body,
			At: it.Queued.Format(time.RFC3339), Sending: it.Unsent()})
	}
	return out
}

// Unsent is something said from a pull request's page that GitHub didn't
// take: kept, to send again or let go of.
type Unsent struct {
	ID int64 `json:"id"`
	// Doing is it in words (your reply on cache.go:3), Body what it said,
	// and Error why GitHub didn't take it.
	Doing string    `json:"doing"`
	Body  string    `json:"body,omitempty"`
	Error string    `json:"error"`
	At    time.Time `json:"at"`
}

// Unsent is what was said from a PR's page that GitHub didn't take, oldest
// first.
func (s *Service) Unsent(repo string, number int) []Unsent {
	out := []Unsent{}
	for _, it := range s.waiting(repo, number) {
		if it.State != outbox.Failed {
			continue
		}
		w, _ := outgoingOf(it)
		out = append(out, Unsent{ID: it.ID, Doing: it.Doing, Body: w.Body, Error: it.Error, At: it.Queued})
	}
	return out
}

// Sends is how what's been said from pull requests' pages stands, across
// the workspace: how many are on their way to GitHub, how many it didn't
// take, and the first pull request with one of those (api#89).
type Sends struct {
	Sending int    `json:"sending,omitempty"`
	Failed  int    `json:"failed,omitempty"`
	At      string `json:"at,omitempty"`
}

// Sends is how what's been said from here stands. nil when there's
// nothing waiting and nothing failed.
func (s *Service) Sends() *Sends {
	if s.Outbox == nil {
		return nil
	}
	items, _ := outbox.In(s.Root, "")
	out := &Sends{}
	for _, it := range items {
		w, ok := outgoingOf(it)
		switch {
		case !ok || !strings.HasPrefix(it.Lane, "pr "):
		case it.Unsent():
			out.Sending++
		case it.State == outbox.Failed:
			if out.Failed++; out.At == "" {
				out.At = Key(w.Repo, w.Number)
			}
		}
	}
	if out.Sending == 0 && out.Failed == 0 {
		return nil
	}
	return out
}

// its is a write GitHub didn't take, if it's this PR's.
func (s *Service) its(repo string, number int, id int64) error {
	if s.Outbox == nil {
		return outbox.ErrNotFound
	}
	it, err := outbox.Get(s.Root, id)
	if err != nil {
		return err
	}
	if it.Lane != laneOf(repo, number) {
		return outbox.ErrNotFound
	}
	return nil
}

// Resend sends something GitHub didn't take again.
func (s *Service) Resend(repo string, number int, id int64) error {
	if err := s.its(repo, number, id); err != nil {
		return err
	}
	if err := s.Outbox.Retry(id); err != nil {
		return err
	}
	s.bump(repo, number)
	return nil
}

// Discard lets go of something GitHub didn't take: it isn't sent.
func (s *Service) Discard(repo string, number int, id int64) error {
	if err := s.its(repo, number, id); err != nil {
		return err
	}
	if err := s.Outbox.Drop(id); err != nil {
		return err
	}
	s.bump(repo, number)
	return nil
}
