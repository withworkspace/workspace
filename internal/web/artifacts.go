package web

import (
	"bytes"
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"html"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/review"
)

// Artifacts in the web UI: the list (in the workspace's view, so it
// follows every change), an artifact's page (GET /api/artifacts/{id}), a
// version's source (.../source?v=2&file=css/app.css), two versions'
// files diffed (GET /api/artifacts/compare?a=a3.v1&b=a5.v2), and each
// version run from a route of its own, GET /frame/a3/v2, in a sandbox;
// and an artifact a conversation proposed, GET /frame/w9/proposals/2, the
// same.
//
// An artifact is HTML an agent wrote, run inside the page that controls
// your missions. So it's served from outside /api and the app's routes,
// with a policy of its own: it runs its scripts, but in an origin of its
// own (no cookie, no storage, nothing of the page around it), and nothing
// leaves it (no fetch, script, font or image from anywhere but its own
// files, no popup, no form, no navigating the page). The web UI frames it
// with sandbox="allow-scripts" and no allow-same-origin; opened alone,
// the policy's sandbox holds it to the same.
//
// A version is its page and the files beside it, and the page loads them
// by paths of its own (css/app.css). A frame with an origin of its own
// sends no cookie when it does, nor when a link in it opens another of
// its pages, so its files can't be behind the sign-in: they're under a
// path with a key in it (/framed/<key>/a3/v2/css/app.css), which is what
// lets them be read. /frame/a3/v2, signed in, sends the frame there. The
// key is made from the workspace's token and the frame's own address, so
// one frame's key reads no other's files, and the policy names that path
// as the only place the page may load from.

// framePolicy is a frame's Content-Security-Policy: sandboxed, with its
// scripts and styles inline or from its own files (own, the address
// they're under: "" for a page that's alone), its images, fonts and
// media in it or from them, and framed only by the web UI.
func framePolicy(own string) string {
	from, connect := "", ""
	if own != "" {
		// Its scripts read its own files too (fetch("data.json")).
		from, connect = " "+own, "; connect-src "+own
	}
	return "sandbox allow-scripts; default-src 'none'; script-src 'unsafe-inline'" + from + "; style-src 'unsafe-inline'" + from + "; " +
		"img-src data: blob:" + from + "; font-src data:" + from + "; media-src data: blob:" + from + connect + "; frame-ancestors 'self'"
}

// FramePolicy is the policy of a page that's alone: nothing's loaded.
var FramePolicy = framePolicy("")

// frameHeaders are what every answer on /frame and /framed has, in place
// of the guard's (which would refuse to be framed, and run no inline
// script).
func frameHeaders(w http.ResponseWriter, own string) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", framePolicy(own))
	h.Set("X-Frame-Options", "SAMEORIGIN")
}

// framedRoot is where frames' files are served from, each under its key.
const framedRoot = "/framed/"

