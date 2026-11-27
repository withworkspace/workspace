package conversation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// Asks: a question asked from an ask bar, about a document as you read it
// (and a passage of it, if you'd picked one out), or a pull request as
// you review it (and lines of its diff). It's a conversation from the
// start, a Talk about the document (Anchor.Doc, and Anchor.Quote for the
// passage) or the pull request (Anchor.PR, and Anchor.Path and its
// lines), answered as any is, so a follow-up is another turn. But it's
// ephemeral until it's kept: left out of the lists, the mission's log,
// the graph and what's found, and swept by the daemon (SweepAsks) once
// it's sat AskFor after its last turn, never while it's being answered.

// AskFor is how long an ask that isn't kept stays after its last turn.
const AskFor = time.Hour

// SweepEvery is how often the daemon looks for asks to sweep.
const SweepEvery = 5 * time.Minute

// OpenAsk starts an ask about what anchor names (a document's ref, and
// the passage asked about; or a pull request's, and the lines), with the
// question asked.
func (s Store) OpenAsk(anchor Anchor, by, body string, at time.Time) (*Conversation, error) {
	return s.OpenAskTurn(anchor, Turn{By: by, At: at, Body: body})
}

// OpenAskTurn starts an ask with its first turn as it's given: with what
// the page it was asked on gave the agent with it, say.
func (s Store) OpenAskTurn(anchor Anchor, t Turn) (*Conversation, error) {
	by, at := t.By, t.At
	t.Body = strings.TrimSpace(t.Body)
	switch {
	case t.Body == "":
		return nil, errors.New("say something")
	case anchor.Doc == "" && anchor.PR == "" && anchor.Thing == "":
		return nil, errors.New("an ask needs a ref: a document, a pull request or a plugin's thing")
	}
	if q := anchor.Quote; q != nil {
		anchor.Quote = &Quote{Exact: clipRunes(q.Exact, quoteMax), Prefix: lastRunes(q.Prefix, contextMax), Suffix: clipRunes(q.Suffix, contextMax)}
		if strings.TrimSpace(anchor.Quote.Exact) == "" {
			anchor.Quote = nil
		}
	}
	return s.open(&Conversation{Kind: Talk, Anchor: anchor, By: by, At: at, Status: Open, Ephemeral: true}, t)
}

// ErrKeepNotYours is the answer for anyone but the user keeping an ask.
var ErrKeepNotYours = errors.New("only the user keeps an ask")

// Keep keeps an ask (Keep as a conversation): from then on it's a
// conversation like any, listed with the rest, about the document or pull
// request it was asked of, and the sweep leaves it. What it's said so far, and what it's
// about, are written into the graph and the words as it's kept. Only the
// user keeps one; keeping one that's kept, or any other conversation, does
// nothing.
func (s Store) Keep(id, by string, at time.Time) (*Conversation, error) {
	if by != You {
		return nil, ErrKeepNotYours
	}
	kept := false
	c, err := s.update(id, func(tx *store.Tx, c *Conversation) error {
		if !c.Ephemeral {
			return nil
		}
		kept = true
		if err := s.add(tx, c, line{Kept: &keptLine{By: by, At: at}}); err != nil {
			return err
		}
		return s.keepGraph(tx, c, c.changed)
	})
	if err != nil || !kept {
		return c, err
	}
	return c, s.log(c, by, c.ID+" kept: "+oneLine(c.Turns[0].Body), at)
}

// SweepAsks removes the asks nobody kept whose last turn is AskFor before
// now, in every scope, but any being answered: their lines, their rows,
// and what their runs printed. A number swept isn't given again. It's
// what it swept, each as w4 or <mission>/t3.
func SweepAsks(ctx context.Context, root string, now time.Time) ([]string, error) {
	db, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	before := now.Add(-AskFor)
	var swept []string
	err = db.Write(ctx, func(tx *store.Tx) error {
		swept = nil
		// What's changed since can't be due: a turn is a line, and a line
		// is a change.
		rows, err := tx.Query(`SELECT DISTINCT scope FROM conversations WHERE ephemeral = 1 AND changed < ?`, stampOf(before))
		if err != nil {
			return err
		}
		var scopes []string
		for rows.Next() {
			var scope string
			if err := rows.Scan(&scope); err != nil {
				rows.Close()
				return err
			}
			scopes = append(scopes, scope)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, scope := range scopes {
			s := For(root, scope)
			due, err := s.loadWhere(tx, `c.ephemeral = 1 AND c.changed < ?`, stampOf(before))
			if err != nil {
				return err
			}
			for _, c := range due {
				if !c.Ephemeral || c.Answering || !c.Last().At.Before(before) {
					continue
				}
				if err := s.sweep(tx, c); err != nil {
					return err
				}
				id := c.ID
				if scope != "" {
					id = scope + "/" + c.ID
				}
				swept = append(swept, id)
			}
		}
		return nil
	})
	return swept, err
}

// sweep removes an ask in tx: what its run printed goes once it's done.
func (s Store) sweep(tx *store.Tx, c *Conversation) error {
	for _, q := range []string{`DELETE FROM conversation_lines WHERE scope = ? AND id = ?`, `DELETE FROM conversations WHERE scope = ? AND id = ?`} {
		if _, err := tx.Exec(q, s.Slug, c.ID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO conversation_numbers (scope, n) VALUES (?, ?)
		ON CONFLICT (scope) DO UPDATE SET n = max(n, excluded.n)`, s.Slug, num(c.ID)); err != nil {
		return err
	}
	if err := tx.Forget(s.origin(c.ID)); err != nil { // none, unless something wrote it
		return err
	}
	tx.Changed("conversation:" + s.Slug + "/" + c.ID)
	run := filepath.Join(s.dir(), c.ID+".run")
	tx.OnCommit(func() { os.Remove(run) })
	return nil
}
