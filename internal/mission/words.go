package mission

import (
	"context"
	"database/sql"
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
//
// Each file read is a thing in the mission as well, in it (store.In): its
// brief (brief, called what the mission is), and a file in files/
// (mission:<its path there>, a handoff or a file, by its own title). What
// its words name is linked from it, as mentions (store.Words): so what
// names a thing leads to the brief or the handoff that does, and from
// there to its mission.

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

// filesOrigin is what the records of a mission's files are under.
func filesOrigin(slug string) store.Origin { return store.Origin("missionfile:" + slug) }

func fileOrigin(slug, rel string) store.Origin {
	return store.Origin(string(filesOrigin(slug)) + "/" + rel)
}

// standing is a mission's state as the graph says it: one that's neither
// parked nor archived is in play.
func standing(state string) string {
	if state == "" {
		return "in play"
	}
	return state
}

// stands is how a mission stands in words every kind shares: in force
// while it's in play (or parked), settled once it's ended, since it was
// (gone, if it was dropped). What's said in it stands as it does, as
// what's found is weighed (store.Stands.Under).
func stands(m *Mission, ended time.Time) store.Stands {
	if m.State != Archived {
		return store.Stands{Standing: store.InForce}
	}
	if m.Ending != nil && m.Ending.Ending == EndDropped {
		return store.Stands{Standing: store.Gone, Since: ended}
	}
	return store.Stands{Standing: store.Settled, Since: ended}
}

// decisionStands is how a decision stands: in force while it's current;
// gone once it's withdrawn, or superseded, since then (when the decision
// that superseded it was made).
func decisionStands(d Decision, all []Decision) store.Stands {
	switch {
	case d.State == DecisionWithdrawn && d.Withdrawn != nil:
		return store.Stands{Standing: store.Gone, Since: *d.Withdrawn}
	case d.State == DecisionSuperseded:
		for _, by := range all {
			if by.ID == d.SupersededBy {
				return store.Stands{Standing: store.Gone, Since: by.At}
			}
		}
		return store.Stands{Standing: store.Gone}
	case d.State == DecisionWithdrawn:
		return store.Stands{Standing: store.Gone}
	}
	return store.Stands{Standing: store.InForce}
}

// BriefRef is what a mission's brief is called in it: its thing in the
// graph, in the mission's scope.
const BriefRef = "brief"

// fileThing is what a file of a mission is in the graph: the brief, by
// the mission's title, or a file in files/ (mission:<its path there>, a
// handoff or a file), by its own, or its name where it has none. It
// stands as its mission does (state).
func fileThing(slug, title, state string, f fileStamp, text []byte) store.Thing {
	changed, _ := time.Parse(store.Stamp, f.mod)
	if f.rel == briefFile {
		return store.Thing{Key: store.Key{Scope: slug, Name: BriefRef}, Kind: placeBrief, Title: title, State: state, Changed: changed, Standing: store.InForce}
	}
	inFiles := strings.TrimPrefix(f.rel, MissionFilesDir+"/")
	kind := placeFile
	if strings.HasPrefix(inFiles, "handoffs/") {
		kind = placeHandoff
	}
	if title = words.Title(text); title == "" {
		title = path.Base(inFiles)
	}
	// It stands in force, as its mission does: once that's ended, what's
	// found in it is weighed as the mission (store.Stands.Under).
	return store.Thing{Key: store.Key{Scope: slug, Name: "mission:" + inFiles}, Kind: kind, Title: title, State: state, Changed: changed, Standing: store.InForce}
}

// placesOf is what a file of a mission says: its sections, each a place.
// The brief's are found as the mission's own, by its title, and said by
// the brief; a file in files/ is its own thing's, by its own title.
func placesOf(slug string, t store.Thing, text []byte) []store.Place {
	in, by := t.Key, store.Key{}
	if t.Kind == placeBrief {
		in, by = store.Key{Name: slug}, t.Key
	}
	var out []store.Place
	for _, sec := range words.Sections(text, t.Title) {
		out = append(out, store.Place{Thing: in, By: by, At: sec.Anchor, Kind: t.Kind, Title: t.Title, Heading: sec.Heading, Text: sec.Body, Said: t.Changed})
	}
	return out
}

// sayFile writes what a file is and says, and how it was when it was
// read, in tx: what was made of it before is forgotten first. A brief
// that's missing or empty is no thing: the mission is still found by its
// title.
func sayFile(tx *store.Tx, slug, title string, f fileStamp, text []byte) error {
	o := fileOrigin(slug, f.rel)
	if err := tx.Forget(o); err != nil {
		return err
	}
	var state string
	if err := tx.QueryRow(`SELECT state FROM missions WHERE slug = ?`, slug).Scan(&state); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	t := fileThing(slug, title, standing(state), f, text)
	if f.rel != briefFile || strings.TrimSpace(string(text)) != "" {
		if err := tx.Thing(o, t); err != nil {
			return err
		}
		if err := tx.Link(o, store.Link{Src: t.Key, Kind: store.In, Dst: store.Key{Name: slug}, At: t.Changed}); err != nil {
			return err
		}
	}
	for _, p := range placesOf(slug, t, text) {
		if err := tx.Words(o, p); err != nil {
			return err
		}
	}
	_, err := tx.Exec(`INSERT INTO mission_files (slug, path, size, mod) VALUES (?, ?, ?, ?)
		ON CONFLICT (slug, path) DO UPDATE SET size = excluded.size, mod = excluded.mod`, slug, f.rel, f.size, f.mod)
	return err
}

// forgetFile lets go of what a file was and said, and that it was read.
func forgetFile(tx *store.Tx, slug, rel string) error {
	if err := tx.Forget(fileOrigin(slug, rel)); err != nil {
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
