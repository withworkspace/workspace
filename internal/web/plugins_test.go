package web

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/plugin/fakeplugin"
	"github.com/withworkspace/workspace/internal/plugin/kitplugin"
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
	// Asked for as a page of its own, not in ws's frame, it's ws's page of
	// that path: the bare shell is no use in a tab.
	for path, want := range map[string]string{
		"/plugin-frame/fake/things/2?tab=links": "/x/fake/things/2?tab=links",
		"/plugin-frame/fake/":                   "/x/fake/",
		"/plugin-frame/fake":                    "/x/fake/",
	} {
		res := do(t, s, "GET", path, true, func(r *http.Request) { r.Header.Set("Sec-Fetch-Dest", "document") })
		if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != want {
			t.Errorf("%s as a page of its own: %d to %q, want %q", path, res.StatusCode, res.Header.Get("Location"), want)
		}
	}
	if res := do(t, s, "GET", "/plugin-frame/fake/things/2", true, func(r *http.Request) { r.Header.Set("Sec-Fetch-Dest", "iframe") }); res.StatusCode != http.StatusOK {
		t.Errorf("in a frame: %d", res.StatusCode)
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

// A page following the plugins is a page that's showing, and says which
// plugin's app it shows: the plugin is told who's looking, and that
// they've gone a moment after they have.
func TestPluginIsToldWhosLooking(t *testing.T) {
	s := startWithPlugin(t, fakeplugin.Options{})
	s.Plugins.LookLinger = 50 * time.Millisecond
	do(t, s, "GET", "/plugin-frame/fake/", true, nil) // it's started
	follow := func(query string) (stop func()) {
		t.Helper()
		req, _ := http.NewRequest("GET", s.Info().Origin()+"/api/events?topic=plugins"+query, nil)
		req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.Info().Token})
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("following the plugins: %v, %v", res, err)
		}
		return func() { res.Body.Close() }
	}
	told := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			got := body(do(t, s, "GET", "/plugins/fake/api/looking", true, nil))
			if got == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("the plugin was told %q, not %q", got, want)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	told("") // nobody's looking, as a plugin takes it until it's told

	onApp := follow("&app=fake")
	told(`{"app":true,"ws":true}`)
	// Another page, on something of ws's own; and one that names a plugin
	// there isn't, which is a page of ws's and no more.
	elsewhere := follow("")
	onApp()
	told(`{"app":false,"ws":true}`)
	elsewhere()
	nowhere := follow("&app=nothing-like-it")
	time.Sleep(s.Plugins.LookLinger + 50*time.Millisecond)
	told(`{"app":false,"ws":true}`)
	nowhere()
	told(`{"app":false,"ws":false}`)
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

