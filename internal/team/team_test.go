package team

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/testgit"
)

func TestMain(m *testing.M) {
	cleanup := testgit.NoSigning()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

var utc = time.UTC

func at(day, hour int) time.Time { return time.Date(2026, 11, day, hour, 0, 0, 0, utc) }

func stamp(day, hour int) string { return at(day, hour).Format(time.RFC3339) }

// pr is a pull request in api, by ana-acme.
func pr(number int, title string, set func(*github.PRActivity)) PR {
	a := github.PRActivity{Number: number, Title: title, State: "OPEN", Author: "ana-acme", HeadRefName: "branch", URL: "https://github.example/api/pull/1", CreatedAt: stamp(3, 9)}
	if set != nil {
		set(&a)
	}
	return PR{Repo: "api", PRActivity: a}
}

func kinds(events []Event) string {
	var out []string
	for _, e := range events {
		out = append(out, e.Kind)
	}
	return strings.Join(out, " ")
}

func find(t *testing.T, events []Event, id string) Event {
	t.Helper()
	for _, e := range events {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("no event %s among %d", id, len(events))
	return Event{}
}

func TestBuild_PullRequest(t *testing.T) {
	p := pr(12, "feat: cache tokens per tenant", func(a *github.PRActivity) {
		a.ReadyAt, a.ReadyBy = stamp(3, 11), "ana-acme"
		a.Reviews = []github.PRReview{
			{Author: "ben-acme", State: "CHANGES_REQUESTED", SubmittedAt: stamp(3, 13)},
			{Author: "ana-acme", State: "COMMENTED", SubmittedAt: stamp(3, 14)}, // a reply on her own
			{Author: "ben-acme", State: "PENDING"},
			{Author: "ben-acme", State: "APPROVED", SubmittedAt: stamp(4, 9)},
		}
		a.State, a.MergedAt, a.MergedBy, a.MergeCommit = "MERGED", stamp(4, 10), "ana-acme", "abc123"
	})
	events := Build([]PR{p}, nil, nil)
	if got, want := kinds(events), "opened ready reviewed reviewed merged"; got != want {
		t.Fatalf("kinds = %q, want %q", got, want)
	}
	if e := events[2]; e.Login != "ben-acme" || e.Verdict != Changes || e.PR != 12 || e.Repo != "api" {
		t.Errorf("first review = %+v", e)
	}
	if e := events[4]; e.ID != "pr:api#12:merged" || e.Login != "ana-acme" || e.Commit != "abc123" || !e.At.Equal(at(4, 10)) {
		t.Errorf("merged = %+v", e)
	}

	// Closed without merging, and a draft.
	closed := pr(13, "spike", func(a *github.PRActivity) { a.State, a.ClosedAt = "CLOSED", stamp(5, 9) })
	draft := pr(14, "wip", func(a *github.PRActivity) { a.IsDraft = true })
	events = Build([]PR{closed, draft}, nil, nil)
	if e := find(t, events, "pr:api#13:closed"); e.Kind != Closed {
		t.Errorf("closed = %+v", e)
	}
	if !find(t, events, "pr:api#14:opened").Draft || find(t, events, "pr:api#13:opened").Draft {
		t.Error("only the open draft's opened event says it's a draft")
	}
}

func TestBuild_Commits(t *testing.T) {
	merged := pr(12, "feat: cache tokens per tenant", func(a *github.PRActivity) {
		a.State, a.MergedAt, a.MergedBy, a.Additions, a.ChangedFiles = "MERGED", stamp(4, 10), "ana-acme", 40, 3
	})
	// Merged with a merge commit, by someone else.
	other := pr(15, "fix: evict on revoke", func(a *github.PRActivity) {
		a.State, a.MergedAt, a.MergedBy = "MERGED", stamp(5, 10), "ben-acme"
	})
	// Rebased: its merge is a commit that doesn't name it.
	rebased := pr(16, "docs: say how", func(a *github.PRActivity) {
		a.State, a.MergedAt, a.MergedBy, a.MergeCommit = "MERGED", stamp(6, 10), "ana-acme", "ccc"
	})
	commits := map[string][]Commit{"api": {
		{SHA: "ccc", Name: "Ana Diaz", Email: "ana@acme.example", Subject: "docs: say how", At: at(6, 10), Parents: 1},
		{SHA: "bbb", Name: "Ben Okoro", Email: "ben@acme.example", Subject: "Merge pull request #15 from acme/evict", At: at(5, 10), Parents: 2},
		{SHA: "aaa", Name: "Ana Diaz", Email: "ana@acme.example", Subject: "feat: cache tokens per tenant (#12)", At: at(4, 10), Parents: 1, Additions: 38, Files: 3, Paths: []string{"cache.go"}},
		{SHA: "ddd", Name: "Cal Reyes", Email: "cal@acme.example", Subject: "docs: add adr 0007 cache keys", At: at(4, 12), Parents: 1, Additions: 20, Files: 1, Paths: []string{"adr/0007.md"}},
		{SHA: "eee", Name: "Cal Reyes", Email: "cal@acme.example", Subject: "fix: typo (#9)", At: at(4, 13), Parents: 1},
	}}
	events := Build([]PR{merged, other, rebased}, commits, map[string]string{"api": "main"})

	// A squash merge is one event, with GitHub's login and git's name.
	e := find(t, events, "pr:api#12:merged")
	if e.Login != "ana-acme" || e.Name != "Ana Diaz" || e.Commit != "aaa" || e.Additions != 40 || len(e.Paths) != 1 {
		t.Errorf("squash merge = %+v", e)
	}
	// A merge commit is whoever merged's, so the author isn't paired with it.
	if e := find(t, events, "pr:api#15:merged"); e.Login != "ana-acme" || e.Name != "" || e.Commit != "bbb" {
		t.Errorf("merge commit = %+v", e)
	}
	// A rebased pull request is found by its commit.
	if e := find(t, events, "pr:api#16:merged"); e.Name != "Ana Diaz" || e.Commit != "ccc" {
		t.Errorf("rebased = %+v", e)
	}
	// A commit with no pull request landed, on the branch read.
	if e := find(t, events, "c:api:ddd"); e.Kind != Landed || e.Name != "Cal Reyes" || e.Branch != "main" || e.PR != 0 {
		t.Errorf("landed = %+v", e)
	}
	// One that names a pull request GitHub hasn't told of is still its merge.
	if e := find(t, events, "pr:api#9:merged"); e.Title != "fix: typo" || e.Name != "Cal Reyes" || e.Login != "" {
		t.Errorf("merge known only to git = %+v", e)
	}
	n := 0
	for _, e := range events {
		if e.Kind == Merged {
			n++
		}
	}
	if n != 4 {
		t.Errorf("%d merged events, want 4 (each pull request once)", n)
	}
}

func TestResolve(t *testing.T) {
	events := []Event{
		{ID: "1", Login: "ana-acme", Name: "Ana Diaz", Email: "ana@acme.example"}, // a squash merge pairs them
		{ID: "2", Login: "ana-acme"},
		{ID: "3", Name: "Ana Diaz", Email: "ana@home.example"},                    // the same name, another address
		{ID: "4", Name: "benny", Email: "1234+ben-acme@users.noreply.github.com"}, // a no-reply address has the login
		{ID: "5", Login: "ben-acme"},
		{ID: "6", Login: "cal-acme"},
		{ID: "7", Name: "C. Reyes", Email: "cal@acme.example"},
		{ID: "8", Login: "dependabot", Bot: true},
		{ID: "9", Name: "Ana Smith", Email: "smith@acme.example"},
		{ID: "10", Login: "dee-acme", Name: "Dee Okafor"}, // her name, as GitHub has it
		{ID: "11", Name: "Dee", Email: "dee@acme.example"},
		{ID: "12", Login: "dee-acme", Name: "Dee", Email: "dee@acme.example"}, // a squash merge: her name in git
		{ID: "13", Name: "Dee", Email: "dee@acme.example"},
	}
	cfg := map[string]PersonConfig{"cal": {Name: "Cal Reyes", Logins: []string{"cal-acme"}, Emails: []string{"cal@acme.example"}}}
	r := Resolve(events, cfg)
	key := func(i int) string { return r.Of(events[i]).Key }
	if key(0) != key(1) || key(0) != key(2) {
		t.Errorf("ana is three people: %s %s %s", key(0), key(1), key(2))
	}
	if key(3) != key(4) {
		t.Errorf("ben's no-reply address isn't his login: %s %s", key(3), key(4))
	}
	if key(5) != "cal" || key(6) != "cal" {
		t.Errorf("cal isn't who the config says: %s %s", key(5), key(6))
	}
	if n := len(r.People()); n != 6 {
		t.Fatalf("%d people, want 6: %+v", n, r.People())
	}
	// A whole name is what someone's called, though a first name alone is
	// on more of what they did.
	if dee := r.Of(events[10]); dee.Name != "Dee Okafor" || dee.Short != "Dee" || dee != r.Of(events[9]) {
		t.Errorf("dee = %+v", dee)
	}
	// A name in git that's a login is that login's person.
	same := Resolve([]Event{{ID: "1", Login: "carol"}, {ID: "2", Name: "Carol", Email: "carol@acme.example"}}, nil)
	if n := len(same.People()); n != 1 {
		t.Errorf("carol the login and Carol in git are %d people", n)
	}
	for name, want := range map[string]bool{"semantic-release-bot": true, "semantic-release": true, "renovate[bot]": true, "Bot Okoro": false, "Ana Diaz": false} {
		if got := botName(name, ""); got != want {
			t.Errorf("botName(%q) = %v", name, got)
		}
	}
	ana, cal, smith := r.Of(events[0]), r.Of(events[5]), r.Of(events[8])
	if ana.Name != "Ana Diaz" || ana.Login != "ana-acme" || ana.Key != "ana-acme" {
		t.Errorf("ana = %+v", ana)
	}
	// Two Anas are called by their whole names; Cal by his first.
	if ana.Short != "Ana Diaz" || smith.Short != "Ana Smith" || cal.Short != "Cal" || cal.Name != "Cal Reyes" {
		t.Errorf("short names: %q %q %q", ana.Short, smith.Short, cal.Short)
	}
	if !r.Of(events[7]).Bot {
		t.Error("dependabot isn't a bot")
	}
	if r.Of(Event{}) != nil {
		t.Error("an event by nobody is by someone")
	}
}

func TestView(t *testing.T) {
	p := pr(12, "feat(cache): cache tokens per tenant", func(a *github.PRActivity) {
		a.Reviews = []github.PRReview{
			{Author: "ben-acme", State: "COMMENTED", SubmittedAt: stamp(3, 12)},
			{Author: "ben-acme", State: "CHANGES_REQUESTED", SubmittedAt: stamp(3, 13)},
			{Author: "ben-acme", State: "COMMENTED", SubmittedAt: stamp(3, 14)},
			{Author: "ben-acme", State: "APPROVED", SubmittedAt: stamp(4, 9)},
		}
		a.State, a.MergedAt, a.MergedBy = "MERGED", stamp(4, 10), "ana-acme"
	})
	open := pr(20, "fix: evict on revoke", func(a *github.PRActivity) { a.CreatedAt, a.Author = stamp(4, 15), "ben-acme" })
	bot := pr(21, "chore(deps): bump x", func(a *github.PRActivity) { a.CreatedAt, a.Author, a.Bot = stamp(4, 16), "dependabot", true })
	commits := map[string][]Commit{
		"api": {{SHA: "aaa", Name: "Ana Diaz", Email: "ana@acme.example", Subject: "feat(cache): cache tokens per tenant (#12)", At: at(4, 10), Parents: 1}},
		"records": {
			{SHA: "r5", Name: "Cal Reyes", Email: "cal@acme.example", Subject: "docs: add a note on ttl", At: at(4, 15), Parents: 1, Paths: []string{"notes/ttl.md"}},
			{SHA: "r3", Name: "Cal Reyes", Email: "cal@acme.example", Subject: "docs: amend adr 0007 for tenants", At: at(4, 14), Parents: 1, Paths: []string{"adr/0007.md", "README.md"}},
			{SHA: "r2", Name: "Cal Reyes", Email: "cal@acme.example", Subject: "docs: add adr 0007 cache keys", At: at(4, 12), Parents: 1, Paths: []string{"adr/0007.md"}},
			{SHA: "r1", Name: "Cal Reyes", Email: "cal@acme.example", Subject: "docs: add note on eviction", At: at(2, 12), Parents: 1},
		},
	}
	events := Build([]PR{p, open, bot}, commits, map[string]string{"api": "main", "records": "main"})
	// Ben has only a login on GitHub: the config says who he is.
	cfg := map[string]PersonConfig{"ben": {Name: "Ben Okoro", Logins: []string{"ben-acme"}}}
	tl := View(events, cfg, at(3, 0), utc)

	if len(tl.Days) != 2 || tl.Days[0].Date != "2026-11-04" || tl.Days[1].Date != "2026-11-03" {
		t.Fatalf("days = %+v", tl.Days)
	}
	by := map[string]Item{}
	for _, d := range tl.Days {
		for _, it := range d.Items {
			by[it.Key] = it
		}
	}
	// The pull request is one strand, on each day it changed, and landed.
	first, second := by["pr:api#12@2026-11-03"], by["pr:api#12@2026-11-04"]
	if first.Name != "Cache tokens per tenant" || first.State != Done || second.State != Done {
		t.Errorf("pr 12: %q %s %s", first.Name, first.State, second.State)
	}
	if want := "Ana opened. Ben asked for changes."; first.Words != want {
		t.Errorf("day one says %q, want %q", first.Words, want)
	}
	if want := "Ben approved. Ana landed."; second.Words != want {
		t.Errorf("day two says %q, want %q", second.Words, want)
	}
	if strings.Join(second.People, " ") != "ana-acme ben" {
		t.Errorf("people = %v, want whose it is first", second.People)
	}
	// On a day only its reviewer did something, it's still its author's.
	if late := View(events, cfg, at(4, 0), utc).Days[0]; len(late.Items) == 0 {
		t.Fatal("no items on the fourth")
	}
	onlyReview := pr(30, "fix: one", func(a *github.PRActivity) {
		a.Reviews = []github.PRReview{{Author: "ben-acme", State: "APPROVED", SubmittedAt: stamp(6, 9)}}
	})
	day := View(Build([]PR{onlyReview}, nil, nil), cfg, at(6, 0), utc).Days[0].Items[0]
	if strings.Join(day.People, " ") != "ana-acme ben" || day.Words != "Ben approved." {
		t.Errorf("a day of only a review: people %v, words %q", day.People, day.Words)
	}
	// What one person did in a row is one sentence.
	busy := pr(31, "fix: two", func(a *github.PRActivity) {
		a.AuthorName = "Ana Diaz" // as GitHub has it: there's no commit of hers to say
		a.CreatedAt, a.ReadyAt, a.ReadyBy = stamp(7, 9), stamp(7, 10), "ana-acme"
		a.Reviews = []github.PRReview{{Author: "ben-acme", State: "APPROVED", SubmittedAt: stamp(7, 11)}}
		a.State, a.MergedAt = "MERGED", stamp(7, 12)
	})
	day = View(Build([]PR{busy}, nil, nil), cfg, at(7, 0), utc).Days[0].Items[0]
	if want := "Ana opened and marked ready for review. Ben approved. Ana landed."; day.Words != want {
		t.Errorf("a busy day says %q, want %q", day.Words, want)
	}
	// One person's commits to one repo in a day are one strand where they
	// touch the same files: a record added, then amended. It's called by
	// the first of them.
	run := by["c:records:cal-reyes:2026-11-04:r2@2026-11-04"]
	if run.Name != "Add adr 0007 cache keys" || run.Kind != "commits" || len(run.Events) != 2 {
		t.Errorf("cal's commits = %+v", run)
	}
	if want := "Cal landed 2 commits straight on main: Add adr 0007 cache keys; Amend adr 0007 for tenants."; run.Words != want {
		t.Errorf("cal's commits say %q, want %q", run.Words, want)
	}
	// Another record the same day is another strand.
	if other := by["c:records:cal-reyes:2026-11-04:r5@2026-11-04"]; other.Name != "Add a note on ttl" || len(other.Events) != 1 {
		t.Errorf("cal's other commit = %+v", other)
	}
	// What's before the period isn't in it.
	for key := range by {
		if strings.HasSuffix(key, "@2026-11-02") {
			t.Errorf("a commit from before the period is in it: %s", key)
		}
	}
	if it := by["pr:api#20@2026-11-04"]; it.State != InReview || it.Words != "Ben opened." {
		t.Errorf("open pr = %+v", it)
	}
	if !by["pr:api#21@2026-11-04"].Bot || by["pr:api#20@2026-11-04"].Bot {
		t.Error("only the bot's pull request is a bot's")
	}
	// The latest to change is first.
	if got := tl.Days[0].Items[0].Key; got != "pr:api#21@2026-11-04" {
		t.Errorf("first item = %s", got)
	}

	// A single commit is called by its subject.
	tl = View(events, cfg, at(2, 0), utc)
	last := tl.Days[len(tl.Days)-1].Items[0]
	if last.Name != "Add note on eviction" || last.Words != "Cal landed straight on main." {
		t.Errorf("one commit = %q, %q", last.Name, last.Words)
	}
}

func TestPlainName(t *testing.T) {
	for in, want := range map[string]string{
		"feat(cache): cache tokens per tenant (#12)": "Cache tokens per tenant",
		"Fix: a typo":     "A typo",
		"Add repo groups": "Add repo groups",
		"Merge pull request #7 from acme/claude/bubbletea-v2":  "Bubbletea v2",
		"Merge pull request #10 from acme/document_geo-scoped": "Document geo scoped",
	} {
		if got := plainName(in); got != want {
			t.Errorf("plainName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSince(t *testing.T) {
	now := time.Date(2026, 11, 10, 15, 30, 0, 0, utc) // a Tuesday
	for in, want := range map[string]string{
		"today": "2026-11-10", "yesterday": "2026-11-09", "week": "2026-11-09", "monday": "2026-11-09", "tue": "2026-11-10",
		"Friday": "2026-11-06", "7d": "2026-11-03", "2w": "2026-10-27", "2026-11-01": "2026-11-01",
	} {
		got, err := Since(in, now)
		if err != nil || got.Format("2006-01-02") != want || got.Hour() != 0 {
			t.Errorf("Since(%q) = %v, %v; want %s", in, got, err, want)
		}
	}
	if _, err := Since("soon", now); err == nil {
		t.Error("Since(soon) didn't refuse")
	}
}

func gitT(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// world is a workspace with one repo, api, cloned from a remote that
// commits can be added to.
type world struct {
	t      *testing.T
	root   string
	remote string
	bare   string
	gh     *github.Stub
	asked  []time.Time
	prs    []github.PRActivity
	now    time.Time
	svc    *Service
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{t: t, root: t.TempDir(), now: at(10, 12)}
	storetest.Own(t, w.root)
	w.remote = filepath.Join(w.root, "remote")
	os.MkdirAll(w.remote, 0o755)
	gitT(t, w.remote, nil, "init", "-q", "--initial-branch=main")
	w.commit("Ana Diaz", "ana@acme.example", "base", at(1, 9), "README.md")
	w.bare = filepath.Join(w.root, "repos", "api", ".bare")
	os.MkdirAll(filepath.Dir(w.bare), 0o755)
	if out, err := exec.Command("git", "clone", "-q", "--bare", w.remote, w.bare).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	gitT(t, w.bare, nil, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	gitT(t, w.bare, nil, "fetch", "-q", "origin")
	w.gh = &github.Stub{ChangedPRsFn: func(org, repo string, since time.Time) ([]github.PRActivity, error) {
		w.asked = append(w.asked, since)
		return w.prs, nil
	}}
	w.svc = w.service()
	return w
}

func (w *world) service() *Service {
	return &Service{
		Root: w.root, GitHub: w.gh, Now: func() time.Time { return w.now },
		Repos: func() []Repo {
			return []Repo{{Name: "api", Bare: w.bare, Branch: "main", GitHub: ref.Remote{Owner: "acme", Name: "api"}}}
		},
	}
}

// commit adds a commit to the remote's main, changing a file, and answers
// its id.
func (w *world) commit(name, email, subject string, when time.Time, file string) string {
	w.t.Helper()
	path := filepath.Join(w.remote, file)
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		w.t.Fatal(err)
	}
	f.WriteString(subject + "\n")
	f.Close()
	date := when.Format(time.RFC3339)
	env := []string{"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + email, "GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + email, "GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}
	gitT(w.t, w.remote, env, "add", "-A")
	gitT(w.t, w.remote, env, "commit", "-q", "-m", subject)
	return gitT(w.t, w.remote, nil, "rev-parse", "HEAD")
}

// write puts a file in the remote, for the next commit to take.
func (w *world) write(file, text string) {
	w.t.Helper()
	if err := os.WriteFile(filepath.Join(w.remote, file), []byte(text), 0o644); err != nil {
		w.t.Fatal(err)
	}
}

func TestCommits(t *testing.T) {
	w := newWorld(t)
	w.commit("Ana Diaz", "ana@acme.example", "feat: one (#3)", at(8, 9), "a.go")
	sha := w.commit("Cal Reyes", "cal@acme.example", "docs: two", at(9, 9), "docs/b.md")

	// Nothing's there until it's fetched.
	got, err := Commits(w.bare, "main", at(5, 0))
	if err != nil || len(got) != 0 {
		t.Fatalf("before a fetch: %v, %v", got, err)
	}
	if err := Fetch(w.bare, "main"); err != nil {
		t.Fatal(err)
	}
	got, err = Commits(w.bare, "main", at(5, 0))
	if err != nil || len(got) != 2 {
		t.Fatalf("commits = %+v, %v", got, err)
	}
	c := got[0]
	if c.SHA != sha || c.Name != "Cal Reyes" || c.Email != "cal@acme.example" || c.Subject != "docs: two" || !c.At.Equal(at(9, 9)) || c.Parents != 1 {
		t.Errorf("newest = %+v", c)
	}
	if c.Files != 1 || c.Additions != 1 || len(c.Paths) != 1 || c.Paths[0] != "docs/b.md" {
		t.Errorf("what it touched = %+v", c)
	}
	if _, err := Commits(w.bare, "nope", at(5, 0)); err == nil {
		t.Error("a branch that isn't there didn't fail")
	}
}

func TestCommits_MergeCommit(t *testing.T) {
	w := newWorld(t)
	gitT(t, w.remote, nil, "checkout", "-q", "-b", "evict")
	w.commit("Ben Okoro", "ben@acme.example", "wip one", at(7, 9), "evict.go")
	w.commit("Ben Okoro", "ben@acme.example", "wip two", at(7, 10), "evict.go")
	gitT(t, w.remote, nil, "checkout", "-q", "main")
	date := at(8, 9).Format(time.RFC3339)
	env := []string{"GIT_AUTHOR_NAME=Ana Diaz", "GIT_AUTHOR_EMAIL=ana@acme.example", "GIT_COMMITTER_NAME=Ana Diaz", "GIT_COMMITTER_EMAIL=ana@acme.example", "GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}
	gitT(t, w.remote, env, "merge", "-q", "--no-ff", "-m", "Merge pull request #15 from acme/evict", "-m", "Evict on revoke\n\nAnd more about it.", "evict")
	Fetch(w.bare, "main")

	// A merged branch's own commits aren't each counted as landing, and
	// the merge has what it brought in.
	got, err := Commits(w.bare, "main", at(5, 0))
	if err != nil || len(got) != 1 {
		t.Fatalf("commits = %+v, %v", got, err)
	}
	if c := got[0]; c.Parents != 2 || c.Name != "Ana Diaz" || c.Files != 1 || c.Additions != 2 {
		t.Errorf("merge = %+v", c)
	}
	// It's called by the line under its subject, which is its pull
	// request's title.
	events := Build(nil, map[string][]Commit{"api": got}, nil)
	if e := find(t, events, "pr:api#15:merged"); e.Title != "Evict on revoke" {
		t.Errorf("merge's title = %q", e.Title)
	}
}

func TestService(t *testing.T) {
	w := newWorld(t)
	sha := w.commit("Ana Diaz", "ana@acme.example", "feat: cache tokens (#12)", at(9, 9), "cache.go")
	w.commit("Cal Reyes", "cal@acme.example", "docs: add adr", at(9, 10), "adr.md")
	w.prs = []github.PRActivity{{
		Number: 12, Title: "feat: cache tokens", State: "MERGED", Author: "ana-acme", CreatedAt: stamp(8, 9), MergedAt: stamp(9, 9), UpdatedAt: stamp(9, 9),
		MergedBy: "ana-acme", MergeCommit: sha,
		Reviews: []github.PRReview{{Author: "ben-acme", State: "APPROVED", SubmittedAt: stamp(8, 15)}},
	}}

	if w.svc.Peek() != nil {
		t.Fatal("something's kept before the first look")
	}
	snap, err := w.svc.Look(false, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	// The first look goes back the window, to the start of that day.
	if want := time.Date(2026, 10, 27, 0, 0, 0, 0, utc); !snap.Since.Equal(want) || !w.asked[0].Equal(want) {
		t.Errorf("first look from %v (GitHub asked from %v), want %v", snap.Since, w.asked[0], want)
	}
	if got, want := kinds(snap.Events), "base opened reviewed merged landed"; strings.TrimPrefix(got, "landed ") != strings.TrimPrefix(want, "base ") {
		t.Errorf("kinds = %q", got)
	}
	if e := find(t, snap.Events, "pr:api#12:merged"); e.Name != "Ana Diaz" || e.Login != "ana-acme" || e.Commit != sha {
		t.Errorf("merged = %+v", e)
	}

	// A shorter period is answered from the same look: it went back the
	// whole window.
	if got, err := w.svc.Look(false, at(10, 0)); err != nil || got != snap || len(w.asked) != 1 {
		t.Errorf("a shorter period looked again: %d asks, %v", len(w.asked), err)
	}

	// Asked again at once, nothing's looked at.
	if _, err := w.svc.Look(false, time.Time{}); err != nil || len(w.asked) != 1 {
		t.Errorf("a second look asked GitHub again: %d asks, %v", len(w.asked), err)
	}

	// Later, GitHub is asked only for what's changed since, and what it
	// said before is kept.
	w.now = w.now.Add(time.Hour)
	w.commit("Cal Reyes", "cal@acme.example", "docs: amend adr", at(10, 12), "adr.md")
	w.prs = []github.PRActivity{{Number: 13, Title: "fix: evict", State: "OPEN", Author: "ben-acme", CreatedAt: stamp(10, 12), UpdatedAt: stamp(10, 12)}}
	snap, err = w.svc.Look(false, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if want := at(10, 12).Add(-askAgain); len(w.asked) != 2 || !w.asked[1].Equal(want) {
		t.Errorf("asked from %v, want %v", w.asked, want)
	}
	find(t, snap.Events, "pr:api#12:merged")
	find(t, snap.Events, "pr:api#13:opened")
	if n := strings.Count(kinds(snap.Events), "landed"); n != 3 {
		t.Errorf("%d landed, want 3 (the base, and Cal's two)", n)
	}

	// When you last looked is kept, and the timeline says.
	if !w.svc.Seen().IsZero() || w.svc.Timeline(snap, time.Time{}).Seen != nil {
		t.Error("it's been seen before anyone's looked")
	}
	if err := w.svc.MarkSeen(); err != nil {
		t.Fatal(err)
	}
	if got := w.svc.Timeline(snap, time.Time{}).Seen; got == nil || !got.Equal(w.now) || !w.service().Seen().Equal(w.now) {
		t.Errorf("seen = %v, want %v", got, w.now)
	}

	// A new service reads what was kept, without looking.
	again := w.service()
	kept := again.Peek()
	if kept == nil || len(kept.Events) != len(snap.Events) || !kept.At.Equal(snap.At) || !kept.Asked.Equal(snap.Asked) {
		t.Fatalf("kept = %+v", kept)
	}
	if len(w.asked) != 2 {
		t.Error("peeking asked GitHub")
	}
	tl := again.Timeline(kept, at(9, 0))
	if len(tl.Days) != 2 || !tl.GitHub || len(tl.People) != 3 {
		t.Errorf("timeline: %d days, %d people", len(tl.Days), len(tl.People))
	}

	// Asked to go further back than it has, it looks again, from there.
	far := time.Date(2026, 10, 1, 0, 0, 0, 0, utc)
	snap, err = again.Look(false, far)
	if err != nil || !snap.Since.Equal(far) || !w.asked[len(w.asked)-1].Equal(far) {
		t.Errorf("further back: since %v, asked %v, %v", snap.Since, w.asked, err)
	}

	// What was kept by a ws that kept less of a pull request is asked for
	// again in full, once.
	snap.Form = keptForm - 1
	w.now = w.now.Add(time.Hour)
	if snap, err = again.Look(false, time.Time{}); err != nil || !w.asked[len(w.asked)-1].Equal(far) || snap.Form != keptForm || !snap.Since.Equal(far) {
		t.Errorf("an older form: asked from %v, form %d, since %v, %v", w.asked[len(w.asked)-1], snap.Form, snap.Since, err)
	}
	w.now = w.now.Add(time.Hour)
	if _, err = again.Look(false, time.Time{}); err != nil || w.asked[len(w.asked)-1].Equal(far) {
		t.Errorf("after that, it asked for everything again: %v", err)
	}
}

func TestService_NoGitHubAndProblems(t *testing.T) {
	w := newWorld(t)
	w.commit("Ana Diaz", "ana@acme.example", "feat: cache tokens (#12)", at(9, 9), "cache.go")
	w.svc.GitHub = nil
	snap, err := w.svc.Look(false, time.Time{})
	if err != nil || snap.GitHub || len(w.asked) != 0 {
		t.Fatalf("without GitHub: %+v, %v", snap, err)
	}
	// Git alone knows the merge, by its subject.
	if e := find(t, snap.Events, "pr:api#12:merged"); e.Name != "Ana Diaz" || e.Title != "feat: cache tokens" {
		t.Errorf("merged = %+v", e)
	}

	// A repo GitHub can't be asked about is said, and asked from as far
	// back next time.
	w.svc = w.service()
	w.gh.ChangedPRsFn = func(org, repo string, since time.Time) ([]github.PRActivity, error) {
		w.asked = append(w.asked, since)
		return nil, os.ErrDeadlineExceeded
	}
	w.now = w.now.Add(time.Hour)
	snap, err = w.svc.Look(false, time.Time{})
	if err != nil || len(snap.Problems) != 1 || !snap.Asked.IsZero() {
		t.Fatalf("a failed ask: %+v, %v", snap, err)
	}
	w.now = w.now.Add(time.Hour)
	if _, err := w.svc.Look(false, time.Time{}); err != nil || !w.asked[len(w.asked)-1].Equal(snap.Since) {
		t.Errorf("after a failed ask, asked from %v, want %v", w.asked[len(w.asked)-1], snap.Since)
	}
}
