package web

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/workspace"
)

// reviewsWorld is a workspace with one repo, api, whose remote has one
// open pull request, #5 (branch fix, for main, by alice, asking for your
// review), and a web UI serving its Reviews page.
type reviewsWorld struct {
	s      *Server
	gh     *github.Stub
	remote string
	bare   string
	head   string
	mu     sync.Mutex
	prs    []github.PR
	posted []github.ReviewSubmission
}

func (w *reviewsWorld) open() []github.PR {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]github.PR(nil), w.prs...)
}

func newReviewsWorld(t *testing.T) *reviewsWorld {
	t.Helper()
	root := t.TempDir()
	w := &reviewsWorld{remote: filepath.Join(root, "remote"), bare: filepath.Join(root, "repos", "api", ".bare")}
	os.MkdirAll(w.remote, 0o755)
	gitT(t, w.remote, "init", "-q", "--initial-branch=main")
	gitT(t, w.remote, "config", "user.name", "Test")
	gitT(t, w.remote, "config", "user.email", "test@example.com")
	os.WriteFile(filepath.Join(w.remote, "a.go"), []byte("package a\n\nvar x = 1\n"), 0o644)
	gitT(t, w.remote, "add", "-A")
	gitT(t, w.remote, "commit", "-q", "-m", "base")
	gitT(t, w.remote, "checkout", "-q", "-b", "fix")
	os.WriteFile(filepath.Join(w.remote, "a.go"), []byte("package a\n\nvar x = 2\n"), 0o644)
	gitT(t, w.remote, "commit", "-q", "-am", "two")
	gitT(t, w.remote, "update-ref", "refs/pull/5/head", "HEAD")
	w.head = gitT(t, w.remote, "rev-parse", "HEAD")
	gitT(t, w.remote, "checkout", "-q", "main")
	os.MkdirAll(filepath.Dir(w.bare), 0o755)
	if out, err := exec.Command("git", "clone", "-q", "--bare", w.remote, w.bare).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	gitT(t, w.bare, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	gitT(t, w.bare, "fetch", "-q", "origin")

	w.prs = []github.PR{{Number: 5, Title: "Two, not one", HeadRefName: "fix", BaseRefName: "main", Author: "alice", HeadRefOid: w.head,
		URL: "https://github.com/acme/api/pull/5", UpdatedAt: "2026-11-10T10:00:00Z", Additions: 1, Deletions: 1, ChangedFiles: 1,
		Requested: []github.ReviewRequest{{Login: "me"}}}}
	w.gh = &github.Stub{Me: "me",
		PRsForRepoFn: func(org, repo string) ([]github.PR, error) { return w.open(), nil },
		PRDetailFn: func(org, repo string, number int) (github.PRDetailResult, error) {
			return github.PRDetailResult{Body: "x is two."}, nil
		},
		SubmitReviewFn: func(org, repo string, number int, r github.ReviewSubmission) (string, error) {
			w.mu.Lock()
			defer w.mu.Unlock()
			w.posted = append(w.posted, r)
			return "https://github.com/acme/api/pull/5#pullrequestreview-9", nil
		},
		ReviewThreadsFn: func(org, repo string, number int) ([]github.ReviewThread, error) {
			return []github.ReviewThread{{ID: "T1", Path: "a.go", Line: 3, Side: "RIGHT", Comments: []github.ThreadComment{
				{Author: "carol", Body: "nitpick: three?", At: "2026-11-10T09:00:00Z", Commit: w.head, OriginalCommit: w.head}}}}, nil
		},
	}
	svc := &pulls.Service{Root: root, Remote: func(repo string) ref.Remote { return ref.Remote{Owner: "acme", Name: repo} }, GitHub: w.gh, Fresh: time.Millisecond,
		Repos:   func() []string { return []string{"api"} },
		BareDir: func(repo string) string { return filepath.Join(root, "repos", repo, ".bare") },
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	storetest.Own(t, root)
	w.s = &Server{Root: root, Poll: 20 * time.Millisecond, ReviewsPoll: 20 * time.Millisecond, Assets: testApp, listener: ln, Reviews: svc}
	// No agent here answers conversations, until a test says otherwise.
	w.s.App = testOps(w.s, ops.Options{Reviews: svc}, func(c *config.Config, _ *workspace.Workspace) { c.Conversation.Harness = "none-such" })
	if err := w.s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.s.Close)
	return w
}

func getInto(t *testing.T, s *Server, path string, v any) int {
	t.Helper()
	res := do(t, s, "GET", path, true, nil)
	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(v); err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
	}
	return res.StatusCode
}

