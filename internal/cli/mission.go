package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/ui"
)

func newMissionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mission",
		Aliases: []string{"missions"},
		Short:   "Track units of work and the capsules they own",
		Long: `A mission is a unit of work the workspace owns: a brief, the capsules
(possibly across repos) that carry it out, and a log of what happened.

Commands that take a mission use, in order: the --mission flag or slug
argument, $WS_MISSION, then the mission owning the capsule you stand in.

With WS_MISSION set, ws lift and ws dock attach new capsules to the mission.`,
		// Usage helps with a mistyped flag or argument, not a command that
		// failed while running: agents read these errors to correct
		// themselves, and the usage text buries them.
		PersistentPreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMissionList(cmd, "", false)
		},
	}
	cmd.AddCommand(newMissionNewCmd())
	cmd.AddCommand(newMissionListCmd())
	cmd.AddCommand(newMissionShowCmd())
	cmd.AddCommand(newMissionNoteCmd())
	cmd.AddCommand(newMissionDecideCmd())
	cmd.AddCommand(newMissionBriefCmd())
	cmd.AddCommand(newMissionPlanCmd())
	cmd.AddCommand(newMissionReplanCmd())
	cmd.AddCommand(newMissionTitleCmd())
	cmd.AddCommand(newMissionReadyCmd())
	cmd.AddCommand(newMissionAskCmd())
	cmd.AddCommand(newMissionDoneCmd())
	cmd.AddCommand(newMissionArtifactCmd())
	cmd.AddCommand(newMissionBundleCmd())
	cmd.AddCommand(newMissionAnswerCmd())
	cmd.AddCommand(newMissionHookCmd())
	cmd.AddCommand(newMissionFlagCmd())
	cmd.AddCommand(newMissionResumeWithCmd())
	cmd.AddCommand(newMissionLimitCmd())
	cmd.AddCommand(newMissionPRReadyCmd(), newMissionReplyCmd())
	cmd.AddCommand(newMissionStartCmd(), newMissionSkipCmd(), newMissionRepeatCmd())
	cmd.AddCommand(newMissionConcludeCmd(), newMissionEndCmd(), newMissionWaitCmd())
	cmd.AddCommand(newMissionHandOnCmd(), newMissionDropCmd(), newMissionOpenPRCmd())
	cmd.AddCommand(newMissionAcceptCmd(), newMissionLookCmd())
	cmd.AddCommand(newMissionOrchestrateCmd(), newMissionDispatchCmd())
	cmd.AddCommand(newMissionAttachCmd(), newMissionAboutCmd())
	cmd.AddCommand(newMissionDetachCmd())
	cmd.AddCommand(newMissionStateCmd("park", mission.Parked, "Park a mission (paused work)"))
	cmd.AddCommand(newMissionArchiveCmd())
	cmd.AddCommand(newMissionStateCmd("reopen", "", "Clear a parked or archived state, or else how it ended"))
	return cmd
}

