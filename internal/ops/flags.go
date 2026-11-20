package ops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
)

// Flagged is a question put to the user on a mission, and what came of
// asking it.
type Flagged struct {
	Slug string
	// Task is the task it opened: a question, or the step's, its checks
	// the user's to say. Zero when there was nothing left to ask (Passed),
	// or the asker's question was taken back.
	Task mission.Task
	// AtStep is the step whose task it became, its checks not all met; 0
	// for a plain question.
	AtStep int
	// Passed is what ws did instead of asking: the user had approved what
	// shows the step's checks already, so it passed. "" when it asked.
	Passed string
}

// Flag asks the user a question on a mission, for whoever asks (the
// orchestrator, crew, or you), with the answers it suggests; "" takes
// theirs back.
//
// The orchestrator's, at a Work step whose checks came round not all met
// and whose crew have left it there, is that step's own: ws passes such a
// step only once every check is met, the orchestrator can't accept it, and
// the planner can't change a step that's begun, so a plain question would
// send the user's answer back to an orchestrator that can't act on it. As
// the step's, the user can pass it as it is from the question, or send it
// back, and approving the bundle that shows its checks passes it; where
// they've approved that already, it passes here, with nothing to ask.
func (w *Workspace) Flag(slug, text string, by Actor, options ...string) (Flagged, error) {
	m, err := w.load(slug)
	if err != nil {
		return Flagged{}, err
	}
	out := Flagged{Slug: m.Slug}
	text = strings.TrimSpace(text)
	if by.Kind == Orchestrator && text != "" {
		brief, _ := w.Missions.ReadBrief(m.Slug)
		if st, round, ok := m.ChecksUnmet(brief); ok {
			if said := w.passApproved(m.Slug, st.N, round); said != "" {
				out.Passed = said
				return out, nil
			}
			// Its question, if it has one open, is this one now.
			w.Missions.Ask(m.Slug, "", by.By())
			met := len(round.With(mission.CheckMet))
			text = fmt.Sprintf("%s Step %d's checks, round %d, came to %d of %d met. %sass it as it is, send it back to %s with what's unmet, or say what to do.",
				sentence(text), st.N, round.N, met, len(round.Results), w.checksShownBy(m.Slug, st.N, round.Crew), round.Crew)
			t, err := w.Missions.OpenTask(m.Slug, mission.Task{Verb: mission.TaskChecks, Text: text, Step: st.N, By: by.By(),
				Options: checksOptions(options)})
			if err != nil {
				return Flagged{}, wrap(err)
			}
			out.Task, out.AtStep = t, st.N
			return out, nil
		}
	}
	t, err := w.Missions.Ask(m.Slug, text, by.By(), options...)
	if err != nil {
		return Flagged{}, wrap(err)
	}
	out.Task = t
	return out, nil
}

// checksOptions is the answers a step's checks suggest (pass it, send it
// back), then the asker's own that aren't those.
func checksOptions(own []string) []string {
	out := []string{mission.ChecksPass, mission.ChecksSendBack}
	for _, o := range own {
		o = strings.TrimSpace(o)
		if o == "" || slices.ContainsFunc(out, func(x string) bool { return strings.EqualFold(x, o) }) {
			continue
		}
		out = append(out, o)
	}
	return out
}
