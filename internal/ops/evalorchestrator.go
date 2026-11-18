package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/withworkspace/workspace/internal/orchestrator"
)

// Evaluating the orchestrator: a fixture is a mission at a moment (a
// script that makes it, in a workspace made for the run) and what the
// orchestrator should do about it: the ws mission commands it's to run,
// and those it's never to. It's run for real, in that workspace, and
// what it ran is read off its run's record. Making the workspace and
// removing it isn't ws's: whoever runs the eval says how (ws eval
// orchestrator --before, --after).

// OrchestratorFixtureName is the file that makes a folder an orchestrator
// fixture.
const OrchestratorFixtureName = "fixture.toml"

// OrchestratorFixture is a folder's fixture.toml, with the seed beside it.
type OrchestratorFixture struct {
	// Name is what the fixture's called (its folder's name without one),
	// and About why it's kept: what it catches.
	Name  string `toml:"name"`
	About string `toml:"about"`
	// Seed is the script that makes the mission, from the fixture's
	// folder (seed.sh without one): run by bash in the workspace, as a
	// try's seed is (scripts/try.sh).
	Seed string `toml:"seed"`
	// Mission is the slug of the mission the seed makes.
	Mission string `toml:"mission"`
	// Trigger is what the run is for (manual without one), and Crew whose
	// event it is, if it's one's: as ws mission orchestrate takes them.
	Trigger string `toml:"trigger"`
	Crew    string `toml:"crew"`
	// Expect are the ws mission commands it's to run, by their word
	// (dispatch, flag): each at least once, and not refused. Never are
	// those it isn't to try at all.
	Expect []string `toml:"expect"`
	Never  []string `toml:"never"`

	dir string
}

var missionVerb = regexp.MustCompile(`^[a-z][a-z-]*$`)

// ReadOrchestratorFixture reads the fixture in dir.
func ReadOrchestratorFixture(dir string) (OrchestratorFixture, error) {
	var f OrchestratorFixture
	path := filepath.Join(dir, OrchestratorFixtureName)
	if _, err := toml.DecodeFile(path, &f); err != nil {
		if os.IsNotExist(err) {
			return f, fmt.Errorf("%s has no %s: it isn't an orchestrator fixture", dir, OrchestratorFixtureName)
		}
		return f, fmt.Errorf("reading %s: %w", path, err)
	}
	f.dir = dir
	if f.Name == "" {
		if abs, err := filepath.Abs(dir); err == nil {
			f.Name = filepath.Base(abs)
		}
	}
	if f.Seed == "" {
		f.Seed = "seed.sh"
	}
	if f.Trigger == "" {
		f.Trigger = "manual"
	}
	switch {
	case f.Mission == "":
		return f, fmt.Errorf("%s: no mission: the slug of the mission its seed makes", path)
	case len(f.Expect)+len(f.Never) == 0:
		return f, fmt.Errorf("%s: neither expect nor never: there's nothing to score", path)
	}
	if info, err := os.Stat(f.SeedPath()); err != nil || info.IsDir() {
		return f, fmt.Errorf("%s: no seed at %s: the script that makes the mission", path, f.SeedPath())
	}
	seen := map[string]string{}
	for list, verbs := range map[string][]string{"expect": f.Expect, "never": f.Never} {
		for _, v := range verbs {
			if !missionVerb.MatchString(v) {
				return f, fmt.Errorf("%s: %s has %q: it's the word after ws mission (dispatch, flag)", path, list, v)
			}
			if other, ok := seen[v]; ok && other != list {
				return f, fmt.Errorf("%s: %s is both expected and never to be run", path, v)
			}
			seen[v] = list
		}
	}
	return f, nil
}

// SeedPath is where the fixture's seed is.
func (f OrchestratorFixture) SeedPath() string {
	if filepath.IsAbs(f.Seed) {
		return f.Seed
	}
	return filepath.Join(f.dir, f.Seed)
}

