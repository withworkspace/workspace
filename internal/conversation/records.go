package conversation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"syscall"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// Conversations are kept in the workspace's records: each line of a
// conversation's log a row of conversation_lines, as its .jsonl held it,
// replayed by the same code to load it; and beside them a row in
// conversations saying what it is and how it stands, written in the same
// transaction as each line, which is what lists read. A scope is a
// mission's slug, or "" for the workspace's own.

func init() {
	store.Define("conversation", store.SQL(`
		-- What a conversation is and how it stands: what its lists ask.
		CREATE TABLE conversations (
			scope TEXT NOT NULL, id TEXT NOT NULL,
			n INTEGER NOT NULL,                   -- its number: 3 of t3
			kind TEXT NOT NULL, what TEXT NOT NULL, goal TEXT NOT NULL,
			status TEXT NOT NULL, title TEXT NOT NULL,
			-- What it's about: a document, a pull request, a page, a pass's
			-- finding, the review a pass is over.
			doc TEXT NOT NULL, pr TEXT NOT NULL, page TEXT NOT NULL,
			finding TEXT NOT NULL, reviewed TEXT NOT NULL,
			opened TEXT NOT NULL,                 -- when, as store.Stamp writes it
			changed TEXT NOT NULL,                -- a line added, a run started or ended
			lines INTEGER NOT NULL,
			-- The process answering it, and since when; 0 for none.
			answering INTEGER NOT NULL DEFAULT 0, answering_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (scope, id)
		) WITHOUT ROWID;
		CREATE INDEX conversations_doc ON conversations (scope, doc) WHERE doc != '';
		CREATE INDEX conversations_pr ON conversations (scope, pr) WHERE pr != '';
		CREATE INDEX conversations_page ON conversations (scope, page) WHERE page != '';
		CREATE INDEX conversations_kind ON conversations (scope, kind, n);
		-- A line of its log, as the .jsonl held it.
		CREATE TABLE conversation_lines (
			scope TEXT NOT NULL, id TEXT NOT NULL, n INTEGER NOT NULL,
			at TEXT NOT NULL, line TEXT NOT NULL,
			PRIMARY KEY (scope, id, n)
		) WITHOUT ROWID;
		-- A document approved, at a version: the latest for a document is
		-- its approval.
		CREATE TABLE approvals (
			n INTEGER PRIMARY KEY, scope TEXT NOT NULL, doc TEXT NOT NULL,
			version TEXT NOT NULL, by TEXT NOT NULL, at TEXT NOT NULL
		);
		CREATE INDEX approvals_doc ON approvals (scope, doc);
	`), store.Retired, sayAllWords, store.SQL(`
		-- An ask that isn't kept (ask.go): left out of the lists, and swept
		-- once it's sat a while after its last turn.
		ALTER TABLE conversations ADD COLUMN ephemeral INTEGER NOT NULL DEFAULT 0;
		CREATE INDEX conversations_ephemeral ON conversations (changed) WHERE ephemeral = 1;
		-- The highest number a scope has given a conversation, once the one
		-- that had it is swept: a number's never given twice.
		CREATE TABLE conversation_numbers (scope TEXT PRIMARY KEY, n INTEGER NOT NULL) WITHOUT ROWID;
	`), standingStep)
}

// sayAllWords is the part's third step: what every conversation says, for
// those kept before words were.
func sayAllWords(tx *store.Tx) error {
	scopes, err := scopesIn(tx)
	if err != nil {
		return err
	}
	for _, scope := range scopes {
		if _, err := For(tx.Root(), scope).sayWordsIn(tx, `1`); err != nil {
			return err
		}
	}
	return nil
}

// scopesIn is every scope that has conversations.
func scopesIn(tx *store.Tx) ([]string, error) {
	rows, err := tx.Query(`SELECT DISTINCT scope FROM conversations ORDER BY scope`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			return nil, err
		}
		out = append(out, scope)
	}
	return out, rows.Err()
}

