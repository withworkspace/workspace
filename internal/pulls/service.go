package pulls

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/review"
)

// Service is the Reviews page's: it asks GitHub for the list (and keeps
// what it said between asks), fetches a PR's commits into its repo's
// clone, reads its diffs from there, and posts your reviews.
type Service struct {
	Root string
	Org  string
	// GitHub is asked for PRs, stacks and threads, and posts reviews.
	GitHub github.Client
	// Repos are the repos whose PRs are listed: those cloned here.
	Repos func() []string
	// BareDir is a repo's clone, where a PR's commits are fetched to.
	BareDir func(repo string) string
	// Local is what the workspace knows that GitHub doesn't: missions'
	// branches, and the capsules checked out. nil for nothing.
	Local func() Local
	// Now is the clock (tests set it), and Fresh how long a list answers
	// before GitHub is asked again (default a minute).
	Now   func() time.Time
	Fresh time.Duration

	mu   sync.Mutex
	list *List
	// listAt is when the list held was asked for, as the records keep it:
	// another process's newer one is read in its place.
	listAt string
	// stale: something has changed what a PR asks of you since GitHub was
	// last asked (a review posted, a PR docked), so it's asked again next
	// time, however lately that was.
	stale   bool
	loading *loadCall
	bodies  map[string]body
	repoMu  map[string]*sync.Mutex

	// The pages of PRs as they're kept (page.go), by Key: how many times
	// each has changed, which are being read, when each was last, and
	// which are due to be read again whenever they were (Stale).
	pages     map[string]*page
	pageSeen  map[string]int64
	reading   map[string]*pageRead
	pageTried map[string]time.Time
	pageDue   map[string]bool
}

type loadCall struct {
	done chan struct{}
	list *List
	err  error
}

type body struct {
	updated string
	text    string
}

// ErrNotOpen is the answer for a PR that isn't among the open ones.
var ErrNotOpen = errors.New("not an open pull request")

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// fresh is how long a list, or a PR's page, answers before GitHub is
// asked again.
func (s *Service) fresh() time.Duration {
	if s.Fresh == 0 {
		return time.Minute
	}
	return s.Fresh
}

// Peek is the list as GitHub last said it, without asking: nil if it's
// never been asked.
func (s *Service) Peek() *List {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kept()
}

// kept is the list in hand, or the records' when they hold a newer one
// (another process asked GitHub since: a command, or the daemon). Called
// with the lock held.
func (s *Service) kept() *List {
	if l, at, err := keptList(s.Root, s.listAt); err == nil && l != nil {
		s.list, s.listAt = l, at
	}
	return s.list
}

// forcedFresh is how long a list answers even when asked to ask again.
const forcedFresh = 5 * time.Second

// List is the open PRs across the workspace's repos. GitHub is asked if
// it hasn't been lately (or, with force, in the last few seconds); when it
// can't be, the list as last known comes back with the error.
func (s *Service) List(force bool) (*List, error) {
	fresh := s.fresh()
	if force {
		fresh = forcedFresh
	}
	s.mu.Lock()
	if l := s.kept(); l != nil && !s.stale && s.now().Sub(l.At) < fresh {
		s.mu.Unlock()
		return l, nil
	}
	if c := s.loading; c != nil {
		s.mu.Unlock()
		<-c.done
		return c.list, c.err
	}
	c := &loadCall{done: make(chan struct{})}
	s.loading = c
	s.mu.Unlock()

	l, err := s.load()
	s.mu.Lock()
	s.loading = nil
	if err == nil {
		s.list, s.stale = l, false
		// Kept for the next process. One that can't be kept is still
		// the list.
		if at, kerr := keepList(s.Root, l); kerr == nil {
			s.listAt = at
		}
	}
	c.list, c.err = s.list, err
	s.mu.Unlock()
	if err == nil {
		s.forgetGone(l)
	}
	close(c.done)
	return c.list, c.err
}

