package page

import (
	"github.com/withworkspace/workspace/internal/store/storetest"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
)

func TestReviewPasses(t *testing.T) {
	defs := ReviewPasses([]PassDef{
		{Name: "security", Looks: "Find what an attacker could do.", Kinds: []string{"vulnerability", "hardening"}, Blocks: []string{"vulnerability"}, Repos: []string{"api"}},
		{Name: "plain", Looks: "Anything."},
		{Name: "skilled", Skill: "code-review", File: "/skills/code-review/SKILL.md"},
		{Name: "NOT OK", Looks: "x"},
		{Name: "empty"},
	})
	if len(defs) != len(ReviewBuiltin)+3 {
		t.Fatalf("defs: %+v", defs)
	}
	plain, _ := PassNamed(defs, "plain")
	if plain.Title != "Plain" || len(plain.Kinds) != 4 || !plain.Blocking("blocking") || plain.Blocking("nit") || plain.Summary == "" {
		t.Errorf("plain: %+v", plain)
	}
	sec, _ := PassNamed(defs, "security")
	if !sec.Blocking("vulnerability") || sec.Blocking("hardening") || sec.Blocking("blocking") {
		t.Errorf("security blocks: %+v", sec.Blocks)
	}
	// A skill's kinds are its own; what blocks is still said.
	skilled, _ := PassNamed(defs, "skilled")
	if len(skilled.Kinds) != 0 || !skilled.Blocking("blocking") {
		t.Errorf("skilled: %+v", skilled)
	}
	if got := ForRepo(defs, "web"); len(got) != len(defs)-1 {
		t.Errorf("for another repo: %+v", got)
	}
	if got := ForRepo(defs, "api"); len(got) != len(defs) {
		t.Errorf("for its repo: %+v", got)
	}
}

// Go or no go is worked out from what a pass found, and moves as it's
// settled: a finding on no lines counts as one on lines does.
func TestReviewStanding(t *testing.T) {
	def := PassDef{Name: "security", Kinds: []string{"vulnerability", "hardening"}, Blocks: []string{"vulnerability"}}
	at := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	if st := ReviewStanding(def, nil); st.Status != StandingNone {
		t.Errorf("never run: %+v", st)
	}
	root := t.TempDir()
	storetest.Own(t, root)
	store := conversation.ForWorkspace(root)
	c, err := store.OpenReviewPass("api#89", "security", at)
	if err != nil {
		t.Fatal(err)
	}
	if st := ReviewStanding(def, c); st.Status != StandingRunning {
		t.Errorf("on its first run: %+v", st)
	}
	fs := []conversation.Finding{
		{Kind: "vulnerability", Text: "This whole pull request is a bad idea."},
		{Kind: "hardening", Path: "a.go", Side: "new", Line: 3, EndLine: 3, Excerpt: []string{"x"}, Text: "Check the length."},
	}
	conversation.KeyFindings(fs)
	c, _ = store.Say(c.ID, conversation.Turn{By: conversation.Agent, At: at, Body: "Not as it is.", Findings: fs})
	if st := ReviewStanding(def, c); st.Status != StandingNoGo || st.Open != 2 || st.Blocking != 1 {
		t.Errorf("a blocker on no lines: %+v", st)
	}
	c, _ = store.SettleFinding(c.ID, fs[0].Key, conversation.FindingDismissed, conversation.DismissWrong, at)
	if st := ReviewStanding(def, c); st.Status != StandingNotes || st.Open != 1 || st.Blocking != 0 || st.Overruled != 1 {
		t.Errorf("the blocker dismissed: %+v", st)
	}
	c, _ = store.SettleFinding(c.ID, fs[1].Key, conversation.FindingDone, "", at)
	if st := ReviewStanding(def, c); st.Status != StandingGo || st.Open != 0 || st.Overruled != 1 {
		t.Errorf("all settled: %+v", st)
	}
	// It couldn't say anything, and never has.
	f, _ := store.OpenReviewPass("api#90", "security", at)
	f, _ = store.Say(f.ID, conversation.Turn{By: conversation.WS, At: at, Body: "The agent couldn't answer."})
	if st := ReviewStanding(def, f); st.Status != StandingFailed {
		t.Errorf("failed: %+v", st)
	}
}
