package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws thread: threads on a mission's documents, from the CLI. The user
// opens them in the web UI, on the text they're about; crew read them with
// ws show and answer them here.

func newThreadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Threads on a mission's documents: read and answer them",
		Long: `The user comments on a mission's documents (proposals, ADRs, plans) in
the web UI, on the text they highlight, as in a shared doc. Each comment
is a thread: ask (they want an answer), change (they want the document
changed) or note. ws show t3 shows one: the text it's on, the markdown
lines that text is in, and what's been said.

Answer with ws thread reply. Change the document where it is (and commit,
if it's in a capsule) before saying you have: for a thread the user sent
you, say so with ws thread changed, which hands it back to them. Only the
user resolves a thread.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
	}
	cmd.AddCommand(newThreadListCmd(), newThreadReplyCmd(), newThreadStatusCmd(conversation.Resolved), newThreadStatusCmd(conversation.Open), newThreadAnswerCmd(),
		newThreadSuggestCmd(), newThreadSettleCmd(conversation.SuggestAccepted), newThreadSettleCmd(conversation.SuggestDeclined),
		newThreadSendCmd(), newThreadChangedCmd())
	return cmd
}

func newThreadListCmd() *cobra.Command {
	var slug, doc string
	var open bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a mission's threads",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			all, err := conversation.For(ctx.WS().Root, m.Slug).List()
			if err != nil {
				return err
			}
			n := 0
			for _, c := range all {
				if c.Kind == conversation.Talk || (doc != "" && c.Anchor.Doc != doc) || (open && c.Status != conversation.Open) {
					continue
				}
				n++
				fmt.Printf("%-4s %s  %s\n", c.ID, c.Anchor.Path, threadLine(c))
			}
			if n == 0 {
				fmt.Fprintf(ui.Stderr, "  %s no threads: the user opens them on a document in the web UI\n", ui.Dim.Render("·"))
			}
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&doc, "doc", "", "only the threads on this document (its artifact ref)")
	cmd.Flags().BoolVar(&open, "open", false, "only open threads")
	return cmd
}

func newThreadReplyCmd() *cobra.Command {
	var slug, commit string
	cmd := &cobra.Command{
		Use:   "reply <thread> <text>",
		Short: "Answer in a thread on a document",
		Long: `Say something in a thread: answer a question, say what you changed (after
changing the document, and committing it if it's in a capsule), or say
why you won't. Crew answer as $WS_CREW.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			c, err := conversation.For(ctx.WS().Root, m.Slug).Load(args[0])
			if err != nil {
				return err
			}
			// The version it was said on, so the user sees what changed.
			text, err := ctx.DocNow(m, c.Anchor.Doc)
			if err != nil {
				text = nil
			}
			c, err = ctx.Reply(doc.OfMission(m.Slug), c.ID, text, strings.Join(args[1:], " "), commit, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s said in %s (%s, %s)\n", ui.Green.Render("✓"), c.ID, c.Kind, c.Status)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&commit, "commit", "", "the commit that changed the document for it, if there's one")
	return cmd
}

func newThreadStatusCmd(status string) *cobra.Command {
	var slug string
	use, short := "resolve <thread>", "Resolve a thread: you're done with it (the user only)"
	if status == conversation.Open {
		use, short = "reopen <thread>", "Reopen a resolved thread (the user only)"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			c, err := ctx.SetThreadStatus(doc.OfMission(m.Slug), args[0], status, ops.ActorFromEnv())
			if errors.Is(err, conversation.ErrNotYours) {
				return errors.New("only the user resolves or reopens a thread: say what you did in it with ws thread reply")
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s is %s\n", ui.Green.Render("✓"), c.ID, c.Status)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}
