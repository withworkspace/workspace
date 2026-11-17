package prompt

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/page"
)

// What the agents of a page are told: a cell's, a question's asked in the
// page (>?), an outline's and its blocks', and a pass's; how what they
// write into a page should read; and what reads their replies (CutPage,
// SplitNotes, PassKinds, and conversation.ParseOutline and ParseFindings).

// How what agents write into a page should read, and where what they have
// to say to you goes instead.
//
// A page is a document: it's read later, by other people, with no agent
// there. What an agent writes into it (a cell, a draft, your own words
// written again, a pass's proposed words) has to read as the document
// does. What the agent has to say about its own work (what it couldn't
// find, didn't check, had to guess) is said to you, beside the cell, and
// isn't part of the page.
//
// The rules about plain statement take from the humanizer skill
// (github.com/blader/humanizer, MIT), which lists what makes prose read as
// a model's: said here in ws's own words, and far shorter.

// pageProse is the style every agent writing into a page is given.
const pageProse = `How it should read. What you write becomes part of a document that other people will read later, without you there and without knowing an agent wrote any of it.

- Write the document, not an account of your work. No "I", "we" or "you". Nothing about what you searched for, ran, read or didn't look at: not "a grep for X found nothing", not "I found no rate limiting". Say what is so: "Nothing under service/ registers a rate limiter, so the 429 it declares is never produced."
- Where something is so, name the file, with its lines when they matter, in the sentence that needs it. That's the evidence. The search isn't.
- Leave out this workspace's own bookkeeping. Missions, pages, cells, capsules and ws commands aren't the subject, unless the page is about them: "no missions exist yet" tells the reader nothing about what the page is for.
- Only what you found to be so goes in the text. Don't write a guess as a fact, and don't flag one in the text either: no "(?)", "probably", "it seems", "as far as I can tell". What you couldn't find, didn't check or had to guess goes in your notes (below), not in the page. Something about the subject that nobody has decided can be said as that, in the page's own terms: "Not decided: whether the limit is per user or per tenant."
- State each thing plainly, once. No sentence built as a contrast to make a point ("not just X, but Y"), no closing line that sums up or sounds wise, no opening that announces what's coming, no lists of three for rhythm, no dashes joining clauses. Use "is", "has" and "does" where they're true.
- Use ordinary words. Not pivotal, crucial, robust, seamless, comprehensive, leverage, streamline, delve, landscape, unlock, empower.
- No formatting for its own sake: no bold label opening each bullet, no heading unless one was asked for, sentence case in any heading.
- Fit the page: its words for things, its tense, how much it says. After a sentence that ends in a colon, write what that sentence promises.
`

// cellForm says what a cell's agent replies with: the cell, then, apart
// from it, its notes to the user.
const cellForm = "Your final message is the cell: " + cellIs

// cellIs is what a cell is, as a reply: the rest of cellForm, and of what
// a question's agent is told about writing one.
const cellIs = "markdown, exactly as it should read in the page, the whole of it, with nothing before it (no \"Here's the table\"). Make it the size of what was asked: a table for a table, a few lines for a question. Take it from what's here (the code, the missions, GitHub), not from memory. Name things as you were told (a file as `api/internal/routes.go:12-20`), never by where they are on this disk: the page links what's named that way, and shows the lines. Don't write the cell's comments: ws does.\n\n" +
	"After the cell, if you've anything to tell the user about it, write a line that is only `<!-- notes -->`, and then your notes to them: what you couldn't find, what you didn't check, what you had to guess and how sure you are of it, anything they should look at themselves. Notes are in your own voice, and short. They're shown to the user beside the cell and are never put in the page, so nothing that belongs in the page goes there, and nothing of that kind goes in the cell. With nothing to tell them, leave the line and the notes out.\n\n"

// cellFormAgain is the same, for a turn in a session that's had it.
const cellFormAgain = "Reply as before: the whole cell as it should now read (not only what changed), then, if there's anything to tell them, a line that is only `<!-- notes -->` and your notes. How it should read is as you were told: the document's voice, and what's yours to say in the notes.\n"

