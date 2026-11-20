package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Pull requests opened by a step, through ws, so its guarantees about them
// are held where ws is the one acting: ws pushes the capsule's branch and
// opens the pull request on GitHub, once the guarantees it holds there
// allow it (GuaranteeReviewBeforePR). In a mission with a plan, it's a
// step that makes a pull request (Step.MakesPR) that opens it: crew open
// one there and nowhere else, and the orchestrator doesn't accept that
// step until one's open.

// NewPullRequest is a pull request to open from a capsule: what it says,
// and whether it's a draft. With no title, the PR writer writes it (and
// its description, where there's none). Quiet stops git asking for a
// password where there's no terminal (the web UI).
type NewPullRequest struct {
	Title, Body string
	Draft       bool
	Quiet       bool
	// inJob is the PR writer's job opening what it marked writing: it's
	// written there and then, not handed to the daemon again.
	inJob bool
}

// PROpened is a pull request opened from a capsule of a mission.
type PROpened struct {
	Slug    string
	Capsule string // repo/name
	Repo    string
	PR      *github.PR
	Step    int // the step of the plan it was opened at, if any
	// Writing is a pull request the PR writer is writing, which ws opens
	// once it's written: PR is nil.
	Writing bool
}

func (p PROpened) String() string {
	at := ""
	if p.Step > 0 {
		at = fmt.Sprintf(", at step %d", p.Step)
	}
	if p.Writing {
		return fmt.Sprintf("The PR writer is writing %s's pull request%s: ws opens it once it's written", p.Capsule, at)
	}
	return fmt.Sprintf("Opened %s#%d from %s%s: %s", p.Repo, p.PR.Number, p.Capsule, at, p.PR.URL)
}

// OpenPR pushes a capsule's branch and opens a pull request from it onto
// the default branch. Yours, and crew's: in a mission with a plan, only at
// the step they're on, when it's the current one and makes a pull
// request. Never the orchestrator's: it moves the mission along, and the
// step's crew open it. Refused in a repo with no pull requests, with
// changes not committed or nothing to open, when one is open from the
// branch already, and when a guarantee ws holds there isn't met.
func (w *Workspace) OpenPR(slug, capsule string, o NewPullRequest, by Actor) (PROpened, error) {
	if by.Kind == Orchestrator || by.Kind == WS {
		return PROpened{}, refused("not-yours", "a pull request is opened by the step that makes one (ws opens an Open Pull Request step's itself), or by the user, through ws mission open-pr: the orchestrator moves the mission along")
	}
	return w.openPR(slug, capsule, o, by)
}

