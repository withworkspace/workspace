package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/workspace"
)

func TestPluginsView(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := &workspace.Workspace{Root: root}
	v, err := pluginsView(ws, "")
	if err != nil || !strings.Contains(v.render(false, false), "None is set up with one") {
		t.Fatalf("with no plugins (%v):\n%s", err, v.render(false, false))
	}

	app := filepath.Join(plugin.Dir(root), "jira", "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "plugin.toml"), []byte("name = \"jira\"\nversion = \"1.0.0\"\ncommand = [\"./x\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, _ := json.Marshal(plugin.Registration{
		Commands: &plugin.Commands{Name: "j"},
		Agent:    &plugin.Agent{Guide: "The workspace has a Jira board.\n\n`ws j mine` lists your issues.", Roles: []string{"crew"}},
	})
	if err := os.WriteFile(filepath.Join(plugin.Dir(root), "jira", "registration.json"), reg, 0o644); err != nil {
		t.Fatal(err)
	}

	v, err = pluginsView(ws, "")
	if list := v.render(false, false); err != nil || !strings.Contains(list, "- jira: The workspace has a Jira board.") || strings.Contains(list, "ws j mine") {
		t.Errorf("the list (%v):\n%s", err, list)
	}
	v, err = pluginsView(ws, "jira")
	if one := v.render(false, false); err != nil || !strings.Contains(one, "`ws j mine` lists your issues.") || !strings.Contains(one, "ws j --help") {
		t.Errorf("jira's guide (%v):\n%s", err, one)
	}
	if _, err := pluginsView(ws, "sketch"); err == nil || !strings.Contains(err.Error(), "ws show plugins lists") {
		t.Errorf("a plugin that isn't there: %v", err)
	}
}
