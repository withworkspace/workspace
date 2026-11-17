package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws plugin check: what a plugin registers, and what's wrong with it, as
// only ws can say: it's ws that would refuse it. Starting a plugin from a
// template and building it on every save are a plugin's own work
// (plugin-dev), on top of this and ws plugin add --build.

// pluginFolder is the folder of the plugin a command's about: the one
// given, else where the command was run.
func pluginFolder(args []string) (string, error) {
	dir := userDirOr()
	if len(args) > 0 {
		dir = args[0]
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(userDirOr(), dir)
		}
	}
	if _, err := plugin.ReadManifest(dir); err != nil {
		return "", err
	}
	return filepath.Abs(dir)
}

func newPluginCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [folder]",
		Short: "Build a plugin, start it, and say what it registers and what's wrong with it",
		Long: `Builds the plugin in a folder (default: this one) and starts it against a
workspace of its own with nothing in it, as it would be started when first
installed: its database's migrations are applied to an empty database, and
it says what it adds. Nothing of yours is touched.

It lists what the plugin registers, and what's wrong with it: something
that would stop it working fails the check; the rest is worth knowing.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			dir, err := pluginFolder(args)
			if err != nil {
				return err
			}
			fatal, err := checkPlugin(dir, ui.Stdout)
			if err != nil {
				return err
			}
			if fatal {
				return &ExitCodeError{Code: 1}
			}
			return nil
		},
	}
}

// checkPlugin builds the plugin in dir, starts it in a workspace made for
// the purpose, and writes what it registers and what's wrong with it. It
// reports whether anything is that would stop it working.
func checkPlugin(dir string, out io.Writer) (fatal bool, err error) {
	m, err := plugin.ReadManifest(dir)
	if err != nil {
		return false, err
	}
	var built strings.Builder
	if err := plugin.Build(dir, &built); err != nil {
		fmt.Fprintf(out, "%s%s: it doesn't build\n", built.String(), err)
		return true, nil
	}
	scratch, err := os.MkdirTemp("", "ws-plugin-check-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(scratch)
	if err := os.WriteFile(filepath.Join(scratch, config.FileName), []byte("[workspace]\norg = \"check\"\n"), 0o644); err != nil {
		return false, err
	}
	if _, err := plugin.Add(scratch, dir, true); err != nil {
		return false, err
	}
	host := &plugin.Host{Root: scratch, Version: "check", Reserved: reservedWord}
	defer host.Shutdown()
	st, err := host.Refresh(m.Name)
	if err != nil {
		fmt.Fprintf(out, "%s %s doesn't start: %v\n", m.Name, m.Version, err)
		if log, lerr := os.ReadFile(plugin.LogPath(scratch, m.Name)); lerr == nil && len(log) > 0 {
			fmt.Fprintf(out, "\nIts log:\n%s", indent(strings.TrimRight(tail(string(log), 20), "\n"), "  ")+"\n")
		}
		return true, nil
	}
	// A plugin says what went wrong as it started in its log: a migration
	// that doesn't apply to an empty database is the usual one.
	time.Sleep(100 * time.Millisecond)
	st = host.Status(m.Name)

	r := st.Registration
	fmt.Fprintf(out, "%s %s\n", m.Name, m.Version)
	if r.Description != "" {
		fmt.Fprintf(out, "  %s\n", r.Description)
	}
	fmt.Fprintln(out)
	for _, v := range r.Views {
		fmt.Fprintf(out, "  view      %s\n", v.Title)
	}
	if word := r.CommandName(m.Name); word != "" {
		for _, c := range r.Commands.Subcommands {
			fmt.Fprintf(out, "  command   ws %s %s\n", word, strings.TrimSpace(c.Name+" "+c.Args))
		}
	}
	for _, k := range r.Config {
		kind := k.Type
		if k.Required {
			kind += ", required"
		}
		fmt.Fprintf(out, "  config    %s (%s)\n", k.Key, kind)
	}
	for _, s := range r.Syncs {
		fmt.Fprintf(out, "  sync      %s\n", s.Name)
	}
	if r.Agent != nil && strings.TrimSpace(r.Agent.Guide) != "" {
		fmt.Fprintf(out, "  guide     %d lines, for agents\n", strings.Count(strings.TrimSpace(r.Agent.Guide), "\n")+1)
	}

	problems := plugin.Problems(m.Name, r)
	if st.Error != "" {
		problems = append([]plugin.Problem{{Fatal: true, Text: "it said as it started: " + st.Error}}, problems...)
	}
	if len(problems) == 0 {
		fmt.Fprintln(out, "\nNothing's wrong with it.")
		return false, nil
	}
	fmt.Fprintln(out)
	for _, p := range problems {
		mark := "  note   "
		if p.Fatal {
			mark, fatal = "  wrong  ", true
		}
		fmt.Fprintln(out, mark+p.Text)
	}
	return fatal, nil
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n") + "\n"
}
