package ops

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
)

// Open Pull Request's expansion, once ws has opened the pull request: the
// daemon watches it (WatchPRs), and what happens to it is an event, each of
// its kind (mission.PREvent). At the step, they're the step's: a merge
// passes it; red checks or a conflict come round for fixing, the
// orchestrator's call by the process ([missions] max_rounds caps it, then
// it's yours); a draft whose checks pass is marked ready (after your review
// where ws holds review_before_ready; otherwise as the process says); what
// someone says on it (a review, a comment) is the orchestrator's to act on,
// by the process, crew drafting replies that wait for you where they're to
// a person (prreplies.go), though a bot's comment on the pull request
// itself is only logged (newSaid); closing it without merging is the
// orchestrator's call. Elsewhere, as before: red checks, changes asked and
// a merge wake the orchestrator, but not the merge of one whose step has
// been passed already.

// Watched is whether a mission's PRs are watched: not once it's archived,
// nor while it's parked.
func Watched(m *mission.Mission) bool {
	return m.State != mission.Archived && m.State != mission.Parked
}

// WatchPRs notices what changed about the missions' PRs since they were
// last seen, as views show them: it logs each change once, as an event of
// its kind, and acts on it. A PR at its Open Pull Request step is the
// step's (atPRStep). Any other that someone has to act on (red checks, a
// conflict, changes asked, a merge, closed) goes to the orchestrator,
// unless crew are at work in its capsule: it's theirs.
func (w *Workspace) WatchPRs(views []MissionView) {
	for _, v := range views {
		if !Watched(v.M) || v.State == mission.Briefing {
			continue
		}
		slug := v.M.Slug
		act, merged := false, false
		for _, c := range v.Capsules {
			capsule := c.Repo + "/" + c.Name
			prev := seenOf(v.M, capsule)
			now, ok := w.sighting(c, prev)
			if !ok {
				continue
			}
			st, atStep := w.prStep(v.M, capsule)
			var said []mission.PREvent
			if prev != nil && prev.Number == now.Number {
				now.Said = prev.Said
				if atStep && !now.Merged && !now.Closed {
					said, now.Said = w.newSaid(c.Repo, now.Number, prev.Said)
				}
			}
			was, changed, err := w.Missions.SeePR(slug, capsule, now)
			if err != nil || !changed {
				continue
			}
			events := append(mission.PRChanges(fmt.Sprintf("%s#%d", c.Repo, now.Number), was, now), said...)
			for _, e := range events {
				w.Missions.AppendLog(slug, mission.Event{Kind: mission.EventPR, What: e.What, Repo: c.Repo, Capsule: c.Name, Text: e.Text})
			}
			if len(events) == 0 {
				continue
			}
			merged = merged || now.Merged
			if atStep {
				w.atPRStep(slug, st, capsule, now, events)
				continue
			}
			if now.Merged && w.PassOnMerge(slug, capsule) {
				continue // ws passed the step it was waiting for
			}
			if now.Merged && w.prStepPassed(slug, capsule) {
				// Its step was passed before ws saw the merge (the
				// orchestrator accepted it, told of the merge): no news.
				continue
			}
			for _, e := range events {
				act = act || e.Act && !v.M.CrewAt(capsule)
			}
		}
		// A merge that passed the plan's last step brings it to Z: the
		// orchestrator suggests how it ends, with Z checked.
		if !act && merged && w.atZ(slug) {
			act = true
		}
		if act {
			w.Notify(slug, mission.OrchestratorEvent{Trigger: "pr"})
		}
		// An ending suggested landed, held until now (crew were still at
		// it, say), ends once nothing holds it.
		if c := v.M.Conclude; c != nil && c.Ending == mission.EndLanded {
			w.endWhenMerged(slug)
		}
	}
}

