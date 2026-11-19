package review_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// reviewsFixture is a workspace whose backend repo has one open pull
// request, #7 (branch existing, for main, by alice, asking for your
// review), as GitHub would say it.
func reviewsFixture(t *testing.T) (*testutil.Workspace, *github.Stub) {
	t.Helper()
	w := testutil.MissionWorkspace(t)
	src := w.Sources["backend"]
	testutil.GitCmd(t, src, "update-ref", "refs/pull/7/head", "existing")
	head, err := exec.Command("git", "-C", src, "rev-parse", "existing").Output()
	if err != nil {
		t.Fatal(err)
	}
	gh := &github.Stub{Me: "me"}
	gh.PRsForRepoFn = func(org, repo string) ([]github.PR, error) {
		if repo != "backend" {
			return nil, nil
		}
		return []github.PR{{Number: 7, Title: "Add the existing file", HeadRefName: "existing", BaseRefName: "main", Author: "alice",
			HeadRefOid: strings.TrimSpace(string(head)), URL: "https://github.com/test-org/backend/pull/7", UpdatedAt: "2026-11-10T10:00:00Z",
			Additions: 1, ChangedFiles: 1, StatusRollup: "success", Requested: []github.ReviewRequest{{Login: "me"}}}}, nil
	}
	gh.PRDetailFn = func(org, repo string, number int) (github.PRDetailResult, error) {
		return github.PRDetailResult{Body: "<!-- Say what it does. -->\nAdds a file that exists."}, nil
	}
	gh.ReviewThreadsFn = func(org, repo string, number int) ([]github.ReviewThread, error) {
		return []github.ReviewThread{{ID: "T1", Path: "existing.txt", Line: 1, Side: "RIGHT", Comments: []github.ThreadComment{
			{Author: "carol", Body: "question: why does it exist?", At: "2026-11-10T09:00:00Z"}}}}, nil
	}
	return w, gh
}

func reviewsContext(t *testing.T, w *testutil.Workspace, gh github.Client) *ops.Workspace {
	t.Helper()
	ctx, err := cli.LoadOps(w.Root, func(o *ops.Options) { o.GitHub = gh })
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func useFakeConverser(t *testing.T, w *testutil.Workspace) {
	t.Helper()
	harness.Register(testutil.FakeConverser{})
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[conversation]\nharness = \"fake-converse\"\n")...), 0o644)
}

// ws show <repo>#<number> works outside a mission: the pull request, what
// it says it does, its changes, and what's been said in review.
func TestShow_PullRequest(t *testing.T) {
	w, gh := reviewsFixture(t)
	out := testutil.Run(t, w, gh, "show", "-f", "llm", "backend#7").Stdout
	for _, want := range []string{
		"Pull request backend#7: Add the existing file",
		"by alice, open; needs your review. +1 −0 in 1 file. Its checks pass.",
		"Branch existing, for main.",
		"What it says it does:\n  Adds a file that exists.",
		"where it left origin/main, to its head",
		"existing.txt",
		"It isn't docked here",
		"Said in review on GitHub (1 thread, 1 open):",
		"existing.txt:1  carol: question: why does it exist?",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ws show backend#7 lacks %q:\n%s", want, out)
		}
	}
	// By an alias, too; and one that isn't open says so.
	if r := testutil.RunCommand(t, w.Root, gh, "show", "be#7"); r.Err != nil && !strings.Contains(r.Err.Error(), "isn't a repo") {
		t.Errorf("by an unknown alias: %v", r.Err)
	}
	if r := testutil.RunCommand(t, w.Root, gh, "show", "backend#99"); r.Err == nil || !strings.Contains(r.Err.Error(), "isn't open") {
		t.Errorf("a PR that isn't open: %v", r.Err)
	}
}

