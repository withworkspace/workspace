package web

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/plugin/fakeplugin"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// startWithPlugin is a web UI for a workspace with the fake plugin
// installed, behaving as o says.
func startWithPlugin(t *testing.T, o fakeplugin.Options) *Server {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.Add(root, fakeplugin.Install(t, "fake", o), false); err != nil {
		t.Fatal(err)
	}
	host := &plugin.Host{Root: root, Version: "test"}
	t.Cleanup(host.Shutdown)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	storetest.Own(t, root)
	s := &Server{Root: root, Poll: 20 * time.Millisecond, Assets: testApp, listener: ln, Plugins: host}
	s.App = testOps(s, ops.Options{})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestPluginView(t *testing.T) {
	s := startWithPlugin(t, fakeplugin.Options{})

	// Its page, asked of the plugin, which is started for it.
	res := do(t, s, "GET", "/plugins/fake/", true, nil)
	page := body(res)
	if res.StatusCode != http.StatusOK || !strings.Contains(page, "from the fake") {
		t.Fatalf("its page: %d %q", res.StatusCode, page)
	}
	// The plugin's told where it's served, and not who's signed in to ws.
	if !strings.Contains(page, "prefix=/plugins/fake") || !strings.Contains(page, `cookie=""`) {
		t.Errorf("what the plugin was sent: %q", page)
	}
	// It can be framed by ws, under ws's policy for a plugin's page, not
	// the plugin's own or the app's.
	if got := res.Header.Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options = %q", got)
	}
	if got := res.Header.Get("Referrer-Policy"); got != "same-origin" {
		t.Errorf("Referrer-Policy = %q", got)
	}
	if got := res.Header.Values("Content-Security-Policy"); len(got) != 1 || got[0] != PluginPolicy {
		t.Errorf("Content-Security-Policy = %q", got)
	}

	// Signed out, there's nothing of it.
	if res := do(t, s, "GET", "/plugins/fake/", false, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("signed out: %d", res.StatusCode)
	}

	// /plugins/fake is its first page.
	if res := do(t, s, "GET", "/plugins/fake", true, nil); res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/plugins/fake/" {
		t.Errorf("without the slash: %d to %q", res.StatusCode, res.Header.Get("Location"))
	}

	// A redirect to a path of the plugin's own stays under its base.
	if res := do(t, s, "GET", "/plugins/fake/go", true, nil); res.Header.Get("Location") != "/plugins/fake/there" {
		t.Errorf("a redirect: %d to %q", res.StatusCode, res.Header.Get("Location"))
	}

	// Its own forms post as forms, from this origin alone.
	form := func(origin string) *http.Response {
		return do(t, s, "POST", "/plugins/fake/form", true, func(r *http.Request) {
			r.Body = http.NoBody
			r.URL.RawQuery = "q=words"
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", origin)
		})
	}
	if res := form(s.Info().Origin()); res.StatusCode != http.StatusOK || body(res) != "posted words" {
		t.Errorf("a form: %d", res.StatusCode)
	}
	if res := form("http://elsewhere.example"); res.StatusCode != http.StatusForbidden {
		t.Errorf("a form from another site: %d", res.StatusCode)
	}
	// The app's own API still wants JSON.
	if res := do(t, s, "POST", "/api/shutdown", true, func(r *http.Request) {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", s.Info().Origin())
	}); res.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("a form to the API: %d", res.StatusCode)
	}

	// A plugin that isn't there.
	if res := do(t, s, "GET", "/plugins/nobody/", true, nil); res.StatusCode != http.StatusBadGateway || !strings.Contains(body(res), "didn't start") {
		t.Errorf("no such plugin: %d", res.StatusCode)
	}
}

func TestPluginsList(t *testing.T) {
	s := startWithPlugin(t, fakeplugin.Options{})
	list := func() []Plugin {
		t.Helper()
		var out []Plugin
		res := do(t, s, "GET", "/api/plugins", true, nil)
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	// Installed and never started: it's there, with nothing known of it.
	if got := list(); len(got) != 1 || got[0].Name != "fake" || got[0].State != "stopped" || len(got[0].Views) != 0 {
		t.Fatalf("before it's started: %+v", got)
	}
	do(t, s, "GET", "/plugins/fake/", true, nil)
	got := list()
	if got[0].State != "ready" || got[0].Starts != 1 || len(got[0].Views) != 1 || got[0].Views[0].Title != "Fake" || got[0].Views[0].Path != "/" {
		t.Errorf("once it's started: %+v", got)
	}
}

func TestPluginNeedsSettingUp(t *testing.T) {
	s := startWithPlugin(t, fakeplugin.Options{Require: "token"})
	res := do(t, s, "GET", "/plugins/fake/", true, nil)
	page := body(res)
	if res.StatusCode != http.StatusServiceUnavailable || !strings.Contains(page, "needs setting up") || !strings.Contains(page, "ws plugin config fake --ask") {
		t.Errorf("before it's set up: %d %q", res.StatusCode, page)
	}
	if !strings.Contains(page, "/plugin-kit.css") {
		t.Errorf("the page isn't in ws's look: %q", page)
	}
}

func TestPluginKitFiles(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	for path, want := range map[string]string{"/plugin-kit.js": "ws:ready", "/plugin-kit.css": "--ws-accent"} {
		res := do(t, s, "GET", path, true, nil)
		if res.StatusCode != http.StatusOK || !strings.Contains(body(res), want) {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
	}
	// With no plugins, none are listed and the app's none the worse.
	res := do(t, s, "GET", "/api/plugins", true, nil)
	if got := strings.TrimSpace(body(res)); res.StatusCode != http.StatusOK || got != "[]" {
		t.Errorf("no plugins: %d %q", res.StatusCode, got)
	}
}
