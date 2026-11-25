// Package events is what this ws did, kept: a capsule landed, a pull
// request opened, an artifact promoted, a mission ended. Each is written
// where ws does the thing (in ops), with who did it and what caused it,
// and kept in the workspace's records for 90 days. It's what a notification
// is made from, and what a workflow will wait for.
//
// An event is only ever something this ws did, for you or for crew and the
// orchestrator on your missions: nothing is shared, and no event is one
// this ws only heard of. How something this ws did turns out (a pull
// request it opened merging) is its too.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// Kinds, named thing.happened.
const (
	CapsuleLanded    = "capsule.landed"
	CapsuleBurned    = "capsule.burned"
	PROpened         = "pr.opened"
	PRMerged         = "pr.merged"
	PRChecksFailed   = "pr.checks_failed"
	MissionStarted   = "mission.started"
	MissionEnded     = "mission.ended"
	ArtifactAdded    = "artifact.added"
	ArtifactPromoted = "artifact.promoted"
	DocAdded         = "doc.added"
	DocAmended       = "doc.amended"
	DecisionMade     = "decision.made"
)

// Kinds is every kind there is, in the order they're listed.
var Kinds = []string{
	CapsuleLanded, CapsuleBurned,
	PROpened, PRMerged, PRChecksFailed,
	MissionStarted, MissionEnded,
	ArtifactAdded, ArtifactPromoted,
	DocAdded, DocAmended,
	DecisionMade,
}

