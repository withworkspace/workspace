package cli

import (
	"testing"

	"github.com/withworkspace/workspace/internal/events"
)

// An event says what changed: a thing's fields, a document as it's
// called and what became of it, how a run ended; else its words.
func TestEventSays(t *testing.T) {
	for _, c := range []struct {
		e    events.Event
		want string
	}{
		{events.Event{Kind: events.ThingChanged, Title: "Token refresh", Changes: []events.FieldChange{
			{Field: "status", From: "In Progress", To: "Done"}, {Field: "assignee", To: "Dana"},
			{Field: "body"}, {Field: "sprint", From: "Sprint 4"},
		}}, "Token refresh: status In Progress → Done, assignee → Dana, body changed, sprint cleared (was Sprint 4)"},
		{events.Event{Kind: events.ThingChanged, Changes: []events.FieldChange{{Field: "status", From: "a", To: "b"}}}, "status a → b"},
		{events.Event{Kind: events.DocChanged, Title: "One queue", Status: "added", Text: "Decide one queue"}, "One queue (added)"},
		{events.Event{Kind: events.RunEnded, Title: "test", Status: "failed (exit 1)", Text: "test"}, "failed (exit 1)"},
		{events.Event{Kind: events.CommitsLanded, Text: "Decide one queue"}, "Decide one queue"},
		{events.Event{Kind: events.PROpened, Title: "Queue per tenant"}, "Queue per tenant"},
	} {
		if got := eventSays(c.e); got != c.want {
			t.Errorf("%s says %q, want %q", c.e.Kind, got, c.want)
		}
	}
}
