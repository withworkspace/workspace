package mission

import (
	"errors"
	"strings"
	"testing"
)

func TestDecisions(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Cache")
	if all, err := s.Decisions(m.Slug); err != nil || len(all) != 0 {
		t.Fatalf("none yet: %v, %v", all, err)
	}
	if _, err := s.Decide(m.Slug, Decision{By: "you"}); err == nil {
		t.Error("a decision needs saying what")
	}
	d1, err := s.Decide(m.Slug, Decision{What: "Key the cache per user.", By: "c1", About: []string{"s1"}})
	if err != nil || d1.ID != "d1" || d1.State != DecisionCurrent {
		t.Fatalf("d1 = %+v, %v", d1, err)
	}
	d2, err := s.Decide(m.Slug, Decision{What: "Key the cache per tenant.", Why: "Tokens are issued per tenant.", By: "you", From: "t2", About: []string{"records/x:p.md"}, Supersedes: "d1"})
	if err != nil || d2.ID != "d2" {
		t.Fatalf("d2 = %+v, %v", d2, err)
	}
	if _, err := s.Decide(m.Slug, Decision{What: "x", By: "you", Supersedes: "d1"}); err == nil || !strings.Contains(err.Error(), "superseded already") {
		t.Errorf("superseding a superseded one: %v", err)
	}
	if _, err := s.Decide(m.Slug, Decision{What: "x", By: "you", Supersedes: "d9"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("superseding nothing: %v", err)
	}
	d3, _ := s.Decide(m.Slug, Decision{What: "Ship on Friday.", By: "orchestrator"})
	if err := s.Withdraw(m.Slug, "d3", "you", ""); err == nil {
		t.Error("withdrawing needs a why")
	}
	if err := s.Withdraw(m.Slug, d3.ID, "you", "Nobody's around on Fridays."); err != nil {
		t.Fatal(err)
	}
	if err := s.Withdraw(m.Slug, d3.ID, "you", "again"); err == nil {
		t.Error("withdrawn twice")
	}

	all, _ := s.Decisions(m.Slug)
	if len(all) != 3 || all[0].State != DecisionSuperseded || all[0].SupersededBy != "d2" || all[2].State != DecisionWithdrawn || all[2].WithdrawnWhy != "Nobody's around on Fridays." {
		t.Fatalf("all = %+v", all)
	}
	cur := CurrentDecisions(all)
	if len(cur) != 1 || cur[0].ID != "d2" {
		t.Errorf("current = %+v", cur)
	}
	if got := cur[0].Line(); got != "d2 (you, from t2): Key the cache per tenant. Why: Tokens are issued per tenant." {
		t.Errorf("line = %q", got)
	}
	if !cur[0].IsAbout("t2") || !cur[0].IsAbout("records/x:p.md") || cur[0].IsAbout("s1") {
		t.Error("IsAbout")
	}
	if !strings.HasSuffix(all[0].Line(), "[superseded by d2]") {
		t.Errorf("superseded line = %q", all[0].Line())
	}

	events, _ := s.ReadLog(m.Slug, 0)
	var texts []string
	for _, e := range events {
		if e.Kind == EventDecision {
			texts = append(texts, e.Crew+"|"+e.Text)
		}
	}
	if got := strings.Join(texts, " ; "); got != "c1|d1: Key the cache per user. ; |d2: Key the cache per tenant. (instead of d1) ; orchestrator|d3: Ship on Friday. ; |d3 withdrawn: Nobody's around on Fridays." {
		t.Errorf("log = %s", got)
	}
}

// A decision keeps the step it was made at: its crew's, else the one the
// plan is at; none where there's no plan, or all of it's passed.
func TestDecisionKeepsItsStep(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Cache")
	step := func(by string) int {
		t.Helper()
		d, err := s.Decide(m.Slug, Decision{What: "x", By: by})
		if err != nil {
			t.Fatal(err)
		}
		all, _ := s.Decisions(m.Slug)
		kept, _ := FindDecision(all, d.ID)
		if kept.Step != d.Step {
			t.Errorf("%s: recorded at step %d, kept as %d", d.ID, d.Step, kept.Step)
		}
		return d.Step
	}
	if n := step("you"); n != 0 {
		t.Errorf("with no plan: step %d", n)
	}
	if err := s.WritePlan(m.Slug, checkedPlan); err != nil {
		t.Fatal(err)
	}
	s.StartCrew(m.Slug, "shell", "")
	if _, err := s.JoinStage(m.Slug, "c1", 1); err != nil {
		t.Fatal(err)
	}
	if n := step("c1"); n != 1 {
		t.Errorf("crew on step 1: step %d", n)
	}
	s.SetCrewStatus(m.Slug, "c1", CrewDone, "cached")
	s.EndCrew(m.Slug, "c1", "exit 0")
	if _, _, err := s.AcceptStage(m.Slug, "you", "passed"); err != nil {
		t.Fatal(err)
	}
	if n := step("you"); n != 2 {
		t.Errorf("yours, with the plan at step 2: step %d", n)
	}
	if n := step("c1"); n != 1 {
		t.Errorf("crew whose step has passed: step %d", n)
	}
	if _, _, err := s.AcceptStage(m.Slug, "you", "read"); err != nil {
		t.Fatal(err)
	}
	if n := step("you"); n != 0 {
		t.Errorf("with every step passed: step %d", n)
	}
}

// A decision made by answering keeps its question beside it, and one
// recorded when the question was written into its why reads the same;
// agents are told both as they always were.
func TestDecisionKeepsWhatItAnswers(t *testing.T) {
	old := `{"made":{"id":"d1","what":"A","why":"the answer to c1's question: Keep it, or drop it? (A) drop it","by":"you","at":"2026-11-20T10:00:00Z"}}`
	now := `{"made":{"id":"d2","what":"Redis","why":"It's there already.","by":"you","at":"2026-11-20T11:00:00Z","asked":{"by":"orchestrator","text":"Which cache?"}}}`
	plain := `{"made":{"id":"d3","what":"Key it per tenant.","why":"the answer to that is in the proposal","by":"c1","at":"2026-11-20T12:00:00Z"}}`
	all := replayDecisions([][]byte{[]byte(old), []byte(now), []byte(plain)})
	if len(all) != 3 {
		t.Fatalf("decisions = %+v", all)
	}
	if d := all[0]; d.Why != "" || d.Asked == nil || *d.Asked != (Asked{By: "c1", Text: "Keep it, or drop it? (A) drop it"}) {
		t.Errorf("an old answer: %+v", d)
	}
	if got, want := all[0].Line(), "d1 (you): A Why: the answer to c1's question: Keep it, or drop it? (A) drop it"; got != want {
		t.Errorf("an old answer's line = %q, want %q", got, want)
	}
	if got, want := all[1].Reason(), "the answer to orchestrator's question: Which cache? It's there already."; got != want {
		t.Errorf("an answer with a why of its own = %q, want %q", got, want)
	}
	if d := all[2]; d.Asked != nil || d.Reason() != "the answer to that is in the proposal" {
		t.Errorf("a why that only starts like one: %+v", d)
	}
}