func TestReviewsListAndPR(t *testing.T) {
	w := newReviewsWorld(t)
	var list ReviewsList
	if code := getInto(t, w.s, "/api/reviews", &list); code != 200 {
		t.Fatalf("list: %d", code)
	}
	if list.Count != 1 || len(list.Sections) != 1 || list.Sections[0].Key != pulls.NeedsReview || list.Sections[0].Rows[0].PR.Number != 5 || list.Me != "me" {
		t.Errorf("list = %+v", list.List)
	}

	var d PRDetail
	if code := getInto(t, w.s, "/api/prs/api/5", &d); code != 200 {
		t.Fatalf("PR: %d", code)
	}
	if d.PR.Title != "Two, not one" || d.Head != w.head || d.BaseRef != "origin/main" || d.Body != "x is two." || d.NoAsk == "" || d.Talks == nil {
		t.Errorf("detail = %+v", d)
	}
	var diff pulls.Diffed
	if code := getInto(t, w.s, "/api/prs/api/5/diff", &diff); code != 200 {
		t.Fatalf("diff: %d", code)
	}
	if diff.Scope != pulls.ScopeLayer || len(diff.Files) != 1 || diff.Files[0].Path != "a.go" || diff.Head != w.head {
		t.Errorf("diff = %+v", diff)
	}
	for path, want := range map[string]int{
		"/api/prs/api/5/diff?scope=whole":                  http.StatusUnprocessableEntity, // not in a stack
		"/api/prs/api/5/diff?scope=since":                  http.StatusUnprocessableEntity, // not reviewed yet
		"/api/prs/api/5/diff?scope=all":                    http.StatusBadRequest,
		"/api/prs/api/99":                                  http.StatusNotFound,
		"/api/prs/api/abc":                                 http.StatusNotFound,
		"/api/prs/web/5":                                   http.StatusNotFound,
		"/api/prs/..%2F..%2Fetc/5":                         http.StatusNotFound, // a repo's name names folders
		"/api/prs/%2E%2E/5/review":                         http.StatusNotFound,
		"/api/prs/api/5/blob?rev=" + w.head + "&path=a.go": http.StatusOK,
	} {
		if res := do(t, w.s, "GET", path, true, nil); res.StatusCode != want {
			t.Errorf("GET %s: %d, want %d (%s)", path, res.StatusCode, want, body(res))
		}
	}
}

func TestReviewsDraftAndSubmit(t *testing.T) {
	w := newReviewsWorld(t)
	var diff pulls.Diffed
	getInto(t, w.s, "/api/prs/api/5/diff", &diff)

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

	// A draft comment, through the handlers a capsule's review uses.
	var st ReviewState
	note := fmt.Sprintf(`{"base":%q,"head":%q,"path":"a.go","side":"new","line":3,"endLine":3,"kind":"question","body":"Why two?"}`, diff.Base, diff.Head)
	if code := send("POST", "/api/prs/api/5/review/comments", note, &st); code != 200 || st.Draft == nil || len(st.Draft.Comments) != 1 || st.Draft.Capsule != "api#5" {
		t.Fatalf("add a comment: %d %+v", code, st.Draft)
	}
	if code := send("PUT", "/api/prs/api/5/review/viewed", `{"path":"a.go","blob":"abc","viewed":true}`, &st); code != 200 || st.Viewed["a.go"] != "abc" {
		t.Errorf("viewed: %d %+v", code, st.Viewed)
	}

	// What submitting would post, without posting it.
	var p pulls.Posting
	if code := send("POST", "/api/prs/api/5/review/preview", `{"head":"`+diff.Head+`","verdict":"comment","summary":"One question."}`, &p); code != 200 {
		t.Fatalf("preview: %d", code)
	}
	if p.Event != github.EventComment || p.Body != "One question." || len(p.Comments) != 1 || p.Comments[0].Body != "question: Why two?" || p.Comments[0].Line != 3 || p.Commit != diff.Head {
		t.Errorf("posting = %+v", p)
	}
	if len(w.posted) != 0 {
		t.Fatal("the preview posted")
	}
	for payload, want := range map[string]int{
		`{"verdict":"merge"}`:                http.StatusUnprocessableEntity,
		`{"head":"abc","verdict":"approve"}`: http.StatusBadRequest,
	} {
		if code := send("POST", "/api/prs/api/5/review/preview", payload, nil); code != want {
			t.Errorf("preview %s: %d, want %d", payload, code, want)
		}
	}

	// Submitting posts it, and keeps it.
	if code := send("POST", "/api/prs/api/5/review/submit", `{"head":"`+diff.Head+`","verdict":"comment","summary":"One question."}`, &st); code != 200 {
		t.Fatalf("submit: %d", code)
	}
	if st.Draft != nil || len(st.Reviews) != 1 || st.Reviews[0].State != review.Submitted || st.URL == "" {
		t.Errorf("after submitting: %+v", st)
	}
	if len(w.posted) != 1 || w.posted[0].Event != github.EventComment || len(w.posted[0].Comments) != 1 || w.posted[0].Commit != diff.Head {
		t.Errorf("posted = %+v", w.posted)
	}
	var d PRDetail
	if getInto(t, w.s, "/api/prs/api/5", &d); d.Reviewed == nil || d.Reviewed.ID != "r1" || d.Since != "" {
		t.Errorf("after submitting, the PR = %+v, since %q", d.Reviewed, d.Since)
	}

	// GitHub's threads, as comments; resolving one posts at once.
	var threads review.Review
	if code := getInto(t, w.s, "/api/prs/api/5/threads", &threads); code != 200 || len(threads.Comments) != 1 || threads.Comments[0].By != "carol" || threads.Comments[0].Kind != "nit" {
		t.Fatalf("threads: %d %+v", code, threads)
	}
	resolved := ""
	w.gh.ResolveThreadFn = func(org, repo string, number int, id string, yes bool) error {
		resolved = fmt.Sprint(id, yes)
		return nil
	}
	if code := send("POST", "/api/prs/api/5/threads/T1/respond", `{"what":"resolve"}`, &threads); code != 200 || resolved != "T1true" {
		t.Errorf("resolve: %d %q", code, resolved)
	}
	if code := send("POST", "/api/prs/api/5/threads/T1/respond", `{"what":"merge"}`, nil); code != http.StatusBadRequest {
		t.Errorf("a response that isn't one: %d", code)
	}
	// A comment kept here isn't answered here once it's submitted: it's GitHub's.
	if code := send("POST", "/api/prs/api/5/review/comments/r1.c1/respond", `{"what":"resolve"}`, nil); code != http.StatusNotFound {
		t.Errorf("responding to a kept comment: %d", code)
	}

	// Asking, and docking: the workspace's to do.
	if code := send("POST", "/api/prs/api/5/ask", `{"body":"Why?"}`, nil); code != http.StatusNotFound {
		t.Errorf("asking with nobody to ask: %d", code)
	}
	// It answers the ask, as the review's ask bar follows it.
	var asked ops.PRQuestion
	fake(w.s).askPR = func(repo string, number int, q ops.PRQuestion) (*conversation.Conversation, error) {
		asked = q
		return conversation.ForWorkspace(w.s.Root).OpenAsk(conversation.Anchor{PR: repo + "#5", Path: q.Path, Side: q.Side, Line: q.Line, EndLine: q.EndLine, Commit: q.Commit}, conversation.You, q.Body, time.Now())
	}
	var answer TalkDetail
	q := fmt.Sprintf(`{"body":"Why two?","path":"a.go","side":"new","line":3,"endLine":3,"commit":%q}`, diff.Head)
	if code := send("POST", "/api/prs/api/5/ask", q, &answer); code != 200 || answer.ID != "w1" || answer.Conversation == nil || !answer.Ephemeral || answer.Anchor.PR != "api#5" || asked.Path != "a.go" || asked.Line != 3 {
		t.Errorf("ask: %d %+v %+v", code, answer, asked)
	}
	for payload, want := range map[string]int{
		`{"body":" "}`: http.StatusUnprocessableEntity,
		`{"body":"x","path":"a.go","side":"new","line":3,"commit":"abc"}`: http.StatusBadRequest,
	} {
		if code := send("POST", "/api/prs/api/5/ask", payload, nil); code != want {
			t.Errorf("ask %s: %d, want %d", payload, code, want)
		}
	}
	fake(w.s).dockPR = func(repo string, number int) (string, error) { return "Docked: there.", nil }
	var docked struct{ Message string }
	if code := send("POST", "/api/prs/api/5/dock", `{}`, &docked); code != 200 || docked.Message != "Docked: there." {
		t.Errorf("dock: %d %+v", code, docked)
	}
}

