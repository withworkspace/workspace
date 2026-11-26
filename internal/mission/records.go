package mission

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// A mission's record, its log and its decisions are kept in the workspace's
// records: a row in missions with the record as JSON (as mission.toml held
// it) beside what's asked of it across missions; a row in mission_log for
// each event, and in decisions for each decision made or withdrawn, as
// log.jsonl and decisions.jsonl held them. What's written to be read stays
// a file in the mission's folder: its brief, its files/, crew output,
// reviews.

func init() {
	store.Define("mission", store.SQL(`
		-- A mission's record, and what's asked of it across missions.
		CREATE TABLE missions (
			slug TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			state TEXT NOT NULL,                  -- as stored: '', parked or archived (briefing, before confirmedStep)
			created TEXT NOT NULL,
			changed TEXT NOT NULL,                -- its record or its log, last
			ended TEXT NOT NULL DEFAULT '',       -- when it was archived; '' while it's in play
			record TEXT NOT NULL                  -- the mission, as JSON
		) WITHOUT ROWID;
		CREATE INDEX missions_in_play ON missions (created, slug) WHERE ended = '';
		-- An event of its log, as log.jsonl held it.
		CREATE TABLE mission_log (
			n INTEGER PRIMARY KEY, slug TEXT NOT NULL,
			time TEXT NOT NULL, kind TEXT NOT NULL, repo TEXT NOT NULL, capsule TEXT NOT NULL, crew TEXT NOT NULL,
			event TEXT NOT NULL
		);
		CREATE INDEX mission_log_slug ON mission_log (slug, n);
		-- A decision made, or one withdrawn, as decisions.jsonl held it.
		CREATE TABLE decisions (
			slug TEXT NOT NULL, n INTEGER NOT NULL,
			id TEXT NOT NULL, at TEXT NOT NULL, by TEXT NOT NULL,
			line TEXT NOT NULL,
			PRIMARY KEY (slug, n)
		) WITHOUT ROWID;
	`), store.Retired, wordsStep, confirmedStep, filesAgainStep, standingStep)
}

// standingStep is the part's sixth step: what a mission wrote before
// things had a standing stands as stands and decisionStands say: a mission
// in force while it's in play, settled once it's archived, since it was
// (gone, if it was dropped); a decision in force while it's current, gone
// once it isn't (since when isn't known here: it ages from when it was
// made); its brief and files in force, as what's in it stands as it does.
var standingStep = store.SQL(`
	UPDATE things SET
		standing = coalesce((SELECT CASE
			WHEN m.state <> 'archived' THEN 'open'
			WHEN json_extract(m.record, '$.ending.ending') = 'dropped' THEN 'gone'
			ELSE 'settled' END FROM missions m WHERE m.slug = things.name), 'open'),
		since = coalesce((SELECT m.ended FROM missions m WHERE m.slug = things.name AND m.state = 'archived'), '')
		WHERE origin LIKE 'mission:%' AND kind = 'mission' AND standing = '';
	UPDATE things SET standing = CASE state WHEN 'current' THEN 'open' ELSE 'gone' END
		WHERE origin LIKE 'decision:%' AND standing = '';
	UPDATE things SET standing = 'open' WHERE origin LIKE 'missionfile:%' AND standing = '';
`)

// filesAgainStep is the part's fifth step: the files of missions in play
// are read again when next asked for (Reread), to be things in the graph,
// what their words name linked from them. An ended mission's are read
// again by ws doctor --reindex.
var filesAgainStep = store.SQL(`DELETE FROM mission_files`)

// confirmedStep is the part's fourth step: the briefing state goes. A
// mission that was being briefed is to brief because its brief isn't
// confirmed (Mission.Confirmed); every other is confirmed, as it was made.
var confirmedStep = store.SQL(`
	UPDATE missions SET record = json_set(record, '$.confirmed', json_extract(record, '$.created'))
		WHERE state <> 'briefing' AND json_extract(record, '$.confirmed') IS NULL;
	UPDATE missions SET state = '', record = json_remove(record, '$.state') WHERE state = 'briefing';
`)

func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

func timeOf(s string) time.Time {
	t, _ := time.Parse(store.Stamp, s)
	return t
}

func (s Store) records() (*store.DB, error) { return store.Open(s.Root) }

func (s Store) read(fn func(tx *store.Tx) error) error {
	db, err := s.records()
	if err != nil {
		return err
	}
	return db.Read(context.Background(), fn)
}

func (s Store) write(fn func(tx *store.Tx) error) error {
	db, err := s.records()
	if err != nil {
		return err
	}
	return db.Write(context.Background(), fn)
}

