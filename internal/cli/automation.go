package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws automation: your automations (internal/automation).
func newAutomationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "automation",
		Short: "Your automations: rules in Starlark, run on what ws does and hears of",
		Long: `An automation is what ws does when something happens: a rule, written as
a small Starlark file, run on the workspace's events (ws events).

  # Notify me if Brendan pushes a new drawing.
  when = event("thing.added", kind = "sketch", by = "brendan")
  may = ["notify"]

  def run(e):
      notify("%s drew %s" % (e.by.short, e.title), open = e.thing)

when is which events it's for: event(kind, field = value...), as a Wait's
until reads them (pr.merged pr="api#212", thing.changed status="Done"), or
a list of them. may is what it may do: notify (a notification in your
feed, from Automations) and converse (start a conversation, with what to
look at). run is called with each event it's for, as e (e.kind, e.by.short,
e.title, e.pr, e.thing, e.changes...); without one, the event's notified as
it is. It can read with show(ref), events(kind, since = "7d", ...) and
refs_in(text), and log(text) for its run's record.

A run can wait: wait(event("run.ended", run = "deploy"), timeout = "2h")
parks it till an event it's for is kept (it's then that event) or its time's
up (None), and it's run again from the top, each step it took before given
what it came to. And when can pace it:

  event("thing.changed", kind = "sketch", settle = "10m", per = "thing")
      run once a sketch's changes have been quiet ten minutes, e.batch
      all of them
  event("pr.checks_failed", at_most = "1/h", per = "pr")
      at most once an hour each pull request; what's past it is held

Or it runs by the clock: when = every("day 03:00") (or "1h", "weekday
09:00", "mon,thu 17:30"), on its own schedule.due; a time missed while the
daemon was away is run once when it's back.

It runs in the daemon, from the next event after it's added: once an
event, each step that reaches outside it (notify, converse) done once
even if the run's run again. You're told once when one can't run.

  ws automation add drawing.star     add it (by its file's name, or --name), on
  ws automation check drawing.star   say what's wrong with it, adding nothing
  ws automation kinds                how one's written: every kind of event, and the builtins
  ws automation list                 each, what it's for and may do, and how its last run went
  ws automation runs [name]          its runs, the latest first
  ws automation run <name>           run one that's on a schedule now, as if its time had come
  ws automation off <name>           stop it (on starts it again, from the next event)
  ws automation rm <name>            take it away`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return listAutomations(false) },
	}
	cmd.AddCommand(newAutomationAddCmd(), newAutomationCheckCmd(), newAutomationKindsCmd(), newAutomationListCmd(), newAutomationRunsCmd(),
		newAutomationSetCmd("on"), newAutomationSetCmd("off"), newAutomationRmCmd(), newAutomationRunNowCmd())
	return cmd
}

// automationFile is a script and the name it's to have: --name, or its
// file's.
func automationFile(path, name string) (string, []byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return name, src, nil
}

