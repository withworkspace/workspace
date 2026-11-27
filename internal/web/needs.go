package web

import (
	"encoding/json"
	"net/http"

	"github.com/withworkspace/workspace/internal/ops"
)

// announceNeeds answers a page that has needs new to it and would tell
// the desktop: which of them no page has told of yet. It's asked while
// the page is out of sight, so the telling is once however many pages
// are open, and once across a reload.
func (s *Server) announceNeeds(w http.ResponseWriter, r *http.Request) {
	var req NeedsAnnounce
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "could not read request: "+err.Error())
		return
	}
	fresh, err := ops.Announce(s.Root, req.Keys, req.Holding)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, NeedsAnnounced{Fresh: fresh})
}
