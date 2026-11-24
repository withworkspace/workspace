package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/runs"
)

// Work's expansion, its checks: once crew report done at a Work step that
// says what has to be true for it to be done, ws runs each check that's a
// command in the step's capsule, and has the judge (an agent with fresh
// context, read-only) judge the rest. The round is kept on the step's
// record. All met, ws passes the step; unmet, it's the orchestrator's
// call, by the process, until [missions] max_rounds have come round, then
// yours; can't tell is yours.

// checkTimeout is how long a check's command may run.
const checkTimeout = 15 * time.Minute

// judgeTimeout is how long the judge may take.
const judgeTimeout = 10 * time.Minute

// checkOutputMax is how much of a command's output is kept as what shows
// its result: its end.
const checkOutputMax = 3 << 10

// CrewDone is what follows crew reporting done (ws mission done): at a
// Work step with checks, ws runs them (in the workspace's daemon, as a
// job: opts.Check) before anyone looks at it; otherwise the orchestrator
// looks, as it always has. It says which.
func (w *Workspace) CrewDone(slug, crewID string) string {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return ""
	}
	brief, _ := w.Missions.ReadBrief(slug)
	states, _ := m.StageProgress(brief)
	if st, ok := mission.StageFor(states, crewID); ok && st.Current && st.Step != nil && st.Step.Type == mission.StepWork && len(st.Step.Checks) == 0 {
		// A run of a step in several repos, with no checks: done when its
		// crew are; the orchestrator passes the step once every run is.
		if c, ok := m.CrewByID(crewID); ok {
			if in, ok := st.Instance(c.Capsule); ok && in.Done == nil {
				w.Missions.SetInstance(slug, st.N, in.Repo, c.Capsule, true)
			}
		}
	}
	if st, ok := mission.StageFor(states, crewID); ok && st.Current && st.Step != nil && st.Step.Type == mission.StepOpenPR {
		// Sent back to merge its base in before its pull request opened:
		// ws tries again now. (Crew at an open one report to the
		// orchestrator, as before.)
		if c, ok := m.CrewByID(crewID); ok {
			if seen := seenOf(m, c.Capsule); seen == nil || seen.Merged || seen.Closed {
				if said := w.RetryOpenPRStep(slug, c.Capsule); said != "" {
					return said
				}
			}
		}
	}
	if said := w.verifyAgain(m, crewID); said != "" {
		return said
	}
	if st, id, ok := m.UncheckedDone(brief); ok && id == crewID {
		if w.start(job.Job{Kind: job.Check, Mission: slug, ID: crewID}) {
			return fmt.Sprintf("ws runs step %d's checks", st.N)
		}
		said, err := w.RunChecks(slug, crewID)
		if err != nil {
			return "couldn't run its checks: " + err.Error()
		}
		return said
	}
	w.Notify(slug, mission.OrchestratorEvent{Crew: crewID, Trigger: "done"})
	return ""
}

