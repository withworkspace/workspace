package store

import (
	"fmt"
	"slices"
	"time"
)

// The graph: a row in things for anything with a ref, and a row in links
// for one thing pointing at another, and why. Each is written by the
// store whose record implies it, in the same transaction as the record,
// named by that record (its origin), and removed with it (Forget): so a
// link is worked out once, when what implies it is written, and can't
// outlive it.

// Key is what a thing is called: the scope it's read in, a mission's slug
// for what's named in one (t3, d2, r1.c2), or "" for what's named the same
// everywhere (a file, a repo, demo#12, a mission, p3, w4); and its name,
// as ref.Ref.Name() writes it, less its lines.
type Key struct{ Scope, Name string }

// Origin names the record that implies a thing and its links: its store's
// table, and its key there (conversation:cache/t3).
type Origin string

// Thing is something with a ref, as it's drawn and listed without opening
// its record.
type Thing struct {
	Key     Key
	Kind    string // what it is: a thread, a talk, a cell, a pass…
	Title   string
	State   string // where it stands: open, resolved…
	Changed time.Time
}

// Link is one thing pointing at another, and why.
type Link struct {
	Src, Dst Key
	Kind     string
	Lines    string // where in Dst, when it's a file: 17, 17-20
	At       time.Time
}

// Kinds of link: a short, closed list, so whatever reads them can say why
// each is worth following.
const (
	About    = "about"    // a thread, a talk or a pass → what it's anchored to
	On       = "on"       // a comment or a finding → a file, at lines
	By       = "by"       // a document, a review, a decision → who made it
	For      = "for"      // a document, crew → the stage it's for
	Of       = "of"       // a review → what's reviewed
	In       = "in"       // a decision → its thread; a cell → its page
	From     = "from"     // a mission, a document a talk proposed → the talk or the crew it came from
	Mentions = "mentions" // text → what it names
	Lands    = "lands"    // a capsule → its pull request; a pull request → its mission
)

var linkKinds = []string{About, On, By, For, Of, In, From, Mentions, Lands}

// graphTables are the graph's: the store part's second step.
const graphTables = `
		CREATE TABLE things (
			scope TEXT NOT NULL, name TEXT NOT NULL,
			kind TEXT NOT NULL, title TEXT NOT NULL, state TEXT NOT NULL, changed TEXT NOT NULL,
			origin TEXT NOT NULL,
			PRIMARY KEY (scope, name)
		) WITHOUT ROWID;
		CREATE INDEX things_origin ON things (origin);
		CREATE TABLE links (
			src_scope TEXT NOT NULL, src_name TEXT NOT NULL, kind TEXT NOT NULL,
			dst_scope TEXT NOT NULL, dst_name TEXT NOT NULL, lines TEXT NOT NULL, at TEXT NOT NULL,
			origin TEXT NOT NULL
		);
		CREATE INDEX links_src ON links (src_scope, src_name, kind);
		CREATE INDEX links_dst ON links (dst_scope, dst_name, kind);
		CREATE INDEX links_origin ON links (origin);
`

// Stamp is how a time is kept in a column: in UTC, to the microsecond,
// and always as long, so one sorts before another as it should.
const Stamp = "2006-01-02T15:04:05.000000Z"

// Thing records what a thing is, for the record at origin.
func (tx *Tx) Thing(origin Origin, t Thing) error {
	_, err := tx.Exec(`INSERT INTO things (scope, name, kind, title, state, changed, origin) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (scope, name) DO UPDATE SET kind = excluded.kind, title = excluded.title, state = excluded.state,
			changed = excluded.changed, origin = excluded.origin`,
		t.Key.Scope, t.Key.Name, t.Kind, t.Title, t.State, t.Changed.UTC().Format(Stamp), string(origin))
	return err
}

// Link records what a thing points at, for the record at origin.
func (tx *Tx) Link(origin Origin, l Link) error {
	if !slices.Contains(linkKinds, l.Kind) {
		return fmt.Errorf("store: a link is %v, not %q", linkKinds, l.Kind)
	}
	_, err := tx.Exec(`INSERT INTO links (src_scope, src_name, kind, dst_scope, dst_name, lines, at, origin) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.Src.Scope, l.Src.Name, l.Kind, l.Dst.Scope, l.Dst.Name, l.Lines, l.At.UTC().Format(Stamp), string(origin))
	return err
}

// Forget removes what the record at origin implied: its things, links and
// words.
func (tx *Tx) Forget(origin Origin) error {
	for _, q := range []string{`DELETE FROM things WHERE origin = ?`, `DELETE FROM links WHERE origin = ?`} {
		if _, err := tx.Exec(q, string(origin)); err != nil {
			return err
		}
	}
	return tx.ForgetWords(origin, "")
}

// Links is what points at a thing, or what it points at (from), latest
// first: for whoever asks of the graph.
func (tx *Tx) Links(k Key, from bool) ([]Link, error) {
	q := `SELECT src_scope, src_name, kind, dst_scope, dst_name, lines, at FROM links WHERE dst_scope = ? AND dst_name = ? ORDER BY at DESC`
	if from {
		q = `SELECT src_scope, src_name, kind, dst_scope, dst_name, lines, at FROM links WHERE src_scope = ? AND src_name = ? ORDER BY at DESC`
	}
	rows, err := tx.Query(q, k.Scope, k.Name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Link
	for rows.Next() {
		var l Link
		var at string
		if err := rows.Scan(&l.Src.Scope, &l.Src.Name, &l.Kind, &l.Dst.Scope, &l.Dst.Name, &l.Lines, &at); err != nil {
			return nil, err
		}
		l.At, _ = time.Parse(Stamp, at)
		out = append(out, l)
	}
	return out, rows.Err()
}

// Linked is a link to a thing, and what's at its other end, as its thing
// says (Kind empty when what points has no thing: a link's own record
// wrote none).
type Linked struct {
	Link
	From Thing
}

// LinksTo is what points at any of keys, each link once (a capsule's file
// and the repo's file are one file to whoever asks), latest first.
func (tx *Tx) LinksTo(keys ...Key) ([]Linked, error) {
	var out []Linked
	seen := map[string]bool{}
	for _, k := range keys {
		rows, err := tx.Query(`SELECT l.src_scope, l.src_name, l.kind, l.dst_scope, l.dst_name, l.lines, l.at,
				coalesce(t.kind, ''), coalesce(t.title, ''), coalesce(t.state, ''), coalesce(t.changed, '')
			FROM links l LEFT JOIN things t ON t.scope = l.src_scope AND t.name = l.src_name
			WHERE l.dst_scope = ? AND l.dst_name = ?`, k.Scope, k.Name)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var l Linked
			var at, changed string
			if err := rows.Scan(&l.Src.Scope, &l.Src.Name, &l.Kind, &l.Dst.Scope, &l.Dst.Name, &l.Lines, &at,
				&l.From.Kind, &l.From.Title, &l.From.State, &changed); err != nil {
				rows.Close()
				return nil, err
			}
			l.At, _ = time.Parse(Stamp, at)
			l.From.Key = l.Src
			l.From.Changed, _ = time.Parse(Stamp, changed)
			id := l.Src.Scope + "\x00" + l.Src.Name + "\x00" + l.Kind + "\x00" + l.Lines
			if !seen[id] {
				seen[id] = true
				out = append(out, l)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	slices.SortStableFunc(out, func(a, b Linked) int { return b.At.Compare(a.At) })
	return out, nil
}
