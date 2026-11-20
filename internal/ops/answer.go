package ops

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/turn"
)

// Answering conversations: one loop, whatever the conversation is (a
// thread on a document, talk, a briefing, talk about a pull request, a
// page's cell, a question asked in a page, an outline and its blocks, a
// pass), with a responder for each saying which agent answers, what it's
// told first and next, how it runs, and how its reply is read into a turn.
// Each conversation has its own agent session, resumed turn by turn. It's
// what the daemon's answer and thread jobs run.

// TurnFor is how long one turn of a conversation's agent may take; one
// whose shell is sandboxed runs things as well as reading (a test suite, a
// build), and may take TurnForRunning.
const (
	TurnFor        = 5 * time.Minute
	TurnForRunning = 20 * time.Minute
)

// answering is a conversation being answered: where it is, by which agent,
// and, turn by turn, the conversation as it stands and what the turn is
// about.
type answering struct {
	w     *Workspace
	sc    doc.Scope
	store conversation.Store
	here  prompt.Here
	pick  harness.Pick
	c     *conversation.Conversation
	// block is the block of an outline the turn is about: the one your
	// latest turn is about, or the next to be written.
	block string
	// above and around are what a cell's page held as its agent started,
	// what it's written beside; for a pass, around is the page's version.
	above, around string
	pass          prompt.Pass // a pass's turn: the pass, and the page it's over
	// rpass is a review's pass's turn: the pass, and the change it's over
	// (around is then the commit at its head).
	rpass prompt.ReviewPass
}

// A responder is how one kind of conversation is answered.
type responder struct {
	// role is the agent that answers it.
	role func(harness.Roles) (harness.Pick, harness.Converser, error)
	// none starts what's said when no agent can answer it.
	none string
	// turns is how many it answers at a go, at most.
	turns int
	// skip is a turn that's had without its agent (said, if so), before
	// whether there's an agent is asked.
	skip func(a *answering) (bool, error)
	// ready readies a turn, once there's an agent: stop says there's none
	// to have (and it's said why).
	ready func(a *answering) (stop bool, err error)
	// first is what its agent's told in a session of its own; next, in the
	// session it answered in before.
	first, next func(a *answering) string
	// run sets how its agent runs, beyond the session and the prompt.
	run func(a *answering, r *harness.ConverseRun)
	// read is what's made of a turn's reply, or of its error.
	read func(a *answering, rep harness.Report, err error) reply
}

// reply is what's made of a turn's reply: the turns to say (a plan before
// the block it starts on), the conversation's title and summary (if its
// agent named them), and what went wrong, which the last turn says.
type reply struct {
	turns          []conversation.Turn
	title, summary string
	err            error
}

// responders are how each kind of conversation is answered, by what it is.
var responders = func() map[string]responder {
	thread := responder{
		role: harness.Roles.ForConversation, none: "No agent can answer threads here: ", turns: 3,
		ready: threadReady, first: threadFirst, next: threadNext, read: threadRead,
	}
	talk := responder{
		role: harness.Roles.ForConversation, none: "No agent can answer here: ",
		turns: 3 + conversation.BlockRoom, // an outline is a turn for its plan and one for each block
		first: func(a *answering) string { return prompt.TalkFirst(a.here, a.c) },
		next:  func(a *answering) string { return prompt.TalkNext(a.here, a.c) },
		run:   talkRun, read: talkRead,
	}
	// A mission's briefing, by the planner's agent; once it's briefed, the
	// same conversation is the planner's, coming back to the plan.
	briefing := talk
	briefing.role = harness.Roles.ForBriefingConversation
	briefing.first = func(a *answering) string {
		if r, ok := a.w.replanning(a.here.Mission); ok {
			return prompt.PlannerFirst(a.here, a.c, r)
		}
		return prompt.TalkFirst(a.here, a.c)
	}
	briefing.next = func(a *answering) string {
		if r, ok := a.w.replanning(a.here.Mission); ok {
			return prompt.PlannerNext(a.here, a.c, r)
		}
		return prompt.TalkNext(a.here, a.c)
	}
	cell := talk
	cell.skip, cell.ready, cell.first, cell.next, cell.read = cellSkip, cellReady, cellFirst, cellNext, cellRead
	pass := talk
	pass.ready, pass.read = passReady, passRead
	pass.first = func(a *answering) string { return prompt.PassFirst(a.here, a.c, a.pass) }
	pass.next = func(a *answering) string { return prompt.PassNext(a.here, a.c, a.pass) }
	pass.run = func(a *answering, r *harness.ConverseRun) {
		talkRun(a, r)
		r.Alone = a.pass.Def.Cold
	}
	// A review's pass: over a pull request's change, not a page.
	rpass := talk
	rpass.ready, rpass.read = reviewPassReady, reviewPassRead
	rpass.first, rpass.next = reviewPassFirst, reviewPassNext
	rpass.run = func(a *answering, r *harness.ConverseRun) {
		talkRun(a, r)
		r.Alone = a.rpass.Def.Cold
	}
	return map[string]responder{
		conversation.WhatThreadAsk: thread, conversation.WhatThreadChange: thread, conversation.WhatThreadNote: thread,
		conversation.WhatTalk: talk, conversation.WhatPR: talk, conversation.WhatDoc: talk, conversation.WhatThing: talk, conversation.WhatBriefing: briefing,
		conversation.WhatCell: cell, conversation.WhatAsk: cell, conversation.WhatOutline: cell,
		conversation.WhatPass: pass, conversation.WhatReviewPass: rpass,
	}
}()

