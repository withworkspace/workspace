package cli

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/ui"
	"golang.org/x/term"
)

// ws plugin, and the commands plugins add: a plugin is a program that adds
// a view to the web UI, commands to ws and a guide for agents, held as one
// process by the workspace's daemon.

// pluginsGroup is the heading plugins' commands are listed under in ws
// --help.
const pluginsGroup = "plugins"

// pluginHost is the workspace's plugins in this process, when it's the
// daemon: nil in a command, which reaches them through the daemon.
var pluginHost *plugin.Host

// newPluginHost is the daemon's host for the workspace's plugins.
func newPluginHost(s *daemon.Server, version string) *plugin.Host {
	return &plugin.Host{Root: s.Root, Version: version, Log: s.Log, Hold: s.Hold, Reserved: reservedWord,
		// How a plugin's things are named is kept in the workspace's
		// records, by the plugin: names are found from there.
		Keep: func(name string, refs []protocol.Ref) ([]string, error) { return plugin.KeepRefs(s.Root, name, refs) },
	}
}

// reservedWord says whether a word after ws is one of ws's own commands,
// which a plugin can't take.
func reservedWord(word string) bool {
	if word == "help" || word == "completion" {
		return true
	}
	for _, c := range newBuiltinRootCmd("").Commands() {
		if c.Name() == word || c.HasAlias(word) {
			return true
		}
	}
	return false
}

// hostRunner is the daemon's own plugins, asked directly.
type hostRunner struct{ h *plugin.Host }

func (r hostRunner) Statuses(context.Context) ([]plugin.Status, error) { return r.h.Statuses(), nil }
func (r hostRunner) Refresh(_ context.Context, name string) (plugin.Status, error) {
	return r.h.Refresh(name)
}
func (r hostRunner) Stop(_ context.Context, name string) error { r.h.Stop(name); return nil }
func (hostRunner) Close()                                      {}

func init() {
	// What adds, updates and removes a plugin (ops) starts and stops it
	// where it runs: in this process, when it's the daemon; otherwise in
	// the workspace's daemon, which is started, for a plugin that's to
	// run, if it isn't running.
	ops.PluginRunners = func(root string, start bool) (ops.PluginRunner, error) {
		switch {
		case pluginHost != nil && pluginHost.Root == root:
			return hostRunner{pluginHost}, nil
		case start:
			return connectPlugins(root)
		}
		return plugin.Dial(root)
	}
}

// connectPlugins is a connection to the workspace's plugins in its daemon,
// which is started if it isn't running.
func connectPlugins(root string) (*plugin.Client, error) {
	c, err := daemon.Connect(root)
	if err != nil {
		return nil, err
	}
	c.Close()
	pc, err := plugin.Dial(root)
	if err != nil {
		return nil, fmt.Errorf("%w (a daemon from before plugins? `ws daemon restart --now` starts it again on this ws)", err)
	}
	return pc, nil
}

// pluginStatuses are the workspace's plugins: as the daemon has them if
// it's running, else as they were left.
func pluginStatuses(root string) []plugin.Status {
	if c, err := plugin.Dial(root); err == nil {
		defer c.Close()
		if out, err := c.Statuses(context.Background()); err == nil {
			return out
		}
	}
	return (&plugin.Host{Root: root}).Statuses()
}

// addPluginCommands adds each installed plugin's commands to root, where
// it's run in a workspace: ws jira … is passed to the plugin as it's
// typed.
func addPluginCommands(root *cobra.Command) {
	wsRoot, err := config.Discover(userDirOr())
	if err != nil {
		return
	}
	added := false
	for _, name := range plugin.Installed(wsRoot) {
		reg, ok := plugin.ReadRegistration(wsRoot, name)
		word := reg.CommandName(name)
		if !ok || word == "" {
			continue
		}
		if c, _, err := root.Find([]string{word}); err == nil && c != root {
			continue // ws's own, or a plugin before it: said when it started
		}
		if !added {
			root.AddGroup(&cobra.Group{ID: pluginsGroup, Title: "Plugins' Commands:"})
			added = true
		}
		short := reg.Commands.Short
		if short == "" {
			short = "The " + name + " plugin's commands"
		}
		root.AddCommand(&cobra.Command{
			Use:                word,
			Short:              short,
			GroupID:            pluginsGroup,
			DisableFlagParsing: true, // every word is the plugin's, --help included
			SilenceUsage:       true,
			SilenceErrors:      true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runPluginCommand(wsRoot, name, args)
			},
		})
	}
}

