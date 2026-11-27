package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store"
)

// Conversations with an agent about the workspace (w1, w2, …): the
// Conversations page lists them, and each has a chat view. What you say is
// stored at once and the agent answers in a run of its own (OnTalk); the
// page sees the run's calls as it goes, from its run file.

// Talk is a conversation in a list: GET /api/workspace has them all.
type Talk struct {
	ID        string       `json:"id"`
	Title     string       `json:"title,omitempty"`
	Opening   string       `json:"opening"` // its first message, clipped
	Summary   string       `json:"summary,omitempty"`
	Status    ThreadStatus `json:"status"`
	Answering bool         `json:"answering,omitempty"`
	Waiting   bool         `json:"waiting,omitempty"`  // the agent replied: your turn
	Became    string       `json:"became,omitempty"`   // the mission it became
	Proposed  int          `json:"proposed,omitempty"` // proposals waiting for you
	PR        string       `json:"pr,omitempty"`       // the pull request it's about (api#89)
	Doc       string       `json:"doc,omitempty"`      // the document it's about, an ask kept (records:notes/x.md)
	Section   string       `json:"section,omitempty"`  // the section it was asked of, if a passage was
	Quote     string       `json:"quote,omitempty"`    // the passage, clipped
	Thing     string       `json:"thing,omitempty"`    // the plugin's thing it's about, asked from its page (sketch-13)
	At        time.Time    `json:"at"`
	Updated   time.Time    `json:"updated"`
}

// TalkDetail is GET /api/conversations/{id}: the conversation, and while
// the agent's answering, what it's done so far.
type TalkDetail struct {
	*conversation.Conversation
	Run *TalkRun `json:"run,omitempty"`
	// Version is the records' change number it's as of, or later: a change
	// told over the socket as of an earlier one (ConversationOp) is in it
	// already.
	Version int64 `json:"version"`
	// Message says what a write did, when it's worth saying (accepting a
	// proposal); Path is where the conversation is now, when it's moved
	// (its mission renamed).
	Message string `json:"message,omitempty"`
	Path    string `json:"path,omitempty"`
}

// TalkRun is a run answering a conversation, as far as it's got: the
// calls it's made, and the last thing it said.
type TalkRun struct {
	Calls []harness.ToolCall `json:"calls"`
	Said  string             `json:"said,omitempty"`
}

// Talks is the workspace's conversations, newest first.
func Talks(root string) []Talk { return TalksIn(conversation.ForWorkspace(root)) }

// DocsUpdated is when the workspace's threads on documents (files in its
// repos, outside any mission) last changed, or nil with none.
func DocsUpdated(root string) *time.Time {
	latest := conversation.ForWorkspace(root).ThreadsChanged()
	if latest.IsZero() {
		return nil
	}
	return &latest
}

// TalksIn is the conversations in a store that aren't on a document,
// newest first.
func TalksIn(store conversation.Store) []Talk {
	list, _ := store.Talks()
	return talksOf(store, list)
}

// talksOf is talks as a list shows them, newest first.
func talksOf(store conversation.Store, list []*conversation.Conversation) []Talk {
	out := []Talk{}
	for _, c := range slices.Backward(list) {

		if c.Kind != conversation.Talk {
			continue
		}
		t := Talk{ID: c.ID, Title: c.Title, Summary: c.Summary, Status: ThreadStatus(c.Status), Answering: c.Answering && c.Status == conversation.Open,
			Waiting: c.WaitingOnYou(), At: c.At, Updated: store.Updated(c.ID), Became: c.Became(), PR: c.Anchor.PR}
		for _, p := range c.Proposals {
			// Put away, what's left open in it isn't asked of you.
			if p.State == conversation.ProposalOpen && c.Status == conversation.Open {
				t.Proposed++
			}
		}
		if len(c.Turns) > 0 {
			t.Opening = clipRunes(strings.Join(strings.Fields(c.Turns[0].Body), " "), 140)
		}
		if c.What == conversation.WhatThing {
			t.Thing = c.Anchor.Thing
		}
		if a := c.Anchor; c.What == conversation.WhatDoc {
			t.Doc, t.Section = a.Doc, a.Section
			if a.Quote != nil {
				t.Quote = clipRunes(strings.Join(strings.Fields(a.Quote.Exact), " "), 140)
			}
		}
		out = append(out, t)
	}
	return out
}

// talkStore is the conversations a request is about: a mission's
// (/api/missions/{slug}/conversations), or the workspace's.
func (s *Server) talkStore(r *http.Request) (conversation.Store, error) {
	sc := scope(r)
	if sc.IsWorkspace() {
		return conversation.ForWorkspace(s.Root), nil
	}
	if _, err := (mission.Store{Root: s.Root}).Load(sc.Mission); err != nil {
		return conversation.Store{}, fmt.Errorf("%w: no mission %s", conversation.ErrNotFound, sc.Mission)
	}
	return conversation.For(s.Root, sc.Mission), nil
}

