package plugin

import (
	"errors"
	"fmt"
	"github.com/withworkspace/plugin/protocol"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// What ws does for a plugin being written: builds it from its source, and
// says what's wrong with what it registers (ws plugin add --build, ws
// plugin check). Starting one, and building it on every save, are a
// plugin's own work: plugin-dev's.

// GuideLines is how much of a plugin's guide an agent is given: it's told
// in every prompt, so it's a pointer to the plugin's commands, not its
// manual.
const GuideLines = 40

// Build builds the plugin in dir as its plugin.toml says (build), with
// what the build prints written to out. A plugin with no build has
// nothing to do.
func Build(dir string, out io.Writer) error {
	m, err := ReadManifest(dir)
	if err != nil {
		return err
	}
	if len(m.Build) == 0 {
		return nil
	}
	cmd := exec.Command(m.Build[0], m.Build[1:]...) //nolint:noctx // no context comes through Build yet
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return fmt.Errorf("%s failed", strings.Join(m.Build, " "))
		}
		return fmt.Errorf("%s: %w", strings.Join(m.Build, " "), err)
	}
	return nil
}

// Executable is the file a plugin's command starts, where it's a file of
// the plugin's own ("./ws-jira"); "" where it's a program found on PATH.
func (m Manifest) Executable(dir string) string {
	if len(m.Command) == 0 || !strings.ContainsRune(m.Command[0], filepath.Separator) {
		return ""
	}
	if filepath.IsAbs(m.Command[0]) {
		return m.Command[0]
	}
	return filepath.Join(dir, m.Command[0])
}

// BuildIfMissing builds the plugin in dir if what its command starts
// isn't there: one added from its source.
func BuildIfMissing(dir string, out io.Writer) error {
	m, err := ReadManifest(dir)
	if err != nil {
		return err
	}
	exe := m.Executable(dir)
	if exe == "" {
		return nil
	}
	if _, err := os.Stat(exe); err == nil {
		return nil
	}
	if len(m.Build) == 0 {
		return fmt.Errorf("%s not found, and %s has no build command (build)", m.Command[0], ManifestName)
	}
	return Build(dir, out)
}

// Problem is something wrong with what a plugin registers. One that isn't
// Fatal is worth knowing, and the plugin works as it is.
type Problem struct {
	Fatal bool
	Text  string
}

// configTypes are the types a config key can have.
var configTypes = []string{"string", "url", "bool", "number", "enum", "list", "secret", "map", "table"}

