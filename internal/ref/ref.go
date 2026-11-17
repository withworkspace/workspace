// Package ref is ws's one grammar for naming things in words: a file, some lines of it, a folder, a
// repo, a capsule's file, a pull request, a mission, a page, a conversation, an artifact or a version of
// one. What a name is written as, what it's read back as, and its address (on GitHub, or ws: for what
// only ws has).
//
// The web UI has the same grammar in TypeScript (web/src/refs/refs.ts).
// Both are tested against one file of cases (testdata/cases.json), so they
// can't drift apart.
//
// It imports nothing of ws's: what there is to name is given to it
// (Known), by whoever asks.
package ref

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Ground is a repo's default branch, checked out read-only: a Ref's
// Worktree when it's not a capsule's.
const Ground = ".ground"

// Kind is what a ref names.
type Kind string

const (
	File    Kind = "file"
	Dir     Kind = "dir"
	Repo    Kind = "repo"
	PR      Kind = "pr"
	Mission Kind = "mission"
	Page    Kind = "page"
	Talk    Kind = "talk"
	// Artifact is a versioned HTML prototype (internal/artifact): a3, and
	// a3.v2 for one of its versions.
	Artifact Kind = "artifact"
)

// Ref is one thing named. Its JSON is the web UI's PageRef.
type Ref struct {
	Kind     Kind   `json:"kind"`
	Repo     string `json:"repo,omitempty"`
	Worktree string `json:"worktree,omitempty"` // Ground, or a capsule's name (files, folders)
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	EndLine  int    `json:"endLine,omitempty"`
	Number   int    `json:"number,omitempty"`  // a pull request's
	Slug     string `json:"slug,omitempty"`    // a mission's
	ID       string `json:"id,omitempty"`      // a page's, a conversation's or an artifact's
	Version  int    `json:"version,omitempty"` // an artifact's version (a3.v2), 0 for the artifact
	// Commit is the commit its lines are of, when it's pinned to one: what
	// they meant when it was written.
	Commit string `json:"commit,omitempty"`
	// Anchor is a heading in a file, as an address names one (#decision).
	Anchor string `json:"anchor,omitempty"`
}

// InCapsule is whether it's a file or folder in a capsule.
func (r Ref) InCapsule() bool {
	return (r.Kind == File || r.Kind == Dir) && r.Worktree != "" && r.Worktree != Ground
}

// Name is how it's written: demo/main.go:12-20, demo/token-cache:a.go,
// demo/internal/, demo, demo#12, a mission's slug, p3, w4, a3, a3.v2.
func (r Ref) Name() string {
	switch r.Kind {
	case File, Dir:
		where := r.Repo + "/"
		if r.InCapsule() {
			where = r.Repo + "/" + r.Worktree + ":"
		}
		if r.Kind == Dir {
			return where + r.Path + "/"
		}
		return where + r.Path + lineSuffix(r.Line, r.EndLine)
	case Repo:
		return r.Repo
	case PR:
		return r.Repo + "#" + strconv.Itoa(r.Number)
	case Mission:
		return r.Slug
	case Page, Talk:
		return r.ID
	case Artifact:
		if r.Version > 0 {
			return r.ID + ".v" + strconv.Itoa(r.Version)
		}
		return r.ID
	}
	return ""
}

func lineSuffix(line, end int) string {
	switch {
	case line <= 0:
		return ""
	case end > line:
		return fmt.Sprintf(":%d-%d", line, end)
	}
	return ":" + strconv.Itoa(line)
}

// --- Names, read one at a time

var (
	prName = regexp.MustCompile(`^([\w.-]+)#([1-9]\d{0,8})$`)
	idName = regexp.MustCompile(`^([pw])([1-9]\d{0,4})$`)
	// An artifact, or a version of one, as internal/artifact names them.
	artifactName = regexp.MustCompile(`^(a[1-9]\d{0,6})(?:\.v([1-9]\d{0,4}))?$`)
	slugName     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	linesSuffix  = regexp.MustCompile(`^(.*?):(\d+)(?:-(\d+))?$`)
	// A heading's name, as GitHub makes one (web/src/markdown/links.ts).
	anchorName = regexp.MustCompile(`^[\p{L}\p{M}\p{N}\p{Pc}-]+$`)
)

// SplitLines takes :12 or :12-20 off the end of a name.
func SplitLines(s string) (rest string, line, end int) {
	m := linesSuffix.FindStringSubmatch(s)
	if m == nil {
		return s, 0, 0
	}
	line, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		end, _ = strconv.Atoi(m[3])
	}
	if line == 0 {
		return s, 0, 0
	}
	if end <= line {
		end = 0
	}
	return m[1], line, end
}

// ParsePR reads a pull request's name, demo#12.
func ParsePR(s string) (repo string, number int, ok bool) {
	m := prName.FindStringSubmatch(s)
	if m == nil {
		return "", 0, false
	}
	n, _ := strconv.Atoi(m[2])
	return m[1], n, true
}

