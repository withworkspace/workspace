package prompt

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/page"
)

// goldenHere is where the golden prompts' agents are: a workspace with a
// guide, two repos, two kinds of document, a mission, a page and its
// style.
var goldenHere = Here{
	Root:   "/ws",
	Agents: "This workspace has a guide for agents: read /ws/AGENTS.md before you start, and follow it.",
	Kind:   "It's a proposal: read /ws/kinds/proposal.md first.\n\n",
	Repos: []string{
		`api: main at 1a2b3c4 ("Cache tokens"), read-only at /ws/repos/api/.ground`,
		`web: main at 5d6e7f8 ("Login page"), read-only at /ws/repos/web/.ground`,
	},
	Git:     []string{"log", "show", "diff"},
	Kinds:   []string{"doc", "plan"},
	Mission: &mission.Mission{Slug: "cache", Title: "Cache lookups", Conversation: "w2"},
	Page:    "/ws/.ws/pages/p1.md",
	Style:   "/ws/guides/plain.md",
}

func you(n int, body string) conversation.Turn {
	return conversation.Turn{N: n, By: conversation.You, Body: body}
}

func agent(n int, body string) conversation.Turn {
	return conversation.Turn{N: n, By: conversation.Agent, Body: body}
}

// Each prompt a conversation's agent is given, pinned: a change to what
// an agent's told shows in a diff (go test ./internal/prompt -update).
func TestAnswer_Golden(t *testing.T) {
	h := goldenHere
	ws := h
	ws.Mission = nil

	// Threads.
	thread := &conversation.Conversation{ID: "t3", Kind: conversation.Change, Anchor: conversation.Anchor{Doc: "s1/proposal"},
		Turns: []conversation.Turn{you(1, "make this shorter")}}
	golden(t, "thread-first", ThreadFirst(h, thread))
	thread.Turns = append(thread.Turns, conversation.Turn{N: 2, By: conversation.Agent, Body: "Suggested.", Version: "v1"}, you(3, "shorter still"))
	golden(t, "thread-next", ThreadNext(thread, "v2"))
	thread.Suggestions = []conversation.Suggestion{{N: 1, To: "Cache it.", State: conversation.SuggestAccepted}}
	golden(t, "draft", Draft(h.Mission, thread, false))
	golden(t, "draft-resumed", Draft(h.Mission, thread, true))

	// Talk.
	talk := &conversation.Conversation{ID: "w4", Kind: conversation.Talk, Turns: []conversation.Turn{you(1, "how do we cache tokens?")}}
	golden(t, "talk-first", TalkFirst(ws, talk))
	golden(t, "talk-first-mission", TalkFirst(h, talk))
	// The cards the user settled since are turns of theirs that ws wrote,
	// the agent told of each as it's told what they say.
	settled := func(n int, body string, quiet bool) conversation.Turn {
		return conversation.Turn{N: n, By: conversation.You, Body: body, Origin: conversation.ProposalOrigin(n - 2), Quiet: quiet}
	}
	talk.Turns = append(talk.Turns, agent(2, "In memory."),
		settled(3, "Accepted proposal 1, the decision “Cache in memory”: kept with w4 until it becomes a mission.", true),
		settled(4, "Accepted proposal 2, the document “Token cache”: added as records:docs/token-cache.md.", true),
		settled(5, "Declined proposal 3, the document “Redis”.", false),
		settled(6, "Accepted proposal 4, the artifact “Horizon glow”: added as a1.v1.", true),
		settled(7, "Accepted proposal 5, a version of a1, on v1: added as a1.v2.", true),
		you(8, "and Redis?"))
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	talk.Turns[1].At, talk.Turns[7].At = at, at.Add(time.Hour)
	golden(t, "talk-next", TalkNext(ws, talk))
	// A mission's: it's told what became of its replans, in the turns ws
	// wrote for them as it does for any card settled.
	mtalk := &conversation.Conversation{ID: "t4", Kind: conversation.Talk, Turns: []conversation.Turn{you(1, "can you replan?"), agent(2, "I've proposed two replans."),
		settled(3, "Accepted proposal 1, the replan “Add a step for the AppBar's dropdown”: the planner's asked, in t1, and revises the plan there or says why not.", true),
		settled(4, "Declined proposal 2, the replan “Drop the browser check”.", false),
		you(5, "thanks")}}
	mtalk.Turns[1].At, mtalk.Turns[4].At = at, at.Add(time.Hour)
	golden(t, "talk-next-mission", TalkNext(h, mtalk))
	// What was said while it answered last (before its reply, after what
	// it answered: a card settled), it's told first.
	missed := &conversation.Conversation{ID: "w4", Kind: conversation.Talk, Turns: []conversation.Turn{you(1, "how?"),
		{N: 2, By: conversation.You, Body: "Declined proposal 1, the decision “Redis”.", Origin: conversation.ProposalOrigin(1)},
		{N: 3, By: conversation.Agent, Body: "In memory.", Seen: 1}, you(4, "and Redis?")}}
	if p := TalkNext(ws, missed); !strings.Contains(p, "The user says:\n\nDeclined proposal 1, the decision “Redis”.\n\nand Redis?\n\n") {
		t.Errorf("what it missed isn't told first:\n%s", p)
	}
	// A workspace with no kinds of document: its talks aren't told they
	// can propose one.
	plain := ws
	plain.Kinds = nil
	if p := TalkFirst(plain, talk); strings.Contains(p, "ws doc add") {
		t.Errorf("told it can propose a document, with no kinds:\n%s", p)
	}
	briefing := &conversation.Conversation{ID: "t1", Kind: conversation.Talk, Goal: conversation.GoalBrief, Turns: []conversation.Turn{you(1, "fe issue 45")}}
	golden(t, "briefing-first", TalkFirst(h, briefing))
	// A mission confirmed with its brief: its planner starts from it, and
	// proposes it with the plan.
	briefed := h
	briefed.Brief = "## Goal\nCache tokens per tenant.\n"
	if p := TalkFirst(briefed, briefing); !strings.Contains(p, "The user confirmed it with its brief, written there") || !strings.Contains(p, "Cache tokens per tenant.") || !strings.Contains(p, "propose the plan, as step 4 says, and it comes with this brief as it is") {
		t.Errorf("a briefing with its brief written:\n%s", p)
	}
	pr := &conversation.Conversation{ID: "w5", Kind: conversation.Talk, Turns: []conversation.Turn{you(1, "why this lock?")},
		Anchor: conversation.Anchor{PR: "api#89", Path: "cache.go", Line: 12, EndLine: 13, Commit: "abcdef123456", Excerpt: []string{"mu.Lock()", "defer mu.Unlock()"}}}
	golden(t, "pr-first", TalkFirst(ws, pr))
	// An ask about a pull request, from its review's bar: it proposes
	// review comments while it isn't kept, and the rest once it is.
	prAsk := &conversation.Conversation{ID: "w10", Kind: conversation.Talk, What: conversation.WhatPR, Ephemeral: true, Turns: []conversation.Turn{you(1, "why this lock?")}, Anchor: pr.Anchor}
	golden(t, "pr-ask-first", TalkFirst(ws, prAsk))
	prKept := at.Add(2 * time.Hour)
	prAsk.Ephemeral, prAsk.Kept = false, &prKept
	prAsk.Turns = append(prAsk.Turns, agent(2, "It guards the map."), you(3, "should we raise it?"))
	prAsk.Turns[1].At, prAsk.Turns[2].At = at.Add(time.Hour), prKept.Add(time.Minute)
	golden(t, "pr-ask-kept-next", TalkNext(ws, prAsk))
	// An ask about a document, and a passage of it.
	doc := ws
	doc.Doc = "/ws/repos/records/.ground/notes/tokens.md"
	asked := &conversation.Conversation{ID: "w9", Kind: conversation.Talk, What: conversation.WhatDoc, Ephemeral: true, Turns: []conversation.Turn{you(1, "why per user?")},
		Anchor: conversation.Anchor{Doc: "records:notes/tokens.md", Path: "notes/tokens.md", Quote: &conversation.Quote{Exact: "We cache tokens per user."}, Section: "Caching", Line: 5, EndLine: 6}}
	golden(t, "doc-ask-first", TalkFirst(doc, asked))
	// Kept, it can propose: told so in the turn after, or from the start
	// of a session that's begun again.
	kept := at.Add(2 * time.Hour)
	asked.Ephemeral, asked.Kept = false, &kept
	asked.Turns = append(asked.Turns, agent(2, "Users never share tokens."), you(3, "should we record that?"))
	asked.Turns[1].At, asked.Turns[2].At = at.Add(time.Hour), kept.Add(time.Minute)
	golden(t, "doc-ask-kept-next", TalkNext(doc, asked))
	golden(t, "doc-ask-kept-first", TalkFirst(doc, asked))

	// A page's cells, questions and outlines.
	cell := &conversation.Conversation{ID: "w6", Kind: conversation.Cell, Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{you(1, "a table of the routes")}}
	golden(t, "cell-first", CellFirst(h, cell))
	cell.Turns = append(cell.Turns, agent(2, "| Route |"), you(3, "add their handlers"))
	golden(t, "cell-next", CellNext(h, cell))
	ask := &conversation.Conversation{ID: "w7", Kind: conversation.Cell, Goal: conversation.GoalAsk, Anchor: conversation.Anchor{Page: "p1"},
		Turns: []conversation.Turn{{N: 1, By: conversation.You, Body: "why no HTTP server?", Ask: true}}}
	golden(t, "ask-first", CellFirst(h, ask))
	ask.Turns = append(ask.Turns, conversation.Turn{N: 2, By: conversation.Agent, Body: "It only prints.", Answer: true}, conversation.Turn{N: 3, By: conversation.You, Body: "and tests?", Ask: true})
	golden(t, "ask-next", CellNext(h, ask))
	outline := &conversation.Conversation{ID: "w8", Kind: conversation.Cell, Goal: conversation.GoalPlan, Anchor: conversation.Anchor{Page: "p1"},
		Turns: []conversation.Turn{you(1, "a section on dependencies, and one on roll out")}}
	golden(t, "outline-first", CellFirst(h, outline))
	_, blocks, _ := conversation.ParseOutline("<!-- outline -->\n=== block\nafter: Constraints\nheading: Dependencies\nsummary: What it needs.\n=== block\nafter: end\nsummary: How it goes out.\n")
	outline.Turns = append(outline.Turns, conversation.Turn{N: 2, By: conversation.Agent, Body: "Two.", Blocks: blocks, Answer: true}, conversation.Turn{N: 3, By: conversation.Agent, Body: "The token service.", Block: "b1"})
	golden(t, "block", Block(outline, blocks[1]))
	golden(t, "block-first", BlockFirst(h, outline, blocks[1]))

	// Passes.
	check, _ := page.PassNamed(page.Passes(nil), "check")
	cold, _ := page.PassNamed(page.Passes(nil), "cold")
	text := []byte("# Greet by name\n\nIt should be fast.\n")
	pass := &conversation.Conversation{ID: "w9", Kind: conversation.Pass, Goal: "check", Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{you(1, "Run it.")}}
	golden(t, "pass-first", PassFirst(h, pass, Pass{Def: check, Text: text}))
	golden(t, "pass-first-cold", PassFirst(h, pass, Pass{Def: cold, Text: text}))
	_, fs := conversation.ParseFindings("## question\n> It should be fast.\nHow fast?\n\n## missing\nWho reviews it.\n")
	pass.Turns = append(pass.Turns, conversation.Turn{N: 2, By: conversation.Agent, Body: "Not ready.", Findings: fs}, you(3, "Run it."))
	pass.Settled = map[string]string{fs[0].Key: conversation.FindingDone, fs[1].Key: conversation.FindingDismissed}
	pass.Dismissed = map[string]string{fs[1].Key: conversation.DismissMinor}
	golden(t, "pass-next", PassNext(h, pass, Pass{Def: check, Text: text}))
	wording := page.PassDef{Name: "wording", Title: "Wording", About: "How it reads.", Summary: "a line on how it reads", Kinds: []string{"tell"}, File: "guides/plain.md", Cold: true}
	golden(t, "pass-first-guide", PassFirst(h, pass, Pass{Def: wording, Text: text, GuidePath: "/ws/guides/plain.md", GuideText: "# Plain words\n\nNo triads.\n"}))
}

