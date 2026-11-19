package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws artifact: versioned HTML prototypes (internal/artifact), added by
// crew and by you, kept on this machine in .ws/artifacts (one you promote
// may live in a repo too), and shown by ws show a3. Not ws mission artifact,
// which records a pointer to what a mission made (crew's artifacts are
// recorded there as a3).

func newArtifactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "artifact",
		Aliases: []string{"artifacts"},
		Short:   "Add, list, show, promote and discard artifacts: versioned HTML prototypes",
		Long: `An artifact is one HTML file, with its CSS and JS inline, that shows an
idea: something to look at and use rather than read, such as a mock-up of a
page, two takes on a colour, or a diagram you can click through. Each
version is a whole file, never changed once written, and says who made it,
where (a mission, or by hand), what it's based on, and what changed.

Each is a folder, {date}-{slug}/, holding its record (artifact.toml) and
v1.html, v2.html, …, named a1, a2, … on this machine: a version is a3.v2.
ws show a3 shows one, with its versions, their notes and where each came
from.

They're on this machine, in .ws/artifacts. Crew's are recorded on their
mission, as one of its artifacts. That's ws mission artifact add's list,
which points at anything a mission made.

They're sketches, and most should go. A live one expires 30 days after its
last version ([artifacts] expire_after), and the daemon discards it. You
promote one worth keeping (ws artifact promote), which ends its expiry, and
discard one that isn't (ws artifact discard), which ws artifact restore
undoes.

A promoted one can live in a repo too, for everyone with it: the version
chosen, in a folder there ([artifacts] path in ws.toml says where, as
"records:artifacts/{date}-{slug}/", or you name one). Where it lives is
chosen the first time it's promoted, and kept: promote it again, versions
later, and the one chosen then goes to the same folder, in one commit
landed on the repo's default branch.`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newArtifactAddCmd(), newArtifactListCmd(), newArtifactShowCmd(), newArtifactPromoteCmd(), newArtifactDiscardCmd(), newArtifactRestoreCmd(), newArtifactClearCmd(), newArtifactTweaksCmd())
	return cmd
}

