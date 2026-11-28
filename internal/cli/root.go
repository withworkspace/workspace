package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/harness"
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
		Long: `Manage multi-repo workspaces with git worktrees and symlinks.

ws alone opens the web UI, as ws web does; ws status lists every repo and capsule.`,
	}

	cmd.Version = version

	// ws alone opens the web UI, as ws web does.
	var printOnly bool
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return openWeb("/", printOnly)
	}
	addPrintFlag(cmd, &printOnly)
	addStdinFlag(cmd)

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
	cmd.AddCommand(newSyncCmd(), newUpgradeCmd())
	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newJumpCmd())
	cmd.AddCommand(newRootPathCmd())
	cmd.AddCommand(newShellInitCmd())
	cmd.AddCommand(newBoardCmd())
	cmd.AddCommand(newUnboardCmd())
	cmd.AddCommand(newSiloCmd())
	cmd.AddCommand(newRunCmd(), newRouterCmd())
	cmd.AddCommand(newRgCmd())
	cmd.AddCommand(newGroupCmd())
	cmd.AddCommand(newRepoCmd())
	cmd.AddCommand(newMissionCmd())
	cmd.AddCommand(newDeckCmd(), newDaemonCmd())
	cmd.AddCommand(newWebCmd())
	cmd.AddCommand(newReviewCmd(), newNavCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newPinCmd(), newUnpinCmd())
	cmd.AddCommand(newDocumentsCmd(), newDocCmd(), newFindCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newThreadCmd())
	cmd.AddCommand(newTalkCmd(), newConversationCmd(), newPageCmd(), newArtifactCmd(), newProposeCmd())
	cmd.AddCommand(newPRCmd())
	cmd.AddCommand(newTeamCmd())
	cmd.AddCommand(newProcessCmd())
	cmd.AddCommand(newPluginCmd())
	cmd.AddCommand(newEvalCmd())
	cmd.AddCommand(newEventsCmd(), newNotificationsCmd(), newAutomationCmd(), newLessonsCmd())
	cmd.AddCommand(newConfigCmd())

	return cmd
}

// stdinFrom is the file a command reads its standard input from, when
// it's given one (--stdin-from).
var stdinFrom string

func init() { cobra.OnInitialize(useStdinFrom) }

// addStdinFlag gives every command --stdin-from: its standard input from a
// file, removed once it's open if ws set it aside for this. It's how a conversation's agent
// in a sandbox gives a command what it wrote for it: a command with a
// heredoc stays inside the sandbox, where ws can't write, so ws's guard
// moves the heredoc's body to a file of ws's own and names it here
// (harness.GuardAnswer). Nobody types it.
func addStdinFlag(cmd *cobra.Command) {
	name := strings.TrimPrefix(harness.StdinFlag, "--")
	cmd.PersistentFlags().StringVar(&stdinFrom, name, "", "read standard input from this file (ws's own: what a conversation's agent gave the command)")
	cmd.PersistentFlags().MarkHidden(name) //nolint:errcheck // it's there
}

// useStdinFrom has the command read its standard input from the file
// --stdin-from names, if it names one. One that can't be opened is left
// to the command, which reads nothing and says what it's missing.
func useStdinFrom() {
	if stdinFrom == "" {
		return
	}
	path := stdinFrom
	stdinFrom = ""
	f, err := os.Open(path)
	if err != nil {
		if null, nerr := os.Open(os.DevNull); nerr == nil {
			os.Stdin = null
		}
		return
	}
	if harness.SetAside(path) {
		os.Remove(path) //nolint:errcheck // open, it's read all the same
	}
	os.Stdin = f
}
