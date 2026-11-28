// Package lessons is what crew keep having to learn about where they work,
// gathered from their handoffs: a test that has to be run a certain way, a
// step a fresh capsule needs, a tool on this machine that gets in the way.
// Each lesson is one thing (one cause, or what one change would fix), with
// every crew that ran into it (its sightings, in the crew's words) and what
// would stop the next crew running into it at all: a change to ws, to the
// repo's capsule setup, to the repo, a line in its agent instructions, or
// something only you can do.
//
// A handoff says it under Environment (mission.Handoff), which crew sort
// out from Learned; one kept from before that was asked for is read whole.
// The daemon reads each handoff once (ops.ReadLessons), and an agent says
// which known lesson each thing is, or what a new one would fix. A lesson
// two crew have run into is a recommendation: one is only a note.
package lessons

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

func init() {
	store.Define("lessons", store.SQL(`
		CREATE TABLE lessons (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			fix TEXT NOT NULL,                 -- who or what should change: a Fix
			change TEXT NOT NULL,              -- what to change, for you to act on
			why TEXT NOT NULL,                 -- why crew run into it
			repo TEXT NOT NULL,                -- the repo it's about; '' for this machine, or ws
			state TEXT NOT NULL,               -- open, fixed or dismissed
			state_at TEXT NOT NULL,
			again TEXT NOT NULL DEFAULT '',    -- when crew ran into it after it was marked fixed
			mission TEXT NOT NULL DEFAULT '',  -- the mission made to fix it
			created TEXT NOT NULL
		);
		-- Each crew that ran into a lesson, once: what it said of it.
		CREATE TABLE lesson_sightings (
			lesson INTEGER NOT NULL,
			mission TEXT NOT NULL,
			crew TEXT NOT NULL,
			repo TEXT NOT NULL,
			said TEXT NOT NULL,
			at TEXT NOT NULL,                  -- when the crew handed off
			PRIMARY KEY (lesson, mission, crew)
		) WITHOUT ROWID;
		-- Each crew's handoff read, as it was handed off then: a crew that
		-- hands off again is read again.
		CREATE TABLE lessons_read (
			mission TEXT NOT NULL,
			crew TEXT NOT NULL,
			handed_off TEXT NOT NULL,
			at TEXT NOT NULL,
			found INTEGER NOT NULL,
			error TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (mission, crew)
		) WITHOUT ROWID;
	`))
}

// Topic is what a change to the lessons says changed.
const Topic = "lessons"

// Who or what should change, so crew stop running into a lesson: what
// takes it away first (ws, the capsule setup, the repo), and only telling
// agents of it (Docs) when nothing should change.
const (
	FixWS    = "ws"    // ws itself: how it makes capsules, sets crew's environment, runs checks
	FixSetup = "setup" // the repo's capsule setup ([repos.<name>] after_create)
	FixRepo  = "repo"  // the repo's own code, scripts or tests
	FixDocs  = "docs"  // a line in the repo's agent instructions (CLAUDE.md, AGENTS.md)
	FixYou   = "you"   // something only you can do, on this machine or your accounts
)

// Fixes are the fixes, in the order they're preferred.
var Fixes = []string{FixWS, FixSetup, FixRepo, FixDocs, FixYou}

// A lesson's states.
const (
	Open      = "open"
	Fixed     = "fixed"     // you fixed it: crew running into it again open it again
	Dismissed = "dismissed" // nothing to do: kept so it's known, never raised again
)

// Recommended is how many crew have to have run into a lesson for it to be
// a recommendation, not a note.
const Recommended = 2

// QuietAfter is how long a lesson nobody's run into is quiet for: fixed,
// maybe, without anyone saying so.
const QuietAfter = 30 * 24 * time.Hour

