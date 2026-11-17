package web

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/ops"
)

// Threads on a document: comments as in a shared doc, live as soon as
// they're posted. Each is a conversation anchored to the text it's on. The
// routes are under /api/missions/{slug}/threads, with the document's
// artifact ref as ?ref=. The document is read fresh for each write: a new
// thread says the version it was made on, and is refused if that isn't the
// document's any more.
//
// A document outside a mission (any markdown file in a repo) has threads
// too, under /api/docs/threads: the workspace's own conversations, with
// no mission ({slug} is empty). What only a mission has isn't there:
// approving it, sending a thread to crew, recording a decision.

// doc is GET /api/docs?ref=: a document outside a mission.
func (s *Server) doc(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		jsonError(w, http.StatusBadRequest, "need a document's ref")
		return
	}
	a, err := s.docText(ref)
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		a.Version, a.Annotatable = doc.Version([]byte(a.Text)), annotatable(a)
		// One you can edit keeps the version you open, to merge your edit
		// with if it changes before you save.
		if a.Editable && a.Markdown {
			s.App.KeepVersion(doc.Scope{}, []byte(a.Text))
		}
		writeJSON(w, http.StatusOK, a)
	}
}

// docOf reads a document as it is now: a mission's artifact, or (the
// workspace's) a file in a repo.
func (s *Server) docOf(sc doc.Scope, ref string) (*ArtifactContent, error) {
	if sc.IsWorkspace() {
		return s.docText(ref)
	}
	return s.artifactText(sc.Mission, ref)
}

// ThreadState is GET .../threads: the document's version now, and its
// threads, oldest first. Message says what a write did, when it's worth
// saying (accepting a suggestion).
type ThreadState struct {
	Version string                       `json:"version"`
	Threads []*conversation.Conversation `json:"threads"`
	Message string                       `json:"message,omitempty"`
	// Approved is your latest approval of the document, at the version it
	// was then; Stage is the stage it was made for, if it was.
	Approved *conversation.Approval `json:"approved,omitempty"`
	Stage    *DocStage              `json:"stage,omitempty"`
}

// annotatable is whether an artifact is a document to comment on:
// markdown, and not a review (ws writes those).
func annotatable(a *ArtifactContent) bool { return a.Markdown && a.Kind != "review" }

// threadDoc reads the request's document as it is now, or answers for it.
func (s *Server) threadDoc(w http.ResponseWriter, r *http.Request) (conversation.Store, *ArtifactContent, bool) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		jsonError(w, http.StatusBadRequest, "need a document's ref")
		return conversation.Store{}, nil, false
	}
	a, err := s.docOf(scope(r), ref)
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
		return conversation.Store{}, nil, false
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return conversation.Store{}, nil, false
	case !annotatable(a):
		jsonError(w, http.StatusUnprocessableEntity, ref+" isn't a document to comment on: only markdown is")
		return conversation.Store{}, nil, false
	}
	// With no mission, these are the workspace's conversations.
	return conversation.For(s.Root, r.PathValue("slug")), a, true
}

