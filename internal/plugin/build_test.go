package plugin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/plugin"
)

// source is a plugin's folder with a plugin.toml whose build is the shell
// command given.
func source(t *testing.T, build string) string {
	t.Helper()
	dir := t.TempDir()
	manifest := "name = \"built\"\nversion = \"0.1.0\"\ncommand = [\"./ws-built\"]\n"
	if build != "" {
		manifest += "build = [\"sh\", \"-c\", " + `"` + build + `"` + "]\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuild(t *testing.T) {
	dir := source(t, "echo compiling; echo x > ws-built")
	var out strings.Builder
	if err := plugin.Build(dir, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "compiling\n" {
		t.Errorf("what the build printed: %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "ws-built")); err != nil {
		t.Errorf("it wasn't run in the plugin's folder: %v", err)
	}

	// One that fails says so, with what it printed.
	out.Reset()
	if err := plugin.Build(source(t, "echo main.go:3: nope >&2; exit 2"), &out); err == nil || !strings.Contains(err.Error(), "failed") {
		t.Errorf("a failing build: %v", err)
	}
	if !strings.Contains(out.String(), "main.go:3: nope") {
		t.Errorf("what the failing build printed: %q", out.String())
	}

	// With no build, there's nothing to do.
	if err := plugin.Build(source(t, ""), &out); err != nil {
		t.Errorf("a plugin with no build: %v", err)
	}
}

func TestBuildIfMissing(t *testing.T) {
	dir := source(t, "echo x >> ws-built")
	var out strings.Builder
	for range 2 {
		if err := plugin.BuildIfMissing(dir, &out); err != nil {
			t.Fatal(err)
		}
	}
	// Built once: the second time it was there.
	if data, _ := os.ReadFile(filepath.Join(dir, "ws-built")); string(data) != "x\n" {
		t.Errorf("built %q", data)
	}
	// One that isn't there and can't be built says what's missing.
	if err := plugin.BuildIfMissing(source(t, ""), &out); err == nil || !strings.Contains(err.Error(), "build") {
		t.Errorf("nothing built and no build: %v", err)
	}
}

func TestProblems(t *testing.T) {
	texts := func(ps []plugin.Problem) (fatal, notes string) {
		for _, p := range ps {
			if p.Fatal {
				fatal += p.Text + "\n"
			} else {
				notes += p.Text + "\n"
			}
		}
		return
	}
	good := plugin.Registration{
		Views:    []plugin.View{{Title: "Jira"}},
		UI:       &plugin.UI{Script: "/ui/app.js?v=3f9c2a", Style: "/ui/app.css"},
		Commands: &plugin.Commands{Short: "Jira issues", Subcommands: []plugin.Subcommand{{Name: "mine", Short: "Yours"}}},
		Agent:    &plugin.Agent{Guide: "`ws jira mine` lists yours."},
		Config:   []plugin.ConfigKey{{Key: "site", Title: "Site", Type: "url"}},
	}
	if ps := plugin.Problems("jira", good); len(ps) != 0 {
		t.Errorf("nothing's wrong with it, and it said: %+v", ps)
	}

	fatal, notes := texts(plugin.Problems("empty", plugin.Registration{}))
	if fatal != "" || !strings.Contains(notes, "nothing to use it by") {
		t.Errorf("one that adds nothing: fatal %q, notes %q", fatal, notes)
	}

	bad := plugin.Registration{
		Views:    []plugin.View{{Title: " "}},
		UI:       &plugin.UI{Script: "/ui/../../api/app.js", Style: "//elsewhere.example/app.css"},
		Commands: &plugin.Commands{Name: "Not OK", Subcommands: []plugin.Subcommand{{Name: "go"}}},
		Agent:    &plugin.Agent{Guide: strings.Repeat("line\n", 60)},
		Config: []plugin.ConfigKey{
			{Key: "a", Type: "colour"},
			{Key: "a"},
			{Key: "pick", Title: "Pick", Type: "enum"},
			{Title: "Nameless"},
		},
	}
	fatal, notes = texts(plugin.Problems("bad", bad))
	for _, want := range []string{"view 1 has no title", `ui.script is "/ui/../../api/app.js"`, `ui.style is "//elsewhere.example/app.css"`, `its command "Not OK"`, `of type "colour"`, "declared twice", "the list is empty", "has no name"} {
		if !strings.Contains(fatal, want) {
			t.Errorf("what's wrong lacks %q:\n%s", want, fatal)
		}
	}
	for _, want := range []string{"has no description", "its guide is 60 lines", "config key a has no title"} {
		if !strings.Contains(notes, want) {
			t.Errorf("what's worth knowing lacks %q:\n%s", want, notes)
		}
	}

	// A view is a place in the plugin's app: one with no app has nothing
	// to show, and an app with no view has nothing that opens it.
	fatal, _ = texts(plugin.Problems("old", plugin.Registration{Views: []plugin.View{{Title: "Old"}}}))
	if !strings.Contains(fatal, "it has a view and no ui.script") {
		t.Errorf("a view and no app: %q", fatal)
	}
	fatal, notes = texts(plugin.Problems("hidden", plugin.Registration{UI: &plugin.UI{Script: "/ui/app.js"}}))
	if fatal != "" || !strings.Contains(notes, "nothing opens it") {
		t.Errorf("an app and no view: fatal %q, notes %q", fatal, notes)
	}

	// How its things are named: a ref ws would leave out is what's wrong,
	// and one that names things and has nothing else is still of use.
	fatal, notes = texts(plugin.Problems("names", plugin.Registration{Refs: []protocol.Ref{
		{Kind: "issue", Title: "Issue", Prefixes: []string{"RAM-"}, Address: "https://acme.example/{ref}"},
		{Kind: "issue", Title: "Issue", Prefixes: []string{"RAP-"}, Path: "/issue/{ref}"},
		{Kind: "sprint", Prefixes: []string{"sprint"}},
		{Kind: "event", Prefixes: []string{"EV-"}, Tail: "hex"},
	}}))
	for _, want := range []string{`its ref "issue" is registered twice`, `its ref "sprint": its prefix "sprint"`} {
		if !strings.Contains(fatal, want) {
			t.Errorf("what's wrong with its refs lacks %q:\n%s", want, fatal)
		}
	}
	for _, want := range []string{`its ref "event" has the tail "hex"`, `its ref "event" has no title`, `its ref "event" has no address and no path`} {
		if !strings.Contains(notes, want) {
			t.Errorf("what's worth knowing of its refs lacks %q:\n%s", want, notes)
		}
	}
	if strings.Contains(notes, "nothing to use it by") {
		t.Errorf("one that names things is of use:\n%s", notes)
	}

	// Config in tables: a table with its rows' fields, one of them a
	// table in turn, and text by text, are nothing wrong; what's wrong
	// with one is said with where it is.
	tables := plugin.Registration{Commands: &plugin.Commands{Short: "x", Subcommands: []plugin.Subcommand{{Name: "a", Short: "b"}}}, Agent: &plugin.Agent{Guide: "g"}, Config: []plugin.ConfigKey{
		{Key: "statuses", Title: "Statuses", Type: protocol.TypeMap},
		{Key: "spaces", Title: "Spaces", Type: protocol.TypeTable, Row: "Space key", Fields: []plugin.ConfigKey{
			{Key: "label", Title: "Label", Type: protocol.TypeString},
			{Key: "columns", Title: "Columns", Type: protocol.TypeMap},
			{Key: "custom_fields", Title: "Custom fields", Type: protocol.TypeTable, Fields: []plugin.ConfigKey{{Key: "card", Title: "On cards", Type: protocol.TypeBool}}},
		}},
	}}
	if ps := plugin.Problems("board", tables); len(ps) != 0 {
		t.Errorf("nothing's wrong with its tables, and it said: %+v", ps)
	}
	fatal, notes = texts(plugin.Problems("board", plugin.Registration{Commands: &plugin.Commands{Short: "x", Subcommands: []plugin.Subcommand{{Name: "a", Short: "b"}}}, Agent: &plugin.Agent{Guide: "g"}, Config: []plugin.ConfigKey{
		{Key: "empty", Title: "Empty", Type: protocol.TypeTable},
		{Key: "site", Title: "Site", Type: protocol.TypeURL, Fields: []plugin.ConfigKey{{Key: "x", Title: "X"}}},
		{Key: "spaces", Title: "Spaces", Type: protocol.TypeTable, Fields: []plugin.ConfigKey{
			{Key: "label", Title: "Label"},
			{Key: "label", Title: "Label again"},
			{Key: "token", Title: "Token", Type: protocol.TypeSecret},
			{Key: "enabled", Title: "Enabled", Type: protocol.TypeBool},
			{Key: "kind", Type: "colour"},
			{Key: "custom_fields", Title: "Custom fields", Type: protocol.TypeTable, Fields: []plugin.ConfigKey{
				{Key: "options", Title: "Options", Type: protocol.TypeTable, Fields: []plugin.ConfigKey{{Key: "x", Title: "X"}}},
			}},
		}},
	}}))
	for _, want := range []string{
		"config key empty is a table, and its rows have no fields",
		"config key site has fields, and isn't a table",
		"config table spaces's field label is declared twice",
		"config table spaces's field token is a secret, and a secret is a key of its own, not a row's",
		"config table spaces's field enabled is every row's already",
		`config table spaces's field kind is of type "colour", which ws doesn't know (it knows string, url, bool, number, enum, list, secret, map, table)`,
		"config table spaces.custom_fields's field options is a table in a table in a table: a table goes one level down",
	} {
		if !strings.Contains(fatal, want) {
			t.Errorf("what's wrong with its tables lacks %q:\n%s", want, fatal)
		}
	}
	if !strings.Contains(notes, "config table spaces's field kind has no title") {
		t.Errorf("a field with no title: %q", notes)
	}

	// Commands and no guide: agents aren't told.
	_, notes = texts(plugin.Problems("quiet", plugin.Registration{Commands: &plugin.Commands{Short: "x", Subcommands: []plugin.Subcommand{{Name: "a", Short: "b"}}}}))
	if !strings.Contains(notes, "agents won't know ws quiet is there") {
		t.Errorf("commands with no guide: %q", notes)
	}
}

func TestValidUIPath(t *testing.T) {
	for path, want := range map[string]bool{
		"/ui/app.js":                     true,
		"/ui/app.js?v=3f9c2a":            true,
		"/app.css":                       true,
		"":                               false,
		"ui/app.js":                      false,
		"//elsewhere.example/a":          false,
		"https://elsewhere.example/a.js": false,
		"/ui/../app.js":                  false,
		"/ui/%2e%2e/app.js":              false,
		"/ui/./app.js":                   false,
		`/\elsewhere.example/a`:          false,
		"/ui/app.js#x":                   false,
	} {
		if got := plugin.ValidUIPath(path); got != want {
			t.Errorf("ValidUIPath(%q) = %v, want %v", path, got, want)
		}
	}
}
