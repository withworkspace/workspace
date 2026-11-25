// Package notification is what happened that you'd want to know, kept: a
// feed of yours, each read or not, dismissed or not. A need (ops.Needs) is
// what can't move without you, worked out each time and never kept; a
// notification is what happened, and stays as the record of it.
//
// Most are made from what this ws did (events), as it's kept and in the
// same write: a mission of yours ended or was started for you, a pull
// request this ws opened merged. What you did yourself isn't told back to
// you. The rest are added by whoever knows (Add): a plugin's update is
// out, ws was upgraded and the daemon restarted onto it.
//
// Which sources reach the feed, and which the desktop too, is yours to say
// ([notifications] in ws.local.toml): that's applied where they're read
// (ops), so nothing's lost by muting a source and changing your mind.
package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/store"
)

// Sources: where a notification comes from, which is what's routed.
const (
	SourceMissions = "missions" // a mission ended, or was started for you
	SourcePulls    = "pulls"    // a pull request this ws opened merged
	SourcePlugins  = "plugins"  // a plugin's update is out
	SourceWS       = "ws"       // ws was upgraded, and the daemon restarted onto it
)

// Sources is every source there is, in the order they're listed.
var Sources = []string{SourceMissions, SourcePulls, SourcePlugins, SourceWS}

// Kinds that aren't an event's: what's added by whoever knows.
const (
	PluginUpdate = "plugin.update"
	WSUpgraded   = "ws.upgraded"
)

// Topic is what a page follows to hear the feed changed.
const Topic = "notifications"

// Notification is one thing that happened.
type Notification struct {
	// N is its number, in the order they were kept: n12 is the 12th.
	N      int64     `json:"n"`
	At     time.Time `json:"at"`
	Source string    `json:"source"`
	// Kind is the event's it was made from (events.MissionEnded…), or one
	// of this package's (PluginUpdate, WSUpgraded).
	Kind string `json:"kind"`
	// Text says what happened, as a sentence.
	Text string `json:"text"`
	// Refs is what it's about, as ws names them, the first what it opens.
	Refs []string `json:"refs,omitempty"`
	// By is who did it, as an event says (events.ByWS…): "" where nobody
	// here did.
	By string `json:"by,omitempty"`
	// Event is the event it was made from: 0 for none.
	Event int64 `json:"event,omitempty"`
	// Attention marks one that went wrong: told apart in the feed, though
	// nothing waits on you.
	Attention bool `json:"attention,omitempty"`
	// Read and Dismissed are when you read it and put it away: nil while
	// you haven't.
	Read      *time.Time `json:"read,omitempty"`
	Dismissed *time.Time `json:"dismissed,omitempty"`
}

// Ref is how a notification is named: n12.
func (n Notification) Ref() string { return fmt.Sprintf("n%d", n.N) }

func init() {
	store.Define("notification", store.SQL(`
		-- What happened that you'd want to know: a feed, kept.
		CREATE TABLE notifications (
			n INTEGER PRIMARY KEY AUTOINCREMENT,
			at TEXT NOT NULL,
			source TEXT NOT NULL,
			read TEXT NOT NULL DEFAULT '',        -- when you read it; '' while you haven't
			dismissed TEXT NOT NULL DEFAULT '',   -- when you put it away
			told TEXT NOT NULL DEFAULT '',        -- when a page told the desktop of it
			notification TEXT NOT NULL            -- the notification, as JSON
		);
		CREATE INDEX notifications_unread ON notifications (source, n) WHERE read = '' AND dismissed = '';
		-- What the feed remembers to know what's news: the ws last run.
		CREATE TABLE notification_state (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID;
	`))
	events.OnAppend(func(tx *store.Tx, e events.Event) error {
		n, ok := Of(e)
		if !ok {
			return nil
		}
		_, err := AddIn(tx, n)
		return err
	})
}

