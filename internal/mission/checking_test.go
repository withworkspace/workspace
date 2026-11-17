package mission

import (
	"strings"
	"testing"
	"time"
)

// checkingMission is c1 reported done at step 1 of 2, a Work step with
// five checks: three commands, then two for the judge.
func checkingMission(t *testing.T) (*Mission, time.Time) {
	t.Helper()
	p, err := ParsePlan("## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Work: cache it.\n"+
		"   - check: `go vet ./...` passes\n   - check: `just test` passes\n   - check: `just check` passes\n"+
		"   - check: the cache is per tenant\n   - check: the tests cover it\n2. Review: it.\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	return &Mission{Slug: "cache", Plan: p, Crew: []Crew{{ID: "c1", Status: CrewDone, StatusAt: &at, Note: "cached", Started: at}},
		Stages: []StageRecord{{Text: p.Steps[0].Text, N: 1, Of: 2, Crew: []string{"c1"}, Started: &at}}}, at
}

func checkingHeadlineOf(t *testing.T, m *Mission, now time.Time) Headline {
	t.Helper()
	for _, h := range Resolve(Snapshot{M: m, State: Active, OrchestratorOff: true, Now: now}) {
		if h.Kind == KindChecking {
			return h
		}
	}
	t.Fatal("no checking headline")
	return Headline{}
}

// While a check runs, the headline names it (n of N), its command, how
// long it's been going (from when it started, so it ticks), its last line
// and when it's stopped, and that the judge comes after; then that the
// judge is reading; without anything running, it's as it was.
func TestChecking_TheHeadlineSaysWhatsRunning(t *testing.T) {
	m, at := checkingMission(t)
	start := at.Add(time.Minute)
	m.Checking = &Checking{Step: 1, Crew: "c1", Of: 5, N: 2, Command: "just test", Line: "ok  internal/cli  4.2s",
		Judged: []int{4, 5}, Since: start, Until: start.Add(15 * time.Minute)}

	h := checkingHeadlineOf(t, m, start.Add(80*time.Second))
	want := "Check 2 of 5 for step 1: `just test`, 1m20s so far (stopped at 15m). Last line: “ok  internal/cli  4.2s”. The judge reads checks 4 and 5 after"
	if h.Text != want || !h.Since.Equal(start) || h.Court != CourtAgent {
		t.Errorf("headline = %q since %v, want %q", h.Text, h.Since, want)
	}
	// It ticks: the same state, later.
	if h := checkingHeadlineOf(t, m, start.Add(3*time.Minute)); !strings.Contains(h.Text, "3m so far") {
		t.Errorf("later: %q", h.Text)
	}
	// No line yet, and nothing for the judge.
	m.Checking.Line, m.Checking.Judged = "", nil
	if h := checkingHeadlineOf(t, m, start.Add(5*time.Second)); h.Text != "Check 2 of 5 for step 1: `just test`, 5s so far (stopped at 15m)" {
		t.Errorf("no line: %q", h.Text)
	}

	// The judge's turn.
	m.Checking = &Checking{Step: 1, Crew: "c1", Of: 5, Judged: []int{4, 5}, Judge: "the judge (claude:sonnet)", Since: start, Until: start.Add(10 * time.Minute)}
	if h := checkingHeadlineOf(t, m, start.Add(40*time.Second)); h.Text != "The judge (claude:sonnet) is reading checks 4 and 5 of step 1: 40s so far (stopped at 10m)" || h.Label != "… judging" {
		t.Errorf("judging: %q (%s)", h.Text, h.Label)
	}

	// Left from a ws that stopped, long past when it would have been
	// stopped, or for another step or crew: the usual headline.
	plain := "ws is checking step 1: what c1 reported done, against what has to be true for it to be done"
	if h := checkingHeadlineOf(t, m, start.Add(time.Hour)); h.Text != plain || !h.Since.IsZero() {
		t.Errorf("stale: %q", h.Text)
	}
	m.Checking.Crew = "c2"
	if h := checkingHeadlineOf(t, m, start.Add(time.Second)); h.Text != plain {
		t.Errorf("another crew's: %q", h.Text)
	}
	m.Checking = nil
	if h := checkingHeadlineOf(t, m, start); h.Text != plain {
		t.Errorf("nothing running: %q", h.Text)
	}
}

// The state is kept on the mission as each check begins, its last line as
// it changes (only for the check it's of), and cleared when the round is
// recorded.
func TestChecking_KeptThenClearedByTheRound(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Cache")
	if err := s.WritePlan(m.Slug, "## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Work: cache it.\n   - check: `true` passes\n2. Review: it.\n"); err != nil {
		t.Fatal(err)
	}
	s.StartCrew(m.Slug, "shell", "")
	if _, err := s.JoinStage(m.Slug, "c1", 1); err != nil {
		t.Fatal(err)
	}
	since := time.Now()
	if err := s.SetChecking(m.Slug, Checking{Step: 1, Crew: "c1", Of: 1, N: 1, Command: "true", Since: since, Until: since.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	s.CheckingLine(m.Slug, since, "halfway")
	s.CheckingLine(m.Slug, since.Add(-time.Second), "from a check that's ended")
	got, _ := s.Load(m.Slug)
	if c := got.Checking; c == nil || c.Line != "halfway" || c.Command != "true" || !c.Since.Equal(since) {
		t.Fatalf("checking = %+v", got.Checking)
	}
	// Another step's isn't cleared by this one's.
	s.ClearChecking(m.Slug, 2)
	if got, _ := s.Load(m.Slug); got.Checking == nil {
		t.Fatal("cleared by another step")
	}
	if _, err := s.RecordChecks(m.Slug, 1, CheckRound{Crew: "c1", Capsule: "api/x", Results: []CheckResult{{Check: "`true` passes", Result: CheckMet, Evidence: "passed in 0s", By: "ws"}}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Load(m.Slug); got.Checking != nil {
		t.Errorf("still checking once the round's recorded: %+v", got.Checking)
	}
}

func TestCheckNumbersAndElapsed(t *testing.T) {
	for in, want := range map[string]string{"2": "check 2", "2,3": "checks 2 and 3", "2,4,5": "checks 2, 4 and 5"} {
		var ns []int
		for _, p := range strings.Split(in, ",") {
			ns = append(ns, int(p[0]-'0'))
		}
		if got := CheckNumbers(ns); got != want {
			t.Errorf("CheckNumbers(%s) = %q", in, got)
		}
	}
	for d, want := range map[time.Duration]string{0: "0s", 45 * time.Second: "45s", 80*time.Second + 400*time.Millisecond: "1m20s", 2 * time.Minute: "2m", -time.Second: "0s"} {
		if got := Elapsed(d); got != want {
			t.Errorf("Elapsed(%s) = %q, want %q", d, got, want)
		}
	}
}
