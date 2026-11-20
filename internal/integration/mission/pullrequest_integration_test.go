package mission_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

const reviewProcess = "We always self-review code before opening a pull request. Bugs are reproduced first.\n"

// holdReviews writes the workspace's process, and has ws hold the user's
// review before a pull request and before landing: guarantees, in
// ws.local.toml's [missions].
func holdReviews(t *testing.T, root string) {
	t.Helper()
	os.WriteFile(filepath.Join(root, "PROCESS.md"), []byte(reviewProcess), 0o644)
	local := filepath.Join(root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[missions]\nreview_before_pr = true\nreview_before_land = true\n")...), 0o644)
}

// fixtureGitHub is a GitHub that remembers the pull requests opened on it.
func fixtureGitHub(t *testing.T) github.FixtureClient {
	t.Helper()
	path := filepath.Join(t.TempDir(), "github.json")
	os.WriteFile(path, []byte(`{"viewer":"me","repos":{}}`), 0o644)
	return github.FixtureClient{Path: path}
}

const prPlan = "## Z\n- Cached, and merged.\n## Stops early\n- None.\n## Steps\n" +
	"1. Cache it.\n   - who: crew, in `backend/cache-lookup`\n" +
	"2. Is it right?\n   - who: you\n" +
	"3. Open its pull request.\n   - who: crew, in `backend/cache-lookup`\n   - makes: a pull request\n" +
	"4. Merged.\n   - who: outside: a teammate's review\n"

// A pull request is opened by the step that makes one, through ws, which
// holds its guarantees about it: crew open it at that step and no other,
// the orchestrator can't, and doesn't accept the step until one's open,
// and review_before_pr refuses it until the user's review approves the
// capsule as it is.
func TestOpenPR_ByAStepHoldingTheGuarantees(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	gh := fixtureGitHub(t)
	holdReviews(t, w.Root)
	testutil.WithStdin(t, prPlan)
	testutil.Run(t, w, gh, "mission", "plan", "-m", "cache", "--set", "-")
	store.JoinStage("cache", "c1", 1)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")

	// The orchestrator is given the process as it's written, what ws holds,
	// and a shorter prompt: the route's rules are ws's to hold now.
	prompt := testutil.Run(t, w, gh, "mission", "orchestrate", "-m", "cache", "--print-prompt").Stdout
	for _, want := range []string{"## The process", "We always self-review code before opening a pull request.", "## What ws holds", "- ws opens a pull request from a mission's capsule only once the user's latest review of it approves the commit it's at.", "ws holds the route", "ws mission resume-with"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("orchestrator prompt lacks %q", want)
		}
	}
	for _, gone := range []string{"Leave the mission in someone's court", "Don't dispatch the next step yourself", "An artifact still a draft"} {
		if strings.Contains(prompt, gone) {
			t.Errorf("orchestrator prompt still has %q", gone)
		}
	}
	if out := testutil.Run(t, w, gh, "show", "-m", "cache", "-f", "llm", "s3").Stdout; !strings.Contains(out, "It opens a pull request, through ws") {
		t.Errorf("ws show s3:\n%s", out)
	}
	if out := testutil.Run(t, w, gh, "process").Stdout; !strings.Contains(out, "missions.review_before_pr: ws opens a pull request") || !strings.Contains(out, "missions.review_before_land: ws lands") {
		t.Errorf("ws process:\n%s", out)
	}
	t.Setenv("WS_ROLE", "orchestrator")
	if r := testutil.RunCommand(t, w.Root, gh, "mission", "dispatch", "-m", "cache", "--stage", "3", "open it"); r.Err == nil || !strings.Contains(r.Err.Error(), "step 3 of the plan isn't reached yet: it's at step 1") {
		t.Errorf("the orchestrator dispatching ahead: %v", r.Err)
	}
	t.Setenv("WS_ROLE", "")
	testutil.CommitIn(t, capsule, "cache.go", "package cache\n", "Cache the lookup")

	openPR := func(env map[string]string, args ...string) testutil.Result {
		t.Helper()
		for _, k := range []string{mission.EnvMission, mission.EnvCrew, "WS_ROLE"} {
			t.Setenv(k, env[k])
		}
		return testutil.RunCommand(t, w.Root, gh, append([]string{"mission", "open-pr", "-m", "cache"}, args...)...)
	}
	asCrew := func(id string) map[string]string {
		return map[string]string{mission.EnvMission: "cache", mission.EnvCrew: id}
	}
	orchestrator := map[string]string{mission.EnvMission: "cache", "WS_ROLE": "orchestrator"}

	// Crew at a step that doesn't make one: refused, saying which does.
	if r := openPR(asCrew("c1"), "--title", "Cache it"); r.Err == nil || !strings.Contains(r.Err.Error(), "step 1 doesn't make a pull request") || !strings.Contains(r.Err.Error(), "Step 3 opens it") {
		t.Errorf("crew at step 1: %v", r.Err)
	}
	// The orchestrator never opens one.
	if r := openPR(orchestrator, "--title", "Cache it"); r.Err == nil || !strings.Contains(r.Err.Error(), "opened by the step that makes one") {
		t.Errorf("the orchestrator: %v", r.Err)
	}

	// Step 1 accepted: step 2, yours, asks you to review here, since step 3
	// opens a pull request and ws holds the user's review before one.
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	t.Setenv("WS_ROLE", "orchestrator")
	testutil.Run(t, w, gh, "mission", "accept", "-m", "cache", "cached")
	if m := testutil.LoadMission(t, w, "cache"); m.Asking() == nil || !strings.Contains(m.Asking().Text, "Step 3 opens a pull request, and ws holds missions.review_before_pr: review the work here") {
		t.Errorf("the checkpoint's check-in: %+v", m.Asking())
	}
	t.Setenv("WS_ROLE", "")
	testutil.Run(t, w, gh, "mission", "accept", "-m", "cache", "go on")
	c2, _ := store.StartCrew("cache", "cursor", "conv-2")
	store.SetCrewProcess("cache", c2.ID, 0, "backend/cache-lookup")
	store.JoinStage("cache", c2.ID, 3)

	// At step 3, the orchestrator can't accept it with no pull request.
	t.Setenv("WS_ROLE", "orchestrator")
	if r := testutil.RunCommand(t, w.Root, gh, "mission", "accept", "-m", "cache", "done"); r.Err == nil || !strings.Contains(r.Err.Error(), "step 3 makes a pull request, and none is open from backend/cache-lookup") {
		t.Errorf("accepting step 3 with no pull request: %v", r.Err)
	}

	// Its crew are refused until the user's review approves it as it is.
	if r := openPR(asCrew(c2.ID), "--title", "Cache it"); r.Err == nil || !strings.Contains(r.Err.Error(), "ws holds missions.review_before_pr") || !strings.Contains(r.Err.Error(), "the user hasn't reviewed backend/cache-lookup") || !strings.Contains(r.Err.Error(), "ws mission ask") {
		t.Errorf("unreviewed: %v", r.Err)
	}
	d, err := review.Compare(capsule, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	reviews := review.For(w.Root, "cache")
	reviews.Submit("backend/cache-lookup", review.Diffed{Dir: capsule, BaseRef: "origin/main", Base: d.Base, Head: d.Head}, review.Approve, "", time.Now())
	testutil.CommitIn(t, capsule, "cache_test.go", "package cache\n", "Test it")
	if r := openPR(asCrew(c2.ID), "--title", "Cache it"); r.Err == nil || !strings.Contains(r.Err.Error(), "approved "+d.Head[:7]+", and it's at") {
		t.Errorf("moved on since the review: %v", r.Err)
	}
	head := strings.TrimSpace(testutil.GitOut(t, capsule, "rev-parse", "HEAD"))
	reviews.Submit("backend/cache-lookup", review.Diffed{Dir: capsule, BaseRef: "origin/main", Base: d.Base, Head: head}, review.Approve, "", time.Now().Add(time.Minute))

	r := openPR(asCrew(c2.ID), "--title", "Cache the lookup", "--body", "It was slow.")
	if r.Err != nil {
		t.Fatalf("opening it: %v\n%s", r.Err, r.Stderr)
	}
	if !strings.Contains(r.Stderr, "Opened backend#1 from backend/cache-lookup, at step 3") || strings.TrimSpace(r.Stdout) != "https://github.com/test-org/backend/pull/1" {
		t.Errorf("said:\n%s\n%s", r.Stderr, r.Stdout)
	}
	open, _ := gh.PRsForRepo("test-org", "backend")
	if len(open) != 1 || open[0].HeadRefName != "cache-lookup" || open[0].Title != "Cache the lookup" {
		t.Errorf("GitHub has %+v", open)
	}
	if got := strings.TrimSpace(testutil.GitOut(t, w.Sources["backend"], "rev-parse", "cache-lookup")); got != head {
		t.Errorf("the remote's branch is at %s, want %s", got, head)
	}
	if !logged(t, store, "backend#1 opened, at step 3: Cache the lookup") {
		t.Error("the log doesn't say it was opened")
	}
	if r := openPR(asCrew(c2.ID), "--title", "Again"); r.Err == nil || !strings.Contains(r.Err.Error(), "backend#1 is open from backend/cache-lookup already") {
		t.Errorf("a second: %v", r.Err)
	}

	// Now the orchestrator accepts it: the next step waits outside.
	store.SetCrewStatus("cache", c2.ID, mission.CrewDone, "opened it")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv("WS_ROLE", "orchestrator")
	if r := testutil.RunCommand(t, w.Root, gh, "mission", "accept", "-m", "cache", "opened"); r.Err != nil || !strings.Contains(r.Stderr, "step 4 waits on a teammate's review, and nobody's watching for it") {
		t.Errorf("accepting step 3 with its pull request open: %v\n%s", r.Err, r.Stderr)
	}
}

// A mission without a plan has no steps: its crew may open one through
// ws whenever they like, and the guarantees ws holds still hold.
func TestOpenPR_WithoutAPlan(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	gh := fixtureGitHub(t)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c1")
	if r := testutil.RunCommand(t, w.Root, gh, "mission", "open-pr", "--title", "x"); r.Err == nil || !strings.Contains(r.Err.Error(), "nothing to open a pull request for") {
		t.Errorf("nothing committed: %v", r.Err)
	}
	os.WriteFile(filepath.Join(capsule, "cache.go"), []byte("package cache\n"), 0o644)
	if r := testutil.RunCommand(t, w.Root, gh, "mission", "open-pr", "--title", "x"); r.Err == nil || !strings.Contains(r.Err.Error(), "changes not committed") {
		t.Errorf("uncommitted: %v", r.Err)
	}
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "commit", "-q", "-m", "cache")
	if r := testutil.RunCommand(t, w.Root, gh, "mission", "open-pr"); r.Err == nil || !strings.Contains(r.Err.Error(), "--title") {
		t.Errorf("no title: %v", r.Err)
	}
	testutil.Run(t, w, gh, "mission", "open-pr", "--title", "Cache it", "--draft")
	if open, _ := gh.PRsForRepo("test-org", "backend"); len(open) != 1 || !open[0].IsDraft {
		t.Errorf("GitHub has %+v", open)
	}
}

