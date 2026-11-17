package mission

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/words"
)

// What a mission says: its decisions, what was decided and why
// (decisionsGraph, with the decision); and its brief's sections and its
// files' (files/: handoffs, notes), which crew and you edit outside ws.
// Each file read is kept in mission_files with its size and time, and read
// again when either's moved: whoever asks (a search) has the files of
// missions in play read again first (Reread), which is dozens of stats
// however many missions there have been.

// Kinds of place a mission's words are at.
const (
	placeDecision = "decision"
	placeBrief    = "brief"
	placeHandoff  = "handoff"
	placeFile     = "file"
)

// filesTable is mission_files: the part's third step makes it.
const filesTable = `
		-- A file of a mission's whose words are said (its brief, what's in
		-- files/), as it was when it was read: read again when it's moved.
		CREATE TABLE mission_files (
			slug TEXT NOT NULL, path TEXT NOT NULL, -- in the mission's folder: brief.md, files/handoffs/c1.md
			size INTEGER NOT NULL, mod TEXT NOT NULL,
			PRIMARY KEY (slug, path)
		) WITHOUT ROWID;
`

// wordsStep is the part's third step: mission_files, and what every
// mission kept before words were says.
func wordsStep(tx *store.Tx) error {
	if _, err := tx.Exec(filesTable); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT slug, title FROM missions ORDER BY created, slug`)
	if err != nil {
		return err
	}
	type kept struct{ slug, title string }
	var all []kept
	for rows.Next() {
		var k kept
		if err := rows.Scan(&k.slug, &k.title); err != nil {
			rows.Close()
			return err
		}
		all = append(all, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	s := Store{Root: tx.Root()}
	for _, k := range all {
		if err := s.sayAll(tx, k.slug, k.title); err != nil {
			return err
		}
	}
	return nil
}

// sayAll writes again all a mission says: its decisions, and its files,
// read now.
func (s Store) sayAll(tx *store.Tx, slug, title string) error {
	all, err := s.decisionsIn(tx, slug)
	if err != nil {
		return err
	}
	if err := s.decisionsGraph(tx, slug, all); err != nil {
		return err
	}
	if err := forgetFiles(tx, slug); err != nil {
		return err
	}
	for _, f := range s.filesOf(slug) {
		text, err := os.ReadFile(filepath.Join(s.missionDir(slug), filepath.FromSlash(f.rel)))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			continue // can't be read now: the next search reads it again
		}
		if err := sayFile(tx, slug, title, f, text); err != nil {
			return err
		}
	}
	return nil
}

// SayWords writes again what every mission says, from its record and its
// files, a transaction a mission: ws doctor --reindex. It's how many it
// read.
func (s Store) SayWords(ctx context.Context) (int, error) {
	db, err := s.records()
	if err != nil {
		return 0, err
	}
	var all []Past
	if err := db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT slug, title FROM missions ORDER BY created, slug`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p Past
			if err := rows.Scan(&p.Slug, &p.Title); err != nil {
				return err
			}
			all = append(all, p)
		}
		return rows.Err()
	}); err != nil {
		return 0, err
	}
	for i, p := range all {
		if err := db.Write(ctx, func(tx *store.Tx) error { return s.sayAll(tx, p.Slug, p.Title) }); err != nil {
			return i, err
		}
	}
	return len(all), nil
}

// fileStamp is a file as it was when it was read: its path in the
// mission's folder (slashed), its size and time.
type fileStamp struct {
	rel  string
	size int64
	mod  string
}

// filesOf is the files of a mission whose words are said, as they now
// are: its brief (one that's missing is as one that's empty: the mission
// is still found by its title), and the markdown in files/, but
// files/reviews/ (reviews rendered for crew: what they say is the
// reviews' own words).
func (s Store) filesOf(slug string) []fileStamp {
	dir := s.missionDir(slug)
	brief := fileStamp{rel: briefFile}
	if info, err := os.Stat(filepath.Join(dir, briefFile)); err == nil {
		brief.size, brief.mod = info.Size(), stampOf(info.ModTime())
	}
	out := []fileStamp{brief}
	files := filepath.Join(dir, MissionFilesDir)
	filepath.WalkDir(files, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == MissionFilesDir+"/reviews" || strings.HasPrefix(d.Name(), ".") && p != files {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(path.Ext(rel), ".md") || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if info, err := d.Info(); err == nil && info.Mode().IsRegular() {
			out = append(out, fileStamp{rel: rel, size: info.Size(), mod: stampOf(info.ModTime())})
		}
		return nil
	})
	return out
}

func fileOrigin(slug, rel string) store.Origin {
	return store.Origin("missionfile:" + slug + "/" + rel)
}