func newArtifactAddCmd() *cobra.Command {
	var title, note, to, on string
	var proposeIt bool
	cmd := &cobra.Command{
		Use:   "add <file>",
		Short: "Add an artifact from an HTML file (- for stdin), or a version of one (--to)",
		Long: `Add an artifact from one HTML file (- for stdin), with its CSS and JS
inline, as its first version; or, with --to a3, a new version of artifact
a3, based on its latest (--on v1 for another). Each version is a whole file.

Its title is --title, or the HTML's <title>. Its note (--note) says, in a
line or two, what changed and why; the first version's says what the
artifact tries out. Crew must give one.

A version is at most 1 MB, and runs in the web UI in a sandboxed frame that
fetches nothing and keeps nothing: ws warns of a script, font or image it
loads from elsewhere, a call to the network, and storage, so they can be
put in the file instead (images as data: URLs).

Where there's something to choose between (a density, a colour, two takes
on a layout), its page says what can be changed (ws.tweaks) and the web UI
draws the controls beside its frame, with the page's presets as buttons:
ws artifact tweaks says how.

Run by crew, it's recorded on their mission, and says it came from it.
Run by you, it's yours, by hand. Either way it's on this machine: a
version of one that lives in a repo too goes there when you next promote
it.

In a conversation, its agent proposes one instead (--propose, first), and
the user adds it, or not, with a button under its reply, where it runs in a
small frame. ws artifact add --propose, with no file, prints how one's
written and proposed.

  ws artifact add glow.html --title "Horizon glow" --note "Accent at 8% in dark, 3% in light."
  ws artifact add glow.html --to a3 --note "No glow in light: at 3% it looked grey."
  ws artifact add --propose --to a3 --on v1 --note "…" - <<'EOF'`,
		Args: func(cmd *cobra.Command, args []string) error {
			if proposeIt {
				return cobra.MaximumNArgs(1)(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			if proposeIt && len(args) == 0 {
				ctx, _ := loadOps()
				showArtifactGuide(ctx)
				return nil
			}
			if !proposeIt && agentConversation() != "" {
				return errors.New("you're in a conversation, which changes nothing: propose it instead (ws artifact add --propose, first, then the rest), and the user adds it with a button under your reply")
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if proposeIt {
				html, err := readHTML(args[0])
				if err != nil {
					return err
				}
				return proposeArtifact(ctx, html, title, note, strings.TrimSpace(to), on)
			}
			a := ops.ArtifactAdd{Title: title, Note: note, To: strings.TrimSpace(to), Mission: os.Getenv(mission.EnvMission)}
			if on != "" {
				if a.To == "" {
					return errors.New("--on says which version a new one is based on: give the artifact too (--to a3)")
				}
				n, ok := artifact.ParseVersion(on)
				if !ok {
					return fmt.Errorf("--on %s: say a version, as v2", on)
				}
				a.On = n
			}
			if a.HTML, err = readHTML(args[0]); err != nil {
				return err
			}
			added, err := ctx.AddArtifact(a, ops.ActorFromEnv())
			if err != nil && added.Version == 0 {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), added)
			for _, w := range added.Warnings {
				fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Orange.Render("⚠"), w)
			}
			fmt.Fprintf(ui.Stderr, "  %s ws show %s shows it, and ws artifact show %s prints it\n", ui.Dim.Render("·"), added.Entry.ID, added.Ref())
			fmt.Println(added.Ref())
			return err
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "its title (default: the HTML's <title>); with --to, a new title for the artifact")
	cmd.Flags().StringVar(&note, "note", "", "what this version changed and why, in a line or two (the first: what it tries out)")
	cmd.Flags().StringVar(&to, "to", "", "add a version to this artifact (a3) instead of a new artifact")
	cmd.Flags().StringVar(&on, "on", "", "with --to: the version it's based on, as v1 (default: the latest)")
	cmd.Flags().BoolVar(&proposeIt, "propose", false, "in a conversation: propose it, for the user to add, instead of adding it (with no file: show how one's written)")
	revisesFlag(cmd)
	return cmd
}

// readHTML is a version's HTML, from a file or stdin (-).
func readHTML(file string) ([]byte, error) {
	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(io.LimitReader(os.Stdin, artifact.MaxSize*4))
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return nil, fmt.Errorf("reading the artifact: %w", err)
	}
	return data, nil
}

func newArtifactListCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the workspace's artifacts, the newest version first",
		Long: `List the workspace's artifacts, the one with the newest version first:
its ref, its latest version, its state, its title, and who added its
latest version, where and when. --all lists discarded ones too.

A live one in its last week says when it expires (expires in 4 days), and
one that has says so (expired) until the daemon discards it, or
ws artifact clear does: ws artifact promote keeps one. A promoted one that
lives in a repo says where, and which version is there.`,
		Args:   cobra.NoArgs,
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			list, err := ctx.Artifacts().List(context.Background())
			if err != nil {
				return err
			}
			cfg, now := ctx.ArtifactsConfig(), time.Now()
			hidden, expired := 0, 0
			for _, e := range list {
				if e.Status == artifact.Discarded && !all {
					hidden++
					continue
				}
				if cfg.Expired(e, now) {
					expired++
				}
				fmt.Println(artifactListLine(e, cfg, now))
			}
			switch {
			case len(list) == 0:
				fmt.Fprintf(ui.Stderr, "  %s no artifacts: ws artifact add <file> adds one\n", ui.Dim.Render("·"))
			case hidden > 0:
				fmt.Fprintf(ui.Stderr, "  %s %d discarded: --all lists them too\n", ui.Dim.Render("·"), hidden)
			}
			if expired > 0 {
				fmt.Fprintf(ui.Stderr, "  %s %d expired: ws artifact clear clears them, and ws artifact promote keeps one\n", ui.Orange.Render("⚠"), expired)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "list discarded artifacts too")
	return cmd
}