// runPluginCommand runs a plugin's command in the daemon, with its input
// read from here as the plugin reads it, its output here as it comes, and
// its exit code this command's.
func runPluginCommand(root, name string, args []string) error {
	// Every word is the plugin's but ws's own --stdin-from, which ws's
	// guard puts last (harness.heredocAside).
	if n := len(args); n >= 2 && args[n-2] == harness.StdinFlag {
		stdinFrom, args = args[n-1], args[:n-2]
		useStdinFrom()
	}
	in := &plugin.Input{Kind: protocol.InputPiped, R: os.Stdin}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		in.Kind = protocol.InputTerminal
	}
	c, err := connectPlugins(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ws:", err)
		return &ExitCodeError{Code: 1}
	}
	defer c.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	t := plugin.Terminal{TTY: term.IsTerminal(int(os.Stdout.Fd()))}
	if t.TTY {
		t.Width, _, _ = term.GetSize(int(os.Stdout.Fd()))
		t.Color = os.Getenv("NO_COLOR") == ""
	}
	exit, err := c.Run(ctx, name, args, userDirOr(), t, in, func(stream, data string) {
		if stream == "stderr" {
			io.WriteString(os.Stderr, data) //nolint:errcheck // nowhere to say so
			return
		}
		io.WriteString(os.Stdout, data) //nolint:errcheck // nowhere to say so
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "ws:", err)
		return &ExitCodeError{Code: 1}
	}
	if exit != 0 {
		return &ExitCodeError{Code: exit}
	}
	return nil
}

func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Plugins: programs that add views, commands and a guide for agents to the workspace",
		Long: `A plugin is a program that adds to the workspace without being part of ws:
a view in the web UI, commands after ws, a guide for agents. The workspace's
daemon runs each as one process, started when it's first needed.

A plugin is a folder with a plugin.toml (its name, version, and the command
that starts it) and an executable. What it adds, it says itself when it
starts.`,
	}
	cmd.AddCommand(newPluginAddCmd(), newPluginListCmd(), newPluginRemoveCmd(), newPluginConfigCmd(), newPluginRestartCmd(), newPluginLogsCmd())
	cmd.AddCommand(newPluginCheckCmd(), newPluginUpdateCmd())
	return cmd
}

