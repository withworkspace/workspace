package web

import (
	"net/http"

	"github.com/withworkspace/workspace/internal/ops"
)

// A mission's decisions, written from its page: one of your own recorded
// (or recorded in place of one that stands), and one that stands
// withdrawn. What a thread or a conversation came to is recorded there
// (threads.go, talk.go).

// DecideRequest is a decision of yours to record on a mission: what was
// decided, why, and the decision it replaces (none: it's new).
type DecideRequest struct {
	What       string `json:"what"`
	Why        string `json:"why,omitempty"`
	Supersedes string `json:"supersedes,omitempty"`
}

// DecideResult says what was recorded, and its name (d4).
type DecideResult struct {
	Message string `json:"message"`
	ID      string `json:"id"`
}

// WithdrawRequest is why a decision is withdrawn.
type WithdrawRequest struct {
	Why string `json:"why"`
}

// decide records a decision of yours on a mission.
func (s *Server) decide(w http.ResponseWriter, r *http.Request) {
	var b DecideRequest
	if !readBody(w, r, &b) {
		return
	}
	d, err := s.App.RecordDecision(r.PathValue("slug"), ops.Deciding{What: b.What, Why: b.Why, Supersedes: b.Supersedes}, ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, DecideResult{Message: d.String(), ID: d.Decision.ID})
}

// withdrawDecision is you taking back a decision that stands, saying why.
func (s *Server) withdrawDecision(w http.ResponseWriter, r *http.Request) {
	var b WithdrawRequest
	if !readBody(w, r, &b) {
		return
	}
	d, err := s.App.WithdrawDecision(r.PathValue("slug"), r.PathValue("id"), b.Why, ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, ActResult{Message: d.String()})
}
