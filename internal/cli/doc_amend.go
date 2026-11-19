package cli

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// ws doc amend: an edit to a document of one of the workspace's kinds,
// saved where it is, as an edit on its page is, with its kind's amend
// message. It's the user's: a conversation's agent proposes one
// (--propose), and the user saves it with a button under its reply.
//
// Without it, an agent that had proposed a document and seen it added
// had no way to change it: asked to, it agreed, then found ws doc add
// refusing the title it had, and proposed a new document instead.
//
// An agent proposing an edit is shown the document as it is first, as one
// proposing a document is shown its kind's guide: ws doc amend --propose
// <ref>, with no file, prints it, its kind's guide, and its version, which
// the edit is proposed with (--on). One without it, or with an old one, is
// refused, with the document as it is now printed. So the edit is made
// from text the agent read through ws, and the version is what accepting
// it merges from, if the document's changed by then.

func newDocAmendCmd() *cobra.Command {
	var proposeIt bool
	var on string
	cmd := &cobra.Command{
		Use:   "amend <ref> [file]",
		Short: "Change a document of one of the workspace's kinds, where it is",
		Long: `Change a document of one of the workspace's kinds (ws show kinds), named
by its ref (records:docs/todos.md; repo/capsule:path in a capsule), to the
whole of a markdown file (- for stdin). It's saved as an edit on its page
is, committed with its kind's amend message: in a capsule, there; on the
default branch of a repo with no pull requests, landed (ws land: the
repo's check runs first); on one that takes them, in a capsule lifted for
it (edit-<name>) and left, to open a pull request from. It stays where it
is, whatever its first heading says.

In a conversation, its agent proposes the edit instead (--propose), and
the user saves it, or not, with a button under its reply that shows what
it changes. First, ws doc amend --propose <ref>, with no file, prints the
document as it is, its kind's guide, and the --on to propose with: the
version that was read, which the edit is merged from if the document
changes before it's accepted.

  ws doc amend records:docs/todos.md todos.md
  ws doc amend --propose records:docs/todos.md
  ws doc amend --propose records:docs/todos.md --on 3f2a9c1b04de - <<'EOF'
  # Todos
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
			a, err := ctx.DocToAmend(args[0])
			if err != nil {
				return err
			}
			if proposeIt && len(args) == 1 {
				showAmending(a)
				return nil
			}
			if len(args) == 1 {
				return errors.New("say the document's file, the whole of it as it should read (- for stdin)")
			}
			if proposeIt {
				switch on = strings.TrimSpace(on); on {
				case a.Version:
				case "":
					showAmending(a)
					return fmt.Errorf("not proposed: an edit is made from the document as ws shows it (above), and proposed with the version it was made from: --on %s", a.Version)
				default:
					showAmending(a)
					return fmt.Errorf("not proposed: %s has changed since the version you give (--on %s). Make your edit to it as it is now (above), and propose it with --on %s", a.Ref, on, a.Version)
				}
			}
			text, err := readDocText(args[1])
			if err != nil {
				return err
			}
			if proposeIt {
				p := conversation.Proposal{Kind: conversation.ProposeDocument, DocKind: a.Kind, To: a.Ref, Path: a.Ref, Was: a.Text, Text: text,
					Title: cmp.Or(workspace.DocTitle([]byte(text)), a.Title), Subject: a.Saves.Subject}
				if a.Guide.Version != "" {
					p.Guide = a.Guide.Name
				}
				if err := propose(ctx, p); err != nil {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s confirmed, it's %s, committed as %q\n", ui.Dim.Render("·"), amendSaves(a), a.Saves.Subject)
				return nil
			}
			if err := refuseInConversation("an edit to a document"); err != nil {
				return err
			}
			if who := raisedBy(); who != "you" {
				return fmt.Errorf("only the user changes a document this way, not %s: change it in a capsule of its repo, and commit it as %q", who, a.Saves.Subject)
			}
			saved, err := ctx.AmendDocument(a.Ref, a.Text, text, "Amended with ws doc amend.")
			if err != nil {
				return err
			}
			if saved.Conflict != nil { // changed while this ran
				return fmt.Errorf("%s changed as it was being saved, in lines this changes too: nothing's saved. Run it again", a.Ref)
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), saved.Message)
			return nil
		},
	}
	cmd.Flags().BoolVar(&proposeIt, "propose", false, "in a conversation: propose the edit, for the user to save, instead of saving it (with no file: show the document as it is, to edit)")
	revisesFlag(cmd)
	cmd.Flags().StringVar(&on, "on", "", "with --propose: the version of the document the edit was made from, as ws doc amend --propose <ref> prints it")
	return cmd
}

// showAmending prints a document as it is, for an agent about to propose
// an edit to it: its text, its kind's guide, and how to propose the edit
// (with the --on that shows it was read).
func showAmending(a *ops.DocAmend) {
	fmt.Printf("%s, of the kind %s (%s), as it is now (version %s):\n\n", a.Ref, a.Guide.Title, a.Kind, a.Version)
	fmt.Println("-----")
	fmt.Print(strings.TrimRight(a.Text, "\n") + "\n")
	fmt.Println("-----")
	if g := a.Guide; g.Version != "" {
		fmt.Printf("\nHow one of the kind is written, from its guide %s:\n\n", g.Name)
		fmt.Println("-----")
		fmt.Print(strings.TrimRight(g.Text, "\n") + "\n")
		if g.Clipped {
			fmt.Printf("… (cut short: the rest is in %s, read it)\n", g.Path)
		}
		fmt.Println("-----")
	}
	fmt.Printf("\nWrite the whole of it as it should read, from the above, and propose it with\n\n  ws doc amend --propose %s --on %s - <<'EOF'\n  …\n  EOF\n\n", a.Ref, a.Version)
	fmt.Printf("The user sees what it changes. Confirmed, it's %s, committed as %q. It stays where it is, whatever its first heading says.\n", amendSaves(a), a.Saves.Subject)
}

// amendSaves is where an edit to a document is saved, in words.
func amendSaves(a *ops.DocAmend) string {
	switch a.Saves.How {
	case "land":
		return "landed on " + a.Saves.Branch
	case "lift":
		return "committed in a capsule lifted for it, for the user to open a pull request from"
	}
	return "committed where it is"
}
