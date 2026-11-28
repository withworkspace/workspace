package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
)

// unmetRound is the cache mission at workPlan's step 1, c1's done checked
// once: the command is met, and the judge says the check in words isn't.
// The orchestrator's been woken for it, and can't pass the step.
func unmetRound(t *testing.T) (*Workspace, *[]string, string) {
	t.Helper()
	w, runs, dir := workFixture(t, "1. unmet: nothing keeps a lookup")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	if said := w.CrewDone("cache", "c1"); !strings.Contains(said, "the orchestrator decides what follows") {
		t.Fatalf("round 1: %q", said)
	}
	return w, runs, dir
}

// A step whose checks came round not all met, left by its crew and with
// nothing asked of it (the orchestrator only noted, or its question was
// dismissed), is yours on the mission's page: it says what the round came
// to, and passing it as it is is what it offers. Without that the mission
// rests with nothing but "launch crew", and nobody else can pass it.
func TestUnmetChecks_LeftIsYoursToPass(t *testing.T) {
	w, _, _ := unmetRound(t)
	_, err := w.AcceptStage("cache", AsOrchestrator, "looks done")
	refusedWith(t, err, "checks")

	h := watching(t, w, "cache").now()
	if h.Court != mission.CourtYou || h.Kind != mission.KindAccept || h.Action.Verb != mission.VerbAccept || h.Action.Label != "pass step 1 as is" ||
		!strings.Contains(h.Text, "Step 1's checks, round 1: 1 of 2 met; unmet: the lookup is cached. Pass as is, or resume c1") {
		t.Fatalf("the headline = %+v", h)
	}

	// Asked a question and dismissing it leaves the same way on.
	if _, err := w.Flag("cache", "Which should it be?", AsCrew("c1")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Dismiss("cache", AsYou); err != nil {
		t.Fatal(err)
	}
	if h := watching(t, w, "cache").now(); h.Action.Verb != mission.VerbAccept {
		t.Errorf("after dismissing: %+v", h)
	}

	// Sent back, it's its crew's again: nothing's yours to pass.
	w.Missions.ClearStatus("cache", "c1")
	m, _ := w.Missions.Load("cache")
	if _, _, ok := m.ChecksUnmet(""); ok {
		t.Error("a step its crew are back on is still yours to pass")
	}
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")

	if _, err := w.AcceptStage("cache", AsYou, ""); err != nil {
		t.Fatal(err)
	}
	if st := stepRecord(t, w, 1); !st.Accepted() || st.Record.AcceptedBy != "you" {
		t.Errorf("step 1 = %+v", st.Record)
	}
}

// The orchestrator's question at a step whose checks aren't all met is the
// step's own: passing it as it is and sending it back are among its
// answers, before the orchestrator's, so the user's answer doesn't have to
// go back through an orchestrator that can't pass the step.
func TestFlag_AtUnmetChecksIsTheSteps(t *testing.T) {
	w, _, _ := unmetRound(t)
	// A question of the orchestrator's already open is this one now.
	if _, err := w.Flag("cache", "Is the cache per org?", AsOrchestrator); err != nil {
		t.Fatal(err)
	}
	f, err := w.Flag("cache", "Check 2 asks for a cache, and you decided against one. Which should it be", AsOrchestrator, "Drop the check", "pass it", "Reverse the decision")
	if err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	task, open := m.StepTask(1)
	if !open || len(m.Tasks) != 1 || f.AtStep != 1 || f.Task.N != task.N || task.Verb != mission.TaskChecks || task.By != "orchestrator" ||
		strings.Join(task.Options, "|") != "Pass it|Send it back|Drop the check|Reverse the decision" ||
		!strings.HasPrefix(task.Text, "Check 2 asks for a cache, and you decided against one. Which should it be. Step 1's checks, round 1: 1 of 2 met. Pass as is, send back to c1 with what's unmet, or say what to do.") {
		t.Fatalf("the task = %+v (flagged %+v, tasks %+v)", task, f, m.Tasks)
	}

	a, err := w.AnswerTask("cache", task.N, AsYou, mission.ChecksPass)
	if err != nil || a.Next == "" {
		t.Fatalf("passing it: %+v, %v", a, err)
	}
	if st := stepRecord(t, w, 1); !st.Accepted() || st.Record.AcceptedBy != "you" {
		t.Errorf("step 1 = %+v", st.Record)
	}
}

// The user's answer in their own words at such a step stands over its
// checks: the orchestrator, run with it, may pass the step, which it
// couldn't before they'd said. An answer from before the latest round is
// about work that's been checked again since.
func TestFlag_TheUsersAnswerLetsTheOrchestratorPass(t *testing.T) {
	w, runs, _ := unmetRound(t)
	f, err := w.Flag("cache", "Check 2 can't be met as it's written. How should step 1 go?", AsOrchestrator)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.AcceptStage("cache", AsOrchestrator, "asked, not answered")
	refusedWith(t, err, "checks")

	a, err := w.AnswerTask("cache", f.Task.N, AsYou, "Accept it with check 2 waived")
	if err != nil || !strings.Contains(a.Next, "passed to the orchestrator; step 1 stays yours") {
		t.Fatalf("answering: %+v, %v", a, err)
	}
	m, _ := w.Missions.Load("cache")
	if _, open := m.StepTask(1); !open || (*runs)[len(*runs)-1] != "cache/answer" {
		t.Errorf("the step's task closed, or the orchestrator wasn't run: %+v, %v", m.Tasks, *runs)
	}

	if _, err := w.AcceptStage("cache", AsOrchestrator, "the user said to accept it, check 2 waived"); err != nil {
		t.Fatalf("the orchestrator passing it on the user's answer: %v", err)
	}
	if st := stepRecord(t, w, 1); !st.Accepted() || st.Record.AcceptedBy != "orchestrator" {
		t.Errorf("step 1 = %+v", st.Record)
	}
}

// An answer from before the latest round is about work that's been
// checked again since: it doesn't let the orchestrator pass that.
func TestFlag_AnAnswerBeforeTheRoundIsStale(t *testing.T) {
	w, _, _ := unmetRound(t)
	f, err := w.Flag("cache", "How should step 1 go?", AsOrchestrator)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AnswerTask("cache", f.Task.N, AsYou, "Go on, it's fine"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Missions.RecordChecks("cache", 1, mission.CheckRound{Crew: "c1", Capsule: "frontend/one", At: time.Now().Add(time.Minute),
		Results: []mission.CheckResult{{Check: "the lookup is cached", Result: mission.CheckUnmet}}}); err != nil {
		t.Fatal(err)
	}
	_, err = w.AcceptStage("cache", AsOrchestrator, "the user said to")
	refusedWith(t, err, "checks")
}

// Sent back from the question, the orchestrator still can't pass it.
func TestFlag_SendingBackIsntSayingToPass(t *testing.T) {
	w, _, _ := unmetRound(t)
	f, err := w.Flag("cache", "How should step 1 go?", AsOrchestrator)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AnswerTask("cache", f.Task.N, AsYou, mission.ChecksSendBack); err != nil {
		t.Fatal(err)
	}
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	_, err = w.AcceptStage("cache", AsOrchestrator, "sent back")
	refusedWith(t, err, "checks")
}

// A question anywhere else is a question, whoever asks: crew's at such a
// step, the orchestrator's once the step's crew are back on it, or at a
// step with no round.
func TestFlag_ElsewhereIsAQuestion(t *testing.T) {
	w, _, _ := unmetRound(t)
	f, err := w.Flag("cache", "Which cache?", AsCrew("c1"), "Redis")
	if err != nil || f.AtStep != 0 || !f.Task.Question() || f.Task.By != "c1" || strings.Join(f.Task.Options, "|") != "Redis" {
		t.Fatalf("crew's question: %+v, %v", f, err)
	}
	w.Missions.ClearStatus("cache", "c1")
	f, err = w.Flag("cache", "Which cache?", AsOrchestrator)
	if err != nil || f.AtStep != 0 || !f.Task.Question() || f.Task.By != "orchestrator" {
		t.Fatalf("the orchestrator's, its crew back on the step: %+v, %v", f, err)
	}
	// Taken back.
	if _, err := w.Flag("cache", "", AsOrchestrator); err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	if _, open := m.Question(); !open || len(m.Tasks) != 1 {
		t.Errorf("tasks = %+v", m.Tasks)
	}
}

// The bundle that shows a step's checks is yours to approve once they've
// come round not all met and its crew have left it, asked or not, and
// approving it passes the step; approved already, the orchestrator's
// question has nothing left to ask.
func TestUnmetChecks_ApprovingTheBundlePasses(t *testing.T) {
	for _, asked := range []bool{false, true} {
		w, _, dir := workFixture(t, "1. unmet: nothing keeps a lookup")
		os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
		os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
		if _, err := w.HandOverBundle("cache", "c1", bundleOf("shot.png"), dir); err != nil {
			t.Fatal(err)
		}
		w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
		w.CrewDone("cache", "c1")
		m, _ := w.Missions.Load("cache")
		if v := w.Bundles(m); !v[0].Asked || v[0].AskedAt != 1 {
			t.Fatalf("b1 isn't yours to approve at step 1: %+v", v[0])
		}
		if asked {
			f, err := w.Flag("cache", "How should step 1 go?", AsOrchestrator)
			if err != nil || !strings.Contains(f.Task.Text, "Review b1 on the Verify tab: approving it passes the step") {
				t.Fatalf("the question: %+v, %v", f, err)
			}
		}
		done, err := w.ApproveBundle("cache", "b1", AsYou)
		if err != nil || !strings.Contains(done.Next, "step 1 passed") {
			t.Fatalf("approving b1 (asked %v): %+v, %v", asked, done, err)
		}
		st := stepRecord(t, w, 1)
		if !st.Accepted() || st.Record.AcceptedBy != "you" || !strings.Contains(st.Record.Why, "you approved b1, checks as they were") {
			t.Errorf("step 1 (asked %v) = %+v", asked, st.Record)
		}
		if m, _ = w.Missions.Load("cache"); len(m.Tasks) != 0 && m.Tasks[0].Step == 1 {
			t.Errorf("step 1's task is still open: %+v", m.Tasks)
		}
	}
}

// The judge is told what the user has decided since the step was written:
// their decisions stand over the step's words. Crew's own aren't given,
// since a judge shown crew's record of the user's choice takes it for the
// user's; nor is one made before the step began, which was the plan's to
// take in.
func TestJudge_IsToldWhatTheUserDecidedSince(t *testing.T) {
	w, _, dir := workFixture(t, "1. met: nothing's cached, as d2 has it")
	// The plan was written, then its step began: a decision before either
	// isn't said.
	old := time.Now().Add(-time.Hour)
	os.Chtimes(w.Missions.PlanPath("cache"), old, old)
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	w.Missions.Decide("cache", mission.Decision{What: "No cache: the lookup is fast enough", By: "you", Why: "the answer to c1's question: cache it, or leave it?"})
	w.Missions.Decide("cache", mission.Decision{What: "Dropped the cache\nthe user asked for", By: "c1"})
	w.Missions.Decide("cache", mission.Decision{What: "Taken back", By: "you"})
	w.Missions.Withdraw("cache", "d3", "you", "not so")

	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "left uncached")
	w.CrewDone("cache", "c1")
	if st := stepRecord(t, w, 1); !st.Accepted() || st.Record.AcceptedBy != "ws" {
		t.Fatalf("step 1 = %+v", st.Record)
	}
	if len(fake.prompts) != 1 {
		t.Fatalf("the judge was asked %d times", len(fake.prompts))
	}
	p := fake.prompts[0]
	for _, want := range []string{
		"A decision of theirs stands over the step's words",
		"Only the decisions listed below are the user's",
		"## Decided by the user since the step was written",
		"- d1: No cache: the lookup is fast enough. Why: the answer to c1's question: cache it, or leave it?\n\n",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the judge's prompt is missing %q", want)
		}
	}
	if strings.Contains(p, "Taken back") || strings.Contains(p, "Dropped the cache") {
		t.Error("the judge is told a decision that was withdrawn, or crew's own")
	}

	// With nothing decided since, it's told nothing of decisions.
	if p := prompt.Judge(prompt.Judging{Checks: []string{"x"}}); strings.Contains(p, "Decided since") || strings.Contains(p, "stands over") {
		t.Error("a judge with nothing decided is told of decisions")
	}
	m, _ := w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	started := *states[0].Record.Started
	os.Chtimes(w.Missions.PlanPath("cache"), started.Add(time.Hour), started.Add(time.Hour))
	if got := w.decidedSince(m, states[0]); len(got) != 1 {
		// The plan revised since for the steps ahead: this step's words
		// still date from when it began.
		t.Errorf("decided since the step began = %v", got)
	}
}