// Stale marks the list as needing GitHub asked again: after a review is
// posted, what a PR asks of you has changed. The pages kept are read again
// too, when they're next looked at.
func (s *Service) Stale() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stale = true
	if s.pageDue == nil {
		s.pageDue = map[string]bool{}
	}
	for key := range s.pages {
		s.pageDue[key] = true
	}
}

// askAtOnce is how many repos GitHub is asked about at a time.
const askAtOnce = 6

func (s *Service) load() (*List, error) {
	if s.GitHub == nil {
		return nil, errors.New("there's no GitHub to ask here")
	}
	me, err := s.GitHub.Viewer()
	if err != nil {
		return nil, fmt.Errorf("asking GitHub who you are: %w", err)
	}
	repos := s.Repos()
	data := make([]repoPRs, len(repos))
	problems := make([]string, len(repos))
	var wg sync.WaitGroup
	sem := make(chan struct{}, askAtOnce)
	for i, repo := range repos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			data[i].repo = repo
			prs, err := s.GitHub.OpenPRs(s.Org, repo)
			if err != nil {
				problems[i] = repo + ": " + err.Error()
				return
			}
			data[i].prs = prs
			// Stacks are new on GitHub: without them, the chains ws works
			// out still show.
			if stacks, err := s.GitHub.StacksForRepo(s.Org, repo); err == nil {
				data[i].stacks = stacks
			}
		}()
	}
	var asked []github.PRRef
	var askedErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		asked, askedErr = s.GitHub.ReviewRequested(s.Org)
	}()
	wg.Wait()

	var local Local
	if s.Local != nil {
		local = s.Local()
	}
	l := build(me, data, asked, local)
	l.At = s.now()
	failed := 0
	for _, p := range problems {
		if p != "" {
			l.Problems = append(l.Problems, p)
			failed++
		}
	}
	if len(repos) > 0 && failed == len(repos) {
		return nil, fmt.Errorf("GitHub couldn't be asked about any repo: %s", problems[0])
	}
	if askedErr != nil {
		l.Problems = append(l.Problems, "requests to your teams couldn't be found: "+askedErr.Error())
	}
	sort.Strings(l.Problems)
	return l, nil
}

// Known finds an open PR in the list as it's kept, or its page as it's
// kept, without asking GitHub: for saving a comment or marking a file
// viewed, which shouldn't wait on it. One that isn't there is looked for
// as Open does.
func (s *Service) Known(repo string, number int) (*PR, *Stack, error) {
	if pr, st := s.Peek().Find(repo, number); pr != nil {
		return pr, st, nil
	}
	if pg := s.keptPage(repo, number); pg != nil {
		pr := pg.PR
		return &pr, pg.Stack, nil
	}
	return s.Open(repo, number)
}

// Open finds an open PR, and its stack if it's in one.
func (s *Service) Open(repo string, number int) (*PR, *Stack, error) {
	l, err := s.List(false)
	if pr, st := l.Find(repo, number); pr != nil {
		return pr, st, nil
	}
	if err != nil {
		return nil, nil, err
	}
	// Opened since the list was made, perhaps.
	if l, _ = s.List(true); l != nil {
		if pr, st := l.Find(repo, number); pr != nil {
			return pr, st, nil
		}
	}
	return nil, nil, fmt.Errorf("%w: %s#%d isn't open (merged or closed, or not one of this workspace's repos)", ErrNotOpen, repo, number)
}

// lockRepo serialises fetches into one repo's clone.
func (s *Service) lockRepo(repo string) func() {
	s.mu.Lock()
	if s.repoMu == nil {
		s.repoMu = map[string]*sync.Mutex{}
	}
	m := s.repoMu[repo]
	if m == nil {
		m = &sync.Mutex{}
		s.repoMu[repo] = m
	}
	s.mu.Unlock()
	m.Lock()
	return m.Unlock
}

