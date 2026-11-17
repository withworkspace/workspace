package web

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/review"
)

// Artifacts in the web UI: the list (in the workspace's view, so it
// follows every change), an artifact's page (GET /api/artifacts/{id}), a
// version's source (.../source?v=2), two versions' sources diffed (GET
// /api/artifacts/compare?a=a3.v1&b=a5.v2), and each version run from a
// route of its own, GET /frame/a3/v2, in a sandbox; and an artifact a
// conversation proposed, GET /frame/w9/proposals/2, the same.
//
// An artifact is HTML an agent wrote, run inside the page that controls
// your missions. So it's served from outside /api and the app's routes,
// with a policy of its own: it runs its scripts, but in an origin of its
// own (no cookie, no storage, nothing of the page around it), and nothing
// leaves it (no fetch, no script, font or image from elsewhere, no popup,
// no form, no navigating the page). The web UI frames it with
// sandbox="allow-scripts" and no allow-same-origin; opened alone, the
// policy's sandbox holds it to the same.

// FramePolicy is a version's Content-Security-Policy: sandboxed, with its
// scripts and styles inline, its images, fonts and media in it, and
// framed only by the web UI.
const FramePolicy = "sandbox allow-scripts; default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; " +
	"img-src data: blob:; font-src data:; media-src data: blob:; frame-ancestors 'self'"

// frameHeaders are what every answer on /frame has, in place of the
// guard's (which would refuse to be framed, and run no inline script).
func frameHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", FramePolicy)
	h.Set("X-Frame-Options", "SAMEORIGIN")
}

// frame is GET /frame/{id}/{version}: a version's HTML, as it was
// written, to run in the web UI's sandboxed frame or opened alone.
func (s *Server) frame(w http.ResponseWriter, r *http.Request) {
	frameHeaders(w)
	id, vs := r.PathValue("id"), r.PathValue("version")
	n, ok := 0, false
	if num, cut := strings.CutPrefix(vs, "v"); cut {
		n, ok = artifact.ParseVersion(num)
	}
	if !ok || !artifact.IsRef(id) || strings.Contains(id, ".") {
		framePage(w, http.StatusNotFound, "There's no such version", "A version's frame is /frame/a3/v2: an artifact, then the version.")
		return
	}
	text, e, _, err := s.artifacts().ReadVersion(r.Context(), id, n)
	if err != nil {
		// No such artifact or version, its folder gone, or its file: there's
		// nothing here to run, whichever.
		framePage(w, http.StatusNotFound, "It can't be shown", html.EscapeString(err.Error()))
		return
	}
	// A version never changes, but what a3.v2 names can: a workspace made
	// again in the same folder (just test-workspace) numbers its own from
	// a1, on the same port. So it's asked again each time, and a 304
	// answers when it's the same file.
	h := fnv.New64a()
	h.Write([]byte(e.Folder))
	h.Write(text)
	w.Header().Set("ETag", `"`+strconv.FormatUint(h.Sum64(), 36)+`"`)
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(text))
}

// proposalFrame is GET /frame/w9/proposals/2 (a mission's conversation:
// /frame/missions/{slug}/t3/proposals/2): an artifact a conversation's
// agent proposed, as it proposed it, from the conversation's records. It's
// served as a version is, so it runs in the card's frame as it would once
// added; and it's there whatever became of the proposal: one declined
// isn't an artifact, but it stays in the conversation, with its frame.
func (s *Server) proposalFrame(w http.ResponseWriter, r *http.Request) {
	frameHeaders(w)
	id := r.PathValue("id")
	n, err := strconv.Atoi(r.PathValue("n"))
	store, serr := s.talkStore(r)
	if err != nil || n < 1 || serr != nil || (store.Workspace() && !conversation.IsWorkspaceRef(id)) || (!store.Workspace() && !conversation.IsRef(id)) {
		framePage(w, http.StatusNotFound, "There's no such proposal", "A proposal's frame is /frame/w9/proposals/2: a conversation, then the proposal.")
		return
	}
	c, err := store.Load(id)
	var p *conversation.Proposal
	if err == nil {
		p = c.Proposal(n)
	}
	if p == nil || p.Kind != conversation.ProposeArtifact {
		framePage(w, http.StatusNotFound, "It can't be shown", html.EscapeString(fmt.Sprintf("%s has no artifact proposed as %d.", id, n)))
		return
	}
	// What a proposal names can change as a version's can (a workspace
	// made again numbers its conversations from w1): asked again each
	// time, as a version is.
	h := fnv.New64a()
	h.Write([]byte(store.Slug + "/" + id + "/" + strconv.Itoa(n) + "\n"))
	h.Write([]byte(p.HTML))
	w.Header().Set("ETag", `"`+strconv.FormatUint(h.Sum64(), 36)+`"`)
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, "", time.Time{}, strings.NewReader(p.HTML))
}

