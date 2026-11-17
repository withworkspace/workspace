package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// Suggested edits in threads: the thread agent (or crew, or you) suggests
// replacing text in the document's markdown, and you accept it, which
// writes it into the document (committing it, in a capsule), or decline
// it.

func newThreadSuggestCmd() *cobra.Command {
	var slug, from, to string
	cmd := &cobra.Command{
		Use:   "suggest <thread>",
		Short: "Suggest an edit to the document a thread is on",
		Long: `Suggest replacing text in the document's markdown: the text as it is,
exactly as it appears in the file (it must appear there once), and what it
should read. The user accepts it (ws writes it in, and commits it if the
document is in a capsule and has no other uncommitted changes) or
declines it. A newer suggestion replaces one
still waiting.

Give the two as flags, or on stdin between conflict markers:

  ws thread suggest t3 <<'EOF'
  <<<<<<<
  Ship behind a flag on Friday.
  =======
  Ship behind a flag on Tuesday.
  >>>>>>>
  EOF`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if conversation.IsWorkspaceRef(args[0]) {
				return errWorkspaceThread(args[0])
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("from") {
				if from, to, err = readSuggestion(os.Stdin); err != nil {
					return err
				}
			}
			store := threadStore(ctx, m)
			c, err := store.Load(args[0])
			if err != nil {
				return err
			}
			text, err := ctx.DocNow(m, c.Anchor.Doc)
			if err != nil {
				return err
			}
			if _, err := ctx.DocWhere(m, c.Anchor.Doc); err != nil {
				return err
			}
			by := raisedBy()
			if by == "you" {
				by = conversation.You
			}
			if _, err := store.Suggest(c.ID, conversation.Suggestion{From: from, To: to, By: by, At: time.Now()}, text); err != nil {
				return err
			}
			store.KeepVersion(text)
			fmt.Fprintf(ui.Stderr, "  %s suggested in %s: the user accepts or declines it\n", ui.Green.Render("✓"), c.ID)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&from, "from", "", "the markdown to replace, exactly as it is in the file")
	cmd.Flags().StringVar(&to, "to", "", "what it should read instead (\"\" deletes it)")
	return cmd
}

// readSuggestion reads a suggestion between conflict markers: the text to
// replace, =======, then what it should read.
func readSuggestion(r io.Reader) (from, to string, err error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	var a, b []string
	part := 0 // 0 before <<<<<<<, 1 the old text, 2 the new, 3 done
	for sc.Scan() {
		l := sc.Text()
		switch {
		case part == 0 && strings.HasPrefix(l, "<<<<<<<"):
			part = 1
		case part == 1 && strings.HasPrefix(l, "======="):
			part = 2
		case part == 2 && strings.HasPrefix(l, ">>>>>>>"):
			part = 3
		case part == 1:
			a = append(a, l)
		case part == 2:
			b = append(b, l)
		}
	}
	if part != 3 {
		return "", "", errors.New("give the suggestion as flags (--from, --to), or on stdin between <<<<<<<, ======= and >>>>>>> lines")
	}
	return strings.Join(a, "\n"), strings.Join(b, "\n"), sc.Err()
}

func newThreadSettleCmd(state string) *cobra.Command {
	var slug string
	use, short := "accept <thread>", "Accept a thread's suggested edit: write it in (the user only)"
	if state == conversation.SuggestDeclined {
		use, short = "decline <thread>", "Decline a thread's suggested edit (the user only)"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if conversation.IsWorkspaceRef(args[0]) {
				return errWorkspaceThread(args[0])
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			if raisedBy() != "you" {
				return errors.New("only the user accepts or declines a suggestion")
			}
			if state == conversation.SuggestDeclined {
				if _, err := ctx.DeclineSuggestion(doc.OfMission(m.Slug), args[0], ops.AsYou); err != nil {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s declined: say why in the thread\n", ui.Green.Render("✓"))
				return nil
			}
			said, err := ctx.AcceptSuggestion(m.Slug, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), said)
			return nil
		},
	}
	addMissionFlag(cmd, &slug)
	return cmd
}

// errWorkspaceThread refuses a change to a thread on a file outside any
// mission. Such files are edited as pages now, where questions and changes
// are asked; the threads made on them before are kept to read (ws show).
func errWorkspaceThread(id string) error {
	return fmt.Errorf("%s is on a file outside any mission, and is kept to read (ws show %s): the file is edited as a page now, and asked about there", id, id)
}
