package mission

import (
	"fmt"
	"strings"
)

// Revising a plan: writing it again once the mission is under way. The
// planner revises it (at a step of its own, when the orchestrator says the
// route doesn't fit, or when you ask), and you do, editing it. A revision
// changes only what's ahead: the steps passed, and the one the mission is
// at once it's begun, stay as they were. Removing or moving a checkpoint,
// changing Z, and a check's command ws hasn't been given to run before are
// yours to confirm; anything else applies, and the log says what changed.

// Revision is how a plan written again differs from the one it replaces,
// read against how far the mission has got along it.
type Revision struct {
	// Held is what it changes of the steps behind the mission (passed, or
	// the one it's at, begun): a revision that has any is refused.
	Held []string
	// Yours is what makes it the user's to confirm: Z changed, a
	// checkpoint ahead removed or moved, a command added for ws to run.
	Yours []string
	// Changes is what it changes, for the log: steps added, removed and
	// changed, Z and where it stops early.
	Changes []string
	// Fixed is how many of its first steps stay as they were.
	Fixed int
}

// Changed reports whether it changes anything.
func (r Revision) Changed() bool { return len(r.Changes) > 0 }

// Problem is why a revision is refused, or "".
func (r Revision) Problem() string {
	if len(r.Held) == 0 {
		return ""
	}
	return "a revision changes only what's ahead, and the steps behind the mission stay as they were: " + strings.Join(r.Held, "; ")
}

// Why is what makes it the user's, in a sentence, or "".
func (r Revision) Why() string {
	if len(r.Yours) == 0 {
		return ""
	}
	return "It " + joinAnd(r.Yours) + ", which is the user's to confirm."
}

// Summary is what it changes, in a line, for the log.
func (r Revision) Summary() string {
	if len(r.Changes) == 0 {
		return "nothing changed"
	}
	return strings.Join(r.Changes, "; ")
}

// FixedSteps is how many of a plan's first steps a revision keeps as they
// were: those passed, and the one the mission is at once it's begun (crew
// have started on it), or when it's the planner's own (it plans what's
// after it).
func FixedSteps(states []StageState) int {
	n := 0
	for _, st := range states {
		if st.Accepted() {
			n++
			continue
		}
		if st.Current && (st.Record != nil && (st.Record.Started != nil || len(st.Record.Crew) > 0) || st.Who() == WhoPlanner) {
			n++
		}
		break
	}
	return n
}

// Revise reads a plan written again (now) against the one it replaces
// (was) and how far the mission's got along it (states, from was).
func Revise(was, now *Plan, states []StageState) Revision {
	var r Revision
	if was == nil {
		return Revision{Changes: []string{fmt.Sprintf("written: %d %s", len(now.Steps), pluralWord(len(now.Steps), "step"))}}
	}
	r.Fixed = min(FixedSteps(states), len(was.Steps))
	for i := 0; i < r.Fixed; i++ {
		old := was.Steps[i]
		switch {
		case i >= len(now.Steps):
			r.Held = append(r.Held, fmt.Sprintf("step %d (%s) is %s, and it's gone", old.N, stepShort(old), behindWord(states, i)))
		case !sameStep(old, now.Steps[i]):
			r.Held = append(r.Held, fmt.Sprintf("step %d (%s) is %s, and it isn't as it was", old.N, stepShort(old), behindWord(states, i)))
		}
	}
	if !sameList(was.Z, now.Z) {
		r.Yours = append(r.Yours, "changes Z")
		r.Changes = append(r.Changes, "Z changed")
	}
	if !sameList(was.Early, now.Early) {
		r.Changes = append(r.Changes, "where it stops early changed")
	}
	ahead := func(p *Plan) []Step {
		if r.Fixed >= len(p.Steps) {
			return nil
		}
		return p.Steps[r.Fixed:]
	}
	olds, news := ahead(was), ahead(now)
	at := matchSteps(olds, news)
	matched := make([]bool, len(news))
	for _, j := range at {
		if j >= 0 {
			matched[j] = true
		}
	}
	// Checkpoints ahead: removed, or moved among the steps that stay.
	for i, old := range olds {
		if !old.Checkpoint() {
			continue
		}
		j := at[i]
		if j < 0 || !news[j].Checkpoint() {
			r.Yours = append(r.Yours, fmt.Sprintf("removes the checkpoint at step %d (%s)", old.N, stepShort(old)))
			continue
		}
		for k, l := range at {
			if k != i && l >= 0 && (k < i) != (l < j) {
				r.Yours = append(r.Yours, fmt.Sprintf("moves the checkpoint at step %d (%s) to step %d", old.N, stepShort(old), news[j].N))
				break
			}
		}
	}
	// A command ws would run that the plan didn't have: the user sees it
	// before ws runs it.
	had := commands(was)
	for _, st := range news {
		for _, c := range st.Checks {
			if c.Run() && !had[c.Command] {
				r.Yours = append(r.Yours, fmt.Sprintf("adds a command ws runs at step %d (`%s`)", st.N, c.Command))
				had[c.Command] = true
			}
		}
	}
	for i, old := range olds {
		if j := at[i]; j < 0 {
			r.Changes = append(r.Changes, fmt.Sprintf("step %d removed (%s)", old.N, stepShort(old)))
		} else if !sameStep(old, news[j]) {
			r.Changes = append(r.Changes, fmt.Sprintf("step %d changed (%s)", news[j].N, stepShort(news[j])))
		} else if old.N != news[j].N {
			r.Changes = append(r.Changes, fmt.Sprintf("step %d is step %d now (%s)", old.N, news[j].N, stepShort(old)))
		}
	}
	for j, st := range news {
		if !matched[j] {
			r.Changes = append(r.Changes, fmt.Sprintf("step %d added (%s, %s)", st.N, stepShort(st), st.WhoSays()))
		}
	}
	return r
}

