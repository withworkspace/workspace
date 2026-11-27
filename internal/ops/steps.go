package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/workspace"
)

// What ws does with a plan's step once the mission reaches it, by its kind
// (mission.Catalogue): the expansion of each, ws's, the same for every
// mission. Work starts crew; Review, Decide and Land ask you; Wait waits;
// Plan asks the planner; Open Pull Request is ws's own: it opens the pull
// request, and passes the step when it merges.

// startStep starts a step the mission is at, by its kind, outside its
// being reached (you going on at it, after ws couldn't start it): crew
// started, or the pull request opened. It says what it did.
func (w *Workspace) startStep(m *mission.Mission, st mission.StageState) string {
	if st.Step != nil && st.Step.Type == mission.StepOpenPR {
		// Its pull requests open, drafts: going on marks them ready.
		var said []string
		for _, capsule := range w.stepCapsules(m, st) {
			if seen := seenOf(m, capsule); seen != nil && seen.Draft && !seen.Merged && !seen.Closed {
				s, err := w.MarkReady(m.Slug, capsule, AsYou)
				if err != nil {
					s = "could not mark ready: " + err.Error()
				}
				said = append(said, s)
			}
		}
		if len(said) > 0 {
			return strings.Join(said, "; ")
		}
		return w.openPRStep(m, st, "")
	}
	if st.Step != nil && st.Step.Type == mission.StepWait {
		return w.reachWait(m, "", st)
	}
	return w.startStage(m, st)
}

// askAtStep asks you at a step that's yours, by its kind: a review to go
// on from, a decision to answer, a capsule to land. It says what it did.
func (w *Workspace) askAtStep(m *mission.Mission, done string, st mission.StageState) string {
	what := sentence(st.Plain())
	var text string
	options := []string{mission.CheckInGoOn, mission.CheckInNotYet}
	verb := mission.TaskReview
	switch st.Step.Type {
	case mission.StepDecide:
		verb = mission.TaskAnswer
		text = fmt.Sprintf("%sStep %d is yours to decide: %s", done, st.N, what)
		if st.Step.From != "" {
			text += fmt.Sprintf(" Options: %s.", strings.TrimSuffix(st.Step.From, "."))
		}
		text += " Your answer passes the step."
		options = []string{mission.CheckInNotYet}
	case mission.StepLand:
		verb = mission.TaskLand
		capsule, ok := w.stepCapsule(m, st)
		if !ok {
			capsule = "the mission's capsule"
		}
		text = fmt.Sprintf("%sStep %d is yours: land %s (Land on the capsule, or ws land). Passes once landed; go on if landed some other way.", done, st.N, capsule)
	default:
		text = fmt.Sprintf("%sStep %d is yours to review: %s%s %s", done, st.N, what, w.reviewHere(m, st.N), w.reviewWhat(m, st.N))
	}
	if _, err := w.Missions.AskAtStep(m.Slug, verb, text, st.N, options...); err != nil {
		return "could not ask the user: " + err.Error()
	}
	switch st.Step.Type {
	case mission.StepDecide:
		return fmt.Sprintf("step %d waits for the user's decision", st.N)
	case mission.StepLand:
		return fmt.Sprintf("step %d waits for the user to land", st.N)
	}
	return fmt.Sprintf("step %d waits for the user's review", st.N)
}

