package artifact

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// An artifact's folder is its record: artifact.toml, and the repo its
// versions' files are in. The workspace's records hold an index of them, a row each in
// artifacts, for listing and finding them and for their refs: its number
// (a3), where it's kept, its folder's name, its title, status and
// versions, and its artifact.toml as it was last read, as JSON. The row
// outlives the files: a discarded artifact's row still says what it was,
// and its number is never used again.
//
// Where the row and artifact.toml differ, the file wins: a folder whose
// artifact.toml has moved since its row was written (edited by hand, or
// a write whose row didn't commit) is read again when the artifacts are
// next listed, and a folder with no row is given one.
//
// A version's commit is written before the [[versions]] entry that names
// it in artifact.toml, and that before the row: so a version in the repo
// that artifact.toml doesn't name is litter, what a write that didn't
// finish left (Litter), and anything else in the folder belongs to the
// artifact.

func init() {
	store.Define("artifact", store.SQL(`
		CREATE TABLE artifacts (
			id TEXT NOT NULL PRIMARY KEY,   -- a3
			n INTEGER NOT NULL UNIQUE,      -- its number: none is used twice
			place TEXT NOT NULL,            -- where it's kept: '.ws', this machine's (a repo's name, for one from before they all were: BringHome)
			folder TEXT NOT NULL,           -- its folder in .ws/artifacts: {date}-{slug}
			title TEXT NOT NULL,
			status TEXT NOT NULL,           -- live, promoted, discarded
			versions INTEGER NOT NULL,      -- its latest version's number
			last TEXT NOT NULL,             -- when its latest version was added
			size INTEGER NOT NULL,          -- its artifact.toml's size when last read; -1 when its folder had gone
			changed TEXT NOT NULL,          -- that file's time then
			record TEXT NOT NULL,           -- artifact.toml as it was last read (Artifact), as JSON
			UNIQUE (place, folder)
		) WITHOUT ROWID;
		CREATE INDEX artifacts_listed ON artifacts (status, last);
	`), store.SQL(`
		-- The artifacts ws found in the workspace's repos (found.go), each
		-- as it was last read there: what's in the graph of them.
		CREATE TABLE artifacts_found (
			name TEXT NOT NULL PRIMARY KEY,   -- records/artifacts/2026-11-20-horizon-glow/
			repo TEXT NOT NULL, folder TEXT NOT NULL,
			title TEXT NOT NULL,
			version TEXT NOT NULL,            -- its folder, as git names it
			changed TEXT NOT NULL, changed_by TEXT NOT NULL,
			record TEXT NOT NULL              -- its artifact.toml there (Artifact), as JSON: '' for none
		) WITHOUT ROWID;
	`), standingStep)
}

// Local is where an artifact on this machine is kept: .ws/artifacts.
const Local = ".ws"

// ThingKind is what an artifact is in the graph.
const ThingKind = "artifact"

// placeVersion is the kind of place an artifact's words are at: a
// version's note.
const placeVersion = "artifact-version"

// ErrNotFound is the answer for an artifact that isn't there.
var ErrNotFound = errors.New("not found")

// Store keeps a workspace's artifacts: on this machine, in .ws/artifacts.
type Store struct {
	Root string
}

// Dir is where artifacts on this machine are kept.
func (s Store) Dir() string { return filepath.Join(s.Root, ".ws", "artifacts") }

// DiscardedDir is where a discarded artifact's folder is moved.
func (s Store) DiscardedDir() string { return filepath.Join(s.Dir(), ".discarded") }

// Entry is an artifact as the workspace knows it: its ref, where it's
// kept, and its record.
type Entry struct {
	ID string `json:"id"` // a3
	N  int    `json:"n"`
	// Place is where it's kept: Local. (A repo's name, for one kept there
	// before they were all kept here, until it's brought home: BringHome.)
	Place string `json:"place"`
	// Folder is its folder's name in .ws/artifacts
	// (2026-11-12-horizon-glow).
	Folder string `json:"folder"`
	// Gone: its folder isn't where it was, and it wasn't discarded. What's
	// known of it is what the records kept.
	Gone bool `json:"gone,omitempty"`
	// Erased: discarded on this machine, and its files deleted for good,
	// 30 days on (or its folder had gone already). There's nothing to
	// restore, and what's known of it is what the records kept.
	Erased bool `json:"erased,omitempty"`
	Artifact
}

// Path is the artifact's folder: in .ws/artifacts, or in its .discarded
// once it's discarded.
func (s Store) Path(e Entry) string {
	if e.Status == Discarded {
		return filepath.Join(s.DiscardedDir(), e.Folder)
	}
	return filepath.Join(s.Dir(), e.Folder)
}

