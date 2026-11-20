package space

import (
	"context"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/workspace"
)

// A space holds things that aren't documents: a folder with a thing.toml
// in it (protocol.ManifestName) is one, its package. The manifest says
// what kind of thing it is, what it's called, and which file in the
// folder is the thing (its main file); the kind is a plugin's, and what
// the main file says is the plugin's to read. What's kept here is what
// the repo says, read from the space's default branch with its documents:
// a cache, keyed by the two files' contents, that can be thrown away.
// What a plugin makes of a main file (what it's found by) is kept with the
// plugin's other things, by ops.

// Thing is a thing in a space: its folder, and what its manifest says.
type Thing struct {
	// Repo is the space's repo, and Path its folder there, ending in /.
	Repo, Path string
	// Kind is what it is, as a plugin registered the kind, and Title what
	// it's called.
	Kind, Title string
	// Main is its main file's name, and Picture its picture's ("" for
	// none): both in its folder.
	Main, Picture string
	// Manifest and Blob are the manifest's content and the main file's,
	// as git names them: the same for the same content.
	Manifest, Blob string
	// Changed is when either last changed (RFC 3339), and ChangedBy who by.
	Changed, ChangedBy string
}

// Name is the thing's name: its folder, as ws names one
// (records/sketches/release-flow/).
func (t Thing) Name() string {
	return ref.Ref{Kind: ref.Dir, Repo: t.Repo, Worktree: ref.Ground, Path: strings.TrimSuffix(t.Path, "/")}.Name()
}

// Version is what the thing is made of: another when its manifest or its
// main file changes.
func (t Thing) Version() string { return t.Manifest + ":" + t.Blob }

// manifest is a thing.toml, as written.
type manifest struct {
	Kind    string `toml:"kind"`
	Title   string `toml:"title"`
	Main    string `toml:"main"`
	Picture string `toml:"picture"`
}

var (
	thingKind = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
	// A file in the package's own folder, by name: nothing that leads out
	// of it.
	thingFile = regexp.MustCompile(`^[^/\\\x00-\x1f]+$`)
)

// readManifest is what a thing.toml says, or ok false for one that names
// no kind or no main file: a folder with that in it isn't a thing.
func readManifest(text []byte) (m manifest, ok bool) {
	if err := toml.Unmarshal(text, &m); err != nil {
		return m, false
	}
	m.Title = strings.Join(strings.Fields(m.Title), " ")
	inFolder := func(name string) bool { return thingFile.MatchString(name) && name != "." && name != ".." }
	if !thingKind.MatchString(m.Kind) || !inFolder(m.Main) || m.Main == protocol.ManifestName {
		return m, false
	}
	if m.Picture != "" && !inFolder(m.Picture) {
		m.Picture = ""
	}
	return m, true
}

