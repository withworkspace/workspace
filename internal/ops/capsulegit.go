package ops

import (
	"context"
	"fmt"
	"hash/fnv"
	"maps"
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

// askHeadBranch reads them from git's own files where they say (headOf),
// and asks git where they don't.
func askHeadBranch(dir string) answer[headAndBranch] {
	if head, branch, ok := headOf(dir); ok {
		return answer[headAndBranch]{is: headAndBranch{head, branch}, keep: true}
	}
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
		_, _ = fmt.Sscanf(strings.TrimSpace(out), "%d\t%d", &ab.ahead, &ab.behind) // what can't be read is none ahead or behind
	}
	return asked(ab, err)
}

func askCommitsSince(dir, base string) answer[int] {
	out, err := git.Combined(dir, "rev-list", "--count", base+"..HEAD")
	var n int
	if err == nil {
		_, _ = fmt.Sscanf(strings.TrimSpace(out), "%d", &n) // what can't be read is none
	}
	return asked(n, err)
}

// askDirty asks read-only: a status that refreshes the index writes in a
// capsule crew may be working in, and moves it (capsuleStamp), so what's
// kept of it would be asked for again.
func askDirty(dir string) answer[bool] {
	out, err := git.Read(context.Background(), dir, "status", "--porcelain")
	return asked(err == nil && strings.TrimSpace(string(out)) != "", err)
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
	// slots bounds the looks behind reads (soon): a page listing every
	// capsule of a repo mustn't start a process for each at once.
	slots chan struct{}
	// keeper, once the daemon keeps the worktrees, names a capsule's git
	// files as it keeps them (stampOf), in place of looking at them on
	// every read.
	keeper atomic.Pointer[capsuleKeeper]
	// found, if set, is told when a look behind a read finds a capsule
	// other than the read was given: whoever tells pages looks now, not at
	// its next poll. It mustn't wait.
	found func()
}

// tell says a look found something other than a read was given.
func (g *keptGit) tell() {
	g.mu.Lock()
	found := g.found
	g.mu.Unlock()
	if found != nil {
		found()
	}
}

// lookSlots is how many looks behind reads run at once.
const lookSlots = 4

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
	// was is what was kept before it last moved, and wasDirty its
	// dirtiness then: what a read that mustn't wait answers with (soon)
	// until it's asked again. asking is which facts a look is out for,
	// as since+1 when it began.
	was      map[string]any
	wasDirty *bool
	asking   map[string]int
}

func (g *keptGit) capsule(dir string) *keptCapsule {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.of == nil {
		g.of = map[string]*keptCapsule{}
		g.slots = make(chan struct{}, lookSlots)
	}
	c := g.of[dir]
	if c == nil {
		c = &keptCapsule{was: map[string]any{}, asking: map[string]int{}}
		g.of[dir] = c
	}
	return c
}

// current drops what's kept if the capsule has moved since, keeping it as
// what was. c.mu is held.
func (c *keptCapsule) current(dir string, g *keptGit) {
	stamp, ok := "", false
	if k := g.keeper.Load(); k != nil {
		stamp, ok = k.stampOf(dir) // as the daemon keeps the capsule: no file's looked at
	}
	if !ok {
		stamp = capsuleStamp(dir)
	}
	if stamp != c.stamp || c.facts == nil {
		maps.Copy(c.was, c.facts)
		if c.dirty != nil {
			c.wasDirty = c.dirty
		}
		c.stamp, c.facts, c.dirty = stamp, map[string]any{}, nil
		c.since++
	}
}

// behind runs a look behind a read, in one of g's slots.
func (g *keptGit) behind(look func()) {
	go func() {
		g.slots <- struct{}{}
		defer func() { <-g.slots }()
		look()
	}()
}

