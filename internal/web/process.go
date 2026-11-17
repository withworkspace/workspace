package web

import (
	"net/http"

	"github.com/withworkspace/workspace/internal/ops"
)

// The workspace's process on the Settings page: PROCESS.md as it's
// written, where it is, and whether it's as committed. The planner and the
// orchestrator read it as it is; what ws holds itself is a guarantee, on
// the same page.

// Processing is the process, as the page shows it.
type Processing interface {
	Process() (ops.ProcessView, error)
}

// ProcessInfo is GET /api/process.
type ProcessInfo struct {
	// File is where it is; Name that from the workspace's root.
	File   string `json:"file"`
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
	Text   string `json:"text"`
	// Commit is the config repo's last commit of it, Uncommitted when the
	// file isn't as committed.
	Commit      string `json:"commit,omitempty"`
	Uncommitted bool   `json:"uncommitted,omitempty"`
}

// ProcessOf is the process as the API serves it.
func ProcessOf(v ops.ProcessView) ProcessInfo {
	return ProcessInfo{File: v.File, Name: v.Name, Exists: v.Exists, Text: v.Text, Commit: v.Commit, Uncommitted: v.Uncommitted}
}

func (s *Server) process(w http.ResponseWriter, r *http.Request) {
	v, err := s.App.Process()
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ProcessOf(v))
}
