package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/spend"
)

// Evaluating the PR writer: a fixture is a capsule's work as it was to be
// opened, and what the pull request was for, and an eval has the PR
// writer write it as ws would (prompt.PRWriter, read-only in a folder laid
// out as the capsule) on whichever harness and model it's asked to, and
// scores what comes back.
//
// A pull request has no one right wording, so what's scored is what can
// be: that it answered in the form ws reads, a title short enough and a
// description, and the fixture's own checks, patterns the title or the
// description is to have or never to. How well it reads is for whoever
// reads the output (or a grader).

// PRFixture is a folder's fixture.toml for the PR writer. Beside it are
// the work, as a judge fixture's is (work and base folders, or [git]);
// handoff*.md, what crew handed off, each as the writer is given it; and
// the workspace's guide, if Guide names one.
type PRFixture struct {
	Name  string `toml:"name"`
	About string `toml:"about"`

	Mission string `toml:"mission"` // the mission's slug
	Title   string `toml:"title"`   // the mission's title
	Goal    string `toml:"goal"`
	// Repo is the repo's name, Capsule repo/name, and Branch the branch
	// it's opened from (Capsule's name without one).
	Repo    string `toml:"repo"`
	Capsule string `toml:"capsule"`
	Branch  string `toml:"branch"`
	// Task is what the pull request is for, a block each, as the writer's
	// told the plan's steps; Decided what the user decided, a line each.
	Task    []string `toml:"task"`
	Decided []string `toml:"decided"`
	// Guide is a file beside the fixture that's the workspace's PR.md.
	Guide string `toml:"guide"`
	// TitleMax is the longest the title's to be, in characters: 72 when
	// it isn't said.
	TitleMax int `toml:"title_max"`

	Checks []PRFixtureCheck `toml:"check"`
	Git    *JudgeFixtureGit `toml:"git"`

	dir string
}

// PRFixtureCheck is a pattern a pull request is to have, or never to.
type PRFixtureCheck struct {
	Text string `toml:"text"`
	// In is where to look: title, body, or both ("").
	In string `toml:"in"`
	// Match is a pattern it's to have, and Never one it's never to; each
	// is a regular expression, matched ignoring case.
	Match string `toml:"match"`
	Never string `toml:"never"`
	Note  string `toml:"note"`
}

// DefaultPRTitleMax is how long a title's to be at most where no guide
// says, as the prompt asks.
const DefaultPRTitleMax = 72

// ReadPRFixture reads the PR writer fixture in dir.
func ReadPRFixture(dir string) (PRFixture, error) {
	var f PRFixture
	path := filepath.Join(dir, JudgeFixtureName)
	if _, err := toml.DecodeFile(path, &f); err != nil {
		if os.IsNotExist(err) {
			return f, fmt.Errorf("%s has no %s: it isn't a fixture", dir, JudgeFixtureName)
		}
		return f, fmt.Errorf("reading %s: %w", path, err)
	}
	f.dir = dir
	if f.Name == "" {
		if abs, err := filepath.Abs(dir); err == nil {
			f.Name = filepath.Base(abs)
		}
	}
	if f.Capsule == "" {
		f.Capsule = firstNonEmpty(f.Repo, "eval") + "/" + f.Name
	}
	if f.Repo == "" {
		f.Repo, _, _ = strings.Cut(f.Capsule, "/")
	}
	if f.Branch == "" {
		f.Branch = filepath.Base(f.Capsule)
	}
	if f.TitleMax == 0 {
		f.TitleMax = DefaultPRTitleMax
	}
	if f.Guide != "" {
		if _, err := os.Stat(filepath.Join(dir, f.Guide)); err != nil {
			return f, fmt.Errorf("%s: its guide %s isn't beside it", path, f.Guide)
		}
	}
	for i, c := range f.Checks {
		switch {
		case strings.TrimSpace(c.Text) == "":
			return f, fmt.Errorf("%s: check %d has no text", path, i+1)
		case (c.Match == "") == (c.Never == ""):
			return f, fmt.Errorf("%s: check %d needs one of match or never", path, i+1)
		case c.In != "" && c.In != "title" && c.In != "body":
			return f, fmt.Errorf("%s: check %d looks in %q: it's title, body, or both (left out)", path, i+1, c.In)
		}
		if _, err := regexp.Compile("(?i)" + c.Match + c.Never); err != nil {
			return f, fmt.Errorf("%s: check %d: %w", path, i+1, err)
		}
	}
	return f, checkWork(dir, path, f.Git)
}

// PRWriterEval is how a fixture is put to the PR writer.
type PRWriterEval struct {
	// Pick is who writes, as harness:model or a harness alone. "": the
	// workspace's [pr_writer].
	Pick string
	// Repo is the repo a [git] fixture's commits are in, over its own.
	Repo string
	// Keep leaves the folder it was written in.
	Keep bool
}