// RunChecks runs the checks of the step crew reported done at, and has the
// judge judge those in words, keeps the round, and acts on it
// (afterChecks). It says what it did; "" when there's nothing to check
// (the step's moved on, or its done is checked already).
func (w *Workspace) RunChecks(slug, crewID string) (string, error) {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return "", err
	}
	brief, _ := w.Missions.ReadBrief(slug)
	st, id, ok := m.UncheckedDone(brief)
	if !ok || id != crewID {
		return "", nil
	}
	capsule, hasCapsule := w.stepCapsule(m, st)
	if c, ok := m.CrewByID(crewID); ok && c.Capsule != "" && c.Capsule != RootPlace {
		capsule, hasCapsule = c.Capsule, true // where the crew worked
	}
	dir, commit, tree := "", "", ""
	if hasCapsule {
		repo, name, _ := strings.Cut(capsule, "/")
		dir = filepath.Join(w.WS().RepoDir(repo), name)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			dir, hasCapsule = "", false
		} else {
			commit = gitOut(dir, "rev-parse", "HEAD")
			tree = cleanTree(dir)
		}
	}
	if st.Step.Type == mission.StepVerify {
		// Its verifier's done is its bundle: ws runs the step's commands,
		// and takes the rest from what the bundle says.
		return w.settleVerify(m, st, crewID, capsule, dir, commit)
	}
	checks := st.Step.Checks
	results := make([]mission.CheckResult, len(checks))
	var judged []int
	for i, c := range checks {
		if !c.Run() {
			judged = append(judged, i)
		}
	}
	run := w.checkRound(slug, st, crewID, judged)
	defer w.Missions.ClearChecking(slug, st.N) // recording the round clears it; failing to, this does
	var ran []string
	for i, c := range checks {
		if !c.Run() {
			continue
		}
		if !hasCapsule {
			results[i] = mission.CheckResult{Check: c.Text, Result: mission.CheckCantTell, By: "ws", Evidence: "the step has no capsule to run it in"}
		} else if before, res, ok := st.Record.MetOn(capsule, tree, c.Text); ok {
			// The same files it was met on, all committed then and now:
			// it's met, and isn't run again (a suite that takes minutes,
			// for a done reported again with nothing changed).
			results[i] = run.metBefore(i, c, before, res)
		} else {
			results[i] = run.command(i, c, capsule, dir, crew.Env(w.Root(), slug, ""))
		}
		ran = append(ran, fmt.Sprintf("%s: %s (%s)", c.Text, results[i].Result, firstLine(results[i].Evidence)))
	}
	if len(judged) > 0 {
		verdicts, by, err := w.judge(m, st, crewID, capsule, dir, commit, judged, ran, run)
		for k, i := range judged {
			r := mission.CheckResult{Check: checks[i].Text, By: by}
			if err != nil {
				r.Result, r.Evidence, r.By = mission.CheckCantTell, "the judge couldn't be asked: "+err.Error(), "ws"
			} else {
				r.Result, r.Evidence = verdicts[k].Result, verdicts[k].Why
			}
			results[i] = r
		}
	}
	done := ""
	if c, ok := m.CrewByID(crewID); ok {
		done = c.DoneKey()
	}
	all := mission.CheckRound{Crew: crewID, Done: done, Capsule: capsule, Commit: commit, Tree: tree, Results: results}
	if all.Met() {
		// Recording the round ends ws's checking, and it's ws's until the
		// step has passed and the next is reached: marked before, so
		// there's no moment between with nobody on it.
		defer w.moving(slug, st.N)()
	}
	round, err := w.Missions.RecordChecks(slug, st.N, all)
	if err != nil {
		return "", err
	}
	return w.afterChecks(slug, st.N, round), nil
}

// checkRecipe is a check's command that asks for one of the repo's own
// recipes: ws run test.
var checkRecipe = regexp.MustCompile(`^ws\s+run\s+([a-z0-9][a-z0-9_-]*)$`)

// runCheckTask runs a step's check that's a command as a task (a run that
// ends by itself, internal/runs) in the capsule at dir, and waits for it:
// met when it passes or fails as the check says; can't tell when it
// couldn't be run, was stopped, or ran past timeout. The run is kept, and
// the result names it: all it printed, how long it took, the commit it
// ran at and the reports it wrote, where the evidence is its last words. A
// command that's `ws run <recipe>`, for a task the capsule's repo has a
// recipe for, is that recipe's run, with its reports. hooks follow it as
// it runs.
func (w *Workspace) runCheckTask(capsule, dir string, c mission.Check, env []string, of runs.CheckOf, timeout time.Duration, hooks checkHooks) mission.CheckResult {
	res := mission.CheckResult{Check: c.Text, By: "ws"}
	repo, name, _ := strings.Cut(capsule, "/")
	ask := RunAsk{
		Repo: repo, Worktree: name, Dir: dir, Command: c.Command, Type: runs.Task,
		Name:  fmt.Sprintf("check-%d-%d", of.Step, of.N),
		Owner: runs.Owner{Kind: runs.ByMission, Mission: of.Mission},
		// Its time outlasts the wait for it, so it's this that stops it.
		Timeout: timeout + 5*time.Minute, By: "ws", Mission: of.Mission, Env: env, Check: &of, Wait: RunUpWait,
	}
	if m := checkRecipe.FindStringSubmatch(strings.TrimSpace(c.Command)); m != nil {
		if w.recipes(repo, name)[m[1]].IsTask() {
			ask.Recipe, ask.Command, ask.Name = m[1], "", ""
		}
	}
	up, err := w.StartRun(ask)
	if err != nil {
		res.Result, res.Evidence = mission.CheckCantTell, "couldn't run it: "+err.Error()
		return res
	}
	run, store := up.Run, w.runStore()
	res.Run = run.NRef()
	if hooks.Started != nil {
		hooks.Started(run.PID)
	}
	stop := followRun(run, hooks.Line)
	now, ended, err := store.WaitEnd(run, timeout, nil)
	stop()
	tail := clipTail(strings.TrimSpace(now.Tail(200)), checkOutputMax)
	switch {
	case err != nil:
		res.Result, res.Evidence = mission.CheckCantTell, "couldn't follow it: "+err.Error()
		return res
	case !ended:
		store.End(now) //nolint:errcheck // it's stopped, and kept as stopped
		res.Result, res.Evidence = mission.CheckCantTell, fmt.Sprintf("it ran past %s, and was stopped", timeout)
		return res
	case now.State != runs.Exited:
		why := "it was stopped before it ended"
		if now.Why != "" {
			why += ": " + now.Why
		}
		res.Result, res.Evidence = mission.CheckCantTell, why
		return res
	case now.Status == 126 || now.Status == 127:
		res.Result, res.Evidence = mission.CheckCantTell, fmt.Sprintf("couldn't run it (exit %d): %s", now.Status, tail)
		return res
	}
	took := now.Took().Round(time.Second)
	how := fmt.Sprintf("passed in %s", took)
	if !now.Passed() {
		how = fmt.Sprintf("failed (exit %d) in %s", now.Status, took)
	}
	if now.Passed() == (c.Expect == mission.CheckPasses) {
		res.Result = mission.CheckMet
	} else {
		res.Result = mission.CheckUnmet
	}
	res.Evidence = how
	if tail != "" {
		res.Evidence += ":\n" + tail
	}
	return res
}