// framePage is what a frame shows when there's no version to run: a
// notice, under the frame's own policy so it shows in it.
func framePage(w http.ResponseWriter, status int, title, body string) {
	w.Header().Set("Cache-Control", "no-store")
	page(w, status, title, body)
}

func (s *Server) artifacts() artifact.Store { return s.App.Artifacts() }

// ArtifactsOf is the workspace's artifacts as its view lists them, the
// latest version's newest first, discarded ones too (the list's filter
// shows them).
func ArtifactsOf(st artifact.Store, cfg artifact.Config) []ArtifactInfo {
	out := []ArtifactInfo{}
	list, err := st.List(context.Background())
	if err != nil {
		return out
	}
	now := time.Now()
	for _, e := range list {
		out = append(out, artifactInfo(e, cfg, now))
	}
	return out
}

func artifactInfo(e artifact.Entry, cfg artifact.Config, now time.Time) ArtifactInfo {
	info := ArtifactInfo{ID: e.ID, Title: e.Title, Status: string(e.Status), Gone: e.Gone, Place: e.Place, Folder: e.Folder,
		Where: e.Where(), Made: e.Made, Versions: len(e.Versions), Promoted: e.Promoted, Discarded: e.Discarded,
		InFlight: e.InFlight(), Flying: len(e.Flying()), Erased: e.Erased, Restorable: e.Restorable()}
	if at, ok := cfg.Expires(e); ok {
		info.Expires, info.Expired = &at, !now.Before(at)
	}
	if v, ok := e.Latest(); ok {
		latest := versionOf(v)
		info.Latest = &latest
	}
	seen := map[string]bool{}
	for i := len(e.Versions) - 1; i >= 0; i-- {
		v := e.Versions[i]
		if v.From != "" && !seen[v.From] {
			seen[v.From] = true
			info.From = append(info.From, ArtifactFrom{From: v.From, Title: v.FromTitle})
		}
	}
	return info
}

func versionOf(v artifact.Version) ArtifactVersion {
	return ArtifactVersion{N: v.N, BasedOn: v.BasedOn, At: v.At, By: v.By, From: v.From, Proposal: v.Proposal, FromTitle: v.FromTitle, Note: v.Note, Capsule: v.Capsule, Was: v.Was}
}

// artifactDetail is GET /api/artifacts/{id}: an artifact with every
// version, oldest first, each with its file's size (0 while there's none
// to read).
func (s *Server) artifactDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !artifact.IsRef(id) || strings.Contains(id, ".") {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("%q isn't an artifact: they're named a1, a2, …", id))
		return
	}
	st := s.artifacts()
	e, err := st.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	d := ArtifactDetail{ArtifactInfo: artifactInfo(e, s.App.ArtifactsConfig(), time.Now()), All: []ArtifactVersion{}}
	for _, v := range e.Versions {
		av := versionOf(v)
		av.Size, _ = st.VersionSize(e, v.N)
		d.All = append(d.All, av)
	}
	writeJSON(w, http.StatusOK, d)
}

// An artifact's lifetime, as you: POST /api/artifacts/{id}/promote,
// /discard and /restore, and /api/artifacts/clear for the expired. Each in
// a repo is a commit landed on its default branch, so it may take a few
// seconds.