// What you tell crew who'd asked you something is a decision of yours,
// with their question kept beside it: crew's own record of it is their
// word for yours.
func TestAnsweredCrew_IsYourDecision(t *testing.T) {
	w, _, _ := workFixture(t, "")
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewAsking, "Keep the VERSION file, or drop it?\n(A) drop it (B) keep it")
	m, _ := w.Missions.Load("cache")
	c, _ := m.CrewByID("c1")
	w.answeredCrew("cache", c, " A ")
	all, _ := w.Missions.Decisions("cache")
	if len(all) != 1 || all[0].By != "you" || all[0].What != "A" || all[0].Why != "" || all[0].Asked == nil || *all[0].Asked != (mission.Asked{By: "c1", Text: "Keep the VERSION file, or drop it? (A) drop it (B) keep it"}) {
		t.Fatalf("decisions = %+v", all)
	}
	// As agents read it, the question is still its why.
	if got := all[0].Reason(); got != "the answer to c1's question: Keep the VERSION file, or drop it? (A) drop it (B) keep it" {
		t.Errorf("its reason = %q", got)
	}
	// Told something it didn't ask about: that's talk, not a decision.
	w.Missions.ClearStatus("cache", "c1")
	m, _ = w.Missions.Load("cache")
	c, _ = m.CrewByID("c1")
	w.answeredCrew("cache", c, "carry on")
	if all, _ = w.Missions.Decisions("cache"); len(all) != 1 {
		t.Errorf("decisions = %+v", all)
	}
}

