package space

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/git"
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
// a cache, keyed by what's in each folder, that can be thrown away.
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
	// Tree is its folder, as git names it: the same while every file in it
	// is, whichever of them is the thing.
	Tree string
	// Changed is when anything in it last changed (RFC 3339), and
	// ChangedBy who by.
	Changed, ChangedBy string
}

// Name is the thing's name: its folder, as ws names one
// (records/sketches/release-flow/).
func (t Thing) Name() string {
	return ref.Ref{Kind: ref.Dir, Repo: t.Repo, Worktree: ref.Ground, Path: strings.TrimSuffix(t.Path, "/")}.Name()
}

// Version is what the thing is made of: its folder, as git names it, so
// another when any file in it changes. A kind whose thing is one file
// with what's made from it beside it, and one whose thing is every file
// there, are versioned the same.
func (t Thing) Version() string { return t.Tree }

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

// Manifest is a thing.toml as ws writes one: the thing's kind, what it's
// called, its main file's name and its picture's ("" for none).
func Manifest(kind, title, main, picture string) ([]byte, error) {
	var b bytes.Buffer
	enc := toml.NewEncoder(&b)
	enc.Indent = ""
	m := struct {
		Kind    string `toml:"kind"`
		Title   string `toml:"title"`
		Main    string `toml:"main"`
		Picture string `toml:"picture,omitempty"`
	}{kind, strings.Join(strings.Fields(title), " "), main, picture}
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	if _, ok := readManifest(b.Bytes()); !ok {
		return nil, fmt.Errorf("a thing.toml of kind %q whose main file is %q isn't one ws would read", kind, main)
	}
	return b.Bytes(), nil
}

// ReadManifest is what a thing.toml says, as ws reads one: ok is false
// for one that names no kind or no main file in its folder.
func ReadManifest(text []byte) (kind, title, main, picture string, ok bool) {
	m, ok := readManifest(text)
	return m.Kind, m.Title, m.Main, m.Picture, ok
}

// BlobID is git's id for a file's content: what a thing's Blob is, for
// its main file.
func BlobID(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// refreshThings reads the things on a space's ground into the records:
// tree is every file under the space's folder at head, by path, with its
// content's id. A thing whose folder is as it was isn't read again. It
// reports how many were read, and how many are gone.
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
	folders := make([]string, len(manifests))
	for i, p := range manifests {
		folders[i] = path.Dir(p)
	}
	trees, err := folderTrees(ctx, ground, head, folders)
	if err != nil {
		return 0, 0, err
	}
	// Each folder's files, to say when it last changed.
	within := map[string][]string{}
	for p := range tree {
		for dir := path.Dir(p); dir != "."; dir = path.Dir(dir) {
			if _, ok := trees[dir]; ok {
				within[dir+"/"] = append(within[dir+"/"], p)
			}
		}
	}
	var now, changed []Thing
	var dated []entry
	for _, p := range manifests {
		folder := path.Dir(p) + "/"
		was, ok := had[folder]
		t := Thing{Repo: s.Repo, Path: folder, Manifest: tree[p], Tree: trees[path.Dir(p)]}
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
		for _, f := range within[folder] {
			dated = append(dated, entry{path: f})
		}
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
		// The latest of its files' last changes.
		for _, p := range within[t.Path] {
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
			if _, err := tx.Exec(`INSERT INTO space_things (repo, path, kind, title, main, picture, manifest, blob, tree, changed, changed_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (repo, path) DO UPDATE SET kind = excluded.kind, title = excluded.title, main = excluded.main, picture = excluded.picture,
					manifest = excluded.manifest, blob = excluded.blob, tree = excluded.tree, changed = excluded.changed, changed_by = excluded.changed_by`,
				t.Repo, t.Path, t.Kind, t.Title, t.Main, t.Picture, t.Manifest, t.Blob, t.Tree, t.Changed, t.ChangedBy); err != nil {
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
	rows, err := tx.Query(`SELECT repo, path, kind, title, main, picture, manifest, blob, tree, changed, changed_by FROM space_things `+where+` ORDER BY repo, path`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Thing
	for rows.Next() {
		var t Thing
		if err := rows.Scan(&t.Repo, &t.Path, &t.Kind, &t.Title, &t.Main, &t.Picture, &t.Manifest, &t.Blob, &t.Tree, &t.Changed, &t.ChangedBy); err != nil {
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

// folderTrees is git's id for each of folders at head, by folder: one
// that isn't there has none.
func folderTrees(ctx context.Context, ground, head string, folders []string) (map[string]string, error) {
	out := map[string]string{}
	if len(folders) == 0 {
		return out, nil
	}
	args := []string{"rev-parse"}
	for _, f := range folders {
		args = append(args, head+":"+f)
	}
	ids, err := git.Cmd{Ctx: ctx, Dir: ground, Args: args, ReadOnly: true}.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(ids)), "\n")
	if len(lines) != len(folders) {
		return nil, fmt.Errorf("git named %d of %d folders", len(lines), len(folders))
	}
	for i, f := range folders {
		out[f] = strings.TrimSpace(lines[i])
	}
	return out, nil
}

// FolderTree is git's id for a folder as a commit has it, in the worktree
// dir: what a thing's Version is, for its folder. "" for one that isn't
// there.
func FolderTree(ctx context.Context, dir, commit, folder string) string {
	out, err := git.Cmd{Ctx: ctx, Dir: dir, Args: []string{"rev-parse", "--verify", "--quiet", commit + ":" + strings.TrimSuffix(folder, "/")}, ReadOnly: true}.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// File is one of a thing's files: its path in the thing's folder, its
// content as git names it, and its size.
type File struct {
	Path, Blob string
	Size       int
}

// Files are a thing's files, as they were when it was read: every file in
// its folder at its Version, whatever the ground has moved to since.
func Files(ctx context.Context, ws *workspace.Workspace, t Thing) ([]File, error) {
	out, err := git.Cmd{Ctx: ctx, Dir: ws.MainWorktree(t.Repo), Args: []string{"ls-tree", "-r", "-l", "-z", t.Tree}, ReadOnly: true}.Output()
	if err != nil {
		return nil, err
	}
	var files []File
	for _, rec := range strings.Split(string(out), "\x00") {
		meta, p, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 4 || f[1] != "blob" {
			continue
		}
		size, _ := strconv.Atoi(f[3])
		files = append(files, File{Path: p, Blob: f[2], Size: size})
	}
	return files, nil
}

// ErrNoFile is a thing having no file by a name.
var ErrNoFile = errors.New("it has no such file")

// FileOf is one of a thing's files, by its path in the thing's folder, as
// it was when the thing was read, with git's id for its content.
func FileOf(ctx context.Context, ws *workspace.Workspace, t Thing, name string) ([]byte, string, error) {
	files, err := Files(ctx, ws, t)
	if err != nil {
		return nil, "", err
	}
	for _, f := range files {
		if f.Path == name {
			out, err := blobs(ctx, ws.MainWorktree(t.Repo), []entry{{path: t.Path + name, blob: f.Blob}})
			if err != nil {
				return nil, "", err
			}
			return out[0], f.Blob, nil
		}
	}
	return nil, "", ErrNoFile
}

// ThingsAt are the things in a repo's space whose folder is tree, as git
// names it: the things that are made of the same files.
func ThingsAt(ctx context.Context, db *store.DB, repo, tree string) ([]Thing, error) {
	var out []Thing
	err := db.Read(ctx, func(tx *store.Tx) (err error) {
		out, err = thingsIn(tx, `WHERE repo = ? AND tree = ?`, repo, tree)
		return err
	})
	return out, err
}
