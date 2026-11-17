package pulls

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// What ws keeps of pull requests is in the workspace's records: a row each
// in pulls (what pr.json held, as JSON, beside its repo, number, title,
// branch, head and who docked it), and in pull_list the Reviews page's
// list as GitHub last said it. A pull request ws keeps something of is a
// thing in the graph (demo#12), and what it says it does, its title and
// description, are its words.

func init() {
	store.Define("pulls", store.SQL(`
		CREATE TABLE pulls (
			repo TEXT NOT NULL, number INTEGER NOT NULL,
			title TEXT NOT NULL, branch TEXT NOT NULL, head TEXT NOT NULL,
			docked TEXT NOT NULL,       -- who docked it here: ws, you, or ''
			changed TEXT NOT NULL,
			pull TEXT NOT NULL,         -- the record, as JSON
			PRIMARY KEY (repo, number)
		) WITHOUT ROWID;
		CREATE INDEX pulls_docked ON pulls (docked);
		-- The open pull requests across the workspace's repos, as GitHub
		-- last said them, sorted into the Reviews page's sections. One row:
		-- the list is made across every repo at once.
		CREATE TABLE pull_list (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			at TEXT NOT NULL,
			list TEXT NOT NULL
		);
	`), store.Retired)
}

func readRecords(root string, fn func(tx *store.Tx) error) error {
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	return db.Read(context.Background(), fn)
}

func writeRecords(root string, fn func(tx *store.Tx) error) error {
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), fn)
}

func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

// Load reads what ws keeps of a PR: an empty record if nothing yet.
func Load(root, repo string, number int) (*Record, error) {
	var r *Record
	err := readRecords(root, func(tx *store.Tx) error {
		var err error
		r, _, err = loadIn(tx, repo, number)
		return err
	})
	return r, err
}

// loadIn is what's kept of a PR in tx, and whether anything is.
func loadIn(tx *store.Tx, repo string, number int) (*Record, bool, error) {
	var data string
	err := tx.QueryRow(`SELECT pull FROM pulls WHERE repo = ? AND number = ?`, repo, number).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return &Record{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var r Record
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		return nil, false, fmt.Errorf("what's kept of %s: %w", Key(repo, number), err)
	}
	return &r, true, nil
}

// Update changes what ws keeps of a PR, in one transaction. Nothing's
// written when nothing changed: a PR's page reads it each time it opens.
func Update(root, repo string, number int, fn func(r *Record)) (*Record, error) {
	var r *Record
	err := writeRecords(root, func(tx *store.Tx) error {
		var found bool
		var err error
		if r, found, err = loadIn(tx, repo, number); err != nil {
			return err
		}
		was, _ := json.Marshal(r)
		fn(r)
		if now, _ := json.Marshal(r); found && string(now) == string(was) {
			return nil
		}
		return saveIn(tx, repo, number, r, time.Now())
	})
	return r, err
}

// saveIn writes a PR's row, and what it is in the graph, in tx.
func saveIn(tx *store.Tx, repo string, number int, r *Record, at time.Time) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	docked := ""
	if r.Docked != nil {
		docked = r.Docked.By
	}
	if _, err := tx.Exec(`INSERT INTO pulls (repo, number, title, branch, head, docked, changed, pull) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (repo, number) DO UPDATE SET title = excluded.title, branch = excluded.branch, head = excluded.head,
			docked = excluded.docked, changed = excluded.changed, pull = excluded.pull`,
		repo, number, r.Title, r.Branch, r.Head, docked, stampOf(at), string(data)); err != nil {
		return err
	}
	tx.Changed("pr:" + Key(repo, number))
	return graph(tx, repo, number, r, at)
}

// The graph: a pull request ws keeps something of is a thing, named the
// same everywhere (demo#12), and its words are what it says it does.
// Reviews of it, and conversations about it, point at it from their own
// records.

