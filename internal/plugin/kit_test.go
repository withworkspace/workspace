package plugin_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/plugin/kitplugin"
)

// These start a plugin written with the framework, so each is ws and the
// framework speaking to each other: where they stop agreeing, one fails.

func addKit(t *testing.T, root string) {
	t.Helper()
	if _, err := plugin.Add(root, kitplugin.Install(t, "kit"), false); err != nil {
		t.Fatal(err)
	}
}

func TestKitRegistersWhatItDeclared(t *testing.T) {
	root, h := workspace(t)
	addKit(t, root)
	st, err := h.Refresh("kit")
	if err != nil {
		t.Fatal(err)
	}
	r := st.Registration
	if st.State != plugin.Ready || r.Description != "A plugin written with the framework, for tests" {
		t.Errorf("started: %+v", st)
	}
	if len(r.Views) != 2 || r.Views[0].Title != "Kit" || r.Views[0].Path != "/" || r.Views[1].Path != "/board" || r.Views[1].Icon != "kanban" {
		t.Errorf("views = %+v", r.Views)
	}
	if r.UI == nil || !strings.HasPrefix(r.UI.Script, "/ui/app.js?v=") || !plugin.ValidUIPath(r.UI.Script) {
		t.Errorf("ui = %+v", r.UI)
	}
	// Its own three, and the framework's sync, for a plugin with syncs.
	if r.CommandName("kit") != "kit" || len(r.Commands.Subcommands) != 4 {
		t.Errorf("commands = %+v", r.Commands)
	}
	if r.Agent == nil || !strings.Contains(r.Agent.Guide, "ws kit hi") || len(r.Agent.Roles) != 1 || r.Agent.Roles[0] != "crew" {
		t.Errorf("agent = %+v", r.Agent)
	}
	keys := map[string]plugin.ConfigKey{}
	for _, k := range r.Config {
		keys[k.Key] = k
	}
	if keys["greeting"].Default != "Hello" || !keys["token"].Secret() {
		t.Errorf("config = %+v", r.Config)
	}
	if len(r.Syncs) != 1 || r.Syncs[0].Name != "things" || r.Syncs[0].Every != 3600 {
		t.Errorf("syncs = %+v", r.Syncs)
	}
}

func TestKitCommandsAndConfig(t *testing.T) {
	root, h := workspace(t)
	addKit(t, root)
	run := func(args ...string) (int, *output) {
		t.Helper()
		o := &output{}
		exit, err := h.Run(context.Background(), "kit", args, root, plugin.Terminal{Width: 80}, o.write)
		if err != nil {
			t.Fatal(err)
		}
		return exit, o
	}
	if exit, o := run("hi"); exit != 0 || o.stdout() != "Hello, ws\n" || !strings.Contains(o.errOut.String(), "to stderr") {
		t.Errorf("hi: exit %d, %q, %q", exit, o.stdout(), o.errOut.String())
	}

	st, _ := h.Refresh("kit")
	for _, k := range st.Registration.Config {
		if k.Key == "greeting" {
			if err := plugin.SetValue(root, "kit", k, "Howdy"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := h.ConfigChanged("kit"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, o := run("hi"); o.stdout() == "Howdy, ws\n" {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("after config/changed: %q", o.stdout())
		}
		time.Sleep(20 * time.Millisecond)
	}

	if exit, _ := run("fail"); exit != 3 {
		t.Errorf("fail: exit %d", exit)
	}
	if exit, _ := run("nothing-like-it"); exit == 0 {
		t.Error("a command it doesn't have succeeded")
	}
}

func TestKitTellsWS(t *testing.T) {
	root, h := workspace(t)
	addKit(t, root)
	if _, err := h.Refresh("kit"); err != nil {
		t.Fatal(err)
	}
	// Its sync runs as it starts, says how it went, and says its view is
	// stale.
	deadline := time.Now().Add(5 * time.Second)
	for st := h.Status("kit"); st.Syncs["things"].At.IsZero() || st.Changed == 0; st = h.Status("kit") {
		if time.Now().After(deadline) {
			t.Fatalf("no sync/state heard: %+v", h.Status("kit").Syncs)
		}
		time.Sleep(20 * time.Millisecond)
	}
	before := h.Status("kit").Changed
	if exit, err := h.Run(context.Background(), "kit", []string{"stale"}, "", plugin.Terminal{}, func(string, string) {}); err != nil || exit != 0 {
		t.Fatalf("stale: %d, %v", exit, err)
	}
	if exit, err := h.Run(context.Background(), "kit", []string{"fail"}, "", plugin.Terminal{}, func(string, string) {}); err != nil || exit != 3 {
		t.Fatalf("fail: %d, %v", exit, err)
	}
	// Notifications sent before a command's answer are heard before it.
	st := h.Status("kit")
	if st.Changed != before+1 || st.Badges["/board"] != 3 {
		t.Errorf("changed %d, badges %v", st.Changed, st.Badges)
	}
	if st.Error != "it went wrong" {
		t.Errorf("last error = %q", st.Error)
	}
}

func TestKitServesItsApp(t *testing.T) {
	root, h := workspace(t)
	addKit(t, root)
	sock, err := h.ViewSocket("kit")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return net.Dial("unix", sock)
	}}}
	get := func(path string) (int, string, string) {
		t.Helper()
		res, err := client.Get("http://plugin" + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header.Get("Content-Type"), string(body)
	}
	script := h.Status("kit").Registration.UI.Script
	if code, ctype, body := get(script); code != 200 || !strings.HasPrefix(ctype, "text/javascript") || body != kitplugin.AppJS {
		t.Errorf("%s: %d %s %q", script, code, ctype, body)
	}
	if code, _, body := get("/api/greeting"); code != 200 || !strings.Contains(body, `"greeting":"Hello"`) {
		t.Errorf("/api/greeting: %d %q", code, body)
	}
}
