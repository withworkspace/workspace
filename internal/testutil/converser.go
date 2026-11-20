package testutil

import (
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/harness"
)

// FakeConverser answers a conversation's turns as Claude Code would print them:
// its session (a new one, or the one resumed) and its reply, which says
// which prompt it got and on what model.
type FakeConverser struct{ Fail bool }

func (f FakeConverser) Name() string {
	if f.Fail {
		return "fail-converse"
	}
	return "fake-converse"
}
func (FakeConverser) Available() error { return nil }
func (FakeConverser) Start(harness.Brief) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (FakeConverser) Resume(string, string) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (FakeConverser) ConverseModel() string { return "fake-small" }
func (f FakeConverser) Converse(r harness.ConverseRun) (harness.Launch, error) {
	if f.Fail {
		return harness.Launch{Args: []string{"/bin/sh", "-c", "echo 'not signed in' >&2; exit 1"}, Dir: r.Dir, Stream: harness.StreamClaude}, nil
	}
	session, reply := r.Session, "first: "
	if session == "" {
		session = "sess-1"
	} else {
		reply = "next: "
	}
	switch {
	case r.Talk && strings.Contains(r.Prompt, "You're briefing a new mission"):
		reply = "Which repo?"
		if strings.Contains(r.Prompt, "ws show -f llm w1") {
			reply += " (from w1)"
		}
		reply += `\nTitle: Briefing\nSummary: Briefing it.`
	case r.Talk && strings.Contains(r.Prompt, "Talk tab"):
		reply = "about the mission"
		if strings.Contains(r.Prompt, "ws mission dispatch --propose") && strings.Contains(r.Prompt, "Start with `ws show -f llm`") {
			reply += ", knowing what it may propose"
		}
		reply += `\nTitle: Stage 2\nSummary: Whether stage 2 is right.`
	case r.Talk && strings.Contains(r.Prompt, "is reviewing a pull request"):
		reply = "about the pull request"
		if strings.Contains(r.Prompt, "Start with `ws show -f llm backend#7`") && strings.Contains(r.Prompt, "ws pr comment --propose backend#7") {
			reply += ", knowing where to start and what it may propose"
		}
		if strings.Contains(r.Prompt, "line 1 of existing.txt") {
			reply += ", and which lines"
		}
		for _, p := range r.ReadPaths {
			if strings.HasSuffix(p, "repos/backend/existing") {
				reply += ", reading where it is docked"
			}
		}
		reply += `\nTitle: The existing file\nSummary: What it is.`
	case r.Talk:
		reply = "talked"
		if session == "sess-1" && r.Session == "" {
			reply = "first: talked"
		}
		reply += fmt.Sprintf(" (%d paths)\\nTitle: Token cache\\nSummary: What a cache would take.", len(r.ReadPaths))
	case strings.Contains(r.Prompt, "You are writing a pull request's title and description"):
		// The PR writer: titled for the steps it was told are its task,
		// and saying whether the workspace's PR.md was given.
		var steps []string
		for _, l := range strings.Split(r.Prompt, "\n") {
			if n, ok := strings.CutPrefix(l, "Step "); ok {
				if i := strings.Index(n, " ("); i > 0 {
					steps = append(steps, n[:i])
				}
			}
		}
		title := "The goal, written"
		if len(steps) > 0 {
			title = "Steps " + strings.Join(steps, ",") + " written"
		}
		reply = "<title>" + title + "</title>\\n<body>\\nWritten for the task."
		if strings.Contains(r.Prompt, "The workspace says so in `PR.md`") {
			reply += " As PR.md says."
		}
		reply += "\\n</body>"
	case strings.Contains(r.Prompt, "draft it as a decision"):
		reply = `What: Key the cache per tenant.\nWhy: Tokens are issued per tenant,\nnot per user.` // \n: a newline in the JSON
	case strings.Contains(r.Prompt, "Start with `ws show -f llm t"):
		reply += "read the thread"
	case strings.Contains(r.Prompt, "The user replied in"):
		reply += "read the reply"
	}
	if !strings.HasPrefix(reply, "What:") && !r.Talk {
		reply += " on " + r.Model
	}
	script := fmt.Sprintf(`printf '%%s\n' '{"type":"system","subtype":"init","session_id":"%s"}' '{"type":"result","subtype":"success","is_error":false,"result":"%s","session_id":"%s"}'`, session, reply, session)
	return harness.Launch{Args: []string{"/bin/sh", "-c", script}, Dir: r.Dir, Stream: harness.StreamClaude}, nil
}
