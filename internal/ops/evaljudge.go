package ops

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/spend"
)

// Evaluating the judge: a fixture is a step's work as its crew left it and
// what each of its checks should come to, and an eval puts it to the judge
// as a mission would (askJudge) on whichever harness and model it's asked
// to, and says how what the judge said compares. It's what says whether a
// change to the judge's prompt, or another model in its place, judges as
// well.

// JudgeFixtureName is the file that makes a folder a judge fixture.
const JudgeFixtureName = "fixture.toml"

// JudgeFixture is a folder's fixture.toml: what the judge is told of a
// step, and what it should say. Beside it are handoff.md (what the crew
// handed off) and the work itself, one of two ways:
//
//   - work/, the capsule's files as the crew left them, and base/ if the
//     capsule began from something (what its default branch had): the
//     diff the judge is shown is between them.
//   - [git]: a commit of a repo on this machine, and the commit it left
//     its default branch at.
type JudgeFixture struct {
	// Name is what the fixture's called (its folder's name without one),
	// and About why it's kept: what it catches.
	Name  string `toml:"name"`
	About string `toml:"about"`

	Mission string `toml:"mission"` // the mission's slug
	Title   string `toml:"title"`
	Step    string `toml:"step"` // as the plan says it: "3: Work: fix it."
	Makes   string `toml:"makes"`
	Goal    string `toml:"goal"`
	Crew    string `toml:"crew"`    // who did it: c1 without one
	Capsule string `toml:"capsule"` // repo/name: the folder it's judged in takes its name
	// Ran is what the checks ws ran came to, and Artifacts what the
	// mission had made, a line each as the judge is told them.
	Ran       []string `toml:"ran"`
	Artifacts []string `toml:"artifacts"`

	Checks []JudgeFixtureCheck `toml:"check"`
	Git    *JudgeFixtureGit    `toml:"git"`

	dir string // the fixture's folder
}

// JudgeFixtureCheck is one check the judge is asked about.
type JudgeFixtureCheck struct {
	Text string `toml:"text"`
	// Expect is what it should come to: met, unmet or can't tell. None:
	// the check is judged, and not scored.
	Expect string `toml:"expect"`
	// Note is why, for whoever reads a wrong answer.
	Note string `toml:"note"`
}

// JudgeFixtureGit is a fixture's work as commits of a repo on this
// machine.
type JudgeFixtureGit struct {
	// Repo is the repo's folder (a bare one will do), from the fixture's
	// own if it isn't absolute. JudgeEval.Repo, if set, is used instead.
	Repo string `toml:"repo"`
	// Commit is the work as it was judged, and Base where it left its
	// default branch.
	Commit string `toml:"commit"`
	Base   string `toml:"base"`
}

// ReadJudgeFixture reads the fixture in dir.
func ReadJudgeFixture(dir string) (JudgeFixture, error) {
	var f JudgeFixture
	path := filepath.Join(dir, JudgeFixtureName)
	if _, err := toml.DecodeFile(path, &f); err != nil {
		if os.IsNotExist(err) {
			return f, fmt.Errorf("%s has no %s: it isn't a judge fixture", dir, JudgeFixtureName)
		}
		return f, fmt.Errorf("reading %s: %w", path, err)
	}
	f.dir = dir
	if f.Name == "" {
		if abs, err := filepath.Abs(dir); err == nil {
			f.Name = filepath.Base(abs)
		}
	}
	if f.Crew == "" {
		f.Crew = "c1"
	}
	if f.Capsule == "" {
		f.Capsule = "eval/" + f.Name
	}
	if len(f.Checks) == 0 {
		return f, fmt.Errorf("%s: no [[check]]: there's nothing to judge", path)
	}
	for i, c := range f.Checks {
		if strings.TrimSpace(c.Text) == "" {
			return f, fmt.Errorf("%s: check %d has no text", path, i+1)
		}
		switch c.Expect {
		case "", mission.CheckMet, mission.CheckUnmet, mission.CheckCantTell:
		default:
			return f, fmt.Errorf("%s: check %d expects %q: it's %s, %s or %s", path, i+1, c.Expect, mission.CheckMet, mission.CheckUnmet, mission.CheckCantTell)
		}
	}
	_, hasWork := isDir(filepath.Join(dir, "work"))
	switch {
	case f.Git != nil && hasWork:
		return f, fmt.Errorf("%s: both [git] and a work folder: the work is one or the other", path)
	case f.Git != nil && (f.Git.Commit == "" || f.Git.Base == ""):
		return f, fmt.Errorf("%s: [git] needs commit (the work) and base (where it left its default branch)", path)
	case f.Git == nil && !hasWork:
		return f, fmt.Errorf("%s: no work to judge: a work folder beside it, or [git]", path)
	}
	return f, nil
}