// talkStoreOf is mission slug's conversations, or the workspace's own
// for "".
func (s *Server) talkStoreOf(slug string) (conversation.Store, error) {
	if slug == "" {
		return conversation.ForWorkspace(s.Root), nil
	}
	if _, err := (mission.Store{Root: s.Root}).Load(slug); err != nil {
		return conversation.Store{}, fmt.Errorf("%w: no mission %s", conversation.ErrNotFound, slug)
	}
	return conversation.For(s.Root, slug), nil
}

// withTalkStore runs fn with the request's conversations, or answers why
// there aren't any.
func (s *Server) withTalkStore(w http.ResponseWriter, r *http.Request, fn func(conversation.Store)) {
	store, err := s.talkStore(r)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	fn(store)
}

// answerTalk replies with a conversation after a write, or the write's
// error.
func (s *Server) answerTalk(w http.ResponseWriter, store conversation.Store, id string, err error) {
	if err == nil {
		var d *TalkDetail
		if d, err = talkDetail(store, id); err == nil {
			writeJSON(w, http.StatusOK, d)
			return
		}
	}
	switch {
	case errors.Is(err, conversation.ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	default:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	}
}

func talkDetail(store conversation.Store, id string) (*TalkDetail, error) {
	// The records' number is read first: what's read after is as of it,
	// or later.
	var version int64
	if db, err := storeOf(store); err == nil {
		version, _ = db.Seq(context.Background())
	}
	c, err := store.Load(id)
	if err != nil {
		return nil, err
	}
	if c.Kind != conversation.Talk {
		return nil, fmt.Errorf("%w: %s is a thread on a document, not a conversation", conversation.ErrNotFound, id)
	}
	// An artifact proposed is run from its frame (proposalFrame), not sent
	// whole with every load of the conversation: its size says what it is,
	// and its files, for one that's more than its page.
	for i := range c.Proposals {
		if c.Proposals[i].Kind == conversation.ProposeArtifact {
			c.Proposals[i].HTML, c.Proposals[i].Tree = "", ""
		}
	}
	d := &TalkDetail{Conversation: c, Version: version}
	if c.Answering {
		d.Run = &TalkRun{Calls: []harness.ToolCall{}}
		if data, err := os.ReadFile(store.RunPath(id)); err == nil {
			rep := harness.ReadRun(string(data))
			if rep.Calls != nil {
				d.Run.Calls = rep.Calls
			}
			d.Run.Said = rep.Output
		}
	}
	return d, nil
}

func (s *Server) getTalk(w http.ResponseWriter, r *http.Request) {
	s.withTalkStore(w, r, func(store conversation.Store) { s.answerTalk(w, store, r.PathValue("id"), nil) })
}

// startTalk is you starting a conversation, with what you're thinking
// about.
func (s *Server) startTalk(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Body string `json:"body"`
	}
	if !readBody(w, r, &b) {
		return
	}
	s.withTalkStore(w, r, func(store conversation.Store) {
		c, err := s.App.StartTalk(scope(r), b.Body, ops.AsYou)
		if err != nil {
			writeError(w, err, http.StatusUnprocessableEntity)
			return
		}
		s.answerTalk(w, store, c.ID, nil)
	})
}

// sayInTalk is you saying something in a conversation (taking a resolved
// one up again): the agent answers it.
func (s *Server) sayInTalk(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Body string `json:"body"`
		// Given is what a plugin's page gave the agent with it.
		Given *conversation.Given `json:"given,omitempty"`
	}
	if !readBody(w, r, &b) {
		return
	}
	s.withTalkStore(w, r, func(store conversation.Store) {
		id := r.PathValue("id")
		if _, err := talkDetail(store, id); err != nil { // a thread has its own routes
			s.answerTalk(w, store, id, err)
			return
		}
		_, err := s.App.SayInTalkGiven(scope(r), id, b.Body, b.Given, ops.AsYou)
		s.answerTalk(w, store, id, err)
	})
}

// setTalkStatus is you putting a conversation away, or taking it up again.
func (s *Server) setTalkStatus(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Status string `json:"status"` // resolved or open
	}
	if !readBody(w, r, &b) {
		return
	}
	s.withTalkStore(w, r, func(store conversation.Store) {
		id := r.PathValue("id")
		if _, err := talkDetail(store, id); err != nil {
			s.answerTalk(w, store, id, err)
			return
		}
		_, err := s.App.SetTalkStatus(scope(r), id, b.Status, ops.AsYou)
		s.answerTalk(w, store, id, err)
	})
}

// TalkModel is what a conversation's next turns run on: its role's
// harness, and the model, yours (Chosen) or the role's.
type TalkModel struct {
	Harness string `json:"harness"`
	Model   string `json:"model"`
	Chosen  bool   `json:"chosen,omitempty"`
}

func (s *Server) talkModel(w http.ResponseWriter, r *http.Request) {
	s.withTalkStore(w, r, func(store conversation.Store) {
		id := r.PathValue("id")
		if _, err := talkDetail(store, id); err != nil {
			s.answerTalk(w, store, id, err)
			return
		}
		s.answerTalkModel(w, r, store, id)
	})
}