// sayWordsIn writes again what each of the scope's conversations whose
// rows match where says: how many there are.
func (s Store) sayWordsIn(tx *store.Tx, where string, args ...any) (int, error) {
	all, err := s.loadWhere(tx, where, args...)
	if err != nil {
		return 0, err
	}
	for _, c := range all {
		if err := s.sayWords(tx, c); err != nil {
			return 0, err
		}
	}
	return len(all), nil
}

// SayWords writes again what every conversation says, from its lines: ws
// doctor --reindex. A transaction is a mission's, or the workspace's, a
// few hundred at a time (a command's transaction has txLimit). It's how
// many it read.
func SayWords(ctx context.Context, root string) (int, error) {
	db, err := store.Open(root)
	if err != nil {
		return 0, err
	}
	var scopes []string
	if err := db.Read(ctx, func(tx *store.Tx) error {
		var err error
		scopes, err = scopesIn(tx)
		return err
	}); err != nil {
		return 0, err
	}
	const batch = 200
	n := 0
	for _, scope := range scopes {
		var most int
		if err := db.Read(ctx, func(tx *store.Tx) error {
			return tx.QueryRow(`SELECT coalesce(max(n), 0) FROM conversations WHERE scope = ?`, scope).Scan(&most)
		}); err != nil {
			return n, err
		}
		for lo := 0; lo < most; lo += batch {
			var k int
			if err := db.Write(ctx, func(tx *store.Tx) error {
				var err error
				k, err = For(root, scope).sayWordsIn(tx, `c.n > ? AND c.n <= ?`, lo, lo+batch)
				return err
			}); err != nil {
				return n, err
			}
			n += k
		}
	}
	return n, nil
}

// stampOf is a time as a column keeps it (store.Stamp).
func stampOf(t time.Time) string { return t.UTC().Format(store.Stamp) }

func timeOf(s string) time.Time {
	t, _ := time.Parse(store.Stamp, s)
	return t
}

func (s Store) records() (*store.DB, error) { return store.Open(s.Missions.Root) }

func (s Store) read(fn func(tx *store.Tx) error) error {
	db, err := s.records()
	if err != nil {
		return err
	}
	return db.Read(context.Background(), fn)
}

// write runs fn in a write, which says it's changed the conversations of
// its mission (or the workspace's): what a page shows of them, and the
// workspace's view of the mission, are of them as they are.
func (s Store) write(fn func(tx *store.Tx) error) error {
	db, err := s.records()
	if err != nil {
		return err
	}
	return db.Write(context.Background(), func(tx *store.Tx) error {
		if err := fn(tx); err != nil {
			return err
		}
		if s.Workspace() {
			tx.Changed("workspace")
		} else {
			tx.Changed("mission:" + s.Slug)
		}
		return nil
	})
}

// notFound is the answer for a conversation that isn't there.
func (s Store) notFound(id string) error {
	if s.Workspace() {
		return fmt.Errorf("%w: the workspace has no conversation %s", ErrNotFound, id)
	}
	return fmt.Errorf("%w: mission %s has no thread %s", ErrNotFound, s.Slug, id)
}

