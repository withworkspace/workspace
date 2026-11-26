package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/orchestrator"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/turn"
)

// OrchestratorRun is a run of the orchestrator asked for: the mission, the
// crew whose event prompted it and the event (trigger), and how it's run.
type OrchestratorRun struct {
	Slug, Crew, Trigger, Model string
	// Auto is ws's own run, for an event: it honours [orchestrator]
	// enabled, and handles each crew event once.
	Auto bool
	// Since is when the run was marked underway as it was asked for (a
	// job's), for it to end that mark; zero for a run by hand, which marks
	// itself.
	Since time.Time
	// Said is told what the run says as it goes: that it's looking, and
	// what the orchestrator said. nil for nobody.
	Said func(line string)
}

func (r OrchestratorRun) say(format string, a ...any) {
	if r.Said != nil {
		r.Said(fmt.Sprintf(format, a...))
	}
}

// OrchestratorPrompt is what the orchestrator would be told on a run, and
// the command it would be run with (its prompt in file), run no further.
func (w *Workspace) OrchestratorPrompt(r OrchestratorRun) (prompt string, args []string, file string, err error) {
	m, rv, run, in, err := w.orchestratorSetup(&r)
	if err != nil {
		return "", nil, "", err
	}
	l, err := rv.Orchestrate(run)
	if err != nil {
		return "", nil, "", err
	}
	return orchestrator.Prompt(in), l.Args, orchestrator.PromptFile(w.Missions, m.Slug, "<run>"), nil
}

// orchestratorSetup is what a run needs: its mission, the harness and its
// run, and what it's told. r's Model is filled in.
func (w *Workspace) orchestratorSetup(r *OrchestratorRun) (*mission.Mission, harness.Orchestrator, harness.OrchestratorRun, orchestrator.Input, error) {
	var none harness.OrchestratorRun
	m, err := w.load(r.Slug)
	if err != nil {
		return nil, nil, none, orchestrator.Input{}, err
	}
	if r.Crew != "" {
		if _, ok := m.CrewByID(r.Crew); !ok {
			return nil, nil, none, orchestrator.Input{}, fmt.Errorf("%w: mission %s has no crew %s", ErrNotFound, m.Slug, r.Crew)
		}
	}
	pick, rv, err := Roles(w.Config()).ForOrchestrator()
	if err != nil {
		return nil, nil, none, orchestrator.Input{}, err
	}
	if r.Model == "" {
		r.Model = pick.Model
	}
	in := w.orchestratorInput(m, r.Crew, r.Trigger)
	run := harness.OrchestratorRun{Prompt: "<a line pointing at the prompt file>", Dir: w.WS().Root, Model: r.Model,
		Scratch: filepath.Join(w.WS().Root, ".ws", "converse")}
	for _, c := range in.Capsules {
		if c.Exists {
			run.Capsules = append(run.Capsules, c.Path)
		}
	}
	for _, g := range Grounds(w.WS()) {
		run.Grounds = append(run.Grounds, g.Path)
	}
	return m, rv, run, in, nil
}

