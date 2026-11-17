package web

import (
	"errors"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/withworkspace/workspace/internal/review"
)

// A capsule's files, for the file explorer: GET .../tree, .../file?path=,
// .../raw?path= (an image, to show) and .../summary (the Work tab's).

func (s *Server) tree(w http.ResponseWriter, r *http.Request) {
	dir, baseRef, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	t, err := review.ReadTree(dir, baseRef)
	if err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	dir, baseRef, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	p := r.URL.Query().Get("path")
	if !validPath(p) {
		jsonError(w, http.StatusBadRequest, "need a path in the capsule")
		return
	}
	v, err := review.ReadFile(dir, baseRef, p)
	switch {
	case errors.Is(err, review.ErrOutside):
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// imageTypes are the files raw serves: images, for the explorer to show.
var imageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".svg": "image/svg+xml", ".ico": "image/x-icon", ".avif": "image/avif",
}

func (s *Server) raw(w http.ResponseWriter, r *http.Request) {
	dir, _, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	p := r.URL.Query().Get("path")
	kind, image := imageTypes[strings.ToLower(path.Ext(p))]
	if !validPath(p) || !image {
		jsonError(w, http.StatusBadRequest, "only images are served as they are")
		return
	}
	full, err := review.ResolvePath(dir, p)
	if err != nil {
		jsonError(w, http.StatusNotFound, "no such image in the capsule")
		return
	}
	f, err := os.Open(full)
	if err != nil {
		jsonError(w, http.StatusNotFound, "no such image in the capsule")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() || info.Size() > review.FileMax*4 {
		jsonError(w, http.StatusUnprocessableEntity, "not an image to show")
		return
	}
	w.Header().Set("Content-Type", kind)
	// An SVG opened on its own runs nothing.
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
	http.ServeContent(w, r, "", info.ModTime(), f)
}
