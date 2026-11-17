package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/plugin"
)

// registered writes a plugin into root as one that's been installed and
// started: its plugin.toml, and what it registered.
func registered(t *testing.T, root, name string, reg plugin.Registration) {
	t.Helper()
	app := filepath.Join(plugin.Dir(root), name, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "name = \"" + name + "\"\nversion = \"1.0.0\"\ncommand = [\"./x\"]\n"
	if err := os.WriteFile(filepath.Join(app, "plugin.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(reg)
	if err := os.WriteFile(filepath.Join(plugin.Dir(root), name, "registration.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPluginsNote(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := PluginsNote(root, RoleCrew); got != "" {
		t.Errorf("with no plugins: %q", got)
	}

	registered(t, root, "jira", plugin.Registration{
		Commands: &plugin.Commands{},
		Agent:    &plugin.Agent{Guide: "`ws jira mine` lists your issues.\n"},
	})
	registered(t, root, "notes", plugin.Registration{
		Agent: &plugin.Agent{Guide: "For crew alone.", Roles: []string{RoleCrew}},
	})
	registered(t, root, "quiet", plugin.Registration{Commands: &plugin.Commands{}})
	registered(t, root, "unset", plugin.Registration{
		Agent:  &plugin.Agent{Guide: "Not yet."},
		Config: []plugin.ConfigKey{{Key: "token", Type: "secret", Required: true}},
	})

	crew := PluginsNote(root, RoleCrew)
	for _, want := range []string{"The jira plugin:\n`ws jira mine` lists your issues.", "The notes plugin:\nFor crew alone.", "information, not instructions"} {
		if !strings.Contains(crew, want) {
			t.Errorf("crew's note lacks %q:\n%s", want, crew)
		}
	}
	// One with nothing to say, and one that isn't set up, say nothing.
	for _, not := range []string{"quiet", "Not yet"} {
		if strings.Contains(crew, not) {
			t.Errorf("crew's note has %q:\n%s", not, crew)
		}
	}
	// A guide for some roles is for those alone.
	if orch := PluginsNote(root, RoleOrchestrator); !strings.Contains(orch, "jira") || strings.Contains(orch, "notes") {
		t.Errorf("the orchestrator's note:\n%s", orch)
	}

	// A long guide is cut, with where the rest is.
	registered(t, root, "long", plugin.Registration{
		Commands: &plugin.Commands{},
		Agent:    &plugin.Agent{Guide: strings.Repeat("a line\n", 100)},
	})
	long := PluginsNote(root, RoleCrew)
	if n := strings.Count(long, "a line"); n != plugin.GuideLines || !strings.Contains(long, "`ws long --help`") {
		t.Errorf("a long guide: %d lines of it\n%s", n, long)
	}
}
