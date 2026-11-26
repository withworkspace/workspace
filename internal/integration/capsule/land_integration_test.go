package capsule_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/testutil"
)

func TestLand_PushesToMainBringsGroundUpAndBurns(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	testutil.Run(t, w, nil, "lift", "records", "note")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	testutil.CommitIn(t, dir, "notes/b.md", "# B\n", "docs: add note b")

	r := testutil.Run(t, w, nil, "land", "records", "note")

	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "docs: add note b | docs: add note a | initial" {
		t.Errorf("the remote's main has: %s", got)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "repos", "records", ".ground", "notes", "b.md")); err != nil {
		t.Errorf("the ground wasn't brought up to it: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the capsule is still there (%v)", err)
	}
	for _, want := range []string{"Landed", "on main: 2 commits", "docs: add note a", "Removed the capsule"} {
		if !strings.Contains(r.Stderr, want) {
			t.Errorf("land said nothing of %q:\n%s", want, r.Stderr)
		}
	}
}

func TestLand_FromTheCapsuleYoureIn(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	testutil.Run(t, w, nil, "lift", "records", "note")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	t.Setenv(cli.ShellPWD, filepath.Join(dir, "notes"))

	testutil.Run(t, w, nil, "land")

	if got := testutil.Subjects(t, w.Sources["records"], "main"); !strings.HasPrefix(got, "docs: add note a") {
		t.Errorf("the remote's main has: %s", got)
	}
}

func TestLand_RebasesOntoWhatLandedSince_NeverAMerge(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	testutil.Run(t, w, nil, "lift", "records", "note")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	testutil.CommitIn(t, w.Sources["records"], "notes/other.md", "# Other\n", "docs: add another note")

	testutil.Run(t, w, nil, "land", "records", "note")

	src := w.Sources["records"]
	if got := testutil.Subjects(t, src, "main"); got != "docs: add note a | docs: add another note | initial" {
		t.Errorf("the remote's main has: %s", got)
	}
	if merges := strings.TrimSpace(testutil.GitOut(t, src, "rev-list", "--merges", "main")); merges != "" {
		t.Errorf("there's a merge commit: %s", merges)
	}
}

func TestLand_Refuses(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	testutil.Run(t, w, nil, "lift", "records", "note")
	testutil.Run(t, w, nil, "lift", "api", "feature")
	dir := testutil.CapsuleDir(w, "records", "note")
	fails := func(want string, args ...string) {
		t.Helper()
		r := testutil.RunCommand(t, w.Root, nil, args...)
		if r.Err == nil || !strings.Contains(r.Err.Error(), want) {
			t.Errorf("ws %s: %v, want an error saying %q", strings.Join(args, " "), r.Err, want)
		}
	}

	fails("nothing to land", "land", "records", "note")
	testutil.CommitIn(t, testutil.CapsuleDir(w, "api", "feature"), "x.go", "package x\n", "add x")
	fails("lands through pull requests", "land", "api", "feature")

	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	os.WriteFile(filepath.Join(dir, "notes", "draft.md"), []byte("not committed\n"), 0o644)
	fails("aren't committed", "land", "records", "note")
	os.Remove(filepath.Join(dir, "notes", "draft.md"))

	// It's yours to do: not crew's, nor the orchestrator's.
	t.Setenv(mission.EnvCrew, "c1")
	fails("the user's to do", "land", "records", "note")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvRole, mission.RoleOrchestrator)
	fails("the user's to do", "land", "records", "note")
	t.Setenv(mission.EnvRole, "")

	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "initial" {
		t.Errorf("something was pushed: %s", got)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the capsule's gone: %v", err)
	}
}

