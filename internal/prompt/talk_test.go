package prompt

import (
	"regexp"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/conversation"
)

// everyAsk finds the rule that every ask of a message is placed.
var everyAsk = regexp.MustCompile(`(?i)\b(every|each) ask`)

// paragraph is the paragraph (or list) of a prompt that re matches in, or
// "".
func paragraph(p string, re *regexp.Regexp) string {
	for _, para := range strings.Split(p, "\n\n") {
		if re.MatchString(para) {
			return para
		}
	}
	return ""
}

// A mission's conversation places every ask of a message that holds more
// than one, and says where each goes: a replan, an edit to the brief or a
// decision, or a new mission. It can ask the mission's planner to replan,
// for the user to confirm, so "can you replan?" is answered with a
// proposal. (In daemon-idle-timeout-of-3-hours, t2, an ask split off went
// nowhere, and "can you replan?" was answered "I can't".) A workspace's
// conversation has no plan to change.
func TestTalkFirst_MissionPlacesEveryAsk(t *testing.T) {
	talk := &conversation.Conversation{ID: "t2", Kind: conversation.Talk, Turns: []conversation.Turn{you(1, "Keep it open until it's closed, and make the AppBar's workspace title a dropdown.")}}
	p := TalkFirst(goldenHere, talk)
	if !strings.Contains(p, "ws mission replan --propose") {
		t.Error("a mission's conversation isn't offered the replan proposal (ws mission replan --propose)")
	}
	rule := paragraph(p, everyAsk)
	if rule == "" {
		t.Fatalf("a mission's conversation has no rule to place every ask of a message:\n%s", p)
	}
	for _, where := range []string{"replan", "brief", "decision", "mission new --propose"} {
		if !strings.Contains(rule, where) {
			t.Errorf("the rule to place every ask doesn't say one can go to %q:\n%s", where, rule)
		}
	}

	ws := goldenHere
	ws.Mission = nil
	if p := TalkFirst(ws, talk); strings.Contains(p, "ws mission replan") {
		t.Errorf("a workspace's conversation is offered a replan, with no mission to replan:\n%s", p)
	}
}
