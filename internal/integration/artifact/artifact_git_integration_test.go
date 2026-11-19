package artifact_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/testutil"
)

// Artifacts kept in git ([artifacts] path names a repo): yours are commits
// landed on its default branch, crew's wait in their capsule (in flight)
// and land with it, and a version someone else landed first moves crew's
// to the next number, or a folder of the same name to the next name.

const glowV3 = `<!doctype html>
<title>Horizon glow</title>
<style>body { background: radial-gradient(ellipse at top, #6cf2 0, transparent 40%) }</style>
`

const glowV3b = `<!doctype html>
<title>Horizon glow</title>
<style>body { background: radial-gradient(ellipse at top, #f6c2 0, transparent 60%) }</style>
`

// gitArtifactsWorkspace is a workspace that keeps artifacts in records,
// which takes commits straight to main, beside api, which takes pull
// requests. extra is more of ws.toml.
func gitArtifactsWorkspace(t *testing.T, extra string) *testutil.Workspace {
	t.Helper()
	t.Setenv(mission.EnvMission, "")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvRole, "")
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(cli.ShellPWD, "")
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{
		Org:           "test-org",
		DefaultBranch: "main",
		Repos: []testutil.RepoOpts{
			{Name: "records", Landing: "direct"},
			{Name: "api"},
		},
	})
	appendFile(t, filepath.Join(w.Root, "ws.toml"), "\n[missions]\nrecords = \"records\"\n\n[artifacts]\npath = \"records:artifacts/{date}-{slug}/\"\n"+extra)
	return w
}

func appendFile(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

// glowFolder is the folder an artifact titled Horizon glow made today is
// at, in the repo.
func glowFolder(suffix string) string {
	return "artifacts/" + time.Now().Format("2006-01-02") + "-horizon-glow" + suffix
}

// remoteFile is a file on the remote's main.
func remoteFile(t *testing.T, w *testutil.Workspace, rel string) string {
	t.Helper()
	return testutil.GitOut(t, w.Sources["records"], "show", "main:"+rel)
}

// You add one by hand, then a version: each is one commit on records'
// main, with [artifacts]' message and where it came from in its body,
// landed, and ws show a1 says it's kept there.
func TestArtifactInGit_YoursLandsAsACommitEach(t *testing.T) {
	w := gitArtifactsWorkspace(t, "")
	dir := t.TempDir()
	src := w.Sources["records"]

	r := testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow.html", glowV1), "--note", "Accent at 8% in dark.")
	folder := glowFolder("")
	if strings.TrimSpace(r.Stdout) != "a1.v1" || !strings.Contains(r.Stderr, "as records:"+folder+"/v1.html") || !strings.Contains(r.Stderr, "landed on main") {
		t.Errorf("add said:\n%s%s", r.Stdout, r.Stderr)
	}
	if got := testutil.Subjects(t, src, "main"); got != "Add artifact: Horizon glow | initial" {
		t.Errorf("records' main: %s", got)
	}
	if body := testutil.GitOut(t, src, "log", "-1", "--format=%b", "main"); !strings.Contains(body, "Added by hand, by you.") || !strings.Contains(body, "Accent at 8% in dark.") {
		t.Errorf("the commit's body:\n%s", body)
	}
	if got := remoteFile(t, w, folder+"/v1.html"); got != glowV1 {
		t.Errorf("v1.html on main: %q", got)
	}
	if _, err := os.Stat(filepath.Join(w.Root, ".ws", "artifacts", filepath.Base(folder))); !os.IsNotExist(err) {
		t.Errorf("it's on this machine too (%v)", err)
	}

	r = testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow2.html", glowV2), "--to", "a1", "--note", "No glow in light.")
	if strings.TrimSpace(r.Stdout) != "a1.v2" {
		t.Errorf("the version: %s%s", r.Stdout, r.Stderr)
	}
	if got := testutil.Subjects(t, src, "main"); got != "Revise artifact horizon-glow: v2 | Add artifact: Horizon glow | initial" {
		t.Errorf("records' main: %s", got)
	}
	record := remoteFile(t, w, folder+"/artifact.toml")
	for _, want := range []string{"n = 2", "based_on = 1", `note = "No glow in light."`, `by = "you"`} {
		if !strings.Contains(record, want) {
			t.Errorf("artifact.toml on main doesn't say %s:\n%s", want, record)
		}
	}
	// No capsule or branch is left of it.
	if entries, _ := os.ReadDir(filepath.Join(w.Root, "repos", "records")); len(entries) != 2 {
		t.Errorf("records has %v", entries)
	}

	show := testutil.Run(t, w, nil, "show", "a1").Stdout
	for _, want := range []string{"(live, in records;", "Kept in records, the repo the workspace keeps artifacts in, at records:" + folder + "/", "v2 · based on v1"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a1 doesn't say %q:\n%s", want, show)
		}
	}
	if list := testutil.Run(t, w, nil, "artifact", "list").Stdout; !strings.Contains(list, "a1   v2   live") || !strings.Contains(list, "· in records") {
		t.Errorf("ws artifact list:\n%s", list)
	}
	if got := testutil.Run(t, w, nil, "artifact", "show", "a1.v1").Stdout; got != glowV1 {
		t.Errorf("ws artifact show a1.v1: %q", got)
	}
}

