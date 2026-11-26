package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Clone all repos from ws.toml",
		Long: `Clones every repo in ws.toml that isn't cloned yet. Repos in hidden groups
are skipped (they clone on first use); pass --group to clone a group.`,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}

			if err := applyGroupFlag(cmd, ctx.WS()); err != nil {
				return err
			}

			if err := ctx.WS().EnsureReposDir(); err != nil {
				return err
			}

			clonedRepos, err := cloneRepos(ctx.WS(), ctx.WS().DefaultRepoNames(), os.Stderr)
			if err != nil {
				return err
			}

			if err := workspace.DisableWorkspaceGit(ctx.WS().Root); err != nil {
				return err
			}
			runAfterCreateHooks(ctx.WS(), clonedRepos, os.Stderr, os.Stderr)

			fmt.Fprintln(ui.Stderr, "\nSetup complete. Run 'ws open' to open in your editor.")
			return nil
		},
	}
	addGroupFlag(cmd)
	return cmd
}
