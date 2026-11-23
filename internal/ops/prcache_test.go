package ops

import (
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

// A page's fetch that comes back changed (a pull request merged) is news
// for the watcher, once; the same answer again isn't, and nor is what the
// watcher fetched itself.
func TestPRCache_NewsWhenAPagesFetchChanges(t *testing.T) {
	var mu sync.Mutex
	open, merged := []github.PR{{Number: 7, HeadRefName: "fix", StatusRollup: "success"}}, []github.PR(nil)
	gh := &github.Stub{
		PRsForRepoFn:       func(string, string) ([]github.PR, error) { mu.Lock(); defer mu.Unlock(); return open, nil },
		MergedPRsForRepoFn: func(string, string) ([]github.PR, error) { mu.Lock(); defer mu.Unlock(); return merged, nil },
	}
	ws := &workspace.Workspace{Org: "acme"}
	c := NewPRCache()
	// peek asks as a page does, and waits for GitHub's answer to land.
	peek := func() {
		t.Helper()
		was := c.Fetched()
		c.Forget("api") // as if it were asked a while ago
		c.Peek(ws, gh, []string{"api"})
		for deadline := time.Now().Add(5 * time.Second); c.Fetched() == was; time.Sleep(time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("GitHub's answer never landed")
			}
		}
	}
	news := func() bool {
		select {
		case <-c.News():
			return true
		default:
			return false
		}
	}

	if peek(); !news() {
		t.Error("the first answer is news")
	}
	if peek(); news() {
		t.Error("the same answer again isn't news")
	}

	mu.Lock()
	open, merged = nil, []github.PR{{Number: 7, HeadRefName: "fix", MergedAt: "2026-11-18T08:19:27Z"}}
	mu.Unlock()
	peek()
	peek() // a second page asks before the watcher has looked
	if !news() {
		t.Error("a merge a page's fetch brought is news")
	}
	if news() {
		t.Error("news not yet taken is told once")
	}
	if pr := c.Peek(ws, gh, []string{"api"})["api"].merged["fix"]; pr == nil || pr.Number != 7 {
		t.Errorf("the cache holds the merge: %+v", pr)
	}

	// The watcher's own fetch is its own to act on: nothing to tell it.
	mu.Lock()
	merged = nil
	mu.Unlock()
	c.Forget("api")
	if c.Get(ws, gh, []string{"api"}); news() {
		t.Error("the watcher's own fetch isn't news to it")
	}
	if (*PRCache)(nil).News() != nil {
		t.Error("no cache has no news")
	}
}