// frameKey is the key a frame's files are under: made from the
// workspace's token and the frame's own address (/frame/a3/v2), so it
// can't be guessed, and reads that frame's files only.
func (s *Server) frameKey(frame string) string {
	mac := hmac.New(sha256.New, []byte(s.info.Token))
	mac.Write([]byte("frame\x00" + frame))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// framed reads the address of a frame's file, /framed/<key>/a3/v2/css/app.css:
// the frame it's of, as its own address names it (a3/v2, w9/proposals/2,
// missions/glow/t3/proposals/2), the file's path in it ("" for its
// page), and whether the key is that frame's.
func (s *Server) framed(path string) (frame, file string, ok bool) {
	rest, cut := strings.CutPrefix(path, framedRoot)
	if !cut {
		return "", "", false
	}
	key, rest, _ := strings.Cut(rest, "/")
	segs := strings.Split(rest, "/")
	n := 2 // a3/v2
	switch {
	case segs[0] == "missions":
		n = 5 // missions/glow/t3/proposals/2
	case len(segs) > 1 && segs[1] == "proposals":
		n = 3 // w9/proposals/2
	}
	if len(segs) < n {
		return "", "", false
	}
	frame, file = strings.Join(segs[:n], "/"), strings.Join(segs[n:], "/")
	// A frame's address is a few plain names (a3/v2): it's written into
	// the frame's policy, and nothing else is one.
	if !frameShape.MatchString(frame) {
		return "", "", false
	}
	return frame, file, subtle.ConstantTimeCompare([]byte(key), []byte(s.frameKey("/frame/"+frame))) == 1
}

var frameShape = regexp.MustCompile(`^(a[1-9][0-9]*/v[1-9][0-9]*|(missions/[A-Za-z0-9._-]+/)?[a-z][0-9]+/proposals/[1-9][0-9]*)$`)

// framedURL is where the frame at address (/frame/a3/v2) has its files.
func (s *Server) framedURL(frame string) string {
	return framedRoot + s.frameKey(frame) + strings.TrimPrefix(frame, "/frame") + "/"
}

// toFramed sends a frame, signed in, to where its files are, with what
// its address says after ? (the state of its tweaks), which is the
// page's to read.
func (s *Server) toFramed(w http.ResponseWriter, r *http.Request, frame string) {
	to := s.framedURL(frame)
	if r.URL.RawQuery != "" {
		to += "?" + r.URL.RawQuery
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, to, http.StatusFound)
}

// frame is GET /frame/{id}/{version}: a version, to run in the web UI's
// sandboxed frame or opened alone. It's sent to where the version's files
// are (framedFile), its page first.
func (s *Server) frame(w http.ResponseWriter, r *http.Request) {
	frameHeaders(w, "")
	id, vs := r.PathValue("id"), r.PathValue("version")
	n, ok := 0, false
	if num, cut := strings.CutPrefix(vs, "v"); cut {
		n, ok = artifact.ParseVersion(num)
	}
	if !ok || !artifact.IsRef(id) || strings.Contains(id, ".") {
		framePage(w, http.StatusNotFound, "There's no such version", "A version's frame is /frame/a3/v2: an artifact, then the version.")
		return
	}
	if _, _, _, err := s.artifacts().ReadVersion(r.Context(), id, n); err != nil {
		// No such artifact or version, its folder gone, or its files: there's
		// nothing here to run, whichever.
		framePage(w, http.StatusNotFound, "It can't be shown", html.EscapeString(err.Error()))
		return
	}
	s.toFramed(w, r, "/frame/"+id+"/v"+strconv.Itoa(n))
}

// proposalFrame is GET /frame/w9/proposals/2 (a mission's conversation:
// /frame/missions/{slug}/t3/proposals/2): an artifact a conversation's
// agent proposed, as it proposed it. It's served as a version is,
// ws.tweaks and all, so it runs in the card's frame as it would once
// added; and it's there whatever became of the proposal: one declined
// isn't an artifact, but it stays in the conversation, with its frame.
// One that's its page alone is answered here, from the conversation's
// records; one with files is sent to where they are.
func (s *Server) proposalFrame(w http.ResponseWriter, r *http.Request) {
	frameHeaders(w, "")
	p, _, ok := s.proposed(w, r, r.PathValue("slug"), r.PathValue("id"), r.PathValue("n"))
	if !ok {
		return
	}
	if p.Tree != "" {
		s.toFramed(w, r, r.URL.Path)
		return
	}
	s.serveFrameFile(w, r, r.URL.Path, artifact.IndexFile, []byte(p.HTML), "", "")
}

// proposed is the artifact proposed as n in conversation id (of mission
// slug, or the workspace's), and where it's kept; it answers for one
// that isn't there.
func (s *Server) proposed(w http.ResponseWriter, r *http.Request, slug, id, num string) (*conversation.Proposal, conversation.Store, bool) {
	n, err := strconv.Atoi(num)
	store, serr := s.talkStoreOf(slug)
	if err != nil || n < 1 || serr != nil || (store.Workspace() && !conversation.IsWorkspaceRef(id)) || (!store.Workspace() && !conversation.IsRef(id)) {
		framePage(w, http.StatusNotFound, "There's no such proposal", "A proposal's frame is /frame/w9/proposals/2: a conversation, then the proposal.")
		return nil, store, false
	}
	c, err := store.Load(id)
	var p *conversation.Proposal
	if err == nil {
		p = c.Proposal(n)
	}
	if p == nil || p.Kind != conversation.ProposeArtifact {
		framePage(w, http.StatusNotFound, "It can't be shown", html.EscapeString(fmt.Sprintf("%s has no artifact proposed as %d.", id, n)))
		return nil, store, false
	}
	return p, store, true
}

// framedFile is GET /framed/<key>/…: one of a frame's files, by its path
// in the version (or in what was proposed), its page for none. The key
// in its address is what lets it be read (guard): the frame that asks
// sends no cookie. A page is served with ws.tweaks written in ahead of
// it (tweaks.go) and the frame's policy, which names this frame's files
// as the only place it loads from; any other file, as it is.
func (s *Server) framedFile(w http.ResponseWriter, r *http.Request) {
	frame, file, ok := s.framed(r.URL.Path)
	frameHeaders(w, "")
	if !ok {
		// A key that isn't this frame's: one from before the workspace's
		// token changed, kept in a tab. Signed in, it's sent to the one
		// that is; otherwise the guard has answered already.
		if frame != "" {
			to := s.framedURL("/frame/"+frame) + file
			if r.URL.RawQuery != "" {
				to += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, to, http.StatusFound)
			return
		}
		framePage(w, http.StatusNotFound, "There's nothing here", "A frame's files are under /framed/, each frame's under its own key.")
		return
	}
	if file == "" || strings.HasSuffix(file, "/") {
		file += artifact.IndexFile
	}
	own := "http://" + r.Host + s.framedURL("/frame/"+frame)
	segs := strings.Split(frame, "/")
	var data []byte
	var blob, salt string
	var err error
	if segs[len(segs)-2] == "proposals" {
		slug := ""
		if segs[0] == "missions" {
			slug = segs[1]
		}
		p, store, ok := s.proposed(w, r, slug, segs[len(segs)-3], segs[len(segs)-1])
		if !ok {
			return
		}
		salt = store.Slug + "/" + frame
		switch {
		case p.Tree != "":
			data, blob, err = s.artifacts().ProposedFile(r.Context(), p.Tree, file)
		case file == artifact.IndexFile:
			data = []byte(p.HTML)
		default:
			err = artifact.ErrNoFile
		}
	} else {
		n, _ := artifact.ParseVersion(strings.TrimPrefix(segs[1], "v"))
		var e artifact.Entry
		data, blob, e, _, err = s.artifacts().ReadFile(r.Context(), segs[0], n, file)
		salt = e.Folder
	}
	if err != nil {
		what := html.EscapeString(err.Error())
		if errors.Is(err, artifact.ErrNoFile) {
			what = html.EscapeString(file) + " isn't one of its files."
		}
		framePage(w, http.StatusNotFound, "It can't be shown", what)
		return
	}
	s.serveFrameFile(w, r, salt, file, data, blob, own)
}

// frameTypes are the types a frame's files are served as, by what their
// names end in: what a page loads. Anything else is served as bytes,
// which a browser runs as nothing.
var frameTypes = map[string]string{
	".html": "text/html; charset=utf-8", ".htm": "text/html; charset=utf-8",
	".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".mjs": "text/javascript; charset=utf-8",
	".json": "application/json", ".map": "application/json", ".txt": "text/plain; charset=utf-8", ".md": "text/plain; charset=utf-8",
	".csv": "text/csv; charset=utf-8", ".xml": "application/xml", ".svg": "image/svg+xml",
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp", ".avif": "image/avif", ".ico": "image/x-icon",
	".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf", ".otf": "font/otf",
	".mp4": "video/mp4", ".webm": "video/webm", ".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg",
}

// serveFrameFile answers with one of a frame's files: a page with
// ws.tweaks written in ahead of it, anything else as it is, under the
// frame's policy (own: where its files are, "" for a page that's alone).
// salt says which frame it is, and blob what names the file's bytes ("":
// they're hashed), for the ETag.
func (s *Server) serveFrameFile(w http.ResponseWriter, r *http.Request, salt, file string, data []byte, blob, own string) {
	frameHeaders(w, own)
	h := w.Header()
	ctype, known := frameTypes[strings.ToLower(path.Ext(file))]
	if !known {
		ctype = "application/octet-stream"
	}
	h.Set("Content-Type", ctype)
	page := strings.HasPrefix(ctype, "text/html")
	if page {
		data = withTweaks(data)
	}
	if own != "" {
		// A module, a font and a fetch are asked for with an origin, and the
		// frame's is no one's: its files are read by whoever has the key.
		h.Set("Access-Control-Allow-Origin", "*")
	}
	// A version never changes, but what a3.v2 names can: a workspace made
	// again in the same folder (just test-workspace) numbers its own from
	// a1, on the same port. So it's asked again each time, and a 304
	// answers when it's the same file.
	sum := fnv.New64a()
	sum.Write([]byte(salt + "\n" + file + "\n"))
	if blob != "" && !page {
		sum.Write([]byte(blob))
	} else {
		sum.Write(data)
	}
	h.Set("ETag", `"`+strconv.FormatUint(sum.Sum64(), 36)+`"`)
	h.Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
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
	info := ArtifactInfo{ID: e.ID, Title: e.Title, Status: string(e.Status), Gone: e.Gone || e.Away(), Folder: e.Folder,
		Where: e.Where(), Made: e.Made, Versions: len(e.Versions), Promoted: e.Promoted, Discarded: e.Discarded,
		Erased: e.Erased, Restorable: e.Restorable(), Here: e.Here && e.Home == nil}
	if h := e.Home; h != nil && e.Lives() != "" {
		info.Home = &ArtifactHome{Where: h.Where(), Repo: h.Repo, Version: h.Version, Commit: h.Commit, At: h.At}
	}
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
	return ArtifactVersion{N: v.N, BasedOn: v.BasedOn, At: v.At, By: v.By, From: v.From, Proposal: v.Proposal, FromTitle: v.FromTitle, Note: v.Note}
}

// artifactDetail is GET /api/artifacts/{id}: an artifact with every
// version, oldest first, each with how many files it has and what they
// come to (0 while there are none to read).
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
	d := ArtifactDetail{ArtifactInfo: artifactInfo(e, s.App.ArtifactsConfig(), time.Now()), All: []ArtifactVersion{}, HomeOffered: s.App.ArtifactHomeOffered(e)}
	for _, v := range e.Versions {
		av := versionOf(v)
		av.Files, av.Size, _ = st.VersionSize(e, v.N)
		d.All = append(d.All, av)
	}
	writeJSON(w, http.StatusOK, d)
}

