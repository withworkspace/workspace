package ops

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/plugin"
)

// Which plugins the workspace has: adding one (from a folder, or the repo
// it's published at), updating one to its repo's latest release, and
// removing one. ws plugin add, update and remove are these, and so is the
// web UI's Plugins page.

// PluginRunner is the workspace's plugins as they run: where each stands,
// one started afresh, one stopped. It's the workspace's daemon, or a
// test's.
type PluginRunner interface {
	Statuses(ctx context.Context) ([]plugin.Status, error)
	Refresh(ctx context.Context, name string) (plugin.Status, error)
	Stop(ctx context.Context, name string) error
	Close()
}

// PluginRunners is how ops reaches the plugins as they run: through the
// daemon's socket for them. start says a plugin's to be run, so a daemon
// that isn't running is to be started: cli's does that, and a test
// replaces it. Without start, no daemon is an error, and nothing's
// running to ask.
var PluginRunners = func(root string, start bool) (PluginRunner, error) {
	c, err := plugin.Dial(root)
	if err != nil {
		return nil, fmt.Errorf("could not reach plugins (is Workspace running?): %w", err)
	}
	return c, nil
}

// pluginRunner is the plugins as they run: the daemon's own, in it
// (Options.Plugins), else through its socket (PluginRunners).
func (w *Workspace) pluginRunner(start bool) (PluginRunner, error) {
	if w.opts.Plugins != nil {
		return w.opts.Plugins, nil
	}
	return PluginRunners(w.root, start)
}

// PluginAdded is how adding a plugin went, as ws plugin add -f json
// prints it, for a program that adds one (plugin-dev, on every save).
// It says how far it got, however it went.
type PluginAdded struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	// Folder is the plugin's own, and Executable what its command starts
	// there ("" for a program found on PATH).
	Folder     string `json:"folder"`
	Executable string `json:"executable,omitempty"`
	// Built says whether it was built here, and BuildOutput what building
	// it printed. Error is why it didn't build, or didn't start.
	Built       bool   `json:"built"`
	BuildOutput string `json:"build_output,omitempty"`
	Error       string `json:"error,omitempty"`
	// State and Missing are where it stands once it's started.
	State   plugin.State `json:"state,omitempty"`
	Missing []string     `json:"missing,omitempty"`
	// From is the repo it was fetched from, and Release which release of
	// it ("" for its source, built here).
	From    string `json:"from,omitempty"`
	Release string `json:"release,omitempty"`
}

// PluginAdd is how a plugin is to be added.
type PluginAdd struct {
	// Link runs it from the folder itself, for one being written.
	Link bool
	// Build builds it first, whether or not it's been built.
	Build bool
	// Say is told what's being done as it's done (a download, a build
	// from source); nil for nobody to tell.
	Say func(line string)
}

func (o PluginAdd) say(line string) {
	if o.Say != nil {
		o.Say(line)
	}
}

// homeFolder is a path as it's typed, with ~ as your home.
func homeFolder(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// AddPlugin installs a plugin into the workspace and starts it, to hear
// what it adds. from is a folder on this machine (its whole path, or from
// ~), or where one's published: owner/repo, with @tag for a release that
// isn't the latest. Adding one that's already here replaces it, keeping
// its config and data. What's returned says how far it got, whatever the
// error.
func (w *Workspace) AddPlugin(ctx context.Context, from string, o PluginAdd) (PluginAdded, plugin.Status, error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return PluginAdded{}, plugin.Status{}, invalid("name a plugin: a folder on this machine, or its repo (owner/repo)")
	}
	w.plugins.Lock()
	defer w.plugins.Unlock()
	if folder := homeFolder(from); filepath.IsAbs(folder) {
		if info, err := os.Stat(folder); err == nil && info.IsDir() {
			res, st, err := w.addPluginFolder(ctx, folder, o.Link, o.Build)
			if err == nil {
				// From a folder now, wherever it was from before.
				plugin.WriteSource(w.root, res.Name, plugin.Source{}) //nolint:errcheck // it only says where from
			}
			return res, st, err
		}
	}
	repo, tag, ok := plugin.ParseRepo(from)
	switch {
	case !ok:
		return PluginAdded{}, plugin.Status{}, invalid("%s is not a folder here or a plugin repo (owner/repo)", from)
	case o.Link:
		return PluginAdded{}, plugin.Status{}, invalid("--link runs a plugin from a folder on this machine: give the folder")
	}
	return w.installPlugin(ctx, repo, tag, o)
}

