package pulls

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/review"
)

// A pull request's page in the web UI is answered from what's kept of it:
// what GitHub says of it, its layer as fetched, what it says it does,
// GitHub's threads on its lines and what's been said on it, and its diffs.
// The first time it's shown, that's read (from GitHub, and its commits
// fetched): what it says it does and its diffs are answered as soon as
// they're in, and what's been said on it (its threads and discussion) is
// read behind them, waited for only by those who ask for it. Every time
// after, it's answered as kept at once, and read again behind the answer
// when it's stale (one read at a time for each PR). A read that finds something different moves the PR's count
// (PageSeen), which its open page follows, and reads again. What's kept
// is in the records too, for the daemon when it starts again, and it's
// forgotten once the PR is merged or closed.
//
// Detail, Diff, Threads and Discussion read afresh each time, for
// commands and agents; the Kept ones are the page's.

// page is a PR's page as it's kept.
type page struct {
	// At is when it was read.
	At    time.Time `json:"at"`
	PR    PR        `json:"pr"`
	Stack *Stack    `json:"stack,omitempty"`
	Layer keptLayer `json:"layer"`
	Body  string    `json:"body,omitempty"`
	// GitHubReviewed is your latest review on GitHub, its commit fetched.
	GitHubReviewed *Reviewed `json:"githubReviewed,omitempty"`
	// Since is what looking again shows, by the review it's since
	// (sinceOf).
	Since map[string]string `json:"since,omitempty"`
	// Threads and Discussion are what's been said, or, when it couldn't be
	// read and nothing was kept, why.
	Threads       *review.Review `json:"threads,omitempty"`
	ThreadsErr    string         `json:"threadsErr,omitempty"`
	Discussion    *Discussion    `json:"discussion,omitempty"`
	DiscussionErr string         `json:"discussionErr,omitempty"`
	// Diffs are its diffs as they've been asked for, by what they're of
	// (diffOf): kept in their own column. Changed only with the
	// service's lock held.
	Diffs map[string]*Diffed `json:"-"`
}

type pageRead struct {
	done chan struct{}
	pg   *page
	err  error
}

// unsaid: what's been said on it is still being read, behind its first
// showing. (Read, each is there, or why it isn't.)
func (pg *page) unsaid() bool {
	return (pg.Threads == nil && pg.ThreadsErr == "") || (pg.Discussion == nil && pg.DiscussionErr == "")
}

// sinceOf names a review for what looking again shows since it.
func sinceOf(rv *Reviewed) string { return rv.ID + " " + rv.Base + ".." + rv.Head }

// diffOf names a diff by what it's of: its scope and the commits it's
// between.
func diffOf(scope string, l keptLayer, rv *Reviewed, st *Stack) string {
	switch scope {
	case ScopeSince:
		return ScopeSince + " " + sinceOf(rv) + " " + l.Base + ".." + l.Head
	case ScopeWhole:
		heads := []string{ScopeWhole}
		if st != nil {
			for _, x := range st.Layers {
				heads = append(heads, x.Head)
			}
		}
		return strings.Join(heads, " ")
	}
	return ScopeLayer + " " + l.Base + ".." + l.Head
}

// keptPage is a PR's page as it's kept: in hand, or from the records. nil
// if there's none.
func (s *Service) keptPage(repo string, number int) *page {
	key := Key(repo, number)
	s.mu.Lock()
	pg := s.pages[key]
	s.mu.Unlock()
	if pg != nil {
		return pg
	}
	pg, _ = keptPage(s.Root, repo, number)
	if pg == nil || pg.unsaid() { // one kept without what's been said is read again
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if had := s.pages[key]; had != nil {
		return had
	}
	if s.pages == nil {
		s.pages = map[string]*page{}
	}
	s.pages[key] = pg
	return pg
}

// shown is a PR's page for showing: as kept, read again behind the answer
// if it's stale; or, when nothing's kept, read now.
func (s *Service) shown(repo string, number int) (*page, error) {
	if pg := s.keptPage(repo, number); pg != nil {
		s.freshen(repo, number, pg)
		return pg, nil
	}
	return s.readPage(repo, number)
}

// Freshen reads a PR's page again, behind, if what's kept is stale. A PR
// with nothing kept is left alone: it's read when it's first shown.
func (s *Service) Freshen(repo string, number int) {
	if pg := s.keptPage(repo, number); pg != nil {
		s.freshen(repo, number, pg)
	}
}

// readYours reads, behind, the page of each PR in l that's yours or asks
// for your review, where nothing's kept of it or what is is stale: those
// are the pages you'll open, and they're there when you do. Others are
// read when they're first shown. One at a time, as one look at a time of
// the list.
func (s *Service) readYours(l *List) {
	s.mu.Lock()
	if s.readingYours {
		s.mu.Unlock()
		return
	}
	s.readingYours = true
	s.mu.Unlock()
	go func() {
		defer func() {
			s.mu.Lock()
			s.readingYours = false
			s.mu.Unlock()
		}()
		for _, pr := range l.Yours() {
			if pg := s.keptPage(pr.Repo, pr.Number); pg != nil {
				if !s.pageStale(pg) {
					continue
				}
			}
			s.readPage(pr.Repo, pr.Number) //nolint:errcheck // shown when it's opened, and read again then
		}
	}()
}

// readingYoursNow is whether readYours is under way.
func (s *Service) readingYoursNow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readingYours
}

