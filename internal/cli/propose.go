package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// Proposals: a conversation's agent changes nothing, but proposes a
// decision or a mission with --propose on the verb that would do it; you
// confirm it under its reply, and ws does it, as you.

// agentConversation is the conversation the agent running this command is
// answering ($WS_CONVERSATION), or "" for anyone else.
func agentConversation() string { return os.Getenv(conversation.EnvConversation) }

// propose records what the agent proposes in the conversation it's
// answering. It's refused for anyone else: the user just does it.
func propose(ctx *ops.Workspace, p conversation.Proposal) error {
	return proposeFrom(ctx, func(conversation.Store, string) (conversation.Proposal, error) { return p, nil })
}

// proposeFrom records what the agent proposes, made from the conversation
// it's in.
func proposeFrom(ctx *ops.Workspace, make func(store conversation.Store, id string) (conversation.Proposal, error)) error {
	id := agentConversation()
	if id == "" {
		return errors.New("--propose is for a conversation's agent, which proposes for the user to confirm: leave it off to do it yourself")
	}
	store, err := ops.ConversationStore(ctx.WS().Root, os.Getenv(mission.EnvMission), id)
	if err != nil {
		return err
	}
	p, err := make(store, id)
	if err != nil {
		return err
	}
	p.By, p.At = conversation.Agent, time.Now()
	c, err := store.Propose(id, p)
	if err != nil {
		return err
	}
	fmt.Fprintf(ui.Stderr, "  %s proposed in %s (proposal %d): the user confirms it, or not, with a button under your reply\n", ui.Green.Render("✓"), id, len(c.Proposals))
	return nil
}

// refuseInConversation stops a conversation's agent doing what it may
// only propose.
func refuseInConversation(what string) error {
	if agentConversation() != "" {
		return fmt.Errorf("you're in a conversation: propose %s with --propose, and the user confirms it", what)
	}
	return nil
}

// ProposalEdits are what you changed in a proposal before accepting it:
// "" keeps the agent's.
type ProposalEdits = ops.ProposalEdits

func newConversationSettleCmd(accept bool) *cobra.Command {
	var slug string
	use, short := "accept <conversation> <proposal>", "Do what the agent proposed in a conversation (the user only)"
	if !accept {
		use, short = "decline <conversation> <proposal>", "Decline what the agent proposed in a conversation (the user only)"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if raisedBy() != "you" || agentConversation() != "" {
				return errors.New("only the user accepts or declines a proposal")
			}
			n, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("proposals are numbered 1, 2, …, not %q", args[1])
			}
			store, err := ops.ConversationStore(ctx.WS().Root, slug, args[0])
			if err != nil {
				return err
			}
			said, err := ctx.SettleProposalIn(store, args[0], n, accept, ProposalEdits{})
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), said)
			return nil
		},
	}
	cmd.Flags().StringVarP(&slug, "mission", "m", "", "the mission whose conversation it is (none: the workspace's)")
	return cmd
}
