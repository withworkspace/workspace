// Package conversation keeps conversations: turns between the user and an
// agent (or crew), about something. A thread on a document is one,
// anchored to the text it's about; discovery about a mission or the
// workspace will be others.
//
// A conversation is a log: a line per thing that happened (opened, a turn,
// resolved or reopened), replayed to load it. Its lines are kept in the
// workspace's records (records.go), a mission's by its slug and the
// workspace's own as its scope; a run answering one writes what it prints
// to a file beside where they used to be (.ws/missions/<slug>/
// conversations/, .ws/conversations/). Each change is also logged to the
// mission.
package conversation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// Thread kinds: what you want from a thread on a document.
const (
	Ask    = "ask"    // an answer
	Change = "change" // the document changed
	Note   = "note"   // to say something
)

// Talk is a conversation that isn't on a document: about the workspace
// (w1, w2, …), or a mission (its Talk tab).
const Talk = "talk"

// Cell is a cell of a page: a question asked in the page, whose agent's
// replies are what fills it (w1, w2, … with the workspace's conversations).
// What you say after its first message is a tweak to what it wrote.
const Cell = "cell"

// Kinds a thread can be.
var Kinds = []string{Ask, Change, Note}

// What a conversation is, which says how it's answered: a thread of each
// kind; talk about the workspace or a mission, a mission's briefing, talk
// about a pull request, or a question about a document asked as it's read
// (an ask, from the reader's ask bar); a page's cell, a question asked in a page, or
// something asked of the whole page (its outline, then its blocks); a pass
// over a page.
const (
	WhatThreadAsk    = "thread-ask"
	WhatThreadChange = "thread-change"
	WhatThreadNote   = "thread-note"
	WhatTalk         = "talk"
	WhatBriefing     = "briefing"
	WhatPR           = "pr"
	WhatDoc          = "doc"
	WhatThing        = "thing"
	WhatCell         = "cell"
	WhatAsk          = "ask"
	WhatOutline      = "outline"
	WhatPass         = "pass"
	WhatReviewPass   = "review-pass"
)

// IsThread is whether a What is a thread on a document's.
func IsThread(what string) bool { return strings.HasPrefix(what, "thread-") }

// whatOf works out what a conversation is from its kind, its goal and the
// shape of its anchor.
func whatOf(c *Conversation) string {
	switch c.Kind {
	case Ask, Change, Note:
		return "thread-" + c.Kind
	case Talk:
		switch {
		case c.Goal == GoalBrief:
			return WhatBriefing
		case c.Anchor.PR != "":
			return WhatPR
		case c.Anchor.Thing != "":
			return WhatThing
		case c.Anchor.Doc != "":
			return WhatDoc
		}
		return WhatTalk
	case Cell:
		switch {
		case c.Goal == GoalAsk || c.Anchor.Finding != "":
			return WhatAsk
		case c.Goal == GoalPlan:
			return WhatOutline
		}
		return WhatCell
	case Pass:
		if c.Anchor.Reviewed() != "" {
			return WhatReviewPass
		}
		return WhatPass
	}
	return ""
}

// States.
const (
	Open     = "open"
	Resolved = "resolved"
)

// You is who the user is in a conversation.
const You = "you"

// EnvConversation names the conversation an agent is answering, in its
// environment: what it proposes is proposed there.
const EnvConversation = "WS_CONVERSATION"

// Quote is text a thread highlights, as it reads rendered, with a little
// of what's either side to find it by again. An empty Exact is a thread on
// the whole of its lines (a diagram).
type Quote struct {
	Exact  string `json:"exact"`
	Prefix string `json:"prefix,omitempty"`
	Suffix string `json:"suffix,omitempty"`
}

// Anchor is what a conversation is about. For a thread, a document's text:
// the artifact's ref, the quote, the markdown lines it's in, and the
// document's version it was opened on.
type Anchor struct {
	Doc     string `json:"doc,omitempty"`
	Path    string `json:"path,omitempty"` // the document's path, from its ref
	Quote   *Quote `json:"quote,omitempty"`
	Line    int    `json:"line,omitempty"`
	EndLine int    `json:"endLine,omitempty"`
	Version string `json:"version,omitempty"`
	// Excerpt is the lines as they were, so it reads the same however the
	// document moves on.
	Excerpt []string `json:"excerpt,omitempty"`
	// PR is the pull request a conversation is about (api#89), asked from
	// the Reviews page: then Path, Line, EndLine and Excerpt are lines of
	// its diff, when it's about some, on Side ("new", or "old" for lines it
	// removes) at Commit.
	PR     string `json:"pr,omitempty"`
	Side   string `json:"side,omitempty"`
	Commit string `json:"commit,omitempty"`
	// Mission and Capsule are the capsule (repo/name) of a mission that a
	// review's pass is over: its changes, as a pull request's are.
	Mission string `json:"mission,omitempty"`
	Capsule string `json:"capsule,omitempty"`
	// Page is the page a cell is in (p3).
	Page string `json:"page,omitempty"`
	// Finding is the finding of a pass that a question is about
	// (check:3f9a…): asked from the pass's tab, it has no place in the page
	// unless you have its answer put there.
	Finding string `json:"finding,omitempty"`
	// Section is the heading of the section a quote is in, as the reader
	// draws it: for an ask about a passage (ask.go).
	Section string `json:"section,omitempty"`
	// Thing is a plugin's thing a conversation is about (sketch-13), asked
	// from the plugin's page as it's open, and Plugin is whose it is.
	Thing  string `json:"thing,omitempty"`
	Plugin string `json:"plugin,omitempty"`
}

