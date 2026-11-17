package page

import (
	"bytes"

	"github.com/withworkspace/workspace/internal/conversation"
)

// A cell of a page is one of the workspace's conversations (its kind
// Cell), marked in the page's text where its agent's reply goes. What's
// here is where a cell stands against the page it's in.

// HasCell is whether a page's text holds the cell id: one asked for and
// since taken out is still a conversation, but isn't the page's.
func HasCell(text []byte, id string) bool {
	return bytes.Contains(text, []byte("ws:cell "+id+" "))
}

// CellMoved says what the cell c was written beside has changed since:
// "above", a cell before it in the page; or "page", the page around it,
// for one that's about the whole page (it has a goal). "" for neither, or
// a cell not written yet. text is its page's.
func CellMoved(text []byte, c *conversation.Conversation) string {
	w := c.Written()
	if w == nil {
		return ""
	}
	if c.Goal != "" && c.Goal != conversation.GoalAsk {
		if now := Around(text, c.ID); w.Around != "" && now != "" && now != w.Around {
			return "page"
		}
		return ""
	}
	if now := Above(text, c.ID); w.Above != "" && now != "" && now != w.Above {
		return "above"
	}
	return ""
}

// StaleCell is whether the cell c, filled and not being written again,
// was written from a file that's changed since (root is the workspace's),
// or beside what has. text is its page's.
func StaleCell(root string, text []byte, c *conversation.Conversation) bool {
	w := c.Written()
	if w == nil || c.Answering {
		return false
	}
	if CellMoved(text, c) != "" {
		return true
	}
	for _, src := range w.Sources {
		if src.State(root) != conversation.SourceSame {
			return true
		}
	}
	return false
}
