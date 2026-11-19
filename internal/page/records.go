package page

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/words"
)

// What a page is, beside its text, is in the workspace's records: a row
// each in pages, saying its number, its title, the kind of document it's
// written as, the version of its text, how large its file was and when it
// changed, whether it's been put away, what it's tied to and where it was
// filed, and the rest of Meta as JSON. Its text stays a file,
// pages/p<n>.md: it's a document, read by agents and saved back to a repo.
// A save writes the file first, by rename, then the row, in one
// transaction, so the check of the version it began on is against the row,
// and no two writers pass it at once.
//
// A page is a thing (p3, named the same everywhere), and its words are its
// sections as it reads (AsText: its cells' words, not its questions), each
// heading a place's title. A page whose file was edited outside ws (by
// hand, or a save whose row didn't commit) is taken in when it's next
// listed, saved or searched for: its file's size or time has moved from
// its row's.

func init() {
	store.Define("page", store.SQL(`
		CREATE TABLE pages (
			id TEXT NOT NULL PRIMARY KEY,   -- p3
			n INTEGER NOT NULL UNIQUE,      -- its number: none is used twice
			title TEXT NOT NULL,
			kind TEXT NOT NULL,             -- the kind of document it's written as, or ''
			version TEXT NOT NULL,          -- of its text, as ws last wrote or read it (doc.Version)
			size INTEGER NOT NULL,          -- its file's size then; -1 when its file had gone
			changed TEXT NOT NULL,          -- its file's time then: when its text last changed
			removed TEXT NOT NULL,          -- when it was put away (p3.md.removed); '' while it's listed
			tied TEXT NOT NULL,             -- the file in a repo it's tied to (Source), or ''
			filed TEXT NOT NULL,            -- where it was filed as a document (Filed), or ''
			meta TEXT NOT NULL              -- what's kept beside it (Meta), as JSON
		) WITHOUT ROWID;
		CREATE INDEX pages_listed ON pages (removed, changed);
		CREATE INDEX pages_tied ON pages (tied) WHERE tied != '';
	`), store.Retired)
}

// ThingKind is what a page is in the graph.
const ThingKind = "page"

// placeSection is the kind of place a page's words are at: a section of
// it.
const placeSection = "page"

func (s Store) records() (*store.DB, error) { return store.Open(s.Root) }

func (s Store) read(fn func(tx *store.Tx) error) error {
	db, err := s.records()
	if err != nil {
		return err
	}
	return db.Read(context.Background(), fn)
}

func (s Store) write(fn func(tx *store.Tx) error) error {
	db, err := s.records()
	if err != nil {
		return err
	}
	return db.Write(context.Background(), fn)
}

func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

func notFound(id string) error {
	return fmt.Errorf("%w: the workspace has no page %s", ErrNotFound, id)
}

// kept is a page's row: what the records hold of it.
type kept struct {
	id, title, version string
	n                  int
	size               int64
	changed, removed   string // as store.Stamp writes them
	meta               Meta
	seen               time.Time // its file's time, as it was just seen
}

// moved reports whether a page's file isn't as its row has it: edited
// outside ws, or back after it had gone.
func (k *kept) moved(info os.FileInfo) bool {
	return info.Size() != k.size || stampOf(info.ModTime()) != k.changed
}

// page is the row as a list has it: updated when its file was, as it was
// just seen, or as the row has it.
func (k *kept) page() Page {
	p := Page{ID: k.id, Title: k.title, Kind: k.meta.Kind, Filed: k.meta.Filed, Source: k.meta.Source, To: k.meta.To, Updated: k.seen}
	if p.Updated.IsZero() {
		p.Updated, _ = time.Parse(store.Stamp, k.changed)
	}
	return p
}

const keptColumns = `id, n, title, version, size, changed, removed, meta`

func scanKept(sc interface{ Scan(...any) error }) (*kept, error) {
	var k kept
	var meta string
	if err := sc.Scan(&k.id, &k.n, &k.title, &k.version, &k.size, &k.changed, &k.removed, &meta); err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(meta), &k.meta) // what can't be read is as nothing kept
	return &k, nil
}