// atZ reports whether the mission's plan has every step passed, and
// nothing yet says how it ends.
func (w *Workspace) atZ(slug string) bool {
	m, err := w.Missions.Load(slug)
	if err != nil || m.Plan == nil || len(m.Plan.Steps) == 0 || m.Ending != nil || m.Conclude != nil {
		return false
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	_, stepToGo := mission.CurrentStage(states)
	return !stepToGo
}

// prStepPassed reports whether the plan's Open Pull Request step for a
// capsule has been passed already.
func (w *Workspace) prStepPassed(slug, capsule string) bool {
	m, err := w.Missions.Load(slug)
	if err != nil || m.Plan == nil {
		return false
	}
	brief, _ := w.Missions.ReadBrief(slug)
	states, _ := m.StageProgress(brief)
	return slices.ContainsFunc(states, func(st mission.StageState) bool {
		return st.Step != nil && st.Step.Type == mission.StepOpenPR && st.Accepted() && slices.Contains(w.stepCapsules(m, st), capsule)
	})
}

// seenOf is what was last seen of a capsule's PR, if anything.
func seenOf(m *mission.Mission, capsule string) *mission.SeenPR {
	for _, c := range m.Capsules {
		if c.Repo+"/"+c.Name == capsule && c.SeenPR != nil {
			s := *c.SeenPR
			return &s
		}
	}
	return nil
}

// sighting is what's seen of a capsule's PR now: as GitHub listed it, or,
// gone from the lists while it was last seen open, as GitHub says it is
// (closed without merging). It reports false when there's nothing to see.
func (w *Workspace) sighting(c CapsuleView, prev *mission.SeenPR) (mission.SeenPR, bool) {
	if c.PR != nil {
		return mission.SeenPR{Number: c.PR.Number, Merged: c.PRS == mission.PRMerged, Draft: c.PR.IsDraft && c.PRS != mission.PRMerged,
			Review: c.PR.ReviewDecision, Checks: c.PR.StatusRollup, Conflict: c.PR.Mergeable == "CONFLICTING"}, true
	}
	gh := w.GitHub()
	if prev == nil || prev.Merged || prev.Closed || gh == nil {
		return mission.SeenPR{}, false
	}
	pr, err := gh.PRFromNumber(w.WS().RemoteOf(c.Repo).Owner, w.WS().RemoteOf(c.Repo).Name, prev.Number)
	if err != nil || pr == nil || pr.State != "CLOSED" {
		return mission.SeenPR{}, false
	}
	now := *prev
	now.Closed = true
	return now, true
}

// prStep is the step the mission's at, when it's an Open Pull Request step
// whose capsule this is.
func (w *Workspace) prStep(m *mission.Mission, capsule string) (mission.StageState, bool) {
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.Step.Type != mission.StepOpenPR {
		return mission.StageState{}, false
	}
	if !slices.Contains(w.stepCapsules(m, cur), capsule) {
		return mission.StageState{}, false
	}
	return cur, true
}

// saidNothing is what's been read of a PR where nobody but you has said
// anything yet: before any time GitHub gives.
const saidNothing = "0"

// newSaid is what people other than you have said on a PR since after
// (its reviews with something to say, its comments, and its review
// threads' comments), oldest first, as events, and when the latest was.
// With no after, nothing's news: what's been said so far is where reading
// starts.
//
// Each is someone's to act on, but a bot's comment on the pull request
// itself: a coverage summary, a deploy preview's link. It's logged, and
// the orchestrator reads it when it next runs; a bot with something to
// have fixed fails a check, reviews, or comments on a line, which do
// bring it round.
func (w *Workspace) newSaid(repo string, number int, after string) ([]mission.PREvent, string) {
	gh := w.GitHub()
	if gh == nil {
		return nil, after
	}
	me, _ := gh.Viewer()
	ref := fmt.Sprintf("%s#%d", repo, number)
	type item struct {
		at, text string
		act      bool
	}
	var items []item
	latest := after
	see := func(author, at, text string, act bool) {
		if author == me || at == "" {
			return
		}
		if at > latest {
			latest = at
		}
		if after != "" && at > after {
			items = append(items, item{at, text, act})
		}
	}
	if said, err := gh.Discussion(w.WS().RemoteOf(repo).Owner, w.WS().RemoteOf(repo).Name, number); err == nil {
		for _, s := range said {
			body := clipString(oneLine(s.Body), 400)
			switch {
			case s.State != "" && body == "":
				continue // a review with nothing to say: its verdict is the review decision
			case s.State != "":
				see(s.Author, s.At, fmt.Sprintf("%s reviewed %s (%s): %s", s.Author, ref, strings.ToLower(strings.ReplaceAll(s.State, "_", " ")), body), true)
			default:
				see(s.Author, s.At, fmt.Sprintf("%s commented on %s: %s", s.Author, ref, body), !s.Bot)
			}
		}
	}
	if threads, err := gh.ReviewThreads(w.WS().RemoteOf(repo).Owner, w.WS().RemoteOf(repo).Name, number); err == nil {
		for _, t := range threads {
			where := t.Path
			if t.Line > 0 {
				where = fmt.Sprintf("%s:%d", t.Path, t.Line)
			}
			for _, c := range t.Comments {
				see(c.Author, c.At, fmt.Sprintf("%s on %s, `%s` (thread %s): %s", c.Author, ref, where, t.ID, clipString(oneLine(c.Body), 400)), true)
			}
		}
	}
	if after == "" && latest == "" {
		latest = saidNothing // read, and nothing's been said: what's said next is news
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].at < items[j].at })
	var out []mission.PREvent
	for _, it := range items {
		out = append(out, mission.PREvent{What: mission.PREventSaid, Text: it.text, Act: it.act})
	}
	return out, latest
}