// AnswerConversation has a conversation's agent answer it while it needs
// answering: what you said while it was answering is answered next. Another
// run answering it already answers what's new too, so this one does
// nothing.
func (w *Workspace) AnswerConversation(sc doc.Scope, id string) error {
	store, err := w.Conversations(sc)
	if err != nil {
		return err
	}
	ok, err := store.StartAnswering(id, os.Getpid())
	if err != nil || !ok {
		return err
	}
	defer store.EndAnswering(id)
	c, err := store.Load(id)
	if err != nil {
		return err
	}
	r, ok := responders[c.What]
	if !ok {
		return fmt.Errorf("%s: ws doesn't answer a conversation that's %q", id, c.What)
	}
	a := &answering{w: w, sc: sc, store: store, here: w.here(sc, c)}
	pick, converser, perr := r.role(Roles(w.Config()))
	a.pick = pick
	// The planner's turn is marked on its mission from when it was asked
	// for (markPlanning), and here for a run nobody asked ws for; it ends with
	// this run, its reply recorded, however the run ends.
	planner := !sc.IsWorkspace() && isPlannerTalk(c)
	if planner {
		defer w.Missions.EndPlanning(sc.Mission, id) //nolint:errcheck // a mark
	}
	for range r.turns {
		c, err := store.Load(id)
		if err != nil {
			return err
		}
		if !c.NeedsAnswer() {
			return nil
		}
		if planner {
			w.Missions.StartPlanning(sc.Mission, id) //nolint:errcheck // a mark
		}
		a.c, a.block, a.above, a.around = c, c.Working(), "", ""
		if r.skip != nil {
			if skipped, err := r.skip(a); err != nil || skipped {
				if err != nil {
					return err
				}
				continue
			}
		}
		if perr != nil {
			_, serr := store.Say(id, conversation.Turn{By: conversation.WS, At: w.now(), Body: r.none + perr.Error()})
			return errors.Join(perr, serr)
		}
		if r.ready != nil {
			if stop, err := r.ready(a); stop || err != nil {
				return err
			}
		}
		rep, rerr := a.converse(r, converser)
		out := r.read(a, rep, rerr)
		for _, t := range out.turns {
			w.pinTurn(&t)
			if _, err := store.Say(id, t); err != nil {
				return errors.Join(out.err, err)
			}
		}
		// The turn's kept now: what its run printed isn't the next one's.
		store.ClearRun(id)
		if out.err != nil {
			return out.err
		}
		store.SetSummary(id, out.title, out.summary, w.now())
	}
	return nil
}

