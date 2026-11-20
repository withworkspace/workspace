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
	"github.com/withworkspace/workspace/internal/testutil"
)

// An artifact's home in a repo ([artifacts] path names one): every
// artifact is on this machine, and one you promote is put there too, the
// version chosen in a folder, in one commit landed on its default branch.
// Where it lives is chosen the first time, and kept: promoted again,
// versions later, the one chosen then goes to the same folder.

const glowV3 = `<!doctype html>
<title>Horizon glow</title>
<style>body { background: radial-gradient(ellipse at top, #6cf2 0, transparent 40%) }</style>
`

const glowV3b = `<!doctype html>
<title>Horizon glow</title>
<style>body { background: radial-gradient(ellipse at top, #f6c2 0, transparent 60%) }</style>
`

// homeWorkspace is a workspace whose promoted artifacts live in records,
// which takes commits straight to main, beside api, which takes pull
// requests. extra is more of ws.toml.
func homeWorkspace(t *testing.T, extra string) *testutil.Workspace {
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

// glowName is the name of the folder of an artifact titled Horizon glow,
// made today; glowFolder, where it lives in the repo.
func glowName() string   { return time.Now().Format("2006-01-02") + "-horizon-glow" }
func glowFolder() string { return "artifacts/" + glowName() }

// remoteFile is a file on the remote's main.
func remoteFile(t *testing.T, w *testutil.Workspace, rel string) string {
	t.Helper()
	return testutil.GitOut(t, w.Sources["records"], "show", "main:"+rel)
}

// fetchGround brings a repo's ground up to its remote, as ws does when it
// fetches.
func fetchGround(t *testing.T, w *testutil.Workspace, repo string) {
	t.Helper()
	dir := filepath.Join(w.Root, "repos", repo)
	testutil.GitCmd(t, dir, "--git-dir=.bare", "fetch", "-q", "origin")
	testutil.GitCmd(t, filepath.Join(dir, ".ground"), "merge", "-q", "--ff-only", "origin/main")
}

// refused runs a command that's to be refused, saying want.
func refused(t *testing.T, w *testutil.Workspace, want string, args ...string) {
	t.Helper()
	if r := testutil.RunCommand(t, w.Root, nil, args...); r.Err == nil || !strings.Contains(r.Err.Error(), want) {
		t.Errorf("ws %s: %v, not %q", strings.Join(args, " "), r.Err, want)
	}
}

// The flow it's for: an artifact, three versions, promoted (where it
// lives is chosen: where [artifacts] says), two more versions, promoted
// again (it knows). Its versions are on this machine all along; records
// gets the version chosen, a commit each time, in the same folder.
func TestArtifactHome_PromotedLivesInRecordsAndAgainIsOneCommit(t *testing.T) {
	w := homeWorkspace(t, "")
	dir := t.TempDir()
	src := w.Sources["records"]
	local := filepath.Join(w.Root, ".ws", "artifacts", glowName())

	r := testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow.html", glowV1), "--note", "Accent at 8% in dark.")
	if strings.TrimSpace(r.Stdout) != "a1.v1" || !strings.Contains(r.Stderr, "in .ws/artifacts/"+glowName()+"/ (on this machine: promote it to keep it)") {
		t.Errorf("add said:\n%s%s", r.Stdout, r.Stderr)
	}
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow2.html", glowV2), "--to", "a1", "--note", "No glow in light.")
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow3.html", glowV3), "--to", "a1", "--note", "Tighter, at 40%.")
	if got := testutil.Subjects(t, src, "main"); got != "initial" {
		t.Errorf("records' main, with nothing promoted: %s", got)
	}

	// Promoted: v2's chosen, and it's put where [artifacts] says.
	r = testutil.Run(t, w, nil, "artifact", "promote", "a1.v2")
	folder := glowFolder()
	if !strings.Contains(r.Stderr, "Promoted a1, \"Horizon glow\": v2 chosen, and put in records:"+folder+"/: landed on main") || !strings.Contains(r.Stderr, "it lives there from now on") {
		t.Errorf("promote said:\n%s", r.Stderr)
	}
	if got := testutil.Subjects(t, src, "main"); got != "Promote artifact horizon-glow: v2 | initial" {
		t.Errorf("records' main: %s", got)
	}
	if files := strings.TrimSpace(testutil.GitOut(t, src, "show", "--name-status", "--format=", "main")); files != "A\t"+folder+"/artifact.toml\nA\t"+folder+"/index.html\nA\t"+folder+"/thing.toml" {
		t.Errorf("the commit has:\n%s", files)
	}
	if got := remoteFile(t, w, folder+"/index.html"); got != glowV2 {
		t.Errorf("index.html on main: %q", got)
	}
	record := remoteFile(t, w, folder+"/artifact.toml")
	for _, want := range []string{`status = "promoted"`, "[promoted]", "version = 2", `note = "Accent at 8% in dark."`, `note = "No glow in light."`} {
		if !strings.Contains(record, want) {
			t.Errorf("its record there doesn't say %s:\n%s", want, record)
		}
	}
	for _, not := range []string{"Tighter", "[home]"} {
		if strings.Contains(record, not) {
			t.Errorf("its record there says %s:\n%s", not, record)
		}
	}
	body := testutil.GitOut(t, src, "log", "-1", "--format=%b", "main")
	for _, want := range []string{"Promoted by you: v2 of 3 chosen.", "Added by hand, by you.", "No glow in light.", "This folder holds the version chosen (index.html, and the files beside it)"} {
		if !strings.Contains(body, want) {
			t.Errorf("the commit's body doesn't say %q:\n%s", want, body)
		}
	}
	// Every version is still here, and its record says where it lives.
	for _, v := range []string{"a1.v1", "a1.v2", "a1.v3"} {
		if out := testutil.Run(t, w, nil, "artifact", "show", v).Stdout; !strings.Contains(out, "<title>Horizon glow</title>") {
			t.Errorf("%s isn't on this machine: %q", v, out)
		}
	}
	mine, _ := os.ReadFile(filepath.Join(local, "artifact.toml"))
	if !strings.Contains(string(mine), "[home]\nrepo = \"records\"\nfolder = \""+folder+"\"\nversion = 2\n") {
		t.Errorf("its record here:\n%s", mine)
	}
	// No capsule or branch is left of it.
	if entries, _ := os.ReadDir(filepath.Join(w.Root, "repos", "records")); len(entries) != 2 {
		t.Errorf("records has %v", entries)
	}
	show := testutil.Run(t, w, nil, "show", "a1").Stdout
	for _, want := range []string{"(promoted, on this machine, and records;", "It lives in records:" + folder + "/ too, for everyone with records: v2 is there", "v3 is newer", "v2 · based on v1"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a1 doesn't say %q:\n%s", want, show)
		}
	}
	if list := testutil.Run(t, w, nil, "artifact", "list").Stdout; !strings.Contains(list, "a1   v3   promoted") || !strings.Contains(list, "· v2 in records:"+folder+"/") {
		t.Errorf("ws artifact list:\n%s", list)
	}
	if added := testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow4.html", glowV3b), "--to", "a1", "--note", "Warmer.").Stderr; !strings.Contains(added, "(records:"+folder+"/ has v2: promote it to put this one there)") {
		t.Errorf("a version of one that lives there:\n%s", added)
	}

	// Two versions later, promoted again: it knows where it lives.
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow5.html", glowV1), "--to", "a1", "--note", "Back to the first.")
	r = testutil.Run(t, w, nil, "artifact", "promote", "a1")
	if !strings.Contains(r.Stderr, "v5 chosen, and put in records:"+folder+"/: landed on main") || strings.Contains(r.Stderr, "from now on") {
		t.Errorf("promoted again:\n%s", r.Stderr)
	}
	if got := testutil.Subjects(t, src, "main"); got != "Promote artifact horizon-glow: v5 | Promote artifact horizon-glow: v2 | initial" {
		t.Errorf("records' main: %s (not one commit more)", got)
	}
	if files := strings.TrimSpace(testutil.GitOut(t, src, "show", "--name-status", "--format=", "main")); files != "M\t"+folder+"/artifact.toml\nM\t"+folder+"/index.html" {
		t.Errorf("the second commit has:\n%s", files)
	}
	if got := remoteFile(t, w, folder+"/index.html"); got != glowV1 {
		t.Errorf("index.html on main, promoted again: %q", got)
	}
	if record := remoteFile(t, w, folder+"/artifact.toml"); !strings.Contains(record, "version = 5") || !strings.Contains(record, `note = "Back to the first."`) {
		t.Errorf("its record there:\n%s", record)
	}
	if body := testutil.GitOut(t, src, "log", "-1", "--format=%b", "main"); !strings.Contains(body, "Promoted by you: v5 of 5 chosen.") || strings.Contains(body, "This folder holds") {
		t.Errorf("the second commit's body:\n%s", body)
	}

	// It isn't moved, nor kept here only: it lives where it lives.
	refused(t, w, "lives in records:"+folder+"/ already", "artifact", "promote", "a1", "--home", "records:elsewhere/glow/")
	refused(t, w, "it isn't kept on this machine only", "artifact", "promote", "a1", "--here")
	// Named as the folder it has, it's the same.
	testutil.Run(t, w, nil, "artifact", "promote", "a1.v4", "--home", "records:"+folder+"/")
	if got := remoteFile(t, w, folder+"/index.html"); got != glowV3b {
		t.Errorf("index.html on main, v4 chosen: %q", got)
	}
}

