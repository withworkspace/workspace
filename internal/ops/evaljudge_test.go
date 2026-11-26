package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
)

const greetFixture = "testdata/judge/greet-by-name"

// An eval's judge is told what a mission's is (the step, its checks, the
// handoff, the diff from where the capsule began), in a folder laid out as
// a capsule is and gone afterwards, and what it says is scored: wrong for
// met against unmet, right for the rest.
func TestEvalJudge_ScoresWhatTheJudgeSays(t *testing.T) {
	w, _ := fixture(t, "")
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	f, err := ReadJudgeFixture(greetFixture)
	if err != nil {
		t.Fatal(err)
	}
	fake.reply, fake.prompts = "1. met: greeting(name) in greet.py\n2. met: the handoff says so\n3. can't tell: nothing here says what deployed", nil
	res := w.EvalJudge(f, JudgeEval{Pick: fake.Name()})
	if res.Error != "" || res.Fixture != "greet-by-name" || res.Pick != fake.Name() || res.Right != 2 || res.Scored != 3 || res.AllRight() {
		t.Fatalf("result = %+v", res)
	}
	if got := []string{res.Checks[0].Score, res.Checks[1].Score, res.Checks[2].Score}; strings.Join(got, ",") != "right,wrong,right" {
		t.Errorf("scores = %v", got)
	}
	if c := res.Checks[1]; c.Expect != mission.CheckUnmet || c.Got != mission.CheckMet || c.Why != "the handoff says so" || c.Note == "" {
		t.Errorf("check 2 = %+v", c)
	}
	if len(fake.prompts) != 1 {
		t.Fatalf("the judge was asked %d times", len(fake.prompts))
	}
	p := fake.prompts[0]
	for _, want := range []string{
		`the mission "Greet people by name" (greet-people-by-name)`,
		"The step is 1: Work: greet takes a name",
		"in the capsule `demo/greet-by-name`",
		"2. With no name, greet prints \"Hello, world\", and a test covers it.",
		"## c1's handoff",
		"With no name it still says `Hello, world`",
		"- `python3 -m unittest` passes: met (passed in 0s)",
		"+    return \"Hello, \" + name",
		"-    return \"Hello, world\"",
		"test_greet.py",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the prompt says nothing of %q:\n%s", want, p)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "*")); len(left) != 0 {
		t.Errorf("the folder it was judged in was left: %v", left)
	}
}

// Kept, the folder it was judged in is the capsule as the judge saw it:
// the work committed on a branch of the capsule's name, from origin's
// default branch.
func TestEvalJudge_KeepsTheFolderWhenAsked(t *testing.T) {
	w, _ := fixture(t, "")
	f, _ := ReadJudgeFixture(greetFixture)
	fake.reply, fake.prompts = "1. met: a\n2. unmet: b\n3. can't tell: c", nil
	res := w.EvalJudge(f, JudgeEval{Pick: fake.Name(), Keep: true})
	if !res.AllRight() || res.Dir == "" {
		t.Fatalf("result = %+v", res)
	}
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(res.Dir)) })
	if filepath.Base(res.Dir) != "greet-by-name" || gitOut(res.Dir, "branch", "--show-current") != "greet-by-name" || gitOut(res.Dir, "status", "--porcelain") != "" {
		t.Errorf("judged in %s, on %q, with %q uncommitted", res.Dir, gitOut(res.Dir, "branch", "--show-current"), gitOut(res.Dir, "status", "--porcelain"))
	}
	if files := gitOut(res.Dir, "diff", "--name-status", "origin/main", "HEAD"); files != "M\tREADME.md\nM\tgreet.py\nA\ttest_greet.py" {
		t.Errorf("the capsule changes %q", files)
	}
}

func TestScoreJudged(t *testing.T) {
	for _, c := range []struct{ expect, got, want string }{
		{"met", "met", ScoreRight},
		{"met", "unmet", ScoreWrong},
		{"unmet", "met", ScoreWrong},
		{"unmet", "can't tell", ScoreAbstained},
		{"can't tell", "met", ScoreGuessed},
		{"can't tell", "can't tell", ScoreRight},
		{"", "met", ScoreNone},
	} {
		if got := scoreJudged(c.expect, c.got); got != c.want {
			t.Errorf("expected %q, got %q: %q, want %q", c.expect, c.got, got, c.want)
		}
	}
}

// A judge that can't be run is the result's error, and a check nothing's
// expected of is judged and not scored.
func TestEvalJudge_NotJudgedAndNotScored(t *testing.T) {
	w, _ := fixture(t, "")
	f, _ := ReadJudgeFixture(greetFixture)
	if res := w.EvalJudge(f, JudgeEval{Pick: "nobody:fast"}); res.Error == "" || res.AllRight() || len(res.Checks) != 0 {
		t.Errorf("a harness there isn't: %+v", res)
	}
	f.Checks[1].Expect = ""
	fake.reply, fake.prompts = "1. met: a\n2. met: b\n3. can't tell: c", nil
	res := w.EvalJudge(f, JudgeEval{Pick: fake.Name()})
	if !res.AllRight() || res.Right != 2 || res.Scored != 2 || res.Checks[1].Score != ScoreNone || res.Checks[1].Got != mission.CheckMet {
		t.Errorf("result = %+v", res)
	}
}

