package ops

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/mission"
)

// prPlan is a draft pull request opened at step 1, then the user's
// review of what merged.
const prPlan = "## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n" +
	"1. Open Pull Request\n   - in: `frontend/one`\n   - draft: yes\n" +
	"2. Review: what merged.\n"

// prFixture is the cache mission at its Open Pull Request step, frontend#5
// open from frontend/one as a draft, its checks pending, on a GitHub that
// remembers what's marked ready and posted, and has said what said says.
type prGitHub struct {
	github.Stub
	readied []int
	posted  []string
	said    []github.Said
	threads []github.ReviewThread
}

func prFixture(t *testing.T) (*Workspace, *[]string, *prGitHub) {
	t.Helper()
	w, runs := fixture(t, "", "one")
	gh := &prGitHub{}
	gh.MarkReadyFn = func(org, repo string, n int) error { gh.readied = append(gh.readied, n); return nil }
	gh.CommentFn = func(org, repo string, n int, body string) error { gh.posted = append(gh.posted, body); return nil }
	gh.ReplyToThreadFn = func(org, repo string, n int, id, body string) error {
		gh.posted = append(gh.posted, id+": "+body)
		return nil
	}
	gh.DiscussionFn = func(string, string, int) ([]github.Said, error) { return gh.said, nil }
	gh.ReviewThreadsFn = func(string, string, int) ([]github.ReviewThread, error) { return gh.threads, nil }
	w.opts.GitHub = gh
	if err := w.Missions.WritePlan("cache", prPlan); err != nil {
		t.Fatal(err)
	}
	w.Missions.SeePR("cache", "frontend/one", mission.SeenPR{Number: 5, Draft: true, Checks: "pending"})
	return w, runs, gh
}

// watchPR has the watch see frontend#5 as pr says.
func watchPR(t *testing.T, w *Workspace, pr github.PR) {
	t.Helper()
	m, err := w.Missions.Load("cache")
	if err != nil {
		t.Fatal(err)
	}
	pr.Number = 5
	w.WatchPRs([]MissionView{{M: m, State: mission.Review, Capsules: []CapsuleView{{Capsule: m.Capsules[0], Exists: true, PR: &pr, PRS: mission.PROpen}}}})
}

func prLog(t *testing.T, w *Workspace) []string {
	t.Helper()
	events, _ := w.Missions.ReadLog("cache", 0)
	var out []string
	for _, e := range events {
		if e.Kind == mission.EventPR {
			out = append(out, e.What+": "+e.Text)
		}
	}
	return out
}

// Red checks at the step come round to the orchestrator, by the process,
// until max_rounds; then they're yours.
func TestPRStep_RedChecksComeRound(t *testing.T) {
	w, runs, _ := prFixture(t)
	w.cur.cfg.Missions.MaxRounds = 1
	watchPR(t, w, github.PR{IsDraft: true, StatusRollup: "failure"})
	if len(*runs) != 1 || (*runs)[0] != "cache/pr" || !strings.Contains(strings.Join(prLog(t, w), "|"), "checks-failed: frontend#5's checks are failing") {
		t.Fatalf("round 1: runs %v, log %v", *runs, prLog(t, w))
	}
	watchPR(t, w, github.PR{IsDraft: true, StatusRollup: "pending"})
	watchPR(t, w, github.PR{IsDraft: true, StatusRollup: "failure", Mergeable: "CONFLICTING"})
	m, _ := w.Missions.Load("cache")
	if q, ok := m.Question(); !ok || !strings.Contains(q.Text, "frontend#5 conflicts with its base, again: that's round 2") || len(*runs) != 1 {
		t.Errorf("round 2: question %+v, runs %v", q, *runs)
	}
	if st := stepRecord(t, w, 1); st.Record.Rounds != 2 {
		t.Errorf("rounds = %d", st.Record.Rounds)
	}
}

