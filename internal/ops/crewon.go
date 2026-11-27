package ops

import (
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// What crew run on. [crew] allowed is what they may (a guarantee), the
// default first: a plan's step names one in its on:, the orchestrator in
// a dispatch's --on (as the process prefers: "bumps on the cheapest
// model"), and ws refuses anything else, whoever asks, saying what's
// allowed.

// CrewPick is what crew run on, as a plan's on: (or a dispatch's --on)
// says it: harness:model, a harness alone, or a model alone, on crew's
// harness; "" for crew's own.
func (w *Workspace) CrewPick(on string) harness.Pick {
	roles := Roles(w.Config())
	on = strings.TrimSpace(on)
	if on == "" {
		return roles.ForCrew()
	}
	if name, model, _ := strings.Cut(on, ":"); name != "" {
		if _, err := harness.Get(name); err == nil {
			return harness.Pick{Harness: name, Model: model}
		}
	}
	return harness.Pick{Harness: roles.ForCrew().Harness, Model: on}
}

// allowsCrew refuses crew on a pick [crew] allowed doesn't list.
func (w *Workspace) allowsCrew(p harness.Pick) error {
	if Roles(w.Config()).AllowsCrew(p) {
		return nil
	}
	return refused("not-allowed", "crew cannot run on %s: not in [crew] allowed (allowed: %s)", p, crewOnList(w))
}

// crewOnList is what crew may run on, in words, the default first.
func crewOnList(w *Workspace) string {
	var picks []string
	for i, p := range Roles(w.Config()).CrewPicks() {
		s := p.String()
		if i == 0 {
			s += " (default)"
		}
		picks = append(picks, s)
	}
	return strings.Join(picks, ", ")
}

// guaranteeCrewAllowed is what crew may run on: always held.
func guaranteeCrewAllowed(w *Workspace) Guarantee {
	return Guarantee{"crew.allowed", fmt.Sprintf("crew run only on what [crew] allowed lists: %s. A plan's on: names one of them, or none, for the default", crewOnList(w))}
}

// stepPick is what a step's crew run on: its on:, where crew may run on
// it, else crew's own (and why, for a plan from before ws checked).
func (w *Workspace) stepPick(st mission.StageState, on string) (harness.Pick, string) {
	if on == "" && st.Step != nil {
		on = st.Step.Model
	}
	if on == "" {
		return harness.Pick{}, ""
	}
	p := w.CrewPick(on)
	if err := w.allowsCrew(p); err != nil {
		return harness.Pick{}, fmt.Sprintf(" (on crew default: %s not in [crew] allowed)", on)
	}
	return p, ""
}

// planPicks refuses a plan whose steps name what crew can't run on, every
// one with its line.
func (w *Workspace) planPicks(p *mission.Plan) error {
	var problems []string
	for _, st := range p.Steps {
		if st.Model == "" {
			continue
		}
		if err := w.allowsCrew(w.CrewPick(st.Model)); err != nil {
			problems = append(problems, fmt.Sprintf("line %d: step %d's on: %s is not in [crew] allowed (%s): name one of those, or leave on: out for the default", st.Line, st.N, st.Model, crewOnList(w)))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return &Invalid{Reason: (&mission.MalformedPlan{Problems: problems}).Error()}
}
