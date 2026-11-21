package web

import (
	"github.com/withworkspace/workspace/internal/store/storetest"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/team"
)

// newTeamServer is a web UI over a workspace with one repo, api, whose
// main has a squash merge of #12 by Ana and a commit of Cal's with no
// pull request, on a clock that says it's noon on 10 November 2026.
func newTeamServer(t *testing.T) (*Server, *int) {
	t.Helper()
	root := t.TempDir()
	remote, bare := filepath.Join(root, "remote"), filepath.Join(root, "repos", "api", ".bare")
	os.MkdirAll(remote, 0o755)
	gitT(t, remote, "init", "-q", "--initial-branch=main")
	commit := func(name, email, subject, date string) {
		t.Helper()
		os.WriteFile(filepath.Join(remote, "a.txt"), []byte(subject+"\n"), 0o644)
		gitT(t, remote, "add", "-A")
		cmd := exec.Command("git", "-C", remote, "commit", "-q", "-m", subject)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email, "GIT_COMMITTER_NAME="+name, "GIT_COMMITTER_EMAIL="+email, "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("commit: %v\n%s", err, out)
		}
	}
	commit("Ana Diaz", "ana@acme.example", "feat: cache tokens (#12)", "2026-11-09T09:00:00Z")
	commit("Cal Reyes", "cal@acme.example", "docs: add adr 0007", "2026-11-10T10:00:00Z")
	os.MkdirAll(filepath.Dir(bare), 0o755)
	if out, err := exec.Command("git", "clone", "-q", "--bare", remote, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	gitT(t, bare, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	gitT(t, bare, "fetch", "-q", "origin")

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	asks := 0
	gh := &github.Stub{ChangedPRsFn: func(org, repo string, since time.Time) ([]github.PRActivity, error) {
		asks++
		return []github.PRActivity{{
			PR:      github.PR{Number: 12, Title: "feat: cache tokens", State: "MERGED", Author: "ana-acme", URL: "https://github.com/acme/api/pull/12", CreatedAt: "2026-11-08T09:00:00Z", MergedAt: "2026-11-09T09:00:00Z"},
			Reviews: []github.PRReview{{Author: "ben-acme", State: "APPROVED", SubmittedAt: "2026-11-08T15:00:00Z"}},
		}}, nil
	}}
	svc := &team.Service{Root: root, GitHub: gh, Now: func() time.Time { return now },
		Repos: func() []team.Repo {
			return []team.Repo{{Name: "api", Bare: bare, Branch: "main", GitHub: ref.Remote{Owner: "acme", Name: "api"}}}
		}}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	storetest.Own(t, root)
	s := &Server{Root: root, Poll: 20 * time.Millisecond, TeamPoll: 20 * time.Millisecond, Assets: testApp, listener: ln, Team: svc,
		clock: func() time.Time { return now },
	}
	s.App = testOps(s, ops.Options{Team: svc})
	fake(s).repos = func() []ops.Repo { return []ops.Repo{{Name: "api", Label: "API", Color: "#af87ff"}} }
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, &asks
}

func TestTeamAPI(t *testing.T) {
	s, asks := newTeamServer(t)
	var v TeamView
	if code := getInto(t, s, "/api/team", &v); code != 200 {
		t.Fatalf("team: %d", code)
	}
	if len(v.Days) != 3 || v.Days[0].Date != "2026-11-10" || !v.GitHub || v.Repos["api"].Label != "API" {
		t.Fatalf("view: %d days, repos %+v", len(v.Days), v.Repos)
	}
	// Cal's commit, with no pull request.
	if it := v.Days[0].Items[0]; it.Kind != "commits" || it.Name != "Add adr 0007" || it.Words != "Cal landed it straight on main." {
		t.Errorf("today = %+v", it)
	}
	// The pull request's merge is one thing, git's and GitHub's together.
	it := v.Days[1].Items[0]
	if it.PR != 12 || it.State != team.Done || it.Words != "Ana landed it." || it.URL == "" || len(it.Events) != 1 || it.Events[0].Commit == "" {
		t.Errorf("yesterday = %+v", it)
	}
	if len(v.People) != 3 {
		t.Errorf("people = %+v", v.People)
	}

	// A period: only what's in it.
	v = TeamView{}
	if code := getInto(t, s, "/api/team?since=today", &v); code != 200 || len(v.Days) != 1 {
		t.Errorf("today: %d, %d days", code, len(v.Days))
	}
	// Opening again answers from what's kept; refresh looks.
	before := *asks
	getInto(t, s, "/api/team", &v)
	if *asks != before {
		t.Error("a second read asked GitHub again")
	}
	if code := do(t, s, "GET", "/api/team?since=soon", true, nil).StatusCode; code != 400 {
		t.Errorf("since=soon: %d, want 400", code)
	}
}

func TestTeamNarrateAPI(t *testing.T) {
	s, _ := newTeamServer(t)
	s.Team.Writer = func(a team.Ask) (string, error) {
		if a.Subject.Kind == "commits" {
			return "gist: ADR 0007 is recorded.\nknow: convention | a.txt | Cache keys are per tenant.", nil
		}
		return "gist: Tokens are cached.\nknow: none", nil
	}
	var v TeamView
	getInto(t, s, "/api/team", &v)
	if v.Narrative == nil || !v.Narrative.On || v.Narrative.Unwritten != 2 || v.Days[0].Items[0].Narrative != "unwritten" {
		t.Fatalf("before: %+v, %+v", v.Narrative, v.Days[0].Items[0])
	}
	post := func(body string) int {
		return do(t, s, "POST", "/api/team/narrate", true, func(r *http.Request) {
			r.Header.Set("Origin", s.Info().Origin())
			r.Header.Set("Content-Type", "application/json")
			r.Body = io.NopCloser(strings.NewReader(body))
			r.ContentLength = int64(len(body))
		}).StatusCode
	}
	if code := post(`{"since":"soon"}`); code != 400 {
		t.Errorf("since=soon: %d, want 400", code)
	}
	// One strand, then the rest of the period.
	if code := post(`{"strand":"` + v.Days[0].Items[0].Strand + `"}`); code != 200 {
		t.Fatalf("narrate one: %d", code)
	}
	if code := post(`{}`); code != 200 {
		t.Fatalf("narrate the period: %d", code)
	}
	for range 400 {
		v = TeamView{}
		getInto(t, s, "/api/team", &v)
		if v.Narrative.Writing == 0 && v.Narrative.Unwritten == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	today := v.Days[0].Items[0]
	if today.Gist != "ADR 0007 is recorded." || len(today.Know) != 1 || today.Narrative != "" {
		t.Errorf("today = %+v", today)
	}
	if len(v.Know) != 1 || v.Know[0].Item != today.Key || v.Know[0].Kind != team.KnowConvention {
		t.Errorf("worth knowing = %+v", v.Know)
	}
	if it := v.Days[1].Items[0]; it.Gist != "Tokens are cached." || len(it.Know) != 0 {
		t.Errorf("the pull request = %+v", it)
	}
}

func TestTeamSummariseAPI(t *testing.T) {
	s, _ := newTeamServer(t)
	s.Team.Writer = func(a team.Ask) (string, error) { return "gist: " + a.Subject.Title + " is done.\nknow: none", nil }
	s.Team.Summariser = func(a team.SummaryAsk) (string, error) {
		return "topic: Token cache | 1, 2 | Tokens are cached, and an ADR says how.", nil
	}
	post := func(path, body string) int {
		return do(t, s, "POST", path, true, func(r *http.Request) {
			r.Header.Set("Origin", s.Info().Origin())
			r.Header.Set("Content-Type", "application/json")
			r.Body = io.NopCloser(strings.NewReader(body))
			r.ContentLength = int64(len(body))
		}).StatusCode
	}
	var v TeamView
	getInto(t, s, "/api/team", &v)
	if v.Summary == nil || v.Summary.Can != 0 || v.Summary.At != nil {
		t.Fatalf("before anything's read: %+v", v.Summary)
	}
	post("/api/team/narrate", `{}`)
	settled := func(ok func() bool) {
		t.Helper()
		for range 400 {
			v = TeamView{}
			getInto(t, s, "/api/team", &v)
			if ok() {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("it never settled: %+v, %+v", v.Narrative, v.Summary)
	}
	settled(func() bool { return v.Narrative.Writing == 0 && v.Summary.Can == 2 })
	if code := post("/api/team/summarise", `{"since":"soon"}`); code != 400 {
		t.Errorf("since=soon: %d, want 400", code)
	}
	if code := post("/api/team/summarise", `{}`); code != 200 {
		t.Fatalf("summarise: %d", code)
	}
	settled(func() bool { return !v.Summary.Writing && len(v.Summary.Topics) > 0 })
	topic := v.Summary.Topics[0]
	if topic.Name != "Token cache" || len(topic.Strands) != 2 || len(topic.People) == 0 || v.Summary.Stale || v.Summary.At == nil {
		t.Errorf("the account = %+v", v.Summary)
	}
	// It's this period's: another has none.
	v = TeamView{}
	getInto(t, s, "/api/team?since=today", &v)
	if v.Summary == nil || v.Summary.At != nil {
		t.Errorf("today's = %+v", v.Summary)
	}
}

func TestTeamSeenAndNear(t *testing.T) {
	s, _ := newTeamServer(t)
	var v TeamView
	getInto(t, s, "/api/team", &v)
	if v.Seen != nil || len(v.Days[0].Items[0].Near) != 0 {
		t.Fatalf("before: seen %v, near %+v", v.Seen, v.Days[0].Items[0].Near)
	}
	res := do(t, s, "POST", "/api/team/seen", true, func(r *http.Request) {
		r.Header.Set("Origin", s.Info().Origin())
		r.Header.Set("Content-Type", "application/json")
	})
	if res.StatusCode != 200 {
		t.Fatalf("seen: %d", res.StatusCode)
	}

	// A capsule of yours that's changed a.txt, which both commits changed.
	capsule := filepath.Join(s.Root, "repos", "api", "try")
	if out, err := exec.Command("git", "clone", "-q", filepath.Join(s.Root, "remote"), capsule).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	os.WriteFile(filepath.Join(capsule, "a.txt"), []byte("mine\n"), 0o644)
	os.WriteFile(filepath.Join(capsule, "new.txt"), []byte("untracked\n"), 0o644)
	s.Team.Capsules = func(repo string) []team.Capsule {
		return []team.Capsule{{Repo: repo, Name: "try", Dir: capsule, Base: "origin/main"}}
	}

	v = TeamView{}
	getInto(t, s, "/api/team", &v)
	if v.Seen == nil {
		t.Error("seen isn't said after looking")
	}
	near := v.Days[0].Items[0].Near
	if len(near) != 1 || near[0].Capsule != "api/try" || len(near[0].Files) != 1 || near[0].Files[0] != "a.txt" {
		t.Errorf("near = %+v", near)
	}
	// The day the pull request was only opened and reviewed has no files
	// to be near.
	if got := v.Days[2].Items[0].Near; len(got) != 0 {
		t.Errorf("an open pull request's day is near: %+v", got)
	}
}

func TestTeamAPI_None(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	if code := do(t, s, "GET", "/api/team", true, nil).StatusCode; code != 404 {
		t.Errorf("no team: %d, want 404", code)
	}
}
