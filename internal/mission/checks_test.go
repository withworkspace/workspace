package mission

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// Crew who report done at a Work step with checks are checked by ws
// before anyone else looks: the court says so, rather than asking you to
// accept it; once a round has checked that done, it's not checked again.
func TestChecks_ADoneUncheckedIsWS(t *testing.T) {
	p, err := ParsePlan("## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Work: cache it.\n   - check: `go test ./...` passes\n2. Review: it.\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	m := &Mission{Slug: "cache", Plan: p, Crew: []Crew{{ID: "c1", Status: CrewDone, StatusAt: &at, Note: "cached", Started: at}},
		Stages: []StageRecord{{Text: p.Steps[0].Text, N: 1, Of: 2, Crew: []string{"c1"}, Started: &at}}}
	st, id, ok := m.UncheckedDone("")
	if !ok || id != "c1" || st.N != 1 {
		t.Fatalf("unchecked = %d %q %v", st.N, id, ok)
	}
	heads := Resolve(Snapshot{M: m, State: Active, OrchestratorOff: true})
	var kinds []string
	for _, h := range heads {
		kinds = append(kinds, string(h.Kind))
	}
	if !strings.Contains(strings.Join(kinds, ","), string(KindChecking)) || strings.Contains(strings.Join(kinds, ","), string(KindAccept)) {
		t.Errorf("headlines: %v", kinds)
	}
	m.Stages[0].Checks = []CheckRound{{N: 1, Crew: "c1", Done: m.Crew[0].DoneKey(), Results: []CheckResult{{Result: CheckUnmet}}}}
	if _, _, ok := m.UncheckedDone(""); ok {
		t.Error("a done checked is checked again")
	}
	later := at.Add(time.Minute)
	m.Crew[0].StatusAt, m.Crew[0].Note = &later, "cached, again"
	if _, _, ok := m.UncheckedDone(""); !ok {
		t.Error("a new done isn't checked")
	}
}

func TestCheckRound_Summary(t *testing.T) {
	r := CheckRound{Results: []CheckResult{{Check: "a", Result: CheckMet}, {Check: "b", Result: CheckUnmet}, {Check: "c", Result: CheckCantTell}}}
	if got := r.Summary(); got != "1 of 3 met; unmet: b; can't tell: c" || r.Met() {
		t.Errorf("summary = %q", got)
	}
}

// Records kept with a briefing state come across: a mission that was being
// briefed is to brief, with no state; every other is confirmed as it was
// made.
func TestConfirmedStep(t *testing.T) {
	s := owned(t)
	ready, _ := s.Create("Ready one")
	idea, _ := s.CreateBriefing("an idea")
	// As they were kept before confirmations.
	err := s.write(func(tx *store.Tx) error {
		if _, err := tx.Exec(`UPDATE missions SET record = json_remove(record, '$.confirmed') WHERE slug = ?`, ready.Slug); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE missions SET state = 'briefing', record = json_set(record, '$.state', 'briefing') WHERE slug = ?`, idea.Slug); err != nil {
			return err
		}
		return confirmedStep(tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := s.Load(ready.Slug)
	i, _ := s.Load(idea.Slug)
	if r.ToBrief() || r.Confirmed == nil || !r.Confirmed.Equal(r.Created) {
		t.Errorf("the ready one: %+v", r)
	}
	if !i.ToBrief() || i.State != "" {
		t.Errorf("the idea: %+v", i)
	}
}

// An agent told what a check came to is told how a command ws ran went,
// and none of what it printed when it was met; when it wasn't, the end of
// what it printed, where a failure says itself, and where the rest is.
func TestCheckResult_Said(t *testing.T) {
	out := "=== RUN TestA\n" + strings.Repeat("ok noise\n", 200) + "--- FAIL: TestCache (0.01s)\n    cache_test.go:12: want 2, got 1\nFAIL"
	failed := CheckResult{Check: "`go test ./...` passes", Result: CheckUnmet, By: "ws", Run: "check-1-1.cache.api#2",
		Evidence: "failed (exit 1) in 40s:\n" + out}
	if got := failed.How(); got != "failed (exit 1) in 40s" {
		t.Errorf("How: %q", got)
	}
	if got := failed.Printed(); got != out {
		t.Errorf("Printed: %q", got)
	}
	said := failed.Said(300)
	if !strings.HasPrefix(said, "failed (exit 1) in 40s: …") || !strings.Contains(said, "cache_test.go:12: want 2, got 1 FAIL") || strings.Contains(said, "=== RUN") {
		t.Errorf("unmet said the wrong end of what it printed: %q", said)
	}
	if !strings.HasSuffix(said, "(all it printed: `ws run logs check-1-1.cache.api#2`)") {
		t.Errorf("unmet said nothing of where the rest is: %q", said)
	}

	passed := CheckResult{Check: "`go test ./...` passes", Result: CheckMet, By: "ws", Run: "check-1-1.cache.api#3",
		Evidence: "passed in 12s:\n" + strings.Repeat("ok  pkg 0.1s\n", 50)}
	if got := passed.Said(600); got != "passed in 12s" {
		t.Errorf("met said more than how it went: %q", got)
	}

	judged := CheckResult{Check: "the cache is named", Result: CheckUnmet, By: "the judge (claude)", Evidence: "cache.go names nothing:\nsee line 3"}
	if got := judged.Said(600); got != "cache.go names nothing: see line 3" {
		t.Errorf("judge's said: %q", got)
	}
	if got := judged.Printed(); got != "" {
		t.Errorf("a judge printed nothing: %q", got)
	}

	cantRun := CheckResult{Check: "`pytest` passes", Result: CheckCantTell, By: "ws", Evidence: "couldn't run it (exit 127): sh: pytest: not found"}
	if got := cantRun.Said(600); got != "couldn't run it (exit 127): sh: pytest: not found" {
		t.Errorf("can't run said: %q", got)
	}
}

// A command that's `ws run <recipe>` alone asks for that recipe: a check's
// is the recipe's task run, a bundle's try: a button. Anything more is a
// command like any other.
func TestRecipeOf(t *testing.T) {
	for cmd, want := range map[string]string{
		"ws run web":           "web",
		"  ws  run   e2e-api ": "e2e-api",
		"ws run web --task":    "",
		"ws run":               "",
		"cd web && ws run web": "",
		"npm run dev":          "",
		"ws run Web":           "",
	} {
		if got := RecipeOf(cmd); got != want {
			t.Errorf("RecipeOf(%q) = %q, want %q", cmd, got, want)
		}
	}
}
