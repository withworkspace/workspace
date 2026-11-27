package ops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
)

// The orchestrator's decisions on the plan. A step that applies only if
// something holds (- only if:), or runs in each of several repos (- in
// each:), waits at its start for someone's call: the orchestrator's, by
// the plan's words and the process, or with it off, yours. They start it
// (in the repos its words name, one run in each), or skip it, saying why.
// The orchestrator may start the Work step the mission's at again, fresh,
// within max_rounds. ws records each call.

// holdStep is the mission reaching a step that waits for someone's call:
// the orchestrator's, or with it off, yours. It says what it did.
func (w *Workspace) holdStep(m *mission.Mission, done string, st mission.StageState) string {
	if w.Notify(m.Slug, mission.OrchestratorEvent{Trigger: "step"}) {
		return fmt.Sprintf("step %d waits for the orchestrator's call: %s", st.N, heldWhy(st))
	}
	return w.askHeld(m, done, st)
}

// askHeld asks you to make the call at a step that waits for one. It says
// what it did.
func (w *Workspace) askHeld(m *mission.Mission, done string, st mission.StageState) string {
	text := fmt.Sprintf("%sStep %d %s: %s", done, st.N, heldWhy(st), sentence(st.Plain()))
	options := []string{mission.CheckInGoOn, mission.StepSkip}
	if st.Step.InEach != "" {
		text += " Name the repos to run in, comma-separated"
		options = nil
		if st.Step.OnlyIf != "" {
			text += ", or skip"
			options = []string{mission.StepSkip}
		}
		text += "."
	} else {
		text += " Go on to start, or skip."
	}
	if options == nil {
		options = []string{mission.CheckInNotYet}
	}
	if _, err := w.Missions.AskAtStep(m.Slug, mission.TaskApprove, text, st.N, options...); err != nil {
		return "could not ask the user: " + err.Error()
	}
	return fmt.Sprintf("step %d waits for the user's call: %s", st.N, heldWhy(st))
}

// heldWhy is why a step waits for a call, in words.
func heldWhy(st mission.StageState) string { return st.HeldWhy() }

// StartStep starts the step the mission's at, which waited for someone's
// call: in repos, one run in each, for a step that runs in several. For
// the orchestrator, and you. It says what it did.
func (w *Workspace) StartStep(slug string, repos []string, by Actor) (string, error) {
	if by.Kind == Crew || by.Kind == WS {
		return "", refused("not-yours", "the orchestrator, or the user, starts a step that waits for a call")
	}
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	cur, ok := currentStep(w.Missions, m)
	switch {
	case !ok:
		return "", refused("no-step", "the mission has no step to start")
	case !cur.Held():
		return "", refused("not-held", "step %d doesn't wait for anyone's call: ws starts it when the mission reaches it (it has, or does once the step before is passed)", cur.N)
	case cur.Step.InEach != "" && len(repos) == 0:
		return "", refused("which-repos", "step %d runs in each %s: name the repos it runs in (--in <repo>, for each)", cur.N, strings.TrimSuffix(cur.Step.InEach, "."))
	case cur.Step.InEach == "" && len(repos) > 0:
		return "", refused("one-place", "step %d runs in one place, as the plan says: start it without --in", cur.N)
	}
	var named []string
	for _, r := range repos {
		repo, ok := w.WS().ResolveAlias(strings.TrimSpace(r))
		if !ok {
			return "", invalid("%s isn't a repo of this workspace", r)
		}
		if !slices.Contains(named, repo) {
			named = append(named, repo)
		}
	}
	if err := w.Missions.StartStep(m.Slug, cur.N, by.By(), named); err != nil {
		return "", wrap(err)
	}
	w.closeStepTasks(m.Slug, cur.N)
	w.youActed(m.Slug, by)
	if m, err = w.Missions.Load(m.Slug); err != nil {
		return "", err
	}
	cur, _ = currentStep(w.Missions, m)
	return w.reachStep(m, mission.StageState{}, cur), nil
}

