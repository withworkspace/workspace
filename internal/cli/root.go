package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd is ws: its own commands, and those the workspace's plugins
// add.
func NewRootCmd(version string) *cobra.Command {
	cmd := newBuiltinRootCmd(version)
	addPluginCommands(cmd)
	return cmd
}

// newBuiltinRootCmd is ws with its own commands alone.
func newBuiltinRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Multi-repo workspace manager",
		Long:  "Manage multi-repo workspaces with git worktrees and symlinks.",
	}

	cmd.Version = version

	// Default command: show status when no subcommand given
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newStatusCmd().RunE(cmd, args)
	}
	addGroupFlag(cmd)

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newSetupCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newLiftCmd())
	cmd.AddCommand(newDockCmd())
	cmd.AddCommand(newBurnCmd())
	cmd.AddCommand(newLandCmd())
	cmd.AddCommand(newOpenCmd())
	cmd.AddCommand(newDebriefCmd())
	cmd.AddCommand(newPromptCmd())
	cmd.AddCommand(newUpgradeCmd())
	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newJumpCmd())
	cmd.AddCommand(newRootPathCmd())
	cmd.AddCommand(newShellInitCmd())
	cmd.AddCommand(newBoardCmd())
	cmd.AddCommand(newUnboardCmd())
	cmd.AddCommand(newSiloCmd())
	cmd.AddCommand(newRgCmd())
	cmd.AddCommand(newGroupCmd())
	cmd.AddCommand(newMissionCmd())
	cmd.AddCommand(newDeckCmd(), newDaemonCmd())
	cmd.AddCommand(newWebCmd())
	cmd.AddCommand(newReviewCmd(), newNavCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newDocumentsCmd(), newDocCmd(), newFindCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newThreadCmd())
	cmd.AddCommand(newTalkCmd(), newConversationCmd(), newPageCmd(), newArtifactCmd())
	cmd.AddCommand(newPRCmd())
	cmd.AddCommand(newTeamCmd())
	cmd.AddCommand(newProcessCmd())
	cmd.AddCommand(newPluginCmd())

	return cmd
}