// soon is a fact of the capsule without waiting on git: as kept, or, when
// the capsule has moved since or it's never been asked, as it was before
// (ok false when it's never been known) while git's asked behind the read.
// A look that finds it other than the read was told counts as found moved
// (dirtyMoved), so the page that was shown it reads again.
func soon[T comparable](g *keptGit, dir, what string, ask func() answer[T]) (v T, ok bool) {
	c := g.capsule(dir)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current(dir, g)
	if v, ok := c.facts[what].(T); ok {
		return v, true
	}
	v, ok = c.was[what].(T)
	if c.asking[what] == c.since+1 {
		return v, ok
	}
	since := c.since
	c.asking[what] = since + 1
	g.behind(func() {
		a := ask()
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.asking[what] == since+1 {
			delete(c.asking, what)
		}
		if !a.keep || since != c.since {
			return // not said, or said of the capsule as it was: the next read asks again
		}
		c.facts[what] = a.is
		if !ok || a.is != v {
			c.moved.Add(1)
			g.tell()
		}
	})
	return v, ok
}

// dirtySoon is whether the capsule has changes not committed, as soon
// says a fact: what's kept (looked at again past dirtyFloor, as dirty
// does), or what was, while git's asked behind the read.
func (g *keptGit) dirtySoon(dir string) (is, ok bool) {
	if k := g.keeper.Load(); k != nil {
		if dirty, ok := k.dirtyOf(dir); ok {
			return dirty, true // as the daemon keeps it, asked again while it's followed
		}
	}
	c := g.capsule(dir)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current(dir, g)
	if c.dirty != nil {
		if time.Since(c.dirtyAt) > dirtyFloor && !c.looking {
			c.looking = true
			go c.look(dir, c.since)
		}
		return *c.dirty, true
	}
	if c.wasDirty != nil {
		is, ok = *c.wasDirty, true
	}
	if c.asking["dirty"] == c.since+1 {
		return is, ok
	}
	since := c.since
	c.asking["dirty"] = since + 1
	g.behind(func() {
		a := askDirty(dir)
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.asking["dirty"] == since+1 {
			delete(c.asking, "dirty")
		}
		if !a.keep || since != c.since || c.dirty != nil {
			return
		}
		c.dirty, c.dirtyAt = &a.is, time.Now()
		if !ok || a.is != is {
			c.moved.Add(1)
			g.tell()
		}
	})
	return is, ok
}

// standing is where a capsule stands, as a mission's page says it:
// whether it has changes not committed, and how far it is from its
// upstream, or, never pushed, how many commits it has that base hasn't
// (what a pull request would open with).
type standing struct {
	dirty         bool
	ahead, behind int
}

// standingOf is a capsule's standing from what's kept of it, if all of
// it is.
func standingOf(facts map[string]any, dirty *bool, base string) (standing, bool) {
	upstream, ok := facts["upstream"].(bool)
	if !ok || dirty == nil {
		return standing{}, false
	}
	s := standing{dirty: *dirty}
	if upstream {
		ab, ok := facts["ahead"].(aheadAndBehind)
		s.ahead, s.behind = ab.ahead, ab.behind
		return s, ok
	}
	n, ok := facts["since "+base].(int)
	s.ahead = n
	return s, ok
}

// standing is where a capsule stands, asked of git where it isn't kept:
// for whoever keeps it current (the daemon, when it sees a capsule has
// moved, before it tells the pages), not for a page's read.
func (g *keptGit) standing(dir, base string) standing {
	s := standing{dirty: g.dirty(dir)}
	if g.hasUpstream(dir) {
		s.ahead, s.behind = g.aheadBehind(dir)
	} else {
		s.ahead = g.commitsSince(dir, base)
	}
	return s
}

