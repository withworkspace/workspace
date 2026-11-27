package ops

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/store"
)

// A plugin's kind of thing can live in a repo (protocol.Remote): a folder
// each, with a thing.toml that says what it is. The spaces are where ws
// finds them (internal/space), and what a plugin makes of one's main file
// is kept with the plugin's other things (internal/plugin): this is what
// brings the two in step, asking the plugin what a file is found by once
// for each version of it, and what answers a plugin asking which there
// are and for one's file.

// RemoteReader is what asks a plugin about its things in a repo: what one's
// main file is found by, one of its own as its folder is to hold it, and
// telling it where one is now. It's the workspace's daemon
// (plugin.Client), or a test's.
type RemoteReader interface {
	ReadRemote(ctx context.Context, name string, in protocol.RemoteReadParams) (map[string]string, error)
	ExportRemote(ctx context.Context, name string, in protocol.RemoteExportParams) (protocol.RemoteExported, error)
	SavedRemote(ctx context.Context, name string, in protocol.RemoteSavedParams) error
	Close()
}

// RemoteReaders is how ops reaches the plugins about their things in a
// repo: through the daemon's socket. A test replaces it.
var RemoteReaders = func(root string) (RemoteReader, error) {
	c, err := plugin.Dial(root)
	if err != nil {
		return nil, fmt.Errorf("could not reach plugins (is Workspace running?): %w", err)
	}
	return c, nil
}

// remoteKinds are the kinds whose things can live in a repo, by kind, as
// the plugins that are installed registered them: the first plugin to
// register a kind by a name has it.
func remoteKinds(root string) (map[string]plugin.KeptRef, error) {
	kept, err := plugin.Refs(root)
	if err != nil {
		return nil, err
	}
	out := map[string]plugin.KeptRef{}
	for _, k := range kept {
		if _, taken := out[k.Kind]; k.Remote == nil || taken || !plugin.IsInstalled(root, k.Plugin) {
			continue
		}
		out[k.Kind] = k
	}
	return out, nil
}

// foundVersion is what a thing found in a space is kept at: what it's
// made of, and its kind as its plugin registered it, so one is read
// again when either changes.
func foundVersion(t space.Thing, k plugin.KeptRef) string {
	return t.Version() + "@" + k.Registered.UTC().Format(store.Stamp)
}

// One sync of the found things at a time: a second, begun while one's
// under way (a plugin asking which there are as its file is read), has
// nothing to add to it.
var syncingRemotes sync.Mutex

