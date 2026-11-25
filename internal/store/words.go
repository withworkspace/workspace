package store

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/withworkspace/workspace/internal/words"
)

// What things say: a row in words, the full-text table, for each place
// something's said (a turn, a decision, a comment, a section of a brief),
// and beside it a row in places sharing its number: the record that said
// it (its origin, by which Forget finds it without reading the full-text
// table), the thing it's in, and where in it. Each store writes its words
// where it writes its things, in the same transaction, and they're ranked
// as the documents' search ranks (words.Rank): a thing is found once, led
// to its best place.

// wordTables are the words' tables: the store part's third step.
const wordTables = `
		CREATE TABLE places (
			id INTEGER PRIMARY KEY,
			origin TEXT NOT NULL,
			scope TEXT NOT NULL, name TEXT NOT NULL, -- the thing it's in
			at TEXT NOT NULL,                       -- where in it: a turn's number, a heading's anchor
			kind TEXT NOT NULL,                     -- what it is: a turn, a decision, a section of a brief…
			said TEXT NOT NULL                      -- when it was said, as Stamp writes it
		);
		CREATE INDEX places_origin ON places (origin, kind);
		CREATE INDEX places_thing ON places (scope, name);
		-- What's said there: the thing's title (weighted 0 by BM25: it adds
		-- to a score as words.Rank says), the place's own heading, and its
		-- words, stemmed.
		CREATE VIRTUAL TABLE words USING fts5 (title, heading, body, tokenize = 'porter unicode61');
`

// Place is somewhere words are, that a search can lead to: a turn, a
// decision, a comment, a section of a brief.
type Place struct {
	Thing Key    // what it's in
	At    string // where in it: a turn's number, a heading's anchor, a finding's key
	Kind  string // what it is: turn, finding, decision, comment, review, brief, file
	// Title is its thing's title, when it has one that's a title (not its
	// first words): one that holds what's asked leads to the thing. Heading
	// is the place's own, when it has one: a section's.
	Title, Heading string
	Text           string
	Said           time.Time // when it was said: of two found alike, the newer first
	// By is what says it, when that's not the thing it's in: a mission's
	// brief, whose words are found as the mission's. What its words name
	// is linked from it (names.go).
	By Key
	// Pinned: its record links what it names itself, as the names were
	// when it was said (a conversation's turn), so they aren't found again
	// here.
	Pinned bool
}

// Words records what's said at a place, for the record at origin: its
// text as words (markdown's marks, and what links point at, left out, so
// a snippet of it reads as words), and what its words name as links from
// what says them (names.go: read before the marks are left out, so a
// link's address names what it leads to). A place with nothing said and
// no title isn't one.
func (tx *Tx) Words(origin Origin, p Place) error {
	if err := tx.named(origin, p); err != nil {
		return err
	}
	p.Text = words.Clean(p.Text)
	if p.Title == "" && p.Heading == "" && p.Text == "" {
		return nil
	}
	if _, err := tx.Exec(`INSERT INTO places (origin, scope, name, at, kind, said) VALUES (?, ?, ?, ?, ?, ?)`,
		string(origin), p.Thing.Scope, p.Thing.Name, p.At, p.Kind, p.Said.UTC().Format(Stamp)); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO words (rowid, title, heading, body) VALUES (last_insert_rowid(), ?, ?, ?)`, p.Title, p.Heading, p.Text)
	return err
}

// ForgetWords removes the words the record at origin said: those of one
// kind of place, or all of them ("").
func (tx *Tx) ForgetWords(origin Origin, kind string) error {
	where, args := `origin = ?`, []any{string(origin)}
	if kind != "" {
		where, args = `origin = ? AND kind = ?`, append(args, kind)
	}
	if _, err := tx.Exec(`DELETE FROM words WHERE rowid IN (SELECT id FROM places WHERE `+where+`)`, args...); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM places WHERE `+where, args...)
	return err
}