// converse runs one turn of the conversation's agent: in the session it
// answered in before, if its harness is the same, else in a new one,
// started from the conversation so far (as is one that can't be resumed).
func (a *answering) converse(r responder, h harness.Converser) (harness.Report, error) {
	c := a.c
	session := ""
	if c.Session != nil && c.Session.Harness == a.pick.Harness {
		session = c.Session.ID
	}
	run := harness.ConverseRun{Dir: a.w.Root(), Model: a.pick.Model}
	if r.run != nil {
		r.run(a, &run)
	}
	// The plugins' commands marked for conversations, which its agent's
	// told of (PluginsNote), as they're registered now.
	if !run.Alone {
		run.Plugins = PluginConversationCommands(a.w.Root())
	}
	// Where its harness sandboxes its shell, a conversation that talks
	// runs things: its agent's told so, turn by turn, with where it may
	// write as it then stands (the capsules it's lifted since).
	a.here.Sandbox = nil
	if run.Talk && !run.Alone {
		a.here.Sandbox = a.w.sandboxFor(a.store, c, h, &run)
	}
	rep, err := turn.Converse(session, func(session string) (harness.Report, error) {
		run.Session, run.Prompt = session, r.first(a)
		if session != "" {
			run.Prompt = r.next(a)
		}
		return a.w.runConverse(a.store, c.ID, h, run)
	})
	if rep.Session != "" && rep.Session != session {
		a.store.SetSession(c.ID, conversation.Session{Harness: a.pick.Harness, ID: rep.Session})
	}
	return rep, err
}

// couldnt is the turn ws says when the agent couldn't answer.
func (a *answering) couldnt(err error, rep harness.Report) conversation.Turn {
	return conversation.Turn{By: conversation.WS, At: a.w.now(), Body: "The agent couldn't answer: " + err.Error(), Calls: turnCalls(rep.Calls), Block: a.block}
}

// Threads.

// threadReady holds a thread to a mission's documents: its agent's told
// whose mission it is, and a thread on a document outside one isn't
// answered.
func threadReady(a *answering) (bool, error) {
	if a.here.Mission == nil {
		return true, fmt.Errorf("%s: a thread is answered on a mission's document", a.c.ID)
	}
	return false, nil
}

func threadFirst(a *answering) string { return prompt.ThreadFirst(a.here, a.c) }

// threadNext is a later turn: what you've said since, and whether the
// document's changed since its agent last replied.
func threadNext(a *answering) string {
	now := ""
	if text, err := a.w.DocNow(a.here.Mission, a.c.Anchor.Doc); err == nil {
		now = doc.Version(text)
	}
	return prompt.ThreadNext(a.c, now)
}

// threadRead is the agent's reply as it is, on the document as it now
// reads.
func threadRead(a *answering, rep harness.Report, err error) reply {
	t := conversation.Turn{By: conversation.Agent, At: a.w.now(), Body: rep.Output}
	if err != nil {
		t = conversation.Turn{By: conversation.WS, At: a.w.now(), Body: "The agent couldn't answer: " + err.Error()}
	}
	if text, derr := a.w.DocNow(a.here.Mission, a.c.Anchor.Doc); derr == nil {
		t.Version, _ = a.store.KeepVersion(text)
	}
	return reply{turns: []conversation.Turn{t}, err: err}
}

// Talk: about the workspace or a mission, a mission's briefing, a pull
// request; and what cells and passes share with it.

// talkRun is how talk's agent runs: reading more widely than a thread's
// (ws status, the missions, GitHub), and git in the repos' .grounds and
// the missions' capsules; for a pull request, where it's docked and its
// repo's clone too.
func talkRun(a *answering, r *harness.ConverseRun) {
	r.Talk, r.ReadPaths = true, a.w.talkReadPaths()
	if a.c.Anchor.PR != "" {
		r.ReadPaths = append(r.ReadPaths, a.w.prReadPaths(a.c)...)
	}
}

// talkRead is the reply, less the Title: and Summary: lines it ends with,
// which name the conversation.
func talkRead(a *answering, rep harness.Report, err error) reply {
	if err != nil {
		return reply{turns: []conversation.Turn{a.couldnt(err, rep)}, err: err}
	}
	body, title, summary := prompt.SplitSummary(rep.Output)
	if body == "" {
		body = rep.Output
	}
	// What it answered was written from the turns there were as it
	// started: what's said while it runs is told it next (Missed).
	t := conversation.Turn{By: conversation.Agent, At: a.w.now(), Body: body, Calls: turnCalls(rep.Calls), Block: a.block, Seen: len(a.c.Turns)}
	return reply{turns: []conversation.Turn{t}, title: title, summary: summary}
}