// reviewWhat says what Review step n looks at, and how it's passed: your
// review approving what the plan put under review, or going on. Crew's
// other documents are named after, to read. A bundle you've approved
// already (at its own step, its checks yours to say) isn't to approve
// again: it says so, and going on is what passes the step.
func (w *Workspace) reviewWhat(m *mission.Mission, n int) string {
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	var bundles, approved, caps, docs, reads []string
	for _, l := range m.Looks(states, n) {
		switch {
		case l.Bundle != "" && l.Approved:
			approved = append(approved, l.Name)
		case l.Bundle != "":
			bundles = append(bundles, l.Name)
		case l.Read:
			reads = append(reads, l.Name)
		case l.Capsule != "":
			caps = append(caps, l.Capsule)
		default:
			docs = append(docs, l.Name)
		}
	}
	if len(bundles) > 0 {
		// A bundle is what's to approve: the work it shows is beside it.
		text := fmt.Sprintf("Look at %s on the Verify tab: approving %s passes the step; what's not right goes back to the crew. Or go on.", strings.Join(bundles, ", "), itOrThem(len(bundles)))
		if len(approved) > 0 {
			text += fmt.Sprintf(" %s already approved.", strings.Join(approved, ", "))
		}
		if len(caps) > 0 {
			text += " Work is in " + strings.Join(caps, ", ") + ", to review alongside."
		}
		return text
	}
	if len(approved) > 0 {
		text := fmt.Sprintf("You approved %s on the Verify tab: go on to pass the step, or say what to change and it goes back to the crew.", strings.Join(approved, ", "))
		if len(caps) > 0 {
			text += " Work is in " + strings.Join(caps, ", ") + ", to review alongside."
		}
		return text
	}
	if len(caps)+len(docs) == 0 {
		found := "their handoffs"
		if len(reads) > 0 {
			found = strings.Join(reads, ", ")
		}
		return "Read what the crew found (" + found + "), then go on, or say what to change and it goes back to them."
	}
	var what []string
	if len(caps) > 0 {
		what = append(what, "review "+strings.Join(caps, ", "))
	}
	if len(docs) > 0 {
		what = append(what, "read "+strings.Join(docs, ", "))
	}
	all := ""
	if len(caps)+len(docs) > 1 {
		all = " all"
	}
	text := fmt.Sprintf("%s: approving%s passes the step; asking for changes sends it back to the crew. Or go on.", upperFirst(strings.Join(what, ", and ")), all)
	if len(reads) > 0 {
		text += " Crew notes: " + strings.Join(reads, ", ") + "."
	}
	return text
}

// openPRStep is ws at an Open Pull Request step: it opens the pull request
// from the mission's capsule in the step's repo, as ws, with a title and
// body drafted from the mission and the handoff of the crew who worked
// there. Where a guarantee wants your review first, or it can't be opened,
// it asks you, saying why; once your review approves it, it's tried again
// (RetryOpenPRStep). The step passes when the pull request merges
// (PassOnMerge). It says what it did.
func (w *Workspace) openPRStep(m *mission.Mission, st mission.StageState, done string) string {
	if ins := st.Instances(); len(ins) > 0 {
		// It runs in several repos: a pull request from each.
		var said, asks []string
		for _, in := range ins {
			if in.Done != nil {
				continue
			}
			capsule, ok := w.capsuleIn(m, st, in.Repo, false)
			if !ok {
				asks = append(asks, fmt.Sprintf("no capsule in %s: lift one, or start the step without it", in.Repo))
				continue
			}
			logFailed(w.Missions.SetInstance(m.Slug, st.N, in.Repo, capsule, false))
			s, ask := w.openPRIn(m, st, capsule, false)
			if ask != "" {
				asks = append(asks, ask)
			} else {
				said = append(said, s)
			}
		}
		if len(asks) > 0 {
			said = append(said, w.askAboutPR(m, st, fmt.Sprintf("%sStep %d opens a pull request in each of %s. Could not open some: %s. Fix and go on to try again.", done, st.N, strings.Join(instanceRepos(st), ", "), strings.Join(asks, "; "))))
		}
		return strings.Join(said, "; ")
	}
	capsule, ok := w.stepCapsule(m, st)
	if !ok {
		where := "a repo of its"
		if st.Step.Capsule != "" {
			where = "`" + st.Step.Capsule + "`"
		}
		return w.askAboutPR(m, st, fmt.Sprintf("%sStep %d opens a pull request from %s, but the mission has no capsule there: lift one, or name one in the plan, then go on.", done, st.N, where))
	}
	said, ask := w.openPRIn(m, st, capsule, false)
	if ask != "" {
		return w.askAboutPR(m, st, done+fmt.Sprintf("Step %d %s.", st.N, ask))
	}
	return said
}