// Of is the notification an event is, if it's one: something done for
// you, not by you.
func Of(e events.Event) (Notification, bool) {
	if e.By == events.ByYou {
		return Notification{}, false
	}
	n := Notification{At: e.At, Kind: e.Kind, By: e.By, Event: e.N}
	switch e.Kind {
	case events.MissionEnded:
		n.Source, n.Refs = SourceMissions, []string{e.Mission}
		n.Text = "Mission " + e.Mission + " ended"
		if e.Text != "" {
			n.Text += ": " + e.Text
		}
	case events.MissionStarted:
		n.Source, n.Refs = SourceMissions, []string{e.Mission}
		n.Text = who(e.By) + " started a mission"
		if e.Text != "" {
			n.Text += ": " + e.Text
		}
	case events.PRMerged:
		n.Source = SourcePulls
		n.Text = "A pull request merged"
		if e.PR != "" {
			n.Refs = append(n.Refs, e.PR)
			n.Text = e.PR + " merged"
		}
		if e.Mission != "" {
			n.Refs = append(n.Refs, e.Mission)
		}
	default:
		return Notification{}, false
	}
	return n, true
}

// who is who did it, to start a sentence with.
func who(by string) string {
	switch {
	case by == events.ByOrchestrator:
		return "The orchestrator"
	case by == events.ByWS:
		return "ws"
	case strings.HasPrefix(by, "crew:"):
		return "Crew " + strings.TrimPrefix(by, "crew:")
	case strings.HasPrefix(by, "workflow:"):
		return strings.TrimPrefix(by, "workflow:")
	}
	return "Someone"
}

func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

func timeOf(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(store.Stamp, s)
	if err != nil {
		return nil
	}
	return &t
}

// AddIn keeps n in tx, numbered. At defaults to now.
func AddIn(tx *store.Tx, n Notification) (Notification, error) {
	if strings.TrimSpace(n.Text) == "" {
		return n, errors.New("notification: it says nothing")
	}
	if !known(n.Source) {
		return n, fmt.Errorf("notification: no source %q", n.Source)
	}
	if n.At.IsZero() {
		n.At = time.Now()
	}
	n.N, n.Read, n.Dismissed = 0, nil, nil
	line, err := json.Marshal(n)
	if err != nil {
		return n, err
	}
	res, err := tx.Exec(`INSERT INTO notifications (at, source, notification) VALUES (?, ?, ?)`, stampOf(n.At), n.Source, string(line))
	if err != nil {
		return n, err
	}
	if n.N, err = res.LastInsertId(); err != nil {
		return n, err
	}
	tx.Changed(Topic, "workspace")
	return n, nil
}

