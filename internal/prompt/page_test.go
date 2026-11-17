package prompt

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/page"
)

// testHere is a workspace's page, as its agents are told it.
var testHere = Here{Root: "/ws", Page: "/ws/.ws/pages/p1.md"}

// Asked to change a cell the user has edited by hand, its agent is told
// how it now reads, in its session or starting again.
func TestCellPromptsEdited(t *testing.T) {
	h := testHere
	c := &conversation.Conversation{ID: "w3", Kind: conversation.Cell, Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{
		{N: 1, By: conversation.You, Body: "the routes"},
		{N: 2, By: conversation.Agent, Body: "- /tokens"},
		{N: 3, By: conversation.You, Body: "add their handlers"},
	}}
	if p := CellNext(h, c); strings.Contains(p, "edited the cell by hand") || !strings.Contains(p, "add their handlers") {
		t.Errorf("not edited:\n%s", p)
	}
	// A cell made around the user's own words starts from them.
	own := &conversation.Conversation{ID: "w4", Kind: conversation.Cell, Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{
		{N: 1, By: conversation.You, Body: "tighten this", Edited: "We want, more or less, a cache."},
	}}
	if p := CellFirst(h, own); !strings.Contains(p, "words the user wrote themselves") || !strings.Contains(p, "It reads:\n\nWe want, more or less, a cache.\n") || !strings.Contains(p, "tighten this") {
		t.Errorf("made around their words:\n%s", p)
	}
	c.Turns[2].Edited = "- /tokens (cached)"
	c.Turns = append(c.Turns, conversation.Turn{N: 4, By: conversation.WS, Body: "The agent couldn't answer"})
	for name, p := range map[string]string{"next": CellNext(h, c), "first": CellFirst(h, c)} {
		if !strings.Contains(p, "It now reads:\n\n- /tokens (cached)\n") || !strings.Contains(p, "add their handlers") {
			t.Errorf("%s, edited:\n%s", name, p)
		}
	}
}

// A cell's agent replies with the cell and, apart from it, its notes to
// the user. It's told to write the document, not an account of its work,
// and that what it couldn't find goes in the notes.
func TestCellProseAndNotes(t *testing.T) {
	cell, notes := SplitNotes("| Route |\n|---|\n| /tokens |\n\n  <!-- Notes -->  \nI didn't look at the upstream model.\n")
	if cell != "| Route |\n|---|\n| /tokens |" || notes != "I didn't look at the upstream model." {
		t.Errorf("cell %q, notes %q", cell, notes)
	}
	if cell, notes = SplitNotes("Just the cell, with <!-- notes --> in a line of words.\n"); cell != "Just the cell, with <!-- notes --> in a line of words." || notes != "" {
		t.Errorf("no notes: cell %q, notes %q", cell, notes)
	}
	if cell, notes = SplitNotes("<!-- notes -->\nNothing in the repo says.\n"); cell != "" || notes != "Nothing in the repo says." {
		t.Errorf("only notes: cell %q, notes %q", cell, notes)
	}

	h := testHere
	c := &conversation.Conversation{ID: "w3", Kind: conversation.Cell, Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{{N: 1, By: conversation.You, Body: "the routes"}}}
	first := CellFirst(h, c)
	for _, want := range []string{"a line that is only `<!-- notes -->`", "Write the document, not an account of your work", "No \"I\", \"we\" or \"you\"", "no \"(?)\"", "goes in your notes", "own bookkeeping"} {
		if !strings.Contains(first, want) {
			t.Errorf("the first prompt lacks %q", want)
		}
	}
	if strings.Contains(first, "say so in a short line in the cell") {
		t.Error("the first prompt still asks for what wasn't found to be said in the cell")
	}
	c.Turns = append(c.Turns, conversation.Turn{N: 2, By: conversation.Agent, Body: "- /tokens"}, conversation.Turn{N: 3, By: conversation.You, Body: "add handlers"})
	if next := CellNext(h, c); !strings.Contains(next, "`<!-- notes -->`") || !strings.Contains(next, "the document's voice") {
		t.Errorf("a later prompt:\n%s", next)
	}
	check, _ := page.PassNamed(page.Passes(nil), "check")
	if form := passForm(check); !strings.Contains(form, "Words you propose go on the page as they are") {
		t.Errorf("a pass's form:\n%s", form)
	}
}

