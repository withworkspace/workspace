package conversation

import (
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// What a conversation is in the graph: a thing, t3 in its mission or w4 in
// the workspace, written with each of its lines; and its links, written
// with the line that implies them: what it's about (a document, a pull
// request, a page, a capsule reviewed), the page a cell is in, both when
// it's opened; what each turn mentions, as the turn pinned it (Turn.Refs),
// when the turn's said; and a document it proposed, once it's added,
// coming from it. What it says is its words: each turn, and a pass's
// findings, those of its latest run you haven't dismissed.

// origin is the record a conversation's thing and links are of.
func (s Store) origin(id string) store.Origin {
	return store.Origin("conversation:" + s.Slug + "/" + id)
}

func (s Store) key(id string) store.Key { return store.Key{Scope: s.Slug, Name: id} }

// keepThing writes what the conversation is, as it now stands.
func (s Store) keepThing(tx *store.Tx, c *Conversation, changed time.Time) error {
	title := c.Title
	if title == "" && len(c.Turns) > 0 {
		title = oneLine(c.Turns[0].Body)
	}
	return tx.Thing(s.origin(c.ID), store.Thing{Key: s.key(c.ID), Kind: c.What, Title: title, State: c.Status, Changed: changed})
}

// linkAnchor links a conversation to what it's anchored to.
func (s Store) linkAnchor(tx *store.Tx, c *Conversation) error {
	o, me, a := s.origin(c.ID), s.key(c.ID), c.Anchor
	link := func(kind string, dst store.Key, lines string) error {
		return tx.Link(o, store.Link{Src: me, Kind: kind, Dst: dst, Lines: lines, At: c.At})
	}
	if a.Doc != "" {
		for _, k := range docKeys(s.Slug, a.Doc) {
			if err := link(store.About, k, lines(a.Line, a.EndLine)); err != nil {
				return err
			}
		}
	}
	if a.PR != "" {
		if err := link(store.About, store.Key{Name: a.PR}, ""); err != nil {
			return err
		}
	}
	if a.Capsule != "" {
		if err := link(store.About, store.Key{Name: a.Capsule}, ""); err != nil {
			return err
		}
	}
	if a.Page != "" {
		kind := store.About // a pass over the page
		if c.Kind == Cell {
			kind = store.In
		}
		if err := link(kind, store.Key{Name: a.Page}, ""); err != nil {
			return err
		}
	}
	return nil
}

// linkTurn links a conversation to what a turn of it names, each once: as
// the turn pinned them when it was said, which is why its words aren't
// read for names again where they're written (store.Place's Pinned).
func (s Store) linkTurn(tx *store.Tx, c *Conversation, t Turn) error {
	seen := map[store.Key]map[string]bool{}
	for _, p := range t.Refs {
		k, at := store.KeyOf(p.Ref)
		if k.Name == "" || seen[k][at] {
			continue
		}
		if seen[k] == nil {
			seen[k] = map[string]bool{}
		}
		seen[k][at] = true
		if err := tx.Link(s.origin(c.ID), store.Link{Src: s.key(c.ID), Kind: store.Mentions, Dst: k, Lines: at, At: t.At}); err != nil {
			return err
		}
	}
	return nil
}

// linkMade links what accepting a proposal made to the conversation it
// came from: a document added (a mission is linked by its own record).
func (s Store) linkMade(tx *store.Tx, c *Conversation, p Proposal) error {
	at := c.At
	if p.SettledAt != nil {
		at = *p.SettledAt
	}
	if p.Kind == ProposePlugin && p.State == ProposalAccepted {
		// What a plugin's items made or changed, by its name (RAM-130),
		// came from it.
		for _, name := range p.made() {
			if err := tx.Link(s.origin(c.ID), store.Link{Src: store.Key{Name: name}, Kind: store.From, Dst: s.key(c.ID), At: at}); err != nil {
				return err
			}
		}
		return nil
	}
	if p.Kind != ProposeDocument || p.State != ProposalAccepted || p.Result == "" {
		return nil
	}
	for _, k := range docKeys(s.Slug, p.Result) {
		if err := tx.Link(s.origin(c.ID), store.Link{Src: k, Kind: store.From, Dst: s.key(c.ID), At: at}); err != nil {
			return err
		}
	}
	return nil
}

// Kinds of place a conversation's words are at.
const (
	placeTurn    = "turn"
	placeFinding = "finding"
)

// sayTurn writes what a turn says: a place at its number. What a cell's
// agent writes is a version of the page's cell, which is the page's words
// to say, not the conversation's; its answers to a question are said.
func (s Store) sayTurn(tx *store.Tx, c *Conversation, t Turn) error {
	if c.Kind == Cell && t.By == Agent && !t.Answer {
		return nil
	}
	body := strings.TrimSpace(t.Body)
	if body == "" {
		return nil
	}
	return tx.Words(s.origin(c.ID), store.Place{Thing: s.key(c.ID), At: strconv.Itoa(t.N), Kind: placeTurn, Title: c.Title, Text: body, Said: t.At, Pinned: true})
}

// sayFindings writes again what a pass found: its latest run's findings,
// each at its key, those you've dismissed left out.
func (s Store) sayFindings(tx *store.Tx, c *Conversation) error {
	o := s.origin(c.ID)
	if err := tx.ForgetWords(o, placeFinding); err != nil {
		return err
	}
	w := c.Written()
	if c.Kind != Pass || w == nil {
		return nil
	}
	for _, f := range w.Findings {
		if c.FindingState(f.Key) == FindingDismissed {
			continue
		}
		text := strings.TrimSpace(strings.TrimSpace(f.Text) + "\n\n" + strings.TrimSpace(f.Suggest))
		if err := tx.Words(o, store.Place{Thing: s.key(c.ID), At: f.Key, Kind: placeFinding, Title: c.Title, Text: text, Said: w.At, Pinned: true}); err != nil {
			return err
		}
	}
	return nil
}

// sayWords writes all a conversation says again: what's there of it is
// forgotten first. An ask that isn't kept says nothing.
func (s Store) sayWords(tx *store.Tx, c *Conversation) error {
	if err := tx.ForgetWords(s.origin(c.ID), ""); err != nil {
		return err
	}
	if c.Ephemeral {
		return nil
	}
	for _, t := range c.Turns {
		if err := s.sayTurn(tx, c, t); err != nil {
			return err
		}
	}
	return s.sayFindings(tx, c)
}

// keepGraph writes all of a conversation's graph again, its words with it:
// what's there of it is forgotten first. An ask that isn't kept has none.
func (s Store) keepGraph(tx *store.Tx, c *Conversation, changed time.Time) error {
	if err := tx.Forget(s.origin(c.ID)); err != nil {
		return err
	}
	if c.Ephemeral {
		return nil
	}
	if err := s.keepThing(tx, c, changed); err != nil {
		return err
	}
	if err := s.linkAnchor(tx, c); err != nil {
		return err
	}
	for _, t := range c.Turns {
		if err := s.linkTurn(tx, c, t); err != nil {
			return err
		}
	}
	for _, p := range c.Proposals {
		if err := s.linkMade(tx, c, p); err != nil {
			return err
		}
	}
	return s.sayWords(tx, c)
}

// docKeys are what a thread's document is in the graph: a file on the
// default branch (repo:path, as it's stored) is the repo's file; a
// capsule's file is that, and the repo's file as well, since the capsule
// won't outlive the question; a mission's own file (mission:path) is read
// in the mission; anything else is as it's stored.
func docKeys(scope, doc string) []store.Key {
	switch {
	case strings.HasPrefix(doc, "mission:"):
		return []store.Key{{Scope: scope, Name: doc}}
	case strings.HasPrefix(doc, "http://"), strings.HasPrefix(doc, "https://"):
		return []store.Key{{Name: doc}}
	}
	if _, _, ok := ref.ParsePR(doc); ok {
		return []store.Key{{Name: doc}}
	}
	r, ok := ref.Parse(doc)
	if !ok || r.Kind != ref.File {
		return []store.Key{{Scope: scope, Name: doc}}
	}
	r.Line, r.EndLine = 0, 0
	out := []store.Key{{Name: r.Name()}}
	if r.InCapsule() {
		r.Worktree = ref.Ground
		out = append(out, store.Key{Name: r.Name()})
	}
	return out
}

// lines is how a link says where in a file: 17, or 17-20.
func lines(line, end int) string {
	switch {
	case line <= 0:
		return ""
	case end > line:
		return strconv.Itoa(line) + "-" + strconv.Itoa(end)
	}
	return strconv.Itoa(line)
}