// placesOf is what a file of a mission says: its sections, each a place.
// The brief's are the mission's own, by its title; a file in files/ is
// read in the mission as mission:<its path there>, by its own title.
func placesOf(slug, title string, f fileStamp, text []byte) []store.Place {
	said, _ := time.Parse(store.Stamp, f.mod)
	thing, kind := store.Key{Name: slug}, placeBrief
	if f.rel != briefFile {
		inFiles := strings.TrimPrefix(f.rel, MissionFilesDir+"/")
		thing, kind = store.Key{Scope: slug, Name: "mission:" + inFiles}, placeFile
		if strings.HasPrefix(inFiles, "handoffs/") {
			kind = placeHandoff
		}
		if title = words.Title(text); title == "" {
			title = path.Base(inFiles)
		}
	}
	var out []store.Place
	for _, sec := range words.Sections(text, title) {
		out = append(out, store.Place{Thing: thing, At: sec.Anchor, Kind: kind, Title: title, Heading: sec.Heading, Text: sec.Body, Said: said})
	}
	return out
}

// sayFile writes what a file says, and how it was when it was read, in tx:
// what it said before is forgotten first.
func sayFile(tx *store.Tx, slug, title string, f fileStamp, text []byte) error {
	o := fileOrigin(slug, f.rel)
	if err := tx.ForgetWords(o, ""); err != nil {
		return err
	}
	for _, p := range placesOf(slug, title, f, text) {
		if err := tx.Words(o, p); err != nil {
			return err
		}
	}
	_, err := tx.Exec(`INSERT INTO mission_files (slug, path, size, mod) VALUES (?, ?, ?, ?)
		ON CONFLICT (slug, path) DO UPDATE SET size = excluded.size, mod = excluded.mod`, slug, f.rel, f.size, f.mod)
	return err
}

// forgetFile lets go of what a file said, and that it was read.
func forgetFile(tx *store.Tx, slug, rel string) error {
	if err := tx.ForgetWords(fileOrigin(slug, rel), ""); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM mission_files WHERE slug = ? AND path = ?`, slug, rel)
	return err
}

// forgetFiles lets go of what every file of a mission's said.
func forgetFiles(tx *store.Tx, slug string) error {
	rows, err := tx.Query(`SELECT path FROM mission_files WHERE slug = ?`, slug)
	if err != nil {
		return err
	}
	var rels []string
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err != nil {
			rows.Close()
			return err
		}
		rels = append(rels, rel)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, rel := range rels {
		if err := forgetFile(tx, slug, rel); err != nil {
			return err
		}
	}
	return nil
}

// Reread reads again into the records what the files of missions in play
// say, where they've moved since they were read (their size or their
// time), and lets go of those that have gone: they're edited outside ws.
// What's read is read before anything's written. It's how many files it
// read.
func (s Store) Reread(ctx context.Context) (int, error) {
	db, err := s.records()
	if err != nil {
		return 0, err
	}
	titles := map[string]string{}
	had := map[string]map[string]fileStamp{}
	var slugs []string
	if err := db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT m.slug, m.title, coalesce(f.path, ''), coalesce(f.size, 0), coalesce(f.mod, '')
			FROM missions m LEFT JOIN mission_files f ON f.slug = m.slug WHERE m.ended = '' ORDER BY m.created, m.slug`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var slug, title string
			var f fileStamp
			if err := rows.Scan(&slug, &title, &f.rel, &f.size, &f.mod); err != nil {
				return err
			}
			if _, ok := titles[slug]; !ok {
				titles[slug], had[slug] = title, map[string]fileStamp{}
				slugs = append(slugs, slug)
			}
			if f.rel != "" {
				had[slug][f.rel] = f
			}
		}
		return rows.Err()
	}); err != nil {
		return 0, err
	}
	type read struct {
		slug string
		f    fileStamp
		text []byte
	}
	var reads []read
	var gone [][2]string
	for _, slug := range slugs {
		now := map[string]bool{}
		for _, f := range s.filesOf(slug) {
			now[f.rel] = true
			if had[slug][f.rel] == f {
				continue
			}
			text, err := os.ReadFile(filepath.Join(s.missionDir(slug), filepath.FromSlash(f.rel)))
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				continue // can't be read now: next time
			}
			reads = append(reads, read{slug, f, text})
		}
		for rel := range had[slug] {
			if !now[rel] {
				gone = append(gone, [2]string{slug, rel})
			}
		}
	}
	if len(reads) == 0 && len(gone) == 0 {
		return 0, nil
	}
	err = db.Write(ctx, func(tx *store.Tx) error {
		for _, g := range gone {
			if err := forgetFile(tx, g[0], g[1]); err != nil {
				return err
			}
		}
		for _, r := range reads {
			if err := sayFile(tx, r.slug, titles[r.slug], r.f, r.text); err != nil {
				return err
			}
		}
		return nil
	})
	return len(reads), err
}
