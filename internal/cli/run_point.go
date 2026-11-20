package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/runs"
)

func newRunPointCmd() *cobra.Command {
	var wait time.Duration
	cmd := &cobra.Command{
		Use:   "point <recipe> [<need>=<capsule|ground|silo|nearest>...]",
		Short: "Point a workspace recipe's needs at a capsule, the ground or the silo",
		Long: `A recipe of the workspace's own (ws.toml's [run.<name>]) runs once for the
whole workspace, so what meets its needs is pointed, not found where it's
asked from: each need of a repo (a run of one of its recipes, or its
folder) is met by the worktree it's pointed at, or by the repo's ground
until it's pointed anywhere.

  ws run point docs                  what docs' needs are pointed at, and could be
  ws run point docs bff=my-capsule   point bff at the capsule my-capsule
  ws run point docs bff=nearest      back to the ground

A pointer is kept, and outlasts the run. Pointing a need of a recipe
that's running starts it again on what it's pointed at now, for whoever
it was for. It's yours to do: crew and agents are refused.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			w, err := loadOps()
			if err != nil {
				return err
			}
			recipe := args[0]
			if len(args) == 1 {
				ptrs, err := w.Pointers(recipe)
				if err != nil {
					return err
				}
				if len(ptrs) == 0 {
					fmt.Fprintf(os.Stderr, "%s has no needs that can be pointed.\n", recipe)
					return nil
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NEED\tREPO\tPOINTED AT\tCOULD BE")
				for _, p := range ptrs {
					at := runs.Label(p.At)
					if p.At == "" {
						at = "ground (nearest)"
					}
					var could []string
					for _, c := range p.Choices {
						could = append(could, runs.Label(c))
					}
					repo := p.Repo
					if p.Folder {
						repo += " (folder)"
					} else {
						repo += "/" + p.Recipe
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Need, repo, at, strings.Join(could, ", "))
				}
				return tw.Flush()
			}
			if os.Getenv(conversation.EnvConversation) != "" {
				return fmt.Errorf("pointing %s is yours to do, not a conversation's agent's: it changes what's shown to whoever has it open", recipe)
			}
			by := ops.ActorFromEnv()
			if by.IsYou() {
				ensureRunsWatched(w.WS().Root)
			}
			for _, a := range args[1:] {
				need, at, ok := strings.Cut(a, "=")
				if !ok || strings.TrimSpace(need) == "" {
					return fmt.Errorf("say <need>=<capsule>, ground, silo or nearest, not %s", a)
				}
				done, err := w.PointRun(recipe, strings.TrimSpace(need), at, by, wait)
				if err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, done)
				if done.Restarted != nil {
					for _, n := range done.Restarted.Notes {
						fmt.Fprintln(os.Stderr, n)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&wait, "wait", time.Minute, "how long to wait for a run started again to be up (0: don't)")
	return cmd
}