// setTalkModel is you choosing the model a conversation's next turns run
// on: {"model": ""} is its role's again.
func (s *Server) setTalkModel(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Model string `json:"model"`
	}
	if !readBody(w, r, &b) {
		return
	}
	s.withTalkStore(w, r, func(store conversation.Store) {
		id := r.PathValue("id")
		if _, err := talkDetail(store, id); err != nil {
			s.answerTalk(w, store, id, err)
			return
		}
		if _, err := s.App.SetTalkModel(scope(r), id, b.Model, ops.AsYou); err != nil {
			s.answerTalk(w, store, id, err)
			return
		}
		s.answerTalkModel(w, r, store, id)
	})
}

func (s *Server) answerTalkModel(w http.ResponseWriter, r *http.Request, store conversation.Store, id string) {
	p, err := s.App.TalkPick(scope(r), id)
	if err != nil {
		s.answerTalk(w, store, id, err)
		return
	}
	c, err := store.Load(id)
	if err != nil {
		s.answerTalk(w, store, id, err)
		return
	}
	writeJSON(w, http.StatusOK, TalkModel{Harness: p.Harness, Model: p.Model, Chosen: c.Model != nil && c.Model.Harness == p.Harness})
}

// settleProposal is you accepting (with any edits) or declining what the
// agent proposed.
func (s *Server) settleProposal(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Action string `json:"action"` // accept or decline
		What   string `json:"what"`
		Why    string `json:"why"`
		Title  string `json:"title"`
		Brief  string `json:"brief"`
		Text   string `json:"text"`
		Plan   string `json:"plan"`
		Note   string `json:"note"`
		Items  []int  `json:"items"`
	}
	if !readBody(w, r, &b) {
		return
	}
	store, err := s.talkStore(r)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	id := r.PathValue("id")
	if _, err := talkDetail(store, id); err != nil {
		s.answerTalk(w, store, id, err)
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		jsonError(w, http.StatusNotFound, "proposal must be a number: 1, 2, …")
		return
	}
	if b.Action != "accept" && b.Action != "decline" {
		jsonError(w, http.StatusBadRequest, "action must be accept or decline")
		return
	}
	said, after, err := s.App.SettleProposal(store.Slug, id, n, b.Action == "accept", ops.ProposalEdits{What: b.What, Why: b.Why, Title: b.Title, Brief: b.Brief, Text: b.Text, Plan: b.Plan, Note: b.Note, Items: b.Items})
	moved := !store.Workspace() && after != "" && after != store.Slug
	if moved {
		store = conversation.For(s.Root, after)
	}
	if err != nil {
		s.answerTalk(w, store, id, err)
		return
	}
	d, err := talkDetail(store, id)
	if err != nil {
		s.answerTalk(w, store, id, err)
		return
	}
	d.Message = said
	if moved {
		d.Path = "/missions/" + after + "/conversations/" + id
	}
	writeJSON(w, http.StatusOK, d)
}

// clipRunes is s to its first n runes.
func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// storeOf is the records a conversation store's are in.
func storeOf(s conversation.Store) (*store.DB, error) { return store.Dial(s.Missions.Root) }

// ConversationOp is how a conversation a page holds has changed, told over
// the socket (SocketSaid.Conversation) so the page needn't read it again:
// its turns from From on (those before are as the page has them), and the
// rest of it as it is now. A page holding fewer than From turns has missed
// one, and holding it as of a later Version has this already: it reads it
// whole, or drops this.
type ConversationOp struct {
	Version int64               `json:"version"`
	From    int                 `json:"from"`
	Turns   []conversation.Turn `json:"turns"`
	// Conversation is all of it but its turns; Run is its agent's run as
	// far as it's got, none when nobody's answering it.
	Conversation *conversation.Conversation `json:"conversation"`
	Run          *TalkRun                   `json:"run,omitempty"`
}

// conversationChange is how the conversation at key (scope/id) has changed
// since was (a *TalkDetail its followers were told of; nil when they may
// hold anything): op is nil when nothing they'd see has, and reread when
// they're to read it whole (it can't be read here: they're told why when
// they do). now is it as it is.
func conversationChange(root, key string, was any) (op, now any) {
	scope, id, _ := strings.Cut(key, "/")
	d, err := talkDetail(conversation.For(root, scope), id)
	if err != nil {
		return reread, nil
	}
	prev, _ := was.(*TalkDetail)
	if prev == nil {
		return reread, d
	}
	from := 0
	for from < len(prev.Turns) && from < len(d.Turns) && sameJSON(prev.Turns[from], d.Turns[from]) {
		from++
	}
	head, prevHead := *d.Conversation, *prev.Conversation
	head.Turns, prevHead.Turns = nil, nil
	if from == len(prev.Turns) && from == len(d.Turns) && sameJSON(head, prevHead) && sameJSON(d.Run, prev.Run) {
		return nil, d
	}
	return &ConversationOp{Version: d.Version, From: from, Turns: d.Turns[from:], Conversation: &head, Run: d.Run}, d
}

// sameJSON is whether a and b are sent as the same JSON.
func sameJSON(a, b any) bool {
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}