// keptIn is page id's row in tx, if it has one, and whether it's listed
// (not put away).
func keptIn(tx *store.Tx, id string) (*kept, bool, error) {
	k, err := scanKept(tx.QueryRow(`SELECT `+keptColumns+` FROM pages WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return k, k.removed == "", nil
}

// listedIn is every page that's listed (not put away), in tx.
func listedIn(tx *store.Tx) ([]*kept, error) {
	rows, err := tx.Query(`SELECT ` + keptColumns + ` FROM pages WHERE removed = '' ORDER BY n`)
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

// saveIn writes a page's row as its text is (its file as info has it,
// which holds text), with what it is in the graph and what it says, in
// tx. One put away is in the graph no more.
func saveIn(tx *store.Tx, k *kept, text []byte, info os.FileInfo) error {
	k.title, k.version, k.size, k.changed = Title(text), doc.Version(text), info.Size(), stampOf(info.ModTime())
	if err := putIn(tx, k); err != nil {
		return err
	}
	if k.removed != "" {
		return tx.Forget(origin(k.id))
	}
	return graph(tx, k.id, k.title, text, info.ModTime())
}

// putIn writes a page's row as k has it, in tx, and says the page has
// changed.
func putIn(tx *store.Tx, k *kept) error {
	meta, err := json.Marshal(k.meta)
	if err != nil {
		return err
	}
	tied, filed := "", ""
	if k.meta.Source != nil {
		tied = k.meta.Source.Ref
	}
	if k.meta.Filed != nil {
		filed = k.meta.Filed.Ref
	}
	if _, err := tx.Exec(`INSERT INTO pages (id, n, title, kind, version, size, changed, removed, tied, filed, meta) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET title = excluded.title, kind = excluded.kind, version = excluded.version, size = excluded.size,
			changed = excluded.changed, removed = excluded.removed, tied = excluded.tied, filed = excluded.filed, meta = excluded.meta`,
		k.id, k.n, k.title, k.meta.Kind, k.version, k.size, k.changed, k.removed, tied, filed, string(meta)); err != nil {
		return err
	}
	tx.Changed("workspace", "page:"+k.id)
	return nil
}

// The graph: a page is a thing, named the same everywhere (p3), and
// what it says is its sections, as it reads. Its cells and passes point
// at it from their own records (in, about).

func origin(id string) store.Origin { return store.Origin("page:" + id) }

func graph(tx *store.Tx, id, title string, text []byte, changed time.Time) error {
	o := origin(id)
	if err := tx.Forget(o); err != nil {
		return err
	}
	me := store.Key{Name: id}
	if err := tx.Thing(o, store.Thing{Key: me, Kind: ThingKind, Title: title, Changed: changed}); err != nil {
		return err
	}
	if title == Untitled {
		title = "" // not words it says
	}
	for _, sec := range words.Sections(AsText(text), title) {
		if err := tx.Words(o, store.Place{Thing: me, At: sec.Anchor, Kind: placeSection, Title: title, Heading: sec.Heading, Text: sec.Body, Said: changed}); err != nil {
			return err
		}
	}
	return nil
}

// IDs is the listed pages' ids, as the records have them: nothing's read
// again from their files, so it's safe where the records are being
// written.
func (s Store) IDs() []string {
	db, err := s.records()
	if err != nil {
		return nil
	}
	var out []string
	db.Read(context.Background(), func(tx *store.Tx) error {
		listed, err := listedIn(tx)
		for _, k := range listed {
			out = append(out, k.id)
		}
		return err
	})
	return out
}

// List is the workspace's pages, the latest changed first. Pages whose
// files have moved since ws last wrote or read them are read again first.
func (s Store) List() ([]Page, error) {
	listed, _, err := s.freshen(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]Page, 0, len(listed))
	for _, k := range listed {
		out = append(out, k.page())
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Updated.Equal(out[j].Updated) {
			return out[i].Updated.After(out[j].Updated)
		}
		return num(out[i].ID) > num(out[j].ID)
	})
	return out, nil
}

// Reread reads again into the records what the pages whose files have
// moved say (edited outside ws), and lets go of what those whose files
// have gone said. It's how many it read.
func (s Store) Reread(ctx context.Context) (int, error) {
	_, n, err := s.freshen(ctx)
	return n, err
}

// freshen is the listed pages that have files, each as its file now is,
// and how many it read: those whose files have moved are taken into the
// records, read before anything's written, and what those whose files
// have gone said is let go of.
func (s Store) freshen(ctx context.Context) ([]*kept, int, error) {
	db, err := s.records()
	if err != nil {
		return nil, 0, err
	}
	var listed []*kept
	if err := db.Read(ctx, func(tx *store.Tx) error {
		var err error
		listed, err = listedIn(tx)
		return err
	}); err != nil {
		return nil, 0, err
	}
	type read struct {
		k    *kept
		text []byte
		info os.FileInfo
	}
	var reads []read
	var gone, there []*kept
	for _, k := range listed {
		info, err := os.Stat(s.Path(k.id))
		if errors.Is(err, os.ErrNotExist) {
			if k.size >= 0 {
				gone = append(gone, k)
			}
			continue
		}
		if err != nil {
			continue // can't be seen now: not listed, and read next time
		}
		k.seen = info.ModTime()
		there = append(there, k)
		if !k.moved(info) {
			continue
		}
		text, err := os.ReadFile(s.Path(k.id))
		if err != nil {
			continue
		}
		reads = append(reads, read{k, text, info})
	}
	if len(reads) == 0 && len(gone) == 0 {
		return there, 0, nil
	}
	// What was read still holds: a page saved through ws since is left
	// as that save wrote it.
	still := func(tx *store.Tx, k *kept) (bool, error) {
		now, listed, err := keptIn(tx, k.id)
		if err != nil || !listed {
			return false, err
		}
		return now.size == k.size && now.changed == k.changed, nil
	}
	err = db.Write(ctx, func(tx *store.Tx) error {
		for _, k := range gone {
			if ok, err := still(tx, k); err != nil || !ok {
				if err != nil {
					return err
				}
				continue
			}
			// Its file's gone: what it said goes too, and its row stays,
			// kept as gone, so its number isn't used again and its file is
			// taken in if it comes back.
			if _, err := tx.Exec(`UPDATE pages SET size = -1 WHERE id = ?`, k.id); err != nil {
				return err
			}
			if err := tx.Forget(origin(k.id)); err != nil {
				return err
			}
			tx.Changed("workspace", "page:"+k.id)
		}
		for _, r := range reads {
			if ok, err := still(tx, r.k); err != nil || !ok {
				if err != nil {
					return err
				}
				continue
			}
			if err := saveIn(tx, r.k, r.text, r.info); err != nil {
				return err
			}
		}
		return nil
	})
	return there, len(reads), err
}

// Read is a page's text, and the page.
func (s Store) Read(id string) ([]byte, Page, error) {
	if !IsRef(id) {
		return nil, Page{}, fmt.Errorf("%w: %q isn't a page (they're named p1, p2, …)", ErrNotFound, id)
	}
	var k *kept
	var listed bool
	if err := s.read(func(tx *store.Tx) error {
		var err error
		k, listed, err = keptIn(tx, id)
		return err
	}); err != nil {
		return nil, Page{}, err
	}
	if !listed {
		return nil, Page{}, notFound(id)
	}
	text, err := os.ReadFile(s.Path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, Page{}, notFound(id)
	}
	if err != nil {
		return nil, Page{}, err
	}
	p := k.page()
	p.Title = Title(text) // as it is, edited by hand or not
	if info, err := os.Stat(s.Path(id)); err == nil {
		p.Updated = info.ModTime()
	}
	return text, p, nil
}

// New starts a page with text, numbered one past the highest there's been.
func (s Store) New(text []byte) (Page, error) {
	if err := os.MkdirAll(s.Dir(), 0o755); err != nil {
		return Page{}, err
	}
	var made string
	var p Page
	err := s.write(func(tx *store.Tx) error {
		var next int
		if err := tx.QueryRow(`SELECT coalesce(max(n), 0) + 1 FROM pages`).Scan(&next); err != nil {
			return err
		}
		for range 100 {
			id := "p" + strconv.Itoa(next)
			// A file with no row (put away, or left by a write that didn't
			// commit) keeps its number too: a ref never comes to mean
			// another page.
			if _, err := os.Stat(s.Path(id) + ".removed"); err == nil {
				next++
				continue
			}
			f, err := os.OpenFile(s.Path(id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if errors.Is(err, os.ErrExist) {
				next++
				continue
			}
			if err != nil {
				return err
			}
			made = s.Path(id)
			_, err = f.Write(text)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
			info, err := os.Stat(made)
			if err != nil {
				return err
			}
			k := &kept{id: id, n: next, seen: info.ModTime()}
			if err := saveIn(tx, k, text, info); err != nil {
				return err
			}
			p = k.page()
			return nil
		}
		return errors.New("couldn't number a new page")
	})
	if err != nil {
		if made != "" {
			os.Remove(made) // its row wasn't written: it's no page
		}
		return Page{}, err
	}
	return p, nil
}

// Write saves a page's text, edited from the version base: ErrChanged if
// it's no longer that version (and isn't already this text). It answers the
// version saved. The check and the save are one transaction: the file's
// written, by rename, then its row. A file edited outside ws since its row
// was written is checked as it is, and taken in.
func (s Store) Write(id, base string, text []byte) (string, error) {
	now := doc.Version(text)
	var at string
	err := s.write(func(tx *store.Tx) error {
		k, listed, err := keptIn(tx, id)
		if err != nil {
			return err
		}
		if !listed {
			return notFound(id)
		}
		path := s.Path(id)
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			return notFound(id)
		}
		if err != nil {
			return err
		}
		at = k.version
		var was []byte
		if k.moved(info) {
			if was, err = os.ReadFile(path); err != nil {
				return err
			}
			at = doc.Version(was)
		}
		if at != base && at != now {
			return ErrChanged
		}
		if at == now {
			if was != nil { // what's there is this text: taken in
				return saveIn(tx, k, was, info)
			}
			return nil
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, text, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, path); err != nil {
			return err
		}
		if info, err = os.Stat(path); err != nil {
			return err
		}
		return saveIn(tx, k, text, info)
	})
	if errors.Is(err, ErrChanged) {
		return at, err
	}
	if err != nil {
		return "", err
	}
	return now, nil
}

// Remove puts a page away: its file is kept beside the others
// (p3.md.removed), and its row in the records, out of every list, its
// number never used again. What it said isn't found any more.
func (s Store) Remove(id string) error {
	if !IsRef(id) {
		return notFound(id)
	}
	moved := false
	err := s.write(func(tx *store.Tx) error {
		k, listed, err := keptIn(tx, id)
		if err != nil {
			return err
		}
		if !listed {
			return notFound(id)
		}
		if err := os.Rename(s.Path(id), s.Path(id)+".removed"); errors.Is(err, os.ErrNotExist) {
			return notFound(id)
		} else if err != nil {
			return err
		}
		moved = true
		k.removed = stampOf(time.Now())
		if err := putIn(tx, k); err != nil {
			return err
		}
		return tx.Forget(origin(id))
	})
	if err != nil && moved {
		os.Rename(s.Path(id)+".removed", s.Path(id)) // its row still lists it
	}
	return err
}

// Meta is what's kept of a page beside its text: nothing, for most.
func (s Store) Meta(id string) Meta {
	var m Meta
	s.read(func(tx *store.Tx) error {
		if k, _, err := keptIn(tx, id); err == nil && k != nil {
			m = k.meta
		}
		return nil
	})
	return m
}

// SetMeta keeps what's known of a page beside its text.
func (s Store) SetMeta(id string, m Meta) error {
	return s.write(func(tx *store.Tx) error {
		k, listed, err := keptIn(tx, id)
		if err != nil {
			return err
		}
		if !listed {
			return notFound(id)
		}
		k.meta = m
		return putIn(tx, k)
	})
}

// TiedTo is the page tied to the file ref names (repo:path, or
// repo/capsule:path), if one is: the latest changed, of several.
func (s Store) TiedTo(ref string) string {
	var ids []string
	s.read(func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT id FROM pages WHERE tied = ? AND removed = '' ORDER BY changed DESC, n DESC`, ref)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		return rows.Err()
	})
	for _, id := range ids {
		if _, err := os.Stat(s.Path(id)); err == nil {
			return id
		}
	}
	return ""
}