// Known is whether kind is one there is.
func Known(kind string) bool {
	for _, k := range Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// Keep is how long an event is kept.
const Keep = 90 * 24 * time.Hour

// Topic is what a page follows to hear of new events.
const Topic = "events"

// Who did it, as By says it: you, the orchestrator, ws itself, crew by
// id (crew:c1), or a workflow by name (workflow:<name>).
const (
	ByYou          = "you"
	ByOrchestrator = "orchestrator"
	ByWS           = "ws"
)

// ByCrew is crew id, as By says it.
func ByCrew(id string) string { return "crew:" + id }

// ByWorkflow is a workflow, as By says it.
func ByWorkflow(name string) string { return "workflow:" + name }

// Event is one thing this ws did.
type Event struct {
	// N is its number, in the order they were kept: e12 is the 12th.
	N    int64     `json:"n"`
	At   time.Time `json:"at"`
	Kind string    `json:"kind"`
	// By is who did it (ByYou, ByCrew…).
	By string `json:"by"`
	// Cause is what caused it, where something did: the orchestrator's
	// run whose agent did it, or (later) a workflow's run.
	Cause string `json:"cause,omitempty"`
	// What it's about, as ws names them.
	Repo     string `json:"repo,omitempty"`
	Capsule  string `json:"capsule,omitempty"` // its name, in Repo
	Mission  string `json:"mission,omitempty"`
	PR       string `json:"pr,omitempty"`       // repo#12
	Doc      string `json:"doc,omitempty"`      // repo:path
	Artifact string `json:"artifact,omitempty"` // a7
	// About is what else it's about: the things a mission is about.
	About []string `json:"about,omitempty"`
	// Text says what happened, in a few words.
	Text string `json:"text,omitempty"`
}

// Ref is how an event is named: e12.
func (e Event) Ref() string { return fmt.Sprintf("e%d", e.N) }

// Refs is what the event is about, as refs, the most particular first.
func (e Event) Refs() []string {
	var out []string
	if e.Capsule != "" {
		out = append(out, e.Repo+"/"+e.Capsule)
	} else if e.Repo != "" && e.PR == "" && e.Doc == "" {
		out = append(out, e.Repo)
	}
	for _, r := range []string{e.PR, e.Doc, e.Artifact, e.Mission} {
		if r != "" {
			out = append(out, r)
		}
	}
	return append(out, e.About...)
}

func init() {
	store.Define("events", store.SQL(`
		-- What this ws did: append-only, kept for 90 days (Keep).
		CREATE TABLE events (
			n INTEGER PRIMARY KEY AUTOINCREMENT,
			at TEXT NOT NULL,
			kind TEXT NOT NULL,
			by TEXT NOT NULL,
			cause TEXT NOT NULL,
			mission TEXT NOT NULL,
			event TEXT NOT NULL             -- the event, as JSON
		);
		CREATE INDEX events_at ON events (at);
		CREATE INDEX events_kind ON events (kind, n);
		CREATE INDEX events_mission ON events (mission, n) WHERE mission != '';
	`))
}

func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

// Append keeps e in tx, numbered, and lets go of what's older than Keep.
// At defaults to now.
func Append(tx *store.Tx, e Event) (Event, error) {
	if !Known(e.Kind) {
		return e, fmt.Errorf("events: no kind %q", e.Kind)
	}
	if e.At.IsZero() {
		e.At = time.Now()
	}
	if e.By == "" {
		e.By = ByYou
	}
	e.N = 0
	line, err := json.Marshal(e)
	if err != nil {
		return e, err
	}
	res, err := tx.Exec(`INSERT INTO events (at, kind, by, cause, mission, event) VALUES (?, ?, ?, ?, ?, ?)`,
		stampOf(e.At), e.Kind, e.By, e.Cause, e.Mission, string(line))
	if err != nil {
		return e, err
	}
	if e.N, err = res.LastInsertId(); err != nil {
		return e, err
	}
	if _, err := tx.Exec(`DELETE FROM events WHERE at < ?`, stampOf(e.At.Add(-Keep))); err != nil {
		return e, err
	}
	tx.Changed(Topic)
	for _, fn := range after {
		if err := fn(tx, e); err != nil {
			return e, err
		}
	}
	return e, nil
}

// after is what's run for each event as it's kept, in its transaction.
var after []func(tx *store.Tx, e Event) error

// OnAppend has fn run for each event as it's kept, in the transaction
// that keeps it: what's made from events (a notification) is made with
// them, or not at all. Called from a package's init.
func OnAppend(fn func(tx *store.Tx, e Event) error) { after = append(after, fn) }

// Emit keeps e in root's records, in a transaction of its own.
func Emit(root string, e Event) (Event, error) {
	db, err := store.Open(root)
	if err != nil {
		return e, err
	}
	err = db.Write(context.Background(), func(tx *store.Tx) error {
		var err error
		e, err = Append(tx, e)
		return err
	})
	return e, err
}

// Filter is which events are listed.
type Filter struct {
	Since time.Time // at or after; zero for every one kept
	After int64     // numbered after this one
	Kinds []string  // any of these kinds; none for every kind
	// Mission is a mission's own, by slug.
	Mission string
	// Limit is the most listed, the latest; 0 for no limit.
	Limit int
}

// List is the events f picks, oldest first.
func List(root string, f Filter) ([]Event, error) {
	db, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	var out []Event
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		out, err = ListIn(tx, f)
		return err
	})
	return out, err
}

// ListIn is List, in tx.
func ListIn(tx *store.Tx, f Filter) ([]Event, error) {
	var where []string
	var args []any
	if !f.Since.IsZero() {
		where = append(where, "at >= ?")
		args = append(args, stampOf(f.Since))
	}
	if f.After > 0 {
		where = append(where, "n > ?")
		args = append(args, f.After)
	}
	if len(f.Kinds) > 0 {
		where = append(where, "kind IN ("+strings.TrimSuffix(strings.Repeat("?, ", len(f.Kinds)), ", ")+")")
		for _, k := range f.Kinds {
			args = append(args, k)
		}
	}
	if f.Mission != "" {
		where = append(where, "mission = ?")
		args = append(args, f.Mission)
	}
	q := `SELECT n, event FROM events`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY n DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := tx.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var n int64
		var line string
		if err := rows.Scan(&n, &line); err != nil {
			return nil, err
		}
		var e Event
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		e.N = n
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
