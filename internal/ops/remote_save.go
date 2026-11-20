package ops

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/slug"
	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Putting a plugin's thing in a repo (promoting it), and saving one
// that's there. ws does both, for you: it asks the plugin for the thing
// as its folder is to hold it (remote/export: its title, its main file,
// and what the plugin made from the file), names the folder, writes it
// with a thing.toml, commits it as you, lands it as its repo lands, and
// tells the plugin where it is (remote/saved). The plugin's own UI asks
// for it (remote/promote, remote/save), and so can anything of ws's.
//
// A save is made from a version: the plugin says which version of the
// thing in its repo it last had. Where the thing has changed there since
// (someone else saved it), ws asks the plugin again, with what's there
// now, and the plugin brings the two together however its kind merges.
// What's written is then the whole folder, as the plugin gave it.

// thingKind is the kind of who's whose things ws puts in a repo.
func (w *Workspace) thingKind(who, kind string) (plugin.KeptRef, error) {
	kinds, err := remoteKinds(w.WS().Root)
	if err != nil {
		return plugin.KeptRef{}, err
	}
	k, ok := kinds[kind]
	switch {
	case !ok || k.Plugin != who:
		return k, &Invalid{Reason: fmt.Sprintf("%s has no kind %q whose things live in a repo", who, kind)}
	case !k.Remote.Exports:
		return k, &Invalid{Reason: fmt.Sprintf("%s doesn't give its %s for a repo: there's nothing for ws to put there", who, kind)}
	}
	return k, nil
}

// thingPlace is somewhere a kind's things go: a repo, and the folder one
// is put in there, with protocol.RemoteSlug where its slug goes.
type thingPlace struct{ repo, folder string }

// named is the place's folder for a thing of that slug.
func (p thingPlace) named(slugName string) string {
	return strings.TrimSuffix(strings.ReplaceAll(p.folder, protocol.RemoteSlug, slugName), "/")
}