// Where it lives is chosen the first time it's promoted: where
// [artifacts] says, a folder you name, or on this machine only, which is
// kept too. A document can cite only one that lives in a repo, and crew
// promote none.
func TestArtifactHome_ChosenTheFirstTime(t *testing.T) {
	w := homeWorkspace(t, "")
	dir := t.TempDir()
	src := w.Sources["records"]
	testutil.CommitIn(t, src, "plans/glow.md", "# The glow\n", "Add plan: the glow")
	fetchGround(t, w, "records")
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow.html", glowV1), "--note", "One.")
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "two.html", glowV2), "--title", "Two takes", "--note", "Two.")
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "three.html", glowV3), "--title", "Third", "--note", "Three.")
	start := testutil.Subjects(t, src, "main")

	// On this machine only: nothing's committed, and so it stays when
	// it's promoted again.
	r := testutil.Run(t, w, nil, "artifact", "promote", "a1", "--here")
	if !strings.Contains(r.Stderr, "Promoted a1, \"Horizon glow\": v1 chosen") || !strings.Contains(r.Stderr, "it's on this machine only") {
		t.Errorf("promoted here:\n%s", r.Stderr)
	}
	testutil.Run(t, w, nil, "artifact", "promote", "a1")
	if got := testutil.Subjects(t, src, "main"); got != start {
		t.Errorf("records' main, with one kept here: %s", got)
	}
	if show := testutil.Run(t, w, nil, "show", "a1").Stdout; !strings.Contains(show, "(promoted, on this machine;") || strings.Contains(show, "It lives in") {
		t.Errorf("ws show a1:\n%s", show)
	}
	// Until it's given a home: a folder you name.
	r = testutil.Run(t, w, nil, "artifact", "promote", "a1", "--home", "records:kept/glow")
	if !strings.Contains(r.Stderr, "and put in records:kept/glow/: landed on main") {
		t.Errorf("given a home:\n%s", r.Stderr)
	}
	if got := remoteFile(t, w, "kept/glow/index.html"); got != glowV1 {
		t.Errorf("kept/glow/index.html: %q", got)
	}

	// What a home can't be.
	before := testutil.Subjects(t, src, "main")
	refused(t, w, "there's something at records:kept/glow/ already", "artifact", "promote", "a2", "--home", "records:kept/glow/")
	refused(t, w, "api takes pull requests", "artifact", "promote", "a2", "--home", "api:artifacts/two/")
	refused(t, w, "nowhere isn't one of the workspace's repos", "artifact", "promote", "a2", "--home", "nowhere:two/")
	refused(t, w, "leads out of records", "artifact", "promote", "a2", "--home", "records:../two/")
	refused(t, w, "isn't a folder in a repo", "artifact", "promote", "a2", "--home", "two")
	refused(t, w, "not both", "artifact", "promote", "a2", "--here", "--home", "records:two/")
	// Into a document, it must live in a repo, where the document cites it.
	refused(t, w, "a document cites an artifact by its folder in a repo", "artifact", "promote", "a2", "--here", "--into", "records:plans/glow.md")
	refused(t, w, "neither a mission nor a document", "artifact", "promote", "a2", "--into", "nosuch")
	if got := testutil.Subjects(t, src, "main"); got != before {
		t.Errorf("records' main, after what was refused: %s", got)
	}
	twos := "artifacts/" + time.Now().Format("2006-01-02") + "-two-takes"
	r = testutil.Run(t, w, nil, "artifact", "promote", "a2", "--into", "records:plans/glow.md")
	if !strings.Contains(r.Stderr, "v1 chosen, into records:plans/glow.md, and put in records:"+twos+"/: landed on main") {
		t.Errorf("into a document:\n%s", r.Stderr)
	}
	if record := remoteFile(t, w, twos+"/artifact.toml"); !strings.Contains(record, `into = "records:plans/glow.md"`) {
		t.Errorf("its record there:\n%s", record)
	}
	// Into a mission, it's recorded on it.
	slug := strings.TrimSpace(testutil.Run(t, w, nil, "mission", "new", "--no-edit", "Build the glow").Stdout)
	r = testutil.Run(t, w, nil, "artifact", "promote", "a2", "--into", slug)
	if !strings.Contains(r.Stderr, "into "+slug+", and put in records:"+twos+"/") || !strings.Contains(r.Stderr, "recorded it on "+slug) {
		t.Errorf("into a mission:\n%s", r.Stderr)
	}
	if m := testutil.LoadMission(t, w, slug); len(m.Artifacts) != 1 || m.Artifacts[0].Ref != "a2" || m.Artifacts[0].Kind != "artifact" {
		t.Errorf("the mission's artifacts: %+v", m.Artifacts)
	}

	// ws.local.toml's .ws keeps yours here, whatever ws.toml says.
	appendFile(t, filepath.Join(w.Root, "ws.local.toml"), "\n[artifacts]\npath = \".ws\"\n")
	before = testutil.Subjects(t, src, "main")
	if r := testutil.Run(t, w, nil, "artifact", "promote", "a3"); !strings.Contains(r.Stderr, "it's on this machine only") {
		t.Errorf("with ws.local.toml's .ws:\n%s", r.Stderr)
	}
	if got := testutil.Subjects(t, src, "main"); got != before {
		t.Errorf("records' main: %s", got)
	}
	dr := testutil.RunCommand(t, w.Root, nil, "doctor")
	if doctor := dr.Stdout + dr.Stderr; !strings.Contains(doctor, "yours stay on this machine, as ws.local.toml says") {
		t.Errorf("ws doctor:\n%s", doctor)
	}

	// Crew don't promote.
	t.Setenv(mission.EnvCrew, "c1")
	refused(t, w, "only the user promotes an artifact", "artifact", "promote", "a3")
}

