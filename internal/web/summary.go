package web

import (
	"net/http"

	"github.com/withworkspace/workspace/internal/review"
)

// unreadSummary is a summary that's never been read: with ?kept, one
// nobody's shown yet; without, one being read, and the workspace's
// readsSeen moves when it has.
type unreadSummary struct {
	*review.Summary
	Unread bool `json:"unread"`
}

// summary is GET .../summary: a capsule in brief, for the Work tab. It
// answers at once, as kept (keptReads.soon). With ?kept=1 it's only what's
// kept, and nothing's read for it: a row in a list of capsules says what's
// known of one, and only the capsule that's shown is read.
func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	dir, baseRef, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	key := keyOf("summary", dir, baseRef)
	var sum any
	var unread bool
	var err error
	if r.URL.Query().Get("kept") != "" {
		sum, ok = s.reads.peek(key)
		unread = !ok
	} else {
		sum, unread, err = s.reads.soon(key, dir, func() (any, error) { return review.Summarize(dir, baseRef) })
	}
	switch {
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	case unread:
		writeJSON(w, http.StatusOK, unreadSummary{&review.Summary{BaseRef: baseRef, Commits: []review.Commit{}, Changed: []review.Changed{}}, true})
	default:
		writeJSON(w, http.StatusOK, sum)
	}
}