// An open Reviews page is what keeps the list fresh: its stream says when
// the list has changed.
func TestReviewsEvents(t *testing.T) {
	w := newReviewsWorld(t)
	getInto(t, w.s, "/api/reviews", &ReviewsList{})
	req, _ := http.NewRequest("GET", w.s.Info().Origin()+"/api/events?topic=reviews", nil)
	req.AddCookie(&http.Cookie{Name: w.s.cookieName(), Value: w.s.Info().Token})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	lines := make(chan string, 32)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	wait := func(want string) bool {
		deadline := time.After(3 * time.Second)
		for {
			select {
			case l := <-lines:
				if l == want {
					return true
				}
			case <-deadline:
				return false
			}
		}
	}
	if !wait("event: hello") {
		t.Fatal("no hello")
	}
	w.mu.Lock()
	w.prs[0].IsDraft = true // it moves to another section
	w.mu.Unlock()
	if !wait("event: reviews") {
		t.Error("the page wasn't told the list changed")
	}
}

func TestReviewsWithoutGitHub(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	for _, path := range []string{"/api/reviews", "/api/prs/api/5", "/api/prs/api/5/diff", "/api/prs/api/5/review", "/api/events?topic=reviews"} {
		if res := do(t, s, "GET", path, true, nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s with no GitHub: %d", path, res.StatusCode)
		}
	}
}

