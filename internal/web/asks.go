package web

import (
	"net/http"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
)

// Asking from an ask bar (web/src/ask/): POST /api/asks with what's asked
// about (a document's ref, and the passage picked out of it) and the
// question. It's the workspace's conversation from the start, answered as
// any is: followed at GET /api/conversations/{id}, and said to again at
// .../say. Unless it's kept (.../keep), it isn't listed, and the daemon
// sweeps it an hour after its last turn (conversation/ask.go).
func (s *Server) ask(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Doc     string              `json:"doc"`
		Quote   *conversation.Quote `json:"quote,omitempty"`
		Section string              `json:"section,omitempty"`
		Body    string              `json:"body"`
	}
	if !readBody(w, r, &b) {
		return
	}
	c, err := s.App.Ask(ops.Asked{Doc: b.Doc, Quote: b.Quote, Section: b.Section, Body: b.Body}, ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusUnprocessableEntity)
		return
	}
	s.answerTalk(w, conversation.ForWorkspace(s.Root), c.ID, nil)
}

// keepAsk is you keeping an ask (Keep as a conversation): it's listed in
// Conversations from then on, about its document.
func (s *Server) keepAsk(w http.ResponseWriter, r *http.Request) {
	s.withTalkStore(w, r, func(store conversation.Store) {
		id := r.PathValue("id")
		if _, err := talkDetail(store, id); err != nil {
			s.answerTalk(w, store, id, err)
			return
		}
		_, err := s.App.KeepAsk(scope(r), id, ops.AsYou)
		s.answerTalk(w, store, id, err)
	})
}
