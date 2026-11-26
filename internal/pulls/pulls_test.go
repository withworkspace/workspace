package pulls

import (
	"fmt"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/testgit"
)

func TestMain(m *testing.M) {
	cleanup := testgit.NoSigning()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// world is a workspace with one repo, api, whose remote has a stack of
// three PRs (88 storage, 89 tenant on it, 90 revoke on that) and one PR on
// its own (92), each also at refs/pull/<n>/head as GitHub keeps them.
type world struct {
	t      *testing.T
	root   string
	remote string
	bare   string
	gh     *github.Stub
	prs    []github.PR
	stacks []github.Stack
	svc    *Service
	now    time.Time
}

// acme is the test's GitHub: every repo is acme's, by its name here.
func acme(repo string) ref.Remote { return ref.Remote{Owner: "acme", Name: repo} }

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{t: t, root: t.TempDir(), now: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
	storetest.Own(t, w.root)
	w.remote = filepath.Join(w.root, "remote")
	os.MkdirAll(w.remote, 0o755)
	gitT(t, w.remote, "init", "-q", "--initial-branch=main")
	gitT(t, w.remote, "config", "user.name", "Test")
	gitT(t, w.remote, "config", "user.email", "test@example.com")
	write(t, w.remote, "cache.go", "package cache\n\nvar ttl = 60\n")
	write(t, w.remote, "README.md", "# api\n")
	gitT(t, w.remote, "add", "-A")
	gitT(t, w.remote, "commit", "-q", "-m", "base")

	w.branch("storage", "main", 88, "store.go", "package cache\n\ntype Store struct{}\n")
	w.branch("tenant", "storage", 89, "tenant.go", "package cache\n\nfunc key(tenant string) string { return tenant }\n")
	w.branch("revoke", "tenant", 90, "revoke.go", "package cache\n\nfunc revoke() {}\n")
	w.branch("typo", "main", 92, "README.md", "# api\n\nThe API.\n")
	gitT(t, w.remote, "checkout", "-q", "main")

	w.bare = filepath.Join(w.root, "repos", "api", ".bare")
	os.MkdirAll(filepath.Dir(w.bare), 0o755)
	if out, err := exec.Command("git", "clone", "-q", "--bare", w.remote, w.bare).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	gitT(t, w.bare, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	gitT(t, w.bare, "fetch", "-q", "origin")

	at := "2026-11-10T10:00:00Z"
	w.prs = []github.PR{
		{Number: 88, Title: "Storage layer", HeadRefName: "storage", BaseRefName: "main", Author: "alice", UpdatedAt: at, Additions: 3,
			Latest: []github.LatestReview{{Author: "me", State: "APPROVED", SubmittedAt: "2026-11-09T10:00:00Z"}}},
		{Number: 89, Title: "Key by tenant", HeadRefName: "tenant", BaseRefName: "storage", Author: "alice", UpdatedAt: at, Additions: 3,
			Requested: []github.ReviewRequest{{Login: "me"}}},
		{Number: 90, Title: "Invalidate on revoke", HeadRefName: "revoke", BaseRefName: "tenant", Author: "alice", UpdatedAt: at, Additions: 3},
		{Number: 92, Title: "Fix a typo", HeadRefName: "typo", BaseRefName: "main", Author: "bob", UpdatedAt: "2026-11-10T09:00:00Z", Mergeable: "CONFLICTING"},
	}
	for i := range w.prs {
		w.prs[i].HeadRefOid = gitT(t, w.remote, "rev-parse", w.prs[i].HeadRefName)
		w.prs[i].URL = fmt.Sprintf("https://github.com/acme/api/pull/%d", w.prs[i].Number)
	}
	w.prs[0].Latest[0].Commit = w.prs[0].HeadRefOid
	w.stacks = []github.Stack{{Number: 3, Base: "main", Open: true, PRs: []github.StackPR{{Number: 88}, {Number: 89}, {Number: 90}}}}
	w.gh = &github.Stub{Me: "me",
		PRsForRepoFn:    func(org, repo string) ([]github.PR, error) { return w.prs, nil },
		StacksForRepoFn: func(org, repo string) ([]github.Stack, error) { return w.stacks, nil },
	}
	w.svc = &Service{Root: w.root, Remote: acme, GitHub: w.gh,
		Repos:   func() []string { return []string{"api"} },
		BareDir: func(repo string) string { return filepath.Join(w.root, "repos", repo, ".bare") },
		Now:     func() time.Time { return w.now },
	}
	return w
}

// branch makes a branch off another in the remote with one file changed,
// and points refs/pull/<n>/head at it.
func (w *world) branch(name, from string, number int, file, text string) {
	w.t.Helper()
	gitT(w.t, w.remote, "checkout", "-q", "-b", name, from)
	write(w.t, w.remote, file, text)
	gitT(w.t, w.remote, "add", "-A")
	gitT(w.t, w.remote, "commit", "-q", "-m", name)
	gitT(w.t, w.remote, "update-ref", fmt.Sprintf("refs/pull/%d/head", number), "HEAD")
}

// push commits a change to a PR's branch in the remote, and tells GitHub.
func (w *world) push(number int, file, text string) string {
	w.t.Helper()
	pr := w.pr(number)
	gitT(w.t, w.remote, "checkout", "-q", pr.HeadRefName)
	write(w.t, w.remote, file, text)
	gitT(w.t, w.remote, "add", "-A")
	gitT(w.t, w.remote, "commit", "-q", "-m", "more "+pr.HeadRefName)
	return w.moved(number)
}

// moved records a PR's branch as GitHub would after a push.
func (w *world) moved(number int) string {
	w.t.Helper()
	pr := w.pr(number)
	pr.HeadRefOid = gitT(w.t, w.remote, "rev-parse", pr.HeadRefName)
	gitT(w.t, w.remote, "update-ref", fmt.Sprintf("refs/pull/%d/head", number), pr.HeadRefOid)
	w.now = w.now.Add(2 * time.Minute) // the list is asked for again
	return pr.HeadRefOid
}

func (w *world) pr(number int) *github.PR {
	for i := range w.prs {
		if w.prs[i].Number == number {
			return &w.prs[i]
		}
	}
	w.t.Fatalf("no PR %d", number)
	return nil
}

func section(l *List, key string) *Section {
	for i := range l.Sections {
		if l.Sections[i].Key == key {
			return &l.Sections[i]
		}
	}
	return nil
}

func TestListSortsByWhatEachAsksOfYou(t *testing.T) {
	w := newWorld(t)
	l, err := w.svc.List(false)
	if err != nil {
		t.Fatal(err)
	}
	// The stack is one row, where its most pressing layer is.
	needs := section(l, NeedsReview)
	if needs == nil || len(needs.Rows) != 1 || needs.Rows[0].Stack == nil {
		t.Fatalf("needs your review = %+v", needs)
	}
	st := needs.Rows[0].Stack
	if st.Title != "Storage layer" || st.Number != 3 || len(st.Layers) != 3 || st.WorkedOut {
		t.Errorf("stack = %+v", st)
	}
	if got := []string{st.Layers[0].State, st.Layers[1].State, st.Layers[2].State}; got[0] != Approved || got[1] != NeedsReview || got[2] != Others {
		t.Errorf("layers' states = %v", got)
	}
	if st.Open != 89 {
		t.Errorf("opens at #%d, want the lowest layer that needs you (89)", st.Open)
	}
	if st.Layers[1].Asked != "you" {
		t.Errorf("asked = %q", st.Layers[1].Asked)
	}
	others := section(l, Others)
	if others == nil || len(others.Rows) != 1 || others.Rows[0].PR == nil || others.Rows[0].PR.Number != 92 {
		t.Fatalf("everyone else's = %+v", others)
	}
	// Whether each merges cleanly, as GitHub says: unknown until it has.
	if got := others.Rows[0].PR.Merges; got != "conflicts" {
		t.Errorf("#92 merges = %q, want conflicts", got)
	}
	if got := st.Layers[2].Merges; got != "" {
		t.Errorf("#90 merges = %q, want unknown (\"\")", got)
	}
	if l.Count != 1 || l.Me != "me" {
		t.Errorf("count = %d, me = %q", l.Count, l.Me)
	}
	// Kept between asks, and on disk for the next daemon.
	w.gh.PRsForRepoFn = func(org, repo string) ([]github.PR, error) { t.Error("asked GitHub again"); return nil, nil }
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	again := &Service{Root: w.root}
	if p := again.Peek(); p == nil || p.Count != 1 {
		t.Errorf("the list wasn't kept: %+v", p)
	}
	// Once something's changed what a PR asks of you, GitHub is asked
	// again, however lately it was; until then the list says when it's of.
	asked := 0
	w.gh.PRsForRepoFn = func(org, repo string) ([]github.PR, error) { asked++; return w.prs, nil }
	at := l.At
	w.svc.Stale()
	if p := w.svc.Peek(); p == nil || !p.At.Equal(at) {
		t.Errorf("a stale list lost when it's of: %+v", p)
	}
	if _, _, err := w.svc.Known("api", 89); err != nil || asked != 0 {
		t.Errorf("finding a PR that's in the list asked GitHub: %d, %v", asked, err)
	}
	if _, err := w.svc.List(false); err != nil || asked != 1 {
		t.Errorf("after going stale, GitHub was asked %d times (%v)", asked, err)
	}
}

func TestClassify(t *testing.T) {
	head := "h2"
	base := github.PR{Author: "alice", HeadRefOid: head, UpdatedAt: "2026-11-10T10:00:00Z"}
	mine := func(state, commit, at string) []github.LatestReview {
		return []github.LatestReview{{Author: "Me", State: state, Commit: commit, SubmittedAt: at}}
	}
	cases := []struct {
		name  string
		pr    func(p *github.PR)
		asked bool
		want  string
	}{
		{"nobody asked", func(p *github.PR) {}, false, Others},
		{"asked by name", func(p *github.PR) { p.Requested = []github.ReviewRequest{{Login: "me"}} }, false, NeedsReview},
		{"asked of a team", func(p *github.PR) { p.Requested = []github.ReviewRequest{{Team: "backend"}} }, true, NeedsReview},
		{"a draft that asks", func(p *github.PR) { p.IsDraft = true; p.Requested = []github.ReviewRequest{{Login: "me"}} }, false, Drafts},
		{"approved", func(p *github.PR) { p.Latest = mine("APPROVED", head, "2026-11-10T10:00:00Z") }, false, Approved},
		{"changes asked, nothing since", func(p *github.PR) { p.Latest = mine("CHANGES_REQUESTED", head, "2026-11-10T09:59:30Z") }, false, WaitingOnThem},
		{"changes asked, pushed since", func(p *github.PR) { p.Latest = mine("CHANGES_REQUESTED", "h1", "2026-11-10T09:00:00Z") }, false, BackWithYou},
		{"commented, replied since", func(p *github.PR) { p.Latest = mine("COMMENTED", head, "2026-11-10T09:00:00Z") }, false, BackWithYou},
		{"asked again after a review", func(p *github.PR) {
			p.Latest = mine("CHANGES_REQUESTED", "h1", "2026-11-10T09:00:00Z")
			p.Requested = []github.ReviewRequest{{Login: "me"}}
		}, false, NeedsReview},
		{"dismissed", func(p *github.PR) { p.Latest = mine("DISMISSED", "h1", "2026-11-10T09:00:00Z") }, false, BackWithYou},
		{"a bot's", func(p *github.PR) { p.Bot = true }, false, Automated},
		{"a bot's that asks", func(p *github.PR) { p.Bot = true }, true, NeedsReview},
		{"someone else's review", func(p *github.PR) {
			p.Latest = []github.LatestReview{{Author: "carol", State: "APPROVED", Commit: head}}
		}, false, Others},
		{"yours, changes requested", func(p *github.PR) { p.Author = "me"; p.ReviewDecision = "CHANGES_REQUESTED" }, false, Returned},
		{"yours, checks failing", func(p *github.PR) { p.Author = "me"; p.ReviewDecision = "APPROVED"; p.StatusRollup = "failure" }, false, Returned},
		{"yours, approved", func(p *github.PR) { p.Author = "me"; p.ReviewDecision = "APPROVED"; p.StatusRollup = "success" }, false, Ready},
		{"yours, approved, and it conflicts", func(p *github.PR) {
			p.Author = "me"
			p.ReviewDecision, p.StatusRollup, p.Mergeable = "APPROVED", "success", "CONFLICTING"
		}, false, Returned},
		{"yours, no review needed, and it conflicts", func(p *github.PR) { p.Author = "me"; p.Mergeable = "CONFLICTING" }, false, Returned},
		{"yours, approved, GitHub still working out whether it merges", func(p *github.PR) {
			p.Author = "me"
			p.ReviewDecision, p.Mergeable = "APPROVED", "UNKNOWN"
		}, false, WaitingReview},
		{"yours, no review needed, GitHub still working out whether it merges", func(p *github.PR) { p.Author = "me"; p.Mergeable = "UNKNOWN" }, false, WaitingReview},
		{"yours, approved, and GitHub says it merges", func(p *github.PR) {
			p.Author = "me"
			p.ReviewDecision, p.Mergeable = "APPROVED", "MERGEABLE"
		}, false, Ready},
		{"yours, no review needed", func(p *github.PR) { p.Author = "me" }, false, Ready},
		{"yours, no review decision, but its rules block it (an admin could bypass)", func(p *github.PR) {
			p.Author = "me"
			p.StatusRollup, p.Mergeable, p.MergeState = "success", "MERGEABLE", "BLOCKED"
		}, false, WaitingReview},
		{"yours, approved, but its rules still block it", func(p *github.PR) {
			p.Author = "me"
			p.ReviewDecision, p.Mergeable, p.MergeState = "APPROVED", "MERGEABLE", "BLOCKED"
		}, false, WaitingReview},
		{"yours, approved, GitHub says it's clean", func(p *github.PR) {
			p.Author = "me"
			p.ReviewDecision, p.Mergeable, p.MergeState = "APPROVED", "MERGEABLE", "CLEAN"
		}, false, Ready},
		{"yours, reviewers asked", func(p *github.PR) { p.Author = "me"; p.Requested = []github.ReviewRequest{{Login: "alice"}} }, false, WaitingReview},
		{"yours, review required", func(p *github.PR) { p.Author = "me"; p.ReviewDecision = "REVIEW_REQUIRED" }, false, WaitingReview},
		{"yours, checks running", func(p *github.PR) { p.Author = "me"; p.ReviewDecision = "APPROVED"; p.StatusRollup = "pending" }, false, WaitingReview},
		{"yours, a draft", func(p *github.PR) { p.Author = "me"; p.IsDraft = true }, false, WaitingReview},
	}
	for _, c := range cases {
		p := base
		c.pr(&p)
		if got, words, _, _ := classify(p, "me", c.asked); got != c.want {
			t.Errorf("%s: %s (%s), want %s", c.name, got, words, c.want)
		}
	}
}

func TestStacksWorkedOut(t *testing.T) {
	pr := func(n int, head, base string) github.PR {
		return github.PR{Number: n, Title: fmt.Sprintf("PR %d", n), HeadRefName: head, BaseRefName: base, Author: "alice", UpdatedAt: "2026-11-10T10:00:00Z"}
	}
	// 1 ← 2 ← 3 is a chain; 4 and 5 both sit on 3; 6 is on a layer of
	// GitHub's stack (7 ← 8); 9 is alone; 10 is a fork's, on 1's branch name.
	fork := pr(10, "a", "main")
	fork.IsCrossRepository = true
	prs := []github.PR{pr(1, "a", "main"), pr(2, "b", "a"), pr(3, "c", "b"), pr(4, "d", "c"), pr(5, "e", "c"), pr(6, "f", "h"),
		pr(8, "h", "g"), pr(7, "g", "main"), pr(9, "i", "main"), fork}
	stacks := []github.Stack{{Number: 1, Base: "main", Open: true, PRs: []github.StackPR{{Number: 5000, Merged: true}, {Number: 8}, {Number: 7}}}}
	l := build("me", []repoPRs{{repo: "api", prs: prs, stacks: stacks}}, nil, Local{})
	got := map[string]string{}
	for _, s := range l.Sections {
		for _, r := range s.Rows {
			if r.Stack != nil {
				var ns []string
				for _, p := range r.Stack.Layers {
					ns = append(ns, fmt.Sprint(p.Number))
				}
				got[r.Key] = fmt.Sprintf("stack %s worked-out=%v merged=%d on=%d", strings.Join(ns, ","), r.Stack.WorkedOut, r.Stack.Merged, r.Stack.OnTopOf)
			} else {
				got[r.Key] = fmt.Sprintf("pr on=%d", r.PR.OnTopOf)
			}
		}
	}
	want := map[string]string{
		"api/stack-1": "stack 7,8 worked-out=false merged=1 on=0", // ordered by what each is based on
		"api/chain-1": "stack 1,2,3 worked-out=true merged=0 on=0",
		"api#4":       "pr on=3",
		"api#5":       "pr on=3",
		"api#6":       "pr on=8",
		"api#9":       "pr on=0",
		"api#10":      "pr on=0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rows = %v", got)
	}
}

func TestLayerShowsOnlyItsOwnChanges(t *testing.T) {
	w := newWorld(t)
	d, err := w.svc.Detail("api", 89)
	if err != nil {
		t.Fatal(err)
	}
	if d.Layer != 2 || d.Stack == nil || d.BaseRef != "origin/storage" || d.NeedsRebase || d.Head != w.pr(89).HeadRefOid {
		t.Errorf("detail = %+v", d)
	}
	diff, err := w.svc.Diff("api", 89, ScopeLayer)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Files) != 1 || diff.Files[0].Path != "tenant.go" || len(diff.Uncommitted) != 0 {
		t.Errorf("layer's files = %+v", diff.Files)
	}
	whole, err := w.svc.Diff("api", 89, ScopeWhole)
	if err != nil {
		t.Fatal(err)
	}
	if len(whole.Files) != 3 || whole.BaseRef != "origin/main" {
		t.Errorf("whole stack = %d files against %s", len(whole.Files), whole.BaseRef)
	}
	// A PR on its own is against the branch it's for.
	if diff, err = w.svc.Diff("api", 92, ScopeLayer); err != nil || len(diff.Files) != 1 || diff.Files[0].Path != "README.md" {
		t.Errorf("#92 = %+v, %v", diff, err)
	}
	if _, err := w.svc.Diff("api", 92, ScopeWhole); err == nil {
		t.Error("a whole-stack diff of a PR in no stack")
	}
	if _, err := w.svc.Detail("api", 404); err == nil {
		t.Error("a PR that isn't open")
	}

	// The layer below moves on, and this one hasn't followed: it says so,
	// and still shows its own changes, against the layer below as it was.
	w.push(88, "store.go", "package cache\n\ntype Store struct{ n int }\n")
	if d, err = w.svc.Detail("api", 89); err != nil || !d.NeedsRebase {
		t.Fatalf("needs rebase = %+v, %v", d, err)
	}
	if diff, err = w.svc.Diff("api", 89, ScopeLayer); err != nil || len(diff.Files) != 1 || diff.Files[0].Path != "tenant.go" {
		t.Errorf("after the layer below moved: %+v, %v", diff, err)
	}
}

