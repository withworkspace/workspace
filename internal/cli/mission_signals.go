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
)

// signallingCrew resolves the mission and crew a signal comes from: crew
// only, since these say something about the crew itself.
func signallingCrew(slug string) (*ops.Workspace, *mission.Mission, mission.Crew, error) {
	ctx, err := loadOps()
	if err != nil {
		return nil, nil, mission.Crew{}, err
	}
	id := os.Getenv(mission.EnvCrew)
	if id == "" {
		return nil, nil, mission.Crew{}, errors.New("this is for crew on a mission (WS_CREW isn't set)")
	}
	m, err := resolveMission(ctx, slug)
	if err != nil {
		return nil, nil, mission.Crew{}, err
	}
	c, ok := m.CrewByID(id)
	if !ok {
		return nil, nil, mission.Crew{}, fmt.Errorf("mission %s has no crew %s", m.Slug, id)
	}
	if c.Role == mission.RoleBriefing {
		return nil, nil, mission.Crew{}, errors.New("you're briefing this mission: finish with ws mission ready")
	}
	return ctx, m, c, nil
}

func newMissionAskCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "ask <question>",
		Short: "Crew: say you need the user's decision (shows in the web UI and notifies)",
		Long: `For crew: flag that you need the user. Ask in your reply as usual, and run
this too, so the web UI marks you as asking and notifies them. It clears when
they next talk to you.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, m, c, err := signallingCrew(slug)
			if err != nil {
				return err
			}
			q := strings.TrimSpace(strings.Join(args, " "))
			if err := missionStore(ctx).SetCrewStatus(m.Slug, c.ID, mission.CrewAsking, q); err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s asked; the user has been notified\n", ui.Orange.Render("?"))
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

func newMissionDoneCmd() *cobra.Command {
	var slug, from string
	cmd := &cobra.Command{
		Use:   "done <summary> --handoff -|<file>",
		Short: "Crew: say you've finished your part, with a summary and a handoff",
		Long: `For crew: report that your part of the mission is finished, with a
one-line summary and a handoff for whoever comes next (the next crew, the
orchestrator and the user), in markdown on stdin (--handoff -) or from a
file, with these four sections:

` + mission.HandoffTemplate + `

