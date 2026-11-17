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