// Asking about a pull request docks it for the agent (not boarded: it's
// ws's to tidy up), starts an ask anchored to it and to the lines asked
// about (the workspace's, and not listed until it's kept), and its agent
// starts from ws show <repo>#<n>. The capsule goes once the PR has merged
// or closed, unless something's been changed in it.
func TestAskPR_DocksAndTidiesUp(t *testing.T) {
	w, gh := reviewsFixture(t)
	useFakeConverser(t, w)
	ctx := reviewsContext(t, w, gh)
	open, _ := gh.PRsForRepoFn("", "backend")
	id, err := testutil.AskPR(ctx, "backend", 7, web.PRQuestion{Body: "Why does it exist?", Path: "existing.txt", Side: "new", Line: 1, Commit: open[0].HeadRefOid})
	if err != nil {
		t.Fatal(err)
	}
	capsule := filepath.Join(w.Root, "repos", "backend", "existing")
	if info, err := os.Stat(capsule); err != nil || !info.IsDir() {
		t.Fatalf("it isn't docked: %v", err)
	}
	rec, _ := pulls.Load(w.Root, "backend", 7)
	if rec.Docked == nil || rec.Docked.By != pulls.ByWS || rec.Docked.Capsule != "existing" {
		t.Errorf("record = %+v", rec.Docked)
	}
	if after := reviewsContext(t, w, gh); after.WS().IsBoarded("backend", "existing") {
		t.Error("a capsule docked for a question is boarded")
	}
	talks := conversation.ForWorkspace(w.Root)
	c, err := talks.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if a := c.Anchor; a.PR != "backend#7" || a.Path != "existing.txt" || a.Line != 1 || a.EndLine != 1 || len(a.Excerpt) != 1 || a.Excerpt[0] != "existing" {
		t.Errorf("anchor = %+v", a)
	}
	if !c.Ephemeral {
		t.Error("an ask about a pull request is kept from the start")
	}
	if got := web.PRTalks(w.Root, "backend#7"); len(got) != 0 {
		t.Errorf("the PR's conversations, before it's kept = %+v", got)
	}

	// Its agent starts from the pull request, and may read where it's docked.
	testutil.Run(t, w, gh, "conversation", "answer", id)
	c, _ = talks.Load(id)
	if len(c.Turns) != 2 || c.Turns[1].Body != "about the pull request, knowing where to start and what it may propose, and which lines, reading where it is docked" {
		t.Fatalf("the agent's answer: %+v", c.Turns)
	}
	if out := testutil.Run(t, w, gh, "show", "-f", "llm", id).Stdout; !strings.Contains(out, "It's about pull request backend#7, existing.txt:1 (ws show backend#7).") {
		t.Errorf("ws show %s:\n%s", id, out)
	}
	if out := testutil.Run(t, w, gh, "show", "-f", "llm", "backend#7").Stdout; !strings.Contains(out, "It's docked at "+capsule) {
		t.Errorf("ws show backend#7 doesn't say where it's docked:\n%s", out)
	}
	// Kept, it's one of the PR's conversations.
	if _, err := talks.Keep(id, conversation.You, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := web.PRTalks(w.Root, "backend#7"); len(got) != 1 || got[0].ID != id {
		t.Errorf("the PR's conversations, kept = %+v", got)
	}

	// Asking again uses the capsule that's there.
	if _, err := testutil.AskPR(ctx, "backend", 7, web.PRQuestion{Body: "And who uses it?"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(capsule + "-2"); err == nil {
		t.Error("docked it a second time")
	}

	// While it's open, it stays. Once it's merged or closed, it goes.
	testutil.TidyDocked(ctx)
	if _, err := os.Stat(capsule); err != nil {
		t.Fatal("tidied up while the PR is open")
	}
	gh.PRsForRepoFn = func(org, repo string) ([]github.PR, error) { return nil, nil }
	testutil.TidyDocked(ctx)
	if _, err := os.Stat(capsule); err == nil {
		t.Error("the capsule is still there after the PR closed")
	}
	if rec, _ = pulls.Load(w.Root, "backend", 7); rec.Docked != nil {
		t.Errorf("still recorded as docked: %+v", rec.Docked)
	}
}

// A capsule ws docked isn't removed if you've changed something in it:
// it's yours then.
func TestTidyDocked_KeepsWhatYouChanged(t *testing.T) {
	w, gh := reviewsFixture(t)
	useFakeConverser(t, w)
	ctx := reviewsContext(t, w, gh)
	if _, err := testutil.AskPR(ctx, "backend", 7, web.PRQuestion{Body: "Why?"}); err != nil {
		t.Fatal(err)
	}
	capsule := filepath.Join(w.Root, "repos", "backend", "existing")
	os.WriteFile(filepath.Join(capsule, "scratch.txt"), []byte("mine"), 0o644)
	gh.PRsForRepoFn = func(org, repo string) ([]github.PR, error) { return nil, nil }
	testutil.TidyDocked(ctx)
	if _, err := os.Stat(capsule); err != nil {
		t.Fatal("removed a capsule with changes in it")
	}
	if rec, _ := pulls.Load(w.Root, "backend", 7); rec.Docked == nil || rec.Docked.By != pulls.ByYou {
		t.Errorf("record = %+v", rec.Docked)
	}
}

// Open here docks a pull request as ws dock does, and then it's yours; a
// capsule ws docked for a question becomes yours, boarded.
func TestOpenHere(t *testing.T) {
	w, gh := reviewsFixture(t)
	useFakeConverser(t, w)
	ctx := reviewsContext(t, w, gh)
	said, err := testutil.OpenHere(ctx, "backend", 7)
	if err != nil || !strings.HasPrefix(said, "Docked: ") {
		t.Fatalf("open here: %q, %v", said, err)
	}
	if !ctx.WS().IsBoarded("backend", "existing") {
		t.Error("docked, but not boarded as ws dock boards it")
	}
	if rec, _ := pulls.Load(w.Root, "backend", 7); rec.Docked == nil || rec.Docked.By != pulls.ByYou {
		t.Errorf("record = %+v", rec.Docked)
	}
	if said, err = testutil.OpenHere(ctx, "backend", 7); err != nil || !strings.HasPrefix(said, "It's here already") {
		t.Errorf("open here again: %q, %v", said, err)
	}

	// One ws docked for a question becomes yours.
	pulls.Update(w.Root, "backend", 7, func(r *pulls.Record) { r.Docked.By = pulls.ByWS })
	if said, err = testutil.OpenHere(ctx, "backend", 7); err != nil || !strings.HasPrefix(said, "It's yours now") {
		t.Fatalf("open here on ws's capsule: %q, %v", said, err)
	}
	if rec, _ := pulls.Load(w.Root, "backend", 7); rec.Docked.By != pulls.ByYou {
		t.Errorf("record = %+v", rec.Docked)
	}
	if after := reviewsContext(t, w, gh); !after.WS().IsBoarded("backend", "existing") {
		t.Error("it isn't boarded")
	}
	testutil.TidyDocked(ctx) // yours: never tidied
	if _, err := os.Stat(filepath.Join(w.Root, "repos", "backend", "existing")); err != nil {
		t.Error("tidied up a capsule that's yours")
	}
}

// A conversation's agent proposes a review comment with ws pr comment
// --propose: it's checked against the pull request as it is, and
// accepting it adds it to your draft review, marked as the agent's.
// Nothing is posted.
func TestPRComment_ProposedAndAdded(t *testing.T) {
	w, gh := reviewsFixture(t)
	useFakeConverser(t, w)
	ctx := reviewsContext(t, w, gh)
	posted := false
	gh.SubmitReviewFn = func(org, repo string, number int, r github.ReviewSubmission) (string, error) {
		posted = true
		return "", nil
	}
	id, err := testutil.AskPR(ctx, "backend", 7, web.PRQuestion{Body: "Anything wrong with it?"})
	if err != nil {
		t.Fatal(err)
	}
	if r := testutil.RunCommand(t, w.Root, gh, "pr", "comment", "--propose", "backend#7", "--path", "existing.txt", "--line", "1", "--kind", "nit", "x"); r.Err == nil || !strings.Contains(r.Err.Error(), "for a conversation's agent") {
		t.Errorf("proposing outside a conversation: %v", r.Err)
	}
	if r := testutil.RunCommand(t, w.Root, gh, "pr", "comment", "backend#7", "x"); r.Err == nil || !strings.Contains(r.Err.Error(), "--propose") {
		t.Errorf("commenting without --propose: %v", r.Err)
	}
	t.Setenv(conversation.EnvConversation, id)
	t.Setenv(mission.EnvCrew, conversation.Agent)
	for _, bad := range [][]string{
		{"--path", "existing.txt", "--line", "9", "--kind", "nit", "x"},                   // no such line
		{"--path", "nope.txt", "--line", "1", "--kind", "nit", "x"},                       // no such file
		{"--path", "existing.txt", "--line", "1", "--kind", "praise", "x"},                // no such kind
		{"--path", "existing.txt", "--line", "1", "--kind", "nit"},                        // nothing said
		{"--path", "existing.txt", "--line", "1", "--side", "left", "--kind", "nit", "x"}, // no such side
	} {
		if r := testutil.RunCommand(t, w.Root, gh, append([]string{"pr", "comment", "--propose", "backend#7"}, bad...)...); r.Err == nil {
			t.Errorf("proposed %v", bad)
		}
	}
	testutil.WithStdin(t, "it exists\n")
	testutil.Run(t, w, gh, "pr", "comment", "--propose", "backend#7", "--path", "existing.txt", "--line", "1", "--kind", "blocking", "--suggest", "-", "Say what it's for.")
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")

	talks := conversation.ForWorkspace(w.Root)
	c, _ := talks.Load(id)
	if len(c.Proposals) != 1 {
		t.Fatalf("proposals = %+v", c.Proposals)
	}
	p := c.Proposals[0]
	if p.Kind != conversation.ProposeComment || p.PR != "backend#7" || p.Path != "existing.txt" || p.Line != 1 || p.EndLine != 1 || p.NoteKind != "blocking" ||
		p.Text != "Say what it's for." || p.Suggestion == nil || *p.Suggestion != "it exists" || len(p.Excerpt) != 1 || p.Excerpt[0] != "existing" {
		t.Errorf("proposal = %+v", p)
	}

	r := testutil.Run(t, w, gh, "conversation", "accept", id, "1")
	if !strings.Contains(r.Stderr, "Added to your draft review of backend#7") {
		t.Errorf("accepting: %s", r.Stderr)
	}
	draft, err := pulls.Reviews(w.Root, "backend", 7).Draft(pulls.Key("backend", 7))
	if err != nil || draft == nil || len(draft.Comments) != 1 {
		t.Fatalf("draft = %+v, %v", draft, err)
	}
	if n := draft.Comments[0]; n.By != conversation.Agent || n.Kind != "blocking" || n.Body != "Say what it's for." || n.Path != "existing.txt" || n.Line != 1 || n.Side != "new" ||
		n.Suggestion == nil || *n.Suggestion != "it exists" || n.Status != review.Open {
		t.Errorf("the comment in the draft = %+v", n)
	}
	if posted {
		t.Error("accepting a proposed comment posted to GitHub")
	}
	if out := testutil.Run(t, w, gh, "show", "-f", "llm", id).Stdout; !strings.Contains(out, "1. review comment on backend#7 existing.txt:1 (blocking): Say what it's for. (added to the user's draft review)") {
		t.Errorf("ws show %s:\n%s", id, out)
	}
	if out := testutil.Run(t, w, gh, "show", "-f", "llm", "backend#7").Stdout; !strings.Contains(out, "a draft, not posted yet, with 1 comment:") || !strings.Contains(out, "(proposed by agent)") {
		t.Errorf("ws show backend#7 doesn't have the draft:\n%s", out)
	}

	// The web UI counts what needs you from the list as GitHub last gave it.
	svc := testutil.ReviewsService(ctx)
	if l, err := svc.List(false); err != nil || l.Count != 1 {
		t.Errorf("list = %+v, %v", l, err)
	}
}
