package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/review"
)

// ErrNotFound is Capsule's answer for a capsule that isn't the mission's.
var ErrNotFound = ops.ErrNotFound

// fullSHA is how the API names commits: the client only ever sends back
// what a diff gave it.
var fullSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// capsuleDir resolves the request's mission capsule, or answers for it.
func (s *Server) capsuleDir(w http.ResponseWriter, r *http.Request) (dir, baseRef string, ok bool) {
	sc, repo, name := scope(r), r.PathValue("repo"), r.PathValue("name")
	if r.PathValue("number") != "" { // a pull request's: its repo's clone
		return s.prDir(w, r)
	}
	var err error
	if sc.IsWorkspace() {
		// The Files page's: any of the workspace's worktrees.
		dir, baseRef, err = s.App.Worktree(repo, name)
	} else {
		dir, baseRef, err = s.App.CapsuleDir(sc.Mission, repo, name)
	}
	switch {
	case errors.Is(err, ErrNotFound) && sc.IsWorkspace():
		jsonError(w, http.StatusNotFound, "the workspace has no "+repo+"/"+name)
		return "", "", false
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, "mission "+sc.Mission+" has no capsule "+repo+"/"+name)
		return "", "", false
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
		return "", "", false
	}
	return dir, baseRef, true
}

// diff is the capsule's committed changes against its base.
func (s *Server) diff(w http.ResponseWriter, r *http.Request) {
	dir, baseRef, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	since := r.URL.Query().Get("since")
	if since != "" && !fullSHA.MatchString(since) {
		jsonError(w, http.StatusBadRequest, "since is a full commit id")
		return
	}
	read := func() (any, error) {
		if since != "" {
			return review.CompareSince(dir, baseRef, since)
		}
		return review.Compare(dir, baseRef)
	}
	// Its commits are kept by the capsule's stamp, which moves with its
	// HEAD and its base. What's not committed isn't in them: that's as the
	// summary has it (kept, and read behind), unless it's never been read.
	stamp := ops.CapsuleStamp(dir)
	if stamp == "" {
		v, err := read()
		if err != nil {
			jsonError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, v)
		return
	}
	v, err := s.reads.kept(keyOf("diff", dir, baseRef, since, stamp), read)
	if err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	d := *v.(*review.Diff)
	if sum, unread, err := s.reads.soon(keyOf("summary", dir, baseRef), dir, func() (any, error) { return review.Summarize(dir, baseRef) }); err == nil && !unread {
		d.Uncommitted = uncommittedOf(sum.(*review.Summary))
	}
	writeJSON(w, http.StatusOK, d)
}

// uncommittedOf is the paths a summary has that aren't committed.
func uncommittedOf(sum *review.Summary) []string {
	out := []string{}
	for _, f := range sum.Changed {
		if f.Uncommitted {
			out = append(out, f.Path)
		}
	}
	return out
}