// A conversation's proposal, confirmed, lands as one commit whose body
// names the conversation, its title and the proposal.
func TestArtifactInGit_AProposalConfirmedLandsNamingItsConversation(t *testing.T) {
	w := gitArtifactsWorkspace(t, "")
	talks := conversation.ForWorkspace(w.Root)
	talks.OpenTalk(conversation.You, "Radial horizon gradient on web UI background", time.Now())
	t.Setenv(conversation.EnvConversation, "w1")
	t.Setenv(mission.EnvCrew, conversation.Agent)

	if guide := testutil.Run(t, w, nil, "artifact", "add", "--propose").Stdout; !strings.Contains(guide, "in records (records:artifacts/{date}-{slug}/), committed and landed") {
		t.Errorf("the guide doesn't say where it goes:\n%s", guide)
	}
	testutil.WithStdin(t, glowV1)
	r := testutil.Run(t, w, nil, "artifact", "add", "--propose", "--note", "Accent at 8% in dark and 3% in light.", "-")
	if !strings.Contains(r.Stderr, "at records:"+glowFolder("")+"/v1.html, committed and landed on the default branch of records") {
		t.Errorf("proposing:\n%s", r.Stderr)
	}

	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")
	if r := testutil.Run(t, w, nil, "conversation", "accept", "w1", "1"); !strings.Contains(r.Stderr, `Added a1.v1, "Horizon glow"`) {
		t.Errorf("accepting:\n%s", r.Stderr)
	}
	src := w.Sources["records"]
	if got := testutil.Subjects(t, src, "main"); got != "Add artifact: Horizon glow | initial" {
		t.Errorf("records' main: %s", got)
	}
	body := testutil.GitOut(t, src, "log", "-1", "--format=%b", "main")
	if !strings.Contains(body, `Proposed in conversation w1, "Radial horizon gradient on web UI background" (proposal 1), and added by you.`) {
		t.Errorf("the commit's body doesn't name the conversation:\n%s", body)
	}
	if record := remoteFile(t, w, glowFolder("")+"/artifact.toml"); !strings.Contains(record, `from = "w1"`) || !strings.Contains(record, "proposal = 1") {
		t.Errorf("artifact.toml on main:\n%s", record)
	}
}