func TestSubmitPostsAndLookingAgainShowsWhatChanged(t *testing.T) {
	w := newWorld(t)
	var posted github.ReviewSubmission
	w.gh.SubmitReviewFn = func(org, repo string, number int, r github.ReviewSubmission) (string, error) {
		posted = r
		return "https://github.com/acme/api/pull/89#pullrequestreview-1", nil
	}
	d, err := w.svc.Detail("api", 89)
	if err != nil {
		t.Fatal(err)
	}
	store := w.svc.Store("api", 89)
	diffed := review.Diffed{Dir: w.bare, BaseRef: d.BaseRef, Base: d.Base, Head: d.Head}
	sug := "func key(tenant string) string { return \"t:\" + tenant }"
	if _, err := store.AddNote(Key("api", 89), diffed, review.NewNote{Path: "tenant.go", Side: "new", Line: 3, Kind: "blocking", Body: "Keys can collide.", Suggestion: &sug}, w.now); err != nil {
		t.Fatal(err)
	}
	// A line that isn't in the PR's diff can't be commented on at GitHub:
	// it goes in the review's body.
	if _, err := store.AddNote(Key("api", 89), diffed, review.NewNote{Path: "cache.go", Side: "new", Line: 3, Kind: "question", Body: "Is 60 right?"}, w.now); err != nil {
		t.Fatal(err)
	}
	p, err := w.svc.Posting("api", 89, d.Head, review.Changes, "Close.")
	if err != nil {
		t.Fatal(err)
	}
	if p.Event != github.EventRequestChanges || len(p.Comments) != 1 || p.Loose != 1 || p.Commit != d.Head {
		t.Fatalf("posting = %+v", p)
	}
	want := "issue (blocking): Keys can collide.\n\n```suggestion\n" + sug + "\n```"
	if c := p.Comments[0]; c.Path != "tenant.go" || c.Side != "RIGHT" || c.Line != 3 || c.Body != want {
		t.Errorf("comment = %+v", c)
	}
	if !strings.HasPrefix(p.Body, "Close.\n\n") || !strings.Contains(p.Body, "**`cache.go`, line 3:** question: Is 60 right?") {
		t.Errorf("body = %q", p.Body)
	}
	if posted.Event != "" {
		t.Fatal("working out what would be posted posted it")
	}

	sub, err := w.svc.Submit("api", 89, d.Head, review.Changes, "Close.")
	if err != nil {
		t.Fatal(err)
	}
	if posted.Event != github.EventRequestChanges || posted.Commit != d.Head || len(posted.Comments) != 1 || posted.Comments[0].Line != 3 || posted.Comments[0].Side != "RIGHT" {
		t.Errorf("posted = %+v", posted)
	}
	if sub.Review == nil || sub.Review.State != review.Submitted || sub.URL == "" {
		t.Errorf("submitted = %+v", sub)
	}
	rec, _ := Load(w.root, "api", 89)
	if rec.Reviewed == nil || rec.Reviewed.Head != d.Head || rec.Reviewed.Prints["tenant.go"] == "" {
		t.Fatalf("record = %+v", rec.Reviewed)
	}
	if got := gitT(t, w.bare, "rev-parse", ReviewedRef(89, sub.Review.ID)); got != d.Head {
		t.Errorf("the reviewed commit isn't kept: %s", got)
	}
	if draft, _ := store.Draft(Key("api", 89)); draft != nil {
		t.Error("the draft is still a draft")
	}

	// Nothing since.
	w.now = w.now.Add(2 * time.Minute)
	if d, _ = w.svc.Detail("api", 89); d.Since != "" || d.Reviewed == nil || d.Reviewed.Prints != nil {
		t.Errorf("since = %q, reviewed = %+v", d.Since, d.Reviewed)
	}

	// They push: looking again shows only that.
	w.push(89, "tenant.go", "package cache\n\nfunc key(tenant string) string { return \"t:\" + tenant }\n")
	if d, _ = w.svc.Detail("api", 89); d.Since != SinceCommits {
		t.Fatalf("after a push, since = %q", d.Since)
	}
	diff, err := w.svc.Diff("api", 89, ScopeSince)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Since != rec.Reviewed.Head || diff.Rebased || len(diff.Files) != 1 || len(diff.Commits) != 1 {
		t.Errorf("since diff = %+v", diff)
	}

	// A review at this commit, then a rebase onto a layer below that's
	// moved: comparing commits would show everything; comparing changes
	// shows nothing to re-read.
	if _, err := w.svc.Submit("api", 89, "", review.Approve, ""); err != nil {
		t.Fatal(err)
	}
	w.push(88, "other.go", "package cache\n\nvar other = 1\n")
	gitT(t, w.remote, "checkout", "-q", "tenant")
	gitT(t, w.remote, "rebase", "-q", "storage")
	w.moved(89)
	if d, err = w.svc.Detail("api", 89); err != nil || d.Since != SinceChanges || d.NeedsRebase {
		t.Fatalf("after a rebase, since = %q (%v)", d.Since, err)
	}
	if diff, err = w.svc.Diff("api", 89, ScopeSince); err != nil {
		t.Fatal(err)
	}
	if !diff.Rebased || len(diff.Files) != 1 || len(diff.Unchanged) != 1 || diff.Unchanged[0] != "tenant.go" {
		t.Errorf("after a rebase that changes nothing: files %d, unchanged %v", len(diff.Files), diff.Unchanged)
	}

	// A rebase that does change a file's change shows it.
	gitT(t, w.remote, "checkout", "-q", "tenant")
	write(t, w.remote, "tenant.go", "package cache\n\nfunc key(tenant string) string { return \"tenant:\" + tenant }\n")
	gitT(t, w.remote, "commit", "-q", "-a", "--amend", "-m", "tenant")
	w.moved(89)
	if diff, err = w.svc.Diff("api", 89, ScopeSince); err != nil || !diff.Rebased || len(diff.Unchanged) != 0 {
		t.Errorf("after an amend: %+v, %v", diff, err)
	}
}