// fetched is a PR's head commit in its repo's clone, fetching it (and the
// branch it's for) unless what's there is what GitHub says its head is.
func (s *Service) fetched(pr *PR) (string, error) {
	dir := s.BareDir(pr.Repo)
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("%s isn't cloned here (ws setup clones it)", pr.Repo)
	}
	defer s.lockRepo(pr.Repo)()
	have := review.CommitOf(dir, HeadRef(pr.Number))
	if have != "" && have == pr.Head && review.CommitOf(dir, BaseRef(pr.Base)) != "" {
		return have, nil
	}
	head, err := Fetch(dir, pr.Number, pr.Branch, pr.Base)
	if err != nil {
		if have != "" { // offline, say: what's here will do
			return have, nil
		}
		return "", err
	}
	return head, nil
}

// layer is what a PR's own changes are: from where it left the layer
// below (or, at the bottom, the branch it's for) to its head.
type layer struct {
	dir     string
	baseRef string // what base is of, for the page: origin/main, or the layer below's branch
	base    string
	head    string
	below   *PR
	// needsRebase: the layer below has moved on, and this hasn't followed.
	needsRebase bool
}

// keptLayer is a layer as it's kept, between reads: what it is, without
// where it's read from.
type keptLayer struct {
	BaseRef     string `json:"baseRef"`
	Base        string `json:"base"`
	Head        string `json:"head"`
	NeedsRebase bool   `json:"needsRebase,omitempty"`
}

func (l *layer) kept() keptLayer {
	return keptLayer{BaseRef: l.baseRef, Base: l.base, Head: l.head, NeedsRebase: l.needsRebase}
}

// in is a kept layer as it's read again, from a repo's clone at dir.
func (k keptLayer) in(dir string) *layer {
	return &layer{dir: dir, baseRef: k.BaseRef, base: k.Base, head: k.Head, needsRebase: k.NeedsRebase}
}

func (s *Service) layer(pr *PR, st *Stack) (*layer, error) {
	head, err := s.fetched(pr)
	if err != nil {
		return nil, err
	}
	l := &layer{dir: s.BareDir(pr.Repo), head: head, baseRef: BaseRef(pr.Base)}
	tip := ""
	if st != nil {
		for i := range st.Layers {
			if st.Layers[i].Number == pr.Number && i > 0 {
				l.below = &st.Layers[i-1]
			}
		}
	}
	if l.below != nil {
		if tip, err = s.fetched(l.below); err != nil {
			return nil, err
		}
		l.baseRef = BaseRef(l.below.Branch)
	} else if tip = review.CommitOf(l.dir, BaseRef(pr.Base)); tip == "" {
		return nil, fmt.Errorf("%s, the branch %s is for, isn't in %s's clone (ws fetch)", pr.Base, pr.Ref(), pr.Repo)
	}
	if l.base, err = review.MergeBase(l.dir, tip, head); err != nil {
		return nil, err
	}
	l.needsRebase = l.below != nil && l.base != tip
	return l, nil
}

// Detail is a PR in full: GET /api/prs/{repo}/{number}.
type Detail struct {
	PR   PR     `json:"pr"`
	Body string `json:"body,omitempty"` // what it says it does
	// BodyID names the description by what it says: the overview is viewed
	// while it's the one you read.
	BodyID string `json:"bodyID,omitempty"`
	// Stack is the stack it's a layer of, and Layer which (1 is the bottom).
	Stack *Stack `json:"stack,omitempty"`
	Layer int    `json:"layer,omitempty"`
	// BaseRef, Base and Head are what its changes are: from where it left
	// BaseRef (the branch it's for, or the layer below) to its head, as
	// fetched.
	BaseRef string `json:"baseRef"`
	Base    string `json:"base"`
	Head    string `json:"head"`
	// NeedsRebase: the layer below has moved on and this hasn't followed.
	// Its changes still show, against the layer below as it was.
	NeedsRebase bool `json:"needsRebase,omitempty"`
	// Reviewed is your last review of it, and Since what looking again
	// shows: "commits" (what's been pushed since), "changes" (it was
	// rebased: each file's change compared with what you reviewed), or ""
	// (nothing has changed, or you haven't reviewed it).
	Reviewed *Reviewed `json:"reviewed,omitempty"`
	Since    string    `json:"since,omitempty"`
	// Docked is the capsule it's checked out in here.
	Docked *Docked `json:"docked,omitempty"`
}