// syncRemotes brings the things ws keeps for plugins in step with the
// things in the spaces: one that's new, or changed, or whose kind's
// plugin has registered it afresh, has its main file read by the plugin
// and is kept; one that's gone, or whose kind no plugin has now, is
// forgotten. Nothing's asked of a plugin when nothing's changed. A plugin
// that can't be reached is left for the next time.
func (w *Workspace) syncRemotes(ctx context.Context, db *store.DB) error {
	if !syncingRemotes.TryLock() {
		return nil
	}
	defer syncingRemotes.Unlock()
	root := w.WS().Root
	things, err := space.Things(ctx, db, "")
	if err != nil {
		return err
	}
	had, err := plugin.Findings(root)
	if err != nil {
		return err
	}
	if len(things) == 0 && len(had) == 0 {
		return nil
	}
	kinds, err := remoteKinds(root)
	if err != nil {
		return err
	}
	type key struct{ plugin, kind, name string }
	kept := map[key]string{}
	for _, f := range had {
		kept[key{f.Plugin, f.Kind, f.Name}] = f.Version
	}
	want := map[key]bool{}
	var read []space.Thing
	for _, t := range things {
		k, ok := kinds[t.Kind]
		if !ok {
			continue // a kind no plugin here has: it's in the space, and that's all
		}
		at := key{k.Plugin, t.Kind, t.Name()}
		want[at] = true
		if kept[at] != foundVersion(t, k) {
			read = append(read, t)
		}
	}
	gone := map[string][]plugin.Finding{}
	for _, f := range had {
		if !want[key{f.Plugin, f.Kind, f.Name}] {
			gone[f.Plugin] = append(gone[f.Plugin], f)
		}
	}
	if len(read) == 0 && len(gone) == 0 {
		return nil
	}
	found := map[string][]plugin.Found{}
	if len(read) > 0 {
		reader, err := RemoteReaders(root)
		if err != nil {
			reader = nil // nobody to ask: what needs reading waits
		} else {
			defer reader.Close()
		}
		unreachable := map[string]bool{}
		for _, t := range read {
			k := kinds[t.Kind]
			if unreachable[k.Plugin] {
				continue
			}
			fields, err := w.readRemote(ctx, reader, k, t)
			if err != nil {
				// The plugin isn't there to ask just now.
				unreachable[k.Plugin] = true
				continue
			}
			changed, _ := time.Parse(time.RFC3339, t.Changed)
			found[k.Plugin] = append(found[k.Plugin], plugin.Found{Kind: t.Kind, Name: t.Name(), Version: foundVersion(t, k), Changed: changed, Fields: fields})
		}
	}
	names := map[string]bool{}
	for name := range found {
		names[name] = true
	}
	for name := range gone {
		names[name] = true
	}
	var errs []error
	for name := range names {
		if err := plugin.KeepFound(root, name, found[name], gone[name]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// readRemote is a thing's values as it's kept: its title, from its
// manifest, and what its kind's plugin says its main file is found by,
// where the plugin reads one. A file the plugin can't read, or one too
// large to send, leaves the thing found by its title. An error is the
// plugin not being there to ask (reader is nil when none can be reached).
func (w *Workspace) readRemote(ctx context.Context, reader RemoteReader, k plugin.KeptRef, t space.Thing) (map[string]string, error) {
	title := map[string]string{k.Typed(protocol.FieldTitle): t.Title}
	if !k.Remote.Reads {
		return title, nil
	}
	content, err := space.MainFile(ctx, w.WS(), t)
	if err != nil || len(content) > protocol.RemoteReadMax {
		return title, nil
	}
	if reader == nil {
		return nil, errors.New("could not reach plugins")
	}
	fields, err := reader.ReadRemote(ctx, k.Plugin, protocol.RemoteReadParams{Kind: t.Kind, Name: t.Name(), Title: t.Title, File: t.Main, Content: content})
	switch {
	case errors.Is(err, plugin.ErrNoRead), errors.Is(err, plugin.ErrCantRead):
		return title, nil
	case err != nil:
		return nil, err
	}
	out := map[string]string{}
	maps.Copy(out, fields)
	out[k.Typed(protocol.FieldTitle)] = t.Title // its manifest's, whatever the plugin said
	return out, nil
}

// remoteThing is a thing in a space as a plugin's told of it.
func remoteThing(t space.Thing) protocol.RemoteThing {
	out := protocol.RemoteThing{Kind: t.Kind, Name: t.Name(), Repo: t.Repo, Path: t.Path, Title: t.Title, Main: t.Main, Picture: t.Picture, Version: t.Version(), ChangedBy: t.ChangedBy}
	out.Changed, _ = time.Parse(time.RFC3339, t.Changed)
	return out
}

// placer says where a thing is among its kind's things, for its plugin to
// list them as they're organised: what ws calls its repo, and the folders
// between the place its kind's things go and its own (thingGroupIn). The
// places are read once, for however many things.
func (w *Workspace) placer(k plugin.KeptRef) func(protocol.RemoteThing) protocol.RemoteThing {
	places, _ := w.thingPlaces(k)
	return func(t protocol.RemoteThing) protocol.RemoteThing {
		t.RepoLabel = w.WS().DisplayNameFor(t.Repo)
		t.Group = thingGroupIn(places, t.Repo, t.Path)
		return t
	}
}

// thingGroupIn is the folders a thing is grouped in: those between the
// place it's in (of places, the one that's nearest above it in its repo)
// and its own folder, as cross-stack or plans/q4, and "" for one straight
// in its place. One in none of the places is grouped by the folders it's
// in from its repo's top.
func thingGroupIn(places []thingPlace, repo, thingPath string) string {
	own := strings.TrimSuffix(thingPath, "/")
	above := func(rel string) string {
		if dir := path.Dir(rel); dir != "." {
			return dir
		}
		return ""
	}
	nearest, group := -1, above(own)
	for _, p := range places {
		if p.repo != repo {
			continue
		}
		base, rel := p.base(), own
		if base != "" {
			var in bool
			if rel, in = strings.CutPrefix(own, base+"/"); !in {
				continue
			}
		}
		if len(base) > nearest {
			nearest, group = len(base), above(rel)
		}
	}
	return group
}

// pluginHasRemote is whether the plugin who has the kind, with things
// that can live in a repo: it's asked of nothing else.
func (w *Workspace) pluginHasRemote(who, kind string) bool {
	kinds, err := remoteKinds(w.WS().Root)
	if err != nil {
		return false
	}
	k, ok := kinds[kind]
	return ok && k.Plugin == who
}

// pluginRemotes answers remote/list: the things of a kind of who's that
// are in the workspace's spaces, read fresh.
func (w *Workspace) pluginRemotes(ctx context.Context, who, kind string) (protocol.RemoteListResult, error) {
	out := protocol.RemoteListResult{Things: []protocol.RemoteThing{}}
	if !w.pluginHasRemote(who, kind) {
		return out, plugin.InvalidParams(fmt.Errorf("%s has no kind %q that can live in a repo", who, kind))
	}
	db, _, err := w.Spaces(ctx)
	if db == nil {
		return out, err
	}
	things, err := space.Things(ctx, db, kind)
	if err != nil {
		return out, err
	}
	kinds, _ := remoteKinds(w.WS().Root)
	placed := w.placer(kinds[kind])
	for _, t := range things {
		out.Things = append(out.Things, placed(remoteThing(t)))
	}
	return out, nil
}

// pluginRemote answers remote/open: one of who's things in a space, by
// name, with its main file.
func (w *Workspace) pluginRemote(ctx context.Context, who, kind, name string) (protocol.RemoteOpened, error) {
	var out protocol.RemoteOpened
	if !w.pluginHasRemote(who, kind) {
		return out, plugin.InvalidParams(fmt.Errorf("%s has no kind %q that can live in a repo", who, kind))
	}
	none := plugin.NotFound("no " + kind + " at " + name)
	r, ok := ref.Parse(name)
	if !ok || r.Kind != ref.Dir || r.InCapsule() {
		return out, none
	}
	db, _, err := w.Spaces(ctx)
	if db == nil {
		return out, err
	}
	t, ok, err := space.ThingAt(ctx, db, r.Repo, r.Path)
	if err != nil {
		return out, err
	}
	if !ok || t.Kind != kind {
		return out, none
	}
	content, err := space.MainFile(ctx, w.WS(), t)
	if err != nil {
		return out, err
	}
	kinds, _ := remoteKinds(w.WS().Root)
	return protocol.RemoteOpened{RemoteThing: w.placer(kinds[kind])(remoteThing(t)), Content: content}, nil
}