// A mission's page never waits on GitHub: it's read with the pull requests
// as kept, GitHub is asked behind the read once however many read
// meanwhile, and the page's stamp (and the workspace's) moves when the
// answer lands, and again only when one is different.
func TestMission_GitHubAskedBehindTheRead(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache")
	var mu sync.Mutex
	var asked int
	answer := make(chan struct{})
	open, merged := []github.PR{{Number: 3, HeadRefName: "cache"}}, []github.PR(nil)
	gh := &github.Stub{
		PRsForRepoFn: func(string, string) ([]github.PR, error) {
			mu.Lock()
			asked++
			mu.Unlock()
			<-answer
			mu.Lock()
			defer mu.Unlock()
			return open, nil
		},
		MergedPRsForRepoFn: func(string, string) ([]github.PR, error) { mu.Lock(); defer mu.Unlock(); return merged, nil },
	}
	w.opts.GitHub, w.opts.PRs = gh, NewPRCache()
	// read is the page's read, which mustn't wait on GitHub.
	read := func() *MissionDetail {
		t.Helper()
		got := make(chan *MissionDetail, 1)
		go func() {
			d, err := w.Mission("cache")
			if err != nil {
				t.Error(err)
			}
			got <- d
		}()
		select {
		case d := <-got:
			return d
		case <-time.After(5 * time.Second):
			t.Fatal("the mission's read waited on GitHub")
			return nil
		}
	}
	// moved waits for the mission's stamp to move from was.
	moved := func(was string) string {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
			if now := w.MissionStamp("cache"); now != was {
				return now
			}
			if time.Now().After(deadline) {
				t.Fatal("GitHub's answer didn't move the mission's stamp")
			}
		}
	}

	// Where its capsule stands is read behind the first read too, and
	// moves the stamps when it has: that's let land first, so what moves
	// them after is GitHub's answer.
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		unread := false
		for _, st := range read().Status {
			unread = unread || st.Unread
		}
		if !unread {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("where the mission's capsule stands was never read behind the read")
		}
	}
	stamp, wsStamp := w.MissionStamp("cache"), w.Stamp()
	for range 5 {
		if d := read(); d.GitHub || d.Capsules[0].PR != nil {
			t.Fatalf("before GitHub's answered: github %v, PR %+v", d.GitHub, d.Capsules[0].PR)
		}
	}
	if w.MissionStamp("cache") != stamp {
		t.Error("the stamp moved before GitHub answered")
	}
	close(answer)
	stamp = moved(stamp)
	if mu.Lock(); asked != 1 {
		t.Errorf("five reads asked GitHub %d times, not once", asked)
	}
	mu.Unlock()
	if w.Stamp() == wsStamp {
		t.Error("GitHub's answer didn't move the workspace's stamp")
	}
	if d := read(); !d.GitHub || d.Capsules[0].PR == nil || d.Capsules[0].PR.Number != 3 {
		t.Fatalf("after GitHub's answered: github %v, PR %+v", d.GitHub, d.Capsules[0].PR)
	}

	// The same answer again moves nothing.
	wsStamp = w.Stamp()
	was := w.opts.PRs.Fetched()
	w.opts.PRs.Forget("frontend") // as if it were asked a while ago
	read()
	for deadline := time.Now().Add(5 * time.Second); w.opts.PRs.Fetched() == was; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("GitHub was never asked again")
		}
	}
	if w.MissionStamp("cache") != stamp || w.Stamp() != wsStamp {
		t.Error("the same answer moved a stamp")
	}

	// A merge does.
	mu.Lock()
	open, merged = nil, []github.PR{{Number: 3, HeadRefName: "cache", MergedAt: "2026-11-20T08:00:00Z"}}
	mu.Unlock()
	w.opts.PRs.Forget("frontend")
	read()
	moved(stamp)
	if d := read(); d.Capsules[0].PRS != mission.PRMerged {
		t.Errorf("after the merge: %v", d.Capsules[0].PRS)
	}
}

// The watcher's Get waits on what a page's read is asking already, rather
// than asking GitHub again.
func TestPRCache_GetWaitsOnAPagesFetch(t *testing.T) {
	var mu sync.Mutex
	var asked int
	answer := make(chan struct{})
	gh := &github.Stub{
		PRsForRepoFn: func(string, string) ([]github.PR, error) {
			mu.Lock()
			asked++
			mu.Unlock()
			<-answer
			return []github.PR{{Number: 1, HeadRefName: "fix"}}, nil
		},
		MergedPRsForRepoFn: func(string, string) ([]github.PR, error) { return nil, nil },
	}
	ws := &workspace.Workspace{Org: "acme"}
	c := NewPRCache()
	if got := c.Peek(ws, gh, []string{"api"}); len(got) != 0 {
		t.Fatalf("before GitHub's answered: %+v", got)
	}
	got := make(chan PRIndex)
	go func() { got <- c.Get(ws, gh, []string{"api"}) }()
	time.Sleep(20 * time.Millisecond) // the watcher's Get, while the page's is out
	close(answer)
	if pr := (<-got)["api"].open["fix"]; pr == nil || pr.Number != 1 {
		t.Errorf("the watcher's: %+v", pr)
	}
	if mu.Lock(); asked != 1 {
		t.Errorf("GitHub was asked %d times, not once", asked)
	}
	mu.Unlock()
}