// Parse reads one name by its form alone, without knowing whether what it
// names is there: for a ws: address, a document's stored ref, a name on
// the command line. A word with no slash is a mission's slug, as only ws
// has those; demo:README.md and records:adr/0001.md (a document's stored
// form: a colon before any slash) are files on the default branch, as
// demo/README.md and records/adr/0001.md are. A colon after the first
// slash is a capsule's: records/x:adr/0001.md. A file's name may end in a
// heading in it, as the reader's links to one are written
// (demo/adr/0026.md#decision).
func Parse(s string) (Ref, bool) {
	s = strings.TrimSpace(s)
	if name, anchor, ok := strings.Cut(s, "#"); ok && anchorName.MatchString(anchor) {
		if r, ok := parse(name); ok && r.Kind == File && r.Line == 0 {
			r.Anchor = anchor
			return r, true
		}
	}
	return parse(s)
}

func parse(s string) (Ref, bool) {
	if s == "" || strings.ContainsAny(s, " \t\n") {
		return Ref{}, false
	}
	if repo, n, ok := ParsePR(s); ok {
		return Ref{Kind: PR, Repo: repo, Number: n}, true
	}
	if m := idName.FindStringSubmatch(s); m != nil {
		if m[1] == "p" {
			return Ref{Kind: Page, ID: s}, true
		}
		return Ref{Kind: Talk, ID: s}, true
	}
	if m := artifactName.FindStringSubmatch(s); m != nil {
		v, _ := strconv.Atoi(m[2])
		return Ref{Kind: Artifact, ID: m[1], Version: v}, true
	}
	// demo:README.md or records:adr/0001.md:12-20, as a document's ref is
	// stored: what's after the colon is the path, folders and all. Not
	// https://…, whose colon is followed by a slash.
	if repo, p, ok := strings.Cut(s, ":"); ok && repo != "" && p != "" && !strings.Contains(repo, "/") && !strings.HasPrefix(p, "/") {
		if dir, ok := strings.CutSuffix(p, "/"); ok {
			return Ref{Kind: Dir, Repo: repo, Worktree: Ground, Path: dir}, true
		}
		p, line, end := SplitLines(p)
		return Ref{Kind: File, Repo: repo, Worktree: Ground, Path: p, Line: line, EndLine: end}, true
	}
	head, rest, slash := strings.Cut(s, "/")
	if !slash {
		if slugName.MatchString(s) {
			return Ref{Kind: Mission, Slug: s}, true
		}
		return Ref{}, false
	}
	if head == "" || rest == "" {
		return Ref{}, false
	}
	// A capsule's: demo/token-cache:internal/cache.go.
	if capsule, p, ok := strings.Cut(rest, ":"); ok && !strings.Contains(capsule, "/") {
		if _, err := strconv.Atoi(strings.SplitN(p, "-", 2)[0]); err != nil && p != "" {
			if dir, ok := strings.CutSuffix(p, "/"); ok {
				return Ref{Kind: Dir, Repo: head, Worktree: capsule, Path: dir}, true
			}
			p, line, end := SplitLines(p)
			return Ref{Kind: File, Repo: head, Worktree: capsule, Path: p, Line: line, EndLine: end}, true
		}
	}
	if dir, ok := strings.CutSuffix(rest, "/"); ok {
		return Ref{Kind: Dir, Repo: head, Worktree: Ground, Path: dir}, true
	}
	p, line, end := SplitLines(rest)
	return Ref{Kind: File, Repo: head, Worktree: Ground, Path: p, Line: line, EndLine: end}, true
}

// --- Addresses

// Where is what an address needs that a ref doesn't say: the GitHub org,
// the default branch, and (for a capsule's file) the branch it's pushed
// to, "" while it isn't.
type Where struct {
	Org           string
	DefaultBranch string
	Branch        string
}

// Address is where it is outside ws: on GitHub, or ws:<name> for what only
// ws has (a mission, a page, a capsule that isn't pushed). Lines are of
// the commit they're pinned to, where there is one.
func (r Ref) Address(w Where) string {
	gh := "https://github.com/" + w.Org + "/" + r.Repo
	switch r.Kind {
	case Repo:
		return gh
	case PR:
		return gh + "/pull/" + strconv.Itoa(r.Number)
	case File, Dir:
		rev := w.DefaultBranch
		if r.InCapsule() {
			if w.Branch == "" {
				return "ws:" + r.Name() + r.heading()
			}
			rev = w.Branch
		}
		if r.Commit != "" {
			rev = r.Commit
		}
		if r.Kind == Dir {
			return gh + "/tree/" + escapePath(rev) + "/" + escapePath(r.Path)
		}
		frag := ""
		if r.Anchor != "" {
			frag = "#" + r.Anchor
		}
		if r.Line > 0 {
			frag = "#L" + strconv.Itoa(r.Line)
			if r.EndLine > r.Line {
				frag += "-L" + strconv.Itoa(r.EndLine)
			}
		}
		return gh + "/blob/" + escapePath(rev) + "/" + escapePath(r.Path) + frag
	}
	return "ws:" + r.Name()
}

// heading is #<anchor> for a file named at a heading, as a ws: address
// ends.
func (r Ref) heading() string {
	if r.Kind != File || r.Line > 0 || r.Anchor == "" {
		return ""
	}
	return "#" + r.Anchor
}

func escapePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

var (
	githubURL = regexp.MustCompile(`^https?://(?:www\.)?github\.com/([^/\s?#]+)/([^/\s?#]+)(?:/([^\s?#]*))?(?:\?[^\s#]*)?(?:#(\S*))?$`)
	linesFrag = regexp.MustCompile(`^L(\d+)(?:C\d+)?(?:-L(\d+)(?:C\d+)?)?$`)
	shaShape  = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

// GitHub is an address on GitHub, read: whose and which repo, what of it
// (pull, issues, blob, tree, commit, or "" for the repo), the rest of its
// path in segments, unescaped, and the lines its fragment names.
type GitHub struct {
	Org, Repo, What string
	Segs            []string
	Line, End       int
	Anchor          string // the fragment, when it isn't lines
}

// ParseGitHub reads an address on GitHub.
func ParseGitHub(addr string) (GitHub, bool) {
	m := githubURL.FindStringSubmatch(addr)
	if m == nil {
		return GitHub{}, false
	}
	g := GitHub{Org: m[1], Repo: strings.TrimSuffix(m[2], ".git")}
	if rest := strings.Trim(m[3], "/"); rest != "" {
		segs := strings.Split(rest, "/")
		for i, s := range segs {
			if u, err := url.PathUnescape(s); err == nil {
				segs[i] = u
			}
		}
		g.What, g.Segs = segs[0], segs[1:]
	}
	if f := linesFrag.FindStringSubmatch(m[4]); f != nil {
		g.Line, _ = strconv.Atoi(f[1])
		if f[2] != "" {
			g.End, _ = strconv.Atoi(f[2])
		}
	} else {
		g.Anchor = m[4]
	}
	return g, true
}

// Splits is each way a blob's or tree's segments may be a revision and a
// path, the shortest revision first: a branch's name may have slashes in
// it, so which is meant is whichever names something that's there.
func (g GitHub) Splits() [][2]string {
	var out [][2]string
	for i := 1; i < len(g.Segs); i++ {
		out = append(out, [2]string{strings.Join(g.Segs[:i], "/"), strings.Join(g.Segs[i:], "/")})
	}
	return out
}

// IsCommit is whether a revision looks like a commit, not a branch.
func IsCommit(rev string) bool { return shaShape.MatchString(rev) }

// ParseAddress reads an address back into what it names: one of the
// workspace's on GitHub (its org, a repo it has), or a ws: one. What an
// address names needn't be there now (a capsule burned since): it was
// named on purpose, so it's read as written.
func ParseAddress(addr string, k *Known) (Ref, bool) {
	if name, ok := strings.CutPrefix(addr, "ws:"); ok {
		return Parse(name)
	}
	g, ok := ParseGitHub(addr)
	if !ok || (k.Org != "" && !strings.EqualFold(g.Org, k.Org)) {
		return Ref{}, false
	}
	if _, known := k.Repos[g.Repo]; !known {
		return Ref{}, false
	}
	switch g.What {
	case "":
		if len(g.Segs) == 0 {
			return Ref{Kind: Repo, Repo: g.Repo}, true
		}
	case "pull":
		if len(g.Segs) >= 1 {
			if n, err := strconv.Atoi(g.Segs[0]); err == nil && n > 0 {
				return Ref{Kind: PR, Repo: g.Repo, Number: n}, true
			}
		}
	case "blob", "tree":
		kind := File
		if g.What == "tree" {
			kind = Dir
		}
		splits := g.Splits()
		pick := func(rev, p string) Ref {
			r := Ref{Kind: kind, Repo: g.Repo, Worktree: Ground, Path: p}
			switch {
			case IsCommit(rev):
				r.Commit = rev
			case rev == k.DefaultBranch:
			default:
				if c := k.Branches[g.Repo][rev]; c != "" {
					r.Worktree = c
				}
			}
			if kind == File {
				r.Anchor = g.Anchor
			}
			if kind == File && g.Line > 0 {
				r.Line, r.EndLine = g.Line, g.End
				if r.EndLine <= r.Line {
					r.EndLine = 0
				}
			}
			return r
		}
		for _, s := range splits {
			if IsCommit(s[0]) || s[0] == k.DefaultBranch || k.Branches[g.Repo][s[0]] != "" {
				return pick(s[0], s[1]), true
			}
		}
		if len(splits) > 0 {
			return pick(splits[0][0], splits[0][1]), true
		}
	}
	return Ref{}, false
}

// Pinned is a name as it was when it was said or written: what it named
// (a file's lines with the commit they were of; a capsule's file with the
// commit the capsule was at), the branch a capsule was on and whether it
// was pushed. Said is the words it was found in. A conversation's turn
// keeps these, so what it said can be drawn as it was meant once the
// capsule's burned or the lines have moved.
type Pinned struct {
	Said   string `json:"said"`
	Ref    Ref    `json:"ref"`
	Branch string `json:"branch,omitempty"`
	Pushed bool   `json:"pushed,omitempty"`
}
