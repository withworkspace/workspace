package ops

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/spend"
)

// The decision look: once crew have recorded decisions and gone quiet a
// moment (DecisionQuiet, so a run of them is looked at once), a small
// model reads them beside their step's checks and the user's own
// decisions, and says of each whether it goes against one. What it finds
// goes to the crew who made it, while it's theirs to put right: a
// decision against a check otherwise comes to light a round of checks
// later. It's the judge's model, and costs a cent or two a look.

// DecisionQuiet is how long after crew's last decision on a mission the
// decision look waits, so decisions made in a run are looked at together.
const DecisionQuiet = 45 * time.Second

// decisionLookTimeout is how long the look may take.
const decisionLookTimeout = 3 * time.Minute

// crewDecisions are the decisions on m crew made that stand and the look
// hasn't looked at, oldest first.
func crewDecisions(m *mission.Mission, all []mission.Decision) []mission.Decision {
	var out []mission.Decision
	for _, d := range all {
		if d.State != mission.DecisionCurrent || mission.DecisionNumber(d.ID) <= m.DecisionsLooked {
			continue
		}
		switch d.By {
		case AsYou.By(), AsOrchestrator.By(), AsWS.By(), "":
			continue
		}
		out = append(out, d)
	}
	return out
}

// DecisionsDue are the missions in play whose crew have decisions the look
// hasn't looked at, the latest DecisionQuiet or more before now.
func (w *Workspace) DecisionsDue(now time.Time) []string {
	missions, err := w.Missions.InPlay()
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range missions {
		all, err := w.Missions.Decisions(m.Slug)
		if err != nil {
			continue
		}
		ds := crewDecisions(m, all)
		if len(ds) > 0 && now.Sub(ds[len(ds)-1].At) >= DecisionQuiet {
			out = append(out, m.Slug)
		}
	}
	return out
}

// LookAtDecisions is the decision look on a mission's crew decisions it
// hasn't looked at: by step, each step's with its checks and the user's
// decisions. It tells the crew of each that goes against one, and says
// what it did ("" with nothing to look at).
func (w *Workspace) LookAtDecisions(slug string) (string, error) {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return "", err
	}
	all, err := w.Missions.Decisions(slug)
	if err != nil {
		return "", err
	}
	ds := crewDecisions(m, all)
	if len(ds) == 0 {
		return "", nil
	}
	last := mission.DecisionNumber(ds[len(ds)-1].ID)
	// Looked at, whatever comes of it: a look that fails isn't run again
	// and again on the same decisions.
	defer func() { logFailed(w.Missions.SetDecisionsLooked(slug, last)) }()

	var yours []string
	for _, d := range all {
		if d.State == mission.DecisionCurrent && d.By == AsYou.By() {
			yours = append(yours, decisionLine(d))
		}
	}
	brief, _ := w.Missions.ReadBrief(slug)
	states, _ := m.StageProgress(brief)
	var said []string
	for _, step := range decisionSteps(ds) {
		var at []mission.Decision
		for _, d := range ds {
			if d.Step == step {
				at = append(at, d)
			}
		}
		l := prompt.DecisionLook{Mission: m.Slug, Title: m.Title, Goal: briefGoal(brief), Yours: yours}
		if st, err := mission.PickStage(states, step); step > 0 && err == nil && st.Step != nil {
			l.Step = fmt.Sprintf("%d: %s: %s", st.N, st.Step.Type, st.Step.Text)
			for _, c := range st.Step.Checks {
				l.Checks = append(l.Checks, c.Text)
			}
		}
		if len(l.Checks) == 0 && len(yours) == 0 {
			continue // nothing they're held to
		}
		ids := make([]string, len(at))
		for i, d := range at {
			ids[i] = d.ID
			l.Decisions = append(l.Decisions, fmt.Sprintf("%s (%s): %s", d.ID, d.By, strings.TrimPrefix(decisionLine(d), d.ID+": ")))
		}
		found, err := w.askDecisionLook(m.Slug, l, ids)
		if err != nil {
			execlog.Note(w.Root(), "decisions "+m.Slug, "couldn't look at %s: %v", strings.Join(ids, ", "), err)
			continue
		}
		for i, f := range found {
			if f.Against == "" {
				continue
			}
			said = append(said, w.tellOfDecision(m, at[i], f, l.Checks))
		}
	}
	if len(said) == 0 {
		return fmt.Sprintf("looked at %s: they fit", decisionSpan(ds)), nil
	}
	return strings.Join(said, "; "), nil
}