func isDir(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	return info, err == nil && info.IsDir()
}

// JudgeEval is how a fixture is put to the judge.
type JudgeEval struct {
	// Pick is who judges, as [crew] allowed writes one (harness:model, or
	// a harness alone for its own model). "": the workspace's judge.
	Pick string
	// Repo is the repo a [git] fixture's commits are in, over its own.
	Repo string
	// Keep leaves the folder it was judged in, for a look.
	Keep bool
}

// JudgeScore is how a check's verdict compares with what was expected.
const (
	// ScoreRight: it said what was expected.
	ScoreRight = "right"
	// ScoreWrong: met for unmet, or unmet for met. The one that matters:
	// a step passes that shouldn't, or is sent back that was done.
	ScoreWrong = "wrong"
	// ScoreAbstained: can't tell, of something it could have told. It
	// costs the user a look, and nothing else.
	ScoreAbstained = "abstained"
	// ScoreGuessed: met or unmet, of something it couldn't have told.
	ScoreGuessed = "guessed"
	// ScoreNone: nothing was expected of it.
	ScoreNone = ""
)

// EvalCheck is one thing an eval looked for, and how it came out: a
// check as the judge came to it, or a command the orchestrator was to
// run, or never to.
type EvalCheck struct {
	N      int    `json:"n"`
	Check  string `json:"check"`
	Expect string `json:"expect,omitempty"`
	Got    string `json:"got"`
	Why    string `json:"why,omitempty"`
	Score  string `json:"score,omitempty"`
	Note   string `json:"note,omitempty"`
}

// EvalResult is an eval of one fixture, whichever role's.
type EvalResult struct {
	Fixture string `json:"fixture"`
	Pick    string `json:"pick"`
	// Checks are its checks as judged. Right is how many came to what was
	// expected, of Scored, those anything was expected of.
	Checks []EvalCheck `json:"checks,omitempty"`
	Right  int         `json:"right"`
	Scored int         `json:"scored"`
	// Calls is how many tools the agent used, and TookMS how long it took.
	Calls  int   `json:"calls"`
	TookMS int64 `json:"took_ms"`
	// Ran is each ws mission command the orchestrator ran, in order, as
	// OrchestratorCommand.String says one.
	Ran []string `json:"ran,omitempty"`
	// Output is what the agent said, as it said it.
	Output string `json:"output,omitempty"`
	// Error is why there's nothing to score: the agent couldn't be run,
	// or failed.
	Error string `json:"error,omitempty"`
	// Dir is the folder it was judged in, or the workspace it was run
	// in, kept.
	Dir string `json:"dir,omitempty"`
}

// AllRight says whether every check that was scored came out right, and
// the judge ran.
func (r EvalResult) AllRight() bool { return r.Error == "" && r.Right == r.Scored }

// scoreJudged is how got compares with expect.
func scoreJudged(expect, got string) string {
	switch {
	case expect == "":
		return ScoreNone
	case got == expect:
		return ScoreRight
	case got == mission.CheckCantTell:
		return ScoreAbstained
	case expect == mission.CheckCantTell:
		return ScoreGuessed
	}
	return ScoreWrong
}

// judgePick is who judges an eval: the workspace's judge, or pick in its
// place.
func (w *Workspace) judgePick(pick string) (harness.Pick, harness.Converser, error) {
	roles := Roles(w.Config())
	if pick != "" {
		h, m, err := config.ParsePick(pick)
		if err != nil {
			return harness.Pick{}, nil, err
		}
		roles.Judge = harness.Pick{Harness: h, Model: m}
	}
	return roles.ForJudge()
}

