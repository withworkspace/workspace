package ops

import (
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/review"
)

// Reviews: your review of a mission's capsule, or of a pull request, as
// you write it (notes on lines, files marked viewed) and once it's
// submitted (answers to its notes). Posting a pull request's review to
// GitHub is pulls.Service's.

// ReviewOf is what's reviewed: a mission's capsule (Mission, and the
// capsule as repo/name), or a pull request (Repo and PR).
type ReviewOf struct {
	Mission, Capsule string
	Repo             string
	PR               int
}

// OfCapsule is a review of a mission's capsule.
func OfCapsule(slug, repo, name string) ReviewOf {
	return ReviewOf{Mission: slug, Capsule: repo + "/" + name}
}

// OfPR is a review of a pull request.
func OfPR(repo string, number int) ReviewOf { return ReviewOf{Repo: repo, PR: number} }

// Reviews is where what's reviewed keeps its reviews, and its key there.
func (w *Workspace) Reviews(of ReviewOf) (review.Store, string) {
	if of.PR != 0 {
		return pulls.Reviews(w.Root(), of.Repo, of.PR), pulls.Key(of.Repo, of.PR)
	}
	return review.For(w.Root(), of.Mission), of.Capsule
}

// DiscardReview throws your unsubmitted review away.
func (w *Workspace) DiscardReview(of ReviewOf) error {
	store, key := w.Reviews(of)
	return store.Discard(key)
}

// AddNote adds a note to your review, on the diff it was written on.
func (w *Workspace) AddNote(of ReviewOf, d review.Diffed, n review.NewNote) error {
	store, key := w.Reviews(of)
	_, err := store.AddNote(key, d, n, w.now())
	return err
}

// EditNote changes a note of your unsubmitted review.
func (w *Workspace) EditNote(of ReviewOf, id, kind, body string, suggestion *string) error {
	store, key := w.Reviews(of)
	_, err := store.EditNote(key, id, kind, body, suggestion)
	return err
}

// DeleteNote takes a note out of your unsubmitted review.
func (w *Workspace) DeleteNote(of ReviewOf, id string) error {
	store, key := w.Reviews(of)
	_, err := store.DeleteNote(key, id)
	return err
}

// SetViewed marks a file of the diff viewed (as it is, blob), or not.
func (w *Workspace) SetViewed(of ReviewOf, path, blob string, viewed bool) error {
	store, key := w.Reviews(of)
	_, err := store.SetViewed(key, path, blob, viewed)
	return err
}

// Respond answers a submitted note: a reply, or resolving or reopening it
// (or, from crew, saying it's addressed or won't be), with the commit that
// addressed it, if there's one. A note that isn't of what's reviewed is
// review.ErrNotFound.
func (w *Workspace) Respond(of ReviewOf, id string, what review.Response, body, commit string, by Actor) (*review.Review, error) {
	store, key := w.Reviews(of)
	if rv, _, err := store.FindNote(id); err == nil && key != "" && rv.Capsule != key {
		return nil, fmt.Errorf("%w: %s is not a comment on %s", review.ErrNotFound, id, key)
	}
	return store.Respond(id, by.By(), what, body, commit, w.now())
}

// SubmitReview submits your review of a mission's capsule, on the diff it
// was written on. It's something you did to the mission (the
// orchestrator's count of crew started without you starts again), and the
// orchestrator's to act on.
func (w *Workspace) SubmitReview(of ReviewOf, d review.Diffed, verdict, summary string, by Actor) (*review.Review, error) {
	store, key := w.Reviews(of)
	rv, err := store.Submit(key, d, verdict, summary, w.now())
	if err != nil {
		return nil, err
	}
	if of.Mission != "" {
		w.youActed(of.Mission, by)
		// An Open Pull Request step waiting on your review opens it now:
		// that's all there was to do about it.
		if verdict == review.Approve && strings.HasPrefix(w.RetryOpenPRStep(of.Mission, key), "step") {
			return rv, nil
		}
		// A Review step whose review this completes is passed: approving
		// what it looks at is what it asked of you.
		if verdict == review.Approve && w.PassReviewStep(of.Mission) != "" {
			return rv, nil
		}
		w.Notify(of.Mission, mission.OrchestratorEvent{Trigger: "review"})
	}
	return rv, nil
}