// fileDiff is one file's changes in full, for a file too large to come
// with the rest.
func (s *Server) fileDiff(w http.ResponseWriter, r *http.Request) {
	dir, _, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	base, head, path := q.Get("base"), q.Get("head"), q.Get("path")
	if !fullSHA.MatchString(base) || !fullSHA.MatchString(head) || !validPath(path) {
		jsonError(w, http.StatusBadRequest, "need base and head (full commit ids) and a path")
		return
	}
	// Between two commits: kept for good (in keptMax).
	f, err := s.reads.kept(keyOf("file-diff", dir, base, head, path), func() (any, error) { return review.FileDiff(dir, base, head, path) })
	if err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// blob is a file's lines at a commit, for showing more context.
func (s *Server) blob(w http.ResponseWriter, r *http.Request) {
	dir, _, ok := s.capsuleDir(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	rev, path := q.Get("rev"), q.Get("path")
	if !fullSHA.MatchString(rev) || !validPath(path) {
		jsonError(w, http.StatusBadRequest, "need rev (a full commit id) and a path")
		return
	}
	lines, err := s.reads.kept(keyOf("blob", dir, rev, path), func() (any, error) { return review.Blob(dir, rev, path) })
	if err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": lines})
}

// validPath is a path in a repo, as git names it: relative, not an option.
func validPath(p string) bool {
	return p != "" && !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "-") && !strings.ContainsRune(p, 0)
}

// The review being written, and those submitted: GET .../review and the
// writes under it. Reviews are the mission's, kept by internal/review.

// ReviewState is GET .../review: the capsule's draft (null if none), its
// submitted reviews (newest first), and which files you've viewed.
type ReviewState struct {
	Draft   *review.Review    `json:"draft"`
	Reviews []*review.Review  `json:"reviews"`
	Viewed  map[string]string `json:"viewed"`
	// URL is where a review just submitted is on GitHub (a pull request's),
	// and Sending that it's on its way there: kept here as submitted, and
	// posted behind.
	URL     string `json:"url,omitempty"`
	Sending bool   `json:"sending,omitempty"`
}

// reviewing resolves what the request reviews (a mission's capsule, or a
// pull request), where it is, and its reviews: to read from, by its key.
func (s *Server) reviewing(w http.ResponseWriter, r *http.Request) (of ops.ReviewOf, store review.Store, capsule, dir, baseRef string, ok bool) {
	dir, baseRef, ok = s.capsuleDir(w, r)
	if !ok {
		return
	}
	of = ops.OfCapsule(r.PathValue("slug"), r.PathValue("repo"), r.PathValue("name"))
	if n := r.PathValue("number"); n != "" { // a pull request's reviews are its own
		number, _ := strconv.Atoi(n)
		of = ops.OfPR(r.PathValue("repo"), number)
	}
	store, capsule = s.App.Reviews(of)
	return of, store, capsule, dir, baseRef, true
}

func (s *Server) reviewState(store review.Store, capsule string) (*ReviewState, error) {
	draft, err := store.Draft(capsule)
	if err != nil {
		return nil, err
	}
	all, err := store.List()
	if err != nil {
		return nil, err
	}
	st := &ReviewState{Draft: draft, Reviews: []*review.Review{}}
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Capsule == capsule && all[i].State == review.Submitted {
			st.Reviews = append(st.Reviews, all[i])
		}
	}
	if st.Viewed, err = store.Viewed(capsule); err != nil {
		return nil, err
	}
	return st, nil
}

// answerReview replies with the review state after a write, or the write's
// error: a missing draft or comment is 404, anything else it refused 422.
func (s *Server) answerReview(w http.ResponseWriter, store review.Store, capsule string, err error) {
	switch {
	case errors.Is(err, review.ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
		return
	case err != nil:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	st, err := s.reviewState(store, capsule)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) getReview(w http.ResponseWriter, r *http.Request) {
	_, store, capsule, _, _, ok := s.reviewing(w, r)
	if ok {
		s.answerReview(w, store, capsule, nil)
	}
}

func (s *Server) discardReview(w http.ResponseWriter, r *http.Request) {
	of, store, capsule, _, _, ok := s.reviewing(w, r)
	if ok {
		s.answerReview(w, store, capsule, s.App.DiscardReview(of))
	}
}

// readBody decodes a write's JSON body.
func readBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		jsonError(w, http.StatusBadRequest, "bad request body: "+err.Error())
		return false
	}
	return true
}

// diffed is the diff a write was made on, as the page sends it back: full
// commit ids it was given.
func diffed(w http.ResponseWriter, dir, baseRef, base, head string) (review.Diffed, bool) {
	if !fullSHA.MatchString(base) || !fullSHA.MatchString(head) {
		jsonError(w, http.StatusBadRequest, "need base and head: the full commit ids of the diff")
		return review.Diffed{}, false
	}
	return review.Diffed{Dir: dir, BaseRef: baseRef, Base: base, Head: head}, true
}

type noteBody struct {
	Base       string  `json:"base"`
	Head       string  `json:"head"`
	Path       string  `json:"path"`
	Side       string  `json:"side"`
	Line       int     `json:"line"`
	EndLine    int     `json:"endLine"`
	Kind       string  `json:"kind"`
	Body       string  `json:"body"`
	Suggestion *string `json:"suggestion"`
}