// Discarding one that lives in a repo takes its folder out of it, in a
// commit, before it goes to .discarded here; restoring it puts the
// version that was there back. One on this machine only touches no repo.
func TestArtifactHome_DiscardTakesItOutAndRestorePutsItBack(t *testing.T) {
	w := homeWorkspace(t, "")
	dir := t.TempDir()
	src := w.Sources["records"]
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow.html", glowV1), "--note", "Accent at 8% in dark.")
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow2.html", glowV2), "--to", "a1", "--note", "No glow in light.")
	testutil.Run(t, w, nil, "artifact", "promote", "a1.v1")
	folder := glowFolder()

	r := testutil.Run(t, w, nil, "artifact", "discard", "a1", "The", "second", "one's", "better.")
	if !strings.Contains(r.Stderr, "Discarded a1, \"Horizon glow\": its folder is in .ws/artifacts/.discarded/"+glowName()+"/ for 30 days") || !strings.Contains(r.Stderr, "It's out of records:"+folder+"/ too: landed on main") {
		t.Errorf("discard said:\n%s", r.Stderr)
	}
	if got := testutil.GitOut(t, src, "log", "-1", "--format=%s%n%b", "main"); !strings.Contains(got, "Discard artifact horizon-glow\nDiscarded by you: The second one's better.") || !strings.Contains(got, "\"Horizon glow\", v1 to v2") {
		t.Errorf("the commit:\n%s", got)
	}
	if out := testutil.GitOut(t, src, "ls-tree", "-r", "--name-only", "main", "artifacts/"); strings.TrimSpace(out) != "" {
		t.Errorf("main still has %s", out)
	}
	if _, err := os.Stat(filepath.Join(w.Root, ".ws", "artifacts", ".discarded", glowName(), "files.git", "HEAD")); err != nil {
		t.Errorf("its versions aren't in .discarded: %v", err)
	}
	show := testutil.Run(t, w, nil, "show", "a1").Stdout
	for _, want := range []string{"(discarded, on this machine;", ": The second one's better.", "ws artifact restore a1 puts it back, and in records:" + folder + "/, where it lived"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a1 doesn't say %q:\n%s", want, show)
		}
	}
	if strings.Contains(show, "It lives in") {
		t.Errorf("discarded, ws show a1 says it lives there:\n%s", show)
	}
	if list := testutil.Run(t, w, nil, "artifact", "list").Stdout; strings.TrimSpace(list) != "" {
		t.Errorf("listed:\n%s", list)
	}

	r = testutil.Run(t, w, nil, "artifact", "restore", "a1")
	if !strings.Contains(r.Stderr, "Restored a1, \"Horizon glow\", to .ws/artifacts/"+glowName()+"/ (promoted), and put back in records:"+folder+"/: landed on main") {
		t.Errorf("restore said:\n%s", r.Stderr)
	}
	if got := testutil.Subjects(t, src, "main"); got != "Restore artifact horizon-glow | Discard artifact horizon-glow | Promote artifact horizon-glow: v1 | initial" {
		t.Errorf("records' main: %s", got)
	}
	if got := remoteFile(t, w, folder+"/index.html"); got != glowV1 {
		t.Errorf("index.html, put back: %q", got)
	}
	if body := testutil.GitOut(t, src, "log", "-1", "--format=%b", "main"); !strings.Contains(body, "Restored by you: put back as it was, at v1.") {
		t.Errorf("its body:\n%s", body)
	}
	if show := testutil.Run(t, w, nil, "show", "a1").Stdout; !strings.Contains(show, "It lives in records:"+folder+"/ too") || !strings.Contains(show, "v1 is there") {
		t.Errorf("restored, ws show a1:\n%s", show)
	}

	// One that's here only: no commit.
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, dir, "two.html", glowV3), "--title", "Two takes", "--note", "Two.")
	before := testutil.Subjects(t, src, "main")
	if r := testutil.Run(t, w, nil, "artifact", "discard", "a2"); strings.Contains(r.Stderr, "out of") {
		t.Errorf("discarding one that's here only:\n%s", r.Stderr)
	}
	testutil.Run(t, w, nil, "artifact", "restore", "a2")
	if got := testutil.Subjects(t, src, "main"); got != before {
		t.Errorf("records' main: %s", got)
	}
}

