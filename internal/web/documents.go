package web

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/space"
)

// Documents: the workspace's spaces of documents (a repo of them, or a
// folder of one), as ws keeps them: answered at once, each read again
// from its ground behind the read when it's moved, the workspace's
// spacesSeen moving when that finds something new.

// Space is a space of documents, as a list of them shows it.
type Space struct {
	Repo    string        `json:"repo"`
	Label   string        `json:"label"`
	Color   string        `json:"color"`
	Folder  string        `json:"folder,omitempty"` // "" for the whole repo
	Count   int           `json:"count"`
	Cloned  bool          `json:"cloned"`
	Folders []SpaceFolder `json:"folders"`
	Kinds   []SpaceKind   `json:"kinds"`
}

// SpaceKind is a kind of document a space has, and where one can stand.
type SpaceKind struct {
	Name   string   `json:"name"`
	Title  string   `json:"title"`
	States []string `json:"states,omitempty"`
}

// SpaceFolder is a folder at a space's top, and how many documents it has.
type SpaceFolder struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SpaceOf is a space as the API serves it.
func SpaceOf(v ops.SpaceView) Space {
	out := Space{Repo: v.Repo, Label: v.Label, Color: v.Color, Folder: v.Folder, Count: v.Count, Cloned: v.Cloned, Folders: []SpaceFolder{}, Kinds: []SpaceKind{}}
	for _, f := range v.Folders {
		out.Folders = append(out.Folders, SpaceFolder(f))
	}
	for _, k := range v.Kinds {
		out.Kinds = append(out.Kinds, SpaceKind(k))
	}
	return out
}

// SpaceDoc is a document in a space: its path there, its title, its kind
// and where it stands, what its path says (a date, a number), and who
// changed it last and when.
type SpaceDoc struct {
	Path      string `json:"path"`
	Title     string `json:"title"`
	Kind      string `json:"kind,omitempty"`
	State     string `json:"state,omitempty"`
	Date      string `json:"date,omitempty"`
	Number    string `json:"number,omitempty"`
	Changed   string `json:"changed,omitempty"`
	ChangedBy string `json:"changedBy,omitempty"`
}

// documents is the workspace's spaces. A space that couldn't be read is
// listed as it last was, and why is said.
func (s *Server) documents(w http.ResponseWriter, r *http.Request) {
	list, err := s.App.SpaceList(r.Context())
	if list == nil && err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]Space, 0, len(list))
	for _, v := range list {
		out = append(out, SpaceOf(v))
	}
	body := map[string]any{"spaces": out}
	if err != nil {
		body["problem"] = err.Error()
	}
	writeJSON(w, http.StatusOK, body)
}

// spaceDocuments is a space and its documents, by path.
func (s *Server) spaceDocuments(w http.ResponseWriter, r *http.Request) {
	v, docs, err := s.App.SpaceDocs(r.Context(), r.PathValue("repo"))
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
		return
	case v == nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]SpaceDoc, 0, len(docs))
	for _, d := range docs {
		out = append(out, SpaceDoc(d))
	}
	body := map[string]any{"space": SpaceOf(*v), "docs": out}
	if err != nil {
		body["problem"] = err.Error()
	}
	writeJSON(w, http.StatusOK, body)
}

// SpaceActivity is a space's top: what changed lately on its default
// branch, what's in flight in capsules, and how fresh its ground is.
type SpaceActivity struct {
	Lately   []SpaceCommit `json:"lately"`
	InFlight []SpaceFlight `json:"inFlight"`
	Fresh    SpaceFresh    `json:"fresh"`
	// Unread: it's not been read yet, and is being: the workspace's
	// spacesSeen moves when it has.
	Unread bool `json:"unread,omitempty"`
}

// SpaceCommit is a commit that changed a space's documents.
type SpaceCommit struct {
	SHA     string           `json:"sha"`
	At      string           `json:"at"`
	By      string           `json:"by"`
	Subject string           `json:"subject"`
	Docs    []SpaceDocChange `json:"docs"`
}

// SpaceDocChange is a document changed: added, changed or removed.
type SpaceDocChange struct {
	Path  string        `json:"path"`
	Title string        `json:"title,omitempty"`
	How   ops.DocChange `json:"how"`
}

// SpaceFlight is a capsule with changes to a space's documents.
type SpaceFlight struct {
	Capsule string           `json:"capsule"`
	Branch  string           `json:"branch,omitempty"`
	Mission string           `json:"mission,omitempty"`
	PR      *SpacePR         `json:"pr,omitempty"`
	Docs    []SpaceDocChange `json:"docs"`
}

// SpacePR is a pull request open from a capsule.
type SpacePR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url,omitempty"`
}

// SpaceFresh is how a space's ground stands with its remote.
type SpaceFresh struct {
	Fetched *time.Time `json:"fetched,omitempty"` // never, here: absent
	Behind  int        `json:"behind"`
	Ahead   int        `json:"ahead,omitempty"`
	Branch  string     `json:"branch"`
	Dirty   bool       `json:"dirty,omitempty"`
}

