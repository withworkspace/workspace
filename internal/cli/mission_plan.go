package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

func newMissionPlanCmd() *cobra.Command {
	var slug, set string
	var proposeIt, format bool
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Print a mission's plan, or replace it with --set",
		Long: `Print the mission's plan (plan.md beside its brief), or replace it:

  ws mission plan --set - <<'EOF'
  ## Z
  - …
  ## Stops early
  - …
  ## Steps
  1. …
     - who: crew, in ` + "`repo/name`" + `
  EOF

The plan is the mission's route from A to Z: what Z is, where it may stop
early, and its steps in order, each with who's on it. ws reads it strictly
and refuses one that isn't right, with every problem and its line
(--format says how it's written). A step's quote has to be the words of
the workspace's process (the one last confirmed).

A mission with a plan works from its steps, in place of its brief's
stages: ws starts crew on a step that's crew's, asks you at a step that's
yours (a checkpoint, which only you pass), and the orchestrator is given
the plan.

The plan is yours, and the planner's: briefing's agent writes it while it
briefs the mission. In a conversation, the agent proposes it instead
(--propose --set -); briefing, it comes with the brief it proposed, and
confirming that makes the mission ready with both.

Once the mission's briefed, the planner comes back to it in the same
conversation (at a step of its own, when the orchestrator says the route
doesn't fit, or when you ask: ws mission replan), and what it proposes
there is a revision: one that removes or moves a checkpoint, or changes Z,
is proposed to you; anything else applies, and the log says what changed.
Written at its own step, it passes that step. A revision, yours too,
changes only what's ahead: the steps passed, and the one the mission is
at once begun, stay as they were.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format {
				_, err := os.Stdout.WriteString(mission.PlanFormat)
				return err
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			store := missionStore(ctx)
			if set == "" {
				if proposeIt {
					return errors.New("--propose proposes the plan given with --set")
				}
				plan, err := store.ReadPlan(m.Slug)
				if err != nil {
					return err
				}
				if strings.TrimSpace(plan) == "" {
					fmt.Fprintf(ui.Stderr, "  %s %s has no plan: it works from its brief (ws mission plan --set - writes one)\n", ui.Dim.Render("·"), m.Slug)
					return nil
				}
				_, err = os.Stdout.WriteString(plan)
				return err
			}
			var data []byte
			if set == "-" {
				data, err = io.ReadAll(os.Stdin)
			} else {
				data, err = os.ReadFile(set)
			}
			if err != nil {
				return fmt.Errorf("reading the plan: %w", err)
			}
			if id := agentConversation(); proposeIt && id != "" && ctx.IsPlannerTalk(m.Slug, id) {
				// The planner, back at the plan once it's briefed: ws
				// applies its revision, or proposes it to the user.
				r, err := ctx.RevisePlan(m.Slug, id, string(data))
				if err != nil {
					return err
				}
				mark := ui.Green.Render("✓")
				if r.Proposed > 0 {
					mark = ui.Dim.Render("·")
				}
				fmt.Fprintf(ui.Stderr, "  %s %s\n", mark, r)
				printWarnings(r.Warnings)
				return nil
			}
			if proposeIt {
				return proposeFrom(ctx, func(cs conversation.Store, id string) (conversation.Proposal, error) {
					p, err := ctx.PlanProposal(cs, id, string(data))
					if err == nil && p.Kind == conversation.ProposeBrief {
						fmt.Fprintf(ui.Stderr, "  %s it comes with the brief you proposed: one proposal of both, which the user confirms to make the mission ready\n", ui.Dim.Render("·"))
					}
					return p, err
				})
			}
			if err := refuseInConversation("the plan"); err != nil {
				return err
			}
			set, err := ctx.SetPlan(m.Slug, string(data), ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s plan for %s written: %d %s\n", ui.Green.Render("✓"), m.Slug, len(set.Plan.Steps), plural(len(set.Plan.Steps), "step"))
			printWarnings(set.Warnings)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&set, "set", "", "replace the plan from a file, or - for stdin")
	cmd.Flags().BoolVar(&proposeIt, "propose", false, "in a conversation: propose the plan (--set), for the user to accept, instead of writing it")
	cmd.Flags().BoolVar(&format, "format", false, "say how a plan is written, and what ws holds it to")
	return cmd
}