// Retitle gives the places the record at origin said its thing's title
// as it now is, where it's changed.
func (tx *Tx) Retitle(origin Origin, title string) error {
	_, err := tx.Exec(`UPDATE words SET title = ? WHERE rowid IN (SELECT id FROM places WHERE origin = ?) AND title != ?`, title, string(origin), title)
	return err
}

// Said is a search of what's said in the records.
type Said struct {
	Words string // as typed
	// Mission keeps to what's said in one mission (its slug), the mission
	// itself among it; "" is everywhere.
	Mission string
	// Gone is what's been withdrawn too: left out unless it's asked for.
	Gone  bool
	Limit int // things at most (20 if none)
	// Now is when it's asked, as what's found ages (standing.go): now, if
	// it's zero.
	Now time.Time
}

// Found is a thing a search found, led to the place in it that's the best
// match.
type Found struct {
	// Thing is what it's in, as its row says; a place in something that
	// isn't a thing (a mission's file) has its key, its place's kind and
	// its title.
	Thing Thing
	// Where it's led: what's there (Place: a turn, a decision, a section of
	// a brief…), where in the thing (At, Heading; empty when the thing's
	// title is what's asked for, and it's the thing as a whole).
	Place, At, Heading string
	Snippet            []words.Part
	Score              float64
	// Every word that matters is in it. Where too few have every word,
	// those with some of them fill the page.
	Every bool
	Said  time.Time // its latest word, of those found
	Ended bool      // it's of a mission that's archived
	// Stands is how it stands, as its score was weighed: its own, or its
	// mission's where that's settled or gone (Stands.Under).
	Stands Stands
}

// How standing moves the score of a thing whose kind doesn't say its
// standing (one written before things had one): what's of an archived
// mission, or a decision since superseded, is found a little below what's
// in play, and none of it ages.
const settled = 0.85

// Find is the things whose words best match q, best first, and how many
// more matched than are given. Every thing with any of the words is
// ranked together, as the documents are (words.Rank), then weighed by how
// it stands and how long since (Stands.Worth): what's no longer so, and
// what was of its moment long ago, is found below what's still so. A
// withdrawn decision is left out unless q.Gone; ties go to the newer.
func (db *DB) Find(ctx context.Context, q Said) (found []Found, more int, err error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}
	m := words.Match(q.Words)
	if m.All == "" {
		return nil, 0, nil
	}
	err = db.Read(ctx, func(tx *Tx) error {
		every, err := everyWord(tx, m.Every(), q.Mission)
		if err != nil {
			return err
		}
		all, err := find(tx, m, q, every)
		if err != nil {
			return err
		}
		for _, f := range all {
			f.Every = every[f.Thing.Key]
			if len(found) < q.Limit {
				found = append(found, f)
			} else {
				more++
			}
		}
		return nil
	})
	return found, more, err
}

// inMission keeps a query to a mission's places, and the mission's own.
func inMission(q string, args []any, slug string) (string, []any) {
	if slug == "" {
		return q, args
	}
	return q + ` AND (p.scope = ? OR (p.scope = '' AND p.name = ?))`, append(args, slug, slug)
}

// everyWord is the things a full-text query matches.
func everyWord(tx *Tx, match, mission string) (map[Key]bool, error) {
	q, args := inMission(`SELECT DISTINCT p.scope, p.name FROM words JOIN places p ON p.id = words.rowid WHERE words MATCH ?`, []any{match}, mission)
	rows, err := tx.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[Key]bool{}
	for rows.Next() {
		var k Key
		if err := rows.Scan(&k.Scope, &k.Name); err != nil {
			return nil, err
		}
		out[k] = true
	}
	return out, rows.Err()
}

// place is a row a search found, as words.Rank is given it.
type place struct {
	thing       Thing
	known       bool   // it has a row in things
	in          Stands // its mission's, for what's in one
	kind, at    string
	heading     string
	said        time.Time
	ended, gone bool
}