// Turn is one message.
type Turn struct {
	N    int       `json:"n"`
	By   string    `json:"by"` // "you", a crew id, "agent"
	At   time.Time `json:"at"`
	Body string    `json:"body"`
	// Version is the document's version when it was said, on a thread.
	Version string `json:"version,omitempty"`
	// Commit is what crew committed for it, if they say.
	Commit string `json:"commit,omitempty"`
	// Calls are what the agent did on the way to saying it: its tool
	// calls, in order.
	Calls []Call `json:"calls,omitempty"`
	// Changed: crew say they've changed the document for it, which hands
	// it back to you.
	Changed bool `json:"changed,omitempty"`
	// Sources are the files a cell's agent read on the way to writing it,
	// each as it then was: what the cell was written from.
	Sources []Source `json:"sources,omitempty"`
	// Above and Around name what a cell's page held when its agent started
	// on this version (page.Above, page.Around): the cells before it, and
	// the page but for the cell itself. They tell when either's changed
	// since.
	Above  string `json:"above,omitempty"`
	Around string `json:"around,omitempty"`
	// Blocks, on a cell's agent's turn, is an outline it planned in place
	// of writing the cell: the blocks it'll write, in order (outline.go).
	// Block, on a later turn, is the block it's about: a version of it its
	// agent wrote, or what you asked of it. Skipped says its agent was told
	// to leave the block out (you'd taken it out of the page).
	Blocks  []Block `json:"blocks,omitempty"`
	Block   string  `json:"block,omitempty"`
	Skipped bool    `json:"skipped,omitempty"`
	// Ask, on a turn of yours in a cell, is a question: it's answered to
	// you, beside the page, and nothing is written into the page for it
	// (>? in a page, or asking in a cell's panel). Answer, on the agent's
	// turn, is that answer: not a version of the cell.
	Ask    bool `json:"ask,omitempty"`
	Answer bool `json:"answer,omitempty"`
	// Asks, on ws's turn, is ws asking the conversation's agent something
	// itself (a mission reaching the planner's step): it's answered as a
	// turn of yours is.
	Asks bool `json:"asks,omitempty"`
	// Origin, on a turn of yours, is what ws wrote it for, where you
	// didn't type it: "proposal:3", your settling proposal 3
	// (ops.tellAgent). The agent reads it as yours, as it reads what you
	// type, and the page shows it as what happened, not as your words.
	Origin string `json:"origin,omitempty"`
	// Quiet, on such a turn, is one the agent isn't woken for: it reads it
	// on its next turn, whenever that is. It doesn't change whose turn it
	// is.
	Quiet bool `json:"quiet,omitempty"`
	// Seen, on the agent's turn in a talk, is how many of the
	// conversation's turns there were when what it answered was written:
	// what was said after them, before this turn (while it was answering:
	// a card settled, a reply sent), it hasn't read (Missed).
	Seen int `json:"seen,omitempty"`
	// Refs are what the names in it were when it was said (internal/ref):
	// what's drawn for them is that, not what the words find now.
	Refs []ref.Pinned `json:"refs,omitempty"`
	// Notes are what a cell's agent had to tell you about a version it
	// wrote (what it couldn't find, didn't check, had to guess): said to
	// you, beside the cell, and never part of the page.
	Notes string `json:"notes,omitempty"`
	// Findings are what a pass found in this run, read from its reply; Body
	// is then what it said of the page as a whole.
	Findings []Finding `json:"findings,omitempty"`
	// Guide is what a review's guide said in this run: what the change
	// does and the order to read it in (guide.go).
	Guide *ReviewGuide `json:"guide,omitempty"`
	// Edited is a cell as you'd edited it by hand when you asked for it to
	// be changed: what its agent starts from, kept so it isn't lost when
	// the cell's written again.
	Edited string `json:"edited,omitempty"`
	// Given, on a turn of yours, is what the page it was said on gave the
	// agent with it: not your words, and not shown as them.
	Given *Given `json:"given,omitempty"`
}

// Given is what a plugin's page gave a conversation's agent with
// something you said there (ws.conversation in the page): information
// for the agent, not instructions, and folded under what you said, so you
// can see what it was told.
type Given struct {
	// From is what gave it: the plugin's title (Sketch).
	From string `json:"from,omitempty"`
	// Page is the page's standing context, its state as words: given only
	// when it's changed since the agent was last given it (StandingGiven).
	Page string `json:"page,omitempty"`
	// Holding is what you had in hand there (a selection): its label, and
	// it as words.
	Holding     string `json:"holding,omitempty"`
	HoldingText string `json:"holdingText,omitempty"`
	// Context is what the page gave with this message alone.
	Context string `json:"context,omitempty"`
}

// Empty is whether it gives nothing.
func (g *Given) Empty() bool {
	return g == nil || strings.TrimSpace(g.Page+g.HoldingText+g.Context) == "" && strings.TrimSpace(g.Holding) == ""
}

// givenMax is the most of any one part of what's given that's kept.
const givenMax = 20000

// Clipped is it with each part no longer than ws keeps.
func (g Given) Clipped() Given {
	g.From = clipRunes(strings.TrimSpace(g.From), 80)
	g.Page = clipRunes(strings.TrimSpace(g.Page), givenMax)
	g.Holding = clipRunes(strings.TrimSpace(g.Holding), 200)
	g.HoldingText = clipRunes(strings.TrimSpace(g.HoldingText), givenMax)
	g.Context = clipRunes(strings.TrimSpace(g.Context), givenMax)
	return g
}

// StandingGiven is the page's standing context as its agent was last
// given it: what a page's next has to differ from to be given again.
func (c *Conversation) StandingGiven() string {
	for _, v := range slices.Backward(c.Turns) {
		if g := v.Given; g != nil && g.Page != "" {
			return g.Page
		}
	}
	return ""
}

// Give is what the page gives with a turn now, against what the
// conversation's agent was given before (nil: a new conversation): its
// standing context only if it's changed since. nil when that leaves
// nothing.
func (c *Conversation) Give(g Given) *Given {
	g = g.Clipped()
	if c != nil && g.Page != "" && g.Page == c.StandingGiven() {
		g.Page = ""
	}
	if g.Empty() {
		return nil
	}
	return &g
}

