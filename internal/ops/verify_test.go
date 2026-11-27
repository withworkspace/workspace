package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
)

// verifyPlan is work, then its verification (a check in words, a command,
// and one that's the user's), then the user's review.
const verifyPlan = "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n" +
	"1. Work: fix it.\n   - in: `frontend/one`\n" +
	"2. Verify: the fix, running.\n   - in: `frontend/one`\n   - check: the page loads\n   - check: `test -f fixed` passes\n   - yours: it reads well\n" +
	"3. Review: the fix.\n"

// verifyFixture is the cache mission on verifyPlan, c1 having done step 1
// in frontend/one, and the mission at step 2 with its verifier started.
func verifyFixture(t *testing.T) (*Workspace, *fakeHost, string) {
	t.Helper()
	w, _, host := reposFixture(t, verifyPlan)
	c, _ := w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewProcess("cache", c.ID, 1, "frontend/one")
	w.Missions.JoinStage("cache", c.ID, 1)
	w.Missions.SetCrewStatus("cache", c.ID, mission.CrewDone, "fixed")
	if _, err := w.AcceptStage("cache", AsOrchestrator, "fixed"); err != nil {
		t.Fatal(err)
	}
	return w, host, filepath.Join(w.WS().RepoDir("frontend"), "one")
}

// verifierBundle is a verifier's bundle for verifyPlan's step 2: its check
// in words as verdict says, the user's left for them, the command left out.
func verifierBundle(verdict string) string {
	return "## Summary\nRan it.\n\n## Checks\n1. " + verdict + ": the page came up\n   - shows: `shot.png` The page\n3. yours: at 390 px wide\n   - shows: `shot.png` Narrow\n\n## Not verified\nNothing.\n"
}

