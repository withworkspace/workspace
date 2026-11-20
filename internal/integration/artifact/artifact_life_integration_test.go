package artifact_test

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/testutil"
)

// An artifact's lifetime on this machine: expiry, the daemon's sweep, and
// clearing the expired now. (Promoting one to where it lives in a repo,
// and discarding and restoring it there, are artifact_home's.)

// addAt adds an artifact as you, on a clock that says it's at.
func addAt(t *testing.T, w *testutil.Workspace, at time.Time, title, html, note string) string {
	t.Helper()
	added, err := testutil.OpsAt(w.Root, at).AddArtifact(ops.ArtifactAdd{HTML: []byte(html), Title: title, Note: note}, ops.AsYou)
	if err != nil {
		t.Fatalf("adding %s: %v", title, err)
	}
	return added.Entry.ID
}

// An artifact with no new version for 31 days is listed as expired, and
// the daemon's sweep discards it, "expired"; one in its last week says
// when; a promoted one doesn't expire, wherever it lives. Restored, one
// is live again, and counts from then. ws artifact clear does now what
// the sweep would.
func TestArtifactLifetime_ExpiredAreDiscarded(t *testing.T) {
	w := homeWorkspace(t, "")
	src := w.Sources["records"]
	old := time.Now().Add(-31 * 24 * time.Hour)

	glow := addAt(t, w, old, "Horizon glow", glowV1, "Accent at 8% in dark.")
	kept := addAt(t, w, old, "Kept sketch", glowV2, "Worth keeping.")
	nearly := addAt(t, w, time.Now().Add(-25*24*time.Hour), "Nearly old", glowV3, "Newer.")
	if glow != "a1" || kept != "a2" || nearly != "a3" {
		t.Fatalf("added %s %s %s", glow, kept, nearly)
	}
	testutil.Run(t, w, nil, "artifact", "promote", "a2")
	before := testutil.Subjects(t, src, "main")

	r := testutil.Run(t, w, nil, "artifact", "list")
	byID := map[string]string{}
	for _, l := range strings.Split(r.Stdout, "\n") {
		if f := strings.Fields(l); len(f) > 0 {
			byID[f[0]] = l
		}
	}
	if !strings.Contains(byID["a1"], "expired") {
		t.Errorf("a1 isn't listed as expired:\n%s", r.Stdout)
	}
	if !strings.Contains(byID["a2"], " promoted ") || strings.Contains(byID["a2"], "expire") {
		t.Errorf("a2, promoted, is listed as expiring:\n%s", r.Stdout)
	}
	if !strings.Contains(byID["a3"], " live ") || !strings.Contains(byID["a3"], "· expires in ") {
		t.Errorf("a3, in its last week, isn't listed as expiring:\n%s", r.Stdout)
	}
	if !strings.Contains(r.Stderr, "1 expired: ws artifact clear clears them") {
		t.Errorf("list's stderr:\n%s", r.Stderr)
	}
	if show := testutil.Run(t, w, nil, "show", "a1").Stdout; !strings.Contains(show, "(expired, on this machine;") || !strings.Contains(show, "and the daemon discards it, unless the user promotes it") {
		t.Errorf("ws show a1:\n%s", show)
	}
	dr := testutil.RunCommand(t, w.Root, nil, "doctor")
	if doctor := dr.Stdout + dr.Stderr; !strings.Contains(doctor, "1 expired, after 30 days with no new version: the daemon discards them within the day") {
		t.Errorf("ws doctor:\n%s", doctor)
	}

	// The daemon's sweep discards it.
	said := cli.SweepArtifacts(w.Root)
	if len(said) != 1 || said[0] != "artifacts: discarded 1 expired (a1)" {
		t.Errorf("the sweep said %q", said)
	}
	if list := testutil.Run(t, w, nil, "artifact", "list").Stdout; strings.Contains(list, "a1 ") || !strings.Contains(list, "a2 ") {
		t.Errorf("after the sweep:\n%s", list)
	}
	show := testutil.Run(t, w, nil, "show", "a1").Stdout
	for _, want := range []string{"(discarded, on this machine;", "by ws: expired", "ws artifact restore a1 puts it back"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a1 doesn't say %q:\n%s", want, show)
		}
	}
	if got := testutil.Subjects(t, src, "main"); got != before {
		t.Errorf("records' main, after the sweep: %s", got)
	}

	// Restored, it's live again, and counts from now.
	r = testutil.Run(t, w, nil, "artifact", "restore", "a1")
	if !strings.Contains(r.Stderr, "Restored a1, \"Horizon glow\", to .ws/artifacts/"+old.Format("2006-01-02")+"-horizon-glow/ (live)") {
		t.Errorf("restore said:\n%s", r.Stderr)
	}
	if list := testutil.Run(t, w, nil, "artifact", "list").Stdout; !strings.Contains(list, "a1   v1   live") || strings.Contains(list, "expired") {
		t.Errorf("restored:\n%s", list)
	}

	// Clear expired does it now, and says so; with none, that too.
	addAt(t, w, old, "Another old one", glowV3b, "Old.")
	r = testutil.Run(t, w, nil, "artifact", "clear")
	if !strings.Contains(r.Stderr, "Discarded a4, \"Another old one\", expired: its folder is in .ws/artifacts/.discarded/") {
		t.Errorf("clear said:\n%s", r.Stderr)
	}
	if r := testutil.Run(t, w, nil, "artifact", "clear"); !strings.Contains(r.Stderr, "nothing's expired") {
		t.Errorf("cleared again:\n%s", r.Stderr)
	}
	t.Setenv(mission.EnvCrew, "c1")
	refused(t, w, "only the user clears expired", "artifact", "clear")
}