// What's said on a pull request itself is read from GitHub, and a comment
// made here posts at once; its description says what names it, for the
// overview's viewed mark.
func TestReviewsDiscussion(t *testing.T) {
	w := newReviewsWorld(t)
	said := []github.Said{{ID: "R1", Author: "carol", State: "APPROVED", Body: "Good.", At: "2026-11-10T09:00:00Z", Commit: w.head}}
	w.gh.DiscussionFn = func(org, repo string, number int) ([]github.Said, error) {
		w.mu.Lock()
		defer w.mu.Unlock()
		return append([]github.Said(nil), said...), nil
	}
	w.gh.CommentFn = func(org, repo string, number int, body string) error {
		w.mu.Lock()
		defer w.mu.Unlock()
		said = append(said, github.Said{ID: "C1", Author: "me", Body: body, At: "2026-11-10T11:00:00Z"})
		return nil
	}
	var d pulls.Discussion
	if code := getInto(t, w.s, "/api/prs/api/5/discussion", &d); code != 200 || len(d.Items) != 1 || d.Items[0].Kind != pulls.SaidApproved || len(d.Reviewers) != 1 {
		t.Fatalf("discussion: %d %+v", code, d)
	}
	req, _ := http.NewRequest("POST", w.s.Info().Origin()+"/api/prs/api/5/discussion", strings.NewReader(`{"body":"Thanks."}`))
	req.AddCookie(&http.Cookie{Name: w.s.cookieName(), Value: w.s.Info().Token})
	req.Header.Set("Origin", w.s.Info().Origin())
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	d = pulls.Discussion{}
	if json.NewDecoder(res.Body).Decode(&d); res.StatusCode != 200 || len(d.Items) != 2 || !d.Items[1].Mine || d.Items[1].Body != "Thanks." {
		t.Errorf("comment: %d %+v", res.StatusCode, d)
	}
	var pr PRDetail
	if code := getInto(t, w.s, "/api/prs/api/5", &pr); code != 200 || pr.Body != "x is two." || pr.BodyID != pulls.BodyID("x is two.") || pr.BodyID == "" {
		t.Errorf("detail: %d body %q id %q", code, pr.Body, pr.BodyID)
	}
}