// atPRStep acts on what happened to the PR of the Open Pull Request step
// the mission's at. It says what it did.
func (w *Workspace) atPRStep(slug string, st mission.StageState, capsule string, pr mission.SeenPR, events []mission.PREvent) string {
	has := func(what string) bool {
		return slices.ContainsFunc(events, func(e mission.PREvent) bool { return e.What == what })
	}
	repo, _, _ := strings.Cut(capsule, "/")
	ref := fmt.Sprintf("%s#%d", repo, pr.Number)
	switch {
	case has(mission.PREventMerged):
		if w.PassOnMerge(slug, capsule) {
			return "passed step " + fmt.Sprint(st.N)
		}
		return ""
	case has(mission.PREventClosed):
		return w.prNeeds(slug, ref+" was closed without merging")
	}
	m, err := w.Missions.Load(slug)
	if err != nil {
		return ""
	}
	crewAt := m.CrewAt(capsule)
	var said []string
	wake := false
	if has(mission.PREventChecksFailed) || has(mission.PREventConflict) {
		n, _ := w.Missions.CountRound(slug, st.N, repo)
		what := ref + "'s checks are failing"
		if has(mission.PREventConflict) {
			what = ref + " conflicts with its base"
		}
		switch {
		case crewAt:
			said = append(said, what+": crew at work in "+capsule+" have it")
		case n > w.Config().MaxRounds():
			text := fmt.Sprintf("%s, again: that's round %d, more than ws lets come round before it's yours (%s). Have it fixed once more, or leave it to you?", what, n, guaranteeMaxRounds(w.Config().MaxRounds()).Key)
			w.Missions.Ask(slug, text, "ws", "Fix it", "Leave it to me")
			said = append(said, what+": it's the user's, past max_rounds")
		default:
			wake = true
		}
	}
	if has(mission.PREventChecksPassed) && pr.Draft {
		said = append(said, w.readyWhenGreen(slug, st, capsule, ref, &wake))
	}
	saidToActOn := slices.ContainsFunc(events, func(e mission.PREvent) bool { return e.What == mission.PREventSaid && e.Act })
	if (saidToActOn || has(mission.PREventChangesAsked)) && !crewAt {
		wake = true
	}
	if wake {
		var what []string
		for _, e := range events {
			what = append(what, strings.TrimSuffix(e.Text, "."))
		}
		said = append(said, w.prNeeds(slug, strings.Join(what, "; ")))
	}
	return strings.Join(said, "; ")
}

// prNeeds puts what happened to a step's PR (what, in words) to the
// orchestrator, which decides by the process, or with it off, to you.
func (w *Workspace) prNeeds(slug, what string) string {
	if w.Notify(slug, mission.OrchestratorEvent{Trigger: "pr"}) {
		return "the orchestrator decides what follows, by the process"
	}
	w.Missions.Ask(slug, what+". The orchestrator isn't running: say what to do, or do it.", "ws")
	return "the user's asked"
}

// readyWhenGreen is a draft whose checks pass at its step: ws marks it
// ready once the user's review approves it, where it holds
// review_before_ready; otherwise it's the orchestrator's call, by the
// process (mark it ready, or have the user review it first), or with it
// off, the user's.
func (w *Workspace) readyWhenGreen(slug string, st mission.StageState, capsule, ref string, wake *bool) string {
	if w.holding(GuaranteeReviewBeforeReady) {
		repo, name, _ := strings.Cut(capsule, "/")
		if w.holdReview(GuaranteeReviewBeforeReady, slug, repo, name, "", "") == nil {
			said, err := w.MarkReady(slug, capsule, AsWS)
			if err != nil {
				return "couldn't mark it ready: " + err.Error()
			}
			return said
		}
		text := fmt.Sprintf("%s's checks pass, and it's a draft. ws holds %s: review %s, approving it, and ws marks it ready.", ref, GuaranteeReviewBeforeReady.Key, capsule)
		w.Missions.AskAtStep(slug, mission.TaskApprove, text, st.N)
		return ref + " is green: the user's asked to review it before it's marked ready"
	}
	if w.Config().OrchestratorEnabled() {
		*wake = true
		return ref + " is green, and a draft"
	}
	text := fmt.Sprintf("%s's checks pass, and it's a draft: go on, and ws marks it ready for review.", ref)
	w.Missions.AskAtStep(slug, mission.TaskApprove, text, st.N)
	return ref + " is green: the user's asked to mark it ready"
}

