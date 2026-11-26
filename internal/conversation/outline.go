package conversation

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// Outlines: asked for more than one block of a page (several sections, or
// things in several places), a cell's agent plans first. Its plan is the
// blocks, each with where it goes, the heading it goes under and a sentence
// on what it'll say. The page puts those in at once, empty, so you see
// what's coming and can go on writing; then the same agent, in the same
// session, writes each block in turn.

// Block is a block of the page an outline plans: where it goes (After: the
// words of a heading already in the page, at the end of whose section it
// goes; or here, start or end), the heading it goes under (none for a
// block that needs none) and that heading's level (0: as its
// neighbours), and a sentence on what it'll say, shown in the page until
// it's written.
type Block struct {
	Key     string `json:"key"`
	After   string `json:"after,omitempty"`
	Heading string `json:"heading,omitempty"`
	Level   int    `json:"level,omitempty"`
	Summary string `json:"summary"`
}

// BlockRoom is how many blocks an outline may plan.
const BlockRoom = 12

var (
	outlineLine = regexp.MustCompile(`(?i)^\s*<!--[ \t]*outline[ \t]*-->[ \t]*\r?(\n|$)`)
	blockLine   = regexp.MustCompile(`(?i)^={2,}[ \t]*block\b.*$`)
	blockField  = regexp.MustCompile(`(?i)^(after|heading|level|summary)[ \t]*:[ \t]*(.*)$`)
)

// ParseOutline reads a plan: a reply whose first line is only
// <!-- outline -->, then (after anything it says of the plan) each block:
//
//	=== block
//	after: Constraints
//	heading: Dependencies
//	summary: Which services this needs, and what each must provide.
//
// ok is false for a reply that isn't one, or plans nothing.
func ParseOutline(reply string) (lead string, blocks []Block, ok bool) {
	at := outlineLine.FindStringIndex(reply)
	if at == nil {
		return "", nil, false
	}
	var head []string
	var cur *Block
	field := ""
	flush := func() {
		if cur != nil && strings.TrimSpace(cur.Summary+cur.Heading) != "" && len(blocks) < BlockRoom {
			cur.Key = "b" + strconv.Itoa(len(blocks)+1)
			cur.After, cur.Heading = strings.TrimSpace(cur.After), strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(cur.Heading), "# "))
			cur.Summary = strings.Join(strings.Fields(cur.Summary), " ")
			if cur.Summary == "" {
				cur.Summary = cur.Heading
			}
			blocks = append(blocks, *cur)
		}
		cur = nil
	}
	for line := range strings.SplitSeq(strings.ReplaceAll(reply[at[1]:], "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		switch m := blockField.FindStringSubmatch(t); {
		case blockLine.MatchString(t):
			flush()
			cur, field = &Block{}, ""
		case cur == nil:
			head = append(head, line)
		case m != nil:
			field = strings.ToLower(m[1])
			v := strings.Trim(strings.TrimSpace(m[2]), "`")
			switch field {
			case "after":
				cur.After = strings.TrimLeft(v, "# ")
			case "heading":
				cur.Heading = v
			case "level":
				cur.Level, _ = strconv.Atoi(v)
				if cur.Level < 1 || cur.Level > 6 {
					cur.Level = 0
				}
			case "summary":
				cur.Summary = v
			}
		case field == "summary" && t != "":
			cur.Summary += " " + t
		}
	}
	flush()
	return strings.TrimSpace(strings.Join(head, "\n")), blocks, len(blocks) > 0
}

// Plan is the blocks a cell's agent planned, in the order they go in the
// page, or nil if it planned none (it wrote the cell).
func (c *Conversation) Plan() []Block {
	for _, v := range slices.Backward(c.Turns) {
		if len(v.Blocks) > 0 {
			return v.Blocks
		}
	}
	return nil
}

// blockOf is the planned block with a key.
func (c *Conversation) blockOf(key string) (Block, bool) {
	for _, b := range c.Plan() {
		if b.Key == key {
			return b, true
		}
	}
	return Block{}, false
}

// BlockOf is the planned block with a key.
func (c *Conversation) BlockOf(key string) (Block, bool) { return c.blockOf(key) }

// NextBlock is the first planned block its agent hasn't written yet (or
// been told to leave out), or nil when they're all written.
func (c *Conversation) NextBlock() *Block {
	plan := c.Plan()
	settled := map[string]bool{}
	for _, t := range c.Turns {
		if t.Block != "" && ((t.By == Agent && !t.Answer) || t.Skipped) {
			settled[t.Block] = true
		}
	}
	for i := range plan {
		if !settled[plan[i].Key] {
			return &plan[i]
		}
	}
	return nil
}

// Working is the block a cell's agent is on now, or is to do next: the one
// your latest unanswered turn is about, else the next to be written. ""
// for none (or for a cell with no plan).
func (c *Conversation) Working() string {
	if c.asked().By == You {
		return c.asked().Block
	}
	if b := c.NextBlock(); b != nil {
		return b.Key
	}
	return ""
}

// YourTurn is whether the latest word in a cell is yours, still to be
// answered (rather than its agent's, with blocks of an outline to go on
// to).
func (c *Conversation) YourTurn() bool { return c.asked().By == You }

// BlockID names a planned block as a cell of its own: its outline's id and
// its key (w12-b2).
func BlockID(outline, key string) string { return outline + "-" + key }

// SplitBlockID takes a block's id apart; key is "" for an id that isn't a
// block's.
func SplitBlockID(id string) (outline, key string) {
	if i := strings.IndexByte(id, '-'); i > 0 {
		return id[:i], id[i+1:]
	}
	return id, ""
}

// View is a planned block as a cell of its own: what it's to say is its
// first message, each version its agent wrote of it and what you said of
// it its turns. It's answering while its outline's agent is on it.
func (c *Conversation) View(b Block) *Conversation {
	v := &Conversation{ID: BlockID(c.ID, b.Key), Kind: Cell, Anchor: Anchor{Page: c.Anchor.Page}, By: c.By, At: c.At, Status: c.Status}
	planned := c.At
	for _, t := range c.Turns {
		if len(t.Blocks) > 0 {
			planned = t.At
		}
	}
	v.Turns = []Turn{{N: 1, By: You, At: planned, Body: b.Summary}}
	for _, t := range c.Turns {
		if t.Block != b.Key || t.Skipped {
			continue
		}
		t.N = len(v.Turns) + 1
		v.Turns = append(v.Turns, t)
	}
	v.Answering = c.Answering && c.Working() == b.Key
	return v
}

// Views are a cell as the page has it: itself, or, where its agent
// planned an outline, each of its blocks as a cell of its own.
func (c *Conversation) Views() []*Conversation {
	plan := c.Plan()
	if len(plan) == 0 {
		return []*Conversation{c}
	}
	out := make([]*Conversation, 0, len(plan))
	for _, b := range plan {
		out = append(out, c.View(b))
	}
	return out
}

// MarkPlaced records that an outline's blocks have been put into its page
// (by the page, in the browser): they're placed once, so a block you've
// taken out isn't put back.
func (s Store) MarkPlaced(id string, at time.Time) error {
	_, err := s.update(id, func(tx *store.Tx, c *Conversation) error { return s.add(tx, c, line{Placed: &at}) })
	return err
}