// Lesson is one thing crew keep running into.
type Lesson struct {
	ID      int64     `json:"id"`
	Title   string    `json:"title"`
	Fix     string    `json:"fix"`
	Change  string    `json:"change"`
	Why     string    `json:"why"`
	Repo    string    `json:"repo,omitempty"`
	State   string    `json:"state"`
	StateAt time.Time `json:"stateAt"`
	// Again is when crew ran into it after it was marked fixed.
	Again   *time.Time `json:"again,omitempty"`
	Mission string     `json:"mission,omitempty"`
	Created time.Time  `json:"created"`
	// Seen is each crew that ran into it, the latest first.
	Seen []Sighting `json:"seen"`
	// Crew and Missions are how many ran into it, LastSeen when one last
	// did: worked out from Seen.
	Crew     int       `json:"crew"`
	Missions int       `json:"missions"`
	LastSeen time.Time `json:"lastSeen"`
}

// Sighting is a crew that ran into a lesson, and what it said of it.
type Sighting struct {
	Mission string    `json:"mission"`
	Crew    string    `json:"crew"`
	Repo    string    `json:"repo,omitempty"`
	Said    string    `json:"said"`
	At      time.Time `json:"at"`
}

// IsRecommended is whether enough crew have run into it to say what to
// change, not only note it.
func (l Lesson) IsRecommended() bool { return l.Crew >= Recommended }

// Quiet is whether nobody's run into it for a while, at now.
func (l Lesson) Quiet(now time.Time) bool { return now.Sub(l.LastSeen) > QuietAfter }

// Found is what reading a handoff found of one lesson: a known one (Same),
// or a new one, with what crew said of it.
type Found struct {
	Same int64 // the known lesson's id; 0 for a new one
	Said string
	// For a new one, the lesson; for a known one, Change is a better
	// change to make, or "".
	Title, Fix, Change, Why, Repo string
}

// Read is a crew's handoff, read: which, and how it went.
type Read struct {
	Mission, Crew, Repo string
	HandedOff           time.Time
	At                  time.Time
	Err                 string // why it couldn't be read; nothing's found then
}

// Key names a crew's handoff, by mission and crew.
func Key(mission, crew string) string { return mission + "/" + crew }

// open is the workspace's records.
func open(root string) (*store.DB, error) { return store.Open(root) }

const stamp = time.RFC3339Nano

func at(s string) time.Time {
	t, _ := time.Parse(stamp, s)
	return t
}

// All is every lesson, with its sightings.
func All(root string) ([]Lesson, error) {
	db, err := open(root)
	if err != nil {
		return nil, err
	}
	var out []Lesson
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		var err error
		out, err = all(tx)
		return err
	})
	return out, err
}

func all(tx *store.Tx) ([]Lesson, error) {
	rows, err := tx.Query(`SELECT id, title, fix, change, why, repo, state, state_at, again, mission, created FROM lessons ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var out []Lesson
	byID := map[int64]int{}
	for rows.Next() {
		var l Lesson
		var stateAt, again, created string
		if err := rows.Scan(&l.ID, &l.Title, &l.Fix, &l.Change, &l.Why, &l.Repo, &l.State, &stateAt, &again, &l.Mission, &created); err != nil {
			rows.Close()
			return nil, err
		}
		l.StateAt, l.Created = at(stateAt), at(created)
		if again != "" {
			a := at(again)
			l.Again = &a
		}
		l.Seen = []Sighting{}
		byID[l.ID] = len(out)
		out = append(out, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = tx.Query(`SELECT lesson, mission, crew, repo, said, at FROM lesson_sightings ORDER BY at DESC, mission, crew`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var s Sighting
		var when string
		if err := rows.Scan(&id, &s.Mission, &s.Crew, &s.Repo, &s.Said, &when); err != nil {
			return nil, err
		}
		s.At = at(when)
		if i, ok := byID[id]; ok {
			out[i].Seen = append(out[i].Seen, s)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		tally(&out[i])
	}
	return out, nil
}

// tally works out how many crew and missions ran into l, and when last.
func tally(l *Lesson) {
	missions := map[string]bool{}
	for _, s := range l.Seen {
		missions[s.Mission] = true
		if s.At.After(l.LastSeen) {
			l.LastSeen = s.At
		}
	}
	l.Crew, l.Missions = len(l.Seen), len(missions)
}

// Get is one lesson.
func Get(root string, id int64) (Lesson, error) {
	list, err := All(root)
	if err != nil {
		return Lesson{}, err
	}
	if i := slices.IndexFunc(list, func(l Lesson) bool { return l.ID == id }); i >= 0 {
		return list[i], nil
	}
	return Lesson{}, ErrNoLesson
}

// ErrNoLesson is the answer for a lesson that isn't there.
var ErrNoLesson = errors.New("no such lesson")

// ReadTo is each crew's handoff read, by Key: when it was handed off, as
// it was read.
func ReadTo(root string) (map[string]time.Time, error) {
	db, err := open(root)
	if err != nil {
		return nil, err
	}
	out := map[string]time.Time{}
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT mission, crew, handed_off FROM lessons_read`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var m, c, h string
			if err := rows.Scan(&m, &c, &h); err != nil {
				return err
			}
			out[Key(m, c)] = at(h)
		}
		return rows.Err()
	})
	return out, err
}

