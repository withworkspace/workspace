package mission

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseEnding(t *testing.T) {
	for in, want := range map[string]Ending{
		"landed": EndLanded, "Answered": EndAnswered, "handed on": EndHandedOn, "handed-on": EndHandedOn,
		"handed_on": EndHandedOn, "hand-on": EndHandedOn, "dropped": EndDropped, "drop": EndDropped,
	} {
		if got, err := ParseEnding(in); err != nil || got != want {
			t.Errorf("ParseEnding(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "merged", "done", "archived"} {
		if _, err := ParseEnding(in); err == nil {
			t.Errorf("ParseEnding(%q) took it", in)
		} else if !strings.Contains(err.Error(), "landed, answered, handed-on or dropped") {
			t.Errorf("ParseEnding(%q): %v doesn't say what's allowed", in, err)
		}
	}
	if EndHandedOn.Words() != "handed on" {
		t.Errorf("words = %q", EndHandedOn.Words())
	}
}

func TestBriefAcceptance(t *testing.T) {
	brief := "# T\n\n## Goal\nDo it.\n\n## Acceptance\n- Tests pass.\n- The doc says\n  what was built.\n1. Numbered too.\n\n### Notes\nnot acceptance\n"
	got := BriefAcceptance(brief)
	want := []string{"Tests pass.", "The doc says what was built.", "Numbered too."}
	if !slices.Equal(got, want) {
		t.Errorf("acceptance = %q, want %q", got, want)
	}
	if got := BriefAcceptance("## Acceptance criteria\nIt works, and\nis documented.\n## Stages\n1. x\n"); !slices.Equal(got, []string{"It works, and is documented."}) {
		t.Errorf("prose acceptance = %q", got)
	}
	if got := BriefAcceptance("## Goal\nx\n"); got != nil {
		t.Errorf("no acceptance = %q", got)
	}
	m := &Mission{}
	if got := m.Z(brief); len(got) != 3 {
		t.Errorf("Z without a plan = %q, want the brief's acceptance", got)
	}
	m.Plan, _ = ParsePlan(planWithOutside, nil)
	if got := m.Z(brief); !slices.Equal(got, []string{"Merged."}) {
		t.Errorf("Z with a plan = %q, want the plan's", got)
	}
}

func TestCheckZ(t *testing.T) {
	z := []string{"Merged.", "Product told.", "Docs updated."}
	checks, err := CheckZ(z, []string{"1: api#12 merged", "Z3. docs PR merged"}, []string{"2) product haven't replied"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !checks[0].Met || checks[1].Met || !checks[2].Met || checks[1].How != "product haven't replied" || checks[0].Z != "Merged." {
		t.Errorf("checks = %+v", checks)
	}
	c := Conclusion{Ending: EndAnswered, Text: "x", Z: checks}
	if met, of := c.Met(); met != 2 || of != 3 || c.AtZ() || c.ZWords() != "early: 2 of 3 of Z met" {
		t.Errorf("met %d of %d, at Z %v, %q", met, of, c.AtZ(), c.ZWords())
	}

	// Refused, with every problem.
	_, err = CheckZ(z, []string{"1: merged", "1: again", "4: nope", "merged", "2:"}, nil, false)
	var mz *MalformedZ
	if !errors.As(err, &mz) {
		t.Fatalf("err = %v, want MalformedZ", err)
	}
	for _, want := range []string{"checked twice", "Z has no part 4", "doesn't say which part", "part 2 of Z is met, but not how", "part 3 of Z (Docs updated.) isn't checked"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q doesn't say %q", err, want)
		}
	}
	if _, err := CheckZ(z, nil, []string{"1:"}, true); err == nil || !strings.Contains(err.Error(), "not why") {
		t.Errorf("an unmet part with no why: %v", err)
	}
	// Some may go unchecked (dropping): they're not met.
	checks, err = CheckZ(z, []string{"2: told"}, nil, true)
	if err != nil || checks[0].Met || !checks[1].Met || checks[2].How != "" {
		t.Errorf("unchecked parts: %+v, %v", checks, err)
	}
	if _, err := CheckZ(nil, []string{"1: x"}, nil, false); err == nil {
		t.Error("checking a Z there isn't was taken")
	}
	if checks, err := CheckZ(nil, nil, nil, false); err != nil || checks != nil {
		t.Errorf("no Z, nothing said: %v, %v", checks, err)
	}
}

// Z nobody checked isn't Z unmet: it's said nowhere, and a conclusion
// that says nothing it came to is its ending alone. Checks carry over to
// Z as it is now by a part's words.
func TestConclusion_ZUnchecked(t *testing.T) {
	c := Conclusion{Ending: EndLanded, Z: []ZCheck{{Z: "Merged."}, {Z: "Seen in a browser."}}}
	if c.ZWords() != "" || c.Summary() != "landed" {
		t.Errorf("unchecked: %q, %q", c.ZWords(), c.Summary())
	}
	c.Z[0] = ZCheck{Z: "Merged.", Met: true, How: "api#4"}
	c.Text = "Shipped."
	if c.Summary() != "landed (early: 1 of 2 of Z met): Shipped." {
		t.Errorf("one checked: %q", c.Summary())
	}
	z := c.OnZ([]string{"Documented.", "Merged."})
	if len(z) != 2 || z[0].Checked() || z[0].Z != "Documented." || !z[1].Met || z[1].How != "api#4" {
		t.Errorf("on Z as it is now: %+v", z)
	}
	if c.OnZ(nil) != nil {
		t.Error("checks on no Z")
	}
}

func TestDeriveState_FromThePlanAndItsEnding(t *testing.T) {
	merged := func(Capsule) CapsuleFacts { return CapsuleFacts{Exists: true, PR: PRMerged} }
	none := func(Capsule) CapsuleFacts { return CapsuleFacts{} }
	m := &Mission{Capsules: []Capsule{{Repo: "api", Name: "fix"}}}
	if got := DeriveState(m, merged); got != Landed {
		t.Errorf("without a plan, every PR merged = %s, want landed", got)
	}
	m.Plan, _ = ParsePlan(planWithOutside, nil)
	if got := DeriveState(m, merged); got != Active {
		t.Errorf("with a plan, every PR merged = %s, want active: its PRs aren't its state", got)
	}
	if !CapsulesLanded(m, merged) || CapsulesLanded(m, none) {
		t.Error("CapsulesLanded doesn't read the capsules")
	}
	if got := DeriveState(&Mission{Plan: m.Plan}, none); got != Draft {
		t.Errorf("a plan nothing's started on = %s, want draft", got)
	}
	started := time.Now()
	if got := DeriveState(&Mission{Plan: m.Plan, Stages: []StageRecord{{Text: "Fix it.", Started: &started}}}, none); got != Active {
		t.Errorf("a plan with a step started = %s, want active", got)
	}
	m.Ending = &Conclusion{Ending: EndAnswered, Text: "x"}
	if got := DeriveState(m, merged); got != Answered || !got.Ended() {
		t.Errorf("ended = %s, want answered", got)
	}
	m.State = Archived
	if got := DeriveState(m, merged); got != Archived {
		t.Errorf("archived and ended = %s, want archived: out of sight wins", got)
	}
}

func TestStore_EndingsAndCapsulesThatEndAlone(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Endings")
	s.Update(m.Slug, func(m *Mission) error {
		m.AddCapsuleBranch("api", "fix", "fix")
		m.AddCapsuleBranch("web", "fix", "fix")
		m.State = Parked
		return nil
	})
	if err := s.EndCapsule(m.Slug, EndedCapsule{Repo: "web", Name: "fix", Ending: EndHandedOn, To: "web-fix", Why: "its own work", By: "you"}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Load(m.Slug)
	if got.HasCapsule("web", "fix") || len(got.EndedCapsules) != 1 || got.EndedCapsules[0].Branch != "fix" || !slices.Equal(got.HandedOnTo(), []string{"web-fix"}) {
		t.Errorf("after handing web/fix on: capsules %+v, ended %+v", got.Capsules, got.EndedCapsules)
	}
	if err := s.EndCapsule(m.Slug, EndedCapsule{Repo: "web", Name: "fix", Ending: EndDropped}); !errors.Is(err, ErrNoCapsule) {
		t.Errorf("ending a capsule that isn't the mission's: %v", err)
	}
	// Back in the mission, it hasn't ended.
	s.Update(m.Slug, func(m *Mission) error { m.AddCapsule("web", "fix"); return nil })
	if got, _ := s.Load(m.Slug); len(got.EndedCapsules) != 0 {
		t.Errorf("a capsule back in the mission still ended: %+v", got.EndedCapsules)
	}

	s.Ask(m.Slug, "which?", "orchestrator")
	s.SetConclude(m.Slug, &Conclusion{Ending: EndAnswered, Text: "found it", By: "orchestrator"})
	if err := s.End(m.Slug, Conclusion{Ending: EndAnswered, Text: "found it", By: "you"}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Load(m.Slug)
	if got.Ending == nil || got.Ending.At.IsZero() || got.Conclude != nil || got.Asking() != nil || got.State != "" {
		t.Errorf("ended: ending %+v, conclude %+v, flag %+v, state %q", got.Ending, got.Conclude, got.Asking(), got.State)
	}
	log, _ := s.ReadLog(m.Slug, 0)
	var said []string
	for _, e := range log {
		if text, ok := Sentence(e); ok && (e.Kind == EventEnded || e.Kind == EventCapsuleEnded || e.Kind == EventConclude) {
			said = append(said, text)
		}
	}
	want := []string{"capsule web/fix handed on to web-fix: its own work", "suggested ending it, answered: found it", "you ended it, answered: found it"}
	if !slices.Equal(said, want) {
		t.Errorf("the log says %q, want %q", said, want)
	}
	if err := s.Unend(m.Slug, "you"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Load(m.Slug); got.Ending != nil {
		t.Error("Unend left the ending")
	}
}

func TestSnapshotPosition(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	plan, _ := ParsePlan("## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n1. Reproduce it as a failing test.\n   - who: crew\n2. Is this the cause?\n   - who: you\n3. Merged.\n   - who: outside: a teammate\n", nil)
	cases := []struct {
		name  string
		state State
		m     Mission
		brief string
		want  string
	}{
		{name: "a crew step, crew on it", state: Active, m: Mission{Plan: plan,
			Stages: []StageRecord{{Text: "Reproduce it as a failing test.", N: 1, Of: 3, Crew: []string{"c1"}, Started: ago(time.Hour)}},
			Crew:   []Crew{{ID: "c1", Activity: ActivityWorking}}},
			want: "Step 1 of 3: Reproduce it as a failing test (c1)"},
		{name: "a checkpoint", state: Active, m: Mission{Plan: plan,
			Stages: []StageRecord{{Text: "Reproduce it as a failing test.", N: 1, Of: 3, Accepted: ago(time.Hour)}}},
			want: "Step 2 of 3, yours: Is this the cause?"},
		{name: "outside", state: Active, m: Mission{Plan: plan,
			Stages: []StageRecord{{Text: "Reproduce it as a failing test.", N: 1, Of: 3, Accepted: ago(time.Hour)}, {Text: "Is this the cause?", N: 2, Of: 3, Accepted: ago(time.Hour)}}},
			want: "Step 3 of 3, waiting on a teammate: Merged"},
		{name: "every step done", state: Active, m: Mission{Plan: plan,
			Stages: []StageRecord{{Text: "Reproduce it as a failing test.", N: 1, Of: 3, Accepted: ago(time.Hour)}, {Text: "Is this the cause?", N: 2, Of: 3, Accepted: ago(time.Hour)}, {Text: "Merged.", N: 3, Of: 3, Accepted: ago(time.Hour)}}},
			want: "Every step done: at Z, if what Z says is true"},
		{name: "ended", state: Answered, m: Mission{Plan: plan, Ending: &Conclusion{Ending: EndAnswered, Text: "x", Z: []ZCheck{{Z: "Merged.", How: "no code"}}}},
			want: "Ended, answered (early: 0 of 1 of Z met)"},
		{name: "a brief's stages", state: Active, brief: "## Stages\n1. Write it.\n2. Build it.\n", want: "Stage 1 of 2: Write it"},
		{name: "a brief without stages", state: Active, brief: "## Goal\nx\n", want: ""},
		{name: "briefing", state: Briefing, m: Mission{Plan: plan}, want: ""},
	}
	for _, c := range cases {
		m := c.m
		if got := (Snapshot{M: &m, State: c.state, Brief: c.brief, Now: now}).Position(); got != c.want {
			t.Errorf("%s: position %q, want %q", c.name, got, c.want)
		}
	}
}