// followRun calls line with the last line a run has printed, every
// checkLineEvery while it changes, until the returned stop is called.
func followRun(r runs.Run, line func(string)) (stop func()) {
	if line == nil {
		return func() {}
	}
	done, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		t := time.NewTicker(checkLineEvery)
		defer t.Stop()
		said := ""
		for {
			select {
			case <-done:
				return
			case <-t.C:
				now := ""
				lines := strings.Split(r.Tail(5), "\n")
				for i := len(lines) - 1; i >= 0 && now == ""; i-- {
					now = cleanLine([]byte(lines[i]))
				}
				if now != said {
					said = now
					line(now)
				}
			}
		}
	}()
	return func() {
		close(done)
		<-finished
	}
}

// runCheckWithin runs a check's command in dir, as ws's own child, with
// nothing of it kept but its last words: what a watched Wait step looks
// with, every few minutes. It's met when it passes or fails as the check
// says; can't tell when it couldn't be run, or ran past timeout, when
// it's stopped. hooks follow it as it runs.
func runCheckWithin(dir string, c mission.Check, env []string, hooks checkHooks, timeout time.Duration) mission.CheckResult {
	r := mission.CheckResult{Check: c.Text, By: "ws"}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", c.Command)
	cmd.Dir, cmd.Env = dir, env
	cmd.WaitDelay = 5 * time.Second
	var out lastLine
	cmd.Stdout, cmd.Stderr = &out, &out
	start := time.Now()
	err := cmd.Start()
	if err == nil {
		if hooks.Started != nil {
			hooks.Started(cmd.Process.Pid)
		}
		stop := out.follow(hooks.Line)
		err = cmd.Wait()
		stop()
	}
	took := time.Since(start).Round(time.Second)
	tail := clipTail(strings.TrimSpace(out.String()), checkOutputMax)
	var exit *exec.ExitError
	switch {
	case ctx.Err() != nil:
		r.Result, r.Evidence = mission.CheckCantTell, fmt.Sprintf("it ran past %s, and was stopped", timeout)
		return r
	case err != nil && !errors.As(err, &exit):
		r.Result, r.Evidence = mission.CheckCantTell, "couldn't run it: "+err.Error()
		return r
	case exit != nil && (exit.ExitCode() == 126 || exit.ExitCode() == 127):
		r.Result, r.Evidence = mission.CheckCantTell, fmt.Sprintf("couldn't run it (exit %d): %s", exit.ExitCode(), tail)
		return r
	}
	passed := err == nil
	how := fmt.Sprintf("passed in %s", took)
	if !passed {
		how = fmt.Sprintf("failed (exit %d) in %s", exit.ExitCode(), took)
	}
	if passed == (c.Expect == mission.CheckPasses) {
		r.Result = mission.CheckMet
	} else {
		r.Result = mission.CheckUnmet
	}
	r.Evidence = how
	if tail != "" {
		r.Evidence += ":\n" + tail
	}
	return r
}