// intoPage says how an agent that's answering has something written into
// the page instead, when that's what the user asks for in words ("draft
// this in the doc", "put that in as a table"). It doesn't edit the file:
// it says its reply is for the page, and ws writes it in.
const intoPage = "If what they now say asks, in so many words, for something to be put into the page (to write it up there, draft it in the doc, add it, make what's there a table), do that, not answer. A question is still a question, wherever in the page it was asked: only their asking for it puts anything in the page. You never edit the page's file, and you can't. Make the first line of your reply only `<!-- page -->`. What follows it is written into the page by ws, at the place they asked, as part of their document, in place of anything that place already holds. It is the cell: " + cellIs

var pageLine = regexp.MustCompile(`(?i)^\s*<!--[ \t]*page[ \t]*-->[ \t]*\r?(\n|$)`)

// CutPage says whether a reply is for the page (its first line is only
// <!-- page -->), and is the reply without that line.
func CutPage(reply string) (string, bool) {
	at := pageLine.FindStringIndex(reply)
	if at == nil {
		return reply, false
	}
	return reply[at[1]:], true
}

var notesLine = regexp.MustCompile(`(?mi)^[ \t]*<!--[ \t]*notes[ \t]*-->[ \t]*\r?$`)

// SplitNotes takes a cell agent's reply apart: the cell, and its notes to
// the user (after a line that's only <!-- notes -->), if it has any.
func SplitNotes(reply string) (cell, notes string) {
	at := notesLine.FindStringIndex(reply)
	if at == nil {
		return strings.TrimSpace(reply), ""
	}
	return strings.TrimSpace(reply[:at[0]]), strings.TrimSpace(reply[at[1]:])
}

// CellFirst starts a cell's agent: its role, the page the cell's in,
// what it can read, and what was asked for (or, picking up a cell whose
// session is gone, where to read it so far).
func CellFirst(h Here, c *conversation.Conversation) string {
	if c.Asking() {
		return askFirst(h, c)
	}
	if c.Goal == conversation.GoalPlan && c.Plan() == nil {
		return planFirst(h, c)
	}
	var b strings.Builder
	path := h.Page
	fmt.Fprintf(&b, "You're filling in a cell of a page the user is writing, in the web UI of ws, which manages this multi-repo workspace (%s). A cell is a place in their page where they asked for something (a table, a list, a summary, an answer) and carried on writing. What you reply is written into the page there, as part of their document.\n\n", h.Root)
	fmt.Fprintf(&b, "The page is %s, the markdown file %s. Read it first: what's around the cell says what it's for. Your cell is %s: in the file it's between the comments `<!-- ws:cell %s … -->` and `<!-- /ws:cell -->`, with what it was asked for in the first. The page's other cells are marked the same way.\n\n", c.Anchor.Page, path, c.ID, c.ID)
	b.WriteString(para(h.Agents))
	b.WriteString(h.Kind)
	b.WriteString(talkReading(h.Git))
	b.WriteString(repos(h))
	said := youSince(c)
	written := false
	for _, t := range c.Turns {
		written = written || t.By == conversation.Agent
	}
	if written {
		fmt.Fprintf(&b, "This cell has been filled before: `ws show -f llm %s` has what was asked for and what was written. The user now asks:\n\n", c.ID)
	} else {
		b.WriteString("The user asks:\n\n")
	}
	for _, s := range said {
		b.WriteString(s + "\n\n")
	}
	b.WriteString(cellEdited(c))
	b.WriteString(cellForm)
	b.WriteString(pageProse)
	b.WriteString(style(h.Style))
	// Asked for more than a block, it may plan (not for words of the user's
	// own it's been asked to change, or a block of a plan already made).
	if len(c.Turns) > 0 && c.Turns[0].Edited == "" && c.Plan() == nil && !strings.Contains(c.ID, "-") {
		b.WriteString(outlineOffer)
	}
	return b.String()
}

