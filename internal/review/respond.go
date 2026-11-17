package review

import (
	"errors"
	"fmt"
	"github.com/withworkspace/workspace/internal/store"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

// After a review is submitted, its comments are a conversation: crew say
// what they did about each (addressed, with the commit, or won't fix, with
// why) or answer it, and you resolve each once you're satisfied, or reopen
// it. Crew can't resolve: that's yours.

// Comment states.
const (
	Open      = "open"
	Addressed = "addressed"
	WontFix   = "wontfix"
	Resolved  = "resolved"
)

// Response is what's said on a comment.
type Response string

const (
	Answer  Response = "reply"     // anyone: words, no change of state
	Address Response = "addressed" // crew: it's dealt with (in Commit)
	Decline Response = "wontfix"   // crew: it won't be, and why
	Resolve Response = "resolve"   // you: done with it
	Reopen  Response = "reopen"    // you: not done after all
	byYou            = "you"
)

// ErrNotYours is the answer for a response only the user may give.
var ErrNotYours = errors.New("only the user resolves or reopens a comment")

// FindNote finds a comment in a submitted review by its id (r1.c2).
func (s Store) FindNote(id string) (r *Review, i int, err error) {
	err = s.read(func(tx *store.Tx) error {
		r, i, err = s.findNoteIn(tx, id)
		return err
	})
	return r, i, err
}

func (s Store) findNoteIn(tx *store.Tx, id string) (*Review, int, error) {
	rid, _, ok := strings.Cut(id, ".")
	if !ok || !idShape.MatchString(rid) {
		return nil, 0, fmt.Errorf("%w: %q isn't a comment id (they look like r1.c2)", ErrNotFound, id)
	}
	r, err := s.loadIn(tx, rid)
	if err != nil {
		return nil, 0, err
	}
	if r.State != Submitted {
		return nil, 0, fmt.Errorf("%w: %s is still a draft", ErrNotFound, rid)
	}
	for i := range r.Comments {
		if r.Comments[i].ID == id {
			return r, i, nil
		}
	}
	return nil, 0, fmt.Errorf("%w: review %s has no comment %s", ErrNotFound, rid, id)
}

// Respond adds to a comment's thread, by crew (their id) or "you", and
// changes its state as the response says. The review is rendered again for
// agents and the response logged.
func (s Store) Respond(id, by string, what Response, body, commit string, at time.Time) (*Review, error) {
	body = strings.TrimSpace(body)
	switch what {
	case Answer:
		if body == "" {
			return nil, errors.New("say something")
		}
	case Decline:
		if body == "" {
			return nil, errors.New("say why it won't be done")
		}
	case Resolve, Reopen:
		if by != byYou {
			return nil, ErrNotYours
		}
	case Address:
	default:
		return nil, fmt.Errorf("can't %q a comment", what)
	}
	var r *Review
	var n Note
	err := s.write(func(tx *store.Tx) error {
		var i int
		var err error
		if r, i, err = s.findNoteIn(tx, id); err != nil {
			return err
		}
		note := &r.Comments[i]
		reply := Reply{By: by, At: at, Body: body, Commit: commit}
		switch what {
		case Address:
			note.Status, reply.Status = Addressed, Addressed
		case Decline:
			note.Status, reply.Status = WontFix, WontFix
		case Resolve:
			note.Status, reply.Status = Resolved, Resolved
		case Reopen:
			note.Status, reply.Status = Open, Open
		}
		note.Thread = append(note.Thread, reply)
		n = *note
		if err := s.saveIn(tx, r); err != nil {
			return err
		}
		s.render(tx, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !s.ofMission() {
		return r, nil
	}
	repo, name, _ := strings.Cut(r.Capsule, "/")
	text := n.ID + " " + responseWords(what, commit)
	if body != "" {
		text += ": " + oneLine(body)
	}
	crew := by
	if by == byYou {
		crew = ""
	}
	return r, s.Missions.AppendLog(s.Slug, mission.Event{Time: at, Kind: mission.EventReviewReply, Repo: repo, Capsule: name, Crew: crew, Text: text})
}

func responseWords(what Response, commit string) string {
	switch what {
	case Address:
		if commit != "" {
			return "addressed in " + short(commit)
		}
		return "addressed"
	case Decline:
		return "won't be done"
	case Resolve:
		return "resolved"
	case Reopen:
		return "reopened"
	}
	return "answered"
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// Tally is how a review's comments stand.
type Tally struct {
	Open, Addressed, WontFix, Resolved int
}

// Answered is how many have had an answer from crew or been settled.
func (t Tally) Answered() int { return t.Addressed + t.WontFix + t.Resolved }

// Total is how many comments there are.
func (t Tally) Total() int { return t.Open + t.Answered() }

// Tally counts the review's comments by state.
func (r *Review) Tally() Tally {
	var t Tally
	for _, n := range r.Comments {
		switch n.Status {
		case Addressed:
			t.Addressed++
		case WontFix:
			t.WontFix++
		case Resolved:
			t.Resolved++
		default:
			t.Open++
		}
	}
	return t
}

// Blocking is the capsules' latest reviews that asked for changes: their
// stage can't be accepted until the user re-reviews and approves.
func (s Store) Blocking(capsules []string) ([]*Review, error) {
	var out []*Review
	for _, c := range capsules {
		r, err := s.Latest(c)
		if err != nil {
			return nil, err
		}
		if r != nil && r.Verdict == Changes {
			out = append(out, r)
		}
	}
	return out, nil
}

// ReReview is a capsule ready for the user to look at again.
type ReReview struct {
	Capsule string
	Review  *Review // the review that asked for changes
	Head    string  // where the capsule is now
}

// AwaitingReReview is the mission's capsules whose latest review asked for
// changes and that have moved on since, with no crew still at it: the work
// is back with the user. head is a capsule's commit now ("" if gone).
func (s Store) AwaitingReReview(m *mission.Mission, head func(capsule string) string) ([]ReReview, error) {
	var out []ReReview
	for _, c := range m.Capsules {
		capsule := c.Repo + "/" + c.Name
		r, err := s.Latest(capsule)
		if err != nil {
			return nil, err
		}
		if r == nil || r.Verdict != Changes {
			continue
		}
		now := head(capsule)
		if now == "" || now == r.Head || crewAt(m, capsule) {
			continue
		}
		out = append(out, ReReview{Capsule: capsule, Review: r, Head: now})
	}
	return out, nil
}

// crewAt says whether crew are still at work in a capsule.
func crewAt(m *mission.Mission, capsule string) bool {
	for _, c := range m.Crew {
		if c.Capsule == capsule && c.Role == "" && c.Ended == nil && c.Status != mission.CrewDone {
			return true
		}
	}
	return false
}
