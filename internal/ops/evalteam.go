package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/spend"
	"github.com/withworkspace/workspace/internal/team"
)

// Evaluating the Team page's writer: a fixture is a change that landed,
// and what's to be said of it, and an eval has the gist written as the
// Team page would (team.GistPrompt, no tools) on whichever harness and
// model it's asked to, and scores the reply.
//
// A gist has no one right wording, so what's scored is what has one: that
// the reply is in the form ws reads, that the gist is short enough, and
// whether something in the change is worth knowing, and of what kind. How
// well the gist says it is for whoever reads the output (or a grader).

// TeamFixture is a folder's fixture.toml: a change, as the Team page saw
// it, and what's expected of its gist.
type TeamFixture struct {
	Name  string `toml:"name"`
	About string `toml:"about"`

	// Repo is the repo's name, as the prompt calls it, and Display how the
	// workspace shows it ("" for Repo).
	Repo    string `toml:"repo"`
	Display string `toml:"display"`
	// PR is the pull request's number, with its Title and Body; 0 for
	// commits with none, which Subjects are the subjects of.
	PR       int      `toml:"pr"`
	Title    string   `toml:"title"`
	Body     string   `toml:"body"`
	Subjects []string `toml:"subjects"`
	// Commits are the commits on the default branch whose changes it is:
	// a pull request's merge, or the commits themselves.
	Commits []string `toml:"commits"`
	// Touches are the workspace's watches its files touch, as the writer
	// is told them.
	Touches []team.Touch `toml:"touch"`

	// Know is what's worth knowing of it: the kinds any of which will do
	// (contract, action, convention, removed, behaviour), or none, an
	// empty list. Left out, it isn't scored.
	Know *[]string `toml:"know"`
	// KnowNote is why, for whoever reads a wrong answer.
	KnowNote string `toml:"know_note"`

	Git TeamFixtureGit `toml:"git"`

	dir string
}

// TeamFixtureGit is where a team fixture's commits are: a clone on this
// machine (a bare one will do), from the fixture's folder if it isn't
// absolute.
type TeamFixtureGit struct {
	Repo string `toml:"repo"`
}

// TeamGistWords is the longest a gist or a thing worth knowing is to be,
// in words, as the prompt says.
const TeamGistWords = 30

// ReadTeamFixture reads the team fixture in dir.
func ReadTeamFixture(dir string) (TeamFixture, error) {
	var f TeamFixture
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
	switch {
	case f.Repo == "":
		return f, fmt.Errorf("%s: no repo: the change is to one", path)
	case len(f.Commits) == 0:
		return f, fmt.Errorf("%s: no commits: the change is those on the default branch", path)
	case f.PR != 0 && f.Title == "":
		return f, fmt.Errorf("%s: pull request #%d has no title", path, f.PR)
	case f.PR == 0 && len(f.Subjects) == 0:
		return f, fmt.Errorf("%s: no pr and no subjects: say what the commits are", path)
	}
	if f.Know != nil {
		for _, k := range *f.Know {
			if !slices.Contains(team.KnowKinds, k) {
				return f, fmt.Errorf("%s: know has %q: it's one of %s, or [] for none", path, k, strings.Join(team.KnowKinds, ", "))
			}
		}
	}
	return f, nil
}

// TeamEval is how a fixture is put to the Team page's writer.
type TeamEval struct {
	// Pick is who writes, as harness:model or a harness alone. "": the
	// workspace's [team].
	Pick string
	// Repo is the clone the fixture's commits are in, over its own.
	Repo string
}

// subject is f as the Team page has a landed change.
func (f TeamFixture) subject() team.Subject {
	sub := team.Subject{Strand: f.Name, Repo: f.Repo, PR: f.PR, Title: f.Title, Body: f.Body, Commits: f.Commits, Of: team.OfLanded, Kind: "pr"}
	if f.PR == 0 {
		sub.Kind, sub.Of, sub.Subjects = "commits", team.OfCommits, f.Subjects
		if sub.Title == "" {
			sub.Title = f.Subjects[0]
		}
	}
	return sub
}

// teamAsk is what f's writer is asked, its change read from the clone.
func (f TeamFixture) teamAsk(e TeamEval) (team.Ask, error) {
	repo := firstNonEmpty(e.Repo, f.Git.Repo)
	if repo == "" {
		return team.Ask{}, fmt.Errorf("its commits are in a clone on this machine: say which with --repo, or repo under [git]")
	}
	if !filepath.IsAbs(repo) {
		repo = filepath.Join(f.dir, repo)
	}
	sub := f.subject()
	m, err := team.MaterialFor(repo, sub)
	if err != nil {
		return team.Ask{}, err
	}
	if len(m.Files) == 0 {
		return team.Ask{}, fmt.Errorf("its commits change no files")
	}
	return team.Ask{Subject: sub, Material: m, Touches: f.Touches}, nil
}

