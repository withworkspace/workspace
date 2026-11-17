package cli

import (
	"errors"
	"fmt"
	"github.com/withworkspace/workspace/internal/ops"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Landing: a capsule's commits pushed straight to the default branch of a
// repo that has no pull requests ([repos.<name>] landing = "direct"). It's
// done in one place (workspace.Land), by you: from a shell (ws land), the
// deck or the web UI.

func newLandCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "land [repo] [capsule]",
		Short: "Land a capsule's commits straight on the default branch (a repo with no pull requests)",
		Long: `Land a capsule of a repo whose work goes straight to its default branch,
with no pull request: one that says landing = "direct" under [repos.<name>]
in ws.toml (or in its own ws.repo.toml).

Its commits are rebased onto the default branch as it is on the remote,
checked with the repo's verify command if it has one, and pushed there. The
ground is brought up to them, and the capsule is burned.

  ws land                  the capsule you're standing in
  ws land <capsule>        a capsule of the repo you're standing in
  ws land <repo> <capsule>

It stops, with nothing pushed, when the capsule has changes that aren't
committed or nothing to land, when the rebase clashes (the capsule is left
as it was), when the check fails, or when the remote refuses the push. If
the default branch moves while it's landing, it fetches, rebases and tries
again, twice. Nothing is ever forced, and there's never a merge commit.

There's no review between a landing and everyone else seeing it but you,
so it's yours to run: crew commit and report done, and the orchestrator
can't land.`,
		Args: cobra.MaximumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return completeRepoNames(cmd, args, toComplete)
			case 1:
				return completeWorktreeNames(0)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			by := ops.ActorFromEnv()
			if err := ops.MayLand(by); err != nil {
				return err
			}
			repo, capsule, err := landTarget(ctx, args)
			if err != nil {
				return err
			}
			name := ctx.WS().FormatRepoName(repo) + " " + ui.TagDim.Render(capsule)
			out, err := ctx.Land(repo, capsule, by, workspace.LandOptions{Step: func(step string) {
				fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Dim.Render("·"), step)
			}})
			var stopped *workspace.LandError
			if errors.As(err, &stopped) {
				if stopped.Output != "" {
					fmt.Fprintln(ui.Stderr, ui.Dim.Render(indent(stopped.Output, "    ")))
				}
				return errors.New(stopped.Msg)
			}
			if err != nil {
				return err
			}
			l := out.Landed
			fmt.Fprintf(ui.Stderr, "  %s Landed %s on %s: %s (%s)\n", ui.Green.Render("✓"), name, l.Branch, commitsWord(len(l.Commits)), shortSHA(l.Head))
			for _, c := range l.Commits {
				fmt.Fprintf(ui.Stderr, "      %s\n", ui.Dim.Render(c))
			}
			for _, said := range l.Settled {
				fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Orange.Render("⚠"), said)
			}
			if l.Ground != nil {
				fmt.Fprintf(ui.Stderr, "  %s The ground wasn't brought up to it: %v\n", ui.Orange.Render("⚠"), l.Ground)
			}
			if out.RecordErr != nil {
				fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Orange.Render("⚠"), out.Unrecorded())
				return nil
			}
			if out.Mission != "" {
				fmt.Fprintf(ui.Stderr, "  %s mission %s\n", ui.Dim.Render("→"), out.Mission)
			}
			if out.BurnErr != nil {
				fmt.Fprintf(ui.Stderr, "  %s The capsule is still there: %v\n", ui.Orange.Render("⚠"), out.BurnErr)
			} else {
				fmt.Fprintf(ui.Stderr, "  %s Removed the capsule\n", ui.Green.Render("✓"))
			}
			return nil
		},
	}
}

// landTarget is the capsule ws land was asked for: named, or the one
// you're standing in.
func landTarget(ctx *ops.Workspace, args []string) (repo, capsule string, err error) {
	if len(args) == 0 {
		cwd, _ := userDir()
		repo, capsule, ok := workspace.DetectRepo(ctx.WS().Root, cwd)
		if !ok || capsule == "" || capsule == workspace.GroundDir {
			return "", "", errors.New("which capsule? Run it from one, or name it: ws land <repo> <capsule>")
		}
		return repo, capsule, nil
	}
	repoArg, name := "", args[0]
	if len(args) == 2 {
		repoArg, name = args[0], args[1]
	}
	if repo, err = resolveRepo(ctx, repoArg); err != nil {
		return "", "", err
	}
	capsule, err = resolveCapsule(ctx, repo, name)
	return repo, capsule, err
}

// commitsWord is "1 commit", "3 commits".
func commitsWord(n int) string { return fmt.Sprintf("%d %s", n, plural(n, "commit")) }
