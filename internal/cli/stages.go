package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

func newMissionAcceptCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "accept [why]",
		Short: "Accept the mission's current stage, or its plan's step: its work is complete and good enough",
		Long: `Accept the current stage of the brief's plan (the first not yet accepted),
or of a mission with a plan, its current step: its crew's work is complete
and good enough to build on. The next becomes the current one, and new
crew work on it.

For a mission with a plan, ws reaches the next step by who's on it: it
starts crew on a step that's crew's; asks you at a step that's yours (a
checkpoint: only you pass it, by going on or accepting it yourself), and
at the planner's. At a Wait it asks you, when it's on you, or waits: one
with checks is watched, and ws passes it once they're met; one without
is passed only by someone accepting it, once it's happened. What you say
as you accept a Wait (the version you tagged) is kept as a decision, for
the crew after.

ws then moves the plan on: it starts the next stage in the capsule the
stage names (or the mission's only one), with the stage's text, unless the
accepted stage says to check in, when it asks the user first.

For the orchestrator, and you. It's refused while the stage's crew are
still working or asking you something, while a review of one of its
capsules asks for changes (the user re-reviews, and approves, first), and
while a change the user asked for on one of its documents is open (they
resolve the thread first). A brief without stages is one stage, the whole
brief.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			acc, err := ctx.AcceptStage(m.Slug, ops.ActorFromEnv(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			if acc.Implicit {
				fmt.Fprintf(ui.Stderr, "  %s accepted the work on %s\n", ui.Green.Render("✓"), m.Slug)
				return nil
			}
			fmt.Fprintf(ui.Stderr, "  %s accepted %s %d of %d: %s\n", ui.Green.Render("✓"), acc.Stage.Noun(), acc.Stage.N, acc.Of, acc.Stage.Title())
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Dim.Render("·"), acc.Next)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

// oneLine joins a stage's lines.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func printWarnings(warnings []string) {
	for _, w := range warnings {
		fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Orange.Render("!"), w)
	}
}
