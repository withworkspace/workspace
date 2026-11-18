package ops

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/review"
)

// Accepted is a stage accepted, and what ws did after.
type Accepted struct {
	Slug     string
	Stage    mission.StageState
	Of       int  // how many stages the plan has
	Implicit bool // a brief without stages: the whole work
	// Next is what ws did to move the plan on: started the next stage,
	// asked you first, or nothing (the last stage). "" when Implicit.
	Next string
}

func (a Accepted) String() string {
	if a.Implicit {
		return "Accepted the work."
	}
	return fmt.Sprintf("Accepted %s %d: %s.", a.Stage.Noun(), a.Stage.N, a.Next)
}

// AcceptStage accepts the mission's current stage (or its plan's step):
// its work is complete and good enough to build on. For the orchestrator,
// and you; refused while the stage's crew are still on it, a review of its
// capsules asks for changes, or a change asked for on one of its documents
// is open. A step of the plan that's yours (a checkpoint) only you pass.
// Then ws moves the plan on (see afterAccept). why may be empty.
func (w *Workspace) AcceptStage(slug string, by Actor, why string) (Accepted, error) {
	if by.Kind != You && by.Kind != Orchestrator {
		return Accepted{}, refused("not-yours", "crew report done (ws mission done); the orchestrator or the user accepts the stage")
	}
	m, err := w.load(slug)
	if err != nil {
		return Accepted{}, err
	}
	if by.Kind == Orchestrator {
		if err := orchestratorMayAccept(w.Missions, m); err != nil {
			return Accepted{}, err
		}
		if cur, ok := currentStep(w.Missions, m); ok {
			if err := w.stepPRs(m, cur); err != nil {
				return Accepted{}, err
			}
		}
	}
	if err := w.acceptAllowed(m); err != nil {
		return Accepted{}, err
	}
	acc, states, err := w.Missions.AcceptStage(m.Slug, by.By(), strings.TrimSpace(why))
	if err != nil {
		return Accepted{}, wrap(err)
	}
	w.youActed(m.Slug, by)
	// The step's done: what ws asked you about it is answered.
	w.Missions.CloseStepTasks(m.Slug, acc.N)
	out := Accepted{Slug: m.Slug, Stage: acc, Of: len(states), Implicit: acc.Implicit}
	if !acc.Implicit {
		out.Next = w.afterAccept(m.Slug, acc, states)
	}
	return out, nil
}

// orchestratorMayAccept refuses the orchestrator passing a step of the
// plan that's yours (a checkpoint) or the planner's: both wait on you.
func orchestratorMayAccept(store mission.Store, m *mission.Mission) error {
	brief, _ := store.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	cur, ok := mission.CurrentStage(states)
	if !ok || cur.Step == nil {
		return nil
	}
	if cur.Held() {
		return refused("held", "step %d waits for your call: it %s. Start it (ws mission start%s) or skip it (ws mission skip \"<why>\"), as its words and the process say", cur.N, heldWhy(cur), map[bool]string{true: " --in <repo>, for each", false: ""}[cur.Step.InEach != ""])
	}
	if left := cur.InstancesLeft(); len(left) > 0 {
		return refused("runs-on", "step %d runs in each of %s, and isn't done yet in %s: it's passed when it's done in every one", cur.N, strings.Join(instanceRepos(cur), ", "), strings.Join(left, ", "))
	}
	if cur.Step.Type == mission.StepWork && len(cur.Step.Checks) > 0 && len(cur.Instances()) == 0 && !cur.ChecksMet() {
		why := "they haven't been run: ws runs them when its crew report done"
		if r, ok := cur.Record.LatestChecks(); ok {
			why = fmt.Sprintf("round %d came to %s", r.N, r.Summary())
		}
		return refused("checks", "step %d has checks, and ws passes it once they're all met (%s). Send it back to its crew with what's unmet (ws mission resume-with), or flag the user, as the process says; the user can pass it as it is", cur.N, why)
	}
	if cur.Step.Type == mission.StepVerify {
		return refused("verify", "step %d is a Verify step: ws passes it on its verifier's bundle, once nothing in it is unmet, and sends the work back to its crew when something is. There's nothing for you to accept: note what it waits on, or flag the user", cur.N)
	}
	switch cur.Step.Who {
	case mission.WhoYou:
		return refused("checkpoint", "step %d is the user's, a checkpoint: only they pass it, by going on when ws asks them (it has, or does once the step before is accepted). There's nothing for you to do until they have: note that it waits for them", cur.N)
	case mission.WhoPlanner:
		return refused("planner-step", "step %d is the planner's: it writes the steps after it, and ws passes the step once it has (or the user goes on). Note that it waits for the planner", cur.N)
	}
	return nil
}

