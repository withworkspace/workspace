package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
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
	`), store.SQL(`
		-- Workflows: what a run's held for (its per key, for at_most), and
		-- what's settling (settle): a key's events, run once they've been
		-- quiet. A run parked at a wait() keeps its waiting step.
		ALTER TABLE automation_runs ADD COLUMN key TEXT NOT NULL DEFAULT '';
		CREATE INDEX automation_runs_key ON automation_runs (name, key, at);
		CREATE TABLE automation_settling (
			name TEXT NOT NULL,
			key TEXT NOT NULL,
			due TEXT NOT NULL,
			events TEXT NOT NULL,              -- the events' numbers, as JSON, oldest first
			PRIMARY KEY (name, key)
		) WITHOUT ROWID;
	`), store.SQL(`
		-- Where in its script each step was called from (line:col), for
		-- its outline to count them on.
		ALTER TABLE automation_steps ADD COLUMN at TEXT NOT NULL DEFAULT '';
	`), store.SQL(`
		-- A schedule's (every()): when its time was last looked at, from
		-- which its next is worked out.
		ALTER TABLE automations ADD COLUMN ticked TEXT NOT NULL DEFAULT '';
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
	// Waiting is a run parked at a wait(), till what it waits for or its
	// time's up.
	Waiting = "waiting"
	// Held is an event at_most held back: it isn't run.
	Held = "held"
)

// State is what ws keeps of an automation.
type State struct {
	Name   string
	On     bool
	After  int64
	Added  time.Time
	Failed string
	// Ticked is when its schedule was last looked at (every()): its next
	// time is after it.
	Ticked time.Time
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
	// Key is its per key (the pull request, the thing), where it has one.
	Key string `json:"key,omitempty"`
}

func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

func timeOf(s string) time.Time {
	t, _ := time.Parse(store.Stamp, s)
	return t
}

// Topic is what a write to what's kept of the automations says it changed
// (store.Tx.Changed): the Automations page follows it.
const Topic = "automations"

func write(root string, fn func(tx *store.Tx) error) error {
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), func(tx *store.Tx) error {
		tx.Changed(Topic)
		return fn(tx)
	})
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
	_, err := tx.Exec(`INSERT INTO automations (name, on_, after, added, ticked) VALUES (?, 1, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET on_ = 1, after = excluded.after, failed = '', ticked = excluded.ticked`, name, after, stampOf(at), stampOf(at))
	return err
}

// States is what's kept of each automation, by name.
func States(root string) (map[string]State, error) {
	out := map[string]State{}
	err := read(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT name, on_, after, added, failed, ticked FROM automations`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s State
			var on int
			var added, ticked string
			if err := rows.Scan(&s.Name, &on, &s.After, &added, &s.Failed, &ticked); err != nil {
				return err
			}
			s.On, s.Added = on == 1, timeOf(added)
			if ticked != "" {
				s.Ticked = timeOf(ticked)
			}
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
			q, args = `UPDATE automations SET on_ = 1, after = ?, failed = '', ticked = ? WHERE name = ?`, []any{latest, stampOf(time.Now()), name}
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
	for _, q := range []string{`DELETE FROM automations WHERE name = ?`, `DELETE FROM automation_runs WHERE name = ?`, `DELETE FROM automation_steps WHERE name = ?`, `DELETE FROM automation_settling WHERE name = ?`, `DELETE FROM automation_tasks WHERE name = ? AND state = 'queued'`, `DELETE FROM automation_series WHERE name = ?`, `DELETE FROM automation_outputs WHERE name = ?`} {
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
		q, args := `SELECT n, name, event, at, state, error, log, key FROM automation_runs`, []any{}
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
			if err := rows.Scan(&r.N, &r.Name, &r.Event, &at, &r.State, &r.Error, &r.Log, &r.Key); err != nil {
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

// startRun keeps name's run on event as running, for key.
func startRun(root, name string, event int64, key string, at time.Time) error {
	return write(root, func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT INTO automation_runs (name, event, at, state, key) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (name, event) DO UPDATE SET state = excluded.state, key = excluded.key`, name, event, stampOf(at), Running, key)
		return err
	})
}