// A [git] fixture is judged at its commit of a repo here, the diff from
// its base, and the repo is left as it was.
func TestEvalJudge_GitFixture(t *testing.T) {
	w, _ := fixture(t, "")
	repo := filepath.Join(t.TempDir(), "demo")
	os.MkdirAll(repo, 0o755)
	git := func(args ...string) string {
		t.Helper()
		if err := evalGit(repo, args...); err != nil {
			t.Fatal(err)
		}
		return gitOut(repo, "rev-parse", "HEAD")
	}
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\n"), 0o644)
	git("add", "-A")
	base := git("commit", "-q", "-m", "base")
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\ntwo\n"), 0o644)
	git("add", "-A")
	work := git("commit", "-q", "-m", "work")
	// What came after the work, on a branch and under a tag: the judge
	// isn't to find it.
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\ntwo\nthree\n"), 0o644)
	git("add", "-A")
	later := git("commit", "-q", "-m", "later")
	git("branch", "finished")
	git("tag", "v1")
	// The work's branch is gone, as a landed capsule's is: the commit is
	// still the repo's.
	git("reset", "-q", "--hard", base)
	before, _ := exec.Command("git", "-C", repo, "for-each-ref").Output()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, JudgeFixtureName), []byte("step = \"1: Work: add two.\"\n[[check]]\ntext = \"a.txt has two\"\nexpect = \"met\"\n[git]\ncommit = \""+work+"\"\nbase = \""+base+"\"\n"), 0o644)
	f, err := ReadJudgeFixture(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res := w.EvalJudge(f, JudgeEval{Pick: fake.Name()}); !strings.Contains(res.Error, "--repo") {
		t.Errorf("with no repo: %+v", res)
	}
	fake.reply, fake.prompts = "1. met: a.txt line 2", nil
	res := w.EvalJudge(f, JudgeEval{Pick: fake.Name(), Repo: repo, Keep: true})
	if !res.AllRight() || res.Scored != 1 {
		t.Fatalf("result = %+v", res)
	}
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(res.Dir)) })
	// The folder stands alone, with the work's own history and no more:
	// its commit, its base as origin/main, and nothing of what came after
	// or of where the repo is.
	if refs := gitOut(res.Dir, "for-each-ref", "--format=%(refname) %(objectname)"); refs != "refs/remotes/origin/main "+base {
		t.Errorf("its refs: %q", refs)
	}
	if gitOut(res.Dir, "rev-parse", "HEAD") != work || gitOut(res.Dir, "cat-file", "-t", later) != "" || gitOut(res.Dir, "remote") != "" {
		t.Errorf("HEAD %s, the later commit is a %q there, remotes %q", gitOut(res.Dir, "rev-parse", "HEAD"), gitOut(res.Dir, "cat-file", "-t", later), gitOut(res.Dir, "remote"))
	}
	if _, err := os.Stat(filepath.Join(res.Dir, ".git", "objects", "info", "alternates")); err == nil {
		t.Error("it still borrows the repo's objects")
	}
	if gitOut(res.Dir, "fsck", "--connectivity-only") != "" || gitOut(res.Dir, "show", "HEAD:a.txt") != "one\ntwo" {
		t.Errorf("it doesn't stand alone: %q", gitOut(res.Dir, "fsck", "--connectivity-only"))
	}
	if p := fake.prompts[0]; !strings.Contains(p, "at commit "+work[:12]) || !strings.Contains(p, "+two") || strings.Contains(p, "+one") {
		t.Errorf("the prompt:\n%s", p)
	}
	if after, _ := exec.Command("git", "-C", repo, "for-each-ref").Output(); string(after) != string(before) {
		t.Errorf("the repo's refs changed:\n%s\nfrom\n%s", after, before)
	}
	f.Git.Commit = strings.Repeat("0", 40)
	if res := w.EvalJudge(f, JudgeEval{Pick: fake.Name(), Repo: repo}); !strings.Contains(res.Error, "has no commit") {
		t.Errorf("a commit that isn't there: %+v", res)
	}
}

// The prompt can be had without a run.
func TestJudgePrompt(t *testing.T) {
	w, _ := fixture(t, "")
	f, _ := ReadJudgeFixture(greetFixture)
	fake.prompts = nil
	p, err := w.JudgePrompt(f, JudgeEval{})
	if err != nil || !strings.Contains(p, "## The checks") || !strings.Contains(p, "+    return \"Hello, \" + name") || len(fake.prompts) != 0 {
		t.Errorf("err %v, asked %d times, prompt:\n%s", err, len(fake.prompts), p)
	}
}

// A folder that isn't a fixture, or one that can't be judged or scored,
// is said before anything runs.
func TestReadJudgeFixture_Refuses(t *testing.T) {
	for name, c := range map[string]struct{ toml, want string }{
		"no checks":  {"step = \"1\"\n", "no [[check]]"},
		"no text":    {"[[check]]\nexpect = \"met\"\n", "check 1 has no text"},
		"bad expect": {"[[check]]\ntext = \"a\"\nexpect = \"pass\"\n", `expects "pass"`},
		"no work":    {"[[check]]\ntext = \"a\"\n", "no work: a work folder"},
		"half a git": {"[[check]]\ntext = \"a\"\n[git]\ncommit = \"abc\"\n", "[git] needs commit"},
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, JudgeFixtureName), []byte(c.toml), 0o644)
		if _, err := ReadJudgeFixture(dir); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
	if _, err := ReadJudgeFixture(t.TempDir()); err == nil || !strings.Contains(err.Error(), "isn't a judge fixture") {
		t.Errorf("an empty folder: %v", err)
	}
}
