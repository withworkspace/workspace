package ops

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/withworkspace/workspace/internal/workspace"
)

// followed is who's following what, as the web server says it: the
// missions whose pages are open, and the worktrees followed on their own
// (Files' capsules, as repo/name).
type followed func() (missions, worktrees []string)

// The daemon keeps every repo's worktrees (its ground and its capsules) in
// memory, and keeps them current itself: where each one's HEAD is and its
// branch, whether it has changes not committed, and how far it is from its
// upstream. A page's read of them is a lookup: it starts no git, and stats
// no file.
//
// What moves them is what git writes (WatchGit tells the keeper which of
// its folders were written), the repos' folders listed again now and then
// (a capsule lifted, or burned), and a sweep of every one's git files
// every few minutes, for what a watch missed. What git keeps doesn't move
// when a file's edited, so a capsule someone's following (a mission's
// page that's open, a capsule in Files), or that a page listed lately, is
// asked whether it's dirty again every dirtyFloor.
//
// Outside the daemon (a command), nothing's kept: the readers ask as they
// did (keeping is false).

var (
	// capsuleSweepEvery is how often every worktree's git files are looked
	// at again, for a move the watch didn't see.
	capsuleSweepEvery = 2 * time.Minute
	// lookedFor is how long a worktree a page has shown is looked after:
	// asked whether it's dirty every dirtyFloor.
	lookedFor = time.Minute
	// capsuleAskers is how many worktrees are asked of git at once.
	capsuleAskers = 4
)

// keptTree is what's kept of one worktree.
type keptTree struct {
	repo, name, dir string
	gitDir, common  string
	ground          bool

	head, branch string
	// refs and index are its git files as last looked at
	// (capsuleStamps), and moves how many times they've moved since it was
	// first kept: what's kept of it from git is of the moves it was asked
	// at.
	refs, index string
	moves       int64

	dirty         bool
	ahead, behind int
	// read: git's been asked whether it's dirty and how far it is from its
	// upstream since it last moved, and has said.
	read bool
	// queued: an ask of it is waiting or under way.
	queued  bool
	dirtyAt time.Time
	looked  time.Time
}

// capsuleKeeper is the daemon's worktrees.
type capsuleKeeper struct {
	keeping atomic.Bool
	version atomic.Int64 // moves whenever any worktree's kept facts do

	mu       sync.Mutex
	repos    map[string][]*keptTree // each repo's, ground first, then by name
	byDir    map[string]*keptTree
	byGitDir map[string]*keptTree
	byCommon map[string][]*keptTree
	asks     chan treeAsk
	tell     func()
	// groundMoved, if set, is told a repo whose ground's head moved (not
	// as it's first read): what landed on it is heard (HearGrounds).
	groundMoved func(repo string)
	// following, once the web server says, is who's following what.
	following atomic.Pointer[followed]
	// capsulesOf is a mission's capsules' folders, as kept.
	capsulesOf func(slug string) []string
}

// treeAsk is a worktree to ask git of: whether it's dirty, and, unless only
// that, how far it is from its upstream.
type treeAsk struct {
	t         *keptTree
	moves     int64
	dirtyOnly bool
}

// KeepCapsules keeps the workspace's worktrees until stop closes: listed,
// read from git's files, asked of git behind, and moved by moved (the git
// folders written to, as WatchGit says them).
func (w *Workspace) keepCapsules(stop <-chan struct{}, moved <-chan []string) {
	k := &w.caps
	k.mu.Lock()
	k.repos, k.byDir, k.byGitDir, k.byCommon = map[string][]*keptTree{}, map[string]*keptTree{}, map[string]*keptTree{}, map[string][]*keptTree{}
	k.asks = make(chan treeAsk, 1024)
	k.tell = w.tellSeen
	k.capsulesOf = w.followedCapsules
	hear := make(chan string, 64)
	k.groundMoved = func(repo string) {
		select {
		case hear <- repo:
		default: // one waiting stands for it
		}
	}
	k.mu.Unlock()
	go w.hearGroundsAsTheyMove(stop, hear)
	for range capsuleAskers {
		go k.asker(stop)
	}
	k.relist(w.WS())
	k.keeping.Store(true)
	w.git.keeper.Store(k)
	defer k.keeping.Store(false)

	list := time.NewTicker(gitFoldersEvery)
	defer list.Stop()
	sweep := time.NewTicker(capsuleSweepEvery)
	defer sweep.Stop()
	dirty := time.NewTicker(dirtyFloor)
	defer dirty.Stop()
	for {
		select {
		case <-stop:
			return
		case dirs := <-moved:
			k.moved(dirs)
		case <-list.C:
			k.relist(w.WS())
		case <-sweep.C:
			k.sweep()
		case <-dirty.C:
			k.askLooked()
		}
	}
}

