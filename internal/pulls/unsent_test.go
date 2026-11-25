package pulls

import (
	"errors"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/outbox"
	"github.com/withworkspace/workspace/internal/review"
)

// With somewhere to send from, what's said from a PR's page is shown at
// once, before GitHub has it, and sent behind; once it has, what's kept is
// GitHub's, and nothing's shown twice. What GitHub refuses is kept, said
// on the page, and sent again or let go of.
func TestSaidBehind(t *testing.T) {
	w := newWorld(t)
	var threads []github.ReviewThread
	var said []github.Said
	threads = []github.ReviewThread{{ID: "T1", Path: "tenant.go", Line: 3, Side: "RIGHT", Comments: []github.ThreadComment{
		{Author: "carol", Body: "collides", At: "2026-11-10T09:00:00Z"}}}}
	w.gh.ReviewThreadsFn = func(org, repo string, number int) ([]github.ReviewThread, error) { return threads, nil }
	w.gh.DiscussionFn = func(org, repo string, number int) ([]github.Said, error) { return said, nil }
	// GitHub, held until the test lets it answer.
	gate := make(chan struct{})
	var refuse error
	w.gh.ReplyToThreadFn = func(org, repo string, number int, id, body string) error {
		<-gate
		if refuse != nil {
			return refuse
		}
		threads[0].Comments = append(threads[0].Comments, github.ThreadComment{Author: "me", Body: body, At: "2026-11-10T12:00:01Z"})
		return nil
	}
	w.gh.ResolveThreadFn = func(org, repo string, number int, id string, yes bool) error {
		<-gate
		threads[0].Resolved = yes
		return nil
	}
	w.gh.CommentFn = func(org, repo string, number int, body string) error {
		<-gate
		said = append(said, github.Said{ID: "c1", Author: "me", Body: body, At: "2026-11-10T12:00:02Z"})
		return nil
	}
	q := &outbox.Queue{Root: w.root}
	w.svc.SendBehind(q)
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.KeptThreads("api", 89); err != nil {
		t.Fatal(err)
	}
	seen := w.svc.PageSeen("api", 89)

	// A reply, a resolve and a comment: each answered with it shown, as on
	// its way, while GitHub hasn't been heard from.
	now, err := w.svc.Respond("api", 89, "T1", review.Answer, " Fixed. ")
	if err != nil {
		t.Fatal(err)
	}
	if n := now.Comments[0]; len(n.Thread) != 1 || n.Thread[0].By != "you" || n.Thread[0].Body != "Fixed." || !n.Thread[0].Sending || n.Status != review.Open {
		t.Errorf("the thread, replied to: %+v", n)
	}
	if now, err = w.svc.Respond("api", 89, "T1", review.Resolve, ""); err != nil || now.Comments[0].Status != review.Resolved || !now.Comments[0].Sending || len(now.Comments[0].Thread) != 1 {
		t.Errorf("the thread, resolved: %v %+v", err, now.Comments[0])
	}
	d, err := w.svc.Comment("api", 89, "Thanks.")
	if err != nil || len(d.Items) != 1 || !d.Items[0].Mine || !d.Items[0].Sending || d.Items[0].Body != "Thanks." {
		t.Errorf("what's said, commented on: %v %+v", err, d)
	}
	if got := w.svc.PageSeen("api", 89); got == seen {
		t.Error("an open page wasn't told to look again")
	}
	if s := w.svc.Sends(); s == nil || s.Sending != 3 || s.Failed != 0 {
		t.Errorf("sends = %+v", s)
	}
	if len(threads[0].Comments) != 1 || len(said) != 0 {
		t.Fatal("GitHub was told before it answered")
	}

	// GitHub takes them: what's kept is what it says, each once, and
	// nothing's on its way.
	close(gate)
	q.Wait()
	now, err = w.svc.KeptThreads("api", 89)
	if err != nil {
		t.Fatal(err)
	}
	if n := now.Comments[0]; len(n.Thread) != 1 || n.Thread[0].Sending || n.Status != review.Resolved || n.Sending {
		t.Errorf("the thread, once sent: %+v", n)
	}
	if d, _ = w.svc.KeptDiscussion("api", 89); len(d.Items) != 1 || d.Items[0].ID != "c1" || d.Items[0].Sending {
		t.Errorf("what's said, once sent: %+v", d.Items)
	}
	if s := w.svc.Sends(); s != nil {
		t.Errorf("sends, once sent = %+v", s)
	}

	// One GitHub refuses isn't shown as made: it's kept, with why.
	refuse = errors.New("replying on GitHub: the thread is locked")
	if _, err := w.svc.Respond("api", 89, "T1", review.Answer, "And another."); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	if now, _ = w.svc.KeptThreads("api", 89); len(now.Comments[0].Thread) != 1 {
		t.Errorf("a refused reply is shown: %+v", now.Comments[0].Thread)
	}
	un := w.svc.Unsent("api", 89)
	if len(un) != 1 || un[0].Doing != "your reply on tenant.go:3" || un[0].Body != "And another." || !strings.Contains(un[0].Error, "locked") {
		t.Fatalf("unsent = %+v", un)
	}
	if s := w.svc.Sends(); s == nil || s.Failed != 1 || s.At != "api#89" {
		t.Errorf("sends = %+v", s)
	}
	// It's this PR's, and no other's.
	if err := w.svc.Resend("api", 88, un[0].ID); !errors.Is(err, outbox.ErrNotFound) {
		t.Errorf("sending another PR's again: %v", err)
	}
	// Sent again, once GitHub would take it.
	refuse = nil
	if err := w.svc.Resend("api", 89, un[0].ID); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	if now, _ = w.svc.KeptThreads("api", 89); len(now.Comments[0].Thread) != 2 || len(w.svc.Unsent("api", 89)) != 0 {
		t.Errorf("after sending it again: %+v, unsent %+v", now.Comments[0].Thread, w.svc.Unsent("api", 89))
	}
	// Or let go of.
	refuse = errors.New("no")
	w.svc.Respond("api", 89, "T1", review.Answer, "Never mind.")
	q.Wait()
	un = w.svc.Unsent("api", 89)
	if len(un) != 1 {
		t.Fatalf("unsent = %+v", un)
	}
	if err := w.svc.Discard("api", 89, un[0].ID); err != nil || len(w.svc.Unsent("api", 89)) != 0 {
		t.Errorf("letting go: %v", err)
	}
	if len(threads[0].Comments) != 3 {
		t.Errorf("GitHub's thread = %+v", threads[0].Comments)
	}
}

// When what a write changed can't be read again, the page keeps showing
// it as made, rather than losing it until GitHub can be read.
func TestSaidBehindUnread(t *testing.T) {
	w := newWorld(t)
	var fail error
	w.gh.DiscussionFn = func(org, repo string, number int) ([]github.Said, error) { return nil, fail }
	w.gh.CommentFn = func(org, repo string, number int, body string) error {
		fail = errors.New("GitHub is down")
		return nil
	}
	q := &outbox.Queue{Root: w.root}
	w.svc.SendBehind(q)
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.KeptDiscussion("api", 89); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Comment("api", 89, "Thanks."); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	d, err := w.svc.KeptDiscussion("api", 89)
	if err != nil || len(d.Items) != 1 || d.Items[0].Body != "Thanks." || d.Items[0].Sending {
		t.Errorf("what's said: %v %+v", err, d)
	}
	if w.svc.Sends() != nil {
		t.Errorf("sends = %+v", w.svc.Sends())
	}
}
