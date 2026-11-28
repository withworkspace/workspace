package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

// Crew's decisions are looked at once they've been quiet a moment, beside
// their step's checks and the user's decisions; one that goes against a
// check is told to the crew who made it, and each is looked at once.
// (In one mission c1 decided pins read git, against a check that nothing
// new reads git on a request path, and it came to light a round later.)
func TestDecisionLook_TellsCrewOfOneAgainstACheck(t *testing.T) {
	w, _, _ := workFixture(t, "d2: check 2: the lookup isn't cached at all")
	host := &listeningHost{}
	w.opts.Host = host
	if _, err := w.RecordDecision("cache", Deciding{What: "Key the cache per tenant", Why: "Tokens are per tenant"}, AsYou); err != nil {
		t.Fatal(err)
	}
	d, err := w.RecordDecision("cache", Deciding{What: "Look the token up each time, no cache", Why: "Simpler"}, AsCrew("c1"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Decision.ID != "d2" || d.Decision.Step != 1 || len(d.Checks) != 2 || d.Checks[1] != "the lookup is cached" {
		t.Fatalf("decided %+v: crew are given their step's checks", d)
	}
	now := time.Now()
	if due := w.DecisionsDue(now); len(due) != 0 {
		t.Errorf("due %v at once: it waits for crew to be quiet", due)
	}
	if due := w.DecisionsDue(now.Add(DecisionQuiet + time.Second)); len(due) != 1 || due[0] != "cache" {
		t.Fatalf("due %v after the quiet", due)
	}
	said, err := w.LookAtDecisions("cache")
	if err != nil || !strings.Contains(said, "told c1: d2 may go against your step's check 2") {
		t.Fatalf("look said %q, %v", said, err)
	}
	if len(fake.prompts) != 1 || !strings.Contains(fake.prompts[0], "2. the lookup is cached") || !strings.Contains(fake.prompts[0], "- d1: Key the cache per tenant.") || !strings.Contains(fake.prompts[0], "- d2 (c1): Look the token up each time") {
		t.Errorf("the look was told %q", fake.prompts)
	}
	if len(host.said) != 1 || !strings.Contains(host.said[0], `check 2 ("the lookup is cached")`) || !strings.Contains(host.said[0], "ws mission decide --supersedes d2") {
		t.Errorf("c1 was told %q", host.said)
	}
	// Looked at once.
	if m, _ := w.Missions.Load("cache"); m.DecisionsLooked != 2 {
		t.Errorf("looked at up to d%d, want d2", m.DecisionsLooked)
	}
	if due := w.DecisionsDue(now.Add(time.Hour)); len(due) != 0 {
		t.Errorf("due again: %v", due)
	}
	if said, _ := w.LookAtDecisions("cache"); said != "" {
		t.Errorf("looked again: %q", said)
	}
}

// A decision that fits is looked at and nobody's told; nor are yours
// looked at, nor is the look's answer anything but what it said.
func TestDecisionLook_OneThatFitsTellsNobody(t *testing.T) {
	w, _, _ := workFixture(t, "d1: fits")
	host := &listeningHost{}
	w.opts.Host = host
	w.RecordDecision("cache", Deciding{What: "Cache in memory"}, AsCrew("c1"))
	said, err := w.LookAtDecisions("cache")
	if err != nil || said != "looked at d1: they fit" || len(host.said) != 0 {
		t.Errorf("said %q, %v; c1 told %q", said, err, host.said)
	}
	if mine, _ := w.RecordDecision("cache", Deciding{What: "Ship it"}, AsYou); len(mine.Checks) != 0 {
		t.Errorf("yours came with checks: %+v", mine)
	}
	if due := w.DecisionsDue(time.Now().Add(time.Hour)); len(due) != 0 {
		t.Errorf("your decision is due a look: %v", due)
	}
}

// One against your own decision, by crew who've ended, comes to you: they
// can't be told, and it was theirs to ask you before going against it.
func TestDecisionLook_AgainstYoursByCrewWhoveEndedIsYours(t *testing.T) {
	w, _, _ := workFixture(t, "d2: d1: it caches for everyone, not per tenant")
	w.RecordDecision("cache", Deciding{What: "Key the cache per tenant"}, AsYou)
	w.RecordDecision("cache", Deciding{What: "One cache for all tenants"}, AsCrew("c1"))
	if err := w.Missions.EndCrew("cache", "c1", "exit 0"); err != nil {
		t.Fatal(err)
	}
	said, err := w.LookAtDecisions("cache")
	if err != nil || !strings.Contains(said, "flagged you: d2 may go against the user's d1") {
		t.Fatalf("said %q, %v", said, err)
	}
	m, _ := w.Missions.Load("cache")
	if task := m.Asking(); task == nil || !strings.Contains(task.Text, "d2, by c1, who've ended, may go against your d1") {
		t.Errorf("your task: %+v", task)
	}
	var noted bool
	log, _ := w.Missions.ReadLog("cache", 0)
	for _, e := range log {
		noted = noted || e.Kind == mission.EventNote && strings.Contains(e.Text, "Decision look: d2 may go against the user's d1")
	}
	if !noted {
		t.Error("nothing noted on the mission")
	}
}
