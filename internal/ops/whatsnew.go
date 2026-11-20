package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/release"
	"github.com/withworkspace/workspace/internal/whatsnew"
)

// What's new: after ws is upgraded, the web UI says what's new in the
// releases since the one last seen here, once (internal/whatsnew says
// where that's from). The daemon keeps which release was last seen, not
// the browser: so it's once for every browser, and a workspace that's new
// isn't told what's new in a ws it's never run anything else of.

// WhatsNew is what's new, for this workspace.
type WhatsNew struct {
	// Version is the ws that's running, and Release whether it's a
	// release's build. A build of a clone says nothing's unseen, and has
	// every entry its build found, unreleased ones too: to see how they
	// read.
	Version string
	Release bool
	// Unseen is what's new since the release last seen here, newest
	// first; Entries is everything this ws says is new, up to and
	// including itself.
	Unseen  []whatsnew.Entry
	Entries []whatsnew.Entry
	// Seen is the release whose news was last seen here: "" for none.
	Seen string
	// Missing is whether some of what's new since the release last seen
	// isn't held by this ws (it's from too many releases ago): it's on
	// Releases, the releases page.
	Missing  bool
	Releases string
	// Pictures is whether entries' pictures can be fetched: they're the
	// release's own files, and fetching is looking at GitHub, which
	// [updates] check = false says not to.
	Pictures bool
	// Local is whether a build of a clone shows entries' pictures from the
	// clone, unreleased ones too: so whoever writes one sees it as it'll
	// read.
	Local bool
}

// keptSeen is the release whose news was last seen here.
type keptSeen struct {
	Seen string `json:"seen"`
}

func seenPath(root string) string { return filepath.Join(root, ".ws", "cache", "whatsnew.json") }

func readSeen(root string) string {
	var k keptSeen
	if data, err := os.ReadFile(seenPath(root)); err == nil {
		json.Unmarshal(data, &k) //nolint:errcheck // one that doesn't read is none
	}
	return k.Seen
}

func writeSeen(root, version string) error {
	data, err := json.Marshal(keptSeen{Seen: version})
	if err != nil {
		return err
	}
	path := seenPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// isRelease is whether a ws is a release's build. One built from a clone
// can have a version that reads like a tag (Go's pseudo-version, v1.4.1-0.2026…),
// so it's what it was built as that says.
func isRelease(ws release.Upgrader) bool { return ws.Release && release.IsTag(ws.Version) }

// held is what this ws says is new; a test's is its own.
var held = whatsnew.Held

// WhatsNew is what's new, and what of it hasn't been seen here.
func (w *Workspace) WhatsNew() WhatsNew {
	ws := w.opts.WS
	ix := held()
	out := WhatsNew{
		Version: ws.Version, Release: isRelease(ws),
		Entries:  slices.Clone(ix.Entries),
		Pictures: ws.Source.Base != "" && checksUpdates(w.root),
	}
	if ws.Source.Base != "" {
		out.Releases = ws.Source.Base + "/releases"
	}
	if !out.Release {
		out.Pictures, out.Local = false, ix.Dir != ""
		return out
	}
	out.Entries = ix.UpTo(ws.Version)
	if seen := readSeen(w.root); release.IsTag(seen) {
		out.Seen = seen
		out.Unseen = ix.After(seen, ws.Version)
		out.Missing = len(out.Unseen) > 0 && ix.Missing(seen)
	}
	return out
}

// SeenWhatsNew says what's new in the ws that's running has been seen.
func (w *Workspace) SeenWhatsNew() error {
	if !isRelease(w.opts.WS) {
		return nil
	}
	return writeSeen(w.root, w.opts.WS.Version)
}

// NoteRunning is the daemon saying which ws it's started as, for what's
// new. A workspace that's seen nothing yet starts from the ws an upgrade
// replaced, where there's one kept (before is its version), so the first
// upgrade it's run through says what's new; and from this one otherwise,
// so a new workspace isn't told what's new in a ws it never ran. A ws
// older than the one last seen (put back after an upgrade) says nothing
// of what's in the newer, and starts from itself.
func (w *Workspace) NoteRunning(before func() string) {
	running := w.opts.WS.Version
	if !isRelease(w.opts.WS) {
		return
	}
	seen := readSeen(w.root)
	switch {
	case release.IsTag(seen) && !release.Newer(seen, running):
		return
	case release.IsTag(seen): // rolled back
		seen = running
	default:
		seen = running
		if was := before(); release.IsTag(was) && release.Newer(running, was) {
			seen = was
		}
	}
	if err := writeSeen(w.root, seen); err != nil {
		execlog.Note(w.root, "whatsnew", "which release was last seen isn't kept: %v", err)
	}
}

// WhatsNewPicture is an entry's picture: fetched from its release the
// first time it's asked for, and kept. Only the pictures this ws's
// entries name are fetched, and none where looking at GitHub is turned
// off.
func (w *Workspace) WhatsNewPicture(ctx context.Context, name string) (string, error) {
	ix := held()
	if !isRelease(w.opts.WS) {
		return localPicture(ix, name)
	}
	var of *whatsnew.Entry
	for _, e := range ix.Entries {
		if e.Picture == name && e.Version != "" {
			of = &e
			break
		}
	}
	base := w.opts.WS.Source.Base
	switch {
	case of == nil:
		return "", fmt.Errorf("no entry has the picture %q: %w", name, ErrNotFound)
	case base == "" || !checksUpdates(w.root):
		return "", refused("not-looking", "pictures of what's new are fetched from the release on GitHub, and this workspace doesn't look there ([updates] check)")
	}
	path := filepath.Join(w.root, ".ws", "cache", "whatsnew", of.Version, name)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	url := fmt.Sprintf("%s/releases/download/%s/%s", base, of.Version, whatsnew.AssetName(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", &Upstream{Err: err}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", &Upstream{Err: fmt.Errorf("%s: %s", url, res.Status)}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	// A picture is small: one that isn't is cut off, not kept.
	if _, err := io.Copy(f, io.LimitReader(res.Body, 8<<20)); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", &Upstream{Err: err}
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return path, os.Rename(tmp, path)
}

// localPicture is an entry's picture as a build of a clone shows it: from
// the entries' folder in the clone, released or not.
func localPicture(ix whatsnew.Index, name string) (string, error) {
	for _, e := range ix.Entries {
		if e.Picture != name || ix.Dir == "" {
			continue
		}
		path := filepath.Join(ix.Dir, name)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("%s isn't in %s any more: %w", name, ix.Dir, ErrNotFound)
		}
		return path, nil
	}
	return "", fmt.Errorf("no entry has the picture %q: %w", name, ErrNotFound)
}
