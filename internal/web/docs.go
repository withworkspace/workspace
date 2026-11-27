package web

import (
	"errors"
	"fmt"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/ops"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Editing a mission's documents: the draft you're writing, kept with the
// mission as you go (not in the document's file, where crew's next commit
// would take it as theirs), and saving it (SaveDoc), merged with whatever
// changed meanwhile.

// draftReply is a document's draft, or none.
type draftReply struct {
	Draft *doc.Draft `json:"draft"`
}

func (s *Server) getDraft(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		jsonError(w, http.StatusBadRequest, "missing document ref")
		return
	}
	d, err := s.drafts(r).Get(ref)
	switch {
	case errors.Is(err, doc.ErrNotFound):
		jsonError(w, http.StatusNotFound, "no mission "+r.PathValue("slug"))
		return
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, draftReply{Draft: d})
}

func (s *Server) putDraft(w http.ResponseWriter, r *http.Request) {
	var d doc.Draft
	if !readBody(w, r, &d) {
		return
	}
	if d.Ref == "" || d.Base == "" {
		jsonError(w, http.StatusBadRequest, "missing document ref or base version")
		return
	}
	d.At = time.Now().UTC()
	err := s.App.SaveDraft(scope(r), d)
	switch {
	case errors.Is(err, doc.ErrNotFound):
		jsonError(w, http.StatusNotFound, "no mission "+r.PathValue("slug"))
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, draftReply{Draft: &d})
	}
}

func (s *Server) dropDraft(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		jsonError(w, http.StatusBadRequest, "missing document ref")
		return
	}
	switch err := s.App.DropDraft(scope(r), ref); {
	case errors.Is(err, doc.ErrNotFound):
		jsonError(w, http.StatusNotFound, "no mission "+r.PathValue("slug"))
		return
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, draftReply{})
}

// saveDoc saves an edited document. An edit that clashes with changes made
// meanwhile comes back (200) with its Conflict: nothing's written.
func (s *Server) saveDoc(w http.ResponseWriter, r *http.Request) {
	var d DocSave
	if !readBody(w, r, &d) {
		return
	}
	if d.Ref == "" || d.Base == "" {
		jsonError(w, http.StatusBadRequest, "missing document ref or base version")
		return
	}
	if strings.TrimSpace(d.Text) == "" {
		jsonError(w, http.StatusBadRequest, "cannot save an empty document")
		return
	}
	saved, err := s.App.SaveDocument(r.PathValue("slug"), d.Ref, d.Base, d.Text, d.Message)
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case err != nil:
		jsonError(w, http.StatusConflict, err.Error())
	default:
		writeJSON(w, http.StatusOK, DocSavedOf(saved))
	}
}

// scope is whose documents a request is about: the mission it names, or
// the workspace's for a file in no mission.
func scope(r *http.Request) doc.Scope { return doc.Scope{Mission: r.PathValue("slug")} }

// drafts are the drafts of the documents a request is about.
func (s *Server) drafts(r *http.Request) doc.Drafts { return doc.Drafts{Root: s.Root, Scope: scope(r)} }

// A document being edited is followed by its editor (doc:<slug>/<ref>), told
// when its file's written, by crew committing to it, say: the editor reads
// it then, to see whether it's another version than it's editing.

// docHub follows documents being edited: each one's file looked at every
// poll while it's followed (a stat), found where it is as it's first
// followed.
func (s *Server) docHub() *keyedHub {
	var mu sync.Mutex
	paths := map[string]string{}
	pathOf := func(key string) string {
		mu.Lock()
		path, ok := paths[key]
		mu.Unlock()
		if ok {
			return path
		}
		slug, ref, _ := strings.Cut(key, "/")
		var d *ops.DocText
		var err error
		if slug == "" {
			d, err = s.App.Doc(ref)
		} else {
			d, err = s.App.Artifact(slug, ref)
		}
		if err == nil && d != nil {
			path = d.Path
		}
		mu.Lock()
		paths[key] = path
		mu.Unlock()
		return path
	}
	h := newKeyedHub(docTopic, s.Poll, func(key string) string {
		path := pathOf(key)
		info, err := os.Stat(path)
		if path == "" || err != nil {
			return ""
		}
		return fmt.Sprintf("%d %d", info.Size(), info.ModTime().UnixNano())
	})
	h.pathOf = pathOf
	h.gone = func(key string) {
		mu.Lock()
		delete(paths, key)
		mu.Unlock()
	}
	return h
}
