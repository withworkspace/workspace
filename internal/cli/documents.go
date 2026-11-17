package cli

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// ws documents: the workspace's spaces of documents (a repo of them, or a
// folder of one: [repos.<name>] documents in ws.toml, and the records repo
// without saying), read from each one's default branch into the
// workspace's records.

func newDocumentsCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "documents [space]",
		Short: "List the workspace's spaces of documents, or one space's documents by folder",
		Long: `The workspace's spaces of documents: a repo of them (documents = true under
[repos.<name>] in ws.toml, or in the repo's own ws.repo.toml), a folder of
one (documents = "docs/"), and the records repo ([missions] records) unless
it says documents = false. Each is read from its default branch, as last
pulled: a document's title, its sections, and what it cites.

With a space (its repo's name), its documents by folder, each by its title.
ws documents search finds documents by what they say.`,
		Args:              cobra.MaximumNArgs(1),
		PersistentPreRun:  func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkFormat(format); err != nil {
				return err
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			db, _, err := ctx.Spaces(cmd.Context())
			if db == nil {
				return err
			}
			warnSpaces(err)
			out := docsOut(format)
			if len(args) == 0 {
				spaces := ctx.WS().Spaces()
				if len(spaces) == 0 {
					fmt.Fprintln(out, "No spaces of documents: say documents = true under a repo in ws.toml, or set [missions] records.")
					return nil
				}
				for _, s := range spaces {
					docs, err := space.Docs(cmd.Context(), db, s.Repo)
					if err != nil {
						return err
					}
					fmt.Fprintf(out, "%s  %s\n", spaceName(ctx.WS(), s, format), docsStyle(format, ui.Dim).Render(fmt.Sprintf("%d %s", len(docs), plural(len(docs), "document"))))
				}
				return nil
			}
			repo, ok := ctx.WS().ResolveAlias(args[0])
			if !ok {
				return fmt.Errorf("%s isn't one of the workspace's repos", args[0])
			}
			s, ok := ctx.WS().SpaceOf(repo)
			if !ok {
				return fmt.Errorf("%s isn't a space of documents: say documents = true under [repos.%s] in ws.toml", repo, repo)
			}
			docs, err := space.Docs(cmd.Context(), db, repo)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "%s  %s\n", spaceName(ctx.WS(), s, format), docsStyle(format, ui.Dim).Render(fmt.Sprintf("%d %s", len(docs), plural(len(docs), "document"))))
			byFolder := map[string][]space.Doc{}
			for _, d := range docs {
				dir := path.Dir(d.Path)
				byFolder[dir] = append(byFolder[dir], d)
			}
			for _, dir := range slices.Sorted(func(yield func(string) bool) {
				for k := range byFolder {
					if !yield(k) {
						return
					}
				}
			}) {
				name := dir + "/"
				if dir == "." {
					name = "(top)"
				}
				fmt.Fprintf(out, "\n  %s\n", docsStyle(format, ui.Bold).Render(name))
				ds := byFolder[dir]
				slices.SortFunc(ds, func(a, b space.Doc) int { return cmp.Compare(a.Path, b.Path) })
				for _, d := range ds {
					fmt.Fprintf(out, "    %s  %s\n", d.Title, docsStyle(format, ui.Dim).Render(repo+":"+d.Path))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "", "text (the default) or llm (plain)")
	cmd.AddCommand(newDocumentsSearchCmd())
	return cmd
}

func newDocumentsSearchCmd() *cobra.Command {
	var (
		format string
		spaces []string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "search <words>...",
		Short: "Find documents in the workspace's spaces by what they say, best first",
		Long: `Find documents in the workspace's spaces by what they say: each one found once,
led to the section that says it best, best first. A document whose title or
section heading holds the words comes before one that only mentions them.
Words are matched whole and by their stems (cache finds caching and caches).
Every word counts for what it's worth (a rare one for more, "the" for
nothing), so a question can be asked in a sentence; a document found with
only some of the words says so. For an exact string, ws rg.`,
		Args:              cobra.MinimumNArgs(1),
		PersistentPreRun:  func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkFormat(format); err != nil {
				return err
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			var repos []string
			for _, s := range spaces {
				repo, ok := ctx.WS().ResolveAlias(s)
				if !ok {
					return fmt.Errorf("%s isn't one of the workspace's repos", s)
				}
				if _, ok := ctx.WS().SpaceOf(repo); !ok {
					return fmt.Errorf("%s isn't a space of documents", repo)
				}
				repos = append(repos, repo)
			}
			db, _, err := ctx.Spaces(cmd.Context())
			if db == nil {
				return err
			}
			warnSpaces(err)
			hits, more, err := space.Search(cmd.Context(), db, space.Query{Words: strings.Join(args, " "), Repos: repos, Limit: limit})
			if err != nil {
				return err
			}
			out := docsOut(format)
			if len(hits) == 0 {
				fmt.Fprintln(out, "No documents say that.")
				return nil
			}
			for _, h := range hits {
				where := h.Repo + ":" + h.Path
				title := h.Title
				if h.Heading != "" {
					title += " › " + h.Heading
					where += "#" + h.Anchor
				}
				line := fmt.Sprintf("%s  %s", docsStyle(format, ui.Bold).Render(title), docsStyle(format, ui.Dim).Render(where))
				if !h.Every {
					line += docsStyle(format, ui.Dim).Render("  (some of the words)")
				}
				fmt.Fprintln(out, line)
				open, close := "**", "**"
				if format != "llm" {
					open, close = "\x1b[1m", "\x1b[22m"
				}
				if snip := space.Text(h.Snippet, open, close); snip != "" {
					fmt.Fprintf(out, "  %s\n", snip)
				}
			}
			if more > 0 {
				fmt.Fprintln(out, docsStyle(format, ui.Dim).Render(fmt.Sprintf("%d more (-n %d shows them)", more, limit+more)))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "", "text (the default) or llm (plain)")
	cmd.Flags().StringSliceVar(&spaces, "space", nil, "only this space, by its repo's name (repeatable)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "documents at most")
	return cmd
}

func checkFormat(format string) error {
	if format != "" && format != "llm" && format != "text" {
		return fmt.Errorf("unknown format %q (want text or llm)", format)
	}
	return nil
}

// warnSpaces says that a space couldn't be read, and goes on with what was.
func warnSpaces(err error) {
	if err != nil {
		fmt.Fprintf(ui.Stderr, "  %s reading the spaces: %v\n", ui.Orange.Render("⚠"), err)
	}
}

func docsOut(format string) io.Writer {
	if format == "llm" {
		return os.Stdout
	}
	return ui.Stdout
}

// docsStyle is s, or nothing for an agent's plain text.
func docsStyle(format string, s interface{ Render(...string) string }) interface{ Render(...string) string } {
	if format == "llm" {
		return plainStyle{}
	}
	return s
}

type plainStyle struct{}

func (plainStyle) Render(s ...string) string { return strings.Join(s, " ") }

// spaceName is how a space is named: its repo's label, and its folder.
func spaceName(ws *workspace.Workspace, s workspace.Space, format string) string {
	name := s.Repo
	if d := ws.DisplayNames[s.Repo]; d != "" && d != s.Repo {
		name = d + " (" + s.Repo + ")"
	}
	if s.Folder != "" {
		name += " " + s.Folder
	}
	return docsStyle(format, ui.Bold).Render(name)
}
