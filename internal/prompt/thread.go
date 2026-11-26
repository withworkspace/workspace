package prompt

import (
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
)

// What a thread's agent is told (on a document, in the margin), and what a
// resolved thread's agent is asked to draft of what was decided, beside
// ParseDraft, which reads it.

var threadKindIntro = map[string]string{
	conversation.Ask:    "an ask: they want an answer",
	conversation.Change: "a change: they want the document changed",
	conversation.Note:   "a note, which they've since replied in",
}

var threadKindGuide = map[string]string{
	conversation.Ask:    "Answer the question, citing what you read it in (refs like s1, c2, d1, or the file). If they ask for an edit, suggest it with `ws thread suggest %[1]s` (see its --help) and say so.",
	conversation.Change: "Suggest the edit: the user accepts it with a key, and ws writes it in. Run `ws thread suggest %[1]s` with, on stdin, the markdown to replace exactly as it is in the file (line breaks and all: it must appear there once), then what it should read, between conflict markers:\n\n    ws thread suggest %[1]s <<'EOF'\n    <<<<<<<\n    <the markdown as it is>\n    =======\n    <what it should read>\n    >>>>>>>\n    EOF\n\nIf it says the text isn't there once, read the file and quote it exactly. Then reply in a line or two: what you suggested, and why. If it's more than an edit to this text (a rewrite, or work in code), don't suggest: say so, and that it can go to crew.",
	conversation.Note:   "Answer what they've asked.%.0[1]s",
}

// ThreadFirst starts a thread's agent: its role, what it's about,
// and where to start reading.
func ThreadFirst(h Here, c *conversation.Conversation) string {
	m := h.Mission
	var b strings.Builder
	fmt.Fprintf(&b, "You're answering a comment on a document, in a multi-repo workspace managed by the ws tool, for the mission %q (%s; WS_MISSION is set).\n\n", m.Title, m.Slug)
	b.WriteString(para(h.Agents))
	b.WriteString(h.Kind)
	fmt.Fprintf(&b, "The user highlighted text in %s and opened thread %s: %s. ", c.Anchor.Doc, c.ID, threadKindIntro[c.Kind])
	fmt.Fprintf(&b, "Start with `ws show -f llm %s`: the text it's on, the markdown lines that text is in, and what's been said. It links to the document, and `ws show -f llm <ref>` shows any linked thing in turn: the document's history and the stage it was made for, who wrote it and their handoff, the decisions about it. ", c.ID)
	b.WriteString("Read the document itself, and the code it talks about, as you need. You may only read: you can't change anything. Run one command at a time, with nothing chained (no ;, && or pipes): `ws show`, `ws thread suggest` and reading files are allowed, anything else is refused, and a refusal is of that command only.\n\n")
	b.WriteString("Then reply. Your final message is your reply in the thread, which the user reads in the margin beside the document: short (a few sentences; markdown is fine), plain, and to the point. ")
	fmt.Fprintf(&b, threadKindGuide[c.Kind], c.ID)
	b.WriteString(" If what ws records doesn't tell you, say what you couldn't find rather than guessing. Don't describe what you ran.\n\n")
	b.WriteString(Naming)
	return b.String()
}

// ThreadNext is a later turn: what the user has said since the agent's
// last reply, and whether the document has changed (docNow is its version
// now, "" if it can't be read).
func ThreadNext(c *conversation.Conversation, docNow string) string {
	var said []string
	since := ""
	for _, t := range slices.Backward(c.Turns) {

		if t.By != conversation.You {
			since = t.Version
			break
		}
		said = append([]string{t.Body}, said...)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "The user replied in %s:\n\n", c.ID)
	for _, s := range said {
		b.WriteString(s + "\n\n")
	}
	for _, v := range slices.Backward(c.Turns) {
		if v.By != conversation.You && v.By != conversation.Agent {
			fmt.Fprintf(&b, "(%s also said in it: %s)\n\n", v.By, v.Body)
			break
		}
	}
	if docNow != "" && since != "" && docNow != since {
		fmt.Fprintf(&b, "The document has changed since your last reply: `ws show -f llm %s` has its history.\n\n", c.Anchor.Doc)
	}
	b.WriteString("Reply as before: your final message is your reply in the thread. Suggest an edit with `ws thread suggest " + c.ID + "` if they've asked for one, or a suggestion of yours needs changing.\n")
	return b.String()
}

// Draft asks a resolved thread's agent what was decided in it: in
// its session, or (resumed false) a fresh agent reading the thread.
func Draft(m *mission.Mission, c *conversation.Conversation, resumed bool) string {
	var b strings.Builder
	if resumed {
		fmt.Fprintf(&b, "The user has resolved %s. ", c.ID)
	} else {
		fmt.Fprintf(&b, "You're reading a resolved comment thread on a document, in a multi-repo workspace managed by the ws tool, for the mission %q (%s; WS_MISSION is set). Start with `ws show -f llm %s`: the text it's on, and what was said in it. You may only read.\n\n", m.Title, m.Slug, c.ID)
	}
	// What became of its edits isn't said in the thread: say it here.
	for _, sg := range c.Suggestions {
		switch sg.State {
		case conversation.SuggestAccepted:
			fmt.Fprintf(&b, "The user accepted the edit to read %q. ", clipString(sg.To, 200))
		case conversation.SuggestDeclined:
			fmt.Fprintf(&b, "The user declined the edit to read %q. ", clipString(sg.To, 200))
		case conversation.SuggestOpen:
			fmt.Fprintf(&b, "The edit to read %q was left neither accepted nor declined. ", clipString(sg.To, 200))
		}
	}
	b.WriteString("If the thread settled something worth remembering about the mission (a choice made, an option ruled out, a fact the document now rests on), draft it as a decision for the user to record: what the user said and did, not what was only suggested. Reply with exactly these two lines, and nothing else:\n\nWhat: <the decision, in one sentence, as an instruction or a fact (not where it's written)>\nWhy: <the reason, in a sentence or two>\n\nIf nothing was decided (a question answered from what's there already, a wording fixed, an edit declined), reply with just: Nothing decided.\n")
	return b.String()
}

// ParseDraft reads a drafted decision: "What: …" and "Why: …" lines (the
// why may run on). None, nothing was decided.
func ParseDraft(reply string) (what, why string, ok bool) {
	field := ""
	for l := range strings.SplitSeq(reply, "\n") {
		l = strings.TrimSpace(strings.ReplaceAll(l, "**", ""))
		lower := strings.ToLower(l)
		switch {
		case strings.HasPrefix(lower, "what:"):
			what, field = strings.TrimSpace(l[5:]), "what"
		case strings.HasPrefix(lower, "why:"):
			why, field = strings.TrimSpace(l[4:]), "why"
		case l == "":
			field = ""
		case field == "why":
			why += " " + l
		}
	}
	return what, why, what != ""
}