// standingSoon is where a capsule stands without waiting on git, as soon
// says a fact: as kept, or, when the capsule has moved since or it's
// never been asked, as it was before (ok false when it's never been
// known) while git's asked behind the read. A look that finds it other
// than the read was told counts as found moved, and says so (tell).
func (g *keptGit) standingSoon(dir, base string) (standing, bool) {
	c := g.capsule(dir)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current(dir, g)
	if s, ok := standingOf(c.facts, c.dirty, base); ok {
		if time.Since(c.dirtyAt) > dirtyFloor && !c.looking {
			c.looking = true
			go c.look(dir, c.since)
		}
		return s, true
	}
	was, known := standingOf(c.was, c.wasDirty, base)
	if c.asking["standing"] == c.since+1 {
		return was, known
	}
	since := c.since
	c.asking["standing"] = since + 1
	g.behind(func() {
		dirty, upstream := askDirty(dir), askUpstream(dir)
		var ahead answer[aheadAndBehind]
		var commits answer[int]
		if upstream.is {
			ahead = askAheadBehind(dir)
		} else {
			commits = askCommitsSince(dir, base)
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.asking["standing"] == since+1 {
			delete(c.asking, "standing")
		}
		if since != c.since || !dirty.keep || !upstream.keep || (upstream.is && !ahead.keep) || (!upstream.is && !commits.keep) {
			return // not said, or said of the capsule as it was: the next read asks again
		}
		c.facts["upstream"] = upstream.is
		if upstream.is {
			c.facts["ahead"] = ahead.is
		} else {
			c.facts["since "+base] = commits.is
		}
		if c.dirty == nil {
			c.dirty, c.dirtyAt = &dirty.is, time.Now()
		}
		if now, _ := standingOf(c.facts, c.dirty, base); !known || now != was {
			c.moved.Add(1)
			g.tell()
		}
	})
	return was, known
}

// soonGit is a capsule's git for a page's view of a mission: nothing
// waits on git. What isn't kept is said as it was (or as nothing, never
// known), and asked for behind the read.
type soonGit struct{ g *keptGit }

// headBranch is the capsule's head and branch as kept, or as git's own
// files say (no process), or else asked of git behind the read: unknown
// until then. Through kept it would run git there and then on a cold
// cache where the files don't say (a ref that's neither loose nor
// packed, a HEAD that isn't a branch's), and a page would wait on it.
func (s soonGit) headBranch(dir string) (string, string) {
	if hb, ok := soon(s.g, dir, "head", func() answer[headAndBranch] { return askHeadBranch(dir) }); ok {
		return hb.head, hb.branch
	}
	if head, branch, ok := headOf(dir); ok {
		return head, branch
	}
	return "", ""
}
func (s soonGit) hasUpstream(dir string) bool {
	is, _ := soon(s.g, dir, "upstream", func() answer[bool] { return askUpstream(dir) })
	return is
}
func (s soonGit) aheadBehind(dir string) (int, int) {
	ab, _ := soon(s.g, dir, "ahead", func() answer[aheadAndBehind] { return askAheadBehind(dir) })
	return ab.ahead, ab.behind
}
func (s soonGit) commitsSince(dir, base string) int {
	n, _ := soon(s.g, dir, "since "+base, func() answer[int] { return askCommitsSince(dir, base) })
	return n
}
func (s soonGit) dirty(dir string) bool {
	is, _ := s.g.dirtySoon(dir)
	return is
}

// kept is a fact of the capsule: as kept, or asked for and kept.
func kept[T any](g *keptGit, dir, what string, ask func() answer[T]) T {
	c := g.capsule(dir)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current(dir, g)
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
	if k := g.keeper.Load(); k != nil {
		if dirty, ok := k.dirtyOf(dir); ok {
			return dirty
		}
	}
	c := g.capsule(dir)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current(dir, g)
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
// dirtiness (or, behind a read that didn't wait, any fact) changed from
// what a read had been given: the stamps pages are
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

// branchOf is the branch dir's HEAD is on, read from git's own files:
// "HEAD" when it's detached, "" when dir isn't a worktree. No process, and
// never stale: it's what git itself would read.
func branchOf(dir string) string {
	gitDir, _ := gitDirs(dir)
	if gitDir == "" {
		return ""
	}
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	ref, ok := strings.CutPrefix(strings.TrimSpace(string(head)), "ref: ")
	if !ok {
		return "HEAD"
	}
	if branch, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return branch
	}
	return ref
}

// headOf is the commit dir's HEAD is at and the branch it's on ("HEAD"
// when it's detached), read from git's own files as branchOf reads the
// branch: its HEAD, then its branch's ref, loose or packed. No process,
// and never behind: it's what git itself would read. ok is false where
// the files don't say (a branch with no commit yet, refs kept as a
// reftable, a ref that names another): git is asked there.
func headOf(dir string) (head, branch string, ok bool) {
	gitDir, common := gitDirs(dir)
	if gitDir == "" {
		return "", "", false
	}
	data, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", "", false
	}
	at := strings.TrimSpace(string(data))
	ref, symbolic := strings.CutPrefix(at, "ref: ")
	if !symbolic {
		return at, "HEAD", isObjectName(at)
	}
	branch, isBranch := strings.CutPrefix(ref, "refs/heads/")
	if !isBranch {
		return "", "", false
	}
	if data, err := os.ReadFile(filepath.Join(common, filepath.FromSlash(ref))); err == nil {
		head = strings.TrimSpace(string(data))
		return head, branch, isObjectName(head)
	}
	packed, err := os.ReadFile(filepath.Join(common, "packed-refs"))
	if err != nil {
		return "", "", false
	}
	for line := range strings.Lines(string(packed)) {
		if name, named := strings.CutSuffix(strings.TrimRight(line, "\r\n"), " "+ref); named && isObjectName(name) {
			return name, branch, true
		}
	}
	return "", "", false
}