// relist lists every repo's worktrees again, keeping what's kept of those
// still there, and reading those new.
func (k *capsuleKeeper) relist(ws *workspace.Workspace) {
	k.mu.Lock()
	defer k.mu.Unlock()
	changed := false
	here := map[string]bool{}
	repos := map[string][]*keptTree{}
	for _, repo := range ws.RepoNames {
		names, _ := workspace.ListAllWorktrees(ws.RepoDir(repo))
		var trees []*keptTree
		for _, name := range names {
			dir := filepath.Join(ws.RepoDir(repo), name)
			here[dir] = true
			t := k.byDir[dir]
			if t != nil && t.gitDir == "" { // a folder that wasn't a worktree yet
				if t.gitDir, t.common = gitDirs(dir); t.gitDir != "" {
					k.refresh(t)
					changed = true
				}
			}
			if t == nil {
				t = &keptTree{repo: repo, name: name, dir: dir, ground: name == workspace.GroundDir}
				t.gitDir, t.common = gitDirs(dir)
				k.byDir[dir] = t
				k.refresh(t)
				changed = true
			}
			trees = append(trees, t)
		}
		if !slices.Equal(trees, k.repos[repo]) {
			changed = true
		}
		repos[repo] = trees
	}
	for dir := range k.byDir {
		if !here[dir] {
			delete(k.byDir, dir)
			changed = true
		}
	}
	if len(repos) != len(k.repos) {
		changed = true
	}
	k.repos = repos
	k.byGitDir, k.byCommon = map[string]*keptTree{}, map[string][]*keptTree{}
	for _, t := range k.byDir {
		if t.gitDir != "" {
			k.byGitDir[filepath.Clean(t.gitDir)] = t
			k.byCommon[filepath.Clean(t.common)] = append(k.byCommon[filepath.Clean(t.common)], t)
		}
	}
	if changed {
		k.changed()
	}
}

// moved is git having written in dirs: a worktree's own folder moves it,
// a repo's every worktree of it (a fetch, a ref packed).
func (k *capsuleKeeper) moved(dirs []string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	changed := false
	for _, dir := range dirs {
		dir = filepath.Clean(dir)
		if t := k.byGitDir[dir]; t != nil {
			changed = k.refresh(t) || changed
		}
		for _, t := range k.byCommon[dir] {
			changed = k.refresh(t) || changed
		}
	}
	if changed {
		k.changed()
	}
}

// sweep looks at every worktree's git files again.
func (k *capsuleKeeper) sweep() {
	k.mu.Lock()
	defer k.mu.Unlock()
	changed := false
	for _, t := range k.byDir {
		changed = k.refresh(t) || changed
		if !t.read {
			k.queue(t, false) // git couldn't be asked last time
		}
	}
	if changed {
		k.changed()
	}
}

// refresh reads t from git's files, and, if they've moved, has git asked
// of it again; it says whether they had (what's kept of it from git is
// then of it as it was, till git's said). k.mu is held.
func (k *capsuleKeeper) refresh(t *keptTree) bool {
	changed := false
	if head, branch, ok := headOf(t.dir); ok {
		changed = head != t.head || branch != t.branch
		if t.ground && t.head != "" && head != t.head && k.groundMoved != nil {
			k.groundMoved(t.repo)
		}
		t.head, t.branch = head, branch
	} else if branch := branchOf(t.dir); branch != t.branch {
		t.branch, changed = branch, true
	}
	refs, index := capsuleStamps(t.dir)
	if refs != t.refs || index != t.index || t.moves == 0 {
		// Only what's staged: whether it's dirty is all that's to ask.
		onlyIndex := t.moves > 0 && refs == t.refs
		t.refs, t.index, changed = refs, index, true
		t.moves++
		if !onlyIndex {
			t.read = false
		}
		k.queue(t, onlyIndex)
	}
	return changed
}

// askLooked asks again whether each worktree a page has shown lately is
// dirty: an edit moves nothing git keeps.
func (k *capsuleKeeper) askLooked() {
	follow := map[string]bool{}
	if f := k.following.Load(); f != nil && k.capsulesOf != nil {
		missions, worktrees := (*f)()
		for _, slug := range missions {
			for _, dir := range k.capsulesOf(slug) {
				follow[filepath.Clean(dir)] = true
			}
		}
		for _, wt := range worktrees {
			follow[wt] = true // repo/name: matched below
		}
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, t := range k.byDir {
		looked := time.Since(t.looked) < lookedFor || follow[t.dir] || follow[t.repo+"/"+t.name]
		if looked && time.Since(t.dirtyAt) >= dirtyFloor {
			k.queue(t, true)
		}
	}
}

// dirtyOf is whether a worktree has changes not committed, as kept: ok
// false when it isn't kept, or git hasn't said.
func (k *capsuleKeeper) dirtyOf(dir string) (dirty, ok bool) {
	if !k.keeping.Load() {
		return false, false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	t := k.byDir[filepath.Clean(dir)]
	if t == nil || t.dirtyAt.IsZero() {
		return false, false
	}
	return t.dirty, true
}

// queue has git asked of t, unless it is already. k.mu is held.
func (k *capsuleKeeper) queue(t *keptTree, dirtyOnly bool) {
	if t.queued || t.gitDir == "" {
		return
	}
	select {
	case k.asks <- treeAsk{t, t.moves, dirtyOnly}:
		t.queued = true
	default: // as many waiting as can: the sweep asks again
	}
}

// asker asks git of the worktrees queued, one at a time.
func (k *capsuleKeeper) asker(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case a := <-k.asks:
			k.answer(a)
		}
	}
}

