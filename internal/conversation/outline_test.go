package conversation

import (
	"strings"
	"testing"
	"time"
)

// A plan is a reply that starts with the outline line, then its blocks:
// where each goes, its heading and a sentence on what it'll say.
func TestParseOutline(t *testing.T) {
	reply := strings.Join([]string{
		"<!-- outline -->",
		"Two sections: what it depends on, and how it rolls out.",
		"",
		"=== block",
		"after: Constraints",
		"heading: ## Dependencies",
		"summary: Which services this needs,",
		"  and what each must provide.",
		"",
		"=== block",
		"After: `Release`",
		"Heading: Roll out",
		"Level: 3",
		"Summary: The order it goes out in.",
		"",
		"=== block",
		"after: end",
		"summary: A closing list of what's still open.",
		"",
		"=== block",
		"",
	}, "\n")
	lead, blocks, ok := ParseOutline(reply)
	if !ok || lead != "Two sections: what it depends on, and how it rolls out." || len(blocks) != 3 {
		t.Fatalf("lead %q, blocks %+v, ok %v", lead, blocks, ok)
	}
	if b := blocks[0]; b.Key != "b1" || b.After != "Constraints" || b.Heading != "Dependencies" || b.Summary != "Which services this needs, and what each must provide." || b.Level != 0 {
		t.Errorf("the first: %+v", b)
	}
	if b := blocks[1]; b.Key != "b2" || b.After != "Release" || b.Heading != "Roll out" || b.Level != 3 {
		t.Errorf("the second: %+v", b)
	}
	if b := blocks[2]; b.After != "end" || b.Heading != "" || b.Summary == "" {
		t.Errorf("the third: %+v", b)
	}
	for _, not := range []string{"| Route |\n|---|\n", "Here's the plan.\n<!-- outline -->\n=== block\nsummary: x\n", "<!-- outline -->\nNothing to add.\n"} {
		if _, _, ok := ParseOutline(not); ok {
			t.Errorf("read as a plan: %q", not)
		}
	}
}

// An outline's agent goes on to each block in turn; each block is a cell
// of its own to the page; and what you say of one is answered before the
// next is written.
func TestOutlineBlocks(t *testing.T) {
	s := ForWorkspace(owned(t))
	c, _ := s.OpenCell("p1", "add a section on dependencies and one on roll out", GoalPlan, "", "", time.Now())
	_, blocks, _ := ParseOutline("<!-- outline -->\n=== block\nafter: Constraints\nheading: Dependencies\nsummary: What it needs.\n=== block\nafter: Release\nheading: Roll out\nsummary: How it goes out.\n")
	c, _ = s.Say(c.ID, Turn{By: Agent, At: time.Now(), Body: "Two sections.", Blocks: blocks, Answer: true})
	if !c.NeedsAnswer() || c.Working() != "b1" || c.NextBlock().Key != "b1" || c.YourTurn() || c.Written() != nil {
		t.Fatalf("planned: needs %v, working %q", c.NeedsAnswer(), c.Working())
	}
	views := c.Views()
	if len(views) != 2 || views[0].ID != c.ID+"-b1" || views[0].Kind != Cell || len(views[0].Turns) != 1 || views[0].Turns[0].Body != "What it needs." || views[0].Written() != nil {
		t.Fatalf("its blocks as cells: %+v", views)
	}
	if outline, key := SplitBlockID(views[1].ID); outline != c.ID || key != "b2" {
		t.Errorf("a block's id apart: %q %q", outline, key)
	}

	// The first is written; it goes on to the second.
	c, _ = s.Say(c.ID, Turn{By: Agent, At: time.Now(), Body: "It needs the token service.", Block: "b1", Notes: "Unchecked."})
	if c.Working() != "b2" || !c.NeedsAnswer() {
		t.Errorf("after the first: working %q", c.Working())
	}
	v1 := c.View(blocks[0])
	if w := v1.Written(); w == nil || w.Body != "It needs the token service." || w.Notes != "Unchecked." || len(v1.Turns) != 2 || v1.Turns[1].N != 2 {
		t.Errorf("the first block, written: %+v", v1.Turns)
	}
	// You ask for the first to be changed: that's answered before the
	// second is written, and is of the first.
	c, _ = s.Say(c.ID, Turn{By: You, At: time.Now(), Body: "name the version too", Block: "b1"})
	if !c.YourTurn() || c.Working() != "b1" || c.Asking() {
		t.Errorf("asked of the first: your turn %v, working %q", c.YourTurn(), c.Working())
	}
	c, _ = s.Say(c.ID, Turn{By: Agent, At: time.Now(), Body: "It needs the token service, v2.", Block: "b1"})
	if c.Working() != "b2" || len(c.View(blocks[0]).Turns) != 4 || c.View(blocks[0]).Written().Body != "It needs the token service, v2." {
		t.Errorf("after the change: working %q, the first %+v", c.Working(), c.View(blocks[0]).Turns)
	}
	// One that couldn't be written is still to write; one left out isn't.
	c, _ = s.Say(c.ID, Turn{By: WS, At: time.Now(), Body: "The agent couldn't answer: exit status 1", Block: "b2"})
	if !c.NeedsAnswer() || c.Working() != "b2" || c.View(blocks[1]).Last().By != WS {
		t.Errorf("after a failure: needs %v, working %q", c.NeedsAnswer(), c.Working())
	}
	c, _ = s.Say(c.ID, Turn{By: Agent, At: time.Now(), Body: "Left out.", Block: "b2", Skipped: true, Answer: true})
	if c.NeedsAnswer() || c.NextBlock() != nil || len(c.View(blocks[1]).Turns) != 2 {
		t.Errorf("left out: needs %v, next %v", c.NeedsAnswer(), c.NextBlock())
	}

	// Placed once.
	if c.Placed != nil {
		t.Error("placed before the page said so")
	}
	s.MarkPlaced(c.ID, time.Now())
	if got, _ := s.Load(c.ID); got.Placed == nil || len(got.Plan()) != 2 {
		t.Errorf("after placing: %+v", got.Placed)
	}
}