// openPRIn opens a step's pull request from capsule, as the PR writer
// writes it (handed to the daemon, unless inJob: the PR writer's job
// itself): what it did, or, where it couldn't, what to ask the user
// ("opens a pull request from …, and …").
func (w *Workspace) openPRIn(m *mission.Mission, st mission.StageState, capsule string, inJob bool) (said, ask string) {
	opened, err := w.openPR(m.Slug, capsule, NewPullRequest{Draft: st.Step.Draft, Quiet: true, inJob: inJob}, AsWS)
	var r *Refused
	switch {
	case err == nil && opened.Writing:
		return fmt.Sprintf("step %d: writing %s's pull request; the step passes once it merges", st.N, capsule), ""
	case errors.As(err, &r) && r.Code == "pr-writing":
		return fmt.Sprintf("step %d: already writing %s's pull request", st.N, capsule), ""
	case err == nil:
		return fmt.Sprintf("step %d: opened %s#%d from %s; the step passes once it merges", st.N, opened.Repo, opened.PR.Number, capsule), ""
	case errors.As(err, &r) && r.Code == "pr-open":
		return fmt.Sprintf("step %d: %s's pull request already open; the step passes once it merges", st.N, capsule), ""
	case errors.As(err, &r) && r.Code == "guarantee":
		return "", fmt.Sprintf("opens a pull request from %s once a review approves it (%s)", capsule, GuaranteeReviewBeforePR.Key)
	case errors.As(err, &r) && r.Code == "conflicts":
		return w.mergeFirst(m, st, capsule)
	}
	return "", fmt.Sprintf("could not open a pull request from %s: %s. Fix and go on to try again, or use Open pull request on the capsule", capsule, strings.TrimSuffix(oneLine(err.Error()), "."))
}

// mergeFirst is ws at an Open Pull Request step whose capsule doesn't
// merge cleanly onto its base: rather than open a pull request that
// conflicts from the start, it sends the capsule back to the crew who
// worked in it, to merge the base in, and tries again when they report
// done (CrewDone). Each time is a round: past max_rounds, or with no crew
// to send it to, merging it is the user's, and going on tries again.
func (w *Workspace) mergeFirst(m *mission.Mission, st mission.StageState, capsule string) (said, ask string) {
	repo, name, _ := strings.Cut(capsule, "/")
	base := w.WS().DefaultBranch
	files, _ := workspace.GitMergeConflicts(filepath.Join(w.WS().RepoDir(repo), name), "origin/"+base)
	why := "conflicts with " + base
	if len(files) > 0 {
		why += " in " + strings.Join(files, ", ")
	}
	crewID := lastCrewIn(m, capsule)
	n, _ := w.Missions.CountRound(m.Slug, st.N, repo)
	if crewID == "" || n > w.Config().MaxRounds() {
		return "", fmt.Sprintf("opens a pull request from %s, which does not merge cleanly (%s): merge origin/%s into the capsule, then go on", capsule, why, base)
	}
	text := fmt.Sprintf("Step %d opens %s's pull request, and ws hasn't opened it: %s. Merge origin/%s into your branch (don't rebase, don't force-push), "+
		"resolve the conflicts so what both sides meant still works, run the tests, commit, and report done with a handoff (ws mission done): ws then opens the pull request.",
		st.N, capsule, why, base)
	s, err := w.carryOn(m, st, crewID, text, AsWS)
	if err != nil {
		return "", fmt.Sprintf("opens a pull request from %s, which does not merge cleanly (%s), and could not send back to %s (%v): merge origin/%s into the capsule, then go on", capsule, why, crewID, err, base)
	}
	w.Missions.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Repo: repo, Capsule: name,
		Text: fmt.Sprintf("did not open %s's pull request at step %d: %s; sent back to %s to merge origin/%s first (round %d)", capsule, st.N, why, crewID, base, n)})
	return fmt.Sprintf("step %d: %s does not merge cleanly (%s), pull request waits: %s", st.N, capsule, why, s), ""
}

