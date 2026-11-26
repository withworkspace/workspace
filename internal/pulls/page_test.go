package pulls

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/review"
)

// pageWorld is a world whose GitHub counts what it's asked, and can be
// held: asking it waits until it's let go.
type pageWorld struct {
	*world
	clock   sync.Mutex
	at      time.Time
	asked   atomic.Int64
	threads atomic.Int64
	hold    chan struct{} // nil: answer at once
	holdMu  sync.Mutex
	prsMu   sync.Mutex
}

func newPageWorld(t *testing.T) *pageWorld {
	w := &pageWorld{world: newWorld(t)}
	w.at = w.now
	w.svc.Now = w.clockNow
	wait := func() {
		w.asked.Add(1)
		w.holdMu.Lock()
		h := w.hold
		w.holdMu.Unlock()
		if h != nil {
			<-h
		}
	}
	w.gh.PRsForRepoFn = func(org, repo string) ([]github.PR, error) {
		wait()
		w.prsMu.Lock()
		defer w.prsMu.Unlock()
		return append([]github.PR(nil), w.prs...), nil
	}
	w.gh.ReviewThreadsFn = func(org, repo string, number int) ([]github.ReviewThread, error) {
		wait()
		w.threads.Add(1)
		return []github.ReviewThread{{ID: "T1", Path: "tenant.go", Line: 3, Side: "RIGHT", Comments: []github.ThreadComment{
			{Author: "carol", Body: "Why?", At: "2026-11-10T09:00:00Z", DiffHunk: "@@ -0,0 +1,1 @@\n+package cache"}}}}, nil
	}
	w.gh.DiscussionFn = func(org, repo string, number int) ([]github.Said, error) {
		wait()
		return []github.Said{{ID: "C1", Author: "carol", Body: "Looks close.", At: "2026-11-10T09:00:00Z"}}, nil
	}
	w.gh.PRDetailFn = func(org, repo string, number int) (github.PRDetailResult, error) {
		wait()
		return github.PRDetailResult{Body: "Keys the cache by tenant."}, nil
	}
	return w
}

func (w *pageWorld) clockNow() time.Time {
	w.clock.Lock()
	defer w.clock.Unlock()
	return w.at
}

func (w *pageWorld) pass(d time.Duration) {
	w.clock.Lock()
	w.at = w.at.Add(d)
	w.clock.Unlock()
}

func (w *pageWorld) holdGitHub() func() {
	h := make(chan struct{})
	w.holdMu.Lock()
	w.hold = h
	w.holdMu.Unlock()
	return func() {
		w.holdMu.Lock()
		w.hold = nil
		w.holdMu.Unlock()
		close(h)
	}
}

