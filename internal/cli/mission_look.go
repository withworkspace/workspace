package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

func newMissionLookCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "look",
		Short: "Have ws look now at whether what the mission's Wait step waits for has happened",
		Long: `Have ws look now at the Wait step the mission is at: it runs the step's
checks (commands that show what it waits for has happened), and passes
the step if every one is met.

A Wait step with checks is watched: ws runs them by itself every few
minutes while the mission waits, so this is only for not waiting for the
next look. A Wait without one has nothing for ws to run: nobody's
watching, and it's passed when someone says it's happened (ws mission
accept, or marking it done on the mission's page).

For you.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			said, err := ctx.CheckWait(m.Slug, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Dim.Render("·"), said)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

// watchWaitsEvery is how often the daemon looks for a watched Wait step
// whose next look is due (the step says how often that is: mission.Watch).
const watchWaitsEvery = time.Minute

// watchWaitsLoop runs the checks of each watched Wait step a mission is
// at, as its looks come due, and passes those that are met, until stop
// closes. A look runs commands, so it's a loop of its own: a slow one
// holds up nothing but the next look.
func watchWaitsLoop(root string, logf func(string, ...any), stop <-chan struct{}) {
	t := time.NewTicker(watchWaitsEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			ctx, err := LoadOps(root)
			if err != nil {
				continue
			}
			for _, said := range ctx.CheckWaits() {
				logf("waits: %s", said)
			}
		}
	}
}
