package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/team"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The Team page's side of the CLI: the service the web UI serves it from,
// and ws team, which opens the page or prints a period.

// teamService is the Team page's service for the workspace: each visible
// repo's default branch read from its clone, and GitHub asked through
// ctx's client.
func teamService(ctx *ops.Workspace) *team.Service {
	root := ctx.WS().Root
	window, _ := team.ParseWindow(ctx.Config().Team.Window)
	// Where there's a file answering for GitHub, there's no GitHub to link to.
	var web func(repo string) string
	if ctx.Config().GitHub.Fixture == "" {
		web = func(repo string) string {
			if !ctx.WS().OnGitHub(repo) {
				return ""
			}
			return ctx.WS().RemoteOf(repo).URL()
		}
	}
	// An agent writes gists unless [team] narrative says not to, or no
	// harness here can.
	var writer func(team.Ask) (string, error)
	noWriter := ""
	if !ctx.Config().TeamNarrative() {
		noWriter = "[team] narrative is off"
	} else if _, _, err := ops.Roles(ctx.Config()).ForTeam(); err != nil {
		noWriter = err.Error()
	} else {
		writer = teamWriter(root)
	}
	var summariser func(team.SummaryAsk) (string, error)
	if writer != nil {
		summariser = teamSummariser(root)
	}
	return &team.Service{
		Root: root, GitHub: ctx.GitHub(), Window: window, Web: web, Writer: writer, NoWriter: noWriter, Summariser: summariser,
		Repos: func() []team.Repo {
			c, err := LoadOps(root)
			if err != nil {
				return nil
			}
			var out []team.Repo
			for _, name := range c.WS().ListedRepoNames() {
				if info, err := os.Stat(c.WS().BareDir(name)); err == nil && info.IsDir() {
					out = append(out, team.Repo{Name: name, Bare: c.WS().BareDir(name), Branch: c.WS().DefaultBranch, GitHub: c.WS().RemoteOf(name)})
				}
			}
			return out
		},
		// Your capsules, compared with the default branch they left.
		Capsules: func(repo string) []team.Capsule {
			c, err := LoadOps(root)
			if err != nil {
				return nil
			}
			branch := c.WS().DefaultBranch
			if branch == "" {
				branch = "main"
			}
			names, _ := workspace.ListWorktrees(c.WS().RepoDir(repo))
			out := make([]team.Capsule, 0, len(names))
			for _, n := range names {
				out = append(out, team.Capsule{Repo: repo, Name: n, Dir: filepath.Join(c.WS().RepoDir(repo), n), Base: "origin/" + branch})
			}
			return out
		},
		Watch: func() []team.Watch {
			c, err := LoadOps(root)
			if err != nil {
				return nil
			}
			var out []team.Watch
			for name, w := range c.Config().Team.Watch {
				out = append(out, team.Watch{Name: name, Title: w.Title, Paths: w.Paths})
			}
			return out
		},
		People: func() map[string]team.PersonConfig {
			c, err := LoadOps(root)
			if err != nil {
				return nil
			}
			out := map[string]team.PersonConfig{}
			for key, p := range c.Config().Team.People {
				out[key] = team.PersonConfig{Name: p.Name, Logins: p.Logins, Emails: p.Emails}
			}
			return out
		},
	}
}

func newTeamCmd() *cobra.Command {
	var printOnly, read bool
	var since, format string
	cmd := &cobra.Command{
		Use:   "team",
		Short: "What the people working in the workspace's repos have been doing",
		Long: `Open the Team page in the web UI: what's landed on each repo's default
branch and what's happened to pull requests, by day and by person.

With --since, print a period here instead: today, yesterday, a weekday,
week (since Monday), a number of days or weeks (7d, 2w) or a date
(2026-11-03). It looks at git and GitHub first, if that hasn't been done
in the last couple of minutes.

What each change does, and what's worth knowing of it, is printed where
an agent has read it. The page has what's recent read while it's open;
--read has everything in the period read before printing, which is an
agent run for each change that hasn't been.

What it's read from is kept in the workspace's records (.ws/ws.db), on
your machine: as far back as [team] window, or further for the rest of
the day when --since asks for more.`,
		Args:   cobra.NoArgs,
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if since == "" && format == "" && !read {
				url, err := webURL(ctx.WS().Root, "/team")
				if err != nil {
					return err
				}
				if printOnly {
					fmt.Println(url)
					return nil
				}
				if err := openBrowser(url); err != nil {
					return fmt.Errorf("opening your browser: %w (ws team --print prints the link)", err)
				}
				return nil
			}
			if format != "" && format != "text" && format != "json" && format != "llm" {
				return fmt.Errorf("-f %s: say text, json or llm", format)
			}
			if _, err := team.ParseWindow(ctx.Config().Team.Window); err != nil {
				return err
			}
			svc := teamService(ctx)
			var from time.Time
			if since != "" {
				if from, err = team.Since(since, time.Now()); err != nil {
					return err
				}
			}
			snap, err := svc.Look(false, from)
			if snap == nil {
				return err
			}
			if err != nil {
				fmt.Fprintf(ui.Stderr, "  %s %v: showing what was last known\n", ui.Orange.Render("!"), err)
			}
			if read {
				if err := readChanges(svc, snap, from); err != nil {
					return err
				}
			}
			tl := svc.Timeline(snap, from)
			if format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(tl)
			}
			if format == "llm" {
				printTimelineLLM(os.Stdout, tl)
				return nil
			}
			printTimeline(ui.Stdout, ctx, tl)
			for _, p := range tl.Problems {
				fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Orange.Render("!"), p)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the page's link instead of opening it")
	cmd.Flags().StringVar(&since, "since", "", "print the period from then on: today, yesterday, monday, week, 7d, 2w, 2026-11-03")
	cmd.Flags().StringVarP(&format, "format", "f", "", "print as text (the default with --since), json, or llm (for an agent: each change once, plainly)")
	cmd.AddCommand(newTeamGistCmd(), newTeamAccountCmd())
	cmd.Flags().BoolVar(&read, "read", false, "have an agent read the period's changes first, where it hasn't: what each does, and what's worth knowing of it")
	return cmd
}