func (f TeamFixture) displayName(repo string) string {
	if repo == f.Repo && f.Display != "" {
		return f.Display
	}
	return repo
}

// TeamPrompt is the prompt f's writer is given, without running it.
func (w *Workspace) TeamPrompt(f TeamFixture, e TeamEval) (string, error) {
	ask, err := f.teamAsk(e)
	if err != nil {
		return "", err
	}
	return team.GistPrompt(ask, f.displayName), nil
}

// teamPick is who writes an eval's gist: the workspace's [team], or pick
// in its place.
func (w *Workspace) teamPick(pick string) (harness.Pick, harness.Converser, error) {
	roles := Roles(w.Config())
	if pick != "" {
		h, m, err := config.ParsePick(pick)
		if err != nil {
			return harness.Pick{}, nil, err
		}
		roles.Team = harness.Pick{Harness: h, Model: m}
	}
	return roles.ForTeam()
}

// EvalTeam has f's gist written and scores the reply. A writer that can't
// be run, or fails, is the result's Error.
func (w *Workspace) EvalTeam(f TeamFixture, e TeamEval) EvalResult {
	res := EvalResult{Fixture: f.Name, Pick: e.Pick}
	pick, h, err := w.teamPick(e.Pick)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.Pick = pick.String()
	ask, err := f.teamAsk(e)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	// It reads nothing, so it's run in an empty folder, not the workspace.
	dir, err := os.MkdirTemp("", "ws-eval-team-")
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer os.RemoveAll(dir)
	run := harness.ConverseRun{Prompt: team.GistPrompt(ask, f.displayName), Dir: dir, Model: pick.Model, Alone: true, Once: true}
	start := time.Now()
	rep, err := RunHeadless(w.life(), w.Root(), "eval team "+f.Name, spend.For{Kind: spend.Eval, Subject: f.Name}, crew.Env(w.Root(), "", ""), "", gistTurnFor, h, run)
	res.TookMS = time.Since(start).Milliseconds()
	res.Calls, res.Output = len(rep.Calls), rep.Output
	if err != nil {
		res.Error = err.Error()
		return res
	}
	gist, know, err := team.ReadReply(rep.Output, ask.Material)
	res.add(yesNo("it replied in the form ws reads", err == nil, true, errText(err)))
	if err != nil {
		return res
	}
	res.add(yesNo(fmt.Sprintf("the gist is %d words at most", TeamGistWords), words(gist) <= TeamGistWords, true, fmt.Sprintf("%d words: %s", words(gist), gist)))
	for i, k := range know {
		res.add(yesNo(fmt.Sprintf("what's worth knowing (%d) is %d words at most", i+1, TeamGistWords), words(k.Text) <= TeamGistWords, true, fmt.Sprintf("%d words: %s", words(k.Text), k.Text)))
	}
	if f.Know == nil {
		return res
	}
	var kinds []string
	for _, k := range know {
		kinds = append(kinds, k.Kind)
	}
	got := strings.Join(kinds, ", ")
	if got == "" {
		got = "none"
	}
	want := *f.Know
	some := yesNo("something in it is worth knowing", len(know) > 0, len(want) > 0, got)
	some.Note = f.KnowNote
	res.add(some)
	if len(want) > 0 && len(know) > 0 {
		c := EvalCheck{Check: "what's worth knowing is a " + strings.Join(want, " or "), Expect: strings.Join(want, " or "), Got: got, Note: f.KnowNote, Score: ScoreWrong}
		if slices.ContainsFunc(kinds, func(k string) bool { return slices.Contains(want, k) }) {
			c.Score = ScoreRight
		}
		res.add(c)
	}
	return res
}

// add counts c in the result, numbered after those before it.
func (r *EvalResult) add(c EvalCheck) {
	c.N = len(r.Checks) + 1
	if c.Score != ScoreNone {
		r.Scored++
	}
	if c.Score == ScoreRight {
		r.Right++
	}
	r.Checks = append(r.Checks, c)
}

// yesNo is a check that's so or not, scored against what's wanted; why is
// what was seen.
func yesNo(check string, got, want bool, why string) EvalCheck {
	c := EvalCheck{Check: check, Expect: yes(want), Got: yes(got), Why: why, Score: ScoreRight}
	if got != want {
		c.Score = ScoreWrong
	}
	return c
}

func yes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// words is how many words s has.
func words(s string) int { return len(strings.Fields(s)) }
