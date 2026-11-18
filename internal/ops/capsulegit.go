package ops

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// A page's view of a mission says, of each capsule, where its HEAD is,
// whether it's dirty, and how far it is from its upstream and from the
// default branch. Git is asked for each (a process apiece), and a page
// reads its view again on every change and after every button: asked each
// time, a mission of six capsules was a few dozen processes a read, and
// most of a second of it on a machine crew are working on.
//
// capsuleGit is where a view gets them: askedGit asks git each time (a
// command, which decides by them), and keptGit keeps what git said until
// the capsule moves (the daemon, which shows them).
type capsuleGit interface {
	// headBranch is the commit the capsule's HEAD is at, and its branch
	// ("HEAD" when it's detached): both "" if git can't say.
	headBranch(dir string) (head, branch string)
	// hasUpstream is whether its branch has an upstream that's there.
	hasUpstream(dir string) bool
	// aheadBehind is how far it is from its upstream: 0, 0 with none.
	aheadBehind(dir string) (ahead, behind int)
	// commitsSince is how many commits it has that base hasn't.
	commitsSince(dir, base string) int
	// dirty is whether it has changes that aren't committed.
	dirty(dir string) bool
}

// answer is what git said, and whether it's worth keeping: not when git
// couldn't be run at all (the machine's busy: it's asked again next time).
// Git refusing is an answer, as when there's no upstream to be ahead of.
type answer[T any] struct {
	is   T
	keep bool
}

func asked[T any](is T, err error) answer[T] {
	return answer[T]{is: is, keep: err == nil || git.ExitCode(err) != -1}
}

type headAndBranch struct{ head, branch string }

type aheadAndBehind struct{ ahead, behind int }

func askHeadBranch(dir string) answer[headAndBranch] {
	out, err := git.Combined(dir, "rev-parse", "HEAD", "--abbrev-ref", "HEAD")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if err != nil || len(lines) != 2 {
		return asked(headAndBranch{}, err)
	}
	return asked(headAndBranch{strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])}, nil)
}

func askUpstream(dir string) answer[bool] {
	_, err := git.Combined(dir, "rev-parse", "--verify", "--quiet", "@{u}")
	return asked(err == nil, err)
}

