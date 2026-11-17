package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/withworkspace/workspace/internal/ops"
)

// Talking to a crew from its page: POST .../crew/{id}/say sends it a
// message (typed in, or resuming it with it if it's ended), and
// .../answer answers the prompt on its screen (a permission, a question)
// or interrupts its turn.

// SayRequest is a message for crew.
type SayRequest struct {
	Text string `json:"text"`
}

// AnswerRequest answers what crew are asking: Permission (allow, always or
// deny), Option (one of its question's options, by label), Text (your own
// words, for a question), or Interrupt (stop its turn: Escape).
type AnswerRequest struct {
	Permission string `json:"permission,omitempty"`
	Option     string `json:"option,omitempty"`
	Text       string `json:"text,omitempty"`
	Interrupt  bool   `json:"interrupt,omitempty"`
}

// ErrNotOnScreen is an answer refused because crew's screen isn't showing
// the prompt it would answer: nothing was typed.
var ErrNotOnScreen = ops.ErrNotOnScreen

func (s *Server) crewSay(w http.ResponseWriter, r *http.Request) {
	var req SayRequest
	if !readBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		jsonError(w, http.StatusBadRequest, "the message is empty")
		return
	}
	msg, err := s.App.Crew().Say(r.PathValue("slug"), r.PathValue("id"), req.Text)
	agentReply(w, msg, err)
}

func (s *Server) crewAnswer(w http.ResponseWriter, r *http.Request) {
	var req AnswerRequest
	if !readBody(w, r, &req) {
		return
	}
	switch req.Permission {
	case "", "allow", "always", "deny":
	default:
		jsonError(w, http.StatusBadRequest, "permission is allow, always or deny")
		return
	}
	if req.Permission == "" && req.Option == "" && strings.TrimSpace(req.Text) == "" && !req.Interrupt {
		jsonError(w, http.StatusBadRequest, "nothing to answer with")
		return
	}
	msg, err := s.App.Crew().Answer(r.PathValue("slug"), r.PathValue("id"), ops.CrewAnswer(req))
	agentReply(w, msg, err)
}

func agentReply(w http.ResponseWriter, msg string, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrNotOnScreen):
		jsonError(w, http.StatusConflict, err.Error())
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeJSON(w, http.StatusOK, ActResult{Message: msg})
	}
}

// PromptView is the menu on crew's screen, as read: what it asks (Title),
// what it's about (Context: the command, the file), and its Options. It's
// what a prompt card shows when the transcript hasn't the step yet.
type PromptView struct {
	Title   string   `json:"title"`
	Context []string `json:"context"`
	Options []string `json:"options"`
}

func (s *Server) crewPrompt(w http.ResponseWriter, r *http.Request) {
	p, ok := s.App.Crew().Prompt(r.PathValue("slug"), r.PathValue("id"))
	if !ok {
		jsonError(w, http.StatusNotFound, "no prompt on its screen")
		return
	}
	writeJSON(w, http.StatusOK, PromptView(p))
}
