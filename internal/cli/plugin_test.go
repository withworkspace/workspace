package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/plugin/fakeplugin"
	"github.com/withworkspace/workspace/internal/testutil"
)

// registerPlugin writes a plugin into a workspace as one that's been
// installed and started: its plugin.toml, and what it registered. Nothing
// here runs one: that's the daemon's (internal/plugin's tests).
func registerPlugin(t *testing.T, root, name string, reg plugin.Registration) {
	t.Helper()
	app := filepath.Join(plugin.Dir(root), name, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "name = \"" + name + "\"\nversion = \"0.3.0\"\ncommand = [\"./ws-" + name + "\"]\n"
	if err := os.WriteFile(filepath.Join(app, "plugin.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(reg)
	if err := os.WriteFile(filepath.Join(plugin.Dir(root), name, "registration.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

var jira = plugin.Registration{
	Views:    []plugin.View{{Title: "Jira", Path: "/"}},
	Commands: &plugin.Commands{Short: "Jira issues"},
	Config: []plugin.ConfigKey{
		{Key: "site", Title: "Jira site", Type: "url", Required: true},
		{Key: "token", Title: "API token", Type: "secret", Required: true},
		{Key: "projects", Title: "Projects to show", Type: "list"},
		{Key: "greeting", Title: "Greeting", Type: "string", Default: "Hello"},
	},
}

func TestPluginList(t *testing.T) {
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{})
	if res := testutil.RunCommand(t, w.Root, nil, "plugin", "list"); res.Err != nil || !strings.Contains(res.Stdout, "No plugins") {
		t.Errorf("with none: %v %q", res.Err, res.Stdout)
	}
	registerPlugin(t, w.Root, "jira", jira)
	res := testutil.RunCommand(t, w.Root, nil, "plugin", "list")
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	// Installed, not running here, and not set up.
	for _, want := range []string{"jira", "0.3.0", "setup", "needs site, token"} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("list lacks %q:\n%s", want, res.Stdout)
		}
	}
	res = testutil.RunCommand(t, w.Root, nil, "plugin", "list", "-f", "json")
	var statuses []plugin.Status
	if err := json.Unmarshal([]byte(res.Stdout), &statuses); err != nil || len(statuses) != 1 || statuses[0].State != plugin.Setup {
		t.Errorf("as JSON: %v\n%s", err, res.Stdout)
	}
}

func TestPluginConfig(t *testing.T) {
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{})
	registerPlugin(t, w.Root, "jira", jira)
	run := func(args ...string) testutil.Result {
		t.Helper()
		return testutil.RunCommand(t, w.Root, nil, append([]string{"plugin", "config", "jira"}, args...)...)
	}

	res := run()
	for _, want := range []string{"site", "(not set, and required)", "Hello (the default)"} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("before anything's set, it lacks %q:\n%s", want, res.Stdout)
		}
	}

	if res := run("site=https://acme.atlassian.net", "token=s3cret", "projects=PROJ, OPS"); res.Err != nil {
		t.Fatal(res.Err)
	}
	local, _ := os.ReadFile(filepath.Join(w.Root, "ws.local.toml"))
	if !strings.Contains(string(local), "[plugin.jira]") || !strings.Contains(string(local), `site = "https://acme.atlassian.net"`) {
		t.Errorf("ws.local.toml:\n%s", local)
	}
	if strings.Contains(string(local), "s3cret") {
		t.Errorf("the secret is in ws.local.toml:\n%s", local)
	}
	res = run()
	if strings.Contains(res.Stdout, "s3cret") || !strings.Contains(res.Stdout, "(set)") || !strings.Contains(res.Stdout, "[PROJ OPS]") {
		t.Errorf("once it's set:\n%s", res.Stdout)
	}
	values, err := plugin.Values(w.Root, "jira")
	if err != nil || values["token"] != "s3cret" || len(plugin.Missing(jira, values)) != 0 {
		t.Errorf("values = %v, %v", values, err)
	}

	// What can't be right is refused, and nothing's set.
	for _, bad := range [][]string{{"site=not an address"}, {"nope=1"}, {"site"}} {
		if res := run(bad...); res.Err == nil {
			t.Errorf("%v was taken", bad)
		}
	}
	if res := run("--unset", "projects"); res.Err != nil {
		t.Fatal(res.Err)
	}
	if values, _ := plugin.Values(w.Root, "jira"); values["projects"] != nil {
		t.Errorf("projects after --unset = %v", values["projects"])
	}
	if res := testutil.RunCommand(t, w.Root, nil, "plugin", "config", "nobody"); res.Err == nil {
		t.Error("a plugin that isn't installed has config")
	}
}

func TestPluginCommandsInHelp(t *testing.T) {
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{})
	registerPlugin(t, w.Root, "jira", jira)
	// One that asks for a word of ws's own isn't given it.
	registerPlugin(t, w.Root, "taker", plugin.Registration{Commands: &plugin.Commands{Name: "status", Short: "Not ws's status"}})
	// One with no commands adds none.
	registerPlugin(t, w.Root, "quiet", plugin.Registration{Views: []plugin.View{{Title: "Quiet"}}})
	t.Setenv(cli.ShellPWD, w.Root)

	root := cli.NewRootCmd("test")
	cmd, _, err := root.Find([]string{"jira", "mine", "--all"})
	if err != nil || cmd.Name() != "jira" || cmd.Short != "Jira issues" {
		t.Fatalf("ws jira: %v, %v", cmd, err)
	}
	if !cmd.DisableFlagParsing {
		t.Error("ws jira's flags are ws's, not the plugin's")
	}
	if cmd, _, _ := root.Find([]string{"status"}); cmd.Short == "Not ws's status" {
		t.Error("a plugin took ws status")
	}
	if cmd, _, err := root.Find([]string{"quiet"}); err == nil && cmd != root {
		t.Error("a plugin with no commands has one")
	}
	res := testutil.RunCommand(t, w.Root, nil, "--help")
	if !strings.Contains(res.Stdout, "Plugins' Commands:") || !strings.Contains(res.Stdout, "Jira issues") {
		t.Errorf("help:\n%s", res.Stdout)
	}
}

func TestPluginCheck(t *testing.T) {
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{})

	// A plugin that starts: what it registers, and what's worth knowing.
	res := testutil.RunCommand(t, w.Root, nil, "plugin", "check", fakeplugin.Install(t, "fake", fakeplugin.Options{}))
	if res.Err != nil {
		t.Fatalf("a plugin with nothing wrong: %v\n%s", res.Err, res.Stdout)
	}
	for _, want := range []string{"fake 1.2.3", "view      Fake", "config    greeting (string)", "guide     1 lines, for agents", "note   ws fake has no subcommands"} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("check lacks %q:\n%s", want, res.Stdout)
		}
	}
	// It ran apart from the workspace: nothing's installed here.
	if got := plugin.Installed(w.Root); len(got) != 0 {
		t.Errorf("check installed %v", got)
	}

	// One that asks for a word of ws's own doesn't start, and fails it.
	res = testutil.RunCommand(t, w.Root, nil, "plugin", "check", fakeplugin.Install(t, "taker", fakeplugin.Options{Command: "status"}))
	if res.Err == nil || !strings.Contains(res.Stdout, "doesn't start") || !strings.Contains(res.Stdout, "ws's own command") {
		t.Errorf("a plugin taking ws status: %v\n%s", res.Err, res.Stdout)
	}

	// A folder that isn't a plugin's.
	if res := testutil.RunCommand(t, w.Root, nil, "plugin", "check", t.TempDir()); res.Err == nil {
		t.Error("a folder with no plugin.toml was checked")
	}
}