// openPR is OpenPR for whoever may: you, crew at their step, or ws at an
// Open Pull Request step (openPRStep).
func (w *Workspace) openPR(slug, capsule string, o NewPullRequest, by Actor) (PROpened, error) {
	m, err := w.load(slug)
	if err != nil {
		return PROpened{}, err
	}
	if err := stillOpen(m); err != nil {
		return PROpened{}, err
	}
	c, err := w.missionCapsule(m, capsule)
	if err != nil {
		return PROpened{}, err
	}
	ws := w.WS()
	out := PROpened{Slug: m.Slug, Capsule: c.Repo + "/" + c.Name, Repo: c.Repo}
	if ws.Direct(c.Repo) {
		return PROpened{}, refused("direct", "%s has no pull requests: its commits go straight to %s when the user lands them. Commit, and report done", c.Repo, ws.DefaultBranch)
	}
	if by.Kind == Crew && m.Plan != nil {
		n, err := crewMayOpenPR(w.Missions, m, by.ID)
		if err != nil {
			return PROpened{}, err
		}
		out.Step = n
	} else if m.Plan != nil {
		if cur, ok := currentStep(w.Missions, m); ok && cur.Step.MakesPR() {
			out.Step = cur.N
		}
	}
	o.Title = strings.TrimSpace(o.Title)
	if _, ok := m.WritingPR(out.Capsule); ok && !o.inJob {
		return PROpened{}, refused("pr-writing", "the PR writer is writing %s's pull request already: ws opens it once it's written", out.Capsule)
	}
	dir := filepath.Join(ws.RepoDir(c.Repo), c.Name)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return PROpened{}, refused("burned", "%s has no worktree any more, so there's nothing to open a pull request from", out.Capsule)
	}
	if workspace.GitIsDirty(dir) {
		return PROpened{}, refused("dirty", "%s has changes not committed: commit them (or set them aside), then open it", out.Capsule)
	}
	if ws.CapsuleAhead(c.Repo, c.Name) == 0 {
		return PROpened{}, refused("nothing", "%s has no commits that %s doesn't: there's nothing to open a pull request for", out.Capsule, ws.DefaultBranch)
	}
	gh := w.GitHub()
	if gh == nil {
		return PROpened{}, refused("no-github", "ws can't reach GitHub here, so it can't open a pull request")
	}
	branch := workspace.GitCurrentBranch(dir)
	if open, err := gh.PRsForRepo(ws.Org, c.Repo); err == nil {
		for _, pr := range open {
			if pr.HeadRefName == branch && branch != "" {
				return PROpened{}, refused("pr-open", "%s#%d is open from %s already (%s): push to the branch to change it", c.Repo, pr.Number, out.Capsule, pr.URL)
			}
		}
	}
	// One that conflicts from the start is no use to anyone: its base is
	// merged in first.
	if files, err := ws.CapsuleConflicts(c.Repo, c.Name); err == nil && len(files) > 0 {
		return PROpened{}, refused("conflicts", "%s doesn't merge cleanly onto %s: it conflicts in %s. Merge origin/%s into it (don't rebase, don't force-push), resolve them, commit, and open it again",
			out.Capsule, ws.DefaultBranch, strings.Join(files, ", "), ws.DefaultBranch)
	}
	if err := w.holdReview(GuaranteeReviewBeforePR, m.Slug, c.Repo, c.Name, "opening a pull request", reviewThen(by, "open it")); err != nil {
		return PROpened{}, err
	}
	if o.Title == "" {
		// It can be opened: the PR writer writes it first. That takes a
		// while, so it's the daemon's, where there's one to hand it to.
		if w.opts.WritePR != nil && !o.inJob {
			_, started, err := w.Missions.StartWritingPR(m.Slug, mission.WritingPR{Capsule: out.Capsule, Step: out.Step, Draft: o.Draft, By: by.By()})
			if err != nil {
				return PROpened{}, err
			}
			if started {
				w.opts.WritePR(m.Slug, out.Capsule)
			}
			out.Writing = true
			return out, nil
		}
		o = w.writtenPR(m, c, out.Step, o)
	}
	branch, said, err := ws.PushBranch(c.Repo, c.Name, o.Quiet)
	if err != nil {
		if said != "" {
			return PROpened{}, fmt.Errorf("%w:\n%s", err, said)
		}
		return PROpened{}, err
	}
	pr, err := gh.CreatePR(ws.Org, c.Repo, github.NewPR{Head: branch, Base: ws.DefaultBranch, Title: o.Title, Body: o.Body, Draft: o.Draft})
	if err != nil {
		return PROpened{}, fmt.Errorf("%s is pushed, but the pull request wasn't opened: %w", branch, err)
	}
	out.PR = pr
	at := ""
	if out.Step > 0 {
		at = fmt.Sprintf(", at step %d", out.Step)
	}
	w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventPR, What: mission.PREventOpened, Repo: c.Repo, Capsule: c.Name, Crew: by.By(),
		Text: fmt.Sprintf("%s#%d opened%s: %s", c.Repo, pr.Number, at, o.Title)})
	// It's seen as it's opened: the watch's first sighting isn't a change.
	w.Missions.SeePR(m.Slug, out.Capsule, mission.SeenPR{Number: pr.Number, Draft: pr.IsDraft})
	w.opts.PRs.Forget(c.Repo)
	w.youActed(m.Slug, by)
	return out, nil
}

// missionCapsule is one of a mission's capsules, named repo/name (the
// repo by any of its names).
func (w *Workspace) missionCapsule(m *mission.Mission, capsule string) (mission.Capsule, error) {
	repo, name, ok := strings.Cut(strings.Trim(strings.TrimSpace(capsule), "`"), "/")
	if !ok || repo == "" || name == "" {
		return mission.Capsule{}, invalid("name the capsule as repo/name")
	}
	if r, ok := w.WS().ResolveAlias(repo); ok {
		repo = r
	}
	i := slices.IndexFunc(m.Capsules, func(c mission.Capsule) bool { return c.Repo == repo && c.Name == name })
	if i < 0 {
		return mission.Capsule{}, refused("not-attached", "%s/%s isn't one of the mission's capsules", repo, name)
	}
	return m.Capsules[i], nil
}

