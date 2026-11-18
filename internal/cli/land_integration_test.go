package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/testutil"
)

// landWorkspace is a workspace with a repo that takes commits straight to
// main (records) and one that takes pull requests (api).
func landWorkspace(t *testing.T, verify string) *testutil.Workspace {
	t.Helper()
	t.Setenv(mission.EnvMission, "")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvRole, "")
	t.Setenv(cli.ShellPWD, "")
	return testutil.SetupWorkspace(t, testutil.WorkspaceOpts{
		Org:           "test-org",
		DefaultBranch: "main",
		Repos: []testutil.RepoOpts{
			{Name: "records", Landing: "direct", Verify: verify},
			{Name: "api"},
		},
	})
}

// commitIn writes a file in a repo's capsule (or its source) and commits it.
func commitIn(t *testing.T, dir, path, text, subject string) {
	t.Helper()
	full := filepath.Join(dir, path)
	os.MkdirAll(filepath.Dir(full), 0o755)
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.GitCmd(t, dir, "add", "-A")
	testutil.GitCmd(t, dir, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-m", subject)
}

func capsuleDir(w *testutil.Workspace, repo, name string) string {
	return filepath.Join(w.Root, "repos", repo, name)
}

func subjects(t *testing.T, dir, rev string) string {
	t.Helper()
	return strings.ReplaceAll(strings.TrimSpace(gitOut(t, dir, "log", "--format=%s", rev)), "\n", " | ")
}

func TestLand_PushesToMainBringsGroundUpAndBurns(t *testing.T) {
	w := landWorkspace(t, "")
	run(t, w, nil, "lift", "records", "note")
	dir := capsuleDir(w, "records", "note")
	commitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	commitIn(t, dir, "notes/b.md", "# B\n", "docs: add note b")

	r := run(t, w, nil, "land", "records", "note")

	if got := subjects(t, w.Sources["records"], "main"); got != "docs: add note b | docs: add note a | initial" {
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
	w := landWorkspace(t, "")
	run(t, w, nil, "lift", "records", "note")
	dir := capsuleDir(w, "records", "note")
	commitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	t.Setenv(cli.ShellPWD, filepath.Join(dir, "notes"))

	run(t, w, nil, "land")

	if got := subjects(t, w.Sources["records"], "main"); !strings.HasPrefix(got, "docs: add note a") {
		t.Errorf("the remote's main has: %s", got)
	}
}

func TestLand_RebasesOntoWhatLandedSince_NeverAMerge(t *testing.T) {
	w := landWorkspace(t, "")
	run(t, w, nil, "lift", "records", "note")
	dir := capsuleDir(w, "records", "note")
	commitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	commitIn(t, w.Sources["records"], "notes/other.md", "# Other\n", "docs: add another note")

	run(t, w, nil, "land", "records", "note")

	src := w.Sources["records"]
	if got := subjects(t, src, "main"); got != "docs: add note a | docs: add another note | initial" {
		t.Errorf("the remote's main has: %s", got)
	}
	if merges := strings.TrimSpace(gitOut(t, src, "rev-list", "--merges", "main")); merges != "" {
		t.Errorf("there's a merge commit: %s", merges)
	}
}

func TestLand_Refuses(t *testing.T) {
	w := landWorkspace(t, "")
	run(t, w, nil, "lift", "records", "note")
	run(t, w, nil, "lift", "api", "feature")
	dir := capsuleDir(w, "records", "note")
	fails := func(want string, args ...string) {
		t.Helper()
		r := testutil.RunCommand(t, w.Root, nil, args...)
		if r.Err == nil || !strings.Contains(r.Err.Error(), want) {
			t.Errorf("ws %s: %v, want an error saying %q", strings.Join(args, " "), r.Err, want)
		}
	}

	fails("nothing to land", "land", "records", "note")
	commitIn(t, capsuleDir(w, "api", "feature"), "x.go", "package x\n", "add x")
	fails("lands through pull requests", "land", "api", "feature")

	commitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
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

	if got := subjects(t, w.Sources["records"], "main"); got != "initial" {
		t.Errorf("something was pushed: %s", got)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the capsule's gone: %v", err)
	}
}

func TestLand_AClashLeavesTheCapsuleAsItWas(t *testing.T) {
	w := landWorkspace(t, "")
	run(t, w, nil, "lift", "records", "note")
	dir := capsuleDir(w, "records", "note")
	commitIn(t, dir, "README.md", "ours\n", "docs: say ours")
	head := strings.TrimSpace(gitOut(t, dir, "rev-parse", "HEAD"))
	commitIn(t, w.Sources["records"], "README.md", "theirs\n", "docs: say theirs")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "note")

	if r.Err == nil || !strings.Contains(r.Err.Error(), "clashes") || !strings.Contains(r.Err.Error(), "README.md") {
		t.Fatalf("land: %v, want a clash in README.md", r.Err)
	}
	if got := strings.TrimSpace(gitOut(t, dir, "rev-parse", "HEAD")); got != head {
		t.Errorf("the capsule moved: %s, was %s", got, head)
	}
	if status := strings.TrimSpace(gitOut(t, dir, "status", "--porcelain")); status != "" {
		t.Errorf("the capsule isn't clean: %s", status)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Error("the capsule's gone")
	}
	if got := subjects(t, w.Sources["records"], "main"); got != "docs: say theirs | initial" {
		t.Errorf("something was pushed: %s", got)
	}
}

func TestLand_VerifyMustPass(t *testing.T) {
	w := landWorkspace(t, "test ! -f notes/bad.md")
	run(t, w, nil, "lift", "records", "bad")
	bad := capsuleDir(w, "records", "bad")
	commitIn(t, bad, "notes/bad.md", "# Bad\n", "docs: add a bad note")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "bad")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "doesn't pass") {
		t.Fatalf("land: %v, want the check to fail", r.Err)
	}
	if got := subjects(t, w.Sources["records"], "main"); got != "initial" {
		t.Errorf("something was pushed: %s", got)
	}

	run(t, w, nil, "lift", "records", "good")
	commitIn(t, capsuleDir(w, "records", "good"), "notes/good.md", "# Good\n", "docs: add a good note")
	run(t, w, nil, "land", "records", "good")
	if got := subjects(t, w.Sources["records"], "main"); got != "docs: add a good note | initial" {
		t.Errorf("the remote's main has: %s", got)
	}
}

