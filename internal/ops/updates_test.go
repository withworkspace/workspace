package ops

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/release"
	"github.com/withworkspace/workspace/internal/release/releasetest"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// publishedPlugin is a plugin's repo as a test has it: its latest
// release, with this machine's build of it.
type publishedPlugin struct {
	name string
	tag  string // its latest release's; "" for none
	err  error  // what reading its releases fails with
}

// build is a release's build of the plugin for this machine: its
// plugin.toml, saying the release's version, and what its command starts.
func (p *publishedPlugin) build(tag string) (file string, archive []byte) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, text := range map[string]string{
		plugin.ManifestName: fmt.Sprintf("name = %q\nversion = %q\ncommand = [\"./run\"]\n", p.name, strings.TrimPrefix(tag, "v")),
		"run":               "#!/bin/sh\n",
	} {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(text)), Typeflag: tar.TypeReg}) //nolint:errcheck // to memory
		tw.Write([]byte(text))                                                                              //nolint:errcheck // to memory
	}
	tw.Close()
	gz.Close()
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", p.name, tag, runtime.GOOS, runtime.GOARCH), buf.Bytes()
}

func (p *publishedPlugin) Release(repo, tag string) (string, []string, bool, error) {
	if p.err != nil {
		return "", nil, false, p.err
	}
	if tag == "" {
		tag = p.tag
	}
	if tag == "" {
		return "", nil, false, nil
	}
	file, _ := p.build(tag)
	return tag, []string{file, "checksums.txt"}, true, nil
}