func newPluginAddCmd() *cobra.Command {
	var link, build bool
	var format string
	cmd := &cobra.Command{
		Use:   "add <owner/repo | folder>",
		Short: "Install a plugin, from where it's published or from a folder, and start it",
		Long: `Installs a plugin into this workspace (.ws/plugins/<name>), starts it to
hear what it adds, and asks for the config it requires.

owner/repo is a plugin published on GitHub: this machine's build of its
latest release is downloaded and checked against the release's checksums
(owner/repo@v1.2.0 for a release that isn't the latest). A repo with no
release, or none built for this machine, is installed from its source,
which is built here. ws plugin update fetches a newer release later.
With gh logged in, a private repo is read as you.

A folder is a plugin on this machine.

A plugin that hasn't been built is built first, as its plugin.toml says
(build). --build builds it whether it has been or not.

With --link the workspace runs the plugin from the folder itself, for one
you're writing. Adding a plugin that's already here replaces it, keeping
its config and data, so ws plugin add --link --build is "build this and
run what I built": what the plugin-dev plugin does on every save.

-f json prints how it went, for a program: whether it built, what the
build printed, and where the plugin stands.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if format != "" && format != "text" && format != "json" {
				return fmt.Errorf("unknown format %q (want text or json)", format)
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			root := ctx.WS().Root
			// What it's doing is said where what it prints isn't read as JSON.
			say := func(line string) { fmt.Fprintln(ui.Stdout, line) }
			if format == "json" {
				say = func(line string) { fmt.Fprintln(os.Stderr, line) }
			}
			// A folder is found from where the command's run; anything else
			// is where a plugin's published.
			from := args[0]
			if folder := from; !filepath.IsAbs(folder) {
				folder = filepath.Join(userDirOr(), folder)
				if _, err := os.Stat(folder); err == nil {
					from = folder
				}
			}
			res, st, err := ctx.AddPlugin(cmd.Context(), from, ops.PluginAdd{Link: link, Build: build, Say: say})
			if format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				enc.Encode(res) //nolint:errcheck // nowhere to say so
				if err != nil {
					return &ExitCodeError{Code: 1}
				}
				return nil
			}
			if err != nil {
				io.WriteString(ui.Stdout, res.BuildOutput) //nolint:errcheck // nowhere to say so
				return err
			}
			fmt.Fprintf(ui.Stdout, "Added %s %s\n", res.Name, res.Version)
			describePlugin(ui.Stdout, st)
			if len(st.Missing) > 0 {
				if !ui.IsInteractive() {
					fmt.Fprintf(ui.Stdout, "\nIt needs setting up: ws plugin config %s %s\n", res.Name, strings.Join(placeholders(st), " "))
					return nil
				}
				fmt.Fprintln(ui.Stdout)
				if err := askPluginConfig(root, st, false); err != nil {
					return err
				}
				if c, err := plugin.Dial(root); err == nil {
					defer c.Close()
					return c.ConfigChanged(context.Background(), res.Name)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&link, "link", false, "run the plugin from the folder itself, for one being written")
	cmd.Flags().BoolVar(&build, "build", false, "build it first, whether or not it's been built")
	cmd.Flags().StringVarP(&format, "format", "f", "", "output format: text or json")
	return cmd
}

// describePlugin says what a plugin adds.
func describePlugin(w io.Writer, st plugin.Status) {
	r := st.Registration
	for _, v := range r.Views {
		fmt.Fprintf(w, "  view      %s, in the web UI's sidebar\n", v.Title)
	}
	if word := r.CommandName(st.Name); word != "" {
		fmt.Fprintf(w, "  commands  ws %s\n", word)
	}
	if r.Agent != nil && r.Agent.Guide != "" {
		fmt.Fprintf(w, "  guide     for agents\n")
	}
}

// placeholders are a plugin's missing keys as ws plugin config takes
// them.
func placeholders(st plugin.Status) []string {
	out := make([]string, len(st.Missing))
	for i, k := range st.Missing {
		out[i] = k + "=…"
	}
	return out
}

func newPluginListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "The workspace's plugins, and where each stands",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			root := ctx.WS().Root
			statuses := pluginStatuses(root)
			if format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(statuses)
			}
			if format != "" && format != "text" {
				return fmt.Errorf("unknown format %q (want text or json)", format)
			}
			// The team's, not here: ws.toml's [plugins].
			var wanted []string
			for name := range ctx.Config().Plugins {
				if !containsStatus(statuses, name) {
					wanted = append(wanted, name)
				}
			}
			sort.Strings(wanted)
			if len(statuses) == 0 && len(wanted) == 0 {
				fmt.Fprintln(ui.Stdout, "No plugins. ws plugin add <folder> installs one.")
				return nil
			}
			tw := tabwriter.NewWriter(ui.Stdout, 0, 0, 2, ' ', 0)
			for _, st := range statuses {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", st.Name, st.Version, st.State, pluginNote(st))
			}
			for _, name := range wanted {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", name, ctx.Config().Plugins[name].Version, "missing", "in ws.toml's [plugins], not installed here: "+ctx.Config().Plugins[name].Source)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "", "output format: text or json")
	return cmd
}

func containsStatus(statuses []plugin.Status, name string) bool {
	for _, st := range statuses {
		if st.Name == name {
			return true
		}
	}
	return false
}

// pluginNote is what's worth saying of a plugin beside where it stands.
func pluginNote(st plugin.Status) string {
	var notes []string
	if len(st.Missing) > 0 {
		notes = append(notes, "needs "+strings.Join(st.Missing, ", "))
	}
	if st.Error != "" {
		notes = append(notes, st.Error)
	}
	for name, s := range st.Syncs {
		if s.Error != "" {
			notes = append(notes, "sync "+name+" failed: "+s.Error)
		}
	}
	// How its things are named, and what of that ws left out.
	var names []string
	for _, r := range st.Registration.Refs {
		for _, p := range r.Prefixes {
			names = append(names, p+"…")
		}
	}
	if len(names) > 0 {
		notes = append(notes, "names "+strings.Join(names, ", "))
	}
	notes = append(notes, st.Refused...)
	if st.Linked != "" {
		notes = append(notes, "linked to "+st.Linked)
	}
	if st.Source != nil {
		from := "from " + st.Source.Repo
		switch {
		case st.Source.File != "":
			from += " " + st.Source.Tag
		case st.Source.Tag != "":
			from += " " + st.Source.Tag + ", built here"
		default:
			from += ", built here"
		}
		notes = append(notes, from)
	}
	return strings.Join(notes, "; ")
}

func newPluginRemoveCmd() *cobra.Command {
	var data bool
	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a plugin from the workspace",
		Long: `Stops a plugin and removes it. What it kept for itself (its data folder)
and its secrets stay unless --data is given, so adding it again picks up
where it was. Its config in ws.toml and ws.local.toml is yours to remove.

What ws kept of it goes with it: its things' names (RAM-123) are words
again, wherever they're written.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if err := ctx.RemovePlugin(cmd.Context(), args[0], data); err != nil {
				return err
			}
			fmt.Fprintf(ui.Stdout, "Removed %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&data, "data", false, "remove what the plugin kept for itself, and its secrets, too")
	return cmd
}

func newPluginUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update [name ...]",
		Short: "Fetch the latest release of the plugins that were installed from a repo",
		Long: `For each plugin installed from where it's published (ws plugin add
owner/repo), or those named: fetches its latest release if it's newer than
what's here, and starts it afresh. Its config and data stay. One built
from its source is fetched and built again, as there's no saying whether
it's changed.

A plugin added from a folder isn't updated: it's the folder's.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			root := ctx.WS().Root
			statuses := pluginStatuses(root)
			for _, name := range args {
				st, err := pluginStatus(root, name)
				switch {
				case err != nil:
					return err
				case st.Source == nil:
					return fmt.Errorf("%s was added from a folder, not from a repo: there's nowhere to update it from", name)
				}
			}
			say := func(line string) { fmt.Fprintln(ui.Stdout, "  "+line) }
			updated, failed := 0, 0
			for _, st := range statuses {
				if st.Source == nil || (len(args) > 0 && !slices.Contains(args, st.Name)) {
					continue
				}
				fmt.Fprintf(ui.Stdout, "%s: looking at %s\n", st.Name, st.Source.Repo)
				got, err := ctx.UpdatePlugin(cmd.Context(), st.Name, say)
				switch {
				case err != nil:
					io.WriteString(ui.Stdout, got.Added.BuildOutput) //nolint:errcheck // nowhere to say so
					fmt.Fprintf(ui.Stdout, "  %v\n", err)
					failed++
				case got.Current:
					fmt.Fprintf(ui.Stdout, "  %s %s is the latest\n", st.Name, got.Now)
				default:
					fmt.Fprintf(ui.Stdout, "  %s: %s → %s\n", st.Name, got.Was, got.Now)
					updated++
				}
			}
			if updated == 0 && failed == 0 && len(args) == 0 {
				fmt.Fprintln(ui.Stdout, "Nothing to update: no plugin here was installed from a repo, or all are the latest.")
			}
			if failed > 0 {
				return &ExitCodeError{Code: 1}
			}
			return nil
		},
	}
}

func newPluginRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <name>",
		Short: "Start a plugin afresh: after building it again, or to hear what it adds now",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			c, err := connectPlugins(ctx.WS().Root)
			if err != nil {
				return err
			}
			defer c.Close()
			st, err := c.Refresh(context.Background(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stdout, "%s %s is %s\n", st.Name, st.Version, st.State)
			return nil
		},
	}
}

func newPluginLogsCmd() *cobra.Command {
	var lines int
	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "The end of a plugin's log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(plugin.LogPath(ctx.WS().Root, args[0]))
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%s has logged nothing (is it installed? ws plugin list)", args[0])
				}
				return err
			}
			all := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			if len(all) > lines {
				all = all[len(all)-lines:]
			}
			fmt.Fprintln(os.Stdout, strings.Join(all, "\n"))
			return nil
		},
	}
	cmd.Flags().IntVarP(&lines, "lines", "n", 100, "how many lines from the end")
	return cmd
}

func newPluginConfigCmd() *cobra.Command {
	var ask bool
	var unset []string
	cmd := &cobra.Command{
		Use:   "config <name> [key=value ...]",
		Short: "A plugin's config: see it, or set it",
		Long: `With nothing after the plugin's name, shows what it can be told and what's
set. key=value sets a key, and --unset removes one. --ask asks for each in
turn, which keeps a secret out of your shell's history.

A key that's a table holds named rows, each with the same fields, and a
field is set by its path: the table, the row's name, then the field. A row
is there once one of its fields is. A part of a path with a dot or a space
in it is quoted:

  ws plugin config jira queries.bugs.query='project = RAM, type = Bug'
  ws plugin config jira spaces.ram.custom_fields."Story Points".card=true
  ws plugin config jira spaces.ram.columns."In Review"=Review
  ws plugin config jira --unset queries.bugs

--unset with a row's path takes the row out of your file. A row that's the
team's stays theirs: <row>.enabled=false leaves it out for you.

What you set is yours: it goes in ws.local.toml ([plugin.<name>]), over the
team's in ws.toml, row by row and field by field. A secret goes in
neither, but in a file only you can read beside the plugin's data.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			root, name := ctx.WS().Root, args[0]
			st, err := pluginStatus(root, name)
			if err != nil {
				return err
			}
			keys := st.Registration.Config
			changed := false
			for _, pair := range args[1:] {
				typed, value, ok := cutPair(pair)
				if !ok {
					return fmt.Errorf("%q isn't key=value", pair)
				}
				at, err := configPlace(name, keys, typed)
				if err != nil {
					return err
				}
				switch {
				case at.Row:
					return fmt.Errorf("%s is a row of %s: set a field of it, as %s.<field>=value (it has: %s)", typed, at.Table.Key, typed, strings.Join(fieldKeys(at.Table), ", "))
				case len(at.Path) == 1 && (at.Key.Type == protocol.TypeTable || at.Key.Type == protocol.TypeMap):
					return fmt.Errorf("%s is %s: set one of its entries, as %s.<name>=value", typed, typeSaid(at.Key), typed)
				case at.Key.Type == protocol.TypeMap && !at.Entry:
					return fmt.Errorf("%s is text by text: set one of its entries, as %s.<name>=value", typed, typed)
				}
				k := at.Key
				if at.Entry {
					k = plugin.ConfigKey{Key: typed, Type: protocol.TypeString} // an entry of a map is text
				}
				v, err := plugin.ParseValue(k, value)
				if err != nil {
					return fmt.Errorf("%s: %w", typed, err)
				}
				if err := plugin.SetAt(root, name, at, v); err != nil {
					return err
				}
				changed = true
			}
			for _, typed := range unset {
				at, err := configPlace(name, keys, typed)
				if err != nil {
					return err
				}
				// A row that's only the team's isn't in your file to take
				// out: it's left out, for you, by saying so.
				if at.Row && !plugin.Yours(root, name, at.Path) {
					return fmt.Errorf("%s isn't in your ws.local.toml: if it's the team's, %s.enabled=false leaves it out for you", typed, typed)
				}
				if err := plugin.SetAt(root, name, at, nil); err != nil {
					return err
				}
				changed = true
			}
			if ask {
				if err := askPluginConfig(root, st, true); err != nil {
					return err
				}
				changed = true
			}
			if changed {
				if c, err := plugin.Dial(root); err == nil {
					c.ConfigChanged(context.Background(), name) //nolint:errcheck // it reads its config when it next starts
					c.Close()
				}
				return nil
			}
			return showPluginConfig(root, st)
		},
	}
	cmd.Flags().BoolVar(&ask, "ask", false, "ask for each key in turn")
	cmd.Flags().StringArrayVar(&unset, "unset", nil, "a key to remove")
	return cmd
}

