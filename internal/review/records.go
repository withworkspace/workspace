package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// Reviews are kept in the workspace's records: a row each in reviews, with
// the review and its comments as JSON beside what's asked of it; and in
// viewed, a row for each file you've marked viewed. A review's scope is its
// mission's slug, or the pull request it's of (api#89). A submitted review
// of a mission's is also rendered, a file, for agents to read.

func init() {
	store.Define("review", store.SQL(`
		CREATE TABLE reviews (
			scope TEXT NOT NULL,                  -- a mission's slug, or a pull request (api#89)
			id TEXT NOT NULL, n INTEGER NOT NULL, -- r3, and its number
			capsule TEXT NOT NULL,                -- repo/name, or the pull request
			state TEXT NOT NULL, verdict TEXT NOT NULL, head TEXT NOT NULL,
			created TEXT NOT NULL,
			submitted TEXT NOT NULL,              -- '' while it's a draft
			changed TEXT NOT NULL,
			review TEXT NOT NULL,                 -- the review and its comments, as JSON
			PRIMARY KEY (scope, id)
		) WITHOUT ROWID;
		CREATE INDEX reviews_capsule ON reviews (scope, capsule, state);
		-- A file you've marked viewed, at its content then.
		CREATE TABLE viewed (
			scope TEXT NOT NULL, capsule TEXT NOT NULL, path TEXT NOT NULL, blob TEXT NOT NULL,
			PRIMARY KEY (scope, capsule, path)
		) WITHOUT ROWID;
	`), store.Retired, sayWords)
}

// scope is what the reviews are kept under: the mission, or the pull
// request.
func (s Store) scope() string {
	if s.ofMission() {
		return s.Slug
	}
	return s.Name
}

func (s Store) records() (*store.DB, error) { return store.Open(s.Missions.Root) }

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

func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

func timeOf(s string) time.Time {
	t, _ := time.Parse(store.Stamp, s)
	return t
}

// loadIn is one review as it stands in tx.
func (s Store) loadIn(tx *store.Tx, id string) (*Review, error) {
	var data string
	err := tx.QueryRow(`SELECT review FROM reviews WHERE scope = ? AND id = ?`, s.scope(), id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s has no review %s", ErrNotFound, s.whose(), id)
	}
	if err != nil {
		return nil, err
	}
	var r Review
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		return nil, fmt.Errorf("review %s: %w", id, err)
	}
	return &r, nil
}

// listIn is the reviews whose rows match where, by their numbers.
func (s Store) listIn(tx *store.Tx, where string, args ...any) ([]*Review, error) {
	rows, err := tx.Query(`SELECT review FROM reviews WHERE scope = ? AND `+where+` ORDER BY n`, append([]any{s.scope()}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Review
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var r Review
		if json.Unmarshal([]byte(data), &r) == nil { // one that can't be read is passed over
			out = append(out, &r)
		}
	}
	return out, rows.Err()
}

func (s Store) list(where string, args ...any) ([]*Review, error) {
	var out []*Review
	err := s.read(func(tx *store.Tx) error {
		var err error
		out, err = s.listIn(tx, where, args...)
		return err
	})
	return out, err
}

// draftIn is the capsule's draft in tx, or nil.
func (s Store) draftIn(tx *store.Tx, capsule string) (*Review, error) {
	all, err := s.listIn(tx, `capsule = ? AND state = ?`, capsule, Draft)
	if err != nil || len(all) == 0 {
		return nil, err
	}
	return all[0], nil
}

// nextIn is the next review's number: one past the highest there is.
func (s Store) nextIn(tx *store.Tx) (int, error) {
	var n int
	err := tx.QueryRow(`SELECT coalesce(max(n), 0) + 1 FROM reviews WHERE scope = ?`, s.scope()).Scan(&n)
	return n, err
}

// saveIn writes a review's row, and what it is in the graph, in tx.
func (s Store) saveIn(tx *store.Tx, r *Review) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	submitted := ""
	if r.Submitted != nil {
		submitted = stampOf(*r.Submitted)
	}
	if _, err := tx.Exec(`INSERT INTO reviews (scope, id, n, capsule, state, verdict, head, created, submitted, changed, review)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (scope, id) DO UPDATE SET capsule = excluded.capsule, state = excluded.state, verdict = excluded.verdict,
			head = excluded.head, submitted = excluded.submitted, changed = excluded.changed, review = excluded.review`,
		s.scope(), r.ID, num(r.ID), r.Capsule, r.State, r.Verdict, r.Head, stampOf(r.Created), submitted, stampOf(time.Now()), string(data)); err != nil {
		return err
	}
	s.changed(tx)
	return s.graph(tx, r)
}

// changed names what a write to the reviews changed: the mission's page
// and the workspace's lists, or the pull request's and the Reviews page.
func (s Store) changed(tx *store.Tx) {
	if s.ofMission() {
		tx.Changed("mission:"+s.Slug, "workspace")
		return
	}
	tx.Changed("pr:"+s.Name, "reviews")
}

