package mission

import (
	"slices"
	"testing"
	"time"
)

// What a Review step asks you to approve is what the plan put under
// review: the capsules its steps worked in, and the documents it names
// (in the Review step, or a step's text, makes or checks). Crew's other
// documents are to read, after them.
func TestLooks_WhatThePlanNames(t *testing.T) {
	m := &Mission{Slug: "fix"}
	m.Plan, _ = ParsePlan("## Z\n- Done.\n## Stops early\n- None.\n## Steps\n"+
		"1. Work: fix it.\n   - in: `api`\n   - check: `docs/why.md` says why it broke\n"+
		"2. Work: write up the plan.\n   - in: `api`\n"+
		"3. Review: the fix, and `docs/plan.md`.\n", nil)
	then := time.Now().Add(-time.Hour)
	m.Stages = []StageRecord{
		{Text: "fix it.", N: 1, Of: 3, Crew: []string{"c1"}, Accepted: &then},
		{Text: "write up the plan.", N: 2, Of: 3, Crew: []string{"c2"}, Accepted: &then},
	}
	m.Crew = []Crew{{ID: "c1", Capsule: "api/fix", Status: CrewDone, Ended: &then}, {ID: "c2", Capsule: "api/fix", Status: CrewDone, Ended: &then}}
	m.Artifacts = []Artifact{
		{Ref: "mission:handoffs/c1.md", Kind: HandoffKind, Crew: "c1"},
		{Ref: "mission:seen.md", Kind: "note", Crew: "c2"},
		{Ref: "api/fix:docs/plan.md", Kind: "doc", Crew: "c2"},
		{Ref: "api/fix:docs/why.md", Kind: "doc", Crew: "c1"},
	}
	states, _ := m.StageProgress("")
	want := []Look{
		{Capsule: "api/fix", Name: "api/fix"},
		{Doc: "api/fix:docs/plan.md", Name: "docs/plan.md"},
		{Doc: "api/fix:docs/why.md", Name: "docs/why.md"},
		{Doc: "mission:handoffs/c1.md", Name: "handoffs/c1.md", Read: true},
		{Doc: "mission:seen.md", Name: "seen.md", Read: true},
	}
	if got := m.Looks(states, 3); !slices.Equal(got, want) {
		t.Errorf("looks:\n got %+v\nwant %+v", got, want)
	}
}
