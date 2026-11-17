package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

func newMissionOpenPRCmd() *cobra.Command {
	var slug, title, body string
	var draft bool
	cmd := &cobra.Command{
		Use:   "open-pr [<repo>/<capsule>] --title <title> [--body <text>|-]",
		Short: "Open a pull request from one of the mission's capsules, through ws",
		Long: `Push a capsule's branch and open a pull request from it onto the default
branch, through ws, so the process's rules about pull requests are held
where ws is the one acting: one the read-back says ws holds before a pull
request (held: your review before a pull request) refuses it until your
latest review of the capsule approves it as it is.

In a mission with a plan, a pull request is opened by the step that makes
one (its makes: says a pull request): crew open it there and nowhere else,
and the orchestrator doesn't accept that step until one's open. You may
open one whenever you like.

The capsule is the one named, else the crew's own, else the one you're
standing in, else the mission's only one. Refused in a repo with no pull
requests, with changes not committed or nothing to open, and when one is
open from its branch already. --body - reads the description from stdin.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			by := ops.ActorFromEnv()
			capsule := ""
			if len(args) == 1 {
				capsule = args[0]
			} else if capsule, err = prCapsule(ctx, m, by); err != nil {
				return err
			}
			if body == "-" {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading the description: %w", err)
				}
				body = string(data)
			}
			out, err := ctx.OpenPR(m.Slug, capsule, ops.NewPullRequest{Title: title, Body: body, Draft: draft}, by)
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), out)
			fmt.Println(out.PR.URL)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&title, "title", "", "the pull request's title")
	cmd.Flags().StringVar(&body, "body", "", "its description (- reads it from stdin)")
	cmd.Flags().BoolVar(&draft, "draft", false, "open it as a draft")
	return cmd
}

// prCapsule is the capsule a pull request is opened from when none is
// named: the crew's own, else the one you're standing in, else the
// mission's only one.
func prCapsule(ctx *ops.Workspace, m *mission.Mission, by ops.Actor) (string, error) {
	if c, ok := m.CrewByID(by.ID); ok && by.Kind == ops.Crew && c.Capsule != "" {
		return c.Capsule, nil
	}
	if cwd, err := userDir(); err == nil {
		if repo, name, ok := workspace.DetectRepo(ctx.WS().Root, cwd); ok && m.HasCapsule(repo, name) {
			return repo + "/" + name, nil
		}
	}
	if len(m.Capsules) == 1 {
		return m.Capsules[0].Repo + "/" + m.Capsules[0].Name, nil
	}
	var names []string
	for _, c := range m.Capsules {
		names = append(names, c.Repo+"/"+c.Name)
	}
	if len(names) == 0 {
		return "", errors.New("the mission has no capsules to open a pull request from")
	}
	return "", fmt.Errorf("which capsule? Name it: ws mission open-pr <repo>/<capsule> (%s)", strings.Join(names, ", "))
}
