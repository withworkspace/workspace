package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws pin, ws unpin: your focus, from the command line. The web UI's
// sidebar shows it; this keeps it as well.

func newPinCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pin [ref...]",
		Short: "Pin what you're on to your focus, or list what's pinned",
		Long: `Pin what you're on to your focus: anything ws names, by its name (a mission,
a conversation w4, a pull request demo#12, a page p3, an artifact a3, a
document records/docs/plan.md, a plugin's thing RAM-123). The web UI keeps
your pins in the sidebar, each as it stands, and one that ends (a mission
landed, a conversation resolved) leaves it by itself. With no ref, it lists
what's pinned.

Pins are yours: crew and the orchestrator can't pin.

  ws pin RAM-123 demo#12
  ws pin`,
		PersistentPreRun:  func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadOps()
			if err != nil {
				return err
			}
			for _, name := range args {
				p, err := w.Pin(ops.ActorFromEnv(), name)
				if err != nil {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Dim.Render("·"), p)
			}
			if len(args) > 0 {
				return nil
			}
			items, err := w.Focus()
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Fprintf(ui.Stderr, "  %s nothing's pinned\n", ui.Dim.Render("·"))
				return nil
			}
			for _, it := range items {
				if it.Title != "" {
					fmt.Printf("%s\t%s\n", it.Name, it.Title)
				} else {
					fmt.Println(it.Name)
				}
			}
			return nil
		},
	}
}

func newUnpinCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unpin <ref...>",
		Short: "Take something out of your focus",
		Long: `Take something out of your focus, by the name it was pinned by (ws pin lists
them).

  ws unpin RAM-123`,
		Args:              cobra.MinimumNArgs(1),
		PersistentPreRun:  func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadOps()
			if err != nil {
				return err
			}
			for _, name := range args {
				u, err := w.Unpin(ops.ActorFromEnv(), name)
				if err != nil {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Dim.Render("·"), u)
			}
			return nil
		},
	}
}
