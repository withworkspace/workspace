package web

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/space"
)

// The find window's side of the server: what a page doesn't hold of the workspace, looked through as you
// type.
//
// 	GET /api/find?q=…&kinds=files,pulls&limit=8
//
// answers a line of JSON for each kind as it's ready (a find.Group, or a
// SaidGroup for what ws records), then {"done":true}: one request for each change to what's typed, since a
// browser allows few connections to a host. A request the page has
// dropped (a new letter typed) stops being answered.
//
// With no q and no kinds it reads whatever of the index is due, waits for
// it, and answers {"warm":…}: what opening Find asks, so the first letter
// is answered from memory.

// findIndex is the index, with the pull requests the Reviews page keeps.
func (s *Server) findIndex() *find.Index {
	s.findMu.Lock()
	defer s.findMu.Unlock()
	if s.Find != nil && s.Find.Pulls == nil && s.Reviews != nil {
		s.Find.Pulls = func() ([]find.Pull, time.Time) {
			l := s.Reviews.Peek()
			if l == nil {
				return nil, time.Time{}
			}
			return findPulls(l), l.At
		}
	}
	return s.Find
}

// findPulls is the Reviews page's list as Find looks through it: every
// pull request in the order the page has them, a stack's layers each as
// its own.
func findPulls(l *pulls.List) []find.Pull {
	out := []find.Pull{}
	one := func(p pulls.PR, stack string) {
		out = append(out, find.Pull{
			Repo: p.Repo, Number: p.Number, Title: p.Title, Author: p.Author, Branch: p.Branch,
			State: p.State, Words: p.Words, Checks: p.Checks, Draft: p.Draft, Mine: p.Mine, Stack: stack,
		})
	}
	for _, sec := range l.Sections {
		for _, row := range sec.Rows {
			switch {
			case row.PR != nil:
				one(*row.PR, "")
			case row.Stack != nil:
				n := len(row.Stack.Layers) + row.Stack.Merged
				for i, p := range row.Stack.Layers {
					one(p, fmt.Sprintf("%d of %d: %s", row.Stack.Merged+i+1, n, row.Stack.Title))
				}
			}
		}
	}
	return out
}