// A revision the orchestrator asked for that's yours to confirm (it
// changes Z) has the orchestrator look again once you accept it, as one
// that applies without you does: it was told it would be run again, and
// without that the mission rests where it was.
func TestPlanRevision_AcceptedRunsTheOrchestratorAgain(t *testing.T) {
	w, runs := fixture(t, "## Goal\nFix it.\n", "one", "two")
	w.Missions.WritePlan("cache", replanPlan)
	if _, err := w.AskPlanner("cache", "the fix needs the backend too", AsOrchestrator); err != nil {
		t.Fatal(err)
	}
	store := conversation.For(w.Root(), "cache")
	id := store.Planner().ID
	rev, err := w.RevisePlan("cache", id, strings.Replace(replanPlan, "- Merged, with its regression test.", "- Merged in both repos, with its regression test.", 1))
	if err != nil || rev.Proposed != 1 {
		t.Fatalf("revised: %+v, %v", rev, err)
	}
	if len(*runs) != 0 {
		t.Fatalf("the orchestrator ran before the user accepted: %v", *runs)
	}
	said, err := w.SettleProposalIn(store, id, 1, true, ProposalEdits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(*runs) != 1 || (*runs)[0] != "cache/plan" || !strings.Contains(said, "The orchestrator reviews the updated plan.") {
		t.Errorf("accepted: %q, runs %v", said, *runs)
	}
}

// Approving a step's bundle while its crew's newer done is being checked
// doesn't pass the step on the round before: ws passes it if the round
// running is met, and if not it goes as unmet checks do. (You approved b6 while
// step 3's second round was running, and it passed on the first.)
func TestUnmetChecks_ApprovingWhileCheckedAgainWaitsForTheRound(t *testing.T) {
	w, _, dir := workFixture(t, "1. unmet: nothing keeps a lookup")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	if _, err := w.HandOverBundle("cache", "c1", bundleOf("shot.png"), dir); err != nil {
		t.Fatal(err)
	}
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	w.CrewDone("cache", "c1")
	now := time.Now()
	if err := w.Missions.SetChecking("cache", mission.Checking{Step: 1, Crew: "c1", Of: 2, N: 1, Command: "test -f cache.go", Since: now, Until: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	done, err := w.ApproveBundle("cache", "b1", AsYou)
	if err != nil || !strings.Contains(done.Next, "step 1 isn't passed yet: ws is checking it again, for c1") {
		t.Fatalf("approving b1: %+v, %v", done, err)
	}
	if st := stepRecord(t, w, 1); st.Accepted() {
		t.Errorf("step 1 passed on the round before: %+v", st.Record)
	}
	if m, _ := w.Missions.Load("cache"); m.Bundles[0].Verdict != mission.BundleApproved {
		t.Errorf("b1's approval isn't kept: %+v", m.Bundles[0])
	}
}
