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

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/plugin/fakeplugin"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
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

	// The plugin's own HTTP, asked of the plugin, which is started for it.
	res := do(t, s, "GET", "/plugins/fake/api/greeting", true, nil)
	said := body(res)
	if res.StatusCode != http.StatusOK || !strings.Contains(said, "from the fake") {
		t.Fatalf("what it answered: %d %q", res.StatusCode, said)
	}
	// The plugin's told where it's served, and not who's signed in to ws.
	if !strings.Contains(said, "prefix=/plugins/fake") || !strings.Contains(said, `cookie=""`) {
		t.Errorf("what the plugin was sent: %q", said)
	}
	// It's under ws's policy for what a plugin serves, not the plugin's own
	// or the app's.
	if got := res.Header.Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options = %q", got)
	}
	if got := res.Header.Get("Referrer-Policy"); got != "same-origin" {
		t.Errorf("Referrer-Policy = %q", got)
	}
	if got := res.Header.Values("Content-Security-Policy"); len(got) != 1 || got[0] != PluginPolicy {
		t.Errorf("Content-Security-Policy = %q", got)
	}
	// Its app's files are there, as what they are.
	if res := do(t, s, "GET", "/plugins/fake/ui/app.js?v=1", true, nil); res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/javascript") {
		t.Errorf("its app's script: %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}

	// Signed out, there's nothing of it.
	if res := do(t, s, "GET", "/plugins/fake/api/greeting", false, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("signed out: %d", res.StatusCode)
	}

	// /plugins/fake is its root.
	if res := do(t, s, "GET", "/plugins/fake", true, nil); res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/plugins/fake/" {
		t.Errorf("without the slash: %d to %q", res.StatusCode, res.Header.Get("Location"))
	}

	// A redirect to a path of the plugin's own stays under its base.
	if res := do(t, s, "GET", "/plugins/fake/go", true, nil); res.Header.Get("Location") != "/plugins/fake/there" {
		t.Errorf("a redirect: %d to %q", res.StatusCode, res.Header.Get("Location"))
	}

	// Its app posts to it as the plugin takes it, from this origin alone.
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
	if res := do(t, s, "GET", "/plugins/nobody/", true, nil); res.StatusCode != http.StatusNotFound || !strings.Contains(body(res), "no such plugin") {
		t.Errorf("no such plugin: %d", res.StatusCode)
	}
}

// The shell is the page ws serves for a plugin's view: ws's tokens, an
// empty #root and the plugin's app, at every path under its name.
func TestPluginShell(t *testing.T) {
	s := startWithPlugin(t, fakeplugin.Options{})

	var first string
	for _, path := range []string{"/plugin-frame/fake/", "/plugin-frame/fake/things/2?tab=links", "/plugin-frame/fake/a/path/the/plugin/never/heard/of"} {
		res := do(t, s, "GET", path, true, nil)
		page := body(res)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d %q", path, res.StatusCode, page)
		}
		for _, want := range []string{
			`<script src="/theme.js"></script>`,                 // light or dark, before it paints
			`<title>Fake</title>`,                               // its view's title
			`<style>:root{--bg:#fff;--ws-bg:var(--bg)}</style>`, // the tokens, written into the page
			`<link rel="stylesheet" href="/plugins/fake/ui/app.css">`,
			`<script type="module" src="/plugins/fake/ui/app.js?v=1"></script>`,
			`<body><div id="root"></div></body>`, // empty: the app's to fill
		} {
			if !strings.Contains(page, want) {
				t.Errorf("%s lacks %q:\n%s", path, want, page)
			}
		}
		// The same page wherever it's asked for: the app says what's there.
		if first == "" {
			first = page
		} else if page != first {
			t.Errorf("%s isn't the page / is:\n%s", path, page)
		}
		// It can be framed by ws, runs no inline script, and is never kept.
		if got := res.Header.Values("Content-Security-Policy"); len(got) != 1 || got[0] != ShellPolicy {
			t.Errorf("Content-Security-Policy = %q", got)
		}
		if strings.Contains(ShellPolicy, "script-src 'self' 'unsafe-inline'") {
			t.Error("the shell's policy runs inline script")
		}
		if got := res.Header.Get("X-Frame-Options"); got != "SAMEORIGIN" {
			t.Errorf("X-Frame-Options = %q", got)
		}
		if got := res.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q", got)
		}
		if got := res.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
	}
	// It was started to be asked what its app is.
	if st := s.Plugins.Status("fake"); st.State != plugin.Ready {
		t.Errorf("after its shell was served it's %s", st.State)
	}

	// Without the slash, it's the app's first path.
	if res := do(t, s, "GET", "/plugin-frame/fake", true, nil); res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/plugin-frame/fake/" {
		t.Errorf("without the slash: %d to %q", res.StatusCode, res.Header.Get("Location"))
	}
	// Signed out, there's nothing of it.
	if res := do(t, s, "GET", "/plugin-frame/fake/", false, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("signed out: %d", res.StatusCode)
	}
	// It's a page to read: nothing's posted to it.
	if res := do(t, s, "POST", "/plugin-frame/fake/", true, func(r *http.Request) {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	}); res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("a post to the shell: %d", res.StatusCode)
	}

	// A plugin that isn't there: the shell, with ws's message and no app.
	res := do(t, s, "GET", "/plugin-frame/nobody/", true, nil)
	page := body(res)
	if res.StatusCode != http.StatusNotFound || !strings.Contains(page, "There&#39;s no such plugin") || strings.Contains(page, `type="module"`) {
		t.Errorf("no such plugin: %d %q", res.StatusCode, page)
	}
	if !strings.Contains(page, "<style>:root{--bg:#fff;--ws-bg:var(--bg)}</style>") || res.Header.Get("Content-Security-Policy") != ShellPolicy {
		t.Errorf("the message isn't in ws's look: %q", page)
	}
}