// Orchestrate runs the orchestrator on a mission: it looks at what's
// happened and acts through ws. A run that ran no ws mission command is
// run once more, told its shell works; one that still does nothing (or
// couldn't run) puts the mission in your court, with what it said.
func (w *Workspace) Orchestrate(r OrchestratorRun) error {
	// The run marked as underway when it was asked for ends however
	// this finishes, but for ws going first: still marked, it's run again
	// by the daemon after (reconcileJobs).
	var ended bool
	if !r.Since.IsZero() && r.Slug != "" {
		defer func() {
			if !ended {
				w.Missions.EndOrchestrating(r.Slug, mission.OrchestratorRun{Since: r.Since})
			}
		}()
	}
	if r.Auto && !w.Config().OrchestratorEnabled() {
		execlog.Note(w.WS().Root, "orchestrator "+r.Slug, "not run for %s: [orchestrator] enabled = false", strings.TrimSpace(r.Crew+" ("+r.Trigger+")"))
		return nil
	}
	m, rv, run, in, err := w.orchestratorSetup(&r)
	if err != nil {
		return err
	}
	store := w.Missions
	crewID, trigger, model := r.Crew, r.Trigger, r.Model
	prompt := orchestrator.Prompt(in)
	what := orchestrator.Describe(crewID, trigger)
	if r.Since.IsZero() { // run by hand: mark it underway here
		if run, err := store.StartOrchestrating(m.Slug, what, orchestrator.RunFor()); err == nil {
			defer store.EndOrchestrating(m.Slug, run)
		}
	}
	with := rv.Name() + " " + model
	r.say("orchestrator looking at %s on %s with %s…", what, m.Slug, with)

	// Each run is kept with the mission (orchestrator.RunRecord): its
	// prompt, what it said, and the calls it made. Its id is on the log
	// events it causes (Event.Run), which is how what it did is known.
	runOnce := func(prompt string, retry bool) (orchestrator.Report, bool, error) {
		rec := orchestrator.RunRecord{Started: time.Now(), For: what, Trigger: trigger, Crew: crewID,
			Harness: rv.Name(), Model: model, Retry: retry, Prompt: prompt}
		rec.ID = orchestrator.NewRunID(store, m.Slug, rec.Started)
		start := "looking at " + what + " with " + with
		if retry {
			start = "it ran no ws mission commands: running it again, told its shell works"
		}
		store.AppendLog(m.Slug, mission.Event{Kind: mission.EventOrchestrateRun, Crew: "orchestrator", Run: rec.ID, Text: start})
		var err error
		run.Prompt = prompt
		if run.Pointer, err = orchestrator.WritePrompt(store, m.Slug, rec.ID, prompt); err != nil {
			return orchestrator.Report{}, false, err
		}
		l, err := rv.Orchestrate(run)
		if err != nil {
			return orchestrator.Report{}, false, err
		}
		rep, runErr := orchestrator.Run(w.life(), l, rv.Name(), model, w.WS().Root, m.Slug, rec.ID)
		summary := orchestrator.Summary(rep.Output, runErr)
		store.AppendLog(m.Slug, mission.Event{Kind: mission.EventOrchestrate, Crew: "orchestrator", Run: rec.ID, Text: summary})
		r.say("%s", summary)
		acted := orchestratorActed(store, m.Slug, rec.ID)
		rec.Ended, rec.Output, rec.Calls, rec.Acted = time.Now(), rep.Output, rep.Calls, acted
		if runErr != nil {
			rec.Error = runErr.Error()
		}
		if err := orchestrator.SaveRun(store, m.Slug, &rec); err != nil {
			execlog.Note(w.WS().Root, "orchestrator "+m.Slug, "keeping the run: %v", err)
		}
		return rep, acted, runErr
	}
	rep, acted, runErr := runOnce(prompt, false)
	if errors.Is(runErr, turn.ErrEnded) {
		ended = true
		return runErr
	}
	// A run that ran no ws mission command did nothing, not even
	// its note: usually a model that took one refused command for
	// "the shell is denied". Once more, told otherwise.
	if runErr == nil && !acted {
		rep, acted, runErr = runOnce(prompt+orchestrator.RetryNote(rep), true)
	}
	// Still nothing, or it couldn't run: the ball would land
	// nowhere, so it's the user's, with what it said.
	if !acted && trigger != "manual" {
		orchestratorDidNothing(store, m.Slug, what, rep.Output, runErr)
	}
	// It had its turn, and suggested no ending for a mission whose plan
	// is done and work merged: ws ends it, landed, with Z unchecked.
	if r.Auto && runErr == nil {
		if c, why := w.endAtZ(m.Slug, r.Since); why == "" {
			r.say("%s", c)
		}
	}
	return runErr
}

