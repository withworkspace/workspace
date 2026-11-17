package ops

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
)

// Verify's expansion: what ws does with a plan's Verify step. Reached, it
// starts a verifier, fresh crew who didn't do the work, in the capsule the
// work is in. The verifier's done is its bundle, handed over: ws then runs
// the step's commands itself and takes the rest from the bundle. Every
// check met, or left for the user (can't tell, yours), and ws passes the
// step; one unmet, and the work goes back to the crew who made it, until
// [missions] max_rounds have come round, then it's the user's. When that
// crew report done, the work is verified again by a fresh verifier, whose
// bundle takes the last one's place. A verifier changes nothing: ws
// refuses its bundle if the capsule has moved since it started.

// verifierAt reports whether crew is a verifier: on the record of a Verify
// step, which is then the step they verify.
func verifierAt(states []mission.StageState, crewID string) (mission.StageState, bool) {
	st, ok := mission.StageFor(states, crewID)
	if !ok || st.Step == nil || st.Step.Type != mission.StepVerify {
		return mission.StageState{}, false
	}
	return st, true
}

// Verifying reports whether crew is a verifier on the mission: its done is
// its bundle, not a handoff.
func (w *Workspace) Verifying(m *mission.Mission, crewID string) bool {
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	_, ok := verifierAt(states, crewID)
	return ok
}

// workCrew is the crew who made the work a Verify step verifies: the
// latest crew working on the mission in the capsule who aren't its
// verifiers.
func workCrew(m *mission.Mission, st mission.StageState, capsule string) (mission.Crew, bool) {
	for i := len(m.Crew) - 1; i >= 0; i-- {
		c := m.Crew[i]
		if c.Role != "" || c.Capsule != capsule || st.Record != nil && slices.Contains(st.Record.Crew, c.ID) {
			continue
		}
		return c, true
	}
	return mission.Crew{}, false
}

// startVerifier starts a verifier at a Verify step: fresh crew in the
// capsule the work is in, with the commit it's at kept, as what their
// bundle has to be of. It says what it did.
func (w *Workspace) startVerifier(m *mission.Mission, st mission.StageState) string {
	capsule, ok := w.stepCapsule(m, st)
	if !ok {
		return fmt.Sprintf("step %d verifies work in a capsule the mission hasn't got, so ws didn't start it: dispatch it with --capsule <repo>/<name>", st.N)
	}
	repo, name, _ := strings.Cut(capsule, "/")
	head := gitOut(filepath.Join(w.WS().RepoDir(repo), name), "rev-parse", "HEAD")
	if err := w.Missions.SetVerifyHead(m.Slug, st.N, head); err != nil {
		return fmt.Sprintf("couldn't start step %d: %v", st.N, err)
	}
	return w.startFresh(m, st, capsule, "")
}

// verifying is what a verifier at step st, in the capsule at dir, is told.
func (w *Workspace) verifying(m *mission.Mission, states []mission.StageState, st mission.StageState, capsule, dir, brief, guide string) prompt.Verifying {
	v := prompt.Verifying{Slug: m.Slug, Title: m.Title, Goal: briefGoal(brief), StepN: st.N, StepOf: len(states), Step: st.Text,
		Limit: st.Step.LimitText, Capsule: capsule, Commit: gitOut(dir, "rev-parse", "HEAD"), Files: w.Missions.FilesDir(m.Slug), Guide: guide}
	for _, c := range st.Step.Checks {
		v.Checks = append(v.Checks, prompt.VerifyCheck{Text: c.Text, Command: c.Run(), Yours: c.Yours})
	}
	if c, ok := workCrew(m, st, capsule); ok {
		v.Worker = c.ID
		if c.HandedOff != nil {
			v.Handoff = mission.HandoffRef(c.ID)
		}
	}
	// The bundle of this step that went back before: what it found unmet.
	if before := m.StandingBundles(st.Text); len(before) > 0 {
		b := before[len(before)-1]
		v.Before = b.ID
		for _, c := range b.With(mission.CheckUnmet) {
			v.Unmet = append(v.Unmet, fmt.Sprintf("check %d (%s): %s", c.N, oneLine(firstNonEmpty(c.Check, c.Said)), oneLine(c.Said)))
		}
	}
	return v
}