// PageSeen is how many times a PR's kept page has changed in this
// process: its open page reads again when it moves.
func (s *Service) PageSeen(repo string, number int) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pageSeen[Key(repo, number)]
}

// retryPage is how long after reading a page it isn't read again however
// stale it is (or Fresh, if that's sooner): one that can't be read isn't
// asked for at every look.
const retryPage = forcedFresh

func (s *Service) freshen(repo string, number int, pg *page) {
	key := Key(repo, number)
	retry := min(s.fresh(), retryPage)
	s.mu.Lock()
	busy := s.reading[key] != nil || s.now().Sub(s.pageTried[key]) < retry
	s.mu.Unlock()
	if busy || !s.pageStale(pg) {
		return
	}
	go s.readPage(repo, number)
}

// pageStale: what's kept of a page should be read again. It's been a
// while, or something here has changed what it asks of you since (Stale),
// or the list says something else of it than the page does.
func (s *Service) pageStale(pg *page) bool {
	s.mu.Lock()
	due := s.pageDue[Key(pg.PR.Repo, pg.PR.Number)]
	s.mu.Unlock()
	if due || s.now().Sub(pg.At) >= s.fresh() {
		return true
	}
	l := s.Peek()
	if l == nil {
		return false
	}
	pr, st := l.Find(pg.PR.Repo, pg.PR.Number)
	if pr == nil {
		return l.At.After(pg.At) // asked since, and it's gone
	}
	was, _ := json.Marshal(struct {
		PR    PR
		Stack *Stack
	}{pg.PR, pg.Stack})
	now, _ := json.Marshal(struct {
		PR    PR
		Stack *Stack
	}{*pr, st})
	return string(was) != string(now)
}

// readPage reads a PR's page, and keeps it: one read at a time for each
// PR, those who ask while it's being read waiting for it.
func (s *Service) readPage(repo string, number int) (*page, error) {
	key := Key(repo, number)
	s.mu.Lock()
	if c := s.reading[key]; c != nil {
		s.mu.Unlock()
		<-c.done
		return c.pg, c.err
	}
	c := &pageRead{done: make(chan struct{})}
	if s.reading == nil {
		s.reading = map[string]*pageRead{}
		s.pageTried = map[string]time.Time{}
	}
	s.reading[key] = c
	s.pageTried[key] = s.now()
	delete(s.pageDue, key)
	s.mu.Unlock()

	c.pg, c.err = s.read(repo, number)
	s.mu.Lock()
	delete(s.reading, key)
	s.mu.Unlock()
	close(c.done)
	return c.pg, c.err
}

