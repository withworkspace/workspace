package automation

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// What ws keeps of each automation, in the workspace's records: whether
// it's on, and its place in the events (the last it's been matched
// against); each run of it, one an event, and what came of it; and each
// step a run took that reached outside it, kept as started and as done,
// so a run that's run again is given what each step that's done did
// rather than doing it again.

func init() {
	store.Define("automation", store.SQL(`
		CREATE TABLE automations (
			name TEXT PRIMARY KEY,
			on_ INTEGER NOT NULL DEFAULT 1,
			after INTEGER NOT NULL DEFAULT 0,   -- the last event it's been matched against
			added TEXT NOT NULL,
			failed TEXT NOT NULL DEFAULT ''     -- what it last failed with, as you were told it
		) WITHOUT ROWID;
		CREATE TABLE automation_runs (
			n INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			event INTEGER NOT NULL,
			at TEXT NOT NULL,
			state TEXT NOT NULL,                -- running, ran, failed, unsure
			error TEXT NOT NULL DEFAULT '',
			log TEXT NOT NULL DEFAULT ''
		);
		CREATE UNIQUE INDEX automation_runs_event ON automation_runs (name, event);
		CREATE TABLE automation_steps (
			name TEXT NOT NULL,
			event INTEGER NOT NULL,
			step INTEGER NOT NULL,
			builtin TEXT NOT NULL,
			state TEXT NOT NULL,                -- started, done
			result TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (name, event, step)
		) WITHOUT ROWID;
	`))
}

// The states of a run.
const (
	Running = "running"
	Ran     = "ran"
	Failed  = "failed"
	// Unsure is a run stopped at a step that was started and never said
	// it was done: whether it was done isn't known, so it isn't again.
	Unsure = "unsure"
)

// State is what ws keeps of an automation.
type State struct {
	Name   string
	On     bool
	After  int64
	Added  time.Time
	Failed string
}

// Run is one run of an automation, on one event.
type Run struct {
	N     int64     `json:"n"`
	Name  string    `json:"name"`
	Event int64     `json:"event"`
	At    time.Time `json:"at"`
	State string    `json:"state"`
	Error string    `json:"error,omitempty"`
	Log   string    `json:"log,omitempty"`
}

func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

func timeOf(s string) time.Time {
	t, _ := time.Parse(store.Stamp, s)
	return t
}

func write(root string, fn func(tx *store.Tx) error) error {
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), fn)
}

func read(root string, fn func(tx *store.Tx) error) error {
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	return db.Read(context.Background(), fn)
}

// keep keeps that name's on, matched from after on.
func keep(tx *store.Tx, name string, after int64, at time.Time) error {
	_, err := tx.Exec(`INSERT INTO automations (name, on_, after, added) VALUES (?, 1, ?, ?)
		ON CONFLICT (name) DO UPDATE SET on_ = 1, after = excluded.after, failed = ''`, name, after, stampOf(at))
	return err
}

