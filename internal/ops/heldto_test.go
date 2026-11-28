package ops

import (
	"strings"
	"testing"
)

// Crew recording a decision are given what their work is held to: their
// step's checks, the later Work steps' (d3 was made at step 1 against a
// check of step 3's), and the user's decisions. Nobody else is.
func TestRecordDecision_CrewAreGivenWhatTheWorkIsHeldTo(t *testing.T) {
	w, _, _ := workFixture(t, "")
	plan := "## Z\n- Cached.\n## Stops early\n- None.\n## Steps\n" +
		"1. Work: cache the lookup.\n   - in: `frontend/one`\n   - check: the lookup is cached\n" +
		"2. Work: wire it up.\n   - in: `frontend/one`\n   - check: nothing new reads git on a request path\n" +
		"3. Review: the cache.\n"
	if err := w.Missions.WritePlan("cache", plan); err != nil {
		t.Fatal(err)
	}
	w.RecordDecision("cache", Deciding{What: "Key the cache per tenant"}, AsYou)
	d, err := w.RecordDecision("cache", Deciding{What: "Pins read the branch from git"}, AsCrew("c1"))
	if err != nil {
		t.Fatal(err)
	}
	h := d.HeldTo
	if h.Step != 1 || strings.Join(h.Checks, "|") != "the lookup is cached" || len(h.Later) != 1 || h.Later[0].Step != 2 ||
		h.Later[0].Checks[0] != "nothing new reads git on a request path" || strings.Join(h.Yours, "|") != "d1: Key the cache per tenant" {
		t.Errorf("held to %+v", h)
	}
	if mine, _ := w.RecordDecision("cache", Deciding{What: "Ship it"}, AsYou); mine.HeldTo.Any() {
		t.Errorf("yours came with %+v", mine.HeldTo)
	}
}