// acceptAllowed refuses accepting the current stage while the latest
// review of one of its capsules asks for changes, or a change asked for on
// one of its documents is open: crew addressing a review or a thread isn't
// the user being satisfied with it. They re-review and approve, or resolve
// the thread.
func (w *Workspace) acceptAllowed(m *mission.Mission) error {
	blocking, err := review.For(w.Root(), m.Slug).Blocking(StageCapsules(w.Missions, m))
	if err != nil {
		return err
	}
	if len(blocking) > 0 {
		r := blocking[0]
		t := r.Tally()
		return refused("changes-asked", "the user asked for changes in review %s of %s (%d of %d comments answered): they re-review it, and approve, before this stage is accepted. ws shows them it's ready once crew have committed and reported done; there's nothing more to do until then",
			r.ID, r.Capsule, t.Answered(), t.Total())
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	_, docs := m.CurrentStageArtifacts(brief)
	if len(docs) == 0 {
		return nil
	}
	threads, err := conversation.For(w.Root(), m.Slug).List()
	if err != nil {
		return err
	}
	for _, a := range docs {
		if open := conversation.OpenChanges(threads, a.Ref); len(open) > 0 {
			return refused("changes-open", "the user asked for changes to %s that are still open (%s): the stage is accepted once they resolve them. ws show %s says where each stands; there's nothing more to do until then",
				a.Ref, strings.Join(open, ", "), open[0])
		}
	}
	return nil
}

// StageCapsules are the capsules the mission's current stage works in: the
// ones it names, or with none named (or no stages), all of them.
func StageCapsules(store mission.Store, m *mission.Mission) []string {
	var all []string
	for _, c := range m.Capsules {
		all = append(all, c.Repo+"/"+c.Name)
	}
	brief, _ := store.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	cur, ok := mission.CurrentStage(states)
	if !ok || cur.Implicit {
		return all
	}
	var named []string
	for _, n := range cur.NamedCapsules() {
		if slices.Contains(all, n) {
			named = append(named, n)
		}
	}
	if len(named) == 0 {
		return all
	}
	return named
}

// ws moves the brief's plan on itself once a stage is accepted, taking the
// steps that are bookkeeping: it starts the next stage, in the capsule the
// stage names, with its text; or, when the accepted stage says to check in
// (or ws has started as many crew without you as the orchestrator may), it
// asks you first, and your "Go on" starts it.

// afterAccept moves the plan on after a stage is accepted, and says what
// it did.
func (w *Workspace) afterAccept(slug string, acc mission.StageState, states []mission.StageState) string {
	next, ok := mission.CurrentStage(states)
	if !ok {
		if acc.Step != nil {
			return "that was the plan's last step: the mission is at Z, if what Z says is true. Check each part of Z, and say how it ends (ws mission conclude --as …, or the user's ws mission end)"
		}
		return "that was the last stage"
	}
	m, err := w.Missions.Load(slug)
	if err != nil {
		return "couldn't read the mission to start the next stage: " + err.Error()
	}
	if acc.CheckIn {
		text := fmt.Sprintf("Stage %d is done%s. Go on to stage %d: %s?", acc.N, stageDoneSummary(m, acc), next.N, next.Short())
		if _, err := w.Missions.AskAtStep(slug, mission.TaskApprove, text, next.N); err != nil {
			return "couldn't ask the user: " + err.Error()
		}
		return fmt.Sprintf("stage %d said to check in, so the user is asked before stage %d starts", acc.N, next.N)
	}
	return w.reachStep(m, acc, next)
}

// reachStep is the mission reaching a stage, or a step of its plan, by its
// kind (steps.go): crew are started on Work; you're asked at Review,
// Decide and Land; the planner is asked to write the steps after its own,
// and ws passes it once it has; ws opens an Open Pull Request step's pull
// request, and passes the step when it merges; a Wait is waited for, and
// the orchestrator passes it once it's happened. It says what it did.
func (w *Workspace) reachStep(m *mission.Mission, acc, next mission.StageState) string {
	if next.Step == nil {
		return w.startStage(m, next)
	}
	done := ""
	if acc.N > 0 {
		done = fmt.Sprintf("Step %d is done%s. ", acc.N, stageDoneSummary(m, acc))
		if acc.Skipped() {
			done = fmt.Sprintf("Step %d is skipped. ", acc.N)
		}
	}
	if next.Held() {
		// It applies only if, or runs in each of several repos: whoever
		// makes the call starts it (steprun.go).
		return w.holdStep(m, done, next)
	}
	switch next.Step.Type {
	case mission.StepReview, mission.StepDecide, mission.StepLand:
		return w.askAtStep(m, done, next)
	case mission.StepPlan:
		said, err := w.plannerAtStep(m, done, next)
		if err == nil {
			return said
		}
		// The planner can't be asked: the user writes what's after it.
		text := fmt.Sprintf("%sStep %d is the planner's: %s The planner couldn't be asked (%v): write the steps after it (edit the plan), then go on.", done, next.N, sentence(next.Plain()), err)
		if _, err := w.Missions.AskAtStep(m.Slug, mission.TaskApprove, text, next.N); err != nil {
			return "couldn't ask the user: " + err.Error()
		}
		return fmt.Sprintf("step %d is the planner's, which couldn't be asked: the user is asked to write the steps after it and go on", next.N)
	case mission.StepWait:
		return fmt.Sprintf("step %d is %s (%s): it's waited for, and accepted once it's happened", next.N, next.Step.Whose(), next.Step.WhoSays())
	case mission.StepOpenPR:
		return w.openPRStep(m, next, done)
	}
	return w.startStage(m, next)
}

// reviewHere is what a checkpoint asks of you besides going on, when the
// step after it opens a pull request and ws holds your review before one:
// review the work here, or the pull request won't open.
func (w *Workspace) reviewHere(m *mission.Mission, n int) string {
	if m.Plan == nil || n >= len(m.Plan.Steps) || !m.Plan.Steps[n].MakesPR() || !w.holding(GuaranteeReviewBeforePR) {
		return ""
	}
	return fmt.Sprintf(" Step %d opens a pull request, and ws holds %s: review the work here, approving it, or it won't open.", n+1, GuaranteeReviewBeforePR.Key)
}

// sentence is s with a full stop, if it ends without one.
func sentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s[len(s)-1:], ".?!") {
		return s
	}
	return s + "."
}