// Skip passes the step the mission's at without its being done, saying
// why: for the orchestrator, a step that applies only if, and doesn't, and
// isn't begun; you may skip any. It says what ws did next.
func (w *Workspace) Skip(slug, why string, by Actor) (string, error) {
	if by.Kind == Crew || by.Kind == WS {
		return "", refused("not-yours", "the orchestrator skips a step whose only if doesn't hold, or the user does")
	}
	why = strings.TrimSpace(why)
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok {
		return "", refused("no-step", "the mission has no step to skip")
	}
	if by.Kind == Orchestrator {
		switch {
		case cur.Step.OnlyIf == "":
			return "", refused("always-runs", "step %d has no only if: it runs. If the plan no longer fits, ask the planner (ws mission replan)", cur.N)
		case cur.Record != nil && (len(cur.Record.Crew) > 0 || len(cur.Record.Instances) > 0 && cur.Record.Go != nil):
			return "", refused("begun", "step %d is begun: it's passed when it's done, or the plan changes", cur.N)
		case why == "":
			return "", invalid("say why it doesn't apply")
		}
	}
	if why == "" {
		why = "skipped by the user"
	}
	if err := w.Missions.SkipStep(m.Slug, cur.N, by.By(), why); err != nil {
		return "", wrap(err)
	}
	w.closeStepTasks(m.Slug, cur.N)
	w.youActed(m.Slug, by)
	m, err = w.Missions.Load(m.Slug)
	if err != nil {
		return "", err
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	acc := states[cur.N-1]
	return fmt.Sprintf("skipped step %d, and %s", cur.N, w.afterAccept(m.Slug, acc, states)), nil
}

// Repeat starts the Work step the mission's at again, with fresh crew (in
// repo, for a step that runs in several, or "" for all its runs), on what
// on says ("" for the step's on:): another attempt, saying why. The orchestrator may, within max_rounds attempts;
// after that it's yours. It says what it did.
func (w *Workspace) Repeat(slug, repo, on, why string, by Actor) (string, error) {
	if by.Kind == Crew || by.Kind == WS {
		return "", refused("not-yours", "the orchestrator, or the user, starts a step again")
	}
	why = strings.TrimSpace(why)
	if why == "" {
		return "", invalid("say why it's started again")
	}
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	cur, ok := currentStep(w.Missions, m)
	switch {
	case !ok:
		return "", refused("no-step", "the mission has no step to start again")
	case cur.Step.Type != mission.StepWork:
		return "", refused("not-work", "step %d is a %s step: only Work is started again, with fresh crew", cur.N, cur.Step.Type)
	case cur.Record == nil || len(cur.Record.Crew) == 0:
		return "", refused("not-begun", "step %d isn't begun: there's nothing to start again", cur.N)
	case by.Kind == Orchestrator && cur.Record.Attempts >= w.Config().MaxRounds():
		return "", refused("max-rounds", "step %d has been started %s already, as many as ws lets come round (%s): it's the user's, so flag them", cur.N, plural(cur.Record.Attempts+1, "time"), guaranteeMaxRounds(w.Config().MaxRounds()).Key)
	}
	if repo != "" {
		r, ok := w.WS().ResolveAlias(repo)
		if _, in := cur.Instance(r); !ok || !in {
			return "", invalid("step %d doesn't run in %s", cur.N, repo)
		}
		repo = r
	}
	if on != "" {
		if err := w.allowsCrew(w.CrewPick(on)); err != nil {
			return "", err
		}
	}
	attempt, err := w.Missions.RepeatStep(m.Slug, cur.N, repo, by.By(), why)
	if err != nil {
		return "", wrap(err)
	}
	w.youActed(m.Slug, by)
	if m, err = w.Missions.Load(m.Slug); err != nil {
		return "", err
	}
	cur, _ = currentStep(w.Missions, m)
	var said string
	if len(cur.Instances()) > 0 {
		said = w.startInstances(m, cur, repo, on)
	} else {
		said = w.startFresh(m, cur, "", on)
	}
	return fmt.Sprintf("attempt %d at step %d: %s", attempt, cur.N, said), nil
}

// instanceRepos are the repos a step runs in.
func instanceRepos(st mission.StageState) []string {
	var out []string
	for _, in := range st.Instances() {
		out = append(out, in.Repo)
	}
	return out
}

// startInstances starts a Work step's run in each of its repos (or in
// only, alone) with fresh crew, lifting a capsule in each repo the mission
// has none in. It says what it did.
func (w *Workspace) startInstances(m *mission.Mission, st mission.StageState, only, on string) string {
	var said []string
	launched := 0
	queued := !w.Config().OrchestratorAutoDispatch() || w.opts.Host == nil
	for _, in := range st.Instances() {
		if only != "" && in.Repo != only || in.Done != nil && only == "" {
			continue
		}
		if queued && len(said) > 0 {
			// Crew wait for you one at a time: the next run's are queued
			// when these are launched (queueNextRun).
			said = append(said, "the rest queue one at a time as each launches")
			break
		}
		capsule, ok := w.capsuleIn(m, st, in.Repo, true)
		if !ok {
			said = append(said, "no capsule in "+in.Repo)
			continue
		}
		logFailed(w.Missions.SetInstance(m.Slug, st.N, in.Repo, capsule, false))
		if m2, err := w.Missions.Load(m.Slug); err == nil {
			m = m2
			st, _ = currentStep(w.Missions, m)
		}
		s := w.startFresh(m, st, capsule, on)
		w.oneDispatch(m.Slug, &launched, s)
		said = append(said, s)
	}
	return strings.Join(said, "; ")
}

// oneDispatch keeps a step's runs, started together, one dispatch toward
// those ws may start without you: each launched after the first gives its
// count back. launched counts them; said is what starting one said.
func (w *Workspace) oneDispatch(slug string, launched *int, said string) {
	if !strings.HasPrefix(said, "started ") && !strings.HasPrefix(said, "resumed ") && !strings.Contains(said, ": started ") {
		return
	}
	if *launched > 0 {
		logFailed(w.Missions.ReleaseDispatch(slug))
	}
	*launched++
}

// carryOnInstances starts a step that carries on from one that ran in
// several repos: it runs in the same repos, fresh crew in each run's
// capsule given its crew's handoff. It says what it did.
func (w *Workspace) carryOnInstances(m *mission.Mission, from, st mission.StageState) string {
	repos := instanceRepos(from)
	if err := w.Missions.StartStep(m.Slug, st.N, "ws", repos); err != nil {
		return "could not start in " + strings.Join(repos, ", ") + ": " + err.Error()
	}
	var said []string
	launched := 0
	for _, in := range from.Instances() {
		id := ""
		for i := len(from.Record.Crew) - 1; i >= 0 && id == ""; i-- {
			if c, ok := m.CrewByID(from.Record.Crew[i]); ok && c.Role == "" && strings.HasPrefix(c.Capsule, in.Repo+"/") {
				id = c.ID
			}
		}
		logFailed(w.Missions.SetInstance(m.Slug, st.N, in.Repo, in.Capsule, false))
		m2, err := w.Missions.Load(m.Slug)
		if err != nil {
			continue
		}
		cur, _ := currentStep(w.Missions, m2)
		c, ok := m2.CrewByID(id)
		if !ok {
			s := w.startFresh(m2, cur, in.Capsule, "")
			w.oneDispatch(m.Slug, &launched, s)
			said = append(said, s)
			continue
		}
		// Fresh crew in the run's capsule, given its crew's handoff
		// (carryOnAt says why).
		why := fmt.Sprintf("Step %d is done, and you carry on from %s, who did it in `%s`", from.N, c.ID, in.Repo)
		s, err := w.carryOnFresh(m2, cur, c, why, "Commit as you go, and report done with a handoff when it's finished (ws mission done).", AsWS)
		if err != nil {
			s = fmt.Sprintf("could not start step %d in %s to carry on from %s: %v", st.N, in.Repo, c.ID, err)
		}
		w.oneDispatch(m.Slug, &launched, s)
		said = append(said, s)
	}
	return strings.Join(said, "; ")
}

// capsuleIn is the mission's capsule in repo for a step, lifted for it
// where the mission has none there and lift says to.
func (w *Workspace) capsuleIn(m *mission.Mission, st mission.StageState, repo string, lift bool) (string, bool) {
	there := st
	step := *st.Step
	step.Capsule, step.InEach = repo, ""
	there.Step = &step
	if c, ok := w.stepCapsule(m, there); ok {
		return c, true
	}
	if !lift || w.liftForStep(m.Slug, there) == "" {
		return "", false
	}
	m2, err := w.Missions.Load(m.Slug)
	if err != nil {
		return "", false
	}
	return w.stepCapsule(m2, there)
}

// stepCapsules are the capsules a step works in: each of its runs', for a
// step that runs in several repos, or the one it names.
func (w *Workspace) stepCapsules(m *mission.Mission, st mission.StageState) []string {
	if ins := st.Instances(); len(ins) > 0 {
		var out []string
		for _, in := range ins {
			if in.Capsule != "" {
				out = append(out, in.Capsule)
			} else if c, ok := w.capsuleIn(m, st, in.Repo, false); ok {
				out = append(out, c)
			}
		}
		return out
	}
	if c, ok := w.stepCapsule(m, st); ok {
		return []string{c}
	}
	return nil
}

// instanceDone records a step's run in capsule's repo done, and passes
// the step once every run is (by, why, for the record). It says what it
// did, or "" for a step that doesn't run in several repos.
func (w *Workspace) instanceDone(slug string, st mission.StageState, capsule, by, why string) string {
	in, ok := st.Instance(capsule)
	if !ok {
		return ""
	}
	all, err := w.Missions.SetInstance(slug, st.N, in.Repo, capsule, true)
	if err != nil {
		return "could not mark run done: " + err.Error()
	}
	if !all {
		m, _ := w.Missions.Load(slug)
		cur, _ := currentStep(w.Missions, m)
		return fmt.Sprintf("step %d done in %s, still running in %s", st.N, in.Repo, strings.Join(cur.InstancesLeft(), ", "))
	}
	if said := w.passStep(slug, st.N, by, why+", in every repo", st.Step.Type); said != "" {
		return said
	}
	return fmt.Sprintf("step %d done in every repo", st.N)
}

// answerHeld answers your task at a step that waits for your call: go on
// starts it, skip it skips it, and repos named start it in each. Anything
// else is answered as at any step of yours (ok false).
func (w *Workspace) answerHeld(m *mission.Mission, t mission.Task, by Actor, text string) (Answered, bool, error) {
	out := Answered{Slug: m.Slug}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.N != t.Step || !cur.Held() {
		return out, false, nil
	}
	var said string
	var err error
	switch {
	case strings.EqualFold(text, mission.StepSkip):
		said, err = w.Skip(m.Slug, "", by)
	case strings.EqualFold(text, mission.CheckInGoOn) && cur.Step.InEach == "":
		said, err = w.StartStep(m.Slug, nil, by)
	case cur.Step.InEach != "":
		repos, ok := w.reposIn(text)
		if !ok {
			return out, false, nil
		}
		said, err = w.StartStep(m.Slug, repos, by)
	default:
		return out, false, nil
	}
	if err != nil {
		return out, true, err
	}
	out.Next = said
	return out, true, nil
}

// reposIn reads repos named in an answer ("api, web"): ok only if every
// word names one.
func (w *Workspace) reposIn(text string) ([]string, bool) {
	words := strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == ';' })
	var out []string
	for _, word := range words {
		word = strings.Trim(word, "`.")
		if word == "" || strings.EqualFold(word, "and") {
			continue
		}
		repo, ok := w.WS().ResolveAlias(word)
		if !ok {
			return nil, false
		}
		out = append(out, repo)
	}
	return out, len(out) > 0
}

