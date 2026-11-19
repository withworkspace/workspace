package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws doc add: a document of one of the workspace's kinds, added where its
// kind says, named from its title, with its kind's commit message. It's the
// user's: a conversation's agent proposes one (--propose), and the user
// adds it with a button under its reply; crew write theirs in their
// capsule.
//
// It's a noun of its own, not under ws documents, which a conversation's
// agent may run to read: its allow list admits ws doc add --propose and
// ws doc amend --propose (doc_amend.go), and nothing else of it.
//
// A kind's guide says how one's written, and an agent proposing one is
// shown it before anything's proposed: ws doc add --propose <kind>, with no
// file, prints it and its version, and a proposal of a kind with a guide
// carries that version (--guide). One without it, or with an old one, is
// refused, with the guide printed. So the guide's text has been in what
// the agent read, whatever it was told, and all through the one command
// its allow list has.

func newDocCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "Add a document of one of the workspace's kinds, or change one",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newDocAddCmd(), newDocAmendCmd())
	return cmd
}

func newDocAddCmd() *cobra.Command {
	var proposeIt, dry bool
	var guide string
	cmd := &cobra.Command{
		Use:   "add <kind> [file]",
		Short: "Add a document of one of the workspace's kinds, where its kind says",
		Long: `Add a document of one of the workspace's kinds (ws show kinds) from a
markdown file (- for stdin). Its first heading is its title: it's named
from it, where its kind says, and committed with its kind's message, on
its repo's default branch. In a repo with no pull requests it's landed
(ws land: the repo's check runs first); in one that takes them, it's
committed in a capsule lifted for it (add-<slug>) and left, to open a pull
request from.

In a conversation, its agent proposes one instead (--propose), and the
user adds it, or not, with a button under its reply. First,
ws doc add --propose <kind>, with no file, prints how one of the kind is
written (its guide) and the --guide to propose it with: a proposal of a
kind with a guide is refused without it.

  ws doc add plan notes.md
  ws doc add --propose doc
  ws doc add --propose doc --guide 3f2a9c1b04de - <<'EOF'
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
			kind := args[0]
			g, err := ctx.KindGuideOf(kind)
			if err != nil {
				return err
			}
			if proposeIt && len(args) == 1 {
				showGuide(g)
				return nil
			}
			if len(args) == 1 {
				return errors.New("say the document's file (- for stdin)")
			}
			if proposeIt && g.Version != "" && strings.TrimSpace(guide) != g.Version {
				showGuide(g)
				if guide == "" {
					return fmt.Errorf("not proposed: a document of the %s kind is written as its guide says (above). Write it so, then propose it with --guide %s", kind, g.Version)
				}
				return fmt.Errorf("not proposed: the guide has changed since the version you give (--guide %s). Read it as it is now (above), write it so, then propose it with --guide %s", guide, g.Version)
			}
			text, err := readDocText(args[1])
			if err != nil {
				return err
			}
			// Where it would go and what it would be called, as the ground
			// has it now: said before anything's proposed or added.
			where, err := ctx.AddDocument(ops.DocAdd{Kind: kind, Text: text, Dry: true})
			if err != nil {
				return err
			}
			if where.Taken {
				return fmt.Errorf("there's a %s already: give it another title (its first heading)", where.Ref)
			}
			if proposeIt {
				p := conversation.Proposal{Kind: conversation.ProposeDocument, DocKind: where.Kind, Title: where.Title, Text: text, Path: where.Ref, Subject: where.Subject}
				p.Guide = g.Name
				if err := propose(ctx, p); err != nil {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s confirmed, it's added as %s, committed as %q\n", ui.Dim.Render("·"), where.Ref, where.Subject)
				return nil
			}
			if err := refuseInConversation("a document"); err != nil {
				return err
			}
			if who := raisedBy(); who != "you" {
				repo, rel, _ := strings.Cut(where.Ref, ":")
				return fmt.Errorf("only the user adds a document to a default branch, not %s: write it in a capsule of %s, at %s, and commit it as %q", who, repo, rel, where.Subject)
			}
			if dry {
				how := "landed on " + where.Branch
				if where.How != "land" {
					how = "committed in a capsule lifted for it, and left for you to open a pull request from"
				}
				fmt.Fprintf(ui.Stderr, "  %s would add %s (%q), committed as %q, %s\n", ui.Dim.Render("·"), where.Ref, where.Title, where.Subject, how)
				return nil
			}
			added, err := ctx.AddDocument(ops.DocAdd{Kind: kind, Text: text, Why: "Added with ws doc add."})
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s Added %s%s\n", ui.Green.Render("✓"), added.Ref, strings.TrimPrefix(added.Message, "Added"))
			fmt.Println(added.Ref)
			return nil
		},
	}
	cmd.Flags().BoolVar(&proposeIt, "propose", false, "in a conversation: propose it, for the user to add, instead of adding it (with no file: show how one of the kind is written)")
	revisesFlag(cmd)
	cmd.Flags().StringVar(&guide, "guide", "", "with --propose: the version of the kind's guide it's written to, as ws doc add --propose <kind> prints it")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "say where it would go and what it would be committed as, and add nothing")
	return cmd
}

// showGuide prints how a kind's documents are written, for an agent about
// to propose one: its guide's text, where one goes and how it's named, and
// how to propose it (with the --guide that shows the guide was read).
func showGuide(g *ops.DocGuide) {
	propose := "ws doc add --propose " + g.Kind
	if g.Version == "" {
		fmt.Printf("The %s kind (%s) has no guide: write one plainly.\n", g.Title, g.Kind)
	} else {
		propose += " --guide " + g.Version
		fmt.Printf("The %s kind (%s): how one's written, from its guide %s (version %s):\n\n", g.Title, g.Kind, g.Name, g.Version)
		fmt.Println("-----")
		fmt.Print(strings.TrimRight(g.Text, "\n") + "\n")
		if g.Clipped {
			fmt.Printf("… (cut short: the rest is in %s, read it)\n", g.Path)
		}
		fmt.Println("-----")
	}
	fmt.Printf("\nIt's added at %s, {slug} made from its title: its first heading (# …), which is all it's named from.", g.Where)
	if g.Numbered {
		fmt.Print(" {number} is filled in when it's added: leave any number out of its title.")
	}
	if g.Add != "" {
		fmt.Printf(" It's committed as %q.", g.Add)
	}
	fmt.Printf("\n\nWrite it as that says, and propose it with\n\n  %s - <<'EOF'\n  …\n  EOF\n", propose)
}

// readDocText is a document's text from a file, or stdin for -.
func readDocText(file string) (string, error) {
	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return "", fmt.Errorf("reading the document: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", errors.New("the document has nothing in it")
	}
	return string(data), nil
}
