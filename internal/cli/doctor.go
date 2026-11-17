package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

func newDoctorCmd() *cobra.Command {
	var fix, reindex bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check workspace health",
		Long: `Check workspace health: its repos, missions, config, and records.

--reindex has every store say its words again from its records first (what
ws find searches): each mission's decisions, brief and files, each
conversation's turns and findings, each review's comments, what each
pull request ws keeps says it does, and each artifact's title and notes. It's what a change to what's searched
needs, and it's safe to run at any time.`,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if reindex {
				start := time.Now()
				n, err := ctx.SaidWords(cmd.Context())
				if err != nil {
					return fmt.Errorf("saying the records' words again: %w", err)
				}
				fmt.Fprintf(ui.Stderr, "%s %s\n\n", ui.Green.Render("✓"), fmt.Sprintf("Said again what %d %s, %d %s, %d %s, %d %s, %d %s and %d %s say (%s)",
					n.Missions, plural(n.Missions, "mission"), n.Conversations, plural(n.Conversations, "conversation"), n.Reviews, plural(n.Reviews, "review"),
					n.PRs, plural(n.PRs, "pull request"), n.Pages, plural(n.Pages, "page"), n.Artifacts, plural(n.Artifacts, "artifact"), time.Since(start).Round(time.Millisecond)))
			}

			categories := ctx.WS().Doctor(checkMissions)
			if cat := checkConfig(ctx); len(cat.Checks) > 0 {
				categories = append(categories, cat)
			}
			if cat := checkRecords(ctx); len(cat.Checks) > 0 {
				categories = append(categories, cat)
			}
			if cat := checkArtifacts(ctx); len(cat.Checks) > 0 {
				categories = append(categories, cat)
			}
			hasFail := printDoctorResults(categories, fix)

			if hasFail {
				return fmt.Errorf("doctor found issues")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Auto-fix safe issues")
	cmd.Flags().BoolVar(&reindex, "reindex", false, "Say every store's words again from its records (what ws find searches)")
	return cmd
}

// checkConfig is what the workspace's config files say that ws doesn't
// read: a key that's misspelt (alias for aliases) or in the wrong file is
// otherwise passed over without a word. A repo's ws.repo.toml that can't
// be read at all is here too: ws carries on as if it weren't there.
func checkConfig(ctx *ops.Workspace) workspace.CheckCategory {
	cat := workspace.CheckCategory{Name: "Config"}
	unknown := func(entry, hint string) {
		file, key, _ := strings.Cut(entry, ": ")
		cat.Checks = append(cat.Checks, workspace.CheckResult{
			Name:    file,
			Status:  workspace.CheckWarn,
			Detail:  fmt.Sprintf("%s isn't something ws reads there", key),
			FixHint: hint,
		})
	}
	for _, entry := range ctx.Config().Unknown {
		unknown(entry, "check the key's spelling, and that it's in the right file (MANUAL.md, Configuration)")
	}
	for _, repo := range ctx.WS().RepoNames {
		path := filepath.Join(ctx.WS().MainWorktree(repo), "ws.repo.toml")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		rc, err := config.ParseRepoConfig(path)
		if err != nil {
			cat.Checks = append(cat.Checks, workspace.CheckResult{
				Name:   repo + "/ws.repo.toml",
				Status: workspace.CheckFail,
				Detail: "can't be read, so nothing in it applies: " + strings.TrimPrefix(err.Error(), "parsing "+path+": "),
			})
			continue
		}
		for _, entry := range rc.Unknown {
			unknown(repo+"/"+entry, "check the key's spelling (MANUAL.md, ws.repo.toml)")
		}
	}
	return cat
}

func printDoctorResults(categories []workspace.CheckCategory, fix bool) bool {
	hasFail := false

	for i, cat := range categories {
		if i > 0 {
			fmt.Fprintln(ui.Stderr)
		}
		fmt.Fprintln(ui.Stderr, ui.Bold.Render(cat.Name))

		for _, check := range cat.Checks {
			// Attempt fix if --fix and a Fix function exists
			if fix && check.Fix != nil && check.Status != workspace.CheckOK {
				if err := check.Fix(); err != nil {
					check.Detail = fmt.Sprintf("fix failed: %v", err)
				} else {
					check.Status = workspace.CheckOK
					check.Detail = "fixed"
				}
			}

			var icon, detail string
			switch check.Status {
			case workspace.CheckOK:
				icon = ui.Green.Render("✓")
				if check.Detail != "" {
					detail = " " + ui.Dim.Render("→ "+check.Detail)
				}
			case workspace.CheckWarn:
				icon = ui.Orange.Render("⚠")
				if check.Detail != "" {
					detail = " — " + check.Detail
				}
			case workspace.CheckFail:
				hasFail = true
				icon = ui.Red.Render("✗")
				if check.Detail != "" {
					detail = " — " + check.Detail
				}
			}
			fmt.Fprintf(ui.Stderr, "  %s %s%s\n", icon, check.Name, detail)

			// Print hint if --fix and a hint exists
			if fix && check.FixHint != "" && check.Status != workspace.CheckOK {
				fmt.Fprintf(ui.Stderr, "    %s %s\n", ui.Dim.Render("hint:"), check.FixHint)
			}
		}
	}

	return hasFail
}
