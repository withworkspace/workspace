package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/withworkspace/workspace/internal/nav"
)

// Code navigation in a review: GET …/nav/names, …/nav/definition and
// …/nav/references, under a capsule and under a pull request. A file is
// read at a commit the diff gave the client, from the capsule's or the
// clone's git directory (internal/nav).

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
