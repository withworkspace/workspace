package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws propose jira:issues: a conversation's agent proposing something of a
// plugin's. What it writes is the plugin's own shape, which ws doesn't
// read: the plugin says how one's written (asked with nothing), drafts
// what it'll do from what's written, or says what's wrong with it. What
// it drafted is the card under the agent's reply.

func newProposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose <plugin>:<what> [-]",
		Short: "Propose something of a plugin's, for the user to accept (a conversation's agent only)",
		Long: `Propose something of a plugin's (issues to create), for the user to
accept under your reply. With nothing more, it proposes nothing, but
prints how one's written: the plugin's guide, as things are now. Then
write it as that says and give it on standard input (-).

  ws propose jira:issues
  ws propose jira:issues - <<'EOF'
  [{"summary": "Cache tokens", "project": "RAM"}]
  EOF

What's written goes to the plugin, which drafts what it'll do (that's
what the user's shown) or says what's wrong with it, to write it again.
--revises <n> proposes it in place of an open proposal of yours.`,
		Args:              cobra.RangeArgs(1, 2),
		PreRun:            func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			id := agentConversation()
			if id == "" {
				return errors.New("ws propose is for a conversation's agent, which proposes for the user to accept: do it in the plugin itself")
			}
			name := args[0]
			if _, _, ok := plugin.SplitProposable(name); !ok {
				return fmt.Errorf("%q isn't one of what a plugin proposes: it's the plugin's name, a colon, and what. Here: %s", name, proposableNames(ctx.WS().Root))
			}
			var input []byte
			if len(args) == 2 {
				if args[1] != "-" {
					return errors.New("give what you wrote on standard input (-), with a heredoc")
				}
				if input, err = io.ReadAll(os.Stdin); err != nil {
					return fmt.Errorf("reading what you wrote: %w", err)
				}
				if strings.TrimSpace(string(input)) == "" {
					return errors.New("nothing was written on standard input: write it as the guide says (ws propose " + name + " prints it)")
				}
			}
			slug := os.Getenv(mission.EnvMission)
			guide, refused, p, err := ops.DraftPlugin(ctx.WS().Root, slug, id, name, input, time.Now())
			if errors.Is(err, plugin.ErrNoProposes) {
				return fmt.Errorf("%w. What the plugins here propose: %s", err, proposableNames(ctx.WS().Root))
			}
			switch {
			case err != nil:
				return err
			case guide != "":
				fmt.Println(guide)
				fmt.Fprintf(ui.Stderr, "\n  %s nothing's proposed yet: write it as that says, then run ws propose %s - <<'EOF', it, then EOF on a line of its own\n", ui.Dim.Render("·"), name)
				return nil
			case refused != "":
				return fmt.Errorf("not proposed: %s", refused)
			}
			if err := propose(ctx, p); err != nil {
				return err
			}
			for i, it := range p.Items {
				fmt.Fprintf(ui.Stderr, "  %s %d. %s\n", ui.Dim.Render("·"), i+1, it.Title)
			}
			return nil
		},
	}
	revisesFlag(cmd)
	return cmd
}

// proposableNames are what the workspace's plugins propose, as an agent
// names them, in a line.
func proposableNames(root string) string {
	var names []string
	for _, p := range plugin.Proposables(root) {
		names = append(names, p.Name())
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}