// Since modes.
const (
	SinceCommits = "commits"
	SinceChanges = "changes"
)

// Detail reads a PR: what GitHub says of it, and its commits, fetched.
func (s *Service) Detail(repo string, number int) (*Detail, error) {
	pr, st, err := s.Open(repo, number)
	if err != nil {
		return nil, err
	}
	bodyCh := make(chan string, 1)
	go func() { bodyCh <- s.body(pr) }()
	l, err := s.layer(pr, st)
	if err != nil {
		return nil, err
	}
	body := Readable(<-bodyCh)
	rec := s.seen(pr, l, body)
	d := newDetail(pr, st, l.kept(), body)
	if rv := s.reviewed(pr, rec, l); rv != nil {
		withReview(d, rv, sinceMode(l, rv))
	}
	s.withDocked(d, pr, st, rec)
	return d, nil
}

// seen writes what's kept of a PR as it's just been read: its title,
// branch and head, and what it says it does. It's what's kept after.
func (s *Service) seen(pr *PR, l *layer, body string) *Record {
	rec, _ := Update(s.Root, pr.Repo, pr.Number, func(r *Record) {
		r.Title, r.Branch, r.Head = pr.Title, pr.Branch, l.head
		if body != "" { // one that couldn't be read leaves what was
			r.Body = body
		}
	})
	if rec == nil {
		rec = &Record{}
	}
	return rec
}

// newDetail is a PR's detail from what GitHub says of it and its layer,
// before your review of it and where it's docked.
func newDetail(pr *PR, st *Stack, l keptLayer, body string) *Detail {
	d := &Detail{PR: *pr, Stack: st, BaseRef: l.BaseRef, Base: l.Base, Head: l.Head, NeedsRebase: l.NeedsRebase, Body: body}
	d.BodyID = BodyID(d.Body)
	if st != nil {
		for i := range st.Layers {
			if st.Layers[i].Number == pr.Number {
				d.Layer = i + 1
			}
		}
	}
	return d
}

// withReview says your last review in a detail, and what looking again
// shows.
func withReview(d *Detail, rv *Reviewed, since string) {
	d.Since = since
	shown := *rv
	shown.Prints = nil
	d.Reviewed = &shown
}

// withDocked says where a PR is docked in its detail.
func (s *Service) withDocked(d *Detail, pr *PR, st *Stack, rec *Record) {
	d.Docked = s.docked(pr, rec)
	if d.Docked == nil && st != nil {
		// A stack is docked at its top layer: one capsule holds every
		// layer, since each layer's branch contains those below.
		top := &st.Layers[len(st.Layers)-1]
		if top.Number != pr.Number {
			if trec, err := Load(s.Root, pr.Repo, top.Number); err == nil {
				d.Docked = s.docked(top, trec)
			}
		}
	}
}

// docked is the capsule a PR is checked out in: the one ws knows it (or
// you, through it) docked, else whichever has its branch.
func (s *Service) docked(pr *PR, rec *Record) *Docked {
	if rec.Docked != nil {
		if info, err := os.Stat(filepath.Join(filepath.Dir(s.BareDir(pr.Repo)), rec.Docked.Capsule)); err == nil && info.IsDir() {
			return rec.Docked
		}
	}
	if pr.Capsule != "" {
		return &Docked{Capsule: pr.Capsule, Branch: pr.Branch, By: ByYou}
	}
	return nil
}