// newTeamGistCmd is ws team gist: one strand's change read, and what an
// agent says of it printed, without keeping it. It's for seeing how the
// prompt does on a real change.
func newTeamGistCmd() *cobra.Command {
	var prompt bool
	cmd := &cobra.Command{
		Use:    "gist <strand>",
		Short:  "Have one change read and print what's said of it, without keeping it",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			svc := teamService(ctx)
			snap, err := svc.Look(false, time.Time{})
			if snap == nil {
				return err
			}
			ask, g, err := svc.Try(snap, args[0], prompt)
			if prompt && err == nil {
				fmt.Println(team.GistPrompt(ask, ctx.WS().DisplayNameFor))
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Printf("gist: %s\n", g.Gist)
			for _, k := range g.Know {
				file := k.File
				if k.In != "" {
					file = k.In + ":" + file
				}
				fmt.Printf("know: %s | %s | %s\n", k.Kind, file, k.Text)
			}
			if len(g.Know) == 0 {
				fmt.Println("know: none")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&prompt, "prompt", false, "print what the agent would be asked, and don't ask it")
	return cmd
}

// newTeamAccountCmd is ws team account: a period's account written and
// printed, without keeping it. It's for seeing how the prompt does on
// real changes.
func newTeamAccountCmd() *cobra.Command {
	var prompt bool
	var since string
	cmd := &cobra.Command{
		Use:    "account",
		Short:  "Have a period's account written and print it, without keeping it",
		Hidden: true,
		Args:   cobra.NoArgs,
		PreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			svc := teamService(ctx)
			var from time.Time
			if since != "" {
				if from, err = team.Since(since, time.Now()); err != nil {
					return err
				}
			}
			snap, err := svc.Look(false, from)
			if snap == nil {
				return err
			}
			ask, topics, err := svc.TryAccount(snap, from, prompt)
			if err != nil {
				return err
			}
			if prompt {
				fmt.Println(team.SummaryPrompt(ask, ctx.WS().DisplayNameFor))
				return nil
			}
			name := map[int]string{}
			strand := map[string]int{}
			for _, c := range ask.Changes {
				name[c.Number], strand[c.Strand] = c.Name, c.Number
			}
			fmt.Fprintf(ui.Stderr, "  %s from %d changes\n", ui.Dim.Render("·"), len(ask.Changes))
			for _, t := range topics {
				fmt.Printf("%s\n  %s\n", ui.Bold.Render(t.Name), t.Text)
				for _, s := range t.Strands {
					fmt.Printf("    %s\n", ui.Dim.Render(name[strand[s]]))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "where the period starts, as ws team --since takes it")
	cmd.Flags().BoolVar(&prompt, "prompt", false, "print what the agent would be asked, and don't ask it")
	return cmd
}

// readChanges has the gists of a period's changes written, where they
// haven't been, and waits for them, saying how it's going.
func readChanges(svc *team.Service, snap *team.Snapshot, from time.Time) error {
	if svc.Writer == nil {
		return fmt.Errorf("nothing reads changes here: %s", firstNonEmpty(svc.NoWriter, "no agent is set up to"))
	}
	if from.IsZero() {
		from = snap.Since
	}
	done := 0
	// One asking starts so many: ask until there's nothing left to.
	for {
		n := svc.Narrate(snap, nil, from)
		if n == 0 {
			break
		}
		fmt.Fprintf(ui.Stderr, "  %s reading %d %s…\n", ui.Dim.Render("·"), n, pluralize(n, "change", "changes"))
		for left := n; left > 0; {
			time.Sleep(500 * time.Millisecond)
			if now := svc.Writing(); now != left {
				left = now
				fmt.Fprintf(ui.Stderr, "  %s %d of %d read\n", ui.Dim.Render("·"), done+n-left, done+n)
			}
		}
		done += n
	}
	return nil
}

// printTimelineLLM writes a timeline for an agent to read: what it is,
// then each change once, at the day it last had something happen, with
// where it is, where it stands, who's on it, what it does and what's
// worth knowing of it.
func printTimelineLLM(w io.Writer, tl *team.Timeline) {
	names := map[string]string{}
	for _, p := range tl.People {
		names[p.Key] = p.Name
	}
	states := map[string]string{team.InProgress: "draft", team.InReview: "in review", team.Done: "landed", team.Shut: "closed"}
	fmt.Fprintf(w, "What's happened in this workspace's repos since %s (as of %s): pull requests, and commits pushed straight to default branches. Each change is listed once, under the day something last happened to it: its name, [where], where it stands, who's on it (whose it is first); then what it does, if an agent has read it; then anything worth knowing of it (!), with the file it rests on.\n", tl.Since.Format("2006-01-02"), tl.At.Format("2006-01-02 15:04"))
	seen := map[string]bool{}
	for _, d := range tl.Days {
		first := true
		for _, it := range d.Items {
			if it.Bot || seen[it.Strand] {
				continue
			}
			seen[it.Strand] = true
			if first {
				fmt.Fprintf(w, "\n%s\n", d.Date)
				first = false
			}
			where := it.Repo
			switch {
			case len(it.PRs) > 1:
				var prs []string
				for _, pr := range it.PRs {
					prs = append(prs, fmt.Sprintf("%s#%d", pr.Repo, pr.Number))
				}
				where = strings.Join(prs, ", ")
			case it.PR != 0:
				where = fmt.Sprintf("%s#%d", it.Repo, it.PR)
			}
			var people []string
			for _, key := range it.People {
				people = append(people, names[key])
			}
			fmt.Fprintf(w, "- %s [%s] %s; %s\n", it.Name, where, states[it.State], strings.Join(people, ", "))
			if it.Gist != "" {
				fmt.Fprintf(w, "  %s\n", it.Gist)
			}
			for _, k := range it.Know {
				file := k.File
				if k.In != "" {
					file = k.In + ":" + file
				}
				fmt.Fprintf(w, "  ! %s (%s): %s\n", k.Kind, file, k.Text)
			}
			for _, t := range it.Touches {
				fmt.Fprintf(w, "  touches %s: %s\n", strings.ToLower(t.Title), strings.Join(t.Files, ", "))
			}
		}
	}
	if len(seen) == 0 {
		fmt.Fprintln(w, "\nNothing.")
	}
}

// printTimeline writes a timeline as text: each day, then each thing
// that happened in it, with what's said of it under.
func printTimeline(w io.Writer, ctx *ops.Workspace, tl *team.Timeline) {
	names := map[string]string{}
	for _, p := range tl.People {
		names[p.Key] = p.Short
	}
	states := map[string]string{team.InProgress: "in progress", team.InReview: "in review", team.Done: "landed", team.Shut: "closed"}
	if len(tl.Days) == 0 {
		fmt.Fprintf(w, "  %s nothing since %s\n", ui.Dim.Render("·"), tl.Since.Format("Mon 2 Jan"))
		return
	}
	for i, d := range tl.Days {
		if i > 0 {
			fmt.Fprintln(w)
		}
		day, _ := time.ParseInLocation("2006-01-02", d.Date, time.Local)
		fmt.Fprintln(w, ui.Bold.Render(day.Format("Mon 2 Jan")))
		bots := 0
		for _, it := range d.Items {
			if it.Bot {
				bots++
				continue
			}
			where := ctx.WS().DisplayNameFor(it.Repo)
			if it.PR != 0 {
				where = fmt.Sprintf("%s#%d", it.Repo, it.PR)
			} else if len(it.Repos) > 1 {
				where = fmt.Sprintf("%d repos", len(it.Repos))
			}
			var people []string
			for _, key := range it.People {
				people = append(people, names[key])
			}
			fmt.Fprintf(w, "  %s  %s\n", it.Name, ui.Dim.Render(strings.Join([]string{where, states[it.State], strings.Join(people, ", ")}, " · ")))
			if it.Gist != "" {
				fmt.Fprintf(w, "    %s\n", it.Gist)
			}
			for _, k := range it.Know {
				file := k.File
				if k.In != "" {
					file = k.In + ":" + file
				}
				fmt.Fprintf(w, "    %s %s %s\n", ui.Orange.Render("◆"), k.Text, ui.Dim.Render(file))
			}
			fmt.Fprintf(w, "    %s\n", ui.Dim.Render(it.Words))
		}
		if bots > 0 {
			fmt.Fprintf(w, "  %s\n", ui.Dim.Render(fmt.Sprintf("%d automated", bots)))
		}
	}
}
