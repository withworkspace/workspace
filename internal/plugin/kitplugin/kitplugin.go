// Package kitplugin is a plugin for tests written with the framework
// plugins are written with (github.com/withworkspace/plugin), run from the
// test's own binary. Where fakeplugin speaks the protocol by hand, to try
// what a plugin might send, this is the framework's own side of it: a test
// with it fails when ws and the framework stop agreeing. A test package's
// TestMain calls RunIfAsked first, and Install puts it in a workspace.
package kitplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/withworkspace/plugin"
)

// The environment that makes a test's binary the plugin.
const envRun = "WS_KIT_PLUGIN"

// RunIfAsked runs the plugin and exits, if this process was started as
// one. Otherwise it returns, and the tests run.
func RunIfAsked() {
	if os.Getenv(envRun) == "" {
		return
	}
	run()
	os.Exit(0)
}

// AppJS is the app's entry, as the plugin serves it.
const AppJS = `console.log("kit")`

func run() {
	p := plugin.New(fstest.MapFS{"ui/dist/app.js": {Data: []byte(AppJS)}})
	p.Describe("A plugin written with the framework, for tests")
	greeting := p.Config.String("greeting", "Greeting", plugin.Default("Hello"))
	p.Config.Secret("token", "Token")

	p.UI("ui/dist")
	p.View("Kit", plugin.Icon("box"))
	p.View("Board", plugin.Icon("kanban"), plugin.At("/board"))
	p.HandleFunc("GET /api/greeting", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"greeting": greeting.Get()}) //nolint:errcheck // to a test
	})
	p.Guide("Use `ws kit hi` to be greeted.", "crew")
	p.Sync("things", time.Hour, func(ctx context.Context) error { return nil })

	p.Command("hi", "Says the greeting", func(c *plugin.Call) error {
		fmt.Fprintln(c.Stderr(), "to stderr")
		return c.Say(greeting.Get() + ", ws")
	})
	p.Command("stale", "Says its view is stale, with a badge", func(c *plugin.Call) error {
		p.Changed()
		p.Badge("/board", 3)
		return c.Say("told")
	})
	p.Command("looking", "Says who ws has said is looking", func(c *plugin.Call) error {
		l := p.Looking()
		return c.Say(fmt.Sprintf("app %t, ws %t", l.App, l.WS))
	})
	p.Command("fail", "Logs an error and exits 3", func(c *plugin.Call) error {
		p.Log.Error("it went wrong")
		return plugin.Exit(3, errors.New("failed"))
	})
	p.Command("ask", "Asks ws for missions, a mission or repos, and prints the answer as JSON", func(c *plugin.Call) error {
		var answer any
		var err error
		switch c.Arg(0) {
		case "missions":
			answer, err = p.Missions(c.Context(), c.Flag("archived"))
		case "mission":
			answer, err = p.Mission(c.Context(), c.Arg(1))
		case "repos":
			answer, err = p.Repos(c.Context())
		}
		switch {
		case errors.Is(err, plugin.ErrNotFound):
			return c.Say("not found: " + err.Error())
		case errors.Is(err, plugin.ErrUnsupported):
			return c.Say("unsupported: " + err.Error())
		case err != nil:
			return err
		}
		return json.NewEncoder(c.Stdout()).Encode(answer)
	}).Args("missions|mission SLUG|repos").Flag("archived", "archived missions too")
	p.Run()
}

// Install puts the plugin, called name, in a folder, and returns the
// folder, for plugin.Add.
func Install(t *testing.T, name string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf("name = %q\nversion = \"0.1.0\"\ncommand = [%q]\n", name, self)
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envRun, "1")
	return dir
}