// A draft whose checks pass is the orchestrator's to mark ready, by the
// process; with the orchestrator off, yours to go on from, which marks it
// ready; and where ws holds review_before_ready, your review's.
func TestPRStep_GreenDraft(t *testing.T) {
	w, runs, gh := prFixture(t)
	watchPR(t, w, github.PR{IsDraft: true, StatusRollup: "success"})
	if len(*runs) != 1 || len(gh.readied) != 0 {
		t.Fatalf("green, the orchestrator on: runs %v, readied %v", *runs, gh.readied)
	}
	_, err := w.MarkReady("cache", "", AsCrew("c1"))
	refusedWith(t, err, "not-yours")
	if said, err := w.MarkReady("cache", "", AsOrchestrator); err != nil || !strings.Contains(said, "marked frontend#5 ready for review") || len(gh.readied) != 1 {
		t.Errorf("marked ready: %q, %v, %v", said, err, gh.readied)
	}
	_, err = w.MarkReady("cache", "", AsOrchestrator)
	refusedWith(t, err, "not-draft")

	// Off: the user's asked, and going on marks it ready.
	w2, _, gh2 := prFixture(t)
	off := false
	w2.cur.cfg.Orchestrator.Enabled = &off
	watchPR(t, w2, github.PR{IsDraft: true, StatusRollup: "success"})
	m, _ := w2.Missions.Load("cache")
	task := m.Asking()
	if task == nil || task.Step != 1 || task.Verb != mission.TaskApprove || !strings.Contains(task.Text, "go on, and ws marks it ready") {
		t.Fatalf("the task = %+v", task)
	}
	if a, err := w2.AnswerTask("cache", task.N, AsYou, mission.CheckInGoOn); err != nil || !strings.Contains(a.Next, "marked frontend#5 ready") || len(gh2.readied) != 1 {
		t.Errorf("going on: %+v, %v, %v", a, err, gh2.readied)
	}
	if m, _ = w2.Missions.Load("cache"); len(m.Tasks) != 0 {
		t.Errorf("tasks left: %+v", m.Tasks)
	}

	// Held: your review first; the orchestrator can't mark it ready.
	w3, runs3, gh3 := prFixture(t)
	held := true
	w3.cur.cfg.Missions.ReviewBeforeReady = &held
	watchPR(t, w3, github.PR{IsDraft: true, StatusRollup: "success"})
	m, _ = w3.Missions.Load("cache")
	if task := m.Asking(); task == nil || !strings.Contains(task.Text, "ws holds missions.review_before_ready: review frontend/one") || len(*runs3) != 0 {
		t.Errorf("held: task %+v, runs %v", task, *runs3)
	}
	_, err = w3.MarkReady("cache", "", AsOrchestrator)
	refusedWith(t, err, "guarantee")
	if len(gh3.readied) != 0 {
		t.Errorf("marked ready past the guarantee: %v", gh3.readied)
	}
}

// What people say on the PR at its step is news, once: the orchestrator's
// to act on by the process, what you said yourself not.
func TestPRStep_WhatsSaid(t *testing.T) {
	w, runs, gh := prFixture(t)
	gh.said = []github.Said{{ID: "C1", Author: "alice", Body: "Early thoughts", At: "2026-11-15T09:00:00Z"}}
	watchPR(t, w, github.PR{IsDraft: true, StatusRollup: "pending"}) // where reading starts
	if len(*runs) != 0 || len(prLog(t, w)) != 0 {
		t.Fatalf("what was said before is news: %v, %v", *runs, prLog(t, w))
	}
	gh.said = append(gh.said, github.Said{ID: "C2", Author: "you", Body: "Thanks!", At: "2026-11-15T10:00:00Z"},
		github.Said{ID: "R1", Author: "bob", State: "CHANGES_REQUESTED", Body: "Key it per tenant.", At: "2026-11-15T10:05:00Z"})
	gh.threads = []github.ReviewThread{{ID: "PRRT_1", Path: "cache.go", Line: 12, Comments: []github.ThreadComment{{Author: "bob", Body: "Why a map?", At: "2026-11-15T10:06:00Z"}}}}
	watchPR(t, w, github.PR{IsDraft: true, StatusRollup: "pending", ReviewDecision: "CHANGES_REQUESTED"})
	got := strings.Join(prLog(t, w), "|")
	for _, want := range []string{"said: bob reviewed frontend#5 (changes requested): Key it per tenant.", "said: bob on frontend#5, `cache.go:12` (thread PRRT_1): Why a map?", "changes-requested: frontend#5: a reviewer asked for changes"} {
		if !strings.Contains(got, want) {
			t.Errorf("the log lacks %q: %s", want, got)
		}
	}
	if strings.Contains(got, "Thanks!") || strings.Contains(got, "Early thoughts") || len(*runs) != 1 {
		t.Errorf("news: %s, runs %v", got, *runs)
	}
	watchPR(t, w, github.PR{IsDraft: true, StatusRollup: "pending", ReviewDecision: "CHANGES_REQUESTED"})
	if len(*runs) != 1 {
		t.Errorf("said twice: runs %v", *runs)
	}
}