// verifierMayHandOver holds a verifier's bundle to the commit its capsule
// was at when it started: a verifier changes nothing.
func verifierMayHandOver(st mission.StageState, capsule, head string) error {
	if st.Record == nil || st.Record.VerifyHead == "" || head == "" || st.Record.VerifyHead == head {
		return nil
	}
	return refused("verifier-changed", "%s was at %s when you started, and is at %s now: a verifier changes nothing, so its bundle is of the work as it was. If the commits are yours, take them out (what isn't right goes back to the crew who made it, in your bundle); if they're someone else's, say so in your reply and stop",
		capsule, shortSHA(st.Record.VerifyHead), shortSHA(head))
}

// settleVerify settles a Verify step on its verifier's done: ws runs the
// step's commands in the capsule, takes what its other checks came to from
// the bundle the verifier handed over, keeps the round on the step's
// record, and acts on it (afterVerify). It says what it did.
func (w *Workspace) settleVerify(m *mission.Mission, st mission.StageState, crewID, capsule, dir, commit string) (string, error) {
	var bundle *mission.Bundle
	for i := len(m.Bundles) - 1; i >= 0 && bundle == nil; i-- {
		if b := m.Bundles[i]; b.Crew == crewID && b.Stage == st.Text {
			bundle = &b
		}
	}
	by := "the verifier (" + crewID + ")"
	results := make([]mission.CheckResult, len(st.Step.Checks))
	run := w.checkRound(m.Slug, st, crewID, nil)
	defer w.Missions.ClearChecking(m.Slug, st.N) // recording the round clears it; failing to, this does
	for i, c := range st.Step.Checks {
		switch {
		case c.Run() && dir == "":
			results[i] = mission.CheckResult{Check: c.Text, Result: mission.CheckCantTell, By: "ws", Evidence: "the step has no capsule to run it in"}
		case c.Run():
			results[i] = run.command(i, c, dir, crew.Env(w.Root(), m.Slug, ""))
		case bundle == nil:
			results[i] = mission.CheckResult{Check: c.Text, Result: mission.CheckCantTell, By: "ws", Evidence: "its verifier reported done without handing over a bundle"}
		default:
			results[i] = mission.CheckResult{Check: c.Text, Result: mission.CheckCantTell, By: by, Evidence: "the bundle says nothing of it"}
			for _, bc := range bundle.Checks {
				if bc.N == i+1 {
					results[i].Result, results[i].Evidence = bc.Verdict, bc.Said
				}
			}
		}
	}
	done := ""
	if c, ok := m.CrewByID(crewID); ok {
		done = c.DoneKey()
	}
	round, err := w.Missions.RecordChecks(m.Slug, st.N, mission.CheckRound{Crew: crewID, Done: done, Capsule: capsule, Commit: commit, Results: results})
	if err != nil {
		return "", err
	}
	id := ""
	if bundle != nil {
		id = bundle.ID
	}
	return w.afterVerify(m.Slug, st.N, round, id), nil
}

// afterVerify acts on a round of a Verify step: nothing unmet, ws passes
// the step (what's left for the user is theirs when they look at the
// bundle); something unmet, the work goes back to the crew who made it,
// until max_rounds have come round, or there's nobody to send it to, and
// then it's the user's. It says what it did.
func (w *Workspace) afterVerify(slug string, n int, round mission.CheckRound, bundle string) string {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return ""
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.N != n {
		return ""
	}
	shown := "its verifier's bundle"
	if bundle != "" {
		shown = bundle
	}
	if len(round.With(mission.CheckUnmet)) == 0 {
		if err := w.acceptAllowed(m); err != nil {
			return fmt.Sprintf("step %d is verified, and passes once the user's review is settled: %v", n, err)
		}
		why := fmt.Sprintf("%s shows it (%s)", shown, round.Summary())
		if said := w.passStep(slug, n, "ws", why, mission.StepVerify); said != "" {
			return said
		}
		return fmt.Sprintf("step %d is verified: %s", n, round.Summary())
	}
	rounds := 0
	if cur.Record != nil {
		for _, r := range cur.Record.Checks {
			if r.Capsule == round.Capsule {
				rounds++
			}
		}
	}
	round.N = max(rounds, 1)
	worker, ok := workCrew(m, cur, round.Capsule)
	switch {
	case !ok:
		return w.askAboutVerify(slug, n, round, shown, "no crew made the work in "+round.Capsule+" to send it back to")
	case round.N >= w.Config().MaxRounds():
		return w.askAboutVerify(slug, n, round, shown, fmt.Sprintf("that's %d %s unmet, as many as ws lets come round (%s)", round.N, plural(round.N, "round"), guaranteeMaxRounds(w.Config().MaxRounds()).Key))
	}
	said, err := w.sendBackVerify(m, cur, round, worker, AsWS)
	if err != nil {
		return w.askAboutVerify(slug, n, round, shown, fmt.Sprintf("ws couldn't send it back to %s (%v)", worker.ID, err))
	}
	return fmt.Sprintf("step %d isn't verified (%s): %s", n, round.Summary(), said)
}