// Source is a file a cell was written from, as it was when it was read:
// its path from the workspace's root, and its size, time and hash, to
// tell when it's changed since.
type Source struct {
	Path    string    `json:"path"`
	Hash    string    `json:"hash"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// SourceOf is a file as a source: read now. ok is false for what isn't a
// file under root, or is ws's own (.ws: the page itself, which changes as
// you type).
func SourceOf(root, path string) (Source, bool) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || rel == ".ws" || strings.HasPrefix(rel, ".ws"+string(filepath.Separator)) {
		return Source{}, false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return Source{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Source{}, false
	}
	return Source{Path: filepath.ToSlash(rel), Hash: doc.Version(data), Size: info.Size(), ModTime: info.ModTime()}, true
}

// Source states.
const (
	SourceSame    = ""
	SourceChanged = "changed"
	SourceGone    = "gone"
)

// State is whether a source is as it was when it was read: the same,
// changed, or gone. A file whose size and time are as they were isn't read
// again.
func (s Source) State(root string) string {
	path := filepath.Join(root, filepath.FromSlash(s.Path))
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return SourceGone
	}
	if info.Size() == s.Size && info.ModTime().Equal(s.ModTime) {
		return SourceSame
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SourceGone
	}
	if doc.Version(data) != s.Hash {
		return SourceChanged
	}
	return SourceSame
}

// Written is the latest thing a cell's agent wrote for the page (or a
// pass's latest run): the cell as it now reads, and what it was written
// from. An answer to a question isn't one. nil for a cell not filled yet.
func (c *Conversation) Written() *Turn {
	for i := len(c.Turns) - 1; i >= 0; i-- {
		if c.Turns[i].By == Agent && !c.Turns[i].Answer {
			return &c.Turns[i]
		}
	}
	return nil
}

// Asking is whether what a cell's agent is to answer now is a question
// (the latest of your turns asks), rather than something to write into
// the page.
func (c *Conversation) Asking() bool {
	return c.Kind == Cell && c.asked().By == You && c.asked().Ask
}

// Call is a tool call an agent made on the way to a turn.
type Call struct {
	Tool    string `json:"tool"`
	Input   string `json:"input"`
	Error   bool   `json:"error,omitempty"`
	Refused bool   `json:"refused,omitempty"`
}

// Sent is a thread handed to crew to change the document for: theirs
// until they say they've changed it.
type Sent struct {
	Crew string    `json:"crew"`
	By   string    `json:"by"`
	At   time.Time `json:"at"`
	// Finished: the crew have ended, or reported done, since, without
	// saying they've changed it (worked out, not stored).
	Finished bool `json:"finished,omitempty"`
}

// Conversation is one conversation, as it stands.
type Conversation struct {
	ID   string `json:"id"` // t1, t2, … per mission; w1, w2, … the workspace's
	Kind string `json:"kind,omitempty"`
	// What is what it is, as it's answered (WhatThreadAsk, WhatTalk,
	// WhatOutline, …): set when it's opened, and worked out from its kind,
	// goal and anchor for one opened before it was kept.
	What string `json:"what,omitempty"`
	// Goal is what a conversation is for, when it's for something: "brief",
	// briefing its mission.
	Goal string `json:"goal,omitempty"`
	// Title and Summary are the agent's: a few words naming it, and what
	// it's about, what's concluded and what's open.
	Title   string    `json:"title,omitempty"`
	Summary string    `json:"summary,omitempty"`
	Anchor  Anchor    `json:"anchor"`
	By      string    `json:"by"`
	At      time.Time `json:"at"`
	// Via is the plugin a talk was started from, for you: it worded the
	// first message. "" for one you started yourself.
	Via string `json:"via,omitempty"`
	// Chain is the automations that led to it being started (an
	// automation's converse, and any that set that one off): what it does
	// says so in its events, so what it sets off can't loop.
	Chain  []string `json:"chain,omitempty"`
	Status string   `json:"status"`
	// StatusBy and StatusAt are who resolved or reopened it, and when.
	StatusBy string     `json:"statusBy,omitempty"`
	StatusAt *time.Time `json:"statusAt,omitempty"`
	Turns    []Turn     `json:"turns"`
	// Session is the agent session its turns resume, once there is one.
	Session *Session `json:"session,omitempty"`
	// Model is the model you chose for its turns, over its role's, on the
	// harness it was chosen on (a name is that harness's): none, or one
	// chosen on another, and its turns are on the role's.
	Model *Model `json:"model,omitempty"`
	// Answering: an agent is answering it now (worked out, not stored),
	// and since when.
	Answering      bool       `json:"answering,omitempty"`
	AnsweringSince *time.Time `json:"answeringSince,omitempty"`
	// Suggestions are the edits suggested in it, oldest first: at most
	// one is open, the latest.
	Suggestions []Suggestion `json:"suggestions,omitempty"`
	// Sent is the crew it's been handed to, until they say they've
	// changed the document for it.
	Sent *Sent `json:"sent,omitempty"`
	// Draft is what was decided in it, drafted by its agent once you
	// resolved it, for you to record as a decision or skip.
	Draft *Draft `json:"draft,omitempty"`
	// Proposals are what its agent proposed, oldest first.
	Proposals []Proposal `json:"proposals,omitempty"`
	// Capsules are those its agent lifted to try things in, oldest first:
	// its own, put away with it (capsules.go).
	Capsules []Capsule `json:"capsules,omitempty"`
	// Settled is, for a pass, the findings you've marked done or dismissed,
	// by key; Dismissed, why each dismissed one was, where you said.
	Settled   map[string]string `json:"settled,omitempty"`
	Dismissed map[string]string `json:"dismissed,omitempty"`
	// Placed is when an outline's blocks were put into its page.
	Placed *time.Time `json:"placed,omitempty"`
	// Ephemeral is an ask that isn't kept (ask.go): it's left out of the
	// lists, the mission's log, the graph and what's found, and swept once
	// it's sat a while after its last turn.
	Ephemeral bool `json:"ephemeral,omitempty"`
	// Kept is when an ask was kept (Keep as a conversation): a
	// conversation like any from then on.
	Kept *time.Time `json:"kept,omitempty"`

	lines   int       // how many lines its log has, as loaded
	changed time.Time // when it last changed, as loaded
}

// Draft states.
const (
	DraftOpen     = "open"
	DraftRecorded = "recorded"
	DraftSkipped  = "skipped"
)

// Draft is a decision drafted from a resolved thread.
type Draft struct {
	What  string    `json:"what"`
	Why   string    `json:"why,omitempty"`
	By    string    `json:"by"`
	At    time.Time `json:"at"`
	State string    `json:"state"`
	// Decision is the decision recorded from it (d3).
	Decision string `json:"decision,omitempty"`
}

// WithCrew is whether crew have it: sent to them, and they're still on it.
func (c *Conversation) WithCrew() bool { return c.Sent != nil && !c.Sent.Finished }

// Suggestion states.
const (
	SuggestOpen       = "open"
	SuggestAccepted   = "accepted"
	SuggestDeclined   = "declined"
	SuggestSuperseded = "superseded"
)

// Suggestion is an edit suggested in a thread: From, text in the
// document's markdown that appears in it exactly once, to be replaced by
// To ("" deletes it).
type Suggestion struct {
	N       int       `json:"n"`
	From    string    `json:"from"`
	To      string    `json:"to"`
	By      string    `json:"by"`
	At      time.Time `json:"at"`
	Version string    `json:"version,omitempty"` // the document's when suggested
	State   string    `json:"state"`
	// SettledBy and SettledAt are who accepted or declined it, and when;
	// Commit is what accepting it committed.
	SettledBy string     `json:"settledBy,omitempty"`
	SettledAt *time.Time `json:"settledAt,omitempty"`
	Commit    string     `json:"commit,omitempty"`
}

// Pending is the suggestion waiting to be accepted or declined, or nil.
func (c *Conversation) Pending() *Suggestion {
	for i := len(c.Suggestions) - 1; i >= 0; i-- {
		if c.Suggestions[i].State == SuggestOpen {
			return &c.Suggestions[i]
		}
	}
	return nil
}

// Session is an agent's session for a conversation, by harness.
type Session struct {
	Harness string `json:"harness"`
	ID      string `json:"id"`
}

// Model is a model chosen for a conversation's turns, as the harness it
// was chosen on names it.
type Model struct {
	Harness string    `json:"harness"`
	Model   string    `json:"model"`
	By      string    `json:"by"`
	At      time.Time `json:"at"`
}

// Agent is who the agent answering a conversation is in its turns; ws is
// who says it couldn't.
const (
	Agent = "agent"
	WS    = "ws"
)

// NeedsAnswer is whether the agent should answer: it's open, and the
// latest word is yours, asking or asking for a change (or replying in a
// note: a note on its own needs nothing). A quiet turn of yours isn't
// that word: it's read when the agent's next asked something.
func (c *Conversation) NeedsAnswer() bool {
	if c.Status != Open || len(c.Turns) == 0 || c.WithCrew() {
		return false
	}
	if t := c.asked(); t.By != You && !(t.By == WS && t.Asks) {
		// What was said while it answered, which it hasn't read, is to
		// answer (but for quiet turns).
		if c.missedAsking() {
			return true
		}
		// An outline with blocks still to write is its agent's to go on with.
		return c.Kind == Cell && c.NextBlock() != nil
	}
	if p := c.Pending(); p != nil && p.By == You && !p.At.Before(c.Last().At) {
		return false // your own edit, yours to accept
	}
	return c.Kind != Note || len(c.Turns) > 1
}

// asked is the latest turn that counts when deciding whether the agent
// should answer: the latest, but for quiet ones (spoken), and in a cell,
// the one before ws saying the agent couldn't fill it. A cell that failed
// still wants filling, when it's tried again; a conversation that failed
// waits for you to say more.
func (c *Conversation) asked() Turn {
	i := c.spokenAt()
	for c.Kind == Cell && i > 0 && c.Turns[i].By == WS {
		i--
	}
	if i < 0 {
		return Turn{}
	}
	return c.Turns[i]
}

// Last is the latest turn.
func (c *Conversation) Last() Turn {
	if len(c.Turns) == 0 {
		return Turn{}
	}
	return c.Turns[len(c.Turns)-1]
}

// spokenAt is where the latest turn is that says whose turn it is: the
// latest but for quiet ones (a card you settled that needs nothing of the
// agent yet), which the agent reads when it's next asked something. -1
// with no turns.
func (c *Conversation) spokenAt() int {
	i := len(c.Turns) - 1
	for i > 0 && c.Turns[i].Quiet {
		i--
	}
	return i
}

// Missed is what was said to the agent while it was answering last,
// after what it answered was written and before its reply (its turn's
// Seen): your turns, quiet or not, and ws asking it something. It hasn't
// read them: they're told it with what's said next. None for a turn that
// doesn't say what it saw.
func (c *Conversation) Missed() []Turn {
	a := len(c.Turns) - 1
	for a >= 0 && c.Turns[a].By != Agent {
		a--
	}
	if a < 0 || c.Turns[a].Seen <= 0 || c.Turns[a].Seen > a {
		return nil
	}
	var out []Turn
	for _, t := range c.Turns[c.Turns[a].Seen:a] {
		if t.By == You || (t.By == WS && t.Asks) {
			out = append(out, t)
		}
	}
	return out
}

// missedAsking is whether something Missed asks for an answer: a turn
// that isn't quiet.
func (c *Conversation) missedAsking() bool {
	for _, t := range c.Missed() {
		if !t.Quiet {
			return true
		}
	}
	return false
}

// spoken is the turn at spokenAt.
func (c *Conversation) spoken() Turn {
	if i := c.spokenAt(); i >= 0 {
		return c.Turns[i]
	}
	return Turn{}
}

// WaitingOnYou is whether an open conversation's latest word isn't yours:
// someone answered, and it's your turn. A quiet turn of yours (a card
// settled) doesn't change that, and something you said while it answered,
// which it hasn't read, makes it the agent's. A thread crew have isn't, until they've
// changed the document for it (or finished without saying so). Nor is an
// ask kept after its answer, which you read before you kept it, until
// it's answered again.
func (c *Conversation) WaitingOnYou() bool {
	if c.Status != Open || c.Answering || c.WithCrew() || c.missedAsking() {
		return false
	}
	last := c.spoken()
	if c.Kept != nil && !last.At.After(*c.Kept) {
		return false
	}
	if c.Sent != nil && c.Sent.Finished {
		return true
	}
	if last.By == You || last.Asks {
		return false
	}
	// One that's only begun isn't, but for one ws began with something
	// for you to accept (an automation's proposal). An automation's, its
	// words ws's, waits on you while it has something open, and no longer.
	if len(c.Turns) <= 1 && (last.By != WS || len(c.Open()) == 0) {
		return false
	}
	if last.By == WS && strings.HasPrefix(c.Via, "automation:") {
		return len(c.Open()) > 0
	}
	// What the agent's latest reply proposed, all settled since (a
	// mission made, a decision kept): what it asked of you is done.
	return !c.settledAllWith(c.spokenAt())
}

// settledAllWith says whether the agent's turn i came with proposals, and
// every one of them is settled now (accepted, declined, or superseded by
// a later one). A proposal comes with the reply it was made in: after the
// agent's turn before it, and not after it.
func (c *Conversation) settledAllWith(i int) bool {
	if i < 0 || c.Turns[i].By != Agent {
		return false
	}
	var after time.Time
	for j := i - 1; j >= 0; j-- {
		if c.Turns[j].By == Agent {
			after = c.Turns[j].At
			break
		}
	}
	with := 0
	for _, p := range c.Proposals {
		if !p.At.After(after) || p.At.After(c.Turns[i].At) {
			continue
		}
		if p.State == ProposalOpen {
			return false
		}
		with++
	}
	return with > 0
}

// line is a line of a conversation's file.
type line struct {
	Opened     *Conversation    `json:"opened,omitempty"`
	Turn       *Turn            `json:"turn,omitempty"`
	Status     *statusLine      `json:"status,omitempty"`
	Session    *Session         `json:"session,omitempty"`
	Model      *Model           `json:"model,omitempty"`
	Suggest    *Suggestion      `json:"suggest,omitempty"`
	Suggestion *settled         `json:"suggestion,omitempty"`
	Sent       *Sent            `json:"sent,omitempty"`
	Draft      *Draft           `json:"draft,omitempty"`
	Drafted    *drafted         `json:"drafted,omitempty"`
	Summary    *summaryLine     `json:"summary,omitempty"`
	Propose    *Proposal        `json:"propose,omitempty"`
	Proposal   *proposalSettled `json:"proposal,omitempty"`
	Items      *itemsDone       `json:"items,omitempty"`
	Settle     *findingSettled  `json:"settle,omitempty"`
	Placed     *time.Time       `json:"placed,omitempty"`
	Kept       *keptLine        `json:"kept,omitempty"`
	Lifted     *Capsule         `json:"lifted,omitempty"`
	Away       *awayLine        `json:"away,omitempty"`
}

// keptLine is an ask kept: Ephemeral cleared.
type keptLine struct {
	By string    `json:"by"`
	At time.Time `json:"at"`
}

// summaryLine is the agent naming and summing up a conversation.
type summaryLine struct {
	Title   string    `json:"title,omitempty"`
	Summary string    `json:"summary,omitempty"`
	At      time.Time `json:"at"`
}

// drafted is a drafted decision recorded (as Decision) or skipped.
type drafted struct {
	State    string    `json:"state"`
	Decision string    `json:"decision,omitempty"`
	By       string    `json:"by"`
	At       time.Time `json:"at"`
}

// settled is a suggestion accepted or declined.
type settled struct {
	N      int       `json:"n"`
	State  string    `json:"state"`
	By     string    `json:"by"`
	At     time.Time `json:"at"`
	Commit string    `json:"commit,omitempty"`
}

type statusLine struct {
	Status string    `json:"status"`
	By     string    `json:"by"`
	At     time.Time `json:"at"`
}

// Store keeps one mission's conversations.
type Store struct {
	Missions mission.Store
	Slug     string
}

// For is a mission's conversations.
func For(root, slug string) Store { return Store{Missions: mission.Store{Root: root}, Slug: slug} }

// ForWorkspace is the workspace's own conversations, before any mission.
func ForWorkspace(root string) Store { return Store{Missions: mission.Store{Root: root}} }

// Workspace is whether these are the workspace's conversations.
func (s Store) Workspace() bool { return s.Slug == "" }

// dir is where runs answering these conversations write what they print.
func (s Store) dir() string {
	if s.Workspace() {
		return filepath.Join(s.Missions.Root, ".ws", "conversations")
	}
	return filepath.Join(s.Missions.Dir(), s.Slug, "conversations")
}

var idShape = regexp.MustCompile(`^t([1-9][0-9]*)$`)
var workspaceShape = regexp.MustCompile(`^w([1-9][0-9]*)$`)

// IsRef reports whether ref names a mission's conversation (t3).
func IsRef(ref string) bool { return idShape.MatchString(ref) }

// IsWorkspaceRef reports whether ref names a workspace conversation (w3).
func IsWorkspaceRef(ref string) bool { return workspaceShape.MatchString(ref) }

// isID is whether id names one of these conversations.
func (s Store) isID(id string) bool {
	if s.Workspace() {
		return IsWorkspaceRef(id)
	}
	return IsRef(id)
}

func (s Store) prefix() string {
	if s.Workspace() {
		return "w"
	}
	return "t"
}

// ErrNotFound is the answer for a conversation that isn't there.
// ErrNotFound is doc.ErrNotFound: a version not kept is not found too.
var ErrNotFound = doc.ErrNotFound

// Load reads one conversation.
func (s Store) Load(id string) (*Conversation, error) {
	if !s.isID(id) {
		if s.Workspace() {
			return nil, fmt.Errorf("%w: %q isn't a workspace conversation (they're named w1, w2, …)", ErrNotFound, id)
		}
		return nil, fmt.Errorf("%w: %q isn't a thread (they're named t1, t2, …)", ErrNotFound, id)
	}
	var c *Conversation
	err := s.read(func(tx *store.Tx) error {
		var err error
		c, err = s.load(tx, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.workOut(c)
	return c, nil
}

// crewFinished is whether the crew a thread was sent to have ended, or
// reported done, since it was sent (or aren't there).
func (s Store) crewFinished(sent *Sent) bool {
	m, err := s.Missions.Load(s.Slug)
	if err != nil {
		return false
	}
	for _, c := range m.Crew {
		if c.ID == sent.Crew {
			return c.Ended != nil || (c.Status == mission.CrewDone && c.StatusAt != nil && !c.StatusAt.Before(sent.At))
		}
	}
	return true
}

// Send hands open threads to crew to change the document for: the thread
// agent stays out of them, and they're crew's until crew say they've
// changed it (Turn.Changed), or finish. Only the user sends them.
func (s Store) Send(ids []string, crew, by string, at time.Time) error {
	if by != You {
		return errors.New("only the user sends a thread to crew")
	}
	err := s.write(func(tx *store.Tx) error {
		var cs []*Conversation
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			c, err := s.load(tx, id)
			if err != nil {
				return err
			}
			if c.Status != Open {
				return fmt.Errorf("%s is resolved: reopen it to send it", id)
			}
			cs = append(cs, c)
		}
		for _, c := range cs {
			if err := s.add(tx, c, line{Sent: &Sent{Crew: crew, By: by, At: at}}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.Missions.AppendLog(s.Slug, mission.Event{Time: at, Kind: mission.EventThread, Text: strings.Join(ids, ", ") + " sent to " + crew})
}

// IDs is the conversations' ids, oldest first, without reading them.
func (s Store) IDs() []string { return s.ids() }

// List reads the conversations, oldest first.
func (s Store) List() ([]*Conversation, error) { return s.list(`1`) }

// OnDoc is the threads on a document, oldest first: not what's asked
// about it (a Talk about it).
func (s Store) OnDoc(ref string) ([]*Conversation, error) {
	return s.list(`c.doc = ? AND c.kind IN (?, ?, ?)`, ref, Ask, Change, Note)
}

// Talks are the conversations that aren't on a document or a page (Talk),
// oldest first: not the asks that aren't kept.
func (s Store) Talks() ([]*Conversation, error) {
	return s.list(`c.kind = ? AND c.ephemeral = 0`, Talk)
}

// OnPages are the cells and passes of every page, oldest first.
func (s Store) OnPages() ([]*Conversation, error) {
	return s.list(`c.page != '' AND c.kind IN (?, ?)`, Cell, Pass)
}

// ThreadsChanged is when a thread on a document last changed (opened,
// answered, resolved), or zero with none.
func (s Store) ThreadsChanged() time.Time {
	return s.latest(`doc != '' AND kind IN (?, ?, ?)`, Ask, Change, Note)
}

// Changed is when any of these conversations last changed, or zero with
// none: a line added, a run started or ended. An ask that isn't kept
// doesn't count.
func (s Store) Changed() time.Time { return s.latest(`ephemeral = 0`) }

func num(id string) int {
	if id == "" {
		return 0
	}
	n, _ := strconv.Atoi(id[1:])
	return n
}

// NewThread is a thread to open on a document.
type NewThread struct {
	Kind   string
	Anchor Anchor
	By     string
	Body   string
}

func validKind(k string) bool {
	return slices.Contains(Kinds, k)
}

// OpenThread opens a thread on a document, with its first message.
func (s Store) OpenThread(n NewThread, at time.Time) (*Conversation, error) {
	body := strings.TrimSpace(n.Body)
	switch {
	case !validKind(n.Kind):
		return nil, fmt.Errorf("a thread is %s, not %q", strings.Join(Kinds, ", "), n.Kind)
	case body == "":
		return nil, errors.New("say something")
	case n.Anchor.Doc == "" || n.Anchor.Line < 1:
		return nil, errors.New("a thread is on a document's text: its ref and lines")
	}
	c, err := s.open(&Conversation{Kind: n.Kind, Anchor: n.Anchor, By: n.By, At: at, Status: Open}, Turn{By: n.By, At: at, Body: body, Version: n.Anchor.Version})
	if err != nil {
		return nil, err
	}
	return c, s.log(c, n.By, fmt.Sprintf("%s %s on %s: %s", c.ID, kindWords[c.Kind], c.Anchor.Path, oneLine(body)), at)
}

// DocThread is a thread you open on a document as you read it: the lines
// you highlighted (EndLine 0 for one), the text as it reads there, and
// what you said of it, at the version you read.
type DocThread struct {
	Version       string
	Line, EndLine int
	Quote         Quote
	Kind          string
	Body          string
	// Suggestion is what you'd have the highlighted text read instead: an
	// edit, when the text is in the markdown once as it reads, or else
	// said in the thread, for its agent to make one of. A suggestion is
	// always a Change.
	Suggestion *string
	// From, with Suggestion, is the markdown it replaces, when that isn't
	// the quote as it reads (an edit made in the editor, whole lines of
	// it): it must be in the document once.
	From string
}

// ErrStale is a thread opened on, or a document approved at, a version
// that isn't the document's any more.
var ErrStale = errors.New("the document has changed since it was read: reload it, and comment on it as it is now")

const (
	quoteMax   = 4000 // runes of quoted text kept
	contextMax = 64   // runes kept either side of it
)

// OpenOnDoc opens a thread on a document as it is now (its ref and text),
// keeping the version it's on: ErrStale if that isn't the one you read.
func (s Store) OpenOnDoc(ref string, text []byte, t DocThread, by string, at time.Time) (*Conversation, error) {
	if t.Version != doc.Version(text) {
		return nil, ErrStale
	}
	lines := strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n"), "\n")
	if t.EndLine == 0 {
		t.EndLine = t.Line
	}
	if t.Line < 1 || t.EndLine < t.Line || t.EndLine > len(lines) {
		return nil, fmt.Errorf("lines %d–%d aren't in the document (it has %d)", t.Line, t.EndLine, len(lines))
	}
	if _, err := s.KeepVersion(text); err != nil {
		return nil, err
	}
	_, path, _ := strings.Cut(ref, ":")
	q := Quote{Exact: clipRunes(t.Quote.Exact, quoteMax), Prefix: lastRunes(t.Quote.Prefix, contextMax), Suffix: clipRunes(t.Quote.Suffix, contextMax)}
	from := q.Exact
	if t.From != "" {
		from = t.From
	}
	edit := t.Suggestion != nil && from != "" && strings.Count(string(text), from) == 1
	if t.Suggestion != nil {
		t.Kind = Change
		if strings.TrimSpace(t.Body) == "" {
			t.Body = "This should read differently."
		}
		if !edit {
			t.Body = strings.TrimSpace(t.Body) + "\n\nIt should read: " + *t.Suggestion
		}
	}
	c, err := s.OpenThread(NewThread{
		Kind: t.Kind, By: by, Body: t.Body,
		Anchor: Anchor{Doc: ref, Path: path, Quote: &q, Line: t.Line, EndLine: t.EndLine, Version: t.Version, Excerpt: lines[t.Line-1 : t.EndLine]},
	}, at)
	if err == nil && edit {
		c, err = s.Suggest(c.ID, Suggestion{From: from, To: *t.Suggestion, By: by, At: at}, text)
	}
	return c, err
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func lastRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// open stores a new conversation, numbered next, with its first turn.
func (s Store) open(c *Conversation, first Turn) (*Conversation, error) {
	err := s.write(func(tx *store.Tx) error {
		next, err := s.next(tx)
		if err != nil {
			return err
		}
		c.ID = s.prefix() + strconv.Itoa(next)
		c.What = whatOf(c)
		c.Turns = []Turn{}
		if err := s.add(tx, c, line{Opened: c}); err != nil {
			return err
		}
		first.N = 1
		return s.add(tx, c, line{Turn: &first})
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

// OpenTalk starts a conversation that isn't on a document (about the
// workspace, or the mission), with your first message.
func (s Store) OpenTalk(by, body string, at time.Time) (*Conversation, error) {
	return s.OpenTalkVia("", Turn{By: by, At: at, Body: body})
}

// OpenTalkVia is OpenTalk for a conversation started from a plugin (via),
// which worded your first message (first: who it's by, when, what it
// says, and what that names, pinned, so what's named says the
// conversation mentions it from the start). It's yours all the same, and
// says where it came from.
func (s Store) OpenTalkVia(via string, first Turn) (*Conversation, error) {
	return s.OpenTalkChained(via, nil, first)
}

// OpenTalkChained is OpenTalkVia for one an automation starts: chain is
// the automations that led to it, which its events carry.
func (s Store) OpenTalkChained(via string, chain []string, first Turn) (*Conversation, error) {
	first.Body = strings.TrimSpace(first.Body)
	if first.Body == "" {
		return nil, errors.New("say something")
	}
	c, err := s.open(&Conversation{Kind: Talk, By: first.By, Via: via, Chain: chain, At: first.At, Status: Open}, first)
	if err != nil {
		return nil, err
	}
	what := c.ID + " talk"
	if via != "" {
		what += ", started from the " + via + " plugin"
	}
	return c, s.log(c, first.By, what+": "+oneLine(first.Body), first.At)
}

// OpenCell starts a cell of a page: what you asked for is its first
// message, and its agent's replies are what fills it. Cells are the
// workspace's.
//
// goal says what it's for, when it's something the page asks of itself (a
// check, a cold read): such a cell is about the whole page, so is out of
// date when the page changes.
//
// holds is what the cell already holds, when it's made around words of
// yours you want changed: its agent starts from them, and they're kept to
// put back.
//
// A goal of "ask" is a question asked at that place (>?): it's answered
// beside the page, and nothing is written into the page until you ask for
// that.
//
// finding is the finding of a pass it's a question about, asked from the
// pass's tab: it's a question whatever its goal.
func (s Store) OpenCell(page, prompt, goal, holds, finding string, at time.Time) (*Conversation, error) {
	if !s.Workspace() {
		return nil, errors.New("a page's cells are the workspace's")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, errors.New("say what the cell should hold")
	}
	if finding != "" {
		goal = GoalAsk
	}
	if !cellGoal.MatchString(goal) {
		return nil, fmt.Errorf("a cell's goal is a word, not %q", goal)
	}
	return s.open(&Conversation{Kind: Cell, Goal: goal, Anchor: Anchor{Page: page, Finding: finding}, By: You, At: at, Status: Open}, Turn{By: You, At: at, Body: prompt, Edited: strings.TrimSpace(holds), Ask: goal == GoalAsk})
}

// GoalAsk is a cell that began as a question.
const GoalAsk = "ask"

// GoalPlan is something asked of a page as a whole, not at a place in it:
// its agent plans what's to be added, and where (outline.go).
const GoalPlan = "plan"

var cellGoal = regexp.MustCompile(`^[a-z]{0,20}$`)

// OnPage is a page's cells, oldest first.
func (s Store) OnPage(page string) []*Conversation {
	out, _ := s.list(`c.page = ? AND c.kind = ?`, page, Cell)
	return out
}

// AboutPR is the conversations about a pull request (api#89), oldest
// first: asks about it that nobody's kept aren't among them.
func (s Store) AboutPR(ref string) []*Conversation {
	out, _ := s.list(`c.pr = ? AND c.kind = ? AND c.ephemeral = 0`, ref, Talk)
	return out
}

// GoalBrief is a mission's briefing conversation: its agent turns the
// request into a brief and capsules, for you to confirm.
const GoalBrief = "brief"

// OpenBriefing starts a mission's briefing conversation, your request its
// first message.
func (s Store) OpenBriefing(request string, at time.Time) (*Conversation, error) {
	if s.Workspace() {
		return nil, errors.New("briefing is a mission's")
	}
	request = strings.TrimSpace(request)
	if request == "" {
		return nil, errors.New("say what the mission is")
	}
	c, err := s.open(&Conversation{Kind: Talk, Goal: GoalBrief, By: You, At: at, Status: Open}, Turn{By: You, At: at, Body: request})
	if err != nil {
		return nil, err
	}
	return c, s.log(c, You, fmt.Sprintf("%s talk: %s", c.ID, oneLine(request)), at)
}

// Briefing is the mission's open briefing conversation, the latest, or
// nil.
func (s Store) Briefing() *Conversation {
	all, _ := s.list(`c.goal = ? AND c.status = ?`, GoalBrief, Open)
	if len(all) == 0 {
		return nil
	}
	return all[len(all)-1]
}

// Planner is the conversation a mission's planner is in: its briefing
// conversation, carried through the mission once it's briefed (the
// latest, open or put away), or nil when it was briefed without one.
func (s Store) Planner() *Conversation {
	all, _ := s.list(`c.goal = ?`, GoalBrief)
	if len(all) == 0 {
		return nil
	}
	return all[len(all)-1]
}

// OpenPlanner starts the planner's conversation for a mission briefed
// without one, with what's asked of it first: by ws, or by you.
func (s Store) OpenPlanner(by, body string, at time.Time) (*Conversation, error) {
	if s.Workspace() {
		return nil, errors.New("the planner is a mission's")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, errors.New("say what the planner's asked")
	}
	c, err := s.open(&Conversation{Kind: Talk, Goal: GoalBrief, By: by, At: at, Status: Open}, Turn{By: by, At: at, Body: body, Asks: by == WS})
	if err != nil {
		return nil, err
	}
	return c, s.log(c, by, fmt.Sprintf("%s %s: %s", c.ID, by, oneLine(body)), at)
}

// AskAgent is ws asking a conversation's agent something itself: said as
// ws's turn, which it answers as it would yours, taking the conversation
// up again if it was put away.
func (s Store) AskAgent(id, body string, at time.Time) (*Conversation, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, errors.New("say what the agent's asked")
	}
	c, err := s.update(id, func(tx *store.Tx, c *Conversation) error {
		if c.Status == Resolved {
			if err := s.add(tx, c, line{Status: &statusLine{Status: Open, By: WS, At: at}}); err != nil {
				return err
			}
		}
		t := Turn{N: len(c.Turns) + 1, By: WS, At: at, Body: body, Asks: true}
		return s.add(tx, c, line{Turn: &t})
	})
	if err != nil {
		return nil, err
	}
	return c, s.log(c, WS, id+" ws: "+oneLine(body), at)
}

// SetSummary records the agent naming a conversation and summing it up
// (either may be "", leaving it as it was).
func (s Store) SetSummary(id, title, summary string, at time.Time) error {
	title, summary = strings.TrimSpace(title), strings.TrimSpace(summary)
	if title == "" && summary == "" {
		return nil
	}
	_, err := s.update(id, func(tx *store.Tx, c *Conversation) error {
		return s.add(tx, c, line{Summary: &summaryLine{Title: title, Summary: summary, At: at}})
	})
	return err
}

// RunPath is where a run answering a conversation writes what it prints,
// as it goes, for its page to show. Its folder is made, for the run.
// RunFile is where a run answering a conversation writes what it's doing
// (RunPath), without making its folder: for a look at it.
func (s Store) RunFile(id string) string { return filepath.Join(s.dir(), id+".run") }

func (s Store) RunPath(id string) string {
	_ = os.MkdirAll(s.dir(), 0o755) // writing the run says if its folder can't be made
	return filepath.Join(s.dir(), id+".run")
}

// ClearRun removes what a run answering a conversation printed, and what
// it was given: it's only what the run's doing while it does it, so the
// next one starts with none (rather than the page showing a turn that's
// done as the next under way).
func (s Store) ClearRun(id string) {
	os.Remove(filepath.Join(s.dir(), id+".run"))
	os.Remove(s.givenPath(id))
}

// givenPath is where what a guide's run was given is kept, beside what
// it prints.
func (s Store) givenPath(id string) string { return filepath.Join(s.dir(), id+".given.json") }

// SetGuideGiven keeps what a guide's run was given while it runs.
func (s Store) SetGuideGiven(id string, g GuideGiven) error {
	data, err := json.Marshal(g)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.givenPath(id), data, 0o644)
}

// GuideGiven is what a guide's run was given, while it runs; nil before
// it's begun, and once it's done.
func (s Store) GuideGiven(id string) *GuideGiven {
	data, err := os.ReadFile(s.givenPath(id))
	if err != nil {
		return nil
	}
	var g GuideGiven
	if json.Unmarshal(data, &g) != nil {
		return nil
	}
	return &g
}

// Updated is when a conversation last changed: a line added to it, a run
// starting or ending, its output.
func (s Store) Updated(id string) time.Time {
	latest := s.latest(`id = ?`, id)
	if info, err := os.Stat(filepath.Join(s.dir(), id+".run")); err == nil && info.ModTime().After(latest) {
		latest = info.ModTime()
	}
	return latest
}

var kindWords = map[string]string{Ask: "asked", Change: "asked for a change", Note: "noted"}

// Say adds a turn to an open conversation (or reopens one you reply to).
func (s Store) Say(id string, t Turn) (*Conversation, error) {
	t.Body = strings.TrimSpace(t.Body)
	if t.Body == "" {
		return nil, errors.New("say something")
	}
	c, err := s.update(id, func(tx *store.Tx, c *Conversation) error {
		if c.Status == Resolved && t.By != You {
			return fmt.Errorf("%s is resolved: only the user reopens it", id)
		}
		t.N = len(c.Turns) + 1
		if err := s.add(tx, c, line{Turn: &t}); err != nil {
			return err
		}
		if t.By == You {
			return nil
		}
		return s.event(tx, c, events.Event{Kind: events.ConversationAnswered, By: byOf(t.By), At: t.At, Text: clipLine(oneLine(t.Body), 200)})
	})
	if err != nil {
		return nil, err
	}
	body := oneLine(t.Body)
	if t.Changed {
		body = "changed the document: " + body
	}
	text := id + " " + t.By + ": " + body
	if t.Commit != "" {
		text = id + " " + t.By + " (" + shortSHA(t.Commit) + "): " + body
	}
	return c, s.log(c, t.By, text, t.At)
}

// ErrNotYours is the answer for crew resolving or reopening a thread.
var ErrNotYours = errors.New("only the user resolves or reopens a thread")

// TakeUp is you saying something in a conversation that isn't on a
// document, taking it up again if it was put away: a resolved one is
// opened first.
func (s Store) TakeUp(id string, t Turn) (*Conversation, error) {
	c, err := s.Load(id)
	if err != nil {
		return nil, err
	}
	if c.Status == Resolved {
		if _, err := s.SetStatus(id, t.By, Open, t.At); err != nil {
			return nil, err
		}
	}
	return s.Say(id, t)
}

// SetStatus resolves or reopens a conversation. Only the user does.
func (s Store) SetStatus(id, by, status string, at time.Time) (*Conversation, error) {
	if by != You {
		return nil, ErrNotYours
	}
	if status != Open && status != Resolved {
		return nil, fmt.Errorf("a thread is open or resolved, not %q", status)
	}
	c, err := s.update(id, func(tx *store.Tx, c *Conversation) error {
		if c.Status == status {
			return nil
		}
		return s.add(tx, c, line{Status: &statusLine{Status: status, By: by, At: at}})
	})
	if err != nil {
		return nil, err
	}
	words := map[string]string{Open: "reopened", Resolved: "resolved"}[status]
	return c, s.log(c, by, id+" "+words, at)
}

// log records a change to the mission's log, which is also how open pages
// hear of it.
func (s Store) log(c *Conversation, by, text string, at time.Time) error {
	if s.Workspace() || c.Ephemeral {
		return nil // no mission to log to, or nothing to keep
	}
	crew := by
	if by == You || by == "agent" {
		crew = ""
	}
	return s.Missions.AppendLog(s.Slug, mission.Event{Time: at, Kind: mission.EventThread, Crew: crew, Text: text})
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 140 {
		s = s[:139] + "…"
	}
	return s
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// versions are where this store's documents' versions are kept.
func (s Store) versions() doc.Versions {
	return doc.Versions{Root: s.Missions.Root, Scope: doc.Scope{Mission: s.Slug}}
}

// KeepVersion keeps a document's text by its version, once, so what's
// changed since can be shown.
func (s Store) KeepVersion(text []byte) (string, error) { return s.versions().Keep(text) }

// VersionText is a document's text at a kept version.
func (s Store) VersionText(version string) ([]byte, error) { return s.versions().Text(version) }

// SetSession records the agent session a conversation's turns resume.
func (s Store) SetSession(id string, sess Session) error {
	_, err := s.update(id, func(tx *store.Tx, c *Conversation) error { return s.add(tx, c, line{Session: &sess}) })
	return err
}

// SetModel records the model its next turns run on, over its role's; a
// model of "" goes back to the role's.
func (s Store) SetModel(id string, m Model) error {
	_, err := s.update(id, func(tx *store.Tx, c *Conversation) error { return s.add(tx, c, line{Model: &m}) })
	return err
}

// answerFor is how long an answering run counts as underway without
// finishing: longer than any turn may be given ([conversation]
// turn_minutes, at most config.MaxConversationTurnMinutes).
const answerFor = (config.MaxConversationTurnMinutes + 10) * time.Minute

// StartAnswering marks an agent answering a conversation, by its process.
// It's false if one already is: that run answers what's new when it's
// done. A mark left by a run that died, or long ago, is taken over. Marked
// afresh, what the last run printed goes: none of it is this answer's.
func (s Store) StartAnswering(id string, pid int) (bool, error) {
	ok, fresh := false, false
	err := s.write(func(tx *store.Tx) error {
		var was int
		var since string
		err := tx.QueryRow(`SELECT answering, answering_at FROM conversations WHERE scope = ? AND id = ?`, s.Slug, id).Scan(&was, &since)
		if errors.Is(err, sql.ErrNoRows) {
			return s.notFound(id)
		}
		if err != nil {
			return err
		}
		if was == pid { // marked for it by whoever started it
			ok = true
			return nil
		}
		if answeringNow(was, since) {
			return nil
		}
		ok, fresh = true, true
		return s.setAnswering(tx, id, pid)
	})
	if err == nil && fresh {
		s.ClearRun(id)
	}
	return ok, err
}

// EndAnswering marks the agent done answering.
func (s Store) EndAnswering(id string) {
	if err := s.write(func(tx *store.Tx) error { return s.setAnswering(tx, id, 0) }); err != nil {
		log.Printf("conversation %s: still says it's being answered: %v", id, err)
	}
}

// EndAnsweringBy ends every answering mark process pid holds, in any
// scope, and says how many there were. A daemon starting calls it with its
// own pid: one restarted in place (exec) keeps the pid of the one before,
// whose marks would otherwise count as live until they ran out, though no
// run of this process is answering yet.
func EndAnsweringBy(root string, pid int) (int, error) {
	n := 0
	s := For(root, "")
	err := s.write(func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT scope, id FROM conversations WHERE answering = ?`, pid)
		if err != nil {
			return err
		}
		type held struct{ scope, id string }
		var marks []held
		for rows.Next() {
			var h held
			if err := rows.Scan(&h.scope, &h.id); err != nil {
				rows.Close()
				return err
			}
			marks = append(marks, h)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, h := range marks {
			if err := For(root, h.scope).setAnswering(tx, h.id, 0); err != nil {
				return err
			}
		}
		n = len(marks)
		return nil
	})
	return n, err
}