// ThingKind is what a pull request is in the graph.
const ThingKind = "pull-request"

func origin(repo string, number int) store.Origin {
	return store.Origin("pulls:" + Key(repo, number))
}

func graph(tx *store.Tx, repo string, number int, r *Record, at time.Time) error {
	o := origin(repo, number)
	if err := tx.Forget(o); err != nil {
		return err
	}
	me := store.Key{Name: Key(repo, number)}
	title := strings.TrimSpace(r.Title)
	if err := tx.Thing(o, store.Thing{Key: me, Kind: ThingKind, Title: title, Changed: at}); err != nil {
		return err
	}
	return tx.Words(o, store.Place{Thing: me, Kind: ThingKind, Title: title, Text: strings.TrimSpace(r.Body), Said: at})
}

// DockedBy is every PR that's docked here by who (ByWS, ByYou).
func DockedBy(root, by string) ([]Kept, error) {
	var out []Kept
	err := readRecords(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT repo, number, pull FROM pulls WHERE docked = ? ORDER BY repo, number`, by)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var k Kept
			var data string
			if err := rows.Scan(&k.Repo, &k.Number, &data); err != nil {
				return err
			}
			var r Record
			if json.Unmarshal([]byte(data), &r) == nil { // one that can't be read is passed over
				k.Record = &r
				out = append(out, k)
			}
		}
		return rows.Err()
	})
	return out, err
}

// SayWords writes every PR's words again, from its record, in one
// transaction: ws doctor --reindex. It's how many it read.
func SayWords(ctx context.Context, root string) (int, error) {
	db, err := store.Open(root)
	if err != nil {
		return 0, err
	}
	n := 0
	err = db.Write(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT repo, number, changed, pull FROM pulls ORDER BY repo, number`)
		if err != nil {
			return err
		}
		type kept struct {
			repo    string
			number  int
			changed time.Time
			r       Record
		}
		var all []kept
		for rows.Next() {
			var k kept
			var changed, data string
			if err := rows.Scan(&k.repo, &k.number, &changed, &data); err != nil {
				rows.Close()
				return err
			}
			k.changed, _ = time.Parse(store.Stamp, changed)
			if json.Unmarshal([]byte(data), &k.r) == nil {
				all = append(all, k)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, k := range all {
			if err := graph(tx, k.repo, k.number, &k.r, k.changed); err != nil {
				return err
			}
		}
		n = len(all)
		return nil
	})
	return n, err
}

// The list, as it's kept: in memory, and in the records for the next
// process (a command that asks, the daemon when it starts again).

// keptList is the list in the records when it isn't what's held: the
// list, and when it was asked for as kept. nil when the records hold none,
// or hold what's held (at).
func keptList(root, at string) (*List, string, error) {
	var l *List
	var kept string
	err := readRecords(root, func(tx *store.Tx) error {
		var data string
		err := tx.QueryRow(`SELECT at, CASE WHEN at = ? THEN '' ELSE list END FROM pull_list WHERE id = 1`, at).Scan(&kept, &data)
		if errors.Is(err, sql.ErrNoRows) || data == "" {
			return nil
		}
		if err != nil {
			return err
		}
		var got List
		if json.Unmarshal([]byte(data), &got) == nil && got.Sections != nil {
			l = &got
		}
		return nil
	})
	return l, kept, err
}

// keepList writes the list to the records, and says when it's of as kept.
func keepList(root string, l *List) (string, error) {
	data, err := json.Marshal(l)
	if err != nil {
		return "", err
	}
	at := stampOf(l.At)
	return at, writeRecords(root, func(tx *store.Tx) error {
		if _, err := tx.Exec(`INSERT INTO pull_list (id, at, list) VALUES (1, ?, ?) ON CONFLICT (id) DO UPDATE SET at = excluded.at, list = excluded.list`, at, string(data)); err != nil {
			return err
		}
		tx.Changed("reviews")
		return nil
	})
}
