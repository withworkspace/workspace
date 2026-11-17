package web

import (
	"net/http"

	"github.com/withworkspace/workspace/internal/review"
)

// summary is GET .../summary: a capsule in brief, for the Work tab.
func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	dir, baseRef, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	sum, err := review.Summarize(dir, baseRef)
	if err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sum)
}
