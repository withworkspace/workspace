package find

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// Ground is the name of a repo's worktree for its default branch.
const Ground = ".ground"

// Worktree is one of a repo's: its ground, or a capsule, with the ref a
// capsule's changes are from (none for the ground).
type Worktree struct {
	Repo    string
	Name    string
	Dir     string
	BaseRef string
}

func (w Worktree) key() string { return w.Repo + "/" + w.Name }

// How long each part is taken as read. Nothing is checked unless Find is
// asked something: closed, the index doesn't look.
const (
	listAge    = 2 * time.Second // which worktrees there are
	groundAge  = 2 * time.Second // a ground's head
	capsuleAge = 3 * time.Second // what a capsule has changed
	// capsuleWait is how long an answer waits for capsules still being
	// read before going without them, and saying so.
	capsuleWait = 150 * time.Millisecond
	// reading is how many capsules are read at once.
	reading = 4
)

// Index is what Find looks through that a page doesn't hold: each repo's
// files, what each capsule has changed, the pull requests. It's in
// memory, built when first asked, and each part read again only when what
// it was built from has moved.
type Index struct {
	// Worktrees is every repo's ground and capsules.
	Worktrees func() ([]Worktree, error)
	// Pulls is the open pull requests as GitHub last said, and when; nil
	// where there's no GitHub. It never asks GitHub.
	Pulls func() ([]Pull, time.Time)
	// Org is the workspace's on GitHub, for reading a pasted address.
	Org string
	// Now is the time (tests set it).
	Now func() time.Time

	mu       sync.Mutex
	listed   time.Time
	wts      []Worktree
	listErr  error
	grounds  map[string]*groundEntry
	capsules map[string]*capsuleEntry
	sem      chan struct{}
}

func (ix *Index) now() time.Time {
	if ix.Now != nil {
		return ix.Now()
	}
	return time.Now()
}

// groundEntry is a repo's default branch: every path on it.
type groundEntry struct {
	data    *groundData
	checked time.Time
	job     chan struct{} // closed when the read under way is done
}

// groundData is a ground as read at a commit. It isn't changed once made.
type groundData struct {
	repo, dir, head string
	paths           []string // sorted
	masks           []uint64
	repoMask        uint64
}

func (g *groundData) has(path string) bool {
	i := sort.SearchStrings(g.paths, path)
	return i < len(g.paths) && g.paths[i] == path
}

// hasDir is whether path is a folder on the default branch.
func (g *groundData) hasDir(path string) bool {
	path = strings.TrimSuffix(path, "/") + "/"
	i := sort.SearchStrings(g.paths, path)
	return i < len(g.paths) && strings.HasPrefix(g.paths[i], path)
}

// capsuleEntry is a capsule: only what it's changed since it branched.
type capsuleEntry struct {
	data *capsuleData
	read time.Time
	job  chan struct{}
}

// Statuses of a file in a capsule, against where the capsule branched.
const (
	Added    = "added"
	Modified = "modified"
	Removed  = "removed"
)

type capsuleData struct {
	wt      Worktree
	changes []change // sorted by path
	status  map[string]string
}

type change struct {
	path, status string
	mask         uint64
}

func nulSplit(out []byte) []string {
	s := strings.TrimSuffix(string(out), "\x00")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x00")
}

// readGround reads a ground's paths, unless it's still at the commit was
// has.
func readGround(ctx context.Context, wt Worktree, was *groundData) (*groundData, error) {
	head, err := git.ReadLine(ctx, wt.Dir, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return nil, err
	}
	if was != nil && was.head == head && was.dir == wt.Dir {
		return was, nil
	}
	out, err := git.Read(ctx, wt.Dir, "ls-tree", "-r", "--name-only", "-z", "HEAD")
	if err != nil {
		return nil, err
	}
	g := &groundData{repo: wt.Repo, dir: wt.Dir, head: head, paths: nulSplit(out), repoMask: Mask(wt.Repo) | maskOf('/')}
	sort.Strings(g.paths)
	g.masks = make([]uint64, len(g.paths))
	for i, p := range g.paths {
		g.masks[i] = Mask(p)
	}
	return g, nil
}