func TestLand_VerifyCannotChangeTheCommitBeingLanded(t *testing.T) {
	w := landWorkspace(t, "git -c user.name=Test -c user.email=test@test.com commit --allow-empty -m generated")
	run(t, w, nil, "lift", "records", "note")
	dir := capsuleDir(w, "records", "note")
	commitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "note")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "changed during landing") {
		t.Fatalf("land: %v, want the changed commit to stop landing", r.Err)
	}
	if got := subjects(t, w.Sources["records"], "main"); got != "initial" {
		t.Errorf("something was pushed: %s", got)
	}
}

func TestLand_TriesAgainWhenMainMovesUnderIt(t *testing.T) {
	// The check itself moves main, the first time it's run: the push that
	// follows isn't a fast-forward.
	mark := filepath.Join(t.TempDir(), "moved")
	w := landWorkspace(t, "")
	src := w.Sources["records"]
	move := "test -f " + mark + " || { touch " + mark + "; git -C " + src + " -c user.name=Other -c user.email=o@example.com -c commit.gpgsign=false commit -q --allow-empty -m 'docs: someone else landed'; }"
	appendTo(t, filepath.Join(w.Root, "ws.toml"), "\n[repos.records]\nlanding = \"direct\"\nverify = \""+strings.ReplaceAll(move, `"`, `\"`)+"\"\n", "[repos.records]\nlanding = \"direct\"\n")
	run(t, w, nil, "lift", "records", "note")
	commitIn(t, capsuleDir(w, "records", "note"), "notes/a.md", "# A\n", "docs: add note a")

	r := run(t, w, nil, "land", "records", "note")

	if got := subjects(t, src, "main"); got != "docs: add note a | docs: someone else landed | initial" {
		t.Errorf("the remote's main has: %s", got)
	}
	if !strings.Contains(r.Stderr, "main moved") {
		t.Errorf("land didn't say main moved:\n%s", r.Stderr)
	}
}

func TestLand_GivesUpWhenMainKeepsMoving(t *testing.T) {
	w := landWorkspace(t, "")
	src := w.Sources["records"]
	move := "git -C " + src + " -c user.name=Other -c user.email=o@example.com -c commit.gpgsign=false commit -q --allow-empty -m moved"
	appendTo(t, filepath.Join(w.Root, "ws.toml"), "\n[repos.records]\nlanding = \"direct\"\nverify = \""+move+"\"\n", "[repos.records]\nlanding = \"direct\"\n")
	run(t, w, nil, "lift", "records", "note")
	dir := capsuleDir(w, "records", "note")
	commitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "note")

	if r.Err == nil || !strings.Contains(r.Err.Error(), "moved 3 times") {
		t.Fatalf("land: %v, want it to give up", r.Err)
	}
	if got := subjects(t, src, "main"); strings.Contains(got, "add note a") {
		t.Errorf("it was pushed all the same: %s", got)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("the capsule's gone")
	}
}