// style points a cell's agent at the workspace's guide to how pages
// should read ([pages] style), if it has one (path, which its caller has
// found is there): read once, at the start of its session.
func style(path string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("\nThis workspace says more about how its pages should read, in the file %s: read it before you write, and write the cell that way too. It may be written for another use (editing a text someone gives you, say): take from it how prose should read, not what to reply with. It doesn't change what the cell is to hold.\n", path)
}

// cellEdited says how a cell reads now, when the user has edited it by
// hand since its agent wrote it: that's what's changed, not what the agent
// last wrote.
func cellEdited(c *conversation.Conversation) string {
	i := len(c.Turns) - 1
	for i >= 0 && c.Turns[i].By == conversation.WS {
		i--
	}
	for ; i >= 0 && c.Turns[i].By == conversation.You; i-- {
		e := c.Turns[i].Edited
		switch {
		case e == "":
		case i == 0: // made around their own words
			return "The cell is made around words the user wrote themselves, which is what they want changed. It reads:\n\n" + e + "\n\nStart from that: it's theirs, so keep what they don't ask to be changed, in their voice.\n\n"
		default:
			return "They've edited the cell by hand since it was last written. It now reads:\n\n" + e + "\n\nStart from that: keep their edits, but for what they now ask to be changed.\n\n"
		}
	}
	return ""
}

// askAnswering says how a question asked in a page is answered: to the
// user, and not into the page.
const askAnswering = "Your final message is your answer, to them: it's shown beside the page, where they asked, and nothing you say is written into the page, wherever in it they asked. Answer the question first, then what backs it, briefly: a few sentences, or a short list. Take it from what's here (the code, the missions, GitHub), not from memory, and say what you couldn't find. Name things as you were told (a file as `api/internal/routes.go:12-20`), never by where they are on this disk. Don't describe what you ran. They may ask more, or ask later for something to be written into the page: you'll be told how if they do. For now, answer.\n"

// askFirst starts the agent of a question asked at a place in a
// page (>?): it answers the user, and nothing goes into the page.
func askFirst(h Here, c *conversation.Conversation) string {
	var b strings.Builder
	path := h.Page
	fmt.Fprintf(&b, "The user has asked a question at a place in a page they're writing, in the web UI of ws, which manages this multi-repo workspace (%s). It's a question to you, about what the page is about: the page stays as it is.\n\n", h.Root)
	if c.Anchor.Finding != "" {
		// Asked of something a pass found: it has no place in the page.
		fmt.Fprintf(&b, "The page is %s, the markdown file %s. Read it first. Their question is about something a pass over the page raised: it quotes the words of the page it's about. Comments like `<!-- ws:cell … -->` in the file mark where an agent wrote part of the page at their asking.\n\n", c.Anchor.Page, path)
	} else {
		fmt.Fprintf(&b, "The page is %s, the markdown file %s. Read it first: where they asked says what they're asking about. That place is marked by the comment `<!-- ws:cell %s … -->`, with their question in it. Other such comments mark where an agent wrote part of the page at their asking.\n\n", c.Anchor.Page, path, c.ID)
	}
	b.WriteString(para(h.Agents))
	b.WriteString(h.Kind)
	b.WriteString(talkReading(h.Git))
	b.WriteString(repos(h))
	said := youSince(c)
	if len(c.Turns) > len(said) {
		fmt.Fprintf(&b, "They've asked here before: `ws show -f llm %s` has what was asked and answered. They now ask:\n\n", c.ID)
	} else {
		b.WriteString("They ask:\n\n")
	}
	for _, s := range said {
		b.WriteString(s + "\n\n")
	}
	b.WriteString(askAnswering)
	return b.String()
}

