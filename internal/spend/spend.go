// Package spend is what the workspace's agents have spent: the tokens each
// run and each crew's session sent and wrote, by model and day, and who it
// was spent for (a mission, and what on it: its orchestrator, a crew, its
// checks). It's kept as tokens, as the harness reported them; what they
// cost is worked out when they're read, from prices that may since have
// changed. Where a harness gives its own figure (Claude Code's, at list
// price), that's kept too.
//
// It's kept by ws as it goes because nothing else keeps it: a harness's
// session files are cleared after a while, and a run's output isn't kept
// past its last few. What isn't caught here can't be counted later.
package spend

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/store"
)

func init() {
	store.Define("spend", store.SQL(`
		CREATE TABLE spend (
			source TEXT NOT NULL,      -- what it was spent in: a run, or a crew's session
			model TEXT NOT NULL,       -- '' where the harness didn't say
			day TEXT NOT NULL,         -- 2006-01-02, UTC
			mission TEXT NOT NULL,     -- '' for the workspace's own
			kind TEXT NOT NULL,        -- what it was for: Orchestrate, Crew, Check…
			subject TEXT NOT NULL,     -- which of them: the crew, the conversation
			harness TEXT NOT NULL,
			input INTEGER NOT NULL DEFAULT 0,
			output INTEGER NOT NULL DEFAULT 0,
			cache_write INTEGER NOT NULL DEFAULT 0,
			cache_read INTEGER NOT NULL DEFAULT 0,
			cost REAL,                 -- the harness's own figure, in US dollars: NULL where it gave none
			PRIMARY KEY (source, model, day)
		) WITHOUT ROWID;
		CREATE INDEX spend_mission ON spend (mission, kind);
		-- How far a crew's session file has been read: its bytes, and the
		-- message read last (Claude Code writes one over several lines).
		CREATE TABLE spend_read (
			source TEXT PRIMARY KEY,
			bytes INTEGER NOT NULL,
			last TEXT NOT NULL
		) WITHOUT ROWID;
	`))
}

// What it was spent for.
const (
	Orchestrate = "orchestrate" // the orchestrator, on a mission
	Crew        = "crew"        // a crew's session
	Check       = "check"       // the judge, on a step's checks
	Decisions   = "decisions"   // the decision look, on crew's decisions
	PR          = "pr"          // the PR writer
	Talk        = "conversation"
	Team        = "team"       // a gist or an account on the Team page
	Eval        = "eval"       // the judge, on an eval's fixture
	Automation  = "automation" // an automation's ask: its subject is the automation
)

// For is who it was spent for: what's kept with it, so it can be added up
// by mission, and by what on it.
type For struct {
	Mission string // "" for the workspace's own
	Kind    string
	Subject string // the crew, the conversation, the run
}

