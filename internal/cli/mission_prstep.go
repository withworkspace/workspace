package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// An Open Pull Request step's pull request, once it's open: marking a
// draft ready, and replying to what's said on it (ops/prstep.go,
// ops/prreplies.go).

func newMissionPRReadyCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "pr-ready [<repo> | <repo>/<capsule>]",
		Short: "Mark the step's draft pull request ready for review",
		Long: `Mark the draft pull request of the Open Pull Request step the mission is
at ready for review, on GitHub. For the orchestrator, when the process
says a draft whose checks pass is marked ready (rather than reviewed by
the user first), and the user. A step that runs in several repos has a pull request in each:
name the repo (or capsule) whose draft it is, where more than one is.

With [missions] review_before_ready on, ws marks it ready only once the
user's latest review of its capsule approves the commit it's at; it's
refused until then. ws passes the step when the pull request merges.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			capsule := ""
			if len(args) == 1 {
				capsule = args[0]
				if repo, name, ok := strings.Cut(capsule, "/"); ok {
					if r, ok := ctx.WS().ResolveAlias(repo); ok {
						capsule = r + "/" + name
					}
				} else if r, ok := ctx.WS().ResolveAlias(capsule); ok {
					capsule = r
				}
			}
			said, err := ctx.MarkReady(m.Slug, capsule, ops.ActorFromEnv())
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

func newMissionReplyCmd() *cobra.Command {
	var slug, thread, to, from string
	cmd := &cobra.Command{
		Use:   "reply <repo>#<number> [--thread <id> | --to <login>] <what to say>",
		Short: "Crew: draft a reply to what someone said on the mission's pull request",
		Long: `Draft a reply to what someone said on one of the mission's pull requests:
in a review thread (--thread, its id, as the mission's log names it, as
in "bob on api#12, ` + "`cache.go:12`" + ` (thread PRRT_kw1): …"), or on the pull
request's own conversation, to someone (--to, their GitHub login). The
text is the rest of the line, or --from - for stdin.

ws posts it on GitHub, as the user. A reply to a person waits for the
user's approval first ([missions] replies_need_me, on unless set off): it's
a task of theirs, and they post it or not. A reply to a bot is posted at
once.

  ws mission reply api#12 --thread PRRT_kw1 "Done: the cache is keyed per tenant now (4e1f2a0)."
  ws mission reply api#12 --to alice "It's covered by TestTotalRounding; see invoice/total_test.go."`,
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
			text := strings.TrimSpace(strings.Join(args[1:], " "))
			if from != "" {
				if text, err = readFlagText(from); err != nil {
					return err
				}
			}
			r, err := ctx.DraftReply(m.Slug, args[0], thread, to, text, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), r)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&thread, "thread", "", "the review thread it answers, by its id")
	cmd.Flags().StringVar(&to, "to", "", "who it's to, by their GitHub login (for a reply on the pull request's own conversation)")
	cmd.Flags().StringVar(&from, "from", "", "the reply, from a file, or - for stdin")
	return cmd
}