// thingPlaces are the places one of a kind's things may be put, the first
// where it goes unless you say: those [things.<kind>] names in ws.toml,
// or, with none, the folder the kind's plugin names for it, in the
// records repo and then in each of the workspace's other spaces. Problems
// are the places ws.toml names that ws can't use, and why.
func (w *Workspace) thingPlaces(k plugin.KeptRef) (places []thingPlace, problems []string) {
	if said := w.Config().Things[k.Kind].Paths; len(said) > 0 {
		for _, path := range said {
			repo, folder, err := w.homeNamed(strings.ReplaceAll(path, protocol.RemoteSlug, "slug"))
			if err != nil {
				problems = append(problems, fmt.Sprintf("%q: %v", path, err))
				continue
			}
			_, tail, _ := strings.Cut(path, ":")
			last := path[strings.LastIndex(strings.TrimSuffix(path, "/"), "/")+1:]
			switch {
			case !strings.HasSuffix(path, "/") || !strings.Contains(last, protocol.RemoteSlug) || strings.Count(tail, protocol.RemoteSlug) != 1:
				problems = append(problems, fmt.Sprintf("%q is a folder for each one: it ends in /, with %s once, in its last part (records:sketches/%s/)", path, protocol.RemoteSlug, protocol.RemoteSlug))
				continue
			}
			if err := w.inSpace(repo, folder); err != nil {
				problems = append(problems, fmt.Sprintf("%q: %v", path, err))
				continue
			}
			places = append(places, thingPlace{repo: repo, folder: strings.Trim(tail, "/")})
		}
		return places, problems
	}
	spaces := w.WS().Spaces()
	// The records repo first: where what's written of the work goes.
	slices.SortStableFunc(spaces, func(a, b workspace.Space) int {
		return cmp.Compare(btoi(a.Repo != w.WS().Records), btoi(b.Repo != w.WS().Records))
	})
	for _, s := range spaces {
		places = append(places, thingPlace{repo: s.Repo, folder: strings.Trim(s.Folder+k.Remote.Path, "/")})
	}
	return places, nil
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// inSpace is a folder of a repo as ws would find a thing in it: the repo
// is a space of documents, and the folder's in it.
func (w *Workspace) inSpace(repo, folder string) error {
	s, ok := w.WS().SpaceOf(repo)
	switch {
	case !ok:
		return &Invalid{Reason: fmt.Sprintf("%s isn't a space of documents, so ws wouldn't find it there", repo)}
	case !strings.HasPrefix(folder+"/", s.Folder):
		return &Invalid{Reason: fmt.Sprintf("%s:%s/ isn't under %s, where %s's documents are, so ws wouldn't find it there", repo, folder, s.Folder, repo)}
	}
	return nil
}

// writeThing writes a thing's folder as a plugin gave it: its main file,
// what's made from it, each by its name there, and its thing.toml, in
// place of whatever was in the folder.
func writeThing(dir, kind, main, slugName string, pkg protocol.RemotePackage) error {
	picture := ""
	names := map[string]bool{strings.ToLower(main): true, strings.ToLower(protocol.ManifestName): true}
	type file struct {
		name    string
		content []byte
	}
	files := []file{{main, pkg.Main}}
	for _, f := range pkg.Files {
		name := strings.ReplaceAll(f.Name, protocol.RemoteSlug, slugName)
		if names[strings.ToLower(name)] {
			return &Invalid{Reason: fmt.Sprintf("its file %s is there already: the main file, ws's own, or another of that name", name)}
		}
		names[strings.ToLower(name)] = true
		files = append(files, file{name, f.Content})
		if f.Picture {
			picture = name
		}
	}
	manifest, err := space.Manifest(kind, pkg.Title, main, picture)
	if err != nil {
		return err
	}
	files = append(files, file{protocol.ManifestName, manifest})
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), f.content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// thingGiven is a thing as a plugin gave it, checked for what ws writes.
func thingGiven(who string, given protocol.RemoteExported, err error) (protocol.RemoteExported, error) {
	switch {
	case errors.Is(err, plugin.ErrNoThing):
		return given, noThingAt(err.Error())
	case errors.Is(err, plugin.ErrNoExport):
		return given, &Invalid{Reason: err.Error()}
	case err != nil:
		return given, &Upstream{Err: err}
	}
	given.Title = strings.Join(strings.Fields(given.Title), " ")
	if problem := given.RemotePackage.Problem(); problem != "" {
		return given, &Invalid{Reason: who + " gave what ws can't put in a repo: " + problem}
	}
	return given, nil
}

// noThingAt is there being nothing where it was looked for: ErrNotFound,
// saying what.
func noThingAt(what string) error { return fmt.Errorf("%s: %w", what, ErrNotFound) }

// errAsItWas is a save that had nothing to change: the folder's as the
// plugin gave it already.
var errAsItWas = errors.New("it's as it was")

// thingPut is what's come of putting a thing's folder in its repo: the
// thing as it is there, and what to say of it.
func (w *Workspace) thingPut(ctx context.Context, remotes RemoteReader, k plugin.KeptRef, name, repo, folder string, written protocol.RemoteThing, saved *DocSaved, changed, merged bool) (protocol.RemoteSaved, error) {
	out := protocol.RemoteSaved{Thing: written, Changed: changed, Merged: merged}
	if saved != nil {
		out.Said = saved.Message
		if !w.WS().Direct(repo) {
			out.Capsule, _, _ = strings.Cut(saved.Ref, ":")
		}
	}
	// As ws finds it there, once it's on the default branch.
	if db, _, _ := w.Spaces(ctx); db != nil && out.Capsule == "" {
		if t, ok, _ := space.ThingAt(ctx, db, repo, folder); ok {
			out.Thing = remoteThing(t)
		}
	}
	// The plugin's told first: it's in the repo whatever comes next, and
	// the plugin has to know, or it would be promoted again.
	if err := remotes.SavedRemote(ctx, k.Plugin, protocol.RemoteSavedParams{Kind: k.Kind, Ref: name, Thing: out.Thing}); err != nil {
		return out, &Upstream{Err: fmt.Errorf("it's in %s:%s/, but %s couldn't be told so: %v", repo, folder, k.Plugin, err)}
	}
	if err := plugin.KeepAlias(w.WS().Root, k.Plugin, k.Kind, name, out.Thing.Name); err != nil {
		return out, fmt.Errorf("it's in %s:%s/, but ws couldn't keep that %s is it now: %w", repo, folder, name, err)
	}
	return out, nil
}

// PromoteThing puts one of the plugin who's own things in a repo, for
// you: name is the plugin's name for it, and to where you said it's to
// live (repo:folder/), or "" for where the kind's things go: the
// workspace's records repo, in the folder the kind names for the title's
// slug. It's one commit, landed on the repo's default branch, or left in
// a capsule where the repo takes pull requests.
func (w *Workspace) PromoteThing(ctx context.Context, who, kind, name, to string) (protocol.RemoteSaved, error) {
	var none protocol.RemoteSaved
	k, err := w.thingKind(who, kind)
	if err != nil {
		return none, err
	}
	remotes, err := RemoteReaders(w.WS().Root)
	if err != nil {
		return none, &Upstream{Err: err}
	}
	defer remotes.Close()
	given, err := remotes.ExportRemote(ctx, who, protocol.RemoteExportParams{Kind: kind, Ref: name})
	if given, err = thingGiven(who, given, err); err != nil {
		return none, err
	}
	if given.Name != "" {
		return none, &Invalid{Reason: fmt.Sprintf("%s lives at %s already: save it there", name, given.Name)}
	}
	// Where: as you said, its files named after the folder; or the first
	// of the places the kind's things go, named for its title once the
	// repo's default branch is in hand.
	repo, folder, named := "", "", ""
	var place thingPlace
	if to = strings.TrimSpace(to); to != "" {
		if repo, folder, err = w.homeNamed(to); err != nil {
			return none, err
		}
		named = path.Base(folder)
		if err := w.inSpace(repo, folder); err != nil {
			return none, err
		}
	} else {
		places, _ := w.thingPlaces(k)
		if len(places) == 0 {
			return none, &Invalid{Reason: fmt.Sprintf("the workspace has no space of documents to put a %s in: say where it's to live, as repo:folder/", kind)}
		}
		place = places[0]
		repo = place.repo
	}
	var written protocol.RemoteThing
	saved, err := w.inLift(repo, "promote-"+slug.Make(given.Title, 40, kind), "Promoted", func(capsule string) (string, string, string, bool, error) {
		if folder == "" {
			named, folder = place.free(capsule, given.Title, kind)
		} else if _, err := os.Lstat(filepath.Join(capsule, filepath.FromSlash(folder))); err == nil {
			return "", "", "", false, &Invalid{Reason: fmt.Sprintf("there's something at %s:%s/ already: name another folder for it to live in", repo, folder)}
		}
		main := strings.ReplaceAll(k.Remote.Main, protocol.RemoteSlug, named)
		if err := writeThing(filepath.Join(capsule, filepath.FromSlash(folder)), kind, main, named, given.RemotePackage); err != nil {
			return "", "", "", false, err
		}
		written = writtenThing(kind, repo, folder, main, named, given.RemotePackage)
		return folder, fmt.Sprintf("Add %s: %s", kind, given.Title), fmt.Sprintf("Promoted from the %s plugin: its folder holds the %s (%s), what's made from it, and a %s that says what it is.", who, kind, main, protocol.ManifestName), true, nil
	})
	if err != nil {
		return none, err
	}
	return w.thingPut(ctx, remotes, k, name, repo, folder, written, saved, true, false)
}

// writtenThing is a thing as its folder was just written, before ws has
// found it there.
func writtenThing(kind, repo, folder, main, slugName string, pkg protocol.RemotePackage) protocol.RemoteThing {
	t := space.Thing{Repo: repo, Path: folder + "/", Kind: kind, Title: pkg.Title, Main: main, Blob: space.BlobID(pkg.Main)}
	for _, f := range pkg.Files {
		if f.Picture {
			t.Picture = strings.ReplaceAll(f.Name, protocol.RemoteSlug, slugName)
		}
	}
	return remoteThing(t)
}

// SaveThing saves one of the plugin who's own things that's in a repo, as
// you have it now: name is the plugin's name for it. The plugin says
// where it lives and the version there that what it gives was made from.
// Where the thing's changed there since, the plugin's asked again, with
// what's there, to bring the two together. It's one commit, landed, or
// left in a capsule where the repo takes pull requests. Nothing's
// committed for a folder that's as it was.
func (w *Workspace) SaveThing(ctx context.Context, who, kind, name string) (protocol.RemoteSaved, error) {
	var none protocol.RemoteSaved
	k, err := w.thingKind(who, kind)
	if err != nil {
		return none, err
	}
	remotes, err := RemoteReaders(w.WS().Root)
	if err != nil {
		return none, &Upstream{Err: err}
	}
	defer remotes.Close()
	ask := func(current *protocol.RemoteOpened) (protocol.RemoteExported, error) {
		given, err := remotes.ExportRemote(ctx, who, protocol.RemoteExportParams{Kind: kind, Ref: name, Current: current})
		return thingGiven(who, given, err)
	}
	given, err := ask(nil)
	if err != nil {
		return none, err
	}
	if given.Name == "" {
		return none, &Invalid{Reason: name + " isn't in a repo: promote it first"}
	}
	at, ok := ref.Parse(given.Name)
	if !ok || at.Kind != ref.Dir || at.InCapsule() {
		return none, &Invalid{Reason: fmt.Sprintf("%s says %s lives at %s, which isn't a folder in a repo", who, name, given.Name)}
	}
	repo, known := w.WS().ResolveAlias(at.Repo)
	if !known {
		return none, noThingAt(fmt.Sprintf("ws has no %s at %s", kind, given.Name))
	}
	folder := at.Path
	var written protocol.RemoteThing
	merged := false
	saved, err := w.inLift(repo, "save-"+path.Base(folder), "Saved", func(capsule string) (string, string, string, bool, error) {
		dir := filepath.Join(capsule, filepath.FromSlash(folder))
		text, err := os.ReadFile(filepath.Join(dir, protocol.ManifestName))
		if err != nil {
			return "", "", "", false, noThingAt(fmt.Sprintf("ws has no %s at %s", kind, given.Name))
		}
		was, title, main, picture, ok := space.ReadManifest(text)
		if !ok || was != kind {
			return "", "", "", false, noThingAt(fmt.Sprintf("ws has no %s at %s", kind, given.Name))
		}
		there, err := os.ReadFile(filepath.Join(dir, main))
		if err != nil {
			return "", "", "", false, noThingAt(fmt.Sprintf("ws has no %s at %s: its %s isn't there", kind, given.Name, main))
		}
		if now := space.BlobID(there); now != given.Base {
			// It's changed there since the plugin last had it: the plugin
			// brings what's there into what it has.
			current := remoteThing(space.Thing{Repo: repo, Path: folder + "/", Kind: kind, Title: title, Main: main, Picture: picture, Blob: now})
			if given, err = ask(&protocol.RemoteOpened{RemoteThing: current, Content: there}); err != nil {
				return "", "", "", false, err
			}
			if given.Base != now {
				return "", "", "", false, &Upstream{Err: fmt.Errorf("%s was given %s as it is in %s now, and what it gave back wasn't made from that", who, name, repo)}
			}
			merged = true
		}
		named := path.Base(folder)
		if err := writeThing(dir, kind, main, named, given.RemotePackage); err != nil {
			return "", "", "", false, err
		}
		written = writtenThing(kind, repo, folder, main, named, given.RemotePackage)
		if out, err := git.Combined(capsule, "status", "--porcelain", "--", filepath.FromSlash(folder)); err == nil && strings.TrimSpace(out) == "" {
			return "", "", "", false, errAsItWas
		}
		return folder, fmt.Sprintf("Amend %s %s", kind, named), fmt.Sprintf("Saved from the %s plugin.", who), true, nil
	})
	switch {
	case errors.Is(err, errAsItWas):
		return w.thingPut(ctx, remotes, k, name, repo, folder, written, nil, false, merged)
	case err != nil:
		return none, err
	}
	return w.thingPut(ctx, remotes, k, name, repo, folder, written, saved, true, merged)
}

// free is the first folder of the place that nothing's in, for a thing
// called title, as the repo stands in worktree: named for the title's
// slug, with -2 and so on where that's taken.
func (p thingPlace) free(worktree, title, kind string) (slugName, folder string) {
	base := slug.Make(strings.Join(strings.Fields(title), " "), 60, kind)
	for n := 1; ; n++ {
		slugName = base
		if n > 1 {
			slugName = fmt.Sprintf("%s-%d", base, n)
		}
		folder = p.named(slugName)
		if _, err := os.Lstat(filepath.Join(worktree, filepath.FromSlash(folder))); err != nil {
			return slugName, folder
		}
	}
}

// ThingOffer is somewhere a plugin's thing could go if it were promoted
// now: for whatever asks you before it's done.
type ThingOffer struct {
	// Where is the folder, as it's said to people and as PromoteThing
	// takes it: records:sketches/release-flow/.
	Where string
	// Repo is the repo it's in, Label what ws calls the repo, and Direct
	// whether a commit lands there straight away: where it doesn't, the
	// thing is left in a capsule to open a pull request from.
	Repo, Label string
	Direct      bool
}

// ThingOffered is where one of the plugin who's things of a kind, called
// title, could go: each of the places the kind's things go (thingPlaces),
// the first where it goes unless you say, with the folder it would have
// there, the first that's free as the repo's ground stands. It's what's
// offered: the folder's named for good only when it's promoted, against
// the repo as it is on its remote then.
func (w *Workspace) ThingOffered(who, kind, title string) ([]ThingOffer, error) {
	k, err := w.thingKind(who, kind)
	if err != nil {
		return nil, err
	}
	places, _ := w.thingPlaces(k)
	out := make([]ThingOffer, 0, len(places))
	for _, p := range places {
		_, folder := p.free(w.WS().MainWorktree(p.repo), title, kind)
		out = append(out, ThingOffer{Where: p.repo + ":" + folder + "/", Repo: p.repo, Label: w.WS().DisplayNameFor(p.repo), Direct: w.WS().Direct(p.repo)})
	}
	return out, nil
}

// ThingPlaceProblems are the places ws.toml names for the plugins' kinds
// of thing that ws can't put one in, each with why: for ws doctor.
func (w *Workspace) ThingPlaceProblems() []string {
	kinds, err := remoteKinds(w.WS().Root)
	if err != nil {
		return nil
	}
	var out []string
	for name := range w.Config().Things {
		k, ok := kinds[name]
		if !ok {
			out = append(out, fmt.Sprintf("[things.%s]: no plugin here has a kind %q whose things live in a repo", name, name))
			continue
		}
		_, problems := w.thingPlaces(k)
		for _, p := range problems {
			out = append(out, fmt.Sprintf("[things.%s] %s", name, p))
		}
	}
	slices.Sort(out)
	return out
}
