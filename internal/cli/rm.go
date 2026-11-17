package cli

import (
	"fmt"
	"github.com/withworkspace/workspace/internal/ops"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/ui"
)

func newBurnCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "burn [repo] <branch>",
		Aliases: []string{"rm"},
		Short:   "Remove a worktree",
		Args:    cobra.RangeArgs(1, 2),
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

			var repoArg, branch string
			if len(args) == 2 {
				repoArg = args[0]
				branch = args[1]
			} else {
				branch = args[0]
			}

			repo, err := resolveRepo(ctx, repoArg)
			if err != nil {
				return err
			}

			capsule, err := resolveCapsule(ctx, repo, branch)
			if err != nil {
				return err
			}

			// Note this before removal, while the capsule still exists. The
			// shell wrapper has already stepped the shell out to the workspace
			// root and reported where it came from; this only decides whether
			// to tell the user why they're no longer where they were.
			shellDir := os.Getenv(ShellPWD)
			burningShellDir := shellDir != "" && dirContains(filepath.Join(ctx.WS().RepoDir(repo), capsule), shellDir)

			check, err := ctx.WS().CheckRemoveWorktree(repo, capsule)
			if err != nil {
				return err
			}

			if check.IsDirty {
				fmt.Fprintf(ui.Stderr, "  %s Worktree %s/%s has uncommitted changes\n", ui.Orange.Render("⚠"), ctx.WS().FormatRepoName(repo), capsule)
				confirmed, err := ui.Confirm("Remove anyway?")
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintf(ui.Stderr, "  %s Aborted\n", ui.Dim.Render("·"))
					return nil
				}
			}

			silo := ops.SiloToGround
			if target, ok := ctx.WS().Silo[repo]; ok && target == capsule {
				fmt.Fprintf(ui.Stderr, "  %s Capsule %s/%s is an active silo target\n",
					ui.Orange.Render("⚠"), ctx.WS().FormatRepoName(repo), capsule)
				repoint, err := ui.Confirm("Repoint silo to .ground?")
				if err != nil {
					return err
				}
				if !repoint {
					silo = ops.SiloDrop
				}
			}

			burned, err := ctx.Burn(repo, capsule, ops.BurnOptions{Force: check.IsDirty, Silo: silo})
			if err != nil {
				return err
			}
			if burned.IDEErr != nil {
				fmt.Fprintf(ui.Stderr, "  %s workspace files: %v\n", ui.Orange.Render("⚠"), burned.IDEErr)
			}
			switch {
			case burned.SiloErr != nil:
				fmt.Fprintf(ui.Stderr, "  %s silo re-sync failed: %v\n", ui.Orange.Render("⚠"), burned.SiloErr)
			case burned.Silo && silo == ops.SiloToGround:
				fmt.Fprintf(ui.Stderr, "  %s Silo repointed to .ground\n", ui.Green.Render("✓"))
			}

			fmt.Fprintf(ui.Stderr, "  %s Removed %s %s\n", ui.Green.Render("✓"), ctx.WS().FormatRepoName(repo), ui.TagDim.Render(capsule))

			if burningShellDir {
				fmt.Fprintf(ui.Stderr, "  %s Back to workspace root\n", ui.Dim.Render("·"))
			}
			return nil
		},
	}
}

// dirContains reports whether path is dir itself or nested below it. Paths are
// also compared with symlinks resolved, so a cwd reached through a symlinked
// workspace root still matches.
func dirContains(dir, path string) bool {
	if isWithin(dir, path) {
		return true
	}
	realDir, dirErr := filepath.EvalSymlinks(dir)
	realPath, pathErr := filepath.EvalSymlinks(path)
	if dirErr != nil || pathErr != nil {
		return false
	}
	return isWithin(realDir, realPath)
}

func isWithin(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