// judge asks the judge about the step's checks at judged (their indexes),
// in the capsule dir (the workspace's root without one), read-only. It
// returns what it said of each, and who it was. round, if there is one,
// keeps and logs its turn.
func (w *Workspace) judge(m *mission.Mission, st mission.StageState, crewID, capsule, dir, commit string, judged []int, ran []string, round *checkRound) (verdicts []prompt.Verdict, by string, err error) {
	pick, h, err := Roles(w.Config()).ForJudge()
	if round != nil {
		var since time.Time
		if err == nil {
			since = round.judging("the judge (" + pick.String() + ")")
		}
		defer func() { round.judgedThem(since, by, verdicts, err) }()
	}
	if err != nil {
		return nil, "", err
	}
	j := prompt.Judging{Mission: m.Slug, Title: m.Title, Step: fmt.Sprintf("%d: %s: %s", st.N, st.Step.Type, st.Step.Text),
		Makes: st.Step.Makes, Crew: crewID, Capsule: firstNonEmpty(capsule, "(none)"), Commit: commit, Dir: dir, Ran: ran}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	j.Goal = briefGoal(brief)
	for _, i := range judged {
		j.Checks = append(j.Checks, st.Step.Checks[i].Text)
	}
	if data, err := os.ReadFile(w.Missions.HandoffPath(m.Slug, crewID)); err == nil {
		j.Handoff = string(data)
	}
	j.Decided = w.decidedSince(m, st)
	if dir != "" {
		j.Diff = capsuleDiff(dir, "origin/"+w.WS().DefaultBranch)
	}
	states, _ := m.StageProgress(brief)
	for _, a := range m.Artifacts {
		if a.Kind == mission.HandoffKind {
			continue
		}
		line := "`" + a.Ref + "`"
		if a.Label != "" {
			line += ": " + a.Label
		}
		if as, ok := mission.ArtifactStage(states, a); ok {
			line += fmt.Sprintf(" (step %d)", as.N)
		}
		j.Artifacts = append(j.Artifacts, line)
	}
	runPath := filepath.Join(w.Missions.Dir(), m.Slug, "judge.log")
	_, verdicts, err = w.askJudge(pick, h, j, "judge "+m.Slug, crew.Env(w.Root(), m.Slug, ""), runPath)
	if err != nil {
		execlog.Note(w.Root(), "judge "+m.Slug, "couldn't judge step %d: %v", st.N, err)
		return nil, "", err
	}
	return verdicts, "the judge (" + pick.String() + ")", nil
}

// decidedSince is what the user has decided on the mission, and still
// stands, since step st's words could last have changed: since its crew
// started on it (a step that's begun stays as it's written), or since the
// plan was last written if that was before. A line each for the judge,
// oldest first. Only the user's stand over a step's words, so only theirs
// are given: crew's own record of what the user chose reads as the user's
// to a judge that's shown it.
func (w *Workspace) decidedSince(m *mission.Mission, st mission.StageState) []string {
	var since time.Time
	if st.Record != nil && st.Record.Started != nil {
		since = *st.Record.Started
	}
	if info, err := os.Stat(w.Missions.PlanPath(m.Slug)); err == nil && (since.IsZero() || info.ModTime().Before(since)) {
		since = info.ModTime()
	}
	if since.IsZero() {
		return nil
	}
	all, err := w.Missions.Decisions(m.Slug)
	if err != nil {
		return nil
	}
	var out []string
	for _, d := range all {
		if d.State != mission.DecisionCurrent || d.By != AsYou.By() || d.At.Before(since) {
			continue
		}
		line := fmt.Sprintf("%s: %s", d.ID, sentence(oneLine(d.What)))
		if why := oneLine(d.Reason()); why != "" {
			line += " Why: " + sentence(clipString(why, 600))
		}
		out = append(out, line)
	}
	return out
}

// askJudge is the judge's one run: what j says, put to pick's agent in
// j.Dir (the workspace's root without one), headless and never gone back
// to, and its answer read as a verdict for each of j's checks. A mission's
// checks and an eval's fixture both come through here, so an eval judges
// as a mission's judge does.
func (w *Workspace) askJudge(pick harness.Pick, h harness.Converser, j prompt.Judging, who string, env []string, runPath string) (harness.Report, []prompt.Verdict, error) {
	run := harness.ConverseRun{Prompt: prompt.Judge(j), Dir: firstNonEmpty(j.Dir, w.Root()), Model: pick.Model, Once: true}
	rep, err := RunHeadless(w.Root(), who, env, runPath, judgeTimeout, h, run)
	if err != nil {
		return rep, nil, err
	}
	return rep, prompt.ReadVerdicts(rep.Output, len(j.Checks)), nil
}

