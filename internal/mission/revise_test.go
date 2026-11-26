package mission

import (
	"strings"
	"testing"
	"time"
)

const fixPlan = `## Z
- Merged, with its regression test.

## Stops early
- It can't be reproduced.

## Steps
1. Reproduce it as a failing test.
   - who: crew, in ` + "`api/fix`" + `
   - makes: the failing test, and the cause
2. Is this the cause?
   - who: you
3. Plan the fix.
   - who: the planner
4. Review the fix.
   - who: you
5. Open a pull request.
   - who: crew, in ` + "`api/fix`" + `
   - makes: a pull request
`

func mustPlan(t *testing.T, text string) *Plan {
	t.Helper()
	p, err := ParsePlan(text, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// progress is the plan's steps with the first accepted ones accepted, and
// the one after begun (crew on it) if begun.
func progress(p *Plan, accepted int, begun bool) []StageState {
	m := &Mission{Plan: p}
	at := time.Now()
	for i := range accepted {
		m.Stages = append(m.Stages, StageRecord{Text: p.Steps[i].Text, N: i + 1, Of: len(p.Steps), Accepted: &at})
	}
	if begun {
		m.Stages = append(m.Stages, StageRecord{Text: p.Steps[accepted].Text, N: accepted + 1, Of: len(p.Steps), Crew: []string{"c9"}, Started: &at})
	}
	states, _ := m.StageProgress("")
	return states
}

func TestFixedSteps(t *testing.T) {
	p := mustPlan(t, fixPlan)
	for _, c := range []struct {
		accepted int
		begun    bool
		want     int
	}{
		{0, false, 0}, // step 1 not begun: it may change
		{0, true, 1},  // crew are on it
		{1, false, 1}, // step 2, a checkpoint, not begun
		{2, false, 3}, // step 3 is the planner's: it plans what's after it
		{5, false, 5}, // every step passed
	} {
		if got := FixedSteps(progress(p, c.accepted, c.begun)); got != c.want {
			t.Errorf("accepted %d, begun %v: fixed %d, want %d", c.accepted, c.begun, got, c.want)
		}
	}
}

// At its own step, the planner writes the steps after it: steps added
// before a checkpoint don't move it, and it applies.
func TestRevise_ThePlannersStep(t *testing.T) {
	was := mustPlan(t, fixPlan)
	now := mustPlan(t, strings.Replace(fixPlan, "4. Review the fix.\n   - who: you\n5. Open a pull request.", "4. Fix it, with the test passing.\n   - who: crew, in `api/fix`\n5. Review the fix.\n   - who: you\n6. Open a pull request.", 1))
	r := Revise(was, now, progress(was, 2, false))
	if r.Problem() != "" || len(r.Yours) != 0 || r.Fixed != 3 {
		t.Fatalf("held %q, yours %q, fixed %d", r.Held, r.Yours, r.Fixed)
	}
	want := "step 4 is step 5 now (Review the fix); step 5 is step 6 now (Open a pull request); step 4 added (Fix it, with the test passing, crew, in `api/fix`)"
	if r.Summary() != want {
		t.Errorf("summary = %q", r.Summary())
	}
	// Written as it was: nothing changed, and that's fine.
	if r := Revise(was, was, progress(was, 2, false)); r.Changed() || r.Problem() != "" || r.Summary() != "nothing changed" {
		t.Errorf("unchanged: %+v", r)
	}
}

// Removing or moving a checkpoint ahead, or changing Z, is the user's.
func TestRevise_Yours(t *testing.T) {
	was := mustPlan(t, fixPlan)
	states := progress(was, 2, false)

	removed := mustPlan(t, strings.Replace(fixPlan, "4. Review the fix.\n   - who: you\n5.", "4.", 1))
	r := Revise(was, removed, states)
	if len(r.Yours) != 1 || r.Yours[0] != "removes the checkpoint at step 4 (Review the fix)" {
		t.Errorf("removed: %q", r.Yours)
	}
	if r.Why() != "It removes the checkpoint at step 4 (Review the fix), which is the user's to confirm." {
		t.Errorf("why = %q", r.Why())
	}

	// After the pull request's opened, not before.
	moved := mustPlan(t, strings.Replace(fixPlan, "4. Review the fix.\n   - who: you\n5. Open a pull request.\n   - who: crew, in `api/fix`\n   - makes: a pull request\n", "4. Open a pull request.\n   - who: crew, in `api/fix`\n   - makes: a pull request\n5. Review the fix.\n   - who: you\n", 1))
	r = Revise(was, moved, states)
	if len(r.Yours) != 1 || r.Yours[0] != "moves the checkpoint at step 4 (Review the fix) to step 5" {
		t.Errorf("moved: %q", r.Yours)
	}

	// Made crew's: it's no longer a checkpoint.
	r = Revise(was, mustPlan(t, strings.Replace(fixPlan, "4. Review the fix.\n   - who: you", "4. Review the fix.\n   - who: crew", 1)), states)
	if len(r.Yours) != 1 || !strings.HasPrefix(r.Yours[0], "removes the checkpoint at step 4") || !strings.Contains(r.Summary(), "step 4 changed (Review the fix)") {
		t.Errorf("made crew's: %q, %q", r.Yours, r.Summary())
	}

	z := mustPlan(t, strings.Replace(fixPlan, "- Merged, with its regression test.", "- Merged, with its regression test.\n- Product told.", 1))
	r = Revise(was, z, states)
	if len(r.Yours) != 1 || r.Yours[0] != "changes Z" || r.Summary() != "Z changed" {
		t.Errorf("Z: %q, %q", r.Yours, r.Summary())
	}

}

// The steps behind the mission stay as they were: a revision that changes
// one, or drops it, is refused, saying which.
func TestRevise_StepsBehind(t *testing.T) {
	was := mustPlan(t, fixPlan)
	reworded := mustPlan(t, strings.Replace(fixPlan, "Reproduce it as a failing test.", "Reproduce it.", 1))
	r := Revise(was, reworded, progress(was, 1, false))
	if r.Problem() != "a revision changes only what's ahead, and the steps behind the mission stay as they were: step 1 (Reproduce it as a failing test) is passed, and it isn't as it was" {
		t.Errorf("reworded a passed step: %q", r.Problem())
	}
	// Not begun, the step the mission's at may change.
	if r := Revise(was, reworded, progress(was, 0, false)); r.Problem() != "" {
		t.Errorf("step 1 not begun: %q", r.Problem())
	}
	// Begun, it may not.
	if r := Revise(was, reworded, progress(was, 0, true)); !strings.Contains(r.Problem(), "step 1 (Reproduce it as a failing test) is the one the mission is at, and it isn't as it was") {
		t.Errorf("step 1 begun: %q", r.Problem())
	}
	short := mustPlan(t, "## Z\n- Merged.\n## Stops early\n- None.\n## Steps\n1. Reproduce it as a failing test.\n   - who: crew, in `api/fix`\n   - makes: the failing test, and the cause\n")
	if r := Revise(was, short, progress(was, 2, false)); !strings.Contains(r.Problem(), "step 2 (Is this the cause?) is passed, and it's gone") {
		t.Errorf("dropped a passed step: %q", r.Problem())
	}
	// Who's on a passed step is part of it.
	if r := Revise(was, mustPlan(t, strings.Replace(fixPlan, "   - who: crew, in `api/fix`\n   - makes: the failing test", "   - who: crew, in `api/other`\n   - makes: the failing test", 1)), progress(was, 1, false)); r.Problem() == "" {
		t.Error("moved a passed step's crew")
	}
}

// Adding a checkpoint, or changing where it may stop early, isn't the
// user's: more checking, not less.
func TestRevise_AddingACheckpoint(t *testing.T) {
	was := mustPlan(t, fixPlan)
	now := mustPlan(t, strings.Replace(strings.Replace(fixPlan, "- It can't be reproduced.", "- It can't be reproduced, or isn't a bug.", 1),
		"5. Open a pull request.", "5. Is it ready for a pull request?\n   - who: you\n6. Open a pull request.", 1))
	r := Revise(was, now, progress(was, 2, false))
	if len(r.Yours) != 0 || r.Problem() != "" {
		t.Fatalf("yours %q, held %q", r.Yours, r.Held)
	}
	if !strings.Contains(r.Summary(), "where it stops early changed") || !strings.Contains(r.Summary(), "step 5 added (Is it ready for a pull request?, you)") {
		t.Errorf("summary = %q", r.Summary())
	}
}

// A command a revision has ws run that the plan didn't is the user's to
// see first: ws runs what's in a check without asking anyone.
func TestRevise_ACommandAddedIsYours(t *testing.T) {
	head := "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: find it.\n   - check: `go test ./...` fails\n"
	was := mustPlan(t, head+"2. Plan\n")
	same := mustPlan(t, head+"2. Plan\n3. Work: fix it.\n   - check: `go test ./...` passes\n   - check: it fixes the cause\n")
	if r := Revise(was, same, progress(was, 1, false)); len(r.Yours) != 0 {
		t.Errorf("a command the plan had, again: %v", r.Yours)
	}
	more := mustPlan(t, head+"2. Plan\n3. Work: fix it.\n   - check: `rm -rf build && make` passes\n")
	r := Revise(was, more, progress(was, 1, false))
	if len(r.Yours) != 1 || !strings.Contains(r.Yours[0], "adds a command ws runs at step 3 (`rm -rf build && make`)") {
		t.Errorf("a new command: %v", r.Yours)
	}
	// A step's checks are part of it: one behind can't have them changed.
	changed := mustPlan(t, "## Z\n- Fixed.\n## Stops early\n- None.\n## Steps\n1. Work: find it.\n   - check: `go test ./...` passes\n2. Plan\n")
	if r := Revise(was, changed, progress(was, 1, false)); r.Problem() == "" {
		t.Error("changed the checks of a step passed")
	}
}
