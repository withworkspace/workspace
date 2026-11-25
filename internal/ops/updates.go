package ops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/release"
)

// Updates: what's been released that's later than what's here. The
// workspace's daemon looks once a day (unless [updates] check says not
// to), and when it's asked to: for ws's latest release, and the latest of
// each plugin installed from a repo. What it found is kept in a file, so
// a daemon that's started again knows when it last looked, and the pages
// that show it (Settings' About, Plugins) read that and ask nobody.
// Nothing's installed by looking.

// UpdatesEvery is how long after a look that answered the daemon looks
// again by itself.
const UpdatesEvery = 24 * time.Hour

// PluginReleases is where plugins are published: GitHub. A test's is its
// own.
var PluginReleases plugin.Releases = github.Releases{}

// Updates is what the last look found.
type Updates struct {
	// Checks is whether the daemon looks by itself ([updates] check).
	Checks bool
	// Checked is when a look last answered, and Tried when one was last
	// made: zero for never.
	Checked, Tried time.Time
	WS             WSUpdate
	// Plugins is what's out for each plugin installed from a repo, by its
	// name.
	Plugins map[string]PluginUpdate
}

// WSUpdate is ws's own: the one that's running, and the latest release.
type WSUpdate struct {
	// Version is the ws that's running (the daemon's), and Release whether
	// it's a release's build: the kind an upgrade replaces.
	Version string
	Release bool
	// Latest is the latest release, as last found ("" for not known), and
	// Newer whether it's later than Version. Error is why the last look
	// didn't find it.
	Latest string
	Newer  bool
	Error  string
	// Page is where the latest release is described, to read what's in
	// it: "" where there's none known.
	Page string
	// Installed is a ws that's been put in the daemon's place, which it
	// isn't running yet ("" for none): it restarts onto it once nothing's
	// at work, and Waits is what's at work.
	Installed string
	Waits     []string
}

// PluginUpdate is a plugin's: its repo's latest release, as last found,
// and whether that's later than the one installed.
type PluginUpdate struct {
	Latest string
	Newer  bool
	Error  string
}

// keptUpdates is what a look found, as it's kept.
type keptUpdates struct {
	Checked time.Time              `json:"checked"`
	Tried   time.Time              `json:"tried"`
	WS      keptRelease            `json:"ws"`
	Plugins map[string]keptRelease `json:"plugins,omitempty"`
}

// keptRelease is the latest release of a repo, or why it wasn't found.
// Repo is the one that was asked, for a plugin: one installed from
// another repo since isn't said to have what the old one had.
type keptRelease struct {
	Repo   string `json:"repo,omitempty"`
	Latest string `json:"latest,omitempty"`
	Error  string `json:"error,omitempty"`
}

func updatesPath(root string) string {
	return filepath.Join(root, ".ws", "cache", "updates.json")
}

func readUpdates(root string) keptUpdates {
	var k keptUpdates
	if data, err := os.ReadFile(updatesPath(root)); err == nil {
		json.Unmarshal(data, &k) //nolint:errcheck // one that doesn't read is none: it's looked for again
	}
	return k
}

func writeUpdates(root string, k keptUpdates) error {
	data, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return err
	}
	path := updatesPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Written beside it and renamed: a page reading it never finds half.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// later is whether a release is later than what's installed. Where both
// are tags it's by their numbers, so a prerelease that's ahead of the
// latest release isn't offered that; otherwise (a branch's name, a build
// of no version) anything else is.
func later(latest, installed string) bool {
	switch {
	case latest == "" || latest == installed:
		return false
	case release.IsTag(latest) && release.IsTag(installed):
		return release.Newer(latest, installed)
	}
	return true
}

// checksUpdates is whether the daemon looks by itself: on, unless your
// ws.local.toml says not to. A file that doesn't read says nothing.
func checksUpdates(root string) bool {
	l, err := config.ReadLocal(root)
	return err != nil || l.Updates.Checks()
}

// Updates is what the last look found, against what's here now.
func (w *Workspace) Updates() Updates {
	k := readUpdates(w.root)
	ws := w.opts.WS
	out := Updates{
		Checks: checksUpdates(w.root), Checked: k.Checked, Tried: k.Tried,
		WS:      WSUpdate{Version: ws.Version, Release: ws.Replaceable(), Latest: k.WS.Latest, Error: k.WS.Error},
		Plugins: map[string]PluginUpdate{},
	}
	// A build of no version (dev) isn't said to be behind anything.
	out.WS.Newer = release.IsTag(ws.Version) && later(k.WS.Latest, ws.Version)
	if k.WS.Latest != "" && ws.Source.Base != "" {
		out.WS.Page = ws.Source.Base + "/releases/tag/" + k.WS.Latest
	}
	if out.WS.Installed = w.pendingWS(); out.WS.Installed != "" && w.opts.AtWork != nil {
		out.WS.Waits = w.opts.AtWork()
	}
	for _, name := range plugin.Installed(w.root) {
		src, ok := plugin.ReadSource(w.root, name)
		kept, found := k.Plugins[name]
		if !ok || !found || kept.Repo != src.Repo {
			continue
		}
		out.Plugins[name] = PluginUpdate{Latest: kept.Latest, Error: kept.Error, Newer: later(kept.Latest, src.Tag)}
	}
	return out
}

