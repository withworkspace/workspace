package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The PR writer writes the title and description of a pull request ws
// opens with none given: ws's own at an Open Pull Request step, crew's
// (ws mission open-pr with no --title), and yours where you leave them to
// it. It's an agent ([pr_writer]), headless and read-only in the capsule,
// told the task the pull request is for (the plan's steps whose work it
// carries, not the whole mission: prTask) and how the workspace writes
// its pull requests (PR.md, workspace.PRGuide).
//
// Once ws has found the pull request can be opened (openPR's refusals),
// it marks the PR writer writing it (mission.WritingPR) and hands it to
// the daemon (job.PR, Options.WritePR), as writing takes a while: the
// job writes it, opens it as it was asked for, and clears the mark. A
// writer that can't write it leaves it to ws's own words (prDraft), so
// the pull request still opens.

// prWriterTimeout is how long the PR writer has.
const prWriterTimeout = 10 * time.Minute

// WritePR has the PR writer write the pull request of one of a mission's
// capsules, without opening it: for you to read and change before you
// open it. Crew and the orchestrator have theirs written as ws opens it.
func (w *Workspace) WritePR(slug, capsule string, by Actor) (NewPullRequest, error) {
	if !by.IsYou() {
		return NewPullRequest{}, refused("not-yours", "the PR writer writes a pull request as ws opens it: ws mission open-pr with no --title")
	}
	m, err := w.load(slug)
	if err != nil {
		return NewPullRequest{}, err
	}
	c, err := w.missionCapsule(m, capsule)
	if err != nil {
		return NewPullRequest{}, err
	}
	step := 0
	if cur, ok := currentStep(w.Missions, m); ok && cur.Step.MakesPR() {
		step = cur.N
	}
	title, body, _, err := w.writePR(m, c, step)
	if err != nil {
		return NewPullRequest{}, fmt.Errorf("the PR writer couldn't write it: %w", err)
	}
	return NewPullRequest{Title: title, Body: body}, nil
}

// writtenPR is o with the PR writer's title, and its description where o
// has none: what ws opens. A writer that can't write it leaves it to ws's
// own words, and that's logged.
func (w *Workspace) writtenPR(m *mission.Mission, c mission.Capsule, step int, o NewPullRequest) NewPullRequest {
	capsule := c.Repo + "/" + c.Name
	title, body, who, err := w.writePR(m, c, step)
	if err != nil {
		title, body = w.prDraft(m, capsule, step)
		w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventNote, Crew: "ws", Repo: c.Repo, Capsule: c.Name,
			Text: fmt.Sprintf("the PR writer couldn't write %s's pull request (%s): ws titled it as the mission is", capsule, strings.TrimSuffix(oneLine(err.Error()), "."))})
	} else {
		w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventNote, Crew: "ws", Repo: c.Repo, Capsule: c.Name,
			Text: fmt.Sprintf("%s wrote %s's pull request: %s", who, capsule, title)})
	}
	o.Title = title
	if strings.TrimSpace(o.Body) == "" {
		o.Body = body
	}
	return o
}