// stageDoneSummary is what a stage's crew said it did, for the check-in:
// the latest done report among them.
func stageDoneSummary(m *mission.Mission, st mission.StageState) string {
	if st.Record == nil {
		return ""
	}
	for i := len(st.Record.Crew) - 1; i >= 0; i-- {
		if c, ok := m.CrewByID(st.Record.Crew[i]); ok && c.Status == mission.CrewDone && c.Note != "" {
			return " (" + c.ID + ": " + strings.TrimSuffix(oneLine(c.Note), ".") + ")"
		}
	}
	return ""
}

// startStage starts crew on a stage of the plan, as ws's own dispatch: in
// the capsule the stage names (or the mission's only one), with the stage
// as what it's dispatched to do. It counts toward the dispatches allowed
// without the user, and is queued for them with [orchestrator] dispatch =
// "ask". It says what it did.
func (w *Workspace) startStage(m *mission.Mission, st mission.StageState) string {
	if w.Config().OrchestratorAutoDispatch() && w.opts.Host != nil {
		// Marked from here, not from the launch: finding its capsule, or
		// lifting one, comes first.
		defer w.launching(m.Slug, launchWhat(st))()
	}
	if st.Step != nil && st.Step.Type == mission.StepVerify {
		return w.startVerifier(m, st)
	}
	if st.Step != nil && st.Step.CarryOn > 0 {
		if said, ok := w.carryOnAt(m, st); ok {
			return said
		}
	}
	if len(st.Instances()) > 0 {
		return w.startInstances(m, st, "", "")
	}
	return w.startFresh(m, st, "", "")
}