func notFound(slug string) error { return fmt.Errorf("%w: %q", ErrNotFound, slug) }

// decode is a mission from its record.
func decode(slug string, record []byte) (*Mission, error) {
	var m Mission
	if err := json.Unmarshal(record, &m); err != nil {
		return nil, fmt.Errorf("mission %s: %w", slug, err)
	}
	m.Slug = slug
	m.migrateFlag()
	return &m, nil
}

// loadIn is a mission as it stands in tx.
func (s Store) loadIn(tx *store.Tx, slug string) (*Mission, error) {
	if !validSlug(slug) {
		return nil, notFound(slug)
	}
	var record string
	err := tx.QueryRow(`SELECT record FROM missions WHERE slug = ?`, slug).Scan(&record)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound(slug)
	}
	if err != nil {
		return nil, err
	}
	m, err := decode(slug, []byte(record))
	return s.withPlan(m), err
}

// listIn is the missions whose rows match where, oldest first.
func (s Store) listIn(tx *store.Tx, where string, args ...any) ([]*Mission, error) {
	rows, err := tx.Query(`SELECT slug, record FROM missions WHERE `+where+` ORDER BY created, slug`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Mission
	for rows.Next() {
		var slug, record string
		if err := rows.Scan(&slug, &record); err != nil {
			return nil, err
		}
		if m, err := decode(slug, []byte(record)); err == nil { // one that can't be read is passed over
			out = append(out, s.withPlan(m))
		}
	}
	return out, rows.Err()
}

func (s Store) list(where string, args ...any) ([]*Mission, error) {
	var out []*Mission
	err := s.read(func(tx *store.Tx) error {
		var err error
		out, err = s.listIn(tx, where, args...)
		return err
	})
	return out, err
}

// InPlay is the missions that aren't archived, oldest first: what the
// workspace's lists show, whatever's been before.
func (s Store) InPlay() ([]*Mission, error) { return s.list(`ended = ''`) }

// Past is an archived mission as a list shows it: what its row says, and
// its capsules. Nothing else of its record is read: its crew, say, are
// its page's to show.
type Past struct {
	Slug, Title             string
	Created, Changed, Ended time.Time
	Capsules                []Capsule
}

// Past is the archived missions, oldest first, as their rows say: reading
// them costs next to nothing however many there have been.
func (s Store) Past() ([]Past, error) {
	var out []Past
	err := s.read(func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT slug, title, created, changed, ended, coalesce(json_extract(record, '$.capsules'), '')
			FROM missions WHERE ended != '' ORDER BY created, slug`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p Past
			var created, changed, ended, capsules string
			if err := rows.Scan(&p.Slug, &p.Title, &created, &changed, &ended, &capsules); err != nil {
				return err
			}
			p.Created, p.Changed, p.Ended = timeOf(created), timeOf(changed), timeOf(ended)
			if capsules != "" {
				_ = json.Unmarshal([]byte(capsules), &p.Capsules) // what can't be read is as none
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

// InPlaySlugs is the slugs of the missions that aren't archived, without
// reading them; ok is false when the records can't be asked.
func (s Store) InPlaySlugs() (slugs map[string]bool, ok bool) {
	slugs = map[string]bool{}
	err := s.read(func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT slug FROM missions WHERE ended = ''`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var slug string
			if err := rows.Scan(&slug); err != nil {
				return err
			}
			slugs[slug] = true
		}
		return rows.Err()
	})
	return slugs, err == nil
}

// Slugs is every mission's slug, archived ones too, without reading them.
func (s Store) Slugs() []string {
	var out []string
	_ = s.read(func(tx *store.Tx) error { // a read it can't make is as none
		rows, err := tx.Query(`SELECT slug FROM missions ORDER BY created, slug`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var slug string
			if rows.Scan(&slug) == nil {
				out = append(out, slug)
			}
		}
		return rows.Err()
	})
	return out
}

// Changed is when a mission's record or log last changed, or zero.
func (s Store) Changed(slug string) time.Time {
	var at string
	_ = s.read(func(tx *store.Tx) error { // a read it can't make is as none
		return tx.QueryRow(`SELECT changed FROM missions WHERE slug = ?`, slug).Scan(&at)
	})
	return timeOf(at)
}

