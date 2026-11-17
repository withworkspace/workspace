package conversation

import (
	"strings"
	"testing"
	"time"
)

// A pass's reply is what it says of the page, then findings under headings
// that are their kinds: the words each is about, what it says, and words to
// put in their place.
func TestParseFindings(t *testing.T) {
	reply := strings.Join([]string{
		"The page says what's wanted, but not how it's checked.",
		"",
		"## question",
		"> It should be",
		"> fast.",
		"What counts as fast: a time to start, or no new dependencies?",
		"",
		"```suggest",
		"It should start in under 50ms.",
		"```",
		"",
		"## **Assumption:**",
		"> a flag for the name",
		"That the flag is `-name`.",
		"> not a quote, once it's begun",
		"",
		"## missing",
		"Nothing says who reviews it.",
		"",
		"## question",
		"> It should be fast.",
		"And on what machine?",
		"",
		"## ",
	}, "\n")
	summary, fs := ParseFindings(reply)
	if summary != "The page says what's wanted, but not how it's checked." {
		t.Errorf("summary: %q", summary)
	}
	if len(fs) != 4 {
		t.Fatalf("findings: %+v", fs)
	}
	q := fs[0]
	if q.Kind != "question" || q.Quote != "It should be fast." || q.Text != "What counts as fast: a time to start, or no new dependencies?" || q.Suggest != "It should start in under 50ms." {
		t.Errorf("the question: %+v", q)
	}
	if a := fs[1]; a.Kind != "assumption" || a.Quote != "a flag for the name" || !strings.Contains(a.Text, "> not a quote") {
		t.Errorf("the assumption: %+v", a)
	}
	if m := fs[2]; m.Kind != "missing" || m.Quote != "" || m.Text != "Nothing says who reviews it." || m.Key == "" {
		t.Errorf("what's missing: %+v", m)
	}
	// The same kind on the same words is the same finding, but two in one
	// run are told apart.
	if fs[3].Key == q.Key || !strings.HasPrefix(fs[3].Key, q.Key) {
		t.Errorf("keys: %q and %q", q.Key, fs[3].Key)
	}
	// Worded differently next time, it's still the first.
	_, again := ParseFindings("## question\n> it should be   FAST.\nHow fast?\n")
	if len(again) != 1 || again[0].Key != q.Key {
		t.Errorf("run again: %+v, want key %s", again, q.Key)
	}

	if s, none := ParseFindings("Nothing's open: it reads clearly.\n"); s != "Nothing's open: it reads clearly." || len(none) != 0 {
		t.Errorf("no findings: %q %+v", s, none)
	}
}

// A pass is opened on a page, found again by its name, and its findings
// settled by key, which holds across runs.
func TestPasses(t *testing.T) {
	s := ForWorkspace(owned(t))
	if _, err := s.OpenPass("p1", "Not A Word", time.Now()); err == nil {
		t.Error("a pass named by more than a word")
	}
	c, err := s.OpenPass("p1", "check", time.Now())
	if err != nil || c.Kind != Pass || c.Goal != "check" || !c.NeedsAnswer() {
		t.Fatalf("open: %+v %v", c, err)
	}
	if s.PassOn("p1", "cold") != nil || s.PassOn("p2", "check") != nil || s.PassOn("p1", "check").ID != c.ID {
		t.Error("finding a pass by its page and name")
	}
	_, fs := ParseFindings("## question\n> fast\nHow fast?\n\n## missing\nWho reviews it.\n")
	s.Say(c.ID, Turn{By: Agent, At: time.Now(), Body: "Two things.", Findings: fs})
	if _, err := s.SettleFinding(c.ID, fs[0].Key, "answered", "", time.Now()); err == nil {
		t.Error("a state that isn't one")
	}
	if _, err := s.SettleFinding(c.ID, fs[0].Key, FindingDone, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	s.SettleFinding(c.ID, fs[1].Key, FindingDismissed, "", time.Now())
	s.SettleFinding(c.ID, fs[1].Key, FindingOpen, "", time.Now())
	got, _ := s.Load(c.ID)
	if got.FindingState(fs[0].Key) != FindingDone || got.FindingState(fs[1].Key) != FindingOpen || len(got.Written().Findings) != 2 {
		t.Errorf("settled: %+v", got.Settled)
	}
	if len(s.PassesOn("p1")) != 1 || len(s.OnPage("p1")) != 0 {
		t.Error("a pass isn't a cell")
	}
}

// A finding's kind is its heading's first words, whatever's said after
// them: a skill's categories are often more than a word.
func TestFindingKind(t *testing.T) {
	for heading, want := range map[string]string{
		"question":                  "question",
		" **Ambiguity** ":           "ambiguity",
		"to write":                  "to-write",
		"To reconcile":              "to-reconcile",
		"ground: false":             "ground",
		"fit — Rollout":             "fit",
		"missing piece of the plan": "missing",
		"question.":                 "question",
	} {
		if got := findingKind(heading); got != want {
			t.Errorf("findingKind(%q) = %q, want %q", heading, got, want)
		}
	}
}

// A finding dismissed says why, and only a dismissed one can; opened again,
// it says nothing.
func TestDismissedAs(t *testing.T) {
	s := ForWorkspace(owned(t))
	c, _ := s.OpenPass("p1", "check", time.Now())
	_, fs := ParseFindings("## question\n> fast\nHow fast?\n\n## missing\nWho reviews it.\n")
	s.Say(c.ID, Turn{By: Agent, At: time.Now(), Body: "Two things.", Findings: fs})

	if _, err := s.SettleFinding(c.ID, fs[0].Key, FindingDone, DismissWrong, time.Now()); err == nil {
		t.Error("done, as wrong")
	}
	if _, err := s.SettleFinding(c.ID, fs[0].Key, FindingDismissed, "boring", time.Now()); err == nil {
		t.Error("a reason that isn't one")
	}
	s.SettleFinding(c.ID, fs[0].Key, FindingDismissed, DismissWrong, time.Now())
	s.SettleFinding(c.ID, fs[1].Key, FindingDismissed, DismissAlready, time.Now())
	s.SettleFinding(c.ID, fs[1].Key, FindingOpen, "", time.Now())
	got, _ := s.Load(c.ID)
	if got.DismissedAs(fs[0].Key) != DismissWrong || got.FindingState(fs[0].Key) != FindingDismissed {
		t.Errorf("dismissed as wrong: %+v %+v", got.Settled, got.Dismissed)
	}
	if got.DismissedAs(fs[1].Key) != "" || got.FindingState(fs[1].Key) != FindingOpen {
		t.Errorf("opened again: %+v %+v", got.Settled, got.Dismissed)
	}
}
