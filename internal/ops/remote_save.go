package ops

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
		return k, &Invalid{Reason: fmt.Sprintf("%s has no kind %q that can live in a repo", who, kind)}
	case !k.Remote.Exports:
		return k, &Invalid{Reason: fmt.Sprintf("%s does not export %s to a repo", who, kind)}
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

// said is the place as ws.toml says one: records:sketches/{slug}/.
func (p thingPlace) said() string { return p.repo + ":" + p.folder + "/" }

// base is the folder the place's things are in, each in a folder of its
// own there: sketches, or "" for the repo's top.
func (p thingPlace) base() string {
	if dir := path.Dir(p.folder); dir != "." {
		return dir
	}
	return ""
}

// under is the place with a folder between it and each thing's own, to
// group them: records:sketches/cross-stack/{slug}/.
func (p thingPlace) under(group string) thingPlace {
	return thingPlace{repo: p.repo, folder: path.Join(p.base(), group, path.Base(p.folder))}
}

// groupPart is a folder's name, as one a thing's grouped in may be
// called: plain, and nothing that leads out of where it is.
var groupPart = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// thingGroup is a folder to group a thing in, as it was said, checked:
// plain names with / between them (cross-stack, plans/q4), "" for none.
func thingGroup(said string) (string, error) {
	said = strings.Trim(strings.TrimSpace(said), "/")
	if said == "" {
		return "", nil
	}
	parts := strings.Split(said, "/")
	for _, part := range parts {
		if !groupPart.MatchString(part) {
			return "", &Invalid{Reason: fmt.Sprintf("a folder is plain names with / between them (cross-stack, plans/q4): %q is not", said)}
		}
	}
	if len(parts) > 6 {
		return "", &Invalid{Reason: fmt.Sprintf("%q is %d folders deep: 6 at most", said, len(parts))}
	}
	return strings.Join(parts, "/"), nil
}

// insideThing is the thing a folder would be inside, as the repo stands in
// worktree: the nearest folder above it with a thing.toml. A thing's
// folder is written whole when it's saved, so nothing else can be in it.
func insideThing(worktree, folder string) (string, bool) {
	for dir := path.Dir(folder); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if _, err := os.Stat(filepath.Join(worktree, filepath.FromSlash(dir), protocol.ManifestName)); err == nil {
			return dir, true
		}
	}
	return "", false
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
		return &Invalid{Reason: fmt.Sprintf("%s is not a space of documents", repo)}
	case !strings.HasPrefix(folder+"/", s.Folder):
		return &Invalid{Reason: fmt.Sprintf("%s:%s/ is not under %s, where %s's documents are", repo, folder, s.Folder, repo)}
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
			return &Invalid{Reason: fmt.Sprintf("file %s is already there: the main file, the manifest, or another of that name", name)}
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
		return given, &Invalid{Reason: who + " gave something that cannot go in a repo: " + problem}
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
	placed := w.placer(k)
	out := protocol.RemoteSaved{Thing: placed(written), Changed: changed, Merged: merged}
	if saved != nil {
		out.Said = saved.Message
		if !w.WS().Direct(repo) {
			out.Capsule, _, _ = strings.Cut(saved.Ref, ":")
		}
	}
	// As ws finds it there, once it's on the default branch.
	if db, _, _ := w.Spaces(ctx); db != nil && out.Capsule == "" {
		if t, ok, _ := space.ThingAt(ctx, db, repo, folder); ok {
			out.Thing = placed(remoteThing(t))
		}
	}
	// The plugin's told first: it's in the repo whatever comes next, and
	// the plugin has to know, or it would be promoted again.
	if err := remotes.SavedRemote(ctx, k.Plugin, protocol.RemoteSavedParams{Kind: k.Kind, Ref: name, Thing: out.Thing}); err != nil {
		return out, &Upstream{Err: fmt.Errorf("saved to %s:%s/, but could not tell %s: %v", repo, folder, k.Plugin, err)}
	}
	if err := plugin.KeepAlias(w.WS().Root, k.Plugin, k.Kind, name, out.Thing.Name); err != nil {
		return out, fmt.Errorf("saved to %s:%s/, but could not link %s to it: %w", repo, folder, name, err)
	}
	return out, nil
}

// ThingWhere is where a thing's to live when it's promoted.
type ThingWhere struct {
	// To is exactly where, as repo:folder/: the folder's the thing's own.
	// "" for one of the places its kind's things go.
	To string
	// Place is which of those places, as ThingOffer.Place says it
	// (records:sketches/{slug}/): "" for the first. Under is a folder of
	// the place to group it in (cross-stack, plans/q4), where the kind's
	// things may be grouped ([things.<kind>] subfolders): "" for straight
	// in the place. Its own folder is named for its title there, as the
	// repo stands when it's promoted.
	Place, Under string
}