// keep writes a mission's record, as it now is, and its row: changed now,
// and ended when it's archived (when it first was, if it was before).
func (s Store) keep(tx *store.Tx, m *Mission) error {
	record, err := json.Marshal(m)
	if err != nil {
		return err
	}
	at := stampOf(time.Now())
	ended := ""
	if m.State == Archived {
		ended = at
		var was string
		if tx.QueryRow(`SELECT ended FROM missions WHERE slug = ?`, m.Slug).Scan(&was) == nil && was != "" {
			ended = was
		}
	}
	if _, err := tx.Exec(`INSERT INTO missions (slug, title, state, created, changed, ended, record) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (slug) DO UPDATE SET title = excluded.title, state = excluded.state, changed = excluded.changed,
			ended = excluded.ended, record = excluded.record`,
		m.Slug, m.Title, string(m.State), stampOf(m.Created), at, ended, string(record)); err != nil {
		return err
	}
	changed(tx, m.Slug)
	return s.missionGraph(tx, m, timeOf(ended))
}

// changed names what a write to a mission changed: its page, and the
// workspace's lists.
func changed(tx *store.Tx, slug string) { tx.Changed("mission:"+slug, "workspace") }

// appendLog adds an event to a mission's log in tx: its time and who's
// filled in as AppendLog says.
func (s Store) appendLog(tx *store.Tx, slug string, e Event) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	at := stampOf(time.Now())
	res, err := tx.Exec(`INSERT INTO mission_log (slug, time, kind, repo, capsule, crew, event)
		SELECT ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM missions WHERE slug = ?)`,
		slug, stampOf(e.Time), e.Kind, e.Repo, e.Capsule, e.Crew, string(line), slug)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(`UPDATE missions SET changed = ? WHERE slug = ?`, at, slug); err != nil {
		return err
	}
	changed(tx, slug)
	return share(tx, slug, e)
}