// startFresh starts fresh crew on a stage: in capsule, or the one the
// stage names (or the mission's only one), on what on says (or the
// step's on:, or crew's own). It says what it did.
func (w *Workspace) startFresh(m *mission.Mission, st mission.StageState, capsule, on string) string {
	noun := st.Noun()
	ok := capsule != ""
	if !ok {
		capsule, ok = w.stepCapsule(m, st)
	}
	if !ok && st.Step != nil && st.Step.Type == mission.StepWork && st.Step.Capsule != "" {
		// Work in a repo the mission has no capsule in: ws lifts one.
		if lifted := w.liftForStep(m.Slug, st); lifted != "" {
			if m2, err := w.Missions.Load(m.Slug); err == nil {
				m = m2
			}
			capsule, ok = w.stepCapsule(m, st)
		}
	}
	if !ok {
		return fmt.Sprintf("%s %d names no capsule of the mission's, so ws didn't start it: dispatch it with --capsule <repo>/<name>", noun, st.N)
	}
	text := strings.Join(strings.Fields(st.Whole()), " ")
	if in, ok := st.Instance(capsule); ok {
		text += fmt.Sprintf(" It runs in each of %s: you're its run in `%s`.", strings.Join(instanceRepos(st), ", "), in.Repo)
	}
	pick, own := w.stepPick(st, on)
	if !w.Config().OrchestratorAutoDispatch() || w.opts.Host == nil {
		if err := w.Missions.QueueDispatch(m.Slug, &mission.Dispatch{Text: text, Capsule: capsule, By: "ws", Stage: st.N, Harness: pick.Harness, Model: pick.Model}); err != nil {
			return fmt.Sprintf("couldn't queue %s %d: %v", noun, st.N, err)
		}
		return fmt.Sprintf("%s %d is queued in %s for the user to launch%s", noun, st.N, capsule, own)
	}
	l, err := w.Launch(m.Slug, Dispatch{Capsule: capsule, Text: text, Stage: st.N, Harness: pick.Harness, Model: pick.Model}, AsWS)
	var r *Refused
	switch {
	case errors.As(err, &r) && r.Code == "dispatch-limit":
		Noun := strings.ToUpper(noun[:1]) + noun[1:]
		text := fmt.Sprintf("%s %d is next, but ws has started as many crew without you as it may. Go on to %s %d: %s?", Noun, st.N, noun, st.N, st.Short())
		w.Missions.AskAtStep(m.Slug, mission.TaskApprove, text, st.N)
		return fmt.Sprintf("the dispatch limit is reached, so the user is asked before %s %d starts", noun, st.N)
	case err != nil:
		return fmt.Sprintf("couldn't start %s %d: %v", noun, st.N, err)
	}
	return fmt.Sprintf("started %s %d: %s in %s%s", noun, st.N, l.ID, l.Capsule, own)
}

// carryOnAt starts a Work step that carries on from an earlier one: the
// crew who did that step carry on (carryOn). It reports false when that
// step had no crew to carry on, and fresh crew start.
func (w *Workspace) carryOnAt(m *mission.Mission, st mission.StageState) (string, bool) {
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	from, err := mission.PickStage(states, st.Step.CarryOn)
	if err != nil || from.Record == nil {
		return "", false
	}
	if len(from.Instances()) > 0 && st.Step.Capsule == "" {
		// It ran in several repos: this one does too, each run's crew
		// carrying on.
		return w.carryOnInstances(m, from, st), true
	}
	id := ""
	for i := len(from.Record.Crew) - 1; i >= 0 && id == ""; i-- {
		if c, ok := m.CrewByID(from.Record.Crew[i]); ok && c.Role == "" {
			id = c.ID
		}
	}
	if id == "" {
		return "", false
	}
	text := fmt.Sprintf("Step %d is done, and you carry on with step %d of the plan: %s\n\nCommit as you go, and report done with a handoff when it's finished (ws mission done).", from.N, st.N, st.Whole())
	said, err := w.carryOn(m, st, id, text, AsWS)
	if err != nil {
		return fmt.Sprintf("couldn't have %s carry on at step %d: %v", id, st.N, err), true
	}
	return said, true
}

// StageWarnings names what the brief's stages point at, as a capsule
// written `repo/name` with a repo of the workspace, that isn't one of the
// mission's capsules. ws only warns: the brief is the user's.
func (w *Workspace) StageWarnings(m *mission.Mission, brief string) []string {
	if m.Plan != nil {
		return nil // the plan's steps are the route: PlanWarnings says of them
	}
	return w.routeWarnings(m, mission.ParseStages(brief), "change the brief")
}