func TestLand_AClashLeavesTheCapsuleAsItWas(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	testutil.Run(t, w, nil, "lift", "records", "note")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "README.md", "ours\n", "docs: say ours")
	head := strings.TrimSpace(testutil.GitOut(t, dir, "rev-parse", "HEAD"))
	testutil.CommitIn(t, w.Sources["records"], "README.md", "theirs\n", "docs: say theirs")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "note")

	if r.Err == nil || !strings.Contains(r.Err.Error(), "clashes") || !strings.Contains(r.Err.Error(), "README.md") {
		t.Fatalf("land: %v, want a clash in README.md", r.Err)
	}
	if got := strings.TrimSpace(testutil.GitOut(t, dir, "rev-parse", "HEAD")); got != head {
		t.Errorf("the capsule moved: %s, was %s", got, head)
	}
	if status := strings.TrimSpace(testutil.GitOut(t, dir, "status", "--porcelain")); status != "" {
		t.Errorf("the capsule isn't clean: %s", status)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Error("the capsule's gone")
	}
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "docs: say theirs | initial" {
		t.Errorf("something was pushed: %s", got)
	}
}

func TestLand_VerifyMustPass(t *testing.T) {
	w := testutil.LandWorkspace(t, "test ! -f notes/bad.md")
	testutil.Run(t, w, nil, "lift", "records", "bad")
	bad := testutil.CapsuleDir(w, "records", "bad")
	testutil.CommitIn(t, bad, "notes/bad.md", "# Bad\n", "docs: add a bad note")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "bad")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "doesn't pass") {
		t.Fatalf("land: %v, want the check to fail", r.Err)
	}
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "initial" {
		t.Errorf("something was pushed: %s", got)
	}

	testutil.Run(t, w, nil, "lift", "records", "good")
	testutil.CommitIn(t, testutil.CapsuleDir(w, "records", "good"), "notes/good.md", "# Good\n", "docs: add a good note")
	testutil.Run(t, w, nil, "land", "records", "good")
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "docs: add a good note | initial" {
		t.Errorf("the remote's main has: %s", got)
	}
}

func TestLand_VerifyCannotChangeTheCommitBeingLanded(t *testing.T) {
	w := testutil.LandWorkspace(t, "git -c user.name=Test -c user.email=test@test.com commit --allow-empty -m generated")
	testutil.Run(t, w, nil, "lift", "records", "note")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "note")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "changed during landing") {
		t.Fatalf("land: %v, want the changed commit to stop landing", r.Err)
	}
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "initial" {
		t.Errorf("something was pushed: %s", got)
	}
}

func TestLand_TriesAgainWhenMainMovesUnderIt(t *testing.T) {
	// The check itself moves main, the first time it's run: the push that
	// follows isn't a fast-forward.
	mark := filepath.Join(t.TempDir(), "moved")
	w := testutil.LandWorkspace(t, "")
	src := w.Sources["records"]
	move := "test -f " + mark + " || { touch " + mark + "; git -C " + src + " -c user.name=Other -c user.email=o@example.com -c commit.gpgsign=false commit -q --allow-empty -m 'docs: someone else landed'; }"
	appendTo(t, filepath.Join(w.Root, "ws.toml"), "\n[repos.records]\nlanding = \"direct\"\nverify = \""+strings.ReplaceAll(move, `"`, `\"`)+"\"\n", "[repos.records]\nlanding = \"direct\"\n")
	testutil.Run(t, w, nil, "lift", "records", "note")
	testutil.CommitIn(t, testutil.CapsuleDir(w, "records", "note"), "notes/a.md", "# A\n", "docs: add note a")

	r := testutil.Run(t, w, nil, "land", "records", "note")

	if got := testutil.Subjects(t, src, "main"); got != "docs: add note a | docs: someone else landed | initial" {
		t.Errorf("the remote's main has: %s", got)
	}
	if !strings.Contains(r.Stderr, "main moved") {
		t.Errorf("land didn't say main moved:\n%s", r.Stderr)
	}
}