// afterChecks acts on a round of step n's checks: all met, ws passes the
// step; any can't tell, it's yours; unmet, the orchestrator's call by the
// process, until max_rounds have come round (or with the orchestrator
// off), then yours. It says what it did.
func (w *Workspace) afterChecks(slug string, n int, round mission.CheckRound) string {
	m, _ := w.Missions.Load(slug)
	cur, _ := currentStep(w.Missions, m)
	if _, ok := cur.Instance(round.Capsule); ok && cur.N == n && round.Met() {
		return w.instanceDone(slug, cur, round.Capsule, "ws", "its checks are met")
	}
	// Rounds count where they ran: a step in several repos has its own in
	// each.
	rounds := 0
	if cur.Record != nil {
		for _, r := range cur.Record.Checks {
			if r.Capsule == round.Capsule {
				rounds++
			}
		}
	}
	round.N = max(rounds, 1)
	if round.Met() {
		return w.passChecked(slug, n, round)
	}
	if cant := round.With(mission.CheckCantTell); len(cant) > 0 && len(round.With(mission.CheckUnmet)) == 0 {
		return w.askAboutChecks(slug, n, round, "the judge can't tell of "+checkList(cant)+": it's yours to say")
	}
	if round.N >= w.Config().MaxRounds() {
		return w.askAboutChecks(slug, n, round, fmt.Sprintf("that's %d %s unmet, as many as ws lets come round (%s)", round.N, plural(round.N, "round"), guaranteeMaxRounds(w.Config().MaxRounds()).Key))
	}
	if w.Notify(slug, mission.OrchestratorEvent{Crew: round.Crew, Trigger: "checks"}) {
		return fmt.Sprintf("step %d's checks aren't all met (%s): the orchestrator decides what follows, by the process", n, round.Summary())
	}
	return w.askAboutChecks(slug, n, round, "the orchestrator isn't running to send it back")
}

// passChecked passes step n for ws, round's checks all met, and says what
// it did. Three things stop it, and each is said on the mission rather
// than left in ws's own log:
//
//   - Its crew are in a turn begun since they reported done (a message
//     sent to them after it). What they're doing now isn't in what was
//     checked, so the step isn't passed under them: TurnEnded looks again
//     when the turn ends.
//   - The capsule isn't at the commit the round ran at: they've committed
//     since, so what's there isn't what was checked. Still running,
//     they're asked to report done again, and it's checked again; gone,
//     it's yours to say.
//   - Anything else that refuses it is yours to pass, asked as checks not
//     all met are.
func (w *Workspace) passChecked(slug string, n int, round mission.CheckRound) string {
	met := fmt.Sprintf("step %d's checks are met", n)
	m, err := w.Missions.Load(slug)
	if err != nil {
		return met
	}
	if err := w.acceptAllowed(m); err != nil {
		return fmt.Sprintf("%s, and it passes once the user's review is settled: %v", met, err)
	}
	brief, _ := w.Missions.ReadBrief(slug)
	if _, id, ok := m.UncheckedDone(brief); ok {
		return fmt.Sprintf("%s, and %s has reported done again since: ws checks that", met, id)
	}
	c, ok := m.CrewByID(round.Crew)
	running := ok && c.Ended == nil
	if running && c.InTurnSinceDone() {
		return w.notePass(slug, fmt.Sprintf("%s (round %d): it passes when %s's turn ends, which began after they reported done", met, round.N, c.ID))
	}
	if head := w.capsuleHead(round.Capsule); round.Commit != "" && head != "" && head != round.Commit {
		moved := fmt.Sprintf("%s is at %s, and the checks ran at %s", round.Capsule, shortCommit(head), shortCommit(round.Commit))
		if running {
			text := fmt.Sprintf("Step %d's checks are met for the work as it was when you reported done, and you've committed since (%s). If what you've added is finished, report done again with a handoff (ws mission done): ws checks it again, and passes the step on that.", n, moved)
			if _, err := w.Say(slug, c.ID, text, AsWS); err == nil {
				return w.notePass(slug, fmt.Sprintf("%s (round %d), but the work has changed since (%s): asked %s to report done again", met, round.N, moved, c.ID))
			}
		}
		return w.askAboutChecks(slug, n, round, "the work has changed since they ran ("+moved+")")
	}
	said, err := w.tryPassStep(slug, n, "ws", fmt.Sprintf("its checks are met (round %d)", round.N), mission.StepWork)
	switch {
	case err == nil && said != "":
		return said
	case err == nil:
		return met
	}
	if t, open := m.StepTask(n); open && t.Verb == mission.TaskChecks {
		// You've been asked of it already: once is enough.
		return fmt.Sprintf("%s, and ws still couldn't pass it: %v", met, err)
	}
	return w.askAboutChecks(slug, n, round, "ws couldn't pass it: "+err.Error())
}

