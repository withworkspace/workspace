package pulls

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/outbox"
	"github.com/withworkspace/workspace/internal/review"
)

// note adds a comment to your draft review of a PR.
func (w *world) note(number int, path, body string) {
	w.t.Helper()
	d, err := w.svc.Detail("api", number)
	if err != nil {
		w.t.Fatal(err)
	}
	diffed := review.Diffed{Dir: w.bare, BaseRef: d.BaseRef, Base: d.Base, Head: d.Head}
	if _, err := w.svc.Store("api", number).AddNote(Key("api", number), diffed, review.NewNote{Path: path, Side: "new", Line: 3, Kind: "blocking", Body: body}, w.now); err != nil {
		w.t.Fatal(err)
	}
}

// With somewhere to send from, your review is kept as submitted at once,
// and posted behind as it was worked out then. One GitHub refuses is said
// on the PR, which asks for your review again, and is sent again or made
// your draft again, with its comments and what you wrote with it.
func TestReviewSentBehind(t *testing.T) {
	w := newWorld(t)
	var mu sync.Mutex
	var posted []github.ReviewSubmission
	gate := make(chan struct{})
	var refuse error
	w.gh.SubmitReviewFn = func(org, repo string, number int, r github.ReviewSubmission) (string, error) {
		<-gate
		mu.Lock()
		defer mu.Unlock()
		if refuse != nil {
			return "", refuse
		}
		posted = append(posted, r)
		return "https://github.com/acme/api/pull/92#pullrequestreview-1", nil
	}
	q := &outbox.Queue{Root: w.root}
	w.svc.SendBehind(q)
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	// 92 asks for your review, by name.
	w.prs[3].Requested = []github.ReviewRequest{{Login: "me"}}
	w.svc.Stale()
	w.svc.List(false)
	w.note(92, "README.md", "Say more.")
	asks := func() bool {
		for _, a := range w.svc.AskedOfYou() {
			if a.PR.Number == 92 {
				return true
			}
		}
		return false
	}
	if !asks() {
		t.Fatal("api#92 doesn't ask for your review")
	}

	w.now = w.now.Add(time.Minute)
	sub, err := w.svc.Submit("api", 92, "", review.Changes, "Nearly.")
	if err != nil {
		t.Fatal(err)
	}
	if !sub.Sending || sub.URL != "" || sub.Review == nil || sub.Review.State != review.Submitted || len(sub.Review.Comments) != 1 {
		t.Fatalf("submitted = %+v", sub)
	}
	// It's kept, and nothing asks for your review, before GitHub has it.
	rec, _ := Load(w.root, "api", 92)
	if rec.Reviewed == nil || rec.Reviewed.ID != sub.Review.ID || rec.Reviewed.URL != "" {
		t.Errorf("kept as reviewed = %+v", rec.Reviewed)
	}
	if asks() {
		t.Error("it still asks for your review")
	}
	if draft, _ := w.svc.Store("api", 92).Draft(Key("api", 92)); draft != nil {
		t.Errorf("there's still a draft: %+v", draft)
	}
	// What's written after doesn't change what's sent.
	w.note(92, "README.md", "And this, later.")
	if s := w.svc.Sends(); s == nil || s.Sending != 1 {
		t.Errorf("sends = %+v", s)
	}

	// GitHub refuses it: the PR asks again, and says what didn't get there.
	refuse = errors.New("Review cannot be requested from pull request author")
	close(gate)
	q.Wait()
	un := w.svc.Unsent("api", 92)
	if len(un) != 1 || !un[0].Draft || un[0].Doing != "your review of api#92" || !strings.HasPrefix(un[0].Body, "Nearly.") {
		t.Fatalf("unsent = %+v", un)
	}
	w.now = w.now.Add(time.Minute)
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	if !asks() {
		t.Error("it doesn't ask for your review again, and GitHub hasn't it")
	}
	// It can't be a draft again while there's a newer one.
	if err := w.svc.Discard("api", 92, un[0].ID); err == nil || !strings.Contains(err.Error(), "newer draft") {
		t.Errorf("taking it back over a newer draft: %v", err)
	}
	// Sent again, as it was: the comment written since isn't in it.
	mu.Lock()
	refuse = nil
	mu.Unlock()
	if err := w.svc.Resend("api", 92, un[0].ID); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	if len(posted) != 1 || posted[0].Event != github.EventRequestChanges || len(posted[0].Comments)+strings.Count(posted[0].Body, "Say more.") != 1 || strings.Contains(posted[0].Body, "later") {
		t.Fatalf("posted = %+v", posted)
	}
	if rec, _ = Load(w.root, "api", 92); rec.Reviewed == nil || !strings.HasSuffix(rec.Reviewed.URL, "pullrequestreview-1") {
		t.Errorf("where it is on GitHub = %+v", rec.Reviewed)
	}
	if len(w.svc.Unsent("api", 92)) != 0 || w.svc.Sends() != nil {
		t.Errorf("unsent = %+v, sends = %+v", w.svc.Unsent("api", 92), w.svc.Sends())
	}

	// Refused, and made a draft again: the newer draft posted first, then
	// one GitHub won't take.
	first := rec.Reviewed
	if _, err := w.svc.Submit("api", 92, "", review.Comment, ""); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	w.note(92, "README.md", "Third time.")
	mu.Lock()
	refuse = errors.New("no")
	mu.Unlock()
	if sub, err = w.svc.Submit("api", 92, "", review.Changes, "Still not."); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	un = w.svc.Unsent("api", 92)
	if len(un) != 1 {
		t.Fatalf("unsent = %+v", un)
	}
	if err := w.svc.Discard("api", 92, un[0].ID); err != nil {
		t.Fatal(err)
	}
	draft, _ := w.svc.Store("api", 92).Draft(Key("api", 92))
	if draft == nil || draft.ID != sub.Review.ID || len(draft.Comments) != 1 || draft.Comments[0].Body != "Third time." || draft.Summary != "Still not." {
		t.Errorf("the draft, taken back = %+v", draft)
	}
	if rec, _ = Load(w.root, "api", 92); rec.Reviewed == nil || rec.Reviewed.ID == sub.Review.ID || rec.Reviewed.ID == first.ID {
		t.Errorf("your review, with that one taken back = %+v", rec.Reviewed)
	}
	if len(w.svc.Unsent("api", 92)) != 0 {
		t.Errorf("unsent = %+v", w.svc.Unsent("api", 92))
	}
}