// orchestratorInput gathers what the orchestrator is told: the capsules
// (diffs, PRs), the recent log, and the prompting crew's output.
func (w *Workspace) orchestratorInput(m *mission.Mission, crewID, trigger string) orchestrator.Input {
	store := w.Missions
	v := BuildMissionViews(w.WS(), w.GitHub(), []*mission.Mission{m})[0]
	brief, _ := store.ReadBrief(m.Slug)
	events, _ := store.ReadLog(m.Slug, orchestrator.LogEvents)
	in := orchestrator.Input{Mission: m, CrewID: crewID, Trigger: trigger, Brief: brief, Guide: AgentsNoteFor(w.WS(), RoleOrchestrator)}
	for _, c := range v.Capsules {
		oc := orchestrator.Capsule{Name: c.Repo + "/" + c.Name, Path: c.Path, Exists: c.Exists, Branch: c.Branch}
		if c.PR != nil {
			oc.PR, oc.PRMerged, oc.Checks, oc.URL = c.PR.Number, c.PRS == mission.PRMerged, c.PR.StatusRollup, c.PR.URL
		}
		oc.Direct, oc.Ahead, oc.Landed = c.Direct, c.Ahead, c.Landed != nil
		if c.Exists {
			oc.DiffStat = orchestrator.DiffStat(c.Path, "origin/"+w.WS().DefaultBranch)
		}
		in.Capsules = append(in.Capsules, oc)
	}
	for _, e := range events {
		in.Log = append(in.Log, prompt.EventLine(e))
	}
	if crewID != "" {
		in.Output = orchestrator.CrewOutput(store, m.Slug, crewID, w.opts.Live)
	}
	in.Stages = prompt.StageLines(m, brief)
	states, _ := m.StageProgress(brief)
	if st, ok := mission.StageFor(states, crewID); ok && crewID != "" && !st.Implicit {
		in.CrewStage = fmt.Sprintf("%d of %d: %s", st.N, len(states), oneLine(st.Text))
	}
	in.Reviews = w.reviewLines(m)
	in.Process = w.ProcessText()
	for _, g := range w.Guarantees() {
		in.Guarantees = append(in.Guarantees, g.What)
	}
	in.Standing = w.standingWithoutOrchestrator(v, brief, events)
	if all, err := store.Decisions(m.Slug); err == nil {
		for _, d := range mission.CurrentDecisions(all) {
			in.Decisions = append(in.Decisions, d.Line())
		}
	}
	if c, ok := m.CrewByID(crewID); ok && trigger == "quiet" {
		in.LastReply = c.LastReply
	}
	if trigger == "review" {
		if r, err := review.For(w.WS().Root, m.Slug).Latest(""); err == nil && r != nil {
			in.Review, in.ReviewRef = review.Render(r), review.RenderedRef(r.ID)
			for _, c := range m.Crew {
				if c.Capsule == r.Capsule && c.Role == "" {
					in.ReviewCrew = append(in.ReviewCrew, c.ID)
				}
			}
		}
	}
	// Crew that reported done left a handoff: it says what they did and
	// learned better than the end of their output.
	if trigger == "checks" {
		if st, ok := mission.StageFor(states, crewID); ok {
			if r, ok := st.Record.LatestChecks(); ok {
				in.Round = r.N
				for _, c := range r.Results {
					line := fmt.Sprintf("%s: %s (by %s)", c.Check, c.Result, c.By)
					if said := c.Said(600); said != "" {
						line += ": " + said
					}
					in.Checks = append(in.Checks, line)
				}
			}
		}
		in.MaxRounds = w.Config().MaxRounds()
	}
	if trigger == "pr" {
		// What happened to its PRs since the orchestrator's last run.
		since := 0
		for i, e := range events {
			if e.Kind == mission.EventOrchestrate {
				since = i + 1
			}
		}
		for _, e := range events[since:] {
			if e.Kind == mission.EventPR {
				in.PREvents = append(in.PREvents, clipLine(e.Text, 600))
			}
		}
		if cur, ok := mission.CurrentStage(states); ok && cur.Step != nil && cur.Step.Type == mission.StepOpenPR {
			in.PRStep = fmt.Sprintf("%d of %d", cur.N, len(states))
			if cur.Record != nil {
				in.Round = cur.Record.Rounds
			}
			in.MaxRounds = w.Config().MaxRounds()
		}
	}
	if trigger == "step" {
		if cur, ok := mission.CurrentStage(states); ok && cur.Held() {
			in.HeldStep = fmt.Sprintf("%d of %d (%s: %s)", cur.N, len(states), cur.Step.Type, oneLine(cur.Text))
			var why []string
			if cur.Step.OnlyIf != "" {
				why = append(why, "it applies only if "+strings.TrimSuffix(oneLine(cur.Step.OnlyIf), "."))
			}
			if cur.Step.InEach != "" {
				why = append(why, "it runs in each "+strings.TrimSuffix(oneLine(cur.Step.InEach), ".")+", so name the repos")
			}
			in.HeldWhy = strings.Join(why, ", and ")
		}
	}
	if trigger == "limit" {
		if cur, ok := mission.CurrentStage(states); ok {
			in.OutOfTime = fmt.Sprintf("%d: %s", cur.N, oneLine(cur.Text))
		}
	}
	if c, ok := m.CrewByID(crewID); ok && (trigger == "done" || trigger == "checks") && c.HandedOff != nil {
		if data, err := os.ReadFile(store.HandoffPath(m.Slug, crewID)); err == nil {
			in.Handoff = string(data)
		}
	}
	return in
}