// PlanWarnings names the capsules a plan's steps work in that aren't the
// mission's, as StageWarnings does for a brief's stages.
func (w *Workspace) PlanWarnings(m *mission.Mission, p *mission.Plan) []string {
	return w.routeWarnings(m, p.Stages(), "change the plan")
}

func (w *Workspace) routeWarnings(m *mission.Mission, route []mission.Stage, change string) []string {
	have := map[string]bool{}
	for _, c := range m.Capsules {
		have[c.Repo+"/"+c.Name] = true
	}
	var out []string
	worked := map[string]bool{} // repos a Work step before has lifted a capsule in, by the time it's reached
	for _, st := range route {
		for _, ref := range st.NamedCapsules() {
			repo, name, named := strings.Cut(ref, "/")
			r, ok := w.WS().ResolveAlias(repo)
			switch {
			case !ok || named && have[r+"/"+name]:
			case !named && st.Step != nil && st.Step.Type == mission.StepWork:
				worked[r] = true // ws lifts one when the step starts
			case !named && (worked[r] || slices.ContainsFunc(m.Capsules, func(c mission.Capsule) bool { return c.Repo == r })):
			case !named:
				out = append(out, fmt.Sprintf("%s %d is in `%s`, where the mission has no capsule, and no Work step before it makes one: ws lift %s %s, or %s", st.Noun(), st.N, ref, r, m.Slug, change))
			default:
				out = append(out, fmt.Sprintf("%s %d names `%s`, which isn't one of the mission's capsules (yet): ws lift %s %s, or %s", st.Noun(), st.N, ref, r, name, change))
			}
		}
	}
	return out
}

// Answered is a mission's task answered, and what happens next.
type Answered struct {
	Slug string
	Next string
}

func (a Answered) String() string { return "Answered: " + a.Next + "." }

// Answer answers the task at the head of the mission's queue: the oldest
// thing it waits on you for.
func (w *Workspace) Answer(slug string, by Actor, text string) (Answered, error) {
	return w.AnswerTask(slug, 0, by, text)
}