// load replays one conversation in a transaction: its lines, in order.
func (s Store) load(tx *store.Tx, id string) (*Conversation, error) {
	all, err := s.loadWhere(tx, `c.id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, s.notFound(id)
	}
	return all[0], nil
}

// loadWhere replays the conversations whose rows match where (of c, the
// row), in order of their numbers: one query for all their lines.
func (s Store) loadWhere(tx *store.Tx, where string, args ...any) ([]*Conversation, error) {
	rows, err := tx.Query(`SELECT c.id, c.answering, c.answering_at, c.changed, l.line FROM conversations c
		JOIN conversation_lines l ON l.scope = c.scope AND l.id = c.id
		WHERE c.scope = ? AND `+where+` ORDER BY c.n, l.n`, append([]any{s.Slug}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Conversation
	var (
		id             string
		lines          [][]byte
		pid            int
		since, changed string
	)
	done := func() {
		if id == "" {
			return
		}
		if c := replay(lines); c != nil {
			c.ID = id // as its row has it, whatever its first line said
			c.lines, c.changed = len(lines), timeOf(changed)
			c.Answering = answeringNow(pid, since)
			if t := timeOf(since); c.Answering && !t.IsZero() {
				c.AnsweringSince = &t
			}
			out = append(out, c)
		}
	}
	for rows.Next() {
		var rid, l, rsince, rchanged string
		var rpid int
		if err := rows.Scan(&rid, &rpid, &rsince, &rchanged, &l); err != nil {
			return nil, err
		}
		if rid != id {
			done()
			id, lines, pid, since, changed = rid, nil, rpid, rsince, rchanged
		}
		lines = append(lines, []byte(l))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	done()
	return out, nil
}

// list is the conversations whose rows match where, each replayed, with
// what's worked out of them: what a list reads, so nothing else replays.
func (s Store) list(where string, args ...any) ([]*Conversation, error) {
	var out []*Conversation
	err := s.read(func(tx *store.Tx) error {
		var err error
		out, err = s.loadWhere(tx, where, args...)
		return err
	})
	if err != nil {
		return nil, err
	}
	for _, c := range out {
		s.workOut(c)
	}
	return out, nil
}

// replay is a conversation from its log's lines, or nil if they don't say
// how it opened. A line that can't be read is passed over, as it always
// was in a file.
func replay(lines [][]byte) *Conversation {
	var c *Conversation
	for _, data := range lines {
		var l line
		if json.Unmarshal(data, &l) != nil {
			continue
		}
		if l.Opened != nil {
			c = l.Opened
			c.Turns = []Turn{}
			continue
		}
		if c != nil {
			c.apply(l)
		}
	}
	if c != nil && c.What == "" { // opened before What was kept
		c.What = whatOf(c)
	}
	return c
}

// apply is a line of its log happening to a conversation.
func (c *Conversation) apply(l line) {
	switch {
	case l.Turn != nil:
		c.Turns = append(c.Turns, *l.Turn)
		if l.Turn.Changed {
			c.Sent = nil
		}
	case l.Sent != nil:
		c.Sent = l.Sent
	case l.Propose != nil:
		c.Proposals = append(c.Proposals, *l.Propose)
	case l.Proposal != nil:
		if p := c.Proposal(l.Proposal.N); p != nil {
			at := l.Proposal.At
			p.State, p.Result, p.SettledBy, p.SettledAt, p.Because = l.Proposal.State, l.Proposal.Result, l.Proposal.By, &at, l.Proposal.Because
			// A plugin's items still open are left with it.
			for i := range p.Items {
				if p.Items[i].State == "" && p.State != ProposalOpen {
					p.Items[i].State = ItemLeft
				}
			}
		}
	case l.Items != nil:
		if p := c.Proposal(l.Items.N); p != nil {
			p.applyItems(l.Items.Items)
		}
	case l.Settle != nil:
		c.settle(l.Settle.Key, l.Settle.State, l.Settle.Why)
	case l.Placed != nil:
		c.Placed = l.Placed
	case l.Kept != nil:
		at := l.Kept.At
		c.Ephemeral, c.Kept = false, &at
	case l.Lifted != nil:
		if cp := c.Capsule(l.Lifted.Repo, l.Lifted.Name); cp != nil {
			*cp = *l.Lifted
		} else {
			c.Capsules = append(c.Capsules, *l.Lifted)
		}
	case l.Away != nil:
		if cp := c.Capsule(l.Away.Repo, l.Away.Name); cp != nil {
			at := l.Away.At
			cp.Away, cp.Kept = &at, l.Away.Kept
		}
	case l.Summary != nil:
		if l.Summary.Title != "" {
			c.Title = l.Summary.Title
		}
		if l.Summary.Summary != "" {
			c.Summary = l.Summary.Summary
		}
	case l.Draft != nil:
		c.Draft = l.Draft
	case l.Drafted != nil:
		if c.Draft == nil {
			c.Draft = &Draft{By: l.Drafted.By, At: l.Drafted.At}
		}
		c.Draft.State, c.Draft.Decision = l.Drafted.State, l.Drafted.Decision
	case l.Status != nil:
		at := l.Status.At
		c.Status, c.StatusBy, c.StatusAt = l.Status.Status, l.Status.By, &at
	case l.Session != nil:
		c.Session = l.Session
	case l.Model != nil:
		if l.Model.Model == "" {
			c.Model = nil
		} else {
			c.Model = l.Model
		}
	case l.Suggest != nil:
		for i := range c.Suggestions {
			if c.Suggestions[i].State == SuggestOpen {
				c.Suggestions[i].State = SuggestSuperseded
			}
		}
		c.Suggestions = append(c.Suggestions, *l.Suggest)
	case l.Suggestion != nil:
		for i := range c.Suggestions {
			if c.Suggestions[i].N == l.Suggestion.N {
				at := l.Suggestion.At
				c.Suggestions[i].State, c.Suggestions[i].SettledBy, c.Suggestions[i].SettledAt, c.Suggestions[i].Commit = l.Suggestion.State, l.Suggestion.By, &at, l.Suggestion.Commit
			}
		}
	}
}

// workOut is what's worked out of a conversation rather than kept: whether
// the crew it was sent to have finished. It reads the mission, so it's
// done outside a transaction.
func (s Store) workOut(c *Conversation) {
	if c.Sent != nil {
		c.Sent.Finished = s.crewFinished(c.Sent)
	}
}

// add appends a line to a conversation's log in tx, has it happen to c (as
// loaded in tx), and writes c's row as it then stands.
func (s Store) add(tx *store.Tx, c *Conversation, l line) error {
	data, err := json.Marshal(l)
	if err != nil {
		return err
	}
	at := time.Now()
	if _, err := tx.Exec(`INSERT INTO conversation_lines (scope, id, n, at, line) VALUES (?, ?, ?, ?, ?)`, s.Slug, c.ID, c.lines+1, stampOf(at), string(data)); err != nil {
		return err
	}
	c.lines++
	title := c.Title
	if l.Opened == nil {
		c.apply(l)
	}
	if err := s.keepRow(tx, c, at); err != nil {
		return err
	}
	if c.Ephemeral {
		return nil // what it says isn't found, nor what it links to, until it's kept
	}
	if c.Title != title && l.Opened == nil {
		if err := tx.Retitle(s.origin(c.ID), c.Title); err != nil {
			return err
		}
	}
	switch {
	case l.Opened != nil:
		return s.linkAnchor(tx, c)
	case l.Turn != nil:
		if err := s.linkTurn(tx, c, *l.Turn); err != nil {
			return err
		}
		if err := s.sayTurn(tx, c, *l.Turn); err != nil {
			return err
		}
		if c.Kind == Pass {
			return s.sayFindings(tx, c)
		}
	case l.Settle != nil:
		return s.sayFindings(tx, c)
	case l.Proposal != nil:
		if p := c.Proposal(l.Proposal.N); p != nil {
			return s.linkMade(tx, c, *p)
		}
	}
	return nil
}

// keepRow writes what a conversation is and how it stands, changed at.
func (s Store) keepRow(tx *store.Tx, c *Conversation, changed time.Time) error {
	_, err := tx.Exec(`INSERT INTO conversations (scope, id, n, kind, what, goal, status, title, doc, pr, page, finding, reviewed, opened, changed, lines, ephemeral)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (scope, id) DO UPDATE SET kind = excluded.kind, what = excluded.what, goal = excluded.goal, status = excluded.status,
			title = excluded.title, doc = excluded.doc, pr = excluded.pr, page = excluded.page, finding = excluded.finding,
			reviewed = excluded.reviewed, changed = excluded.changed, lines = excluded.lines, ephemeral = excluded.ephemeral`,
		s.Slug, c.ID, num(c.ID), c.Kind, c.What, c.Goal, c.Status, c.Title,
		c.Anchor.Doc, c.Anchor.PR, c.Anchor.Page, c.Anchor.Finding, c.Anchor.Reviewed(),
		stampOf(c.At), stampOf(changed), c.lines, c.Ephemeral)
	if err != nil {
		return err
	}
	c.changed = changed
	s.changed(tx, c)
	if c.Ephemeral {
		return nil // nothing of it's in the graph until it's kept
	}
	return s.keepThing(tx, c, changed)
}

// changed names what a write to a conversation changed: itself, and the
// lists it's in (its mission's, or the workspace's, and its page's). An
// ask that isn't kept is in none.
func (s Store) changed(tx *store.Tx, c *Conversation) {
	if c.Ephemeral {
		tx.Changed("conversation:" + s.Slug + "/" + c.ID)
		return
	}
	tx.Changed("conversation:"+s.Slug+"/"+c.ID, "workspace")
	if !s.Workspace() {
		tx.Changed("mission:" + s.Slug)
	}
	if c.Anchor.Page != "" {
		tx.Changed("page:" + c.Anchor.Page)
	}
}

// update loads a conversation in a write and has fn change it, through add.
// Its worked-out parts are worked out after the write.
func (s Store) update(id string, fn func(tx *store.Tx, c *Conversation) error) (*Conversation, error) {
	var c *Conversation
	err := s.write(func(tx *store.Tx) error {
		var err error
		if c, err = s.load(tx, id); err != nil {
			return err
		}
		return fn(tx, c)
	})
	if err != nil {
		return nil, err
	}
	s.workOut(c)
	return c, nil
}

// next is the next conversation's number in this scope: one past the
// highest there's been, swept or not.
func (s Store) next(tx *store.Tx) (int, error) {
	var n int
	err := tx.QueryRow(`SELECT max(
		(SELECT coalesce(max(n), 0) FROM conversations WHERE scope = ?),
		(SELECT coalesce(max(n), 0) FROM conversation_numbers WHERE scope = ?)) + 1`, s.Slug, s.Slug).Scan(&n)
	return n, err
}

// answeringNow is whether a run marked as answering (by its process, since
// a time) is live and recent.
func answeringNow(pid int, since string) bool {
	if pid <= 0 || time.Since(timeOf(since)) > answerFor {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

// ids is the conversations' ids, in order.
func (s Store) ids() []string {
	var out []string
	_ = s.read(func(tx *store.Tx) error { // a read it can't make is as none
		out = s.idsIn(tx)
		return nil
	})
	return out
}

func (s Store) idsIn(tx *store.Tx) []string {
	var out []string
	rows, err := tx.Query(`SELECT id FROM conversations WHERE scope = ? ORDER BY n`, s.Slug)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	if rows.Err() != nil {
		return nil
	}
	return out
}

// latest is the latest a matching conversation changed, or zero.
func (s Store) latest(where string, args ...any) time.Time {
	var at sql.NullString
	_ = s.read(func(tx *store.Tx) error { // a read it can't make is as none
		return tx.QueryRow(`SELECT max(changed) FROM conversations WHERE scope = ? AND `+where, append([]any{s.Slug}, args...)...).Scan(&at)
	})
	return timeOf(at.String)
}

// MoveMission takes a mission's conversations with it to its new slug,
// once its folder has moved (mission.Store.Rename).
func MoveMission(root, old, slug string) error {
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), func(tx *store.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT count(*) FROM conversations WHERE scope = ?`, slug).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("the records have conversations of a mission %s already", slug)
		}
		was, now := For(root, old), For(root, slug)
		for _, id := range was.idsIn(tx) {
			if err := tx.Forget(was.origin(id)); err != nil {
				return err
			}
		}
		for _, table := range []string{"conversations", "conversation_lines", "approvals"} {
			if _, err := tx.Exec(`UPDATE `+table+` SET scope = ? WHERE scope = ?`, slug, old); err != nil {
				return err
			}
		}
		moved, err := now.loadWhere(tx, `1`)
		if err != nil {
			return err
		}
		for _, c := range moved {
			if err := now.keepGraph(tx, c, c.changed); err != nil {
				return err
			}
		}
		tx.Changed("workspace", "mission:"+old, "mission:"+slug)
		return nil
	})
}

// standingStep is the part's fifth step: a conversation written before
// things had a standing is of its moment while it's open, and settled
// once it's resolved (since when isn't known: it ages from its last word).
var standingStep = store.SQL(`
	UPDATE things SET standing = CASE state WHEN 'resolved' THEN 'settled' ELSE 'passing' END
		WHERE origin LIKE 'conversation:%' AND standing = '';
`)