// Changed is when any of these reviews (or what you'd viewed) last
// changed, or zero.
func (s Store) Changed() time.Time {
	var at sql.NullString
	s.read(func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT max(changed) FROM reviews WHERE scope = ?`, s.scope()).Scan(&at)
	})
	return timeOf(at.String)
}

// The graph: a review is a thing, of what it reviews; each comment is
// one, on the file it's on, at its lines: a capsule's file is the repo's
// file as well. What's said in them is their words: a review's summary,
// and each comment with its thread's replies.

func (s Store) origin(id string) store.Origin {
	return store.Origin("review:" + s.scope() + "/" + id)
}

func (s Store) key(name string) store.Key { return store.Key{Scope: s.scope(), Name: name} }

func (s Store) graph(tx *store.Tx, r *Review) error {
	o := s.origin(r.ID)
	if err := tx.Forget(o); err != nil {
		return err
	}
	at := r.Created
	if r.Submitted != nil {
		at = *r.Submitted
	}
	state := r.State
	if r.State == Submitted {
		state = r.Verdict
	}
	me := s.key(r.ID)
	if err := tx.Thing(o, store.Thing{Key: me, Kind: "review", Title: "Review " + r.ID + " of " + r.Capsule, State: state, Changed: at}); err != nil {
		return err
	}
	if err := tx.Words(o, store.Place{Thing: me, Kind: "review", Text: strings.TrimSpace(r.Summary), Said: at}); err != nil {
		return err
	}
	if r.Capsule != "" {
		if err := tx.Link(o, store.Link{Src: me, Kind: store.Of, Dst: store.Key{Name: r.Capsule}, At: at}); err != nil {
			return err
		}
	}
	for _, n := range r.Comments {
		k := s.key(n.ID)
		title := oneLine(n.Body)
		if len([]rune(title)) > 100 {
			title = string([]rune(title)[:100]) + "…"
		}
		if err := tx.Thing(o, store.Thing{Key: k, Kind: "comment", Title: title, State: n.Status, Changed: at}); err != nil {
			return err
		}
		if err := tx.Words(o, store.Place{Thing: k, Kind: "comment", Text: said(n), Said: at}); err != nil {
			return err
		}
		if n.Path == "" {
			continue
		}
		lines := strconv.Itoa(n.Line)
		if n.EndLine > n.Line {
			lines += "-" + strconv.Itoa(n.EndLine)
		}
		for _, file := range s.files(r.Capsule, n.Path) {
			if err := tx.Link(o, store.Link{Src: k, Kind: store.On, Dst: file, Lines: lines, At: at}); err != nil {
				return err
			}
		}
	}
	return nil
}

// said is what's said in a comment: its words, and its thread's replies.
func said(n Note) string {
	parts := []string{strings.TrimSpace(n.Body)}
	for _, r := range n.Thread {
		if b := strings.TrimSpace(r.Body); b != "" {
			parts = append(parts, b)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

// sayWords is the part's third step: every review's graph written again,
// its words with it, for records kept before words were.
func sayWords(tx *store.Tx) error { return sayWordsOf(tx, "") }

// sayWordsOf writes the graph of a scope's reviews again, or every one's
// ("").
func sayWordsOf(tx *store.Tx, scope string) error {
	rows, err := tx.Query(`SELECT scope, review FROM reviews WHERE ? = '' OR scope = ? ORDER BY scope, n`, scope, scope)
	if err != nil {
		return err
	}
	type kept struct {
		scope string
		r     Review
	}
	var all []kept
	for rows.Next() {
		var k kept
		var data string
		if err := rows.Scan(&k.scope, &data); err != nil {
			rows.Close()
			return err
		}
		if json.Unmarshal([]byte(data), &k.r) == nil { // one that can't be read is passed over
			all = append(all, k)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, k := range all {
		s := For(tx.Root(), k.scope)
		if _, _, ok := ref.ParsePR(k.scope); ok {
			s = Store{Missions: mission.Store{Root: tx.Root()}, Dir: ".", Name: k.scope}
		}
		if err := s.graph(tx, &k.r); err != nil {
			return err
		}
	}
	return nil
}

// SayWords writes every review's words again, from its record, a
// transaction for each mission's or pull request's: ws doctor --reindex.
// It's how many reviews it read.
func SayWords(ctx context.Context, root string) (int, error) {
	db, err := store.Open(root)
	if err != nil {
		return 0, err
	}
	scopes := map[string]int{}
	var order []string
	if err := db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT scope, count(*) FROM reviews GROUP BY scope ORDER BY scope`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var scope string
			var n int
			if err := rows.Scan(&scope, &n); err != nil {
				return err
			}
			scopes[scope] = n
			order = append(order, scope)
		}
		return rows.Err()
	}); err != nil {
		return 0, err
	}
	n := 0
	for _, scope := range order {
		if err := db.Write(ctx, func(tx *store.Tx) error { return sayWordsOf(tx, scope) }); err != nil {
			return n, err
		}
		n += scopes[scope]
	}
	return n, nil
}

// files are what a comment's path is in the graph: in a mission's capsule,
// the capsule's file and the repo's; in a pull request, the repo's.
func (s Store) files(capsule, path string) []store.Key {
	if repo, _, ok := ref.ParsePR(capsule); ok {
		return []store.Key{{Name: ref.Ref{Kind: ref.File, Repo: repo, Worktree: ref.Ground, Path: path}.Name()}}
	}
	repo, name, ok := strings.Cut(capsule, "/")
	if !ok {
		return nil
	}
	return []store.Key{
		{Name: ref.Ref{Kind: ref.File, Repo: repo, Worktree: name, Path: path}.Name()},
		{Name: ref.Ref{Kind: ref.File, Repo: repo, Worktree: ref.Ground, Path: path}.Name()},
	}
}