// A review's passes: listed before any's run, run (cold: nothing's
// docked), what the agent said read as findings on the diff's lines, go or
// no go worked out from them, and each taken into the draft or dismissed.
func TestReviewsPasses(t *testing.T) {
	w := newReviewsWorld(t)
	var asked []string
	w.s.App = testOps(w.s, ops.Options{Reviews: w.s.Reviews, Answer: func(kind, slug, id string) { asked = append(asked, id) }},
		func(c *config.Config, _ *workspace.Workspace) {
			c.Conversation.Harness = "claude"
			c.Reviews.Passes = map[string]config.ReviewPassConfig{
				"security": {Looks: "Find what an attacker could do with it.", Kinds: []string{"vulnerability", "hardening"}, Blocks: []string{"vulnerability"}, Cold: true},
				"web":      {Looks: "x", Repos: []string{"web"}},
			}
		})
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
	list := func() []ReviewPass {
		t.Helper()
		var got struct {
			Passes []ReviewPass `json:"passes"`
		}
		if code := getInto(t, w.s, "/api/prs/api/5/passes", &got); code != 200 {
			t.Fatalf("passes: %d", code)
		}
		return got.Passes
	}
	// The pull request's commits are fetched when it's opened.
	var diff pulls.Diffed
	getInto(t, w.s, "/api/prs/api/5/diff", &diff)

	// One that's for another repo's reviews isn't this one's.
	if ps := list(); len(ps) != 1 || ps[0].Name != "security" || ps[0].Standing.Status != "none" || ps[0].ID != "" || len(ps[0].Blocks) != 1 {
		t.Fatalf("before any's run: %+v", ps)
	}
	if code := send("POST", "/api/prs/api/5/passes/nope/run", "{}", nil); code != http.StatusNotFound {
		t.Errorf("no such pass: %d", code)
	}
	var p ReviewPass
	if code := send("POST", "/api/prs/api/5/passes/security/run", "{}", &p); code != 200 || p.ID == "" || p.Standing.Status != "running" || len(asked) != 1 {
		t.Fatalf("run: %d %+v %v", code, p, asked)
	}
	// Run again before its agent has said anything, it's the same run.
	send("POST", "/api/prs/api/5/passes/security/run", "{}", &p)
	if c, err := conversation.ForWorkspace(w.s.Root).Load(p.ID); err != nil || len(c.Turns) != 1 {
		t.Errorf("asked for twice: %+v, %v", c, err)
	}

	// Its agent's reply, as ws reads it: a finding on the diff's lines, and
	// one on none that blocks.
	store := conversation.ForWorkspace(w.s.Root)
	summary, fs := conversation.ParseReviewFindings("x is two, and shouldn't be.\n\n## hardening\n@ a.go:3\nSay why two.\n\n```suggestion\nvar x = 2 // two\n```\n\n## vulnerability\nThis whole pull request is a bad idea.\n")
	fs[0].Commit, fs[0].Excerpt = diff.Head, []string{"var x = 2"}
	conversation.KeyFindings(fs)
	if _, err := store.Say(p.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: summary, Findings: fs, Around: diff.Head}); err != nil {
		t.Fatal(err)
	}
	sec := list()[0]
	if sec.Standing.Status != "nogo" || sec.Standing.Open != 2 || sec.Standing.Blocking != 1 || sec.Summary != summary || sec.Ran != diff.Head || sec.Moved {
		t.Fatalf("after its run: %+v", sec)
	}
	if len(sec.Findings) != 2 || sec.Findings[0].Blocks || !sec.Findings[1].Blocks || sec.Findings[0].Path != "a.go" || sec.Findings[1].Path != "" {
		t.Fatalf("findings: %+v", sec.Findings)
	}

	// Taken: the one on lines is a comment there in your draft, as the
	// agent's, with its suggestion; nothing's posted.
	if code := send("POST", "/api/prs/api/5/passes/security/findings/"+sec.Findings[0].Key, `{"state":"taken"}`, &p); code != 200 || p.Findings[0].State != "done" || p.Standing.Open != 1 {
		t.Fatalf("take: %d %+v", code, p)
	}
	var st ReviewState
	getInto(t, w.s, "/api/prs/api/5/review", &st)
	if st.Draft == nil || len(st.Draft.Comments) != 1 {
		t.Fatalf("the draft: %+v", st.Draft)
	}
	if n := st.Draft.Comments[0]; n.Path != "a.go" || n.Line != 3 || n.Kind != "suggestion" || n.By != conversation.Agent || n.Body != "Say why two." || n.Suggestion == nil || *n.Suggestion != "var x = 2 // two" {
		t.Errorf("the comment: %+v", n)
	}
	// Taken again, it's not added twice.
	send("POST", "/api/prs/api/5/passes/security/findings/"+sec.Findings[0].Key, `{"state":"taken"}`, &p)
	if getInto(t, w.s, "/api/prs/api/5/review", &st); len(st.Draft.Comments) != 1 {
		t.Errorf("taken twice: %d comments", len(st.Draft.Comments))
	}

	// Dismissed, the blocker's overruled, and it's go; open again, no go.
	key := sec.Findings[1].Key
	if code := send("POST", "/api/prs/api/5/passes/security/findings/"+key, `{"state":"dismissed","why":"wrong"}`, &p); code != 200 || p.Standing.Status != "go" || p.Standing.Overruled != 1 || p.Findings[1].Why != "wrong" {
		t.Fatalf("dismiss: %d %+v", code, p)
	}
	if code := send("POST", "/api/prs/api/5/passes/security/findings/"+key, `{"state":"open"}`, &p); code != 200 || p.Standing.Status != "nogo" {
		t.Fatalf("open again: %d %+v", code, p.Standing)
	}
	if code := send("POST", "/api/prs/api/5/passes/security/findings/"+key, `{"state":"maybe"}`, nil); code != http.StatusBadRequest {
		t.Errorf("a state that isn't one: %d", code)
	}

	// Taken, one on no lines is a comment on the change as a whole, which
	// goes in the review's body, as blocking: its kind blocks.
	if code := send("POST", "/api/prs/api/5/passes/security/findings/"+key, `{"state":"taken"}`, &p); code != 200 || p.Standing.Status != "go" {
		t.Fatalf("take the one on no lines: %d %+v", code, p.Standing)
	}
	getInto(t, w.s, "/api/prs/api/5/review", &st)
	if len(st.Draft.Comments) != 2 || st.Draft.Comments[1].Path != "" || st.Draft.Comments[1].Kind != "blocking" {
		t.Fatalf("the draft: %+v", st.Draft.Comments)
	}
	var posting pulls.Posting
	if code := send("POST", "/api/prs/api/5/review/preview", `{"head":"`+diff.Head+`","verdict":"changes","summary":"Not yet."}`, &posting); code != 200 {
		t.Fatalf("preview: %d", code)
	}
	if posting.Body != "Not yet.\n\nissue (blocking): This whole pull request is a bad idea." || len(posting.Comments) != 1 || posting.Loose != 0 {
		t.Errorf("posting: %+v", posting)
	}
}

