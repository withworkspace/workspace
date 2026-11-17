package web

import (
	"errors"
	"net/http"
	"time"
)

// The orchestrator's page: how it's set up, what it's doing now, and its
// runs, each with the prompt it was given, what it said and the calls it
// made.

// OrchestratorInfo is GET /api/orchestrator.
type OrchestratorInfo struct {
	Enabled bool   `json:"enabled"`
	Harness string `json:"harness"`
	Model   string `json:"model,omitempty"`
	// Problem is why it can't run as set up (its harness missing), if so.
	Problem string `json:"problem,omitempty"`
	// Dispatch is "auto" (dispatched crew start at once) or "ask" (they're
	// queued for you), and MaxDispatches how many crew it may start in a
	// row without you.
	Dispatch      string `json:"dispatch"`
	MaxDispatches int    `json:"maxDispatches"`
	// Runs are the latest runs across missions, newest first.
	Runs []OrchestratorRun `json:"runs"`
}

// OrchestratorRun is one run, in brief.
type OrchestratorRun struct {
	Mission      string    `json:"mission"`
	MissionTitle string    `json:"missionTitle"`
	ID           string    `json:"id"`
	Started      time.Time `json:"started"`
	Ended        time.Time `json:"ended"`
	For          string    `json:"for"`
	Trigger      string    `json:"trigger"`
	Harness      string    `json:"harness"`
	Model        string    `json:"model,omitempty"`
	Retry        bool      `json:"retry,omitempty"`
	Acted        bool      `json:"acted"`
	Error        string    `json:"error,omitempty"`
	Said         string    `json:"said,omitempty"` // the start of what it said
	Calls        int       `json:"calls"`
	Refused      int       `json:"refused"`
}

// OrchestratorRunDetail is GET /api/missions/{slug}/orchestrator/{id}.
type OrchestratorRunDetail struct {
	OrchestratorRun
	Prompt string             `json:"prompt"`
	Output string             `json:"output"`
	Log    []OrchestratorCall `json:"log"`
}

// OrchestratorCall is one call a run made.
type OrchestratorCall struct {
	Tool    string `json:"tool"`
	Input   string `json:"input"`
	Result  string `json:"result,omitempty"`
	Error   bool   `json:"error,omitempty"`
	Refused bool   `json:"refused,omitempty"`
}

func (s *Server) orchestrator(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, OrchestratorOf(s.App))
}

func (s *Server) orchestratorRun(w http.ResponseWriter, r *http.Request) {
	d, err := OrchestratorRunOf(s.App, r.PathValue("slug"), r.PathValue("id"))
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, "no such run")
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, d)
	}
}
