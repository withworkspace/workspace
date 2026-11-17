package plugin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	for _, want := range []string{"view 1 has no title", `its command "Not OK"`, `of type "colour"`, "declared twice", "the list is empty", "has no name"} {
		if !strings.Contains(fatal, want) {
			t.Errorf("what's wrong lacks %q:\n%s", want, fatal)
		}
	}
	for _, want := range []string{"has no description", "its guide is 60 lines", "config key a has no title"} {
		if !strings.Contains(notes, want) {
			t.Errorf("what's worth knowing lacks %q:\n%s", want, notes)
		}
	}

	// Commands and no guide: agents aren't told.
	_, notes = texts(plugin.Problems("quiet", plugin.Registration{Commands: &plugin.Commands{Short: "x", Subcommands: []plugin.Subcommand{{Name: "a", Short: "b"}}}}))
	if !strings.Contains(notes, "agents won't know ws quiet is there") {
		t.Errorf("commands with no guide: %q", notes)
	}
}