// Follow-up is every review's, once you've reviewed: it runs over your
// open comments, what it says of each is on that comment's thread, a
// blocking comment not dealt with is a no go, and resolving the thread
// settles it.
func TestReviewsFollowUp(t *testing.T) {
	w := newReviewsWorld(t)
	var asked []string
	w.s.App = testOps(w.s, ops.Options{Reviews: w.s.Reviews, Answer: func(kind, slug, id string) { asked = append(asked, id) }},
		func(c *config.Config, _ *workspace.Workspace) { c.Conversation.Harness = "claude" })
	send := func(path, payload string, v any) int {
		t.Helper()
		req, _ := http.NewRequest("POST", w.s.Info().Origin()+path, strings.NewReader(payload))
		req.AddCookie(&http.Cookie{Name: w.s.cookieName(), Value: w.s.Info().Token})
		req.Header.Set("Origin", w.s.Info().Origin())
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if v != nil && res.StatusCode == 200 {
			json.NewDecoder(res.Body).Decode(v)
		}
		return res.StatusCode
	}
	list := func() []ReviewPass {
		t.Helper()
		var got struct {
			Passes []ReviewPass `json:"passes"`
		}
		getInto(t, w.s, "/api/prs/api/5/passes", &got)
		return got.Passes
	}
	var diff pulls.Diffed
	getInto(t, w.s, "/api/prs/api/5/diff", &diff)

	// You haven't reviewed it: there's nothing to follow up.
	if ps := list(); len(ps) != 0 {
		t.Fatalf("before you've reviewed: %+v", ps)
	}
	w.mu.Lock()
	w.prs[0].Latest = []github.LatestReview{{Author: "me", State: "CHANGES_REQUESTED", SubmittedAt: "2026-11-10T09:00:00Z", Commit: w.head}}
	w.mu.Unlock()
	resolved := map[string]bool{}
	w.gh.ReviewThreadsFn = func(org, repo string, number int) ([]github.ReviewThread, error) {
		w.mu.Lock()
		defer w.mu.Unlock()
		return []github.ReviewThread{
			{ID: "T1", Path: "a.go", Line: 3, Side: "RIGHT", Resolved: resolved["T1"], Comments: []github.ThreadComment{
				{Author: "me", Body: "issue (blocking): why two?", At: "2026-11-10T09:00:00Z", Commit: w.head, OriginalCommit: w.head}, {Author: "alice", Body: "Because.", At: "2026-11-10T09:30:00Z"}}},
			{ID: "T2", Path: "a.go", Line: 3, Side: "RIGHT", Comments: []github.ThreadComment{{Author: "carol", Body: "nitpick: three?", At: "2026-11-10T09:00:00Z", Commit: w.head, OriginalCommit: w.head}}},
		}, nil
	}
	w.gh.ResolveThreadFn = func(org, repo string, number int, id string, yes bool) error {
		w.mu.Lock()
		defer w.mu.Unlock()
		resolved[id] = yes
		return nil
	}
	w.s.Reviews.Stale() // GitHub's asked again
	if _, err := w.s.Reviews.List(false); err != nil {
		t.Fatal(err)
	}
	if ps := list(); len(ps) != 1 || ps[0].Name != "followup" || ps[0].Standing.Status != "none" {
		t.Fatalf("once you've reviewed: %+v", ps)
	}

	// Run: nothing's docked for it.
	var p ReviewPass
	if code := send("/api/prs/api/5/passes/followup/run", "{}", &p); code != 200 || p.ID == "" || len(asked) != 1 {
		t.Fatalf("run: %d %+v", code, p)
	}
	store := conversation.ForWorkspace(w.s.Root)
	fs := []conversation.Finding{{Kind: "not-addressed", Thread: "T1", Path: "a.go", Side: "new", Line: 3, EndLine: 3, Text: "Nothing's been pushed.", Blocks: true}}
	conversation.KeyFindings(fs)
	store.Say(p.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "None of one is dealt with.", Findings: fs, Around: diff.Head})
	if f := list()[0]; f.Standing.Status != "nogo" || len(f.Findings) != 1 || f.Findings[0].Thread != "T1" || !f.Findings[0].Blocks {
		t.Fatalf("after its run: %+v", f)
	}
	// It's about a comment already made: there's nothing to take.
	if code := send("/api/prs/api/5/passes/followup/findings/"+fs[0].Key, `{"state":"taken"}`, nil); code != http.StatusUnprocessableEntity {
		t.Errorf("take: %d", code)
	}
	// Resolving its thread settles it; reopening opens it again.
	if code := send("/api/prs/api/5/threads/T1/respond", `{"what":"resolve"}`, nil); code != 200 {
		t.Fatalf("resolve: %d", code)
	}
	if f := list()[0]; f.Standing.Status != "go" || f.Findings[0].State != "done" {
		t.Errorf("resolved: %+v", f)
	}
	send("/api/prs/api/5/threads/T1/respond", `{"what":"reopen"}`, nil)
	if f := list()[0]; f.Standing.Status != "nogo" {
		t.Errorf("reopened: %+v", f.Standing)
	}
	// With no comment of yours open, there's nothing to run it over.
	send("/api/prs/api/5/threads/T1/respond", `{"what":"resolve"}`, nil)
	store.Say(p.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "x", Around: diff.Head})
	if code := send("/api/prs/api/5/passes/followup/run", "{}", nil); code != http.StatusUnprocessableEntity {
		t.Errorf("run with nothing open: %d", code)
	}
}