// A question asked in a page (>?) is answered to the user: its agent is
// told nothing it says goes into the page. Asked then to write, it's told
// the form and how it should read for the first time; and a question in a
// cell that's been written leaves the cell as it is.
func TestAskPrompts(t *testing.T) {
	h := testHere
	c := &conversation.Conversation{ID: "w8", Kind: conversation.Cell, Goal: conversation.GoalAsk, Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{
		{N: 1, By: conversation.You, Body: "why is there no HTTP server?", Ask: true},
	}}
	if !c.Asking() || c.Written() != nil {
		t.Fatal("a question is asking, and nothing's written")
	}
	first := CellFirst(h, c)
	// It answers; and it's told how, asked in words to put something in the
	// page, to have that written in: by saying its reply is for the page.
	// What's first asked is answered: it isn't told how to write into the
	// page until it's asked something more.
	if !strings.Contains(first, "nothing you say is written into the page") || !strings.Contains(first, "why is there no HTTP server?") || strings.Contains(first, "<!-- page -->") || strings.Contains(first, "Write the document, not an account") {
		t.Errorf("a question's first prompt:\n%s", first)
	}
	if rest, ok := CutPage("  <!-- Page -->\n| a |\n|---|\n\n<!-- notes -->\nUnchecked.\n"); !ok || !strings.HasPrefix(rest, "| a |") {
		t.Errorf("a reply for the page: %q %v", rest, ok)
	}
	if rest, ok := CutPage("No. It prints hello.\n\n<!-- page -->\n"); ok || rest != "No. It prints hello.\n\n<!-- page -->\n" {
		t.Errorf("an answer that mentions it: %q %v", rest, ok)
	}
	// Asked of a pass's finding, it has no place in the page to be told of.
	about := &conversation.Conversation{ID: "w9", Kind: conversation.Cell, Goal: conversation.GoalAsk, Anchor: conversation.Anchor{Page: "p1", Finding: "check:abc"}, Turns: []conversation.Turn{
		{N: 1, By: conversation.You, Body: "\"It should be fast\": what counts as fast?", Ask: true},
	}}
	if p := CellFirst(h, about); !strings.Contains(p, "something a pass over the page raised") || strings.Contains(p, "That place is marked") || !strings.Contains(p, "what counts as fast?") {
		t.Errorf("a question about a finding:\n%s", p)
	}
	c.Turns = append(c.Turns, conversation.Turn{N: 2, By: conversation.Agent, Body: "Because it only prints.", Answer: true})
	if c.Written() != nil {
		t.Error("an answer isn't something written for the page")
	}
	// A follow-up question.
	c.Turns = append(c.Turns, conversation.Turn{N: 3, By: conversation.You, Body: "and tests?", Ask: true})
	// Asked more, it may be asked for the page in words: it's told how, and
	// how what's written there should read.
	next0 := CellNext(h, c)
	for _, want := range []string{"and tests?", "If it's a question, your final message is your answer", "first line of your reply only `<!-- page -->`", "You never edit the page's file", "only their asking for it puts anything in the page", "Write the document, not an account of your work", "`<!-- notes -->`"} {
		if !strings.Contains(next0, want) {
			t.Errorf("a follow-up, which may ask or ask for the page, lacks %q:\n%s", want, next0)
		}
	}
	// Then: put it in the page.
	c.Turns = append(c.Turns, conversation.Turn{N: 4, By: conversation.Agent, Body: "One test.", Answer: true}, conversation.Turn{N: 5, By: conversation.You, Body: "write that up here as two sentences"})
	if c.Asking() {
		t.Error("asked to write, it isn't asking")
	}
	next := CellNext(h, c)
	for _, want := range []string{"now wants something written into the page", "write that up here as two sentences", "`<!-- notes -->`", "Write the document, not an account of your work"} {
		if !strings.Contains(next, want) {
			t.Errorf("asked to write, the prompt lacks %q:\n%s", want, next)
		}
	}
	// Written; then a question about it leaves it alone, and a change is a change.
	c.Turns = append(c.Turns, conversation.Turn{N: 6, By: conversation.Agent, Body: "demo has one test."}, conversation.Turn{N: 7, By: conversation.You, Body: "which file?", Ask: true})
	if next = CellNext(h, c); !c.Asking() || !strings.Contains(next, "which file?") || !strings.Contains(next, "in place of anything that place already holds") {
		t.Errorf("a question about a written cell:\n%s", next)
	}
	if w := c.Written(); w == nil || w.Body != "demo has one test." {
		t.Errorf("what's written: %+v", w)
	}
	c.Turns = append(c.Turns, conversation.Turn{N: 8, By: conversation.Agent, Body: "main_test.go", Answer: true}, conversation.Turn{N: 9, By: conversation.You, Body: "name the file"})
	if next = CellNext(h, c); !strings.Contains(next, "asks for the cell to be changed") || !strings.Contains(next, "Reply as before") {
		t.Errorf("a change after questions:\n%s", next)
	}
}