func TestLand_ARefusedPushIsNeverForced(t *testing.T) {
	w := landWorkspace(t, "")
	src := w.Sources["records"]
	// A remote that takes no pushes to main, as a protected branch doesn't.
	testutil.GitCmd(t, src, "config", "receive.denyCurrentBranch", "refuse")
	run(t, w, nil, "lift", "records", "note")
	dir := capsuleDir(w, "records", "note")
	commitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "note")

	if r.Err == nil || !strings.Contains(r.Err.Error(), "refused the push") || !strings.Contains(r.Err.Error(), "isn't direct") {
		t.Fatalf("land: %v, want the push refused", r.Err)
	}
	if got := subjects(t, src, "main"); got != "initial" {
		t.Errorf("something was pushed: %s", got)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("the capsule's gone")
	}
}

func TestLand_AMissionsCapsuleCountsAsLanded(t *testing.T) {
	w := landWorkspace(t, "")
	run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nWrite a note.")
	t.Setenv(mission.EnvMission, "notes")
	run(t, w, nil, "lift", "records", "note")
	t.Setenv(mission.EnvMission, "")
	commitIn(t, capsuleDir(w, "records", "note"), "notes/a.md", "# A\n", "docs: add note a")

	// With commits and nobody in it, it's yours to land.
	if r := run(t, w, nil, "mission", "show", "notes", "-f", "llm"); !strings.Contains(r.Stdout, "State: active") || !strings.Contains(r.Stdout, "has 1 commit to land on main") {
		t.Errorf("before landing, the mission says:\n%s", r.Stdout)
	}

	r := run(t, w, nil, "land", "records", "note")

	if !strings.Contains(r.Stderr, "mission notes") {
		t.Errorf("land didn't name the mission:\n%s", r.Stderr)
	}
	m := loadMission(t, w, "notes")
	if len(m.Capsules) != 1 || m.Capsules[0].Landed == nil || m.Capsules[0].Landed.Head == "" {
		t.Fatalf("the mission's capsule: %+v", m.Capsules)
	}
	if kinds := strings.Join(logKinds(t, w, "notes"), " "); !strings.Contains(kinds, "landed capsule_burned") {
		t.Errorf("the log has: %s", kinds)
	}
	// Burned, with no pull request: it still counts, as landed.
	if r := run(t, w, nil, "mission", "show", "notes", "-f", "llm"); !strings.Contains(r.Stdout, "State: landed") || !strings.Contains(r.Stdout, "Everything has landed") {
		t.Errorf("after landing, the mission says:\n%s", r.Stdout)
	}
}

// The push is made before the mission is told. If the mission can't be
// written then, the capsule stays: burning it would leave the mission with
// neither the capsule nor a word of what became of it.
func TestLand_AMissionThatCantRecordItKeepsTheCapsule(t *testing.T) {
	w := landWorkspace(t, "")
	run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nWrite a note.")
	t.Setenv(mission.EnvMission, "notes")
	run(t, w, nil, "lift", "records", "note")
	t.Setenv(mission.EnvMission, "")
	commitIn(t, capsuleDir(w, "records", "note"), "notes/a.md", "# A\n", "docs: add note a")

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

	r := run(t, w, nil, "land", "records", "note")

	if got := subjects(t, filepath.Join(w.Root, "repos", "records", ".ground"), "-1"); got != "docs: add note a" {
		t.Fatalf("it should have landed all the same; the ground is at %q", got)
	}
	if !strings.Contains(r.Stderr, "couldn't record the landing") || strings.Contains(r.Stderr, "Removed the capsule") {
		t.Errorf("land should say the mission wasn't told, and keep the capsule:\n%s", r.Stderr)
	}
	if _, err := os.Stat(capsuleDir(w, "records", "note")); err != nil {
		t.Errorf("the capsule was burned: %v", err)
	}
	if m := loadMission(t, w, "notes"); len(m.Capsules) != 1 || m.Capsules[0].PendingLanding == nil {
		t.Fatalf("the mission lost its pending landing: %+v", m.Capsules)
	}
	run(t, w, nil, "debrief", "records", "--days", "0")
	if _, err := os.Stat(capsuleDir(w, "records", "note")); err != nil {
		t.Fatalf("debrief burned the capsule before it could be reconciled: %v", err)
	}
	commitIn(t, w.Sources["records"], "notes/later.md", "# Later\n", "docs: add a later note")
	if err := records.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`DROP TRIGGER notes_read_only`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	run(t, w, nil, "land", "records", "note")
	m := loadMission(t, w, "notes")
	if c := m.Capsules[0]; c.PendingLanding != nil || c.Landed == nil ||
		c.Landed.Head != strings.TrimSpace(gitOut(t, w.Sources["records"], "rev-parse", "main^")) {
		t.Fatalf("retry didn't record the commit that landed: %+v", c)
	}
	if got := strings.Join(logKinds(t, w, "notes"), " "); strings.Count(got, "landed") != 1 {
		t.Errorf("expected one landing in the log, got %s", got)
	}
	if _, err := os.Stat(capsuleDir(w, "records", "note")); !os.IsNotExist(err) {
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