// Problems are what's wrong with what a plugin called name registers, as
// someone using it would find it: nothing to see it by, a command nothing
// says the use of, a guide agents are given part of.
func Problems(name string, r Registration) []Problem {
	var out []Problem
	note := func(format string, args ...any) { out = append(out, Problem{Text: fmt.Sprintf(format, args...)}) }
	fatal := func(format string, args ...any) {
		out = append(out, Problem{Fatal: true, Text: fmt.Sprintf(format, args...)})
	}

	word := r.CommandName(name)
	if len(r.Views) == 0 && word == "" && len(r.Refs) == 0 && len(r.Actions) == 0 {
		note("it adds no view and no commands: there's nothing to use it by")
	}
	for i, v := range r.Views {
		if strings.TrimSpace(v.Title) == "" {
			fatal("view %d has no title: it's what its sidebar entry says", i+1)
		}
	}
	// A view is a place in the plugin's app, which ws loads into a page of
	// its own: with no app, there's nothing to show there.
	switch {
	case len(r.Views) > 0 && (r.UI == nil || r.UI.Script == ""):
		fatal("it has a view and no ui.script: ws serves a plugin's page, and loads the app the plugin names there")
	case r.UI != nil && len(r.Views) == 0:
		note("it has a ui and no view: nothing opens it")
	}
	if r.UI != nil {
		for _, f := range []struct{ field, path string }{{"ui.script", r.UI.Script}, {"ui.style", r.UI.Style}} {
			if f.path != "" && !ValidUIPath(f.path) {
				fatal("%s is %q: it has to be a path in the plugin's own HTTP, starting with one / and with no ..", f.field, f.path)
			}
		}
	}
	if r.Commands != nil {
		if word != "" && !ValidName(word) {
			fatal("its command %q must be lower-case letters, digits and hyphens", word)
		}
		if r.Commands.Short == "" {
			note("ws %s has no description: ws --help lists it with nothing beside it", word)
		}
		if len(r.Commands.Subcommands) == 0 {
			note("ws %s has no subcommands", word)
		}
		for _, c := range r.Commands.Subcommands {
			if c.Short == "" {
				note("ws %s %s has no description", word, c.Name)
			}
		}
	}
	guide := ""
	if r.Agent != nil {
		guide = strings.TrimSpace(r.Agent.Guide)
	}
	switch lines := strings.Count(guide, "\n") + 1; {
	case guide == "" && word != "":
		note("it has commands and no guide (agents.md): agents won't know ws %s is there", word)
	case guide != "" && lines > GuideLines:
		note("its guide is %d lines, and agents are given the first %d: keep it to what the commands are", lines, GuideLines)
	}
	// What it can do from Find.
	if problem := protocol.ActionsProblem(r.Actions); problem != "" {
		fatal("%s", problem)
	}
	// How its things are named: ws leaves out a ref that's wrong, so one
	// here would name nothing.
	kinds := map[string]bool{}
	for _, ref := range r.Refs {
		if problem := ref.Problem(); problem != "" {
			fatal("its ref %q: %s", ref.Kind, problem)
			continue
		}
		if kinds[ref.Kind] {
			fatal("its ref %q is registered twice", ref.Kind)
		}
		kinds[ref.Kind] = true
		if !named(name, ref).Names(ref.Prefixes[0] + "1") {
			note("its ref %q has the tail %q, which this ws doesn't know: none of its names are read", ref.Kind, ref.Tail)
		}
		if ref.Title == "" {
			note("its ref %q has no title: one is called %s %s", ref.Kind, name, ref.Kind)
		}
		if ref.Address == "" && ref.Path == "" {
			note("its ref %q has no address and no path: a name of its goes nowhere but the plugin's first page", ref.Kind)
		}
	}
	// A key, and for a table its rows' fields in turn: at is where it is
	// (spaces.custom_fields), and depth how many tables it's inside.
	var checkKeys func(keys []ConfigKey, at string, depth int)
	checkKeys = func(keys []ConfigKey, at string, depth int) {
		what := "config key"
		if depth > 0 {
			what = "config table " + at + "'s field"
		}
		seen := map[string]bool{}
		for _, k := range keys {
			switch {
			case k.Key == "":
				fatal("a %s has no name", what)
				continue
			case seen[k.Key]:
				fatal("%s %s is declared twice", what, k.Key)
			case depth > 0 && k.Key == RowEnabled:
				fatal("%s %s is every row's already: whether it's given to the plugin", what, k.Key)
			}
			seen[k.Key] = true
			if k.Title == "" {
				note("%s %s has no title: it's asked for by its name", what, k.Key)
			}
			typ := k.Type
			if typ == "" {
				typ = "string"
			}
			known := false
			for _, t := range configTypes {
				known = known || t == typ
			}
			switch {
			case !known:
				fatal("%s %s is of type %q, which ws doesn't know (it knows %s)", what, k.Key, typ, strings.Join(configTypes, ", "))
			case typ == "enum" && len(k.Options) == 0:
				fatal("%s %s is one of a list, and the list is empty", what, k.Key)
			case typ == "secret" && depth > 0:
				fatal("%s %s is a secret, and a secret is a key of its own, not a row's", what, k.Key)
			case typ == protocol.TypeTable && depth > 1:
				fatal("%s %s is a table in a table in a table: a table goes one level down", what, k.Key)
			case typ == protocol.TypeTable && len(k.Fields) == 0:
				fatal("%s %s is a table, and its rows have no fields", what, k.Key)
			case typ != protocol.TypeTable && len(k.Fields) > 0:
				fatal("%s %s has fields, and isn't a table", what, k.Key)
			}
			if typ == protocol.TypeTable {
				in := k.Key
				if at != "" {
					in = at + "." + k.Key
				}
				checkKeys(k.Fields, in, depth+1)
			}
		}
	}
	checkKeys(r.Config, "", 0)
	return out
}