// CellNext is a later turn, in the agent's session: a question to
// answer, or what the user wants written (the first time, after questions)
// or changed.
func CellNext(h Here, c *conversation.Conversation) string {
	// What's said of one block of an outline is said of that block.
	if blk, ok := c.BlockOf(c.Working()); ok && len(c.Plan()) > 0 {
		return aboutBlock(c, blk) + CellNext(h, c.View(blk))
	}
	var b strings.Builder
	if c.Asking() {
		b.WriteString("The user says, in what they've been asking you about:\n\n")
		for _, s := range youSince(c) {
			b.WriteString(s + "\n\n")
		}
		b.WriteString("If it's a question, your final message is your answer, shown to them beside the page, and nothing's written into it. (The shell works for the commands your first message listed: if one was refused, that was it alone.)\n\n")
		b.WriteString(intoPage)
		b.WriteString(pageProse)
		b.WriteString(style(h.Style))
		return b.String()
	}
	if c.Written() == nil {
		// Asked about, and now to be written: its session has only answered.
		b.WriteString("The user now wants something written into the page, at the place they asked:\n\n")
		for _, s := range youSince(c) {
			b.WriteString(s + "\n\n")
		}
		fmt.Fprintf(&b, "What you reply this time is written into the page there, as part of their document: it's a cell of the page from now on. Read the page again first (%s): the place is marked by the comment `<!-- ws:cell %s … -->`.\n\n", h.Page, c.ID)
		b.WriteString(cellForm)
		b.WriteString(pageProse)
		b.WriteString(style(h.Style))
		return b.String()
	}
	b.WriteString("The user asks for the cell to be changed:\n\n")
	for _, s := range youSince(c) {
		b.WriteString(s + "\n\n")
	}
	b.WriteString(cellEdited(c))
	fmt.Fprintf(&b, "The page may have moved on: read %s again if it matters. (The shell works for the commands your first message listed: if one was refused, that was it alone.)\n\n", h.Page)
	b.WriteString(cellFormAgain)
	return b.String()
}

// Outlines: asked for more than one block of a page, a cell's agent plans
// the blocks first (conversation.ParseOutline), the page puts them in
// empty, and then the same agent, in the same session, is asked for each
// in turn. One agent throughout: what it read to lay the page out, it
// still knows when it writes.

// outlineForm says how a plan is written. here is whether there's a place
// in the page the user asked from.
func outlineForm(here bool) string {
	where := "the words of a heading already in the page (the block goes at the end of that heading's section), or `start`, or `end`"
	if here {
		where = "`here` for where they asked, or " + where
	}
	return "Make the first line of your reply only `<!-- outline -->`. After it you may say, in a line, what you're going to add. Then each block, in the order they'll be in the page:\n\n" +
		"=== block\nafter: <where it goes: " + where + ">\nheading: <the heading it goes under, if it's a section of its own; leave the line out for a block that needs none>\nsummary: <one sentence on what the block will say: it's shown in the page until the block's written>\n\n" +
		"ws puts the headings and the empty blocks into the page at once, so the user sees what's coming and can go on writing around them. You're then asked to write each block, one at a time, in this same session: write none of them now. Be quick about the plan: the user is waiting to see what's coming. Lay it out from the page and what was asked, look further only if you can't lay it out without it, and leave the reading for when you write each. Twelve blocks at most, and none for what the page already has.\n"
}

// outlineOffer tells a cell's agent it may plan, when what's asked is more
// than a block.
var outlineOffer = "\nIf what's asked is more than one block of the page (several sections, or things to go in several places), don't write it in one reply: plan it. " + outlineForm(true) +
	"For one block (a table, a paragraph, a list, an answer), don't plan: write it, as above.\n"

// planFirst starts the agent of something asked of the page as a
// whole (not at a place in it): it plans the blocks, and then writes them.
func planFirst(h Here, c *conversation.Conversation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "The user has asked for something to be added to a page they're writing, in the web UI of ws, which manages this multi-repo workspace (%s). You plan what's to be added and where, and then write it, a block at a time. What you write becomes part of their document.\n\n", h.Root)
	fmt.Fprintf(&b, "The page is %s, the markdown file %s. Read it first. Where an agent wrote part of it at the user's asking, that part is between `<!-- ws:cell … -->` comments: it's the page's all the same.\n\n", c.Anchor.Page, h.Page)
	b.WriteString(para(h.Agents))
	b.WriteString(h.Kind)
	b.WriteString(talkReading(h.Git))
	b.WriteString(repos(h))
	b.WriteString("They ask:\n\n")
	for _, s := range youSince(c) {
		b.WriteString(s + "\n\n")
	}
	b.WriteString("Plan it first, even if it's one block. " + outlineForm(false))
	b.WriteString("\nWhen you're asked to write a block, it's written as a cell of the page is. " + cellForm + pageProse + style(h.Style))
	return b.String()
}