// matchSteps pairs each step of olds with the step of news it is, by its
// text (and who's on it, failing that by text alone), keeping their order:
// at[i] is olds[i]'s index in news, or -1 if it's gone.
func matchSteps(olds, news []Step) []int {
	at := make([]int, len(olds))
	used := make([]bool, len(news))
	for i := range at {
		at[i] = -1
	}
	pass := func(same func(a, b Step) bool) {
		for i, old := range olds {
			if at[i] >= 0 {
				continue
			}
			for j, st := range news {
				if !used[j] && same(old, st) {
					at[i], used[j] = j, true
					break
				}
			}
		}
	}
	pass(func(a, b Step) bool { return normStage(a.Text) == normStage(b.Text) && a.Who == b.Who })
	pass(func(a, b Step) bool { return normStage(a.Text) == normStage(b.Text) })
	return at
}

// sameStep reports whether two steps say the same: its kind, what's done,
// its quote, who's on it, and its parameters.
func sameStep(a, b Step) bool {
	return a.Type == b.Type && normStage(a.Text) == normStage(b.Text) && a.Quote == b.Quote && a.Who == b.Who &&
		a.Capsule == b.Capsule && a.Model == b.Model && a.Whom == b.Whom && normStage(a.Makes) == normStage(b.Makes) &&
		normStage(a.From) == normStage(b.From) && a.Draft == b.Draft && sameChecks(a.Checks, b.Checks) &&
		a.CarryOn == b.CarryOn && a.Fresh == b.Fresh && normStage(a.LimitText) == normStage(b.LimitText) &&
		normStage(a.OnlyIf) == normStage(b.OnlyIf) && normStage(a.InEach) == normStage(b.InEach)
}

func sameChecks(a, b []Check) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if normStage(a[i].Text) != normStage(b[i].Text) || a[i].Yours != b[i].Yours {
			return false
		}
	}
	return true
}

// commands are the commands a plan's checks run.
func commands(p *Plan) map[string]bool {
	out := map[string]bool{}
	for _, st := range p.Steps {
		for _, c := range st.Checks {
			if c.Run() {
				out[c.Command] = true
			}
		}
	}
	return out
}

func sameList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if normStage(a[i]) != normStage(b[i]) {
			return false
		}
	}
	return true
}

// behindWord is how far the mission's got with the step at i: passed, or
// the one it's at.
func behindWord(states []StageState, i int) string {
	if i < len(states) && states[i].Accepted() {
		return "passed"
	}
	return "the one the mission is at"
}

// stepShort is a step in a few words: a question stays one.
func stepShort(st Step) string {
	s := Stage{N: st.N, Text: st.Text}
	short := s.Short()
	if strings.HasPrefix(s.Plain(), short+"?") {
		short += "?"
	}
	return short
}

// joinAnd joins phrases with commas, and "and" before the last.
func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func pluralWord(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