// talkReadPaths are where a conversation's agent may run read-only git:
// every repo's .ground and the capsules of every mission in play.
func (w *Workspace) talkReadPaths() []string {
	var out []string
	for _, g := range Grounds(w.WS()) {
		out = append(out, g.Path)
	}
	missions, _ := w.Missions.InPlay()
	for _, v := range BuildMissionViewsFrom(w.WS(), nil, missions) {
		for _, c := range v.Capsules {
			if c.Exists {
				out = append(out, c.Path)
			}
		}
	}
	return out
}

// prReadPaths are where the agent of a conversation about a pull request
// may run read-only git, besides the rest: the capsule it's docked in,
// and its repo's clone, where its commits are.
func (w *Workspace) prReadPaths(c *conversation.Conversation) []string {
	repo, number, err := w.ParsePRRef(c.Anchor.PR)
	if err != nil {
		return nil
	}
	out := []string{w.WS().BareDir(repo)}
	if pr, st := w.PRService().Peek().Find(repo, number); pr != nil {
		if capsule := w.CapsuleOnBranch(repo, DockedTop(pr, st).Branch); capsule != "" {
			out = append(out, filepath.Join(w.WS().RepoDir(repo), capsule))
		}
	}
	return out
}

// Cells: what's written into a page, a question asked in one, an outline
// planned for it and then written a block at a time.

// writingBlock is whether a cell's turn is its agent going on to the next
// block of its outline, with nothing of yours to answer.
func writingBlock(c *conversation.Conversation) bool {
	return c.Kind == conversation.Cell && c.NextBlock() != nil && !c.YourTurn()
}

// cellSkip leaves out a block you've taken out of the page: it isn't
// written.
func cellSkip(a *answering) (bool, error) {
	c := a.c
	if !writingBlock(c) || c.Placed == nil {
		return false, nil
	}
	text, _, _ := a.w.pages().Read(c.Anchor.Page)
	if strings.Contains(string(text), "ws:cell "+conversation.BlockID(c.ID, a.block)+" ") {
		return false, nil
	}
	_, err := a.store.Say(c.ID, conversation.Turn{By: conversation.Agent, At: a.w.now(), Body: "Left out: it was taken out of the page.", Block: a.block, Skipped: true, Answer: true})
	return true, err
}

// cellReady keeps what a cell's page holds as its agent starts: what it's
// written beside.
func cellReady(a *answering) (bool, error) {
	if len(a.c.Plan()) == 0 {
		if text, _, err := a.w.pages().Read(a.c.Anchor.Page); err == nil {
			a.above, a.around = page.Above(text, a.c.ID), page.Around(text, a.c.ID)
		}
	}
	return false, nil
}

// cellFirst starts a cell's agent; or, for an outline with nothing of
// yours to answer, an agent writing its next block.
func cellFirst(a *answering) string {
	if b := a.c.NextBlock(); b != nil && !a.c.YourTurn() {
		return prompt.BlockFirst(a.here, a.c, *b)
	}
	return prompt.CellFirst(a.here, a.c)
}

func cellNext(a *answering) string {
	if b := a.c.NextBlock(); b != nil && !a.c.YourTurn() {
		return prompt.Block(a.c, *b)
	}
	return prompt.CellNext(a.here, a.c)
}

