package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/orchestrator"
	"github.com/withworkspace/workspace/internal/ui"
)

// orchestrateSpawner has the workspace's daemon run the orchestrator for
// an event; tests stub it.
var orchestrateSpawner orchestrator.Spawner = spawnOrchestrator

func newMissionOrchestrateCmd() *cobra.Command {
	var slug, crewID, trigger, model string
	var auto, printPrompt bool
	var runSince int64
	cmd := &cobra.Command{
		Use:   "orchestrate",
		Short: "Have the orchestrator decide what happens next on a mission",
		Long: `Run the orchestrator on a mission: a headless agent that reads the brief,
the log, the capsules' diffs and PRs and the crew's output, then decides
what's next and acts through ws: ws mission dispatch (start new crew on
the next piece of work), resume-with (crew should carry on), flag (needs you), new (a
follow-up mission) or conclude (looks finished), always leaving a note. It
doesn't change code.

ws runs this automatically when crew report done, or end by themselves. Configure it under [orchestrator] in ws.toml / ws.local.toml:
harness (cursor, cursor-acp, claude, pi, codex or opencode; default the crew's, if it can run headless),
model (default ` + harness.CursorOrchestratorModel + ` on cursor, ` + harness.ClaudeOrchestratorModel + ` on claude,
` + harness.CodexOrchestratorModel + ` on codex, your own default on pi and opencode),
enabled = false to stop the automatic runs, dispatch = "ask" to queue
dispatched crew for you instead of launching them, and max_dispatches
(default 3) dispatches in a row without you before it must flag you.`,
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
			r := ops.OrchestratorRun{Slug: m.Slug, Crew: crewID, Trigger: trigger, Model: model, Auto: auto, Said: func(line string) {
				fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Dim.Render("·"), line)
			}}
			if runSince != 0 {
				r.Since = time.Unix(runSince, 0).UTC()
			}
			if printPrompt {
				prompt, args, file, err := ctx.OrchestratorPrompt(r)
				if err != nil {
					return err
				}
				fmt.Println(prompt)
				fmt.Fprintf(ui.Stderr, "  %s would write that to %s and run: %s\n", ui.Dim.Render("·"), file, strings.Join(args, " "))
				return nil
			}
			return ctx.Orchestrate(r)
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&crewID, "crew", "", "the crew whose event prompted this")
	cmd.Flags().StringVar(&trigger, "trigger", "manual", "what prompted the run: done, ended, answer (the user answered a flag), review (the user submitted one) or manual")
	cmd.Flags().StringVar(&model, "model", "", "model to use (default: [orchestrator] model, or the harness's own)")
	cmd.Flags().BoolVar(&auto, "auto", false, "run as ws's automatic run: honour [orchestrator] enabled, handle each crew event once")
	cmd.Flags().BoolVar(&printPrompt, "print-prompt", false, "print the orchestrator's prompt and command instead of running it")
	cmd.Flags().Int64Var(&runSince, "run", 0, "the run (its start, in Unix seconds) the mission records as underway, to end when this finishes")
	cmd.Flags().MarkHidden("run")
	return cmd
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func firstNonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// liveOutput is what running crew show, from the workspace's daemon, if
// it's running.
func liveOutput(root string) func(key string) string {
	return func(key string) string {
		c, err := daemon.Dial(root)
		if err != nil {
			return ""
		}
		defer c.Close()
		text, _ := c.Transcript(key)
		return text
	}
}