func newAutomationAddCmd() *cobra.Command {
	var name string
	var asProposal bool
	cmd := &cobra.Command{
		Use:   "add <file.star | ->",
		Short: "Add an automation, on, run from the next event (one by its name already is replaced); --propose, in a conversation",
		Long: `Add an automation from its file (- for stdin, with --name), on: it's run
in the daemon from the next event. One by its name already is replaced.

In a conversation, its agent proposes one instead (--propose first): ws
checks it and dry-runs it over the last 30 days of events, and the user
reads what it would have done on its card before they turn it on. With
--propose and no file, it prints how one's written (ws automation kinds).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			w, err := loadOps()
			if err != nil {
				return err
			}
			if asProposal && len(args) == 0 {
				fmt.Fprint(ui.Stdout, automation.Guide())
				return nil
			}
			if len(args) == 0 {
				return errors.New("say its file (or - for stdin, with --name)")
			}
			if !asProposal {
				if err := refuseInConversation("an automation"); err != nil {
					return err
				}
			}
			var src []byte
			if args[0] == "-" {
				if name == "" {
					return errors.New("give it a name (--name), as it's read from stdin")
				}
				if src, err = io.ReadAll(cmd.InOrStdin()); err != nil {
					return err
				}
			} else if name, src, err = automationFile(args[0], name); err != nil {
				return err
			}
			if asProposal {
				return proposeAutomation(w, name, src)
			}
			a, err := (&automation.Engine{Root: w.Root()}).Add(name, src)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			fmt.Fprintf(ui.Stdout, "Added %s, on: when %s, it %s.\n", a.Name, a.WhenSaid(), maySaid(a))
			fmt.Fprintln(ui.Stdout, ui.Dim.Render("  It runs in the daemon, from the next event."))
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "what it's called (default: its file's name)")
	cmd.Flags().BoolVar(&asProposal, "propose", false, "propose it, for the user to turn on (a conversation's agent)")
	revisesFlag(cmd)
	return cmd
}

// dryRunBack is how far back a proposed automation's dry run looks.
const dryRunBack = 30 * 24 * time.Hour

// proposeAutomation checks an automation, dry-runs it, and proposes it in
// the conversation the agent's answering, saying what the dry run found.
func proposeAutomation(w *ops.Workspace, name string, src []byte) error {
	a, err := automation.Check(name, src)
	if err != nil {
		return fmt.Errorf("not proposed: %w", err)
	}
	hooks := w.AutomationHooks()
	hooks.Show = func(ref string) (map[string]any, error) {
		shown, err := shownFor(w, protocol.ShowParams{Ref: ref})
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(shown)
		var out map[string]any
		return out, json.Unmarshal(b, &out)
	}
	dr, err := (&automation.Engine{Root: w.Root(), Hooks: hooks}).DryRun(a, dryRunBack)
	if err != nil {
		return fmt.Errorf("not proposed: its dry run: %w", err)
	}
	_, had := os.Stat(automation.Path(w.Root(), name))
	var outline *automation.Outline
	if o, err := automation.OutlineOf(a); err == nil {
		outline = &o
	}
	mine, _ := automation.List(w.Root())
	states, _ := automation.States(w.Root())
	var on []*automation.Automation
	for _, b := range mine {
		if states[b.Name].On {
			on = append(on, b)
		}
	}
	setsOff, setOffBy := automation.Linked(a, on)
	p := conversation.Proposal{Kind: conversation.ProposeAutomation, Title: name, Text: string(src), Note: a.About,
		Automation: &automation.Proposed{Name: name, About: a.About, When: a.WhenSaid(), May: append([]string{}, a.May...), DryRun: dr, Replaces: had == nil, Outline: outline,
			SetsOff: setsOff, SetOffBy: setOffBy}}
	if err := propose(w, p); err != nil {
		return err
	}
	said := fmt.Sprintf("over the last 30 days (%d events), it would have run %d %s", dr.Events, dr.Ran, plural(dr.Ran, "time"))
	if dr.Held > 0 {
		said += fmt.Sprintf(", %d held back", dr.Held)
	}
	if dr.Failed > 0 {
		said += fmt.Sprintf(", %d failing", dr.Failed)
	}
	if dr.Settling > 0 {
		said += fmt.Sprintf(", %d settling still", dr.Settling)
	}
	fmt.Fprintf(ui.Stderr, "  %s %s: the user reads it on the card\n", ui.Dim.Render("·"), said)
	if len(setsOff) > 0 {
		fmt.Fprintf(ui.Stderr, "  %s the conversations it starts would set off %s (a chain stops %d deep, and at a loop)\n", ui.Orange.Render("⚠"), strings.Join(setsOff, ", "), automation.MaxChain)
	}
	if len(setOffBy) > 0 {
		fmt.Fprintf(ui.Stderr, "  %s it'd be set off by the conversations %s starts\n", ui.Orange.Render("⚠"), strings.Join(setOffBy, ", "))
	}
	for _, l := range dr.Lines {
		if l.Error != "" {
			fmt.Fprintf(ui.Stderr, "  %s %s failed: %s\n", ui.Orange.Render("⚠"), l.Event, firstLineOf(l.Error))
		}
	}
	return nil
}

func newAutomationKindsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kinds",
		Short: "Print how an automation's written: every kind of event, what each says, and the builtins",
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, args []string) { fmt.Fprint(ui.Stdout, automation.Guide()) },
	}
}

func newAutomationCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <file.star>",
		Short: "Say what's wrong with an automation, adding nothing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			name, src, err := automationFile(args[0], "")
			if err != nil {
				return err
			}
			a, err := automation.Check(name, src)
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stdout, "%s is good: when %s, it %s.\n", a.Name, a.WhenSaid(), maySaid(a))
			return nil
		},
	}
}

func newAutomationListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Your automations: each, what it's for, what it may do, and how its last run went",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return listAutomations(format == "json")
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "", "text (the default) or json")
	return cmd
}

// automationListed is an automation as ws automation list says it.
type automationListed struct {
	Name    string          `json:"name"`
	About   string          `json:"about,omitempty"`
	When    string          `json:"when,omitempty"`
	May     []string        `json:"may"`
	On      bool            `json:"on"`
	Problem string          `json:"problem,omitempty"`
	Failed  string          `json:"failed,omitempty"`
	Last    *automation.Run `json:"last,omitempty"`
}

func listAutomations(asJSON bool) error {
	w, err := loadOps()
	if err != nil {
		return err
	}
	list, bad := automation.List(w.Root())
	states, err := automation.States(w.Root())
	if err != nil {
		return err
	}
	var out []automationListed
	for _, a := range list {
		st := states[a.Name]
		l := automationListed{Name: a.Name, About: a.About, When: a.WhenSaid(), May: append([]string{}, a.May...), On: st.On, Failed: st.Failed}
		if runs, _ := automation.Runs(w.Root(), a.Name, 1); len(runs) > 0 {
			l.Last = &runs[0]
		}
		out = append(out, l)
	}
	for name, err := range bad {
		out = append(out, automationListed{Name: name, May: []string{}, On: states[name].On, Problem: err.Error()})
	}
	if asJSON {
		for _, l := range out {
			line, _ := json.Marshal(l)
			fmt.Fprintln(ui.Stdout, string(line))
		}
		return nil
	}
	if len(out) == 0 {
		fmt.Fprintln(ui.Stderr, ui.Dim.Render("  none yet: ws automation add <file.star> (ws automation --help says how one's written)"))
		return nil
	}
	for _, l := range out {
		state := "on"
		if !l.On {
			state = "off"
		}
		fmt.Fprintf(ui.Stdout, "%s  %s\n", l.Name, ui.Dim.Render(state))
		if l.About != "" {
			fmt.Fprintf(ui.Stdout, "  %s\n", l.About)
		}
		switch {
		case l.Problem != "":
			fmt.Fprintf(ui.Stdout, "  can't be read: %s\n", firstLineOf(l.Problem))
			continue
		case l.Failed != "":
			fmt.Fprintf(ui.Stdout, "  couldn't run: %s\n", firstLineOf(l.Failed))
		}
		may := "notify the event"
		if len(l.May) > 0 {
			may = strings.Join(l.May, ", ")
		}
		fmt.Fprintf(ui.Stdout, "  %s\n", ui.Dim.Render("when "+l.When+" · may "+may))
		if l.Last != nil {
			fmt.Fprintf(ui.Stdout, "  %s\n", ui.Dim.Render(fmt.Sprintf("last ran on e%d, %s: %s", l.Last.Event, l.Last.At.Local().Format("Jan 02 15:04"), l.Last.State)))
		}
	}
	return nil
}

func newAutomationRunsCmd() *cobra.Command {
	var limit int
	var format string
	cmd := &cobra.Command{
		Use:   "runs [name]",
		Short: "An automation's runs (every one's, with no name), the latest first",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			w, err := loadOps()
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			runs, err := automation.Runs(w.Root(), name, limit)
			if err != nil {
				return err
			}
			for _, r := range runs {
				if format == "json" {
					line, _ := json.Marshal(r)
					fmt.Fprintln(ui.Stdout, string(line))
					continue
				}
				line := fmt.Sprintf("%s %-20s e%-5d %s", ui.Dim.Render(r.At.Local().Format("Jan 02 15:04")), r.Name, r.Event, r.State)
				if r.Key != "" {
					line += ui.Dim.Render(" (" + r.Key + ")")
				}
				if r.Error != "" {
					line += ": " + firstLineOf(r.Error)
				}
				fmt.Fprintln(ui.Stdout, line)
				for _, l := range strings.Split(strings.TrimSpace(r.Log), "\n") {
					if l != "" {
						fmt.Fprintln(ui.Stdout, ui.Dim.Render("    "+l))
					}
				}
			}
			if len(runs) == 0 && format != "json" {
				fmt.Fprintln(ui.Stderr, ui.Dim.Render("  none yet"))
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "how many of the latest")
	cmd.Flags().StringVarP(&format, "format", "f", "", "text (the default) or json (one run per line)")
	return cmd
}

func newAutomationSetCmd(to string) *cobra.Command {
	short := "Stop an automation: it isn't run until it's on again"
	if to == "on" {
		short = "Start an automation that's off again, from the next event"
	}
	return &cobra.Command{
		Use:   to + " <name>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			w, err := loadOps()
			if err != nil {
				return err
			}
			if err := automation.SetOn(w.Root(), args[0], to == "on"); errors.Is(err, automation.ErrNotFound) {
				return fmt.Errorf("there's no automation %s (ws automation list)", args[0])
			} else if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stdout, "%s is %s.\n", args[0], to)
			return nil
		},
	}
}

func newAutomationRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Take an automation away: its file, and what's kept of its runs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			w, err := loadOps()
			if err != nil {
				return err
			}
			if err := (&automation.Engine{Root: w.Root()}).Remove(args[0]); errors.Is(err, automation.ErrNotFound) {
				return fmt.Errorf("there's no automation %s (ws automation list)", args[0])
			} else if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stdout, "Took %s away.\n", args[0])
			return nil
		},
	}
}

// maySaid is what an automation may do, in a sentence.
func maySaid(a *automation.Automation) string {
	if len(a.May) == 0 {
		return "notifies the event"
	}
	return "may " + strings.Join(a.May, " and ")
}

func firstLineOf(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

func newAutomationRunNowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <name>",
		Short: "Run an automation that's on a schedule now, as if its time had come",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			w, err := loadOps()
			if err != nil {
				return err
			}
			ev, err := w.RunAutomationNow(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "%s %s: its time, now (%s); the daemon runs it\n", ui.Green.Render("✓"), args[0], ev.Ref())
			return nil
		},
	}
}