// prWritingOf is what f's writer is told, with the work laid out as the
// capsule's folder. done removes the folder (unless it's kept).
func (w *Workspace) prWritingOf(f PRFixture, e PRWriterEval) (prompt.PRWriting, func(), error) {
	base := firstNonEmpty(w.WS().DefaultBranch, "main")
	dir, _, done, err := fixtureWork{Dir: f.dir, Capsule: f.Capsule, Git: f.Git}.layOut(e.Repo, e.Keep, base)
	if err != nil {
		return prompt.PRWriting{}, nil, err
	}
	p := prompt.PRWriting{Mission: firstNonEmpty(f.Mission, f.Name), Title: firstNonEmpty(f.Title, f.Name), Goal: f.Goal, Task: f.Task, Decided: f.Decided,
		Repo: f.Repo, Capsule: f.Capsule, Branch: f.Branch, Base: base, Dir: dir}
	handoffs, _ := filepath.Glob(filepath.Join(f.dir, "handoff*.md"))
	sort.Strings(handoffs)
	for _, h := range handoffs {
		if data, err := os.ReadFile(h); err == nil && strings.TrimSpace(string(data)) != "" {
			p.Handoffs = append(p.Handoffs, strings.TrimSpace(string(data)))
		}
	}
	if f.Guide != "" {
		if data, err := os.ReadFile(filepath.Join(f.dir, f.Guide)); err == nil {
			p.Guide, p.GuidePath = string(data), filepath.Base(f.Guide)
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
	return p, done, nil
}

// PRWriterPrompt is the prompt f's writer is given, without running it.
func (w *Workspace) PRWriterPrompt(f PRFixture, e PRWriterEval) (string, error) {
	e.Keep = false
	p, done, err := w.prWritingOf(f, e)
	if err != nil {
		return "", err
	}
	defer done()
	return prompt.PRWriter(p), nil
}

// prWriterPick is who writes an eval's pull request: the workspace's
// [pr_writer], or pick in its place.
func (w *Workspace) prWriterPick(pick string) (harness.Pick, harness.Converser, error) {
	roles := Roles(w.Config())
	if pick != "" {
		h, m, err := config.ParsePick(pick)
		if err != nil {
			return harness.Pick{}, nil, err
		}
		roles.PRWriter = harness.Pick{Harness: h, Model: m}
	}
	return roles.ForPRWriter()
}

// EvalPRWriter has f's pull request written and scores it. A writer that
// can't be run, or fails, is the result's Error.
func (w *Workspace) EvalPRWriter(f PRFixture, e PRWriterEval) EvalResult {
	res := EvalResult{Fixture: f.Name, Pick: e.Pick}
	pick, h, err := w.prWriterPick(e.Pick)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.Pick = pick.String()
	p, done, err := w.prWritingOf(f, e)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer done()
	if e.Keep {
		res.Dir = p.Dir
	}
	run := harness.ConverseRun{Prompt: prompt.PRWriter(p), Dir: p.Dir, Model: pick.Model, Once: true}
	start := time.Now()
	// No mission, no crew: its ws commands act for nobody.
	rep, err := RunHeadless(w.life(), w.Root(), "eval pr-writer "+f.Name, spend.For{Kind: spend.Eval, Subject: f.Name}, crew.Env(w.Root(), "", ""), "", prWriterTimeout, h, run)
	res.TookMS = time.Since(start).Milliseconds()
	res.Calls, res.Output = len(rep.Calls), rep.Output
	if err != nil {
		res.Error = err.Error()
		return res
	}
	title, body, err := prompt.ReadPR(rep.Output)
	res.add(yesNo("it answered with a <title>", err == nil, true, errText(err)))
	if err != nil {
		return res
	}
	n := len([]rune(title))
	res.add(yesNo(fmt.Sprintf("the title is %d characters at most", f.TitleMax), n <= f.TitleMax, true, fmt.Sprintf("%d: %s", n, title)))
	res.add(yesNo("it has a description", strings.TrimSpace(body) != "", true, ""))
	for _, c := range f.Checks {
		in := title + "\n" + body
		switch c.In {
		case "title":
			in = title
		case "body":
			in = body
		}
		pattern, want := c.Match, true
		if c.Never != "" {
			pattern, want = c.Never, false
		}
		found := regexp.MustCompile("(?i)" + pattern).FindString(in)
		ec := yesNo(c.Text, found != "", want, "")
		if found != "" {
			ec.Why = "found " + strings.TrimSpace(found)
		}
		ec.Note = c.Note
		res.add(ec)
	}
	return res
}
