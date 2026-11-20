package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

func newSiloCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "silo",
		Short: "Manage repo silos for stable runtime directories",
	}
	cmd.AddCommand(newSiloPointCmd())
	cmd.AddCommand(newSiloStopCmd())
	cmd.AddCommand(newSiloStatusCmd())
	cmd.AddCommand(newSiloWatchCmd())
	return cmd
}

func newSiloPointCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "point <repo> <capsule>",
		Short: "Point a repo's silo at a capsule",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return completeRepoNames(cmd, args, toComplete)
			case 1:
				return completeWorktreeNames(0)(cmd, args, toComplete)
			default:
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}

			repo, err := resolveRepo(ctx, args[0])
			if err != nil {
				return err
			}

			capsule := args[1]
			if capsule != workspace.GroundDir {
				capsule, err = resolveCapsule(ctx, repo, capsule)
				if err != nil {
					return err
				}
			}

			if err := ctx.PointSilo(repo, capsule, os.Stderr); err != nil {
				return err
			}

			fmt.Fprintf(ui.Stderr, "  %s Silo for %s now points at %s\n",
				ui.Green.Render("✓"), ctx.WS().FormatRepoName(repo), ui.TagDim.Render(capsule))
			return nil
		},
	}
}

func newSiloStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <repo>",
		Short: "Remove a repo's silo",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return completeRepoNames(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			repo, err := resolveRepo(ctx, args[0])
			if err != nil {
				return err
			}
			siloDir := ctx.WS().SiloWorktree(repo)
			if _, err := os.Stat(siloDir); os.IsNotExist(err) {
				return fmt.Errorf("no silo exists for %s", repo)
			}
			// Let go of first, so the daemon stops keeping it before it's gone.
			delete(ctx.WS().Silo, repo)
			if err := config.SaveSilo(ctx.WS().Root, ctx.WS().Silo); err != nil {
				return fmt.Errorf("saving silo state: %w", err)
			}
			bareDir := ctx.WS().BareDir(repo)
			// Force remove since .silo/ may have build artifacts that make it "dirty"
			if err := workspace.GitWorktreeRemoveForce(bareDir, siloDir); err != nil {
				return fmt.Errorf("removing silo worktree: %w", err)
			}
			fmt.Fprintf(ui.Stderr, "  %s Removed silo for %s\n",
				ui.Green.Render("✓"), ctx.WS().FormatRepoName(repo))
			return nil
		},
	}
}

func newSiloStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show where each silo points, and how the daemon keeping it stands",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if len(ctx.WS().Silo) == 0 {
				fmt.Fprintf(ui.Stderr, "No active silos.\n")
				return nil
			}
			states, err := askSilos(ctx.WS().Root)
			if err != nil {
				fmt.Fprintf(ui.Stderr, "  %s can't ask the daemon how they stand: %v\n", ui.Orange.Render("⚠"), err)
			}
			byRepo := map[string]workspace.SiloState{}
			for _, st := range states {
				byRepo[st.Repo] = st
			}
			now := time.Now()
			for _, repo := range ctx.WS().RepoNames {
				target, ok := ctx.WS().Silo[repo]
				if !ok {
					continue
				}
				st, kept := byRepo[repo]
				if err == nil && !kept {
					st.Error = "not kept yet"
				}
				fmt.Fprintf(ui.Stderr, "  %-20s %-20s %s\n", ctx.WS().FormatRepoName(repo), target, siloStanding(st, err == nil, now))
				if h := st.Hook; h != nil && h.Error != "" {
					fmt.Fprintf(ui.Stderr, "  %-20s %s\n", "", ui.Orange.Render(fmt.Sprintf("%s hook failed %s ago: %s", h.Name, ago(now, h.At), h.Error)))
				}
			}
			return nil
		},
	}
}

// siloStanding is a silo's state in a few words: whether it's kept, and
// when it was last synced.
func siloStanding(st workspace.SiloState, asked bool, now time.Time) string {
	switch {
	case !asked:
		return ui.Dim.Render("(unknown)")
	case st.Error != "":
		return ui.Orange.Render("(" + st.Error + ")")
	}
	said := fmt.Sprintf("watching %d folder(s)", st.Dirs)
	if !st.Watching {
		said = "not watching"
	}
	if n := len(st.Syncs); n > 0 {
		last := st.Syncs[n-1]
		said += fmt.Sprintf(", synced %s ago", ago(now, last.At))
		if last.Error != "" {
			return ui.Orange.Render("(" + said + ": " + last.Error + ")")
		}
	}
	return ui.Green.Render("(" + said + ")")
}

// ago is how long before now t was, rounded for reading.
func ago(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return d.Round(time.Second).String()
	case d < time.Hour:
		return d.Round(time.Minute).String()
	}
	return d.Round(time.Hour).String()
}

func newSiloWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Show the daemon's syncs into each silo as they're made",
		Long: `The daemon keeps every silo: it mirrors the capsule each is pointed at as
files change, with or without this. ws silo watch shows its syncs as they're
made, and r asks it for a full resync of every silo.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			lines, _ := cmd.Flags().GetBool("verbose")
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if len(ctx.WS().Silo) == 0 {
				return fmt.Errorf("no active silos — use 'ws silo point' first")
			}
			c, err := daemon.Connect(ctx.WS().Root)
			if err != nil {
				return err
			}
			defer c.Close()
			if _, err := silosOf(c); err != nil {
				return err
			}
			if ui.IsInteractive() && !lines {
				m := newSiloWatchModel(ctx.WS().FormatRepoName, func() ([]workspace.SiloState, error) { return silosOf(c) }, func() error { return c.ResyncSilo("") })
				_, err := tea.NewProgram(m, tea.WithOutput(os.Stderr)).Run()
				return err
			}
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
			return printSiloSyncs(os.Stderr, func() ([]workspace.SiloState, error) { return silosOf(c) }, stop)
		},
	}
	cmd.Flags().BoolP("verbose", "v", false, "Print a line per sync and hook, rather than a screen")
	return cmd
}