// artifactDone answers with what was done, or why it wasn't: an error
// that comes after part of it was (promoted, but not recorded on the
// mission) says so itself, and the list shows the rest.
func (s *Server) artifactDone(w http.ResponseWriter, d ops.ArtifactDone, err error) {
	if err != nil {
		writeError(w, err, http.StatusUnprocessableEntity)
		return
	}
	out := ArtifactDone{Artifact: artifactInfo(d.Entry, s.App.ArtifactsConfig(), time.Now()), Landed: d.Landed, Moved: d.Moved, Into: d.Into, Started: d.Started, Recorded: d.Recorded}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) promoteArtifact(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Version int    `json:"version"`
		Into    string `json:"into"`
		Start   string `json:"start"`
		Move    bool   `json:"move"`
	}
	if !readBody(w, r, &b) {
		return
	}
	d, err := s.App.PromoteArtifact(ops.ArtifactPromote{Ref: r.PathValue("id"), Version: b.Version, Into: b.Into, Start: b.Start, Move: b.Move}, ops.AsYou)
	s.artifactDone(w, d, err)
}

func (s *Server) discardArtifact(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Reason string `json:"reason"`
	}
	if !readBody(w, r, &b) {
		return
	}
	d, err := s.App.DiscardArtifact(r.PathValue("id"), b.Reason, ops.AsYou)
	s.artifactDone(w, d, err)
}

func (s *Server) restoreArtifact(w http.ResponseWriter, r *http.Request) {
	d, err := s.App.RestoreArtifact(r.PathValue("id"), ops.AsYou)
	s.artifactDone(w, d, err)
}

func (s *Server) clearArtifacts(w http.ResponseWriter, r *http.Request) {
	c, err := s.App.ClearExpiredArtifacts(ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusUnprocessableEntity)
		return
	}
	out := ArtifactsCleared{Local: []string{}, Repo: []string{}, Landed: c.Landed}
	for _, e := range c.Local {
		out.Local = append(out.Local, e.ID)
	}
	for _, e := range c.Repo {
		out.Repo = append(out.Repo, e.ID)
	}
	writeJSON(w, http.StatusOK, out)
}

// artifactSource is GET /api/artifacts/{id}/source?v=2: a version's HTML,
// as text to read (the latest, with no v).
func (s *Server) artifactSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	n := 0
	if v := r.URL.Query().Get("v"); v != "" {
		var ok bool
		if n, ok = artifact.ParseVersion(v); !ok {
			jsonError(w, http.StatusBadRequest, "v is a version: 2, or v2")
			return
		}
	}
	if !artifact.IsRef(id) || strings.Contains(id, ".") {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("%q isn't an artifact: they're named a1, a2, …", id))
		return
	}
	text, _, n, err := s.artifacts().ReadVersion(r.Context(), id, n)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ArtifactSource{Version: n, Text: string(text)})
}

// artifactCompare is GET /api/artifacts/compare?a=a3.v1&b=a5.v2: one
// version's HTML against another's, of one artifact or of two, diffed as
// a review's file is, for the compare page's source diff. A version left
// out (a3) is its latest. b's lines come too, to show the unchanged ones
// between the changes.
func (s *Server) artifactCompare(w http.ResponseWriter, r *http.Request) {
	st := s.artifacts()
	var texts [2][]byte
	var refs [2]string
	for i, side := range []string{"a", "b"} {
		id, n, ok := artifact.ParseRef(r.URL.Query().Get(side))
		if !ok {
			jsonError(w, http.StatusBadRequest, side+" is a version to compare: a3.v2 (or a3, its latest)")
			return
		}
		text, _, n, err := st.ReadVersion(r.Context(), id, n)
		if err != nil {
			jsonError(w, http.StatusNotFound, err.Error())
			return
		}
		texts[i], refs[i] = text, artifact.VersionRef(id, n)
	}
	f, err := review.TextDiff(r.Context(), refs[1]+".html", texts[0], texts[1])
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "couldn't diff "+refs[0]+" and "+refs[1]+": "+err.Error())
		return
	}
	lines := []string{}
	if t := strings.TrimSuffix(string(texts[1]), "\n"); t != "" {
		lines = strings.Split(t, "\n")
	}
	writeJSON(w, http.StatusOK, ArtifactCompare{A: refs[0], B: refs[1], File: *f, Lines: lines})
}
