package cli

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/release"
	"github.com/withworkspace/workspace/internal/ui"
)

// ReleaseBuild is whether this ws is a release's build: stamped with its
// tag as it was built (.github/workflows/release.yml), which main says.
// It's the only kind ws upgrade replaces: one built from a clone, or by
// go install, is changed where it was built.
var ReleaseBuild bool

// thisWS is the ws that's running, which says it's the version given,
// upgraded from where ws is released.
func thisWS(version string) release.Upgrader {
	return release.Upgrader{
		Exe: ownBinary(), Version: version, Release: ReleaseBuild,
		Source: release.FromEnv(), OS: runtime.GOOS, Arch: runtime.GOARCH,
	}
}

func newUpgradeCmd() *cobra.Command {
	var check, rollback bool
	cmd := &cobra.Command{
		Use:   "upgrade [version]",
		Short: "Upgrade ws to the latest release",
		Long: `Installs the latest release of ws in place of the one that's running: its
build for this machine, checked against the release's checksums and run
once before it's put in place. A version (v1.4.0) installs that release,
an older one or a prerelease among them.

The ws it replaces is kept, and --rollback puts it back. A workspace's
records are moved on by the first newer ws that opens them, and an older
ws won't open them after: .ws/backup has the day's copy.

A workspace's daemon keeps the ws it started from. It finds the new one in
its place and starts again on it once nothing's at work (no crew mid-turn,
no agent's run); ws daemon restart --now doesn't wait.

Only a release's build is upgraded: a ws built from a clone, or by go
install, is changed where it was built. (Pulling the workspace's config,
which ws upgrade used to do, is ws sync.)`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if err := yoursToDo("upgrading ws", "it changes the ws every crew and the daemon run"); err != nil {
				return err
			}
			u := thisWS(cmd.Root().Version)
			if rollback {
				if len(args) > 0 || check {
					return errors.New("--rollback puts back the ws the last upgrade replaced: it takes no version, and not --check")
				}
				if !u.Replaceable() {
					return u.ErrNotARelease()
				}
				version, err := release.Rollback(u.Exe)
				if err != nil {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s ws %s is back at %s (it was %s)\n", ui.Green.Render("✓"), version, u.Exe, u.Version)
				fmt.Fprintf(ui.Stderr, "    %s if %s moved a workspace's records on, %s won't open them: .ws/backup has the day's copy\n", ui.Dim.Render("·"), u.Version, version)
				sayDaemonAfterInstall(u.Exe, version)
				return nil
			}
			version := ""
			if len(args) > 0 {
				version = args[0]
			}
			r, err := u.Look(cmd.Context(), version)
			if err != nil {
				return err
			}
			if !r.Current && !r.Older && !check {
				fmt.Fprintf(ui.Stderr, "  %s downloading ws %s for %s/%s\n", ui.Dim.Render("↓"), r.To, u.OS, u.Arch)
				if r, err = u.Install(cmd.Context(), r); err != nil {
					return err
				}
			}
			switch {
			case r.Installed:
				kept := ""
				if r.Kept {
					kept = fmt.Sprintf(" (%s is kept: ws upgrade --rollback puts it back)", r.From)
				}
				fmt.Fprintf(ui.Stderr, "  %s ws %s installed at %s%s\n", ui.Green.Render("✓"), r.To, u.Exe, kept)
				sayDaemonAfterInstall(u.Exe, r.To)
			case r.Current && version == "":
				fmt.Fprintf(ui.Stderr, "  %s ws %s is the latest release\n", ui.Green.Render("✓"), r.To)
			case r.Current:
				fmt.Fprintf(ui.Stderr, "  %s this ws is %s already\n", ui.Green.Render("✓"), r.To)
			case r.Older:
				fmt.Fprintf(ui.Stderr, "  %s this ws (%s) is later than the latest release (%s): ws upgrade %s installs that one\n", ui.Dim.Render("·"), r.From, r.To, r.To)
			default:
				fmt.Fprintf(ui.Stderr, "  %s ws %s is out (this is %s): ws upgrade installs it\n", ui.Orange.Render("↑"), r.To, r.From)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "say what the latest release is, and install nothing")
	cmd.Flags().BoolVar(&rollback, "rollback", false, "put back the ws the last upgrade replaced")
	return cmd
}

// sayDaemonAfterInstall says what the daemon of the workspace you're in
// does about the ws just installed at exe: nothing to say outside a
// workspace, or with no daemon up.
func sayDaemonAfterInstall(exe, version string) {
	ctx, err := loadOps()
	if err != nil {
		fmt.Fprintf(ui.Stderr, "    %s a workspace's daemon restarts onto it once nothing's at work there\n", ui.Dim.Render("·"))
		return
	}
	c, err := daemon.DialAny(ctx.WS().Root)
	if err != nil {
		return
	}
	defer c.Close()
	for _, line := range daemonAfterInstall(c, exe, version) {
		fmt.Fprintf(ui.Stderr, "    %s %s\n", ui.Orange.Render("↻"), line)
	}
}

// daemonAfterInstall is what the daemon c reaches does about ws version,
// just installed at exe, in lines.
func daemonAfterInstall(c *daemon.Client, exe, version string) []string {
	switch {
	case !c.Compatible() || c.Build() == "":
		return []string{fmt.Sprintf("this workspace's daemon (pid %d) is from a ws before it restarted itself: ws daemon restart --now starts it again on %s, hanging up the crew it holds (those at their work are resumed)", c.ServerPid(), version)}
	case c.Exe() != exe:
		return []string{fmt.Sprintf("this workspace's daemon runs another ws (%s, at %s), which this didn't change", c.Build(), c.Exe())}
	}
	lines := []string{"this workspace's daemon restarts onto it once nothing's at work (ws daemon restart --now doesn't wait)"}
	waits, _ := c.AtWork()
	for _, w := range waits {
		lines = append(lines, "waiting for "+w)
	}
	return lines
}