// LaterWS is the release of ws that's out and later than the one running,
// as last found: "" for none, or for one that's installed already and
// waiting for the daemon to restart onto it.
func (w *Workspace) LaterWS() string {
	k := readUpdates(w.root)
	ws := w.opts.WS
	if !release.IsTag(ws.Version) || !later(k.WS.Latest, ws.Version) {
		return ""
	}
	if w.pendingWS() == k.WS.Latest {
		return ""
	}
	return k.WS.Latest
}

// pendingWS is a ws that's been put in the daemon's place which it isn't
// running yet: as the daemon's found it, or, in the moments before it
// has, as an upgrade from here installed it. "" for none.
func (w *Workspace) pendingWS() string {
	if w.opts.Pending != nil {
		if version := w.opts.Pending(); version != "" {
			return version
		}
	}
	if version, _ := w.upgraded.Load().(string); version != w.opts.WS.Version {
		return version
	}
	return ""
}

// UpdatesDue is whether it's time the daemon looked by itself: it's a day
// since a look answered (or none has), and looking isn't turned off.
func (w *Workspace) UpdatesDue() bool {
	if !checksUpdates(w.root) {
		return false
	}
	k := readUpdates(w.root)
	return k.Checked.IsZero() || w.now().Sub(k.Checked) >= UpdatesEvery
}

// CheckUpdates looks now: for ws's latest release, and the latest of each
// plugin installed from a repo. What it finds is kept, and returned. A
// look that couldn't ask (no network) keeps what the last one found, with
// why, and isn't counted as having looked: the daemon tries again within
// the hour.
func (w *Workspace) CheckUpdates(ctx context.Context) Updates {
	w.updates.Lock()
	defer w.updates.Unlock()
	was := readUpdates(w.root)
	now := w.now().UTC()
	k := keptUpdates{Checked: was.Checked, Tried: now, WS: was.WS, Plugins: map[string]keptRelease{}}
	answered := false
	// A ws that doesn't know where it's released (a test's, a command's)
	// isn't looked for.
	if w.opts.WS.Source.Base != "" {
		latest, err := w.opts.WS.Source.Latest(ctx)
		if err != nil {
			k.WS.Error = err.Error()
		} else {
			k.WS, answered = keptRelease{Latest: latest}, true
		}
	} else {
		answered = true
	}
	for _, name := range plugin.Installed(w.root) {
		src, ok := plugin.ReadSource(w.root, name)
		if !ok {
			continue // added from a folder: it's the folder's
		}
		kept := keptRelease{Repo: src.Repo}
		if old, found := was.Plugins[name]; found && old.Repo == src.Repo {
			kept.Latest = old.Latest
		}
		switch tag, _, found, err := PluginReleases.Release(src.Repo, ""); {
		case err != nil:
			kept.Error = err.Error()
		case found:
			kept.Latest = tag
		default:
			kept.Latest = "" // no release: its source is what there is
		}
		k.Plugins[name] = kept
	}
	if answered {
		k.Checked = now
	}
	if err := writeUpdates(w.root, k); err != nil {
		execlog.Note(w.root, "updates", "what was found isn't kept: %v", err)
	}
	w.notifyPluginUpdates(was, k, func(name string) string {
		src, _ := plugin.ReadSource(w.root, name)
		return src.Tag
	})
	return w.Updates()
}

// UpgradeWS installs the latest release of ws in place of the one the
// daemon runs, as ws upgrade does: fetched, checked against the release's
// checksums, run once, and put in place with the old one kept beside it.
// The daemon restarts onto it once nothing's at work. Nothing's installed
// where the latest isn't later than what's running, or where this ws
// isn't a release's build.
func (w *Workspace) UpgradeWS(ctx context.Context, by Actor) (release.Upgraded, error) {
	if !by.IsYou() {
		return release.Upgraded{}, refused("not-yours", "upgrading ws is for whoever runs the workspace, not %s: it changes the ws every crew and the daemon run", by.By())
	}
	u := w.opts.WS
	if !u.Replaceable() {
		return release.Upgraded{From: u.Version}, &Refused{Code: "not-a-release", Reason: u.ErrNotARelease().Error()}
	}
	w.updates.Lock()
	defer w.updates.Unlock()
	r, err := u.Upgrade(ctx, "", false)
	if err != nil {
		return r, &Upstream{Err: err}
	}
	if r.Installed {
		w.upgraded.Store(r.To)
	}
	// What was found is what's out now, whether or not it was installed.
	k := readUpdates(w.root)
	k.WS = keptRelease{Latest: r.To}
	if err := writeUpdates(w.root, k); err != nil {
		execlog.Note(w.root, "updates", "what was found isn't kept: %v", err)
	}
	return r, nil
}
