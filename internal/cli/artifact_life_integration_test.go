package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/testutil"
)

// An artifact's lifetime: expiry in both stores, the daemon's sweep on
// this machine and Clear expired in git, promoting (and moving to the
// repo), discarding and restoring.

// addAt adds an artifact as you, on a clock that says it's at.
func addAt(t *testing.T, w *testutil.Workspace, at time.Time, title, html, note string) string {
	t.Helper()
	added, err := cli.OpsAt(w.Root, at).AddArtifact(ops.ArtifactAdd{HTML: []byte(html), Title: title, Note: note}, ops.AsYou)
	if err != nil {
		t.Fatalf("adding %s: %v", title, err)
	}
	return added.Entry.ID
}

// The plan's Done-when: an artifact dated 31 days ago is discarded on this
// machine (by the daemon's sweep), and listed as expired in git, where
// Clear expired takes it away in one commit. Restored, it's live again.
func TestArtifactLifetime_ExpiredDiscardedHereAndListedInGit(t *testing.T) {
	w := gitArtifactsWorkspace(t, "")
	src := w.Sources["records"]
	old := time.Now().Add(-31 * 24 * time.Hour)

	glow := addAt(t, w, old, "Horizon glow", glowV1, "Accent at 8% in dark.")
	appendFile(t, filepath.Join(w.Root, "ws.local.toml"), "\n[artifacts]\npath = \".ws\"\n")
	sketch := addAt(t, w, old, "Old sketch", glowV2, "Here only.")
	nearly := addAt(t, w, time.Now().Add(-25*24*time.Hour), "Nearly old", glowV3, "Here only, newer.")
	if glow != "a1" || sketch != "a2" || nearly != "a3" {
		t.Fatalf("added %s %s %s", glow, sketch, nearly)
	}
	oldFolder := "artifacts/" + old.Format("2006-01-02") + "-horizon-glow"

	r := run(t, w, nil, "artifact", "list")
	lines := strings.Split(r.Stdout, "\n")
	byID := map[string]string{}
	for _, l := range lines {
		if f := strings.Fields(l); len(f) > 0 {
			byID[f[0]] = l
		}
	}
	if !strings.Contains(byID["a1"], "expired") || !strings.Contains(byID["a1"], "· in records") {
		t.Errorf("a1, in git, isn't listed as expired:\n%s", r.Stdout)
	}
	if !strings.Contains(byID["a2"], "expired") {
		t.Errorf("a2, here, isn't listed as expired:\n%s", r.Stdout)
	}
	if !strings.Contains(byID["a3"], " live ") || !strings.Contains(byID["a3"], "· expires in ") {
		t.Errorf("a3, in its last week, isn't listed as expiring:\n%s", r.Stdout)
	}
	if !strings.Contains(r.Stderr, "2 expired: ws artifact clear clears them") {
		t.Errorf("list's stderr:\n%s", r.Stderr)
	}

	// The daemon's sweep discards the one here, and leaves git's alone.
	said := cli.SweepArtifacts(w.Root)
	if len(said) != 1 || said[0] != "artifacts: discarded 1 expired (a2)" {
		t.Errorf("the sweep said %q", said)
	}
	if list := run(t, w, nil, "artifact", "list").Stdout; strings.Contains(list, "a2 ") || !strings.Contains(list, "a1 ") {
		t.Errorf("after the sweep:\n%s", list)
	}
	show := run(t, w, nil, "show", "a2").Stdout
	for _, want := range []string{"(discarded, on this machine;", "by ws: expired", "ws artifact restore a2 puts it back"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a2 doesn't say %q:\n%s", want, show)
		}
	}
	if !strings.Contains(run(t, w, nil, "show", "a1").Stdout, "(expired, in records;") {
		t.Errorf("ws show a1:\n%s", run(t, w, nil, "show", "a1").Stdout)
	}
	dr := testutil.RunCommand(t, w.Root, nil, "doctor")
	if doctor := dr.Stdout + dr.Stderr; !strings.Contains(doctor, "1 expired in records, after 30 days with no new version") {
		t.Errorf("ws doctor:\n%s", doctor)
	}
	before := subjects(t, src, "main")

	// Clear expired: one commit taking a1's folder away, saying what it took.
	r = run(t, w, nil, "artifact", "clear")
	if !strings.Contains(r.Stderr, "Cleared a1 from records, expired: landed on main") || !strings.Contains(r.Stderr, "History keeps them (records@") {
		t.Errorf("clear said:\n%s", r.Stderr)
	}
	if got := subjects(t, src, "main"); got != "Clear expired artifacts | "+before {
		t.Errorf("records' main: %s", got)
	}
	body := gitOut(t, src, "log", "-1", "--format=%b", "main")
	if !strings.Contains(body, "- "+oldFolder+"/: \"Horizon glow\", v1") || !strings.Contains(body, "Cleared by you: 1 artifact, each live and with no new version for 30 days.") {
		t.Errorf("its body:\n%s", body)
	}
	if out := gitOut(t, src, "ls-tree", "-r", "--name-only", "main", "artifacts/"); strings.TrimSpace(out) != "" {
		t.Errorf("main still has %s", out)
	}
	show = run(t, w, nil, "show", "a1").Stdout
	for _, want := range []string{"(discarded, in records's history;", "by you: expired", "History keeps it, at records@", "ws artifact restore a1 puts it back"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a1 doesn't say %q:\n%s", want, show)
		}
	}
	if got := run(t, w, nil, "artifact", "show", "a1.v1").Stdout; got != glowV1 {
		t.Errorf("a1.v1, from history: %q", got)
	}
	if r := run(t, w, nil, "artifact", "clear"); !strings.Contains(r.Stderr, "nothing's expired") {
		t.Errorf("cleared again:\n%s", r.Stderr)
	}

	// Restored, it's back in a new commit, and counts from now.
	r = run(t, w, nil, "artifact", "restore", "a1")
	if !strings.Contains(r.Stderr, "Restored a1, \"Horizon glow\", to records:"+oldFolder+"/ (live): landed on main") {
		t.Errorf("restore said:\n%s", r.Stderr)
	}
	if got := subjects(t, src, "main"); !strings.HasPrefix(got, "Restore artifact horizon-glow | Clear expired artifacts") {
		t.Errorf("records' main: %s", got)
	}
	if record := remoteFile(t, w, oldFolder+"/artifact.toml"); !strings.Contains(record, "[restored]") || strings.Contains(record, "discarded") {
		t.Errorf("its record:\n%s", record)
	}
	if list := run(t, w, nil, "artifact", "list").Stdout; !strings.Contains(list, "a1   v1   live") || strings.Contains(list, "expired") {
		t.Errorf("restored:\n%s", list)
	}
	// And the one here, from .discarded.
	run(t, w, nil, "artifact", "restore", "a2")
	if list := run(t, w, nil, "artifact", "list").Stdout; !strings.Contains(list, "a2   v1   live") {
		t.Errorf("a2 restored:\n%s", list)
	}
}

