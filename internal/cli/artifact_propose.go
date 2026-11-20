package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws artifact add --propose: a conversation's agent proposes an artifact, or
// a version of one (--to, --on), and the user adds it, or not, with a button
// under its reply. It's the one verb of ws artifact add a conversation's
// allow list admits (harness.converseCommands), with --propose first; ws show
// a3 and ws artifact show a3.v2 are its reads. With no file it proposes
// nothing, and prints how one's written (artifactGuide).

// artifactGuide is how an artifact is written and proposed, for a
// conversation's agent: ws artifact add --propose, with no file, prints
// it. The rules are few, and the frame holds it to the ones that matter
// (nothing leaves it, and it keeps nothing), so it's told, not checked.
const artifactGuide = `An artifact is one HTML file that shows an idea, to look at and use rather than read: a mock-up of a page, two takes on a colour, a diagram you can click through, a chart of what you found. The user sees it running under your reply, in a small sandboxed frame, and adds it with a click, or not. Each version is a whole file, never changed once added.

` + artifactRules + artifactTweaks + artifactHowTo

// artifactRules is how one's written, as ws has it: the workspace's own
// guide ([artifacts] guide) is shown in its place.
const artifactRules = `How one's written:
- One HTML file, starting <!doctype html>, with its CSS and JS inline (<style>, <script>), and a <title>.
- No network: no CDN, no web font, no API, no script, stylesheet or image from elsewhere. The frame fetches nothing. Put images in as data: URLs, or draw them (SVG, CSS, canvas). Show the data it would fetch in the file.
- At most %s.
- Cover both colour schemes where it matters (prefers-color-scheme): the frame shows it in the scheme the user's in.
- Don't depend on localStorage, sessionStorage or cookies: the frame gives it none (they throw). Keep state in variables.

`

// artifactTweaks is how a page says what can be changed about it, for ws
// to draw the controls: said to a conversation's agent with the rest
// (whichever guide says how one's written: this is what ws gives a page,
// not how a workspace likes them), and to crew by ws artifact add --help.
// It has no verbs for Printf: it's printed as it is.
const artifactTweaks = `Tweaks: where there's something to choose between (a density, a colour, two takes on a layout), don't draw controls for it in the page. Say what can be changed, and ws draws a tray of controls beside the frame, with your presets as buttons:

  <script>
  ws.tweaks({
    tweaks: {
      density: { label: "Density", description: "How tall a row is.",
                 options: ["Cozy", { value: "compact", label: "Compact", description: "28px rows" }] },
      pips:    { label: "Progress pips" },
      radius:  { label: "Corner radius", min: 0, max: 16, unit: "px" },
      accent:  { label: "Accent", type: "color" },
      heading: { label: "Heading" },
    },
    presets: [
      { label: "As it is today", description: "What ships now.",
        state: { density: "Cozy", pips: true, radius: 8, accent: "#5b8def", heading: "Plan" } },
      { label: "Suggested", description: "Compact rows, softer corners.",
        state: { density: "compact", radius: 12 } },
    ],
  });
  </script>

- ws is there already (ws writes it into the page). Call it from a <script> in <head>, and the page is drawn as it's set from the first paint.
- The first preset is what the page opens as, and gives every tweak a value. A tweak is what its options or that value make it: true or false, a switch; a number, a box, or a slider with min and max; a string, a text box; type: "color", a colour. The other presets set the tweaks they list and leave the rest: use them for how it is today, what you'd suggest, and each treatment worth a look.
- An option is a string, which is its own label, or { value, label, description }. A tweak's name is letters, digits, - and _, as your CSS and JS read it. A label and a description are for the user: say what the tweak is for.
- Each value is on <html> for your styles: data-density="compact" (data-pips there or not, for a switch), and --radius: 12px and --accent: #5b8def for a number and a colour. So most of it is CSS: :root[data-density="compact"] .row { … }, border-radius: var(--radius).
- For what CSS can't do: ws.tweaks.state is the state now, and window hears "ws:tweaks" on every change, its detail the state. Or give ws.tweaks a function as a second argument, called with the state now and on every change.
- A version's tweaks are its own. When the user says what they'd keep (the tray copies what each is set to), make the next version open as that, with tweaks for what's still open.

`

// newArtifactTweaksCmd is ws artifact tweaks: how a page declares its
// tweaks, for whoever's writing one (crew, who aren't shown the
// conversation's guide, and you).
func newArtifactTweaksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tweaks",
		Short: "Print how an artifact's page says what can be changed about it (ws.tweaks)",
		Long: `Print how an artifact's page declares its tweaks: what can be changed
about it, such as a density, a colour or two takes on a layout. The web UI
draws a control for each beside the page's frame, and the page's presets as
buttons, so the page draws none itself. The state they're in is in the
page's address (/artifacts/a3/v2?t.density=compact), and comparing puts a
version beside itself in two states.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(strings.TrimRight(artifactTweaks, "\n") + "\n")
		},
	}
}

// artifactHowTo is how one's proposed and revised, whichever guide says
// how it's written.
const artifactHowTo = `Propose a new one:

  ws artifact add --propose --title "<what it is>" --note "<what it tries out, in a line or two>" - <<'EOF'
  <!doctype html>
  …
  EOF