// MarkReady marks the draft pull request of the Open Pull Request step the
// mission's at ready for review, holding review_before_ready: for the
// orchestrator, as the process says, you, and ws once your review
// approves it. It says what it did.
func (w *Workspace) MarkReady(slug, capsule string, by Actor) (string, error) {
	if by.Kind == Crew {
		return "", refused("not-yours", "crew commit and report done; the orchestrator (as the process says) or the user marks the step's pull request ready")
	}
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.Step.Type != mission.StepOpenPR {
		return "", refused("no-pr-step", "the mission isn't at an Open Pull Request step: a pull request is marked ready at its step")
	}
	capsules := w.stepCapsules(m, cur)
	if capsule == "" {
		var drafts []string
		for _, c := range capsules {
			if seen := seenOf(m, c); seen != nil && seen.Draft && !seen.Merged && !seen.Closed {
				drafts = append(drafts, c)
			}
		}
		switch {
		case len(drafts) == 1:
			capsule = drafts[0]
		case len(drafts) > 1:
			return "", invalid("step %d has drafts open from %s: say which (ws mission pr-ready <repo>)", cur.N, strings.Join(drafts, ", "))
		case len(capsules) > 0:
			capsule = capsules[0]
		}
	} else if !strings.Contains(capsule, "/") {
		for _, c := range capsules {
			if strings.HasPrefix(c, capsule+"/") {
				capsule = c
			}
		}
	}
	seen := seenOf(m, capsule)
	switch {
	case !slices.Contains(capsules, capsule) || seen == nil || seen.Merged || seen.Closed:
		return "", refused("no-pr", "step %d's pull request from %s isn't open", cur.N, firstNonEmpty(capsule, "its capsule"))
	case !seen.Draft:
		return "", refused("not-draft", "%s's pull request isn't a draft: it's ready for review already", capsule)
	}
	repo, name, _ := strings.Cut(capsule, "/")
	if err := w.holdReview(GuaranteeReviewBeforeReady, m.Slug, repo, name, "marking a draft ready", reviewThen(by, "mark it ready")); err != nil {
		return "", err
	}
	gh := w.GitHub()
	if gh == nil {
		return "", refused("no-github", "ws can't reach GitHub here, so it can't mark the pull request ready")
	}
	if err := gh.MarkReady(w.WS().RemoteOf(repo).Owner, w.WS().RemoteOf(repo).Name, seen.Number); err != nil {
		return "", err
	}
	ready := *seen
	ready.Draft = false
	w.Missions.SeePR(m.Slug, capsule, ready) // seen as it's marked: not news
	ref := fmt.Sprintf("%s#%d", repo, seen.Number)
	w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventPR, What: mission.PREventMarkedReadyBy, Repo: repo, Capsule: name, Crew: by.By(),
		Text: fmt.Sprintf("%s marked ready for review, at step %d", ref, cur.N)})
	if t, ok := m.StepTask(cur.N); ok && t.Verb == mission.TaskApprove {
		w.Missions.CloseTask(m.Slug, t.N, by.By(), "")
	}
	w.opts.PRs.Forget(repo)
	w.youActed(m.Slug, by)
	return fmt.Sprintf("marked %s ready for review: ws passes step %d when it merges", ref, cur.N), nil
}

// readyOnApproval marks the step's draft ready once your review of its
// capsule approves it, when its task asked for that review. It says what
// it did, or "".
func (w *Workspace) readyOnApproval(m *mission.Mission, cur mission.StageState, capsule string) string {
	seen := seenOf(m, capsule)
	if seen == nil || !seen.Draft || seen.Merged || seen.Closed {
		return ""
	}
	if _, ok := m.StepTask(cur.N); !ok {
		return ""
	}
	said, err := w.MarkReady(m.Slug, capsule, AsWS)
	var r *Refused
	if errors.As(err, &r) {
		return ""
	}
	if err != nil {
		return "couldn't mark it ready: " + err.Error()
	}
	return said
}