// An outline's one agent: asked of the page as a whole it's told to plan;
// a cell asked for something may plan, but not one made around the user's
// own words; then it's asked for each block in turn, in its session or (a
// fresh agent) told what was planned; and what's said of a block is said
// of that block.
func TestOutlinePrompts(t *testing.T) {
	h := testHere
	asked := &conversation.Conversation{ID: "w12", Kind: conversation.Cell, Goal: conversation.GoalPlan, Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{
		{N: 1, By: conversation.You, Body: "add a section after Constraints on dependencies, and one after Release on roll out"},
	}}
	first := CellFirst(h, asked)
	for _, want := range []string{"Plan it first, even if it's one block", "first line of your reply only `<!-- outline -->`", "=== block", "the words of a heading already in the page", "write none of them now", "Be quick about the plan", "Write the document, not an account of your work", "add a section after Constraints"} {
		if !strings.Contains(first, want) {
			t.Errorf("asked of the page, the prompt lacks %q", want)
		}
	}
	if strings.Contains(first, "`here` for where they asked") {
		t.Error("asked of the whole page, there's no here")
	}

	cell := &conversation.Conversation{ID: "w13", Kind: conversation.Cell, Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{{N: 1, By: conversation.You, Body: "three sections on the cache"}}}
	if p := CellFirst(h, cell); !strings.Contains(p, "more than one block of the page") || !strings.Contains(p, "`here` for where they asked") || !strings.Contains(p, "don't plan: write it") {
		t.Errorf("a cell may plan:\n%s", p)
	}
	cell.Turns[0].Edited = "My own words."
	if p := CellFirst(h, cell); strings.Contains(p, "<!-- outline -->") {
		t.Error("a cell made around the user's words is offered a plan")
	}

	_, blocks, _ := conversation.ParseOutline("<!-- outline -->\n=== block\nafter: Constraints\nheading: Dependencies\nsummary: What it needs.\n=== block\nafter: Release\nsummary: How it goes out.\n")
	asked.Turns = append(asked.Turns, conversation.Turn{N: 2, By: conversation.Agent, Body: "Two.", Blocks: blocks, Answer: true})
	if p := Block(asked, blocks[0]); !strings.Contains(p, "block 1 of 2, under the heading \"Dependencies\"") || !strings.Contains(p, "It's to say: What it needs.") || !strings.Contains(p, "without its heading") || !strings.Contains(p, "`<!-- notes -->`") {
		t.Errorf("asked for the first block:\n%s", p)
	}
	if p := Block(asked, blocks[1]); !strings.Contains(p, "block 2 of 2, which has no heading of its own") {
		t.Errorf("asked for one with no heading:\n%s", p)
	}
	asked.Turns = append(asked.Turns, conversation.Turn{N: 3, By: conversation.Agent, Body: "The token service.", Block: "b1"})
	fresh := BlockFirst(h, asked, blocks[1])
	for _, want := range []string{"- Dependencies (written): What it needs.", "- (no heading) (to write): How it goes out.", "`<!-- ws:cell w12-<its key> … -->`", "add a section after Constraints", "Write block 2 of 2", "Write the document, not an account of your work"} {
		if !strings.Contains(fresh, want) {
			t.Errorf("a fresh agent, asked for the second, lacks %q:\n%s", want, fresh)
		}
	}
	// Said of the first block: a change to that block.
	asked.Turns = append(asked.Turns, conversation.Turn{N: 4, By: conversation.You, Body: "name the version", Block: "b1"})
	if p := CellNext(h, asked); !strings.Contains(p, "This is about one block of what you planned: block 1 of 2") || !strings.Contains(p, "asks for the cell to be changed") || !strings.Contains(p, "name the version") {
		t.Errorf("said of a block:\n%s", p)
	}
}
