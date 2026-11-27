// Package events is what this ws did, and what it heard of, kept: a
// capsule landed, a pull request opened, an artifact promoted, a mission
// ended; and commits landing on a repo's default branch, a teammate's
// pull request asking for your review, a run ending. Each is written
// where ws does the thing, or where the daemon keeps what it heard (in
// ops), with who did it and what caused it, and kept in the workspace's
// records for 90 days. It's what a notification is made from, and what
// an automation will wait for.
//
// An event this ws did is done for you, or by crew and the orchestrator
// on your missions; how something it did turns out (a pull request it
// opened merging) is its too. Only those make notifications. An event it
// heard of (Origin Heard) is what someone else did, as the daemon found
// it when what it keeps moved: it's said by whom, a person where ws knows
// who (ByPerson), and is nobody's to be told of unless an automation says
// so.
package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// Kinds, named thing.happened.
const (
	CapsuleLanded    = "capsule.landed"
	CapsuleLifted    = "capsule.lifted"
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
	CrewStarted      = "crew.started"
	CrewDone         = "crew.done"  // crew handed their work over
	CrewAsked        = "crew.asked" // crew asked you something
	MissionFlagged   = "mission.flagged"
	MissionWaiting   = "mission.waiting" // on something outside it
	BundleHanded     = "bundle.handed"   // a verification bundle, for you
	ChecksJudged     = "checks.judged"   // a step's checks, run and judged

	ConversationAnswered = "conversation.answered" // its agent said something
	ProposalMade         = "proposal.made"
	ProposalAccepted     = "proposal.accepted"
	ProposalDeclined     = "proposal.declined"

	// Heard of: what the daemon found had moved.
	CommitsLanded      = "commits.landed"       // a repo's default branch moved: its commits
	DocChanged         = "doc.changed"          // a space's document changed on its default branch
	ThingAdded         = "thing.added"          // a thing (a folder with a thing.toml) added to a space
	ThingChanged       = "thing.changed"        // a space's thing changed
	PRReviewRequested  = "pr.review_requested"  // your review asked for on a pull request
	PRApproved         = "pr.approved"          // a pull request approved
	PRChangesRequested = "pr.changes_requested" // changes asked for on a pull request
	PRChecksPassed     = "pr.checks_passed"     // a pull request's checks went green
	PRClosed           = "pr.closed"            // a pull request closed without merging
	RunStarted         = "run.started"
	RunUp              = "run.up" // a service answering on its port
	RunEnded           = "run.ended"
	// ScheduleDue is an automation's time come (every()): its own, which
	// only it's for (Automation names it).
	ScheduleDue = "schedule.due"
	// ActionRun is one of an automation's actions run by you (from Find,
	// a menu): its own, which only it's for (Automation names it, Title
	// the action), on what About names first, if anything, with Input.
	ActionRun = "action.run"
)

// Origins: whether this ws did it, or heard of it.
const (
	Did   = ""
	Heard = "heard"
)

// Kinds is every kind there is, in the order they're listed.
var Kinds = []string{
	CapsuleLifted, CapsuleLanded, CapsuleBurned, CommitsLanded,
	PROpened, PRReviewRequested, PRApproved, PRChangesRequested, PRChecksFailed, PRChecksPassed, PRMerged, PRClosed,
	MissionStarted, MissionFlagged, MissionWaiting, MissionEnded,
	CrewStarted, CrewAsked, CrewDone, BundleHanded, ChecksJudged,
	ConversationAnswered, ProposalMade, ProposalAccepted, ProposalDeclined,
	ArtifactAdded, ArtifactPromoted,
	DocAdded, DocAmended, DocChanged,
	ThingAdded, ThingChanged,
	RunStarted, RunUp, RunEnded,
	DecisionMade,
	ScheduleDue, ActionRun,
}

// Known is whether kind is one there is.
func Known(kind string) bool {
	return slices.Contains(Kinds, kind)
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
	// ByGitHub is for what GitHub says happened without saying who did
	// it (your review asked for, a pull request approved).
	ByGitHub = "github"
)

// ByCrew is crew id, as By says it.
func ByCrew(id string) string { return "crew:" + id }

// ByWorkflow is a workflow, as By says it.
func ByWorkflow(name string) string { return "workflow:" + name }

// ByPerson is someone in the workspace's repos, by their key in its
// roster (team.Person), as By says it: person:brendan.
func ByPerson(key string) string { return "person:" + key }

// ByPlugin is a plugin, as By says it, for what it told ws of without
// saying who did it: plugin:jira.
func ByPlugin(name string) string { return "plugin:" + name }

