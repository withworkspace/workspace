package web

import (
	"errors"
	"net/http"
	"net/url"
	"os"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/ops"
)

// mission is the mission's page: everything about it.
func (s *Server) mission(w http.ResponseWriter, r *http.Request) {
	d, err := s.missionDetail(r.PathValue("slug"), r.URL.Query().Get("github") == "1")
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, "no mission "+r.PathValue("slug"))
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, d)
	}
}

// artifact is a readable artifact's text, by its reference: only the
// mission's own artifacts, so this can't be pointed at any file.
func (s *Server) artifact(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		jsonError(w, http.StatusBadRequest, "need an artifact's ref")
		return
	}
	a, err := s.artifactText(r.PathValue("slug"), ref)
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		if a.Media != "" {
			// An image or a video is shown from its raw route.
			a.Raw = "/api/missions/" + url.PathEscape(r.PathValue("slug")) + "/artifact/raw?ref=" + url.QueryEscape(ref)
		}
		a.Version, a.Annotatable = doc.Version([]byte(a.Text)), annotatable(a)
		// A document you can edit keeps the version you open, to merge
		// your edit with if it changes before you save.
		if a.Editable && a.Markdown {
			s.App.KeepVersion(scope(r), []byte(a.Text))
		}
		writeJSON(w, http.StatusOK, a)
	}
}

// artifactRaw is a mission's artifact that's an image or a video, as it
// is, to show: only the mission's own (as artifact reads them), and only
// those, so this can't be pointed at any file. A video is served in ranges,
// to seek in; an SVG opened on its own runs nothing.
func (s *Server) artifactRaw(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		jsonError(w, http.StatusBadRequest, "need an artifact's ref")
		return
	}
	t, err := s.App.Artifact(r.PathValue("slug"), ref)
	switch {
	case errors.Is(err, ErrNotFound) || errors.Is(err, ops.ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
		return
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case t.Media == "":
		jsonError(w, http.StatusBadRequest, "only images and videos are served as they are")
		return
	}
	f, err := os.Open(t.Path)
	if err != nil {
		jsonError(w, http.StatusNotFound, "it isn't there to show")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		jsonError(w, http.StatusUnprocessableEntity, "it isn't a file to show")
		return
	}
	w.Header().Set("Content-Type", t.Media)
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
	http.ServeContent(w, r, "", info.ModTime(), f)
}

// bundleFile is a file of one of a mission's verification bundles, as it
// is, to show: only a file the bundle's record names, so this can't be
// pointed at any other. A video is served in ranges, to seek in; an SVG
// opened on its own runs nothing.
func (s *Server) bundleFile(w http.ResponseWriter, r *http.Request) {
	path, media, err := s.App.BundleFile(r.PathValue("slug"), r.PathValue("id"), r.PathValue("name"))
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	f, err := os.Open(path)
	if err != nil {
		jsonError(w, http.StatusNotFound, "it isn't there to show")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		jsonError(w, http.StatusUnprocessableEntity, "it isn't a file to show")
		return
	}
	w.Header().Set("Content-Type", media)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
	http.ServeContent(w, r, "", info.ModTime(), f)
}

// act does what a mission's headline asks of you, as far as a page can.
func (s *Server) act(w http.ResponseWriter, r *http.Request) {
	var a ActRequest
	if !readBody(w, r, &a) {
		return
	}
	said, err := ActOn(s.App, r.PathValue("slug"), a)
	switch {
	case errors.Is(err, ErrNotFound) || errors.Is(err, ops.ErrNotFound):
		jsonError(w, http.StatusNotFound, "no mission "+r.PathValue("slug"))
	case err != nil:
		writeError(w, err, http.StatusConflict)
	default:
		res := ActResult{Message: said}
		// Briefing in a conversation: the page goes to the brief's own,
		// which talks in it.
		if a.Verb == "brief" {
			if c := conversation.For(s.Root, r.PathValue("slug")).Briefing(); c != nil {
				res.Path = "/missions/" + r.PathValue("slug") + "/brief"
			}
		}
		writeJSON(w, http.StatusOK, res)
	}
}