// settled waits for a PR's page to have been read again, found
// different, and kept: its count moved past was.
func (w *pageWorld) settled(number int, was int64) {
	w.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for w.svc.PageSeen("api", number) == was {
		if time.Now().After(deadline) {
			w.t.Fatalf("api#%d's page wasn't read again", number)
		}
		time.Sleep(5 * time.Millisecond)
	}
	// And the read is done.
	for {
		w.svc.mu.Lock()
		busy := w.svc.reading[Key("api", number)] != nil
		w.svc.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPageIsKeptAndAnsweredAtOnce(t *testing.T) {
	w := newPageWorld(t)
	// The first time it's shown, it's read.
	d, err := w.svc.KeptDetail("api", 89)
	if err != nil {
		t.Fatal(err)
	}
	w.said(89)
	if d.Head != w.pr(89).HeadRefOid || d.Body != "Keys the cache by tenant." || d.Layer != 2 {
		t.Fatalf("detail = %+v", d)
	}
	asked, gits := w.asked.Load(), git.Started()

	// Every time after, it's answered as kept: no GitHub, no git.
	for range 2 {
		if _, err := w.svc.KeptDetail("api", 89); err != nil {
			t.Fatal(err)
		}
		diff, err := w.svc.KeptDiff("api", 89, ScopeLayer)
		if err != nil {
			t.Fatal(err)
		}
		if len(diff.Files) != 1 || diff.Files[0].Path != "tenant.go" {
			t.Fatalf("diff = %+v", diff.Files)
		}
		th, err := w.svc.KeptThreads("api", 89)
		if err != nil || len(th.Comments) != 1 || th.Comments[0].By != "carol" {
			t.Fatalf("threads = %+v, %v", th, err)
		}
		said, err := w.svc.KeptDiscussion("api", 89)
		if err != nil || len(said.Items) != 1 {
			t.Fatalf("discussion = %+v, %v", said, err)
		}
	}
	if w.asked.Load() != asked {
		t.Errorf("GitHub was asked %d more times", w.asked.Load()-asked)
	}
	if n := git.Started() - gits; n != 0 {
		t.Errorf("%d git processes started", n)
	}

	// Kept in the records: a daemon started again answers as kept too.
	again := &Service{Root: w.root, Remote: acme, GitHub: w.gh, Repos: w.svc.Repos, BareDir: w.svc.BareDir, Now: w.clockNow}
	gits = git.Started()
	if d, err := again.KeptDetail("api", 89); err != nil || d.Head != w.pr(89).HeadRefOid || d.Body != "Keys the cache by tenant." {
		t.Fatalf("after a restart: %+v, %v", d, err)
	}
	if diff, err := again.KeptDiff("api", 89, ScopeLayer); err != nil || len(diff.Files) != 1 {
		t.Fatalf("after a restart, diff: %+v, %v", diff, err)
	}
	if w.asked.Load() != asked || git.Started() != gits {
		t.Errorf("after a restart, it was read again: GitHub %d, git %d", w.asked.Load()-asked, git.Started()-gits)
	}
}

func TestStalePageIsReadBehindAndToldWhenItChanged(t *testing.T) {
	w := newPageWorld(t)
	was := w.pr(89).HeadRefOid
	if _, err := w.svc.KeptDetail("api", 89); err != nil {
		t.Fatal(err)
	}
	w.said(89)
	seen := w.svc.PageSeen("api", 89)

	// A push: the head moves on GitHub. While GitHub's held, a read answers
	// with what was kept, without waiting on it or starting git.
	w.prsMu.Lock()
	now := w.push(89, "tenant.go", "package cache\n\nfunc key(tenant, id string) string { return tenant + id }\n")
	w.prsMu.Unlock()
	w.pass(2 * time.Minute)
	release := w.holdGitHub()
	gits := git.Started()
	start := time.Now()
	d, err := w.svc.KeptDetail("api", 89)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := w.svc.KeptDiff("api", 89, ScopeLayer)
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("the read took %v: it waited", took)
	}
	if d.Head != was || diff.Head != was {
		t.Errorf("answered %s / %s, want what was kept (%s)", d.Head, diff.Head, was)
	}
	if n := git.Started() - gits; n != 0 {
		t.Errorf("%d git processes started while GitHub was held", n)
	}
	if w.svc.PageSeen("api", 89) != seen {
		t.Error("the count moved before anything was found")
	}

	// Let go: the read behind fetches the new head, and the count moves.
	release()
	w.settled(89, seen)
	d, err = w.svc.KeptDetail("api", 89)
	if err != nil {
		t.Fatal(err)
	}
	if d.Head != now {
		t.Errorf("head = %s, want %s", d.Head, now)
	}
	diff, err = w.svc.KeptDiff("api", 89, ScopeLayer)
	if err != nil || diff.Head != now {
		t.Fatalf("diff = %+v, %v", diff, err)
	}

	// Read again with nothing new, the count stays where it is.
	seen = w.svc.PageSeen("api", 89)
	w.pass(3 * time.Minute)
	threads := w.threads.Load()
	if _, err := w.svc.KeptDetail("api", 89); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for w.threads.Load() == threads && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	w.settledQuietly(89)
	if w.svc.PageSeen("api", 89) != seen {
		t.Error("the count moved, but nothing had changed")
	}
}

// settledQuietly waits for a read behind of a PR's page to be done.
func (w *pageWorld) settledQuietly(number int) {
	for {
		w.svc.mu.Lock()
		busy := w.svc.reading[Key("api", number)] != nil
		w.svc.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestReadsBehindAreOneAtATime(t *testing.T) {
	w := newPageWorld(t)
	if _, err := w.svc.KeptDetail("api", 92); err != nil {
		t.Fatal(err)
	}
	w.said(92)
	w.pass(3 * time.Minute)
	threads := w.threads.Load()
	release := w.holdGitHub()
	for range 5 {
		w.svc.KeptDetail("api", 92)
		w.svc.KeptThreads("api", 92)
		w.svc.Freshen("api", 92)
	}
	release()
	deadline := time.Now().Add(10 * time.Second)
	for w.threads.Load() == threads && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	w.settledQuietly(92)
	if n := w.threads.Load() - threads; n != 1 {
		t.Errorf("threads asked for %d times, want once", n)
	}
}

func TestPageIsForgottenOnceMerged(t *testing.T) {
	w := newPageWorld(t)
	if _, err := w.svc.KeptDetail("api", 92); err != nil {
		t.Fatal(err)
	}
	w.said(92)
	seen := w.svc.PageSeen("api", 92)
	w.prsMu.Lock()
	w.prs = w.prs[:3] // 92 is merged
	w.prsMu.Unlock()
	w.pass(3 * time.Minute)
	// Asked for once more, it's as kept; the read behind finds it gone.
	if _, err := w.svc.KeptDetail("api", 92); err != nil {
		t.Fatal(err)
	}
	w.settled(92, seen)
	if pg, _ := keptPage(w.root, "api", 92); pg != nil {
		t.Error("its page is still in the records")
	}
	if _, err := w.svc.KeptDetail("api", 92); !errors.Is(err, ErrNotOpen) {
		t.Errorf("err = %v, want it not open", err)
	}
	// The others are kept.
	if _, err := w.svc.KeptDetail("api", 89); err != nil {
		t.Fatal(err)
	}
	w.said(89) // kept in the records once what's been said is in
	if pg, _ := keptPage(w.root, "api", 89); pg == nil {
		t.Error("89's page wasn't kept")
	}
}

func TestPostingReadsWhatsSaidAgainAtOnce(t *testing.T) {
	w := newPageWorld(t)
	if _, err := w.svc.KeptThreads("api", 89); err != nil {
		t.Fatal(err)
	}
	seen := w.svc.PageSeen("api", 89)
	w.gh.DiscussionFn = func(org, repo string, number int) ([]github.Said, error) {
		return []github.Said{{ID: "C1", Author: "carol", Body: "Looks close.", At: "2026-11-10T09:00:00Z"}, {ID: "C2", Author: "me", Body: "Thanks!", At: "2026-11-10T10:00:00Z"}}, nil
	}
	d, err := w.svc.DiscussionNow("api", 89)
	if err != nil || len(d.Items) != 2 {
		t.Fatalf("discussion now = %+v, %v", d, err)
	}
	if kept, _ := w.svc.KeptDiscussion("api", 89); len(kept.Items) != 2 || !strings.Contains(kept.Items[1].Body, "Thanks") {
		t.Errorf("kept = %+v", kept)
	}
	if w.svc.PageSeen("api", 89) == seen {
		t.Error("the count didn't move")
	}
}

// The first time a PR is shown, it's found in the list as it's kept:
// showing it doesn't wait on GitHub being asked about every repo again,
// which is what a stale list (one from a while ago, or from before a
// review was posted) had it do.
func TestFirstShowDoesntWaitOnTheList(t *testing.T) {
	for name, stale := range map[string]func(*pageWorld){
		"a while since it was asked": func(w *pageWorld) { w.pass(3 * time.Minute) },
		"a review just posted":       func(w *pageWorld) { w.svc.Stale() },
	} {
		t.Run(name, func(t *testing.T) {
			w := newPageWorld(t)
			if _, err := w.svc.List(false); err != nil {
				t.Fatal(err)
			}
			stale(w)

			// From here, GitHub doesn't answer for the list until it's let go.
			var lists atomic.Int64
			held := make(chan struct{})
			answer := w.gh.PRsForRepoFn
			w.gh.PRsForRepoFn = func(org, repo string) ([]github.PR, error) {
				lists.Add(1)
				<-held
				return answer(org, repo)
			}
			var d *Detail
			done := make(chan error, 1)
			go func() {
				var err error
				d, err = w.svc.KeptDetail("api", 92)
				done <- err
			}()
			select {
			case err := <-done:
				if err != nil {
					close(held)
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				close(held)
				<-done
				t.Fatal("the first show waited on the list being asked for again")
			}
			if d.Head != w.pr(92).HeadRefOid || d.Body != "Keys the cache by tenant." {
				t.Errorf("detail = %+v", d)
			}

			// The list is still asked for, behind: what it says of the PR
			// now is what its page is next compared with.
			deadline := time.Now().Add(10 * time.Second)
			for lists.Load() == 0 && time.Now().Before(deadline) {
				time.Sleep(5 * time.Millisecond)
			}
			if lists.Load() == 0 {
				t.Error("the stale list wasn't asked for again behind")
			}
			close(held)
			w.said(92)
			for {
				w.svc.mu.Lock()
				busy := w.svc.loading != nil
				w.svc.mu.Unlock()
				if !busy {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}

// A PR that isn't in the list as it's kept is still looked for: GitHub is
// asked, as it may have been opened since.
func TestFirstShowLooksForOneTheListHasnt(t *testing.T) {
	w := newPageWorld(t)
	w.prsMu.Lock()
	all := w.prs
	w.prs = all[:3] // 92 isn't open yet
	w.prsMu.Unlock()
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	w.prsMu.Lock()
	w.prs = all
	w.prsMu.Unlock()
	w.pass(10 * time.Second)
	if d, err := w.svc.KeptDetail("api", 92); err != nil || d.Head != w.pr(92).HeadRefOid {
		t.Fatalf("detail = %+v, %v", d, err)
	}
	w.said(92)
}

// The first time a PR is shown, what it says it does and its diff don't
// wait on what's been said on it: GitHub's threads and its discussion are
// read behind, and those who ask for them wait for that alone.
func TestFirstShowDoesntWaitOnWhatsBeenSaid(t *testing.T) {
	w := newPageWorld(t)
	held := make(chan struct{})
	threads, said := w.gh.ReviewThreadsFn, w.gh.DiscussionFn
	w.gh.ReviewThreadsFn = func(org, repo string, number int) ([]github.ReviewThread, error) {
		<-held
		return threads(org, repo, number)
	}
	w.gh.DiscussionFn = func(org, repo string, number int) ([]github.Said, error) {
		<-held
		return said(org, repo, number)
	}
	type shown struct {
		d    *Detail
		diff *Diffed
		err  error
	}
	done := make(chan shown, 1)
	go func() {
		var x shown
		if x.d, x.err = w.svc.KeptDetail("api", 89); x.err == nil {
			x.diff, x.err = w.svc.KeptDiff("api", 89, ScopeLayer)
		}
		done <- x
	}()
	var x shown
	select {
	case x = <-done:
	case <-time.After(5 * time.Second):
		close(held)
		<-done
		t.Fatal("the first show waited on what's been said")
	}
	if x.err != nil {
		close(held)
		t.Fatal(x.err)
	}
	if x.d.Body != "Keys the cache by tenant." || len(x.diff.Files) != 1 || x.diff.Files[0].Path != "tenant.go" {
		t.Errorf("detail = %+v, diff = %+v", x.d, x.diff.Files)
	}
	seen := w.svc.PageSeen("api", 89)

	// Asking for what's been said waits for it, and no one asks GitHub twice.
	type saidNow struct {
		n   int
		err error
	}
	got := make(chan saidNow, 2)
	go func() {
		th, err := w.svc.KeptThreads("api", 89)
		if err != nil {
			got <- saidNow{err: err}
			return
		}
		got <- saidNow{n: len(th.Comments)}
	}()
	go func() {
		d, err := w.svc.KeptDiscussion("api", 89)
		if err != nil {
			got <- saidNow{err: err}
			return
		}
		got <- saidNow{n: len(d.Items)}
	}()
	select {
	case g := <-got:
		close(held)
		t.Fatalf("what's been said was answered before GitHub had: %+v", g)
	case <-time.After(100 * time.Millisecond):
	}
	asked := w.threads.Load()
	close(held)
	for range 2 {
		if g := <-got; g.err != nil || g.n != 1 {
			t.Errorf("what's been said = %+v", g)
		}
	}
	if n := w.threads.Load() - asked; n != 1 {
		t.Errorf("threads asked for %d times, want once", n)
	}
	// It's told as a change, for the open page, and kept in the records.
	if w.svc.PageSeen("api", 89) == seen {
		t.Error("the count didn't move when what's been said came in")
	}
	w.said(89)
	again := &Service{Root: w.root, Remote: acme, GitHub: w.gh, Repos: w.svc.Repos, BareDir: w.svc.BareDir, Now: w.clockNow}
	asks := w.asked.Load()
	if th, err := again.KeptThreads("api", 89); err != nil || len(th.Comments) != 1 {
		t.Errorf("after a restart, threads = %+v, %v", th, err)
	}
	if w.asked.Load() != asks {
		t.Error("after a restart, GitHub was asked again")
	}
}

// A daemon that stopped before what's been said came in reads the page
// again, rather than answering one without it.
func TestPageKeptWithoutWhatsBeenSaidIsReadAgain(t *testing.T) {
	w := newPageWorld(t)
	held := make(chan struct{})
	threads := w.gh.ReviewThreadsFn
	w.gh.ReviewThreadsFn = func(org, repo string, number int) ([]github.ReviewThread, error) {
		<-held
		return threads(org, repo, number)
	}
	first := make(chan error, 1)
	go func() {
		_, err := w.svc.KeptDetail("api", 92)
		first <- err
	}()
	select {
	case err := <-first:
		if err != nil {
			close(held)
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		close(held)
		<-first
		t.Fatal("the first show waited on what's been said")
	}
	again := &Service{Root: w.root, Remote: acme, GitHub: w.gh, Repos: w.svc.Repos, BareDir: w.svc.BareDir, Now: w.clockNow}
	close(held)
	if th, err := again.KeptThreads("api", 92); err != nil || len(th.Comments) != 1 {
		t.Errorf("threads = %+v, %v", th, err)
	}
	w.said(92)
	was := w.svc
	w.svc = again
	w.said(92)
	w.svc = was
}

// said waits for what's been said on a PR to have been read behind its
// first showing, and kept.
func (w *pageWorld) said(number int) {
	w.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		w.svc.mu.Lock()
		pg, busy := w.svc.pages[Key("api", number)], w.svc.saying[Key("api", number)] != nil
		w.svc.mu.Unlock()
		if !busy && (pg == nil || !pg.unsaid()) {
			return
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("what's been said on api#%d wasn't read", number)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Posting a review marks the list stale, and posting a stack's is one
// review after another: each one finds its PR in the list as it's kept,
// rather than asking GitHub about every repo before it posts.
func TestPostingAStacksReviewsDoesntAskForTheList(t *testing.T) {
	w := newPageWorld(t)
	w.gh.SubmitReviewFn = func(org, repo string, number int, r github.ReviewSubmission) (string, error) {
		return "https://github.com/acme/api/pull/1#pullrequestreview-1", nil
	}
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	w.pass(3 * time.Minute) // you've been reading it a while
	var lists atomic.Int64
	answer := w.gh.PRsForRepoFn
	w.gh.PRsForRepoFn = func(org, repo string) ([]github.PR, error) {
		lists.Add(1)
		return answer(org, repo)
	}
	for _, n := range []int{88, 89, 90} {
		// As the dialog does: what it would post, then posting it.
		if _, err := w.svc.Posting("api", n, "", review.Approve, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := w.svc.Submit("api", n, "", review.Approve, ""); err != nil {
			t.Fatal(err)
		}
	}
	if n := lists.Load(); n != 0 {
		t.Errorf("GitHub was asked for the list %d times while three reviews were posted", n)
	}
	// It's still stale, for whoever next looks at the list.
	if _, stale := w.svc.peekStale(); !stale {
		t.Error("the list isn't stale after a review was posted")
	}
}

// In the daemon, the pages of pull requests asking for your review (or
// yours) are read as the list is, so they're there when you open one;
// everyone else's is read when it's first shown.
func TestReadYours_PagesReadBehindTheList(t *testing.T) {
	w := newWorld(t)
	w.gh.ReviewRequestedFn = func(org string) ([]github.PRRef, error) {
		return []github.PRRef{{Repo: "api", Number: 92}}, nil
	}
	w.svc.ReadYours = true
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); w.svc.keptPage("api", 92) == nil; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the page of the PR asking for your review was never read")
		}
	}
	for w.svc.readingYoursNow() {
		time.Sleep(10 * time.Millisecond)
	}
	if w.svc.keptPage("api", 88) != nil {
		t.Error("someone else's PR's page was read before it was shown")
	}
}