// lastCrewIn is the crew who last worked in a capsule (not briefing or
// answering), or "".
func lastCrewIn(m *mission.Mission, capsule string) string {
	for _, c := range slices.Backward(m.Crew) {
		if c.Capsule == capsule && c.Role == "" {
			return c.ID
		}
	}
	return ""
}

// askAboutPR asks you at an Open Pull Request step ws couldn't open yet.
func (w *Workspace) askAboutPR(m *mission.Mission, st mission.StageState, text string) string {
	if _, err := w.Missions.AskAtStep(m.Slug, mission.TaskApprove, text, st.N); err != nil {
		return "could not ask the user: " + err.Error()
	}
	return fmt.Sprintf("step %d waits for the user before opening a pull request", st.N)
}

// prDraft is ws's own words for a pull request, where the PR writer
// couldn't write it: titled as the mission is, and saying what the crew
// who worked in the capsule did (the Done of their latest handoff), else
// what the mission's for.
func (w *Workspace) prDraft(m *mission.Mission, capsule string, step int) (title, body string) {
	title = m.Title
	if m.TitleIsRequest() || strings.TrimSpace(title) == "" {
		title = m.Slug
	}
	for i := len(m.Crew) - 1; i >= 0 && body == ""; i-- {
		c := m.Crew[i]
		if c.Capsule != capsule || c.HandedOff == nil {
			continue
		}
		if data, err := os.ReadFile(w.Missions.HandoffPath(m.Slug, c.ID)); err == nil {
			if h, err := mission.ParseHandoff(string(data)); err == nil {
				body = strings.TrimSpace(h.Done)
			}
		}
	}
	if body == "" {
		brief, _ := w.Missions.ReadBrief(m.Slug)
		body = strings.TrimSpace(briefGoal(brief))
	}
	if step > 0 {
		body += fmt.Sprintf("\n\nOpened by ws, at step %d of the mission %q.", step, m.Title)
	} else {
		body += fmt.Sprintf("\n\nOpened by ws, from the mission %q.", m.Title)
	}
	return title, strings.TrimSpace(body)
}

