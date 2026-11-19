package web

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// Conversations with an agent about the workspace (w1, w2, …): the
// Conversations page lists them, and each has a chat view. What you say is
// stored at once and the agent answers in a run of its own (OnTalk); the
// page sees the run's calls as it goes, from its run file.

// Talk is a conversation in a list: GET /api/workspace has them all.
type Talk struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	Opening   string    `json:"opening"` // its first message, clipped
	Summary   string    `json:"summary,omitempty"`
	Status    string    `json:"status"`
	Answering bool      `json:"answering,omitempty"`
	Waiting   bool      `json:"waiting,omitempty"`  // the agent replied: your turn
	Became    string    `json:"became,omitempty"`   // the mission it became
	Proposed  int       `json:"proposed,omitempty"` // proposals waiting for you
	PR        string    `json:"pr,omitempty"`       // the pull request it's about (api#89)
	Doc       string    `json:"doc,omitempty"`      // the document it's about, an ask kept (records:notes/x.md)
	Section   string    `json:"section,omitempty"`  // the section it was asked of, if a passage was
	Quote     string    `json:"quote,omitempty"`    // the passage, clipped
	At        time.Time `json:"at"`
	Updated   time.Time `json:"updated"`
}

// TalkDetail is GET /api/conversations/{id}: the conversation, and while
// the agent's answering, what it's done so far.
type TalkDetail struct {
	*conversation.Conversation
	Run *TalkRun `json:"run,omitempty"`
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
	for i := len(list) - 1; i >= 0; i-- {
		c := list[i]
		if c.Kind != conversation.Talk {
			continue
		}
		t := Talk{ID: c.ID, Title: c.Title, Summary: c.Summary, Status: c.Status, Answering: c.Answering && c.Status == conversation.Open,
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
	c, err := store.Load(id)
	if err != nil {
		return nil, err
	}
	if c.Kind != conversation.Talk {
		return nil, fmt.Errorf("%w: %s is a thread on a document, not a conversation", conversation.ErrNotFound, id)
	}
	// An artifact proposed is run from its frame (proposalFrame), not sent
	// whole with every load of the conversation: its size says what it is.
	for i := range c.Proposals {
		if c.Proposals[i].Kind == conversation.ProposeArtifact {
			c.Proposals[i].HTML = ""
		}
	}
	d := &TalkDetail{Conversation: c}
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
		_, err := s.App.SayInTalk(scope(r), id, b.Body, ops.AsYou)
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
		jsonError(w, http.StatusNotFound, "proposals are numbered 1, 2, …")
		return
	}
	if b.Action != "accept" && b.Action != "decline" {
		jsonError(w, http.StatusBadRequest, "action is accept or decline")
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
