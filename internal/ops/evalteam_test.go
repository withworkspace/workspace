package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// teamRepo is a repo with a pull request's merge on main: the change a
// team fixture is of.
func teamRepo(t *testing.T) (repo, merge string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), "demo")
	os.MkdirAll(repo, 0o755)
	git := func(args ...string) string {
		t.Helper()
		if err := evalGit(repo, args...); err != nil {
			t.Fatal(err)
		}
		return gitOut(repo, "rev-parse", "HEAD")
	}
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(repo, "api.go"), []byte("package api\n\nfunc Greet() string { return \"hi\" }\n"), 0o644)
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	git("checkout", "-q", "-b", "greet")
	os.WriteFile(filepath.Join(repo, "api.go"), []byte("package api\n\nfunc Greet(name string) string { return \"hi \" + name }\n"), 0o644)
	git("commit", "-qam", "greet by name")
	git("checkout", "-q", "main")
	return repo, git("merge", "-q", "--no-ff", "-m", "Merge pull request #7", "greet")
}

func teamFixture(t *testing.T, repo, merge, know string) TeamFixture {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "greet-takes-a-name")
	os.MkdirAll(dir, 0o755)
	toml := "repo = \"demo\"\npr = 7\ntitle = \"Greet by name\"\nbody = \"Greet takes a name.\"\ncommits = [\"" + merge + "\"]\n" + know + "\n[git]\nrepo = \"" + repo + "\"\n"
	os.WriteFile(filepath.Join(dir, JudgeFixtureName), []byte(toml), 0o644)
	f, err := ReadTeamFixture(dir)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// An eval's gist is written of the change as the Team page has it (the
// pull request, its merge's diff), and scored on its form, its length and
// whether what it says is worth knowing is what the fixture expects.
func TestEvalTeam_ScoresTheGist(t *testing.T) {
	w, _ := fixture(t, "")
	repo, merge := teamRepo(t)
	f := teamFixture(t, repo, merge, "know = [\"contract\"]\nknow_note = \"Greet's callers have to pass a name.\"")

	fake.reply, fake.prompts = "gist: Greet takes the name of who it greets.\nknow: contract | api.go | Greet now takes a name: pass one.", nil
	res := w.EvalTeam(f, TeamEval{Pick: fake.Name()})
	if res.Error != "" || !res.AllRight() || res.Scored != 5 {
		t.Fatalf("result = %+v", res)
	}
	if len(fake.prompts) != 1 || !strings.Contains(fake.prompts[0], "Greet by name") || !strings.Contains(fake.prompts[0], "+func Greet(name string)") {
		t.Fatalf("prompts = %q", fake.prompts)
	}

	fake.reply = "gist: Greet takes a name.\nknow: none"
	res = w.EvalTeam(f, TeamEval{Pick: fake.Name()})
	if res.Right != 2 || res.Scored != 3 || res.Checks[2].Got != "no" || res.Checks[2].Note == "" {
		t.Errorf("with nothing worth knowing: %+v", res)
	}

	fake.reply = "gist: " + strings.Repeat("word ", 31) + "\nknow: behaviour | api.go | Greet says hi and the name."
	res = w.EvalTeam(f, TeamEval{Pick: fake.Name()})
	if res.Right != 3 || res.Checks[1].Score != ScoreWrong || res.Checks[4].Score != ScoreWrong || res.Checks[4].Got != "behaviour" {
		t.Errorf("a long gist, of another kind: %+v", res)
	}

	fake.reply = "It renames Greet."
	if res := w.EvalTeam(f, TeamEval{Pick: fake.Name()}); res.Scored != 1 || res.Right != 0 {
		t.Errorf("not in the form: %+v", res)
	}
}

// With nothing expected of what's worth knowing, it isn't scored; [] is
// none expected.
func TestEvalTeam_KnowNoneAndUnscored(t *testing.T) {
	w, _ := fixture(t, "")
	repo, merge := teamRepo(t)
	fake.reply = "gist: Greet takes a name.\nknow: contract | api.go | Pass a name."
	if res := w.EvalTeam(teamFixture(t, repo, merge, ""), TeamEval{Pick: fake.Name()}); !res.AllRight() || res.Scored != 3 {
		t.Errorf("unscored: %+v", res)
	}
	res := w.EvalTeam(teamFixture(t, repo, merge, "know = []"), TeamEval{Pick: fake.Name()})
	if res.AllRight() || res.Checks[3].Expect != "no" || res.Checks[3].Got != "yes" {
		t.Errorf("none expected: %+v", res)
	}
}

func TestReadTeamFixture_Refuses(t *testing.T) {
	for name, c := range map[string]struct{ toml, want string }{
		"no repo":     {"commits = [\"a\"]\n", "no repo"},
		"no commits":  {"repo = \"d\"\n", "no commits"},
		"no title":    {"repo = \"d\"\npr = 3\ncommits = [\"a\"]\n", "#3 has no title"},
		"no subjects": {"repo = \"d\"\ncommits = [\"a\"]\n", "no pr and no subjects"},
		"bad kind":    {"repo = \"d\"\npr = 3\ntitle = \"t\"\ncommits = [\"a\"]\nknow = [\"api\"]\n", `know has "api"`},
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, JudgeFixtureName), []byte(c.toml), 0o644)
		if _, err := ReadTeamFixture(dir); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}
