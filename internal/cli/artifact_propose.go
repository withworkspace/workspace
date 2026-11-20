package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
// a3, ws artifact show a3.v2 and ws artifact checkout a3.v2 are its reads.
// With no file it proposes nothing, and prints how one's written
// (artifactGuide).

// artifactGuide is how an artifact is written and proposed, for a
// conversation's agent: ws artifact add --propose, with no file, prints
// it. The rules are few, and the frame holds it to the ones that matter
// (nothing leaves it, and it keeps nothing), so it's told, not checked.
const artifactGuide = `An artifact is a page that shows an idea, to look at and use rather than read: a mock-up of a page, two takes on a colour, a diagram you can click through, a chart of what you found. The user sees it running under your reply, in a small sandboxed frame, and adds it with a click, or not. Each version is whole, never changed once added.

` + artifactRules + artifactTweaks + artifactHowTo

// artifactRules is how one's written, as ws has it: the workspace's own
// guide ([artifacts] guide) is shown in its place.
const artifactRules = `How one's written:
- Its page is index.html, starting <!doctype html>, with a <title>. A small one is that file alone, with its CSS and JS inline (<style>, <script>).
- One with more to it is a folder: index.html at its top, and beside it what the page loads (stylesheets, scripts and modules, images, fonts, data), each a real file. Load them by paths from the page (css/app.css, not /css/app.css). Its scripts can read its own files (fetch("data.json")), and a link to another of its pages opens it in the frame.
- Nothing from elsewhere: no CDN, no web font, no API. The frame loads the artifact's own files and nothing else. Put what it needs among its files, or draw it (SVG, CSS, canvas).
- At most %s for all of a version's files, and %d of them.
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
const artifactHowTo = `Propose a new one. Its page alone:

  ws artifact add --propose --title "<what it is>" --note "<what it tries out, in a line or two>" - <<'EOF'
  <!doctype html>
  …
  EOF

Or, for one with files, the folder you wrote them in (index.html at its top):

  ws artifact add --propose --title "<what it is>" --note "<what it tries out>" <folder>

Revise one (a new version, whole): read it first. ws show a3 says its versions, their notes and files, and where each came from; ws artifact show a3.v2 prints a version's page, and ws artifact show a3.v2 css/app.css another of its files. For one that's its page alone, propose the whole file again:

  ws artifact add --propose --to a3 --on v2 --note "<what changed, and why>" - <<'EOF'
  …
  EOF

For one with files, check the version out, change what you're changing in the folder it prints, and propose the folder. The files you leave alone go in as they are:

  ws artifact checkout a3.v2
  ws artifact add --propose --to a3 --on v2 --note "<what changed, and why>" <folder>

--on is the version it revises (default: the latest). Each proposal is a new button: propose again to change it, and say in your reply what you proposed.

It's checked before anything's proposed: a page, within the limits, with a title. What won't load in the frame is said then, to put right and propose again. Confirming adds it as the user's, from this conversation, %s.`

// showArtifactGuide prints how an artifact's written and proposed: the
// workspace's own guide in place of ws's rules, where [artifacts] names
// one that's there.
func showArtifactGuide(ctx *ops.Workspace) {
	where := "on this machine (in .ws/artifacts)"
	if ctx != nil {
		if name, text, ok := ctx.ArtifactGuide(); ok {
			fmt.Printf("An artifact is a page that shows an idea, to look at and use rather than read: index.html, alone or with the files it loads beside it, in a folder. The user sees it running under your reply, in a small sandboxed frame, and adds it with a click, or not. Each version is whole, never changed once added. Its frame loads the artifact's own files and nothing else (no network), and keeps nothing (no storage). A version's files come to at most %s, and %d of them.\n\nHow one's written, from the workspace's guide, %s:\n\n-----\n%s\n-----\n\n", artifact.Size(artifact.MaxSize), artifact.MaxFiles, name, strings.TrimRight(text, "\n"))
			fmt.Print(artifactTweaks)
			fmt.Printf(artifactHowTo+"\n", where)
			return
		}
	}
	fmt.Printf(artifactGuide+"\n", artifact.Size(artifact.MaxSize), artifact.MaxFiles, where)
}

// inProposal is the most a page alone may be to be kept in its proposal.
const inProposal = 1 << 20

// proposeArtifact is what ws artifact add --propose does with a file or a
// folder: checks it, works out where it would go, and proposes it in the
// conversation the agent's answering. A page alone is kept in the
// proposal; one with files, beside the artifacts until it's added
// (artifact.Store.Propose), the proposal naming them.
func proposeArtifact(ctx *ops.Workspace, files artifact.Files, title, note, to, on string) error {
	checked, err := artifact.Check(files)
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
	p := conversation.Proposal{Kind: conversation.ProposeArtifact, Note: note, Size: checked.Files.Size(), Warnings: checked.Warnings}
	// A page alone is kept in the proposal, as the conversation's records
	// keep what's said; one with files, or too big a page for that, beside
	// the artifacts.
	if len(checked.Files) == 1 && p.Size <= inProposal {
		page, _ := checked.Files.Index()
		p.HTML = string(page)
	} else {
		for _, f := range checked.Files.Infos() {
			p.Files = append(p.Files, conversation.ProposedFile{Path: f.Path, Size: f.Size})
		}
	}
	if to == "" {
		if on != "" {
			return errors.New("--on says which version a new one is based on: give the artifact too (--to a3)")
		}
		p.Title = artifact.CleanTitle(title)
		if p.Title == "" {
			p.Title = checked.Title
		}
		if p.Title == "" {
			return errors.New("not proposed: give it a title (--title), or give its page one (<title>)")
		}
		p.Path = ctx.ArtifactWhere(p.Title)
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
		case e.Gone || e.Away():
			return fmt.Errorf("not proposed: %s's folder has gone (it was %s): propose a new artifact", id, e.Where())
		}
		latest, _ := e.Latest()
		if n == 0 {
			n = latest.N
		}
		if _, ok := e.Version(n); !ok {
			return fmt.Errorf("not proposed: %s has no v%d to base it on (ws show %s lists its versions)", id, n, id)
		}
		p.To, p.On, p.Latest, p.Title = id, n, latest.N, artifact.CleanTitle(title)
		p.Path = e.Where()
	}
	if len(p.Files) > 0 {
		what := p.Title
		if p.To != "" {
			what = "a version of " + p.To
		}
		if p.Tree, err = ctx.Artifacts().Propose(context.Background(), checked.Files, what, time.Now()); err != nil {
			return fmt.Errorf("not proposed: keeping its files: %w", err)
		}
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
	what := fmt.Sprintf("it's added as the user's (%s), from this conversation, in %s (on this machine; a -2 if the name's taken by then)", artifact.FilesWords(len(checked.Files), p.Size), p.Path)
	if p.To != "" {
		what = fmt.Sprintf("it's added to %s as v%d (the number's taken then), based on v%d, from this conversation", p.To, p.Latest+1, p.On)
	}
	fmt.Fprintf(ui.Stderr, "  %s confirmed, %s\n", ui.Dim.Render("·"), what)
	return nil
}
