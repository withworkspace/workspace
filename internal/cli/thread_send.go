package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ui"
)

// Bigger changes: "rewrite the rollout section" is work, not an edit. You
// send a thread (or every Change thread open on a document) to crew: the
// crew that wrote the document, told in its terminal if it's running or
// resumed if it's ended, or else fresh crew dispatched into the
// document's capsule. Crew answer in the threads and say when they've
// changed the document (ws thread changed), which hands each back to you.

func newThreadSendCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "send <thread>...",
		Short: "Send threads to crew to change the document for (the user only)",
		Long: `Hand threads on a document to crew, for changes bigger than an edit: the
crew that wrote the document if it can be reached (typed into if it's
running, resumed if it's ended), else fresh crew dispatched into the
document's capsule. They're crew's until crew say they've changed the
document for them (ws thread changed), or finish.`,
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
			if raisedBy() != "you" {
				return errors.New("only the user sends threads to crew")
			}
			said, err := ctx.SendToCrew(m.Slug, "", args)
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

func newThreadChangedCmd() *cobra.Command {
	var slug, commit string
	cmd := &cobra.Command{
		Use:   "changed <thread> <what changed>",
		Short: "Say you've changed the document for a thread: it's the user's to look at",
		Long: `Say what you changed in the document for a thread you were sent, once
it's changed (and committed, if it's in a capsule: give the commit with
--commit). That hands the thread back to the user, to look at and
resolve.`,
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
			store := conversation.For(ctx.WS().Root, m.Slug)
			c, err := store.Load(args[0])
			if err != nil {
				return err
			}
			t := conversation.Turn{By: raisedBy(), At: time.Now(), Body: strings.Join(args[1:], " "), Commit: commit, Changed: true}
			if text, err := ctx.DocNow(m, c.Anchor.Doc); err == nil {
				t.Version, _ = store.KeepVersion(text)
			}
			if _, err := store.Say(c.ID, t); err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s said in %s: it's the user's to look at and resolve\n", ui.Green.Render("✓"), c.ID)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&commit, "commit", "", "the commit that changed the document for it")
	return cmd
}
