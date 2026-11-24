package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// A Work step's limit: ws holds its clock (ops/limit.go), and this sets
// it.

func newMissionLimitCmd() *cobra.Command {
	var slug string
	var step int
	cmd := &cobra.Command{
		Use:   "limit [<duration>]",
		Short: "Set how long a step has from now: more time, or for crew, less",
		Long: `Set how long the step the mission is at (or --step n) has, from now: 45m,
1h, 1h30m. Without a duration, it's the step's limit again (- limit: in
the plan), or 30m for a step with none.

ws holds a step's clock from when crew start on it, and says once when it
runs out while they're still at it: to the orchestrator, whose call it is
by the process, or with the orchestrator off, to the user.

Crew may set themselves less time than they have, never more. The
orchestrator and the user may give a step more:

  ws mission limit 45m          # crew: I'll take 45 minutes, not the hour
  ws mission limit --step 1 1h  # another hour for step 1`,
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
			var d time.Duration
			if len(args) == 1 {
				var ok bool
				var rest string
				if d, rest, ok = mission.ParseLimit(args[0]); !ok || strings.TrimSpace(rest) != "" {
					return fmt.Errorf("%q isn't how long: say it as 45m, 1h or 1h30m", args[0])
				}
			}
			said, err := ctx.MoreTime(m.Slug, step, d, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), said)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().IntVar(&step, "step", 0, "the step, by its number (default: the one the mission's at)")
	return cmd
}