func known(source string) bool {
	for _, s := range Sources {
		if s == source {
			return true
		}
	}
	return false
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

// Add keeps n in root's records, in a transaction of its own.
func Add(root string, n Notification) (Notification, error) {
	err := write(root, func(tx *store.Tx) error {
		var err error
		n, err = AddIn(tx, n)
		return err
	})
	return n, err
}

// Filter is which notifications are listed.
type Filter struct {
	// Unread is only those you haven't read.
	Unread bool
	// Dismissed has those you put away listed too.
	Dismissed bool
	// Muted is the sources left out: those routed nowhere.
	Muted []string
	// Limit is the most listed, the latest; 0 for no limit.
	Limit int
}

func (f Filter) where() (string, []any) {
	var where []string
	var args []any
	if f.Unread {
		where = append(where, "read = ''")
	}
	if !f.Dismissed {
		where = append(where, "dismissed = ''")
	}
	if len(f.Muted) > 0 {
		where = append(where, "source NOT IN ("+strings.TrimSuffix(strings.Repeat("?, ", len(f.Muted)), ", ")+")")
		for _, s := range f.Muted {
			args = append(args, s)
		}
	}
	if len(where) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

// List is the notifications f picks, newest first.
func List(root string, f Filter) ([]Notification, error) {
	var out []Notification
	err := read(root, func(tx *store.Tx) error {
		where, args := f.where()
		q := `SELECT n, read, dismissed, notification FROM notifications` + where + ` ORDER BY n DESC`
		if f.Limit > 0 {
			q += fmt.Sprintf(" LIMIT %d", f.Limit)
		}
		rows, err := tx.Query(q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var readAt, dismissed, line string
			if err := rows.Scan(&id, &readAt, &dismissed, &line); err != nil {
				return err
			}
			var n Notification
			if json.Unmarshal([]byte(line), &n) != nil {
				continue
			}
			n.N, n.Read, n.Dismissed = id, timeOf(readAt), timeOf(dismissed)
			out = append(out, n)
		}
		return rows.Err()
	})
	return out, err
}

// Unread is how many you haven't read, but for the muted sources.
func Unread(root string, muted []string) (int, error) {
	var count int
	err := read(root, func(tx *store.Tx) error {
		where, args := Filter{Unread: true, Muted: muted}.where()
		return tx.QueryRow(`SELECT count(*) FROM notifications`+where, args...).Scan(&count)
	})
	return count, err
}

// mark stamps a column of the notifications numbered ns, where it isn't
// stamped yet: every one, but for the muted sources, when ns is nil.
func mark(root, column string, ns []int64, muted []string) (int, error) {
	if ns != nil && len(ns) == 0 {
		return 0, nil
	}
	var done int64
	err := write(root, func(tx *store.Tx) error {
		q := `UPDATE notifications SET ` + column + ` = ? WHERE ` + column + ` = ''`
		args := []any{stampOf(time.Now())}
		if ns != nil {
			q += " AND n IN (" + strings.TrimSuffix(strings.Repeat("?, ", len(ns)), ", ") + ")"
			for _, n := range ns {
				args = append(args, n)
			}
		} else if len(muted) > 0 {
			q += " AND source NOT IN (" + strings.TrimSuffix(strings.Repeat("?, ", len(muted)), ", ") + ")"
			for _, s := range muted {
				args = append(args, s)
			}
		}
		res, err := tx.Exec(q, args...)
		if err != nil {
			return err
		}
		if done, _ = res.RowsAffected(); done > 0 {
			tx.Changed(Topic, "workspace")
		}
		return nil
	})
	return int(done), err
}

// MarkRead says you've read the notifications numbered ns, and answers
// how many that was news of. ReadAll says so of every one.
func MarkRead(root string, ns []int64) (int, error) {
	if ns == nil {
		ns = []int64{}
	}
	return mark(root, "read", ns, nil)
}

// ReadAll says you've read every one, but for the muted sources.
func ReadAll(root string, muted []string) (int, error) { return mark(root, "read", nil, muted) }

// Dismiss puts the notifications numbered ns away: read, and out of the
// feed.
func Dismiss(root string, ns []int64) (int, error) {
	if ns == nil {
		ns = []int64{}
	}
	if _, err := mark(root, "read", ns, nil); err != nil {
		return 0, err
	}
	return mark(root, "dismissed", ns, nil)
}

// Claim answers which of the notifications numbered ns no page has told
// the desktop of yet, and says they've been told: so it's once, however
// many pages are open.
func Claim(root string, ns []int64) ([]int64, error) {
	fresh := []int64{}
	err := write(root, func(tx *store.Tx) error {
		at := stampOf(time.Now())
		for _, n := range ns {
			res, err := tx.Exec(`UPDATE notifications SET told = ? WHERE n = ? AND told = '' AND read = '' AND dismissed = ''`, at, n)
			if err != nil {
				return err
			}
			if done, _ := res.RowsAffected(); done > 0 {
				fresh = append(fresh, n)
			}
		}
		return nil
	})
	return fresh, err
}

// Was answers what the feed remembered under key, and remembers value
// from now on: what's news is what differs.
func Was(root, key, value string) (was string, err error) {
	err = write(root, func(tx *store.Tx) error {
		err := tx.QueryRow(`SELECT value FROM notification_state WHERE key = ?`, key).Scan(&was)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if was == value {
			return nil
		}
		_, err = tx.Exec(`INSERT INTO notification_state (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
		return err
	})
	return was, err
}
