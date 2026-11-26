package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// briefingHangupDelay gives the briefing crew time to show ready's output
// before its session ends.
const briefingHangupDelay = "2"

func newMissionBriefCmd() *cobra.Command {
	var slug, set, title, newSlug string
	var proposeIt bool
	var capsules, docks []string
	cmd := &cobra.Command{
		Use:   "brief",
		Short: "Print a mission's brief, or replace it with --set",
		Long: `Print the mission's brief, or replace it:

  ws mission brief --set - <<'EOF'
  ## Goal
  ...
  EOF

Crew can only replace the brief while briefing a mission. In a
conversation about the mission, its agent proposes a new brief instead
(--propose --set -): the user sees what changes, and accepts it or not.
Briefing a mission, it adds --title, a --capsule for each capsule to lift
and a --dock for existing work to build on (repo/branch, or repo#PR), and
--slug for a better slug: accepting it makes the mission ready.`,
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
			store := missionStore(ctx)
			if set == "" {
				brief, err := store.ReadBrief(m.Slug)
				if err != nil {
					return err
				}
				_, err = os.Stdout.WriteString(brief)
				return err
			}
			if !proposeIt {
				if err := refuseInConversation("an edit to the brief"); err != nil {
					return err
				}
			}
			var data []byte
			if set == "-" {
				data, err = io.ReadAll(os.Stdin)
			} else {
				data, err = os.ReadFile(set)
			}
			if err != nil {
				return fmt.Errorf("reading brief: %w", err)
			}
			if strings.TrimSpace(string(data)) == "" {
				return errors.New("the new brief is empty")
			}
			if proposeIt {
				was, _ := store.ReadBrief(m.Slug)
				return proposeFrom(ctx, func(cs conversation.Store, id string) (conversation.Proposal, error) {
					p := conversation.Proposal{Kind: conversation.ProposeBrief, Brief: string(data), Was: was, Title: title, Capsules: capsules, Docks: docks, Slug: newSlug}
					// Briefing, the plan proposed before it comes with it.
					ctx.WithPlan(cs, id, &p)
					return p, nil
				})
			}
			if title != "" || len(capsules) > 0 || len(docks) > 0 || newSlug != "" {
				return errors.New("--title, --capsule, --dock and --slug are for proposing a brief (--propose)")
			}
			set, err := ctx.SetBrief(m.Slug, string(data), ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s brief for %s written\n", ui.Green.Render("✓"), m.Slug)
			printWarnings(set.Warnings)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&set, "set", "", "replace the brief from a file, or - for stdin")
	cmd.Flags().BoolVar(&proposeIt, "propose", false, "in a conversation: propose the new brief (--set), for the user to accept, instead of writing it")
	revisesFlag(cmd)
	cmd.Flags().StringVar(&title, "title", "", "with --propose: a title for the mission, when briefing it")
	cmd.Flags().StringArrayVar(&capsules, "capsule", nil, "with --propose: a capsule for it, as repo/branch (repeatable), when briefing it: lifted, or joined if it's lifted already")
	cmd.Flags().StringArrayVar(&docks, "dock", nil, "with --propose: existing work to build on, as repo/branch or repo#PR (repeatable), when briefing it")
	cmd.Flags().StringVar(&newSlug, "slug", "", "with --propose: a better slug for the mission, when briefing it")
	return cmd
}

func newMissionTitleCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "title <title>",
		Short: "Set a mission's title: a terse name of at most 50 characters (its slug stays)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			title := strings.Join(args, " ")
			if err := ctx.SetTitle(m.Slug, title, ops.ActorFromEnv()); err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s is now %q\n", ui.Green.Render("✓"), m.Slug, strings.TrimSpace(title))
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

func newMissionReadyCmd() *cobra.Command {
	var slug, newSlug string
	var noCapsules, noPlan bool
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "Finish briefing a mission: its brief is confirmed, and it starts",
		Long: `Finish briefing: checks the brief is written and, run by a briefing crew,
its plan (or, without a plan, that the mission has capsules), then
records the brief confirmed. That's the go-ahead: the plan's first step
is reached at once (its crew start, or it waits for a call). Run by a
briefing crew, with the user in its terminal, it also ends that crew's
session. Agents that suggest a mission (ws mission new) brief it this way
too, without a conversation.

--slug renames the mission (only here: renaming mid-briefing would leave the
briefing crew's WS_MISSION pointing nowhere).`,
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
			if !m.ToBrief() {
				return fmt.Errorf("mission %s isn't being briefed", m.Slug)
			}
			store := missionStore(ctx)
			brief, _ := store.ReadBrief(m.Slug)
			if mission.IsUnwrittenBrief(brief, m.Title) {
				return errors.New("write the brief first: ws mission brief --set - <<'EOF' … EOF")
			}
			v := ops.BuildMissionViews(ctx.WS(), nil, []*mission.Mission{m})[0]
			live := 0
			for _, c := range v.Capsules {
				if c.Exists {
					live++
				}
			}
			if live == 0 && !noCapsules && m.Plan == nil {
				// A plan's steps name their repos, and ws lifts a capsule
				// when each starts; a brief's stages don't.
				return errors.New("create the capsules the work needs first (ws lift <repo> <branch>, or ws dock <repo> <branch|PR>), or write the plan; pass --no-capsules if it really needs none")
			}
			// The planner writes the plan as it briefs: you may make a
			// mission ready without one, and it works from its brief.
			if c, ok := m.CrewByID(os.Getenv(mission.EnvCrew)); ok && c.Role == mission.RoleBriefing && m.Plan == nil && !noPlan {
				if m.PlanErr != nil {
					return fmt.Errorf("the plan doesn't read: %w", m.PlanErr)
				}
				return errors.New("write the plan first: ws mission plan --set - (ws mission plan --format says how); pass --no-plan if the brief's stages should be its route")
			}
			// Warned, not refused: the brief is the user's. Logged too, for
			// them to see after this session ends.
			warnings := ctx.StageWarnings(m, brief)
			if m.Plan != nil {
				warnings = append(warnings, ctx.PlanWarnings(m, m.Plan)...)
			}
			if len(warnings) > 0 {
				printWarnings(warnings)
				store.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Text: "at ready: " + strings.Join(warnings, "; ")})
			}

			// Record this briefing crew's end before it is hung up, so the
			// end reads "briefed" (the first recorded end wins).
			crewID := os.Getenv(mission.EnvCrew)
			pid := 0
			if c, ok := m.CrewByID(crewID); ok && c.Role == mission.RoleBriefing {
				pid = c.PID
				if err := store.EndCrew(m.Slug, crewID, "briefed"); err != nil {
					fmt.Fprintf(ui.Stderr, "  %s couldn't end %s as briefed: %v\n", ui.Orange.Render("⚠"), crewID, err)
				}
			}
			final := m.Slug
			if newSlug != "" && newSlug != m.Slug {
				if err := store.Rename(m.Slug, newSlug); err != nil {
					return fmt.Errorf("renaming the mission failed: %w", err)
				}
				if err := conversation.MoveMission(store.Root, m.Slug, newSlug); err != nil {
					return fmt.Errorf("mission is renamed, but its conversations weren't moved with it: %w", err)
				}
				final = newSlug
			}
			// Confirming it is the go-ahead: it starts. Run by its briefing
			// crew, it's confirmed with you, in its terminal.
			by := ops.AsYou
			if c, ok := m.CrewByID(crewID); ok && c.Role == mission.RoleBriefing {
				by = ops.AsCrew(crewID)
			}
			started, err := ctx.ConfirmBrief(final, by)
			if err != nil {
				return err
			}
			got, _ := store.Load(final)
			title := m.Title
			if got != nil {
				title = got.Title
			}
			noun := "capsules"
			if live == 1 {
				noun = "capsule"
			}
			counted := fmt.Sprintf(" · %d %s", live, noun)
			if live == 0 {
				counted = ""
			}
			fmt.Fprintf(ui.Stderr, "  %s Mission %s is confirmed: %s%s\n", ui.Green.Render("✓"), ui.Bold.Render(final), title, counted)
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Dim.Render("·"), started)
			if pid > 0 {
				fmt.Fprintf(ui.Stderr, "  %s this briefing session ends now; crew can be launched from the mission's page\n", ui.Dim.Render("·"))
				execlog.Note(ctx.WS().Root, "crew "+final, "briefing done: hanging up its session (pid %d) in %s", pid, briefingHangupDelay)
				hangUpLater(pid)
			}
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&newSlug, "slug", "", "rename the mission to this slug")
	cmd.Flags().BoolVar(&noCapsules, "no-capsules", false, "allow a mission with no capsules")
	cmd.Flags().BoolVar(&noPlan, "no-plan", false, "briefing crew: allow a mission with no plan, which works from its brief's stages")
	return cmd
}

// hangUpLater ends a briefing crew's session (its process group) after a
// short delay, from a detached helper so this command can return first and
// the crew sees its output.
func hangUpLater(pid int) {
	script := fmt.Sprintf("sleep %s; kill -HUP -- -%d 2>/dev/null || kill -HUP %d 2>/dev/null", briefingHangupDelay, pid, pid)
	c := exec.Command("/bin/sh", "-c", script) //nolint:noctx // it's to outlive ws: it hangs up later
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if c.Start() == nil {
		_ = c.Process.Release() // it's let go: what it does next isn't ws's
	}
}
