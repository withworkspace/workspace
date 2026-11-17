package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// briefStage tells new work crew which stage of the brief's plan they're
// for: stage n, or for 0 the current one (the first not accepted). A brief
// without stages leaves it out.
func briefStage(b *harness.Brief, m *mission.Mission, brief string, n int) error {
	states, _ := m.StageProgress(brief)
	st, err := mission.PickStage(states, n)
	if err != nil {
		return err
	}
	if !st.Implicit {
		b.Stage, b.StageN, b.StageOf, b.Steps = st.Whole(), st.N, len(states), st.Step != nil
	}
	return nil
}

func newMissionAcceptCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "accept [why]",
		Short: "Accept the mission's current stage, or its plan's step: its work is complete and good enough",
		Long: `Accept the current stage of the brief's plan (the first not yet accepted),
or of a mission with a plan, its current step: its crew's work is complete
and good enough to build on. The next becomes the current one, and new
crew work on it.

For a mission with a plan, ws reaches the next step by who's on it: it
starts crew on a step that's crew's; asks you at a step that's yours (a
checkpoint: only you pass it, by going on or accepting it yourself), and
at the planner's; and waits on CI and anyone outside, whose step the
orchestrator accepts once it's happened.

ws then moves the plan on: it starts the next stage in the capsule the
stage names (or the mission's only one), with the stage's text, unless the
accepted stage says to check in, when it asks the user first.

For the orchestrator, and you. It's refused while the stage's crew are
still working or asking you something, while a review of one of its
capsules asks for changes (the user re-reviews, and approves, first), and
while a change the user asked for on one of its documents is open (they
resolve the thread first). A brief without stages is one stage, the whole
brief.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			acc, err := ctx.AcceptStage(m.Slug, ops.ActorFromEnv(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			if acc.Implicit {
				fmt.Fprintf(ui.Stderr, "  %s accepted the work on %s\n", ui.Green.Render("✓"), m.Slug)
				return nil
			}
			fmt.Fprintf(ui.Stderr, "  %s accepted %s %d of %d: %s\n", ui.Green.Render("✓"), acc.Stage.Noun(), acc.Stage.N, acc.Of, acc.Stage.Title())
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Dim.Render("·"), acc.Next)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

// stageLines describes the brief's stages and their progress in words, one
// line each, for agents and plain output. A brief without stages has none,
// unless its work has been accepted.
func stageLines(m *mission.Mission, brief string) []string {
	states, orphans := m.StageProgress(brief)
	var out []string
	for _, st := range states {
		if st.Implicit && !st.Accepted() {
			return nil
		}
		mark := "to do"
		switch {
		case st.Skipped():
			mark = "skipped"
		case st.Accepted():
			mark = "accepted"
		case st.Current && st.Held():
			mark = "current, waiting for your call"
		case st.Current:
			mark = "current"
		}
		line := fmt.Sprintf("%d. [%s] %s", st.N, mark, oneLine(st.Text))
		if st.Implicit {
			line = "[accepted] the whole brief"
		}
		var facts []string
		if st.Step != nil {
			facts = append(facts, st.Step.Type+", "+st.Step.WhoSays())
			if st.Step.Makes != "" {
				facts = append(facts, "makes: "+oneLine(st.Step.Makes))
			}
			if st.Step.From != "" {
				facts = append(facts, "options: "+oneLine(st.Step.From))
			}
			if st.Step.OnlyIf != "" {
				facts = append(facts, "only if "+oneLine(st.Step.OnlyIf))
			}
			if st.Step.InEach != "" {
				facts = append(facts, "in each "+oneLine(st.Step.InEach))
			}
			if st.Step.CarryOn > 0 {
				facts = append(facts, fmt.Sprintf("carrying on from step %d", st.Step.CarryOn))
			}
			if st.Step.LimitText != "" {
				facts = append(facts, "limit: "+oneLine(st.Step.LimitText))
			}
			if n := len(st.Step.Checks); n > 0 {
				var checks []string
				for _, c := range st.Step.Checks {
					checks = append(checks, oneLine(c.Text))
				}
				facts = append(facts, "checks: "+strings.Join(checks, "; "))
			}
		}
		if r := st.Record; r != nil && !st.Accepted() {
			if r.OutOfTime != nil {
				facts = append(facts, "out of time")
			} else if r.Deadline != nil {
				facts = append(facts, "time runs out at "+r.Deadline.Local().Format("15:04"))
			}
		}
		if ins := st.Instances(); len(ins) > 0 {
			var runs []string
			for _, in := range ins {
				r := in.Repo
				switch {
				case in.Done != nil:
					r += " done"
				case in.Capsule != "":
					r += " in " + in.Capsule
				}
				runs = append(runs, r)
			}
			facts = append(facts, "runs in "+strings.Join(runs, ", "))
		}
		if r := st.Record; r != nil && r.Attempts > 0 {
			facts = append(facts, fmt.Sprintf("attempt %d", r.Attempts+1))
		}
		if round, ok := st.Record.LatestChecks(); ok {
			facts = append(facts, fmt.Sprintf("checks round %d (%s's done): %s", round.N, round.Crew, round.Summary()))
		}
		if st.CheckIn {
			facts = append(facts, "check in with the user after it")
		}
		if st.Record != nil && len(st.Record.Crew) > 0 {
			facts = append(facts, "crew "+strings.Join(st.Record.Crew, ", "))
		}
		if st.Accepted() {
			a := "accepted by " + st.Record.AcceptedBy
			if st.Skipped() {
				a = "skipped by " + st.Record.AcceptedBy
			}
			if st.Record.Why != "" {
				a += ": " + st.Record.Why
			}
			facts = append(facts, a)
		}
		if len(facts) > 0 {
			line += " (" + strings.Join(facts, "; ") + ")"
		}
		out = append(out, line)
	}
	where := "the brief"
	if m.Plan != nil {
		where = "the plan"
	}
	for _, o := range orphans {
		line := "no longer in " + where + ": " + oneLine(o.Text)
		if o.Accepted != nil {
			line += " (accepted)"
		}
		out = append(out, line)
	}
	return out
}

// oneLine joins a stage's lines.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func printWarnings(warnings []string) {
	for _, w := range warnings {
		fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Orange.Render("!"), w)
	}
}