// blockWhich says which block, in words.
func blockWhich(c *conversation.Conversation, b conversation.Block) string {
	plan := c.Plan()
	n := 0
	for i := range plan {
		if plan[i].Key == b.Key {
			n = i + 1
		}
	}
	which := fmt.Sprintf("block %d of %d", n, len(plan))
	if b.Heading != "" {
		return fmt.Sprintf("%s, under the heading %q", which, b.Heading)
	}
	return which + ", which has no heading of its own"
}

// blockWriting says how a planned block is written.
const blockWriting = "Your final message is that block, written as a cell of the page is: its content only, without its heading (the heading's in the page already), with nothing before it. Then, if there's anything to tell the user about it, a line that is only `<!-- notes -->` and your notes. Look at what you need for it first. Don't say again what another block says, or will.\n"

// Block asks an outline's agent, in its session, for the next block.
func Block(c *conversation.Conversation, b conversation.Block) string {
	return fmt.Sprintf("The headings and the empty blocks are in the page now. Write %s. It's to say: %s\n\n%s", blockWhich(c, b), b.Summary, blockWriting)
}

// BlockFirst asks for the next block of an outline whose session is
// gone: a fresh agent is told what was planned, and what's written.
func BlockFirst(h Here, c *conversation.Conversation, b conversation.Block) string {
	var s strings.Builder
	fmt.Fprintf(&s, "You're writing part of a page the user is writing, in the web UI of ws, which manages this multi-repo workspace (%s). Earlier, at their asking, an outline was planned for it, and its headings and empty blocks put into the page. Each block is written one at a time. What you write becomes part of their document.\n\n", h.Root)
	fmt.Fprintf(&s, "The page is %s, the markdown file %s. Read it first. Each block of the outline is between the comments `<!-- ws:cell %s-<its key> … -->` and `<!-- /ws:cell -->`.\n\n", c.Anchor.Page, h.Page, c.ID)
	s.WriteString(para(h.Agents))
	s.WriteString(h.Kind)
	s.WriteString(talkReading(h.Git))
	s.WriteString(repos(h))
	if len(c.Turns) > 0 {
		fmt.Fprintf(&s, "What they asked for:\n\n%s\n\n", c.Turns[0].Body)
	}
	s.WriteString("The outline:\n")
	next := c.NextBlock()
	written := true
	for _, p := range c.Plan() {
		if next != nil && p.Key == next.Key {
			written = false
		}
		state := "to write"
		if written {
			state = "written"
		}
		fmt.Fprintf(&s, "- %s (%s): %s\n", firstNonEmpty(p.Heading, "(no heading)"), state, p.Summary)
	}
	fmt.Fprintf(&s, "\nWrite %s. It's to say: %s\n\n%s\n", blockWhich(c, b), b.Summary, blockWriting)
	s.WriteString(pageProse)
	s.WriteString(style(h.Style))
	return s.String()
}

// aboutBlock says, of a turn of the user's about one planned block, which
// it is.
func aboutBlock(c *conversation.Conversation, b conversation.Block) string {
	return fmt.Sprintf("This is about one block of what you planned: %s (it says: %s). ", blockWhich(c, b), b.Summary)
}

// passSkill is how a pass that's one of the workspace's skills is done: the
// skill, followed for what the pass asks of it, on this page; and its
// report given in ws's form in place of its own.
func passSkill(def page.PassDef) string {
	var b strings.Builder
	fmt.Fprintf(&b, "This pass uses the workspace's skill %q", def.Skill)
	if def.Args != "" {
		fmt.Fprintf(&b, ", asked: %q", def.Args)
	}
	fmt.Fprintf(&b, ". Its instructions are in %s: read all of it first, then the files it sends you to for this (a template, a check, a reference), and do what it says", def.File)
	if def.Args != "" {
		b.WriteString(" for that")
	}
	b.WriteString(", with this page as what it's run on. The page isn't a file in a repo yet: where the skill wants the document's repo, its template or ground truth, take them from what the page says of itself (its header, say), and from the workspace's repos.\n\n")
	b.WriteString("What the skill says about how to report is replaced by the form below, which is how the user reads it, beside the page: the verdict or overall word it would open its report with is your first line. Each item it would list is a finding, on the words of the page it's about (none, for something that isn't there), of the kind the skill would file it under. Where it would say what fixes it, and the fix is words on the page, propose them. Don't rewrite the page, and don't report the skill's steps.\n\n")
	return b.String()
}

