package find

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/withworkspace/workspace/internal/ref"
)

// splitLines takes the lines off the end of what's typed.
func splitLines(q string) (rest string, line, end int) { return ref.SplitLines(q) }

// fileRef names a file as ws does: demo/main.go on the default branch,
// demo/token-cache:internal/cache.go in a capsule, with its lines after.
func fileRef(repo, worktree, path string, line, end int) string {
	return ref.Ref{Kind: ref.File, Repo: repo, Worktree: worktree, Path: path, Line: line, EndLine: end}.Name()
}

// hit is a file that matched, kept while it's among the best.
type hit struct {
	score                        float64
	repo, worktree, path, status string
	at                           []int
	size                         int // of its title
}

func (a *hit) before(b *hit) bool {
	switch {
	case a.score != b.score:
		return a.score > b.score
	case a.size != b.size:
		return a.size < b.size
	case a.repo != b.repo:
		return a.repo < b.repo
	case a.worktree != b.worktree:
		return a.worktree < b.worktree
	}
	return a.path < b.path
}

// top keeps the best few of what's offered, in order.
type top struct {
	limit int
	hits  []hit
	total int
}

// offer takes a match if it's among the best so far. Its at is copied
// only then.
func (t *top) offer(h hit) {
	t.total++
	if len(t.hits) == t.limit && !h.before(&t.hits[len(t.hits)-1]) {
		return
	}
	h.at = append([]int(nil), h.at...)
	i := sort.Search(len(t.hits), func(i int) bool { return h.before(&t.hits[i]) })
	if len(t.hits) < t.limit {
		t.hits = append(t.hits, hit{})
	}
	copy(t.hits[i+1:], t.hits[i:])
	t.hits[i] = h
}

// searchFiles is the files that match: each of the default branch's once,
// with the capsules that have changed it, and what only a capsule has as
// its own. With nothing typed, it's what capsules have changed.
func searchFiles(ctx context.Context, snap *snapshot, q string, limit int) ([]Result, int) {
	rest, line, end := splitLines(q)
	query := NewQuery(rest)
	if query.Empty() {
		return changedFiles(snap, limit)
	}
	// Each repo is looked through on its own, at once, and the best of
	// each put together.
	tops := make([]*top, len(snap.grounds))
	var wg sync.WaitGroup
	for n, g := range snap.grounds {
		tops[n] = &top{limit: limit}
		wg.Add(1)
		go func(g *groundData, t *top) {
			defer wg.Done()
			s := &Scratch{}
			for i, p := range g.paths {
				if i&0xfff == 0 && ctx.Err() != nil {
					return
				}
				if !query.Covers(g.masks[i] | g.repoMask) {
					continue
				}
				s.Text = append(append(append(s.Text[:0], g.repo...), '/'), p...)
				m, ok := query.MatchBytes(s.Text, s)
				if !ok {
					continue
				}
				t.offer(hit{score: m.Score, repo: g.repo, worktree: Ground, path: p, at: m.At, size: len(s.Text)})
			}
		}(g, tops[n])
	}
	wg.Wait()
	if ctx.Err() != nil {
		return nil, 0
	}
	t := &top{limit: limit}
	for _, each := range tops {
		for _, h := range each.hits {
			t.offer(h)
		}
		t.total += each.total - len(each.hits)
	}
	s := &Scratch{}
	// A capsule's own file is found by its repo and path, as any file is:
	// its capsule's name is said of it, and isn't letters to match, or
	// every file of a capsule called cache would be found by "cache". With
	// a colon typed, it's the capsule that's being asked for by name
	// (token-cache:store), and the name counts.
	named := strings.Contains(rest, ":")
	for _, c := range snap.capsules {
		g := snap.ground(c.wt.Repo)
		nameMask := Mask(c.wt.Repo) | maskOf('/')
		if named {
			nameMask |= Mask(c.wt.Name) | maskOf(':')
		}
		// Where the path starts in what's matched, and how far on it is in
		// the name shown (demo/token-cache:path).
		start, shift := len(c.wt.Repo)+1, len(c.wt.Name)+1
		for _, ch := range c.changes {
			// What the default branch has is its line, whatever a capsule's
			// done to it.
			if ch.status == Removed || (g != nil && g.has(ch.path)) {
				continue
			}
			if !query.Covers(ch.mask | nameMask) {
				continue
			}
			s.Text = append(append(s.Text[:0], c.wt.Repo...), '/')
			if named {
				s.Text = append(append(s.Text, c.wt.Name...), ':')
			}
			s.Text = append(s.Text, ch.path...)
			m, ok := query.MatchBytes(s.Text, s)
			if !ok {
				continue
			}
			if !named {
				for i, a := range m.At {
					if a >= start {
						m.At[i] = a + shift
					}
				}
			}
			t.offer(hit{score: m.Score, repo: c.wt.Repo, worktree: c.wt.Name, path: ch.path, status: Added, at: m.At, size: len(s.Text)})
		}
	}
	out := make([]Result, 0, len(t.hits))
	for _, h := range t.hits {
		r := fileResult(snap, h.repo, h.worktree, h.path, line, end)
		r.Score, r.Status = h.score, h.status
		r.At = Marks(r.Title, h.at)
		out = append(out, r)
	}
	return out, t.total - len(out)
}