// A Verify step starts a verifier, fresh crew who didn't do the work, told
// to show it and change nothing. Their bundle is their done: ws runs the
// step's commands itself, and with nothing unmet passes the step, what's
// the user's left for them in the bundle the Review then looks at.
func TestVerify_ABundleWithNothingUnmetPassesTheStep(t *testing.T) {
	w, host, dir := verifyFixture(t)
	if len(host.specs) != 1 {
		t.Fatalf("launched %d crew", len(host.specs))
	}
	told := host.specs[0].Prompt
	for _, want := range []string{
		"You are verifying work on the mission", "You're on step 2 of the plan's 3, a Verify step",
		"1. the page loads (yours to settle: met, unmet or can't tell)", "2. `test -f fixed` passes (a command: ws runs it itself",
		"3. it reads well (the user's to settle, not yours", "The work is in the capsule `frontend/one`", "Crew c1 made it",
		"Change nothing in the capsule", "ws mission bundle --set -", "there's no `ws mission done` for you",
	} {
		if !strings.Contains(told, want) {
			t.Errorf("the verifier isn't told %q:\n%s", want, told)
		}
	}
	if strings.Contains(told, "open PRs") || strings.Contains(told, "This is the work verified again") {
		t.Errorf("the verifier is told what crew doing the work are:\n%s", told)
	}
	m, _ := w.Missions.Load("cache")
	if !w.Verifying(m, "c2") || w.Verifying(m, "c1") {
		t.Error("who's verifying is wrong")
	}
	// It isn't the orchestrator's to pass.
	_, err := w.AcceptStage("cache", AsOrchestrator, "looks fine")
	refusedWith(t, err, "verify")

	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "fixed"), nil, 0o644)
	// A check that's the user's can't be settled by the verifier, nor one of
	// its own left to them.
	_, err = w.HandOver("cache", "c2", strings.Replace(verifierBundle("met"), "3. yours:", "3. met:", 1), dir)
	if err == nil || !strings.Contains(err.Error(), "check 3 is the user's to settle, not yours") {
		t.Errorf("settled the user's check: %v", err)
	}
	_, err = w.HandOver("cache", "c2", strings.Replace(verifierBundle("met"), "1. met:", "1. yours:", 1), dir)
	if err == nil || !strings.Contains(err.Error(), "check 1 is yours to settle") {
		t.Errorf("left its own check to the user: %v", err)
	}

	h, err := w.HandOver("cache", "c2", verifierBundle("met"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if h.Bundle.Own || h.Bundle.ID != "b1" || h.Bundle.Step != 2 || h.Bundle.Tally() != "1 met, 1 yours" || !strings.Contains(h.Next, "passed step 2") {
		t.Errorf("handed over: %+v, next %q", h.Bundle, h.Next)
	}
	m, _ = w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	st := states[1]
	r, ok := st.Record.LatestChecks()
	if !st.Accepted() || st.Record.AcceptedBy != "ws" || !ok || len(r.Results) != 3 {
		t.Fatalf("step 2 = %+v, round %+v", st.Record, r)
	}
	if r.Results[0].Result != mission.CheckMet || r.Results[0].By != "the verifier (c2)" || r.Results[1].By != "ws" || r.Results[1].Result != mission.CheckMet || r.Results[2].Result != mission.CheckYours {
		t.Errorf("results = %+v", r.Results)
	}
	if c, _ := m.CrewByID("c2"); c.Status != mission.CrewDone || !strings.Contains(c.Note, "handed over b1") {
		t.Errorf("the verifier isn't done: %+v", c)
	}
	// The Review looks at the bundle, and what ws ran shows with it.
	task, ok := m.StepTask(3)
	if !ok || !strings.Contains(task.Text, "Look at b1 on the Verify tab") {
		t.Errorf("the review's task: %+v", task)
	}
	if v := w.Bundles(m); len(v) != 1 || !v[0].Asked || v[0].Ran == nil || len(v[0].StepChecks) != 3 || !v[0].StepChecks[2].Yours {
		t.Errorf("views = %+v", v)
	}
}

// Something unmet, and the work goes back to the crew who made it, with
// what the verifier found; when they report done it's verified again,
// afresh, and the bundle that comes of it takes the first one's place.
func TestVerify_UnmetGoesBackAndIsVerifiedAgain(t *testing.T) {
	w, host, dir := verifyFixture(t)
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	h, err := w.HandOver("cache", "c2", verifierBundle("unmet"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Next, "step 2 isn't verified (0 of 3 met") || !strings.Contains(h.Next, "c1 could not be resumed: started c3 in frontend/one to carry on at step 1") {
		t.Errorf("next = %q", h.Next)
	}
	back := host.specs[len(host.specs)-1].Prompt
	for _, want := range []string{"Step 2 verified your work in frontend/one (round 1, by c2, who didn't do it)", "- the page loads: unmet. the page came up", "- `test -f fixed` passes: unmet", "`ws show -f llm b1` has what the verifier saw", "ws has it verified again"} {
		if !strings.Contains(back, want) {
			t.Errorf("the crew aren't told %q:\n%s", want, back)
		}
	}
	if strings.Contains(back, "it reads well") {
		t.Errorf("what's the user's went back to crew:\n%s", back)
	}
	m, _ := w.Missions.Load("cache")
	if cur, _ := currentStep(w.Missions, m); cur.N != 2 {
		t.Fatalf("at step %d", cur.N)
	}

	// The crew put it right and report done: a fresh verifier starts, told
	// what went back.
	os.WriteFile(filepath.Join(dir, "fixed"), nil, 0o644)
	w.Missions.SetCrewStatus("cache", "c3", mission.CrewDone, "fixed, again")
	if said := w.CrewDone("cache", "c3"); !strings.Contains(said, "step 2 verifies it again: started step 2: c4 in frontend/one") {
		t.Fatalf("CrewDone said %q", said)
	}
	again := host.specs[len(host.specs)-1].Prompt
	for _, want := range []string{"This is the work verified again: b1 went back to its crew", "- check 1 (the page loads): the page came up", "Verify all of it afresh", "Crew c3 made it"} {
		if !strings.Contains(again, want) {
			t.Errorf("the second verifier isn't told %q:\n%s", want, again)
		}
	}
	h, err = w.HandOver("cache", "c4", verifierBundle("met"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if h.Bundle.ID != "b2" || h.Bundle.Replaces != "b1" || !strings.Contains(h.Next, "passed step 2") {
		t.Errorf("handed over again: %+v, next %q", h.Bundle, h.Next)
	}
	m, _ = w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if looks := m.Looks(states, 3); len(looks) == 0 || looks[0].Bundle != "b2" {
		t.Errorf("the review looks at %+v", looks)
	}
}

// After as many rounds as ws lets come round, an unverified step is the
// user's: to pass as it is, or send the work back.
func TestVerify_PastMaxRoundsItsYours(t *testing.T) {
	w, host, dir := verifyFixture(t)
	w.Config().Missions.MaxRounds = 1
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	launched := len(host.specs)
	h, err := w.HandOver("cache", "c2", verifierBundle("unmet"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Next, "it's the user's, as 1 round unmet") || len(host.specs) != launched {
		t.Errorf("next = %q, launched %d", h.Next, len(host.specs)-launched)
	}
	m, _ := w.Missions.Load("cache")
	task, ok := m.StepTask(2)
	if !ok || task.Verb != mission.TaskChecks || !strings.Contains(task.Text, "Step 2 verification, round 1 (b1)") {
		t.Fatalf("task = %+v", task)
	}
	// Sent back by you, it goes to the crew who made the work.
	a, err := w.AnswerTask("cache", task.N, AsYou, mission.ChecksSendBack)
	if err != nil || !strings.Contains(a.Next, "started c3 in frontend/one to carry on at step 1") {
		t.Fatalf("sent back: %q, %v", a.Next, err)
	}
	if !strings.Contains(host.specs[len(host.specs)-1].Prompt, "Step 2 verified your work") {
		t.Errorf("the crew aren't told what's unmet")
	}
	// Verified again, unmet again: yours, and passing it as it is passes it.
	w.Missions.SetCrewStatus("cache", "c3", mission.CrewDone, "fixed, again")
	w.CrewDone("cache", "c3")
	if _, err := w.HandOver("cache", "c4", verifierBundle("unmet"), dir); err != nil {
		t.Fatal(err)
	}
	m, _ = w.Missions.Load("cache")
	task, ok = m.StepTask(2)
	if !ok || !strings.Contains(task.Text, "round 2 (b2)") {
		t.Fatalf("task = %+v", task)
	}
	if a, err := w.AnswerTask("cache", task.N, AsYou, mission.ChecksPass); err != nil || !strings.Contains(a.Next, "step 3") {
		t.Fatalf("passed as it is: %q, %v", a.Next, err)
	}
}

// A verifier changes nothing: its bundle is refused if its capsule has
// moved since it started, or its files have changed.
func TestVerify_AVerifierChangesNothing(t *testing.T) {
	w, _, host := reposFixture(t, verifyPlan)
	dir := filepath.Join(w.WS().RepoDir("frontend"), "one")
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	os.WriteFile(filepath.Join(dir, "page.html"), []byte("<p>hi</p>\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "fixed"), nil, 0o644)
	git("add", "page.html", "fixed")
	git("commit", "-q", "-m", "the page")
	c, _ := w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewProcess("cache", c.ID, 1, "frontend/one")
	w.Missions.JoinStage("cache", c.ID, 1)
	w.Missions.SetCrewStatus("cache", c.ID, mission.CrewDone, "fixed")
	if _, err := w.AcceptStage("cache", AsOrchestrator, "fixed"); err != nil {
		t.Fatal(err)
	}
	if len(host.specs) != 1 {
		t.Fatalf("launched %d crew", len(host.specs))
	}
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644) // what it saw: not tracked

	os.WriteFile(filepath.Join(dir, "page.html"), []byte("<p>hello</p>\n"), 0o644)
	_, err := w.HandOver("cache", "c2", verifierBundle("met"), dir)
	refusedWith(t, err, "verifier-changed")
	if err == nil || !strings.Contains(err.Error(), "has changes to its files") {
		t.Errorf("err = %v", err)
	}
	git("commit", "-q", "-am", "a fix of the verifier's own")
	_, err = w.HandOver("cache", "c2", verifierBundle("met"), dir)
	refusedWith(t, err, "verifier-changed")
	if err == nil || !strings.Contains(err.Error(), "a verifier changes nothing, so its bundle is of the work as it was") {
		t.Errorf("err = %v", err)
	}
	git("reset", "-q", "--hard", "HEAD~1")
	if h, err := w.HandOver("cache", "c2", verifierBundle("met"), dir); err != nil || !strings.Contains(h.Next, "passed step 2") {
		t.Errorf("as it was: %q, %v", h.Next, err)
	}
}
