package mission_test

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// A capsule ends alone, the orchestrator suggests how the mission ends,
// and you end it, from the command line: ws show says each.
func TestEndings_FromTheCommandLine(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	t.Setenv(mission.EnvMission, "cache")
	testutil.Run(t, w, nil, "lift", "backend", "cache-docs")
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "done")

	// Handing a capsule on is the orchestrator's, and yours.
	t.Setenv("WS_ROLE", "orchestrator")
	r := testutil.Run(t, w, nil, "mission", "hand-on", "backend/cache-docs", "the docs need their own review")
	raised := strings.TrimSpace(r.Stdout)
	if raised == "" {
		t.Fatalf("hand-on printed no mission: %s", r.Stderr)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "drop", "backend/cache-lookup", "not needed"); r.Err == nil || !strings.Contains(r.Err.Error(), "the user's") {
		t.Errorf("the orchestrator dropping a capsule: %v", r.Err)
	}
	testutil.Run(t, w, nil, "mission", "conclude", "Cached, and the docs handed on", "--as", "answered", "--unmet", "1: no test run here")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "end"); r.Err == nil || !strings.Contains(r.Err.Error(), "the user's") {
		t.Errorf("the orchestrator ending it: %v", r.Err)
	}

	t.Setenv("WS_ROLE", "")
	show := testutil.Run(t, w, nil, "mission", "show", "-f", "llm")
	for _, want := range []string{
		"Capsules that ended alone (no longer the mission's, and they don't hold it open):\n  backend/cache-docs handed on to " + raised + ": the docs need their own review",
		"Suggested ending (orchestrator, for the user to end it): answered (early: 0 of 1 of Z met): Cached",
		"Z 1 not met: no test run here (tests pass)",
		"Z (the brief's Acceptance: where it ends; each is checked by its number when it ends):\n  1. tests pass",
	} {
		if !strings.Contains(show.Stdout, want) {
			t.Errorf("show missing %q:\n%s", want, show.Stdout)
		}
	}
	if m, _ := store.Load(raised); m == nil || !m.HasCapsule("backend", "cache-docs") || m.From != "cache" {
		t.Errorf("the mission it was handed on to: %+v", m)
	}

	// You end it as suggested; reopening takes it back.
	testutil.Run(t, w, nil, "mission", "end")
	show = testutil.Run(t, w, nil, "mission", "show", "-f", "llm")
	for _, want := range []string{"State: answered", "At: Ended, answered (early: 0 of 1 of Z met)", "Ended (you, "} {
		if !strings.Contains(show.Stdout, want) {
			t.Errorf("show after ending missing %q:\n%s", want, show.Stdout)
		}
	}
	testutil.Run(t, w, nil, "mission", "reopen")
	if m, _ := store.Load("cache"); m.Ending != nil {
		t.Error("reopen left the ending")
	}
	testutil.Run(t, w, nil, "mission", "end", "--as", "dropped", "Not worth it now")
	if m, _ := store.Load("cache"); m.Ending == nil || m.Ending.Ending != mission.EndDropped {
		t.Errorf("dropped: %+v", m.Ending)
	}
}

// On the mission's page: where it is on its plan, the ending suggested
// with Z as checked, ended there (and taken back), and a capsule dropped.
func TestWeb_Endings(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "done")
	act := testutil.WebAct(w.Root)
	plan := "## Z\n- Cached.\n- Product told.\n## Stops early\n- None.\n## Steps\n1. Cache it.\n   - who: crew, in `backend/cache-lookup`\n2. Merged.\n   - who: outside: a teammate's merge\n"
	if _, err := act("cache", web.ActRequest{Verb: "edit-plan", Text: plan}); err != nil {
		t.Fatal(err)
	}
	d, _ := testutil.WebMission(w.Root)("cache")
	if d.At != "Step 1 of 2: Cache it" || len(d.Z) != 2 {
		t.Errorf("at %q, Z %q", d.At, d.Z)
	}

	t.Setenv(mission.EnvMission, "cache")
	t.Setenv("WS_ROLE", "orchestrator")
	testutil.Run(t, w, nil, "mission", "conclude", "Cached; product told", "--as", "answered", "--met", "1: api#3", "--met", "2: in the thread")
	t.Setenv("WS_ROLE", "")
	d, _ = testutil.WebMission(w.Root)("cache")
	if c := d.Conclude; c == nil || c.Ending != "answered" || len(c.Z) != 2 || !c.Z[1].Met || c.Z[1].How != "in the thread" {
		t.Errorf("conclude = %+v", d.Conclude)
	}
	if h := d.Headlines[0]; h.Kind != "wrap-up" || h.Action.Verb != "end" {
		t.Errorf("headline = %+v", h)
	}

	said, err := act("cache", web.ActRequest{Verb: "end"})
	if err != nil || !strings.HasPrefix(said, "Ended, answered (at Z: 2 of 2 met)") {
		t.Fatalf("end: %q, %v", said, err)
	}
	d, _ = testutil.WebMission(w.Root)("cache")
	if d.State != "answered" || d.Ending == nil || d.Conclude != nil || d.Headlines[0].Kind != "ended" || d.Headlines[0].Group != "past" {
		t.Errorf("ended: state %q, ending %+v, headline %+v", d.State, d.Ending, d.Headlines[0])
	}
	if _, err := act("cache", web.ActRequest{Verb: "unend"}); err != nil {
		t.Fatal(err)
	}

	if _, err := act("cache", web.ActRequest{Verb: "drop", Capsule: "backend/cache-lookup"}); err == nil {
		t.Error("dropped with no why")
	}
	if _, err := act("cache", web.ActRequest{Verb: "drop", Capsule: "backend/cache-lookup", Text: "done elsewhere"}); err != nil {
		t.Fatal(err)
	}
	if _, err := act("cache", web.ActRequest{Verb: "end", Ending: "answered", Text: "x", Met: []string{"1: y"}}); err == nil || !strings.Contains(err.Error(), "part 2 of Z") {
		t.Errorf("ended with part of Z unchecked: %v", err)
	}
	if _, err := act("cache", web.ActRequest{Verb: "end", Ending: "dropped", Text: "done elsewhere"}); err != nil {
		t.Fatal(err)
	}
	d, _ = testutil.WebMission(w.Root)("cache")
	if len(d.EndedCapsules) != 1 || d.EndedCapsules[0].Ending != "dropped" || d.State != "dropped" || len(d.Capsules) != 0 {
		t.Errorf("after dropping: state %q, capsules %+v, ended %+v", d.State, d.Capsules, d.EndedCapsules)
	}
}