Learned matters most: where things live, what you tried that didn't work,
surprises. Keep it short. Each "Read first" entry is a file, a reference
(records:adr/0042.md, api#88) or a URL, and ws checks it can find each.

ws keeps the handoff with the mission (mission:handoffs/<crew>.md) and
gives it to the crew that come next. It finds PRs from the mission's
capsules itself; record other outputs (a proposal or ADR you wrote, notes,
a link) with ws mission artifact add <file|url> --kind <kind> "label".

  ws mission done "Added the per-org token cache" --handoff - <<'EOF'
  ## Done
  ...
  EOF`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, m, c, err := signallingCrew(slug)
			if err != nil {
				return err
			}
			if ctx.Verifying(m, c.ID) {
				return errors.New("you're verifying this step: your done is your bundle, handed over (ws mission bundle --set -; ws mission bundle --format says how it's written)")
			}
			summary := strings.TrimSpace(strings.Join(args, " "))
			// Work crew leave a handoff. Crew answering the orchestrator's
			// question for the user answered already; one is welcome.
			var h *mission.Handoff
			if from == "" && c.Role != mission.RoleAnswer {
				return fmt.Errorf("give a handoff for whoever comes next, in markdown on stdin:\n\n"+
					"  ws mission done %q --handoff - <<'EOF'\n%s\n  EOF", summary, indentLines(mission.HandoffTemplate, "  "))
			}
			if from != "" {
				text, err := readFlagText(from)
				if err != nil {
					return fmt.Errorf("reading the handoff: %w", err)
				}
				if h, err = mission.ParseHandoff(text); err != nil {
					return fmt.Errorf("the handoff: %w\n\nIt takes these four sections:\n\n%s", err, indentLines(mission.HandoffTemplate, "  "))
				}
				if err := ctx.CanonicalReadFirst(m, h, userDirOr()); err != nil {
					return fmt.Errorf("the handoff: %w\n\nName each as a file, a reference (records:adr/0042.md, api#88) or a URL, or drop it", err)
				}
			}
			store := missionStore(ctx)
			if err := store.ReportDone(m.Slug, c.ID, summary, h); err != nil {
				return err
			}
			if next := ctx.CrewDone(m.Slug, c.ID); next != "" {
				fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Dim.Render("·"), next)
			}
			if h != nil {
				fmt.Fprintf(ui.Stderr, "  %s done; handoff kept at %s; the user has been notified\n", ui.Green.Render("✓"), store.HandoffPath(m.Slug, c.ID))
			} else {
				fmt.Fprintf(ui.Stderr, "  %s done; the user has been notified\n", ui.Green.Render("✓"))
			}
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&from, "handoff", "", "the handoff, in markdown: - for stdin, or a file")
	return cmd
}

// readFlagText reads a flag's text: - for stdin, else a file.
func readFlagText(from string) (string, error) {
	var data []byte
	var err error
	if from == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(from)
	}
	return string(data), err
}

// indentLines indents every line of s.
func indentLines(s, indent string) string {
	return indent + strings.ReplaceAll(s, "\n", "\n"+indent)
}

func newMissionArtifactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact",
		Short: "Record what a mission produced or relies on: files, PRs, links",
		Long: `Artifacts point at what a mission produced or relies on, so the next crew
(and the orchestrator) can find it: ws mission show -f llm lists them with
where to read each one. ws finds the capsules' own PRs itself.

What you can record:
  a file path       classified by where it is: one of the mission's capsules
                    (repo/capsule:path, followed onto the default branch when
                    its PR merges), a repo's .ground (repo:path), or the
                    mission's private files folder (mission:path)
  repo:path         a file on a repo's default branch
  repo#N, PR URL    a PR or issue
  https://…         anything else

Kinds say what it is: proposal, adr, note, doc, plan, handoff, pr, issue,
link, or any lowercase word.`,
	}
	var slug, kind string
	add := &cobra.Command{
		Use:   "add <file|ref|url> [label]",
		Short: "Record an artifact (adding it again updates its kind and label)",
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
			if kind != "" && !ops.KindShape.MatchString(kind) {
				return fmt.Errorf("kind %q: use one lowercase word, like proposal, adr, note, doc or link", kind)
			}
			r, err := ctx.NormalizeArtifact(m, args[0], userDirOr())
			if err != nil {
				return err
			}
			if kind == "" {
				kind = ops.DefaultKind(r)
			}
			a := mission.Artifact{Ref: r.String(), Kind: kind, Label: strings.TrimSpace(strings.Join(args[1:], " ")), Crew: os.Getenv(mission.EnvCrew)}
			store := missionStore(ctx)
			if err := store.AddArtifact(m.Slug, a); err != nil {
				return err
			}
			fresh, _ := store.Load(m.Slug)
			v := ops.BuildMissionViewsFrom(ctx.WS(), nil, []*mission.Mission{fresh})[0]
			for _, av := range ops.ResolveArtifacts(ctx.WS(), store, v) {
				if av.Ref == a.Ref {
					fmt.Fprintf(ui.Stderr, "  %s recorded on %s: %s\n", ui.Green.Render("✓"), m.Slug, ops.ArtifactLine(av))
				}
			}
			return nil
		},
	}
	add.Flags().StringVar(&kind, "kind", "", "what it is: proposal, adr, note, doc, plan, handoff, pr, issue, link… (default: pr, link or doc)")
	addMissionFlag(add, &slug)
	rm := &cobra.Command{
		Use:   "rm <ref>",
		Short: "Forget an artifact, by its reference as ws mission show lists it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			if err := missionStore(ctx).RemoveArtifact(m.Slug, strings.TrimSpace(args[0]), os.Getenv(mission.EnvCrew)); err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s removed from %s\n", ui.Green.Render("✓"), m.Slug)
			return nil
		},
	}
	addMissionFlag(rm, &slug)
	cmd.AddCommand(add, rm)
	return cmd
}

// raisedBy is who a proposal comes from: the orchestrator, crew, or the user.
func raisedBy() string {
	if os.Getenv(mission.EnvRole) == mission.RoleOrchestrator {
		return mission.RoleOrchestrator
	}
	if c := os.Getenv(mission.EnvCrew); c != "" {
		return c
	}
	return "you"
}

func newMissionFlagCmd() *cobra.Command {
	var slug string
	var clear bool
	var options []string
	cmd := &cobra.Command{
		Use:   "flag <question for the user>",
		Short: "Mark a mission as needing the user, with a question (and answers to pick from)",
		Long: `Mark a mission as needing the user: the web UI shows the question under
Needs you, with any --option answers to pick from, and the user answers it
there or with ws mission answer. The orchestrator then runs again with
their answer and acts on it. Give each option as a short instruction you'd
act on ("Commit both and open PRs"); the user can always write their own.

The orchestrator's question at a Work step whose checks came round not all
met is that step's own: ws adds "Pass it" and "Send it back" to its
answers, so the user can pass the step as it is from the question, and
approving the bundle that shows its checks passes it too.` + fmt.Sprintf(onStdin, "flag"),
		Args: func(cmd *cobra.Command, args []string) error {
			if !clear && len(args) == 0 {
				return errors.New("say why the user is needed")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			if clear {
				d, err := ctx.Dismiss(m.Slug, ops.ActorFromEnv())
				if err != nil {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s %s on %s\n", ui.Green.Render("✓"), strings.TrimSuffix(d.String(), "."), m.Slug)
				return nil
			}
			question, err := wordsOf(args)
			if err != nil {
				return err
			}
			f, err := ctx.Flag(m.Slug, question, ops.ActorFromEnv(), options...)
			if err != nil {
				return err
			}
			switch {
			case f.Passed != "":
				// The user had approved what shows the step's checks: there
				// was nothing left to ask.
				fmt.Fprintf(ui.Stderr, "  %s nothing to ask on %s: %s\n", ui.Green.Render("✓"), m.Slug, f.Passed)
			case f.AtStep > 0:
				fmt.Fprintf(ui.Stderr, "  %s %s asks the user (task %d), as step %d's own: its checks aren't all met, so they can pass it as it is from your question, or send it back. It waits on them: there's nothing more for you to do\n", ui.Orange.Render("!"), m.Slug, f.Task.N, f.AtStep)
			default:
				fmt.Fprintf(ui.Stderr, "  %s %s asks the user (task %d)\n", ui.Orange.Render("!"), m.Slug, f.Task.N)
			}
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().BoolVar(&clear, "clear", false, "remove the flag")
	cmd.Flags().StringArrayVar(&options, "option", nil, "an answer the user can pick (repeatable)")
	return cmd
}

func newMissionAnswerCmd() *cobra.Command {
	var slug string
	var task int
	cmd := &cobra.Command{
		Use:   "answer <your answer>",
		Short: "Answer what a mission waits on you for: its oldest task, or --task n",
		Long: `Answer a task a mission waits on you for: the oldest, or the one --task
numbers (ws mission show lists them; the mission's page in the web UI
answers them too). A question the orchestrator asked: your answer goes in
the log as a decision, and the orchestrator runs again with it. A step of
yours: "Go on" passes it, "Not yet" leaves it yours, and anything else
goes to the orchestrator, the step staying yours; a Decide step's answer
is the decision, and passes it. A Wait on you, something you do yourself:
"Done" marks it done, alone or with what you did after it ("Done: tagged
v1.4.0"), which is kept as a decision for the crew after; where the step
has a check, ws looks first, and it stays yours until the look shows it,
or you answer "Pass it". Crew you asked to answer a question (Ask an
agent) answer the same way, and that's their part done.`,
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
			a, err := ctx.AnswerTask(m.Slug, task, ops.ActorFromEnv(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s answered on %s; %s\n", ui.Green.Render("✓"), m.Slug, a.Next)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().IntVar(&task, "task", 0, "the task to answer, by its number (default: the oldest)")
	return cmd
}

// wordsOf is what a command was told, in its arguments: joined, or, where
// they're a lone "-", read from stdin. A message with a backtick, a
// dollar or a quote in it (a command, a check's words) is otherwise the
// shell's to read first: inside double quotes it runs what's in the
// backticks and leaves what that printed in their place.
func wordsOf(args []string) (string, error) {
	if len(args) == 1 && args[0] == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading it from stdin: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return strings.TrimSpace(strings.Join(args, " ")), nil
}

// onStdin is how a command's help says so.
const onStdin = `

Give it as - to have it read from stdin, where it has a backtick, a $ or
a quote in it, which a shell would read as its own:

  ws mission %s - <<'EOF'
  …
  EOF`

func newMissionResumeWithCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "resume-with <crew> <message>",
		Short: "Have crew carry on, with a message (typed in now if it's running)",
		Long: `Have crew carry on with its work, with a message saying what's left ("the
error path has no test").

Crew still running get the message typed straight in; for the
orchestrator that counts toward [orchestrator] max_dispatches, as
dispatching does. Crew that reported done and were closed (ws closes them
once their turn is over) are resumed with it, and so count too; with
[orchestrator] dispatch = "ask", or for crew that ended otherwise, the
mission's page offers to resume them, which types the message in. Only crew that
recorded a conversation can be resumed; for anything else, raise a
follow-up with ws mission new.` + fmt.Sprintf(onStdin, "resume-with c2"),
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			c, ok := m.CrewByID(args[0])
			if !ok {
				var ids []string
				for _, c := range m.Crew {
					ids = append(ids, c.ID)
				}
				return fmt.Errorf("mission %s has no crew %s (it has: %s)", m.Slug, args[0], strings.Join(ids, ", "))
			}
			if c.Role == mission.RoleBriefing {
				return fmt.Errorf("%s briefed the mission; resume work crew, or raise a follow-up with ws mission new", c.ID)
			}
			if c.Conversation == "" {
				return fmt.Errorf("%s recorded no conversation, so it can't be resumed; raise a follow-up with ws mission new instead", c.ID)
			}
			msg, err := wordsOf(args[1:])
			if err != nil {
				return err
			}
			if msg == "" {
				return errors.New("say what's left for it to do")
			}
			store := missionStore(ctx)
			actor := ops.ActorFromEnv()
			// Crew closed after reporting done (ws closes them once their
			// turn's over) are resumed with it, as a failing check resumes
			// them, unless crew wait for the user to start them.
			handedBack := c.Ended != nil && c.Status == mission.CrewDone &&
				(actor.IsYou() || ctx.Config().OrchestratorAutoDispatch())
			if c.Ended == nil || handedBack {
				by := raisedBy()
				if by == mission.RoleOrchestrator {
					if err := store.CountDispatch(m.Slug, ctx.Config().OrchestratorMaxDispatches()); err != nil {
						return dispatchLimitErr(err)
					}
				}
				_, err := ctx.Say(m.Slug, c.ID, msg, actor)
				switch {
				case err == nil && c.Ended == nil:
					fmt.Fprintf(ui.Stderr, "  %s typed it into %s, which is still running\n", ui.Green.Render("✓"), c.ID)
					return nil
				case err == nil:
					fmt.Fprintf(ui.Stderr, "  %s resumed %s, which had handed back its work, with it\n", ui.Green.Render("✓"), c.ID)
					return nil
				}
				if by == mission.RoleOrchestrator {
					store.ReleaseDispatch(m.Slug)
				}
				how := "type it into"
				if c.Ended != nil {
					how = "resume"
				}
				fmt.Fprintf(ui.Stderr, "  %s couldn't %s %s (%v): the user is asked to pass it on\n", ui.Dim.Render("·"), how, c.ID, err)
			}
			if err := store.SetResumeWithBy(m.Slug, c.ID, msg, ops.ActorFromEnv().By()); err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s the mission's page offers to resume %s with that message\n", ui.Green.Render("✓"), c.ID)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

func newMissionConcludeCmd() *cobra.Command {
	var slug string
	var as endAsFlags
	cmd := &cobra.Command{
		Use:   "conclude <what it came to>",
		Short: "Suggest how the mission ends, with Z checked (the user ends it)",
		Long: `Suggest how the mission ends, for the user to end it (ws mission end, or
on its page): what it came to, how it ends, and each part of Z checked.

  ws mission conclude "Cache shipped and documented" --as landed \
    --met "1: go test ./... passes in api/cache-lookup" \
    --met "2: docs PR #14 merged"

  ws mission conclude "Not a bug: the total is rounded as product asked" \
    --as answered --met "1: answered in the handoff" --unmet "2: no fix needed"

--as is how it ends: landed (at Z, its code merged or landed), answered (a
finding and no code, at Z or stopping early as the plan allows), handed-on
(its work raised as other missions: --to each, or hand its capsules on
first with ws mission hand-on) or dropped.

Z is the plan's (ws show lists it), or without a plan, the brief's
## Acceptance. Each part is checked, by its number: --met "<n>: <how>"
(what shows it) or --unmet "<n>: <why not>". ws doesn't judge whether a
part is met; it refuses a conclusion that doesn't check each one (dropping
needn't), and landed with a part unmet or work not merged.`,
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
			c, err := ctx.Conclude(m.Slug, as.endAs(args), ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s suggested ending %s, %s; the user ends it\n", ui.Green.Render("✓"), m.Slug, c.Conclusion.Summary())
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	as.add(cmd, true)
	return cmd
}

// endAsFlags are how a mission ends, as said on the command line.
type endAsFlags struct {
	ending     string
	met, unmet []string
	to         []string
}

func (f *endAsFlags) add(cmd *cobra.Command, required bool) {
	cmd.Flags().StringVar(&f.ending, "as", "", "how it ends: landed, answered, handed-on or dropped")
	cmd.Flags().StringArrayVar(&f.met, "met", nil, `a part of Z that's met, and how: "<n>: <how>" (repeatable)`)
	cmd.Flags().StringArrayVar(&f.unmet, "unmet", nil, `a part of Z that isn't met, and why: "<n>: <why not>" (repeatable)`)
	cmd.Flags().StringArrayVar(&f.to, "to", nil, "handed-on: a mission its work goes to (repeatable)")
	if required {
		cmd.MarkFlagRequired("as")
	}
}

func (f *endAsFlags) given() bool {
	return f.ending != "" || len(f.met)+len(f.unmet)+len(f.to) > 0
}

func (f *endAsFlags) endAs(args []string) ops.EndAs {
	return ops.EndAs{Ending: f.ending, Text: strings.Join(args, " "), Met: f.met, Unmet: f.unmet, To: f.to}
}

func newMissionEndCmd() *cobra.Command {
	var slug string
	var as endAsFlags
	cmd := &cobra.Command{
		Use:   "end [what it came to]",
		Short: "End the mission: as suggested, or as you say (the user's)",
		Long: `End the mission. Without --as, it ends as the orchestrator (or crew)
suggested with ws mission conclude. With --as, it ends as you say, with
each part of Z checked as conclude does it:

  ws mission end --as dropped "Not worth it now"
  ws mission end --as answered "Not a bug" --met "1: …" --unmet "2: …"

An ended mission is in the past, out of what needs you; archive it to put
it out of sight. ws mission reopen takes the ending back.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			var said *ops.EndAs
			if as.given() || len(args) > 0 {
				e := as.endAs(args)
				said = &e
			}
			c, err := ctx.End(m.Slug, said, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s ended, %s\n", ui.Green.Render("✓"), m.Slug, c.Conclusion.Summary())
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	as.add(cmd, false)
	return cmd
}

func newMissionHandOnCmd() *cobra.Command {
	var slug, to string
	cmd := &cobra.Command{
		Use:   "hand-on <repo>/<capsule> <what the mission it goes to is for>",
		Short: "Hand a capsule on to a mission of its own, which takes it",
		Long: `Hand one of the mission's capsules on: a mission is raised for it from
what you say (it waits to be briefed), or --to names one there is, and
that mission takes the capsule. It ends alone here: it's no longer this
mission's, and doesn't hold it open. For a capsule whose work belongs in a
mission of its own, as a plan may say for a repo that fails.

The orchestrator's and the user's. To end the whole mission handed on:
ws mission conclude --as handed-on.`,
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
			e, err := ctx.HandOnCapsule(m.Slug, args[0], strings.Join(args[1:], " "), to, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), e)
			if e.Raised {
				fmt.Println(e.To)
			}
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&to, "to", "", "a mission there is, to hand it on to (rather than raising one)")
	return cmd
}

func newMissionDropCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "drop <repo>/<capsule> <why>",
		Short: "Drop one of the mission's capsules: it ends alone (the user's)",
		Long: `Drop one of the mission's capsules: its work won't be part of the
mission, and it no longer holds it open. Its worktree is kept, in no
mission (ws burn removes it). To drop the whole mission: ws mission end
--as dropped.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			e, err := ctx.DropCapsule(m.Slug, args[0], strings.Join(args[1:], " "), ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), e)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

func newMissionWaitCmd() *cobra.Command {
	var slug string
	var clear bool
	cmd := &cobra.Command{
		Use:   "wait <what it waits on>",
		Short: "Say the mission waits on something outside it, that ws can't see",
		Long: `Say the mission waits on something outside it that ws can't see for
itself: a teammate, another team's deploy, a PR in a repo outside the
workspace. Its headline becomes "Waiting on <what>", in nobody's court
here: nothing for the user to do until it happens.

For the orchestrator, when nothing else should happen yet. Not for what
ws already sees (a PR waiting for review, crew at work, the user's
re-review), and not for something the user must do: flag them instead.
Nor at a Wait step of the plan, which says what it waits on itself, and
how it's passed. Anything happening on the mission clears it: crew
starting, the user acting, the orchestrator's next run.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if clear {
				return cobra.NoArgs(cmd, args)
			}
			return cobra.MinimumNArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			text := strings.TrimSpace(strings.Join(args, " "))
			if err := ctx.SayWaiting(m.Slug, text, ops.ActorFromEnv()); err != nil {
				return err
			}
			if clear {
				fmt.Fprintf(ui.Stderr, "  %s %s no longer waits\n", ui.Green.Render("✓"), m.Slug)
				return nil
			}
			fmt.Fprintf(ui.Stderr, "  %s %s waits on %s\n", ui.Green.Render("✓"), m.Slug, text)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().BoolVar(&clear, "clear", false, "it no longer waits")
	return cmd
}