// Reads is how many handoffs have been read, and how many of those
// couldn't be.
func Reads(root string) (read, failed int, err error) {
	db, err := open(root)
	if err != nil {
		return 0, 0, err
	}
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT count(*), count(nullif(error, '')) FROM lessons_read`).Scan(&read, &failed)
	})
	return read, failed, err
}

// Change is what keeping a read changed of one lesson.
type Change struct {
	Lesson Lesson
	// New is whether the lesson's new; Recommended whether it became a
	// recommendation with this read; Again whether crew ran into it after
	// it was marked fixed.
	New, Recommended, Again bool
}

// Keep keeps a handoff read and what it found, at once: each found lesson
// a sighting of a known one or a new one. A known one that was fixed is
// open again, crew having run into it since. What it says changed is each
// lesson touched, as it stands now.
func Keep(root string, r Read, found []Found) ([]Change, error) {
	db, err := open(root)
	if err != nil {
		return nil, err
	}
	var touched []int64
	news := map[int64]bool{}
	agains := map[int64]bool{}
	before := map[int64]int{}
	err = db.Write(context.Background(), func(tx *store.Tx) error {
		known, err := all(tx)
		if err != nil {
			return err
		}
		byID := map[int64]Lesson{}
		for _, l := range known {
			byID[l.ID] = l
		}
		handed := r.HandedOff.UTC().Format(stamp)
		for _, f := range found {
			if f.Said == "" {
				continue
			}
			id := f.Same
			l, ok := byID[id]
			if !ok {
				if f.Title == "" {
					continue // a known one that isn't, with nothing to make it a new one
				}
				fix := f.Fix
				if !slices.Contains(Fixes, fix) {
					fix = FixDocs
				}
				res, err := tx.Exec(`INSERT INTO lessons (title, fix, change, why, repo, state, state_at, created) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					f.Title, fix, f.Change, f.Why, f.Repo, Open, handed, handed)
				if err != nil {
					return err
				}
				if id, err = res.LastInsertId(); err != nil {
					return err
				}
				news[id] = true
				l = Lesson{ID: id, State: Open}
				byID[id] = l
			} else if _, seen := before[id]; !seen {
				before[id] = l.Crew
			}
			res, err := tx.Exec(`INSERT OR IGNORE INTO lesson_sightings (lesson, mission, crew, repo, said, at) VALUES (?, ?, ?, ?, ?, ?)`,
				id, r.Mission, r.Crew, r.Repo, f.Said, handed)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				continue // this crew's said it already
			}
			if l.State == Fixed {
				if _, err := tx.Exec(`UPDATE lessons SET state = ?, state_at = ?, again = ? WHERE id = ?`, Open, handed, handed, id); err != nil {
					return err
				}
				agains[id] = true
			}
			if f.Same != 0 && f.Change != "" && l.State == Open && l.Mission == "" {
				if _, err := tx.Exec(`UPDATE lessons SET change = ? WHERE id = ?`, f.Change, id); err != nil {
					return err
				}
			}
			if !slices.Contains(touched, id) {
				touched = append(touched, id)
			}
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO lessons_read (mission, crew, handed_off, at, found, error) VALUES (?, ?, ?, ?, ?, ?)`,
			r.Mission, r.Crew, handed, r.At.UTC().Format(stamp), len(touched), r.Err); err != nil {
			return err
		}
		tx.Changed(Topic)
		return nil
	})
	if err != nil || len(touched) == 0 {
		return nil, err
	}
	now, err := All(root)
	if err != nil {
		return nil, err
	}
	var out []Change
	for _, l := range now {
		if !slices.Contains(touched, l.ID) {
			continue
		}
		was, known := before[l.ID]
		out = append(out, Change{
			Lesson:      l,
			New:         news[l.ID],
			Recommended: l.IsRecommended() && (news[l.ID] || known && was < Recommended),
			Again:       agains[l.ID],
		})
	}
	return out, nil
}

// SetState marks a lesson open, fixed or dismissed, at now.
func SetState(root string, id int64, state string, now time.Time) error {
	if state != Open && state != Fixed && state != Dismissed {
		return fmt.Errorf("a lesson is open, fixed or dismissed, not %q", state)
	}
	return update(root, id, `UPDATE lessons SET state = ?, state_at = ?, again = CASE WHEN ? = 'open' THEN again ELSE '' END WHERE id = ?`,
		state, now.UTC().Format(stamp), state, id)
}

// SetMission records the mission made to fix a lesson.
func SetMission(root string, id int64, slug string) error {
	return update(root, id, `UPDATE lessons SET mission = ? WHERE id = ?`, slug, id)
}

func update(root string, id int64, q string, args ...any) error {
	db, err := open(root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), func(tx *store.Tx) error {
		res, err := tx.Exec(q, args...)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNoLesson
		}
		tx.Changed(Topic)
		return nil
	})
}

// Put keeps lessons as given, with their sightings: for a seed, which has
// no agent to read its handoffs.
func Put(root string, list []Lesson) error {
	db, err := open(root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), func(tx *store.Tx) error {
		for _, l := range list {
			if l.State == "" {
				l.State = Open
			}
			created := l.Created
			for _, s := range l.Seen {
				if created.IsZero() || s.At.Before(created) {
					created = s.At
				}
			}
			again := ""
			if l.Again != nil {
				again = l.Again.UTC().Format(stamp)
			}
			stateAt := l.StateAt
			if stateAt.IsZero() {
				stateAt = created
			}
			res, err := tx.Exec(`INSERT INTO lessons (title, fix, change, why, repo, state, state_at, again, mission, created) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				l.Title, l.Fix, l.Change, l.Why, l.Repo, l.State, stateAt.UTC().Format(stamp), again, l.Mission, created.UTC().Format(stamp))
			if err != nil {
				return err
			}
			id, err := res.LastInsertId()
			if err != nil {
				return err
			}
			for _, s := range l.Seen {
				if _, err := tx.Exec(`INSERT OR IGNORE INTO lesson_sightings (lesson, mission, crew, repo, said, at) VALUES (?, ?, ?, ?, ?, ?)`,
					id, s.Mission, s.Crew, s.Repo, s.Said, s.At.UTC().Format(stamp)); err != nil {
					return err
				}
			}
		}
		tx.Changed(Topic)
		return nil
	})
}

// Stamp names where the lessons stand, cheaply: it moves with each write.
func Stamp(root string) string {
	db, err := open(root)
	if err != nil {
		return "error: " + err.Error()
	}
	var seq int64
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		var err error
		seq, err = tx.ChangedAt(Topic)
		return err
	})
	if err != nil {
		return "error: " + err.Error()
	}
	return strconv.FormatInt(seq, 10)
}