Revise one (a new version, the whole file again): read it first, ws show a3 (its versions, notes and where each came from) and ws artifact show a3.v2 (a version's HTML), then

  ws artifact add --propose --to a3 --on v2 --note "<what changed, and why>" - <<'EOF'
  …
  EOF

--on is the version it revises (default: the latest). Each proposal is a new button: propose again to change it, and say in your reply what you proposed.

It's checked before anything's proposed: one HTML file, under the limit, with a title. What won't load in the frame is said then, to put right and propose again. Confirming adds it as the user's, from this conversation, %s.`

// showArtifactGuide prints how an artifact's written and proposed: the
// workspace's own guide in place of ws's rules, where [artifacts] names
// one that's there.
func showArtifactGuide(ctx *ops.Workspace) {
	where := "on this machine (in .ws/artifacts)"
	if ctx != nil {
		if c := ctx.ArtifactsConfig(); c.InRepo() {
			where = fmt.Sprintf("in %s (%s), committed and landed on its default branch when the user adds it", c.Repo, c.Path)
		}
		if name, text, ok := ctx.ArtifactGuide(); ok {
			fmt.Printf("An artifact is one HTML file that shows an idea, to look at and use rather than read. The user sees it running under your reply, in a small sandboxed frame, and adds it with a click, or not. Each version is a whole file, never changed once added. Its frame fetches nothing (no network: images as data: URLs) and keeps nothing (no storage), and a version is at most %s.\n\nHow one's written, from the workspace's guide, %s:\n\n-----\n%s\n-----\n\n", artifact.Size(artifact.MaxSize), name, strings.TrimRight(text, "\n"))
			fmt.Print(artifactTweaks)
			fmt.Printf(artifactHowTo+"\n", where)
			return
		}
	}
	fmt.Printf(artifactGuide+"\n", artifact.Size(artifact.MaxSize), where)
}

// proposeArtifact is what ws artifact add --propose does with a file:
// checks it, works out where it would go, and proposes it in the
// conversation the agent's answering.
func proposeArtifact(ctx *ops.Workspace, html []byte, title, note, to, on string) error {
	checked, err := artifact.Check(html)
	if err != nil {
		return fmt.Errorf("not proposed: %v", err)
	}
	note = artifact.CleanNote(note)
	if note == "" {
		what := "what it tries out"
		if to != "" {
			what = "what changed in it and why"
		}
		return fmt.Errorf("not proposed: give it a note (--note), saying %s in a line or two: the user reads it on the card, and whoever revises it next reads it first", what)
	}
	p := conversation.Proposal{Kind: conversation.ProposeArtifact, HTML: string(html), Note: note, Size: len(html), Warnings: checked.Warnings}
	if to == "" {
		if on != "" {
			return errors.New("--on says which version a new one is based on: give the artifact too (--to a3)")
		}
		p.Title = artifact.CleanTitle(title)
		if p.Title == "" {
			p.Title = checked.Title
		}
		if p.Title == "" {
			return errors.New("not proposed: give it a title (--title), or give its HTML one (<title>)")
		}
		p.Path = ctx.ArtifactWhere(p.Title) + artifact.VersionFile(1)
	} else {
		id, n, ok := artifact.ParseRef(to)
		if !ok {
			return fmt.Errorf("%q isn't an artifact: they're named a1, a2, … (ws show a3 shows one)", to)
		}
		if on != "" {
			if n, ok = artifact.ParseVersion(on); !ok {
				return fmt.Errorf("--on %s: say a version, as v2", on)
			}
		}
		e, err := ctx.Artifacts().Get(context.Background(), id)
		if err != nil {
			return fmt.Errorf("not proposed: %w", err)
		}
		switch {
		case e.Status == artifact.Discarded:
			return fmt.Errorf("not proposed: %s was discarded, so it takes no new versions: propose a new artifact", id)
		case e.Gone:
			return fmt.Errorf("not proposed: %s's folder has gone (it was %s): propose a new artifact", id, e.Where())
		case e.InFlight() != "":
			return fmt.Errorf("not proposed: %s hasn't landed: it's crew's, in %s, and lands with it. Propose a version once it has, or a new artifact", id, e.InFlight())
		}
		latest, _ := e.Latest()
		if n == 0 {
			n = latest.N
		}
		if _, ok := e.Version(n); !ok {
			return fmt.Errorf("not proposed: %s has no v%d to base it on (ws show %s lists its versions)", id, n, id)
		}
		p.To, p.On, p.Latest, p.Title = id, n, latest.N, artifact.CleanTitle(title)
		p.Path = e.Where() + artifact.VersionFile(latest.N+1)
	}
	if err := propose(ctx, p); err != nil {
		return err
	}
	for _, w := range checked.Warnings {
		fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Orange.Render("⚠"), w)
	}
	if len(checked.Warnings) > 0 {
		fmt.Fprintf(ui.Stderr, "  %s it's proposed as it is: to put that right, propose it again, and say which to add\n", ui.Dim.Render("·"))
	}
	what := fmt.Sprintf("it's added as the user's, from this conversation, at %s (on this machine)", p.Path)
	if c := ctx.ArtifactsConfig(); c.InRepo() {
		what = fmt.Sprintf("it's added as the user's, from this conversation, at %s, committed and landed on the default branch of %s (a -2, or the next number, if the name's taken by then)", p.Path, c.Repo)
	}
	if p.To != "" {
		what = fmt.Sprintf("it's added to %s as v%d (the number's taken then), based on v%d, from this conversation", p.To, p.Latest+1, p.On)
	}
	fmt.Fprintf(ui.Stderr, "  %s confirmed, %s\n", ui.Dim.Render("·"), what)
	return nil
}