// Repo is the artifact's repo, where its versions' files are.
func (s Store) Repo(e Entry) Repo { return Repo(filepath.Join(s.Path(e), RepoDir)) }

// Away reports whether it's one from before artifacts were all kept on
// this machine, still where it was: its row names a repo, and its files
// aren't here until it's brought home (BringHome).
func (e Entry) Away() bool { return e.Place != Local && e.Place != "" }

// Where is its folder as it's said to people: .ws/artifacts/<folder>/.
func (e Entry) Where() string {
	if e.Status == Discarded {
		return ".ws/artifacts/.discarded/" + e.Folder + "/"
	}
	return ".ws/artifacts/" + e.Folder + "/"
}

// Lives is where it lives in a repo, as it's said to people
// (records:artifacts/<folder>/): "" for one on this machine only, and
// for one discarded, whose folder there was taken away.
func (e Entry) Lives() string {
	if e.Home == nil || e.Status == Discarded {
		return ""
	}
	return e.Home.Where()
}

// Kept is where it's kept, as ws show says it: this machine, and the
// repo it lives in, promoted.
func (e Entry) Kept() string {
	if e.Home != nil && e.Status != Discarded {
		return "this machine, and " + e.Home.Repo
	}
	return "this machine"
}

func (s Store) records() (*store.DB, error) { return store.Open(s.Root) }

func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

func notFound(id string) error {
	return fmt.Errorf("%w: the workspace has no artifact %s", ErrNotFound, id)
}

// awayError is the answer for a change to an artifact that hasn't been
// brought home.
func awayError(e Entry) error {
	return fmt.Errorf("%s is from when artifacts were kept in %s, and hasn't been brought onto this machine yet: ws doctor does it", e.ID, e.Place)
}

// kept is an artifact's row: what the records hold of it.
type kept struct {
	Entry
	size    int64
	changed string // as store.Stamp writes it
}

// moved reports whether an artifact's record isn't as its row has it.
func (k *kept) moved(info os.FileInfo) bool {
	return info.Size() != k.size || stampOf(info.ModTime()) != k.changed
}

const keptColumns = `id, n, place, folder, size, changed, record`

func scanKept(sc interface{ Scan(...any) error }) (*kept, error) {
	var k kept
	var record string
	if err := sc.Scan(&k.ID, &k.N, &k.Place, &k.Folder, &k.size, &k.changed, &record); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(record), &k.Artifact) // what can't be read is as nothing kept
	if k.Status == "" {
		k.Status = Live
	}
	k.Gone, k.Erased = gone(&k)
	return &k, nil
}