// read is a PR's page read now: what GitHub says of it found in the list
// (as kept, the first time; asked for again if it's stale, after), its
// commits fetched if they've moved, and its threads, discussion and diffs
// read. One that's kept is answered when it can't be.
func (s *Service) read(repo string, number int) (*page, error) {
	was := s.keptPage(repo, number)
	var pr *PR
	var st *Stack
	if was == nil {
		// The first time it's shown, it's found in the list as it's kept:
		// showing it doesn't wait on GitHub being asked about every repo.
		// A stale list is asked for again behind, and the page read again
		// if it says something else of the PR (pageStale). One the list
		// hasn't is looked for.
		l, stale := s.peekStale()
		if pr, st = l.Find(repo, number); pr == nil {
			var err error
			if pr, st, err = s.Open(repo, number); err != nil {
				return nil, err
			}
		} else if stale {
			go s.List(false)
		}
	} else {
		l, err := s.List(false)
		pr, st = l.Find(repo, number)
		if pr == nil {
			if err != nil || l == nil || l.failed(repo) {
				return was, err // it can't be said whether it's still open
			}
			s.forgetPages(func(r string, n int) bool { return r == repo && n == number })
			return nil, fmt.Errorf("%w: %s isn't open now (merged or closed)", ErrNotOpen, Key(repo, number))
		}
	}

	// What GitHub says is asked for while its commits are fetched. The
	// first time it's shown, what's been said on it isn't waited for: it's
	// not what the page opens on, and can be the slowest of them.
	first := was == nil
	var wg, saying sync.WaitGroup
	var threads []github.ReviewThread
	var said []github.Said
	var threadsErr, saidErr error
	var body string
	saying.Add(2)
	go func() {
		defer saying.Done()
		threads, threadsErr = s.GitHub.ReviewThreads(s.owner(repo), s.name(repo), number)
	}()
	go func() { defer saying.Done(); said, saidErr = s.GitHub.Discussion(s.owner(repo), s.name(repo), number) }()
	wg.Go(func() { ; body = s.body(pr) })
	l, err := s.layer(pr, st)
	wg.Wait()
	if !first {
		saying.Wait()
	}
	if err != nil {
		if was != nil {
			return was, err
		}
		return nil, err
	}
	body = Readable(body)
	if body == "" && was != nil {
		body = was.Body // one that couldn't be read leaves what was
	}
	rec := s.seen(pr, l, body)
	me := s.me()

	pg := &page{At: s.now(), PR: *pr, Stack: st, Layer: l.kept(), Body: body, Since: map[string]string{}, Diffs: map[string]*Diffed{}}
	pg.GitHubReviewed = s.githubReviewed(pr, l)
	for _, rv := range []*Reviewed{rec.Reviewed, pg.GitHubReviewed} {
		if rv != nil {
			pg.Since[sinceOf(rv)] = sinceMode(l, rv)
		}
	}
	// What's been said, once GitHub has answered for it.
	withSaid := func(pg *page) {
		switch {
		case threadsErr == nil:
			pg.Threads = threadsReview(repo, number, threads, l, me)
		case was != nil && was.Threads != nil:
			pg.Threads = was.Threads // as last read, rather than nothing
		default:
			pg.ThreadsErr = threadsErr.Error()
		}
		switch {
		case saidErr == nil:
			pg.Discussion = discussion(said, me, pr.Head)
		case was != nil && was.Discussion != nil:
			pg.Discussion = was.Discussion
		default:
			pg.DiscussionErr = saidErr.Error()
		}
	}
	if !first {
		withSaid(pg)
	}

	// Its diffs: its own changes, what's changed since your review, and
	// the whole stack's if that's been asked for before.
	if d, err := layerDiff(l); err == nil {
		pg.Diffs[diffOf(ScopeLayer, pg.Layer, nil, st)] = d
	}
	if rv := latest(rec.Reviewed, pg.GitHubReviewed); rv != nil {
		if since := pg.Since[sinceOf(rv)]; since != "" {
			if d, err := sinceDiff(pr, l, rv, since); err == nil {
				pg.Diffs[diffOf(ScopeSince, pg.Layer, rv, st)] = d
			}
		}
	}
	if st != nil && was != nil && s.hadWhole(was) {
		if d, err := s.whole(pr, st); err == nil {
			pg.Diffs[diffOf(ScopeWhole, pg.Layer, nil, st)] = d
		}
	}
	if !first {
		s.keepRead(repo, number, was, pg)
		return pg, nil
	}
	// Shown without what's been said, which is kept when it's in: unless
	// the page has been read again by then, with its own.
	key := Key(repo, number)
	c := make(chan struct{})
	s.mu.Lock()
	if s.saying == nil {
		s.saying = map[string]chan struct{}{}
	}
	s.saying[key] = c
	s.mu.Unlock()
	s.keepRead(repo, number, nil, pg)
	go func() {
		saying.Wait()
		s.mu.Lock()
		still := s.pages[key] == pg
		s.mu.Unlock()
		if still {
			s.keepSaid(repo, number, pg, withSaid)
		}
		s.mu.Lock()
		delete(s.saying, key)
		s.mu.Unlock()
		close(c)
	}()
	return pg, nil
}

// said is a PR's page with what's been said on it: the page itself,
// unless that's still being read behind its first showing, which is
// waited for.
func (s *Service) said(repo string, number int, pg *page) *page {
	if !pg.unsaid() {
		return pg
	}
	key := Key(repo, number)
	s.mu.Lock()
	c := s.saying[key]
	s.mu.Unlock()
	if c != nil {
		<-c
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if now := s.pages[key]; now != nil {
		return now
	}
	return pg
}

// errUnsaid answers for what's been said on a page that was kept without
// it and hasn't been read again yet.
var errUnsaid = errors.New("what's been said on it hasn't been read yet")

func (s *Service) hadWhole(pg *page) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range pg.Diffs {
		if strings.HasPrefix(k, ScopeWhole+" ") {
			return true
		}
	}
	return false
}

