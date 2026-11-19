package cli

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/config"
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
			bareDir := ctx.WS().BareDir(repo)
			// Force remove since .silo/ may have build artifacts that make it "dirty"
			if err := workspace.GitWorktreeRemoveForce(bareDir, siloDir); err != nil {
				return fmt.Errorf("removing silo worktree: %w", err)
			}
			delete(ctx.WS().Silo, repo)
			if err := config.SaveSilo(ctx.WS().Root, ctx.WS().Silo); err != nil {
				return fmt.Errorf("saving silo state: %w", err)
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
		Short: "Show silo state for all repos",
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
			lockPath := filepath.Join(ctx.WS().Root, ".silo.lock")
			watching := isWatcherRunning(lockPath)
			for _, repo := range ctx.WS().RepoNames {
				target, ok := ctx.WS().Silo[repo]
				if !ok {
					continue
				}
				status := "synced"
				if watching {
					status = "watching"
				}
				targetDir := filepath.Join(ctx.WS().RepoDir(repo), target)
				if _, err := os.Stat(targetDir); os.IsNotExist(err) {
					status = "target missing"
				}
				fmt.Fprintf(ui.Stderr, "  %-20s %-20s (%s)\n",
					ctx.WS().FormatRepoName(repo), target, status)
			}
			return nil
		},
	}
}

func isWatcherRunning(lockPath string) bool {
	return workspace.IsLockHeld(lockPath)
}

func newSiloWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch and sync all active silos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")

			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if len(ctx.WS().Silo) == 0 {
				return fmt.Errorf("no active silos — use 'ws silo point' first")
			}
			lockPath := filepath.Join(ctx.WS().Root, ".silo.lock")
			if err := workspace.AcquireLockFile(lockPath); err != nil {
				return err
			}
			defer workspace.ReleaseLockFile(lockPath)

			stop := make(chan struct{})

			if ui.IsInteractive() && !verbose {
				// Silence the logger in interactive mode — TUI handles display
				silentLogger := log.New(io.Discard, "", 0)
				watcher, err := workspace.NewSiloWatcher(ctx.WS(), silentLogger)
				if err != nil {
					return err
				}

				syncCh := make(chan workspace.SyncEvent, 64)
				watcher.OnSync = func(ev workspace.SyncEvent) {
					select {
					case syncCh <- ev:
					default: // drop if UI is behind
					}
				}

				targetsCh := make(chan map[string]string, 8)
				watcher.OnTargetsChanged = func(targets map[string]string) {
					select {
					case targetsCh <- targets:
					default:
					}
				}

				go func() {
					watcher.Watch(stop, ctx.WS().Silo)
					close(syncCh)
					close(targetsCh)
				}()

				m := newSiloWatchModel(ctx.WS(), syncCh, targetsCh, watcher.FullResyncAll)
				p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
				if _, err := p.Run(); err != nil {
					close(stop)
					return err
				}
				close(stop)
				return nil
			}

			// Non-interactive (or verbose): plain log output
			logger := log.New(os.Stderr, "", log.LstdFlags)
			watcher, err := workspace.NewSiloWatcher(ctx.WS(), logger)
			if err != nil {
				return err
			}
			watcher.Verbose = verbose
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigCh
				logger.Println("Shutting down...")
				close(stop)
			}()

			return watcher.Watch(stop, ctx.WS().Silo)
		},
	}
	cmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging of watcher events")
	return cmd
}