// An artifact's lifetime, as you: POST /api/artifacts/{id}/promote,
// /discard and /restore, and /api/artifacts/clear for the expired. For
// one that lives in a repo, each of the first three is a commit landed on
// its default branch, so it may take a few seconds.

// artifactDone answers with what was done, or why it wasn't: an error
// that comes after part of it was (promoted, but not recorded on the
// mission) says so itself, and the list shows the rest.
func (s *Server) artifactDone(w http.ResponseWriter, d ops.ArtifactDone, err error) {
	if err != nil {
		writeError(w, err, http.StatusUnprocessableEntity)
		return
	}
	out := ArtifactDone{Artifact: artifactInfo(d.Entry, s.App.ArtifactsConfig(), time.Now()), Home: d.Home, Landed: d.Landed, First: d.First, Into: d.Into, Started: d.Started, Recorded: d.Recorded}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) promoteArtifact(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Version int    `json:"version"`
		Into    string `json:"into"`
		Start   string `json:"start"`
		Home    string `json:"home"`
		Here    bool   `json:"here"`
	}
	if !readBody(w, r, &b) {
		return
	}
	d, err := s.App.PromoteArtifact(ops.ArtifactPromote{Ref: r.PathValue("id"), Version: b.Version, Into: b.Into, Start: b.Start, Home: b.Home, Here: b.Here}, ops.AsYou)
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
	out := ArtifactsCleared{Local: []string{}}
	for _, e := range c.Local {
		out.Local = append(out.Local, e.ID)
	}
	writeJSON(w, http.StatusOK, out)
}