// installPlugin fetches the plugin published at repo (its release tagged
// tag, or with "" its latest), installs it and starts it, and records
// where it's from.
func (w *Workspace) installPlugin(ctx context.Context, repo, tag string, o PluginAdd) (PluginAdded, plugin.Status, error) {
	tmp, err := os.MkdirTemp("", "ws-plugin-")
	if err != nil {
		return PluginAdded{}, plugin.Status{}, err
	}
	defer os.RemoveAll(tmp)
	from, err := plugin.Fetch(PluginReleases, repo, tag, runtime.GOOS, runtime.GOARCH, tmp, o.say)
	if err != nil {
		return PluginAdded{From: repo, Error: err.Error()}, plugin.Status{}, &Upstream{Err: err}
	}
	res, st, err := w.addPluginFolder(ctx, tmp, false, o.Build)
	res.From, res.Release, res.Folder, res.Executable = from.Repo, from.Tag, "", ""
	if err != nil {
		return res, st, err
	}
	if err := plugin.WriteSource(w.root, res.Name, from); err != nil {
		return res, st, err
	}
	return res, st, nil
}

// addPluginFolder builds the plugin in src if it needs it (or build says
// to), installs it, and starts it.
func (w *Workspace) addPluginFolder(ctx context.Context, src string, link, build bool) (PluginAdded, plugin.Status, error) {
	res := PluginAdded{Folder: src}
	m, err := plugin.ReadManifest(src)
	if err != nil {
		res.Error = err.Error()
		return res, plugin.Status{}, &Invalid{Reason: err.Error()}
	}
	res.Name, res.Version, res.Executable = m.Name, m.Version, m.Executable(src)

	var out strings.Builder
	if build {
		err = plugin.Build(src, &out)
		res.Built = err == nil && len(m.Build) > 0
	} else {
		_, missing := os.Stat(res.Executable)
		err = plugin.BuildIfMissing(src, &out)
		res.Built = err == nil && res.Executable != "" && missing != nil
	}
	res.BuildOutput = out.String()
	if err != nil {
		res.Error = err.Error()
		return res, plugin.Status{}, &Refused{Code: "build", Reason: err.Error(), Err: err}
	}
	if _, err := plugin.Add(w.root, src, link); err != nil {
		res.Error = err.Error()
		return res, plugin.Status{}, err
	}
	run, err := w.pluginRunner(true)
	if err != nil {
		res.Error = err.Error()
		return res, plugin.Status{}, err
	}
	defer run.Close()
	st, err := run.Refresh(ctx, m.Name)
	if err != nil {
		res.State, res.Error = plugin.Failed, err.Error()
		return res, st, &Refused{Code: "start", Reason: fmt.Sprintf("%s is installed, but did not start: %v", m.Name, err), Err: err}
	}
	res.State, res.Missing = st.State, st.Missing
	return res, st, nil
}

// RemovePlugin stops a plugin and takes it out of the workspace, with
// what ws kept of it in its records: how its things are named, and
// anything of its in the graph. What it kept for itself (its data folder)
// and its secrets stay unless data says they go too.
func (w *Workspace) RemovePlugin(ctx context.Context, name string, data bool) error {
	w.plugins.Lock()
	defer w.plugins.Unlock()
	// One whose folder is there with nothing in it that would run is still
	// there to remove.
	if !plugin.ValidName(name) || !slices.Contains(pluginFolders(w.root), name) {
		return fmt.Errorf("%w: no plugin called %s", ErrNotFound, name)
	}
	if run, err := w.pluginRunner(false); err == nil {
		run.Stop(ctx, name) //nolint:errcheck // not running is as good
		run.Close()
	}
	if err := plugin.Remove(w.root, name, data); err != nil {
		return err
	}
	if err := plugin.Forget(w.root, name); err != nil {
		return fmt.Errorf("removed %s, but could not clear its records: %w", name, err)
	}
	return nil
}

// pluginFolders are the names under the workspace's plugins folder that
// have a plugin's files in them, whether or not they'd run.
func pluginFolders(root string) []string {
	entries, err := os.ReadDir(plugin.Dir(root))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if _, err := os.Lstat(filepath.Join(plugin.Dir(root), e.Name(), "app")); err == nil {
			out = append(out, e.Name())
		}
	}
	return out
}