// Crew's version is written in their mission's capsule of records, in
// flight until it lands; landing the capsule lands it.
func TestArtifactInGit_CrewsWaitsInTheirCapsuleAndLandsWithIt(t *testing.T) {
	w := gitArtifactsWorkspace(t, "")
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, t.TempDir(), "glow.html", glowV1), "--note", "The first.")
	testutil.Run(t, w, nil, "mission", "new", "Glow", "--brief", "## Goal\nTry a glow.\n\n## Stages\n1. Sketch it.")
	t.Setenv(mission.EnvMission, "glow")

	// No capsule of records: they're told to lift one.
	t.Setenv(mission.EnvCrew, "c1")
	r := testutil.RunCommand(t, w.Root, nil, "artifact", "add", writeHTML(t, t.TempDir(), "g.html", glowV2), "--to", "a1", "--note", "x")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "Lift one (ws lift records <name>)") {
		t.Fatalf("with no capsule: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "")
	testutil.Run(t, w, nil, "lift", "records", "glow-sketch")
	capsule := testutil.CapsuleDir(w, "records", "glow-sketch")
	t.Setenv(mission.EnvCrew, "c1")
	t.Setenv(cli.ShellPWD, capsule)

	r = testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, t.TempDir(), "g.html", glowV2), "--to", "a1", "--note", "No glow in light.")
	if strings.TrimSpace(r.Stdout) != "a1.v2" || !strings.Contains(r.Stderr, "as records/glow-sketch:"+glowFolder("")+"/v2.html") || !strings.Contains(r.Stderr, "in flight: commit it with your work") {
		t.Errorf("crew's add said:\n%s%s", r.Stdout, r.Stderr)
	}
	if _, err := os.Stat(filepath.Join(capsule, filepath.FromSlash(glowFolder("")), "v2.html")); err != nil {
		t.Errorf("it isn't in the capsule: %v", err)
	}
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "Add artifact: Horizon glow | initial" {
		t.Errorf("crew's landed something: %s", got)
	}
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(cli.ShellPWD, "")
	show := testutil.Run(t, w, nil, "show", "a1").Stdout
	for _, want := range []string{"In flight: v2", "v2 · based on v1", "in flight, in records/glow-sketch"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a1 doesn't say %q:\n%s", want, show)
		}
	}
	if list := testutil.Run(t, w, nil, "artifact", "list").Stdout; !strings.Contains(list, "in flight") || !strings.Contains(list, "· in records/glow-sketch") {
		t.Errorf("ws artifact list:\n%s", list)
	}
	if got := testutil.Run(t, w, nil, "artifact", "show", "a1.v2").Stdout; got != glowV2 {
		t.Errorf("a1.v2 in flight reads %q", got)
	}

	// Committed with their work, it lands with the capsule.
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-q", "-m", "Sketch the glow without light")
	testutil.Run(t, w, nil, "land", "records", "glow-sketch")
	if record := remoteFile(t, w, glowFolder("")+"/artifact.toml"); !strings.Contains(record, "n = 2") || !strings.Contains(record, `by = "c1"`) {
		t.Errorf("artifact.toml on main:\n%s", record)
	}
	show = testutil.Run(t, w, nil, "show", "a1").Stdout
	if strings.Contains(show, "in flight") || !strings.Contains(show, "(live, in records;") {
		t.Errorf("landed, ws show a1 says:\n%s", show)
	}
}