// Promoting one on this machine and moving it to the repo is one commit,
// with all its versions and its record, promoted; it keeps its ref, and
// it's gone from .ws. A document can cite only one in a repo.
func TestArtifactLifetime_PromoteMovesALocalOneInOneCommit(t *testing.T) {
	w := gitArtifactsWorkspace(t, "")
	src, dir := w.Sources["records"], t.TempDir()
	commitIn(t, src, "plans/glow.md", "# The glow\n", "Add plan: the glow")
	fetchGround(t, w, "records")
	appendFile(t, filepath.Join(w.Root, "ws.local.toml"), "\n[artifacts]\npath = \".ws\"\n")
	run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow.html", glowV1), "--note", "Accent at 8% in dark.")
	run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow2.html", glowV2), "--to", "a1", "--note", "No glow in light.")
	local := filepath.Join(w.Root, ".ws", "artifacts", time.Now().Format("2006-01-02")+"-horizon-glow")
	if _, err := os.Stat(local); err != nil {
		t.Fatal(err)
	}

	// Crew don't promote.
	t.Setenv(mission.EnvCrew, "c1")
	if r := testutil.RunCommand(t, w.Root, nil, "artifact", "promote", "a1"); r.Err == nil || !strings.Contains(r.Err.Error(), "only the user promotes an artifact") {
		t.Errorf("crew promoting: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "")
	if r := testutil.RunCommand(t, w.Root, nil, "artifact", "promote", "a1", "--into", "records:plans/glow.md"); r.Err == nil || !strings.Contains(r.Err.Error(), "move it to records as you promote it (--move)") {
		t.Errorf("into a document, here: %v", r.Err)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "artifact", "promote", "a1", "--into", "nosuch"); r.Err == nil || !strings.Contains(r.Err.Error(), "neither a mission nor a document") {
		t.Errorf("into nothing that's there: %v", r.Err)
	}
	before := subjects(t, src, "main")

	r := run(t, w, nil, "artifact", "promote", "a1.v1", "--into", "records:plans/glow.md", "--move")
	folder := glowFolder("")
	if !strings.Contains(r.Stderr, "Promoted a1, \"Horizon glow\": v1 chosen, into records:plans/glow.md, and moved to records:"+folder+"/: landed on main") {
		t.Errorf("promote said:\n%s", r.Stderr)
	}
	if got := subjects(t, src, "main"); got != "Promote artifact horizon-glow | "+before {
		t.Errorf("records' main: %s (not one commit)", got)
	}
	files := gitOut(t, src, "show", "--name-status", "--format=", "main")
	for _, want := range []string{"A\t" + folder + "/artifact.toml", "A\t" + folder + "/v1.html", "A\t" + folder + "/v2.html"} {
		if !strings.Contains(files, want) {
			t.Errorf("the commit doesn't have %s:\n%s", want, files)
		}
	}
	if body := gitOut(t, src, "log", "-1", "--format=%b", "main"); !strings.Contains(body, "Promoted by you: v1 of 2 chosen, into records:plans/glow.md.") || !strings.Contains(body, "moved here") {
		t.Errorf("its body:\n%s", body)
	}
	record := remoteFile(t, w, folder+"/artifact.toml")
	for _, want := range []string{`status = "promoted"`, "[promoted]", "version = 1", `into = "records:plans/glow.md"`, `note = "No glow in light."`} {
		if !strings.Contains(record, want) {
			t.Errorf("its record doesn't say %s:\n%s", want, record)
		}
	}
	if got := remoteFile(t, w, folder+"/v2.html"); got != glowV2 {
		t.Errorf("v2.html: %q", got)
	}
	if entries, _ := os.ReadDir(filepath.Join(w.Root, ".ws", "artifacts")); len(entries) != 0 {
		t.Errorf(".ws/artifacts still has %v", entries)
	}
	show := run(t, w, nil, "show", "a1").Stdout
	for _, want := range []string{"(promoted, in records;", "v1 was chosen, into records:plans/glow.md", "Kept in records"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a1 doesn't say %q:\n%s", want, show)
		}
	}
	if list := run(t, w, nil, "artifact", "list").Stdout; strings.Count(list, "\n") != 1 || !strings.Contains(list, "a1   v2   promoted") {
		t.Errorf("listed:\n%s", list)
	}

	// Promoted again, in git: a commit choosing v2, into a mission it's
	// recorded on.
	slug := strings.TrimSpace(run(t, w, nil, "mission", "new", "--no-edit", "Build the glow").Stdout)
	r = run(t, w, nil, "artifact", "promote", "a1", "--into", slug)
	if !strings.Contains(r.Stderr, "v2 chosen, into "+slug+": landed on main") || !strings.Contains(r.Stderr, "recorded it on "+slug) {
		t.Errorf("promoted again:\n%s", r.Stderr)
	}
	if record := remoteFile(t, w, folder+"/artifact.toml"); !strings.Contains(record, "version = 2") || !strings.Contains(record, `into = "`+slug+`"`) {
		t.Errorf("its record:\n%s", record)
	}
	m := loadMission(t, w, slug)
	if len(m.Artifacts) != 1 || m.Artifacts[0].Ref != "a1" || m.Artifacts[0].Kind != "artifact" {
		t.Errorf("the mission's artifacts: %+v", m.Artifacts)
	}
}

// Discarded in git: a commit takes its folder away, its reason in the
// body; a teammate who fetches sees it discarded, why, and where history
// keeps it, and can put it back.
func TestArtifactLifetime_DiscardAndRestoreInGit(t *testing.T) {
	w := gitArtifactsWorkspace(t, "")
	src, dir := w.Sources["records"], t.TempDir()
	run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow.html", glowV1), "--note", "Accent at 8% in dark.")
	folder := glowFolder("")

	// Someone else discards it, from their clone.
	other := otherClone(t, w)
	r := run(t, other, nil, "artifact", "discard", "a1", "The", "second", "one's", "better.")
	if !strings.Contains(r.Stderr, "Discarded a1, \"Horizon glow\": landed on main") || !strings.Contains(r.Stderr, "ws artifact restore a1 puts it back") {
		t.Errorf("discard said:\n%s", r.Stderr)
	}
	if got := gitOut(t, src, "log", "-1", "--format=%s%n%b", "main"); !strings.Contains(got, "Discard artifact horizon-glow\nDiscarded by you: The second one's better.") {
		t.Errorf("the commit:\n%s", got)
	}

	// Here, once fetched, it's discarded, not gone.
	fetchGround(t, w, "records")
	show := run(t, w, nil, "show", "a1").Stdout
	for _, want := range []string{"(discarded, in records's history;", ": The second one's better.", "History keeps it, at records@", "ws artifact restore a1 puts it back"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a1 doesn't say %q:\n%s", want, show)
		}
	}
	if list := run(t, w, nil, "artifact", "list").Stdout; strings.TrimSpace(list) != "" {
		t.Errorf("listed:\n%s", list)
	}
	if got := run(t, w, nil, "artifact", "show", "a1").Stdout; got != glowV1 {
		t.Errorf("a1 from history: %q", got)
	}

	run(t, w, nil, "artifact", "restore", "a1")
	if got := remoteFile(t, w, folder+"/v1.html"); got != glowV1 {
		t.Errorf("restored v1.html: %q", got)
	}
	run(t, w, nil, "artifact", "discard", "a1")
	if got := subjects(t, src, "main"); !strings.HasPrefix(got, "Discard artifact horizon-glow | Restore artifact horizon-glow | Discard artifact horizon-glow") {
		t.Errorf("records' main: %s", got)
	}
	if body := gitOut(t, src, "log", "-1", "--format=%b", "main"); !strings.Contains(body, "Discarded by you, with no reason given.") {
		t.Errorf("its body:\n%s", body)
	}
	if list := run(t, w, nil, "artifact", "list", "--all").Stdout; !strings.Contains(list, "a1   v1   discarded") {
		t.Errorf("listed with --all:\n%s", list)
	}
}

