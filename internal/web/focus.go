package web

import (
	"net/http"

	"github.com/withworkspace/workspace/internal/ops"
)

// focus is POST /api/focus: pin one thing ({"pin": name}), unpin one
// ({"unpin": name}), or put the pins in order ({"order": [names…]}), as
// you. It answers what was done and the pins as they now stand; every
// page's sidebar is told on the workspace's topic.
func (s *Server) focus(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Pin   string   `json:"pin"`
		Unpin string   `json:"unpin"`
		Order []string `json:"order"`
	}
	if !readBody(w, r, &b) {
		return
	}
	var said string
	var err error
	switch {
	case b.Pin != "" && b.Unpin == "" && b.Order == nil:
		var p ops.Pinned
		p, err = s.App.Pin(ops.AsYou, b.Pin)
		said = p.String()
	case b.Unpin != "" && b.Pin == "" && b.Order == nil:
		var u ops.Unpinned
		u, err = s.App.Unpin(ops.AsYou, b.Unpin)
		said = u.String()
	case b.Order != nil && b.Pin == "" && b.Unpin == "":
		err = s.App.ArrangeFocus(ops.AsYou, b.Order)
		said = "Arranged."
	default:
		jsonError(w, http.StatusBadRequest, "say one of pin, unpin or order")
		return
	}
	if err != nil {
		writeError(w, err, http.StatusUnprocessableEntity)
		return
	}
	items, err := s.App.Focus()
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, FocusDone{Said: said, Focus: focusOf(items)})
}

// FocusDone is what POST /api/focus answers: what was done, in a
// sentence, and the pins as they now stand.
type FocusDone struct {
	Said  string     `json:"said"`
	Focus []FocusPin `json:"focus"`
}