// setAnswering marks a conversation answered by pid (0: by nothing), now.
func (s Store) setAnswering(tx *store.Tx, id string, pid int) error {
	at := stampOf(time.Now())
	since := at
	if pid == 0 {
		since = ""
	}
	if _, err := tx.Exec(`UPDATE conversations SET answering = ?, answering_at = ?, changed = ? WHERE scope = ? AND id = ?`, pid, since, at, s.Slug, id); err != nil {
		return err
	}
	tx.Changed("conversation:"+s.Slug+"/"+id, "workspace")
	if !s.Workspace() {
		tx.Changed("mission:" + s.Slug)
	}
	return nil
}

// ErrNotOnce is the answer for a suggestion whose text isn't in the
// document exactly once.
var ErrNotOnce = errors.New("the text to replace isn't in the document exactly once")

// Suggest records an edit suggested in an open thread, replacing any
// suggestion still open. text is the document now: From must appear in it
// exactly once.
func (s Store) Suggest(id string, sg Suggestion, text []byte) (*Conversation, error) {
	if sg.From == "" {
		return nil, errors.New("say what text to replace")
	}
	if sg.From == sg.To {
		return nil, errors.New("the suggestion changes nothing")
	}
	if n := strings.Count(string(text), sg.From); n != 1 {
		if n == 0 {
			return nil, fmt.Errorf("%w: it isn't there. Quote the markdown exactly as it is in the file, line breaks and all", ErrNotOnce)
		}
		return nil, fmt.Errorf("%w: it's there %d times. Quote enough around it to say which", ErrNotOnce, n)
	}
	c, err := s.update(id, func(tx *store.Tx, c *Conversation) error {
		if c.Status != Open {
			return fmt.Errorf("%s is resolved: reopen it to suggest something", id)
		}
		sg.N, sg.State, sg.Version = len(c.Suggestions)+1, SuggestOpen, doc.Version(text)
		return s.add(tx, c, line{Suggest: &sg})
	})
	if err != nil {
		return nil, err
	}
	return c, s.log(c, sg.By, fmt.Sprintf("%s %s suggests: %s", id, sg.By, oneLine(sg.To)), sg.At)
}

