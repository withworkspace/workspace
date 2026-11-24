package daemon

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// The daemon looks at who's asking for your review by itself, with no
// page open, and again soon after something here made the list stale.
func TestFollowReviewsLooksByItselfAndWhenTheListGoesStale(t *testing.T) {
	first, every, soon := followReviewsFirst, followReviewsEvery, followReviewsSoon
	followReviewsFirst, followReviewsEvery, followReviewsSoon = 5*time.Millisecond, time.Hour, time.Millisecond
	t.Cleanup(func() { followReviewsFirst, followReviewsEvery, followReviewsSoon = first, every, soon })

	root := t.TempDir()
	storetest.Own(t, root)
	var searches, lists atomic.Int32
	asking := []github.PR{{Number: 7, Title: "Asks you", Author: "alice", HeadRefName: "asks", BaseRefName: "main",
		Requested: []github.ReviewRequest{{Login: "me"}}}}
	svc := &pulls.Service{Root: root, Remote: func(repo string) ref.Remote { return ref.Remote{Owner: "acme", Name: repo} },
		Repos: func() []string { return []string{"api"} },
		GitHub: &github.Stub{Me: "me",
			PRsForRepoFn: func(org, repo string) ([]github.PR, error) { lists.Add(1); return asking, nil },
			AskedByNameFn: func(org string) ([]github.PRRef, error) {
				searches.Add(1)
				return []github.PRRef{{Owner: org, Repo: "api", Number: 7}}, nil
			},
		},
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		followReviewsLoop(svc, t.Logf, stop)
		close(done)
	}()
	t.Cleanup(func() { close(stop); <-done })

	until := func(what string, ok func() bool) {
		t.Helper()
		for end := time.Now().Add(5 * time.Second); !ok(); time.Sleep(2 * time.Millisecond) {
			if time.Now().After(end) {
				t.Fatalf("%s: searches %d, lists %d", what, searches.Load(), lists.Load())
			}
		}
	}
	until("the first look", func() bool { return len(svc.AskedOfYou()) == 1 })
	if searches.Load() != 1 || lists.Load() != 1 {
		t.Errorf("the first look: searches %d, lists %d, want one of each", searches.Load(), lists.Load())
	}
	// A review posted here: the list is asked for again, without waiting
	// for the next look's turn.
	svc.Stale()
	until("the look after it went stale", func() bool { return lists.Load() == 2 })
}

// With no GitHub to ask there's nothing to follow.
func TestFollowReviewsWithoutGitHubDoesNothing(t *testing.T) {
	done := make(chan struct{})
	go func() {
		followReviewsLoop(nil, t.Logf, make(chan struct{}))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("it's waiting on a service that isn't there")
	}
}
