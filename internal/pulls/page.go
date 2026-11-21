package pulls

import (
	"encoding/json"
	"errors"
	"fmt"
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
// fetched); every time after, it's answered as kept at once, and read
// again behind the answer when it's stale (one read at a time for each
// PR). A read that finds something different moves the PR's count
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
	if pg == nil {
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

// read is a PR's page read now: GitHub asked what it says of it (the list,
// if it's stale), its commits fetched if they've moved, and its threads,
// discussion and diffs read. One that's kept is answered when it can't be.
func (s *Service) read(repo string, number int) (*page, error) {
	was := s.keptPage(repo, number)
	var pr *PR
	var st *Stack
	if was == nil {
		var err error
		if pr, st, err = s.Open(repo, number); err != nil {
			return nil, err
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

	// What GitHub says is asked for while its commits are fetched.
	var wg sync.WaitGroup
	var threads []github.ReviewThread
	var said []github.Said
	var threadsErr, saidErr error
	var body string
	wg.Add(3)
	go func() {
		defer wg.Done()
		threads, threadsErr = s.GitHub.ReviewThreads(s.owner(repo), s.name(repo), number)
	}()
	go func() { defer wg.Done(); said, saidErr = s.GitHub.Discussion(s.owner(repo), s.name(repo), number) }()
	go func() { defer wg.Done(); body = s.body(pr) }()
	l, err := s.layer(pr, st)
	wg.Wait()
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
	s.keepRead(repo, number, was, pg)
	return pg, nil
}

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
	if changed {
		s.persist(repo, number, pg)
	}
}

// persist writes a page to the records. One that can't be written is
// still the page.
func (s *Service) persist(repo string, number int, pg *page) {
	s.mu.Lock()
	diffs := make(map[string]*Diffed, len(pg.Diffs))
	for k, d := range pg.Diffs {
		diffs[k] = d
	}
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
// them: as kept.
func (s *Service) KeptThreads(repo string, number int) (*review.Review, error) {
	pg, err := s.shown(repo, number)
	if err != nil {
		return nil, err
	}
	if pg.Threads == nil {
		return nil, errors.New(pg.ThreadsErr)
	}
	return pg.Threads, nil
}

// KeptDiscussion is what's been said on a PR as its page shows it: as
// kept.
func (s *Service) KeptDiscussion(repo string, number int) (*Discussion, error) {
	pg, err := s.shown(repo, number)
	if err != nil {
		return nil, err
	}
	if pg.Discussion == nil {
		return nil, errors.New(pg.DiscussionErr)
	}
	return pg.Discussion, nil
}

// ThreadsNow reads GitHub's threads on a PR again, now, and keeps them:
// after you've replied to one, or resolved it.
func (s *Service) ThreadsNow(repo string, number int) (*review.Review, error) {
	pg, err := s.shown(repo, number)
	if err != nil {
		return nil, err
	}
	threads, err := s.GitHub.ReviewThreads(s.owner(repo), s.name(repo), number)
	if err != nil {
		return nil, err
	}
	t := threadsReview(repo, number, threads, pg.Layer.in(s.BareDir(repo)), s.me())
	s.keepSaid(repo, number, pg, func(x *page) { x.Threads, x.ThreadsErr = t, "" })
	return t, nil
}

// DiscussionNow reads what's been said on a PR again, now, and keeps it:
// after you've said something.
func (s *Service) DiscussionNow(repo string, number int) (*Discussion, error) {
	pg, err := s.shown(repo, number)
	if err != nil {
		return nil, err
	}
	said, err := s.GitHub.Discussion(s.owner(repo), s.name(repo), number)
	if err != nil {
		return nil, err
	}
	d := discussion(said, s.me(), pg.PR.Head)
	s.keepSaid(repo, number, pg, func(x *page) { x.Discussion, x.DiscussionErr = d, "" })
	return d, nil
}

// keepSaid keeps a page with what's been said read again: a copy, as a
// page in hand isn't changed.
func (s *Service) keepSaid(repo string, number int, pg *page, change func(*page)) {
	x := *pg
	change(&x)
	s.mu.Lock()
	x.Diffs = make(map[string]*Diffed, len(pg.Diffs))
	for k, d := range pg.Diffs {
		x.Diffs[k] = d
	}
	s.mu.Unlock()
	s.keepRead(repo, number, pg, &x)
}
