package find

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/ref"
)

// What's typed may name something outright: a file by its repo and path
// (demo/main.go:12-20), a capsule's (demo/token-cache:internal/cache.go),
// one by where it is on disk, a pull request (demo#12), or any of those
// by its address on GitHub. That's read here, before anything is matched
// by its letters, in ws's one grammar (internal/ref); what's here is
// whether it's there.

// exact is what q names, if it names one thing that's there.
func (ix *Index) exact(q string, snap *snapshot, pulls []Pull) *Result {
	if q == "" || strings.ContainsAny(q, " \t\n") {
		return nil
	}
	var r *Result
	if g, ok := ref.ParseGitHub(q); ok {
		r = ix.exactURL(g, snap, pulls)
	} else if repo, n, ok := ref.ParsePR(q); ok {
		r = exactPull(repo, n, snap, pulls)
	} else {
		r = exactFile(q, snap)
	}
	if r != nil {
		r.Exact, r.Score = true, scoreExact
	}
	return r
}

// knows is whether repo is one of the workspace's.
func knows(repo string, snap *snapshot, pulls []Pull) bool {
	if snap.ground(repo) != nil {
		return true
	}
	for _, p := range pulls {
		if p.Repo == repo {
			return true
		}
	}
	return false
}

// exactPull is a pull request by its repo and number: as the list has it,
// or, when it hasn't, one to ask GitHub for when it's chosen.
func exactPull(repo string, number int, snap *snapshot, pulls []Pull) *Result {
	if !knows(repo, snap, pulls) {
		return nil
	}
	for _, p := range pulls {
		if p.Repo == repo && p.Number == number {
			r := pullResult(p)
			return &r
		}
	}
	ref := repo + "#" + strconv.Itoa(number)
	return &Result{Kind: KindPull, Ref: ref, Title: "Open " + ref, Hint: "not among the open ones here: GitHub's asked", Repo: repo, Number: number, Asks: true}
}

// exactURL reads an address on GitHub: a pull request's, or a file's or a
// folder's at some branch or commit.
func (ix *Index) exactURL(g ref.GitHub, snap *snapshot, pulls []Pull) *Result {
	if ix.Org != "" && !strings.EqualFold(g.Org, ix.Org) {
		return nil
	}
	if !knows(g.Repo, snap, pulls) || len(g.Segs) == 0 {
		return nil
	}
	switch g.What {
	case "pull", "issues":
		n, err := strconv.Atoi(g.Segs[0])
		if err != nil || n <= 0 {
			return nil
		}
		r := exactPull(g.Repo, n, snap, pulls)
		if r != nil && r.Asks && g.What == "issues" {
			return nil // an issue, as far as anyone here knows
		}
		return r
	case "blob", "tree":
		// A branch's name may have slashes in it, so where it ends and the
		// path starts is whichever split names something that's there.
		for _, s := range g.Splits() {
			if c := snap.capsule(g.Repo, s[0]); c != nil {
				if r := inWorktree(snap, g.Repo, c.wt.Name, s[1], g.Line, g.End); r != nil {
					return r
				}
			}
			if r := inWorktree(snap, g.Repo, Ground, s[1], g.Line, g.End); r != nil {
				return r
			}
		}
	}
	return nil
}

// exactFile reads a file's name: by its repo and path, as a capsule's, or
// by where it is on disk.
func exactFile(q string, snap *snapshot) *Result {
	q, line, end := splitLines(q)
	// On disk: under a worktree's folder, whole or from the workspace's root.
	for _, wt := range snap.worktrees() {
		for _, dir := range []string{wt.Dir, path.Join("repos", wt.Repo, wt.Name)} {
			if p, ok := strings.CutPrefix(filepath.ToSlash(q), filepath.ToSlash(dir)+"/"); ok && dir != "" {
				return inWorktree(snap, wt.Repo, wt.Name, p, line, end)
			}
		}
	}
	// By its name: demo/main.go, a capsule's demo/token-cache:a.go.
	r, ok := ref.Parse(q)
	if !ok || (r.Kind != ref.File && r.Kind != ref.Dir) {
		return nil
	}
	p := r.Path
	if r.Kind == ref.Dir {
		p += "/"
	}
	if r.InCapsule() && snap.capsule(r.Repo, r.Worktree) == nil {
		return nil
	}
	return inWorktree(snap, r.Repo, r.Worktree, p, line, end)
}

// worktrees are those the snapshot has read.
func (s *snapshot) worktrees() []Worktree {
	out := make([]Worktree, 0, len(s.grounds)+len(s.capsules))
	for _, g := range s.grounds {
		out = append(out, Worktree{Repo: g.repo, Name: Ground, Dir: g.dir})
	}
	for _, c := range s.capsules {
		out = append(out, c.wt)
	}
	return out
}

// inWorktree is a file or a folder in a worktree, if it's there.
func inWorktree(snap *snapshot, repo, worktree, p string, line, end int) *Result {
	dir := strings.HasSuffix(p, "/")
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") || path.IsAbs(p) {
		return nil
	}
	isDir := false
	if worktree == Ground {
		g := snap.ground(repo)
		switch {
		case g == nil:
			return nil
		case !dir && g.has(p):
		case g.hasDir(p):
			isDir = true
		default:
			return nil
		}
	} else {
		c := snap.capsule(repo, worktree)
		if c == nil {
			return nil
		}
		info, err := os.Lstat(filepath.Join(c.wt.Dir, filepath.FromSlash(p)))
		if err != nil {
			return nil
		}
		isDir = info.IsDir()
	}
	if isDir {
		ref := repo + "/" + p + "/"
		if worktree != Ground {
			ref = repo + "/" + worktree + ":" + p + "/"
		}
		return &Result{Kind: KindDir, Ref: ref, Title: ref, Repo: repo, Worktree: worktree, Path: p}
	}
	r := fileResult(snap, repo, worktree, p, line, end)
	if c := snap.capsule(repo, worktree); c != nil {
		r.Status = c.status[p]
	}
	return &r
}