// writePR is the PR writer's one run on capsule c's pull request, opened
// at step (0 for none): its title and description, and who wrote them.
func (w *Workspace) writePR(m *mission.Mission, c mission.Capsule, step int) (title, body, who string, err error) {
	pick, h, err := Roles(w.Config()).ForPRWriter()
	if err != nil {
		return "", "", "", err
	}
	ws := w.WS()
	capsule := c.Repo + "/" + c.Name
	dir := filepath.Join(ws.RepoDir(c.Repo), c.Name)
	base := ws.DefaultBranch
	p := prompt.PRWriting{Mission: m.Slug, Title: m.Title, Repo: c.Repo, Capsule: capsule, Base: base, Dir: dir,
		Branch: workspace.GitCurrentBranch(dir)}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	p.Goal = briefGoal(brief)
	tasks := w.prTask(m, capsule, step)
	for _, st := range tasks {
		p.Task = append(p.Task, taskBlock(st))
	}
	p.Handoffs = w.taskHandoffs(m, capsule, tasks)
	p.Decided = w.yourDecisions(m)
	if path := ws.PRGuide(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			p.Guide = string(data)
			if rel, err := filepath.Rel(ws.Root, path); err == nil && !strings.HasPrefix(rel, "..") {
				path = rel
			}
			p.GuidePath = path
		}
	}
	p.Template = prTemplate(dir)
	if log := gitOut(dir, "log", "--reverse", "--format=%h %s", "origin/"+base+"..HEAD"); log != "" {
		p.Commits = strings.Split(log, "\n")
		if len(p.Commits) > 60 {
			p.Commits = append(p.Commits[:60], fmt.Sprintf("… and %d more (git log shows them)", len(p.Commits)-60))
		}
	}
	p.Diff = capsuleDiff(dir, "origin/"+base)

	run := harness.ConverseRun{Prompt: prompt.PRWriter(p), Dir: dir, Model: pick.Model, Once: true}
	who = "the PR writer (" + pick.String() + ")"
	runPath := filepath.Join(w.Missions.Dir(), m.Slug, "pr-writer.log")
	rep, err := RunHeadless(w.life(), w.Root(), "pr-writer "+m.Slug, crew.Env(w.Root(), m.Slug, ""), runPath, prWriterTimeout, h, run)
	if err != nil {
		execlog.Note(w.Root(), "pr-writer "+m.Slug, "couldn't write %s's pull request: %v", capsule, err)
		return "", "", who, err
	}
	title, body, err = prompt.ReadPR(rep.Output)
	if err != nil {
		execlog.Note(w.Root(), "pr-writer "+m.Slug, "%s's pull request: %v", capsule, err)
		return "", "", who, err
	}
	return title, body, who, nil
}