// OrchestratorCommand is a ws mission command the orchestrator ran: its
// word, and whether it went through. Refused: its harness wouldn't run
// it; Failed: it ran, and ws said no.
type OrchestratorCommand struct {
	Verb    string
	Refused bool
	Failed  bool
}

// String is the command in a word or two: "dispatch", "dispatch
// (refused)".
func (c OrchestratorCommand) String() string {
	switch {
	case c.Refused:
		return c.Verb + " (refused)"
	case c.Failed:
		return c.Verb + " (failed)"
	}
	return c.Verb
}

// missionCommand finds ws mission commands in what a shell was given: at
// its start, or after something that begins another command.
var missionCommand = regexp.MustCompile(`(?:^|[\s;&|(])(?:ws|workspace)\s+mission\s+([a-z][a-z-]*)`)

// OrchestratorCommands are the ws mission commands among a run's calls,
// in order: what it did, and what it tried to.
func OrchestratorCommands(calls []orchestrator.Call) []OrchestratorCommand {
	var out []OrchestratorCommand
	for _, c := range calls {
		for _, m := range missionCommand.FindAllStringSubmatch(c.Input, -1) {
			out = append(out, OrchestratorCommand{Verb: m[1], Refused: c.Refused, Failed: c.Error && !c.Refused})
		}
	}
	return out
}

// What a command an eval looked for came to.
const (
	OrchestratorRuns   = "runs"
	OrchestratorNever  = "never runs"
	gotRan             = "ran"
	gotRefused         = "tried, and was refused"
	gotFailed          = "tried, and it failed"
	gotDidnt           = "didn't"
	orchestratorPrefix = "ws mission "
)

// Score is how what the orchestrator ran compares with what f expects: a
// check for each command it's to run (right if it ran, and wasn't
// refused), then one for each it's never to (right if it didn't try).
func (f OrchestratorFixture) Score(ran []OrchestratorCommand) []EvalCheck {
	got := func(verb string) string {
		out := gotDidnt
		for _, c := range ran {
			if c.Verb != verb {
				continue
			}
			switch {
			case !c.Refused && !c.Failed:
				return gotRan
			case c.Refused && out == gotDidnt:
				out = gotRefused
			case c.Failed:
				out = gotFailed
			}
		}
		return out
	}
	var checks []EvalCheck
	add := func(verb, expect string, right bool, g string) {
		c := EvalCheck{N: len(checks) + 1, Check: orchestratorPrefix + verb, Expect: expect, Got: g, Score: ScoreWrong}
		if right {
			c.Score = ScoreRight
		}
		checks = append(checks, c)
	}
	for _, v := range f.Expect {
		g := got(v)
		add(v, OrchestratorRuns, g == gotRan, g)
	}
	for _, v := range f.Never {
		g := got(v)
		add(v, OrchestratorNever, g == gotDidnt, g)
	}
	return checks
}

// OrchestratorResult is f's eval, from the run's record: what it ran,
// scored. A run that failed is the result's Error, with what it ran
// before it did.
func (f OrchestratorFixture) OrchestratorResult(pick string, rec *orchestrator.RunRecord) EvalResult {
	res := EvalResult{Fixture: f.Name, Pick: pick}
	if rec == nil {
		res.Error = "the orchestrator left no record of a run"
		return res
	}
	ran := OrchestratorCommands(rec.Calls)
	for _, c := range ran {
		res.Ran = append(res.Ran, c.String())
	}
	res.Calls, res.Output, res.Error = len(rec.Calls), strings.TrimSpace(rec.Output), rec.Error
	res.TookMS = rec.Ended.Sub(rec.Started).Milliseconds()
	if res.Error != "" {
		return res
	}
	res.Checks = f.Score(ran)
	for _, c := range res.Checks {
		res.Scored++
		if c.Score == ScoreRight {
			res.Right++
		}
	}
	return res
}