// reviewLines says where each capsule's latest review stands: its verdict,
// its comments' answers, and whether the work is back with the user.
func (w *Workspace) reviewLines(m *mission.Mission) []string {
	store := review.For(w.WS().Root, m.Slug)
	waiting := ReReviews(w.WS(), m)
	verdicts := map[string]string{review.Approve: "approved", review.Changes: "changes requested", review.Comment: "comments"}
	tally := func(r *review.Review) string {
		if t := r.Tally(); t.Total() > 0 {
			return fmt.Sprintf("; comments: %d open, %d addressed, %d won't fix, %d resolved", t.Open, t.Addressed, t.WontFix, t.Resolved)
		}
		return ""
	}
	var lines []string
	for _, c := range m.Capsules {
		capsule := c.Repo + "/" + c.Name
		r, err := store.Latest(capsule)
		if err != nil || r == nil {
			continue
		}
		line := fmt.Sprintf("%s of %s: %s, at %s", r.ID, capsule, verdicts[r.Verdict], shortSHA(r.Head)) + tally(r)
		for _, w := range waiting {
			if w.Capsule == capsule {
				line += fmt.Sprintf("; the capsule has moved on to %s and nobody's working in it: waiting for the user's re-review", shortSHA(w.Head))
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// standingWithoutOrchestrator is where the mission stands if the
// orchestrator does nothing: its headline, with the orchestrator's own run
// (and its turn to look) left out.
func (w *Workspace) standingWithoutOrchestrator(v MissionView, brief string, events []mission.Event) string {
	mm := *v.M
	mm.Orchestrating = nil
	v.M = &mm
	s := MissionSnapshot(v, brief, events, ReReviews(w.WS(), v.M), DocThreadCourt(w.WS(), v, brief), !w.Config().OrchestratorEnabled())
	h := mission.Resolve(s)[0]
	return prompt.CourtWords(h.Court) + ": " + h.Text
}

// orchestratorActs are what the orchestrator's ws mission commands log:
// its note, and each way of placing the ball.
// orchestratorActed says whether the orchestrator's run did anything
// through ws: whatever its agent ran is logged with the run (Event.Run),
// its note too. What ws logs of the run itself (its start, what it said)
// doesn't count.
func orchestratorActed(store mission.Store, slug, run string) bool {
	events, _ := store.ReadLog(slug, orchestrator.LogEvents)
	for _, e := range slices.Backward(events) {

		if e.Run != run {
			continue
		}
		if e.Kind == mission.EventOrchestrateRun {
			return false // back to its start: nothing since
		}
		if e.Kind != mission.EventOrchestrate {
			return true
		}
	}
	return false
}

// orchestratorDidNothing puts a mission the orchestrator left nowhere in the
// user's court: a question, with what it said, unless one is open already.
// Answering it runs the orchestrator again, with the answer.
func orchestratorDidNothing(store mission.Store, slug, what, out string, runErr error) {
	m, err := store.Load(slug)
	if err != nil {
		return
	}
	if _, asking := m.Question(); asking {
		return // the user's asked something already
	}
	text := "The orchestrator looked at " + what + " but ran no commands, twice, so nothing happened."
	if runErr != nil {
		text = "The orchestrator couldn't run on " + what + ": " + oneLine(runErr.Error()) + "."
	}
	if gist := orchestrator.Gist(out); gist != "" {
		text += " It said: " + gist
	}
	text += " What should happen next?"
	store.Ask(slug, text, "ws")
}