func keptWhere(tx *store.Tx, where string, args ...any) (*kept, error) {
	k, err := scanKept(tx.QueryRow(`SELECT `+keptColumns+` FROM artifacts WHERE `+where, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return k, err
}

// gone is whether an artifact's folder has gone without its being
// discarded, and whether one discarded on this machine has no files left.
func gone(k *kept) (bool, bool) {
	return k.size < 0 && k.Status != Discarded, k.size < 0 && k.Status == Discarded
}

// keptIn is artifact id's row in tx, if it has one.
func keptIn(tx *store.Tx, id string) (*kept, error) { return keptWhere(tx, `id = ?`, id) }

// keptAt is the row of the artifact in folder, where it's kept, if it has
// one.
func keptAt(tx *store.Tx, place, folder string) (*kept, error) {
	return keptWhere(tx, `place = ? AND folder = ?`, place, folder)
}

// allIn is every artifact's row, the latest version's newest first.
func allIn(tx *store.Tx) ([]*kept, error) {
	rows, err := tx.Query(`SELECT ` + keptColumns + ` FROM artifacts ORDER BY last DESC, n DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*kept
	for rows.Next() {
		k, err := scanKept(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// saveIn writes an artifact's row as its record is, with its record's
// file as info has it (nil for one whose folder has gone), in tx.
func saveIn(tx *store.Tx, k *kept, info os.FileInfo) error {
	if info == nil {
		return saveAs(tx, k, -1, k.changed)
	}
	return saveAs(tx, k, info.Size(), stampOf(info.ModTime()))
}

// saveAs writes an artifact's row as its record is, its records' size and
// time as said (size -1: its folder has gone), in tx.
func saveAs(tx *store.Tx, k *kept, size int64, changed string) error {
	k.size, k.changed = size, changed
	k.Gone, k.Erased = gone(k)
	record, err := json.Marshal(k.Artifact)
	if err != nil {
		return err
	}
	latest, _ := k.Latest()
	if _, err := tx.Exec(`INSERT INTO artifacts (id, n, place, folder, title, status, versions, last, size, changed, record) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET title = excluded.title, status = excluded.status, versions = excluded.versions, last = excluded.last,
			size = excluded.size, changed = excluded.changed, record = excluded.record`,
		k.ID, k.N, k.Place, k.Folder, k.Title, string(k.Status), latest.N, stampOf(k.LastAt()), k.size, k.changed, string(record)); err != nil {
		return err
	}
	tx.Changed("workspace", "artifact:"+k.ID)
	return graph(tx, k)
}

// The graph: an artifact is a thing, named the same everywhere (a3), its
// words are its title and its versions' notes, and it points at the
// conversations and missions its versions came from. One discarded, or
// whose folder has gone, is in the graph no more.

func origin(id string) store.Origin { return store.Origin("artifact:" + id) }

func graph(tx *store.Tx, k *kept) error {
	o := origin(k.ID)
	if err := tx.Forget(o); err != nil {
		return err
	}
	if k.Status == Discarded || k.Gone {
		return nil
	}
	me := store.Key{Name: k.ID}
	if err := tx.Thing(o, store.Thing{Key: me, Kind: ThingKind, Title: k.Title, State: string(k.Status), Changed: k.LastAt(), Stands: k.Status.stands(k.LastAt())}); err != nil {
		return err
	}
	linked := map[string]bool{}
	for _, v := range k.Versions {
		heading := "v" + strconv.Itoa(v.N)
		if err := tx.Words(o, store.Place{Thing: me, At: heading, Kind: placeVersion, Title: k.Title, Heading: heading, Text: v.Note, Said: v.At}); err != nil {
			return err
		}
		if v.From != "" && !linked[v.From] {
			linked[v.From] = true
			if err := tx.Link(o, store.Link{Src: me, Dst: FromKey(v.From), Kind: store.From, At: v.At}); err != nil {
				return err
			}
		}
	}
	return nil
}

// FromKey is where a version came from, as the graph names it: a
// mission's conversation (slug/t3) in its mission, and a workspace
// conversation (w9) or a mission (its slug) as they are.
func FromKey(from string) store.Key {
	if scope, id, ok := strings.Cut(from, "/"); ok {
		return store.Key{Scope: scope, Name: id}
	}
	return store.Key{Name: from}
}

// --- Reading

// List is every artifact the workspace knows, the latest version's newest
// first, discarded ones too. Folders that have moved since ws last read
// them are read again first.
func (s Store) List(ctx context.Context) ([]Entry, error) {
	if _, err := s.freshen(ctx); err != nil {
		return nil, err
	}
	db, err := s.records()
	if err != nil {
		return nil, err
	}
	var out []Entry
	err = db.Read(ctx, func(tx *store.Tx) error {
		all, err := allIn(tx)
		for _, k := range all {
			out = append(out, k.Entry)
		}
		return err
	})
	return out, err
}

// Get is the artifact id names (a3), as its folder now is.
func (s Store) Get(ctx context.Context, id string) (Entry, error) {
	if _, err := s.freshen(ctx); err != nil {
		return Entry{}, err
	}
	if e, ok := s.Known(id); ok {
		return e, nil
	}
	return Entry{}, notFound(id)
}

// Known is the artifact id names as the records last had it, without
// looking at its folder: for a view that lists many.
func (s Store) Known(id string) (Entry, bool) {
	db, err := s.records()
	if err != nil {
		return Entry{}, false
	}
	var k *kept
	db.Read(context.Background(), func(tx *store.Tx) error {
		k, err = keptIn(tx, id)
		return err
	})
	if k == nil {
		return Entry{}, false
	}
	return k.Entry, true
}

// IDs is every artifact's ref the records have (a3), discarded ones too,
// without looking in their folders: what there is to name.
func (s Store) IDs() []string {
	db, err := s.records()
	if err != nil {
		return nil
	}
	var out []string
	db.Read(context.Background(), func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT id FROM artifacts ORDER BY n`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				out = append(out, id)
			}
		}
		return rows.Err()
	})
	return out
}