// keepRead keeps a page just read in place of what was: in the records
// too, when it's changed, and then its count moves.
func (s *Service) keepRead(repo string, number int, was, pg *page) {
	key := Key(repo, number)
	changed := was == nil || !samePage(s, was, pg)
	s.mu.Lock()
	if s.pages == nil {
		s.pages = map[string]*page{}
	}
	if s.pageSeen == nil {
		s.pageSeen = map[string]int64{}
	}
	s.pages[key] = pg
	if changed {
		s.pageSeen[key]++
	}
	s.mu.Unlock()
	// (One still without what's been said is kept in the records with it.)
	if changed && !pg.unsaid() {
		s.persist(repo, number, pg)
	}
}

// persist writes a page to the records. One that can't be written is
// still the page.
func (s *Service) persist(repo string, number int, pg *page) {
	s.mu.Lock()
	diffs := make(map[string]*Diffed, len(pg.Diffs))
	maps.Copy(diffs, pg.Diffs)
	s.mu.Unlock()
	keepPage(s.Root, repo, number, pg, diffs)
}

// samePage: two reads of a page found the same, whenever they were.
func samePage(s *Service, a, b *page) bool {
	of := func(pg *page) string {
		x := *pg
		x.At = time.Time{}
		s.mu.Lock()
		diffs, _ := json.Marshal(pg.Diffs)
		s.mu.Unlock()
		data, _ := json.Marshal(x)
		return string(data) + string(diffs)
	}
	return of(a) == of(b)
}

// forgetPages forgets the pages of PRs gone says are gone, in hand and in
// the records, and moves their counts: an open page reads again, and
// finds it isn't open.
func (s *Service) forgetPages(gone func(repo string, number int) bool) {
	dropped, _ := dropPages(s.Root, gone)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pageSeen == nil {
		s.pageSeen = map[string]int64{}
	}
	for key, pg := range s.pages {
		if gone(pg.PR.Repo, pg.PR.Number) {
			delete(s.pages, key)
			s.pageSeen[key]++
		}
	}
	for _, k := range dropped {
		if key := Key(k.Repo, k.Number); s.pages[key] == nil {
			s.pageSeen[key]++
		}
	}
}

// forgetGone forgets the pages of PRs a list just asked for doesn't have,
// in the repos it could ask about: they've been merged or closed.
func (s *Service) forgetGone(l *List) {
	repos := map[string]bool{}
	for _, r := range s.Repos() {
		repos[r] = !l.failed(r)
	}
	s.forgetPages(func(repo string, number int) bool {
		pr, _ := l.Find(repo, number)
		return pr == nil && repos[repo]
	})
}

// failed: GitHub couldn't be asked about a repo's PRs for the list.
func (l *List) failed(repo string) bool {
	if l == nil {
		return true
	}
	for _, p := range l.Problems {
		if strings.HasPrefix(p, repo+": ") {
			return true
		}
	}
	return false
}

// The page's reads.

// KeptDetail is a PR in full, as its page shows it: as kept (read now if
// nothing is), with your review and where it's docked as they are here.
func (s *Service) KeptDetail(repo string, number int) (*Detail, error) {
	pg, err := s.shown(repo, number)
	if err != nil {
		return nil, err
	}
	rec, err := Load(s.Root, repo, number)
	if err != nil || rec == nil {
		rec = &Record{}
	}
	d := newDetail(&pg.PR, pg.Stack, pg.Layer, pg.Body)
	if rv := latest(rec.Reviewed, pg.GitHubReviewed); rv != nil {
		withReview(d, rv, s.keptSince(pg, rv))
	}
	s.withDocked(d, &pg.PR, pg.Stack, rec)
	return d, nil
}

// keptSince is what looking again at a kept page shows since a review: as
// kept, or worked out from the clone for a review the page wasn't read
// with.
func (s *Service) keptSince(pg *page, rv *Reviewed) string {
	if rv.Head == pg.Layer.Head {
		return ""
	}
	if since, ok := pg.Since[sinceOf(rv)]; ok {
		return since
	}
	return sinceMode(pg.Layer.in(s.BareDir(pg.PR.Repo)), rv)
}