func (k *capsuleKeeper) answer(a treeAsk) {
	dirty := askDirty(a.t.dir)
	var ab answer[aheadAndBehind]
	if !a.dirtyOnly {
		ab = askAheadBehind(a.t.dir)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	t := a.t
	t.queued = false
	changed := false
	if dirty.keep {
		changed = dirty.is != t.dirty
		t.dirty, t.dirtyAt = dirty.is, time.Now()
	}
	switch {
	case a.moves != t.moves:
		k.queue(t, a.dirtyOnly && t.read) // asked of it as it was: again, of it as it is
	case a.dirtyOnly:
	case dirty.keep && ab.keep:
		changed = changed || ab.is.ahead != t.ahead || ab.is.behind != t.behind || !t.read
		t.ahead, t.behind, t.read = ab.is.ahead, ab.is.behind, true
	}
	if changed {
		k.changed()
	}
}

// changed says what's kept has moved: on Seen, for the pages. k.mu is
// held; telling doesn't wait.
func (k *capsuleKeeper) changed() {
	k.version.Add(1)
	if k.tell != nil {
		k.tell()
	}
}

// Following has the keeper ask after the worktrees of what follow says is
// followed, for as long as it is: the daemon's web server says.
func (w *Workspace) Following(follow func() (missions, worktrees []string)) {
	f := followed(follow)
	w.caps.following.Store(&f)
}

// followedCapsules is a mission's capsules' folders: as its row in the
// workspace's view keeps them, or read.
func (w *Workspace) followedCapsules(slug string) []string {
	var out []string
	w.overview.mu.Lock()
	row, ok := w.overview.items[slug]
	w.overview.mu.Unlock()
	if ok {
		for _, c := range row.it.Capsules {
			out = append(out, c.Path)
		}
		return out
	}
	m, err := w.Missions.Load(slug)
	if err != nil {
		return nil
	}
	for _, c := range m.Capsules {
		out = append(out, filepath.Join(w.WS().RepoDir(c.Repo), c.Name))
	}
	return out
}

// keptWorktree is one worktree as kept, for a reader.
type keptWorktree struct {
	Name, Dir     string
	Ground        bool
	Head, Branch  string
	Dirty         bool
	Ahead, Behind int
	Read          bool
}

// worktrees is a repo's worktrees as kept, ground first, and whether
// they're kept at all (the daemon's keeping them). Those a page's shown
// are looked after: if look.
func (k *capsuleKeeper) worktrees(repo string, look bool) ([]keptWorktree, bool) {
	if !k.keeping.Load() {
		return nil, false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	out := []keptWorktree{}
	now := time.Now()
	for _, t := range k.repos[repo] {
		if look {
			t.looked = now
		}
		out = append(out, keptWorktree{Name: t.name, Dir: t.dir, Ground: t.ground, Head: t.head, Branch: t.branch, Dirty: t.dirty, Ahead: t.ahead, Behind: t.behind, Read: t.read})
	}
	return out, true
}

// headOfKept is where a worktree's HEAD is, as kept: ok false when it
// isn't (not kept, or not a worktree the keeper knows).
func (k *capsuleKeeper) headOfKept(dir string) (string, bool) {
	if !k.keeping.Load() {
		return "", false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	t := k.byDir[filepath.Clean(dir)]
	if t == nil || t.head == "" {
		return "", false
	}
	return t.head, true
}

// stampOf names how many times a worktree's git files have moved, as the
// keeper saw them: what's kept of it from git (keptGit) is good while it
// hasn't. ok false when it isn't kept.
func (k *capsuleKeeper) stampOf(dir string) (string, bool) {
	if !k.keeping.Load() {
		return "", false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	t := k.byDir[filepath.Clean(dir)]
	if t == nil {
		return "", false
	}
	return "kept " + strconv.FormatInt(t.moves, 10), true
}

// factsOf names what's kept of a worktree, its dirtiness with it: what a
// mission's page is told by (MissionStamp). ok false when it isn't kept.
func (k *capsuleKeeper) factsOf(dir string) (string, bool) {
	if !k.keeping.Load() {
		return "", false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	t := k.byDir[filepath.Clean(dir)]
	if t == nil {
		return "", false
	}
	return fmt.Sprintf("kept %d %t %t %d %d", t.moves, t.read, t.dirty, t.ahead, t.behind), true
}

// stamp names every worktree's kept facts as they are now: it moves when
// any of them does. ok false when they aren't kept.
func (k *capsuleKeeper) stamp() (string, bool) {
	if !k.keeping.Load() {
		return "", false
	}
	return "capsules " + strconv.FormatInt(k.version.Load(), 10), true
}