// Crew's are on this machine like anyone's, whatever [artifacts] says of
// where promoted ones live: recorded on their mission, with no capsule of
// the repo to put them in.
func TestArtifactHome_CrewsAreOnThisMachine(t *testing.T) {
	w := homeWorkspace(t, "")
	testutil.Run(t, w, nil, "mission", "new", "Glow", "--brief", "## Goal\nTry a glow.\n\n## Stages\n1. Sketch it.")
	t.Setenv(mission.EnvMission, "glow")
	t.Setenv(mission.EnvCrew, "c1")
	r := testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, t.TempDir(), "glow.html", glowV1), "--note", "Accent at 8% in dark.")
	if strings.TrimSpace(r.Stdout) != "a1.v1" || !strings.Contains(r.Stderr, "in .ws/artifacts/"+glowName()+"/") || !strings.Contains(r.Stderr, "recorded it on glow") {
		t.Errorf("crew's add said:\n%s%s", r.Stdout, r.Stderr)
	}
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "initial" {
		t.Errorf("records' main: %s", got)
	}
	if m := testutil.LoadMission(t, w, "glow"); len(m.Artifacts) != 1 || m.Artifacts[0].Ref != "a1" || len(m.Capsules) != 0 {
		t.Errorf("the mission: %+v, capsules %+v", m.Artifacts, m.Capsules)
	}
}