// runsSince is how many of name's runs for key began at or after since,
// held back ones aside: what at_most counts.
func runsSince(root, name, key string, since time.Time) (int, error) {
	var n int
	err := read(root, func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM automation_runs WHERE name = ? AND key = ? AND at >= ? AND state != ?`, name, key, stampOf(since), Held).Scan(&n)
	})
	return n, err
}

// waitingStep is a run parked at a wait(): its event, its step, and what
// it waits for.
type waitingStep struct {
	event int64
	step  int
	w     waitFor
}

// waitFor is a wait() as kept: what it waits for, after which event, till
// when.
type waitFor struct {
	// Match is a wait()'s event; Items a wait_all's or wait_any's (Any).
	Match string     `json:"match,omitempty"`
	Items []waitItem `json:"items,omitempty"`
	Any   bool       `json:"any,omitempty"`
	After int64      `json:"after"`
	Due   time.Time  `json:"due"`
}

// waitingSteps are name's runs parked at a wait().
func waitingSteps(root, name string) ([]waitingStep, error) {
	var out []waitingStep
	err := read(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT event, step, result FROM automation_steps WHERE name = ? AND state = 'waiting' ORDER BY event, step`, name)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ws waitingStep
			var result string
			if err := rows.Scan(&ws.event, &ws.step, &result); err != nil {
				return err
			}
			if json.Unmarshal([]byte(result), &ws.w) == nil {
				out = append(out, ws)
			}
		}
		return rows.Err()
	})
	return out, err
}

// settling is a key's events, settling: run once they've been quiet till
// due.
type settling struct {
	key    string
	due    time.Time
	events []int64
}

// settle keeps event as one of key's settling for name, quiet till due.
func settle(root, name, key string, event int64, due time.Time) error {
	return write(root, func(tx *store.Tx) error {
		var had string
		err := tx.QueryRow(`SELECT events FROM automation_settling WHERE name = ? AND key = ?`, name, key).Scan(&had)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var evs []int64
		_ = json.Unmarshal([]byte(had), &evs)
		evs = append(evs, event)
		b, _ := json.Marshal(evs)
		_, err = tx.Exec(`INSERT INTO automation_settling (name, key, due, events) VALUES (?, ?, ?, ?)
			ON CONFLICT (name, key) DO UPDATE SET due = excluded.due, events = excluded.events`, name, key, stampOf(due), string(b))
		return err
	})
}

// settled are name's keys whose events have been quiet till now, taken
// from what's settling. With none, nothing's written: a pass with
// nothing to do writes nothing, or the loop that runs them would be told
// of its own write and pass again, for good.
func settled(root, name string, now time.Time) ([]settling, error) {
	var out []settling
	err := read(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT key, due, events FROM automation_settling WHERE name = ? AND due <= ?`, name, stampOf(now))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var st settling
			var due, evs string
			if err := rows.Scan(&st.key, &due, &evs); err != nil {
				return err
			}
			st.due = timeOf(due)
			_ = json.Unmarshal([]byte(evs), &st.events)
			out = append(out, st)
		}
		return rows.Err()
	})
	if err != nil || len(out) == 0 {
		return nil, err
	}
	err = write(root, func(tx *store.Tx) error {
		for _, st := range out {
			if _, err := tx.Exec(`DELETE FROM automation_settling WHERE name = ? AND key = ?`, name, st.key); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
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
// came to), or forgotten (state "": it failed, and may be done again);
// at is where in its script it was called from ("" keeps what's kept).
func keepStep(root, name string, event int64, step int, builtin, state, result, at string) error {
	return write(root, func(tx *store.Tx) error {
		if state == "" {
			_, err := tx.Exec(`DELETE FROM automation_steps WHERE name = ? AND event = ? AND step = ?`, name, event, step)
			return err
		}
		_, err := tx.Exec(`INSERT INTO automation_steps (name, event, step, builtin, state, result, at) VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (name, event, step) DO UPDATE SET state = excluded.state, result = excluded.result,
				at = CASE WHEN excluded.at != '' THEN excluded.at ELSE at END`, name, event, step, builtin, state, result, at)
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

// Stamp names where what's kept of the automations stands, cheaply: it
// moves with each write to it. An automation's file changed by hand isn't
// in it.
func Stamp(root string) string {
	var seq int64
	err := read(root, func(tx *store.Tx) error {
		var err error
		seq, err = tx.ChangedAt(Topic)
		return err
	})
	if err != nil {
		return "error: " + err.Error()
	}
	return strconv.FormatInt(seq, 10)
}

// Tally is how an automation's runs stand: how many it's had (held ones
// aside), held, failed, waiting at a wait(), and its batches settling.
type Tally struct {
	Runs     int `json:"runs"`
	Held     int `json:"held,omitempty"`
	Failed   int `json:"failed,omitempty"`
	Waiting  int `json:"waiting,omitempty"`
	Settling int `json:"settling,omitempty"`
}

// Tallies is each automation's tally, by name.
func Tallies(root string) (map[string]Tally, error) {
	out := map[string]Tally{}
	err := read(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT name, state, count(*) FROM automation_runs GROUP BY name, state`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var name, state string
			var n int
			if err := rows.Scan(&name, &state, &n); err != nil {
				rows.Close()
				return err
			}
			t := out[name]
			switch state {
			case Held:
				t.Held += n
			case Failed, Unsure:
				t.Failed += n
				t.Runs += n
			case Waiting:
				t.Waiting += n
				t.Runs += n
			default:
				t.Runs += n
			}
			out[name] = t
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = tx.Query(`SELECT name, count(*) FROM automation_settling GROUP BY name`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var n int
			if err := rows.Scan(&name, &n); err != nil {
				return err
			}
			t := out[name]
			t.Settling = n
			out[name] = t
		}
		return rows.Err()
	})
	return out, err
}