// currentStep is the plan's current step, if the mission has a plan and
// isn't past its last step.
func currentStep(store mission.Store, m *mission.Mission) (mission.StageState, bool) {
	brief, _ := store.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	cur, ok := mission.CurrentStage(states)
	if !ok || cur.Step == nil {
		return mission.StageState{}, false
	}
	return cur, true
}

// crewMayOpenPR is the step crew open a pull request at: theirs, the
// current one, which makes a pull request. Refused anywhere else, saying
// which step opens one.
func crewMayOpenPR(store mission.Store, m *mission.Mission, crew string) (int, error) {
	brief, _ := store.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	mine, ok := mission.StageFor(states, crew)
	cur, isCur := mission.CurrentStage(states)
	switch {
	case ok && isCur && mine.N == cur.N && cur.Step != nil && cur.Step.Type == mission.StepOpenPR:
		return 0, refused("not-the-step", "step %d is ws's: it opens the pull request itself. Commit your work and report done", cur.N)
	case ok && isCur && mine.N == cur.N && cur.Step != nil && cur.Step.MakesPR():
		return cur.N, nil
	case ok && isCur && mine.N == cur.N && cur.Step != nil:
		return 0, refused("not-the-step", "step %d doesn't make a pull request, so it's not opened here: commit your work and report done. %s", cur.N, prStepWords(states, cur.N))
	case ok && mine.Accepted():
		return 0, refused("not-the-step", "step %d, the one you're on, has been accepted: the plan has moved on. %s", mine.N, prStepWords(states, mine.N))
	}
	return 0, refused("not-the-step", "you're not on the plan's current step, so you don't open its pull request: commit your work and report done. %s", prStepWords(states, 0))
}

// prStepWords says which step after the n'th opens a pull request, if one
// does.
func prStepWords(states []mission.StageState, n int) string {
	for _, st := range states {
		if st.N > n && st.Step != nil && st.Step.MakesPR() {
			return fmt.Sprintf("Step %d opens it (%s).", st.N, st.Short())
		}
	}
	return "No step after it makes one: if the work needs a pull request, say so in your handoff."
}

// stepPRs refuses accepting a step that makes a pull request while none
// is open, or merged, from the capsules it works in: for the orchestrator,
// which moves the mission along as the plan says. The step's crew (or the
// user) open it; an Open Pull Request step's, ws opens, and that step is
// done only once it's merged.
func (w *Workspace) stepPRs(m *mission.Mission, cur mission.StageState) error {
	if cur.Step == nil || !cur.Step.MakesPR() {
		return nil
	}
	ws := w.WS()
	var mine []mission.Capsule
	named := StageCapsules(w.Missions, m)
	for _, c := range m.Capsules {
		if slices.Contains(named, c.Repo+"/"+c.Name) && !ws.Direct(c.Repo) {
			mine = append(mine, c)
		}
	}
	if len(mine) == 0 || w.GitHub() == nil {
		return nil // nowhere it could open one (a direct repo's work lands), or no GitHub to ask
	}
	for _, c := range mine {
		if p, ok := m.WritingPR(c.Repo + "/" + c.Name); ok {
			return refused("pr-writing", "the PR writer is writing %s's pull request, for step %d: ws opens it once it's written. Nothing to do but wait for it", p.Capsule, cur.N)
		}
	}
	v := BuildMissionViews(ws, w.GitHub(), []*mission.Mission{{Slug: m.Slug, Capsules: mine}})[0]
	if cur.Step.Type == mission.StepOpenPR {
		// ws's step: done when its pull request merges, which passes it.
		for _, c := range v.Capsules {
			if c.PRS == mission.PRMerged {
				return nil
			}
		}
		return refused("not-merged", "step %d is ws's: it opens the pull request, and the step's done when it merges, which passes it. Note that it waits on its review and merge (and resume the crew who worked in its capsule, if its checks fail or a review asks for changes)", cur.N)
	}
	for _, c := range v.Capsules {
		if c.PR != nil {
			return nil
		}
	}
	names := make([]string, len(mine))
	for i, c := range mine {
		names[i] = c.Repo + "/" + c.Name
	}
	return refused("no-pr", "step %d makes a pull request, and none is open from %s: its crew open it (ws mission open-pr), so resume them to, or dispatch crew to the step if nobody's on it. If the user's review is what holds it up, note that it waits for them", cur.N, strings.Join(names, " or "))
}
