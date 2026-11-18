package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/orchestrator"
)

func orchestratorFixtureDir(t *testing.T, toml string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ready")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, OrchestratorFixtureName), []byte(toml), 0o644)
	os.WriteFile(filepath.Join(dir, "seed.sh"), []byte("true\n"), 0o644)
	return dir
}

// The fixtures kept here read, with their seeds where they say.
func TestReadOrchestratorFixture_TheKeptOnes(t *testing.T) {
	for name, want := range map[string]string{"ready-to-start": "dispatch", "review-is-yours": "note"} {
		f, err := ReadOrchestratorFixture(filepath.Join("testdata", "orchestrator", name))
		if err != nil {
			t.Fatal(err)
		}
		if f.Name != name || f.Mission != "greet-by-name" || f.Trigger != "manual" || len(f.Expect) != 1 || f.Expect[0] != want || len(f.Never) == 0 || filepath.Base(f.SeedPath()) != "seed.sh" {
			t.Errorf("%s = %+v", name, f)
		}
	}
}

// A folder that isn't a fixture, or one with nothing to seed or score, is
// said before a workspace is made for it.
func TestReadOrchestratorFixture_Refuses(t *testing.T) {
	for name, c := range map[string]struct{ toml, want string }{
		"no mission":    {"expect = [\"note\"]\n", "no mission"},
		"nothing asked": {"mission = \"m\"\n", "neither expect nor never"},
		"not a word":    {"mission = \"m\"\nexpect = [\"ws mission note\"]\n", "the word after ws mission"},
		"both":          {"mission = \"m\"\nexpect = [\"note\"]\nnever = [\"note\"]\n", "both expected and never"},
		"no seed":       {"mission = \"m\"\nexpect = [\"note\"]\nseed = \"gone.sh\"\n", "no seed at"},
	} {
		if _, err := ReadOrchestratorFixture(orchestratorFixtureDir(t, c.toml)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
	if _, err := ReadOrchestratorFixture(t.TempDir()); err == nil || !strings.Contains(err.Error(), "isn't an orchestrator fixture") {
		t.Errorf("an empty folder: %v", err)
	}
}

// What the orchestrator ran is its ws mission commands, in order, each
// with whether it went through: a read (ws show), another tool, and words
// about a command in what it was told aren't among them.
func TestOrchestratorCommands(t *testing.T) {
	calls := []orchestrator.Call{
		{Tool: "Read", Input: "/w/.ws/missions/m/orchestrator/1.prompt.md"},
		{Tool: "Bash", Input: "ws show -f llm"},
		{Tool: "Bash", Input: "ws mission dispatch \"make `go run . Ada` work\"", Error: true, Refused: true},
		{Tool: "bash", Input: "ws mission dispatch 'make it work'"},
		{Tool: "Bash", Input: "ws mission resume-with c1 \"tests\"", Error: true},
		{Tool: "Bash", Input: "cd /w && workspace mission note \"Orchestrator: done\"; ws mission flag 'ok?'"},
		{Tool: "Bash", Input: "echo aws mission accept"},
	}
	var got []string
	for _, c := range OrchestratorCommands(calls) {
		got = append(got, c.String())
	}
	if want := "dispatch (refused), dispatch, resume-with (failed), note, flag"; strings.Join(got, ", ") != want {
		t.Errorf("ran %q, want %q", strings.Join(got, ", "), want)
	}
}

// A command it's to run is right once it's run and gone through; one it's
// never to run is wrong if it so much as tried.
func TestOrchestratorFixture_Score(t *testing.T) {
	f := OrchestratorFixture{Expect: []string{"dispatch", "note", "flag", "wait"}, Never: []string{"accept", "skip", "conclude"}}
	ran := []OrchestratorCommand{
		{Verb: "dispatch", Refused: true}, {Verb: "dispatch"}, // refused, then through
		{Verb: "note", Refused: true},
		{Verb: "flag", Failed: true},
		{Verb: "accept", Refused: true}, // tried
		{Verb: "skip"},
	}
	var got []string
	for _, c := range f.Score(ran) {
		got = append(got, c.Check+" "+c.Expect+": "+c.Got+" = "+c.Score)
	}
	want := []string{
		"ws mission dispatch runs: ran = right",
		"ws mission note runs: tried, and was refused = wrong",
		"ws mission flag runs: tried, and it failed = wrong",
		"ws mission wait runs: didn't = wrong",
		"ws mission accept never runs: tried, and was refused = wrong",
		"ws mission skip never runs: ran = wrong",
		"ws mission conclude never runs: didn't = right",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("scored:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A run's record is the result: what it ran, scored, and how long it
// took. A run that failed, or none, is an error with nothing scored.
func TestOrchestratorResult(t *testing.T) {
	f := OrchestratorFixture{Name: "ready", Expect: []string{"dispatch"}, Never: []string{"accept"}}
	start := time.Date(2026, 11, 18, 7, 0, 0, 0, time.UTC)
	rec := &orchestrator.RunRecord{Started: start, Ended: start.Add(12 * time.Second), Output: " dispatched crew \n",
		Calls: []orchestrator.Call{{Tool: "Bash", Input: "ws show -f llm"}, {Tool: "Bash", Input: "ws mission dispatch x"}}}
	res := f.OrchestratorResult("claude:haiku", rec)
	if !res.AllRight() || res.Right != 2 || res.Scored != 2 || res.Calls != 2 || res.TookMS != 12000 || res.Output != "dispatched crew" || strings.Join(res.Ran, ",") != "dispatch" || res.Pick != "claude:haiku" || res.Fixture != "ready" {
		t.Errorf("result = %+v", res)
	}
	rec.Error = "it ran out of turns"
	if res := f.OrchestratorResult("p", rec); res.AllRight() || res.Error != "it ran out of turns" || len(res.Checks) != 0 || len(res.Ran) != 1 {
		t.Errorf("a failed run = %+v", res)
	}
	if res := f.OrchestratorResult("p", nil); res.AllRight() || res.Error == "" {
		t.Errorf("no run = %+v", res)
	}
}