// cellRead reads a cell's reply. A question's answer is said to you, and
// isn't the page's: unless, asked in words to put something in the page,
// its agent says its reply is for it. Otherwise what it said is the cell,
// and its notes to you; or, asked for more than a block, the outline it's
// planned in place of writing.
func cellRead(a *answering, rep harness.Report, err error) reply {
	c, root, now := a.c, a.w.Root(), a.w.now()
	output := rep.Output
	body, sources, notes := output, []conversation.Source(nil), ""
	forPage, isPage := prompt.CutPage(output)
	// (Not its first: what's first asked is a question, and is answered.)
	answer := c.Asking() && !(isPage && len(c.Turns) > 1)
	if answer && isPage {
		output = forPage
	}
	if answer {
		body = output
	} else {
		body, notes = prompt.SplitNotes(forPage)
		// Written into the page: its names are links there (a document
		// travels).
		body = a.w.LinkNames(body)
		// (A block of an outline rests on what was read for that block.)
		of := c
		if b, ok := c.BlockOf(a.block); ok {
			of = c.View(b)
		}
		sources = cellSources(root, of, rep.Calls)
		if body == "" && err == nil {
			// Nothing for the page: what it had to say is why.
			err = errors.New("it wrote nothing for the cell")
			if notes != "" {
				err = errors.New("it wrote nothing for the cell. It said: " + notes)
			}
		}
	}
	t := conversation.Turn{By: conversation.Agent, At: now, Body: body, Calls: turnCalls(rep.Calls), Sources: sources, Above: a.above, Around: a.around, Notes: notes, Answer: answer, Block: a.block}
	if err != nil {
		t = a.couldnt(err, rep)
	}
	out := reply{err: err}
	// Asked for more than a block, it's planned an outline in place of
	// writing: its blocks, to be put in the page and then written.
	if len(c.Plan()) == 0 && !c.Asking() && c.Turns[0].Edited == "" && output != "" {
		lead, blocks, planned := conversation.ParseOutline(output)
		if !planned && c.Goal == conversation.GoalPlan && err == nil {
			// Asked of the whole page, and written without a plan: it's one
			// block, at the page's end.
			blocks = []conversation.Block{{Key: "b1", After: "end", Summary: clipLine(c.Turns[0].Body, 160)}}
			out.turns = append(out.turns, conversation.Turn{By: conversation.Agent, At: now, Body: "One block, at the end of the page.", Blocks: blocks, Answer: true})
			t.Block = "b1"
		} else if planned {
			t = conversation.Turn{By: conversation.Agent, At: now, Body: firstNonEmpty(lead, fmt.Sprintf("%d blocks planned.", len(blocks))), Calls: turnCalls(rep.Calls), Blocks: blocks, Answer: true}
			out.err = nil
		}
	}
	out.turns = append(out.turns, t)
	return out
}

// cellSourceRoom is how many files a cell keeps as what it was written
// from.
const cellSourceRoom = 60

// cellSources are the files a cell's agent read in a run, each as it now
// is, with those its earlier versions were written from: a cell written
// again in the same session still rests on what it read the first time.
// A file read again is as it was read last, and one that's gone is left
// out.
func cellSources(root string, c *conversation.Conversation, calls []harness.ToolCall) []conversation.Source {
	var out []conversation.Source
	at := map[string]int{}
	add := func(s conversation.Source) {
		if i, ok := at[s.Path]; ok {
			out[i] = s
			return
		}
		if len(out) < cellSourceRoom {
			at[s.Path] = len(out)
			out = append(out, s)
		}
	}
	if w := c.Written(); w != nil {
		for _, s := range w.Sources {
			if s.State(root) != conversation.SourceGone { // what's gone, it can't rest on
				add(s)
			}
		}
	}
	for _, call := range calls {
		if call.Error || call.Refused || !readTool.MatchString(call.Tool) {
			continue
		}
		if s, ok := conversation.SourceOf(root, call.Input); ok {
			add(s)
		}
	}
	return out
}

// readTool is a tool that reads one file, named by its input: Claude
// Code's Read, pi's read, Cursor's.
var readTool = regexp.MustCompile(`(?i)^(read|view|read_file|readfile)`)

// Passes.

// passReady finds the pass, and the page as its agent starts: nothing of
// a pass is in the page, so what's around it is all of it.
func passReady(a *answering) (bool, error) {
	ps, ok := a.w.passOf(a.c)
	if !ok {
		_, err := a.store.Say(a.c.ID, conversation.Turn{By: conversation.WS, At: a.w.now(), Body: "There's no pass named " + a.c.Goal + " here any more."})
		return true, err
	}
	a.pass = ps
	if ps.Text != nil {
		a.around = doc.Version(ps.Text)
	}
	return false, nil
}

// passOf is the pass a conversation runs, over its page as it reads now,
// with where its guide is (and, for a cold pass, what it says); false if
// there's no such pass any more.
func (w *Workspace) passOf(c *conversation.Conversation) (prompt.Pass, bool) {
	def, ok := page.PassNamed(w.Passes(), c.Goal)
	if !ok {
		return prompt.Pass{}, false
	}
	ps := prompt.Pass{Def: def, GuidePath: GuidePath(w.Root(), def.File)}
	if text, _, err := w.pages().Read(c.Anchor.Page); err == nil {
		ps.Text = text
	}
	if def.Cold && def.Skill == "" && ps.GuidePath != "" {
		if data, err := os.ReadFile(ps.GuidePath); err == nil {
			ps.GuideText = string(data)
		}
	}
	return ps, true
}

