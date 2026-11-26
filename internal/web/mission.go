package web

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/ops"
)

// missionFloor is how often a mission's page that's open is built again
// when nothing it's built from has moved: a headline says how long things
// have taken.
var missionFloor = 30 * time.Second

// mission is the mission's page: everything about it. While a page
// follows it, its hub keeps it current (built when what it's built from
// moves, before the page is told), so it's served as kept, built by
// nobody's read. A write from a page forgets it, so that page reads what
// it wrote.
func (s *Server) mission(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	key := missionTopic + slug
	if s.missions != nil && s.missions.follows(slug) {
		if data, version, ok := s.views.kept(key); ok {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set(versionHeader, strconv.FormatInt(version, 10))
			w.Write(data) //nolint:errcheck // a page gone mid-answer is nothing to say
			return
		}
	}
	data, err := s.views.mission(s, slug)
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, "no mission "+slug)
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(versionHeader, strconv.FormatInt(s.views.versionOf(key), 10))
		w.Write(data)
	}
}

// missionChange is how a mission's page has changed since was (what it
// was last served as): built now, it's told to read it again only if it's
// other than it was.
func (s *Server) missionChange(slug string, was any) (op, now any) {
	data, err := s.views.mission(s, slug)
	if err != nil {
		return reread, nil // and the next change is from nothing
	}
	if prev, ok := was.([]byte); ok && bytes.Equal(prev, data) {
		return nil, data
	}
	return reread, data
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
