package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/orchestrator"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/ui"
)

// orchestrateSpawner has the workspace's daemon run the orchestrator for
// an event; tests stub it.
var orchestrateSpawner orchestrator.Spawner = spawnOrchestrator

// orchestrateHook is the end hook hosts run: crew's end is an event the
// orchestrator may look at (it does when crew ended by themselves).
func orchestrateHook(root string) crew.EndHook {
	return func(spec crew.Spec, exit string, stopped bool) {
		if ctx, err := LoadOps(root); err == nil {
			ctx.Notify(spec.Slug, mission.OrchestratorEvent{Crew: spec.CrewID, Trigger: "ended", Stopped: stopped})
		}
	}
}

func newMissionOrchestrateCmd() *cobra.Command {
	var slug, crewID, trigger, model string
	var auto, printPrompt bool
	var runSince int64
	cmd := &cobra.Command{
		Use:   "orchestrate",
		Short: "Have the orchestrator decide what happens next on a mission",
		Long: `Run the orchestrator on a mission: a headless agent that reads the brief,
the log, the capsules' diffs and PRs and the crew's output, then decides
what's next and acts through ws: ws mission dispatch (start new crew on
the next piece of work), resume-with (crew should carry on), flag (needs you), new (a
follow-up mission) or conclude (looks finished), always leaving a note. It
doesn't change code.

ws runs this automatically when crew report done, or end by themselves. Configure it under [orchestrator] in ws.toml / ws.local.toml:
harness (cursor, cursor-acp, claude, pi, codex or opencode; default the crew's, if it can run headless),
model (default ` + harness.CursorOrchestratorModel + ` on cursor, ` + harness.ClaudeOrchestratorModel + ` on claude,
` + harness.CodexOrchestratorModel + ` on codex, your own default on pi and opencode),
enabled = false to stop the automatic runs, dispatch = "ask" to queue
dispatched crew for you instead of launching them, and max_dispatches
(default 3) dispatches in a row without you before it must flag you.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			o := orchestrateOptions{Slug: slug, Crew: crewID, Trigger: trigger, Model: model, Auto: auto, PrintPrompt: printPrompt}
			if runSince != 0 {
				o.Since = time.Unix(runSince, 0).UTC()
			}
			return orchestrate(ctx, o, ui.Stderr)
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&crewID, "crew", "", "the crew whose event prompted this")
	cmd.Flags().StringVar(&trigger, "trigger", "manual", "what prompted the run: done, ended, answer (the user answered a flag), review (the user submitted one) or manual")
	cmd.Flags().StringVar(&model, "model", "", "model to use (default: [orchestrator] model, or the harness's own)")
	cmd.Flags().BoolVar(&auto, "auto", false, "run as ws's automatic run: honour [orchestrator] enabled, handle each crew event once")
	cmd.Flags().BoolVar(&printPrompt, "print-prompt", false, "print the orchestrator's prompt and command instead of running it")
	cmd.Flags().Int64Var(&runSince, "run", 0, "the run (its start, in Unix seconds) the mission records as underway, to end when this finishes")
	cmd.Flags().MarkHidden("run")
	return cmd
}

// orchestratorInput gathers what the orchestrator is told: the capsules
// (diffs, PRs), the recent log, and the prompting crew's output.
func orchestratorInput(ctx *ops.Workspace, m *mission.Mission, crewID, trigger string) orchestrator.Input {
	store := missionStore(ctx)
	v := ops.BuildMissionViews(ctx.WS(), ctx.GitHub(), []*mission.Mission{m})[0]
	brief, _ := store.ReadBrief(m.Slug)
	events, _ := store.ReadLog(m.Slug, orchestrator.LogEvents)
	in := orchestrator.Input{Mission: m, CrewID: crewID, Trigger: trigger, Brief: brief, Guide: ops.AgentsNoteFor(ctx.WS(), ops.RoleOrchestrator)}
	for _, c := range v.Capsules {
		oc := orchestrator.Capsule{Name: c.Repo + "/" + c.Name, Path: c.Path, Exists: c.Exists, Branch: c.Branch}
		if c.PR != nil {
			oc.PR, oc.PRMerged, oc.Checks, oc.URL = c.PR.Number, c.PRS == mission.PRMerged, c.PR.StatusRollup, c.PR.URL
		}
		oc.Direct, oc.Ahead, oc.Landed = c.Direct, c.Ahead, c.Landed != nil
		if c.Exists {
			oc.DiffStat = orchestrator.DiffStat(c.Path, "origin/"+ctx.WS().DefaultBranch)
		}
		in.Capsules = append(in.Capsules, oc)
	}
	for _, e := range events {
		in.Log = append(in.Log, formatEventLine(e, true))
	}
	if crewID != "" {
		in.Output = orchestrator.CrewOutput(store, m.Slug, crewID, liveOutput(ctx.WS().Root))
	}
	in.Stages = stageLines(m, brief)
	states, _ := m.StageProgress(brief)
	if st, ok := mission.StageFor(states, crewID); ok && crewID != "" && !st.Implicit {
		in.CrewStage = fmt.Sprintf("%d of %d: %s", st.N, len(states), oneLine(st.Text))
	}
	in.Reviews = reviewLines(ctx, m)
	in.Process = ctx.ProcessText()
	for _, g := range ctx.Guarantees() {
		in.Guarantees = append(in.Guarantees, g.What)
	}
	in.Standing = standingWithoutOrchestrator(ctx, v, brief, events)
	if all, err := store.Decisions(m.Slug); err == nil {
		for _, d := range mission.CurrentDecisions(all) {
			in.Decisions = append(in.Decisions, d.Line())
		}
	}
	if c, ok := m.CrewByID(crewID); ok && trigger == "quiet" {
		in.LastReply = c.LastReply
	}
	if trigger == "review" {
		if r, err := review.For(ctx.WS().Root, m.Slug).Latest(""); err == nil && r != nil {
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
					if ev := strings.TrimSpace(c.Evidence); ev != "" {
						line += ": " + clipLine(ev, 600)
					}
					in.Checks = append(in.Checks, line)
				}
			}
		}
		in.MaxRounds = ctx.Config().MaxRounds()
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
			in.MaxRounds = ctx.Config().MaxRounds()
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
func reviewLines(ctx *ops.Workspace, m *mission.Mission) []string {
	store := review.For(ctx.WS().Root, m.Slug)
	waiting := ops.ReReviews(ctx.WS(), m)
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

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func firstNonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// standingWithoutOrchestrator is where the mission stands if the
// orchestrator does nothing: its headline, with the orchestrator's own run
// (and its turn to look) left out.
func standingWithoutOrchestrator(ctx *ops.Workspace, v ops.MissionView, brief string, events []mission.Event) string {
	mm := *v.M
	mm.Orchestrating = nil
	v.M = &mm
	s := ops.MissionSnapshot(v, brief, events, ops.ReReviews(ctx.WS(), v.M), ops.DocThreadCourt(ctx.WS(), v, brief), !ctx.Config().OrchestratorEnabled())
	h := mission.Resolve(s)[0]
	return courtWords(h.Court, true) + ": " + h.Text
}

// orchestratorActs are what the orchestrator's ws mission commands log:
// its note, and each way of placing the ball.
// orchestratorActed says whether the orchestrator's run did anything
// through ws: whatever its agent ran is logged with the run (Event.Run),
// its note too. What ws logs of the run itself (its start, what it said)
// doesn't count.
func orchestratorActed(store mission.Store, slug, run string) bool {
	events, _ := store.ReadLog(slug, orchestrator.LogEvents)
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
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

// liveOutput is what running crew show, from the workspace's daemon, if
// it's running.
func liveOutput(root string) func(key string) string {
	return func(key string) string {
		c, err := daemon.Dial(root)
		if err != nil {
			return ""
		}
		defer c.Close()
		text, _ := c.Transcript(key)
		return text
	}
}

// orchestrateOptions are what a run of the orchestrator is for: the mission,
// the crew whose event prompted it and the event (trigger), and how it's run.
type orchestrateOptions struct {
	Slug, Crew, Trigger, Model string
	// Auto is ws's own run, for an event: it honours [orchestrator]
	// enabled, and handles each crew event once.
	Auto, PrintPrompt bool
	// Since is when the run was marked underway as it was asked for (a
	// job's), for it to end that mark; zero for a run by hand.
	Since time.Time
}

// orchestrate runs the orchestrator on a mission, saying what it does to w.
func orchestrate(ctx *ops.Workspace, o orchestrateOptions, w io.Writer) error {
	slug, crewID, trigger, model, auto, printPrompt := o.Slug, o.Crew, o.Trigger, o.Model, o.Auto, o.PrintPrompt
	// The run marked as underway when it was asked for ends however
	// this finishes.
	if !o.Since.IsZero() && slug != "" {
		defer missionStore(ctx).EndOrchestrating(slug, mission.OrchestratorRun{Since: o.Since})
	}
	if auto && !ctx.Config().OrchestratorEnabled() {
		execlog.Note(ctx.WS().Root, "orchestrator "+slug, "not run for %s: [orchestrator] enabled = false", strings.TrimSpace(crewID+" ("+trigger+")"))
		return nil
	}
	m, err := resolveMission(ctx, slug)
	if err != nil {
		return err
	}
	store := missionStore(ctx)
	if crewID != "" {
		if _, ok := m.CrewByID(crewID); !ok {
			return fmt.Errorf("mission %s has no crew %s", m.Slug, crewID)
		}
	}
	pick, rv, err := ops.Roles(ctx.Config()).ForOrchestrator()
	if err != nil {
		return err
	}
	if model == "" {
		model = pick.Model
	}
	in := orchestratorInput(ctx, m, crewID, trigger)
	prompt := orchestrator.Prompt(in)
	run := harness.OrchestratorRun{Prompt: "<a line pointing at the prompt file>", Dir: ctx.WS().Root, Model: model,
		Scratch: filepath.Join(ctx.WS().Root, ".ws", "converse")}
	for _, c := range in.Capsules {
		if c.Exists {
			run.Capsules = append(run.Capsules, c.Path)
		}
	}
	for _, g := range ops.Grounds(ctx.WS()) {
		run.Grounds = append(run.Grounds, g.Path)
	}
	if printPrompt {
		l, err := rv.Orchestrate(run)
		if err != nil {
			return err
		}
		fmt.Println(prompt)
		fmt.Fprintf(w, "  %s would write that to %s and run: %s\n", ui.Dim.Render("·"),
			orchestrator.PromptFile(store, m.Slug, "<run>"), strings.Join(l.Args, " "))
		return nil
	}

	what := orchestrator.Describe(crewID, trigger)
	if o.Since.IsZero() { // run by hand: mark it underway here
		if run, err := store.StartOrchestrating(m.Slug, what, orchestrator.RunFor()); err == nil {
			defer store.EndOrchestrating(m.Slug, run)
		}
	}
	with := rv.Name() + " " + model
	fmt.Fprintf(w, "  %s orchestrator looking at %s on %s with %s…\n", ui.Dim.Render("·"), what, m.Slug, with)

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
		if run.Prompt, err = orchestrator.WritePrompt(store, m.Slug, rec.ID, prompt); err != nil {
			return orchestrator.Report{}, false, err
		}
		l, err := rv.Orchestrate(run)
		if err != nil {
			return orchestrator.Report{}, false, err
		}
		rep, runErr := orchestrator.Run(l, ctx.WS().Root, m.Slug, rec.ID)
		summary := orchestrator.Summary(rep.Output, runErr)
		store.AppendLog(m.Slug, mission.Event{Kind: mission.EventOrchestrate, Crew: "orchestrator", Run: rec.ID, Text: summary})
		fmt.Fprintln(w, summary)
		acted := orchestratorActed(store, m.Slug, rec.ID)
		rec.Ended, rec.Output, rec.Calls, rec.Acted = time.Now(), rep.Output, rep.Calls, acted
		if runErr != nil {
			rec.Error = runErr.Error()
		}
		if err := orchestrator.SaveRun(store, m.Slug, &rec); err != nil {
			execlog.Note(ctx.WS().Root, "orchestrator "+m.Slug, "keeping the run: %v", err)
		}
		return rep, acted, runErr
	}
	rep, acted, runErr := runOnce(prompt, false)
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
	return runErr
}