func TestLand_GivesUpWhenMainKeepsMoving(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	src := w.Sources["records"]
	move := "git -C " + src + " -c user.name=Other -c user.email=o@example.com -c commit.gpgsign=false commit -q --allow-empty -m moved"
	appendTo(t, filepath.Join(w.Root, "ws.toml"), "\n[repos.records]\nlanding = \"direct\"\nverify = \""+move+"\"\n", "[repos.records]\nlanding = \"direct\"\n")
	testutil.Run(t, w, nil, "lift", "records", "note")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "note")

	if r.Err == nil || !strings.Contains(r.Err.Error(), "moved 3 times") {
		t.Fatalf("land: %v, want it to give up", r.Err)
	}
	if got := testutil.Subjects(t, src, "main"); strings.Contains(got, "add note a") {
		t.Errorf("it was pushed all the same: %s", got)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("the capsule's gone")
	}
}

func TestLand_ARefusedPushIsNeverForced(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	src := w.Sources["records"]
	// A remote that takes no pushes to main, as a protected branch doesn't.
	testutil.GitCmd(t, src, "config", "receive.denyCurrentBranch", "refuse")
	testutil.Run(t, w, nil, "lift", "records", "note")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "note")

	if r.Err == nil || !strings.Contains(r.Err.Error(), "refused the push") || !strings.Contains(r.Err.Error(), "isn't direct") {
		t.Fatalf("land: %v, want the push refused", r.Err)
	}
	if got := testutil.Subjects(t, src, "main"); got != "initial" {
		t.Errorf("something was pushed: %s", got)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("the capsule's gone")
	}
}

func TestLand_AMissionsCapsuleCountsAsLanded(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	testutil.Run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nWrite a note.")
	t.Setenv(mission.EnvMission, "notes")
	testutil.Run(t, w, nil, "lift", "records", "note")
	t.Setenv(mission.EnvMission, "")
	testutil.CommitIn(t, testutil.CapsuleDir(w, "records", "note"), "notes/a.md", "# A\n", "docs: add note a")

	// With commits and nobody in it, it's yours to land.
	if r := testutil.Run(t, w, nil, "mission", "show", "notes", "-f", "llm"); !strings.Contains(r.Stdout, "State: active") || !strings.Contains(r.Stdout, "has 1 commit to land on main") {
		t.Errorf("before landing, the mission says:\n%s", r.Stdout)
	}

	r := testutil.Run(t, w, nil, "land", "records", "note")

	if !strings.Contains(r.Stderr, "mission notes") {
		t.Errorf("land didn't name the mission:\n%s", r.Stderr)
	}
	m := testutil.LoadMission(t, w, "notes")
	if len(m.Capsules) != 1 || m.Capsules[0].Landed == nil || m.Capsules[0].Landed.Head == "" {
		t.Fatalf("the mission's capsule: %+v", m.Capsules)
	}
	if kinds := strings.Join(testutil.LogKinds(t, w, "notes"), " "); !strings.Contains(kinds, "landed capsule_burned") {
		t.Errorf("the log has: %s", kinds)
	}
	// Burned, with no pull request: it still counts, as landed.
	if r := testutil.Run(t, w, nil, "mission", "show", "notes", "-f", "llm"); !strings.Contains(r.Stdout, "State: landed") || !strings.Contains(r.Stdout, "Everything has landed") {
		t.Errorf("after landing, the mission says:\n%s", r.Stdout)
	}
}

