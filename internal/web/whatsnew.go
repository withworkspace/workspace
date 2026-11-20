package web

import (
	"context"
	"net/http"
	"net/url"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/whatsnew"
)

// What's new: the few lines ws says of each release, once after an
// upgrade, in a dialog over whatever page is open, and on Settings' About
// (internal/whatsnew, ops/whatsnew.go).

// WhatsNewing is what's new, saying it's been seen, and its pictures.
type WhatsNewing interface {
	WhatsNew() ops.WhatsNew
	SeenWhatsNew() error
	WhatsNewPicture(ctx context.Context, name string) (string, error)
}

// WhatsNewInfo is GET /api/whats-new.
type WhatsNewInfo struct {
	// Version is the ws that's running, and Release whether it's a
	// release's build. A build of a clone has nothing unseen, and its
	// entries are all its build found, unreleased ones too.
	Version string `json:"version"`
	Release bool   `json:"release"`
	// Unseen is what's new since the release last seen in this workspace,
	// and Entries everything this ws says is new up to itself: newest
	// first, each.
	Unseen  []WhatsNewEntry `json:"unseen"`
	Entries []WhatsNewEntry `json:"entries"`
	// Seen is the release whose news was last seen in this workspace.
	Seen string `json:"seen,omitempty"`
	// Missing is whether some of what's unseen is from too many releases
	// ago for this ws to hold: it's on Releases.
	Missing  bool   `json:"missing"`
	Releases string `json:"releases,omitempty"`
}

// WhatsNewEntry is one thing that's new.
type WhatsNewEntry struct {
	Slug string `json:"slug"`
	// Version is the release it's in: "" for one in no release yet.
	Version string `json:"version,omitempty"`
	Title   string `json:"title"`
	// Body is a sentence or two, in markdown.
	Body string `json:"body"`
	// Link is a path in the web UI to see it at.
	Link string `json:"link,omitempty"`
	// Picture is where its picture is served from: none where there's no
	// picture, or it can't be fetched.
	Picture string `json:"picture,omitempty"`
}

func whatsNewEntries(es []whatsnew.Entry, pictures bool) []WhatsNewEntry {
	out := make([]WhatsNewEntry, 0, len(es))
	for _, e := range es {
		entry := WhatsNewEntry{Slug: e.Slug, Version: e.Version, Title: e.Title, Body: e.Body, Link: e.Link}
		if pictures && e.Picture != "" && e.Version != "" {
			entry.Picture = "/api/whats-new/pictures/" + url.PathEscape(e.Picture)
		}
		out = append(out, entry)
	}
	return out
}

func (s *Server) whatsNew(w http.ResponseWriter, r *http.Request) {
	n := s.App.WhatsNew()
	writeJSON(w, http.StatusOK, WhatsNewInfo{
		Version: n.Version, Release: n.Release,
		Unseen:  whatsNewEntries(n.Unseen, n.Pictures),
		Entries: whatsNewEntries(n.Entries, n.Pictures),
		Seen:    n.Seen,
		Missing: n.Missing, Releases: n.Releases,
	})
}

// seenWhatsNew is POST /api/whats-new/seen: what's new in the ws that's
// running has been seen, so it isn't said again.
func (s *Server) seenWhatsNew(w http.ResponseWriter, r *http.Request) {
	if err := s.App.SeenWhatsNew(); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// whatsNewPicture is an entry's picture, fetched from its release on GitHub
// the first time and kept: the page's own policy loads pictures from here
// alone.
func (s *Server) whatsNewPicture(w http.ResponseWriter, r *http.Request) {
	path, err := s.App.WhatsNewPicture(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, err, http.StatusBadGateway)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, path)
}