func (s *Server) find(w http.ResponseWriter, r *http.Request) {
	ix := s.findIndex()
	if ix == nil {
		jsonError(w, http.StatusNotFound, "nothing to search here")
		return
	}
	q := r.URL.Query()
	req := find.Request{Q: q.Get("q")}
	if k := q.Get("kinds"); k != "" {
		req.Kinds = strings.Split(k, ",")
	}
	if n, err := strconv.Atoi(q.Get("limit")); err == nil {
		req.Limit = n
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	line := func(v any) {
		if enc.Encode(v) == nil && flusher != nil {
			flusher.Flush()
		}
	}
	if strings.TrimSpace(req.Q) == "" && len(req.Kinds) == 0 {
		st, err := ix.Warm(r.Context())
		if err != nil {
			line(map[string]any{"done": true, "problem": err.Error()})
			return
		}
		line(map[string]any{"warm": st})
		line(map[string]any{"done": true})
		return
	}
	if err := ix.Search(r.Context(), req, func(g find.Group) {
		if g.Results == nil {
			g.Results = []find.Result{}
		}
		line(g)
	}); err != nil {
		return // dropped: nobody's reading
	}
	if g, ok := s.findSaid(r, req); ok {
		line(g)
	}
	if g, ok := s.findDocuments(r, req); ok {
		line(g)
	}
	line(map[string]any{"done": true})
}

// wants is whether a kind is asked for: every kind is, when none's named.
func wants(req find.Request, kind string) bool {
	wanted := len(req.Kinds) == 0
	for _, k := range req.Kinds {
		wanted = wanted || k == kind
	}
	return wanted && len(strings.TrimSpace(req.Q)) >= 2
}

// SaidGroup is what ws records that says what's typed: a group of its own,
// as the documents are, its scores against its own rows.
type SaidGroup struct {
	Kind    string      `json:"kind"` // "said"
	Results []SaidFound `json:"results"`
	More    int         `json:"more,omitempty"`
	Problem string      `json:"problem,omitempty"`
}

// SaidFound is a thing found by what it says, led to where in it says it
// best.
type SaidFound struct {
	Kind string `json:"kind"` // "said"
	// Ref is what it's called where it's read (Scope: a mission's slug, a
	// pull request, "" for the workspace's): d1, t3, r1.c2, w4,
	// mission:handoffs/c1.md, or a mission's slug for the mission itself.
	Ref   string `json:"ref"`
	Scope string `json:"scope,omitempty"`
	// Thing is its kind (decision, thread-ask, talk, comment, review,
	// mission, handoff…); Place what's there where it's led (turn,
	// finding, decision, comment, review, brief, handoff, file), and At
	// where in it (a turn's number, a section's anchor, a finding's key):
	// none for the thing as a whole.
	Thing  string `json:"thing"`
	Place  string `json:"place"`
	Anchor string `json:"anchor,omitempty"`
	// What and Where say it in words: "a thread (ask), turn 2", "mission
	// house-sorting-cli".
	What  string `json:"what"`
	Where string `json:"where"`
	Title string `json:"title"`
	// Snippet is its words, those found marked; Hint the same, plain.
	Snippet []space.Part `json:"snippet"`
	Hint    string       `json:"hint,omitempty"`
	State   string       `json:"state,omitempty"`
	On      string       `json:"on,omitempty"` // a comment's file, at its lines
	// Of is what it's of, for a page to go to: a review's capsule or pull
	// request, the page a cell or a pass is of.
	Of    string  `json:"of,omitempty"`
	Ended bool    `json:"ended,omitempty"`
	Every bool    `json:"every,omitempty"`
	Score float64 `json:"score"`
	// Card is the plugin's thing it is, as a ref (the web UI's PageRef):
	// what its card asks its plugin about. To is where it opens: in the
	// plugin's app, or, with nowhere there, where it is outside ws.
	Card *ref.Ref `json:"card,omitempty"`
	To   string   `json:"to,omitempty"`
}

// findSaid is what ws records that says what's typed, ranked: asked for
// when every kind is, or "said" is, with a word or more typed; sent only
// when something's found.
func (s *Server) findSaid(r *http.Request, req find.Request) (SaidGroup, bool) {
	if !wants(req, "said") || s.App == nil {
		return SaidGroup{}, false
	}
	g := SaidGroup{Kind: "said", Results: []SaidFound{}}
	found, more, err := s.App.Find(r.Context(), ops.FindQuery{Words: req.Q, Limit: max(req.Limit, 5)})
	if err != nil {
		g.Problem = err.Error()
		return g, true
	}
	if len(found) == 0 {
		return SaidGroup{}, false
	}
	for _, f := range found {
		g.Results = append(g.Results, saidFound(f))
	}
	g.More = more
	return g, true
}

// saidFound is a thing ws records as the find window draws one.
func saidFound(f ops.Found) SaidFound {
	said := SaidFound{
		Kind: "said", Ref: f.Ref, Scope: f.Scope, Thing: f.Thing.Kind, Place: f.Place, Anchor: f.At, Of: f.Of,
		What: f.What(), Where: f.Where(), Title: f.Title(),
		Snippet: f.Snippet, Hint: space.Text(f.Snippet, "", ""),
		State: f.Thing.State, On: f.On, Ended: f.Ended, Every: f.Every, Score: f.Score,
	}
	if f.Named != nil {
		said.Card, said.To = f.Named, cmp.Or(f.Opens, f.Named.URL)
	}
	return said
}

// findDocuments is the spaces' documents that say what's typed, ranked: a
// group of its own, as its scores are against its own rows and don't
// compare with the others'. Asked for when every kind is, or "documents"
// is, with a word or more typed; sent only when something's found.
func (s *Server) findDocuments(r *http.Request, req find.Request) (find.Group, bool) {
	if !wants(req, "documents") {
		return find.Group{}, false
	}
	g := find.Group{Kind: "documents", Results: []find.Result{}}
	hits, more, err := s.App.SearchDocuments(r.Context(), ops.DocQuery{Words: req.Q, Limit: max(req.Limit, 5)})
	if err != nil {
		g.Problem = err.Error()
		return g, true
	}
	if len(hits) == 0 {
		return find.Group{}, false // nothing to say: no group
	}
	for _, h := range hits {
		title := h.Title
		if h.Heading != "" {
			title += " › " + h.Heading
		}
		g.Results = append(g.Results, find.Result{Kind: "document", Ref: h.Repo + ":" + h.Path, Title: title, Hint: space.Text(h.Snippet, "", ""), Score: h.Score,
			Repo: h.Repo, Worktree: ".ground", Path: h.Path, Anchor: h.Anchor})
	}
	g.More = more
	return g, true
}
