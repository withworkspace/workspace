package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

func newMissionBundleCmd() *cobra.Command {
	var slug, set string
	var format, full bool
	cmd := &cobra.Command{
		Use:   "bundle [b<n>]",
		Short: "A mission's verification bundles: list them, show one, or hand one over with --set",
		Long: `A verification bundle is what's handed over to show that work is as its
checks say: each check with what it came to and what shows it (images,
video, text), how to try it yourself, and what wasn't verified.

  ws mission bundle            the mission's bundles
  ws mission bundle b3         one of them (as ws show b3)
  ws mission bundle --format   how one is written

A bundle is yours to approve or send back, as a review is:

  ws mission bundle approve b3
  ws mission bundle send-back b3 --check "3: it's gone after a reload in Firefox"

Approving one a Review step looks at passes that step. Sending one back
tells the crew who made the work what you said, with what the bundle
showed, and the bundle they hand over next takes its place.

For crew, on the step they're on:

  ws mission bundle --set - <<'EOF'
  ## Summary
  …
  ## Checks
  1. met: …
     - shows: ` + "`shot.png`" + ` what it shows
  ## Not verified
  Nothing.
  EOF

ws reads it strictly against the step's checks and refuses one that isn't
right, with every problem and its line. It copies each file the bundle
names into the bundle's own folder, so the bundle is whole whatever
becomes of the files it was made from, and keeps it on the mission with
the commit its capsule is at. A bundle is of a commit: commit first. It's
never edited: hand over another, and it takes this one's place.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format {
				_, err := os.Stdout.WriteString(mission.BundleFormat + stepChecksForCrew(slug))
				return err
			}
			if set != "" {
				if len(args) > 0 {
					return errors.New("--set hands over a new bundle: it takes no bundle's ref")
				}
				ctx, m, c, err := signallingCrew(slug)
				if err != nil {
					return err
				}
				text, err := readFlagText(set)
				if err != nil {
					return fmt.Errorf("reading the bundle: %w", err)
				}
				handed, err := ctx.HandOver(m.Slug, c.ID, text, userDirOr())
				b := handed.Bundle
				if err != nil {
					if mission.IsMalformedBundle(err) {
						return fmt.Errorf("%w\n\nws mission bundle --format says how one is written", err)
					}
					return err
				}
				said := b.ID + " kept: " + b.Tally()
				if b.Step > 0 {
					said = fmt.Sprintf("%s kept for step %d: %s", b.ID, b.Step, b.Tally())
				}
				if b.Replaces != "" {
					said += ", in place of " + b.Replaces
				}
				fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), said)
				switch {
				case !b.Own:
					// A verifier's: handing it over is its done.
					if handed.Next != "" {
						fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Dim.Render("·"), handed.Next)
					}
					fmt.Fprintf(ui.Stderr, "  %s that's your part done: the bundle is your handoff, and the user has been notified\n", ui.Green.Render("✓"))
				case b.Commit != "":
					fmt.Fprintf(ui.Stderr, "  %s it's of %s at %s: a commit after it makes it stale, so hand over another then\n", ui.Dim.Render("·"), b.Capsule, shortSHA(b.Commit))
				}
				return nil
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				ref := strings.TrimSpace(args[0])
				if !mission.IsBundleRef(ref) {
					return fmt.Errorf("%q isn't a bundle's ref: one is b1, b2…", ref)
				}
				v, err := loadGraph(ctx, m, full).view(ref)
				if err != nil {
					return err
				}
				_, err = io.WriteString(ui.Stdout, v.render(true, full))
				return err
			}
			views := ctx.Bundles(m)
			if len(views) == 0 {
				fmt.Fprintf(ui.Stderr, "  %s %s has no bundles (crew hand one over with ws mission bundle --set -)\n", ui.Dim.Render("·"), m.Slug)
				return nil
			}
			for _, v := range views {
				fmt.Fprintln(ui.Stdout, "  "+bundleLine(v))
			}
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&set, "set", "", "crew: hand over a bundle, in markdown: - for stdin, or a file")
	cmd.Flags().BoolVar(&format, "format", false, "say how a bundle is written, and what ws holds it to")
	cmd.Flags().BoolVar(&full, "full", false, "show everything a bundle's view would leave out")
	cmd.AddCommand(newBundleApproveCmd(), newBundleSendBackCmd())
	return cmd
}

func newBundleApproveCmd() *cobra.Command {
	var slug string
	cmd := &cobra.Command{
		Use:   "approve <b<n>>",
		Short: "Approve a verification bundle: yours, as approving a review is",
		Long: `Approve a bundle: what it shows is as its checks say. It's yours alone
(crew and the orchestrator are refused). Where the mission is at a Review
step that looks at the bundle, approving it passes that step, as going on
does; a bundle something has taken the place of can't be approved.`,
		Args:              cobra.ExactArgs(1),
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
			done, err := ctx.ApproveBundle(m.Slug, strings.TrimSpace(args[0]), ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), done)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

var bundleNoteShape = regexp.MustCompile(`^\s*(\d+)\s*[:.]\s*(.+)$`)

func newBundleSendBackCmd() *cobra.Command {
	var slug string
	var checks []string
	cmd := &cobra.Command{
		Use:   "send-back <b<n>> [--check \"<n>: <what's not right>\"]... [<what's not right, of it as a whole>]",
		Short: "Send a verification bundle back to its crew, with what's not right",
		Long: `Send a bundle back: say what's not right of a check, by its number
(--check "3: it's gone after a reload", once for each), or of the bundle
as a whole (in words after its ref). It's yours alone.

What you said is kept on the bundle and goes to the crew who made the
work, with each check and what the bundle showed for it. They put it
right and hand over another, which takes this one's place. A Review step
that looks at it stays yours.`,
		Args:              cobra.MinimumNArgs(1),
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
			var notes []mission.BundleNote
			for _, c := range checks {
				g := bundleNoteShape.FindStringSubmatch(c)
				if g == nil {
					return fmt.Errorf("--check %q: say the check's number, then what's not right (--check \"3: it's gone after a reload\")", c)
				}
				n, _ := strconv.Atoi(g[1])
				notes = append(notes, mission.BundleNote{Check: n, Text: g[2]})
			}
			if whole := strings.TrimSpace(strings.Join(args[1:], " ")); whole != "" {
				notes = append(notes, mission.BundleNote{Text: whole})
			}
			done, err := ctx.SendBackBundle(m.Slug, strings.TrimSpace(args[0]), notes, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), done)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringArrayVar(&checks, "check", nil, "what's not right of a check, as \"<n>: <words>\" (repeatable)")
	return cmd
}

// stepChecksForCrew is, for crew on a step with checks, those checks by
// their numbers, after the bundle's format: what theirs answers. "" for
// anyone else.
func stepChecksForCrew(slug string) string {
	id := os.Getenv(mission.EnvCrew)
	if id == "" {
		return ""
	}
	ctx, err := loadOps()
	if err != nil {
		return ""
	}
	m, err := resolveMission(ctx, slug)
	if err != nil {
		return ""
	}
	brief, _ := missionStore(ctx).ReadBrief(m.Slug)
	states, _ := m.StageProgress(brief)
	st, ok := mission.StageFor(states, id)
	if !ok || st.Step == nil || len(st.Step.Checks) == 0 {
		return "\nYour step has no checks, so the bundle's checks are your own.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\nYour step (s%d) has these checks, by number:\n\n", st.N)
	for i, c := range st.Step.Checks {
		who := "yours to show"
		switch {
		case c.Run():
			who = "ws runs it: leave it out"
		case c.Yours:
			who = "the user's to settle: say yours:, with what you've left to help"
		}
		fmt.Fprintf(&b, "%d. %s (%s)\n", i+1, c.Text, who)
	}
	return b.String()
}

// bundleLine is a bundle in a line: "b3 step 5 (Browser check), by c2: 2
// met, 1 can't tell, replaced by b4".
func bundleLine(v ops.BundleView) string {
	line := v.ID
	switch {
	case v.StepN > 0:
		line += fmt.Sprintf(" step %d (%s)", v.StepN, v.StepName)
	case v.Step > 0:
		line += fmt.Sprintf(" a step the plan no longer has (it was step %d)", v.Step)
	}
	line += ", by " + v.Crew + ": " + v.Tally()
	switch {
	case v.ReplacedBy != "":
		line += ", replaced by " + v.ReplacedBy
	case v.Verdict != "":
		line += ", " + v.Verdict + " by you"
	case v.Asked:
		line += fmt.Sprintf(", waiting for the user (step %d looks at it)", v.AskedAt)
	}
	return line
}

// bundleView is a bundle: who handed it over and of what, its summary,
// each check with what it came to and what shows it, and what ws ran.
func (g *graph) bundleView(id string) (*view, error) {
	var b ops.BundleView
	found := false
	for _, v := range g.ctx.Bundles(g.m) {
		if v.ID == id {
			b, found = v, true
		}
	}
	if !found {
		return nil, fmt.Errorf("mission %s has no bundle %s", g.m.Slug, id)
	}
	of := "of no step"
	if b.StepN > 0 {
		of = fmt.Sprintf("of step %d, %s", b.StepN, b.StepName)
	}
	v := &view{head: fmt.Sprintf("Bundle %s (%s): %s", b.ID, b.Tally(), of)}
	by := "Handed over " + when(b.At) + " by " + b.Crew
	if b.By != "" {
		by += " (" + b.By + ")"
	}
	if b.Own {
		by += ", who did the work it shows"
	}
	if b.Capsule != "" {
		by += ", of " + b.Capsule + " at " + firstNonEmpty(shortSHA(b.Commit), "no commit")
	}
	v.body = append(v.body, by+".")
	switch {
	case b.ReplacedBy != "":
		v.body = append(v.body, b.ReplacedBy+" has taken its place.")
	case b.Replaces != "":
		v.body = append(v.body, "It took the place of "+b.Replaces+".")
	}
	switch {
	case b.Verdict == mission.BundleApproved && b.VerdictAt != nil:
		v.body = append(v.body, "The user approved it "+when(*b.VerdictAt)+".")
	case b.Verdict == mission.BundleSentBack && b.VerdictAt != nil:
		lines := []string{"The user sent it back " + when(*b.VerdictAt) + ", saying what's not right:"}
		for _, n := range b.Notes {
			if n.Check > 0 {
				lines = append(lines, fmt.Sprintf("- check %d: %s", n.Check, n.Text))
			} else {
				lines = append(lines, "- of it as a whole: "+n.Text)
			}
		}
		v.body = append(v.body, strings.Join(lines, "\n"))
	case b.Asked && b.ReplacedBy == "":
		v.body = append(v.body, fmt.Sprintf("It waits for the user: step %d looks at it, and their approving it passes that step (ws mission bundle approve %s is theirs alone).", b.AskedAt, b.ID))
	}
	v.body = append(v.body, g.clip(b.Summary))
	if len(b.Try) > 0 {
		lines := []string{"To try it:"}
		for _, t := range b.Try {
			lines = append(lines, "- "+tryLine(t))
		}
		v.body = append(v.body, strings.Join(lines, "\n"))
	}
	for _, c := range b.Checks {
		lines := []string{fmt.Sprintf("%d. %s: %s", c.N, c.Verdict, firstNonEmpty(c.Check, c.Said))}
		if c.Check != "" {
			lines = append(lines, "   "+c.Said)
		}
		for _, pt := range c.Points {
			lines = append(lines, "   - "+pt)
		}
		for _, e := range c.Shows {
			lines = append(lines, "   shows: "+evidenceLine(b.Dir, e))
			if e.Before != nil {
				lines = append(lines, "     before: "+evidenceLine(b.Dir, *e.Before))
			}
			if e.MadeBy != "" {
				lines = append(lines, "     made by: `"+e.MadeBy+"`")
			}
		}
		for _, l := range c.Lines {
			lines = append(lines, "   lines: "+l)
		}
		for _, t := range c.Try {
			lines = append(lines, "   try: "+tryLine(t))
		}
		for _, a := range c.Asks {
			lines = append(lines, "   ask: "+a)
		}
		v.body = append(v.body, strings.Join(lines, "\n"))
	}
	if r := b.Ran; r != nil {
		lines := []string{fmt.Sprintf("Its step's checks, as ws last ran and had them judged (round %d, %s, %s): %s.", r.N, firstNonEmpty(shortSHA(r.Commit), "no commit"), when(r.At), r.Summary())}
		for _, c := range r.Results {
			line := fmt.Sprintf("- %s: %s, by %s", c.Check, c.Result, c.By)
			if ev := strings.TrimSpace(c.Evidence); ev != "" {
				line += ": " + clipLine(ev, 300)
			}
			lines = append(lines, line)
		}
		v.body = append(v.body, strings.Join(lines, "\n"))
	}
	if len(b.NotVerified) > 0 {
		v.body = append(v.body, "Not verified:\n- "+strings.Join(b.NotVerified, "\n- "))
	} else {
		v.body = append(v.body, "Not verified: nothing.")
	}
	if len(b.Noticed) > 0 {
		v.body = append(v.body, "Noticed, beside the checks:\n- "+strings.Join(b.Noticed, "\n- "))
	}
	if b.StepN > 0 {
		v.add(fmt.Sprintf("s%d", b.StepN), "the step it's of: "+b.StepName)
	}
	g.crewLink(v, b.Crew, "handed it over")
	if repo, name, ok := strings.Cut(b.Capsule, "/"); ok && g.m.HasCapsule(repo, name) {
		v.add(b.Capsule, "the capsule it's of")
	}
	if b.Replaces != "" {
		v.add(b.Replaces, "the bundle it took the place of")
	}
	if b.ReplacedBy != "" {
		v.add(b.ReplacedBy, "the bundle that took its place")
	}
	v.lately = g.about(func(e mission.Event) bool {
		return e.Kind == mission.EventBundle && (strings.HasPrefix(e.Text, b.ID+" ") || strings.HasPrefix(e.Text, b.ID+":"))
	})
	if len(b.Files()) > 0 {
		v.more = append(v.more, "its files are in "+b.Dir)
	}
	return v, nil
}

// tryLine is something to try, as a bundle's view says it.
func tryLine(t mission.BundleTry) string {
	if t.Look {
		return "look for: " + t.Text
	}
	return t.Text
}

// evidenceLine is a file a check shows: where it's kept, what it is, and
// what it shows.
func evidenceLine(dir string, e mission.Evidence) string {
	kind := "text"
	switch {
	case strings.HasPrefix(e.Media, "image/"):
		kind = "an image"
	case strings.HasPrefix(e.Media, "video/"):
		kind = "a video"
	}
	line := filepath.Join(dir, e.File) + " (" + kind + ")"
	if e.Caption != "" {
		line += ": " + e.Caption
	}
	return line
}