// notePass says on the mission why a step whose checks are met isn't
// passed yet, and gives it back to say.
func (w *Workspace) notePass(slug, text string) string {
	w.Missions.AppendLog(slug, mission.Event{Kind: mission.EventNote, Crew: "ws", Text: text}) //nolint:errcheck // a note
	return text
}

// capsuleHead is the commit capsule (repo/name) is at, or "" where it
// can't be read.
func (w *Workspace) capsuleHead(capsule string) string {
	repo, name, ok := strings.Cut(capsule, "/")
	if !ok {
		return ""
	}
	return gitOut(filepath.Join(w.WS().RepoDir(repo), name), "rev-parse", "HEAD")
}

// shortCommit is a commit as it's said in a sentence.
func shortCommit(c string) string {
	if len(c) > 9 {
		return c[:9]
	}
	return c
}

// TurnEnded is what follows a turn of crew's ending. A step whose checks
// came round met while they were in a turn begun since their done is
// looked at again (passChecked), now that the turn's over; otherwise the
// orchestrator looks, where the turn is its to look at (Notify).
func (w *Workspace) TurnEnded(slug, crewID string) {
	if said, ok := w.settleChecked(slug, crewID); ok {
		execlog.Note(w.Root(), "checks "+slug, "%s", said)
		return
	}
	w.Notify(slug, mission.OrchestratorEvent{Crew: crewID, Trigger: "quiet"})
}

// settleChecked looks again at the step the mission is at, when its
// latest round of checks is all met, was run on crew crewID's done, which
// still stands, and nothing passed it: what held it may be over. It says
// what it did, and whether there was such a step.
func (w *Workspace) settleChecked(slug, crewID string) (string, bool) {
	m, err := w.Missions.Load(slug)
	if err != nil || m.Checking != nil {
		return "", false
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.Step.Type != mission.StepWork || len(cur.Step.Checks) == 0 || len(cur.Instances()) > 0 {
		return "", false
	}
	round, ok := cur.Record.LatestChecks()
	if !ok || !round.Met() || round.Crew != crewID {
		return "", false
	}
	if c, ok := m.CrewByID(crewID); !ok || c.Status != mission.CrewDone || round.Done != "" && round.Done != c.DoneKey() {
		return "", false
	}
	return w.passChecked(slug, cur.N, round), true
}

// askAboutChecks makes step n yours, its checks not all met: pass it as it
// is, send it back to its crew with what's unmet, or say what to do.
func (w *Workspace) askAboutChecks(slug string, n int, round mission.CheckRound, why string) string {
	if said := w.passApproved(slug, n, round); said != "" {
		return said
	}
	text := fmt.Sprintf("Step %d's checks, round %d: %s. %s. %sass it as it is, send it back to %s with what's unmet, or say what to do.", n, round.N, round.Summary(), upperFirst(why), w.checksShownBy(slug, n, round.Crew), round.Crew)
	if _, err := w.Missions.AskAtStep(slug, mission.TaskChecks, text, n, mission.ChecksPass, mission.ChecksSendBack); err != nil {
		return "couldn't ask the user: " + err.Error()
	}
	return fmt.Sprintf("step %d's checks aren't all met (%s): it's the user's, as %s", n, round.Summary(), why)
}

// passApproved passes step n, its checks about to be yours to say, where
// you've said already: you approved every bundle that shows them before
// ws came to ask, and the work is as it was when they were handed over.
// Approving them once asked passes the step, and so does having approved
// them. A bundle whose capsule has moved on since shows work that's
// changed, so that's still asked. It says what it did, or "" when you're
// to be asked.
func (w *Workspace) passApproved(slug string, n int, round mission.CheckRound) string {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return ""
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.N != n || len(cur.Instances()) > 0 {
		// A step that runs in several repos passes run by run.
		return ""
	}
	brief, _ := w.Missions.ReadBrief(slug)
	states, _ := m.StageProgress(brief)
	looks := m.ChecksLooks(states, n)
	if len(looks) == 0 {
		return ""
	}
	var ids []string
	for _, l := range looks {
		b, ok := m.Bundle(l.Bundle)
		if !ok || !l.Approved || w.bundleMovedOn(b) {
			return ""
		}
		ids = append(ids, b.ID)
	}
	why := fmt.Sprintf("you approved %s, its checks as they were (round %d: %s)", strings.Join(ids, ", "), round.N, round.Summary())
	said, err := w.tryPassStep(slug, n, "you", why, cur.Step.Type)
	if err != nil {
		return "" // it couldn't be passed on that: you're asked, and told why
	}
	return said
}

// bundleMovedOn reports whether a bundle's capsule isn't at the commit it
// was handed over at: the work it shows has changed since, or can't be
// found to say.
func (w *Workspace) bundleMovedOn(b mission.Bundle) bool {
	if b.Capsule == "" || b.Commit == "" {
		return false
	}
	repo, name, _ := strings.Cut(b.Capsule, "/")
	head := gitOut(filepath.Join(w.WS().RepoDir(repo), name), "rev-parse", "HEAD")
	return head == "" || !(strings.HasPrefix(head, b.Commit) || strings.HasPrefix(b.Commit, head))
}

// checksShownBy says which bundles show step n's checks, yours to say,
// and what approving them does, leading into the answers ws suggests:
// "Look at b4, …. Or p" with a bundle, "P" with none. One you've approved
// already isn't there to approve: the mission's headline says so.
func (w *Workspace) checksShownBy(slug string, n int, crew string) string {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return "P"
	}
	brief, _ := w.Missions.ReadBrief(slug)
	states, _ := m.StageProgress(brief)
	var bundles []string
	for _, l := range m.ChecksLooks(states, n) {
		if !l.Approved {
			bundles = append(bundles, l.Name)
		}
	}
	if len(bundles) == 0 {
		return "P"
	}
	return fmt.Sprintf("Look at %s, on the mission's Verify tab: approving %s passes the step, and saying what's not right sends it back to %s. Or p", strings.Join(bundles, ", "), itOrThem(len(bundles)), crew)
}