// askAboutVerify makes a Verify step yours, its work not verified: pass
// it as it is, send the work back with what's unmet, or say what to do.
func (w *Workspace) askAboutVerify(slug string, n int, round mission.CheckRound, shown, why string) string {
	text := fmt.Sprintf("Step %d's verification, round %d (%s): %s. %s. Pass it as it is, send the work back with what's unmet, or say what to do.", n, round.N, shown, round.Summary(), upperFirst(why))
	if _, err := w.Missions.AskAtStep(slug, mission.TaskChecks, text, n, mission.ChecksPass, mission.ChecksSendBack); err != nil {
		return "couldn't ask the user: " + err.Error()
	}
	return fmt.Sprintf("step %d isn't verified (%s): it's the user's, as %s", n, round.Summary(), why)
}

// sendBackVerify sends the work a Verify step found unmet back to the crew
// who made it, with what's unmet and what the verifier said: typed in if
// they're running, resumed if they've ended, else fresh crew in their
// capsule. They carry on at the step they did; their done verifies the
// work again.
func (w *Workspace) sendBackVerify(m *mission.Mission, st mission.StageState, round mission.CheckRound, worker mission.Crew, by Actor) (string, error) {
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	at, ok := mission.StageFor(states, worker.ID)
	if !ok {
		at = st
	}
	var t strings.Builder
	fmt.Fprintf(&t, "Step %d verified your work in %s (round %d, by %s, who didn't do it), and it isn't right yet.\n\n", st.N, round.Capsule, round.N, round.Crew)
	for _, r := range round.Results {
		if r.Result != mission.CheckUnmet {
			continue
		}
		fmt.Fprintf(&t, "- %s: unmet", r.Check)
		if ev := strings.TrimSpace(r.Evidence); ev != "" {
			fmt.Fprintf(&t, ". %s", clipString(oneLine(ev), 600))
		}
		t.WriteString("\n")
	}
	for i := len(m.Bundles) - 1; i >= 0; i-- {
		if b := m.Bundles[i]; b.Crew == round.Crew && b.Stage == st.Text {
			fmt.Fprintf(&t, "\n`ws show -f llm %s` has what the verifier saw, with its files.", b.ID)
			break
		}
	}
	t.WriteString("\n\nPut it right, commit, and report done again with a handoff (ws mission done): ws has it verified again, afresh.")
	return w.carryOn(m, at, worker.ID, t.String(), by)
}

// verifyAgain starts a fresh verifier when the crew who made the work a
// Verify step verifies report done at it: the work came back to them, and
// they've put it right. It says what it did, or "" when that's not what
// their done is.
func (w *Workspace) verifyAgain(m *mission.Mission, crewID string) string {
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.Step.Type != mission.StepVerify || cur.Held() {
		return ""
	}
	c, ok := m.CrewByID(crewID)
	if !ok || c.Role != "" || cur.Record != nil && slices.Contains(cur.Record.Crew, crewID) {
		return ""
	}
	if capsule, ok := w.stepCapsule(m, cur); !ok || capsule != c.Capsule {
		return ""
	}
	// What ws asked the user of the last round is moot: it's verified again.
	w.Missions.CloseStepTasks(m.Slug, cur.N)
	return fmt.Sprintf("step %d verifies it again: %s", cur.N, w.startVerifier(m, cur))
}