// artifactSource is GET /api/artifacts/{id}/source?v=2&file=css/app.css:
// one of a version's files, as text to read (the latest, with no v; its
// page, with no file), and the version's files, to choose another. A
// file that isn't text comes with none: its size says what it is.
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
	file := cmp.Or(r.URL.Query().Get("file"), artifact.IndexFile)
	st := s.artifacts()
	data, _, _, n, err := st.ReadFile(r.Context(), id, n, file)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	out := ArtifactSource{Version: n, File: file, Size: len(data), Files: []ArtifactFile{}}
	if isText(data) {
		out.Text = string(data)
	} else {
		out.Binary = true
	}
	if list, _, _, err := st.VersionFiles(r.Context(), id, n); err == nil {
		for _, f := range list {
			out.Files = append(out.Files, ArtifactFile{Path: f.Path, Size: f.Size})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// isText is whether a file's bytes are text to read and to diff.
func isText(data []byte) bool {
	return utf8.Valid(data) && bytes.IndexByte(data, 0) < 0
}

// artifactCompare is GET /api/artifacts/compare?a=a3.v1&b=a5.v2: one
// version's files against another's, of one artifact or of two: each
// file that differs, diffed as a review's file is, for the compare page's
// source diff. A version left out (a3) is its latest. b's lines come with
// each, to show the unchanged ones between the changes. A file that isn't
// text is named as changed, and not diffed.
func (s *Server) artifactCompare(w http.ResponseWriter, r *http.Request) {
	st := s.artifacts()
	var files [2]artifact.Files
	var refs [2]string
	for i, side := range []string{"a", "b"} {
		id, n, ok := artifact.ParseRef(r.URL.Query().Get(side))
		if !ok {
			jsonError(w, http.StatusBadRequest, side+" is a version to compare: a3.v2 (or a3, its latest)")
			return
		}
		all, _, n, err := st.AllFiles(r.Context(), id, n)
		if err != nil {
			jsonError(w, http.StatusNotFound, err.Error())
			return
		}
		files[i], refs[i] = all, artifact.VersionRef(id, n)
	}
	out := ArtifactCompare{A: refs[0], B: refs[1], Files: []ArtifactCompareFile{}}
	for _, c := range artifact.Differ(files[0], files[1]) {
		was, _ := files[0].File(c.Path)
		is, _ := files[1].File(c.Path)
		status := map[string]string{"added": "added", "removed": "deleted", "changed": "modified"}[c.Status]
		if !isText(was) || !isText(is) {
			out.Files = append(out.Files, ArtifactCompareFile{File: review.File{Path: c.Path, Status: status, Binary: true, Hunks: []review.Hunk{}}, Lines: []string{}})
			continue
		}
		f, err := review.TextDiff(r.Context(), c.Path, was, is)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "couldn't diff "+c.Path+" of "+refs[0]+" and "+refs[1]+": "+err.Error())
			return
		}
		f.Status = status
		lines := []string{}
		if t := strings.TrimSuffix(string(is), "\n"); t != "" {
			lines = strings.Split(t, "\n")
		}
		out.Files = append(out.Files, ArtifactCompareFile{File: *f, Lines: lines})
	}
	writeJSON(w, http.StatusOK, out)
}
