package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/nav"
)

// ws nav: code navigation from the command line, as a review asks it.
// Hidden: it's for checking what the web UI will be told, and later for
// agents.

func newNavCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "nav",
		Short:  "Code navigation: names, definitions and uses, at a commit",
		Hidden: true,
	}
	cmd.AddCommand(newNavNamesCmd(), newNavDefCmd(), newNavRefsCmd())
	return cmd
}

// navTarget is a repo's git directory and a commit in it, from a repo's
// name and anything git reads as a commit.
func navTarget(repo, rev string) (svc *nav.Service, dir, commit string, err error) {
	ctx, err := loadOps()
	if err != nil {
		return nil, "", "", err
	}
	dir = ctx.WS().BareDir(repo)
	if _, err := os.Stat(dir); err != nil {
		return nil, "", "", fmt.Errorf("no repo %s in this workspace", repo)
	}
	commit, err = git.Line(dir, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil || commit == "" {
		return nil, "", "", fmt.Errorf("no commit %s in %s", rev, repo)
	}
	return nav.New(navCache(ctx.Root())), dir, commit, nil
}

func newNavNamesCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "names <repo> <rev> <path>",
		Short: "Every name in a file at a commit, by line, with what part each plays",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, dir, commit, err := navTarget(args[0], args[1])
			if err != nil {
				return err
			}
			n, err := svc.Names(dir, commit, args[2])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(out).Encode(n)
			}
			if n.Why != "" {
				fmt.Fprintln(out, n.Why)
				return nil
			}
			src, err := git.Output(dir, "cat-file", "blob", commit+":"+args[2])
			if err != nil {
				return err
			}
			text := strings.Split(src, "\n")
			var lines []int
			for l := range n.Lines {
				lines = append(lines, l)
			}
			sort.Ints(lines)
			for _, l := range lines {
				units := utf16.Encode([]rune(text[l-1]))
				var names []string
				for _, nm := range n.Lines[l] {
					names = append(names, string(nm.Part)+":"+string(utf16.Decode(units[nm.From:nm.To])))
				}
				fmt.Fprintf(out, "%4d  %s\n", l, strings.Join(names, " "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "as the web UI is told it")
	return cmd
}

func newNavDefCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "def <repo> <rev> <path>:<line>:<col>",
		Short: "Where the name at a place leads (line and column from 1, the column in characters)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, dir, commit, err := navTarget(args[0], args[1])
			if err != nil {
				return err
			}
			p, line, units, err := placeIn(dir, commit, args[2])
			if err != nil {
				return err
			}
			a, err := svc.Definition(dir, commit, p, line, units)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(out).Encode(a)
			}
			fmt.Fprintf(out, "%s: %s", a.Name, a.Answer)
			if a.How != "" {
				fmt.Fprintf(out, " (%s)", a.How)
			}
			if a.Partial {
				fmt.Fprint(out, ", partial")
			}
			fmt.Fprintln(out)
			for _, t := range a.Targets {
				fmt.Fprintf(out, "  %s:%d:%d  %s\n", t.Path, t.Line, t.Col+1, t.Signature)
			}
			if a.External != nil {
				fmt.Fprintf(out, "  %s %s  %s\n", a.External.Module, a.External.Name, a.External.URL)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "as the web UI is told it")
	return cmd
}

// parsePlace reads path:line:col.
func parsePlace(s string) (p string, line, col int, err error) {
	i := strings.LastIndexByte(s, ':')
	j := strings.LastIndexByte(s[:max(i, 0)], ':')
	if i < 0 || j < 0 {
		return "", 0, 0, fmt.Errorf("want path:line:col, not %q", s)
	}
	line, lerr := strconv.Atoi(s[j+1 : i])
	col, cerr := strconv.Atoi(s[i+1:])
	if lerr != nil || cerr != nil || line < 1 || col < 1 {
		return "", 0, 0, fmt.Errorf("want path:line:col (from 1), not %q", s)
	}
	return s[:j], line, col, nil
}

// placeIn is path:line:col (from 1, the column in characters, as an
// editor counts) as a nav question has it: the column in UTF-16 units
// from 0.
func placeIn(dir, commit, s string) (p string, line, units int, err error) {
	p, line, col, err := parsePlace(s)
	if err != nil {
		return "", 0, 0, err
	}
	src, err := git.Output(dir, "cat-file", "blob", commit+":"+p)
	if err != nil {
		return "", 0, 0, err
	}
	lines := strings.Split(src, "\n")
	if line > len(lines) {
		return "", 0, 0, fmt.Errorf("%s has %d lines", p, len(lines))
	}
	runes := []rune(lines[line-1])
	if col-1 > len(runes) {
		return "", 0, 0, fmt.Errorf("line %d has %d characters", line, len(runes))
	}
	return p, line, len(utf16.Encode(runes[:col-1])), nil
}

func newNavRefsCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "refs <repo> <rev> <path>:<line>:<col>",
		Short: "Where the definition at a place (or the one the name there leads to) is used",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, dir, commit, err := navTarget(args[0], args[1])
			if err != nil {
				return err
			}
			p, line, units, err := placeIn(dir, commit, args[2])
			if err != nil {
				return err
			}
			r, err := svc.References(dir, commit, p, line, units)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(out).Encode(r)
			}
			if r.Def == nil {
				fmt.Fprintf(out, "%s isn't defined in the repo\n", r.Name)
				return nil
			}
			fmt.Fprintf(out, "%s, defined at %s:%d: %d %s", r.Name, r.Def.Path, r.Def.Line, len(r.Uses), map[bool]string{true: "use", false: "uses"}[len(r.Uses) == 1])
			if r.Partial {
				fmt.Fprint(out, " (partial)")
			}
			fmt.Fprintln(out)
			for _, u := range r.Uses {
				m := " "
				if u.Maybe {
					m = "?"
				}
				fmt.Fprintf(out, "%s %s:%d:%d  %s\n", m, u.Path, u.Line, u.Col+1, u.Text)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "as the web UI is told it")
	return cmd
}