// version is the artifact id names and the version n does (0 for the
// latest), where there are files of it to read.
func (s Store) version(ctx context.Context, id string, n int) (Entry, int, error) {
	e, err := s.Get(ctx, id)
	if err != nil {
		return Entry{}, 0, err
	}
	if n == 0 {
		latest, ok := e.Latest()
		if !ok {
			return e, 0, fmt.Errorf("%s has no versions", id)
		}
		n = latest.N
	}
	if _, ok := e.Version(n); !ok {
		return e, 0, fmt.Errorf("%w: %s has no v%d (%s)", ErrNotFound, id, n, versionsSaid(e.Artifact))
	}
	if e.Gone {
		return e, n, fmt.Errorf("%s's folder has gone from %s: there's no file to read", id, strings.TrimSuffix(e.Where(), path.Base(e.Folder)+"/"))
	}
	if e.Erased {
		return e, n, fmt.Errorf("%s was discarded, and its files are gone for good: there's no file to read", id)
	}
	return e, n, nil
}

// ReadVersion is version n's page (0 for the latest), and which version
// it is.
func (s Store) ReadVersion(ctx context.Context, id string, n int) ([]byte, Entry, int, error) {
	data, _, e, n, err := s.ReadFile(ctx, id, n, IndexFile)
	return data, e, n, err
}

// ReadFile is the file at path in version n (0 for the latest), what
// names its bytes (the same for the same file), and which version it is.
// A file the version hasn't is ErrNoFile.
func (s Store) ReadFile(ctx context.Context, id string, n int, p string) ([]byte, string, Entry, int, error) {
	e, n, err := s.version(ctx, id, n)
	if err != nil {
		return nil, "", e, n, err
	}
	repo := s.Repo(e)
	if !repo.Exists() {
		// From when a version was a file in the folder, and not yet put in
		// a repo (Convert): its page is that file.
		if p != IndexFile {
			return nil, "", e, n, fmt.Errorf("%w: %s has no %s", ErrNoFile, VersionRef(id, n), p)
		}
		old := filepath.Join(s.Path(e), VersionFile(n))
		data, err := os.ReadFile(old)
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", e, n, fmt.Errorf("%s's files aren't there (%s)", VersionRef(id, n), s.Path(e))
		}
		return data, "", e, n, err
	}
	if !repo.Has(ctx, n) {
		return nil, "", e, n, fmt.Errorf("%s's files aren't there (%s)", VersionRef(id, n), string(repo))
	}
	data, blob, err := repo.Read(ctx, n, p)
	if errors.Is(err, ErrNoFile) {
		return nil, "", e, n, fmt.Errorf("%w: %s has no %s", ErrNoFile, VersionRef(id, n), p)
	}
	return data, blob, e, n, err
}

// VersionFiles lists version n's files (0 for the latest), by path, and
// which version it is.
func (s Store) VersionFiles(ctx context.Context, id string, n int) ([]FileInfo, Entry, int, error) {
	e, n, err := s.version(ctx, id, n)
	if err != nil {
		return nil, e, n, err
	}
	repo := s.Repo(e)
	if !repo.Exists() {
		info, err := os.Stat(filepath.Join(s.Path(e), VersionFile(n)))
		if err != nil {
			return nil, e, n, fmt.Errorf("%s's files aren't there (%s)", VersionRef(id, n), s.Path(e))
		}
		return []FileInfo{{Path: IndexFile, Size: int(info.Size())}}, e, n, nil
	}
	list, err := repo.List(ctx, n)
	if err != nil {
		return nil, e, n, fmt.Errorf("%s's files aren't there (%s)", VersionRef(id, n), string(repo))
	}
	return list, e, n, nil
}

// AllFiles is version n's files, whole (0 for the latest), and which
// version it is.
func (s Store) AllFiles(ctx context.Context, id string, n int) (Files, Entry, int, error) {
	e, n, err := s.version(ctx, id, n)
	if err != nil {
		return nil, e, n, err
	}
	repo := s.Repo(e)
	if !repo.Exists() {
		data, _, _, _, err := s.ReadFile(ctx, id, n, IndexFile)
		if err != nil {
			return nil, e, n, err
		}
		return Page(data), e, n, nil
	}
	files, err := repo.All(ctx, n)
	if err != nil || len(files) == 0 {
		return nil, e, n, fmt.Errorf("%s's files aren't there (%s)", VersionRef(id, n), string(repo))
	}
	return files, e, n, nil
}