// decisionSteps are the steps ds were made at, in order.
func decisionSteps(ds []mission.Decision) []int {
	var out []int
	for _, d := range ds {
		if !slices.Contains(out, d.Step) {
			out = append(out, d.Step)
		}
	}
	slices.Sort(out)
	return out
}

// decisionLine is a decision in a line: its id, what, and why.
func decisionLine(d mission.Decision) string {
	line := fmt.Sprintf("%s: %s", d.ID, sentence(oneLine(d.What)))
	if why := oneLine(d.Reason()); why != "" {
		line += " Why: " + sentence(clipString(why, 400))
	}
	return line
}

// decisionSpan is ds's ids in words: "d3", "d3 to d5".
func decisionSpan(ds []mission.Decision) string {
	if len(ds) == 1 {
		return ds[0].ID
	}
	return ds[0].ID + " to " + ds[len(ds)-1].ID
}

// askDecisionLook puts l to the judge's agent, headless and read-only, and
// reads what it said of each of ids.
func (w *Workspace) askDecisionLook(slug string, l prompt.DecisionLook, ids []string) ([]prompt.DecisionFinding, error) {
	pick, h, err := Roles(w.Config()).ForJudge()
	if err != nil {
		return nil, err
	}
	run := harness.ConverseRun{Prompt: prompt.LookAtDecisions(l), Dir: w.Root(), Model: pick.Model, Once: true}
	runPath := filepath.Join(w.Missions.Dir(), slug, "decisions.log")
	rep, err := RunHeadless(w.life(), w.Root(), "decisions "+slug, spend.For{Mission: slug, Kind: spend.Decisions, Subject: strings.Join(ids, " ")},
		crew.Env(w.Root(), slug, ""), runPath, decisionLookTimeout, h, run)
	if err != nil {
		return nil, err
	}
	return prompt.ReadDecisionFindings(rep.Output, ids), nil
}

// tellOfDecision tells the crew who made d what the look found it goes
// against: typed in while they're running, kept on the mission as a note
// where they've ended (and, where it's against your own decision, put to
// you, as they'd have had to ask). It says what it did.
func (w *Workspace) tellOfDecision(m *mission.Mission, d mission.Decision, f prompt.DecisionFinding, checks []string) string {
	against := f.Against
	if n, ok := strings.CutPrefix(f.Against, "check "); ok {
		var i int
		if _, err := fmt.Sscan(n, &i); err == nil && i >= 1 && i <= len(checks) {
			against = fmt.Sprintf("your step's check %d (%q)", i, clipString(oneLine(checks[i-1]), 160))
		}
	} else {
		against = fmt.Sprintf("the user's %s", f.Against)
	}
	why := ""
	if f.Why != "" {
		why = " " + sentence(f.Why)
	}
	text := fmt.Sprintf("ws looked at %s beside what your work is held to: it may go against %s.%s If it does, put it right, or replace it (ws mission decide --supersedes %s); if it's what's held to that's wrong, ask the user (ws mission ask). If it doesn't, carry on: this was a quick look, and can be wrong.", d.ID, against, why, d.ID)
	if !strings.HasPrefix(f.Against, "check ") {
		text = fmt.Sprintf("ws looked at %s beside what your work is held to: it may go against %s, which stands over yours.%s Don't go on against it without asking the user (ws mission ask). If it doesn't, carry on: this was a quick look, and can be wrong.", d.ID, against, why)
	}
	note := fmt.Sprintf("%s may go against %s.%s", d.ID, against, why)
	if c, ok := m.CrewByID(d.By); ok && c.Ended == nil {
		if _, err := w.Say(m.Slug, c.ID, text, AsWS); err == nil {
			w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventNote, Text: "Decision look: " + note + " Told " + c.ID + "."}) //nolint:errcheck // a note
			return fmt.Sprintf("told %s: %s", c.ID, note)
		}
	}
	w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventNote, Text: "Decision look: " + note}) //nolint:errcheck // a note
	if !strings.HasPrefix(f.Against, "check ") {
		if _, err := w.Flag(m.Slug, fmt.Sprintf("%s, by %s, who've ended, may go against your %s.%s Say whether it stands.", d.ID, d.By, f.Against, why), AsWS); err == nil {
			return "flagged you: " + note
		}
	}
	return "noted: " + note
}
