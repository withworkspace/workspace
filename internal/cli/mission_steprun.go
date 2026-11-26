package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// The orchestrator's decisions on the plan (ops/steprun.go): starting a
// step that waits for a call, in the repos its words name; skipping one
// that applies only if, and doesn't; starting a Work step again.

func newMissionStartCmd() *cobra.Command {
	var slug string
	var in []string
	cmd := &cobra.Command{
		Use:   "start [--in <repo>]...",
		Short: "Start the step the mission is at, which waits for your call",
		Long: `Start the step the mission is at, where it waits for someone's call: one
that applies only if something holds (- only if: in the plan), or runs in
each of several repos (- in each:). For the orchestrator, by the step's
words and the process, and the user.

A step that runs in several repos runs once in each named with --in: crew
in each repo's capsule (lifted where the mission has none), or a pull
request from each. It's done when every one is.

  ws mission start
  ws mission start --in api --in web`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			var repos []string
			for _, r := range in {
				for one := range strings.SplitSeq(r, ",") {
					if one = strings.TrimSpace(one); one != "" {
						repos = append(repos, one)
					}
				}
			}
			said, err := ctx.StartStep(m.Slug, repos, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), said)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringArrayVar(&in, "in", nil, "a repo it runs in, for a step that runs in several (again for each)")
	return cmd
}

func newMissionSkipCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "skip <why it doesn't apply>",
		Short: "Skip the step the mission is at: its only if doesn't hold",
		Long: `Pass the step the mission is at without doing it, saying why: for the
orchestrator, a step that applies only if something holds (- only if: in
the plan), when it doesn't, before it's begun. A step that always runs
can't be skipped by the orchestrator: if the plan no longer fits, it asks
the planner (ws mission replan). The user may skip any step.

ws records it skipped, with why, and moves on to the next.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			said, err := ctx.Skip(m.Slug, strings.Join(args, " "), ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), said)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

func newMissionRepeatCmd() *cobra.Command {
	var slug, in, on string
	cmd := &cobra.Command{
		Use:   "repeat [--in <repo>] [--on <harness:model>] <why>",
		Short: "Start the Work step the mission is at again, with fresh crew",
		Long: `Start the Work step the mission is at again, with fresh crew: another
attempt, saying why (the crew before got it wrong, or went too far down a
dead end to resume). For a step that runs in several repos, --in starts
its run in one again; without it, every run. --on runs the fresh crew on
another harness and model ("If a fix fails twice, try a stronger one"),
one [crew] allowed lists.

The orchestrator may start a step again within [missions] max_rounds
attempts; after that it's the user's. To have the same crew carry on,
resume them instead (ws mission resume-with).`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			said, err := ctx.Repeat(m.Slug, in, on, strings.Join(args, " "), ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), said)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&in, "in", "", "the repo whose run starts again, for a step that runs in several")
	cmd.Flags().StringVar(&on, "on", "", "what the fresh crew run on, as harness:model or a model alone: one [crew] allowed lists (default: the step's on:, or crew's own)")
	return cmd
}
