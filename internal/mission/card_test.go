package mission

import (
	"strings"
	"testing"
	"time"
)

// courtCards is each case of the court's table (court_test.go), as cards:
// kept as they're checked, for the web UI's stories.
var courtCards []struct {
	Name  string `json:"name"`
	Cards []Card `json:"cards"`
}

// cardsKeepTheRule checks a case's cards against what a card promises:
// a title; one action meant to move it at most, never a quiet one; the
// quiet ones after the rest; nothing to press where it says there's
// nothing to do but look; and a card of yours with something to do.
func cardsKeepTheRule(t *testing.T, name string, s Snapshot, hs []Headline) {
	t.Helper()
	cards := CardsOf(s, hs)
	if len(cards) == 0 || len(cards) > len(hs) {
		t.Errorf("%s: %d cards of %d headlines", name, len(cards), len(hs))
		return
	}
	for _, c := range cards {
		if strings.TrimSpace(c.Title) == "" {
			t.Errorf("%s: %s has no title", name, c.Kind)
		}
		if strings.Contains(c.Title, "nothing for you to do") || strings.Contains(c.Detail, "· nothing for you to do") {
			t.Errorf("%s: %s says there's nothing to do in its words: %q / %q", name, c.Kind, c.Title, c.Detail)
		}
		if c.Primary != nil && c.Primary.Quiet {
			t.Errorf("%s: %s leads with a quiet action: %+v", name, c.Kind, c.Primary)
		}
		quiet := false
		for _, a := range c.More {
			if a.Label == "" || a.Do == "" {
				t.Errorf("%s: %s offers an action with no words: %+v", name, c.Kind, a)
			}
			if quiet && !a.Quiet {
				t.Errorf("%s: %s has %q after a quiet action", name, c.Kind, a.Label)
			}
			quiet = quiet || a.Quiet
			if a.Field != "" && !a.Quiet {
				t.Errorf("%s: %s opens a field from %q, which isn't quiet", name, c.Kind, a.Label)
			}
		}
		if c.Court == CourtYou && c.Primary == nil && len(c.More) == 0 {
			t.Errorf("%s: %s is yours, with nothing to press", name, c.Kind)
		}
	}
	courtCards = append(courtCards, struct {
		Name  string `json:"name"`
		Cards []Card `json:"cards"`
	}{name, cards})
}

// The step the plan waits at is never a second card under a question: the
// question's card carries its control, first.
func TestCards_AStepUnderAQuestionIsOneCard(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	done := now.Add(-time.Minute)
	at := func(plan string, second StageRecord) Snapshot {
		p, err := ParsePlan(plan, nil)
		if err != nil {
			t.Fatal(err)
		}
		second.N, second.Of = 2, 2
		return Snapshot{Now: now, State: Active, M: &Mission{Slug: "s", Plan: p,
			Stages: []StageRecord{{Text: "Fix it.", N: 1, Of: 2, Accepted: &done}, second},
			Tasks:  []Task{{N: 1, Verb: TaskAnswer, Text: "Its last run failed. Run it again, or wait?", Options: []string{"Run it again", "Wait"}, By: "orchestrator", At: now}},
		}}
	}
	labels := func(c Card) string {
		var out []string
		if c.Primary != nil {
			out = append(out, "*"+c.Primary.Label)
		}
		for _, a := range c.More {
			if a.Quiet {
				out = append(out, "~"+a.Label)
			} else {
				out = append(out, a.Label)
			}
		}
		return strings.Join(out, " | ")
	}
	const z = "## Z\n- Released.\n## Stops early\n- None.\n## Steps\n1. Fix it.\n   - who: crew\n"

	watched := at(z+"2. Wait: **Deploy.** It's deployed.\n   - on: npm\n   - check: `npm view pkg@1.0.0 version` passes\n", StageRecord{Text: "**Deploy.** It's deployed.", Watch: &Watch{Since: done}})
	cards := Cards(watched)
	if got := labels(cards[0]); len(cards) != 1 || cards[0].Folded != KindWaitingOn || got != "*Look now | Run it again | Wait | ~Answer… | ~Ask an agent | ~Dismiss" {
		t.Errorf("a question at a Wait ws watches: %d cards, %s", len(cards), got)
	}

	manual := at(z+"2. Wait: **Deploy.** It's deployed.\n   - on: the platform team\n", StageRecord{Text: "**Deploy.** It's deployed."})
	cards = Cards(manual)
	if got := labels(cards[0]); len(cards) != 1 || got != "*Mark step 2 done | Run it again | Wait | ~Answer… | ~Ask an agent | ~Dismiss" {
		t.Errorf("a question at a Wait nobody's watching: %d cards, %s", len(cards), got)
	}

	// Alone, each Wait leads with what's meant to move it.
	watched.M.Tasks, manual.M.Tasks = nil, nil
	if got := labels(Cards(watched)[0]); got != "*Look now | ~Mark done anyway…" {
		t.Errorf("a Wait ws watches: %s", got)
	}
	if c := Cards(manual)[0]; labels(c) != "*Mark done" || c.Primary.Field == "" {
		t.Errorf("a Wait nobody's watching: %s, %+v", labels(c), c.Primary)
	}
}

// What a card says is a title and a sentence: where its headline's
// sentence has a title, that; a task's, written when it was asked, says
// whose the step is, and what it is.
func TestCards_SayWhatsHappening(t *testing.T) {
	for _, c := range []struct {
		kind                Kind
		text, title, detail string
	}{
		{KindMerge, "api#46 is ready to merge: it's approved and its checks pass", "api#46 is ready to merge", "It's approved and its checks pass"},
		{KindChecksRunning, "api#45's checks are running · nothing for you to do", "api#45's checks are running", ""},
		{KindWrapUp, "Ready to end, answered (at Z: 1 of 1 met): The list is the slow one.", "Ready to end, answered (at Z: 1 of 1 met)", "The list is the slow one."},
		{KindParked, "Parked", "Parked", ""},
	} {
		got := cardBuilder{}.card(Headline{Court: CourtOutside, Kind: c.kind, Text: c.text})
		if got.Title != c.title || got.Detail != c.detail {
			t.Errorf("%s: %q / %q, want %q / %q", c.kind, got.Title, got.Detail, c.title, c.detail)
		}
	}
	if c := (cardBuilder{}).card(Headline{Court: CourtOutside, Kind: KindChecksRunning, Text: "api#45's checks are running · nothing for you to do"}); !c.Nothing {
		t.Error("nothing to do isn't said of it")
	}
}
