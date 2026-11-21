package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "The workspace's repos: the team's, in ws.toml, and yours, in ws.local.toml",
		Long: `A repo in ws.toml is the team's: everyone with the workspace has it. One in
ws.local.toml alone ([repos.<name>], with github = "owner/name" when it isn't
the org's by that name) is yours: cloned and worked in here as any is, and
not in a teammate's workspace. share gives one of yours to the team.`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	cmd.AddCommand(newRepoShareCmd())
	return cmd
}

func newRepoShareCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "share <repo>",
		Short: "Give a repo of yours to the team: into ws.toml, committed and pushed",
		Long: `Writes the repo into ws.toml as ws.local.toml says it, commits that on the
workspace's config repo and pushes it, then takes it out of ws.local.toml.
ws sync resets the config repo to its remote, so the commit is pushed or
taken back: if the remote refuses it, the repo stays yours.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			done, err := ctx.ShareRepo(cmd.Context(), ops.ActorFromEnv(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "%s %s\n", ui.Green.Render("✓"), done)
			return nil
		},
	}
}