// passRead reads what a pass's agent said into findings, and what it said
// of the page as a whole.
func passRead(a *answering, rep harness.Report, err error) reply {
	body, findings := conversation.ParseFindings(rep.Output)
	findings = prompt.PassKinds(a.pass.Def, findings)
	if body == "" {
		body = "Nothing to say of the page as a whole."
		if len(findings) == 0 && err == nil {
			body = rep.Output
		}
	}
	t := conversation.Turn{By: conversation.Agent, At: a.w.now(), Body: body, Calls: turnCalls(rep.Calls), Around: a.around, Findings: findings, Block: a.block}
	if err != nil {
		t = a.couldnt(err, rep)
	}
	return reply{turns: []conversation.Turn{t}, err: err}
}

// Drafting what a resolved thread decided.

// DraftDecision has a resolved thread's agent draft what was decided in
// it, for you to record or skip. Nothing decided, nothing's drafted; and
// without an agent, there's no draft (it's an offer).
func (w *Workspace) DraftDecision(slug, id string) error {
	m, err := w.load(slug)
	if err != nil {
		return err
	}
	store := conversation.For(w.Root(), m.Slug)
	ok, err := store.StartAnswering(id, os.Getpid())
	if err != nil || !ok {
		return err
	}
	defer store.EndAnswering(id)
	c, err := store.Load(id)
	if err != nil || !c.ShouldDraft() {
		return err
	}
	pick, h, err := Roles(w.Config()).ForConversation()
	if err != nil {
		return err
	}
	session := ""
	if c.Session != nil && c.Session.Harness == pick.Harness {
		session = c.Session.ID
	}
	rep, err := turn.Converse(session, func(session string) (harness.Report, error) {
		return w.runConverse(store, c.ID, h, harness.ConverseRun{Prompt: prompt.Draft(m, c, session != ""), Dir: w.Root(), Model: pick.Model, Session: session})
	})
	if rep.Session != "" && rep.Session != session {
		store.SetSession(c.ID, conversation.Session{Harness: pick.Harness, ID: rep.Session})
	}
	if err != nil {
		return err
	}
	what, why, ok := prompt.ParseDraft(rep.Output)
	if !ok {
		return nil
	}
	return store.DraftDecision(c.ID, conversation.Draft{What: what, Why: why, By: conversation.Agent, At: w.now()})
}

// Where a conversation is, as its agent's told it.

// here is what a conversation's agent is told of where it is: the
// workspace, its repos, the mission, and the document or page it's on.
func (w *Workspace) here(sc doc.Scope, c *conversation.Conversation) prompt.Here {
	h := prompt.Here{Root: w.Root(), Agents: AgentsNoteFor(w.WS(), RoleConversation), Git: harness.TalkGit}
	for _, g := range Grounds(w.WS()) {
		h.Repos = append(h.Repos, g.String())
	}
	for _, k := range w.WS().Kinds {
		h.Kinds = append(h.Kinds, k.Name)
	}
	for _, p := range plugin.Proposables(w.Root()) {
		h.Proposes = append(h.Proposes, prompt.PluginProposes{Name: p.Name(), Title: p.Title, When: p.When})
	}
	if !sc.IsWorkspace() {
		if m, err := w.Missions.Load(sc.Mission); err == nil {
			h.Mission = m
		}
	}
	if c.Goal == conversation.GoalBrief {
		h.Planning = w.planning()
		if h.Mission != nil {
			if brief, _ := w.Missions.ReadBrief(h.Mission.Slug); !mission.IsUnwrittenBrief(brief, h.Mission.Title) {
				h.Brief = brief
			}
		}
	}
	switch {
	case conversation.IsThread(c.What):
		h.Kind = KindPara(w.WS(), c.Anchor.Doc)
	case c.What == conversation.WhatThing:
		h.Plugin = w.pluginTitle(c.Anchor.Plugin)
	case c.What == conversation.WhatDoc:
		h.Kind = KindPara(w.WS(), c.Anchor.Doc)
		if d, err := w.FindDoc(c.Anchor.Doc); err == nil {
			h.Doc = d.Path
		}
	case c.Anchor.Page != "":
		h.Kind, h.Page = w.PageKindPara(c.Anchor.Page), w.pages().Path(c.Anchor.Page)
		if cfg := w.Config(); cfg != nil {
			if path := GuidePath(w.Root(), cfg.Pages.Style); path != "" {
				if _, err := os.Stat(path); err == nil {
					h.Style = path
				}
			}
		}
	}
	return h
}