// A stack's reviews are posted in the order they were submitted, and one
// GitHub refuses holds those above it: they're not posted over it.
func TestStackReviewsSentBehindInOrder(t *testing.T) {
	w := newWorld(t)
	var mu sync.Mutex
	var posted []int
	gate := make(chan struct{})
	w.gh.SubmitReviewFn = func(org, repo string, number int, r github.ReviewSubmission) (string, error) {
		<-gate
		mu.Lock()
		defer mu.Unlock()
		if number == 89 {
			return "", errors.New("GitHub said no")
		}
		posted = append(posted, number)
		return "https://github.com/acme/api/pull/x", nil
	}
	q := &outbox.Queue{Root: w.root}
	w.svc.SendBehind(q)
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{88, 89, 90} {
		if sub, err := w.svc.Submit("api", n, "", review.Approve, ""); err != nil || !sub.Sending {
			t.Fatalf("submitting %d: %v", n, err)
		}
	}
	close(gate)
	q.Wait()
	if len(posted) != 1 || posted[0] != 88 {
		t.Errorf("posted %v, want the bottom layer's alone", posted)
	}
	if un := w.svc.Unsent("api", 89); len(un) != 1 || un[0].Error != "GitHub said no" {
		t.Errorf("the refused layer's = %+v", un)
	}
	un := w.svc.Unsent("api", 90)
	if len(un) != 1 || !strings.Contains(un[0].Error, "your review of api#89, which came first, failed") {
		t.Fatalf("the layer above's = %+v", un)
	}
	if s := w.svc.Sends(); s == nil || s.Failed != 2 || s.At != "api#89" {
		t.Errorf("sends = %+v", s)
	}
	// Sent by itself, it's posted.
	if err := w.svc.Resend("api", 90, un[0].ID); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	if len(posted) != 2 || posted[1] != 90 {
		t.Errorf("posted %v", posted)
	}
}

// A review the daemon stopped in the middle of posting is looked for on
// GitHub when ws is next up: there, it's done with, and where it is kept;
// not there, it's posted. It's never posted twice.
func TestReviewInterrupted(t *testing.T) {
	for _, there := range []bool{true, false} {
		w := newWorld(t)
		var mu sync.Mutex
		posts, first := 0, true
		gone := make(chan struct{}) // never closed: the daemon before went as it posted
		w.gh.SubmitReviewFn = func(org, repo string, number int, r github.ReviewSubmission) (string, error) {
			mu.Lock()
			hang := first
			first = false
			mu.Unlock()
			if hang {
				<-gone
			}
			mu.Lock()
			defer mu.Unlock()
			posts++
			return "https://github.com/acme/api/pull/92#pullrequestreview-posted", nil
		}
		w.gh.DiscussionFn = func(org, repo string, number int) ([]github.Said, error) {
			said := []github.Said{{ID: "old", Author: "me", State: "APPROVED", Commit: w.pr(92).HeadRefOid, At: "2026-11-01T09:00:00Z", URL: "https://github.com/acme/api/pull/92#pullrequestreview-old"}}
			if there {
				said = append(said, github.Said{ID: "new", Author: "me", State: "APPROVED", Commit: w.pr(92).HeadRefOid, At: w.now.Add(2 * time.Second).Format(time.RFC3339),
					URL: "https://github.com/acme/api/pull/92#pullrequestreview-found"})
			}
			return said, nil
		}
		// The daemon before: it took the review, and went as it posted it.
		before := &outbox.Queue{Root: w.root, Now: func() time.Time { return w.now }}
		w.svc.SendBehind(before)
		if _, err := w.svc.List(false); err != nil {
			t.Fatal(err)
		}
		if _, err := w.svc.Submit("api", 92, "", review.Approve, ""); err != nil {
			t.Fatal(err)
		}
		for i := 0; ; i++ {
			items, _ := outbox.In(w.root, "")
			if len(items) == 1 && items[0].State == outbox.Sending {
				break
			}
			if i > 2000 {
				t.Fatalf("it was never being sent: %+v", items)
			}
			time.Sleep(time.Millisecond)
		}

		q := &outbox.Queue{Root: w.root}
		w.svc.SendBehind(q)
		q.Resume()
		q.Wait()
		rec, _ := Load(w.root, "api", 92)
		want, wantPosts := "pullrequestreview-found", 0
		if !there {
			want, wantPosts = "pullrequestreview-posted", 1
		}
		mu.Lock()
		got := posts
		mu.Unlock()
		if got != wantPosts || rec.Reviewed == nil || !strings.HasSuffix(rec.Reviewed.URL, want) {
			t.Errorf("there %v: posted %d times, kept at %+v", there, got, rec.Reviewed)
		}
		if items, _ := outbox.In(w.root, ""); len(items) != 0 {
			t.Errorf("there %v: still kept to send: %+v", there, items)
		}
	}
}