// answerThreads replies with the document's threads after a write, or the
// write's error.
func (s *Server) answerThreads(w http.ResponseWriter, store conversation.Store, a *ArtifactContent, err error) {
	switch {
	case errors.Is(err, conversation.ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
		return
	case errors.Is(err, conversation.ErrStale):
		jsonError(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		writeError(w, err, http.StatusUnprocessableEntity)
		return
	}
	st, err := threadState(store, a, "")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// threadState is where the document's threads stand, and the document.
func threadState(store conversation.Store, a *ArtifactContent, message string) (ThreadState, error) {
	threads, err := store.OnDoc(a.Ref)
	if err != nil {
		return ThreadState{}, err
	}
	if threads == nil {
		threads = []*conversation.Conversation{}
	}
	approved, err := store.Approved(a.Ref)
	if err != nil {
		return ThreadState{}, err
	}
	return ThreadState{Version: doc.Version([]byte(a.Text)), Threads: threads, Message: message, Approved: approved, Stage: a.Stage}, nil
}

// approveDoc is you approving the document as you read it. It's refused
// while a Change thread on it is open.
func (s *Server) approveDoc(w http.ResponseWriter, r *http.Request) {
	store, a, ok := s.threadDoc(w, r)
	var b struct {
		Version string `json:"version"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	said, err := s.App.Approve(scope(r), a.Ref, []byte(a.Text), b.Version, ops.AsYou)
	if err != nil {
		s.answerThreads(w, store, a, err)
		return
	}
	st, err := threadState(store, a, said)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// settleSuggestion is you accepting or declining a thread's suggested edit.
func (s *Server) settleSuggestion(w http.ResponseWriter, r *http.Request) {
	store, a, ok := s.threadDoc(w, r)
	var b struct {
		Action string `json:"action"` // accept or decline
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	id := r.PathValue("id")
	if err := onThisDoc(store, a, id); err != nil {
		s.answerThreads(w, store, a, err)
		return
	}
	switch b.Action {
	case "decline":
		_, err := s.App.DeclineSuggestion(scope(r), id, ops.AsYou)
		s.answerThreads(w, store, a, err)
	case "accept":
		said, err := s.App.AcceptSuggestion(r.PathValue("slug"), id)
		if err != nil {
			s.answerThreads(w, store, a, err)
			return
		}
		// The document has changed: read it again for its version.
		if fresh, err := s.docOf(scope(r), a.Ref); err == nil {
			a = fresh
		}
		st, err := threadState(store, a, said)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, st)
	default:
		jsonError(w, http.StatusBadRequest, "action is accept or decline")
	}
}

// sendThreads is you sending threads on the document to crew: those
// named, or every Change thread open on it that crew don't have.
func (s *Server) sendThreads(w http.ResponseWriter, r *http.Request) {
	store, a, ok := s.threadDoc(w, r)
	var b struct {
		IDs []string `json:"ids"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	for _, id := range b.IDs {
		if err := onThisDoc(store, a, id); err != nil {
			s.answerThreads(w, store, a, err)
			return
		}
	}
	said, err := s.App.SendToCrew(r.PathValue("slug"), a.Ref, b.IDs)
	if err != nil {
		s.answerThreads(w, store, a, err)
		return
	}
	st, err := threadState(store, a, said)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) getThreads(w http.ResponseWriter, r *http.Request) {
	if store, a, ok := s.threadDoc(w, r); ok {
		s.answerThreads(w, store, a, nil)
	}
}

// threadBody is what a page sends to open a thread: conversation.DocThread.
type threadBody struct {
	Version    string             `json:"version"`
	Line       int                `json:"line"`
	EndLine    int                `json:"endLine"`
	Quote      conversation.Quote `json:"quote"`
	Kind       string             `json:"kind"`
	Body       string             `json:"body"`
	Suggestion *string            `json:"suggestion"`
	From       string             `json:"from"`
}

func (s *Server) openThread(w http.ResponseWriter, r *http.Request) {
	store, a, ok := s.threadDoc(w, r)
	var b threadBody
	if !ok || !readBody(w, r, &b) {
		return
	}
	_, err := s.App.OpenThread(scope(r), a.Ref, []byte(a.Text), conversation.DocThread(b), ops.AsYou)
	s.answerThreads(w, store, a, err)
}

// onThisDoc loads a thread, if it's on the request's document.
func onThisDoc(store conversation.Store, a *ArtifactContent, id string) error {
	c, err := store.Load(id)
	if err != nil {
		return err
	}
	if c.Anchor.Doc != a.Ref {
		return fmt.Errorf("%w: %s isn't a thread on %s", conversation.ErrNotFound, id, a.Ref)
	}
	return nil
}

// sayOnThread is you replying in a thread.
func (s *Server) sayOnThread(w http.ResponseWriter, r *http.Request) {
	store, a, ok := s.threadDoc(w, r)
	var b struct {
		Body string `json:"body"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	id := r.PathValue("id")
	if err := onThisDoc(store, a, id); err != nil {
		s.answerThreads(w, store, a, err)
		return
	}
	_, err := s.App.Reply(scope(r), id, []byte(a.Text), b.Body, "", ops.AsYou)
	s.answerThreads(w, store, a, err)
}

// setThreadStatus is you resolving or reopening a thread.
func (s *Server) setThreadStatus(w http.ResponseWriter, r *http.Request) {
	store, a, ok := s.threadDoc(w, r)
	var b struct {
		Status string `json:"status"` // resolved or open
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	id := r.PathValue("id")
	if err := onThisDoc(store, a, id); err != nil {
		s.answerThreads(w, store, a, err)
		return
	}
	_, err := s.App.SetThreadStatus(scope(r), id, b.Status, ops.AsYou)
	s.answerThreads(w, store, a, err)
}

// recordDecision is you recording what was decided in a thread (as its
// agent drafted it, or as you've put it), or skipping its draft. The
// decision is from the thread, and about the document and the stage it
// was made for.
func (s *Server) recordDecision(w http.ResponseWriter, r *http.Request) {
	store, a, ok := s.threadDoc(w, r)
	var b struct {
		Action string `json:"action"` // record or skip
		What   string `json:"what"`
		Why    string `json:"why"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	id, slug := r.PathValue("id"), r.PathValue("slug")
	if err := onThisDoc(store, a, id); err != nil {
		s.answerThreads(w, store, a, err)
		return
	}
	switch b.Action {
	case "skip":
		s.answerThreads(w, store, a, s.App.SkipDraft(scope(r), id, ops.AsYou))
	case "record":
		about := []string{a.Ref}
		if a.Stage != nil {
			about = append(about, fmt.Sprintf("s%d", a.Stage.N))
		}
		said, err := s.App.Decide(slug, id, b.What, b.Why, about, ops.AsYou)
		if err != nil {
			s.answerThreads(w, store, a, err)
			return
		}
		st, err := threadState(store, a, said)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, st)
	default:
		jsonError(w, http.StatusBadRequest, "action is record or skip")
	}
}

// threadVersion is the document's text at a version one of its threads
// was opened or answered on: what's changed since is the difference.
func (s *Server) threadVersion(w http.ResponseWriter, r *http.Request) {
	store, a, ok := s.threadDoc(w, r)
	if !ok {
		return
	}
	v := r.URL.Query().Get("v")
	threads, err := store.OnDoc(a.Ref)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	on := false
	for _, c := range threads {
		on = on || c.Anchor.Version == v
		for _, t := range c.Turns {
			on = on || t.Version == v
		}
	}
	if !on {
		jsonError(w, http.StatusNotFound, "no thread on "+a.Ref+" is on version "+v)
		return
	}
	text, err := store.VersionText(v)
	switch {
	case errors.Is(err, conversation.ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, map[string]string{"text": string(text)})
	}
}