// States is what's kept of each automation, by name.
func States(root string) (map[string]State, error) {
	out := map[string]State{}
	err := read(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT name, on_, after, added, failed FROM automations`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s State
			var on int
			var added string
			if err := rows.Scan(&s.Name, &on, &s.After, &added, &s.Failed); err != nil {
				return err
			}
			s.On, s.Added = on == 1, timeOf(added)
			out[s.Name] = s
		}
		return rows.Err()
	})
	return out, err
}

// SetOn turns name on or off. Turned on again, it's matched from the next
// event: what happened while it was off isn't run.
func SetOn(root, name string, on bool) error {
	return write(root, func(tx *store.Tx) error {
		latest, err := latestEvent(tx)
		if err != nil {
			return err
		}
		q := `UPDATE automations SET on_ = 0 WHERE name = ?`
		args := []any{name}
		if on {
			q, args = `UPDATE automations SET on_ = 1, after = ?, failed = '' WHERE name = ?`, []any{latest, name}
		}
		res, err := tx.Exec(q, args...)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ErrNotFound is the answer for an automation there isn't.
var ErrNotFound = errors.New("there's no automation by that name")

// forget lets go of what's kept of name.
func forget(tx *store.Tx, name string) error {
	for _, q := range []string{`DELETE FROM automations WHERE name = ?`, `DELETE FROM automation_runs WHERE name = ?`, `DELETE FROM automation_steps WHERE name = ?`} {
		if _, err := tx.Exec(q, name); err != nil {
			return err
		}
	}
	return nil
}

// latestEvent is the number of the latest event kept.
func latestEvent(tx *store.Tx) (int64, error) {
	var n sql.NullInt64
	err := tx.QueryRow(`SELECT max(n) FROM events`).Scan(&n)
	return n.Int64, err
}

// Runs is name's runs ("" for every automation's), the latest first, at
// most limit.
func Runs(root, name string, limit int) ([]Run, error) {
	var out []Run
	err := read(root, func(tx *store.Tx) error {
		q, args := `SELECT n, name, event, at, state, error, log FROM automation_runs`, []any{}
		if name != "" {
			q, args = q+` WHERE name = ?`, append(args, name)
		}
		q += ` ORDER BY n DESC LIMIT ?`
		rows, err := tx.Query(q, append(args, limit)...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r Run
			var at string
			if err := rows.Scan(&r.N, &r.Name, &r.Event, &at, &r.State, &r.Error, &r.Log); err != nil {
				return err
			}
			r.At = timeOf(at)
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

// runState is the state of name's run on event, "" for none.
func runState(tx *store.Tx, name string, event int64) (string, error) {
	var state string
	err := tx.QueryRow(`SELECT state FROM automation_runs WHERE name = ? AND event = ?`, name, event).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return state, err
}

// startRun keeps name's run on event as running.
func startRun(root, name string, event int64, at time.Time) error {
	return write(root, func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT INTO automation_runs (name, event, at, state) VALUES (?, ?, ?, ?)
			ON CONFLICT (name, event) DO UPDATE SET state = excluded.state, at = excluded.at`, name, event, stampOf(at), Running)
		return err
	})
}

// endRun keeps how name's run on event ended, and moves its place in the
// events past it.
func endRun(root, name string, event int64, state, errText, log string) error {
	return write(root, func(tx *store.Tx) error {
		if _, err := tx.Exec(`UPDATE automation_runs SET state = ?, error = ?, log = ? WHERE name = ? AND event = ?`, state, errText, log, name, event); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE automations SET after = max(after, ?) WHERE name = ?`, event, name)
		return err
	})
}

// passed moves name's place in the events past event, with no run: it
// wasn't one name's for.
func passed(tx *store.Tx, name string, event int64) error {
	_, err := tx.Exec(`UPDATE automations SET after = max(after, ?) WHERE name = ?`, event, name)
	return err
}

// stepState is a run's step as kept: its state ("" for none) and result.
func stepState(root, name string, event int64, step int) (state, result string, err error) {
	err = read(root, func(tx *store.Tx) error {
		err := tx.QueryRow(`SELECT state, result FROM automation_steps WHERE name = ? AND event = ? AND step = ?`, name, event, step).Scan(&state, &result)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	})
	return state, result, err
}

// keepStep keeps a run's step as it stands: started, done (with what it
// came to), or forgotten (state "": it failed, and may be done again).
func keepStep(root, name string, event int64, step int, builtin, state, result string) error {
	return write(root, func(tx *store.Tx) error {
		if state == "" {
			_, err := tx.Exec(`DELETE FROM automation_steps WHERE name = ? AND event = ? AND step = ?`, name, event, step)
			return err
		}
		_, err := tx.Exec(`INSERT INTO automation_steps (name, event, step, builtin, state, result) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (name, event, step) DO UPDATE SET state = excluded.state, result = excluded.result`, name, event, step, builtin, state, result)
		return err
	})
}

// conversationsOf are the conversations name's runs started (converse),
// by id: what's said in them is its own doing, and not run on.
func conversationsOf(tx *store.Tx, name string) (map[string]bool, error) {
	rows, err := tx.Query(`SELECT result FROM automation_steps WHERE name = ? AND builtin = 'converse' AND state = 'done'`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// setFailed keeps what name last failed with ("" for nothing): you're
// told of a failure once, not at every event.
func setFailed(root, name, why string) error {
	return write(root, func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE automations SET failed = ? WHERE name = ?`, why, name)
		return err
	})
}
