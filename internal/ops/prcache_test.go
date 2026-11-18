package ops

import (
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
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