// AnswerTask answers a mission's task n (0: the oldest), by you, or by
// crew you asked to answer a question for you, and acts on it, by what it
// asks:
//
//   - a question: your answer is recorded as a decision, and goes to the
//     orchestrator, which runs again with it;
//   - a Decide step: your answer is the decision, and passes the step;
//   - a step of yours otherwise (to review, land, or approve going on):
//     going on passes it, or, where ws was waiting on you to start it (a
//     stage, a pull request to open), starts it; not yet leaves it yours
//     (a stage to start waits, and launching crew starts it); anything else
//     is recorded as a decision and goes to the orchestrator, and the step
//     stays yours.
func (w *Workspace) AnswerTask(slug string, n int, by Actor, text string) (Answered, error) {
	text = strings.TrimSpace(text)
	if by.Kind == Orchestrator {
		return Answered{}, refused("not-yours", "the orchestrator asks with ws mission flag; the user, or crew they ask, answers")
	}
	if text == "" {
		return Answered{}, invalid("the answer is empty")
	}
	m, err := w.load(slug)
	if err != nil {
		return Answered{}, err
	}
	var t mission.Task
	switch head := m.Asking(); {
	case n > 0:
		var ok bool
		if t, ok = m.Task(n); !ok {
			return Answered{}, refused("nothing-asked", "the mission has no task %d open: ws show -f llm lists what it waits on the user for", n)
		}
	case head == nil:
		return Answered{}, refused("nothing-asked", "nothing to answer: the mission isn't asking anything")
	default:
		t = *head
	}
	if by.Kind == Crew {
		c, ok := m.CrewByID(by.ID)
		if !ok || c.Role != mission.RoleAnswer {
			return Answered{}, refused("not-yours", "only the user answers a question, or crew they asked to answer it; crew ask the user with ws mission ask")
		}
		if !t.Question() {
			if q, ok := m.Question(); ok {
				t = q
			} else {
				return Answered{}, refused("not-yours", "the mission's question is answered: what's left is the user's own (a step of theirs)")
			}
		}
	}
	if t.Verb == mission.TaskReply {
		if out, ok, err := w.answerReply(m, t, by, text); ok || err != nil {
			return out, err
		}
	}
	if t.Step > 0 && t.Verb == mission.TaskApprove {
		if out, ok, err := w.answerHeld(m, t, by, text); ok || err != nil {
			return out, err
		}
	}
	if t.Step > 0 && (t.Verb == mission.TaskChecks || t.Verb == mission.TaskTime) {
		if out, ok, err := w.answerAtWork(m, t, by, text); ok || err != nil {
			return out, err
		}
	}
	goOn, notYet := strings.EqualFold(text, mission.CheckInGoOn), strings.EqualFold(text, mission.CheckInNotYet)
	decide := t.Step > 0 && t.Verb == mission.TaskAnswer // a Decide step
	atStep := t.Step > 0 && !decide
	if !(atStep && (goOn || notYet)) && !(decide && notYet) {
		// An answer is a decision whoever comes next should know: going on,
		// or not yet, at a step of yours isn't. It's recorded first, so the
		// answer is the log's latest.
		w.Missions.Decide(m.Slug, mission.Decision{
			What: text, By: by.By(),
			Why: fmt.Sprintf("the answer to %s's question: %s", firstNonEmpty(t.By, mission.RoleOrchestrator), oneLine(t.Text)),
		})
	}
	// A step of yours stays yours unless you go on (or decide); a stage
	// ws asked to start is put aside by not yet.
	keep := atStep && !goOn && !(notYet && t.Verb == mission.TaskApprove) || decide && notYet
	if _, _, err := w.Missions.AnswerTask(m.Slug, t.N, text, by.By(), keep); err != nil {
		return Answered{}, wrap(err)
	}
	if by.Kind == Crew {
		// Answering was its whole job: it has handed back.
		w.Missions.SetCrewStatus(m.Slug, by.ID, mission.CrewDone, "answered: "+text)
	}
	out := Answered{Slug: m.Slug}
	switch {
	case decide && !notYet:
		// The answer is the decision (recorded above), and passes the step.
		acc, err := w.AcceptStage(m.Slug, AsYou, "decided: "+oneLine(text))
		if err != nil {
			return Answered{}, err
		}
		out.Next = acc.Next
	case atStep && goOn:
		m, err := w.Missions.Load(m.Slug)
		if err != nil {
			return Answered{}, wrap(err)
		}
		brief, _ := w.Missions.ReadBrief(m.Slug)
		states, _ := m.StageProgress(brief)
		cur, ok := mission.CurrentStage(states)
		if !ok {
			out.Next = "every stage is accepted already"
			return out, nil
		}
		if cur.Step != nil && cur.N == t.Step && (cur.Step.Who == mission.WhoYou || cur.Step.Who == mission.WhoPlanner) {
			// Going on at a checkpoint (or the planner's step) passes it.
			acc, err := w.AcceptStage(m.Slug, AsYou, "the user went on")
			if err != nil {
				return Answered{}, err
			}
			out.Next = acc.Next
			return out, nil
		}
		out.Next = w.startStep(m, cur)
	case (atStep || decide) && notYet && keep:
		out.Next = fmt.Sprintf("step %d stays yours", t.Step)
	case atStep && notYet:
		out.Next = "it waits for you: launching crew starts the next stage"
	case !w.Config().OrchestratorEnabled():
		out.Next = "the orchestrator is off ([orchestrator] enabled = false), so act on it yourself"
	default:
		w.Notify(m.Slug, mission.OrchestratorEvent{Trigger: "answer"})
		out.Next = "the orchestrator is acting on it"
		if atStep {
			out.Next += fmt.Sprintf("; step %d stays yours", t.Step)
		}
	}
	return out, nil
}