// body is what a PR says it does, asked for again only when it's changed.
func (s *Service) body(pr *PR) string {
	key := pr.Ref()
	s.mu.Lock()
	b, ok := s.bodies[key]
	s.mu.Unlock()
	if ok && b.updated == pr.Updated {
		return b.text
	}
	d, err := s.GitHub.PRDetail(s.Org, pr.Repo, pr.Number)
	if err != nil {
		return b.text
	}
	s.mu.Lock()
	if s.bodies == nil {
		s.bodies = map[string]body{}
	}
	s.bodies[key] = body{updated: pr.Updated, text: d.Body}
	s.mu.Unlock()
	return d.Body
}

// reviewed is your last review of a PR: the one made here, or, when you
// reviewed it on GitHub instead (or since), that one, if its commit can
// still be had.
func (s *Service) reviewed(pr *PR, rec *Record, l *layer) *Reviewed {
	rv := rec.Reviewed
	gh := pr.Reviewed
	if gh == nil || gh.Commit == "" || (rv != nil && rv.Head == gh.Commit) {
		return rv
	}
	at, _ := time.Parse(time.RFC3339, gh.At)
	if rv != nil && !at.After(rv.At.Add(time.Minute)) {
		return rv // the one made here is the latest
	}
	return latest(rv, s.githubReviewed(pr, l))
}

// githubReviewed is your latest review of a PR on GitHub, its commit
// fetched into the clone: nil for none, or for one whose commit can't be
// had.
func (s *Service) githubReviewed(pr *PR, l *layer) *Reviewed {
	gh := pr.Reviewed
	if gh == nil || gh.Commit == "" {
		return nil
	}
	at, _ := time.Parse(time.RFC3339, gh.At)
	ref := ReviewedRef(pr.Number, "github")
	if review.CommitOf(l.dir, ref) != gh.Commit {
		release := s.lockRepo(pr.Repo)
		err := FetchCommit(l.dir, gh.Commit, ref)
		release()
		if err != nil {
			return nil
		}
	}
	base := l.base
	if gh.Commit != l.head {
		tip := review.CommitOf(l.dir, l.baseRef)
		if b, err := review.MergeBase(l.dir, tip, gh.Commit); err == nil {
			base = b
		}
	}
	return &Reviewed{ID: "github", Verdict: map[string]string{"approved": review.Approve, "changes_requested": review.Changes, "commented": review.Comment}[gh.State],
		Head: gh.Commit, Base: base, At: at}
}

// latest is the later of your last review made here and your last on
// GitHub: the one here, unless GitHub's is of another commit and more
// than a minute after it.
func latest(here, gh *Reviewed) *Reviewed {
	if gh == nil || (here != nil && (here.Head == gh.Head || !gh.At.After(here.At.Add(time.Minute)))) {
		return here
	}
	return gh
}

// sinceMode is what looking again at a layer shows, given your last
// review of it.
func sinceMode(l *layer, rv *Reviewed) string {
	switch {
	case rv.Head == l.head || review.CommitOf(l.dir, rv.Head) == "":
		return ""
	case rv.Base == l.base && review.IsAncestor(l.dir, rv.Head, l.head):
		return SinceCommits
	}
	return SinceChanges
}

// Diffed is a PR's diff: GET /api/prs/{repo}/{number}/diff.
type Diffed struct {
	*review.Diff
	// Scope is what it's of: "layer" (the PR's own changes), "since" (what's
	// changed since your last review) or "whole" (every layer of its stack
	// together).
	Scope string `json:"scope"`
	// Reviewed is the review a since diff is since. Rebased says it's
	// compared by changes, not commits: the diff is the whole layer, and
	// Unchanged are the files whose change is as you reviewed it.
	Reviewed  string   `json:"reviewed,omitempty"`
	Rebased   bool     `json:"rebased,omitempty"`
	Unchanged []string `json:"unchanged,omitempty"`
}

// Diff scopes.
const (
	ScopeLayer = "layer"
	ScopeSince = "since"
	ScopeWhole = "whole"
)

