package pulls

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// A pull request that asks for your review by name can't move without
// you, so it's among what needs you (ops.Needs) whether or not a Reviews
// page is open. The list is what says so, and asking GitHub for it is a
// request for every repo: so the daemon looks with one search of each
// owner instead (Look), and asks for the list only when the search and
// the list disagree.

// Asked is a row of the list that asks for your review by name: a pull
// request, or a stack with a layer that does.
type Asked struct {
	// Key is the row's.
	Key string
	// PR is the one to open: a stack's lowest layer that asks you.
	PR PR
	// Stack is the stack it's a layer of, and More how many layers above
	// it ask you too.
	Stack *Stack
	More  int
}

// AskedOfYou is what asks for your review by name, as GitHub last said
// and in the list's order: not a draft, not one you've reviewed since it
// was asked, and not one dealt with here since the list was asked for (a
// review posted, your name taken off), which GitHub would say next time.
func (s *Service) AskedOfYou() []Asked {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.kept()
	if l == nil {
		return nil
	}
	asks := func(p PR) bool {
		at, dealt := s.settled[Key(p.Repo, p.Number)]
		return p.State == NeedsReview && p.Asked == "you" && !(dealt && at.After(l.At))
	}
	var out []Asked
	for _, sec := range l.Sections {
		for _, r := range sec.Rows {
			switch {
			case r.PR != nil && asks(*r.PR):
				out = append(out, Asked{Key: r.Key, PR: *r.PR})
			case r.Stack != nil:
				var a *Asked
				for _, p := range r.Stack.Layers {
					switch {
					case !asks(p):
					case a == nil:
						a = &Asked{Key: r.Key, PR: p, Stack: r.Stack}
					default:
						a.More++
					}
				}
				if a != nil {
					out = append(out, *a)
				}
			}
		}
	}
	return out
}

// settle notes that a PR was dealt with here, now: it doesn't ask for
// your review, though the list as it's kept still says it does.
func (s *Service) settle(repo string, number int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled == nil {
		s.settled = map[string]time.Time{}
	}
	// Those the list has caught up with are done with.
	if l := s.list; l != nil {
		for key, at := range s.settled {
			if !at.After(l.At) {
				delete(s.settled, key)
			}
		}
	}
	s.settled[Key(repo, number)] = s.now()
	s.moved++
}

// Seen moves whenever what's listed may have changed: a list asked for, a
// newer one another process kept, a PR dealt with here. What's built from
// the list (the workspace's view) is built again when it has.
func (s *Service) Seen() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moved
}

// Due says when the list has gone stale by something done here (Stale):
// whoever follows the list looks again then, not at its next turn.
func (s *Service) Due() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.due == nil {
		s.due = make(chan struct{}, 1)
	}
	return s.due
}

// Look keeps what asks for your review by name current, for the price of
// one search of each owner: the list is asked for only when the search's
// PRs aren't the list's, or something here has made it stale. asked is
// whether it was.
//
// GitHub's search can be behind its pull requests, or the two can
// disagree for good (a repo that couldn't be listed): a search that says
// what it said when this list was asked for doesn't ask again.
func (s *Service) Look() (asked bool, err error) {
	if s.GitHub == nil {
		return false, nil
	}
	s.mu.Lock()
	stale := s.stale && s.kept() != nil
	s.mu.Unlock()
	if stale {
		_, err := s.List(false)
		return true, err
	}
	found, err := s.searched(s.Repos(), s.GitHub.AskedByName)
	if err != nil {
		return false, err // part of an answer would read as PRs gone
	}
	want := make([]string, 0, len(found))
	for _, a := range found {
		want = append(want, fmt.Sprintf("%s#%d", a.Repo, a.Number))
	}
	slices.Sort(want)
	want = slices.Compact(want)

	s.mu.Lock()
	held := s.kept()
	same := slices.Equal(want, byName(held)) || (s.looked && slices.Equal(want, s.lookedFor))
	s.mu.Unlock()
	if same {
		return false, nil
	}
	l, err := s.List(true)
	if l == held {
		return false, err // asked for a moment ago, or it couldn't be: the next look tries again
	}
	s.mu.Lock()
	s.looked, s.lookedFor = true, want
	s.mu.Unlock()
	return true, err
}

// byName is the list's PRs that ask you by name, drafts left out, as the
// search names them: what a look's search is held against.
func byName(l *List) []string {
	out := []string{}
	if l == nil {
		return out
	}
	add := func(p PR) {
		if p.Asked == "you" && !p.Draft {
			out = append(out, p.Ref())
		}
	}
	for _, sec := range l.Sections {
		for _, r := range sec.Rows {
			if r.PR != nil {
				add(*r.PR)
			}
			if r.Stack != nil {
				for _, p := range r.Stack.Layers {
					add(p)
				}
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Decline takes your name off a pull request's reviewers on GitHub: it
// isn't yours to review, and nothing here can say so but GitHub. It no
// longer asks for your review from now, before GitHub's asked again.
//
// GitHub's told behind, where there's somewhere to send from: if it
// refuses, that's said on the PR's page (Unsent), and the PR asks for your
// review again the next time GitHub's asked.
func (s *Service) Decline(repo string, number int) error {
	if s.GitHub == nil {
		return errors.New("not connected to GitHub")
	}
	behind, err := s.say(sendDecline, "removing you from "+Key(repo, number)+"'s reviewers", outgoing{Repo: repo, Number: number})
	if err != nil {
		return err
	}
	s.settle(repo, number)
	if !behind {
		s.Stale()
	}
	return nil
}