// pluginStatus is one installed plugin, or an error that says there's no
// such one.
func pluginStatus(root, name string) (plugin.Status, error) {
	for _, st := range pluginStatuses(root) {
		if st.Name == name {
			return st, nil
		}
	}
	return plugin.Status{}, fmt.Errorf("there's no plugin called %s (ws plugin list)", name)
}

// askRows asks for a table's rows in the terminal, until it's told
// they're done: a row added, by its name and then its fields; one
// changed, a field at a time; or one taken out (of your file, or, where
// it's the team's, left out for you). path is the table's: a key, or a
// field of a row.
func askRows(root string, st plugin.Status, table plugin.ConfigKey, path []string) error {
	what := cmp.Or(table.Title, table.Key)
	for {
		rows, err := rowsAt(root, st, path)
		if err != nil {
			return err
		}
		options := []huh.Option[string]{huh.NewOption("Done", ""), huh.NewOption("Add a "+rowSaid(table), "+")}
		for _, r := range rows {
			said := r.Name
			if r.Off {
				said += " (off)"
			}
			options = append(options, huh.NewOption("Change "+said, "="+r.Name))
		}
		for _, r := range rows {
			if !r.Off {
				options = append(options, huh.NewOption("Take out "+r.Name, "-"+r.Name))
			}
		}
		choice := ""
		title := fmt.Sprintf("%s: %d %s", what, len(rows), plural(len(rows), "row"))
		if err := huh.NewSelect[string]().Title(title).Description(table.Help).Options(options...).Value(&choice).Run(); err != nil {
			return err
		}
		switch {
		case choice == "":
			return nil
		case choice == "+":
			name := ""
			if err := huh.NewInput().Title(cmp.Or(table.Row, "Name")).Description("What the new row is called: it's how it's named in the config.").Value(&name).Run(); err != nil {
				return err
			}
			if name = strings.TrimSpace(name); name == "" {
				continue
			}
			if err := askRow(root, st, table, append(slices.Clone(path), name), plugin.Row{Name: name}); err != nil {
				return err
			}
		case strings.HasPrefix(choice, "="):
			for _, r := range rows {
				if r.Name == choice[1:] {
					if err := askRow(root, st, table, append(slices.Clone(path), r.Name), r); err != nil {
						return err
					}
				}
			}
		default:
			if err := takeOutRow(root, st.Name, append(slices.Clone(path), choice[1:])); err != nil {
				return err
			}
		}
	}
}

