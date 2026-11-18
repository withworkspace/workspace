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

// An agent whose shell is sandboxed is told it can run things, where it
// writes, that ws and gh run alone, and how to lift a capsule of the
// conversation's own and carry on what it tries there; one without is
// told, as before, that it only reads. A later turn says how the shell is
// now, for a session begun when it could only read, and what's lifted.
func TestTalk_Sandboxed(t *testing.T) {
	talk := &conversation.Conversation{ID: "w3", Kind: conversation.Talk, What: conversation.WhatTalk, Turns: []conversation.Turn{you(1, "How slow are our tests, any perf left on the table?")}}
	h := goldenHere
	h.Mission = nil
	plain := TalkFirst(h, talk)
	if !strings.Contains(plain, "You can read anything here, and change nothing") || strings.Contains(plain, "sandbox") || strings.Contains(plain, "ws lift") {
		t.Errorf("an agent with no sandbox is told of one:\n%s", plain)
	}
	if next := TalkNext(h, talk); !strings.Contains(next, "The shell works for the commands your first message listed") {
		t.Errorf("its later turn: %s", next)
	}

	h.Sandbox = &Sandbox{Own: "/tmp/ws-1/workspace/w3"}
	p := TalkFirst(h, talk)
	for _, want := range []string{
		"run what you like: your shell is in a sandbox",
		"say what you found rather than what you'd expect",
		"/tmp/ws-1/workspace/w3",
		"operation not permitted",
		"`ws` and `gh` run outside the sandbox, so each runs on its own",
		"gh run list|view",
		"`ws lift <repo> <name>`",
		"It's this conversation's, not a mission's:",
		"put away when the user puts this conversation away",
		"don't push it or open a pull request",
		"propose a mission (`ws mission new --propose`, below) whose brief says what you did and names the branch",
		"What's the workspace's (its missions, decisions, documents and artifacts) you don't change yourself, but you can propose",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("a sandboxed agent isn't told %q:\n%s", want, p)
		}
	}
	for _, never := range []string{"change nothing:", "Run one command at a time, with nothing chained", "Capsules you've lifted"} {
		if strings.Contains(p, never) {
			t.Errorf("a sandboxed agent is told %q", never)
		}
	}

	// On a mission's Talk tab, a capsule it lifts isn't the mission's.
	m := goldenHere
	m.Sandbox = h.Sandbox
	if p := TalkFirst(m, talk); !strings.Contains(p, "not this mission's either: its capsules are its plan's") {
		t.Errorf("a mission's conversation isn't told its capsules aren't the mission's:\n%s", p)
	}

	// With capsules, it's told which are there and which were put away;
	// and one that can't lift, why not, with no way to.
	h.Sandbox = &Sandbox{Own: "/tmp/ws-1/workspace/w3",
		Capsules: []SandboxCapsule{{Repo: "api", Name: "parallel-tests", Branch: "parallel-tests", Path: "/ws/repos/api/parallel-tests"}},
		Away:     []SandboxCapsule{{Repo: "web", Name: "vitest-pool", Branch: "vitest-pool"}}}
	next := TalkNext(h, talk)
	for _, want := range []string{"Your shell is in a sandbox", "If your first message said you may only read", "ws lift <repo> <name> [<branch>]",
		"- api/parallel-tests, at /ws/repos/api/parallel-tests (branch parallel-tests)", "- web/vitest-pool: `ws lift web vitest-pool` brings it back"} {
		if !strings.Contains(next, want) {
			t.Errorf("a later turn doesn't say %q:\n%s", want, next)
		}
	}
	h.Sandbox = &Sandbox{Own: "/tmp/x", NoLift: "this is an ask nobody's kept"}
	if p := TalkFirst(h, talk); !strings.Contains(p, "You can't lift a capsule here: this is an ask nobody's kept.") || strings.Contains(p, "`ws lift <repo> <name>` makes") {
		t.Errorf("one that can't lift:\n%s", p)
	}
	if next := TalkNext(h, talk); strings.Contains(next, "ws lift") {
		t.Errorf("one that can't lift, later: %s", next)
	}

	// Which conversations run things: those that talk.
	for what, want := range map[string]bool{conversation.WhatTalk: true, conversation.WhatPR: true, conversation.WhatDoc: true, conversation.WhatBriefing: true,
		conversation.WhatCell: false, conversation.WhatPass: false, conversation.WhatThreadAsk: false, conversation.WhatReviewPass: false} {
		if got := Sandboxed(&conversation.Conversation{What: what}); got != want {
			t.Errorf("Sandboxed(%s) = %v", what, got)
		}
	}
}