// PersonOf is the person By names, or ok false for one it doesn't.
func PersonOf(by string) (key string, ok bool) {
	key, ok = strings.CutPrefix(by, "person:")
	return key, ok && key != ""
}

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
	// Chain is the automations that led to it, first first: one that
	// started the conversation it's from, and any that set that one off.
	// An automation doesn't run on what it's in the chain of, or on what's
	// MaxChain deep (internal/automation).
	Chain []string `json:"chain,omitempty"`
	// RunN is which run of a task a run's event is of (its #n), 0 for a
	// service's.
	RunN int `json:"runN,omitempty"`
	// Automation is the automation a schedule.due is for.
	Automation string `json:"automation,omitempty"`
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

	// Origin is whether this ws did it (Did, as every event was before
	// any was heard), or heard of it (Heard).
	Origin string `json:"origin,omitempty"`
	// Title is what it's about is called: a pull request's title, a
	// thing's, a document's.
	Title string `json:"title,omitempty"`
	// Branch is the branch it's on: a default branch commits landed on.
	Branch string `json:"branch,omitempty"`
	// Commits are a default branch's new commits, oldest first, for
	// commits.landed; and the commit a document or thing changed in, for
	// theirs.
	Commits []Commit `json:"commits,omitempty"`
	// Paths are the files the commits touched (at most PathsMax).
	Paths []string `json:"paths,omitempty"`
	// Thing is a space's thing, by its path (records:sketches/flow/), and
	// ThingKind its kind (sketch).
	Thing     string `json:"thing,omitempty"`
	ThingKind string `json:"thingKind,omitempty"`
	// Run is a run's ref (web.ground.demo), and Status how it ended
	// (passed, failed, stopped, gone) for run.ended.
	Run    string `json:"run,omitempty"`
	Status string `json:"status,omitempty"`
	// Conversation is a conversation's id (t3), in Mission's scope or the
	// workspace's; Proposal one of its proposals, by number, and its kind
	// (a plan, a mission, jira:issues).
	Conversation string `json:"conversation,omitempty"`
	Proposal     int    `json:"proposal,omitempty"`
	ProposalKind string `json:"proposalKind,omitempty"`
	// Changes are what changed in a thing a plugin keeps, field by field,
	// for thing.changed: a Jira issue's status, from In Progress to Done.
	Changes []FieldChange `json:"changes,omitempty"`
	// Input is what you gave an action as you ran it (action.run), by its
	// fields.
	Input map[string]any `json:"input,omitempty"`
}

// FieldChange is one field of a thing, as it was and as it is. A long
// one (text, markdown) is said to have changed, without either.
type FieldChange struct {
	Field string `json:"field"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}

// Commit is one commit an event's about.
type Commit struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
	// By is its author, as By says it: a person (ByPerson), or you.
	By string `json:"by"`
}

// PathsMax is the most paths an event keeps.
const PathsMax = 200

// IsHeard is whether this ws heard of it, rather than did it.
func (e Event) IsHeard() bool { return e.Origin == Heard }

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
	for _, r := range []string{e.PR, e.Doc, e.Thing, e.Artifact, e.Run, e.Mission} {
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
	`), store.SQL(`
		-- What this ws heard of, besides what it did: origin is 'heard'
		-- for those, '' for what it did.
		ALTER TABLE events ADD COLUMN origin TEXT NOT NULL DEFAULT '';
		CREATE INDEX events_origin ON events (origin, n);
	`), store.SQL(`
		-- Where what's heard of was last heard to: a ground's head, as the
		-- commits that landed on it were written.
		CREATE TABLE events_heard (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID;
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
		if e.IsHeard() {
			return e, fmt.Errorf("events: a heard %s says by whom", e.Kind)
		}
		e.By = ByYou
	}
	if e.Origin != Did && e.Origin != Heard {
		return e, fmt.Errorf("events: no origin %q", e.Origin)
	}
	e.N = 0
	line, err := json.Marshal(e)
	if err != nil {
		return e, err
	}
	res, err := tx.Exec(`INSERT INTO events (at, kind, by, cause, mission, event, origin) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		stampOf(e.At), e.Kind, e.By, e.Cause, e.Mission, string(line), e.Origin)
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
	// Origin is only what this ws did (Did, with Only set) or heard of
	// (Heard); both when Only's unset.
	Origin string
	Only   bool
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
	if f.Only {
		where = append(where, "origin = ?")
		args = append(args, f.Origin)
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

// HeardTo is where what's heard of under key was last heard to, in tx:
// "" for never.
func HeardTo(tx *store.Tx, key string) (string, error) {
	var v string
	err := tx.QueryRow(`SELECT value FROM events_heard WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// SetHeardTo says what's heard of under key has been heard to value, in
// tx: written with the events it was heard as, so neither is without the
// other.
func SetHeardTo(tx *store.Tx, key, value string) error {
	_, err := tx.Exec(`INSERT INTO events_heard (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// Says is what an event says, in a line: what changed in a thing,
// field by field; a document as it's called and what became of it; how a
// run ended; else its text, or what it's about is called.
func (e Event) Says() string {
	switch {
	case len(e.Changes) > 0:
		var cs []string
		for _, c := range e.Changes {
			switch {
			case c.From == "" && c.To == "":
				cs = append(cs, c.Field+" changed")
			case c.From == "":
				cs = append(cs, c.Field+" → "+c.To)
			case c.To == "":
				cs = append(cs, c.Field+" cleared (was "+c.From+")")
			default:
				cs = append(cs, c.Field+" "+c.From+" → "+c.To)
			}
		}
		return strings.TrimPrefix(e.Title+": ", ": ") + strings.Join(cs, ", ")
	case e.Kind == DocChanged && e.Status != "":
		return e.Title + " (" + e.Status + ")"
	case e.Kind == RunEnded && e.Status != "":
		return e.Status
	case e.Kind == ScheduleDue:
		return e.Automation + "'s time: " + e.Title
	case e.Kind == ActionRun && len(e.About) > 0:
		return e.Title + ", on " + e.About[0]
	case e.Kind == ActionRun:
		return e.Title
	case e.Text != "":
		return e.Text
	}
	return e.Title
}