func TestYourOwnPRTakesCommentsOnly(t *testing.T) {
	w := newWorld(t)
	w.pr(92).Author = "me"
	if _, err := w.svc.Posting("api", 92, "", review.Approve, ""); err == nil || !strings.Contains(err.Error(), "your own") {
		t.Errorf("approving your own PR: %v", err)
	}
	if _, err := w.svc.Posting("api", 92, "", review.Comment, "A note."); err != nil {
		t.Errorf("commenting on your own PR: %v", err)
	}
}

func TestParseKind(t *testing.T) {
	cases := []struct{ body, kind, rest string }{
		{"issue (blocking): Keys can collide.", "blocking", "Keys can collide."},
		{"issue: Keys can collide.", "blocking", "Keys can collide."},
		{"issue (non-blocking): maybe", "suggestion", "maybe"},
		{"suggestion: use a map", "suggestion", "use a map"},
		{"suggestion (blocking): use a map", "blocking", "use a map"},
		{"**question:** why?", "question", "why?"},
		{"nitpick: spacing", "nit", "spacing"},
		{"nit: spacing", "nit", "spacing"},
		{"praise: lovely", "", "praise: lovely"},
		{"Looks fine to me", "", "Looks fine to me"},
		{"This issue: is not one", "", "This issue: is not one"},
	}
	for _, c := range cases {
		if kind, rest := ParseKind(c.body); kind != c.kind || rest != c.rest {
			t.Errorf("ParseKind(%q) = %q, %q; want %q, %q", c.body, kind, rest, c.kind, c.rest)
		}
	}
	// What ws writes, it reads back.
	for kind := range kindLabels {
		if got, rest := ParseKind(commentBody(review.Note{Kind: kind, Body: "x"})); got != kind || rest != "x" {
			t.Errorf("%s reads back as %q, %q", kind, got, rest)
		}
	}
}