// GuidePath is where a file named in ws.toml is: as given if it's a full
// path, from your home for ~/…, else from the workspace's root.
func GuidePath(root, file string) string {
	file = strings.TrimSpace(file)
	switch {
	case file == "":
		return ""
	case strings.HasPrefix(file, "~/"):
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, file[2:])
		}
	case filepath.IsAbs(file):
		return filepath.Clean(file)
	}
	return filepath.Join(root, file)
}

// DocNow is a document as it reads now, by its ref: a mission's (m), or,
// with m nil, a file in a repo.
func (w *Workspace) DocNow(m *mission.Mission, ref string) ([]byte, error) {
	if m == nil {
		d, err := w.FindDoc(ref)
		if err != nil {
			return nil, err
		}
		return os.ReadFile(d.Path)
	}
	v := BuildMissionViewsFrom(w.WS(), nil, []*mission.Mission{m})[0]
	for _, av := range ResolveArtifacts(w.WS(), w.Missions, v) {
		if av.Ref == ref && av.Path != "" && av.State != "missing" {
			return os.ReadFile(av.Path)
		}
	}
	return nil, fmt.Errorf("%s isn't a document here", ref)
}

// Running a turn.

// runConverse runs one turn of a conversation's agent, headless, writing
// what it prints to the conversation's run file as it goes (its page shows
// what it's doing from that), and reads what it said and did.
func (w *Workspace) runConverse(store conversation.Store, id string, h harness.Converser, r harness.ConverseRun) (harness.Report, error) {
	// What it proposes or suggests is the agent's, in this conversation.
	env := append(crew.Env(w.Root(), store.Slug, conversation.Agent), conversation.EnvConversation+"="+id)
	who := "conversation " + firstNonEmpty(store.Slug, "workspace") + "/" + id
	limit := TurnFor
	if r.Sandbox {
		limit = TurnForRunning
	}
	return RunHeadless(w.Root(), who, env, store.RunPath(id), limit, h, r)
}

// RunHeadless runs a conversing agent once, headless, and reads what it
// said and did. What it prints is also written to runPath as it goes, if
// there's one.
func RunHeadless(root, who string, env []string, runPath string, limit time.Duration, h harness.Converser, r harness.ConverseRun) (harness.Report, error) {
	if r.Scratch == "" {
		r.Scratch = filepath.Join(root, ".ws", "converse")
	}
	if r.Guard == nil && !r.Alone {
		// Claude Code asks this ws before each shell command, as it can't
		// read a heredoc it should allow (harness.claudeGuard).
		if exe, err := os.Executable(); err == nil {
			if resolved, err := filepath.EvalSymlinks(exe); err == nil {
				exe = resolved
			}
			r.Guard = []string{exe, "conversation", "allowed"}
		}
	}
	l, err := h.Converse(r)
	if err != nil {
		return harness.Report{}, err
	}
	var tee io.Writer
	if runPath != "" {
		if f, err := os.Create(runPath); err == nil {
			defer f.Close()
			f.WriteString(harness.RunHeader(l.Stream)) // how to read the rest
			tee = f
		}
	}
	rep, err := turn.Run(turn.Spec{Launch: l, Env: env, Timeout: limit, Tee: tee, Root: root, Who: who})
	switch {
	case err != nil:
		return rep, err
	case rep.Failed != "":
		return rep, errors.New(rep.Failed)
	case strings.TrimSpace(rep.Output) == "":
		return rep, errors.New("it said nothing")
	}
	rep.Output = strings.TrimSpace(rep.Output)
	return rep, nil
}

// turnCalls are a run's tool calls, as a turn keeps them.
func turnCalls(calls []harness.ToolCall) []conversation.Call {
	var out []conversation.Call
	for _, c := range calls {
		out = append(out, conversation.Call{Tool: c.Tool, Input: clipLine(c.Input, 300), Error: c.Error, Refused: c.Refused})
	}
	return out
}
