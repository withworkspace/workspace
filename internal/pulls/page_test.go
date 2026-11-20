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
	again := &Service{Root: w.root, Org: "acme", GitHub: w.gh, Repos: w.svc.Repos, BareDir: w.svc.BareDir, Now: w.clockNow}
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
