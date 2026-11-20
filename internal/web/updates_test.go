package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/plugin/fakeplugin"
	"github.com/withworkspace/workspace/internal/release"
	"github.com/withworkspace/workspace/internal/release/releasetest"
)

// asks has s do something, as a page does: signed in, from its own
// origin, with v as JSON. What it answered is read into out, if given.
func asks(t *testing.T, s *Server, method, path string, v, out any) *http.Response {
	t.Helper()
	data, _ := json.Marshal(v)
	res := do(t, s, method, path, true, func(r *http.Request) {
		r.Body = io.NopCloser(bytes.NewReader(data))
		r.ContentLength = int64(len(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	})
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: what it answered (%d) isn't what was expected: %v", method, path, res.StatusCode, err)
		}
	}
	return res
}

// Settings' About says which ws this is and what the last look found; a
// look asked for finds the latest release; and Upgrade installs it in
// place of the ws the daemon runs.
func TestUpdates(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	exe := filepath.Join(t.TempDir(), "workspace")
	if err := os.WriteFile(exe, releasetest.Script("v1.3.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	setOps(s, testOps(s, ops.Options{WS: release.Upgrader{Exe: exe, Version: "v1.3.0", Release: true, Source: release.Source{Base: r.URL}, OS: "linux", Arch: "amd64"}}))

	var up UpdatesInfo
	if res := asks(t, s, "GET", "/api/updates", nil, &up); res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/updates: %d", res.StatusCode)
	}
	if up.Version != "v1.3.0" || !up.Release || !up.Checks || up.Checked != nil || up.Latest != "" || up.Newer {
		t.Errorf("before any look: %+v", up)
	}
	var ws Workspace
	asks(t, s, "GET", "/api/workspace", nil, &ws)
	if ws.Later != "" {
		t.Errorf("the workspace says %q is out before any look", ws.Later)
	}

	if res := asks(t, s, "POST", "/api/updates/check", struct{}{}, &up); res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/updates/check: %d", res.StatusCode)
	}
	if up.Latest != "v1.4.0" || !up.Newer || up.Checked == nil || up.Error != "" || up.Page != r.URL+"/releases/tag/v1.4.0" {
		t.Errorf("after a look: %+v", up)
	}
	asks(t, s, "GET", "/api/workspace", nil, &ws)
	if ws.Later != "v1.4.0" {
		t.Errorf("the workspace says %q is out, want v1.4.0", ws.Later)
	}

	var did Upgraded
	if res := asks(t, s, "POST", "/api/upgrade", struct{}{}, &did); res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/upgrade: %d", res.StatusCode)
	}
	if !did.Installed || !did.Kept || did.From != "v1.3.0" || did.To != "v1.4.0" {
		t.Errorf("the upgrade: %+v", did)
	}
	if v, err := release.VersionOf(exe); err != nil || v != "v1.4.0" {
		t.Errorf("the ws in place is %s (%v), want v1.4.0", v, err)
	}

	// A ws that's no release's build is refused, with why.
	setOps(s, testOps(s, ops.Options{WS: release.Upgrader{Exe: exe, Version: "v1.3.0", Source: release.Source{Base: r.URL}, OS: "linux", Arch: "amd64"}}))
	var refused struct{ Error, Code string }
	if res := asks(t, s, "POST", "/api/upgrade", struct{}{}, &refused); res.StatusCode != http.StatusConflict || refused.Code != "not-a-release" {
		t.Errorf("upgrading a ws that's no release's build: %d %+v", res.StatusCode, refused)
	}
	// A write needs the page's own origin, as every write does.
	if res := do(t, s, "POST", "/api/upgrade", true, nil); res.StatusCode != http.StatusForbidden {
		t.Errorf("an upgrade asked for from elsewhere: %d", res.StatusCode)
	}
}

// hostRunner is a host's plugins, as ops starts and stops them.
type hostRunner struct{ h *plugin.Host }

