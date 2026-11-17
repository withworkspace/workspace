package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/review"
)

// A Review step's task is a review of what the steps before made: asking
// for changes leaves it yours (the orchestrator sends it back to the crew
// who made it), and your review approving all of it passes the step. A
// document the plan names is part of it, to approve as it reads.
func TestReviewStep_PassedByTheReview(t *testing.T) {
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: fix it.\n   - in: `frontend/one`\n   - makes: the fix, and why in `docs/why.md`\n2. Review: the fix.\n3. Plan\n"
	w, runs, _ := reposFixture(t, plan)
	c, _ := w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewProcess("cache", c.ID, 1, "frontend/one")
	w.Missions.JoinStage("cache", c.ID, 1)
	w.Missions.SetCrewStatus("cache", c.ID, mission.CrewDone, "fixed")
	w.Missions.AddArtifact("cache", mission.Artifact{Ref: "frontend/one:docs/why.md", Crew: c.ID})
	if _, err := w.AcceptStage("cache", AsOrchestrator, "fixed"); err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	task, ok := m.StepTask(2)
	if !ok || task.Verb != mission.TaskReview || !strings.Contains(task.Text, "Step 2 is yours, to review: the fix. Review frontend/one, and read docs/why.md: approving it all passes the step") {
		t.Fatalf("the review task: %+v", task)
	}

	// Changes asked: it's the orchestrator's to send back, and yours after.
	of := OfCapsule("cache", "frontend", "one")
	*runs = nil
	if _, err := w.SubmitReview(of, review.Diffed{}, review.Changes, "not yet", AsYou); err != nil {
		t.Fatal(err)
	}
	if m, _ = w.Missions.Load("cache"); !stepIs(m, 2) || strings.Join(*runs, ",") != "cache/review" {
		t.Fatalf("changes asked: runs %v, tasks %+v", *runs, m.Tasks)
	}
	// The capsule approved, the document isn't yet: still yours.
	if _, err := w.SubmitReview(of, review.Diffed{}, review.Approve, "", AsYou); err != nil {
		t.Fatal(err)
	}
	if m, _ = w.Missions.Load("cache"); !stepIs(m, 2) {
		t.Fatalf("with the document to read: %+v", m.Tasks)
	}
	if said := w.PassReviewStep("cache"); said != "" {
		t.Errorf("passed without the document: %q", said)
	}
	// The document approved as it reads: that's all of it.
	text := []byte("# Why\n\nThe key ran together.\n")
	path := filepath.Join(w.WS().RepoDir("frontend"), "one", "docs", "why.md")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, text, 0o644)
	said, err := w.Approve(doc.OfMission("cache"), "frontend/one:docs/why.md", text, doc.Version(text), AsYou)
	if err != nil || !strings.HasPrefix(said, "Approved docs/why.md: step 2 passed: your review approved frontend/one, docs/why.md") {
		t.Fatalf("approved: %q, %v", said, err)
	}
	if m, _ = w.Missions.Load("cache"); stepIs(m, 2) {
		t.Errorf("still at the review: %+v", m.Tasks)
	}
}

// stepIs reports whether the mission's at step n, with its task open.
func stepIs(m *mission.Mission, n int) bool {
	states, _ := m.StageProgress("")
	cur, ok := mission.CurrentStage(states)
	_, open := m.StepTask(n)
	return ok && cur.N == n && open
}

// Crew's notes the plan doesn't name are to read beside the review: the
// capsule's review approving it passes the step without them.
func TestReviewStep_NotesAreRead(t *testing.T) {
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: fix it.\n   - in: `frontend/one`\n   - makes: the fix\n2. Review: the fix.\n3. Plan\n"
	w, _, _ := reposFixture(t, plan)
	c, _ := w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewProcess("cache", c.ID, 1, "frontend/one")
	w.Missions.JoinStage("cache", c.ID, 1)
	w.Missions.SetCrewStatus("cache", c.ID, mission.CrewDone, "fixed")
	w.Missions.AddArtifact("cache", mission.Artifact{Ref: "mission:seen.md", Kind: "note", Crew: c.ID})
	w.Missions.AddArtifact("cache", mission.Artifact{Ref: "mission:handoffs/c1.md", Kind: mission.HandoffKind, Crew: c.ID})
	if _, err := w.AcceptStage("cache", AsOrchestrator, "fixed"); err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	task, ok := m.StepTask(2)
	want := "Review frontend/one: approving it passes the step, and asking for changes sends it back to the crew who made it. Or go on. The crew's notes on it, to read: seen.md, handoffs/c1.md."
	if !ok || !strings.Contains(task.Text, want) {
		t.Fatalf("the review task: %q", task.Text)
	}
	if _, err := w.SubmitReview(OfCapsule("cache", "frontend", "one"), review.Diffed{}, review.Approve, "", AsYou); err != nil {
		t.Fatal(err)
	}
	m, _ = w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if !states[1].Accepted() || states[1].Record.Why != "your review approved frontend/one" || !states[2].Current {
		t.Errorf("after the review: %+v", states[1].Record)
	}
}

// Only notes to read: nothing's to approve, so going on passes it.
func TestReviewStep_OnlyNotesWaitsToGoOn(t *testing.T) {
	plan := "## Z\n- Found.\n## Stops early\n- None.\n## Steps\n1. Work: find out why.\n   - makes: the findings\n2. Review: what was found.\n3. Plan\n"
	w, _, _ := reposFixture(t, plan)
	c, _ := w.Missions.StartCrew("cache", "shell", "")
	w.Missions.JoinStage("cache", c.ID, 1)
	w.Missions.SetCrewStatus("cache", c.ID, mission.CrewDone, "found")
	w.Missions.AddArtifact("cache", mission.Artifact{Ref: "mission:why.md", Kind: "note", Crew: c.ID})
	if _, err := w.AcceptStage("cache", AsOrchestrator, "found"); err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	if task, ok := m.StepTask(2); !ok || !strings.Contains(task.Text, "Read what the crew found (why.md), and go on when you're satisfied") {
		t.Fatalf("the review task: %+v", task)
	}
	if said := w.PassReviewStep("cache"); said != "" {
		t.Errorf("passed with nothing approved: %q", said)
	}
}

// Without a document, the capsule's review approving it passes the step.
func TestReviewStep_ApprovedPasses(t *testing.T) {
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: fix it.\n   - in: `frontend/one`\n2. Review: the fix.\n3. Plan\n"
	w, _, _ := reposFixture(t, plan)
	c, _ := w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewProcess("cache", c.ID, 1, "frontend/one")
	w.Missions.JoinStage("cache", c.ID, 1)
	w.Missions.SetCrewStatus("cache", c.ID, mission.CrewDone, "fixed")
	if _, err := w.AcceptStage("cache", AsOrchestrator, "fixed"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.SubmitReview(OfCapsule("cache", "frontend", "one"), review.Diffed{}, review.Approve, "", AsYou); err != nil {
		t.Fatal(err)
	}
	m, _ := w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if !states[1].Accepted() || states[1].Record.Why != "your review approved frontend/one" || !states[2].Current {
		t.Errorf("after the review: %+v", states[1].Record)
	}
	if _, open := m.StepTask(2); open {
		t.Errorf("its task is still open: %+v", m.Tasks)
	}
}
