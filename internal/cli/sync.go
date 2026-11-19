package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/ide"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// newSyncCmd is ws sync, which was ws upgrade until that became upgrading
// ws itself.
func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "sync",
		Short:             "Pull latest config and set up new repos",
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}

			root := ctx.WS().Root
			configPath := filepath.Join(root, config.FileName)

			// Snapshot old config
			oldCfg, err := config.Parse(configPath)
			if err != nil {
				return err
			}

			// Enable workspace git, pull, then disable
			workspace.EnableWorkspaceGit(root)

			oldHead := workspace.GitRevParse(root, "HEAD")
			newCfg, pullErr := pullAndParse(root, configPath)
			if pullErr != nil {
				workspace.DisableWorkspaceGit(root)
				return pullErr
			}
			files := changedFiles(root, oldHead)

			workspace.DisableWorkspaceGit(root)

			// Diff configs
			diff := config.Diff(oldCfg, newCfg)

			if diff.IsEmpty() && len(files) == 0 {
				fmt.Fprintln(ui.Stderr, "Already up to date.")
				return nil
			}

			// Print summary
			printFileChanges(files)
			printDiffSummary(diff)

			// Clone new repos
			if len(diff.Added) > 0 {
				// Rebuild context with new config so repo list includes added repos
				newCtx, err := LoadOps(root)
				if err != nil {
					return err
				}

				if err := newCtx.WS().EnsureReposDir(); err != nil {
					return err
				}

				var toClone []string
				for _, name := range diff.Added {
					if newCtx.WS().IsHidden(name) {
						fmt.Fprintf(ui.Stderr, "    %s is in hidden group %s — it will clone on first use\n", name, newCtx.WS().GroupOf(name))
						continue
					}
					toClone = append(toClone, name)
				}

				if len(toClone) > 0 {
					fmt.Fprintln(ui.Stderr)
					clonedRepos, err := cloneRepos(newCtx.WS(), toClone, os.Stderr)
					if err != nil {
						return err
					}
					runAfterCreateHooks(newCtx.WS(), clonedRepos, os.Stderr, os.Stderr)
				}
			}

			// Resync IDE workspace files in case config changed
			refreshCtx, err := LoadOps(root)
			if err == nil {
				if err := ide.Regenerate(refreshCtx.WS().Root, refreshCtx.WS().Boarded, refreshCtx.WS().DisplayNames, refreshCtx.WS().Org); err != nil {
					fmt.Fprintf(ui.Stderr, "  %s workspace files: %v\n", ui.Orange.Render("⚠"), err)
				}
			}

			fmt.Fprintln(ui.Stderr, "\nSynced.")
			return nil
		},
	}
}

// pullAndParse fetches the latest remote config and resets tracked files to match.
// Untracked files (like ws.local.toml) are preserved.
func pullAndParse(root, configPath string) (*config.Config, error) {
	if err := workspace.GitFetchOrigin(root); err != nil {
		return nil, fmt.Errorf("git fetch: %w", err)
	}
	if err := workspace.GitResetHard(root, "origin/HEAD"); err != nil {
		return nil, fmt.Errorf("git reset: %w", err)
	}

	cfg, err := config.Parse(configPath)
	if err != nil {
		return nil, fmt.Errorf("parsing updated config: %w", err)
	}

	return cfg, nil
}

// fileChange is a file in the workspace root that sync changed.
type fileChange struct {
	Path string
	From string // where it was, for a rename
	Verb string // added, updated, removed, renamed
}

// changedFiles is what the pull changed in the root's files since oldHead.
// None when oldHead is unknown (an empty repo) or git can't say.
func changedFiles(root, oldHead string) []fileChange {
	if oldHead == "" {
		return nil
	}
	return parseNameStatus(git.Lines(root, "diff", "--no-color", "--name-status", "-M", oldHead, "HEAD"))
}

// parseNameStatus reads git diff --name-status lines.
func parseNameStatus(lines []string) []fileChange {
	var out []fileChange
	for _, l := range lines {
		parts := strings.Split(l, "\t")
		if len(parts) < 2 || parts[0] == "" {
			continue
		}
		c := fileChange{Path: parts[len(parts)-1]}
		switch parts[0][0] {
		case 'A':
			c.Verb = "added"
		case 'D':
			c.Verb = "removed"
		case 'R':
			c.Verb = "renamed"
			if len(parts) == 3 {
				c.From = parts[1]
			}
		case 'C':
			c.Verb = "added"
		default:
			c.Verb = "updated"
		}
		out = append(out, c)
	}
	return out
}

func printFileChanges(files []fileChange) {
	for _, f := range files {
		mark := ui.Dim.Render("~")
		switch f.Verb {
		case "added":
			mark = ui.Green.Render("+")
		case "removed":
			mark = ui.Orange.Render("-")
		}
		if f.Verb == "renamed" && f.From != "" {
			fmt.Fprintf(ui.Stderr, "  %s %s renamed to %s\n", mark, f.From, f.Path)
			continue
		}
		fmt.Fprintf(ui.Stderr, "  %s %s %s\n", mark, f.Path, f.Verb)
	}
}

func printDiffSummary(diff config.ConfigDiff) {
	if len(diff.Added) > 0 {
		fmt.Fprintf(ui.Stderr, "  %s %d new %s (%s)\n",
			ui.Green.Render("+"),
			len(diff.Added),
			pluralize(len(diff.Added), "repo", "repos"),
			strings.Join(diff.Added, ", "))
	}
	if len(diff.Removed) > 0 {
		fmt.Fprintf(ui.Stderr, "  %s %d removed %s (%s)\n",
			ui.Orange.Render("-"),
			len(diff.Removed),
			pluralize(len(diff.Removed), "repo", "repos"),
			strings.Join(diff.Removed, ", "))
		for _, name := range diff.Removed {
			fmt.Fprintf(ui.Stderr, "    %s is now orphaned — remove when ready\n", name)
		}
	}
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