// SendBack sends a Work step back to the crew whose done its latest checks
// were of, with what's unmet: typed in if they're still running, resumed if
// they've ended, else fresh crew in the capsule, with their handoff. It
// says what it did.
func (w *Workspace) SendBack(slug string, n int, by Actor) (string, error) {
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
	round, ok := st.Record.LatestChecks()
	if !ok {
		return "", refused("no-checks", "step %d's checks haven't been run, so there's nothing unmet to send back", st.N)
	}
	if st.Step != nil && st.Step.Type == mission.StepVerify {
		worker, ok := workCrew(m, st, round.Capsule)
		if !ok {
			return "", refused("no-crew", "no crew made the work step %d verifies in %s, so there's nobody to send it back to: dispatch crew there with what's unmet", st.N, round.Capsule)
		}
		return w.sendBackVerify(m, st, round, worker, by)
	}
	return w.carryOn(m, st, round.Crew, sendBackText(st, round), by)
}

// sendBackText is what crew are told when their step comes back to them.
func sendBackText(st mission.StageState, round mission.CheckRound) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Step %d isn't done yet: ws ran its checks on what you reported done (round %d), and not all are met.\n\n", st.N, round.N)
	for _, r := range round.Results {
		if r.Result == mission.CheckMet {
			continue
		}
		fmt.Fprintf(&b, "- %s: %s", r.Check, r.Result)
		if ev := strings.TrimSpace(r.Evidence); ev != "" {
			fmt.Fprintf(&b, ". %s", clipString(oneLine(ev), 600))
		}
		b.WriteString("\n")
	}
	b.WriteString("\nPut it right, commit, and report done again with a handoff (ws mission done): ws runs the checks again.")
	return b.String()
}

