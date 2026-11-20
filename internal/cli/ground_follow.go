package cli

import (
	"context"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
)

// The daemon keeps every repo's ground up to its remote's default branch
// (ops.FollowGrounds): each look asks GitHub where all the repos' branches
// are in one request, and fetches and moves only those that have moved.
// Pages see a ground that's moved by themselves (the view's groundsSeen
// follows each ground's HEAD).

// followGroundsFirst is how long after it starts a daemon first looks,
// and followGroundsEvery how often after that.
var (
	followGroundsFirst = time.Minute
	followGroundsEvery = 5 * time.Minute
)

// followGroundsLoop follows the repos' grounds until stop closes, and
// logs what's changed: a ground moved, or one that couldn't be (said
// once, until what stops it does).
func followGroundsLoop(w *ops.Workspace, logf func(string, ...any), stop <-chan struct{}) {
	t := time.NewTimer(followGroundsFirst)
	defer t.Stop()
	said := map[string]string{} // repo → why it couldn't be followed, as last logged
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		for _, f := range w.FollowGrounds(ctx) {
			if f.Moved > 0 {
				logf("grounds: %s moved forward %d %s, to %s", f.Repo, f.Moved, plural(f.Moved, "commit"), f.At)
			}
			why := groundHeld(f)
			if why != said[f.Repo] && why != "" {
				logf("grounds: %s couldn't be followed: %s", f.Repo, why)
			}
			said[f.Repo] = why
		}
		cancel()
		t.Reset(followGroundsEvery)
	}
}

// groundHeld is why a ground wasn't followed, in a line: "" for nothing
// in its way.
func groundHeld(f ops.GroundFollowed) string {
	switch {
	case f.Err != nil:
		return f.Err.Error()
	case f.Held != nil:
		return f.Held.Reason
	}
	return ""
}