// The guide is asked for, written (cold: nothing's docked), and read as
// what the change does and its files in groups; it says when the pull
// request has moved on since.
func TestReviewsGuide(t *testing.T) {
	w := newReviewsWorld(t)
	var asked []string
	w.s.App = testOps(w.s, ops.Options{Reviews: w.s.Reviews, Answer: func(kind, slug, id string) { asked = append(asked, id) }},
		func(c *config.Config, _ *workspace.Workspace) { c.Conversation.Harness = "claude" })
	var diff pulls.Diffed
	getInto(t, w.s, "/api/prs/api/5/diff", &diff)
	var g ReviewGuide
	if code := getInto(t, w.s, "/api/prs/api/5/guide", &g); code != 200 || g.Guide != nil || g.ID != "" || g.Running {
		t.Fatalf("before it's asked for: %d %+v", code, g)
	}
	req, _ := http.NewRequest("POST", w.s.Info().Origin()+"/api/prs/api/5/guide/run", strings.NewReader("{}"))
	req.AddCookie(&http.Cookie{Name: w.s.cookieName(), Value: w.s.Info().Token})
	req.Header.Set("Origin", w.s.Info().Origin())
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	json.NewDecoder(res.Body).Decode(&g)
	res.Body.Close()
	if res.StatusCode != 200 || g.ID == "" || !g.Running || len(asked) != 1 {
		t.Fatalf("run: %d %+v %v", res.StatusCode, g, asked)
	}
	// It isn't one of the review's passes.
	var ps struct {
		Passes []ReviewPass `json:"passes"`
	}
	if getInto(t, w.s, "/api/prs/api/5/passes", &ps); len(ps.Passes) != 0 {
		t.Errorf("passes: %+v", ps.Passes)
	}
	store := conversation.ForWorkspace(w.s.Root)

	// While it's written: since when, what it was given, and what it's
	// written so far, read as a guide, its files those it was given.
	store.StartAnswering(g.ID, os.Getpid())
	if getInto(t, w.s, "/api/prs/api/5/guide", &g); !g.Running || g.Since == nil || g.Given != nil || g.Draft != nil {
		t.Fatalf("started: %+v", g)
	}
	store.SetGuideGiven(g.ID, conversation.GuideGiven{Files: []string{"a.go", "go.sum"}, Left: []conversation.GuideLeft{{Path: "go.sum", Why: "lockfile"}}})
	thinking := harness.RunHeader(harness.StreamClaude) + `{"type":"stream_event","event":{"type":"message_start","message":{"content":[]}}}` + "\n" +
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}}` + "\n"
	os.WriteFile(store.RunPath(g.ID), []byte(thinking), 0o644)
	if getInto(t, w.s, "/api/prs/api/5/guide", &g); !g.Thinking || g.Draft != nil || g.Given == nil {
		t.Fatalf("thinking: %+v", g)
	}
	delta := func(text string) string {
		b, _ := json.Marshal(map[string]any{"type": "stream_event", "event": map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]string{"type": "text_delta", "text": text}}})
		return string(b) + "\n"
	}
	run := harness.RunHeader(harness.StreamClaude) + `{"type":"stream_event","event":{"type":"message_start","message":{"content":[]}}}` + "\n" +
		delta("x is two.\n\n## differs\n- Nothing.\n\n## read: The change\n- b.go: not given\n- a.go: one bec")
	os.WriteFile(store.RunPath(g.ID), []byte(run), 0o644)
	g = ReviewGuide{}
	if getInto(t, w.s, "/api/prs/api/5/guide", &g); !g.Running || g.Thinking || g.Guide != nil || g.Given == nil || len(g.Given.Files) != 2 || g.Given.Left[0].Why != "lockfile" ||
		g.Draft == nil || g.Draft.Does != "x is two." || len(g.Draft.Groups) != 1 || len(g.Draft.Groups[0].Files) != 1 || g.Draft.Groups[0].Files[0].Note != "one bec" {
		t.Fatalf("part way: %+v %+v", g, g.Draft)
	}
	store.ClearRun(g.ID)
	store.EndAnswering(g.ID)

	guide := conversation.ParseGuide("x is two.\n\n## differs\n- Nothing.\n\n## read: The change\n- a.go: one becomes two\n", func(p string) bool { return p == "a.go" })
	store.Say(g.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: guide.Does, Guide: guide, Around: diff.Head})
	g = ReviewGuide{}
	if getInto(t, w.s, "/api/prs/api/5/guide", &g); g.Guide == nil || g.Guide.Does != "x is two." || len(g.Guide.Groups) != 1 || g.Guide.Groups[0].Files[0].Path != "a.go" || g.Running || g.Moved || g.Ran != diff.Head || g.Draft != nil || g.Given != nil || g.Since != nil {
		t.Errorf("written: %+v", g)
	}
}

// A pass set to run by itself runs over a pull request that needs you,
// one at a time, again once it's been pushed to, and not again after it's
// failed; how it stands is on the list's row.
func TestReviewsAutoPasses(t *testing.T) {
	w := newReviewsWorld(t)
	var asked []string
	app := testOps(w.s, ops.Options{Reviews: w.s.Reviews, Answer: func(kind, slug, id string) { asked = append(asked, id) }},
		func(c *config.Config, _ *workspace.Workspace) {
			c.Conversation.Harness = "claude"
			c.Reviews.Guide.Auto = true
			c.Reviews.Passes = map[string]config.ReviewPassConfig{
				"check":  {Looks: "x", Cold: true, Auto: true},
				"manual": {Looks: "x", Cold: true},
			}
		})
	w.s.App = app
	var diff pulls.Diffed
	getInto(t, w.s, "/api/prs/api/5/diff", &diff)
	store := conversation.ForWorkspace(w.s.Root)

	// The guide first, and nothing else while it's being written.
	if said := app.AutoPasses(); said != "guide over api#5" || len(asked) != 1 {
		t.Fatalf("first: %q %v", said, asked)
	}
	if said := app.AutoPasses(); said != "" {
		t.Errorf("while one's running: %q", said)
	}
	g := store.ReviewPassOn("api#5", "guide")
	store.Say(g.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "x", Guide: &conversation.ReviewGuide{Does: "x", Groups: []conversation.GuideGroup{{Name: "It", Files: []conversation.GuideFile{{Path: "a.go"}}}}}, Around: w.head})

	// Then the pass that's auto; the one that isn't is left to you.
	if said := app.AutoPasses(); said != "check over api#5" {
		t.Fatalf("second: %q", said)
	}
	c := store.ReviewPassOn("api#5", "check")
	fs := []conversation.Finding{{Kind: "blocking", Text: "No."}}
	conversation.KeyFindings(fs)
	store.Say(c.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "No.", Findings: fs, Around: w.head})
	if said := app.AutoPasses(); said != "" {
		t.Errorf("with each run at its head: %q", said)
	}

	// The list says how it stands, and the guide isn't a pass on a row.
	var list ReviewsList
	getInto(t, w.s, "/api/reviews", &list)
	if m := list.Passes["api#5"]; len(m) != 1 || m[0].Name != "check" || m[0].Standing.Status != "nogo" || m[0].Moved {
		t.Errorf("on the row: %+v", list.Passes)
	}

	// Pushed to: they're run again, and the row says what's there is of
	// the commit before.
	w.mu.Lock()
	w.prs[0].HeadRefOid = strings.Repeat("b", 40)
	w.mu.Unlock()
	w.s.Reviews.Stale()
	w.s.Reviews.List(false)
	getInto(t, w.s, "/api/reviews", &list)
	if m := list.Passes["api#5"]; len(m) != 1 || !m[0].Moved {
		t.Errorf("after a push: %+v", list.Passes)
	}
	if said := app.AutoPasses(); said != "guide over api#5" {
		t.Errorf("after a push: %q", said)
	}
	// It fails: it's had its go.
	store.Say(g.ID, conversation.Turn{By: conversation.WS, At: time.Now(), Body: "The agent couldn't answer."})
	if said := app.AutoPasses(); said != "check over api#5" {
		t.Errorf("after the guide failed: %q", said)
	}
	store.Say(c.ID, conversation.Turn{By: conversation.WS, At: time.Now(), Body: "The agent couldn't answer."})
	if said := app.AutoPasses(); said != "" {
		t.Errorf("after both failed: %q", said)
	}

	// A bot's pull request is never run over.
	w.mu.Lock()
	w.prs[0].Bot = true
	w.mu.Unlock()
	w.s.Reviews.Stale()
	w.s.Reviews.List(false)
	store.Say(c.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "x", Around: w.head})
	if said := app.AutoPasses(); said != "" {
		t.Errorf("a bot's: %q", said)
	}
}

// A pull request's page follows its own topic: what's shown is kept, and
// read again behind, and the page is told when that finds it changed.
func TestPREvents(t *testing.T) {
	w := newReviewsWorld(t)
	var said atomic.Value
	said.Store("nitpick: three?")
	w.gh.ReviewThreadsFn = func(org, repo string, number int) ([]github.ReviewThread, error) {
		return []github.ReviewThread{{ID: "T1", Path: "a.go", Line: 3, Side: "RIGHT", Comments: []github.ThreadComment{
			{Author: "carol", Body: said.Load().(string), At: "2026-11-10T09:00:00Z", Commit: w.head, OriginalCommit: w.head}}}}, nil
	}
	var threads review.Review
	if code := getInto(t, w.s, "/api/prs/api/5/threads", &threads); code != 200 || len(threads.Comments) != 1 {
		t.Fatalf("threads: %d %+v", code, threads)
	}
	req, _ := http.NewRequest("GET", w.s.Info().Origin()+"/api/events?topic="+url.QueryEscape("pr:api#5"), nil)
	req.AddCookie(&http.Cookie{Name: w.s.cookieName(), Value: w.s.Info().Token})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("stream: %d", res.StatusCode)
	}
	lines := make(chan string, 32)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	wait := func(want string, within time.Duration) bool {
		deadline := time.After(within)
		for {
			select {
			case l := <-lines:
				if l == want {
					return true
				}
			case <-deadline:
				return false
			}
		}
	}
	if !wait("event: hello", 3*time.Second) {
		t.Fatal("no hello")
	}
	if wait("event: pr:api#5", 300*time.Millisecond) {
		t.Error("told of a change when nothing had changed")
	}
	said.Store("nitpick: three, surely?")
	if !wait("event: pr:api#5", 3*time.Second) {
		t.Fatal("the page wasn't told its threads changed")
	}
	if code := getInto(t, w.s, "/api/prs/api/5/threads", &threads); code != 200 || threads.Comments[0].Body != "three, surely?" {
		t.Errorf("threads after: %d %+v", code, threads.Comments)
	}
	for _, topic := range []string{"pr:api", "pr:api#x", "pr:../x#1"} {
		if code := do(t, w.s, "GET", "/api/events?topic="+url.QueryEscape(topic), true, nil).StatusCode; code != http.StatusNotFound {
			t.Errorf("topic %s: %d, want 404", topic, code)
		}
	}
}