// Two clones each add a v3: the second to land takes v4. Crew's waits in
// their capsule as v3, is listed as v4 once the other's v3 has been
// fetched, and lands as v4, said so; yours, landing while another's
// lands, is numbered again from what's there.
func TestArtifactInGit_TwoV3sEndAsV3AndV4(t *testing.T) {
	w := gitArtifactsWorkspace(t, "")
	dir := t.TempDir()
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow.html", glowV1), "--note", "The first.")
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow2.html", glowV2), "--to", "a1", "--note", "The second.")
	testutil.Run(t, w, nil, "mission", "new", "Glow", "--brief", "## Goal\nTry a glow.\n\n## Stages\n1. Sketch it.")
	t.Setenv(mission.EnvMission, "glow")
	testutil.Run(t, w, nil, "lift", "records", "glow-sketch")
	capsule := testutil.CapsuleDir(w, "records", "glow-sketch")
	t.Setenv(mission.EnvCrew, "c1")
	t.Setenv(cli.ShellPWD, capsule)
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "crew.html", glowV3), "--to", "a1", "--note", "Crew's: a smaller glow.")
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-q", "-m", "Sketch a smaller glow")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(cli.ShellPWD, "")

	// Another clone lands its v3 first.
	other := otherClone(t, w)
	testutil.Run(t, other, nil, "artifact", "add", writeHTML(t, dir, "theirs.html", glowV3b), "--to", "a1", "--note", "Theirs: a pink glow.")
	fetchGround(t, w, "records")
	show := testutil.Run(t, w, nil, "show", "a1").Stdout
	if !strings.Contains(show, "v4 · based on v2") || !strings.Contains(show, "(as v3 there: v3 landed first, so it lands as v4)") {
		t.Errorf("before landing, ws show a1:\n%s", show)
	}
	if got := testutil.Run(t, w, nil, "artifact", "show", "a1.v4").Stdout; got != glowV3 {
		t.Errorf("a1.v4, crew's in flight, reads %q", got)
	}

	r := testutil.Run(t, w, nil, "land", "records", "glow-sketch")
	if !strings.Contains(r.Stderr, "v3 of "+glowFolder("")+"/ lands as v4: v3 was taken on the branch") {
		t.Errorf("landing didn't say so:\n%s", r.Stderr)
	}
	folder := glowFolder("")
	if got := remoteFile(t, w, folder+"/v3.html"); got != glowV3b {
		t.Errorf("v3 on main is %q", got)
	}
	if got := remoteFile(t, w, folder+"/v4.html"); got != glowV3 {
		t.Errorf("v4 on main is %q", got)
	}
	record := remoteFile(t, w, folder+"/artifact.toml")
	if strings.Count(record, "[[versions]]") != 4 || !strings.Contains(record, "n = 4\nbased_on = 2") || !strings.Contains(record, `note = "Crew's: a smaller glow."`) {
		t.Errorf("artifact.toml on main:\n%s", record)
	}
	if subj := testutil.Subjects(t, w.Sources["records"], "main"); !strings.HasPrefix(subj, "Sketch a smaller glow | Revise artifact horizon-glow: v3") {
		t.Errorf("records' main: %s", subj)
	}

	// Yours, while another's lands: as records' check runs, someone's v5
	// lands, and yours is numbered again from what's there, as v6.
	race := filepath.Join(t.TempDir(), "race.sh")
	theirs := writeHTML(t, dir, "v5.html", glowV1)
	script := `#!/bin/sh
[ -e "$0.done" ] && exit 0
touch "$0.done"
tmp=$(mktemp -d)
git clone -q "` + w.Sources["records"] + `" "$tmp/c" && cd "$tmp/c" || exit 1
cp "` + theirs + `" "` + folder + `/v5.html"
printf '\n[[versions]]\nn = 5\nbased_on = 4\nat = 2026-11-16T10:00:00Z\nby = "you"\nnote = "Theirs again."\n' >> "` + folder + `/artifact.toml"
git add -A && git -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -q -m "Revise artifact horizon-glow: v5" && git push -q origin HEAD:main
`
	if err := os.WriteFile(race, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	appendFile(t, filepath.Join(w.Root, "ws.local.toml"), "\n[repos.records]\nverify = \""+race+"\"\n")
	r = testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "mine.html", glowV2), "--to", "a1", "--note", "Mine, while theirs landed.")
	if strings.TrimSpace(r.Stdout) != "a1.v6" {
		t.Errorf("yours, as another's landed: %s%s", r.Stdout, r.Stderr)
	}
	if subj := testutil.Subjects(t, w.Sources["records"], "main"); !strings.HasPrefix(subj, "Revise artifact horizon-glow: v6 | Revise artifact horizon-glow: v5 | Sketch a smaller glow") {
		t.Errorf("records' main: %s", subj)
	}
	if record := remoteFile(t, w, folder+"/artifact.toml"); !strings.Contains(record, "n = 6\nbased_on = 5") || !strings.Contains(record, `note = "Mine, while theirs landed."`) {
		t.Errorf("artifact.toml on main:\n%s", record)
	}
}