func askAheadBehind(dir string) answer[aheadAndBehind] {
	out, err := git.Combined(dir, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	var ab aheadAndBehind
	if err == nil {
		fmt.Sscanf(strings.TrimSpace(out), "%d\t%d", &ab.ahead, &ab.behind)
	}
	return asked(ab, err)
}

func askCommitsSince(dir, base string) answer[int] {
	out, err := git.Combined(dir, "rev-list", "--count", base+"..HEAD")
	var n int
	if err == nil {
		fmt.Sscanf(strings.TrimSpace(out), "%d", &n)
	}
	return asked(n, err)
}

func askDirty(dir string) answer[bool] {
	out, err := git.Combined(dir, "status", "--porcelain")
	return asked(err == nil && strings.TrimSpace(out) != "", err)
}

// askedGit asks git each time.
type askedGit struct{}

func (askedGit) headBranch(dir string) (string, string) {
	hb := askHeadBranch(dir).is
	return hb.head, hb.branch
}
func (askedGit) hasUpstream(dir string) bool { return askUpstream(dir).is }
func (askedGit) aheadBehind(dir string) (int, int) {
	ab := askAheadBehind(dir).is
	return ab.ahead, ab.behind
}
func (askedGit) commitsSince(dir, base string) int { return askCommitsSince(dir, base).is }
func (askedGit) dirty(dir string) bool             { return askDirty(dir).is }

// dirtyFloor is how long a capsule's dirtiness is shown as it was last
// seen before git is asked again. Nothing git keeps moves when a file in
// the capsule is edited, so it can't be known to have changed: past the
// floor it's shown as it was, asked for behind the read, and the stamps
// pages are told by (Stamp, MissionStamp) move if it's changed (dirtyMoved).
var dirtyFloor = 10 * time.Second

// keptGit keeps what git said of each capsule until the capsule moves:
// its HEAD, what's staged, its branch, or the refs it's measured against
// (capsuleStamp). A read of a capsule that hasn't moved starts no process.
type keptGit struct {
	mu sync.Mutex
	of map[string]*keptCapsule // by its folder
}

// keptCapsule is what git said of one capsule, as of stamp.
type keptCapsule struct {
	// One asker at a time: reads that come together (a page's, after a
	// button and its event) wait for the one answer.
	mu    sync.Mutex
	stamp string
	facts map[string]any
	// since counts the times it's moved, for a look that began before one
	// to know its answer's of a capsule that's since changed.
	since   int
	dirty   *bool
	dirtyAt time.Time
	looking bool
	moved   atomic.Int64 // see dirtyMoved
}

func (g *keptGit) capsule(dir string) *keptCapsule {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.of == nil {
		g.of = map[string]*keptCapsule{}
	}
	c := g.of[dir]
	if c == nil {
		c = &keptCapsule{}
		g.of[dir] = c
	}
	return c
}

// current drops what's kept if the capsule has moved since. c.mu is held.
func (c *keptCapsule) current(dir string) {
	if stamp := capsuleStamp(dir); stamp != c.stamp || c.facts == nil {
		c.stamp, c.facts, c.dirty = stamp, map[string]any{}, nil
		c.since++
	}
}

// kept is a fact of the capsule: as kept, or asked for and kept.
func kept[T any](g *keptGit, dir, what string, ask func() answer[T]) T {
	c := g.capsule(dir)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current(dir)
	if v, ok := c.facts[what].(T); ok {
		return v
	}
	a := ask()
	if a.keep {
		c.facts[what] = a.is
	}
	return a.is
}

func (g *keptGit) headBranch(dir string) (string, string) {
	hb := kept(g, dir, "head", func() answer[headAndBranch] { return askHeadBranch(dir) })
	return hb.head, hb.branch
}

func (g *keptGit) hasUpstream(dir string) bool {
	return kept(g, dir, "upstream", func() answer[bool] { return askUpstream(dir) })
}

func (g *keptGit) aheadBehind(dir string) (int, int) {
	ab := kept(g, dir, "ahead", func() answer[aheadAndBehind] { return askAheadBehind(dir) })
	return ab.ahead, ab.behind
}

func (g *keptGit) commitsSince(dir, base string) int {
	return kept(g, dir, "since "+base, func() answer[int] { return askCommitsSince(dir, base) })
}

func (g *keptGit) dirty(dir string) bool {
	c := g.capsule(dir)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current(dir)
	if c.dirty == nil {
		a := askDirty(dir)
		if a.keep {
			c.dirty, c.dirtyAt = &a.is, time.Now()
		}
		return a.is
	}
	if time.Since(c.dirtyAt) > dirtyFloor && !c.looking {
		c.looking = true
		go c.look(dir, c.since)
	}
	return *c.dirty
}

// look asks git whether the capsule's dirty, behind a read that was
// answered with what was kept.
func (c *keptCapsule) look(dir string, since int) {
	a := askDirty(dir)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.looking = false
	if !a.keep || since != c.since || c.dirty == nil {
		return // not said, or said of the capsule as it was
	}
	if *c.dirty != a.is {
		c.moved.Add(1)
	}
	c.dirty, c.dirtyAt = &a.is, time.Now()
}

// dirtyMoved counts, for each capsule, the times a look found its
// dirtiness changed from what a read had been given: the stamps pages are
// told by carry it, so the page that was shown the old answer reads again.
// Of the capsules named, or of all it keeps. It starts no process, and
// forgets capsules that have gone.
func (g *keptGit) dirtyMoved(dirs ...string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(dirs) == 0 {
		for dir := range g.of {
			if _, err := os.Stat(dir); err != nil {
				delete(g.of, dir)
				continue
			}
			dirs = append(dirs, dir)
		}
		slices.Sort(dirs)
	}
	var b strings.Builder
	for _, dir := range dirs {
		if c := g.of[dir]; c != nil {
			if n := c.moved.Load(); n > 0 {
				fmt.Fprintf(&b, "%s %d\n", dir, n)
			}
		}
	}
	return b.String()
}

// capsuleStamp names what git's answers about a capsule are built from,
// as it is now, without asking git: its HEAD, what's staged and its
// HEAD's log (a commit, a checkout, a reset), its branch's ref, the refs
// it's measured against (its own on origin, and the others there: a
// fetch, a push), and its repo's config (an upstream set). File sizes and
// times, and what HEAD says: no process is started.
func capsuleStamp(dir string) string {
	gitDir, common := gitDirs(dir)
	if gitDir == "" {
		return "" // not a capsule, or gone: nothing's kept of it for long
	}
	h := fnv.New64a()
	note := func(path string) {
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(h, "%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
		}
	}
	head, _ := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	h.Write(head)
	for _, f := range []string{"HEAD", "index", filepath.Join("logs", "HEAD"), "FETCH_HEAD"} {
		note(filepath.Join(gitDir, f))
	}
	for _, f := range []string{"config", "packed-refs", "FETCH_HEAD", filepath.Join("reftable", "tables.list")} {
		note(filepath.Join(common, f))
	}
	if ref, ok := strings.CutPrefix(strings.TrimSpace(string(head)), "ref: "); ok {
		note(filepath.Join(common, filepath.FromSlash(ref)))
		if branch, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
			note(filepath.Join(common, "refs", "remotes", "origin", filepath.FromSlash(branch)))
		}
	}
	// The default branch as last fetched, whichever it is: a ref written
	// there moves its folder.
	note(filepath.Join(common, "refs", "remotes", "origin"))
	entries, _ := os.ReadDir(filepath.Join(common, "refs", "remotes", "origin"))
	for _, e := range entries {
		if !e.IsDir() {
			note(filepath.Join(common, "refs", "remotes", "origin", e.Name()))
		}
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// gitDirs is where git keeps a worktree's own files (its HEAD, its index)
// and its repository's (refs, config): the same folder for a repository
// that's not a linked worktree. Both "" when dir isn't one.
func gitDirs(dir string) (gitDir, common string) {
	dot := filepath.Join(dir, ".git")
	info, err := os.Stat(dot)
	if err != nil {
		return "", ""
	}
	if info.IsDir() {
		return dot, dot
	}
	// A linked worktree: .git is a file that says where.
	data, err := os.ReadFile(dot)
	if err != nil {
		return "", ""
	}
	at, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
	if !ok {
		return "", ""
	}
	if !filepath.IsAbs(at) {
		at = filepath.Join(dir, at)
	}
	gitDir, common = at, at
	if data, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		common = strings.TrimSpace(string(data))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitDir, common)
		}
	}
	return gitDir, common
}
