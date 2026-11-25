package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/outbox"
	"github.com/withworkspace/workspace/internal/review"
)

// Where the Reviews page's service sends behind (the daemon's does), a
// reply answers with it shown, before GitHub has it; one GitHub refuses
// is said on the pull request and in the workspace's count, and is sent
// again or let go of from there.
func TestReviewsSentBehind(t *testing.T) {
	w := newReviewsWorld(t)
	q := &outbox.Queue{Root: w.s.Root}
	w.s.Reviews.SendBehind(q)
	send := func(method, path, payload string, v any) int {
		t.Helper()
		req, _ := http.NewRequest(method, w.s.Info().Origin()+path, strings.NewReader(payload))
		req.AddCookie(&http.Cookie{Name: w.s.cookieName(), Value: w.s.Info().Token})
		req.Header.Set("Origin", w.s.Info().Origin())
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if v != nil && res.StatusCode == 200 {
			if err := json.NewDecoder(res.Body).Decode(v); err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
		}
		return res.StatusCode
	}
	var threads review.Review
	if code := getInto(t, w.s, "/api/prs/api/5/threads", &threads); code != 200 || len(threads.Comments) != 1 {
		t.Fatalf("threads: %d %+v", code, threads)
	}

	// GitHub is held: the reply is answered with it shown, as on its way.
	gate := make(chan struct{})
	refuse := errors.New("replying on GitHub: the thread is locked")
	w.gh.ReplyToThreadFn = func(org, repo string, number int, id, body string) error {
		<-gate
		return refuse
	}
	if code := send("POST", "/api/prs/api/5/threads/T1/respond", `{"what":"reply","body":"Three."}`, &threads); code != 200 {
		t.Fatalf("reply: %d", code)
	}
	if th := threads.Comments[0].Thread; len(th) != 1 || !th[0].Sending || th[0].Body != "Three." {
		t.Errorf("the thread, replied to: %+v", th)
	}
	var ws Workspace
	if getInto(t, w.s, "/api/workspace", &ws); ws.Sends == nil || ws.Sends.Sending != 1 {
		t.Errorf("the workspace's sends: %+v", ws.Sends)
	}

	// GitHub refuses it: it's off the thread, and said on the PR.
	close(gate)
	q.Wait()
	if getInto(t, w.s, "/api/prs/api/5/threads", &threads); len(threads.Comments[0].Thread) != 0 {
		t.Errorf("a refused reply is still shown: %+v", threads.Comments[0].Thread)
	}
	var d PRDetail
	if getInto(t, w.s, "/api/prs/api/5", &d); len(d.Unsent) != 1 || d.Unsent[0].Doing != "your reply on a.go:3" || d.Unsent[0].Body != "Three." || !strings.Contains(d.Unsent[0].Error, "locked") {
		t.Fatalf("unsent: %+v", d.Unsent)
	}
	if getInto(t, w.s, "/api/workspace", &ws); ws.Sends == nil || ws.Sends.Failed != 1 || ws.Sends.At != "api#5" {
		t.Errorf("the workspace's sends: %+v", ws.Sends)
	}
	id := d.Unsent[0].ID

	// Sent again, and refused again; then let go of.
	if code := send("POST", fmt.Sprintf("/api/prs/api/5/unsent/%d/resend", id), "", nil); code != 200 {
		t.Fatalf("resend: %d", code)
	}
	q.Wait()
	if d = (PRDetail{}); getInto(t, w.s, "/api/prs/api/5", &d) != 200 || len(d.Unsent) != 1 {
		t.Fatalf("after sending it again: %+v", d.Unsent)
	}
	for path, want := range map[string]int{
		"/api/prs/api/5/unsent/999":                 http.StatusNotFound,
		"/api/prs/api/5/unsent/x":                   http.StatusNotFound,
		fmt.Sprintf("/api/prs/api/5/unsent/%d", id): http.StatusOK,
	} {
		if code := send("DELETE", path, "", nil); code != want {
			t.Errorf("DELETE %s: %d, want %d", path, code, want)
		}
	}
	if d = (PRDetail{}); getInto(t, w.s, "/api/prs/api/5", &d) != 200 || len(d.Unsent) != 0 {
		t.Errorf("after letting go: %+v", d.Unsent)
	}
	if ws = (Workspace{}); getInto(t, w.s, "/api/workspace", &ws) != 200 || ws.Sends != nil {
		t.Errorf("the workspace's sends, with nothing waiting: %+v", ws.Sends)
	}
}
