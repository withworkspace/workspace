package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/review"
)

// bundleOf is a bundle for workPlan's step 1: its check in words (2), with
// the files named.
func bundleOf(shows ...string) string {
	b := "## Summary\nThe lookup is cached.\n\n## Try it\n- try: `go run .`\n\n## Checks\n2. met: a second lookup doesn't reach the server.\n"
	for _, s := range shows {
		b += "   - shows: `" + s + "` what it shows\n"
	}
	return b + "\n## Not verified\nNothing.\n"
}

// Crew hand over a bundle for the step they're on: read against its checks,
// its files copied into a folder of its own, and kept on the mission with
// who made it and where.
func TestBundle_HandedOver(t *testing.T) {
	w, _, dir := workFixture(t, "")
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	files := w.Missions.FilesDir("cache")
	os.MkdirAll(filepath.Join(files, "proof"), 0o755)
	os.WriteFile(filepath.Join(files, "proof", "out.txt"), []byte("hit\nhit\n"), 0o644)

	b, err := w.HandOverBundle("cache", "c1", bundleOf("shot.png", "proof/out.txt", "mission:proof/out.txt"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != "b1" || b.Step != 1 || b.Crew != "c1" || !b.Own || b.Capsule != "frontend/one" || len(b.Checks) != 1 || b.Checks[0].Check != "the lookup is cached" {
		t.Errorf("bundle = %+v", b)
	}
	shows := b.Checks[0].Shows
	if len(shows) != 3 || shows[0].File != "shot.png" || shows[0].Media != "image/png" || shows[1].File != "out.txt" || shows[1].Media != BundleText || shows[2].File != "out.txt" {
		t.Fatalf("shows = %+v", shows)
	}
	kept := w.Missions.BundleDir("cache", "b1")
	if data, err := os.ReadFile(filepath.Join(kept, "out.txt")); err != nil || string(data) != "hit\nhit\n" {
		t.Errorf("kept out.txt = %q, %v", data, err)
	}
	// It's whole: what it was made from can go.
	os.Remove(filepath.Join(dir, "shot.png"))
	if path, media, err := w.BundleFile("cache", "b1", "shot.png"); err != nil || media != "image/png" || path != filepath.Join(kept, "shot.png") {
		t.Errorf("file = %q %q %v", path, media, err)
	}
	if _, _, err := w.BundleFile("cache", "b1", "../../brief.md"); err == nil {
		t.Error("served a file the bundle doesn't name")
	}

	m, _ := w.Missions.Load("cache")
	views := w.Bundles(m)
	if len(views) != 1 || views[0].StepN != 1 || views[0].StepName != "cache the lookup" || views[0].By != "shell" || views[0].Ran != nil {
		t.Errorf("views = %+v", views)
	}
	// Once ws has run the step's checks, the bundle shows what they came to.
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	fake.reply = "1. unmet: nothing's cached"
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	w.CrewDone("cache", "c1")
	m, _ = w.Missions.Load("cache")
	if v := w.Bundles(m)[0]; v.Ran == nil || len(v.Ran.Results) != 2 || v.Ran.Results[1].Result != mission.CheckUnmet {
		t.Errorf("ran = %+v", v.Ran)
	}

	// The next of its step takes its place.
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	two, err := w.HandOverBundle("cache", "c1", bundleOf("shot.png"), dir)
	if err != nil || two.ID != "b2" || two.Replaces != "b1" {
		t.Fatalf("two = %+v, %v", two, err)
	}
	m, _ = w.Missions.Load("cache")
	if v := w.Bundles(m); v[0].ReplacedBy != "b2" || v[1].ReplacedBy != "" {
		t.Errorf("replaced: %+v", v)
	}
}

// A bundle whose files can't be kept is refused, with each on its line, and
// nothing is kept.
func TestBundle_FilesRefused(t *testing.T) {
	w, _, dir := workFixture(t, "")
	os.WriteFile(filepath.Join(dir, "blob.bin"), []byte{0, 1, 2, 0xff}, 0o644)
	os.WriteFile(filepath.Join(dir, "empty.png"), nil, 0o644)
	_, err := w.HandOverBundle("cache", "c1", bundleOf("gone.png", "blob.bin", "empty.png"), dir)
	if err == nil || !mission.IsMalformedBundle(err) {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"line 9: shows: gone.png: there's no such file", "line 10: shows: blob.bin: it isn't an image", "line 11: shows: empty.png: it's empty"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("want %q in:\n%s", want, err)
		}
	}
	if m, _ := w.Missions.Load("cache"); len(m.Bundles) != 0 {
		t.Errorf("kept: %+v", m.Bundles)
	}
	if entries, _ := os.ReadDir(w.Missions.BundlesDir("cache")); len(entries) != 0 {
		t.Errorf("left a folder: %v", entries)
	}
}

// A bundle is of a commit: one handed over while its capsule has changes
// that aren't committed is refused, and once they are, it says which.
func TestBundle_OfACommit(t *testing.T) {
	w, _, dir := workFixture(t, "")
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	git("add", "cache.go")
	git("commit", "-q", "-m", "cache")
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644) // not tracked: it's only proof
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n\nvar hits int\n"), 0o644)

	_, err := w.HandOverBundle("cache", "c1", bundleOf("shot.png"), dir)
	refusedWith(t, err, "bundle-uncommitted")

	git("commit", "-q", "-am", "count hits")
	b, err := w.HandOverBundle("cache", "c1", bundleOf("shot.png"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if head := git("rev-parse", "HEAD"); b.Commit != head {
		t.Errorf("commit = %q, want %q", b.Commit, head)
	}
}

// Only crew working on the mission hand one over.
func TestBundle_WhoMay(t *testing.T) {
	w, _, dir := workFixture(t, "")
	if _, err := w.HandOverBundle("cache", "c9", bundleOf(), dir); err == nil {
		t.Error("crew the mission hasn't handed one over")
	}
	w.Missions.StartCrewAs("cache", "shell", "", mission.RoleAnswer)
	_, err := w.HandOverBundle("cache", "c2", bundleOf(), dir)
	refusedWith(t, err, "bundle-is-works")
}

// What ws ran is shown on a bundle only when it ran at the bundle's commit:
// a round of another commit isn't of what the bundle shows.
func TestBundle_RanAtItsCommit(t *testing.T) {
	w, _, _ := workFixture(t, "")
	m, _ := w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	stage := states[0].Text
	w.Missions.Update("cache", func(m *mission.Mission) error {
		m.Bundles = []mission.Bundle{{ID: "b1", Stage: stage, Capsule: "frontend/one", Commit: "aaa"}, {ID: "b2", Stage: stage, Capsule: "frontend/one", Commit: "bbb", Replaces: "b1"}}
		return nil
	})
	if _, err := w.Missions.RecordChecks("cache", 1, mission.CheckRound{Crew: "c1", Capsule: "frontend/one", Commit: "aaa", Results: []mission.CheckResult{{Check: "x", Result: mission.CheckMet}}}); err != nil {
		t.Fatal(err)
	}
	m, _ = w.Missions.Load("cache")
	views := w.Bundles(m)
	if views[0].Ran == nil || views[1].Ran != nil {
		t.Errorf("ran: b1 %v, b2 %v", views[0].Ran, views[1].Ran)
	}
}

// reviewedBundle is the cache mission on a plan of a Work step then a
// Review, c1's bundle of the work handed over, and the mission at the
// Review: with the work in dir.
func reviewedBundle(t *testing.T) (*Workspace, string, *fakeHost) {
	t.Helper()
	plan := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: fix it.\n   - in: `frontend/one`\n2. Review: the fix.\n3. Plan\n"
	w, _, host := reposFixture(t, plan)
	c, _ := w.Missions.StartCrew("cache", "shell", "")
	w.Missions.SetCrewProcess("cache", c.ID, 1, "frontend/one")
	w.Missions.JoinStage("cache", c.ID, 1)
	dir := filepath.Join(w.WS().RepoDir("frontend"), "one")
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	if _, err := w.HandOverBundle("cache", "c1", ownBundle, dir); err != nil {
		t.Fatal(err)
	}
	w.Missions.SetCrewStatus("cache", c.ID, mission.CrewDone, "fixed")
	if _, err := w.AcceptStage("cache", AsOrchestrator, "fixed"); err != nil {
		t.Fatal(err)
	}
	return w, dir, host
}

// ownBundle is a bundle for a step with no checks: its own, in order.
const ownBundle = "## Summary\nFixed.\n\n## Checks\n1. met: the page loads\n   - shows: `shot.png` The page\n2. can't tell: whether it holds under load\n\n## Not verified\nNothing.\n"

// A Review step looks at the bundles of the steps before it: the bundle is
// what's to approve, on the Verify tab, with the work beside it, and
// approving it passes the step, as going on does.
func TestBundle_ApprovingPassesTheReview(t *testing.T) {
	w, _, _ := reviewedBundle(t)
	m, _ := w.Missions.Load("cache")
	task, ok := m.StepTask(2)
	if !ok || !strings.Contains(task.Text, "Look at b1 on the Verify tab: approving it passes the step") || !strings.Contains(task.Text, "Work is in frontend/one, to review alongside.") {
		t.Fatalf("the review task: %+v", task)
	}
	states, _ := m.StageProgress("")
	looks := m.Looks(states, 2)
	if len(looks) != 2 || looks[0].Bundle != "b1" || looks[1].Capsule != "frontend/one" {
		t.Fatalf("looks = %+v", looks)
	}
	if v := w.Bundles(m); !v[0].Asked || v[0].AskedAt != 2 {
		t.Errorf("not asked of you: %+v", v[0])
	}

	// It's yours: nobody else approves it, or marks it.
	_, err := w.ApproveBundle("cache", "b1", AsOrchestrator)
	refusedWith(t, err, "bundle-is-yours")
	_, err = w.ApproveBundle("cache", "b1", AsCrew("c1"))
	refusedWith(t, err, "bundle-is-yours")
	refusedWith(t, w.MarkBundleLooked("cache", "b1", "c1", true, AsCrew("c1")), "bundle-is-yours")

	if err := w.MarkBundleLooked("cache", "b1", "c2", true, AsYou); err != nil {
		t.Fatal(err)
	}
	w.MarkBundleLooked("cache", "b1", "c1", true, AsYou)
	w.MarkBundleLooked("cache", "b1", "c2", true, AsYou)
	w.MarkBundleLooked("cache", "b1", "c1", false, AsYou)
	if m, _ = w.Missions.Load("cache"); strings.Join(m.Bundles[0].Looked, ",") != "c2" {
		t.Errorf("looked = %v", m.Bundles[0].Looked)
	}

	done, err := w.ApproveBundle("cache", "b1", AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(done.String(), "Approved b1: step 2 passed; ") || done.Bundle.Verdict != mission.BundleApproved || done.Bundle.VerdictAt == nil {
		t.Errorf("approved: %q %+v", done, done.Bundle)
	}
	if m, _ = w.Missions.Load("cache"); stepIs(m, 2) {
		t.Errorf("still at the review: %+v", m.Tasks)
	}
	_, err = w.ApproveBundle("cache", "b1", AsYou)
	refusedWith(t, err, "bundle-approved")
	events, _ := w.Missions.ReadLog("cache", 0)
	var said []string
	for _, e := range events {
		if e.Kind == mission.EventBundle {
			s, _ := mission.Sentence(e)
			said = append(said, s)
		}
	}
	if strings.Join(said, " | ") != "c1 handed over bundle b1 for step 1: 1 met, 1 can't tell | you: bundle b1 approved" {
		t.Errorf("log: %v", said)
	}
}

// Sending a bundle back records what you said of each check, and goes to
// the crew who made the work, with what the bundle showed; the Review
// stays yours, and the bundle that comes back takes its place.
func TestBundle_SentBack(t *testing.T) {
	w, dir, host := reviewedBundle(t)
	_, err := w.SendBackBundle("cache", "b1", nil, AsYou)
	if err == nil || !strings.Contains(err.Error(), "say what is wrong") {
		t.Errorf("sent back with nothing said: %v", err)
	}
	_, err = w.SendBackBundle("cache", "b1", []mission.BundleNote{{Check: 7, Text: "no"}}, AsYou)
	if err == nil || !strings.Contains(err.Error(), "b1 has no check 7 (checks: 1, 2)") {
		t.Errorf("a check it hasn't: %v", err)
	}
	_, err = w.SendBackBundle("cache", "b1", []mission.BundleNote{{Check: 1, Text: "no"}}, AsOrchestrator)
	refusedWith(t, err, "bundle-is-yours")

	done, err := w.SendBackBundle("cache", "b1", []mission.BundleNote{{Check: 1, Text: "the page loads, but the header's cut off"}, {Check: 2, Text: " "}, {Text: "try it at phone width too"}}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if done.Bundle.Verdict != mission.BundleSentBack || len(done.Bundle.Notes) != 2 || !strings.HasPrefix(done.String(), "Sent b1 back: ") {
		t.Errorf("sent back: %q %+v", done, done.Bundle)
	}
	m, _ := w.Missions.Load("cache")
	if !stepIs(m, 2) {
		t.Errorf("the review isn't yours any more: %+v", m.Tasks)
	}
	// c1 can't be reached or resumed here, so fresh crew start in its
	// capsule with what you said.
	if len(host.specs) == 0 {
		t.Fatalf("nobody was told: %q", done)
	}
	told := host.specs[len(host.specs)-1]
	for _, want := range []string{"Check 1 (the page loads): the page loads, but the header's cut off", "The bundle showed: The page.", "Of it as a whole: try it at phone width too", "it takes b1's place"} {
		if !strings.Contains(told.Prompt, want) {
			t.Errorf("crew aren't told %q:\n%s", want, told.Prompt)
		}
	}
	if told.Capsule != "frontend/one" {
		t.Errorf("told in %s", told.Capsule)
	}
	if v := w.Bundles(m); !v[0].Asked {
		t.Errorf("no longer looked at by the review: %+v", v[0])
	}

	// The bundle that comes back takes its place, and is the one to approve.
	if _, err := w.HandOverBundle("cache", "c2", ownBundle, dir); err != nil {
		t.Fatal(err)
	}
	_, err = w.ApproveBundle("cache", "b1", AsYou)
	refusedWith(t, err, "bundle-replaced")
	m, _ = w.Missions.Load("cache")
	states, _ := m.StageProgress("")
	if looks := m.Looks(states, 2); looks[0].Bundle != "b2" || len(looks) != 2 {
		t.Errorf("looks = %+v", looks)
	}
	if done, err := w.ApproveBundle("cache", "b2", AsYou); err != nil || !strings.Contains(done.Next, "step 2 passed") {
		t.Errorf("approving the one that came back: %q, %v", done, err)
	}
}

// Your review of the capsule alone doesn't pass a Review step that looks
// at a bundle too: the bundle is what's to approve.
func TestBundle_TheCapsulesReviewWaitsForIt(t *testing.T) {
	w, _, _ := reviewedBundle(t)
	if _, err := w.SubmitReview(OfCapsule("cache", "frontend", "one"), review.Diffed{}, review.Approve, "", AsYou); err != nil {
		t.Fatal(err)
	}
	if m, _ := w.Missions.Load("cache"); !stepIs(m, 2) {
		t.Fatalf("passed on the capsule's review alone: %+v", m.Tasks)
	}
	if done, err := w.ApproveBundle("cache", "b1", AsYou); err != nil || !strings.Contains(done.Next, "step 2 passed") {
		t.Errorf("approved: %q, %v", done, err)
	}
}

// checkedBundle is the cache mission at workPlan's step 1, c1's bundle of
// it handed over and their done checked: the judge can't tell of the check
// in words, so the step's checks are yours to say.
func checkedBundle(t *testing.T) (*Workspace, mission.Task) {
	t.Helper()
	w, _, dir := workFixture(t, "1. can't tell: it needs production's traffic")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	if _, err := w.HandOverBundle("cache", "c1", bundleOf("shot.png"), dir); err != nil {
		t.Fatal(err)
	}
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	w.CrewDone("cache", "c1")
	m, _ := w.Missions.Load("cache")
	task := m.Asking()
	if task == nil || task.Verb != mission.TaskChecks || task.Step != 1 {
		t.Fatalf("the task = %+v", task)
	}
	return w, *task
}

// A step's checks that are yours to say are shown by the bundle its crew
// handed over: what ws asks you names it, its headline looks at it (the
// page's way to it), and it waits for you on the Verify tab, as one a
// Review step looks at does.
func TestBundle_ShowsChecksThatAreYours(t *testing.T) {
	w, task := checkedBundle(t)
	if !strings.Contains(task.Text, "Review b1 on the Verify tab: approving it passes the step, saying what's wrong sends back to c1. Or pass as is, send back to c1 with what's unmet, or say what to do.") {
		t.Errorf("the task doesn't name the bundle: %q", task.Text)
	}
	m, _ := w.Missions.Load("cache")
	var flag *mission.Headline
	for _, h := range mission.Resolve(mission.Snapshot{M: m, State: mission.Active, OrchestratorOff: true}) {
		if h.Kind == mission.KindFlag {
			flag = &h
		}
	}
	if flag == nil || flag.Action.Verb != mission.VerbAnswer || flag.Action.Target != "1" || strings.Join(flag.Action.Options, "|") != "Pass it|Send it back" {
		t.Fatalf("the flag = %+v", flag)
	}
	if looks := flag.Action.Look; len(looks) != 1 || looks[0].Bundle != "b1" || looks[0].Name != "b1" {
		t.Errorf("the flag looks at %+v, want b1", looks)
	}
	if v := w.Bundles(m); !v[0].Asked || v[0].AskedAt != 1 {
		t.Errorf("b1 doesn't wait for you at step 1: asked %v at %d", v[0].Asked, v[0].AskedAt)
	}

	// A question that isn't a step's checks looks at nothing.
	if _, err := w.Missions.Ask("cache", "Which cache?", "c1"); err != nil {
		t.Fatal(err)
	}
	m, _ = w.Missions.Load("cache")
	for _, h := range mission.Resolve(mission.Snapshot{M: m, State: mission.Active, OrchestratorOff: true}) {
		if h.Kind == mission.KindFlag && h.Action.Target != "1" && len(h.Action.Look) != 0 {
			t.Errorf("a question looks at %+v", h.Action.Look)
		}
	}
}

// Approving the bundle that shows a step's checks passes the step, as
// passing it as it is does: what ws asked is answered.
func TestBundle_ApprovingPassesChecksThatAreYours(t *testing.T) {
	w, _ := checkedBundle(t)
	_, err := w.ApproveBundle("cache", "b1", AsOrchestrator)
	refusedWith(t, err, "bundle-is-yours")

	done, err := w.ApproveBundle("cache", "b1", AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(done.String(), "Approved b1: step 1 passed; ") {
		t.Errorf("approved: %q", done)
	}
	st := stepRecord(t, w, 1)
	if !st.Accepted() || st.Record.AcceptedBy != "you" || !strings.Contains(st.Record.Why, "you approved b1") {
		t.Errorf("step 1 = %+v", st.Record)
	}
	m, _ := w.Missions.Load("cache")
	if _, open := m.StepTask(1); open {
		t.Errorf("step 1's task is still open: %+v", m.Tasks)
	}
	if m.Answer == nil || m.Answer.Verb != mission.TaskChecks || !strings.Contains(m.Answer.Text, "b1") {
		t.Errorf("the answer kept for the orchestrator = %+v", m.Answer)
	}
	if v := w.Bundles(m); v[0].Asked {
		t.Errorf("b1 still waits for you: %+v", v[0])
	}
}

// Sending that bundle back sends the step back, as sending it back from
// what ws asked does, with what you said: its crew put it right and
// report done, and the checks run again.
func TestBundle_SendingBackChecksThatAreYours(t *testing.T) {
	w, _ := checkedBundle(t)
	done, err := w.SendBackBundle("cache", "b1", []mission.BundleNote{{Check: 2, Text: "the shot is of the wrong page"}}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(done.String(), "Sent b1 back: ") || !strings.Contains(done.Next, "queued") {
		t.Errorf("sent back: %q", done)
	}
	m, _ := w.Missions.Load("cache")
	if len(m.Tasks) != 0 {
		t.Errorf("step 1's checks are still yours to say: %+v", m.Tasks)
	}
	if st := stepRecord(t, w, 1); st.Accepted() {
		t.Errorf("step 1 passed: %+v", st.Record)
	}
	if m.Dispatch == nil || m.Dispatch.Stage != 1 {
		t.Fatalf("nobody's to carry on at step 1: %+v", m.Dispatch)
	}
	for _, want := range []string{"Check 2 (the lookup is cached): the shot is of the wrong page", "it takes b1's place", "report done again with a handoff (ws mission done): ws runs step 1's checks again"} {
		if !strings.Contains(m.Dispatch.Text, want) {
			t.Errorf("crew aren't told %q:\n%s", want, m.Dispatch.Text)
		}
	}
	if v := w.Bundles(m); v[0].Asked {
		t.Errorf("b1 still waits for you: %+v", v[0])
	}
}

// approvedEarly is the cache mission at workPlan's step 1, c1's bundle of
// it handed over and approved by you before their done is checked: the
// judge hasn't said yet. With moved, c1 commit more to the capsule after
// you approved, before they report done.
func approvedEarly(t *testing.T, moved bool) *Workspace {
	t.Helper()
	w, _, dir := workFixture(t, "1. can't tell: it needs production's traffic")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n"), 0o644)
	git("add", "cache.go")
	git("commit", "-q", "-m", "cache")
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	if _, err := w.HandOverBundle("cache", "c1", bundleOf("shot.png"), dir); err != nil {
		t.Fatal(err)
	}
	done, err := w.ApproveBundle("cache", "b1", AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if done.Next != "" {
		t.Fatalf("approving b1 before its checks are run did something: %q", done.Next)
	}
	if moved {
		os.WriteFile(filepath.Join(dir, "cache.go"), []byte("package cache\n\nvar hits int\n"), 0o644)
		git("commit", "-q", "-am", "count the hits")
	}
	w.Missions.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	w.CrewDone("cache", "c1")
	return w
}

// A bundle you approved before ws came to ask of its step's checks is your
// answer when it does: the step passes, and nothing's asked. Approving is
// only possible once, so asking would leave the step with nothing to
// approve.
func TestBundle_ApprovedBeforeItsChecksAreYours(t *testing.T) {
	w := approvedEarly(t, false)
	st := stepRecord(t, w, 1)
	if !st.Accepted() || st.Record.AcceptedBy != "you" || !strings.Contains(st.Record.Why, "you approved b1 (round 1: ") {
		t.Errorf("step 1 = %+v", st.Record)
	}
	m, _ := w.Missions.Load("cache")
	if _, open := m.StepTask(1); open {
		t.Errorf("step 1's checks were asked of: %+v", m.Tasks)
	}

	// The Review step after looks at b1 too: it isn't to approve again.
	if _, err := w.AcceptStage("cache", AsYou, "tested"); err != nil {
		t.Fatal(err)
	}
	m, _ = w.Missions.Load("cache")
	task, open := m.StepTask(3)
	if !open || !strings.Contains(task.Text, "You approved b1 on the Verify tab: go on to pass the step") || strings.Contains(task.Text, "passes the step") {
		t.Errorf("step 3's task = %+v", task)
	}
	for _, h := range mission.Resolve(mission.Snapshot{M: m, State: mission.Active, OrchestratorOff: true}) {
		if h.Kind == mission.KindStepReview && (strings.HasPrefix(h.Action.Label, "verify") || len(h.Action.Look) == 0 || !h.Action.Look[0].Approved) {
			t.Errorf("step 3's review = %+v", h.Action)
		}
	}
}

// One approved before its capsule moved on shows work that's changed: the
// step's checks are still yours to say. What ws asks doesn't send you to
// approve it again, and the headline says passing it is what's left.
func TestBundle_ApprovedBeforeItsCapsuleMovedOn(t *testing.T) {
	w := approvedEarly(t, true)
	if st := stepRecord(t, w, 1); st.Accepted() {
		t.Fatalf("step 1 passed on a bundle of work that's changed: %+v", st.Record)
	}
	m, _ := w.Missions.Load("cache")
	task := m.Asking()
	if task == nil || task.Verb != mission.TaskChecks || task.Step != 1 {
		t.Fatalf("the task = %+v", task)
	}
	if strings.Contains(task.Text, "approving it passes the step") {
		t.Errorf("the task sends you to approve b1 again: %q", task.Text)
	}
	var flag *mission.Headline
	for _, h := range mission.Resolve(mission.Snapshot{M: m, State: mission.Active, OrchestratorOff: true}) {
		if h.Kind == mission.KindFlag {
			flag = &h
		}
	}
	if flag == nil || flag.Action.Label != "pass it" || !strings.HasSuffix(flag.Text, "Already approved b1: pass it to go on.") {
		t.Fatalf("the flag = %+v", flag)
	}
	if looks := flag.Action.Look; len(looks) != 1 || looks[0].Bundle != "b1" || !looks[0].Approved {
		t.Errorf("the flag looks at %+v, want b1, approved", looks)
	}
}
