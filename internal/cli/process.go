package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws process: the workspace's process, PROCESS.md in the config repo: how
// work goes here, in prose. The planner and the orchestrator read it as
// it's written; what ws holds itself whatever they read is a guarantee, in
// ws.toml.

func newProcessCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "process",
		Short: "The workspace's process: how work goes here, in prose",
		Long: `The workspace's process is how work goes here, in prose: the steps work
takes, whose turn each is, what done means, and when it's yours. It's the
team's, in the config repo: PROCESS.md beside ws.toml, or where
[agents] process says, changed by commit like the rest of the config.

The planner reads it as it's written to plan a mission, and the
orchestrator to know what to do when something happens. A plan's steps may
quote it, and ws checks the words are there. What ws holds for every
mission, whatever an agent reads, is a guarantee in ws.toml's [missions]
(review_before_pr, review_before_land), shown here too.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "" && format != "llm" && format != "text" {
				return fmt.Errorf("unknown format %q (want text or llm)", format)
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			v, err := processView(ctx)
			if err != nil {
				return err
			}
			if format == "llm" {
				_, err = os.Stdout.WriteString(v.render(false, true))
			} else {
				_, err = io.WriteString(ui.Stdout, v.render(true, true))
			}
			return err
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "", "output format: text or llm")
	return cmd
}

// processView is ws process, and ws show process: the process as it's
// written, where it is, and the guarantees ws holds beside it.
func processView(ctx *ops.Workspace) (*view, error) {
	p, err := ctx.Process()
	if err != nil {
		return nil, err
	}
	v := &view{}
	if strings.TrimSpace(p.Text) == "" {
		v.head = "The workspace's process: none"
		v.body = append(v.body, fmt.Sprintf("%s isn't there, or says nothing. Write how work goes here in it (the steps work takes, whose turn each is, when it's yours), in the config repo beside ws.toml; [agents] process names another file.", p.Name))
	} else {
		v.head = "The workspace's process: " + p.Name
		where := p.File
		switch {
		case p.Commit != "" && !p.Uncommitted:
			where += ", as committed in " + p.Commit
		case p.Commit != "":
			where += ", with changes not committed since " + p.Commit
		default:
			where += ", not committed"
		}
		v.body = append(v.body, where, "", "````markdown", strings.TrimRight(p.Text, "\n"), "````")
	}
	if g := ctx.Guarantees(); len(g) > 0 {
		v.body = append(v.body, "", "What ws holds for every mission, whatever an agent reads (ws.toml's [missions]):")
		for _, x := range g {
			v.body = append(v.body, "- "+x.Key+": "+x.What+".")
		}
	}
	return v, nil
}