// answerAtWork answers a Work step's task with one of the answers it
// suggests: its checks' (pass it, send it back) or its limit's (more time,
// wrap up). Anything else is answered as at any step of yours (ok false):
// a decision, for the orchestrator, the step staying yours.
func (w *Workspace) answerAtWork(m *mission.Mission, t mission.Task, by Actor, text string) (Answered, bool, error) {
	is := func(answers ...string) bool {
		for _, a := range answers {
			if strings.EqualFold(text, a) {
				return true
			}
		}
		return false
	}
	out := Answered{Slug: m.Slug}
	closeTask := func() error {
		_, _, err := w.Missions.AnswerTask(m.Slug, t.N, text, by.By(), false)
		return wrap(err)
	}
	switch {
	case t.Verb == mission.TaskChecks && is(mission.ChecksPass, mission.CheckInGoOn):
		if err := closeTask(); err != nil {
			return out, true, err
		}
		acc, err := w.AcceptStage(m.Slug, by, "the user passed it, its checks as they were")
		if err != nil {
			return out, true, err
		}
		out.Next = acc.Next
	case t.Verb == mission.TaskChecks && is(mission.ChecksSendBack):
		if err := closeTask(); err != nil {
			return out, true, err
		}
		said, err := w.SendBack(m.Slug, t.Step, by)
		if err != nil {
			return out, true, err
		}
		out.Next = said
	case t.Verb == mission.TaskTime && is(mission.TimeMore):
		if err := closeTask(); err != nil {
			return out, true, err
		}
		said, err := w.MoreTime(m.Slug, t.Step, 0, by)
		if err != nil {
			return out, true, err
		}
		out.Next = said
	case t.Verb == mission.TaskTime && is(mission.TimeWrapUp):
		if err := closeTask(); err != nil {
			return out, true, err
		}
		out.Next = w.wrapUp(m.Slug, t.Step, by)
	case is(mission.CheckInNotYet):
		out.Next = fmt.Sprintf("step %d stays yours", t.Step)
	default:
		return out, false, nil
	}
	w.youActed(m.Slug, by)
	return out, true, nil
}

// Dismissed is a mission's question, the crew queued for you, or the
// ending suggested, dismissed.
type Dismissed struct {
	Slug string
	What string // "question", "queued crew" or "suggested ending"
}

func (d Dismissed) String() string {
	return strings.ToUpper(d.What[:1]) + d.What[1:] + " dismissed."
}

// Dismiss puts aside what a mission asks of you without doing it: its
// oldest question if it has one (or a stage ws asked to start), else the
// crew queued for you to launch, else the ending suggested (it doesn't end
// yet). A step of yours isn't put aside: go on, or change the plan. Anyone
// else may only take back their own question.
func (w *Workspace) Dismiss(slug string, by Actor) (Dismissed, error) {
	m, err := w.load(slug)
	if err != nil {
		return Dismissed{}, err
	}
	if !by.IsYou() {
		i := slices.IndexFunc(m.Tasks, func(t mission.Task) bool { return t.Question() && t.By == by.By() })
		if i < 0 {
			return Dismissed{}, refused("nothing-to-dismiss", "nothing to dismiss: the mission isn't asking anything of yours")
		}
		if err := w.Missions.CloseTask(m.Slug, m.Tasks[i].N, by.By(), "taken back"); err != nil {
			return Dismissed{}, wrap(err)
		}
		return Dismissed{Slug: m.Slug, What: "question"}, nil
	}
	if i := slices.IndexFunc(m.Tasks, func(t mission.Task) bool { return t.Question() || t.Verb == mission.TaskApprove }); i >= 0 {
		t := m.Tasks[i]
		if err := w.Missions.CloseTask(m.Slug, t.N, by.By(), "dismissed"); err != nil {
			return Dismissed{}, wrap(err)
		}
		w.youActed(m.Slug, by)
		return Dismissed{Slug: m.Slug, What: "question"}, nil
	}
	switch {
	case m.Dispatch != nil:
		if err := w.Missions.QueueDispatch(m.Slug, nil); err != nil {
			return Dismissed{}, wrap(err)
		}
		w.youActed(m.Slug, by)
		return Dismissed{Slug: m.Slug, What: "queued crew"}, nil
	case m.Conclude != nil:
		if err := w.Missions.SetConclude(m.Slug, nil); err != nil {
			return Dismissed{}, wrap(err)
		}
		w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventNote, Text: "the ending suggested is put aside: it doesn't end yet"})
		w.youActed(m.Slug, by)
		return Dismissed{Slug: m.Slug, What: "suggested ending"}, nil
	case len(m.Tasks) > 0:
		t := m.Tasks[0]
		return Dismissed{}, refused("yours", "step %d is yours: go on, or not yet (or change the plan), rather than dismissing it", t.Step)
	}
	return Dismissed{}, refused("nothing-to-dismiss", "nothing to dismiss: no question, no crew queued, and no ending suggested")
}

// oneLine joins a text's lines into one, its spaces single.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func firstNonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// clipString is s, trimmed, to its first n bytes.
func clipString(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// clipLine is s on one line, to n bytes.
func clipLine(s string, n int) string {
	s = oneLine(s)
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

// plural is word, or its plural for n other than one.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
