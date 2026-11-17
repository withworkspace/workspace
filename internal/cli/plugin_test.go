package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	for _, want := range []string{"fake 1.2.3", "view      Fake", "ui        /ui/app.js?v=1, /ui/app.css", "config    greeting (string)", "guide     1 lines, for agents", "note   ws fake has no subcommands"} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("check lacks %q:\n%s", want, res.Stdout)
		}
	}
	// It ran apart from the workspace: nothing's installed here.
	if got := plugin.Installed(w.Root); len(got) != 0 {
		t.Errorf("check installed %v", got)
	}

	if strings.Contains(res.Stdout, "wrong") {
		t.Errorf("something's wrong with one whose app is there:\n%s", res.Stdout)
	}

	// One with a view and no app, from before a plugin's page was an app:
	// there's nothing to show at its view.
	res = testutil.RunCommand(t, w.Root, nil, "plugin", "check", fakeplugin.Install(t, "old", fakeplugin.Options{NoUI: true}))
	if res.Err == nil || !strings.Contains(res.Stdout, "wrong  it has a view and no ui.script") || strings.Contains(res.Stdout, "  ui  ") {
		t.Errorf("a plugin with a view and no app: %v\n%s", res.Err, res.Stdout)
	}

	// One that serves a page of its own: ws serves the page now.
	res = testutil.RunCommand(t, w.Root, nil, "plugin", "check", fakeplugin.Install(t, "paged", fakeplugin.Options{HTML: true}))
	if res.Err == nil || !strings.Contains(res.Stdout, "wrong  it serves HTML at /: ws serves a plugin's page now, and the plugin serves its app's bundle") {
		t.Errorf("a plugin that serves HTML: %v\n%s", res.Err, res.Stdout)
	}

	// One whose app's files aren't what it says: not there, or not served
	// as what a browser would load.
	res = testutil.RunCommand(t, w.Root, nil, "plugin", "check", fakeplugin.Install(t, "broken", fakeplugin.Options{BrokenUI: true}))
	for _, want := range []string{"wrong  ui.script (/ui/missing.js) isn't there: the plugin answered 404", "wrong  ui.style (/ui/app.txt) is served as text/plain, not as CSS"} {
		if res.Err == nil || !strings.Contains(res.Stdout, want) {
			t.Errorf("a plugin whose app's files are wrong lacks %q: %v\n%s", want, res.Err, res.Stdout)
		}
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

// The fixture plugin (testdata/plugins/frame-fixture) is what ws's page for
// a plugin's app is tried with by hand: it has to be a plugin ws takes,
// with the app that's committed there served as a browser would load it.
func TestFrameFixtureChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the fixture")
	}
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{})
	_, here, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(here), "..", "..", "testdata", "plugins", "frame-fixture")
	// Built apart from the source, which a test doesn't write into.
	dir := filepath.Join(t.TempDir(), "frame-fixture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(dir, "frame-fixture"), ".")
	build.Dir = src
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the fixture: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte("name = \"frame-fixture\"\nversion = \"0.1.0\"\ncommand = [\"./frame-fixture\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := testutil.RunCommand(t, w.Root, nil, "plugin", "check", dir)
	if res.Err != nil {
		t.Fatalf("the fixture: %v\n%s", res.Err, res.Stdout)
	}
	for _, want := range []string{"view      Frame fixture", "view      Fixture: thing 2", "command   ws frame-fixture poke", "Nothing's wrong with it."} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("check lacks %q:\n%s", want, res.Stdout)
		}
	}
	// Its app's files are named with a hash of what's in each.
	if !regexp.MustCompile(`ui        /ui/app\.js\?v=[0-9a-f]{8}, /ui/app\.css\?v=[0-9a-f]{8}\n`).MatchString(res.Stdout) {
		t.Errorf("its ui line:\n%s", res.Stdout)
	}
}