// Checkout writes version n's files (0 for the latest) into dir, to be
// worked on: what's made of them is added as a new version, whole. dir
// is made, and must be empty if it's there. It's which version it was.
func (s Store) Checkout(ctx context.Context, id string, n int, dir string) (Entry, int, []FileInfo, error) {
	files, e, n, err := s.AllFiles(ctx, id, n)
	if err != nil {
		return e, n, nil, err
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return e, n, nil, fmt.Errorf("%s has files in it already: name an empty folder, or one that isn't there", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return e, n, nil, err
	}
	if err := files.WriteFolder(dir); err != nil {
		return e, n, nil, err
	}
	return e, n, files.Infos(), nil
}

// VersionSize is how many files version n has and what they come to, and
// false where there are none to read: its folder's gone, or they're
// missing.
func (s Store) VersionSize(e Entry, n int) (files, size int, ok bool) {
	if e.Gone || e.Erased {
		return 0, 0, false
	}
	if v, has := e.Version(n); has && v.Size > 0 {
		return max(v.Files, 1), v.Size, true
	}
	repo := s.Repo(e)
	if !repo.Exists() {
		info, err := os.Stat(filepath.Join(s.Path(e), VersionFile(n)))
		if err != nil {
			return 0, 0, false
		}
		return 1, int(info.Size()), true
	}
	list, err := repo.List(context.Background(), n)
	if err != nil || len(list) == 0 {
		return 0, 0, false
	}
	for _, f := range list {
		size += f.Size
	}
	return len(list), size, true
}

// versionsSaid lists an artifact's versions: v1, v2 and v3.
func versionsSaid(a Artifact) string {
	if len(a.Versions) == 0 {
		return "it has none"
	}
	var vs []string
	for _, v := range a.Versions {
		vs = append(vs, "v"+strconv.Itoa(v.N))
	}
	return "its versions are " + strings.Join(vs, ", ")
}

// Reread reads again into the records what the artifacts whose records
// have moved say, and gives a row to each folder that has none. It's how
// many it read.
func (s Store) Reread(ctx context.Context) (int, error) { return s.freshen(ctx) }

// freshen brings the rows of the artifacts on this machine up to their
// folders: a folder whose artifact.toml has moved since its row was
// written is read again, one with no row is given one (the next number),
// and one whose folder has gone is kept as gone. An artifact.toml that
// can't be read is left as it was, and not listed until it can. Folders
// are read before anything's written, and a row written by ws since is
// left as that write had it.
func (s Store) freshen(ctx context.Context) (int, error) {
	db, err := s.records()
	if err != nil {
		return 0, err
	}
	var all []*kept
	if err := db.Read(ctx, func(tx *store.Tx) error {
		var err error
		all, err = allIn(tx)
		return err
	}); err != nil {
		return 0, err
	}
	byFolder := map[string]*kept{}
	for _, k := range all {
		if k.Place == Local {
			byFolder[k.Folder] = k
		}
	}
	type read struct {
		k      *kept // nil for a folder with no row
		folder string
		a      *Artifact
		info   os.FileInfo
	}
	var reads []read
	here := map[string]bool{}
	entries, _ := os.ReadDir(s.Dir())
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(s.Dir(), name, RecordFile)
		info, err := os.Stat(path)
		if err != nil {
			continue // no record yet (a write under way), or litter
		}
		here[name] = true
		k := byFolder[name]
		if k != nil && !k.moved(info) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		a, err := Parse(data)
		if err != nil {
			continue // can't be read: left as it was
		}
		reads = append(reads, read{k, name, a, info})
	}
	var gone []*kept
	for _, k := range all {
		if k.Place == Local && k.Status != Discarded && k.size >= 0 && !here[k.Folder] {
			gone = append(gone, k)
		}
	}
	if len(reads) == 0 && len(gone) == 0 {
		return 0, nil
	}
	// What was read still holds: a row written through ws since is left
	// as that write had it.
	still := func(tx *store.Tx, was *kept) (*kept, bool, error) {
		now, err := keptIn(tx, was.ID)
		if err != nil || now == nil {
			return nil, false, err
		}
		return now, now.size == was.size && now.changed == was.changed, nil
	}
	err = db.Write(ctx, func(tx *store.Tx) error {
		for _, k := range gone {
			now, ok, err := still(tx, k)
			if err != nil {
				return err
			}
			if ok {
				if err := saveIn(tx, now, nil); err != nil {
					return err
				}
			}
		}
		for _, r := range reads {
			var k *kept
			if r.k != nil {
				now, ok, err := still(tx, r.k)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
				k = now
			} else {
				now, err := keptAt(tx, Local, r.folder)
				if err != nil {
					return err
				}
				if now != nil {
					continue // given a row since it was read
				}
				n, err := nextNumber(tx)
				if err != nil {
					return err
				}
				k = &kept{ID: "a" + strconv.Itoa(n), N: n, Place: Local, Folder: r.folder}
			}
			k.Artifact = *r.a
			if err := saveIn(tx, k, r.info); err != nil {
				return err
			}
		}
		return nil
	})
	return len(reads), err
}