func (r hostRunner) Statuses(context.Context) ([]plugin.Status, error) { return r.h.Statuses(), nil }
func (r hostRunner) Refresh(_ context.Context, name string) (plugin.Status, error) {
	return r.h.Refresh(name)
}
func (r hostRunner) Stop(_ context.Context, name string) error { r.h.Stop(name); return nil }
func (hostRunner) Close()                                      {}

// The Plugins page lists what's installed with what each adds, adds a
// plugin from a folder and starts it, and removes one.
func TestPluginsPage(t *testing.T) {
	s := startWithPlugin(t, fakeplugin.Options{})
	was := ops.PluginRunners
	ops.PluginRunners = func(string, bool) (ops.PluginRunner, error) { return hostRunner{s.Plugins}, nil }
	t.Cleanup(func() { ops.PluginRunners = was })

	var list PluginsInstalled
	if res := asks(t, s, "GET", "/api/plugins/installed", nil, &list); res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/plugins/installed: %d", res.StatusCode)
	}
	if len(list.Plugins) != 1 || list.Plugins[0].Name != "fake" || list.Plugins[0].From != (PluginFrom{}) || len(list.Wanted) != 0 || !list.Checks {
		t.Fatalf("what's installed: %+v", list)
	}

	// Another, from its folder: it's installed and started, and says what
	// it adds.
	folder := fakeplugin.Install(t, "other", fakeplugin.Options{})
	var added PluginChanged
	if res := asks(t, s, "POST", "/api/plugins", PluginAddRequest{From: folder}, &added); res.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/plugins: %d", res.StatusCode)
	}
	if added.Name != "other" || added.State != plugin.Ready {
		t.Errorf("what was added: %+v", added)
	}
	asks(t, s, "GET", "/api/plugins/installed", nil, &list)
	if len(list.Plugins) != 2 {
		t.Fatalf("after adding one: %+v", list.Plugins)
	}
	other := list.Plugins[1]
	if other.Name != "other" || other.State != plugin.Ready || len(other.Views) == 0 || other.Path != "/x/other" {
		t.Errorf("the one added, as it's listed: %+v", other)
	}

	// What isn't a plugin, or a place one's published, is refused in words.
	var refused struct{ Error, Code string }
	if res := asks(t, s, "POST", "/api/plugins", PluginAddRequest{From: "not a repo"}, &refused); res.StatusCode != http.StatusUnprocessableEntity || refused.Error == "" {
		t.Errorf("adding what isn't a plugin: %d %+v", res.StatusCode, refused)
	}
	if res := asks(t, s, "POST", "/api/plugins", PluginAddRequest{From: t.TempDir()}, &refused); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("adding a folder that's no plugin: %d %+v", res.StatusCode, refused)
	}
	// One from a folder has nowhere to be updated from.
	if res := asks(t, s, "POST", "/api/plugins/other/update", struct{}{}, &refused); res.StatusCode != http.StatusConflict || refused.Code != "folder" {
		t.Errorf("updating one added from a folder: %d %+v", res.StatusCode, refused)
	}

	if res := asks(t, s, "POST", "/api/plugins/other/remove", PluginRemoveRequest{Data: true}, nil); res.StatusCode != http.StatusOK {
		t.Fatalf("removing it: %d", res.StatusCode)
	}
	if plugin.IsInstalled(s.Root, "other") {
		t.Error("it's still installed after it was removed")
	}
	if _, err := os.Stat(filepath.Join(plugin.Dir(s.Root), "other")); !os.IsNotExist(err) {
		t.Errorf("what it kept is still there after it was removed with its data: %v", err)
	}
	if res := asks(t, s, "POST", "/api/plugins/other/remove", PluginRemoveRequest{}, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("removing one that isn't here: %d", res.StatusCode)
	}
	asks(t, s, "GET", "/api/plugins/installed", nil, &list)
	if len(list.Plugins) != 1 || list.Plugins[0].Name != "fake" {
		t.Errorf("after removing it: %+v", list.Plugins)
	}
}