// prTask is what a capsule's pull request is for, as the plan says: the
// steps whose work it carries, up to the step it's opened at (step; else
// the plan's current step, else its last). They're the steps that work in
// the capsule after the last one before that which made a pull request
// from it: what's been done there since. Only work is a task (a Work
// step, or crew's, from before steps had kinds): a review, a decision or
// a wait is the mission's, not the change's. With no plan, none: the
// brief's goal is the task.
func (w *Workspace) prTask(m *mission.Mission, capsule string, step int) []mission.StageState {
	if m.Plan == nil {
		return nil
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	upTo := step
	if upTo == 0 {
		if cur, ok := mission.CurrentStage(states); ok {
			upTo = cur.N
		} else {
			upTo = len(states)
		}
	}
	in := func(st mission.StageState) bool { return slices.Contains(w.stepCapsules(m, st), capsule) }
	from := 0
	for _, st := range states {
		if st.N < upTo && st.Step != nil && st.Step.MakesPR() && in(st) {
			from = st.N
		}
	}
	isWork := func(st mission.StageState) bool {
		return st.Step != nil && (st.Step.Type == mission.StepWork || st.Step.Type == "" && st.Step.Who == mission.WhoCrew) && !st.Skipped()
	}
	var here, inRange []mission.StageState
	for _, st := range states {
		if st.N <= from || st.N > upTo || !isWork(st) {
			continue
		}
		inRange = append(inRange, st)
		if in(st) {
			here = append(here, st)
		}
	}
	if len(here) > 0 {
		return here
	}
	return inRange // none says it's in this capsule: the work in the range is the best there is
}

// taskBlock is a step of the task as the PR writer's told it: what it
// says, what it makes, and what had to be true for it to be done.
func taskBlock(st mission.StageState) string {
	var b strings.Builder
	what := strings.TrimSpace(st.Plain())
	if st.Step != nil && st.Step.Text != "" {
		what = strings.TrimSpace(st.Step.Text)
	}
	kind := "Work"
	if st.Step != nil && st.Step.Type != "" {
		kind = st.Step.Type
	}
	fmt.Fprintf(&b, "Step %d (%s): %s\n", st.N, kind, sentence(oneLine(what)))
	if st.Step != nil {
		if st.Step.Makes != "" {
			fmt.Fprintf(&b, "  It makes: %s\n", sentence(oneLine(st.Step.Makes)))
		}
		if len(st.Step.Checks) > 0 {
			b.WriteString("  What had to be true for it to be done:\n")
			for _, c := range st.Step.Checks {
				fmt.Fprintf(&b, "  - %s\n", oneLine(c.Text))
			}
		}
	}
	return b.String()
}

// taskHandoffs are what crew handed off for the task's steps in the
// capsule, each headed by who and the step.
func (w *Workspace) taskHandoffs(m *mission.Mission, capsule string, tasks []mission.StageState) []string {
	if len(tasks) == 0 {
		return nil
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	var out []string
	for _, c := range m.Crew {
		if c.Capsule != capsule || c.HandedOff == nil {
			continue
		}
		st, ok := mission.StageFor(states, c.ID)
		if !ok || !slices.ContainsFunc(tasks, func(t mission.StageState) bool { return t.N == st.N }) {
			continue
		}
		data, err := os.ReadFile(w.Missions.HandoffPath(m.Slug, c.ID))
		if err != nil || strings.TrimSpace(string(data)) == "" {
			continue
		}
		out = append(out, fmt.Sprintf("## %s's handoff, at step %d\n\n%s", c.ID, st.N, clipHeadBytes(strings.TrimSpace(string(data)), 8<<10)))
	}
	return out
}

// yourDecisions are what you've decided on the mission that still
// stands, a line each, oldest first.
func (w *Workspace) yourDecisions(m *mission.Mission) []string {
	all, err := w.Missions.Decisions(m.Slug)
	if err != nil {
		return nil
	}
	var out []string
	for _, d := range all {
		if d.State != mission.DecisionCurrent || d.By != AsYou.By() {
			continue
		}
		line := fmt.Sprintf("%s: %s", d.ID, sentence(oneLine(d.What)))
		if why := oneLine(d.Why); why != "" {
			line += " Why: " + sentence(clipString(why, 600))
		}
		out = append(out, line)
	}
	return out
}

// prTemplateNames are where GitHub looks for a repo's pull request
// template.
var prTemplateNames = []string{
	".github/pull_request_template.md", ".github/PULL_REQUEST_TEMPLATE.md",
	"pull_request_template.md", "PULL_REQUEST_TEMPLATE.md",
	"docs/pull_request_template.md", "docs/PULL_REQUEST_TEMPLATE.md",
}

// prTemplate is the repo's own pull request template, as the capsule has
// it, or "".
func prTemplate(dir string) string {
	for _, name := range prTemplateNames {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			return clipHeadBytes(string(data), 8<<10)
		}
	}
	return ""
}

// OpenWrittenPR is the daemon's job for a pull request the PR writer's
// marked writing (job.PR): it writes it, opens it as it was asked for,
// and clears the mark. Where it can't be opened now, it's put to whoever
// it would be: an Open Pull Request step's, as the step puts it; anyone
// else's, to you. It says what it did.
func (w *Workspace) OpenWrittenPR(slug, capsule string) string {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return ""
	}
	p, ok := m.WritingPR(capsule)
	if !ok {
		return ""
	}
	defer w.Missions.EndWritingPR(slug, capsule)
	by := actorBy(p.By)
	if by.Kind == WS {
		cur, ok := currentStep(w.Missions, m)
		if !ok || cur.N != p.Step || cur.Step.Type != mission.StepOpenPR {
			return fmt.Sprintf("%s's pull request wasn't opened: the plan's moved on from step %d", capsule, p.Step)
		}
		said, ask := w.openPRIn(m, cur, capsule, true)
		if ask != "" {
			return w.askAboutPR(m, cur, fmt.Sprintf("Step %d %s.", cur.N, ask))
		}
		return said
	}
	opened, err := w.openPR(slug, capsule, NewPullRequest{Draft: p.Draft, Quiet: true, inJob: true}, by)
	if err != nil {
		text := fmt.Sprintf("ws couldn't open %s's pull request, which %s asked for: %s. Put it right and open it (Open pull request on the capsule, or ws mission open-pr).", capsule, by.By(), strings.TrimSuffix(oneLine(err.Error()), "."))
		w.Missions.Ask(slug, text, "ws")
		return "couldn't open it: " + err.Error()
	}
	if by.Kind == Crew {
		// What crew were waiting on: the orchestrator decides what follows.
		w.Notify(slug, mission.OrchestratorEvent{Crew: by.ID, Trigger: "pr"})
	}
	return opened.String()
}

// actorBy is who an Actor.By names.
func actorBy(by string) Actor {
	switch by {
	case "you", "":
		return AsYou
	case "ws":
		return AsWS
	case mission.RoleOrchestrator:
		return AsOrchestrator
	}
	return AsCrew(by)
}