func nextNumber(tx *store.Tx) (int, error) {
	var n int
	err := tx.QueryRow(`SELECT coalesce(max(n), 0) + 1 FROM artifacts`).Scan(&n)
	return n, err
}

// --- Writing

// New is an artifact to add: its title, its first version's files (its
// page alone, HTML, or the page with the files beside it, Files), and
// that version's provenance and note (its number, basis and time are set
// when it's added).
type New struct {
	Title   string
	HTML    []byte
	Files   Files
	Version Version
	At      time.Time
}

// filesOf is a version's files as they were given: the files, or the page
// alone.
func filesOf(html []byte, files Files) Files {
	if len(files) > 0 {
		return files
	}
	return Page(html)
}

// Add adds an artifact on this machine, with its first version: its
// folder, named from the day and its title (with -2, -3 for a name that's
// been used), then its repo with v1 in it, then artifact.toml, then its
// row, numbered one past the highest there's been.
func (s Store) Add(ctx context.Context, n New) (Entry, error) {
	title := CleanTitle(n.Title)
	if title == "" {
		return Entry{}, errors.New("an artifact needs a title")
	}
	files, err := checkFiles(filesOf(n.HTML, n.Files))
	if err != nil {
		return Entry{}, err
	}
	at := n.At.UTC().Truncate(time.Second)
	if err := os.MkdirAll(s.Dir(), 0o755); err != nil {
		return Entry{}, err
	}
	db, err := s.records()
	if err != nil {
		return Entry{}, err
	}
	var made string
	var out Entry
	err = db.Write(ctx, func(tx *store.Tx) error {
		base := FolderName(at, title)
		for i := 1; i <= 100; i++ {
			name := base
			if i > 1 {
				name = base + "-" + strconv.Itoa(i)
			}
			// A name a row has, or a discarded folder, isn't used again:
			// a folder's name is what's written for other people.
			if k, err := keptAt(tx, Local, name); err != nil {
				return err
			} else if k != nil {
				continue
			}
			if _, err := os.Lstat(filepath.Join(s.DiscardedDir(), name)); err == nil {
				continue
			}
			dir := filepath.Join(s.Dir(), name)
			if err := os.Mkdir(dir, 0o755); errors.Is(err, os.ErrExist) {
				continue
			} else if err != nil {
				return err
			}
			made = dir
			v := n.Version
			v.N, v.BasedOn, v.At, v.Note = 1, 0, at, CleanNote(v.Note)
			v.Files, v.Size = len(files), files.Size()
			if _, err := Repo(filepath.Join(dir, RepoDir)).CommitVersion(ctx, 1, 0, files, versionMessage(title, v), at); err != nil {
				return err
			}
			a := &Artifact{Title: title, Status: Live, Made: at, Versions: []Version{v}}
			info, err := writeRecord(dir, a)
			if err != nil {
				return err
			}
			num, err := nextNumber(tx)
			if err != nil {
				return err
			}
			k := &kept{ID: "a" + strconv.Itoa(num), N: num, Place: Local, Folder: name, Artifact: *a}
			if err := saveIn(tx, k, info); err != nil {
				return err
			}
			out = k.Entry
			return nil
		}
		return fmt.Errorf("couldn't name a folder for %q in %s", title, s.Dir())
	})
	if err != nil {
		if made != "" {
			os.RemoveAll(made) // its row wasn't written: it's no artifact
		}
		return Entry{}, err
	}
	return out, nil
}

// Revision is a new version of an artifact: its files (its page alone,
// HTML, or the page with the files beside it, Files), the version it's
// based on (0 for the latest), a new title for the artifact ("" keeps
// it), and the version's provenance and note.
type Revision struct {
	HTML    []byte
	Files   Files
	On      int
	Title   string
	Version Version
	At      time.Time
}

func (r Revision) files() Files { return filesOf(r.HTML, r.Files) }

// ErrDiscarded is the answer for a change to an artifact that's been
// discarded.
var ErrDiscarded = errors.New("discarded")