// passForm says how a pass's agent replies: ws reads it into findings.
func passForm(def page.PassDef) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Reply in exactly this form, and nothing else. First, %s. Then each finding, the most important first, twelve at most, as:\n\n", def.Summary)
	kinds := strings.Join(def.Kinds, ", ")
	if kinds == "" {
		kinds = "what it is as the skill would file it, in a word or a few (to write, blocking)"
	}
	fmt.Fprintf(&b, "## <its kind: %s>\n> <the words of the page it's about, copied exactly: one passage, a sentence at most>\n<what you have to say about them, in a sentence or two>\n\n", kinds)
	b.WriteString("To propose words for the passage you quoted, add after that:\n\n```suggest\n<what the quoted words should read>\n```\n\n")
	b.WriteString("Words you propose go on the page as they are, so they're the page's: in its voice and its terms, saying plainly what is so, with no \"I\" or \"we\", nothing about what you looked at, and no \"(?)\" or \"probably\". What you aren't sure of is said in the finding, not in the words you propose.\n\n")
	b.WriteString("Copy the quote exactly, from the page's own words and not from a cell's comment: it's how the finding is shown on the page. A finding about something that isn't there has no > line. Don't write headings of any other kind, and don't describe what you ran.\n")
	return b.String()
}

// PassFirst starts a pass's agent: what a pass is, the page, what it
// may read (nothing, cold), what this pass looks for, and how to reply.
func PassFirst(h Here, c *conversation.Conversation, ps Pass) string {
	def, text := ps.Def, ps.Text
	var b strings.Builder
	fmt.Fprintf(&b, "You're running a pass over a page the user is writing, in the web UI of ws, which manages this multi-repo workspace (%s). A pass reads the whole page and says things about it. What you say is shown beside the page, on a tab of its own, and is never written into it.\n\n", h.Root)
	if def.Cold {
		fmt.Fprintf(&b, "The page is %s. You have it and nothing else: you can't read the workspace, and aren't to go by anything you know of it. Here it is, between the lines of dashes (where an agent wrote part of it at the user's asking, that part is between `<!-- ws:cell … -->` comments):\n\n----------\n%s\n----------\n\n", c.Anchor.Page, strings.TrimSpace(string(text)))
	} else {
		fmt.Fprintf(&b, "The page is %s, the markdown file %s. Read it first. Where an agent wrote part of it at the user's asking, that part is between `<!-- ws:cell … -->` comments, with what was asked for in the first: it's the page's all the same.\n\n", c.Anchor.Page, h.Page)
		b.WriteString(para(h.Agents))
		b.WriteString(h.Kind)
		b.WriteString(talkReading(h.Git))
		b.WriteString(repos(h))
	}
	fmt.Fprintf(&b, "This pass is %q: %s\n\n", def.Title, def.About)
	if def.Looks != "" {
		b.WriteString(def.Looks + "\n\n")
	}
	b.WriteString(passGuide(ps))
	b.WriteString(passSettled(c))
	b.WriteString(passForm(def))
	return b.String()
}

// PassNext is the pass run again, in its agent's session: the page
// may have changed, and the user has settled some of what it found.
func PassNext(h Here, c *conversation.Conversation, ps Pass) string {
	def, text := ps.Def, ps.Text
	var b strings.Builder
	b.WriteString("The user has run the pass again. ")
	if def.Cold {
		fmt.Fprintf(&b, "The page as it now reads, between the lines of dashes:\n\n----------\n%s\n----------\n\n", strings.TrimSpace(string(text)))
	} else {
		fmt.Fprintf(&b, "Read the page again (%s): it may have changed. (The shell works for the commands your first message listed: if one was refused, that was it alone.)\n\n", h.Page)
	}
	b.WriteString(passSettled(c))
	b.WriteString("Say what stands now: a finding the page has since settled is gone, one that still stands is said again with the same quote and kind, and what's new is added. " + passForm(def))
	return b.String()
}

