package team

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// What ws keeps of the team is in the workspace's records: the last look
// (when, how far back, what it found wanting) and, as it left them, the
// events it worked out and the pull requests as GitHub said them; when you
// last looked at the page; each strand's gist; each period's account. A
// look's write is one transaction, so the daemon and ws team, which both
// look, can't leave one look's events beside another's pull requests; a
// gist or an account is a row of its own, so neither loses what the other
// wrote.

func init() {
	store.Define("team", store.SQL(`
		-- The last look at the team's repos: one row.
		CREATE TABLE team_look (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			at TEXT NOT NULL, since TEXT NOT NULL,
			look TEXT NOT NULL             -- the rest of it, as JSON
		);
		-- Each event, as the last look worked it out, in its order.
		CREATE TABLE team_events (
			id TEXT PRIMARY KEY, n INTEGER NOT NULL,
			at TEXT NOT NULL, kind TEXT NOT NULL, repo TEXT NOT NULL, pr INTEGER NOT NULL,
			login TEXT NOT NULL, name TEXT NOT NULL, email TEXT NOT NULL,
			event TEXT NOT NULL            -- as JSON
		) WITHOUT ROWID;
		CREATE INDEX team_events_n ON team_events (n);
		-- Each pull request as GitHub last said it, so only what's
		-- changed is asked for again.
		CREATE TABLE team_prs (
			repo TEXT NOT NULL, number INTEGER NOT NULL,
			updated TEXT NOT NULL,         -- as GitHub says it
			pr TEXT NOT NULL,              -- as JSON
			PRIMARY KEY (repo, number)
		) WITHOUT ROWID;
		-- When you last looked at the page: one row.
		CREATE TABLE team_seen (id INTEGER PRIMARY KEY CHECK (id = 1), at TEXT NOT NULL);
		-- What an agent wrote of a strand, or why it couldn't.
		CREATE TABLE team_gists (
			strand TEXT PRIMARY KEY,
			of TEXT NOT NULL, at TEXT NOT NULL, failed TEXT NOT NULL,
			gist TEXT NOT NULL             -- as JSON
		) WITHOUT ROWID;
		-- A period's account, by what the period's called (today, 7d, ''
		-- for as far back as is kept).
		CREATE TABLE team_summaries (
			period TEXT PRIMARY KEY, at TEXT NOT NULL,
			summary TEXT NOT NULL          -- as JSON
		) WITHOUT ROWID;
	`), store.Retired)
}

func (s *Service) read(fn func(tx *store.Tx) error) error {
	db, err := store.Open(s.Root)
	if err != nil {
		return err
	}
	return db.Read(context.Background(), fn)
}

func (s *Service) write(fn func(tx *store.Tx) error) error {
	db, err := store.Open(s.Root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), fn)
}

func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

// keptLook is the last look as the records keep it, with its pull
// requests, and when it was as kept: nil when there's none, or it's at.
func (s *Service) keptLook(at string) (*Snapshot, map[string]PR, string, error) {
	var snap *Snapshot
	var prs map[string]PR
	var kept string
	err := s.read(func(tx *store.Tx) error {
		var data string
		err := tx.QueryRow(`SELECT at, CASE WHEN at = ? THEN '' ELSE look END FROM team_look WHERE id = 1`, at).Scan(&kept, &data)
		if errors.Is(err, sql.ErrNoRows) || data == "" {
			return nil
		}
		if err != nil {
			return err
		}
		var got Snapshot
		if json.Unmarshal([]byte(data), &got) != nil || got.At.IsZero() {
			return nil
		}
		if got.Events, err = scanAll[Event](tx, `SELECT event FROM team_events ORDER BY n`); err != nil {
			return err
		}
		list, err := scanAll[PR](tx, `SELECT pr FROM team_prs`)
		if err != nil {
			return err
		}
		prs = make(map[string]PR, len(list))
		for _, pr := range list {
			prs[pr.Key()] = pr
		}
		snap = &got
		return nil
	})
	return snap, prs, kept, err
}

