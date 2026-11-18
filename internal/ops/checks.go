package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
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
		if w.opts.Check != nil {
			w.opts.Check(slug, crewID)
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
	dir, commit := "", ""
	if hasCapsule {
		repo, name, _ := strings.Cut(capsule, "/")
		dir = filepath.Join(w.WS().RepoDir(repo), name)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			dir, hasCapsule = "", false
		} else {
			commit = gitOut(dir, "rev-parse", "HEAD")
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
		} else {
			results[i] = run.command(i, c, dir, crew.Env(w.Root(), slug, ""))
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
	round, err := w.Missions.RecordChecks(slug, st.N, mission.CheckRound{Crew: crewID, Done: done, Capsule: capsule, Commit: commit, Results: results})
	if err != nil {
		return "", err
	}
	return w.afterChecks(slug, st.N, round), nil
}

// runCheck runs a check's command in dir: met when it passes or fails as
// the check says; can't tell when it couldn't be run, or ran out of time.
// hooks follow it as it runs; what it keeps as evidence is the same
// without them.
func runCheck(dir string, c mission.Check, env []string, hooks checkHooks) mission.CheckResult {
	r := mission.CheckResult{Check: c.Text, By: "ws"}
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
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
		r.Result, r.Evidence = mission.CheckCantTell, fmt.Sprintf("it ran past %s, and was stopped", checkTimeout)
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
		m, err := w.Missions.Load(slug)
		if err == nil {
			if err := w.acceptAllowed(m); err != nil {
				return fmt.Sprintf("step %d's checks are met, and it passes once the user's review is settled: %v", n, err)
			}
		}
		if said := w.passStep(slug, n, "ws", fmt.Sprintf("its checks are met (round %d)", round.N), mission.StepWork); said != "" {
			return said
		}
		return fmt.Sprintf("step %d's checks are met", n)
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
	return w.passStep(slug, n, "you", why, cur.Step.Type)
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
