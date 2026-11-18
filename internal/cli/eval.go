package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/ops"
)

// ws eval: an agent's role put to a fixture, a case whose right answer is
// known, and scored. It's what says whether a change to a prompt, or
// another model in a role, does as well. ws runs one fixture on one pick
// and says how it went; running many across many picks, and keeping what
// they came to, is for what calls it (a loop, a plugin).
//
// It's off unless WS_EVAL is set (crew.EnvEval): not listed, and refused.
// It isn't a command for using a workspace, and an agent that came on it
// in ws --help could start agents of its own with it; no agent ws starts
// is given the variable.

// evalOn says whether ws eval is turned on.
func evalOn() bool { return os.Getenv(crew.EnvEval) != "" }

func newEvalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "eval",
		Hidden: !evalOn(),
		Short:  "Put an agent's role to cases whose answers are known",
		Long: `ws eval puts one of the roles agents play to fixtures: cases whose right
answer is known, as folders. It runs the role as a mission would (the same
prompt, on the harness and model you say), and scores what comes back.

It's for telling whether a change to a prompt, or another model in a role,
does as well as what you have. The judge is the role it knows so far.

It's off unless WS_EVAL is set: WS_EVAL=1 ws eval judge ...`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newEvalJudgeCmd())
	return cmd
}

func newEvalJudgeCmd() *cobra.Command {
	var e ops.JudgeEval
	var asJSON, render bool
	cmd := &cobra.Command{
		Use:   "judge <fixture>...",
		Short: "Have the judge judge fixtures, and score what it says",
		Long: `ws eval judge has the judge judge each fixture: a step's work as its crew
left it, and what each of its checks should come to. The judge is run as a
mission's is (the same prompt, headless, in a folder laid out as a capsule
is), on the workspace's judge or the --pick you name, and each check it
judged is scored against what the fixture expects:

  right       it said what was expected
  wrong       met for unmet, or unmet for met
  abstained   can't tell, of something it could have told
  guessed     met or unmet, of something it couldn't have told

A fixture is a folder with a fixture.toml:

  about = "why this case is kept"
  step = "2: Work: greet by name."
  goal = "Greet people by name."

  [[check]]
  text = "greet prints the name it's given"
  expect = "met"            # met, unmet or can't tell; none: not scored

beside it handoff.md (what the crew handed off) and the work: a work
folder (the capsule's files as they were left) and, if it began from
something, a base folder; or, for work that's commits of a repo here,

  [git]
  repo = "../../repos/demo/.bare"   # or --repo
  commit = "<the work>"
  base = "<where it left its default branch>"

There's no mission behind a fixture, so a check that turns on one of the
mission's documents (ws show <ref>) can't be shown, and comes out can't
tell. And one run is one sample: an agent doesn't say the same thing twice,
so run a fixture several times before reading much into one answer.

It exits 1 unless every scored check came out right. --json prints a line
of JSON for each fixture instead; --render prints the prompt the judge
would be given, and runs nothing.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// What went wrong is said in what's printed: no usage after it.
			cmd.SilenceUsage = true
			if !evalOn() {
				return fmt.Errorf("ws eval is off: it starts agents of its own, so it runs only where %s is set (%s=1 ws eval judge ...)", crew.EnvEval, crew.EnvEval)
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			from := userDirOr()
			abs := func(p string) string {
				if p == "" || filepath.IsAbs(p) {
					return p
				}
				return filepath.Join(from, p)
			}
			e.Repo = abs(e.Repo)
			var fixtures []ops.JudgeFixture
			for _, a := range args {
				f, err := ops.ReadJudgeFixture(abs(a))
				if err != nil {
					return err
				}
				fixtures = append(fixtures, f)
			}
			if render {
				for i, f := range fixtures {
					p, err := ctx.JudgePrompt(f, e)
					if err != nil {
						return fmt.Errorf("%s: %w", f.Name, err)
					}
					if i > 0 {
						fmt.Fprintln(os.Stdout)
					}
					fmt.Fprintln(os.Stdout, strings.TrimRight(p, "\n"))
				}
				return nil
			}
			ok := true
			enc := json.NewEncoder(os.Stdout)
			for _, f := range fixtures {
				res := ctx.EvalJudge(f, e)
				ok = ok && res.AllRight()
				if asJSON {
					if err := enc.Encode(res); err != nil {
						return err
					}
					continue
				}
				writeJudgeResult(os.Stdout, res)
			}
			if !ok {
				cmd.SilenceErrors = true
				return &ExitCodeError{Code: 1}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&e.Pick, "pick", "", "who judges, as harness:model or a harness alone (default: the workspace's judge)")
	cmd.Flags().StringVar(&e.Repo, "repo", "", "the repo a [git] fixture's commits are in, over the fixture's own")
	cmd.Flags().BoolVar(&e.Keep, "keep", false, "leave the folder each was judged in, and say where")
	cmd.Flags().BoolVar(&asJSON, "json", false, "a line of JSON for each fixture")
	cmd.Flags().BoolVar(&render, "render", false, "print the prompt the judge would be given, and run nothing")
	return cmd
}

// writeJudgeResult is a fixture's eval as you read it: a line for the
// fixture, then one for each check, with what the judge said of any that
// didn't come out right.
func writeJudgeResult(w io.Writer, r ops.JudgeResult) {
	took := (time.Duration(r.TookMS) * time.Millisecond).Round(time.Second)
	if r.Error != "" {
		fmt.Fprintf(w, "%s on %s: not judged: %s\n", r.Fixture, r.Pick, r.Error)
		return
	}
	fmt.Fprintf(w, "%s on %s: %d of %d right, in %s\n", r.Fixture, r.Pick, r.Right, r.Scored, took)
	for _, c := range r.Checks {
		switch c.Score {
		case ops.ScoreNone:
			fmt.Fprintf(w, "  %d. %s (not scored): %s\n", c.N, c.Got, c.Check)
		case ops.ScoreRight:
			fmt.Fprintf(w, "  %d. right, %s: %s\n", c.N, c.Got, c.Check)
		default:
			fmt.Fprintf(w, "  %d. %s, %s for %s: %s\n", c.N, c.Score, c.Got, c.Expect, c.Check)
			if c.Why != "" {
				fmt.Fprintf(w, "       it said: %s\n", c.Why)
			}
			if c.Note != "" {
				fmt.Fprintf(w, "       expected because: %s\n", c.Note)
			}
		}
	}
	if r.Dir != "" {
		fmt.Fprintf(w, "  judged in %s\n", r.Dir)
	}
}