// Each form a prompt asks for is read by the parser beside it: a reply
// written as the prompt says comes apart as the loop expects.
func TestAnswer_FormsRead(t *testing.T) {
	h := goldenHere
	talk := &conversation.Conversation{ID: "w4", Kind: conversation.Talk, Turns: []conversation.Turn{you(1, "how?")}}
	if p := TalkFirst(h, talk); !strings.Contains(p, "\nTitle: <") || !strings.Contains(p, "\nSummary: <") {
		t.Fatalf("talk asks for no Title: and Summary: lines:\n%s", p)
	}
	if body, title, summary := SplitSummary("In memory.\n\nTitle: Token cache\nSummary: Where to cache tokens."); body != "In memory." || title != "Token cache" || summary != "Where to cache tokens." {
		t.Errorf("talk's ending: %q %q %q", body, title, summary)
	}

	thread := &conversation.Conversation{ID: "t3", Kind: conversation.Ask, Turns: []conversation.Turn{you(1, "why?")}}
	if p := Draft(h.Mission, thread, true); !strings.Contains(p, "\nWhat: <") || !strings.Contains(p, "\nWhy: <") || !strings.Contains(p, "Nothing decided.") {
		t.Fatalf("a draft asks for no What: and Why: lines:\n%s", p)
	}
	if what, why, ok := ParseDraft("What: Cache in memory.\nWhy: One process."); !ok || what != "Cache in memory." || why != "One process." {
		t.Errorf("a draft: %q %q %v", what, why, ok)
	}
	if _, _, ok := ParseDraft("Nothing decided."); ok {
		t.Error("nothing decided is a draft")
	}

	cell := &conversation.Conversation{ID: "w6", Kind: conversation.Cell, Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{you(1, "the routes")}}
	if p := CellFirst(h, cell); !strings.Contains(p, "a line that is only `<!-- notes -->`") {
		t.Fatalf("a cell asks for no notes line:\n%s", p)
	}
	if body, notes := SplitNotes("| Route |\n\n<!-- notes -->\nUnchecked."); body != "| Route |" || notes != "Unchecked." {
		t.Errorf("a cell's notes: %q %q", body, notes)
	}

	ask := &conversation.Conversation{ID: "w7", Kind: conversation.Cell, Goal: conversation.GoalAsk, Anchor: conversation.Anchor{Page: "p1"},
		Turns: []conversation.Turn{{N: 1, By: conversation.You, Body: "why?", Ask: true}, {N: 2, By: conversation.Agent, Body: "Because.", Answer: true}, {N: 3, By: conversation.You, Body: "write it up", Ask: true}}}
	if p := CellNext(h, ask); !strings.Contains(p, "first line of your reply only `<!-- page -->`") {
		t.Fatalf("a question asks for no page line:\n%s", p)
	}
	if rest, ok := CutPage("<!-- page -->\nIt only prints."); !ok || rest != "It only prints." {
		t.Errorf("a reply for the page: %q %v", rest, ok)
	}

	outline := &conversation.Conversation{ID: "w8", Kind: conversation.Cell, Goal: conversation.GoalPlan, Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{you(1, "two sections")}}
	if p := CellFirst(h, outline); !strings.Contains(p, "first line of your reply only `<!-- outline -->`") || !strings.Contains(p, "=== block\nafter: <") {
		t.Fatalf("an outline asks for no plan:\n%s", p)
	}
	if lead, blocks, ok := conversation.ParseOutline("<!-- outline -->\nTwo sections.\n\n=== block\nafter: end\nheading: Roll out\nsummary: How it goes out.\n"); !ok || lead != "Two sections." || len(blocks) != 1 || blocks[0].Heading != "Roll out" {
		t.Errorf("a plan: %q %+v %v", lead, blocks, ok)
	}

	check, _ := page.PassNamed(page.Passes(nil), "check")
	pass := &conversation.Conversation{ID: "w9", Kind: conversation.Pass, Goal: "check", Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{you(1, "Run it.")}}
	if p := PassFirst(h, pass, Pass{Def: check}); !strings.Contains(p, "## <its kind: ") || !strings.Contains(p, "```suggest") {
		t.Fatalf("a pass asks for no findings:\n%s", p)
	}
	body, fs := conversation.ParseFindings("Not ready.\n\n## question\n> It should be fast.\nHow fast?\n\n```suggest\nIt answers within 100 ms.\n```\n\n## made-up\nWho reviews it.\n")
	fs = PassKinds(check, fs)
	if body != "Not ready." || len(fs) != 2 || fs[0].Quote != "It should be fast." || fs[0].Suggest == "" || fs[1].Kind != check.Kinds[0] {
		t.Errorf("a pass's findings: %q %+v", body, fs)
	}
}