// SettleSuggestion records the open suggestion accepted (with the commit
// that made it, if any) or declined. Only the user does.
func (s Store) SettleSuggestion(id, by, state, commit string, at time.Time) (*Conversation, error) {
	if by != You {
		return nil, errors.New("only the user accepts or declines a suggestion")
	}
	if state != SuggestAccepted && state != SuggestDeclined {
		return nil, fmt.Errorf("a suggestion is accepted or declined, not %q", state)
	}
	c, err := s.update(id, func(tx *store.Tx, c *Conversation) error {
		p := c.Pending()
		if p == nil {
			return fmt.Errorf("%w: %s has no suggestion waiting", ErrNotFound, id)
		}
		return s.add(tx, c, line{Suggestion: &settled{N: p.N, State: state, By: by, At: at, Commit: commit}})
	})
	if err != nil {
		return nil, err
	}
	text := id + " suggestion " + state
	if commit != "" {
		text += " (" + shortSHA(commit) + ")"
	}
	return c, s.log(c, by, text, at)
}

// ShouldDraft is whether a resolved thread is worth drafting a decision
// from: something was said in it besides yours, or an edit accepted, and
// nothing's been drafted yet.
func (c *Conversation) ShouldDraft() bool {
	if c.Status != Resolved || c.Draft != nil {
		return false
	}
	for _, t := range c.Turns {
		if t.By != You {
			return true
		}
	}
	for _, sg := range c.Suggestions {
		if sg.State == SuggestAccepted {
			return true
		}
	}
	return false
}