// How a plugin's things are named is served from what ws keeps of it,
// running or not, and what one is comes from the plugin when it's asked.
func TestPluginRefsAndAThing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.Add(root, kitplugin.Install(t, "kit"), false); err != nil {
		t.Fatal(err)
	}
	storetest.Own(t, root)
	host := &plugin.Host{Root: root, Version: "test", Keep: func(name string, refs []protocol.Ref) ([]string, error) { return plugin.KeepRefs(root, name, refs) }}
	// What the plugin tells ws of its things is kept, as the daemon keeps it.
	host.Answer = func(_ context.Context, who, method string, params json.RawMessage) (any, error) {
		if answer, ok, err := plugin.AnswerThings(root, who, method, params); ok {
			return answer, err
		}
		return nil, plugin.NoMethod(method)
	}
	t.Cleanup(host.Shutdown)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Root: root, Poll: 20 * time.Millisecond, Assets: testApp, listener: ln, Plugins: host}
	s.App = testOps(s, ops.Options{})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	list := func() Plugin {
		t.Helper()
		var out []Plugin
		if err := json.NewDecoder(do(t, s, "GET", "/api/plugins", true, nil).Body).Decode(&out); err != nil || len(out) != 1 {
			t.Fatalf("plugins: %+v, %v", out, err)
		}
		return out[0]
	}
	if got := list(); len(got.Refs) != 0 {
		t.Errorf("before it's ever started: %+v", got.Refs)
	}

	// Asking about a thing starts the plugin, which registers its refs.
	res := do(t, s, "GET", "/api/plugins/kit/refs/thing/KIT-1", true, nil)
	var thing PluginThing
	if err := json.NewDecoder(res.Body).Decode(&thing); err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("KIT-1: %d, %v", res.StatusCode, err)
	}
	// The plugin's sync told ws of KIT-1 as it started (or the plugin was
	// asked, if this got there first): either way, what its kind has a
	// field for, and nothing else.
	if f := thing.Fields; thing.Key != "KIT-1" || f["title"] != "The first thing" || f["status"] != "Open" || f["owner"] != "Dana" || !strings.HasPrefix(f["body"], "It says hello") || f["colour"] != "" || len(f) != 5 || thing.Changed == nil {
		t.Errorf("KIT-1: %+v", thing)
	}
	want := PluginRef{Kind: "thing", Title: "Kit thing", Icon: "box", Prefixes: []string{"KIT-"}, Address: "https://kit.example/things/{ref}", Path: "/thing/{ref}"}
	if got := list(); len(got.Refs) != 1 || got.Refs[0].Kind != want.Kind || got.Refs[0].Title != want.Title || got.Refs[0].Icon != want.Icon ||
		strings.Join(got.Refs[0].Prefixes, " ") != "KIT-" || got.Refs[0].Address != want.Address || got.Refs[0].Path != want.Path {
		t.Errorf("refs once it's started: %+v", got.Refs)
	}
	// What its things are made of, and how they're drawn, for the web UI
	// to draw them by.
	if r := list().Refs[0]; len(r.Fields) != 6 || r.Fields[0].Type != protocol.FieldTitle || len(r.Views[protocol.ViewCard]) != 3 || r.Views[protocol.ViewPane][4].Block != protocol.BlockMarkdown {
		t.Errorf("fields %+v, views %+v", r.Fields, r.Views)
	}
	// Told of, it's answered from ws's own records, the plugin stopped or
	// not; one the plugin has and didn't tell ws of is asked for.
	waitTold := time.Now().Add(5 * time.Second)
	for {
		var told PluginThing
		json.NewDecoder(do(t, s, "GET", "/api/plugins/kit/refs/thing/KIT-1", true, nil).Body).Decode(&told) //nolint:errcheck // told says
		if told.Told {
			break
		}
		if time.Now().After(waitTold) {
			t.Fatal("ws wasn't told of KIT-1 in five seconds")
		}
		time.Sleep(20 * time.Millisecond)
	}
	var off PluginThing
	if res := do(t, s, "GET", "/api/plugins/kit/refs/thing/KIT-3", true, nil); json.NewDecoder(res.Body).Decode(&off) != nil || res.StatusCode != http.StatusOK || off.Told || off.Fields["title"] != "The third thing" || off.Fields["status"] != "Off the board" {
		t.Errorf("KIT-3, which ws wasn't told of: %d %+v", res.StatusCode, off)
	}
	// Stopped, its names are still served: they're ws's to find. And what
	// it told ws of is still drawn, with nobody started to ask.
	host.Stop("kit")
	if got := list(); got.State != "stopped" || len(got.Refs) != 1 {
		t.Errorf("stopped: %s, %+v", got.State, got.Refs)
	}
	var kept PluginThing
	if res := do(t, s, "GET", "/api/plugins/kit/refs/thing/KIT-1", true, nil); json.NewDecoder(res.Body).Decode(&kept) != nil || !kept.Told || kept.Fields["title"] != "The first thing" {
		t.Errorf("KIT-1 with the plugin stopped: %d %+v", res.StatusCode, kept)
	}
	if got := list(); got.State != "stopped" {
		t.Errorf("drawing what ws was told started the plugin: %s", got.State)
	}

	// None by that name is 404, and says so; a plugin that isn't there
	// to ask isn't.
	for path, status := range map[string]int{
		"/api/plugins/kit/refs/thing/KIT-2":    http.StatusNotFound,
		"/api/plugins/kit/refs/thing/OTHER-1":  http.StatusNotFound,
		"/api/plugins/kit/refs/sprint/KIT-1":   http.StatusNotFound,
		"/api/plugins/nobody/refs/thing/KIT-1": http.StatusBadGateway,
	} {
		res := do(t, s, "GET", path, true, nil)
		if said := body(res); res.StatusCode != status || !strings.Contains(said, `"error"`) {
			t.Errorf("%s: %d %s, want %d", path, res.StatusCode, said, status)
		}
	}
	if res := do(t, s, "GET", "/api/plugins/kit/refs/thing/KIT-1", false, nil); res.StatusCode == http.StatusOK {
		t.Error("a thing was shown to someone not signed in")
	}

	// What it can do is served for Find: its own, and what's done to one
	// of its things. Run, one asks first, and is run again with the answer.
	host.Refresh("kit") //nolint:errcheck // it was stopped above
	if got := list(); len(got.Actions) != 1 || got.Actions[0].ID != "ping" || got.Actions[0].Hint != "It answers" || len(got.Refs[0].Actions) != 1 || got.Refs[0].Actions[0].Title != "Move to…" {
		t.Errorf("actions: %+v, a thing's %+v", got.Actions, got.Refs[0].Actions)
	}
	post := func(path, body string) (*http.Response, PluginActed) {
		t.Helper()
		res := do(t, s, "POST", path, true, func(r *http.Request) {
			r.Body, r.ContentLength = io.NopCloser(strings.NewReader(body)), int64(len(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", s.Info().Origin())
		})
		var acted PluginActed
		json.NewDecoder(res.Body).Decode(&acted) //nolint:errcheck // the status says
		return res, acted
	}
	if res, acted := post("/api/plugins/kit/actions/move", `{"kind":"thing","ref":"KIT-1","answers":[]}`); res.StatusCode != http.StatusOK || acted.Ask == nil || len(acted.Ask.Options) != 2 {
		t.Errorf("move: %d %+v", res.StatusCode, acted)
	}
	// Done, it says so, and where it opens is its page in ws.
	if res, acted := post("/api/plugins/kit/actions/move", `{"kind":"thing","ref":"KIT-1","answers":["Closed"]}`); res.StatusCode != http.StatusOK || acted.Say != "KIT-1 is Closed" || acted.Open != "/x/kit/thing/KIT-1" {
		t.Errorf("move, answered: %d %+v", res.StatusCode, acted)
	}
	if res, acted := post("/api/plugins/kit/actions/ping", `{"answers":["ws"]}`); res.StatusCode != http.StatusOK || acted.Say != "pong, ws" {
		t.Errorf("ping: %d %+v", res.StatusCode, acted)
	}
	for path, status := range map[string]int{
		"/api/plugins/kit/actions/delete":  http.StatusNotFound,
		"/api/plugins/nobody/actions/ping": http.StatusNotFound,
		"/api/plugins/kit/actions/move":    http.StatusNotFound, // a thing's, with no thing
	} {
		if res, _ := post(path, `{"answers":[]}`); res.StatusCode != status {
			t.Errorf("%s: %d, want %d", path, res.StatusCode, status)
		}
	}
	if res, _ := post("/api/plugins/kit/actions/move", `{"kind":"thing","ref":"KIT-2","answers":["Closed"]}`); res.StatusCode != http.StatusBadGateway {
		t.Errorf("one that fails: %d", res.StatusCode)
	}

	// ws's own, on any thing: a mission about it, made to brief, asked
	// for as its name and what it's called.
	res = do(t, s, "POST", "/api/missions", true, func(r *http.Request) {
		body := `{"about":"KIT-1"}`
		r.Body, r.ContentLength = io.NopCloser(strings.NewReader(body)), int64(len(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	})
	var started Started
	if err := json.NewDecoder(res.Body).Decode(&started); err != nil || res.StatusCode != http.StatusOK || started.Slug == "" || started.Path != "/missions/"+started.Slug {
		t.Fatalf("a mission about KIT-1: %d %+v, %v", res.StatusCode, started, err)
	}
	var made MissionDetail
	if err := json.NewDecoder(do(t, s, "GET", "/api/missions/"+started.Slug, true, nil).Body).Decode(&made); err != nil || made.Title != "KIT-1: The first thing" || strings.Join(made.About, " ") != "KIT-1" {
		t.Errorf("the mission it made: %+v, %v", made, err)
	}
	res = do(t, s, "POST", "/api/missions", true, func(r *http.Request) {
		body := `{"about":"UTF-8"}`
		r.Body, r.ContentLength = io.NopCloser(strings.NewReader(body)), int64(len(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	})
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a mission about a word: %d", res.StatusCode)
	}
}