func find(tx *Tx, m words.Query, s Said, every map[Key]bool) ([]Found, error) {
	q, args := inMission(`SELECT p.id, p.scope, p.name, p.at, p.kind, p.said, words.title, words.heading,
			highlight(words, 0, char(2), char(3)), highlight(words, 1, char(2), char(3)),
			snippet(words, 2, char(2), char(3), '…', 16), substr(words.body, 1, 160),
			bm25(words, 0.0, 1.0, 1.0),
			t.kind IS NOT NULL, coalesce(t.kind, ''), coalesce(t.title, ''), coalesce(t.state, ''), coalesce(t.changed, ''),
			coalesce(t.standing, ''), coalesce(t.since, ''),
			coalesce(m.state, ''), coalesce(m.standing, ''), coalesce(m.since, '')
		FROM words JOIN places p ON p.id = words.rowid
			LEFT JOIN things t ON t.scope = p.scope AND t.name = p.name
			LEFT JOIN things m ON m.scope = '' AND m.kind = 'mission' AND m.name = (CASE p.scope WHEN '' THEN p.name ELSE p.scope END)
		WHERE words MATCH ?`, []any{m.Looked()}, s.Mission)
	rows, err := tx.Query(q+` LIMIT 5000`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found []words.Row
	latest := map[Key]time.Time{}
	for rows.Next() {
		var (
			r                    words.Row
			p                    place
			id                   int64
			said, changed, state string
			since, inSince       string
		)
		if err := rows.Scan(&id, &p.thing.Key.Scope, &p.thing.Key.Name, &p.at, &p.kind, &said, &r.Title, &r.Heading,
			&r.TitleMarked, &r.HeadingMarked, &r.Snippet, &r.Lead, &r.BM25,
			&p.known, &p.thing.Kind, &p.thing.Title, &p.thing.State, &changed, &p.thing.Standing, &since,
			&state, &p.in.Standing, &inSince); err != nil {
			return nil, err
		}
		if p.thing.State == "withdrawn" && !s.Gone {
			continue
		}
		p.said, _ = time.Parse(Stamp, said)
		p.thing.Changed, _ = time.Parse(Stamp, changed)
		p.thing.Since, _ = time.Parse(Stamp, since)
		p.in.Since, _ = time.Parse(Stamp, inSince)
		if p.thing.Key.Scope == "" && p.thing.Kind == "mission" {
			p.in = Stands{} // the mission itself: its own
		}
		p.ended = state == "archived"
		p.gone = p.thing.State == "superseded" || p.thing.State == "withdrawn"
		if !p.known { // a place in something that isn't a thing: as it says
			p.thing.Kind, p.thing.Title = p.kind, r.Title
		}
		p.heading = r.Heading
		r.Key, r.N, r.Of = p.thing.Key.Scope+"\x00"+p.thing.Key.Name, int(id), p
		r.Every = every[p.thing.Key]
		if p.said.After(latest[p.thing.Key]) {
			latest[p.thing.Key] = p.said
		}
		found = append(found, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	now := s.Now
	if now.IsZero() {
		now = time.Now()
	}
	ranked := words.Rank(m, found)
	out := make([]Found, 0, len(ranked))
	for _, r := range ranked {
		p := r.Of.(place)
		f := Found{Thing: p.thing, Place: p.kind, At: p.at, Heading: p.heading, Snippet: r.Snippet, Score: r.Score, Said: latest[p.thing.Key], Ended: p.ended}
		if r.Top {
			f.At, f.Heading = "", "" // it's the thing as a whole that's asked for
		}
		f.Stands = p.thing.Stands.Under(p.in)
		switch {
		case f.Stands.Standing != "":
			said := f.Said
			if p.thing.Changed.After(said) {
				said = p.thing.Changed
			}
			f.Score *= f.Stands.Worth(said, now)
		case p.ended || p.gone:
			f.Score *= settled
		}
		out = append(out, f)
	}
	// Ties, as near as makes no difference to a reader, go to the newer: a
	// young workspace's words score next to nothing (a word in more than
	// half the rows scores 0), and newest first is the order then.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := math.Round(out[i].Score*100), math.Round(out[j].Score*100)
		if a != b {
			return a > b
		}
		return out[i].Said.After(out[j].Said)
	})
	return out, nil
}