// ws holds the user's review before landing, as a guarantee.
func TestLand_HoldsTheReviewGuarantee(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	holdReviews(t, w.Root)
	testutil.Run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nA note.\n")
	t.Setenv(mission.EnvMission, "notes")
	testutil.Run(t, w, nil, "lift", "records", "note")
	t.Setenv(mission.EnvMission, "")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	if r := testutil.RunCommand(t, w.Root, nil, "land", "records", "note"); r.Err == nil || !strings.Contains(r.Err.Error(), "ws holds missions.review_before_land") || !strings.Contains(r.Err.Error(), "Review it first") {
		t.Fatalf("landing unreviewed: %v", r.Err)
	}
	d, err := review.Compare(dir, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := review.For(w.Root, "notes").Submit("records/note", review.Diffed{Dir: dir, BaseRef: "origin/main", Base: d.Base, Head: d.Head}, review.Approve, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	testutil.CommitIn(t, w.Sources["records"], "notes/other.md", "# Other\n", "docs: add another note")
	testutil.Run(t, w, nil, "land", "records", "note")
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "docs: add note a | docs: add another note | initial" {
		t.Errorf("approved change didn't land after a clean rebase: %s", got)
	}
}

func TestLand_AcceptsApprovalFromChangesSinceTheLastReview(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	holdReviews(t, w.Root)
	testutil.Run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nA note.\n")
	t.Setenv(mission.EnvMission, "notes")
	testutil.Run(t, w, nil, "lift", "records", "note")
	t.Setenv(mission.EnvMission, "")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	first := strings.TrimSpace(testutil.GitOut(t, dir, "rev-parse", "HEAD"))
	testutil.CommitIn(t, dir, "notes/a.md", "# A with more detail\n", "docs: clarify note a")
	d, err := review.CompareSince(dir, "origin/main", first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := review.For(w.Root, "notes").Submit("records/note", review.Diffed{Dir: dir, BaseRef: "origin/main", Base: d.Base, Head: d.Head}, review.Approve, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	testutil.Run(t, w, nil, "land", "records", "note")
}

func TestLand_RequiresReviewOfChangesMadeDuringRebase(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	holdReviews(t, w.Root)
	testutil.Run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nA note.\n")
	t.Setenv(mission.EnvMission, "notes")
	testutil.Run(t, w, nil, "lift", "records", "note")
	t.Setenv(mission.EnvMission, "")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	d, err := review.Compare(dir, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := review.For(w.Root, "notes").Submit("records/note", review.Diffed{Dir: dir, BaseRef: "origin/main", Base: d.Base, Head: d.Head}, review.Approve, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	testutil.CommitIn(t, w.Sources["records"], "notes/other.md", "# Other\n", "docs: add another note")
	hooks := t.TempDir()
	testutil.GitCmd(t, dir, "config", "core.hooksPath", hooks)
	if err := os.WriteFile(filepath.Join(hooks, "post-rewrite"), []byte(`#!/bin/sh
set -e
printf 'New during rebase\n' > notes/rewrite.md
git add notes/rewrite.md
git -c user.name=Test -c user.email=test@test.com -c commit.gpgsign=false commit -qm rewritten
`), 0755); err != nil {
		t.Fatal(err)
	}

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "note")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "the rebased changes differ from what was approved") {
		t.Fatalf("land: %v, want a fresh review of the changed diff", r.Err)
	}
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "docs: add another note | initial" {
		t.Errorf("unapproved changes landed: %s", got)
	}
}

func logged(t *testing.T, store mission.Store, text string) bool {
	t.Helper()
	events, _ := store.ReadLog("cache", 0)
	for _, e := range events {
		if strings.Contains(e.Text, text) {
			return true
		}
	}
	return false
}

// On the mission's page: the step that makes a pull request is marked,
// and Open pull request opens one through ws, refused (saying why) until
// the guarantees ws holds allow it. Settings shows them.
func TestWeb_OpenPR(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	gh := fixtureGitHub(t)
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[github]\nfixture = \""+gh.Path+"\"\n")...), 0o644)
	holdReviews(t, w.Root)
	act := testutil.WebAct(w.Root)
	if _, err := act("cache", web.ActRequest{Verb: "edit-plan", Text: prPlan}); err != nil {
		t.Fatal(err)
	}
	d, _ := testutil.WebMissionAnswered(w.Root)("cache")
	if d == nil || len(d.Stages) != 4 || !d.Stages[2].OpensPR || d.Stages[0].OpensPR {
		t.Fatalf("the steps: %+v", d)
	}
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	testutil.CommitIn(t, capsule, "cache.go", "package cache\n", "Cache the lookup")
	open := web.ActRequest{Verb: "open-pr", Capsule: "backend/cache-lookup", Title: "Cache the lookup", Text: "It was slow."}
	if _, err := act("cache", open); err == nil || !strings.Contains(err.Error(), "Review it first") {
		t.Errorf("unreviewed, from the page: %v", err)
	}
	d2, _ := review.Compare(capsule, "origin/main")
	review.For(w.Root, "cache").Submit("backend/cache-lookup", review.Diffed{Dir: capsule, BaseRef: "origin/main", Base: d2.Base, Head: d2.Head}, review.Approve, "", time.Now())
	said, err := act("cache", open)
	if err != nil || !strings.Contains(said, "Opened backend#1 from backend/cache-lookup") {
		t.Fatalf("opened: %q, %v", said, err)
	}
	if d, _ = testutil.WebMissionAnswered(w.Root)("cache"); d.Capsules[0].PR == nil || d.Capsules[0].PR.Number != 1 {
		t.Errorf("the page's capsule: %+v", d.Capsules[0])
	}

	ctx, err := cli.LoadOps(w.Root)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := ctx.Settings()
	if err != nil {
		t.Fatal(err)
	}
	held := map[string]any{}
	for _, g := range settings.Groups {
		for _, st := range g.Settings {
			if st.Section == "missions" {
				held[st.Key] = st.Value
			}
		}
	}
	if held["review_before_pr"] != true || held["review_before_land"] != true {
		t.Errorf("Settings' guarantees: %v", held)
	}
	if info := web.ProcessOf(mustProcess(t, w.Root)); info.Text != reviewProcess {
		t.Errorf("Settings' process: %+v", info)
	}
}

func mustProcess(t *testing.T, root string) ops.ProcessView {
	t.Helper()
	ctx, err := cli.LoadOps(root)
	if err != nil {
		t.Fatal(err)
	}
	v, err := ctx.Process()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// A pull request that would conflict from the start isn't opened, by crew
// or anyone: it says what conflicts, and to merge main in first.
func TestOpenPR_RefusesOneThatConflicts(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	gh := fixtureGitHub(t)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	testutil.CommitIn(t, capsule, "cache.go", "package cache\n\n// Lookup caches.\n", "Cache the lookup")
	testutil.CommitIn(t, w.Sources["backend"], "cache.go", "package cache\n\n// Lookup, uncached.\n", "Another lookup")
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c1")
	r := testutil.RunCommand(t, w.Root, gh, "mission", "open-pr", "--title", "Cache it")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "backend/cache-lookup doesn't merge cleanly onto main: it conflicts in cache.go. Merge origin/main into it (don't rebase, don't force-push)") {
		t.Errorf("opening one that conflicts: %v", r.Err)
	}
	if open, _ := gh.PRsForRepo("test-org", "backend"); len(open) != 0 {
		t.Errorf("GitHub has %+v", open)
	}
}
