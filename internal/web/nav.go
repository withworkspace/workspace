package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/withworkspace/workspace/internal/nav"
)

// Code navigation in a review and in the explorer: GET …/nav/names,
// …/nav/definition and …/nav/references, under a capsule, a pull request
// and any of the workspace's worktrees. A file is read at a commit the
// client was given (by the diff, or by …/nav/now for the files as they
// are), from the capsule's or the clone's git directory (internal/nav).

// navAt reads what every nav question has: the git directory, the commit
// and the path, or answers for them.
func (s *Server) navAt(w http.ResponseWriter, r *http.Request) (dir, rev, path string, ok bool) {
	if s.Nav == nil {
		jsonError(w, http.StatusNotFound, "no code navigation here")
		return "", "", "", false
	}
	dir, _, ok = s.capsuleDir(w, r)
	if !ok {
		return "", "", "", false
	}
	q := r.URL.Query()
	rev, path = q.Get("rev"), q.Get("path")
	if !fullSHA.MatchString(rev) || !validPath(path) {
		jsonError(w, http.StatusBadRequest, "need rev (a full commit id) and a path")
		return "", "", "", false
	}
	return dir, rev, path, true
}

// NavNow is GET …/nav/now: the commit a worktree's files are as they are
// now, for the explorer to ask its names at (nav.Service.Now).
type NavNow struct {
	Rev string `json:"rev"`
	// Unread: it's never been read, and is being (Rev is ""): the
	// workspace's readsSeen moves when it has been.
	Unread bool `json:"unread,omitempty"`
}

func (s *Server) navNow(w http.ResponseWriter, r *http.Request) {
	if s.Nav == nil {
		jsonError(w, http.StatusNotFound, "no code navigation here")
		return
	}
	dir, _, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	// What the worktree's files are takes git (its status, and a commit
	// of them when they've changed): it's answered as kept, and read
	// again behind every answer (a file edited doesn't move the capsule's
	// stamp), the page told (readsSeen) when that's other than it was
	// given.
	v, unread, err := s.reads.soonWithin(keyOf("nav-now", dir), dir, 0, func() (any, error) { return s.Nav.Now(dir) })
	switch {
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	case unread:
		writeJSON(w, http.StatusOK, NavNow{Unread: true})
	default:
		writeJSON(w, http.StatusOK, NavNow{Rev: v.(string)})
	}
}

func (s *Server) navNames(w http.ResponseWriter, r *http.Request) {
	dir, rev, path, ok := s.navAt(w, r)
	if !ok {
		return
	}
	n, err := s.Nav.Names(dir, rev, path)
	if err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// navPlace reads a nav question about a place: the file, and its line
// and column.
func (s *Server) navPlace(w http.ResponseWriter, r *http.Request) (dir, rev, path string, line, col int, ok bool) {
	dir, rev, path, ok = s.navAt(w, r)
	if !ok {
		return
	}
	line, lerr := strconv.Atoi(r.URL.Query().Get("line"))
	col, cerr := strconv.Atoi(r.URL.Query().Get("col"))
	if lerr != nil || cerr != nil || line < 1 || col < 0 {
		jsonError(w, http.StatusBadRequest, "need a line (from 1) and a col (from 0)")
		return "", "", "", 0, 0, false
	}
	return dir, rev, path, line, col, true
}

func (s *Server) navReferences(w http.ResponseWriter, r *http.Request) {
	dir, rev, path, line, col, ok := s.navPlace(w, r)
	if !ok {
		return
	}
	refs, err := s.Nav.References(dir, rev, path, line, col)
	switch {
	case errors.Is(err, nav.ErrNoName):
		jsonError(w, http.StatusNotFound, err.Error())
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeJSON(w, http.StatusOK, refs)
	}
}

func (s *Server) navDefinition(w http.ResponseWriter, r *http.Request) {
	dir, rev, path, line, col, ok := s.navPlace(w, r)
	if !ok {
		return
	}
	a, err := s.Nav.Definition(dir, rev, path, line, col)
	switch {
	case errors.Is(err, nav.ErrNoName):
		jsonError(w, http.StatusNotFound, err.Error())
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeJSON(w, http.StatusOK, a)
	}
}