func completeMissionSlugs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, err := loadOps()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	missions, _ := missionStore(ctx).List()
	var out []string
	for _, m := range missions {
		out = append(out, m.Slug+"\t"+m.Title)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// --- new ---

func newMissionNewCmd() *cobra.Command {
	var brief, briefFile string
	var noEdit, edit, proposeIt, distinct bool
	var capsules, about []string
	cmd := &cobra.Command{
		Use:   "new <what the mission is>",
		Short: "Create a mission: say what it is, and a briefing agent sets it up",
		Long: `Create a mission from a plain description, e.g.

  ws mission new "fe issue 45"
  ws mission new "the token refresh test is flaky: the clock should be injected"

It waits to be briefed: Brief it on its page in the web UI (ws web) and an
agent writes the brief and creates the capsules with you. Run by crew inside another
mission, the new mission records where it came from.

A mission that's about something ws names (an issue a plugin has, a
pull request) says so with --about RAM-123: whoever reads the mission is
told to read it, and it's found from the thing. ws mission about adds one
later.

To create a mission ready straight away, with a brief you write yourself:
--brief "...", --brief-file <path|->, --edit (opens $EDITOR), or --no-edit
(the template brief). The arguments are then the title.

In a conversation, its agent proposes one instead (--propose): an idea to
brief later, or with --brief or --brief-file, ready, with --capsule for
each capsule to lift, and --about for what it's about. The user creates
it, or not.

Raised from inside a mission (by crew, or the orchestrator), one that looks
like a mission in play is refused, naming it: note what you found there
(ws mission note -m <mission>), or, if it's different work, say so with
--distinct.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if brief != "" && briefFile != "" {
				return errors.New("use either --brief or --brief-file, not both")
			}
			text := strings.Join(args, " ")
			store := missionStore(ctx)
			if briefFile == "-" && proposeIt {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading brief: %w", err)
				}
				brief, briefFile = string(data), ""
			}
			if proposeIt {
				if briefFile != "" {
					data, err := os.ReadFile(briefFile)
					if err != nil {
						return fmt.Errorf("reading brief: %w", err)
					}
					brief = string(data)
				}
				// What it's about is read now, so a name ws reads
				// nothing by is said to the agent, not to the user
				// accepting it.
				if about, err = ctx.AboutNames(about); err != nil {
					return err
				}
				return propose(ctx, conversation.Proposal{Kind: conversation.ProposeMission, Title: text, Brief: brief, Capsules: capsules, About: about})
			}
			if len(capsules) > 0 {
				return errors.New("--capsule is for proposing a mission: lift its capsules with ws lift once it's created")
			}
			if err := refuseInConversation("a mission"); err != nil {
				return err
			}
			if os.Getenv(mission.EnvMission) != "" && !distinct {
				if err := refuseAlike(store, text); err != nil {
					return err
				}
			}
			// What it's about is read before it's made: a name ws reads
			// nothing by makes no mission.
			if about, err = ctx.AboutNames(about); err != nil {
				return err
			}
			said := func(slug string) error {
				if len(about) == 0 {
					return nil
				}
				_, err := ctx.SetAbout(slug, about, nil)
				return err
			}

			if brief == "" && briefFile == "" && !edit && !noEdit {
				m, err := store.CreateBriefing(text)
				if err != nil {
					return err
				}
				noteOrigin(store, m.Slug, text)
				if err := said(m.Slug); err != nil {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s Mission %s is waiting to be briefed: Brief it on its page in the web UI (ws web)\n", ui.Green.Render("✓"), ui.Bold.Render(m.Slug))
				fmt.Println(m.Slug)
				return nil
			}

			body := brief
			if briefFile != "" {
				var data []byte
				if briefFile == "-" {
					data, err = io.ReadAll(os.Stdin)
				} else {
					data, err = os.ReadFile(briefFile)
				}
				if err != nil {
					return fmt.Errorf("reading brief: %w", err)
				}
				body = string(data)
			}
			m, err := store.Create(text)
			if err != nil {
				return err
			}
			noteOrigin(store, m.Slug, text)
			if err := said(m.Slug); err != nil {
				return err
			}
			if body != "" {
				if err := store.WriteBrief(m.Slug, body); err != nil {
					return err
				}
			} else if edit && ui.IsInteractive() {
				if err := openEditor(store.BriefPath(m.Slug)); err != nil {
					fmt.Fprintf(ui.Stderr, "  %s editor: %v\n", ui.Orange.Render("⚠"), err)
				}
			}
			fmt.Fprintf(ui.Stderr, "  %s Mission %s created\n", ui.Green.Render("✓"), ui.Bold.Render(m.Slug))
			fmt.Fprintf(ui.Stderr, "  %s brief: %s\n", ui.Dim.Render("·"), store.BriefPath(m.Slug))
			fmt.Println(m.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&brief, "brief", "", "create it ready with this brief (markdown)")
	cmd.Flags().StringVar(&briefFile, "brief-file", "", "create it ready with the brief from a file, or - for stdin")
	cmd.Flags().BoolVar(&edit, "edit", false, "create it ready and write the brief in $EDITOR")
	cmd.Flags().BoolVar(&noEdit, "no-edit", false, "create it ready with the template brief")
	cmd.Flags().BoolVar(&proposeIt, "propose", false, "in a conversation: propose it, for the user to create, instead of creating it")
	revisesFlag(cmd)
	cmd.Flags().StringArrayVar(&capsules, "capsule", nil, "with --propose: a capsule for it, as repo/branch (repeatable): lifted, or joined if it's lifted already")
	cmd.Flags().StringArrayVar(&about, "about", nil, "what it's about, by a name ws reads: one of a plugin's things (RAM-123), a pull request (repeatable)")
	cmd.Flags().BoolVar(&distinct, "distinct", false, "from inside a mission: raise it though it looks like a mission in play, as it's different work")
	return cmd
}

// refuseAlike refuses raising a mission that looks like one in play that
// hasn't ended (mission.Alike), naming each: crew on different missions
// notice the same things, and a duplicate is one more for the user to
// brief or drop.
func refuseAlike(store mission.Store, text string) error {
	missions, err := store.InPlay()
	if err != nil {
		return nil // the check is a help: not being able to make it stops nothing
	}
	var like []*mission.Mission
	var named []string
	for _, m := range missions {
		if m.Ending == nil && mission.Alike(text, m.Says()) {
			like = append(like, m)
			named = append(named, fmt.Sprintf("%s (%s)", m.Slug, m.Title))
		}
	}
	if len(like) == 0 {
		return nil
	}
	return fmt.Errorf("this looks like %s, in play: if it's the same work, note what you found there instead (ws mission note -m %s \"<what you found>\"); if it's different work, run this again with --distinct",
		strings.Join(named, " and "), like[0].Slug)
}

// noteOrigin links a mission created by crew to the mission it came from:
// the new one records From, the parent's log says what was raised.
func noteOrigin(store mission.Store, slug, text string) {
	parent, crew := os.Getenv(mission.EnvMission), os.Getenv(mission.EnvCrew)
	if parent == "" || parent == slug {
		return
	}
	if _, err := store.Load(parent); err != nil {
		return
	}
	from := parent
	if crew != "" {
		from += "/" + crew
	}
	store.Update(slug, func(m *mission.Mission) error {
		m.From = from
		return nil
	})
	store.AppendLog(parent, mission.Event{Kind: mission.EventNote, Text: "raised mission " + slug + ": " + text})
}

func openEditor(path string) error {
	c := editorCmd(path)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stderr, os.Stderr
	return c.Run()
}

// editorCmd builds a command running $EDITOR (which may carry arguments, e.g.
// "code -w") on path, defaulting to vim.
func editorCmd(path string) *exec.Cmd {
	editor := strings.Fields(os.Getenv("EDITOR"))
	if len(editor) == 0 {
		editor = []string{"vim"}
	}
	return exec.Command(editor[0], append(editor[1:], path)...) //nolint:noctx // the person editing ends it
}

// --- list ---

func newMissionListCmd() *cobra.Command {
	var format string
	var all bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List missions",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMissionList(cmd, format, all)
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "", "output format: llm")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include archived missions")
	return cmd
}

func runMissionList(cmd *cobra.Command, format string, all bool) error {
	ctx, err := loadOps()
	if err != nil {
		return err
	}
	missions, err := missionStore(ctx).List()
	if err != nil {
		return err
	}
	views := ops.BuildMissionViews(ctx.WS(), ctx.GitHub(), missions)
	shown := views[:0:0]
	hidden := 0
	for _, v := range views {
		if v.State == mission.Archived && !all {
			hidden++
			continue
		}
		shown = append(shown, v)
	}

	if format == "llm" {
		var b strings.Builder
		if len(shown) == 0 {
			b.WriteString("No missions.\n")
		}
		for _, v := range shown {
			fmt.Fprintf(&b, "%s [%s] %s", v.M.Slug, v.State, v.M.Title)
			if names := capsuleNames(v); names != "" {
				fmt.Fprintf(&b, " — %s", names)
			}
			brief, _ := missionStore(ctx).ReadBrief(v.M.Slug)
			if at := (mission.Snapshot{M: v.M, State: v.State, Brief: brief}).Position(); at != "" {
				fmt.Fprintf(&b, " · at %s", at)
			}
			b.WriteByte('\n')
		}
		_, err := os.Stdout.WriteString(b.String())
		return err
	} else if format != "" {
		return fmt.Errorf("unknown format %q (want llm)", format)
	}

	if len(shown) == 0 {
		fmt.Fprintf(ui.Stderr, "  No missions. Create one with %s\n", ui.Bold.Render(`ws mission new "<title>"`))
		if hidden > 0 {
			fmt.Fprintf(ui.Stderr, "  %s %d archived (ws mission list --all)\n", ui.Dim.Render("·"), hidden)
		}
		return nil
	}

	slugW, stateW := 0, 0
	for _, v := range shown {
		slugW = max(slugW, lipgloss.Width(v.M.Slug))
		stateW = max(stateW, len(v.State))
	}
	for _, v := range shown {
		line := fmt.Sprintf("  %-*s  %s  %s", slugW, v.M.Slug,
			stateStyle(v.State).Render(fmt.Sprintf("%-*s", stateW, v.State)), v.M.Title)
		if names := capsuleNames(v); names != "" {
			line += "  " + ui.Dim.Render(names)
		}
		fmt.Fprintln(ui.Stderr, line)
	}
	if hidden > 0 {
		fmt.Fprintf(ui.Stderr, "  %s %d archived (ws mission list --all)\n", ui.Dim.Render("·"), hidden)
	}
	return nil
}

func capsuleNames(v ops.MissionView) string {
	names := make([]string, len(v.Capsules))
	for i, c := range v.Capsules {
		names[i] = c.Repo + "/" + c.Name
	}
	return strings.Join(names, ", ")
}

func stateStyle(s mission.State) lipgloss.Style {
	switch s {
	case mission.Active:
		return ui.Blue
	case mission.Review:
		return ui.Orange
	case mission.Landed:
		return ui.Green
	case mission.Briefing:
		return ui.Orange
	default:
		return ui.Dim
	}
}

// --- show ---

const missionShowLogLimit = 10

// missionShowDecisions is how many decisions the mission's view lists.
const missionShowDecisions = 12

func newMissionShowCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:               "show [mission]",
		Short:             "Show a mission's brief, capsules and recent log",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeMissionSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, firstArg(args))
			if err != nil {
				return err
			}
			store := missionStore(ctx)
			v := ops.BuildMissionViews(ctx.WS(), ctx.GitHub(), []*mission.Mission{m})[0]
			brief, _ := store.ReadBrief(m.Slug)
			events, _ := store.ReadLog(m.Slug, missionShowLogLimit)

			switch format {
			case "llm":
				_, err := os.Stdout.WriteString(formatMissionLLM(ctx, v, store.BriefPath(m.Slug), brief, events))
				return err
			case "":
				printMissionText(ctx, v, store.BriefPath(m.Slug), brief, events)
				return nil
			default:
				return fmt.Errorf("unknown format %q (want llm)", format)
			}
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "", "output format: llm")
	return cmd
}

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// skillsListed is how many skills a mission's view lists with what each is
// for: more are named, and ws show skills has the rest.
const skillsListed = 8

func capsuleFacts(c ops.CapsuleView) string {
	var parts []string
	if !c.Exists {
		parts = append(parts, "burned")
	} else if c.Branch != "" && c.Branch != c.Name {
		parts = append(parts, "branch "+c.Branch)
	}
	if c.PR != nil {
		state := "open"
		if c.PRS == mission.PRMerged {
			state = "merged"
		}
		parts = append(parts, fmt.Sprintf("PR #%d %s", c.PR.Number, state))
	}
	switch {
	case c.Landed != nil:
		parts = append(parts, "landed on the default branch "+c.Landed.At.Format("2 Jan 15:04"))
	case c.Direct && c.Exists:
		// No pull request stands between its commits and everyone else: the
		// user is the only gate, and says when.
		parts = append(parts, fmt.Sprintf("no pull request: its commits land straight on the default branch when the user says (ws land is theirs to run). Commit here, don't push, and report done; %s to land so far", commitsWord(c.Ahead)))
	}
	return strings.Join(parts, ", ")
}

func formatMissionLLM(ctx *ops.Workspace, v ops.MissionView, briefPath, brief string, events []mission.Event) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Mission: %s\n", v.M.Slug)
	fmt.Fprintf(&b, "Title: %s\n", v.M.Title)
	fmt.Fprintf(&b, "State: %s\n", v.State)
	snap := missionSnapshot(ctx, v, brief, events)
	if at := snap.Position(); at != "" {
		fmt.Fprintf(&b, "At: %s\n", at)
	}
	h := mission.Resolve(snap)[0]
	fmt.Fprintf(&b, "Where it stands: %s: %s (what moves it on: %s)\n", courtWords(h.Court, true), h.Text, h.Action.Label)
	if v.M.Request != "" {
		fmt.Fprintf(&b, "Requested as: %s\n", strings.ReplaceAll(v.M.Request, "\n", " "))
	}
	if v.M.From != "" {
		fmt.Fprintf(&b, "Raised from: %s\n", v.M.From)
	}
	for _, name := range v.M.About {
		fmt.Fprintf(&b, "About: %s (ws show %s says what it is: read it)\n", name, name)
	}
	if c := v.M.Conversation; c != "" {
		show := "ws show " + c
		if slug, id, ok := strings.Cut(c, "/"); ok {
			show = "ws show -m " + slug + " " + id
		}
		fmt.Fprintf(&b, "Came out of conversation %s (%s: its summary and turns)\n", c, show)
	}
	for _, t := range v.M.Tasks {
		fmt.Fprintf(&b, "Waits on you, task %d (%s, by %s): %s\n", t.N, t.Verb, firstNonEmpty(t.By, "ws"), t.Text)
	}
	if e := v.M.Ending; e != nil {
		fmt.Fprintf(&b, "Ended (%s, %s): %s\n", e.By, e.At.Local().Format("2 Jan 15:04"), e.Summary())
		for _, l := range conclusionLines(*e) {
			b.WriteString("  " + l + "\n")
		}
	}
	if c := v.M.Conclude; c != nil && c.Ending != "" {
		fmt.Fprintf(&b, "Suggested ending (%s, for the user to end it): %s\n", c.By, c.Summary())
		for _, l := range conclusionLines(*c) {
			b.WriteString("  " + l + "\n")
		}
	} else if c != nil {
		fmt.Fprintf(&b, "Suggested conclusion (%s): %s\n", c.By, c.Text)
	}
	for _, c := range v.M.Crew {
		if c.ResumeWith != "" {
			fmt.Fprintf(&b, "Suggested: resume %s with: %s\n", c.ID, c.ResumeWith)
		}
	}
	if d := v.M.Dispatch; d != nil {
		fmt.Fprintf(&b, "Queued dispatch (%s, waiting for the user): %s\n", d.By, d.Text)
	}
	fmt.Fprintf(&b, "Brief file: %s\n", briefPath)
	if len(v.Capsules) == 0 {
		b.WriteString("Capsules: none yet. Create one with `ws lift <repo> <name>`; with WS_MISSION set it joins this mission.\n")
	} else {
		b.WriteString("Capsules:\n")
		for _, c := range v.Capsules {
			fmt.Fprintf(&b, "  %s/%s", c.Repo, c.Name)
			if c.Exists {
				fmt.Fprintf(&b, " — %s", c.Path)
			}
			if f := capsuleFacts(c); f != "" {
				fmt.Fprintf(&b, " [%s]", f)
			}
			b.WriteByte('\n')
		}
	}
	if len(v.M.EndedCapsules) > 0 {
		b.WriteString("Capsules that ended alone (no longer the mission's, and they don't hold it open):\n")
		for _, e := range v.M.EndedCapsules {
			b.WriteString("  " + e.Words() + "\n")
		}
	}
	if grounds := ops.Grounds(ctx.WS()); len(grounds) > 0 {
		b.WriteString("Repos' default branches, as last fetched (read a repo's code and history here or in a capsule, with git -C <path>: repos/<repo>/ itself isn't a repository):\n")
		for _, g := range grounds {
			b.WriteString("  " + g.String() + "\n")
		}
	}
	if p := v.M.Plan; p != nil {
		fmt.Fprintf(&b, "Plan file: %s (its route from A to Z, in place of the brief's stages: ws mission plan prints it)\n", missionStore(ctx).PlanPath(v.M.Slug))
		b.WriteString("Z (where it ends: each has to be true, and each is checked by its number when it ends):\n")
		for i, z := range p.Z {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, oneLine(z))
		}
		if len(p.Early) > 0 {
			b.WriteString("Stops early:\n")
			for _, e := range p.Early {
				b.WriteString("  - " + oneLine(e) + "\n")
			}
		}
		b.WriteString("Steps (the plan's, with who's on each and ws's record of it; the current one is the first not accepted; ws show s<n> for one):\n")
		for _, l := range prompt.StageLines(v.M, brief) {
			b.WriteString("  " + l + "\n")
		}
	} else {
		if lines := prompt.StageLines(v.M, brief); len(lines) > 0 {
			b.WriteString("Stages (the brief's plan, with ws's record of each; the current one is the first not accepted, and new crew work on it; ws show s<n> for one):\n")
			for _, l := range lines {
				b.WriteString("  " + l + "\n")
			}
		}
		if z := mission.BriefAcceptance(brief); len(z) > 0 && v.M.Ending == nil {
			b.WriteString("Z (the brief's Acceptance: where it ends; each is checked by its number when it ends):\n")
			for i, part := range z {
				fmt.Fprintf(&b, "  %d. %s\n", i+1, oneLine(part))
			}
		}
	}
	if v.M.PlanErr != nil {
		fmt.Fprintf(&b, "Plan file: %s doesn't read, so the mission works from its brief: %v\n", missionStore(ctx).PlanPath(v.M.Slug), v.M.PlanErr)
	}
	store := missionStore(ctx)
	if avs := ops.ResolveArtifacts(ctx.WS(), store, v); len(avs) > 0 {
		b.WriteString("Artifacts (kind, where it is now; read the file named, or open the link):\n")
		for _, av := range avs {
			b.WriteString("  " + ops.ArtifactLine(av) + "\n")
		}
	}
	if bundles := ctx.Bundles(v.M); len(bundles) > 0 {
		b.WriteString("Bundles (what was handed over to show the work is as its checks say; ws show b<n> for one, with its files):\n")
		for _, bv := range bundles {
			b.WriteString("  " + bundleLine(bv) + "\n")
		}
	}
	if guide := ctx.WS().Guide(); guide != "" {
		fmt.Fprintf(&b, "Guide for agents: %s (the workspace's own: read it before you start, and follow it)\n", guide)
	}
	// A few skills are listed whole; many, by name, with where to read more.
	if skills := ctx.WS().Skills(); len(skills) > skillsListed {
		names := make([]string, len(skills))
		for i, s := range skills {
			names[i] = s.Name
		}
		fmt.Fprintf(&b, "Skills (how particular kinds of work are done here; when yours is one of them, read its file first and do it that way): %s. `ws show skills` says what each is for, and where its file is.\n", strings.Join(names, ", "))
	} else if len(skills) > 0 {
		b.WriteString("Skills (how particular kinds of work are done here; when yours is one of them, read its file first and do it that way; ws show skills says more of each):\n")
		for _, s := range skills {
			fmt.Fprintf(&b, "  %s: %s — %s\n", s.Name, firstNonEmpty(clipString(s.About, 160), "(it doesn't say what it's for)"), s.Path)
		}
	}
	fmt.Fprintf(&b, "Private files: %s (notes and scratch for this mission only; record one with ws mission artifact add <file>)\n", store.FilesDir(v.M.Slug))
	if records := ctx.Config().Missions.Records; records != "" {
		fmt.Fprintf(&b, "Records repo: %s (proposals, ADRs and notes to share go here: lift a capsule in it with ws lift %s <branch>, follow its README/AGENTS.md, and record the file with ws mission artifact add)\n", records, records)
		if name, ok := ctx.WS().ResolveAlias(records); ok && ctx.WS().Direct(name) {
			fmt.Fprintf(&b, "  %s has no pull requests: what's committed in a capsule of it lands straight on %s when the user says. Commit there, never push or open a PR, and report done: they review it and land it.\n", records, ctx.WS().DefaultBranch)
		}
	}
	for _, c := range v.M.Crew {
		if c.Status != "" && c.Ended == nil {
			fmt.Fprintf(&b, "Crew %s %s: %s\n", c.ID, c.Status, c.Note)
		}
		if c.Dispatch != "" {
			fmt.Fprintf(&b, "Crew %s was dispatched with: %s\n", c.ID, c.Dispatch)
		}
	}
	if len(v.M.Crew) > 0 {
		b.WriteString("Crew (ws show c<n> for one: its dispatch, handoff and what it made):\n")
		for _, c := range v.M.Crew {
			line := "  " + c.ID + " " + c.Agent()
			switch c.Role {
			case mission.RoleBriefing:
				line += ", briefing"
			case mission.RoleAnswer:
				line += ", answering a question"
			}
			if c.Capsule != "" {
				line += " in " + c.Capsule
			}
			b.WriteString(line + ": " + crewStanding(c) + "\n")
		}
	}
	if reviews, _ := review.For(ctx.WS().Root, v.M.Slug).List(); len(reviews) > 0 {
		var lines []string
		for _, r := range reviews {
			if r.State == review.Submitted {
				lines = append(lines, "  "+r.ID+" "+reviewLine(r))
			}
		}
		if len(lines) > 0 {
			b.WriteString("Reviews by the user (ws show r<n> for one, ws review show r<n> for it whole):\n" + strings.Join(lines, "\n") + "\n")
		}
	}
	if threads, _ := conversation.For(ctx.WS().Root, v.M.Slug).List(); len(threads) > 0 {
		var lines []string
		var talks []string
		for _, c := range threads {
			switch {
			case c.Kind == conversation.Talk:
				talks = append(talks, "  "+c.ID+" "+talkLine(c))
			case c.Status == conversation.Open:
				lines = append(lines, "  "+c.ID+" on "+c.Anchor.Doc+": "+threadLine(c))
			}
		}
		if len(talks) > 0 {
			b.WriteString("Conversations about it (the user's, with an agent, on its Talk tab; ws show t<n> for one: its summary and latest turns):\n" + strings.Join(talks, "\n") + "\n")
		}
		if len(lines) > 0 {
			b.WriteString("Open threads on documents (the user's comments on their text; ws show t<n> for one, ws thread reply t<n> to answer):\n" + strings.Join(lines, "\n") + "\n")
		}
	}
	if all, _ := store.Decisions(v.M.Slug); len(all) > 0 {
		if cur := mission.CurrentDecisions(all); len(cur) > 0 {
			b.WriteString("Decisions (what's been decided and why, newest first; don't go against one without asking the user; ws show d<n> for one):\n")
			for i, d := range cur {
				if i == missionShowDecisions {
					fmt.Fprintf(&b, "  … %d more: ws mission show lists them all in the log; ws show d<n> shows one\n", len(cur)-i)
					break
				}
				b.WriteString("  " + d.Line() + "\n")
			}
		}
	}
	if latest := v.M.LatestHandoffs(); len(latest) > 0 {
		b.WriteString("Latest handoffs (what crew left for whoever comes next; the newest first, one per capsule):\n")
		for _, c := range latest {
			fmt.Fprintf(&b, "  %s, crew %s: %s — read: %s\n", firstNonEmpty(c.Capsule, "no capsule"), c.ID, ops.HandoffLabel(v.M, c.ID), store.HandoffPath(v.M.Slug, c.ID))
		}
	}
	b.WriteString("\n## Brief\n\n")
	if strings.TrimSpace(brief) == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(strings.TrimRight(brief, "\n") + "\n")
	}
	if len(events) > 0 {
		b.WriteString("\n## Recent log\n\n")
		for _, e := range events {
			b.WriteString(formatEventLine(e, true) + "\n")
		}
	}
	b.WriteString("\nws show <ref> shows any of these with what it links to: a stage or step (s1), crew (c1), a capsule (repo/name), an artifact by its ref, a review (r1) or one of its comments (r1.c2), a thread on a document (t3), a decision (d1), a bundle (b1).\n")
	return b.String()
}

func printMissionText(ctx *ops.Workspace, v ops.MissionView, briefPath, brief string, events []mission.Event) {
	w := ui.Stderr
	fmt.Fprintf(w, "\n  %s  %s  %s\n", ui.Bold.Render(v.M.Title), ui.TagDim.Render(v.M.Slug), stateStyle(v.State).Render(string(v.State)))
	snap := missionSnapshot(ctx, v, brief, events)
	if at := snap.Position(); at != "" {
		fmt.Fprintf(w, "  %s\n", ui.Dim.Render(at))
	}
	h := mission.Resolve(snap)[0]
	fmt.Fprintf(w, "\n  %s  %s %s\n    %s\n", ui.Bold.Render("Now"), styledLabel(h), h.Text, ui.Dim.Render(courtWords(h.Court, false)+" · "+h.Action.Label))
	for _, c := range []*mission.Conclusion{v.M.Ending, v.M.Conclude} {
		if c == nil || c.Ending == "" {
			continue
		}
		for _, l := range conclusionLines(*c) {
			fmt.Fprintf(w, "    %s\n", ui.Dim.Render(l))
		}
	}

	if len(v.M.About) > 0 {
		fmt.Fprintf(w, "\n  %s  %s\n", ui.Bold.Render("About"), strings.Join(v.M.About, ", "))
	}

	fmt.Fprintf(w, "\n  %s\n", ui.Bold.Render("Capsules"))
	if len(v.Capsules) == 0 {
		fmt.Fprintf(w, "    %s\n", ui.Dim.Render("none yet"))
	}
	nameW := 0
	for _, c := range v.Capsules {
		nameW = max(nameW, lipgloss.Width(ctx.WS().FormatRepoName(c.Repo)))
	}
	for _, c := range v.Capsules {
		repo := ctx.WS().FormatRepoName(c.Repo)
		pad := strings.Repeat(" ", nameW-lipgloss.Width(repo))
		line := fmt.Sprintf("    %s%s  %s", repo, pad, ui.TagDim.Render(c.Name))
		if f := capsuleFacts(c); f != "" {
			line += "  " + ui.Dim.Render(f)
		}
		fmt.Fprintln(w, line)
	}
	for _, e := range v.M.EndedCapsules {
		fmt.Fprintf(w, "    %s\n", ui.Dim.Render(e.Words()))
	}

	if lines := prompt.StageLines(v.M, brief); len(lines) > 0 {
		title := "Stages"
		if v.M.Plan != nil {
			title = "Plan"
			fmt.Fprintf(w, "\n  %s  %s\n", ui.Bold.Render(title), ui.Dim.Render(missionStore(ctx).PlanPath(v.M.Slug)))
			for i, z := range v.M.Plan.Z {
				fmt.Fprintf(w, "    %s %s\n", ui.Dim.Render(fmt.Sprintf("Z%d", i+1)), oneLine(z))
			}
		} else {
			fmt.Fprintf(w, "\n  %s\n", ui.Bold.Render(title))
		}
		for _, l := range lines {
			fmt.Fprintf(w, "    %s\n", l)
		}
	}

	if avs := ops.ResolveArtifacts(ctx.WS(), missionStore(ctx), v); len(avs) > 0 {
		fmt.Fprintf(w, "\n  %s\n", ui.Bold.Render("Artifacts"))
		for _, av := range avs {
			fmt.Fprintf(w, "    %s\n", artifactText(av))
		}
	}

	fmt.Fprintf(w, "\n  %s  %s\n", ui.Bold.Render("Brief"), ui.Dim.Render(briefPath))
	if strings.TrimSpace(brief) == "" {
		fmt.Fprintf(w, "    %s\n", ui.Dim.Render("(empty)"))
	}
	for line := range strings.SplitSeq(strings.TrimRight(brief, "\n"), "\n") {
		if strings.TrimSpace(brief) != "" {
			fmt.Fprintf(w, "    %s\n", line)
		}
	}

	if len(events) > 0 {
		fmt.Fprintf(w, "\n  %s\n", ui.Bold.Render("Log"))
		for _, e := range events {
			fmt.Fprintf(w, "    %s\n", formatEventLine(e, false))
		}
	}
	fmt.Fprintln(w)
}

func formatEventLine(e mission.Event, plain bool) string {
	if plain {
		return prompt.EventLine(e)
	}
	var detail []string
	if e.Repo != "" || e.Capsule != "" {
		detail = append(detail, e.Repo+"/"+e.Capsule)
	}
	if e.Text != "" {
		detail = append(detail, e.Text)
	}
	who := ""
	if e.Crew != "" {
		who = " (" + e.Crew + ")"
	}
	return fmt.Sprintf("%s  %s%s  %s", ui.Dim.Render(e.Time.Local().Format("2006-01-02 15:04")), e.Kind, ui.Dim.Render(who), strings.Join(detail, " — "))
}

// --- note ---

func newMissionNoteCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "note <text>",
		Short: "Append a note to a mission's log",
		Long:  "Append a note to a mission's log." + fmt.Sprintf(onStdin, "note"),
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
			text, err := wordsOf(args)
			if err != nil {
				return err
			}
			if text == "" {
				return errors.New("note text is empty")
			}
			if err := missionStore(ctx).AppendLog(m.Slug, mission.Event{Kind: mission.EventNote, Text: text}); err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s Noted on %s\n", ui.Green.Render("✓"), m.Slug)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

func addMissionFlag(cmd *cobra.Command, slug *string) {
	cmd.Flags().StringVarP(slug, "mission", "m", "", "mission slug (default: $WS_MISSION, then the current capsule's mission)")
	cmd.RegisterFlagCompletionFunc("mission", completeMissionSlugs)
}

// --- attach / detach ---

func newMissionAttachCmd() *cobra.Command {
	var slug string
	var move bool
	cmd := &cobra.Command{
		Use:   "attach <repo> <capsule>",
		Short: "Add an existing capsule to a mission",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return completeRepoNames(cmd, args, toComplete)
			}
			if len(args) == 1 {
				return completeWorktreeNames(0)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
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
			repo, err := resolveRepoStrict(ctx, args[0])
			if err != nil {
				return err
			}
			capsule, err := resolveWorktreeStrict(ctx, repo, args[1])
			if err != nil {
				return err
			}
			if _, err := ctx.Attach(m.Slug, repo, capsule, move, ops.ActorFromEnv()); err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s %s → mission %s\n", ui.Green.Render("✓"), ctx.WS().FormatRepoName(repo), ui.TagDim.Render(capsule), m.Slug)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().BoolVar(&move, "move", false, "take the capsule from the mission that owns it")
	return cmd
}

func newMissionDetachCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "detach <repo> <capsule>",
		Short: "Remove a capsule from a mission (the capsule itself is kept)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			// Fall back to the literal name so a mission can drop capsules of a
			// repo that has since left ws.toml (ws doctor suggests this).
			repo, err := resolveRepoStrict(ctx, args[0])
			if err != nil {
				if !m.HasCapsule(args[0], args[1]) {
					return err
				}
				repo = args[0]
			}
			capsule := args[1]
			if err := ctx.Detach(m.Slug, repo, capsule, ops.ActorFromEnv()); err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s %s detached from %s\n", ui.Green.Render("✓"), ctx.WS().FormatRepoName(repo), ui.TagDim.Render(capsule), m.Slug)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

// --- park / archive / reopen ---

func newMissionStateCmd(name string, state mission.State, short string) *cobra.Command {
	return &cobra.Command{
		Use:               name + " [mission]",
		Short:             short,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeMissionSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, firstArg(args))
			if err != nil {
				return err
			}
			set, err := ctx.SetState(m.Slug, state, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s %s\n", ui.Green.Render("✓"), m.Slug, set.Label())
			return nil
		},
	}
}

// newMissionArchiveCmd archives a mission, burning its capsules: refused
// while one's work hasn't merged, unless --burn-all.
func newMissionArchiveCmd() *cobra.Command {
	var burnAll bool
	cmd := &cobra.Command{
		Use:   "archive [mission]",
		Short: "Archive a mission, burning its capsules once their work has merged",
		Long: `Archive a mission, burning its capsules: archived is cleaned up.

Each capsule's pull request must have merged at the commit it's at (or it
landed from there), with nothing uncommitted. Otherwise archiving is
refused, saying which and why; --burn-all burns them anyway, their
branches kept and what's not committed lost.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeMissionSlugs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, firstArg(args))
			if err != nil {
				return err
			}
			set, err := ctx.Archive(m.Slug, burnAll, ops.ActorFromEnv())
			var r *ops.Refused
			if errors.As(err, &r) && r.Code == "unmerged-capsules" {
				return fmt.Errorf("%w (ws mission archive %s --burn-all)", err, m.Slug)
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s archived\n", ui.Green.Render("✓"), m.Slug)
			for _, c := range set.Burned {
				fmt.Fprintf(ui.Stderr, "    burned %s\n", c)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&burnAll, "burn-all", false, "Burn every capsule, even one whose work hasn't merged: what's not committed is lost")
	return cmd
}

// --- about ---

func newMissionAboutCmd() *cobra.Command {
	var slug string
	var remove []string
	cmd := &cobra.Command{
		Use:   "about [ref...]",
		Short: "Say what a mission is about: an issue a plugin has, a pull request",
		Long: `Say what a mission is about, by a name ws reads: one of a plugin's things
(RAM-123, as ws plugin list says they're named), a pull request (demo#12).
Whoever reads the mission is told to read it, and the mission is found
from it (ws show RAM-123). With no ref, it prints what the mission's about.

  ws mission about RAM-123
  ws mission about --remove RAM-123`,
		PersistentPreRun:  func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			if len(args) > 0 || len(remove) > 0 {
				if m, err = ctx.SetAbout(m.Slug, args, remove); err != nil {
					return err
				}
			}
			if len(m.About) == 0 {
				fmt.Fprintf(ui.Stderr, "  %s %s isn't about anything ws names\n", ui.Dim.Render("·"), m.Slug)
				return nil
			}
			for _, name := range m.About {
				fmt.Println(name)
			}
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringArrayVar(&remove, "remove", nil, "something it's about no more (repeatable)")
	return cmd
}