// The accent is on the shell's <html> as it is on the web UI's, so the
// tokens worked out from it are right before anything runs.
func TestPluginShellAccent(t *testing.T) {
	s := startWithPlugin(t, fakeplugin.Options{})
	if page := body(do(t, s, "GET", "/plugin-frame/fake/", true, nil)); !strings.Contains(page, `<html lang="en">`) {
		t.Errorf("with no accent set: %q", page)
	}
	setOps(s, testOps(s, ops.Options{}, func(c *config.Config, _ *workspace.Workspace) { c.UI.Accent = "#2A9D8F" }))
	if page := body(do(t, s, "GET", "/plugin-frame/fake/", true, nil)); !strings.Contains(page, `<html lang="en" style="--accent-pick: #2a9d8f">`) {
		t.Errorf("with an accent set: %q", page)
	}
	// The message has it too.
	if page := body(do(t, s, "GET", "/plugin-frame/nobody/", true, nil)); !strings.Contains(page, `<html lang="en" style="--accent-pick: #2a9d8f">`) {
		t.Errorf("a message, with an accent set: %q", page)
	}
}

// A plugin built before a plugin's page was an app has a view and no app:
// its shell says so.
func TestPluginShellWithNoApp(t *testing.T) {
	s := startWithPlugin(t, fakeplugin.Options{NoUI: true})
	res := do(t, s, "GET", "/plugin-frame/fake/", true, nil)
	page := body(res)
	if res.StatusCode != http.StatusBadGateway || !strings.Contains(page, "fake has no app to show") || !strings.Contains(page, "update it") || strings.Contains(page, `type="module"`) {
		t.Errorf("a plugin with no app: %d %q", res.StatusCode, page)
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
	do(t, s, "GET", "/plugin-frame/fake/", true, nil)
	got := list()
	if got[0].State != "ready" || got[0].Starts != 1 || len(got[0].Views) != 1 || got[0].Views[0].Title != "Fake" || got[0].Views[0].Path != "/" {
		t.Errorf("once it's started: %+v", got)
	}
}

func TestPluginNeedsSettingUp(t *testing.T) {
	s := startWithPlugin(t, fakeplugin.Options{Require: "token"})
	// Its shell says so, in ws's look, and so does its own HTTP.
	for _, path := range []string{"/plugin-frame/fake/", "/plugins/fake/api/greeting"} {
		res := do(t, s, "GET", path, true, nil)
		page := body(res)
		if res.StatusCode != http.StatusServiceUnavailable || !strings.Contains(page, "needs setting up") || !strings.Contains(page, "ws plugin config fake --ask") {
			t.Errorf("%s before it's set up: %d %q", path, res.StatusCode, page)
		}
		if !strings.Contains(page, "--ws-bg") || strings.Contains(page, `type="module"`) {
			t.Errorf("%s isn't ws's message in ws's look: %q", path, page)
		}
	}
}

// What a plugin's page used to be given by ws is gone, and with no plugins
// the app's none the worse.
func TestNoPlugins(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	res := do(t, s, "GET", "/api/plugins", true, nil)
	if got := strings.TrimSpace(body(res)); res.StatusCode != http.StatusOK || got != "[]" {
		t.Errorf("no plugins: %d %q", res.StatusCode, got)
	}
	if res := do(t, s, "GET", "/plugin-frame/fake/", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a shell with no plugins: %d", res.StatusCode)
	}
}
