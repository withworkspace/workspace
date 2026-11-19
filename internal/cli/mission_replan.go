package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

func newMissionReplanCmd() *cobra.Command {
	var slug string
	var proposeIt bool
	cmd := &cobra.Command{
		Use:   `replan "<what doesn't fit, or what to change>"`,
		Short: "Ask the mission's planner to revise its plan",
		Long: `Ask the mission's planner to revise its plan: the orchestrator, when the
route doesn't fit what's happened; you, for a change you want.

The planner comes back in the conversation the mission was briefed in (or
one opened for it), reads where the mission stands and what its steps
made, and writes the plan again. The steps behind the mission stay as they
were. A revision that removes or moves a checkpoint, or changes Z, comes to
you there as a proposal; anything else applies, and the log says what
changed.

The orchestrator may ask once at a step: if the plan still doesn't fit, it
flags you. Crew say what doesn't fit in their handoff. A conversation's
agent proposes it (--propose), for you to confirm: confirming asks the
planner as this does, as you; declining changes nothing.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			what := strings.TrimSpace(args[0])
			if proposeIt {
				// Refused now if the planner can't be asked, rather than
				// when the user confirms it.
				if err := ctx.MayAskPlanner(m.Slug); err != nil {
					return err
				}
				return propose(ctx, conversation.Proposal{Kind: conversation.ProposeReplan, Text: what})
			}
			if err := refuseInConversation("a replan"); err != nil {
				return err
			}
			a, err := ctx.AskPlanner(m.Slug, what, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s asked the planner of %s, in %s: it revises the plan, or says why not (ws show -f llm %s)\n", ui.Green.Render("✓"), m.Slug, a.Conversation, a.Conversation)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().BoolVar(&proposeIt, "propose", false, "in a conversation: propose it, for the user to confirm, instead of asking the planner")
	revisesFlag(cmd)
	return cmd
}