// SayWords writes what every page says again, from its file, in one
// transaction: ws doctor --reindex. It's how many it read.
func SayWords(ctx context.Context, root string) (int, error) {
	s := Store{Root: root}
	db, err := s.records()
	if err != nil {
		return 0, err
	}
	n := 0
	err = db.Write(ctx, func(tx *store.Tx) error {
		listed, err := listedIn(tx)
		if err != nil {
			return err
		}
		for _, k := range listed {
			info, err := os.Stat(s.Path(k.id))
			if err != nil {
				continue // gone, or can't be seen: as it was
			}
			text, err := os.ReadFile(s.Path(k.id))
			if err != nil {
				continue
			}
			if err := saveIn(tx, k, text, info); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// Litter is what's in the pages' folder that no record has (ws doctor): a
// page's text (p7.md, or p7.md.removed) with no row, what was kept beside
// a page before the records (p3.json), and what a save that didn't finish
// left (a .tmp more than a minute old).
func (s Store) Litter(ctx context.Context) ([]string, error) {
	db, err := s.records()
	if err != nil {
		return nil, err
	}
	kept := map[string]bool{}
	if err := db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT id FROM pages`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			kept[id] = true
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}
	entries, _ := os.ReadDir(s.Dir())
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		id := strings.TrimSuffix(strings.TrimSuffix(name, ".removed"), ".md")
		switch {
		case strings.HasSuffix(name, ".tmp"):
			if info, err := e.Info(); err != nil || time.Since(info.ModTime()) < time.Minute {
				continue // a save under way, perhaps
			}
		case strings.HasSuffix(name, ".json") && IsRef(strings.TrimSuffix(name, ".json")):
		case IsRef(id) && id != name && !kept[id]:
		default:
			continue
		}
		out = append(out, filepath.Join(s.Dir(), name))
	}
	return out, nil
}