// The push is made before the mission is told. If the mission can't be
// written then, the capsule stays: burning it would leave the mission with
// neither the capsule nor a word of what became of it.
func TestLand_AMissionThatCantRecordItKeepsTheCapsule(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	testutil.Run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nWrite a note.")
	t.Setenv(mission.EnvMission, "notes")
	testutil.Run(t, w, nil, "lift", "records", "note")
	t.Setenv(mission.EnvMission, "")
	testutil.CommitIn(t, testutil.CapsuleDir(w, "records", "note"), "notes/a.md", "# A\n", "docs: add note a")

	// The mission's record refuses to be written.
	records, _ := store.Open(w.Root)
	if err := records.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`CREATE TRIGGER notes_read_only BEFORE UPDATE ON missions
			WHEN old.slug = 'notes' AND json_extract(new.record, '$.capsules[0].landed.head') IS NOT NULL
			BEGIN SELECT RAISE(ABORT, 'read-only'); END`)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	r := testutil.Run(t, w, nil, "land", "records", "note")

	if got := testutil.Subjects(t, filepath.Join(w.Root, "repos", "records", ".ground"), "-1"); got != "docs: add note a" {
		t.Fatalf("it should have landed all the same; the ground is at %q", got)
	}
	if !strings.Contains(r.Stderr, "couldn't record the landing") || strings.Contains(r.Stderr, "Removed the capsule") {
		t.Errorf("land should say the mission wasn't told, and keep the capsule:\n%s", r.Stderr)
	}
	if _, err := os.Stat(testutil.CapsuleDir(w, "records", "note")); err != nil {
		t.Errorf("the capsule was burned: %v", err)
	}
	if m := testutil.LoadMission(t, w, "notes"); len(m.Capsules) != 1 || m.Capsules[0].PendingLanding == nil {
		t.Fatalf("the mission lost its pending landing: %+v", m.Capsules)
	}
	testutil.Run(t, w, nil, "debrief", "records", "--days", "0")
	if _, err := os.Stat(testutil.CapsuleDir(w, "records", "note")); err != nil {
		t.Fatalf("debrief burned the capsule before it could be reconciled: %v", err)
	}
	testutil.CommitIn(t, w.Sources["records"], "notes/later.md", "# Later\n", "docs: add a later note")
	if err := records.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`DROP TRIGGER notes_read_only`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	testutil.Run(t, w, nil, "land", "records", "note")
	m := testutil.LoadMission(t, w, "notes")
	if c := m.Capsules[0]; c.PendingLanding != nil || c.Landed == nil ||
		c.Landed.Head != strings.TrimSpace(testutil.GitOut(t, w.Sources["records"], "rev-parse", "main^")) {
		t.Fatalf("retry didn't record the commit that landed: %+v", c)
	}
	if got := strings.Join(testutil.LogKinds(t, w, "notes"), " "); strings.Count(got, "landed") != 1 {
		t.Errorf("expected one landing in the log, got %s", got)
	}
	if _, err := os.Stat(testutil.CapsuleDir(w, "records", "note")); !os.IsNotExist(err) {
		t.Errorf("capsule remained after recovery: %v", err)
	}
}

// appendTo adds text to a file, dropping drop from it first (a section
// about to be written again).
func appendTo(t *testing.T, path, text, drop string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), drop, "", 1)+text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A landing is something this ws did: it's kept as an event, by you, and
// so is the capsule's burning after it. ws events lists them.
func TestLand_IsKeptAsAnEvent(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	testutil.Run(t, w, nil, "lift", "records", "note")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	testutil.Run(t, w, nil, "land", "records", "note")

	r := testutil.Run(t, w, nil, "events", "-f", "json")
	var kinds []string
	for line := range strings.SplitSeq(strings.TrimSpace(r.Stdout), "\n") {
		var e struct{ Kind, By, Repo, Capsule, Text string }
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		if e.By != "you" || e.Repo != "records" || e.Capsule != "note" {
			t.Errorf("event: %+v", e)
		}
		if e.Kind == "capsule.landed" && !strings.Contains(e.Text, "1 commit on main") {
			t.Errorf("what it says: %q", e.Text)
		}
		kinds = append(kinds, e.Kind)
	}
	if got := strings.Join(kinds, " "); got != "capsule.landed capsule.burned" {
		t.Errorf("kept: %s\n%s", got, r.Stdout)
	}

	r = testutil.Run(t, w, nil, "events", "--kind", "capsule.landed")
	if !strings.Contains(r.Stdout, "capsule.landed") || strings.Contains(r.Stdout, "capsule.burned") {
		t.Errorf("one kind:\n%s", r.Stdout)
	}
}
