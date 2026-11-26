package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const greetPRFixture = "testdata/prwriter/greet-by-name"

// An eval's PR writer is told what ws's is (the task, the handoff, what
// was decided, the guide, the commits and the diff), in a folder laid out
// as the capsule, gone afterwards; what it writes is scored on its form,
// its title's length, and the fixture's own checks.
func TestEvalPRWriter_ScoresWhatItWrites(t *testing.T) {
	w, _ := fixture(t, "")
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	f, err := ReadPRFixture(greetPRFixture)
	if err != nil {
		t.Fatal(err)
	}
	fake.reply, fake.prompts = "<title>Greet people by the name they give</title>\n<body>\ngreet takes a name. c1 made it.\n</body>", nil
	res := w.EvalPRWriter(f, PRWriterEval{Pick: fake.Name()})
	if res.Error != "" || res.Fixture != "greet-by-name" || res.Scored != 5 || res.Right != 4 || res.AllRight() {
		t.Fatalf("result = %+v", res)
	}
	if c := res.Checks[3]; c.Check != "it doesn't name the mission or its crew" || c.Score != ScoreWrong || c.Why != "found c1" {
		t.Errorf("the check it failed = %+v", c)
	}
	if len(fake.prompts) != 1 {
		t.Fatalf("the writer was asked %d times", len(fake.prompts))
	}
	p := fake.prompts[0]
	for _, want := range []string{
		"from `demo/greet-by-name` (the branch `greet-by-name`) onto `main`",
		"Step 1 (Work): greet takes a name",
		"d1: Keep \"Hello, world\" with no name.",
		"under 60 characters",
		"## c1's handoff, at step 1",
		"+    return \"Hello, \" + name",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the prompt says nothing of %q:\n%s", want, p)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "*")); len(left) != 0 {
		t.Errorf("the folder it was written in was left: %v", left)
	}

	fake.reply = "I couldn't decide."
	if res := w.EvalPRWriter(f, PRWriterEval{Pick: fake.Name()}); res.Scored != 1 || res.Right != 0 || res.Checks[0].Got != "no" {
		t.Errorf("with no title: %+v", res)
	}
}

func TestReadPRFixture_Refuses(t *testing.T) {
	for name, c := range map[string]struct{ toml, want string }{
		"both":       {"[[check]]\ntext = \"a\"\nmatch = \"x\"\nnever = \"y\"\n", "one of match or never"},
		"neither":    {"[[check]]\ntext = \"a\"\n", "one of match or never"},
		"bad where":  {"[[check]]\ntext = \"a\"\nmatch = \"x\"\nin = \"head\"\n", `looks in "head"`},
		"bad regexp": {"[[check]]\ntext = \"a\"\nmatch = \"(\"\n", "check 1"},
		"no guide":   {"guide = \"PR.md\"\n", "isn't beside it"},
		"no work":    {"title = \"t\"\n", "no work"},
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, JudgeFixtureName), []byte(c.toml), 0o644)
		if _, err := ReadPRFixture(dir); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}
