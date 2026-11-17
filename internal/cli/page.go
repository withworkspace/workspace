package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/ui"
)

// Pages: documents you write in the web UI, with cells an agent fills in
// while you write on. A cell is one of the workspace's conversations
// (conversation.Cell): what you asked for is its first message, and what
// its agent replies is what the cell holds, written into the page by the
// browser.

func newPageCmd() *cobra.Command {
	var printOnly bool
	cmd := &cobra.Command{
		Use:     "page [title]",
		Aliases: []string{"pages"},
		Short:   "Write a page in the web UI, with cells an agent fills in",
		Long: `Open the workspace's pages in the web UI; with a title, start a page
and open it. A page is a document you write: start a line with >> and say
what you want there (>> a table of the existing routes), and an agent
fills it in while you write on. Click a cell to ask for it to be changed.

A page's text is a markdown file in .ws/pages (p1.md, p2.md, …), local to
your machine; what it is beside that is in the workspace's records. ws page
list lists them, and ws show p1 shows one.`,
		Args:   cobra.ArbitraryArgs,
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			path := "/documents"
			if title := strings.Join(strings.Fields(strings.Join(args, " ")), " "); title != "" {
				p, err := (page.Store{Root: ctx.WS().Root}).New([]byte("# " + title + "\n"))
				if err != nil {
					return err
				}
				path = "/pages/" + p.ID
				fmt.Fprintf(ui.Stderr, "  %s started %s\n", ui.Green.Render("✓"), p.ID)
			}
			url, err := webURL(ctx.WS().Root, path)
			if err != nil {
				return err
			}
			if printOnly {
				fmt.Println(url)
				return nil
			}
			if err := openBrowser(url); err != nil {
				return fmt.Errorf("opening your browser: %w (ws page --print prints the link)", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the link instead of opening it")
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the workspace's pages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			store := page.Store{Root: ctx.WS().Root}
			list, err := store.List()
			if err != nil {
				return err
			}
			for _, p := range list {
				fmt.Printf("%-4s %s  %s\n", p.ID, p.Title, store.Path(p.ID))
			}
			if len(list) == 0 {
				fmt.Fprintf(ui.Stderr, "  %s no pages: ws page \"…\" starts one\n", ui.Dim.Render("·"))
			}
			return nil
		},
	})
	return cmd
}