// PromoteThing puts one of the plugin who's own things in a repo, for
// you: name is the plugin's name for it, and where says where it's to
// live: a folder you named, or one of the places the kind's things go
// (the first, with nothing said: the workspace's records repo, in the
// folder the kind names for the title's slug, unless ws.toml says
// otherwise), in a folder of it where you said one. It's one commit,
// landed on the repo's default branch, or left in a capsule where the
// repo takes pull requests.
func (w *Workspace) PromoteThing(ctx context.Context, who, kind, name string, where ThingWhere) (protocol.RemoteSaved, error) {
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
		return none, &Invalid{Reason: fmt.Sprintf("%s already lives at %s: save it there", name, given.Name)}
	}
	// Where: as you said, its files named after the folder; or the first
	// of the places the kind's things go, named for its title once the
	// repo's default branch is in hand.
	repo, folder, named := "", "", ""
	var place thingPlace
	if to := strings.TrimSpace(where.To); to != "" {
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
			return none, &Invalid{Reason: fmt.Sprintf("no space of documents for a %s: say where it goes, as repo:folder/", kind)}
		}
		place = places[0]
		if said := strings.TrimSpace(where.Place); said != "" {
			i := slices.IndexFunc(places, func(p thingPlace) bool { return p.said() == said })
			if i < 0 {
				return none, &Invalid{Reason: fmt.Sprintf("%s is not a place for a %s: ws doctor lists them", said, kind)}
			}
			place = places[i]
		}
		group, err := thingGroup(where.Under)
		if err != nil {
			return none, err
		}
		if group != "" {
			if !w.Config().Things[kind].Subfolders {
				return none, &Invalid{Reason: fmt.Sprintf("a %s goes straight in %s: set [things.%s] subfolders = true in ws.toml to allow folders", kind, place.repo+":"+place.base()+"/", kind)}
			}
			place = place.under(group)
		}
		repo = place.repo
	}
	var written protocol.RemoteThing
	saved, err := w.inLift(repo, "promote-"+slug.Make(given.Title, 40, kind), "Promoted", func(capsule string) (string, string, string, bool, error) {
		if folder == "" {
			named, folder = place.free(capsule, given.Title, kind)
		} else if _, err := os.Lstat(filepath.Join(capsule, filepath.FromSlash(folder))); err == nil {
			return "", "", "", false, &Invalid{Reason: fmt.Sprintf("%s:%s/ is already taken: name another folder", repo, folder)}
		}
		if outer, inside := insideThing(capsule, folder); inside {
			return "", "", "", false, &Invalid{Reason: fmt.Sprintf("%s:%s/ is a thing already, and things cannot nest: name a folder beside it", repo, outer)}
		}
		main := strings.ReplaceAll(k.Remote.Main, protocol.RemoteSlug, named)
		if err := writeThing(filepath.Join(capsule, filepath.FromSlash(folder)), kind, main, named, given.RemotePackage); err != nil {
			return "", "", "", false, err
		}
		if written, err = writtenThing(capsule, kind, repo, folder, main, named, given.RemotePackage); err != nil {
			return "", "", "", false, err
		}
		return folder, fmt.Sprintf("Add %s: %s", kind, given.Title), fmt.Sprintf("Promoted from the %s plugin: its folder holds the %s (%s), what's made from it, and a %s that says what it is.", who, kind, main, protocol.ManifestName), true, nil
	})
	if err != nil {
		return none, err
	}
	return w.thingPut(ctx, remotes, k, name, repo, folder, written, saved, true, false)
}

