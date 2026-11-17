package find

import (
	"context"
	"strings"
	"time"
)

// Kinds of thing a result is, and of group it's in.
const (
	KindRef      = "ref"      // a group: what's typed, read as a name or an address
	KindPulls    = "pulls"    // a group of pull requests
	KindFiles    = "files"    // a group of files
	KindCapsules = "capsules" // a group of capsules, by their names
	KindFile     = "file"
	KindDir      = "dir"
	KindPull     = "pr"
	KindCapsule  = "capsule"
)

// Result is one thing found. Ref names it as ws does (demo/main.go:12-20,
// demo/token-cache:internal/cache.go, demo#12); Title is what's shown,
// with the letters that matched At.
type Result struct {
	Kind  string  `json:"kind"`
	Ref   string  `json:"ref"`
	Title string  `json:"title"`
	At    []int   `json:"at,omitempty"`
	Hint  string  `json:"hint,omitempty"`
	Score float64 `json:"score"`
	// Exact: what was typed names this, and nothing else.
	Exact bool `json:"exact,omitempty"`
	// Anchor is where in a document it's found: the heading of the
	// section that says it best ("" for its top).
	Anchor string `json:"anchor,omitempty"`

	// A file or a folder: its repo, its worktree (".ground", or a
	// capsule), and its path there; the lines asked for; for a capsule's
	// own file, how it stands there; for a ground's, the capsules that
	// have changed it.
	Repo     string    `json:"repo,omitempty"`
	Worktree string    `json:"worktree,omitempty"`
	Path     string    `json:"path,omitempty"`
	Line     int       `json:"line,omitempty"`
	EndLine  int       `json:"endLine,omitempty"`
	Status   string    `json:"status,omitempty"`
	In       []Changed `json:"in,omitempty"`

	// A pull request.
	Number int    `json:"number,omitempty"`
	Author string `json:"author,omitempty"`
	Branch string `json:"branch,omitempty"`
	State  string `json:"state,omitempty"`
	Words  string `json:"words,omitempty"`
	Checks string `json:"checks,omitempty"`
	Draft  bool   `json:"draft,omitempty"`
	Mine   bool   `json:"mine,omitempty"`
	Stack  string `json:"stack,omitempty"`
	// Asks: it isn't among the pull requests in hand, so GitHub is asked
	// when it's chosen.
	Asks bool `json:"asks,omitempty"`
}

// Changed is a capsule that's changed a file, and how.
type Changed struct {
	Capsule string `json:"capsule"`
	Status  string `json:"status"`
}

// Group is the results of one kind, the best first.
type Group struct {
	Kind    string   `json:"kind"`
	Results []Result `json:"results"`
	// More is how many matched beyond these.
	More int `json:"more,omitempty"`
	// Reading is how many capsules hadn't been read when this was made:
	// their files may be missing from it.
	Reading int `json:"reading,omitempty"`
	// At is when the pull requests were last read from GitHub.
	At *time.Time `json:"at,omitempty"`
	// Problem is why this kind couldn't be looked through.
	Problem string `json:"problem,omitempty"`
}

// Request is something to find: what's typed, the kinds wanted (none:
// all of them), and how many of each.
type Request struct {
	Q     string
	Kinds []string
	Limit int
}

func (r Request) wants(kind string) bool {
	if len(r.Kinds) == 0 {
		return true
	}
	for _, k := range r.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// Limits on how many of a kind are answered.
const (
	DefaultLimit = 8
	MaxLimit     = 200
)

// Search finds what's asked for, and gives each kind's group to emit as
// it's ready: pull requests first (they're in hand), then what's typed
// read as a ref, then capsules by name, then files. It stops when ctx
// ends.
func (ix *Index) Search(ctx context.Context, req Request, emit func(Group)) error {
	q := strings.TrimSpace(req.Q)
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	var pulls []Pull
	var at time.Time
	if ix.Pulls != nil {
		pulls, at = ix.Pulls()
	}
	if req.wants(KindPulls) && ix.Pulls != nil {
		g := Group{Kind: KindPulls}
		g.Results, g.More = searchPulls(pulls, q, limit)
		if !at.IsZero() {
			g.At = &at
		}
		emit(g)
	}
	if !req.wants(KindRef) && !req.wants(KindFiles) && !req.wants(KindCapsules) {
		return ctx.Err()
	}
	snap, err := ix.fresh(ctx, capsuleWait)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if req.wants(KindFiles) {
			emit(Group{Kind: KindFiles, Results: []Result{}, Problem: err.Error()})
		}
		return nil
	}
	if req.wants(KindRef) {
		g := Group{Kind: KindRef, Results: []Result{}}
		if r := ix.exact(q, snap, pulls); r != nil {
			g.Results = append(g.Results, *r)
		}
		emit(g)
	}
	if req.wants(KindCapsules) {
		emit(Group{Kind: KindCapsules, Results: searchCapsules(snap, q, limit)})
	}
	if req.wants(KindFiles) {
		g := Group{Kind: KindFiles, Reading: snap.reading}
		g.Results, g.More = searchFiles(ctx, snap, q, limit)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		emit(g)
	}
	return nil
}