// Step is a step a run took that reached outside it: its builtin
// (notify, converse, wait, show…), where it stands (started, done,
// waiting) and what it came to (notify: what it said; converse: the
// conversation it started; wait: what it waits for, or the event it got).
type Step struct {
	N       int    `json:"n"`
	Builtin string `json:"builtin"`
	State   string `json:"state"`
	Result  string `json:"result,omitempty"`
}

// StepsOf is the steps of name's runs on events, by event, in order.
func StepsOf(root, name string, evs []int64) (map[int64][]Step, error) {
	out := map[int64][]Step{}
	if len(evs) == 0 {
		return out, nil
	}
	want := map[int64]bool{}
	lo, hi := evs[0], evs[0]
	for _, n := range evs {
		want[n] = true
		lo, hi = min(lo, n), max(hi, n)
	}
	err := read(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT event, step, builtin, state, result FROM automation_steps WHERE name = ? AND event BETWEEN ? AND ? ORDER BY event, step`, name, lo, hi)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ev int64
			var st Step
			if err := rows.Scan(&ev, &st.N, &st.Builtin, &st.State, &st.Result); err != nil {
				return err
			}
			if want[ev] {
				out[ev] = append(out[ev], st)
			}
		}
		return rows.Err()
	})
	return out, err
}

// StepCount is how a step of a script has gone, over its runs: where it
// is (At, line:col, as an outline's steps are), its builtin, and how many
// times it was done, begun and never said done (Started), is waiting
// still, and, for a wait, was met or had its time up.
type StepCount struct {
	At       string `json:"at"`
	Builtin  string `json:"builtin"`
	Done     int    `json:"done,omitempty"`
	Started  int    `json:"started,omitempty"`
	Waiting  int    `json:"waiting,omitempty"`
	Met      int    `json:"met,omitempty"`
	TimedOut int    `json:"timedOut,omitempty"`
}

// count adds a step that stands so to c.
func (c *StepCount) count(state, result string, n int) {
	switch {
	case state == "started":
		c.Started += n
	case state == "waiting":
		c.Waiting += n
	case c.Builtin == "wait" && result == "null":
		c.TimedOut += n
	case c.Builtin == "wait":
		c.Met += n
	default:
		c.Done += n
	}
}

// Counts is how each step of name's script has gone over its runs (those
// kept where they were called from), and where its runs failed: line:col,
// and how many.
func Counts(root, name string) ([]StepCount, map[string]int, error) {
	by := map[string]*StepCount{}
	var order []string
	failed := map[string]int{}
	err := read(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT at, builtin, state, result = 'null', count(*) FROM automation_steps WHERE name = ? AND at != '' GROUP BY at, builtin, state, result = 'null'`, name)
		if err != nil {
			return err
		}
		for rows.Next() {
			var at, builtin, state string
			var null bool
			var n int
			if err := rows.Scan(&at, &builtin, &state, &null, &n); err != nil {
				rows.Close()
				return err
			}
			c := by[at+" "+builtin]
			if c == nil {
				c = &StepCount{At: at, Builtin: builtin}
				by[at+" "+builtin] = c
				order = append(order, at+" "+builtin)
			}
			result := ""
			if null {
				result = "null"
			}
			c.count(state, result, n)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = tx.Query(`SELECT error FROM automation_runs WHERE name = ? AND state = ?`, name, Failed)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var why string
			if err := rows.Scan(&why); err != nil {
				return err
			}
			if at := failedAt(name, why); at != "" {
				failed[at]++
			}
		}
		return rows.Err()
	})
	out := []StepCount{}
	for _, k := range order {
		out = append(out, *by[k])
	}
	return out, failed, err
}

// failedAt is where in name's script a run failed, as its error says it
// (name.star:12:9: …): line:col, "" where it doesn't say.
func failedAt(name, why string) string {
	rest, ok := strings.CutPrefix(why, name+".star:")
	if !ok {
		return ""
	}
	parts := strings.SplitN(rest, ":", 3)
	if len(parts) < 3 {
		return ""
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return ""
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return ""
	}
	return parts[0] + ":" + parts[1]
}

// tick keeps that name's schedule was looked at, at.
func tick(root, name string, at time.Time) error {
	return write(root, func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE automations SET ticked = ? WHERE name = ?`, stampOf(at), name)
		return err
	})
}