// HeadOf is the commit a capsule's (or a ground's) HEAD is at, read from
// git's own files where they say and asked of git where they don't: ""
// if it has none. For what's read often (a page's passes, a mission's
// re-reviews), which a git apiece would make wait.
func HeadOf(dir string) string { return askHeadBranch(dir).is.head }

// isObjectName is whether s is a commit's whole name: 40 hex digits, or
// 64 in a repository that names by SHA-256.
func isObjectName(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// capsuleStamp names what git's answers about a capsule are built from,
// as it is now, without asking git (capsuleStamps, both halves).
func capsuleStamp(dir string) string {
	refs, index := capsuleStamps(dir)
	if refs == "" {
		return "" // not a capsule, or gone: nothing's kept of it for long
	}
	return refs + " " + index
}

// capsuleStamps names what git's answers about a capsule are built from,
// as it is now: refs, where its HEAD is and what it's measured against
// (its HEAD and its HEAD's log, so a commit, a checkout, a reset; its
// branch's ref; the refs on origin, its own and the others, so a fetch
// that moved one, or a push; its repo's config, so an upstream set), and
// index, what's staged, which moves whether it's dirty and nothing else.
// File sizes and times, and what HEAD says: no process is started. Not
// FETCH_HEAD: a fetch writes it whether or not anything moved, and one
// that moved a ref moves the ref. Both "" when dir isn't a worktree.
func capsuleStamps(dir string) (refs, index string) {
	gitDir, common := gitDirs(dir)
	if gitDir == "" {
		return "", ""
	}
	h := fnv.New64a()
	note := func(path string) {
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(h, "%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
		}
	}
	head, _ := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	h.Write(head)
	for _, f := range []string{"HEAD", filepath.Join("logs", "HEAD")} {
		note(filepath.Join(gitDir, f))
	}
	for _, f := range []string{"config", "packed-refs", filepath.Join("reftable", "tables.list")} {
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
	refs = fmt.Sprintf("%x", h.Sum64())
	index = "-"
	if info, err := os.Stat(filepath.Join(gitDir, "index")); err == nil {
		index = fmt.Sprintf("%d %d", info.Size(), info.ModTime().UnixNano())
	}
	return refs, index
}

// changesStamp names what a capsule's changes since it left base (a ref
// on its remote, as origin/main) are read from, as it is now, without
// asking git: its HEAD and its branch's ref (a commit, a checkout), what's
// staged, and base's own ref (a fetch that moved it). Narrower than
// capsuleStamp, which every fetch moves. "" when dir isn't a worktree. A
// file edited and not staged moves none of it.
func changesStamp(dir, base string) string {
	gitDir, common := gitDirs(dir)
	if gitDir == "" {
		return ""
	}
	h := fnv.New64a()
	note := func(path string) {
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(h, "%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
		}
	}
	head, _ := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	h.Write(head)
	note(filepath.Join(gitDir, "index"))
	if ref, ok := strings.CutPrefix(strings.TrimSpace(string(head)), "ref: "); ok {
		note(filepath.Join(common, filepath.FromSlash(ref)))
	}
	note(filepath.Join(common, "refs", "remotes", filepath.FromSlash(base)))
	// Refs that aren't files of their own.
	note(filepath.Join(common, "packed-refs"))
	note(filepath.Join(common, "reftable", "tables.list"))
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