// rowsAt are the rows of the table at path, as they're set: a key's, or
// those of a row's field that's a table.
func rowsAt(root string, st plugin.Status, path []string) ([]plugin.Row, error) {
	values, _, err := plugin.Shaped(root, st.Name, st.Registration.Config)
	if err != nil {
		return nil, err
	}
	rows, _ := values[path[0]].([]plugin.Row)
	for i := 1; i+1 < len(path); i += 2 {
		var next []plugin.Row
		for _, r := range rows {
			if r.Name == path[i] {
				next, _ = r.Values[path[i+1]].([]plugin.Row)
			}
		}
		rows = next
	}
	return rows, nil
}

// askRow asks for each field of a row in turn, with what it's set to now:
// one left empty is left as it was. Its tables are asked for after, a
// row at a time.
func askRow(root string, st plugin.Status, table plugin.ConfigKey, path []string, row plugin.Row) error {
	for _, f := range table.Fields {
		fieldPath := append(slices.Clone(path), f.Key)
		switch f.Type {
		case protocol.TypeTable:
			if err := askRows(root, st, f, fieldPath); err != nil {
				return err
			}
			continue
		case protocol.TypeMap:
			fmt.Fprintf(ui.Stderr, "  %s is text by text: ws plugin config %s %s.<name>=value sets an entry\n", f.Key, st.Name, plugin.JoinPath(fieldPath))
			continue
		}
		answer := ""
		if v, ok := row.Values[f.Key]; ok {
			answer = configShown(v)
		}
		title := cmp.Or(f.Title, f.Key)
		if f.Required {
			title += " (needed)"
		}
		if err := huh.NewInput().Title(row.Name + ": " + title).Description(f.Help).Placeholder(f.Example).Value(&answer).Run(); err != nil {
			return err
		}
		if strings.TrimSpace(answer) == "" {
			continue // left as it was
		}
		v, err := plugin.ParseValue(f, answer)
		if err != nil {
			return err
		}
		at, err := plugin.Find(st.Registration.Config, fieldPath)
		if err != nil {
			return err
		}
		if err := plugin.SetAt(root, st.Name, at, v); err != nil {
			return err
		}
	}
	return nil
}

