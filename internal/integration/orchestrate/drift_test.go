package orchestrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// These pinned where the command and the web UI did one thing two ways.
// Since slice 3 each is one operation in ops, and they check the two
// agree.
//
// Not pinned here, as each needs running crew or an agent: a message to
// crew (TalkedTo against Nudged), accepting a suggestion in a mission
// against outside one, and whether a review tells the orchestrator when
// it's off.

// stepsIn sets a mission up as one the orchestrator has dispatched for
// twice and is waiting on: what UserActed clears.
func stepsIn(t *testing.T, store mission.Store, slug string) {
	t.Helper()
	if _, err := store.Update(slug, func(m *mission.Mission) error {
		m.Dispatches = 2
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func dispatches(t *testing.T, store mission.Store, slug string) int {
	t.Helper()
	m, err := store.Load(slug)
	if err != nil {
		t.Fatal(err)
	}
	return m.Dispatches
}

func driftFixture(t *testing.T) (*testutil.Workspace, mission.Store, func(slug, verb, text string) (string, error)) {
	t.Helper()
	t.Setenv("WS_ROLE", "")
	w := testutil.MissionWorkspace(t)
	_, restore := cli.CaptureOrchestratorRuns()
	t.Cleanup(restore)
	for _, title := range []string{"Alpha", "Beta"} {
		testutil.Run(t, w, nil, "mission", "new", title, "--brief", "## Goal\nDo it.\n")
	}
	act := testutil.WebAct(w.Root)
	return w, mission.Store{Root: w.Root}, func(slug, verb, text string) (string, error) {
		return act(slug, web.ActRequest{Verb: verb, Text: text})
	}
}

// Accepting work with no stages, as you, counts as you stepping in.
func TestDrift_AcceptingAsYouResetsTheDispatchCount(t *testing.T) {
	w, store, act := driftFixture(t)
	stepsIn(t, store, "alpha")
	stepsIn(t, store, "beta")

	testutil.Run(t, w, nil, "mission", "accept", "-m", "alpha", "good")
	if got := dispatches(t, store, "alpha"); got != 0 {
		t.Errorf("after ws mission accept: %d dispatches", got)
	}
	if _, err := act("beta", "accept", ""); err != nil {
		t.Fatal(err)
	}
	if got := dispatches(t, store, "beta"); got != 0 {
		t.Errorf("after accepting in the web UI: %d dispatches", got)
	}
}

// Editing the brief, as you: the same.
func TestDrift_EditingTheBriefResetsTheDispatchCount(t *testing.T) {
	w, store, act := driftFixture(t)
	stepsIn(t, store, "alpha")
	stepsIn(t, store, "beta")
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("## Goal\nDo it well.\n"), 0o644)

	testutil.Run(t, w, nil, "mission", "brief", "-m", "alpha", "--set", brief)
	if got := dispatches(t, store, "alpha"); got != 0 {
		t.Errorf("after ws mission brief --set: %d dispatches", got)
	}
	if _, err := act("beta", "edit-brief", "## Goal\nDo it well.\n"); err != nil {
		t.Fatal(err)
	}
	if got := dispatches(t, store, "beta"); got != 0 {
		t.Errorf("after editing it in the web UI: %d dispatches", got)
	}
}

// Archiving, as you, counts too.
func TestDrift_ArchivingResetsTheDispatchCount(t *testing.T) {
	w, store, act := driftFixture(t)
	stepsIn(t, store, "alpha")
	stepsIn(t, store, "beta")

	testutil.Run(t, w, nil, "mission", "archive", "alpha")
	if got := dispatches(t, store, "alpha"); got != 0 {
		t.Errorf("after ws mission archive: %d dispatches", got)
	}
	if _, err := act("beta", "archive", ""); err != nil {
		t.Fatal(err)
	}
	if got := dispatches(t, store, "beta"); got != 0 {
		t.Errorf("after archiving in the web UI: %d dispatches", got)
	}
}

// Dismissing clears a flag, or else crew queued for you to launch, or
// else refuses: from the web UI and the command alike.
func TestDrift_Dismissing(t *testing.T) {
	w, store, act := driftFixture(t)
	queue := func(slug string) {
		t.Helper()
		if err := store.QueueDispatch(slug, &mission.Dispatch{Text: "build it"}); err != nil {
			t.Fatal(err)
		}
	}
	queued := func(slug string) bool {
		t.Helper()
		m, err := store.Load(slug)
		if err != nil {
			t.Fatal(err)
		}
		return m.Dispatch != nil
	}

	queue("alpha")
	testutil.Run(t, w, nil, "mission", "flag", "-m", "alpha", "--clear")
	if queued("alpha") {
		t.Error("ws mission flag --clear left the queued crew")
	}
	queue("beta")
	if _, err := act("beta", "dismiss", ""); err != nil {
		t.Fatal(err)
	}
	if queued("beta") {
		t.Error("the web UI left the queued crew")
	}

	// Nothing to dismiss: refused by both.
	if _, err := act("beta", "dismiss", ""); err == nil {
		t.Error("the web UI dismissed nothing")
	}
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "flag", "-m", "beta", "--clear"); r.Err == nil || !strings.Contains(r.Err.Error(), "nothing to dismiss") {
		t.Errorf("ws mission flag --clear with nothing to clear: %v", r.Err)
	}
}
