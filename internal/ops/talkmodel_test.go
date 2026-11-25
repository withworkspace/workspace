package ops

import (
	"testing"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/harness"
)

// A conversation's turns run on the model chosen for it, where it was
// chosen on the harness its role runs on now; otherwise on the role's.
func TestChosenModel(t *testing.T) {
	role := harness.Pick{Harness: "claude", Model: "sonnet"}
	for _, tc := range []struct {
		c    *conversation.Conversation
		want string
	}{
		{&conversation.Conversation{}, "sonnet"},
		{&conversation.Conversation{Model: &conversation.Model{Harness: "claude", Model: "opus"}}, "opus"},
		{&conversation.Conversation{Model: &conversation.Model{Harness: "cursor", Model: "gpt-5.5-high"}}, "sonnet"},
		{nil, "sonnet"},
	} {
		if got := chosenModel(role, tc.c); got.Model != tc.want || got.Harness != "claude" {
			t.Errorf("%+v: %+v", tc.c, got)
		}
	}
}