// Diff reads a PR's changes from its repo's clone.
func (s *Service) Diff(repo string, number int, scope string) (*Diffed, error) {
	pr, st, err := s.Open(repo, number)
	if err != nil {
		return nil, err
	}
	if scope == ScopeWhole {
		return s.whole(pr, st)
	}
	l, err := s.layer(pr, st)
	if err != nil {
		return nil, err
	}
	if scope != ScopeSince {
		return layerDiff(l)
	}
	rec, err := Load(s.Root, repo, number)
	if err != nil {
		return nil, err
	}
	rv := s.reviewed(pr, rec, l)
	if rv == nil {
		return nil, fmt.Errorf("you haven't reviewed %s yet", pr.Ref())
	}
	return sinceDiff(pr, l, rv, sinceMode(l, rv))
}

// layerDiff is a layer's own changes.
func layerDiff(l *layer) (*Diffed, error) {
	d, err := review.CompareCommits(l.dir, l.baseRef, l.base, l.head)
	if err != nil {
		return nil, err
	}
	return &Diffed{Diff: d, Scope: ScopeLayer}, nil
}

// sinceDiff is what's changed in a layer since your review of it, as
// looking again (since) shows it.
func sinceDiff(pr *PR, l *layer, rv *Reviewed, since string) (*Diffed, error) {
	switch since {
	case SinceCommits:
		d, err := review.CompareCommits(l.dir, l.baseRef, rv.Head, l.head)
		if err != nil {
			return nil, err
		}
		d.Since = rv.Head
		return &Diffed{Diff: d, Scope: ScopeSince, Reviewed: rv.ID}, nil
	case SinceChanges:
		d, err := review.CompareCommits(l.dir, l.baseRef, l.base, l.head)
		if err != nil {
			return nil, err
		}
		was := rv.Prints
		if was == nil {
			if was, err = review.Fingerprints(l.dir, rv.Base, rv.Head); err != nil {
				return nil, err
			}
		}
		now, err := review.Fingerprints(l.dir, l.base, l.head)
		if err != nil {
			return nil, err
		}
		out := &Diffed{Diff: d, Scope: ScopeSince, Reviewed: rv.ID, Rebased: true, Unchanged: []string{}}
		for _, f := range d.Files {
			if id, ok := was[f.Path]; ok && id == now[f.Path] {
				out.Unchanged = append(out.Unchanged, f.Path)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("nothing has changed in %s since your review", pr.Ref())
}

// whole is every layer of a stack together: from where its top left the
// branch the stack is for.
func (s *Service) whole(pr *PR, st *Stack) (*Diffed, error) {
	if st == nil || len(st.Layers) == 0 {
		return nil, fmt.Errorf("%s isn't in a stack", pr.Ref())
	}
	bottom, top := &st.Layers[0], &st.Layers[len(st.Layers)-1]
	if _, err := s.fetched(bottom); err != nil {
		return nil, err
	}
	head, err := s.fetched(top)
	if err != nil {
		return nil, err
	}
	dir := s.BareDir(pr.Repo)
	tip := review.CommitOf(dir, BaseRef(bottom.Base))
	if tip == "" {
		return nil, fmt.Errorf("%s, the branch the stack is for, isn't in %s's clone (ws fetch)", bottom.Base, pr.Repo)
	}
	base, err := review.MergeBase(dir, tip, head)
	if err != nil {
		return nil, err
	}
	d, err := review.CompareCommits(dir, BaseRef(bottom.Base), base, head)
	if err != nil {
		return nil, err
	}
	return &Diffed{Diff: d, Scope: ScopeWhole}, nil
}

// Store is your reviews of a PR, and Key what its draft is of.
func (s *Service) Store(repo string, number int) review.Store { return Reviews(s.Root, repo, number) }

// Key is what a PR's reviews are of, in its store: its ref.
func Key(repo string, number int) string { return fmt.Sprintf("%s#%d", repo, number) }