// briefGoal is what's under a brief's ## Goal, or "".
func briefGoal(brief string) string {
	var b strings.Builder
	in := false
	for line := range strings.SplitSeq(brief, "\n") {
		if strings.HasPrefix(line, "#") {
			in = strings.EqualFold(strings.TrimSpace(strings.TrimLeft(line, "# ")), "goal")
			continue
		}
		if in {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// RetryOpenPRStep has ws open the pull request of the Open Pull Request
// step a mission is at, once your review of its capsule has approved it:
// the step was waiting for that; or, open already as a draft that waited
// on your review to be marked ready, marks it ready. It says what it did,
// or "" when the mission isn't at such a step for that capsule.
func (w *Workspace) RetryOpenPRStep(slug, capsule string) string {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return ""
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.Step.Type != mission.StepOpenPR {
		return ""
	}
	if !slices.Contains(w.stepCapsules(m, cur), capsule) {
		return ""
	}
	if seen := seenOf(m, capsule); seen != nil && !seen.Merged && !seen.Closed {
		// Open already: a draft waiting on this review is marked ready.
		return w.readyOnApproval(m, cur, capsule)
	}
	w.closeStepTasks(slug, cur.N) // what it asked of you is done
	if _, ok := cur.Instance(capsule); ok {
		said, ask := w.openPRIn(m, cur, capsule, false)
		if ask != "" {
			return w.askAboutPR(m, cur, fmt.Sprintf("Step %d %s.", cur.N, ask))
		}
		return said
	}
	return w.openPRStep(m, cur, "")
}

// PassOnMerge passes the Open Pull Request step a mission is at when its
// capsule's pull request has merged: the step's done, and ws reaches the
// next. It reports whether it did, so the merge needn't wake the
// orchestrator.
func (w *Workspace) PassOnMerge(slug, capsule string) bool {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return false
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.Step.Type != mission.StepOpenPR {
		return false
	}
	if !slices.Contains(w.stepCapsules(m, cur), capsule) {
		return false
	}
	began := w.stepBegan(m, cur.N)
	if _, ok := cur.Instance(capsule); ok {
		// One of its runs: the step passes once every one has merged.
		said := w.instanceDone(slug, cur, capsule, "ws", "pull requests merged")
		if !strings.HasPrefix(said, "passed") {
			return said != ""
		}
	} else if w.passStep(slug, cur.N, "ws", capsule+"'s pull request merged", mission.StepOpenPR) == "" {
		return false
	}
	// What ws asked about the pull request while it was open is moot, and
	// so is what the orchestrator asked at the step (whether to merge it,
	// what to do about a check).
	logFailed(w.Missions.Ask(slug, "", "ws"))
	w.settleAsked(m, cur.N, began, "pull request merged")
	return true
}

// settleAsked closes what the orchestrator asked you at step n, since the
// plan reached it (began), now that the step has passed (how, in words):
// m is the mission as it was before. A question about a step that's done,
// left open, would be what the mission waits on you for and lead its
// page, and hold an ending that's otherwise ws's to make. What it asked
// before the step, and what crew asked, stay yours to answer.
func (w *Workspace) settleAsked(m *mission.Mission, n int, began time.Time, how string) {
	for _, t := range m.Tasks {
		if t.Question() && t.By == AsOrchestrator.By() && !t.At.Before(began) {
			w.Missions.CloseTask(m.Slug, t.N, "ws", fmt.Sprintf("step %d passed, %s: orchestrator's question settled", n, how)) //nolint:errcheck // answered meanwhile, it's closed already
		}
	}
}

// stepBegan is when the plan reached step n: when the step before it was
// passed. The zero time for the first step, or where that isn't recorded.
func (w *Workspace) stepBegan(m *mission.Mission, n int) time.Time {
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	if n < 2 || n-2 >= len(states) {
		return time.Time{}
	}
	if r := states[n-2].Record; r != nil && r.Accepted != nil {
		return *r.Accepted
	}
	return time.Time{}
}

// passOnLand passes the Land step a mission is at, once its capsule in
// the step's repo has landed.
func (w *Workspace) passOnLand(slug, repo, name string) {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.Step.Type != mission.StepLand {
		return
	}
	if at := cur.Step.Capsule; at != "" && at != repo && at != repo+"/"+name {
		if r, ok := w.WS().ResolveAlias(at); !ok || r != repo {
			return
		}
	}
	w.passStep(slug, cur.N, "you", repo+"/"+name+" landed", mission.StepLand)
}

// passStep passes step n of the plan, when the mission's at it and it's
// one of types, for whoever's done it (by, why), and moves the plan on.
// It says what it did, or "" when the mission isn't at such a step.
func (w *Workspace) passStep(slug string, n int, by, why string, types ...string) string {
	said, err := w.tryPassStep(slug, n, by, why, types...)
	if err != nil {
		return fmt.Sprintf("could not pass step %d: %v", n, err)
	}
	return said
}

// tryPassStep is passStep, with why it couldn't where it couldn't: for a
// caller with something to do about that. A step someone else passed
// meanwhile is as one the mission isn't at: "", and no error.
func (w *Workspace) tryPassStep(slug string, n int, by, why string, types ...string) (string, error) {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return "", nil
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.N != n || !slices.Contains(types, cur.Step.Type) {
		return "", nil
	}
	defer w.moving(slug, n)()
	acc, states, err := w.Missions.AcceptStageAt(slug, n, by, why)
	switch {
	case errors.Is(err, mission.ErrStageMoved):
		return "", nil
	case err != nil:
		return "", err
	}
	// What ws asked the user about the step is done.
	w.closeStepTasks(slug, n)
	return fmt.Sprintf("passed step %d, and %s", n, w.afterAccept(slug, acc, states)), nil
}

// liftForStep lifts the capsule a Work step is in, when the mission hasn't
// one there: its repo/name (joined, if it's lifted already), or in a repo,
// one named for the mission. It says what it lifted or joined, or "".
func (w *Workspace) liftForStep(slug string, st mission.StageState) string {
	for _, said := range w.liftForPlan(slug, &mission.Plan{Steps: []mission.Step{*st.Step}}, 0) {
		if strings.HasPrefix(said, "lifted ") || strings.HasPrefix(said, "joined ") {
			return said
		}
	}
	return ""
}

// stepCapsule is the capsule a step works in: the mission's capsule it
// names (repo/name), or its capsule in the repo it names, else, naming
// none, the mission's only one. Burned capsules don't count.
func (w *Workspace) stepCapsule(m *mission.Mission, st mission.StageState) (string, bool) {
	var existing []string
	for _, c := range m.Capsules {
		if info, err := os.Stat(filepath.Join(w.WS().RepoDir(c.Repo), c.Name)); err == nil && info.IsDir() {
			existing = append(existing, c.Repo+"/"+c.Name)
		}
	}
	inRepo := false // it names a repo, not a capsule: the mission's only one won't do
	for _, n := range st.NamedCapsules() {
		if slices.Contains(existing, n) {
			return n, true
		}
		if !strings.Contains(n, "/") {
			inRepo = true
			repo := n
			if r, ok := w.WS().ResolveAlias(n); ok {
				repo = r
			}
			for _, e := range existing {
				if strings.HasPrefix(e, repo+"/") {
					return e, true
				}
			}
		}
	}
	if !inRepo && len(existing) == 1 {
		return existing[0], true
	}
	return "", false
}

func itOrThem(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}

// PassReviewStep passes the Review step the mission is at once your
// reviews approve what the plan put under review (mission.Looks, but for
// the documents only to read) as it is now: each capsule's latest review
// approving its head, each document the plan names approved as it reads.
// A Review step with nothing to approve (findings in words, or crew's
// notes) is passed by going on. It says what it did, or "" when it
// didn't.
func (w *Workspace) PassReviewStep(slug string) string {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return ""
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.Step.Type != mission.StepReview {
		return ""
	}
	brief, _ := w.Missions.ReadBrief(slug)
	states, _ := m.StageProgress(brief)
	var names []string
	for _, l := range m.Looks(states, cur.N) {
		if l.Read {
			continue
		}
		if !w.approvedAsIs(m, l) {
			return ""
		}
		names = append(names, l.Name)
	}
	if len(names) == 0 {
		return ""
	}
	acc, err := w.AcceptStage(slug, AsYou, "your review approved "+strings.Join(names, ", "))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("step %d passed: your review approved %s; %s", cur.N, strings.Join(names, ", "), acc.Next)
}

// approvedAsIs reports whether your latest review approves what a Review
// step looks at as it is now: a capsule at its head, a document as it
// reads.
func (w *Workspace) approvedAsIs(m *mission.Mission, l mission.Look) bool {
	ws := w.WS()
	if l.Bundle != "" {
		b, ok := m.Bundle(l.Bundle)
		return ok && b.Verdict == mission.BundleApproved
	}
	if l.Capsule != "" {
		repo, name, _ := strings.Cut(l.Capsule, "/")
		r, _ := review.For(w.Root(), m.Slug).Latest(l.Capsule)
		return r != nil && r.Verdict == review.Approve && r.Head == workspace.GitRevParse(filepath.Join(ws.RepoDir(repo), name), "HEAD")
	}
	i := slices.IndexFunc(m.Artifacts, func(a mission.Artifact) bool { return a.Ref == l.Doc })
	if i < 0 {
		return false
	}
	approved, err := conversation.For(w.Root(), m.Slug).Approved(l.Doc)
	if err != nil || approved == nil {
		return false
	}
	v := w.missionView(m)
	av := ResolveArtifact(ws, w.Missions, v, m.Artifacts[i])
	if av.Path == "" {
		return false
	}
	text, err := os.ReadFile(av.Path)
	return err == nil && approved.Version == doc.Version(text)
}