func freshOf(f ops.SpaceFresh) SpaceFresh {
	out := SpaceFresh{Behind: f.Behind, Ahead: f.Ahead, Branch: f.Branch, Dirty: f.Dirty}
	if !f.Fetched.IsZero() {
		out.Fetched = &f.Fetched
	}
	return out
}

func changesOf(cs []ops.SpaceDocChange) []SpaceDocChange {
	out := make([]SpaceDocChange, 0, len(cs))
	for _, c := range cs {
		out = append(out, SpaceDocChange(c))
	}
	return out
}

// spaceActivity is what's happening in a space.
func (s *Server) spaceActivity(w http.ResponseWriter, r *http.Request) {
	a, err := s.App.SpaceActivity(r.Context(), r.PathValue("repo"))
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
		return
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := SpaceActivity{Lately: []SpaceCommit{}, InFlight: []SpaceFlight{}, Fresh: freshOf(a.Fresh), Unread: a.Unread}
	for _, c := range a.Lately {
		out.Lately = append(out.Lately, SpaceCommit{SHA: c.SHA, At: c.At, By: c.By, Subject: c.Subject, Docs: changesOf(c.Docs)})
	}
	for _, f := range a.InFlight {
		sf := SpaceFlight{Capsule: f.Capsule, Branch: f.Branch, Mission: f.Mission, Docs: changesOf(f.Docs)}
		if f.PR != nil {
			sf.PR = &SpacePR{Number: f.PR.Number, Title: f.PR.Title, URL: f.PR.URL}
		}
		out.InFlight = append(out.InFlight, sf)
	}
	writeJSON(w, http.StatusOK, out)
}

// fetchSpace brings a space's default branch up to date in its repo's
// clone, and says how its ground stands then.
func (s *Server) fetchSpace(w http.ResponseWriter, r *http.Request) {
	f, err := s.App.FetchSpace(r.Context(), r.PathValue("repo"))
	s.freshReply(w, f, err)
}

// pullSpace moves a space's ground forward to its remote's, as fetched.
func (s *Server) pullSpace(w http.ResponseWriter, r *http.Request) {
	f, err := s.App.PullSpace(r.Context(), r.PathValue("repo"))
	s.freshReply(w, f, err)
}

func (s *Server) freshReply(w http.ResponseWriter, f ops.SpaceFresh, err error) {
	var refused *ops.Refused
	var up *ops.Upstream
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.As(err, &refused):
		writeJSON(w, http.StatusConflict, map[string]any{"error": refused.Reason, "code": refused.Code, "fresh": freshOf(f)})
	case errors.As(err, &up):
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": up.Error(), "fresh": freshOf(f)})
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, map[string]any{"fresh": freshOf(f)})
	}
}

// DocHit is a document a search found, led to the section that says it
// best: its snippet's words found marked.
type DocHit struct {
	Repo    string       `json:"repo"`
	Label   string       `json:"label"`
	Path    string       `json:"path"`
	Title   string       `json:"title"`
	Anchor  string       `json:"anchor,omitempty"`
	Heading string       `json:"heading,omitempty"`
	Snippet []space.Part `json:"snippet"`
	Score   float64      `json:"score"`
	Every   bool         `json:"every,omitempty"` // every word was found in it
	Kind    string       `json:"kind,omitempty"`
	State   string       `json:"state,omitempty"`
}

func hitOf(h ops.DocHit) DocHit {
	snip := h.Snippet
	if snip == nil {
		snip = []space.Part{}
	}
	return DocHit{Repo: h.Repo, Label: h.Label, Path: h.Path, Title: h.Title, Anchor: h.Anchor, Heading: h.Heading, Snippet: snip, Score: h.Score, Every: h.Every, Kind: h.Kind, State: h.State}
}

// searchDocuments finds documents in the workspace's spaces by what they
// say (q), best first: in the spaces asked (repo, repeated), of a kind
// and where it stands.
func (s *Server) searchDocuments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	hits, more, err := s.App.SearchDocuments(r.Context(), ops.DocQuery{Words: q.Get("q"), Repos: q["repo"], Kind: q.Get("kind"), State: q.Get("state"), Limit: limit})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]DocHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, hitOf(h))
	}
	writeJSON(w, http.StatusOK, map[string]any{"hits": out, "more": more})
}

// Citation is a document citing another, where, and how (a link, or a
// ref).
type Citation struct {
	Repo  string      `json:"repo"`
	Path  string      `json:"path"`
	Title string      `json:"title,omitempty"`
	Line  int         `json:"line"`
	How   CitationHow `json:"how"`
}

func citationsOf(cs []space.Citation) []Citation {
	out := make([]Citation, 0, len(cs))
	for _, c := range cs {
		out = append(out, Citation{Repo: c.Repo, Path: c.Path, Title: c.Title, Line: c.Line, How: CitationHow(c.How)})
	}
	return out
}

// docCites is what cites a space's document (path), and what it cites.
func (s *Server) docCites(w http.ResponseWriter, r *http.Request) {
	by, cites, err := s.App.DocCitations(r.Context(), r.PathValue("repo"), r.URL.Query().Get("path"))
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, map[string]any{"citedBy": citationsOf(by), "cites": citationsOf(cites)})
	}
}