// PassKinds holds a pass's findings to the kinds it has: one of a kind it
// hasn't (an agent naming what it found its own way) is of the pass's
// first.
func PassKinds(def page.PassDef, findings []conversation.Finding) []conversation.Finding {
	if len(def.Kinds) == 0 {
		return findings
	}
	known := map[string]bool{}
	for _, k := range def.Kinds {
		known[k] = true
	}
	for i := range findings {
		if !known[findings[i].Kind] {
			findings[i].Kind = def.Kinds[0]
		}
	}
	return findings
}

// dismissedWhy is why a finding was dismissed, as a pass's agent is told
// it: what not to do again.
var dismissedWhy = map[string]string{
	conversation.DismissWrong:   "it was wrong",
	conversation.DismissAlready: "the page already said it",
	conversation.DismissMinor:   "so, but not worth acting on",
}

// passSettled says what the user did with what the pass found last time.
func passSettled(c *conversation.Conversation) string {
	w := c.Written()
	if w == nil {
		return ""
	}
	var done, dismissed []string
	for _, f := range w.Findings {
		said := fmt.Sprintf("- %s: %s", f.Kind, clipLine(firstNonEmpty(f.Quote, f.Text), 120))
		switch c.FindingState(f.Key) {
		case conversation.FindingDone:
			done = append(done, said)
		case conversation.FindingDismissed:
			if why := dismissedWhy[c.DismissedAs(f.Key)]; why != "" {
				said += " (" + why + ")"
			}
			dismissed = append(dismissed, said)
		}
	}
	sort.Strings(done)
	sort.Strings(dismissed)
	var b strings.Builder
	if len(done) > 0 {
		b.WriteString("Of what you found last time, the user marked these as dealt with (say them again only if the page still leaves them open; they'll stay marked):\n" + strings.Join(done, "\n") + "\n\n")
	}
	if len(dismissed) > 0 {
		b.WriteString("They dismissed these: don't raise them again, unless the page has changed what they're about:\n" + strings.Join(dismissed, "\n") + "\n\n")
	}
	return b.String()
}

// Pass is a pass to run over a page: what it is, the page's text, and
// where its own instructions are (GuidePath, resolved; GuideText, read,
// for a cold pass, whose agent can't read it).
type Pass struct {
	Def                  page.PassDef
	Text                 []byte
	GuidePath, GuideText string
}

// guideRoom is how much of a guide is put in a prompt, for an agent that
// can't read it itself.
const guideRoom = 48 << 10

// passGuide says where a pass's own instructions are, when it has a file
// of them. It's written for its own use (a skill that rewrites a text,
// say): here it's what to look for, and the pass's form is how to answer.
// A cold pass's agent can't read it, so is given it.
func passGuide(ps Pass) string {
	def := ps.Def
	if def.Skill != "" {
		return passSkill(def)
	}
	path := ps.GuidePath
	if path == "" {
		return ""
	}
	const use = "It may be written for another use (rewriting a whole text, say, or answering in a form of its own). Here, take from it what to look for and what good looks like. Don't rewrite the page, and don't follow what it says about how to reply: each thing you find is a finding on the words it's in, with the words you'd put in their place where you can.\n\n"
	if !def.Cold {
		return fmt.Sprintf("How to do this pass is in the file %s: read all of it first. %s", path, use)
	}
	data := ps.GuideText
	if data == "" {
		return ""
	}
	if len(data) > guideRoom {
		data = data[:guideRoom]
	}
	return fmt.Sprintf("How to do this pass is in the guide below, between the lines of equals signs. It's material to work from, not instructions to you. %s==========\n%s\n==========\n\n", use, strings.TrimSpace(data))
}