// Keep keeps what a headless run spent. source names the run, and is the
// one place it's kept: keeping it again replaces it.
func Keep(root, source string, f For, harnessName string, at time.Time, usage []harness.Usage) error {
	if len(usage) == 0 {
		return nil
	}
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	day := at.UTC().Format(time.DateOnly)
	return db.Write(context.Background(), func(tx *store.Tx) error {
		for _, u := range usage {
			var cost any
			if u.Cost != 0 {
				cost = u.Cost
			}
			if _, err := tx.Exec(`INSERT OR REPLACE INTO spend
				(source, model, day, mission, kind, subject, harness, input, output, cache_write, cache_read, cost)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				source, u.Model, day, f.Mission, f.Kind, f.Subject, harnessName,
				u.Input, u.Output, u.CacheWrite, u.CacheRead, cost); err != nil {
				return err
			}
		}
		tx.Changed(Topic)
		return nil
	})
}

// Topic is what a change to what's spent says changed: its own, as nothing
// shown yet reads it, and a crew's session is read every few minutes.
const Topic = "spend"

// SessionSource is the source a crew's session is kept under: its
// harness's session, or (for one ws keeps the record of, an ACP crew's)
// that record, by mission and crew.
func SessionSource(harnessName, session string) string {
	return "session " + harnessName + "/" + session
}

// RecordSource is the source an ACP crew's record is kept under.
func RecordSource(mission, crew string) string {
	return "record " + mission + "/" + crew
}

// ReadSession reads what a crew's session has spent since it was last
// read, from the file at path (its session file, or ws's record of it),
// kept under source, and adds it to what's kept. model is the one it's on
// where its harness doesn't say. It reads only what was appended since (a
// file that shrank was replaced, and is read again from the start, what
// was kept of it first let go), and only whole lines. It says whether it
// found anything new.
func ReadSession(root, path, source string, f For, harnessName, model string) (bool, error) {
	if !harness.CapsOf(harnessName).Spends.Crew {
		return false, nil
	}
	db, err := store.Open(root)
	if err != nil {
		return false, err
	}
	var from int64
	var last string
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		err := tx.QueryRow(`SELECT bytes, last FROM spend_read WHERE source = ?`, source).Scan(&from, &last)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	})
	if err != nil {
		return false, err
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	restart := info.Size() < from
	if restart {
		from, last = 0, ""
	}
	if info.Size() == from {
		return false, nil
	}
	if _, err := file.Seek(from, io.SeekStart); err != nil {
		return false, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return false, err
	}
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		return false, nil // a line not yet ended: read once it is
	}
	spent, last := harness.SessionSpend(harnessName, model, bytes.Split(data[:end], []byte("\n")), last)
	err = db.Write(context.Background(), func(tx *store.Tx) error {
		if restart {
			if _, err := tx.Exec(`DELETE FROM spend WHERE source = ?`, source); err != nil {
				return err
			}
		}
		for _, s := range spent {
			if _, err := tx.Exec(`INSERT INTO spend
				(source, model, day, mission, kind, subject, harness, input, output, cache_write, cache_read)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (source, model, day) DO UPDATE SET
					input = input + excluded.input, output = output + excluded.output,
					cache_write = cache_write + excluded.cache_write, cache_read = cache_read + excluded.cache_read`,
				source, s.Model, s.Day, f.Mission, f.Kind, f.Subject, harnessName,
				s.Input, s.Output, s.CacheWrite, s.CacheRead); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO spend_read (source, bytes, last) VALUES (?, ?, ?)`,
			source, from+int64(end)+1, last); err != nil {
			return err
		}
		if len(spent) > 0 || restart {
			tx.Changed(Topic)
		}
		return nil
	})
	return len(spent) > 0, err
}

// Line is what was spent on one model, for one kind of thing, as kept.
type Line struct {
	Kind, Harness string
	harness.Usage
	// Costed: the harness gave its own figure for all of it (Cost).
	Costed bool
}

// OfMission is what's been spent on a mission ("" for the workspace's
// own), by what it was for and model.
func OfMission(root, mission string) ([]Line, error) {
	db, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	var out []Line
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT kind, harness, model, sum(input), sum(output), sum(cache_write), sum(cache_read),
				coalesce(sum(cost), 0), count(cost) = count(*)
			FROM spend WHERE mission = ? GROUP BY kind, harness, model ORDER BY kind, harness, model`, mission)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var l Line
			if err := rows.Scan(&l.Kind, &l.Harness, &l.Model, &l.Input, &l.Output, &l.CacheWrite, &l.CacheRead, &l.Cost, &l.Costed); err != nil {
				return err
			}
			out = append(out, l)
		}
		return rows.Err()
	})
	return out, err
}

// Tokens is how many tokens were spent for kind and subject (an
// automation's asks) on the days from since's (UTC): what came in, went
// out and was written to the cache. What was read from the cache isn't
// counted: it's a tenth the price, and every turn reads its fixed prompt.
func Tokens(root, kind, subject string, since time.Time) (int64, error) {
	db, err := store.Open(root)
	if err != nil {
		return 0, err
	}
	var n int64
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT coalesce(sum(input + output + cache_write), 0) FROM spend WHERE kind = ? AND subject = ? AND day >= ?`,
			kind, subject, since.UTC().Format("2006-01-02")).Scan(&n)
	})
	return n, err
}
