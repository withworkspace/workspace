package mission_test

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// Decisions are recorded by whoever makes them, about things ws can show,
// and are seen wherever they matter: in the views of what they're about,
// the mission's, and the orchestrator's prompt.
func TestDecide(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	t.Setenv(mission.EnvMission, "cache")

	if r := testutil.RunCommand(t, w.Root, nil, "mission", "decide", "Cache per user.", "--about", "nope/what"); r.Err == nil || !strings.Contains(r.Err.Error(), "--about and --from name things ws show can show") {
		t.Errorf("about something that isn't: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "c1")
	testutil.Run(t, w, nil, "mission", "decide", "Cache per user.", "--about", "backend/cache-lookup")
	t.Setenv(mission.EnvCrew, "")
	testutil.Run(t, w, nil, "mission", "decide", "Cache per tenant, not per user.", "--why", "Tokens are issued per tenant.", "--about", "backend/cache-lookup", "--from", "c1", "--supersedes", "d1")
	t.Setenv("WS_ROLE", "orchestrator")
	testutil.Run(t, w, nil, "mission", "decide", "Spike Redis before choosing.", "--about", "backend/cache-lookup")
	t.Setenv("WS_ROLE", "")
	testutil.Run(t, w, nil, "mission", "decide", "--withdraw", "d3", "An in-process cache is enough.")

	all, _ := store.Decisions("cache")
	if len(all) != 3 || all[0].By != "c1" || all[0].State != mission.DecisionSuperseded || all[1].By != "you" || all[2].By != "orchestrator" || all[2].State != mission.DecisionWithdrawn {
		t.Fatalf("decisions = %+v", all)
	}

	show := func(args ...string) string {
		t.Helper()
		return testutil.Run(t, w, nil, append([]string{"show", "-f", "llm"}, args...)...).Stdout
	}
	if out := show("backend/cache-lookup"); !strings.Contains(out, "d2  decided: Cache per tenant, not per user.") || strings.Contains(out, "d1  decided") || strings.Contains(out, "d3  decided") {
		t.Errorf("the capsule's view should list only what stands:\n%s", out)
	}
	if out := show("c1"); !strings.Contains(out, "decided: Cache per tenant") {
		t.Errorf("crew's view lacks the decision made from it:\n%s", out)
	}
	out := show("d2")
	for _, want := range []string{"Decision d2 (current): Cache per tenant, not per user.", "Why: Tokens are issued per tenant.", "Decided by you", "c1", "where it was decided", "backend/cache-lookup", "what it's about", "d1", "what it replaced: Cache per user."} {
		if !strings.Contains(out, want) {
			t.Errorf("d2's view lacks %q:\n%s", want, out)
		}
	}
	if out := show("d1"); !strings.Contains(out, "Decision d1 (superseded)") || !strings.Contains(out, "what replaced it") {
		t.Errorf("d1:\n%s", out)
	}
	if out := show(); !strings.Contains(out, "Decisions (what's been decided") || !strings.Contains(out, "  d2 (you, from c1): Cache per tenant, not per user. Why: Tokens are issued per tenant.") || strings.Contains(out, "  d3 (") {
		t.Errorf("the mission's view:\n%s", out)
	}
	p := testutil.Run(t, w, nil, "mission", "orchestrate", "-m", "cache", "--crew", "c1", "--trigger", "done", "--print-prompt").Stdout
	for _, want := range []string{"## Decisions", "- d2 (you, from c1): Cache per tenant", "ws mission decide \"<what was decided>\""} {
		if !strings.Contains(p, want) {
			t.Errorf("orchestrator prompt lacks %q", want)
		}
	}
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "decide", "--withdraw", "d3", "again"); r.Err == nil {
		t.Error("withdrawn twice")
	}
}
