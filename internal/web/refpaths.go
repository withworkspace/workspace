package web

import (
	"fmt"
	"hash/fnv"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// What a page's words name on a repo's default branch (demo/main.go, a
// folder) is checked when it's shown, path by path, against the files the
// find index keeps of each repo (find.Index.GroundsKept): no repo's whole
// tree is sent for it, and nothing waits on git.
//
//	POST /api/refs/paths {"repos": {"demo": ["main.go", "internal"]}}
//
// answers, for each repo whose files are kept, the commit they were read
// at and which of the paths are files and which folders: any other isn't
// there. A repo that's never been read isn't in the answer: its files are
// read behind it, and the workspace's groundsSeen moves when they have.

// RefPaths is what a page asks of the repos' default branches: paths in
// each, by repo.
type RefPaths struct {
	Repos map[string][]string `json:"repos"`
}

// RefPathsFound is what's there of what was asked, by repo: those whose
// files are kept.
type RefPathsFound struct {
	Repos map[string]RepoPaths `json:"repos"`
}

// RepoPaths is what's there on a repo's default branch, of what was asked:
// the files and the folders, as read at Head.
type RepoPaths struct {
	Head  string   `json:"head"`
	Files []string `json:"files"`
	Dirs  []string `json:"dirs"`
}

// refPathsMax is the most paths one request checks.
const refPathsMax = 4000

func (s *Server) refPaths(w http.ResponseWriter, r *http.Request) {
	var req RefPaths
	if !readBody(w, r, &req) {
		return
	}
	n := 0
	for _, ps := range req.Repos {
		n += len(ps)
	}
	if n > refPathsMax {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("too many paths (%d): at most %d at once", n, refPathsMax))
		return
	}
	out := RefPathsFound{Repos: map[string]RepoPaths{}}
	ix := s.findIndex()
	if ix == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	grounds := ix.GroundsKept()
	for repo, paths := range req.Repos {
		g, ok := grounds[repo]
		if !ok {
			continue
		}
		found := RepoPaths{Head: g.Head(), Files: []string{}, Dirs: []string{}}
		for _, p := range paths {
			p = strings.TrimSuffix(p, "/")
			switch {
			case p == "":
			case g.Has(p):
				found.Files = append(found.Files, p)
			case g.HasDir(p):
				found.Dirs = append(found.Dirs, p)
			}
		}
		out.Repos[repo] = found
	}
	writeJSON(w, http.StatusOK, out)
}

// groundsSeen names the repos' default branches as kept for refs: how many
// times the index has found one other than it was, and where each kept
// one's HEAD is, which moves with it (a sync) before the index has read it
// again. It starts no process.
func (s *Server) groundsSeen() string {
	ix := s.findIndex()
	if ix == nil {
		return ""
	}
	h := fnv.New64a()
	fmt.Fprintf(h, "%d\n", ix.Seen())
	read := ix.GroundsRead()
	for _, repo := range slices.Sorted(maps.Keys(read)) {
		fmt.Fprintf(h, "%s %s\n", repo, headStamp(read[repo].Dir()))
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// headStamp names where a worktree's HEAD is, from git's own files: its
// HEAD and its HEAD's log (a commit, a reset, a merge). Not its refs on
// origin, which a fetch moves while the worktree stays where it is.
func headStamp(dir string) string {
	gitDir := filepath.Join(dir, ".git")
	if data, err := os.ReadFile(gitDir); err == nil {
		at, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
		if !ok {
			return ""
		}
		if !filepath.IsAbs(at) {
			at = filepath.Join(dir, at)
		}
		gitDir = at
	}
	var b strings.Builder
	for _, f := range []string{"HEAD", filepath.Join("logs", "HEAD")} {
		if info, err := os.Stat(filepath.Join(gitDir, f)); err == nil {
			fmt.Fprintf(&b, "%d %d ", info.Size(), info.ModTime().UnixNano())
		}
	}
	return b.String()
}

// keptStamp is what the workspace's view says of what's kept behind
// pages' reads (groundsSeen, readsSeen), cheaply: counters, and a stamp of
// each kept ground.
func (s *Server) keptStamp() string {
	stamp := s.groundsSeen() + " " + seenOf(s.reads.Seen())
	// The pull requests listed, which the view's needs and its Reviews
	// count are from: the daemon asks GitHub of them behind.
	if s.Reviews != nil {
		stamp += " " + seenOf(s.Reviews.Seen())
	}
	return stamp
}

// seenOf is a counter as a view says it: none for 0.
func seenOf(n int64) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprint(n)
}