// DraftDecision records a decision drafted from a resolved thread, for you
// to record or skip.
func (s Store) DraftDecision(id string, d Draft) error {
	d.What, d.Why = strings.TrimSpace(d.What), strings.TrimSpace(d.Why)
	if d.What == "" {
		return errors.New("say what was decided")
	}
	d.State = DraftOpen
	_, err := s.update(id, func(tx *store.Tx, c *Conversation) error { return s.add(tx, c, line{Draft: &d}) })
	return err
}

// SettleDraft records a thread's decision recorded (as decision, d3) or
// skipped. Only the user does.
func (s Store) SettleDraft(id, state, decision, by string, at time.Time) (*Conversation, error) {
	if by != You {
		return nil, errors.New("only the user records or skips a drafted decision")
	}
	if state != DraftRecorded && state != DraftSkipped {
		return nil, fmt.Errorf("a drafted decision is recorded or skipped, not %q", state)
	}
	return s.update(id, func(tx *store.Tx, c *Conversation) error {
		if c.Draft != nil && c.Draft.State != DraftOpen {
			return fmt.Errorf("what was decided in %s is %s already", id, c.Draft.State)
		}
		return s.add(tx, c, line{Drafted: &drafted{State: state, Decision: decision, By: by, At: at}})
	})
}

// event keeps e as one of the workspace's events, of conversation c, in
// tx: what this ws did, in its conversations. An ask that isn't kept
// makes none.
func (s Store) event(tx *store.Tx, c *Conversation, e events.Event) error {
	if c.Ephemeral {
		return nil
	}
	e.Conversation, e.Mission = c.ID, s.Slug
	if len(c.Chain) > 0 {
		e.Chain = append([]string{}, c.Chain...)
	}
	if e.Title == "" {
		e.Title = c.Title
	}
	_, err := events.Append(tx, e)
	return err
}

// byOf is who said something in a conversation, as an event says it.
func byOf(by string) string {
	switch by {
	case "", You:
		return events.ByYou
	case Agent, WS:
		return events.ByWS
	}
	if name, ok := strings.CutPrefix(by, "automation:"); ok && name != "" {
		return events.ByWorkflow(name)
	}
	return events.ByCrew(by)
}

// clipLine is s cut to at most n bytes, at a rune.
func clipLine(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}