// queueNextRun queues crew for the next run of the step the mission's at,
// one that runs in several repos, that has none yet: crew waiting for you
// are queued one at a time. It says what it did, or "".
func (w *Workspace) queueNextRun(slug string) string {
	m, err := w.Missions.Load(slug)
	if err != nil || m.Dispatch != nil {
		return ""
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.Step.Type != mission.StepWork || len(cur.Instances()) == 0 {
		return ""
	}
	for _, in := range cur.Instances() {
		if in.Done != nil || in.Capsule != "" && w.crewInCapsule(m, cur, in.Capsule) {
			continue
		}
		capsule, ok := w.capsuleIn(m, cur, in.Repo, true)
		if !ok {
			continue
		}
		logFailed(w.Missions.SetInstance(slug, cur.N, in.Repo, capsule, false))
		if m, err = w.Missions.Load(slug); err != nil {
			return ""
		}
		cur, _ = currentStep(w.Missions, m)
		return w.startFresh(m, cur, capsule, "")
	}
	return ""
}

// crewInCapsule reports whether work crew of the step have worked in
// capsule.
func (w *Workspace) crewInCapsule(m *mission.Mission, st mission.StageState, capsule string) bool {
	if st.Record == nil {
		return false
	}
	for _, id := range st.Record.Crew {
		if c, ok := m.CrewByID(id); ok && c.Role == "" && c.Capsule == capsule {
			return true
		}
	}
	return false
}

// launchAtHeld is crew launched at the step the mission's at, when it
// waits for a call: launching it is your call, where it applies only if
// (it's started); where it runs in several repos, you're asked which, and
// it's refused until you've said. The orchestrator makes its call first
// (ws mission start).
func (w *Workspace) launchAtHeld(m *mission.Mission, d Dispatch, by Actor) error {
	cur, ok := currentStep(w.Missions, m)
	if !ok || !cur.Held() || d.Stage != 0 && d.Stage != cur.N {
		return nil
	}
	switch {
	case !by.IsYou():
		return refused("held", "step %d waits for a call: it %s. Start it (ws mission start) or skip it (ws mission skip \"<why>\") first", cur.N, heldWhy(cur))
	case cur.Step.InEach != "":
		if _, ok := m.StepTask(cur.N); !ok {
			w.askHeld(m, "", cur)
		}
		return refused("which-repos", "step %d runs in each %s: name the repos in the step's task (or ws mission start --in <repo>, for each)", cur.N, strings.TrimSuffix(cur.Step.InEach, "."))
	}
	return w.Missions.StartStep(m.Slug, cur.N, by.By(), nil)
}
