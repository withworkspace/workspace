package cli

import (
	"time"

	"github.com/withworkspace/workspace/internal/pulls"
)

// A pull request that asks for your review by name needs you whether or
// not a Reviews page is open (ops.Needs), so the daemon keeps that
// current itself: each look is one search of each owner, and the list is
// asked for only when the search and the list disagree (pulls.Look). The
// workspace's view moves with the list, so pages see it by themselves.

// followReviewsFirst is how long after it starts a daemon first looks,
// followReviewsEvery how often after that, and followReviewsSoon how long
// after something here made the list stale (a review posted, your name
// taken off): long enough for GitHub to say so, and for the reviews of a
// stack posted together to be one look.
var (
	followReviewsFirst = time.Minute
	followReviewsEvery = 5 * time.Minute
	followReviewsSoon  = 5 * time.Second
)

// followReviewsLoop looks at who's asking for your review until stop
// closes. reviews is the service the web UI reads from, so what a look
// finds is what a page shows; nil where there's no GitHub to ask. What
// stops it looking is logged once, until it changes.
func followReviewsLoop(reviews *pulls.Service, logf func(string, ...any), stop <-chan struct{}) {
	if reviews == nil {
		return
	}
	t := time.NewTimer(followReviewsFirst)
	defer t.Stop()
	due := reviews.Due()
	said := ""
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		case <-due:
			select {
			case <-stop:
				return
			case <-time.After(followReviewsSoon):
			}
		}
		why := ""
		if _, err := reviews.Look(); err != nil {
			why = err.Error()
		}
		if why != said && why != "" {
			logf("reviews: who's asking for your review couldn't be looked at: %s", why)
		}
		said = why
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
		t.Reset(followReviewsEvery)
	}
}
