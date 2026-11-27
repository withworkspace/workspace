package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws config edit: a change to ws's config (ws.toml, or a repo's
// ws.repo.toml), as the whole of the file as it should read, opened as a
// pull request (ops.EditConfig). It's the user's: a conversation's agent
// proposes one (--propose), and the user confirms it with a button under
// its reply that shows what it changes.
//
// As with an edit to a document, the agent is shown the file as it is
// first (--propose with no file), and proposes with the version it read
// (--on): so the change is made from the file as ws has it, and is merged
// from that if the file changes before it's accepted.

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Change ws's config by a pull request",
		Long: `Change ws's config: the team's ws.toml, or a repo's ws.repo.toml. Both are
committed, so a change is a pull request: ws show config says what each
says now, and what every key is for.`,
	}
	cmd.AddCommand(newConfigEditCmd())
	return cmd
}

func newConfigEditCmd() *cobra.Command {
	var proposeIt bool
	var on, title, why string
	cmd := &cobra.Command{
		Use:   "edit <file> [text]",
		Short: "Change ws.toml or a repo's ws.repo.toml, as a pull request",
		Long: `Change ws.toml, or a repo's ws.repo.toml (<repo>:ws.repo.toml), to the
whole of a file as it should read (- for stdin), as a pull request: a
repo's from a capsule lifted for it (config-<title>), ws.toml's from a
branch of the workspace's config repo. A repo with no pull requests has it
committed in the capsule, for you to land. ws checks the file as ws would
read it first: TOML that doesn't parse, or a key ws doesn't read, is
refused. What changed in the file since is merged in.

ws.local.toml is yours and committed nowhere: change it in Settings, or by
hand.

In a conversation, its agent proposes the change instead (--propose), and
the user opens it, or not, with a button under its reply that shows what it
changes. First, ws config edit --propose <file>, with no text, prints the
file as it is and the --on to propose with.

  ws config edit --propose web:ws.repo.toml
  ws config edit --propose web:ws.repo.toml --on 3f2a9c1b04de \
    --title "Run web's dev server and tests with ws run" \
    --why "Crew start the dev server on a free port, and checks run the tests." - <<'EOF'
  [run.service.web]
  …
  EOF`,
		Args:              cobra.RangeArgs(1, 2),
		PreRun:            func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			f, err := ctx.ConfigToEdit(args[0])
			if err != nil {
				return err
			}
			if proposeIt && len(args) == 1 {
				showConfigEditing(f)
				return nil
			}
			if len(args) == 1 {
				return errors.New("say the file's text, the whole of it as it should read (- for stdin)")
			}
			if proposeIt {
				switch on = strings.TrimSpace(on); on {
				case f.Version:
				case "":
					showConfigEditing(f)
					return fmt.Errorf("not proposed: a change is made from the file as ws shows it (above), and proposed with the version it was made from: --on %s", f.Version)
				default:
					showConfigEditing(f)
					return fmt.Errorf("not proposed: %s has changed since the version you give (--on %s). Make your change to it as it is now (above), and propose it with --on %s", f.Ref, on, f.Version)
				}
			}
			text, err := readDocText(args[1])
			if err != nil {
				return err
			}
			if strings.TrimSpace(title) == "" {
				return errors.New("give the pull request a title (--title): what the change sets up, in a line")
			}
			if err := ops.CheckConfig(f, text); err != nil {
				return fmt.Errorf("not %s: %w", map[bool]string{true: "proposed", false: "changed"}[proposeIt], err)
			}
			if proposeIt {
				p := conversation.Proposal{Kind: conversation.ProposeConfig, Path: f.Ref, Was: f.Text, Text: text,
					Title: strings.TrimSpace(title), Why: strings.TrimSpace(why), Lands: f.Lands, Where: f.Where}
				if err := propose(ctx, p); err != nil {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s confirmed, it's %s\n", ui.Dim.Render("·"), f.Where)
				return nil
			}
			if err := refuseInConversation("a change to ws's config"); err != nil {
				return err
			}
			if who := raisedBy(); who != "you" {
				return fmt.Errorf("only the user changes ws's config this way, not %s", who)
			}
			body := strings.TrimSpace(why)
			if body == "" {
				body = "Changed with ws config edit."
			}
			made, err := ctx.EditConfig(f.Ref, f.Text, text, title, body)
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), made.Message)
			return nil
		},
	}
	cmd.Flags().BoolVar(&proposeIt, "propose", false, "in a conversation: propose the change, for the user to open as a pull request, instead of opening it (with no text: show the file as it is, to change)")
	revisesFlag(cmd)
	cmd.Flags().StringVar(&on, "on", "", "with --propose: the version of the file the change was made from, as ws config edit --propose <file> prints it")
	cmd.Flags().StringVar(&title, "title", "", "the pull request's title, and its commit's: what the change sets up, in a line")
	cmd.Flags().StringVar(&why, "why", "", "what the change gets the user, in a sentence or two: the pull request's description")
	return cmd
}

// showConfigEditing prints a config file as it is, for an agent about to
// propose a change to it, and how to propose it (with the --on that shows
// it was read).
func showConfigEditing(f *ops.ConfigFile) {
	if f.Version == ops.NewFileVersion {
		fmt.Printf("%s isn't there yet (version %s): what you propose is the whole of it.\n", f.Ref, f.Version)
	} else {
		fmt.Printf("%s as it is now (version %s):\n\n-----\n%s\n-----\n", f.Ref, f.Version, strings.TrimRight(f.Text, "\n"))
	}
	fmt.Printf("\nWrite the whole of it as it should read, from the above (keep what's there that isn't changing, and its comments), and propose it with\n\n  ws config edit --propose %s --on %s --title \"<what it sets up, in a line>\" --why \"<what it gets the user>\" - <<'EOF'\n  …\n  EOF\n\n", f.Ref, f.Version)
	fmt.Printf("ws checks it as ws would read it: a key ws doesn't read is refused (ws show config lists every key, and its file). The user sees what it changes. Confirmed, it's %s.\n", f.Where)
}