// KeptDiff is a PR's diff as its page shows it: as kept, or read from the
// clone (as its page was read) and kept, the first time it's asked for.
func (s *Service) KeptDiff(repo string, number int, scope string) (*Diffed, error) {
	pg, err := s.shown(repo, number)
	if err != nil {
		return nil, err
	}
	var rv *Reviewed
	var since string
	switch scope {
	case ScopeWhole:
		if pg.Stack == nil || len(pg.Stack.Layers) == 0 {
			return nil, fmt.Errorf("%s isn't in a stack", pg.PR.Ref())
		}
	case ScopeSince:
		rec, err := Load(s.Root, repo, number)
		if err != nil {
			return nil, err
		}
		if rv = latest(rec.Reviewed, pg.GitHubReviewed); rv == nil {
			return nil, fmt.Errorf("you haven't reviewed %s yet", pg.PR.Ref())
		}
		if since = s.keptSince(pg, rv); since == "" {
			return nil, fmt.Errorf("nothing has changed in %s since your review", pg.PR.Ref())
		}
	default:
		scope = ScopeLayer
	}
	key := diffOf(scope, pg.Layer, rv, pg.Stack)
	s.mu.Lock()
	d := pg.Diffs[key]
	s.mu.Unlock()
	if d != nil {
		return d, nil
	}
	l := pg.Layer.in(s.BareDir(repo))
	switch scope {
	case ScopeWhole:
		d, err = s.whole(&pg.PR, pg.Stack)
	case ScopeSince:
		d, err = sinceDiff(&pg.PR, l, rv, since)
	default:
		d, err = layerDiff(l)
	}
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	pg.Diffs[key] = d
	s.mu.Unlock()
	s.persist(repo, number, pg)
	return d, nil
}

// KeptThreads are GitHub's threads on a PR's lines as its page shows
// them: as kept, waited for while they're read behind its first showing,
// with what's been said from here and not yet sent on them.
func (s *Service) KeptThreads(repo string, number int) (*review.Review, error) {
	pg, err := s.shown(repo, number)
	if err != nil {
		return nil, err
	}
	if pg = s.said(repo, number, pg); pg.Threads == nil {
		if pg.ThreadsErr == "" {
			return nil, errUnsaid
		}
		return nil, errors.New(pg.ThreadsErr)
	}
	return withThreadWrites(pg.Threads, s.waiting(repo, number)), nil
}

// KeptDiscussion is what's been said on a PR as its page shows it: as
// kept, waited for while it's read behind its first showing, with
// comments made from here and not yet sent under it.
func (s *Service) KeptDiscussion(repo string, number int) (*Discussion, error) {
	pg, err := s.shown(repo, number)
	if err != nil {
		return nil, err
	}
	if pg = s.said(repo, number, pg); pg.Discussion == nil {
		if pg.DiscussionErr == "" {
			return nil, errUnsaid
		}
		return nil, errors.New(pg.DiscussionErr)
	}
	return withComments(pg.Discussion, s.waiting(repo, number), s.me()), nil
}

// ThreadsNow reads GitHub's threads on a PR again, now, and keeps them:
// after you've replied to one, or resolved it.
func (s *Service) ThreadsNow(repo string, number int) (*review.Review, error) {
	pg, err := s.shown(repo, number)
	if err != nil {
		return nil, err
	}
	pg = s.said(repo, number, pg)
	threads, err := s.GitHub.ReviewThreads(s.owner(repo), s.name(repo), number)
	if err != nil {
		return nil, err
	}
	t := threadsReview(repo, number, threads, pg.Layer.in(s.BareDir(repo)), s.me())
	s.keepSaid(repo, number, pg, func(x *page) { x.Threads, x.ThreadsErr = t, "" })
	return withThreadWrites(t, s.waiting(repo, number)), nil
}

// DiscussionNow reads what's been said on a PR again, now, and keeps it:
// after you've said something.
func (s *Service) DiscussionNow(repo string, number int) (*Discussion, error) {
	pg, err := s.shown(repo, number)
	if err != nil {
		return nil, err
	}
	pg = s.said(repo, number, pg)
	said, err := s.GitHub.Discussion(s.owner(repo), s.name(repo), number)
	if err != nil {
		return nil, err
	}
	d := discussion(said, s.me(), pg.PR.Head)
	s.keepSaid(repo, number, pg, func(x *page) { x.Discussion, x.DiscussionErr = d, "" })
	return withComments(d, s.waiting(repo, number), s.me()), nil
}

// keepSaid keeps a page with what's been said read again: a copy, as a
// page in hand isn't changed.
func (s *Service) keepSaid(repo string, number int, pg *page, change func(*page)) {
	x := *pg
	change(&x)
	s.mu.Lock()
	x.Diffs = make(map[string]*Diffed, len(pg.Diffs))
	maps.Copy(x.Diffs, pg.Diffs)
	s.mu.Unlock()
	s.keepRead(repo, number, pg, &x)
}