// decisionsIn is every decision on a mission in tx, oldest first, with how
// each stands now: its lines, replayed.
func (s Store) decisionsIn(tx *store.Tx, slug string) ([]Decision, error) {
	rows, err := tx.Query(`SELECT line FROM decisions WHERE slug = ? ORDER BY n`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines [][]byte
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		lines = append(lines, []byte(l))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return replayDecisions(lines), nil
}

// replayDecisions is decisions as their lines say, in order. A line that
// can't be read is passed over.
func replayDecisions(lines [][]byte) []Decision {
	var out []Decision
	at := map[string]int{}
	for _, data := range lines {
		var l decisionLine
		if json.Unmarshal(data, &l) != nil {
			continue
		}
		switch {
		case l.Made != nil:
			d := *l.Made
			d.State = DecisionCurrent
			d.asked()
			if i, ok := at[d.Supersedes]; ok && d.Supersedes != "" {
				out[i].State, out[i].SupersededBy = DecisionSuperseded, d.ID
			}
			at[d.ID] = len(out)
			out = append(out, d)
		case l.Withdraw != nil:
			if i, ok := at[l.Withdraw.ID]; ok {
				w := l.Withdraw.At
				out[i].State, out[i].Withdrawn, out[i].WithdrawnWhy = DecisionWithdrawn, &w, l.Withdraw.Why
			}
		}
	}
	return out
}

// addDecision appends a decision's line, then writes its things and links
// again as they all stand.
func (s Store) addDecision(tx *store.Tx, slug string, l decisionLine) error {
	data, err := json.Marshal(l)
	if err != nil {
		return err
	}
	id, at, by := "", time.Time{}, ""
	switch {
	case l.Made != nil:
		id, at, by = l.Made.ID, l.Made.At, l.Made.By
	case l.Withdraw != nil:
		id, at, by = l.Withdraw.ID, l.Withdraw.At, l.Withdraw.By
	}
	if _, err := tx.Exec(`INSERT INTO decisions (slug, n, id, at, by, line)
		VALUES (?, (SELECT coalesce(max(n), 0) + 1 FROM decisions WHERE slug = ?), ?, ?, ?, ?)`,
		slug, slug, id, stampOf(at), by, string(data)); err != nil {
		return err
	}
	all, err := s.decisionsIn(tx, slug)
	if err != nil {
		return err
	}
	return s.decisionsGraph(tx, slug, all)
}

// The graph: a mission is a thing, from the talk or the crew it came out
// of, about what it says it's about; each decision is one, by whoever made it, in where it was made, and
// about what it bears on. A decision's words are what was decided (its
// title) and why; the mission's are its brief's (words.go), by its title.
// Its brief and the files beside it are things in it too (words.go).

func missionOrigin(slug string) store.Origin { return store.Origin("mission:" + slug) }

func decisionOrigin(slug, id string) store.Origin {
	return store.Origin("decision:" + slug + "/" + id)
}

func (s Store) missionGraph(tx *store.Tx, m *Mission, ended time.Time) error {
	o := missionOrigin(m.Slug)
	if err := tx.Forget(o); err != nil {
		return err
	}
	me := store.Key{Name: m.Slug}
	state := standing(string(m.State))
	if err := tx.Thing(o, store.Thing{Key: me, Kind: "mission", Title: m.Title, State: state, Changed: time.Now(), Stands: stands(m, ended)}); err != nil {
		return err
	}
	if err := tx.Retitle(fileOrigin(m.Slug, briefFile), m.Title); err != nil {
		return err
	}
	// Its brief is called what it is, and its files stand as it does.
	if err := tx.Titled(fileOrigin(m.Slug, briefFile), m.Title); err != nil {
		return err
	}
	if err := tx.StatedUnder(filesOrigin(m.Slug), state); err != nil {
		return err
	}
	if m.Conversation != "" {
		if err := tx.Link(o, store.Link{Src: me, Kind: store.From, Dst: store.Key{Name: m.Conversation}, At: m.Created}); err != nil {
			return err
		}
	}
	if parent, crew, ok := cutFrom(m.From); ok {
		if err := tx.Link(o, store.Link{Src: me, Kind: store.From, Dst: store.Key{Scope: parent, Name: crew}, At: m.Created}); err != nil {
			return err
		}
	}
	// What it's about: found from the thing, as what's been said of it.
	for _, name := range m.About {
		if err := tx.Link(o, store.Link{Src: me, Kind: store.About, Dst: store.Key{Name: name}, At: m.Created}); err != nil {
			return err
		}
	}
	return nil
}

// cutFrom reads a mission's From (parent/c2).
func cutFrom(from string) (parent, crew string, ok bool) {
	for i := len(from) - 1; i >= 0; i-- {
		if from[i] == '/' {
			return from[:i], from[i+1:], from[:i] != "" && from[i+1:] != ""
		}
	}
	return "", "", false
}

func (s Store) decisionsGraph(tx *store.Tx, slug string, all []Decision) error {
	for _, d := range all {
		o := decisionOrigin(slug, d.ID)
		if err := tx.Forget(o); err != nil {
			return err
		}
		me := store.Key{Scope: slug, Name: d.ID}
		changed := d.At
		if d.Withdrawn != nil {
			changed = *d.Withdrawn
		}
		if err := tx.Thing(o, store.Thing{Key: me, Kind: "decision", Title: d.What, State: d.State, Changed: changed, Stands: decisionStands(d, all)}); err != nil {
			return err
		}
		if err := tx.Words(o, store.Place{Thing: me, Kind: placeDecision, Title: d.What, Text: strings.TrimSpace(d.Reason()), Said: d.At}); err != nil {
			return err
		}
		link := func(kind string, dst store.Key) error {
			lines := ""
			if r, ok := ref.Parse(dst.Name); ok && dst.Scope == "" && r.Line > 0 {
				lines = linesOf(r.Line, r.EndLine)
				r.Line, r.EndLine = 0, 0
				dst.Name = r.Name()
			}
			return tx.Link(o, store.Link{Src: me, Kind: kind, Dst: dst, Lines: lines, At: d.At})
		}
		if err := link(store.By, whoKey(slug, d.By)); err != nil {
			return err
		}
		if d.From != "" {
			if err := link(store.In, refKey(slug, d.From)); err != nil {
				return err
			}
		}
		for _, a := range d.About {
			if err := link(store.About, refKey(slug, a)); err != nil {
				return err
			}
		}
	}
	return nil
}

// missionRef is the shape of what's named in a mission: a thread, a
// decision, crew, a stage, a review and its comments.
var missionRef = regexp.MustCompile(`^(?:[tdcsr][1-9][0-9]*)(?:\.c[1-9][0-9]*)?$`)

// refKey is what a ref said in a mission is in the graph: a thing of the
// mission's (t3, d2, c1, mission:notes.md), or one named the same
// everywhere, as refs write it.
func refKey(slug, s string) store.Key {
	if missionRef.MatchString(s) || len(s) > 8 && s[:8] == "mission:" {
		return store.Key{Scope: slug, Name: s}
	}
	if r, ok := ref.Parse(s); ok {
		return store.Key{Name: r.Name()} // repo:path, as a document's stored, is repo/path
	}
	return store.Key{Name: s}
}

// whoKey is who made something: you, the orchestrator or crew of the
// mission.
func whoKey(slug, by string) store.Key {
	if by == "you" {
		return store.Key{Name: "you"}
	}
	return store.Key{Scope: slug, Name: by}
}

func linesOf(line, end int) string {
	if end > line {
		return fmt.Sprintf("%d-%d", line, end)
	}
	return fmt.Sprint(line)
}