// artifactListLine is an artifact on a line: a3 v2 live "Title" · by c1
// in mission-slug, 2026-11-12 18:52. A live one in its last week says when
// it expires, one expired says so, and one that lives in a repo says
// where, and which version is there.
func artifactListLine(e artifact.Entry, cfg artifact.Config, now time.Time) string {
	latest, _ := e.Latest()
	state := string(e.Status)
	switch {
	case e.Gone || e.Away():
		state = "gone"
	case cfg.Expired(e, now):
		state = "expired"
	}
	expiring := ""
	if at, ok := cfg.Expires(e); ok && cfg.Expiring(e, now) {
		expiring = " · " + artifact.ExpiresWords(at, now)
	}
	where := ""
	if h := e.Home; h != nil && e.Lives() != "" {
		where = fmt.Sprintf(" · v%d in %s", h.Version, h.Where())
	}
	return fmt.Sprintf("%-4s v%-3d %-9s %s · %s, %s%s%s", e.ID, latest.N, state, e.Title, provenance(latest), when(latest.At), where, expiring)
}

func newArtifactShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <a3|a3.v2>",
		Short: "Print a version of an artifact: its HTML (the latest, for a3)",
		Long: `Print a version of an artifact, its HTML as it was added: a3.v2 for v2,
or a3 for the latest. ws show a3 says what each version is: its note,
what it's based on, and where it came from.`,
		Args:   cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			id, n, ok := artifact.ParseRef(args[0])
			if !ok {
				return fmt.Errorf("%q isn't an artifact: they're named a1, a2, … and a version a3.v2 (ws artifact list)", args[0])
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			text, _, _, err := ctx.Artifacts().ReadVersion(context.Background(), id, n)
			if err != nil {
				return err
			}
			_, err = os.Stdout.Write(text)
			return err
		},
	}
}

func newArtifactPromoteCmd() *cobra.Command {
	var version int
	var into, start, home string
	var here bool
	cmd := &cobra.Command{
		Use:   "promote <a3|a3.v2>",
		Short: "Keep an artifact for good: the version chosen, what it went into, and where it lives (the user only)",
		Long: `Promote an artifact worth keeping: it expires no more, and says which version
was chosen (a3.v2, or --version 2; its latest for neither) and what it went
into, if anything:

  --into <slug>        a mission: it's recorded on it, so its crew read it
  --into <repo:path>   a document, which cites it by its folder in a repo
  --start "<what>"     a new mission started from it, waiting to be briefed,
                       with it recorded on it

A promoted one can live in a repo, for everyone with it: the version chosen
(index.html) and a record of it (artifact.toml), in a folder there, in one
commit landed on the repo's default branch. Where it lives is chosen the
first time it's promoted:

  (neither)                 where [artifacts] path in ws.toml says promoted
                            ones live; with none, it stays on this machine
  --home <repo:folder/>     that folder
  --here                    on this machine only, whatever ws.toml says

and kept in its record. Promote it again, versions later, and the version
chosen then goes to the same folder, with nothing asked: ws show a3 says
which version is there. Its other versions stay on this machine. A document
can cite only one that lives in a repo.

A promoted one still takes new versions. Promote it again to choose
another, or say what it went into now. Only you promote one: crew say in
their handoff which they'd keep.`,
		Args:   cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			done, err := ctx.PromoteArtifact(ops.ArtifactPromote{Ref: args[0], Version: version, Into: into, Start: start, Home: home, Here: here}, ops.ActorFromEnv())
			if err != nil && done.Entry.ID == "" {
				return err
			}
			e := done.Entry
			line := fmt.Sprintf("Promoted %s, %q: v%d chosen", e.ID, e.Title, e.Promoted.Version)
			if done.Into != "" {
				line += ", into " + done.Into
			}
			if done.Home != "" {
				line += fmt.Sprintf(", and put in %s: %s", done.Home, done.Landed)
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), line)
			switch {
			case done.Started != "":
				fmt.Fprintf(ui.Stderr, "  %s started mission %s from it, recorded on it: Brief it on its page in the web UI (ws web)\n", ui.Dim.Render("·"), done.Started)
			case done.Recorded != "":
				fmt.Fprintf(ui.Stderr, "  %s recorded it on %s, among its artifacts\n", ui.Dim.Render("·"), done.Recorded)
			}
			switch {
			case done.First:
				fmt.Fprintf(ui.Stderr, "  %s it lives there from now on: promoting it again puts the version chosen then in the same folder\n", ui.Dim.Render("·"))
			case done.Home == "":
				fmt.Fprintf(ui.Stderr, "  %s it's on this machine only: --home <repo:folder/> gives it a folder in a repo, for everyone with it\n", ui.Dim.Render("·"))
			}
			return err
		},
	}
	cmd.Flags().IntVar(&version, "version", 0, "the version chosen (default: its latest, or the one the ref names)")
	cmd.Flags().StringVar(&into, "into", "", "what it went into: a mission's slug, or a document's ref (repo:path)")
	cmd.Flags().StringVar(&start, "start", "", "start a mission from it, saying what the mission is")
	cmd.Flags().StringVar(&home, "home", "", "the first time: the folder in a repo it's to live in, as repo:folder/ (default: where [artifacts] path says)")
	cmd.Flags().BoolVar(&here, "here", false, "the first time: keep it on this machine only, whatever [artifacts] path says")
	return cmd
}

func newArtifactDiscardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discard <a3> [why]",
		Short: "Discard an artifact, saying why (the user only)",
		Long: `Discard an artifact, with its versions, saying why if you like. It's listed
no more (ws artifact list --all lists it), and ws show a3 still says what it
was, where it came from, and why it was discarded.

Its folder moves to .ws/artifacts/.discarded/, and is deleted for good 30
days later. Where it lives in a repo, a commit takes its folder there away
first, landed on the repo's default branch. ws artifact restore puts it
back, there too. Only you discard one.`,
		Args:   cobra.MinimumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			done, err := ctx.DiscardArtifact(args[0], strings.Join(args[1:], " "), ops.ActorFromEnv())
			if err != nil {
				return err
			}
			e := done.Entry
			out := ""
			if done.Home != "" {
				out = fmt.Sprintf(" It's out of %s too: %s.", done.Home, done.Landed)
			}
			fmt.Fprintf(ui.Stderr, "  %s Discarded %s, %q: its folder is in %s for %d days, ws show %s still says what it was, and ws artifact restore %s puts it back.%s\n", ui.Green.Render("✓"), e.ID, e.Title, e.Where(), int(artifact.KeepDiscarded.Hours()/24), e.ID, e.ID, out)
			return nil
		},
	}
}

func newArtifactRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <a3>",
		Short: "Put a discarded artifact back (the user only)",
		Long: `Put a discarded artifact back, as it was (live, or promoted if it had been),
with its ref: its folder out of .ws/artifacts/.discarded/, while it's there
(30 days). Where it lived in a repo, the version that was there goes back,
in a commit landed on the repo's default branch. Its expiry counts from
now.`,
		Args:   cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			done, err := ctx.RestoreArtifact(args[0], ops.ActorFromEnv())
			if err != nil && done.Entry.ID == "" {
				return err
			}
			e := done.Entry
			line := fmt.Sprintf("Restored %s, %q, to %s (%s)", e.ID, e.Title, e.Where(), e.Status)
			if done.Home != "" {
				line += fmt.Sprintf(", and put back in %s: %s", done.Home, done.Landed)
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), line)
			return err
		},
	}
}

func newArtifactClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear the expired artifacts now (the user only)",
		Long: `Clear the expired artifacts: live ones with no new version for 30 days
([artifacts] expire_after). Each is discarded, as the daemon does each
day: listed as discarded, "expired", and ws artifact restore puts one
back.`,
		Args:   cobra.NoArgs,
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			done, err := ctx.ClearExpiredArtifacts(ops.ActorFromEnv())
			for _, e := range done.Local {
				fmt.Fprintf(ui.Stderr, "  %s Discarded %s, %q, expired: its folder is in %s\n", ui.Green.Render("✓"), e.ID, e.Title, e.Where())
			}
			if err != nil {
				return err
			}
			if len(done.Local) == 0 {
				fmt.Fprintf(ui.Stderr, "  %s nothing's expired\n", ui.Dim.Render("·"))
			}
			return nil
		},
	}
}