// writtenThing is a thing as its folder was just written in a capsule,
// before it's committed and before ws has found it there: its version is
// its folder's as the commit will have it, which staging it says.
func writtenThing(capsule, kind, repo, folder, main, slugName string, pkg protocol.RemotePackage) (protocol.RemoteThing, error) {
	t := space.Thing{Repo: repo, Path: folder + "/", Kind: kind, Title: pkg.Title, Main: main, Blob: space.BlobID(pkg.Main)}
	for _, f := range pkg.Files {
		if f.Picture {
			t.Picture = strings.ReplaceAll(f.Name, protocol.RemoteSlug, slugName)
		}
	}
	if out, err := git.Combined(capsule, "add", "-A", "--", filepath.FromSlash(folder)); err != nil {
		return protocol.RemoteThing{}, fmt.Errorf("could not stage %s: %s: %w", folder, strings.TrimSpace(out), err)
	}
	tree, err := git.Combined(capsule, "write-tree", "--prefix="+folder+"/")
	if err != nil {
		return protocol.RemoteThing{}, fmt.Errorf("could not read back %s: %s: %w", folder, strings.TrimSpace(tree), err)
	}
	t.Tree = strings.TrimSpace(tree)
	return remoteThing(t), nil
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
		return none, &Invalid{Reason: name + " is not in a repo: promote it first"}
	}
	at, ok := ref.Parse(given.Name)
	if !ok || at.Kind != ref.Dir || at.InCapsule() {
		return none, &Invalid{Reason: fmt.Sprintf("%s says %s lives at %s, which is not a folder in a repo", who, name, given.Name)}
	}
	repo, known := w.WS().ResolveAlias(at.Repo)
	if !known {
		return none, noThingAt(fmt.Sprintf("no %s at %s", kind, given.Name))
	}
	folder := at.Path
	var written protocol.RemoteThing
	merged := false
	saved, err := w.inLift(repo, "save-"+path.Base(folder), "Saved", func(capsule string) (string, string, string, bool, error) {
		dir := filepath.Join(capsule, filepath.FromSlash(folder))
		text, err := os.ReadFile(filepath.Join(dir, protocol.ManifestName))
		if err != nil {
			return "", "", "", false, noThingAt(fmt.Sprintf("no %s at %s", kind, given.Name))
		}
		was, title, main, picture, ok := space.ReadManifest(text)
		if !ok || was != kind {
			return "", "", "", false, noThingAt(fmt.Sprintf("no %s at %s", kind, given.Name))
		}
		there, err := os.ReadFile(filepath.Join(dir, main))
		if err != nil {
			return "", "", "", false, noThingAt(fmt.Sprintf("no %s at %s: %s is missing", kind, given.Name, main))
		}
		if now := space.FolderTree(ctx, capsule, "HEAD", folder); now != given.Base {
			// It's changed there since the plugin last had it: the plugin
			// brings what's there into what it has.
			current := w.placer(k)(remoteThing(space.Thing{Repo: repo, Path: folder + "/", Kind: kind, Title: title, Main: main, Picture: picture, Blob: space.BlobID(there), Tree: now}))
			if given, err = ask(&protocol.RemoteOpened{RemoteThing: current, Content: there}); err != nil {
				return "", "", "", false, err
			}
			if given.Base != now {
				return "", "", "", false, &Upstream{Err: fmt.Errorf("%s was sent the current %s from %s, but did not build on it", who, name, repo)}
			}
			merged = true
		}
		named := path.Base(folder)
		if err := writeThing(dir, kind, main, named, given.RemotePackage); err != nil {
			return "", "", "", false, err
		}
		if written, err = writtenThing(capsule, kind, repo, folder, main, named, given.RemotePackage); err != nil {
			return "", "", "", false, err
		}
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
	// Place is the place it's one of, as ws.toml says one
	// (records:sketches/{slug}/): what PromoteThing takes to put it there.
	Place string
	// Subfolders is whether the kind's things may be grouped there, in a
	// folder of the place, and Groups the folders its things are grouped
	// in now, by name (cross-stack, plans, plans/q4).
	Subfolders bool
	Groups     []string
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
	grouped := w.Config().Things[kind].Subfolders
	var things []space.Thing
	if grouped {
		// As the spaces were last read: it's what's offered, not what's done.
		if db, err := w.Records(); err == nil {
			things, _ = space.Things(context.Background(), db, kind)
		}
	}
	out := make([]ThingOffer, 0, len(places))
	for _, p := range places {
		_, folder := p.free(w.WS().MainWorktree(p.repo), title, kind)
		offer := ThingOffer{Where: p.repo + ":" + folder + "/", Repo: p.repo, Label: w.WS().DisplayNameFor(p.repo), Direct: w.WS().Direct(p.repo), Place: p.said(), Subfolders: grouped}
		if grouped {
			offer.Groups = p.groups(things)
		}
		out = append(out, offer)
	}
	return out, nil
}

// groups are the folders of the place that things are grouped in, by
// name, and the folders those are in: for things of the place's repo.
func (p thingPlace) groups(things []space.Thing) []string {
	base := p.base()
	if base != "" {
		base += "/"
	}
	seen := map[string]bool{}
	for _, t := range things {
		rel, ok := strings.CutPrefix(t.Path, base)
		if t.Repo != p.repo || !ok {
			continue
		}
		// What's above the thing's own folder, and each folder above that.
		for dir := path.Dir(strings.TrimSuffix(rel, "/")); dir != "." && dir != "/"; dir = path.Dir(dir) {
			seen[dir] = true
		}
	}
	out := make([]string, 0, len(seen))
	for g := range seen {
		out = append(out, g)
	}
	slices.Sort(out)
	return out
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