func TestThreadsReadAsComments(t *testing.T) {
	w := newWorld(t)
	head := w.pr(89).HeadRefOid
	w.gh.ReviewThreadsFn = func(org, repo string, number int) ([]github.ReviewThread, error) {
		return []github.ReviewThread{
			{ID: "T1", Path: "tenant.go", Line: 3, Side: "RIGHT", Comments: []github.ThreadComment{
				{Author: "carol", Body: "issue (blocking): collides\n\n```suggestion\nfunc key() {}\n```", At: "2026-11-10T09:00:00Z", Commit: head, OriginalCommit: head,
					DiffHunk: "@@ -0,0 +1,3 @@\n+package cache\n+\n+func key(tenant string) string { return tenant }"},
				{Author: "alice", Body: "Fixed.", At: "2026-11-10T09:30:00Z"},
				{Author: "me", Body: "Thanks.", At: "2026-11-10T09:40:00Z"},
			}},
			{ID: "T2", Path: "tenant.go", Outdated: true, OriginalLine: 2, Side: "RIGHT", Resolved: true, Comments: []github.ThreadComment{
				{Author: "me", Body: "Why blank?", At: "2026-11-10T09:00:00Z", OriginalCommit: "abc"}}},
		}, nil
	}
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	r, err := w.svc.Threads("api", 89)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Comments) != 2 {
		t.Fatalf("comments = %+v", r.Comments)
	}
	a, b := r.Comments[0], r.Comments[1]
	if a.By != "carol" || a.Kind != "blocking" || a.Body != "collides" || a.Suggestion == nil || *a.Suggestion != "func key() {}" || a.Line != 3 || a.Commit != head || a.Status != review.Open {
		t.Errorf("first = %+v", a)
	}
	if len(a.Excerpt) != 1 || !strings.Contains(a.Excerpt[0], "func key") {
		t.Errorf("excerpt = %q", a.Excerpt)
	}
	if want := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC); !a.At.Equal(want) {
		t.Errorf("said at %v, want its first comment's %v", a.At, want)
	}
	if len(a.Thread) != 2 || a.Thread[0].By != "alice" || a.Thread[1].By != "you" {
		t.Errorf("thread = %+v", a.Thread)
	}
	if b.By != "" || b.Kind != "" || b.Status != review.Resolved || b.Line != 2 || b.Commit != "abc" {
		t.Errorf("second = %+v", b)
	}

	var said, resolved string
	w.gh.ReplyToThreadFn = func(org, repo string, number int, id, body string) error { said = id + ": " + body; return nil }
	w.gh.ResolveThreadFn = func(org, repo string, number int, id string, yes bool) error {
		resolved = fmt.Sprint(id, " ", yes)
		return nil
	}
	// With nowhere to send from, each is posted at once, and answers with
	// the threads read again.
	if now, err := w.svc.Respond("api", 89, "T1", review.Answer, " Good. "); err != nil || said != "T1: Good." || len(now.Comments) != 2 {
		t.Errorf("reply: %v, %q, %+v", err, said, now)
	}
	if _, err := w.svc.Respond("api", 89, "T1", review.Resolve, ""); err != nil || resolved != "T1 true" {
		t.Errorf("resolve: %v, %q", err, resolved)
	}
	if _, err := w.svc.Respond("api", 89, "T1", review.Answer, " "); err == nil {
		t.Error("an empty reply was sent")
	}
}