// scanAll is every row of a query of one JSON column, read as T; a row
// that can't be read is passed over.
func scanAll[T any](tx *store.Tx, q string, args ...any) ([]T, error) {
	rows, err := tx.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var v T
		if json.Unmarshal([]byte(data), &v) == nil {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}

// keepLook writes a look in tx, its events and pull requests in place of
// the last's, and says when it was as kept.
func keepLook(tx *store.Tx, snap *Snapshot, prs []PR) (string, error) {
	look, err := json.Marshal(snap)
	if err != nil {
		return "", err
	}
	// Each event as a row: what's asked of it beside it, its time as the
	// records keep times.
	type row struct {
		At    string `json:"at"`
		Event *Event `json:"event"`
	}
	rows := make([]row, len(snap.Events))
	for i := range snap.Events {
		rows[i] = row{At: stampOf(snap.Events[i].At), Event: &snap.Events[i]}
	}
	events, err := json.Marshal(rows)
	if err != nil {
		return "", err
	}
	kept, err := json.Marshal(prs)
	if err != nil {
		return "", err
	}
	at := stampOf(snap.At)
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO team_look (id, at, since, look) VALUES (1, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET at = excluded.at, since = excluded.since, look = excluded.look`, []any{at, stampOf(snap.Since), string(look)}},
		{`DELETE FROM team_events`, nil},
		// Each event from the list, in one statement: a look's are many.
		{`INSERT INTO team_events (id, n, at, kind, repo, pr, login, name, email, event)
			SELECT value ->> '$.event.id', key, value ->> '$.at', value ->> '$.event.kind', value ->> '$.event.repo',
				coalesce(value ->> '$.event.pr', 0), coalesce(value ->> '$.event.login', ''), coalesce(value ->> '$.event.name', ''),
				coalesce(value ->> '$.event.email', ''), value -> '$.event'
			FROM json_each(?)`, []any{string(events)}},
		{`DELETE FROM team_prs`, nil},
		{`INSERT INTO team_prs (repo, number, updated, pr)
			SELECT value ->> 'repo', value ->> 'number', coalesce(value ->> 'updatedAt', ''), value
			FROM json_each(?)`, []any{string(kept)}},
	} {
		if _, err := tx.Exec(q.sql, q.args...); err != nil {
			return "", err
		}
	}
	tx.Changed("team")
	return at, nil
}

// Seen is when you last looked at the team's page: the zero time if you
// never have.
func (s *Service) Seen() time.Time {
	var at string
	s.read(func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT at FROM team_seen WHERE id = 1`).Scan(&at)
	})
	t, _ := time.Parse(store.Stamp, at)
	return t
}

// MarkSeen says you've looked, now.
func (s *Service) MarkSeen() error {
	return s.write(func(tx *store.Tx) error {
		if _, err := tx.Exec(`INSERT INTO team_seen (id, at) VALUES (1, ?) ON CONFLICT (id) DO UPDATE SET at = excluded.at`, stampOf(s.now())); err != nil {
			return err
		}
		tx.Changed("team")
		return nil
	})
}

// gistsOf is the gists kept of some strands, by strand.
func (s *Service) gistsOf(strands []string) map[string]Gist {
	out := map[string]Gist{}
	if len(strands) == 0 {
		return out
	}
	list, _ := json.Marshal(strands)
	s.read(func(tx *store.Tx) error {
		gists, err := scanAll[Gist](tx, `SELECT gist FROM team_gists WHERE strand IN (SELECT value FROM json_each(?))`, string(list))
		for _, g := range gists {
			out[g.Strand] = g
		}
		return err
	})
	return out
}

// keepGist writes what was written of a strand. One that failed keeps
// what was written before, with why it couldn't be again, and counts the
// tries.
func (s *Service) keepGist(g Gist) error {
	return s.write(func(tx *store.Tx) error {
		if g.Failed != "" {
			prev, err := scanAll[Gist](tx, `SELECT gist FROM team_gists WHERE strand = ?`, g.Strand)
			if err != nil {
				return err
			}
			if len(prev) > 0 {
				g.Tries = prev[0].Tries + 1
				if prev[0].Gist != "" {
					g.Gist, g.Know = prev[0].Gist, prev[0].Know
				}
			}
		}
		if err := putGist(tx, g); err != nil {
			return err
		}
		tx.Changed("team")
		return nil
	})
}

func putGist(tx *store.Tx, g Gist) error {
	data, err := json.Marshal(g)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO team_gists (strand, of, at, failed, gist) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (strand) DO UPDATE SET of = excluded.of, at = excluded.at, failed = excluded.failed, gist = excluded.gist`,
		g.Strand, g.Of, stampOf(g.At), g.Failed, string(data))
	return err
}

// summaryOf is the account kept of a period, by what it's called.
func (s *Service) summaryOf(period string) (Summary, bool) {
	var list []Summary
	s.read(func(tx *store.Tx) error {
		var err error
		list, err = scanAll[Summary](tx, `SELECT summary FROM team_summaries WHERE period = ?`, period)
		return err
	})
	if len(list) == 0 {
		return Summary{}, false
	}
	return list[0], true
}

// keepSummary writes a period's account, and lets go of the oldest beyond
// the latest summariesKept written. One that failed leaves what was
// written before, with why.
func (s *Service) keepSummary(period string, sum Summary) error {
	return s.write(func(tx *store.Tx) error {
		if sum.Failed != "" {
			prev, err := scanAll[Summary](tx, `SELECT summary FROM team_summaries WHERE period = ?`, period)
			if err != nil {
				return err
			}
			if len(prev) > 0 && len(prev[0].Topics) > 0 {
				prev[0].Failed = sum.Failed
				sum = prev[0]
			}
		}
		if err := putSummary(tx, period, sum); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM team_summaries WHERE period NOT IN (SELECT period FROM team_summaries ORDER BY at DESC LIMIT ?)`, summariesKept); err != nil {
			return err
		}
		tx.Changed("team")
		return nil
	})
}

func putSummary(tx *store.Tx, period string, sum Summary) error {
	data, err := json.Marshal(sum)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO team_summaries (period, at, summary) VALUES (?, ?, ?)
		ON CONFLICT (period) DO UPDATE SET at = excluded.at, summary = excluded.summary`, period, stampOf(sum.At), string(data))
	return err
}