// Crew draft replies: to a person, it waits for you, and you post it or
// not; to a bot, it's posted at once. The orchestrator doesn't draft them.
func TestPRStep_Replies(t *testing.T) {
	w, _, gh := prFixture(t)
	gh.threads = []github.ReviewThread{{ID: "PRRT_1", Path: "cache.go", Comments: []github.ThreadComment{{Author: "bob", Body: "Why a map?", At: "2026-11-15T10:06:00Z"}}}}
	gh.said = []github.Said{{ID: "C9", Author: "codecov", Bot: true, Body: "Coverage down 2%", At: "2026-11-15T10:00:00Z"}}

	_, err := w.DraftReply("cache", "frontend#5", "PRRT_1", "", "A map's enough here.", AsOrchestrator)
	refusedWith(t, err, "not-yours")
	_, err = w.DraftReply("cache", "frontend#9", "", "bob", "Hi", AsCrew("c1"))
	refusedWith(t, err, "not-the-missions")

	r, err := w.DraftReply("cache", "frontend#5", "PRRT_1", "", "A map's enough: lookups are by key.", AsCrew("c1"))
	if err != nil || r.Posted || r.Reply.To != "bob" || len(gh.posted) != 0 {
		t.Fatalf("to a person: %+v, %v, posted %v", r, err, gh.posted)
	}
	m, _ := w.Missions.Load("cache")
	task := m.Asking()
	if task == nil || task.Verb != mission.TaskReply || task.Reply != 1 || !strings.Contains(task.Text, "c1 drafted a reply on thread PRRT_1 on frontend#5, to bob: “A map's enough") || strings.Join(task.Options, "|") != "Post it|Don't post" {
		t.Fatalf("the task = %+v", task)
	}
	if a, err := w.AnswerTask("cache", task.N, AsYou, mission.ReplyPost); err != nil || a.Next != "posted it on thread PRRT_1 on frontend#5" || strings.Join(gh.posted, "|") != "PRRT_1: A map's enough: lookups are by key." {
		t.Errorf("posting it: %+v, %v, %v", a, err, gh.posted)
	}
	if m, _ = w.Missions.Load("cache"); m.Replies[0].Posted == nil || len(m.Tasks) != 0 {
		t.Errorf("after posting: %+v, tasks %+v", m.Replies[0], m.Tasks)
	}

	// Another, not posted.
	w.DraftReply("cache", "frontend#5", "", "bob", "Thanks for looking.", AsCrew("c1"))
	m, _ = w.Missions.Load("cache")
	w.AnswerTask("cache", m.Asking().N, AsYou, mission.ReplyDont)
	if m, _ = w.Missions.Load("cache"); !m.Replies[1].Dropped || len(gh.posted) != 1 {
		t.Errorf("not posted: %+v, %v", m.Replies[1], gh.posted)
	}

	// To a bot: posted at once.
	if r, err := w.DraftReply("cache", "frontend#5", "", "codecov", "Covered by the new tests.", AsCrew("c1")); err != nil || !r.Posted || gh.posted[1] != "@codecov Covered by the new tests." {
		t.Errorf("to a bot: %+v, %v, %v", r, err, gh.posted)
	}
	// replies_need_me off: posted at once.
	off := false
	w.cur.cfg.Missions.RepliesNeedMe = &off
	if r, err := w.DraftReply("cache", "frontend#5", "", "bob", "Done.", AsCrew("c1")); err != nil || !r.Posted {
		t.Errorf("not held: %+v, %v", r, err)
	}
}

