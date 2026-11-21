package cli

import (
	"fmt"
	"strings"

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
	cmd.AddCommand(newRepoAddCmd())
	cmd.AddCommand(newRepoShareCmd())
	return cmd
}

func newRepoAddCmd() *cobra.Command {
	var as string
	var team bool
	cmd := &cobra.Command{
		Use:   "add <owner/name>",
		Short: "Add a repo from GitHub, for you (ws.local.toml) or the team (--team), and clone it",
		Long: `Adds the repo at owner/name on GitHub to the workspace, by its name there or
--as another (a name here can't be taken twice), and clones it. For you, it's
written into ws.local.toml; with --team, into ws.toml, committed on the
workspace's config repo and pushed (if the remote refuses it, nothing's
added). It's written with github = "owner/name" unless it's the org's repo of
its name here.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, name, ok := strings.Cut(args[0], "/")
			if !ok {
				return fmt.Errorf("say where it is on GitHub, as owner/name: %q isn't", args[0])
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "Adding and cloning %s…\n", args[0])
			done, err := ctx.AddRepo(cmd.Context(), ops.ActorFromEnv(), ops.RepoAdd{Owner: owner, Name: name, As: as, Team: team, Wait: true})
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "%s %s\n", ui.Green.Render("✓"), done)
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "its name here, when it isn't its name on GitHub (or that's taken)")
	cmd.Flags().BoolVar(&team, "team", false, "the team's: into ws.toml, committed and pushed, rather than ws.local.toml")
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