// JudgePrompt is the prompt f's judge is given, without running it. The
// folder it would be judged in is made and removed, so the diff is the
// one it would see; the folder's path in the prompt is gone by the time
// it's read.
func (w *Workspace) JudgePrompt(f JudgeFixture, e JudgeEval) (string, error) {
	e.Keep = false
	j, done, err := w.judgingOf(f, e)
	if err != nil {
		return "", err
	}
	defer done()
	return prompt.Judge(j), nil
}

// EvalJudge puts f to the judge and scores what it says. A judge that
// can't be run, or fails, is the result's Error, not EvalJudge's: one
// fixture failing doesn't end a run of them.
func (w *Workspace) EvalJudge(f JudgeFixture, e JudgeEval) EvalResult {
	res := EvalResult{Fixture: f.Name, Pick: e.Pick}
	pick, h, err := w.judgePick(e.Pick)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.Pick = pick.String()
	j, done, err := w.judgingOf(f, e)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer done()
	if e.Keep {
		res.Dir = j.Dir
	}
	start := time.Now()
	// No mission, no crew: the judge's ws commands act for nobody.
	rep, verdicts, err := w.askJudge(pick, h, j, "eval judge "+f.Name, spend.For{Kind: spend.Eval, Subject: f.Name}, crew.Env(w.Root(), "", ""), "")
	res.TookMS = time.Since(start).Milliseconds()
	res.Calls, res.Output = len(rep.Calls), rep.Output
	if err != nil {
		res.Error = err.Error()
		return res
	}
	for i, c := range f.Checks {
		jc := EvalCheck{N: i + 1, Check: c.Text, Expect: c.Expect, Got: verdicts[i].Result, Why: verdicts[i].Why, Note: c.Note}
		jc.Score = scoreJudged(c.Expect, jc.Got)
		if jc.Score != ScoreNone {
			res.Scored++
		}
		if jc.Score == ScoreRight {
			res.Right++
		}
		res.Checks = append(res.Checks, jc)
	}
	return res
}

// judgingOf is what f's judge is told, with the work laid out in a folder
// of its own as a capsule's is: a git repo at the work's commit, whose
// origin/<default branch> is where it began. done removes the folder
// (unless it's kept).
func (w *Workspace) judgingOf(f JudgeFixture, e JudgeEval) (prompt.Judging, func(), error) {
	branch := firstNonEmpty(w.WS().DefaultBranch, "main")
	dir, commit, done, err := f.layOut(e, branch)
	if err != nil {
		return prompt.Judging{}, nil, err
	}
	j := prompt.Judging{Mission: firstNonEmpty(f.Mission, f.Name), Title: firstNonEmpty(f.Title, f.Name), Step: f.Step, Makes: f.Makes, Goal: f.Goal,
		Crew: f.Crew, Capsule: f.Capsule, Commit: commit, Dir: dir, Ran: f.Ran, Artifacts: f.Artifacts}
	for _, c := range f.Checks {
		j.Checks = append(j.Checks, c.Text)
	}
	if data, err := os.ReadFile(filepath.Join(f.dir, "handoff.md")); err == nil {
		j.Handoff = string(data)
	}
	j.Diff = capsuleDiff(dir, "origin/"+branch)
	return j, done, nil
}

// layOut makes the folder f is judged in: named as its capsule is, in a
// temporary folder of its own, never the fixture's (an agent told to
// change nothing is still best kept from what's kept).
func (f JudgeFixture) layOut(e JudgeEval, branch string) (dir, commit string, done func(), err error) {
	tmp, err := os.MkdirTemp("", "ws-eval-")
	if err != nil {
		return "", "", nil, err
	}
	// macOS's temp folder is a link: git and the agent say where it leads.
	if resolved, err := filepath.EvalSymlinks(tmp); err == nil {
		tmp = resolved
	}
	done = func() {
		if !e.Keep {
			os.RemoveAll(tmp)
		}
	}
	dir = filepath.Join(tmp, filepath.Base(f.Capsule))
	if f.Git != nil {
		err = f.layOutGit(dir, firstNonEmpty(e.Repo, f.Git.Repo), branch)
	} else {
		err = f.layOutFolders(dir, branch)
	}
	if err != nil {
		os.RemoveAll(tmp)
		return "", "", nil, err
	}
	return dir, gitOut(dir, "rev-parse", "HEAD"), done, nil
}