// ws doctor says where a promoted one lives, and what's wrong with it: a
// records repo with no [artifacts], a repo that takes pull requests, a
// path it can't use, a guide that isn't there; and what's left in the
// repo from when artifacts were kept there whole.
func TestArtifactHome_Doctor(t *testing.T) {
	doctor := func(w *testutil.Workspace) string {
		t.Helper()
		r := testutil.RunCommand(t, w.Root, nil, "doctor")
		return r.Stdout + r.Stderr
	}
	w := homeWorkspace(t, "guide = \"records:ARTIFACTS.md\"\n")
	if got := doctor(w); !strings.Contains(got, "in records (records:artifacts/{date}-{slug}/): the version chosen, in one commit landed on its default branch each time it's promoted") || !strings.Contains(got, "records:ARTIFACTS.md isn't there") || strings.Contains(got, "left in records") {
		t.Errorf("living in records:\n%s", got)
	}
	// A folder that holds an artifact whole, as they used to be kept.
	src := w.Sources["records"]
	testutil.CommitIn(t, src, "artifacts/2026-11-01-old-sketch/artifact.toml", "title = \"Old sketch\"\nstatus = \"live\"\n", "Add artifact: Old sketch")
	testutil.CommitIn(t, src, "artifacts/2026-11-01-old-sketch/v1.html", glowV1, "Add its file")
	fetchGround(t, w, "records")
	if got := doctor(w); !strings.Contains(got, "1 folder in records hold every version of an artifact") || !strings.Contains(got, "2026-11-01-old-sketch") {
		t.Errorf("an old folder there:\n%s", got)
	}
	// A promoted one's home isn't one of them.
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, t.TempDir(), "g.html", glowV1), "--note", "x")
	testutil.Run(t, w, nil, "artifact", "promote", "a1")
	if got := doctor(w); !strings.Contains(got, "1 folder in records hold") {
		t.Errorf("with one promoted there:\n%s", got)
	}

	w = testutil.LandWorkspace(t, "")
	appendFile(t, filepath.Join(w.Root, "ws.toml"), "\n[missions]\nrecords = \"records\"\n")
	if got := doctor(w); !strings.Contains(got, "on this machine only (.ws/artifacts): there's a records repo, records, and no [artifacts]") || !strings.Contains(got, `path = "records:artifacts/{date}-{slug}/"`) {
		t.Errorf("a records repo and no [artifacts]:\n%s", got)
	}

	w = testutil.LandWorkspace(t, "")
	appendFile(t, filepath.Join(w.Root, "ws.toml"), "\n[artifacts]\npath = \"api:artifacts/{date}-{slug}/\"\n")
	if got := doctor(w); !strings.Contains(got, "which takes pull requests: a promoted one would wait in review") {
		t.Errorf("in a repo with pull requests:\n%s", got)
	}
	// One's added all the same, here; it's promoting it there that isn't done.
	if r := testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, t.TempDir(), "g.html", glowV1), "--note", "x"); !strings.Contains(r.Stderr, "(on this machine: promote it to keep it)") {
		t.Errorf("adding one:\n%s", r.Stderr)
	}
	refused(t, w, "api takes pull requests", "artifact", "promote", "a1")
	if r := testutil.Run(t, w, nil, "artifact", "promote", "a1", "--here"); !strings.Contains(r.Stderr, "it's on this machine only") {
		t.Errorf("promoted here:\n%s", r.Stderr)
	}

	w = testutil.LandWorkspace(t, "")
	appendFile(t, filepath.Join(w.Root, "ws.toml"), "\n[artifacts]\npath = \"records:artifacts/{date}.html\"\nexpire_after = \"a month\"\n")
	got := doctor(w)
	for _, want := range []string{`ws.toml's path = "records:artifacts/{date}.html": it's a folder, so it ends in /`, `expire_after = "a month": it's a number of days, as 30d, or never`} {
		if !strings.Contains(got, want) {
			t.Errorf("a path it can't use: no %q in\n%s", want, got)
		}
	}
	// It can't be used, so a promoted one stays here.
	testutil.Run(t, w, nil, "artifact", "add", writeHTML(t, t.TempDir(), "g.html", glowV1), "--note", "x")
	if r := testutil.Run(t, w, nil, "artifact", "promote", "a1"); !strings.Contains(r.Stderr, "it's on this machine only") {
		t.Errorf("with a path it can't use:\n%s", r.Stderr)
	}
}
