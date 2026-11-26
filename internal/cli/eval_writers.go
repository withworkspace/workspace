package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/ops"
)

// ws eval team and ws eval pr-writer: the two agents that write for
// people, the Team page's gists and the pull requests ws opens, put to
// fixtures. Neither has one right answer, so ws scores what has one (the
// form, the length, what the fixture says to look for) and prints what
// was written for whoever, or whatever, reads it.

func newEvalTeamCmd() *cobra.Command {
	var e ops.TeamEval
	var asJSON, render bool
	cmd := &cobra.Command{
		Use:   "team <fixture>...",
		Short: "Have the Team page's gists written of fixtures, and score them",
		Long: `ws eval team has the Team page's writer write the gist of each fixture: a
change that landed. It's written as the Team page has one written (the same
prompt, no tools), on the workspace's [team] or the --pick you name, and
scored:

  the reply is in the form ws reads (gist:, then know: lines)
  the gist, and each thing worth knowing, is 30 words at most
  whether something in it is worth knowing, and of what kind, if the
    fixture says

A fixture is a folder with a fixture.toml:

  about = "why this case is kept"
  repo = "demo"
  pr = 12                        # 0, or left out, for commits with none
  title = "Greet people by name"
  body = "The pull request's description."
  subjects = ["..."]             # with no pr: the commits' subjects
  commits = ["<sha>"]            # on the default branch: a pull request's merge
  know = ["contract"]            # what's worth knowing, any of; [] for none
  know_note = "why"

  [git]
  repo = "../../repos/demo/.bare"   # a clone with the commits, or --repo

How well a gist says what the change does isn't scored: --json has what was
written, under output, for whoever compares them. --render prints the
prompt, and runs nothing. One run is one sample.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if !evalOn() {
				return evalOff("team")
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			e.Repo = evalAbs(e.Repo)
			var fixtures []ops.TeamFixture
			for _, a := range args {
				f, err := ops.ReadTeamFixture(evalAbs(a))
				if err != nil {
					return err
				}
				fixtures = append(fixtures, f)
			}
			return runEvals(cmd, len(fixtures), asJSON, render,
				func(i int) (string, error) { return ctx.TeamPrompt(fixtures[i], e) },
				func(i int) ops.EvalResult { return ctx.EvalTeam(fixtures[i], e) })
		},
	}
	cmd.Flags().StringVar(&e.Pick, "pick", "", "who writes, as harness:model or a harness alone (default: the workspace's [team])")
	cmd.Flags().StringVar(&e.Repo, "repo", "", "the clone the fixtures' commits are in, over each fixture's own")
	cmd.Flags().BoolVar(&asJSON, "json", false, "a line of JSON for each fixture")
	cmd.Flags().BoolVar(&render, "render", false, "print the prompt the writer would be given, and run nothing")
	return cmd
}

func newEvalPRWriterCmd() *cobra.Command {
	var e ops.PRWriterEval
	var asJSON, render bool
	cmd := &cobra.Command{
		Use:   "pr-writer <fixture>...",
		Short: "Have the PR writer write fixtures' pull requests, and score them",
		Long: `ws eval pr-writer has the PR writer write the pull request of each fixture:
a capsule's work as it was to be opened. It's written as ws has one
written (the same prompt, read-only in a folder laid out as the capsule),
on the workspace's [pr_writer] or the --pick you name, and scored:

  it answered with a <title>, and a description
  the title is title_max characters at most (72 where it isn't said)
  each of the fixture's checks: a pattern it's to have, or never to

A fixture is a folder with a fixture.toml:

  about = "why this case is kept"
  mission = "greet-by-name"
  title = "Greet people by name"     # the mission's
  goal = "What the mission is for."
  capsule = "demo/greet-by-name"
  task = ["Step 1 (Work): greet takes a name.\n"]   # as the writer's told the plan's steps
  decided = ["d1: Keep Hello, world with no name."]
  guide = "PR.md"                    # a file beside it: the workspace's guide
  title_max = 72

  [[check]]
  text = "it doesn't name the mission"
  never = "greet-by-name|mission"    # or match = "...": a pattern, ignoring case
  in = "body"                        # title, body, or both when left out

beside it handoff*.md (what crew handed off, each as the writer's given
it), and the work, as a judge fixture has it (ws eval judge --help): work
and base folders, or [git] with repo, commit and base.

How well it reads isn't scored: --json has what was written, under output,
for whoever compares them. --render prints the prompt, and runs nothing.
One run is one sample.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if !evalOn() {
				return evalOff("pr-writer")
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			e.Repo = evalAbs(e.Repo)
			var fixtures []ops.PRFixture
			for _, a := range args {
				f, err := ops.ReadPRFixture(evalAbs(a))
				if err != nil {
					return err
				}
				fixtures = append(fixtures, f)
			}
			return runEvals(cmd, len(fixtures), asJSON, render,
				func(i int) (string, error) { return ctx.PRWriterPrompt(fixtures[i], e) },
				func(i int) ops.EvalResult { return ctx.EvalPRWriter(fixtures[i], e) })
		},
	}
	cmd.Flags().StringVar(&e.Pick, "pick", "", "who writes, as harness:model or a harness alone (default: the workspace's [pr_writer])")
	cmd.Flags().StringVar(&e.Repo, "repo", "", "the repo a [git] fixture's commits are in, over the fixture's own")
	cmd.Flags().BoolVar(&e.Keep, "keep", false, "leave the folder each was written in, and say where")
	cmd.Flags().BoolVar(&asJSON, "json", false, "a line of JSON for each fixture")
	cmd.Flags().BoolVar(&render, "render", false, "print the prompt the writer would be given, and run nothing")
	return cmd
}

// evalOff is why an eval of role isn't run: WS_EVAL isn't set.
func evalOff(role string) error {
	return fmt.Errorf("ws eval is off: it starts agents of its own, so it runs only where %s is set (%s=1 ws eval %s ...)", crew.EnvEval, crew.EnvEval, role)
}

// evalAbs is p from where ws was run.
func evalAbs(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(userDirOr(), p)
}

// runEvals runs n fixtures' evals, or prints their prompts, and exits 1
// unless every one came out right.
func runEvals(cmd *cobra.Command, n int, asJSON, render bool, promptOf func(int) (string, error), eval func(int) ops.EvalResult) error {
	if render {
		for i := range n {
			p, err := promptOf(i)
			if err != nil {
				return err
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
	for i := range n {
		res := eval(i)
		ok = ok && res.AllRight()
		if asJSON {
			if err := enc.Encode(res); err != nil {
				return err
			}
			continue
		}
		writeEvalResult(os.Stdout, res)
		if out := strings.TrimSpace(res.Output); out != "" {
			// What it wrote is what's to be read: nothing scores how well.
			fmt.Fprintln(os.Stdout, "  it wrote:")
			for line := range strings.SplitSeq(out, "\n") {
				fmt.Fprintln(os.Stdout, strings.TrimRight("    "+line, " "))
			}
		}
	}
	if !ok {
		cmd.SilenceErrors = true
		return &ExitCodeError{Code: 1}
	}
	return nil
}