// Crew's version in their capsule, of one you discard meanwhile, isn't
// lost: landing the capsule brings the artifact back, live, with it.
func TestArtifactLifetime_CrewsVersionBringsBackOneDiscardedMeanwhile(t *testing.T) {
	w := gitArtifactsWorkspace(t, "")
	run(t, w, nil, "artifact", "add", writeHTML(t, t.TempDir(), "glow.html", glowV1), "--note", "The first.")
	run(t, w, nil, "mission", "new", "Glow", "--brief", "## Goal\nTry a glow.\n\n## Stages\n1. Sketch it.")
	t.Setenv(mission.EnvMission, "glow")
	run(t, w, nil, "lift", "records", "glow-sketch")
	capsule := capsuleDir(w, "records", "glow-sketch")
	t.Setenv(mission.EnvCrew, "c1")
	t.Setenv(cli.ShellPWD, capsule)
	run(t, w, nil, "artifact", "add", writeHTML(t, t.TempDir(), "g.html", glowV2), "--to", "a1", "--note", "No glow in light.")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(cli.ShellPWD, "")
	t.Setenv(mission.EnvMission, "")

	// Crew's in flight: you can't discard what hasn't landed, but you can
	// what has, and their version waits.
	run(t, w, nil, "artifact", "discard", "a1", "Not this one.")
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-q", "-m", "Sketch the glow without light")
	run(t, w, nil, "land", "records", "glow-sketch")
	record := remoteFile(t, w, glowFolder("")+"/artifact.toml")
	if !strings.Contains(record, "n = 2") || !strings.Contains(record, `status = "live"`) {
		t.Errorf("landed on main:\n%s", record)
	}
	if list := run(t, w, nil, "artifact", "list").Stdout; !strings.Contains(list, "a1   v2   live") {
		t.Errorf("listed:\n%s", list)
	}
}