// AddVersion adds a version to artifact id: numbered one past its latest,
// committed to its repo, then named in artifact.toml, then its row. It's
// the artifact as it now is, and the version's number.
func (s Store) AddVersion(ctx context.Context, id string, r Revision) (Entry, int, error) {
	db, err := s.records()
	if err != nil {
		return Entry{}, 0, err
	}
	var undo func()
	var out Entry
	var n int
	err = db.Write(ctx, func(tx *store.Tx) error {
		k, err := keptIn(tx, id)
		if err != nil {
			return err
		}
		if k == nil {
			return notFound(id)
		}
		if k.Status == Discarded {
			return fmt.Errorf("%w: %s was discarded, so it takes no new versions: add a new artifact", ErrDiscarded, id)
		}
		if k.Away() {
			return awayError(k.Entry)
		}
		dir := s.Path(k.Entry)
		a, num, drop, err := revise(ctx, dir, id, r)
		undo, n = drop, num
		if err != nil {
			return err
		}
		info, err := os.Stat(filepath.Join(dir, RecordFile))
		if err != nil {
			return err
		}
		undo = nil // artifact.toml names it: it's a version, whatever happens to the row
		k.Artifact = *a
		if err := saveIn(tx, k, info); err != nil {
			return err
		}
		out = k.Entry
		return nil
	})
	if err != nil {
		if undo != nil {
			undo()
		}
		return Entry{}, 0, err
	}
	return out, n, nil
}

// Discard discards artifact id: its record says so, and why, and its
// folder is moved to .ws/artifacts/.discarded. Its row stays, so its ref
// still says what it was, and it's in the graph no more.
func (s Store) Discard(ctx context.Context, id string, d Discard) (Entry, error) {
	d.At = d.At.UTC().Truncate(time.Second)
	d.Reason = strings.TrimSpace(d.Reason)
	db, err := s.records()
	if err != nil {
		return Entry{}, err
	}
	var out Entry
	err = db.Write(ctx, func(tx *store.Tx) error {
		k, err := keptIn(tx, id)
		if err != nil {
			return err
		}
		if k == nil {
			return notFound(id)
		}
		if k.Status == Discarded {
			return fmt.Errorf("%w: %s was discarded already, on %s", ErrDiscarded, id, k.Discarded.At.Local().Format("2006-01-02"))
		}
		if k.Away() {
			return awayError(k.Entry)
		}
		dir := s.Path(k.Entry)
		was, err := os.ReadFile(filepath.Join(dir, RecordFile))
		if errors.Is(err, os.ErrNotExist) {
			// Its folder's gone already: the row says it's discarded.
			k.Status, k.Discarded = Discarded, &d
			if err := saveIn(tx, k, nil); err != nil {
				return err
			}
			out = k.Entry
			return nil
		}
		if err != nil {
			return err
		}
		a, err := Parse(was)
		if err != nil {
			return fmt.Errorf("%s's record can't be read (%s): %w", id, filepath.Join(dir, RecordFile), err)
		}
		a.Status, a.Discarded = Discarded, &d
		if _, err := writeRecord(dir, a); err != nil {
			return err
		}
		undo := func() { _ = os.WriteFile(filepath.Join(dir, RecordFile), was, 0o644) } // undoing: the error that called for it is the one returned
		to := filepath.Join(s.DiscardedDir(), k.Folder)
		if err := os.MkdirAll(s.DiscardedDir(), 0o755); err != nil {
			undo()
			return err
		}
		if _, err := os.Lstat(to); err == nil {
			undo()
			return fmt.Errorf("there's a folder at %s already: move it aside, and discard %s again", to, id)
		}
		if err := os.Rename(dir, to); err != nil {
			undo()
			return err
		}
		info, err := os.Stat(filepath.Join(to, RecordFile))
		if err == nil {
			k.Artifact = *a
			err = saveIn(tx, k, info)
		}
		if err != nil {
			_ = os.Rename(to, dir) // its row still lists it
			undo()
			return err
		}
		out = k.Entry
		return nil
	})
	return out, err
}

// writeRecord writes an artifact's record in its folder, by rename, and
// says how it is now.
func writeRecord(dir string, a *Artifact) (os.FileInfo, error) {
	data, err := Encode(a)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, RecordFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return nil, err
	}
	return os.Stat(path)
}

// --- What's beside them