// takeOutRow takes a row out: of your file, where it's there, and if
// it's still set after that (it's the team's too), it's left out for you
// with enabled = false.
func takeOutRow(root, name string, path []string) error {
	if plugin.Yours(root, name, path) {
		if err := plugin.SetAt(root, name, plugin.Place{Path: path, Row: true}, nil); err != nil {
			return err
		}
	}
	cfg, _, err := config.Load(root)
	if err != nil {
		return err
	}
	var in any = cfg.Plugin[name]
	for _, part := range path {
		m, ok := in.(map[string]any)
		if !ok {
			return nil
		}
		if in, ok = m[part]; !ok {
			return nil // it was only yours, and it's gone
		}
	}
	off := plugin.Place{Path: append(slices.Clone(path), plugin.RowEnabled), Key: plugin.ConfigKey{Key: plugin.RowEnabled, Type: protocol.TypeBool}}
	return plugin.SetAt(root, name, off, false)
}

// cutPair is key=value, cut at the first = that isn't in a quoted part
// of the key: a row's name may have one in it.
func cutPair(pair string) (key, value string, ok bool) {
	quoted := false
	for i, r := range pair {
		switch {
		case r == '"':
			quoted = !quoted
		case r == '=' && !quoted:
			return pair[:i], pair[i+1:], true
		}
	}
	return pair, "", false
}