// Closed without merging is the orchestrator's call; with it off, yours.
func TestPRStep_Closed(t *testing.T) {
	w, runs, gh := prFixture(t)
	gh.PRFromNumberFn = func(org, repo string, n int) (*github.PR, error) {
		return &github.PR{Number: n, State: "CLOSED"}, nil
	}
	m, _ := w.Missions.Load("cache")
	w.WatchPRs([]MissionView{{M: m, State: mission.Review, Capsules: []CapsuleView{{Capsule: m.Capsules[0], Exists: true}}}})
	if got := strings.Join(prLog(t, w), "|"); got != "closed: frontend#5 was closed without merging" || len(*runs) != 1 {
		t.Errorf("closed: log %q, runs %v", got, *runs)
	}
}

// The first thing anyone says on a PR nobody had said anything on is news.
func TestPRStep_FirstThingSaid(t *testing.T) {
	w, runs, gh := prFixture(t)
	watchPR(t, w, github.PR{IsDraft: true, StatusRollup: "pending"})
	gh.said = []github.Said{{ID: "C1", Author: "alice", Body: "Looks good so far", At: "2026-11-15T09:00:00Z"}}
	watchPR(t, w, github.PR{IsDraft: true, StatusRollup: "pending"})
	if got := strings.Join(prLog(t, w), "|"); got != "said: alice commented on frontend#5: Looks good so far" || len(*runs) != 1 {
		t.Errorf("the first comment: %q, runs %v", got, *runs)
	}
}

// Merged, the step passes, and what ws asked about the pull request while
// it was open is moot.
func TestPRStep_MergedTakesBackItsQuestion(t *testing.T) {
	w, _, _ := prFixture(t)
	off := false
	w.cur.cfg.Orchestrator.Enabled = &off
	watchPR(t, w, github.PR{IsDraft: true, StatusRollup: "failure"})
	if m, _ := w.Missions.Load("cache"); m.Asking() == nil || !strings.Contains(m.Asking().Text, "frontend#5's checks are failing. The orchestrator isn't running") {
		t.Fatalf("asked: %+v", m.Asking())
	}
	m, _ := w.Missions.Load("cache")
	pr := github.PR{Number: 5}
	w.WatchPRs([]MissionView{{M: m, State: mission.Review, Capsules: []CapsuleView{{Capsule: m.Capsules[0], Exists: true, PR: &pr, PRS: mission.PRMerged}}}})
	m, _ = w.Missions.Load("cache")
	if st := stepRecord(t, w, 1); !st.Accepted() || len(m.Tasks) != 1 || m.Tasks[0].Step != 2 {
		t.Errorf("after the merge: step 1 %+v, tasks %+v", st.Record, m.Tasks)
	}
}

// A merge that passes the plan's last step brings the mission to Z: the
// orchestrator is woken to suggest how it ends. One that leaves a step to
// go doesn't wake it.
func TestPRStep_MergeAtTheLastStepWakesTheOrchestrator(t *testing.T) {
	merge := func(w *Workspace) {
		m, _ := w.Missions.Load("cache")
		pr := github.PR{Number: 5}
		w.WatchPRs([]MissionView{{M: m, State: mission.Review, Capsules: []CapsuleView{{Capsule: m.Capsules[0], Exists: true, PR: &pr, PRS: mission.PRMerged}}}})
	}
	w, runs, _ := prFixture(t)
	merge(w)
	if st := stepRecord(t, w, 1); !st.Accepted() || len(*runs) != 0 {
		t.Errorf("a step to go: step 1 %+v, orchestrator ran %v", st.Record, *runs)
	}

	w, runs, _ = prFixture(t)
	if err := w.Missions.WritePlan("cache", "## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n1. Open Pull Request\n   - in: `frontend/one`\n"); err != nil {
		t.Fatal(err)
	}
	merge(w)
	if st := stepRecord(t, w, 1); !st.Accepted() || len(*runs) != 1 || (*runs)[0] != "cache/pr" {
		t.Errorf("the last step: step 1 %+v, orchestrator ran %v", st.Record, *runs)
	}
}