// A folder made in a capsule by a name someone else's artifact took on
// the branch meanwhile lands under the next name.
func TestArtifactInGit_AFolderByATakenNameLandsUnderTheNext(t *testing.T) {
	w := gitArtifactsWorkspace(t, "")
	dir := t.TempDir()
	testutil.Run(t, w, nil, "mission", "new", "Glow", "--brief", "## Goal\nTry a glow.\n\n## Stages\n1. Sketch it.")
	t.Setenv(mission.EnvMission, "glow")
	testutil.Run(t, w, nil, "lift", "records", "glow-sketch")
	capsule := testutil.CapsuleDir(w, "records", "glow-sketch")
	t.Setenv(mission.EnvCrew, "c1")
	t.Setenv(cli.ShellPWD, capsule)
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "crew.html", glowV3), "--note", "Crew's glow.")
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "crew2.html", glowV2), "--to", "a1", "--note", "Crew's second.")
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-q", "-m", "Sketch a glow")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(cli.ShellPWD, "")

	other := otherClone(t, w)
	time.Sleep(1100 * time.Millisecond) // made in another second: another artifact
	testutil.Run(t, other, nil, "artifact", "add", writeHTML(t, dir, "theirs.html", glowV3b), "--note", "Their glow.")

	r := testutil.Run(t, w, nil, "land", "records", "glow-sketch")
	if !strings.Contains(r.Stderr, glowFolder("")+"/ lands as "+glowFolder("-2")+"/") {
		t.Errorf("landing didn't say so:\n%s", r.Stderr)
	}
	if got := remoteFile(t, w, glowFolder("")+"/v1.html"); got != glowV3b {
		t.Errorf("theirs is %q", got)
	}
	for name, want := range map[string]string{"v1.html": glowV3, "v2.html": glowV2} {
		if got := remoteFile(t, w, glowFolder("-2")+"/"+name); got != want {
			t.Errorf("crew's %s is %q", name, got)
		}
	}
	if record := remoteFile(t, w, glowFolder("-2")+"/artifact.toml"); strings.Count(record, "[[versions]]") != 2 || !strings.Contains(record, `note = "Crew's second."`) {
		t.Errorf("crew's artifact.toml:\n%s", record)
	}
	if record := remoteFile(t, w, glowFolder("")+"/artifact.toml"); strings.Count(record, "[[versions]]") != 1 || !strings.Contains(record, `note = "Their glow."`) {
		t.Errorf("theirs' artifact.toml:\n%s", record)
	}
}

// otherClone is another workspace of the same repos, as a teammate's ws
// is: its records' remote is w's.
func otherClone(t *testing.T, w *testutil.Workspace) *testutil.Workspace {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	toml, err := os.ReadFile(filepath.Join(w.Root, "ws.toml"))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "ws.toml"), toml, 0o644)
	for _, name := range []string{"records", "api"} {
		repo := filepath.Join(root, "repos", name)
		os.MkdirAll(repo, 0o755)
		testutil.GitCmd(t, root, "clone", "-q", "--bare", w.Sources[name], filepath.Join(repo, ".bare"))
		testutil.GitCmd(t, repo, "--git-dir=.bare", "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
		testutil.GitCmd(t, repo, "--git-dir=.bare", "fetch", "-q", "origin")
		testutil.GitCmd(t, repo, "--git-dir=.bare", "worktree", "add", "-q", filepath.Join(repo, ".ground"), "main")
	}
	return &testutil.Workspace{Root: root, Org: w.Org, Sources: w.Sources}
}

// fetchGround brings a repo's ground up to its remote, as ws does when it
// fetches.
func fetchGround(t *testing.T, w *testutil.Workspace, repo string) {
	t.Helper()
	dir := filepath.Join(w.Root, "repos", repo)
	testutil.GitCmd(t, dir, "--git-dir=.bare", "fetch", "-q", "origin")
	testutil.GitCmd(t, filepath.Join(dir, ".ground"), "merge", "-q", "--ff-only", "origin/main")
}