// configPlace is where a key's path, as it's typed, leads in a plugin's
// config.
func configPlace(name string, keys []plugin.ConfigKey, typed string) (plugin.Place, error) {
	path, err := plugin.SplitPath(typed)
	if err != nil {
		return plugin.Place{}, err
	}
	at, err := plugin.Find(keys, path)
	if err != nil {
		return at, fmt.Errorf("%s: %w", name, err)
	}
	return at, nil
}

func fieldKeys(table plugin.ConfigKey) []string {
	out := make([]string, len(table.Fields))
	for i, f := range table.Fields {
		out[i] = f.Key
	}
	return out
}

// typeSaid is what a key that holds more than one value is, in words.
func typeSaid(k plugin.ConfigKey) string {
	if k.Type == protocol.TypeTable {
		return "a table of rows"
	}
	return "text by text"
}

// showPluginConfig lists a plugin's keys, and what each is set to: a
// table's rows under it, in the order the plugin's given them, and what's
// wrong in any of it last.
func showPluginConfig(root string, st plugin.Status) error {
	if len(st.Registration.Config) == 0 {
		fmt.Fprintf(ui.Stdout, "%s has no config.\n", st.Name)
		return nil
	}
	values, problems, err := plugin.Shaped(root, st.Name, st.Registration.Config)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(ui.Stdout, 0, 0, 2, ' ', 0)
	var tables []plugin.ConfigKey
	for _, k := range st.Registration.Config {
		shown := "(not set)"
		switch v, ok := values[k.Key]; {
		case k.Type == protocol.TypeTable:
			rows, _ := v.([]plugin.Row)
			shown = fmt.Sprintf("%d %s", len(rows), plural(len(rows), "row"))
			if len(rows) == 0 && k.Required {
				shown = "(no rows, and it needs one)"
			}
			tables = append(tables, k)
		case ok && k.Secret():
			shown = "(set)"
		case ok:
			shown = configShown(v)
		case k.Default != nil:
			shown = fmt.Sprintf("%v (the default)", k.Default)
		case k.Required:
			shown = "(not set, and required)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", k.Key, shown, k.Title)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	for _, k := range tables {
		rows, _ := values[k.Key].([]plugin.Row)
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(ui.Stdout, "\n%s, a row for each %s (%s.<%s>.<field>=value sets a field):\n", k.Key, rowSaid(k), k.Key, rowSaid(k))
		showRows(ui.Stdout, k, rows, []string{k.Key}, "  ")
	}
	if len(problems) > 0 {
		fmt.Fprintf(ui.Stdout, "\nWrong in it, and left out of what %s is given:\n", st.Name)
		for _, p := range problems {
			fmt.Fprintf(ui.Stdout, "  %s\n", p)
		}
	}
	return nil
}

// rowSaid is what a table's rows are named by, in words.
func rowSaid(table plugin.ConfigKey) string {
	if table.Row != "" {
		return strings.ToLower(table.Row)
	}
	return "name"
}

// showRows prints a table's rows, each with its fields that are set: a
// field that's a table as its own rows, under it.
func showRows(w io.Writer, table plugin.ConfigKey, rows []plugin.Row, path []string, indent string) {
	for _, r := range rows {
		name := plugin.JoinPath([]string{r.Name})
		switch {
		case r.Off:
			name += "  (off: enabled = false)"
		case r.Left != "":
			name += "  (left out: " + r.Left + ")"
		}
		fmt.Fprintf(w, "%s%s\n", indent, name)
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		var inner []plugin.ConfigKey
		for _, f := range table.Fields {
			v, ok := r.Values[f.Key]
			switch {
			case f.Type == protocol.TypeTable:
				if rows, _ := v.([]plugin.Row); len(rows) > 0 {
					inner = append(inner, f)
				}
			case ok:
				fmt.Fprintf(tw, "%s  %s\t%s\n", indent, f.Key, configShown(v))
			case f.Default != nil:
				fmt.Fprintf(tw, "%s  %s\t%v (the default)\n", indent, f.Key, f.Default)
			}
		}
		tw.Flush() //nolint:errcheck // to the terminal
		for _, f := range inner {
			fmt.Fprintf(w, "%s  %s:\n", indent, f.Key)
			showRows(w, f, r.Values[f.Key].([]plugin.Row), append(slices.Clone(path), r.Name, f.Key), indent+"    ")
		}
	}
}

// configShown is a value as it's listed: a list with commas, text by
// text as name = value.
func configShown(v any) string {
	switch x := v.(type) {
	case []string:
		return strings.Join(x, ", ")
	case map[string]string:
		names := make([]string, 0, len(x))
		for name := range x {
			names = append(names, name)
		}
		slices.Sort(names)
		for i, name := range names {
			names[i] = name + " = " + x[name]
		}
		return strings.Join(names, ", ")
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

// askPluginConfig asks for a plugin's keys in the terminal: every one, or
// (all false) those it requires that aren't set.
func askPluginConfig(root string, st plugin.Status, all bool) error {
	if !ui.IsInteractive() {
		return errors.New("asking needs a terminal: give key=value instead")
	}
	values, err := plugin.Values(root, st.Name)
	if err != nil {
		return err
	}
	for _, k := range st.Registration.Config {
		_, set := values[k.Key]
		if !all && (set || !k.Required || k.Default != nil) {
			continue
		}
		// A table's rows are asked for a row at a time, and text by text
		// is set an entry at a time: neither is one line to type.
		if k.Type == protocol.TypeTable {
			if err := askRows(root, st, k, []string{k.Key}); err != nil {
				return err
			}
			continue
		}
		if k.Type == protocol.TypeMap {
			fmt.Fprintf(ui.Stderr, "  %s is text by text: ws plugin config %s %s.<name>=value sets an entry\n", k.Key, st.Name, k.Key)
			continue
		}
		answer := ""
		if set && !k.Secret() {
			answer = fmt.Sprint(values[k.Key])
			if list, ok := values[k.Key].([]string); ok {
				answer = strings.Join(list, ", ")
			}
		}
		title := k.Title
		if title == "" {
			title = k.Key
		}
		input := huh.NewInput().Title(title).Description(k.Help).Placeholder(k.Example).Value(&answer)
		if k.Secret() {
			input = input.EchoMode(huh.EchoModePassword)
		}
		if err := input.Run(); err != nil {
			return err
		}
		if strings.TrimSpace(answer) == "" {
			continue // left as it was
		}
		v, err := plugin.ParseValue(k, answer)
		if err != nil {
			return err
		}
		if err := plugin.SetValue(root, st.Name, k, v); err != nil {
			return err
		}
	}
	return nil
}