// PluginUpdated is how updating a plugin went.
type PluginUpdated struct {
	Name string
	// Was and Now are what it was and what it is, as they're said: a
	// release's tag, or "its source".
	Was, Now string
	// Current says it was the latest already: nothing was fetched.
	Current bool
	// Added is how installing the newer one went.
	Added PluginAdded
}

// UpdatePlugin fetches the latest release of a plugin that was installed
// from a repo, if it's not what's here, and starts it afresh. Its config
// and data stay. One built from its source is fetched and built again, as
// there's no saying whether it's changed. One added from a folder isn't
// updated: it's the folder's.
func (w *Workspace) UpdatePlugin(ctx context.Context, name string, say func(line string)) (PluginUpdated, error) {
	w.plugins.Lock()
	defer w.plugins.Unlock()
	out := PluginUpdated{Name: name}
	if !plugin.IsInstalled(w.root, name) {
		return out, fmt.Errorf("%w: no plugin called %s (ws plugin list)", ErrNotFound, name)
	}
	src, ok := plugin.ReadSource(w.root, name)
	if !ok {
		return out, refused("folder", "%s was added from a folder, not a repo: nothing to update from", name)
	}
	out.Was = cmp.Or(src.Tag, "source")
	latest, _, found, err := PluginReleases.Release(src.Repo, "")
	if err != nil {
		return out, &Upstream{Err: err}
	}
	if found {
		w.keepPluginLatest(name, src.Repo, latest)
	}
	if found && src.File != "" && latest == src.Tag {
		out.Now, out.Current = src.Tag, true
		return out, nil
	}
	res, _, err := w.installPlugin(ctx, src.Repo, "", PluginAdd{Say: say})
	out.Added = res
	if err != nil {
		return out, err
	}
	out.Now = cmp.Or(res.Release, "built from source")
	return out, nil
}

// keepPluginLatest keeps a plugin's repo's latest release, as it was just
// found, among what the last look for updates found: so the pages that
// show what's out don't say an update that's just been made is still to
// make.
func (w *Workspace) keepPluginLatest(name, repo, latest string) {
	w.updates.Lock()
	defer w.updates.Unlock()
	k := readUpdates(w.root)
	if k.Plugins == nil {
		k.Plugins = map[string]keptRelease{}
	}
	k.Plugins[name] = keptRelease{Repo: repo, Latest: latest}
	writeUpdates(w.root, k) //nolint:errcheck // the next look finds it again
}

// PluginsView is the workspace's plugins, as the Plugins page lists them.
type PluginsView struct {
	Plugins []PluginRow
	// Wanted are the team's (ws.toml's [plugins]) that aren't installed
	// here.
	Wanted []WantedPlugin
	// Checks is whether the daemon looks for updates by itself, and
	// Checked when a look last answered: zero for never.
	Checks  bool
	Checked time.Time
}

// PluginRow is one installed plugin: where it stands, and what the last
// look for updates found for it (the zero value, for one that isn't from
// a repo, or that hasn't been looked for).
type PluginRow struct {
	plugin.Status
	Update PluginUpdate
}

// WantedPlugin is a plugin the team uses that isn't installed here, and
// where it comes from.
type WantedPlugin struct{ Name, Source, Version string }

// PluginStatuses are the workspace's plugins: as the daemon has them if
// it can be reached, else as they were left.
func (w *Workspace) PluginStatuses(ctx context.Context) []plugin.Status {
	if run, err := w.pluginRunner(false); err == nil {
		defer run.Close()
		if out, err := run.Statuses(ctx); err == nil {
			return out
		}
	}
	return (&plugin.Host{Root: w.root}).Statuses()
}

// Plugins is the workspace's plugins, with what's out for each and the
// team's that aren't here.
func (w *Workspace) Plugins(ctx context.Context) PluginsView {
	up := w.Updates()
	out := PluginsView{Plugins: []PluginRow{}, Wanted: []WantedPlugin{}, Checks: up.Checks, Checked: up.Checked}
	here := map[string]bool{}
	for _, st := range w.PluginStatuses(ctx) {
		here[st.Name] = true
		out.Plugins = append(out.Plugins, PluginRow{Status: st, Update: up.Plugins[st.Name]})
	}
	for name, p := range w.Config().Plugins {
		if !here[name] {
			out.Wanted = append(out.Wanted, WantedPlugin{Name: name, Source: p.Source, Version: p.Version})
		}
	}
	sort.Slice(out.Wanted, func(i, j int) bool { return out.Wanted[i].Name < out.Wanted[j].Name })
	return out
}