// evalGit runs git in dir as the eval: its commits are nobody's, and no
// config of the user's (a hook, a signing key) has a say.
func evalGit(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=ws eval", "-c", "user.email=eval@ws.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// layOutFolders makes dir from the fixture's folders: base as the default
// branch had it (nothing, without one), then work as the capsule's one
// commit on a branch of its name.
func (f JudgeFixture) layOutFolders(dir, branch string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := evalGit(dir, "init", "-q", "-b", branch); err != nil {
		return err
	}
	if base := filepath.Join(f.dir, "base"); dirExists(base) {
		if err := os.CopyFS(dir, os.DirFS(base)); err != nil {
			return err
		}
	}
	steps := [][]string{
		{"add", "-A"},
		{"commit", "-q", "--allow-empty", "-m", "base"},
		{"update-ref", "refs/remotes/origin/" + branch, "HEAD"},
		{"checkout", "-q", "-b", filepath.Base(f.Capsule)},
		{"rm", "-rq", "--ignore-unmatch", "."},
	}
	for _, s := range steps {
		if err := evalGit(dir, s...); err != nil {
			return err
		}
	}
	if err := os.CopyFS(dir, os.DirFS(filepath.Join(f.dir, "work"))); err != nil {
		return err
	}
	if err := evalGit(dir, "add", "-A"); err != nil {
		return err
	}
	return evalGit(dir, "commit", "-q", "--allow-empty", "-m", "work")
}

func dirExists(path string) bool {
	_, ok := isDir(path)
	return ok
}

// layOutGit makes dir from commits of repo: the work's commit checked
// out, its origin/<branch> where the work began, and nothing else of the
// repo's. It's cloned borrowing repo's objects (so nothing of repo's
// changes, and a commit no branch holds any more is still there), then
// made to stand alone with only the work's own history: no other branch,
// no tag, no object from after it, and no word of where repo is. A judge
// that goes looking finds what the capsule had then, not the work as it
// was finished later.
func (f JudgeFixture) layOutGit(dir, repo, branch string) error {
	if repo == "" {
		return errors.New("its commits are in a repo on this machine: say which with --repo, or repo under [git]")
	}
	if !filepath.IsAbs(repo) {
		repo = filepath.Join(f.dir, repo)
	}
	for _, c := range []string{f.Git.Commit, f.Git.Base} {
		if gitOut(repo, "cat-file", "-t", c) != "commit" {
			return fmt.Errorf("%s has no commit %s: fetch it there first", repo, c)
		}
	}
	if out, err := exec.Command("git", "clone", "-q", "--shared", "--no-checkout", repo, dir).CombinedOutput(); err != nil {
		return fmt.Errorf("cloning %s: %w: %s", repo, err, strings.TrimSpace(string(out)))
	}
	if err := evalGit(dir, "checkout", "-q", "--detach", f.Git.Commit); err != nil {
		return err
	}
	// Every ref it came with goes (the remote's branches, tags), and the
	// remote with them: only HEAD and the base are left to reach from.
	if err := evalGit(dir, "remote", "remove", "origin"); err != nil {
		return err
	}
	for _, ref := range strings.Fields(gitOut(dir, "for-each-ref", "--format=%(refname)")) {
		if err := evalGit(dir, "update-ref", "-d", ref); err != nil {
			return err
		}
	}
	if err := evalGit(dir, "update-ref", "refs/remotes/origin/"+branch, f.Git.Base); err != nil {
		return err
	}
	// What they reach, in a pack of its own, and the borrowing ended: the
	// rest stays behind with the repo.
	if err := evalGit(dir, "repack", "-a", "-d", "-q"); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, ".git", "objects", "info", "alternates"))
}