// readCapsule reads what a capsule has changed since it branched from its
// base: committed or not, and what's new and untracked. Against where it
// branched, not the base as it is now: a capsule behind the default
// branch hasn't changed what's merged since.
func readCapsule(ctx context.Context, wt Worktree) (*capsuleData, error) {
	c := &capsuleData{wt: wt, status: map[string]string{}}
	set := func(path, status string) {
		if path != "" {
			c.status[path] = status
		}
	}
	base := ""
	if wt.BaseRef != "" {
		base, _ = git.ReadLine(ctx, wt.Dir, "merge-base", wt.BaseRef, "HEAD")
	}
	if base != "" {
		out, err := git.Read(ctx, wt.Dir, "diff", "-z", "--name-status", "-M", base)
		if err != nil {
			return nil, err
		}
		f := nulSplit(out)
		for i := 0; i+1 < len(f); i += 2 {
			code := f[i]
			if code == "" {
				continue
			}
			switch code[0] {
			case 'A':
				set(f[i+1], Added)
			case 'D':
				set(f[i+1], Removed)
			case 'R', 'C':
				// Renamed or copied: the old path, then the new.
				if i+2 >= len(f) {
					break
				}
				if code[0] == 'R' {
					set(f[i+1], Removed)
				}
				set(f[i+2], Added)
				i++
			default:
				set(f[i+1], Modified)
			}
		}
	}
	out, err := git.Read(ctx, wt.Dir, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	for _, p := range nulSplit(out) {
		set(p, Added)
	}
	for p, st := range c.status {
		c.changes = append(c.changes, change{path: p, status: st, mask: Mask(p)})
	}
	sort.Slice(c.changes, func(i, j int) bool { return c.changes[i].path < c.changes[j].path })
	return c, nil
}

// snapshot is the index as an answer is made from it.
type snapshot struct {
	grounds  []*groundData
	capsules []*capsuleData
	// reading is how many capsules haven't been read yet.
	reading int
}

func (s *snapshot) ground(repo string) *groundData {
	for _, g := range s.grounds {
		if g.repo == repo {
			return g
		}
	}
	return nil
}

func (s *snapshot) capsule(repo, name string) *capsuleData {
	for _, c := range s.capsules {
		if c.wt.Repo == repo && c.wt.Name == name {
			return c
		}
	}
	return nil
}

// fresh brings the index up to date and answers it as it stands. Reads
// that are due are started (one at a time for each worktree, whoever
// asks), and go on if ctx ends: a letter typed drops the request, not the
// reading. It waits for every ground, and for capsules for wait (0: as
// long as it takes); after that a capsule answers as it was last read, or
// not at all if it never has been.
func (ix *Index) fresh(ctx context.Context, wait time.Duration) (*snapshot, error) {
	now := ix.now()
	ix.mu.Lock()
	if ix.grounds == nil {
		ix.grounds, ix.capsules, ix.sem = map[string]*groundEntry{}, map[string]*capsuleEntry{}, make(chan struct{}, reading)
	}
	if ix.wts == nil || now.Sub(ix.listed) >= listAge {
		wts, err := ix.Worktrees()
		ix.listed, ix.listErr = now, err
		if err == nil {
			ix.wts = wts
			here := map[string]bool{}
			for _, wt := range wts {
				here[wt.key()] = true
			}
			for k := range ix.capsules {
				if !here[k] {
					delete(ix.capsules, k)
				}
			}
			for repo := range ix.grounds {
				if !here[repo+"/"+Ground] {
					delete(ix.grounds, repo)
				}
			}
		}
	}
	if ix.wts == nil && ix.listErr != nil {
		err := ix.listErr
		ix.mu.Unlock()
		return nil, err
	}
	var grounds, capsules []chan struct{}
	for _, wt := range ix.wts {
		wt := wt
		if wt.Name == Ground {
			e := ix.grounds[wt.Repo]
			if e == nil {
				e = &groundEntry{}
				ix.grounds[wt.Repo] = e
			}
			if e.job == nil && (e.data == nil || now.Sub(e.checked) >= groundAge) {
				job := make(chan struct{})
				e.job = job
				was := e.data
				go func() {
					c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					data, err := readGround(c, wt, was)
					cancel()
					ix.mu.Lock()
					if err == nil {
						e.data = data
					}
					e.checked, e.job = ix.now(), nil
					ix.mu.Unlock()
					close(job)
				}()
			}
			if e.job != nil {
				grounds = append(grounds, e.job)
			}
			continue
		}
		e := ix.capsules[wt.key()]
		if e == nil {
			e = &capsuleEntry{}
			ix.capsules[wt.key()] = e
		}
		if e.job == nil && (e.data == nil || now.Sub(e.read) >= capsuleAge) {
			job := make(chan struct{})
			e.job = job
			go func() {
				ix.sem <- struct{}{}
				c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				data, err := readCapsule(c, wt)
				cancel()
				<-ix.sem
				ix.mu.Lock()
				if err == nil {
					e.data = data
				}
				e.read, e.job = ix.now(), nil
				ix.mu.Unlock()
				close(job)
			}()
		}
		if e.job != nil {
			capsules = append(capsules, e.job)
		}
	}
	ix.mu.Unlock()

	for _, job := range grounds {
		select {
		case <-job:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	var deadline <-chan time.Time
	if wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		deadline = t.C
	}
waiting:
	for _, job := range capsules {
		select {
		case <-job:
		case <-deadline:
			break waiting
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	ix.mu.Lock()
	defer ix.mu.Unlock()
	snap := &snapshot{}
	for _, wt := range ix.wts {
		if wt.Name == Ground {
			if e := ix.grounds[wt.Repo]; e != nil && e.data != nil {
				snap.grounds = append(snap.grounds, e.data)
			}
			continue
		}
		if e := ix.capsules[wt.key()]; e != nil && e.data != nil {
			snap.capsules = append(snap.capsules, e.data)
		} else {
			snap.reading++
		}
	}
	return snap, nil
}

// Stats is what the index holds.
type Stats struct {
	Repos    int `json:"repos"`
	Files    int `json:"files"`
	Capsules int `json:"capsules"`
	// Changed is how many files capsules have changed between them.
	Changed int `json:"changed"`
}

// Warm reads whatever's due, and waits for all of it: what opening Find
// asks for, so the first letter typed is answered from memory.
func (ix *Index) Warm(ctx context.Context) (Stats, error) {
	snap, err := ix.fresh(ctx, 0)
	if err != nil {
		return Stats{}, err
	}
	st := Stats{Repos: len(snap.grounds), Capsules: len(snap.capsules)}
	for _, g := range snap.grounds {
		st.Files += len(g.paths)
	}
	for _, c := range snap.capsules {
		st.Changed += len(c.changes)
	}
	return st, nil
}

// GroundTree is a repo's default branch as the index last read it: every
// path on it, at a commit. It's a ref.Tree, for telling a file that's
// there from words that only look like one.
type GroundTree struct{ g *groundData }

func (t GroundTree) Has(path string) bool    { return t.g.has(path) }
func (t GroundTree) HasDir(path string) bool { return t.g.hasDir(path) }

// Head is the commit it was read at.
func (t GroundTree) Head() string { return t.g.head }

// Grounds is each repo's default branch, brought up to date (capsules
// aren't waited for).
func (ix *Index) Grounds(ctx context.Context) (map[string]GroundTree, error) {
	snap, err := ix.fresh(ctx, time.Nanosecond)
	if err != nil {
		return nil, err
	}
	out := make(map[string]GroundTree, len(snap.grounds))
	for _, g := range snap.grounds {
		out[g.repo] = GroundTree{g}
	}
	return out, nil
}
