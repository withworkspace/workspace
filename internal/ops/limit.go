package ops

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

// Work's expansion, its limit: a Work step that says how long its crew
// have (- limit:) has a clock ws holds, from when crew start on it. Crew
// may set themselves less time; the orchestrator and you may give it
// more. When it runs out while crew are still at it, ws says so, once: an
// event for the orchestrator, whose call it is by the process (more time,
// wrap up, or yours), or with the orchestrator off, a task of yours.

// defaultMoreTime is more time for a step that has no limit of its own.
const defaultMoreTime = 30 * time.Minute

// MoreTime sets how long step n (0: the one the mission's at) has from
// now: d, or with d 0, its limit again. Crew can only give themselves less
// time than they had. It says when it runs out.
func (w *Workspace) MoreTime(slug string, n int, d time.Duration, by Actor) (string, error) {
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	st, err := mission.PickStage(states, n)
	if err != nil {
		return "", invalid("%v", err)
	}
	if st.Accepted() {
		return "", refused("passed", "%s %d has already passed", st.Noun(), st.N)
	}
	if d <= 0 {
		d = defaultMoreTime
		if st.Step != nil && st.Step.Limit > 0 {
			d = st.Step.Limit
		}
	}
	shorterOnly := by.Kind == Crew
	if shorterOnly && (st.Record == nil || !slices.Contains(st.Record.Crew, by.ID)) {
		return "", refused("not-yours", "crew set the time of the step they're on: you aren't on %s %d", st.Noun(), st.N)
	}
	deadline, err := w.Missions.SetLimit(m.Slug, st.N, d, by.By(), shorterOnly)
	if err != nil {
		return "", wrap(err)
	}
	// What ws asked about the step's time is answered.
	if t, ok := m.StepTask(st.N); ok && t.Verb == mission.TaskTime {
		logFailed(w.Missions.CloseTask(m.Slug, t.N, by.By(), ""))
	}
	w.youActed(m.Slug, by)
	left := time.Until(deadline).Round(time.Minute)
	return fmt.Sprintf("%s %d's time runs out at %s (in %s)", st.Noun(), st.N, deadline.Local().Format("15:04"), mission.FormatLimit(max(left, time.Minute))), nil
}

// wrapUp tells the crew still at step n that its time is up: commit what
// they have, and report done with what they found and what's left. It
// says what it did.
func (w *Workspace) wrapUp(slug string, n int, by Actor) string {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return "could not read the mission: " + err.Error()
	}
	brief, _ := w.Missions.ReadBrief(slug)
	states, _ := m.StageProgress(brief)
	st, err := mission.PickStage(states, n)
	if err != nil {
		return err.Error()
	}
	text := fmt.Sprintf("Your time on step %d is up. Wrap up now: commit what you have, write what you found and what's left into your handoff, and report done (ws mission done).", n)
	var told, missed []string
	for _, id := range workingOn(m, st.Record) {
		if _, err := w.Say(slug, id, text, by); err != nil {
			missed = append(missed, id)
		} else {
			told = append(told, id)
		}
	}
	said := ""
	if len(told) > 0 {
		said = fmt.Sprintf("told %s to wrap up step %d and report done", strings.Join(told, ", "), n)
	}
	switch {
	case len(missed) > 0 && said != "":
		said += fmt.Sprintf("; could not reach %s (tell them on their page)", strings.Join(missed, ", "))
	case len(missed) > 0:
		said = fmt.Sprintf("could not reach %s to wrap up step %d: tell them on their page", strings.Join(missed, ", "), n)
	case said == "":
		said = fmt.Sprintf("nobody at step %d to wrap up", n)
	}
	return said
}

// workingOn is the crew still at work on a step (its record): started on
// it, not ended, and not reported done.
func workingOn(m *mission.Mission, r *mission.StageRecord) []string {
	if r == nil {
		return nil
	}
	var out []string
	for _, id := range r.Crew {
		if c, ok := m.CrewByID(id); ok && c.Role == "" && c.Ended == nil && c.Status != mission.CrewDone {
			out = append(out, id)
		}
	}
	return out
}

// CheckLimits says, once, of each mission in play whose step's time has
// run out while crew are still at it: the orchestrator's call, or yours.
// The daemon calls it every minute. It returns what it said, a line each.
func (w *Workspace) CheckLimits() []string {
	missions, err := w.Missions.InPlay()
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range missions {
		if said := w.checkLimit(m); said != "" {
			out = append(out, m.Slug+": "+said)
		}
	}
	return out
}

func (w *Workspace) checkLimit(m *mission.Mission) string {
	brief, _ := w.Missions.ReadBrief(m.Slug)
	o, ok := m.RanOutOfTime(brief, w.now())
	if !ok {
		return ""
	}
	crew := workingOn(m, o.Step.Record)
	if len(crew) == 0 {
		return "" // nobody's at it: nothing to run out
	}
	if marked, err := w.Missions.MarkOutOfTime(m.Slug, o.Step.N, w.now()); err != nil || !marked {
		return ""
	}
	limit := " (at " + o.Deadline.Local().Format("15:04") + ")"
	if w.Notify(m.Slug, mission.OrchestratorEvent{Trigger: "limit"}) {
		return fmt.Sprintf("step %d is out of time%s: the orchestrator decides what follows, by the process", o.Step.N, limit)
	}
	are := " are"
	if len(crew) == 1 {
		are = " is"
	}
	text := fmt.Sprintf("Step %d is out of time%s, and %s still working. Give more time, or have them wrap up and report done.", o.Step.N, limit, strings.Join(crew, ", ")+are)
	if _, err := w.Missions.AskAtStep(m.Slug, mission.TaskTime, text, o.Step.N, mission.TimeMore, mission.TimeWrapUp); err != nil {
		return "couldn't ask the user: " + err.Error()
	}
	return fmt.Sprintf("step %d is out of time%s: the user's asked", o.Step.N, limit)
}
