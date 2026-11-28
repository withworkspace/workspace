// Package outbox is what's been done here and has still to be told to
// somewhere slow: a reply on a pull request, which GitHub takes a second
// or more over. It's shown here at once, kept in the workspace's records,
// and sent behind by the daemon (Queue): a lane's writes in the order they
// were made, one at a time, a few lanes at once.
//
// A write that's refused isn't dropped: it stays, failed, with why, until
// it's sent again or let go of. Nothing made here is lost because the
// other end said no after the page had moved on.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

func init() {
	store.Define("outbox", store.SQL(`
		CREATE TABLE outbox (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			lane TEXT NOT NULL,           -- what its writes take turns on: a pull request
			kind TEXT NOT NULL,           -- who sends it (Queue.Handle)
			op TEXT NOT NULL,             -- the write, as its kind wrote it (JSON)
			doing TEXT NOT NULL,          -- the write in words: your reply on cache.go:3
			state TEXT NOT NULL,
			error TEXT NOT NULL DEFAULT '', -- why it failed
			queued TEXT NOT NULL
		);
		CREATE INDEX outbox_lane ON outbox (lane, id);
	`))
}

// How a write stands.
const (
	Waiting = "waiting" // made here, not yet sent
	Sending = "sending" // being sent: a daemon that stopped here may have made it
	Sent    = "sent"    // made there: kept until what it changed has been read again
	Failed  = "failed"  // refused, or it couldn't be said whether it was made
)

// Topic is what a write to the outbox says it changed: the workspace, whose
// foot says how many are on their way and how many didn't get there.
const Topic = "workspace"

// ErrNotFound is the answer for a write that isn't waiting here, or isn't
// as it was asked of (only one that failed is sent again, or let go of).
var ErrNotFound = errors.New("nothing waiting to send")

// Item is one write.
type Item struct {
	ID   int64
	Lane string
	Kind string
	// Op is the write, as its kind wrote it.
	Op json.RawMessage
	// Doing is the write in words, to say it by.
	Doing  string
	State  string
	Error  string
	Queued time.Time
}

// Unsent: it hasn't been made there yet, and may still be: it's shown
// here as if it had.
func (it Item) Unsent() bool { return it.State == Waiting || it.State == Sending }

func db(root string) (*store.DB, error) { return store.Open(root) }

func write(root string, fn func(tx *store.Tx) error) error {
	d, err := db(root)
	if err != nil {
		return err
	}
	return d.Write(context.Background(), fn)
}

// add keeps a write, waiting.
func add(root, lane, kind, doing string, op any, at time.Time) (Item, error) {
	data, err := json.Marshal(op)
	if err != nil {
		return Item{}, err
	}
	it := Item{Lane: lane, Kind: kind, Op: data, Doing: doing, State: Waiting, Queued: at.UTC()}
	err = write(root, func(tx *store.Tx) error {
		res, err := tx.Exec(`INSERT INTO outbox (lane, kind, op, doing, state, queued) VALUES (?, ?, ?, ?, ?, ?)`,
			lane, kind, string(data), doing, Waiting, it.Queued.Format(store.Stamp))
		if err != nil {
			return err
		}
		it.ID, err = res.LastInsertId()
		tx.Changed(Topic)
		return err
	})
	return it, err
}

// In is what's kept for a lane, in the order it was made: sent or not,
// failed or not. With lane "" it's every lane's.
func In(root, lane string) ([]Item, error) {
	d, err := db(root)
	if err != nil {
		return nil, err
	}
	var out []Item
	err = d.Read(context.Background(), func(tx *store.Tx) error {
		q, args := `SELECT id, lane, kind, op, doing, state, error, queued FROM outbox ORDER BY id`, []any{}
		if lane != "" {
			q, args = `SELECT id, lane, kind, op, doing, state, error, queued FROM outbox WHERE lane = ? ORDER BY id`, []any{lane}
		}
		rows, err := tx.Query(q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var it Item
			var op, queued string
			if err := rows.Scan(&it.ID, &it.Lane, &it.Kind, &op, &it.Doing, &it.State, &it.Error, &queued); err != nil {
				return err
			}
			it.Op = json.RawMessage(op)
			it.Queued, _ = time.Parse(store.Stamp, queued)
			out = append(out, it)
		}
		return rows.Err()
	})
	return out, err
}

// Get is one write, by its id.
func Get(root string, id int64) (Item, error) {
	all, err := In(root, "")
	if err != nil {
		return Item{}, err
	}
	for _, it := range all {
		if it.ID == id {
			return it, nil
		}
	}
	return Item{}, ErrNotFound
}

// Standing is how many writes are on their way, and how many failed.
func Standing(root string) (unsent, failed int, err error) {
	all, err := In(root, "")
	for _, it := range all {
		switch {
		case it.Unsent():
			unsent++
		case it.State == Failed:
			failed++
		}
	}
	return unsent, failed, err
}

// set moves a write to a state, from one of the states given (any, for
// none), and says whether it was in one. told: the foot's count changed.
func set(root string, id int64, state, why string, told bool, from ...string) (bool, error) {
	moved := false
	err := write(root, func(tx *store.Tx) error {
		q, args := `UPDATE outbox SET state = ?, error = ? WHERE id = ?`, []any{state, why, id}
		if len(from) > 0 {
			q += ` AND state IN (?` // one at least
			args = append(args, from[0])
			for _, f := range from[1:] {
				q += `, ?`
				args = append(args, f)
			}
			q += `)`
		}
		res, err := tx.Exec(q, args...)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if moved = n > 0; moved && told {
			tx.Changed(Topic)
		}
		return err
	})
	return moved, err
}

// remove forgets writes: sent, or let go of. Only those in state, if one's
// given. It says how many it forgot.
func remove(root, state string, ids ...int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	gone := 0
	err := write(root, func(tx *store.Tx) error {
		for _, id := range ids {
			q, args := `DELETE FROM outbox WHERE id = ?`, []any{id}
			if state != "" {
				q, args = q+` AND state = ?`, append(args, state)
			}
			res, err := tx.Exec(q, args...)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			gone += int(n)
		}
		if gone > 0 && state == Failed {
			tx.Changed(Topic)
		}
		return nil
	})
	return gone, err
}

// touch says what's kept has changed, for what's shown of it.
func touch(root string) {
	_ = write(root, func(tx *store.Tx) error {
		tx.Changed(Topic)
		return nil
	})
}

func (it Item) String() string { return fmt.Sprintf("%s (%s)", it.Doing, it.Lane) }