// searchCapsules is the capsules whose names match: each with how many
// files it's changed. A page holds a mission's capsules; these are every
// capsule there is, a mission's or not.
func searchCapsules(snap *snapshot, q string, limit int) []Result {
	query := NewQuery(q)
	if query.Empty() {
		return nil
	}
	out := []Result{}
	s := &Scratch{}
	for _, c := range snap.capsules {
		title := c.wt.Repo + "/" + c.wt.Name
		m, ok := query.Match(title, s)
		if !ok {
			continue
		}
		n := 0
		for _, ch := range c.changes {
			if ch.status != Removed {
				n++
			}
		}
		hint := "nothing changed"
		if n == 1 {
			hint = "1 file changed"
		} else if n > 1 {
			hint = strconv.Itoa(n) + " files changed"
		}
		out = append(out, Result{Kind: KindCapsule, Ref: title, Title: title, Hint: hint, Repo: c.wt.Repo, Worktree: c.wt.Name, Score: m.Score, At: Marks(title, append([]int(nil), m.At...))})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// fileResult is a file as a result: named, and for the default branch's,
// with the capsules that have changed it.
func fileResult(snap *snapshot, repo, worktree, path string, line, end int) Result {
	r := Result{Kind: KindFile, Repo: repo, Worktree: worktree, Path: path, Line: line, EndLine: end}
	r.Ref = fileRef(repo, worktree, path, line, end)
	r.Title = fileRef(repo, worktree, path, 0, 0)
	if worktree != Ground {
		return r
	}
	for _, c := range snap.capsules {
		if c.wt.Repo != repo {
			continue
		}
		if st, ok := c.status[path]; ok {
			if st == Added {
				st = Modified // the default branch has it too by now
			}
			r.In = append(r.In, Changed{Capsule: c.wt.Name, Status: st})
		}
	}
	return r
}

// changedFiles is what capsules have changed, each as it is in its
// capsule: what Find shows of files before anything's typed.
func changedFiles(snap *snapshot, limit int) ([]Result, int) {
	out := []Result{}
	total := 0
	for _, c := range snap.capsules {
		for _, ch := range c.changes {
			if ch.status == Removed {
				continue
			}
			total++
			if len(out) >= limit {
				continue
			}
			r := Result{Kind: KindFile, Repo: c.wt.Repo, Worktree: c.wt.Name, Path: ch.path, Status: ch.status}
			r.Ref = fileRef(c.wt.Repo, c.wt.Name, ch.path, 0, 0)
			r.Title = r.Ref
			out = append(out, r)
		}
	}
	return out, total - len(out)
}