// refreshThings reads the things on a space's ground into the records:
// tree is every file under the space's folder at head, by path, with its
// content's id. A thing whose manifest and main file are as they were
// isn't read again. It reports how many were read, and how many are gone.
func refreshThings(ctx context.Context, db *store.DB, ws *workspace.Workspace, s workspace.Space, head string, tree map[string]string) (nread, nremoved int, err error) {
	had := map[string]Thing{}
	if err := db.Read(ctx, func(tx *store.Tx) error {
		things, err := thingsIn(tx, `WHERE repo = ?`, s.Repo)
		for _, t := range things {
			had[t.Path] = t
		}
		return err
	}); err != nil {
		return 0, 0, err
	}
	var manifests []string // each thing.toml's path
	for p := range tree {
		if path.Base(p) == protocol.ManifestName && path.Dir(p) != "." {
			manifests = append(manifests, p)
		}
	}
	slices.Sort(manifests)
	if len(manifests) == 0 && len(had) == 0 {
		return 0, 0, nil
	}
	// A manifest that's changed is read, to say which file is the main one.
	ground := ws.MainWorktree(s.Repo)
	var changedManifests []entry
	for _, p := range manifests {
		if was, ok := had[path.Dir(p)+"/"]; !ok || was.Manifest != tree[p] {
			changedManifests = append(changedManifests, entry{path: p, blob: tree[p]})
		}
	}
	texts, err := blobs(ctx, ground, changedManifests)
	if err != nil {
		return 0, 0, err
	}
	said := map[string]manifest{}
	for i, e := range changedManifests {
		if m, ok := readManifest(texts[i]); ok {
			said[e.path] = m
		}
	}
	var now, changed []Thing
	var dated []entry
	for _, p := range manifests {
		folder := path.Dir(p) + "/"
		was, ok := had[folder]
		t := Thing{Repo: s.Repo, Path: folder, Manifest: tree[p]}
		switch m, read := said[p]; {
		case read:
			t.Kind, t.Title, t.Main, t.Picture = m.Kind, m.Title, m.Main, m.Picture
		case ok && was.Manifest == tree[p]:
			t.Kind, t.Title, t.Main, t.Picture = was.Kind, was.Title, was.Main, was.Picture
		default:
			continue // a thing.toml that says nothing ws can keep
		}
		if t.Blob = tree[folder+t.Main]; t.Blob == "" {
			continue // its main file isn't there
		}
		if t.Title == "" {
			t.Title = path.Base(strings.TrimSuffix(folder, "/"))
		}
		if ok && was.Version() == t.Version() {
			t.Changed, t.ChangedBy = was.Changed, was.ChangedBy
			now = append(now, t)
			continue
		}
		now = append(now, t)
		changed = append(changed, t)
		dated = append(dated, entry{path: p}, entry{path: folder + t.Main})
	}
	keep := map[string]bool{}
	for _, t := range now {
		keep[t.Path] = true
	}
	var gone []string
	for p := range had {
		if !keep[p] {
			gone = append(gone, p)
		}
	}
	if len(changed) == 0 && len(gone) == 0 {
		return 0, 0, nil
	}
	last, err := lastChanged(ctx, ground, head, s.Folder, dated)
	if err != nil {
		return 0, 0, err
	}
	for i := range changed {
		t := &changed[i]
		// The later of its two files' last changes: its main file's, of
		// two at once.
		for _, p := range []string{t.Path + t.Main, t.Path + protocol.ManifestName} {
			if c := last[p]; c.at > t.Changed {
				t.Changed, t.ChangedBy = c.at, c.by
			}
		}
	}
	err = db.Write(ctx, func(tx *store.Tx) error {
		for _, p := range gone {
			if _, err := tx.Exec(`DELETE FROM space_things WHERE repo = ? AND path = ?`, s.Repo, p); err != nil {
				return err
			}
		}
		for _, t := range changed {
			if _, err := tx.Exec(`INSERT INTO space_things (repo, path, kind, title, main, picture, manifest, blob, changed, changed_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (repo, path) DO UPDATE SET kind = excluded.kind, title = excluded.title, main = excluded.main, picture = excluded.picture,
					manifest = excluded.manifest, blob = excluded.blob, changed = excluded.changed, changed_by = excluded.changed_by`,
				t.Repo, t.Path, t.Kind, t.Title, t.Main, t.Picture, t.Manifest, t.Blob, t.Changed, t.ChangedBy); err != nil {
				return err
			}
		}
		tx.Changed("space:" + s.Repo)
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return len(changed), len(gone), nil
}

func thingsIn(tx *store.Tx, where string, args ...any) ([]Thing, error) {
	rows, err := tx.Query(`SELECT repo, path, kind, title, main, picture, manifest, blob, changed, changed_by FROM space_things `+where+` ORDER BY repo, path`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Thing
	for rows.Next() {
		var t Thing
		if err := rows.Scan(&t.Repo, &t.Path, &t.Kind, &t.Title, &t.Main, &t.Picture, &t.Manifest, &t.Blob, &t.Changed, &t.ChangedBy); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Things are the things in the workspace's spaces, as they were last
// read, by repo and folder: of one kind, or of every kind ("").
func Things(ctx context.Context, db *store.DB, kind string) ([]Thing, error) {
	var out []Thing
	err := db.Read(ctx, func(tx *store.Tx) (err error) {
		if kind == "" {
			out, err = thingsIn(tx, ``)
		} else {
			out, err = thingsIn(tx, `WHERE kind = ?`, kind)
		}
		return err
	})
	return out, err
}

// ThingAt is the thing whose folder is path in a space's repo, if there's
// one: path with or without its last /.
func ThingAt(ctx context.Context, db *store.DB, repo, p string) (Thing, bool, error) {
	var out []Thing
	err := db.Read(ctx, func(tx *store.Tx) (err error) {
		out, err = thingsIn(tx, `WHERE repo = ? AND path = ?`, repo, strings.TrimSuffix(p, "/")+"/")
		return err
	})
	if err != nil || len(out) == 0 {
		return Thing{}, false, err
	}
	return out[0], true, nil
}

// MainFile is a thing's main file, as it was when the thing was read: its
// content by git's id for it, so it's what Version says whatever the
// ground has moved to since.
func MainFile(ctx context.Context, ws *workspace.Workspace, t Thing) ([]byte, error) {
	out, err := blobs(ctx, ws.MainWorktree(t.Repo), []entry{{path: t.Path + t.Main, blob: t.Blob}})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}