// What's said on a PR itself reads as the overview shows it: reviews with
// their verdicts, comments, and each reviewer's latest word; a review
// that's only the holder of comments on lines is left out.
func TestDiscussion(t *testing.T) {
	w := newWorld(t)
	head := w.pr(89).HeadRefOid
	w.gh.DiscussionFn = func(org, repo string, number int) ([]github.Said, error) {
		return []github.Said{
			{ID: "R1", Author: "carol", State: "CHANGES_REQUESTED", Body: "<!-- template -->\nThe key collides.", At: "2026-11-10T09:00:00Z", Commit: "abc"},
			{ID: "R2", Author: "carol", State: "COMMENTED", At: "2026-11-10T09:05:00Z", Commit: "abc"},
			{ID: "C1", Author: "alice", Body: "Fixed in the last push.", At: "2026-11-10T09:30:00Z"},
			{ID: "C2", Author: "ci-bot", Bot: true, Body: "Coverage 81%", At: "2026-11-10T09:31:00Z"},
			{ID: "R3", Author: "carol", State: "APPROVED", At: "2026-11-10T09:40:00Z", Commit: head},
			{ID: "R4", Author: "Me", State: "COMMENTED", Body: "One question.", At: "2026-11-10T09:50:00Z", Commit: head},
		}, nil
	}
	if _, err := w.svc.List(false); err != nil {
		t.Fatal(err)
	}
	d, err := w.svc.Discussion("api", 89)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range d.Items {
		got = append(got, it.ID+":"+it.Kind)
	}
	if want := "R1:changes C1:comment C2:comment R3:approved R4:commented"; strings.Join(got, " ") != want {
		t.Errorf("items = %v, want %s", got, want)
	}
	if first := d.Items[0]; first.Body != "The key collides." || !first.Stale || first.Mine {
		t.Errorf("first = %+v", first)
	}
	if !d.Items[2].Bot || !d.Items[4].Mine || d.Items[3].Stale {
		t.Errorf("items = %+v", d.Items)
	}
	if len(d.Reviewers) != 1 || d.Reviewers[0] != (Reviewer{Login: "carol", Kind: SaidApproved, At: "2026-11-10T09:40:00Z"}) {
		t.Errorf("reviewers = %+v", d.Reviewers)
	}

	var said string
	w.gh.CommentFn = func(org, repo string, number int, body string) error {
		said = fmt.Sprintf("%s/%s#%d: %s", org, repo, number, body)
		return nil
	}
	if _, err := w.svc.Comment("api", 89, "  "); err == nil {
		t.Error("a comment with no words was taken")
	}
	if now, err := w.svc.Comment("api", 89, " Thanks. "); err != nil || !strings.HasSuffix(said, "/api#89: Thanks.") || now == nil {
		t.Errorf("comment: %v, said %q", err, said)
	}
}