func (s *Server) addNote(w http.ResponseWriter, r *http.Request) {
	of, store, capsule, dir, baseRef, ok := s.reviewing(w, r)
	var b noteBody
	if !ok || !readBody(w, r, &b) {
		return
	}
	d, ok := diffed(w, dir, baseRef, b.Base, b.Head)
	if !ok {
		return
	}
	if !validPath(b.Path) {
		jsonError(w, http.StatusBadRequest, "need a path")
		return
	}
	err := s.App.AddNote(of, d, review.NewNote{Path: b.Path, Side: b.Side, Line: b.Line, EndLine: b.EndLine, Kind: b.Kind, Body: b.Body, Suggestion: b.Suggestion})
	s.answerReview(w, store, capsule, err)
}

func (s *Server) editNote(w http.ResponseWriter, r *http.Request) {
	of, store, capsule, _, _, ok := s.reviewing(w, r)
	var b struct {
		Kind       string  `json:"kind"`
		Body       string  `json:"body"`
		Suggestion *string `json:"suggestion"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	err := s.App.EditNote(of, r.PathValue("id"), b.Kind, b.Body, b.Suggestion)
	s.answerReview(w, store, capsule, err)
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	of, store, capsule, _, _, ok := s.reviewing(w, r)
	if ok {
		s.answerReview(w, store, capsule, s.App.DeleteNote(of, r.PathValue("id")))
	}
}

func (s *Server) setViewed(w http.ResponseWriter, r *http.Request) {
	of, store, capsule, _, _, ok := s.reviewing(w, r)
	var b struct {
		Path   string `json:"path"`
		Blob   string `json:"blob"`
		Viewed bool   `json:"viewed"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	if !validPath(b.Path) {
		jsonError(w, http.StatusBadRequest, "need a path")
		return
	}
	err := s.App.SetViewed(of, b.Path, b.Blob, b.Viewed)
	s.answerReview(w, store, capsule, err)
}

// respond is you answering a submitted comment: a reply, or resolving or
// reopening it. (Crew answer with ws review addressed/wontfix/reply.)
func (s *Server) respond(w http.ResponseWriter, r *http.Request) {
	of, store, capsule, _, _, ok := s.reviewing(w, r)
	var b struct {
		What string `json:"what"` // reply, resolve, reopen
		Body string `json:"body"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	what := review.Response(b.What)
	if what != review.Answer && what != review.Resolve && what != review.Reopen {
		jsonError(w, http.StatusBadRequest, "what is reply, resolve or reopen")
		return
	}
	_, err := s.App.Respond(of, r.PathValue("id"), what, b.Body, "", ops.AsYou)
	s.answerReview(w, store, capsule, err)
}

func (s *Server) submitReview(w http.ResponseWriter, r *http.Request) {
	of, store, capsule, dir, baseRef, ok := s.reviewing(w, r)
	var b struct {
		Base    string `json:"base"`
		Head    string `json:"head"`
		Verdict string `json:"verdict"`
		Summary string `json:"summary"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	d, ok := diffed(w, dir, baseRef, b.Base, b.Head)
	if !ok {
		return
	}
	_, err := s.App.SubmitReview(of, d, b.Verdict, b.Summary, ops.AsYou)
	s.answerReview(w, store, capsule, err)
}

// warmCapsules reads, behind, what a mission's page that's open reads of
// its capsules next (each one's diff and summary, as its Work and Verify
// tabs and its review ask for them), so they're kept when it does: a
// read of one runs no git. One mission's at a time, one capsule after
// another; what's kept as its capsule is now is a lookup.
func (s *Server) warmCapsules(slug string, d *MissionDetail) {
	if _, busy := s.warming.LoadOrStore(slug, true); busy {
		return
	}
	caps := make([]Capsule, 0, len(d.Capsules))
	for _, c := range d.Capsules {
		if c.Exists {
			caps = append(caps, c.Capsule)
		}
	}
	go func() {
		defer s.warming.Delete(slug)
		for _, c := range caps {
			dir, baseRef, err := s.App.CapsuleDir(slug, c.Repo, c.Name)
			if err != nil {
				continue
			}
			stamp := ops.CapsuleStamp(dir)
			if stamp == "" {
				continue
			}
			s.reads.kept(keyOf("diff", dir, baseRef, "", stamp), func() (any, error) { return review.Compare(dir, baseRef) })
			s.reads.soon(keyOf("summary", dir, baseRef), dir, func() (any, error) { return review.Summarize(dir, baseRef) })
		}
	}()
}