func (p *publishedPlugin) Download(repo, tag string, files []string, dir string) error {
	file, archive := p.build(tag)
	sum := sha256.Sum256(archive)
	for _, f := range files {
		data := archive
		if f == "checksums.txt" {
			data = []byte(hex.EncodeToString(sum[:]) + "  " + file + "\n")
		}
		if err := os.WriteFile(filepath.Join(dir, f), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (p *publishedPlugin) Clone(repo, ref, dir string) error {
	return errors.New("its source isn't here")
}

// fakeRunner stands in for the daemon's plugins: nothing's run, and what
// was started afresh and stopped is kept.
type fakeRunner struct {
	root               string
	refreshed, stopped []string
}

func (f *fakeRunner) Statuses(context.Context) ([]plugin.Status, error) {
	return (&plugin.Host{Root: f.root}).Statuses(), nil
}

func (f *fakeRunner) Refresh(_ context.Context, name string) (plugin.Status, error) {
	f.refreshed = append(f.refreshed, name)
	return (&plugin.Host{Root: f.root}).Status(name), nil
}

func (f *fakeRunner) Stop(_ context.Context, name string) error {
	f.stopped = append(f.stopped, name)
	return nil
}

func (f *fakeRunner) Close() {}

// updatesWorld is a workspace whose daemon runs a release's build of ws
// at version, with r as where ws is released, pub as where its plugins
// are published, and a clock the test moves.
func updatesWorld(t *testing.T, r *releasetest.Releases, version string, pub *publishedPlugin) (*Workspace, *fakeRunner, *time.Time) {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	exe := filepath.Join(t.TempDir(), "workspace")
	if err := os.WriteFile(exe, releasetest.Script(version), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	run := &fakeRunner{root: root}
	wasRunners, wasReleases := PluginRunners, PluginReleases
	PluginRunners = func(string, bool) (PluginRunner, error) { return run, nil }
	if pub != nil {
		PluginReleases = pub
	}
	t.Cleanup(func() { PluginRunners, PluginReleases = wasRunners, wasReleases })
	w := New(&config.Config{}, &workspace.Workspace{Root: root}, nil, Options{
		Now: func() time.Time { return now },
		WS:  release.Upgrader{Exe: exe, Version: version, Release: true, Source: release.Source{Base: r.URL}, OS: "linux", Arch: "amd64"},
	})
	return w, run, &now
}

// The daemon looks once a day: a look finds ws's latest release and each
// plugin's, keeps them, and isn't due again until a day's gone.
func TestUpdates_ALookFindsWhatsOutAndIsDueADayLater(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	pub := &publishedPlugin{name: "notes", tag: "v0.1.0"}
	w, _, now := updatesWorld(t, r, "v1.3.0", pub)
	ctx := context.Background()
	if _, _, err := w.AddPlugin(ctx, "acme/plugin-notes", PluginAdd{}); err != nil {
		t.Fatal(err)
	}
	pub.tag = "v0.2.0"

	if !w.UpdatesDue() {
		t.Error("a workspace that's never looked isn't due a look")
	}
	if got := w.LaterWS(); got != "" {
		t.Errorf("before any look, a later ws is said to be out: %s", got)
	}
	up := w.CheckUpdates(ctx)
	if up.WS.Latest != "v1.4.0" || !up.WS.Newer || up.WS.Error != "" || !up.Checked.Equal(*now) {
		t.Errorf("ws: %+v, checked %v", up.WS, up.Checked)
	}
	if !strings.HasSuffix(up.WS.Page, "/releases/tag/v1.4.0") {
		t.Errorf("where the release is described: %q", up.WS.Page)
	}
	if got := up.Plugins["notes"]; got.Latest != "v0.2.0" || !got.Newer {
		t.Errorf("the plugin: %+v", got)
	}
	if got := w.LaterWS(); got != "v1.4.0" {
		t.Errorf("LaterWS() = %q, want v1.4.0", got)
	}
	if rows := w.Plugins(ctx).Plugins; len(rows) != 1 || !rows[0].Update.Newer || rows[0].Update.Latest != "v0.2.0" {
		t.Errorf("the Plugins page's rows: %+v", rows)
	}

	if w.UpdatesDue() {
		t.Error("due again straight after a look")
	}
	*now = now.Add(23 * time.Hour)
	if w.UpdatesDue() {
		t.Error("due again within the day")
	}
	*now = now.Add(2 * time.Hour)
	if !w.UpdatesDue() {
		t.Error("not due a day after the last look")
	}
}

// [updates] check = false in ws.local.toml: the daemon doesn't look by
// itself, and a look that's asked for still does.
func TestUpdates_NotLookedForWhenTurnedOff(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	w, _, _ := updatesWorld(t, r, "v1.3.0", nil)
	if err := os.WriteFile(filepath.Join(w.Root(), config.LocalFileName), []byte("[updates]\ncheck = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w.UpdatesDue() {
		t.Error("due a look with looking turned off")
	}
	if up := w.CheckUpdates(context.Background()); up.Checks || up.WS.Latest != "v1.4.0" {
		t.Errorf("asked to look with looking off: %+v", up)
	}
}

// A look that couldn't ask keeps what the last one found, says why, and
// isn't counted: the daemon tries again within the hour.
func TestUpdates_ALookThatCantAskIsTriedAgain(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	pub := &publishedPlugin{name: "notes", tag: "v0.1.0"}
	w, _, now := updatesWorld(t, r, "v1.3.0", pub)
	ctx := context.Background()
	if _, _, err := w.AddPlugin(ctx, "acme/plugin-notes", PluginAdd{}); err != nil {
		t.Fatal(err)
	}
	first := w.CheckUpdates(ctx).Checked

	r.Close() // nothing answers now
	pub.err = errors.New("no network")
	*now = now.Add(25 * time.Hour)
	up := w.CheckUpdates(ctx)
	if up.WS.Error == "" || up.WS.Latest != "v1.4.0" || !up.WS.Newer {
		t.Errorf("ws, after a look that couldn't ask: %+v", up.WS)
	}
	if got := up.Plugins["notes"]; got.Error == "" || got.Latest != "v0.1.0" {
		t.Errorf("the plugin, after a look that couldn't ask: %+v", got)
	}
	if !up.Checked.Equal(first) || !up.Tried.Equal(*now) {
		t.Errorf("checked %v (want %v, as before), tried %v (want %v)", up.Checked, first, up.Tried, *now)
	}
	if !w.UpdatesDue() {
		t.Error("a look that couldn't ask isn't due to be made again")
	}
}

// A ws that's no release's (a dev build) isn't said to be behind, and one
// ahead of the latest release (a prerelease) isn't offered it.
func TestUpdates_WhatIsntBehind(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	for _, version := range []string{"(devel)", "v1.5.0-rc.1", "v1.4.0", "v1.4.1-0.20261120101500-56368bcb84bf+dirty"} {
		w, _, _ := updatesWorld(t, r, version, nil)
		if up := w.CheckUpdates(context.Background()); up.WS.Newer || w.LaterWS() != "" {
			t.Errorf("ws %s is said to be behind v1.4.0: %+v", version, up.WS)
		}
	}
}

// Upgrading from the web UI installs the latest release in place of the
// ws the daemon runs, as ws upgrade does, and is yours alone to do.
func TestUpgradeWS(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	ctx := context.Background()

	w, _, _ := updatesWorld(t, r, "v1.3.0", nil)
	var refusal *Refused
	if _, err := w.UpgradeWS(ctx, AsCrew("c1")); !errors.As(err, &refusal) || refusal.Code != "not-yours" {
		t.Errorf("crew upgrading ws: %v", err)
	}
	got, err := w.UpgradeWS(ctx, AsYou)
	if err != nil || !got.Installed || !got.Kept || got.From != "v1.3.0" || got.To != "v1.4.0" {
		t.Fatalf("UpgradeWS() = %+v, %v", got, err)
	}
	if v, err := release.VersionOf(w.opts.WS.Exe); err != nil || v != "v1.4.0" {
		t.Errorf("the ws in place is %s (%v), want v1.4.0", v, err)
	}
	// What's out is known from the upgrade, with no look made, and it's
	// said to be waiting before the daemon's found it in its place.
	if up := w.Updates(); up.WS.Latest != "v1.4.0" || up.WS.Installed != "v1.4.0" || w.LaterWS() != "" {
		t.Errorf("after upgrading: %+v, and LaterWS() = %q", up.WS, w.LaterWS())
	}

	// A ws built from a clone is changed where it was built.
	built, _, _ := updatesWorld(t, r, "v1.3.0", nil)
	built.opts.WS.Release = false
	if _, err := built.UpgradeWS(ctx, AsYou); !errors.As(err, &refusal) || refusal.Code != "not-a-release" {
		t.Errorf("upgrading a ws that's no release's build: %v", err)
	}
	if v, _ := release.VersionOf(built.opts.WS.Exe); v != "v1.3.0" {
		t.Errorf("a ws that's no release's build was replaced: it's %s", v)
	}
}

// A ws installed and waiting for the daemon to restart onto it isn't
// offered again.
func TestUpdates_AnInstalledWSIsntOfferedAgain(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	w, _, _ := updatesWorld(t, r, "v1.3.0", nil)
	w.CheckUpdates(context.Background())
	w.opts.Pending = func() string { return "v1.4.0" }
	w.opts.AtWork = func() []string { return []string{"crew c1, mid-turn"} }
	if got := w.LaterWS(); got != "" {
		t.Errorf("LaterWS() = %q with v1.4.0 installed and waiting", got)
	}
	if up := w.Updates(); up.WS.Installed != "v1.4.0" || len(up.WS.Waits) != 1 {
		t.Errorf("what's installed and waiting: %+v", up.WS)
	}
}

// A plugin's added from where it's published, updated when a later
// release is out, and removed: each started or stopped where it runs.
func TestPlugins_AddUpdateRemove(t *testing.T) {
	r := releasetest.New(t)
	pub := &publishedPlugin{name: "notes", tag: "v0.1.0"}
	w, run, _ := updatesWorld(t, r, "v1.3.0", pub)
	ctx := context.Background()

	var said []string
	added, _, err := w.AddPlugin(ctx, " acme/plugin-notes ", PluginAdd{Say: func(line string) { said = append(said, line) }})
	if err != nil {
		t.Fatal(err)
	}
	if added.Name != "notes" || added.Version != "0.1.0" || added.From != "acme/plugin-notes" || added.Release != "v0.1.0" {
		t.Errorf("AddPlugin() = %+v", added)
	}
	if len(said) == 0 || !strings.Contains(said[0], "downloading") {
		t.Errorf("what it said it was doing: %q", said)
	}
	if src, ok := plugin.ReadSource(w.Root(), "notes"); !ok || src.Repo != "acme/plugin-notes" || src.Tag != "v0.1.0" {
		t.Errorf("where it's from: %+v, %v", src, ok)
	}
	if len(run.refreshed) != 1 || run.refreshed[0] != "notes" {
		t.Errorf("started afresh: %v", run.refreshed)
	}

	// Nothing later: nothing's fetched.
	got, err := w.UpdatePlugin(ctx, "notes", nil)
	if err != nil || !got.Current || got.Now != "v0.1.0" || len(run.refreshed) != 1 {
		t.Errorf("updating the latest: %+v, %v (started afresh %v)", got, err, run.refreshed)
	}
	pub.tag = "v0.2.0"
	got, err = w.UpdatePlugin(ctx, "notes", nil)
	if err != nil || got.Current || got.Was != "v0.1.0" || got.Now != "v0.2.0" || got.Added.Version != "0.2.0" {
		t.Errorf("UpdatePlugin() = %+v, %v", got, err)
	}
	if up := w.Updates().Plugins["notes"]; up.Newer || up.Latest != "v0.2.0" {
		t.Errorf("after updating, it's still said to have a later release: %+v", up)
	}
	if _, err := w.UpdatePlugin(ctx, "nope", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("updating a plugin that isn't here: %v", err)
	}

	// The team's that aren't here are listed with those that are.
	w.Config().Plugins = map[string]config.PluginSource{
		"notes": {Source: "acme/plugin-notes"},
		"jira":  {Source: "acme/plugin-jira", Version: "v0.3.0"},
	}
	view := w.Plugins(ctx)
	if len(view.Plugins) != 1 || view.Plugins[0].Name != "notes" || len(view.Wanted) != 1 || view.Wanted[0] != (WantedPlugin{Name: "jira", Source: "acme/plugin-jira", Version: "v0.3.0"}) {
		t.Errorf("Plugins() = %+v", view)
	}

	if err := w.RemovePlugin(ctx, "notes", false); err != nil {
		t.Fatal(err)
	}
	if plugin.IsInstalled(w.Root(), "notes") || len(run.stopped) != 1 {
		t.Errorf("removed: installed %v, stopped %v", plugin.IsInstalled(w.Root(), "notes"), run.stopped)
	}
	if err := w.RemovePlugin(ctx, "notes", false); !errors.Is(err, ErrNotFound) {
		t.Errorf("removing a plugin that isn't here: %v", err)
	}
}

// A plugin added from a folder is the folder's: it's from nowhere, and
// isn't updated.
func TestPlugins_AddFromAFolder(t *testing.T) {
	r := releasetest.New(t)
	pub := &publishedPlugin{name: "notes", tag: "v0.1.0"}
	w, _, _ := updatesWorld(t, r, "v1.3.0", pub)
	ctx := context.Background()
	if _, _, err := w.AddPlugin(ctx, "acme/plugin-notes", PluginAdd{}); err != nil {
		t.Fatal(err)
	}

	folder := filepath.Join(t.TempDir(), "plugin-notes")
	os.MkdirAll(folder, 0o755)
	os.WriteFile(filepath.Join(folder, plugin.ManifestName), []byte("name = \"notes\"\nversion = \"0.9.0\"\ncommand = [\"./run\"]\n"), 0o644)
	os.WriteFile(filepath.Join(folder, "run"), []byte("#!/bin/sh\n"), 0o755)
	added, _, err := w.AddPlugin(ctx, folder, PluginAdd{})
	if err != nil || added.Version != "0.9.0" || added.Folder != folder {
		t.Fatalf("AddPlugin(folder) = %+v, %v", added, err)
	}
	if _, ok := plugin.ReadSource(w.Root(), "notes"); ok {
		t.Error("one added from a folder is still said to be from a repo")
	}
	var refusal *Refused
	if _, err := w.UpdatePlugin(ctx, "notes", nil); !errors.As(err, &refusal) || refusal.Code != "folder" {
		t.Errorf("updating one added from a folder: %v", err)
	}

	var inv *Invalid
	for _, from := range []string{"", "not a repo", filepath.Join(t.TempDir(), "nowhere")} {
		if _, _, err := w.AddPlugin(ctx, from, PluginAdd{}); !errors.As(err, &inv) {
			t.Errorf("AddPlugin(%q): %v, want it refused as invalid", from, err)
		}
	}
	if _, _, err := w.AddPlugin(ctx, "acme/plugin-notes", PluginAdd{Link: true}); !errors.As(err, &inv) {
		t.Errorf("linking to a repo: %v", err)
	}
}