// ws.local.toml can keep yours on this machine whatever ws.toml says: new
// ones go in .ws, and what's in the repo is still read.
func TestArtifactInGit_LocalTomlKeepsYoursHere(t *testing.T) {
	w := gitArtifactsWorkspace(t, "")
	dir := t.TempDir()
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow.html", glowV1), "--note", "In records.")
	appendFile(t, filepath.Join(w.Root, "ws.local.toml"), "\n[artifacts]\npath = \".ws\"\n")
	r := testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "mine.html", glowV3), "--title", "Mine", "--note", "Here only.")
	if strings.TrimSpace(r.Stdout) != "a2.v1" || !strings.Contains(r.Stderr, "as .ws/artifacts/") || !strings.Contains(r.Stderr, "(on this machine only)") {
		t.Errorf("with ws.local.toml's .ws:\n%s%s", r.Stdout, r.Stderr)
	}
	// A version of the one in records still goes there.
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow2.html", glowV2), "--to", "a1", "--note", "Still records.")
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "Revise artifact horizon-glow: v2 | Add artifact: Horizon glow | initial" {
		t.Errorf("records' main: %s", got)
	}
	list := testutil.Run(t, w, nil, "artifact", "list").Stdout
	if !strings.Contains(list, "a1 ") || !strings.Contains(list, "a2 ") {
		t.Errorf("both aren't listed:\n%s", list)
	}
	dr := testutil.RunCommand(t, w.Root, nil, "doctor")
	if doctor := dr.Stdout + dr.Stderr; !strings.Contains(doctor, "yours go on this machine, as ws.local.toml says") {
		t.Errorf("ws doctor:\n%s", dr.Stderr)
	}
}

// ws doctor says where artifacts are kept, and what's wrong with it: a
// records repo with no [artifacts], a repo that takes pull requests, a
// path it can't use, a guide that isn't there.
func TestArtifactInGit_Doctor(t *testing.T) {
	doctor := func(w *testutil.Workspace) string {
		t.Helper()
		r := testutil.RunCommand(t, w.Root, nil, "doctor")
		return r.Stdout + r.Stderr
	}
	w := gitArtifactsWorkspace(t, "guide = \"records:ARTIFACTS.md\"\n")
	if got := doctor(w); !strings.Contains(got, "in records (records:artifacts/{date}-{slug}/): each one you add is a commit") || !strings.Contains(got, "records:ARTIFACTS.md isn't there") {
		t.Errorf("kept in records:\n%s", got)
	}

	w = testutil.LandWorkspace(t, "")
	appendFile(t, filepath.Join(w.Root, "ws.toml"), "\n[missions]\nrecords = \"records\"\n")
	if got := doctor(w); !strings.Contains(got, "on this machine only (.ws/artifacts): there's a records repo, records, and no [artifacts]") || !strings.Contains(got, `path = "records:artifacts/{date}-{slug}/"`) {
		t.Errorf("a records repo and no [artifacts]:\n%s", got)
	}

	w = testutil.LandWorkspace(t, "")
	appendFile(t, filepath.Join(w.Root, "ws.toml"), "\n[artifacts]\npath = \"api:artifacts/{date}-{slug}/\"\n")
	if got := doctor(w); !strings.Contains(got, "which takes pull requests: each version would wait in review") {
		t.Errorf("in a repo with pull requests:\n%s", got)
	}
	r := testutil.RunCommand(t, w.Root, nil, "artifact", "add", writeHTML(t, t.TempDir(), "g.html", glowV1), "--note", "x")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "which takes pull requests") {
		t.Errorf("adding there: %v", r.Err)
	}

	w = testutil.LandWorkspace(t, "")
	appendFile(t, filepath.Join(w.Root, "ws.toml"), "\n[artifacts]\npath = \"records:artifacts/{date}.html\"\nexpire_after = \"a month\"\n")
	got := doctor(w)
	for _, want := range []string{`ws.toml's path = "records:artifacts/{date}.html": it's a folder, so it ends in /`, `expire_after = "a month": it's a number of days, as 30d, or never`} {
		if !strings.Contains(got, want) {
			t.Errorf("a path it can't use: no %q in\n%s", want, got)
		}
	}
	// It can't be used, so they're kept here.
	if r := testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, t.TempDir(), "g.html", glowV1), "--note", "x"); !strings.Contains(r.Stderr, "(on this machine only)") {
		t.Errorf("with a path it can't use:\n%s", r.Stderr)
	}
}