// SayWords writes what every artifact says again, from its row, in one
// transaction: ws doctor --reindex. It's how many it said.
func SayWords(ctx context.Context, root string) (int, error) {
	s := Store{Root: root}
	db, err := s.records()
	if err != nil {
		return 0, err
	}
	n := 0
	err = db.Write(ctx, func(tx *store.Tx) error {
		all, err := allIn(tx)
		if err != nil {
			return err
		}
		for _, k := range all {
			if err := graph(tx, k); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// Litter is what's in .ws/artifacts that belongs to no artifact (ws
// doctor): a folder with no artifact.toml, a version in its repo (or,
// from when each was a file, a version file) that its artifact.toml
// doesn't name, a file beside the folders, and what a write that didn't
// finish left (a .tmp). Each is more than a minute old: a younger one may
// be a write under way. Anything else in an artifact's folder belongs to
// it.
func (s Store) Litter(ctx context.Context) ([]string, error) {
	old := func(info os.FileInfo) bool { return time.Since(info.ModTime()) >= time.Minute }
	entries, err := os.ReadDir(s.Dir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // .discarded, and the system's own (.DS_Store)
		}
		path := filepath.Join(s.Dir(), name)
		info, err := e.Info()
		if err != nil {
			continue
		}
		if !e.IsDir() {
			if old(info) {
				out = append(out, path)
			}
			continue
		}
		data, err := os.ReadFile(filepath.Join(path, RecordFile))
		if errors.Is(err, os.ErrNotExist) {
			if old(info) {
				out = append(out, path) // a folder no record names
			}
			continue
		}
		if err != nil {
			continue
		}
		a, err := Parse(data)
		if err != nil {
			continue // someone's, perhaps: it's not for ws to say
		}
		named := map[int]bool{}
		for _, v := range a.Versions {
			named[v.N] = true
		}
		files, _ := os.ReadDir(path)
		for _, f := range files {
			fi, err := f.Info()
			if err != nil || !old(fi) {
				continue
			}
			if strings.HasSuffix(f.Name(), ".tmp") {
				out = append(out, filepath.Join(path, f.Name()))
				continue
			}
			if f.IsDir() {
				continue
			}
			if n, isVersion := IsVersionFile(f.Name()); isVersion && !named[n] {
				out = append(out, filepath.Join(path, f.Name()))
			}
		}
		// A version committed that the record doesn't name, once the record
		// has sat a minute since: a write whose record was never written.
		if rec, err := os.Stat(filepath.Join(path, RecordFile)); err == nil && old(rec) {
			repo := Repo(filepath.Join(path, RepoDir))
			for _, n := range repo.Versions(ctx) {
				if !named[n] {
					out = append(out, string(repo)+" (v"+strconv.Itoa(n)+", which "+RecordFile+" doesn't name)")
				}
			}
		}
	}
	return out, nil
}

// Convert puts in a repo of its own the versions of each artifact from
// when a version was a file in its folder (v1.html, v2.html, …): ws
// doctor, and the daemon as it starts. Until it has, such a one's pages
// are read from those files, and a new version of it converts it first.
// It's those it converted.
func (s Store) Convert(ctx context.Context) ([]Entry, error) {
	db, err := s.records()
	if err != nil {
		return nil, err
	}
	var old []*kept
	if err := db.Read(ctx, func(tx *store.Tx) error {
		all, err := allIn(tx)
		for _, k := range all {
			if !k.Away() && k.size >= 0 && !s.Repo(k.Entry).Exists() {
				old = append(old, k)
			}
		}
		return err
	}); err != nil || len(old) == 0 {
		return nil, err
	}
	var out []Entry
	err = db.Write(ctx, func(tx *store.Tx) error {
		for _, was := range old {
			k, err := keptIn(tx, was.ID)
			if err != nil {
				return err
			}
			if k == nil || k.Away() || k.size < 0 {
				continue
			}
			dir := s.Path(k.Entry)
			data, err := os.ReadFile(filepath.Join(dir, RecordFile))
			if err != nil {
				continue // its folder's gone: there's nothing to convert
			}
			a, err := Parse(data)
			if err != nil {
				continue // can't be read: left as it is, to be put right
			}
			did, err := convert(ctx, dir, a)
			if err != nil {
				return fmt.Errorf("putting %s's versions in its repo: %w", k.ID, err)
			}
			if !did {
				continue
			}
			info, err := writeRecord(dir, a)
			if err != nil {
				return err
			}
			k.Artifact = *a
			if err := saveIn(tx, k, info); err != nil {
				return err
			}
			out = append(out, k.Entry)
		}
		return nil
	})
	return out, err
}

// stands is how an artifact stands in words every kind shares: a live one
// is of its moment (it expires), a promoted one is kept for good, and a
// discarded one is gone, since it last changed.
func (s Status) stands(last time.Time) store.Stands {
	switch s {
	case Promoted:
		return store.Stands{Standing: store.Lasting}
	case Discarded:
		return store.Stands{Standing: store.Gone, Since: last}
	}
	return store.Stands{Standing: store.Passing}
}

// standingStep is the part's third step: an artifact written before things
// had a standing stands as its status says (Status.stands).
var standingStep = store.SQL(`
	UPDATE things SET standing = CASE state WHEN 'promoted' THEN 'lasting' WHEN 'discarded' THEN 'gone' ELSE 'passing' END
		WHERE (origin LIKE 'artifact:%' OR origin LIKE 'artifact-found:%') AND kind = 'artifact' AND standing = '';
`)