// carryOn has crew carry on at step st with text: typed into them if
// they're still running, resumed (and joined to the step) if they've
// ended and can be, else fresh crew in their capsule, told to read their
// handoff. It says what it did.
func (w *Workspace) carryOn(m *mission.Mission, st mission.StageState, crewID, text string, by Actor) (string, error) {
	c, ok := m.CrewByID(crewID)
	if !ok {
		return "", fmt.Errorf("%w: mission %s has no crew %s", ErrNotFound, m.Slug, crewID)
	}
	if c.Ended == nil {
		if _, err := w.Say(m.Slug, c.ID, text, by); err == nil {
			// It has more to do: its done is no longer its latest word.
			w.Missions.ClearStatus(m.Slug, c.ID)
			w.Missions.JoinStage(m.Slug, c.ID, st.N)
			return fmt.Sprintf("told %s, which is still running, to carry on at step %d", c.ID, st.N), nil
		}
	}
	if c.Conversation != "" {
		if err := w.Missions.SetResumeWithBy(m.Slug, c.ID, text, by.By()); err != nil {
			return "", err
		}
		if w.opts.Host == nil || !by.IsYou() && !w.Config().OrchestratorAutoDispatch() {
			w.Missions.JoinStage(m.Slug, c.ID, st.N)
			return fmt.Sprintf("%s has ended: the mission's page offers to resume it to carry on at step %d", c.ID, st.N), nil
		}
		l, err := w.Launch(m.Slug, Dispatch{Resume: c.ID, Stage: st.N}, by)
		var r *Refused
		switch {
		case err == nil:
			// As for crew still running: resumed by you with what to put
			// right, their done would otherwise stand (Store.ResumeCrew).
			w.Missions.ClearStatus(m.Slug, c.ID)
			w.Missions.JoinStage(m.Slug, c.ID, st.N)
			return fmt.Sprintf("resumed %s to carry on at step %d", l.ID, st.N), nil
		case errors.As(err, &r) && r.Code == "dispatch-limit":
			w.Missions.JoinStage(m.Slug, c.ID, st.N)
			return fmt.Sprintf("ws has started as many crew without the user as it may: the mission's page offers to resume %s to carry on at step %d", c.ID, st.N), nil
		}
		w.Missions.SetResumeWith(m.Slug, c.ID, "")
	}
	// Fresh crew, with what the crew before them handed off.
	capsule := c.Capsule
	if capsule == "" || capsule == RootPlace {
		if cap, ok := w.stepCapsule(m, st); ok {
			capsule = cap
		}
	}
	whole := strings.Join(strings.Fields(st.Whole()), " ")
	text = fmt.Sprintf("%s\n\n%s couldn't be resumed, so you carry on from them: read their handoff first (ws show -f llm %s).\n\n%s", whole, c.ID, mission.HandoffRef(c.ID), text)
	pick, _ := w.stepPick(st, "")
	if w.opts.Host == nil || !w.Config().OrchestratorAutoDispatch() {
		if err := w.Missions.QueueDispatch(m.Slug, &mission.Dispatch{Text: text, Capsule: capsule, By: by.By(), Stage: st.N, Harness: pick.Harness, Model: pick.Model}); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s can't be resumed: fresh crew to carry on at step %d are queued in %s for the user to launch", c.ID, st.N, capsule), nil
	}
	l, err := w.Launch(m.Slug, Dispatch{Capsule: capsule, Text: text, Stage: st.N, Harness: pick.Harness, Model: pick.Model}, by)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s can't be resumed: started %s in %s to carry on at step %d, with its handoff", c.ID, l.ID, l.Capsule, st.N), nil
}

// checkList is checks in a phrase: "the cause is named" and "…".
func checkList(rs []mission.CheckResult) string {
	var parts []string
	for _, r := range rs {
		parts = append(parts, "“"+clipString(oneLine(r.Check), 80)+"”")
	}
	return strings.Join(parts, ", ")
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSuffix(first, ":")
}

// gitOut is what git says in dir, trimmed; "" if it fails.
func gitOut(dir string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// cleanTree is the files the capsule at dir is at, as git names them (its
// commit's tree), when nothing there is uncommitted: "" when something
// is, or it can't be told.
func cleanTree(dir string) string {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil || strings.TrimSpace(string(out)) != "" {
		return ""
	}
	return gitOut(dir, "rev-parse", "HEAD^{tree}")
}

// capsuleDiff is what a capsule's commits change from where it left base:
// a stat, then the change, cut to fit a prompt.
func capsuleDiff(dir, base string) string {
	mb := gitOut(dir, "merge-base", base, "HEAD")
	if mb == "" {
		return ""
	}
	stat := gitOut(dir, "diff", "--stat", mb, "HEAD")
	if stat == "" {
		return ""
	}
	diff := gitOut(dir, "diff", mb, "HEAD")
	return stat + "\n\n" + clipHeadBytes(diff, 48<<10)
}

func clipHeadBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n… (cut: git diff shows the rest)"
}

func clipTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