// A kept ask's agent is told it may propose in the first turn it's
// prompted for after the keep, and not again: kept while an answer runs,
// that's the turn after the answer, whose prompt said to propose nothing.
func TestTalkNext_KeptAsk(t *testing.T) {
	t0 := time.Date(2026, 11, 12, 20, 0, 0, 0, time.UTC)
	turn := func(n int, by string, at time.Duration) conversation.Turn {
		return conversation.Turn{N: n, By: by, At: t0.Add(at), Body: "x"}
	}
	told := func(kept time.Duration, turns ...conversation.Turn) bool {
		k := t0.Add(kept)
		c := &conversation.Conversation{ID: "w9", Kind: conversation.Talk, What: conversation.WhatDoc, Kept: &k, Turns: turns}
		return strings.Contains(TalkNext(goldenHere, c), "The user kept this conversation")
	}
	for _, c := range []struct {
		name  string
		kept  time.Duration
		turns []conversation.Turn
		want  bool
	}{
		{"kept after the answer", 40 * time.Second, []conversation.Turn{turn(1, "you", 0), turn(2, "agent", 30*time.Second), turn(3, "you", time.Minute)}, true},
		{"kept while the answer ran", 10 * time.Second, []conversation.Turn{turn(1, "you", 0), turn(2, "agent", 30*time.Second), turn(3, "you", time.Minute)}, true},
		{"told already", 10 * time.Second, []conversation.Turn{turn(1, "you", 0), turn(2, "agent", 30*time.Second), turn(3, "you", time.Minute), turn(4, "agent", 90*time.Second), turn(5, "you", 2*time.Minute)}, false},
	} {
		if got := told(c.kept, c.turns...); got != c.want {
			t.Errorf("%s: told %v, want %v", c.name, got, c.want)
		}
	}
}
