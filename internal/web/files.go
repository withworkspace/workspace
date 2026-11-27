package web

import (
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/review"
)

// A capsule's files, for the file explorer: GET .../tree, .../file?path=,
// .../raw?path= (an image, to show) and .../summary (the Work tab's).

// unreadTree is a tree that's never been read, and is being: the
// workspace's readsSeen moves when it has.
type unreadTree struct {
	*review.Tree
	Unread bool `json:"unread"`
}

// tree answers at once, as kept (keptReads.soon): read again behind the
// read when the capsule's moved or a while has passed.
func (s *Server) tree(w http.ResponseWriter, r *http.Request) {
	dir, baseRef, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	t, unread, err := s.reads.soon(keyOf("tree", dir, baseRef), dir, func() (any, error) { return review.ReadTree(dir, baseRef) })
	switch {
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	case unread:
		writeJSON(w, http.StatusOK, unreadTree{&review.Tree{BaseRef: baseRef, Files: []review.TreeFile{}}, true})
	default:
		writeJSON(w, http.StatusOK, t)
	}
}

func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	dir, baseRef, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	p := r.URL.Query().Get("path")
	if !validPath(p) {
		jsonError(w, http.StatusBadRequest, "missing path")
		return
	}
	// Kept by the capsule as it is (its commits, what's staged, its
	// base) and the file as it is: what the lines that changed are of.
	read := func() (any, error) { return review.ReadFile(dir, baseRef, p) }
	var v any
	var err error
	if stamp := ops.CapsuleStamp(dir); stamp != "" {
		if as, ok := s.fileAsIs(dir, baseRef, p, stamp); ok {
			writeJSON(w, http.StatusOK, as)
			return
		}
		v, err = s.reads.kept(keyOf("file", dir, baseRef, p, stamp, fileStamp(dir, p)), read)
	} else {
		v, err = read()
	}
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
		jsonError(w, http.StatusBadRequest, "only images can be shown")
		return
	}
	full, err := review.ResolvePath(dir, p)
	if err != nil {
		jsonError(w, http.StatusNotFound, "image not found in capsule")
		return
	}
	f, err := os.Open(full)
	if err != nil {
		jsonError(w, http.StatusNotFound, "image not found in capsule")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() || info.Size() > review.FileMax*4 {
		jsonError(w, http.StatusUnprocessableEntity, "not an image")
		return
	}
	w.Header().Set("Content-Type", kind)
	// An SVG opened on its own runs nothing.
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
	http.ServeContent(w, r, "", info.ModTime(), f)
}

// fileAsIs is a file read with no git, when the capsule's tree is kept
// as the capsule is (stamp: the explorer reads it before it opens a
// file), lists it unchanged, and it's not been written since that tree
// was read: nothing's changed in it since the branch point, so there's
// nothing to mark. false (not listed, changed, written since, or no tree
// kept): read it with git.
func (s *Server) fileAsIs(dir, baseRef, path, stamp string) (*review.FileView, bool) {
	kept, from, ok := s.reads.current(keyOf("tree", dir, baseRef), stamp)
	if !ok {
		return nil, false
	}
	t := kept.(*review.Tree)
	if t.Base == "" || t.Truncated {
		return nil, false
	}
	listed := false
	for _, f := range t.Files {
		if f.Path == path {
			if f.Status != "" {
				return nil, false
			}
			listed = true
			break
		}
	}
	if !listed {
		return nil, false
	}
	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path)))
	if err != nil || !info.ModTime().Before(from) {
		return nil, false
	}
	v, err := review.ReadFileAsIs(dir, path)
	if err != nil {
		return nil, false
	}
	return v, true
}
