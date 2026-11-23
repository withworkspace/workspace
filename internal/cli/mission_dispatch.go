package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

func newMissionDispatchCmd() *cobra.Command {
	var slug, capsule, harnessName, on string
	var queue, proposeIt bool
	var reads []string
	var stage int
	cmd := &cobra.Command{
		Use:   "dispatch <what to do>",
		Short: "Start new crew on a mission's next piece of work",
		Long: `Start fresh crew on the mission's next piece of work. What you dispatch it
with is what it's for: what to do, where earlier crew's output is (files,
commits, PRs) and what to watch out for. ws gives it, ahead of the brief,
where earlier crew left off: the latest handoff from its capsule (or the
mission's), and where the others are. Name what else it should read first
with --read.

The crew runs in the workspace's daemon, and the web UI shows it. With
[orchestrator] dispatch = "ask" (or --queue), it's queued instead: the
mission's page shows it, and launches it when you say.

The orchestrator may dispatch [orchestrator] max_dispatches crew (default
3) in a row; after that it has to flag you, and anything you do on the
mission (launching crew, dismissing a flag) starts the count again.

In a conversation about the mission, its agent proposes a dispatch instead
(--propose): the user starts the crew, or not.` + fmt.Sprintf(onStdin, "dispatch"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			if proposeIt {
				if len(reads) > 0 {
					if _, err := ctx.ReadFirstList(m, reads, userDirOr()); err != nil {
						return err
					}
				}
				return propose(ctx, conversation.Proposal{Kind: conversation.ProposeDispatch, Text: strings.Join(args, " "), Capsule: capsule, Reads: reads})
			}
			if err := refuseInConversation("a dispatch"); err != nil {
				return err
			}
			by := raisedBy()
			if by != mission.RoleOrchestrator && by != "you" {
				return errors.New("crew don't dispatch other crew: run ws mission done when your part is finished, and the orchestrator decides what follows")
			}
			if m.ToBrief() {
				return fmt.Errorf("mission %s is still being briefed", m.Slug)
			}
			brief, _ := missionStore(ctx).ReadBrief(m.Slug)
			stages, _ := m.StageProgress(brief)
			st, err := mission.PickStage(stages, stage)
			if err != nil {
				return err
			}
			if by == mission.RoleOrchestrator && st.Step != nil && st.Step.Who != mission.WhoCrew {
				return fmt.Errorf("step %d of the plan is a %s step, %s, not crew's: crew don't do it, and ws does it, asks the user or waits on it. Accept the step before it if that's done, or flag the user if the plan no longer fits", st.N, st.Step.Type, st.Step.Whose())
			}
			if cur, ok := mission.CurrentStage(stages); by == mission.RoleOrchestrator && st.Step != nil && ok && st.N > cur.N {
				return fmt.Errorf("step %d of the plan isn't reached yet: it's at step %d. Accept step %d once it's done, and ws starts what's next itself", st.N, cur.N, cur.N)
			}
			model := ""
			if on != "" {
				// What crew run on, as a plan's on: says it: one [crew]
				// allowed lists (Launch holds it to that).
				pick := ctx.CrewPick(on)
				harnessName, model = pick.Harness, pick.Model
			}
			dispatch, err := wordsOf(args)
			if err != nil {
				return err
			}
			if dispatch == "" {
				return errors.New("say what the crew are to do")
			}
			if len(reads) > 0 {
				list, err := ctx.ReadFirstList(m, reads, userDirOr())
				if err != nil {
					return err
				}
				dispatch += "\n\nRead these first:\n" + list
			}
			queued := queue || !ctx.Config().OrchestratorAutoDispatch()
			// Out of dispatches is the answer that matters most: say it
			// before anything else (Launch enforces it again).
			if by == mission.RoleOrchestrator && !queued && m.Dispatches >= ctx.Config().OrchestratorMaxDispatches() {
				return dispatchLimitErr(fmt.Errorf("%w: %d crew dispatched without the user", mission.ErrDispatchLimit, m.Dispatches))
			}
			o := ctx
			if queued {
				// Placed now, as Launch would place it; launched later, when
				// sharing a capsule is confirmed or not.
				place, err := o.Place(m.Slug, capsule, true)
				if err != nil {
					return err
				}
				if err := missionStore(ctx).QueueDispatch(m.Slug, &mission.Dispatch{Text: dispatch, Capsule: place, Harness: harnessName, Model: model, By: by, Stage: stage}); err != nil {
					if errors.Is(err, mission.ErrDispatchQueued) {
						return fmt.Errorf("%w; only one waits at a time, so don't queue another: say what you'd dispatch next in ws mission note, or ws mission flag the user", err)
					}
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s queued: %s's page in the web UI shows it, and launches it\n", ui.Green.Render("✓"), m.Slug)
				return nil
			}
			l, err := o.Launch(m.Slug, ops.Dispatch{Harness: harnessName, Model: model, Capsule: capsule, Text: dispatch, Stage: stage}, ops.ActorFromEnv())
			if err != nil {
				return dispatchLimitErr(err)
			}
			for _, id := range l.HandedBack {
				fmt.Fprintf(ui.Stderr, "  %s ended %s's session in %s: it had handed back its work\n", ui.Dim.Render("·"), id, l.Capsule)
			}
			if len(l.Missed) > 0 {
				fmt.Fprintf(ui.Stderr, "  %s %s handed back but couldn't be reached to end its session; it stays open in %s\n", ui.Orange.Render("⚠"), strings.Join(l.Missed, ", "), l.Capsule)
			}
			id, place := l.ID, l
			fmt.Fprintf(ui.Stderr, "  %s dispatched %s on %s in %s; it's running in the workspace's daemon\n", ui.Green.Render("✓"), id, m.Slug, place.Capsule)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&capsule, "capsule", "", "capsule to work in, as repo/name (default: one no crew is working in)")
	cmd.Flags().StringVar(&harnessName, "harness", "", "harness to launch (default: the crew's: $WS_DECK_HARNESS, [crew] harness, or cursor)")
	cmd.Flags().StringVar(&on, "on", "", "what the crew run on, as harness:model or a model alone: one [crew] allowed lists (default: crew's own)")
	cmd.Flags().StringArrayVar(&reads, "read", nil, "an artifact the crew should read first (repeatable): its reference, a file, or a URL; ws tells the crew where to read it")
	cmd.Flags().IntVar(&stage, "stage", 0, "the stage of the brief's plan it's for (default: the current one, the first not accepted)")
	cmd.Flags().BoolVar(&queue, "queue", false, "queue it for the user to launch instead of launching now")
	cmd.Flags().BoolVar(&proposeIt, "propose", false, "in a conversation: propose it, for the user to start, instead of starting it")
	revisesFlag(cmd)
	return cmd
}

// readFirstList resolves what a dispatch names with --read into lines for
// it, with where to read each.
func readFirstList(ctx *ops.Workspace, m *mission.Mission, reads []string) (string, error) {
	refs := make([]string, len(reads))
	for i, in := range reads {
		r, err := ctx.NormalizeArtifact(m, in, userDirOr())
		if err != nil {
			return "", fmt.Errorf("--read %s: %w", in, err)
		}
		refs[i] = r.String()
	}
	return ops.WhereToRead(ctx.WS(), missionStore(ctx), m, refs), nil
}

// dispatchLimitErr tells the orchestrator what to do instead.
func dispatchLimitErr(err error) error {
	if errors.Is(err, mission.ErrDispatchLimit) {
		return fmt.Errorf("%w; the user hasn't stepped in since, so don't dispatch more: run ws mission flag \"<what you'd dispatch next, and why>\" instead", err)
	}
	return err
}
