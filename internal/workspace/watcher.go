package workspace

import (
	"bytes"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/git"
)

// The daemon keeps every silo: it mirrors the capsule each is pointed at
// into it as files change, gives it the capsule's files in full when it's
// pointed somewhere else or the capsule's HEAD moves, and runs its hooks.
// One SiloWatcher does it for every repo, for as long as the daemon runs;
// as it starts, it syncs each silo, taking what changed while it was down.

// SiloState is where a silo stands, as the daemon keeping it has it.
type SiloState struct {
	Repo   string `json:"repo"`
	Target string `json:"target"` // the capsule it's pointed at, or .ground
	// Watching is true while its target's changes reach it as they're
	// made; Error says why not, when it isn't.
	Watching bool   `json:"watching"`
	Error    string `json:"error,omitempty"`
	Dirs     int    `json:"dirs"` // folders of the target watched
	// Syncs are the last few, the latest last.
	Syncs []SiloSync `json:"syncs,omitempty"`
	Hook  *SiloHook  `json:"hook,omitempty"` // the last hook run
}

// SiloSync is one sync into a silo.
type SiloSync struct {
	At    time.Time `json:"at"`
	Files int       `json:"files"` // files changed: written or removed
	Full  bool      `json:"full,omitempty"`
	// Why a full one was made: "start" (the daemon's), "pointed",
	// "HEAD change" or "resync" (asked for).
	Why   string `json:"why,omitempty"`
	Error string `json:"error,omitempty"`
}

// SiloHook is a hook a silo ran.
type SiloHook struct {
	Name   string    `json:"name"` // after_create, after_switch or after_change
	At     time.Time `json:"at"`
	Error  string    `json:"error,omitempty"`
	Output string    `json:"output,omitempty"` // the end of what it said
}

// siloSyncsKept is how many syncs a silo's state keeps, and
// siloHookOutput how much of a hook's output.
const (
	siloSyncsKept  = 5
	siloHookOutput = 4 << 10
)

// SiloWatcher keeps the workspace's silos.
type SiloWatcher struct {
	Root             string
	RepoDir          func(name string) string
	SiloWorktree     func(name string) string
	MainWorktree     func(name string) string
	AfterCreateHooks map[string]string
	Verbose          bool
	// OnTargetsChanged, if set, is told the silos' targets when they've
	// changed.
	OnTargetsChanged func(map[string]string)

	watcher  *fsnotify.Watcher
	targets  map[string]string // repo -> capsule
	mu       sync.Mutex
	log      *log.Logger
	debounce map[string]*time.Timer     // repo -> debounce timer
	pending  map[string]map[string]bool // repo -> set of changed relative paths
	watched  map[string]map[string]bool // repo -> folders of its target watched
	states   map[string]*SiloState      // repo -> where it stands
	hooks    map[string]*hookTurn       // repo -> its after_change hook's turn

	// Git HEAD watching: map from resolved gitdir path to repo name,
	// so we can match .bare/worktrees/<name>/HEAD events back to a repo.
	gitdirToRepo map[string]string      // gitdir path -> repo name
	gitDebounce  map[string]*time.Timer // repo -> git HEAD debounce timer
	lastFullSync map[string]time.Time   // repo -> last fullResync completion time
}

// hookTurn keeps a repo's after_change hook to one run at a time: changes
// while it runs have it run once more when it's done.
type hookTurn struct {
	running, again bool
}

func NewSiloWatcher(w *Workspace, logger *log.Logger) (*SiloWatcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating fsnotify watcher: %w", err)
	}
	return &SiloWatcher{
		Root:             w.Root,
		RepoDir:          w.RepoDir,
		SiloWorktree:     w.SiloWorktree,
		MainWorktree:     w.MainWorktree,
		AfterCreateHooks: w.AfterCreateHooks,
		watcher:          fsw,
		targets:          make(map[string]string),
		log:              logger,
		debounce:         make(map[string]*time.Timer),
		pending:          make(map[string]map[string]bool),
		gitdirToRepo:     make(map[string]string),
		gitDebounce:      make(map[string]*time.Timer),
		lastFullSync:     make(map[string]time.Time),
	}, nil
}

func (sw *SiloWatcher) verbose(format string, args ...any) {
	if sw.Verbose {
		sw.log.Printf("[verbose] "+format, args...)
	}
}

// Watch keeps the silos until stop closes: those silo names (repo →
// capsule) as it starts, synced at once, then whatever ws.local.toml says
// as it changes.
func (sw *SiloWatcher) Watch(stop <-chan struct{}, silo map[string]string) error {
	localPath := filepath.Join(sw.Root, config.LocalFileName)
	// The workspace's folder, not the file: an editor that saves by
	// writing another file and renaming it over this one would leave a
	// watch on the file watching nothing.
	if err := sw.watcher.Add(sw.Root); err != nil {
		sw.log.Printf("silos: can't see %s change: %v", config.LocalFileName, err)
	}
	for _, repo := range sortedKeys(silo) {
		sw.keep(repo, silo[repo], "start")
	}
	if len(silo) > 0 {
		sw.log.Printf("silos: keeping %d", len(silo))
	}

	for {
		select {
		case <-stop:
			sw.watcher.Close()
			return nil
		case event, ok := <-sw.watcher.Events:
			if !ok {
				return nil
			}
			sw.handleEvent(event, localPath)
		case err, ok := <-sw.watcher.Errors:
			if !ok {
				return nil
			}
			sw.log.Printf("silos: watcher error: %v", err)
		}
	}
}

// keep starts keeping repo's silo on capsule: it gives the silo the
// capsule's files, runs its hooks if the silo had another's (after_create
// and after_switch), or its after_change if the daemon's starting and
// something changed while it was down, and watches the capsule.
func (sw *SiloWatcher) keep(repo, capsule, why string) {
	st := sw.setState(repo, func(st *SiloState) {
		st.Target, st.Watching, st.Error, st.Dirs = capsule, false, "", 0
	})
	_ = st
	capsuleDir := filepath.Join(sw.RepoDir(repo), capsule)
	siloDir := sw.SiloWorktree(repo)
	if capsule == "" || !dirExists(capsuleDir) {
		sw.fail(repo, fmt.Sprintf("%s has no capsule %s", repo, capsule))
		return
	}
	if !dirExists(siloDir) {
		sw.fail(repo, "there's no "+SiloDir+" folder (ws silo point makes it)")
		return
	}
	switched := SiloTarget(siloDir) != capsule
	sw.mu.Lock()
	sw.lastFullSync[repo] = time.Now()
	sw.mu.Unlock()
	n, err := FullSync(capsuleDir, siloDir)
	sw.synced(repo, SiloSync{At: time.Now(), Files: n, Full: true, Why: why}, err)
	if switched {
		MarkSiloTarget(siloDir, capsule) //nolint:errcheck // unmarked, the next start runs the hooks again
	}
	if err := sw.addWatch(repo, capsule); err != nil {
		sw.fail(repo, fmt.Sprintf("can't watch %s: %v", capsule, err))
	}
	switch {
	case switched:
		sw.runSwitchHooks(repo, siloDir)
	case n > 0 && why == "start":
		sw.changed(repo, siloDir)
	}
}

// addWatch watches capsule's folders that hold files a silo is given (and
// those between them and the capsule), and its HEAD. An ignored folder
// (node_modules, a build's output) isn't watched: on macOS every file in a
// watched folder holds a file open.
func (sw *SiloWatcher) addWatch(repo, capsule string) error {
	capsuleDir := filepath.Join(sw.RepoDir(repo), capsule)
	dirs, err := syncableDirs(capsuleDir, "")
	if err != nil {
		return err
	}
	for _, d := range dirs {
		sw.watchDir(repo, filepath.Join(capsuleDir, d))
	}

	// Watch the real gitdir for HEAD changes. In a bare-repo worktree layout,
	// .git is a file containing "gitdir: <path>" pointing to .bare/worktrees/<name>/.
	sw.watchGitDir(repo, capsuleDir)

	sw.mu.Lock()
	sw.targets[repo] = capsule
	sw.mu.Unlock()
	n := sw.watchedCount(repo)
	sw.setState(repo, func(st *SiloState) {
		st.Watching = st.Error == ""
		st.Dirs = n
	})
	sw.log.Printf("silos: watching %s -> %s (%d folders)", repo, capsule, n)
	return nil
}

// syncableDirs are the folders under dir's sub (relative to dir; "" for
// all of it) that hold files a silo is given, with every folder between
// them and sub, sub among them.
func syncableDirs(dir, sub string) ([]string, error) {
	args := []string{"ls-files", "-z", "--cached", "--others", "--exclude-standard"}
	if sub != "" {
		args = append(args, "--", sub)
	}
	out, err := git.Combined(dir, args...)
	if err != nil {
		return nil, err
	}
	stop := filepath.Clean(sub)
	if sub == "" {
		stop = "."
	}
	seen := map[string]bool{stop: true}
	for f := range strings.SplitSeq(strings.TrimRight(out, "\x00"), "\x00") {
		if f == "" {
			continue
		}
		for d := filepath.Dir(f); !seen[d]; d = filepath.Dir(d) {
			seen[d] = true
			if d == "." || d == "/" {
				break
			}
		}
	}
	return sortedKeys(seen), nil
}

func (sw *SiloWatcher) watchDir(repo, dir string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.watched == nil {
		sw.watched = make(map[string]map[string]bool)
	}
	if sw.watched[repo] == nil {
		sw.watched[repo] = make(map[string]bool)
	}
	if sw.watched[repo][dir] {
		return
	}
	if err := sw.watcher.Add(dir); err != nil {
		sw.verbose("%s: can't watch %s: %v", repo, dir, err)
		return
	}
	sw.watched[repo][dir] = true
}

// watchedCount is how many of repo's target's folders are watched.
func (sw *SiloWatcher) watchedCount(repo string) int {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return len(sw.watched[repo])
}

// watchGitDir resolves the real gitdir for a worktree and watches it
// for HEAD changes (triggered by checkout, pull, rebase, etc.).
func (sw *SiloWatcher) watchGitDir(repo, capsuleDir string) {
	gitdir := resolveGitDir(capsuleDir)
	if gitdir == "" {
		return
	}
	if err := sw.watcher.Add(gitdir); err != nil {
		sw.log.Printf("silos: could not watch gitdir for %s: %v", repo, err)
		return
	}
	sw.mu.Lock()
	sw.gitdirToRepo[gitdir] = repo
	sw.mu.Unlock()
}

// resolveGitDir returns the actual git directory for a worktree.
// For linked worktrees, .git is a file containing "gitdir: <path>".
// For regular repos, .git is a directory.
func resolveGitDir(wtPath string) string {
	gitPath := filepath.Join(wtPath, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return gitPath
	}
	// Linked worktree — .git is a file
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir: ") {
		return ""
	}
	gitdir := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(wtPath, gitdir)
	}
	gitdir = filepath.Clean(gitdir)
	return gitdir
}

func (sw *SiloWatcher) removeWatch(repo string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	delete(sw.targets, repo)
	delete(sw.pending, repo)
	if t, ok := sw.debounce[repo]; ok {
		t.Stop()
		delete(sw.debounce, repo)
	}
	for gitdir, r := range sw.gitdirToRepo {
		if r == repo {
			_ = sw.watcher.Remove(gitdir) // it may not have been watched
			delete(sw.gitdirToRepo, gitdir)
		}
	}
	for dir := range sw.watched[repo] {
		_ = sw.watcher.Remove(dir) // it may not have been watched
	}
	delete(sw.watched, repo)
}

func (sw *SiloWatcher) handleEvent(event fsnotify.Event, localPath string) {
	if event.Name == localPath {
		if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
			sw.reloadTargets()
		}
		return
	}
	if filepath.Dir(event.Name) == sw.Root {
		return // another of the workspace's own files
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Check if this is a HEAD change from a watched gitdir.
	// HEAD lives at <gitdir>/HEAD (e.g. .bare/worktrees/<name>/HEAD).
	// We watch HEAD instead of the index because index changes fire for many
	// reasons (git gc, stat cache refreshes) including from OTHER worktrees,
	// while HEAD only changes for operations that modify the working tree
	// (checkout, pull, rebase, merge, reset) in THIS worktree.
	if filepath.Base(event.Name) == "HEAD" {
		gitdir := filepath.Dir(event.Name)
		if repo, ok := sw.gitdirToRepo[gitdir]; ok {
			sw.verbose("%s: HEAD event", repo)
			// Ignore HEAD events that arrive shortly after a full resync
			// to prevent potential feedback loops.
			if last, ok := sw.lastFullSync[repo]; ok && time.Since(last) < 2*time.Second {
				sw.verbose("%s: HEAD event suppressed (fullResync %.1fs ago)", repo, time.Since(last).Seconds())
				return
			}
			if t, ok := sw.gitDebounce[repo]; ok {
				t.Stop()
			}
			sw.verbose("%s: HEAD debounce scheduled (500ms)", repo)
			r := repo
			sw.gitDebounce[repo] = time.AfterFunc(500*time.Millisecond, func() {
				sw.fullResync(r, "HEAD change")
			})
			return
		}
	}

	for repo, capsule := range sw.targets {
		capsuleDir := filepath.Join(sw.RepoDir(repo), capsule)
		if !strings.HasPrefix(event.Name, capsuleDir+string(os.PathSeparator)) {
			continue
		}

		relPath, _ := filepath.Rel(capsuleDir, event.Name)

		// Skip .git events (the .git file itself, not the resolved gitdir)
		if strings.HasPrefix(relPath, ".git"+string(os.PathSeparator)) || relPath == ".git" {
			return
		}

		// Skip attribute-only events. On macOS, kqueue reports atime updates
		// from the watcher's own SyncFile reads as Chmod events, which would
		// otherwise retrigger a sync of the same file forever.
		if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
			sw.verbose("%s: attribute-only event, skipping", relPath)
			return
		}

		sw.verbose("event: %s %q", event.Op, relPath)

		// A folder made: watched, unless it's ignored, and what's in it
		// already queued (it may have been made with its files, as a
		// checkout or an unpacked archive makes them, before the watch).
		if event.Has(fsnotify.Create) {
			if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
				if IsGitIgnored(capsuleDir, relPath+"/") {
					sw.verbose("%s: ignored folder made, not watched", relPath)
					return
				}
				sw.mu.Unlock()
				sw.addFolder(repo, capsuleDir, relPath)
				sw.mu.Lock()
				return
			}
		}

		if IsGitIgnored(capsuleDir, relPath) {
			sw.verbose("%s: gitignored, skipping", relPath)
			return
		}

		if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
			// Don't blindly delete — the file may have been replaced (git uses
			// rename-to-temp + rename-into-place). Mark it for a check at flush time.
			sw.queue(repo, "CHECK:"+relPath)
		} else {
			sw.queue(repo, relPath)
		}
		return
	}
}

// addFolder watches a folder made in repo's target, and queues the files
// already in it. sw.mu isn't held.
func (sw *SiloWatcher) addFolder(repo, capsuleDir, rel string) {
	dirs, err := syncableDirs(capsuleDir, rel)
	if err != nil {
		sw.verbose("%s: can't list %s: %v", repo, rel, err)
		return
	}
	for _, d := range dirs {
		sw.watchDir(repo, filepath.Join(capsuleDir, d))
	}
	out, err := git.Combined(capsuleDir, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", rel)
	if err != nil {
		return
	}
	sw.mu.Lock()
	defer sw.mu.Unlock()
	for f := range strings.SplitSeq(strings.TrimRight(out, "\x00"), "\x00") {
		if f != "" {
			sw.queue(repo, f)
		}
	}
	sw.verbose("%s: folder %s made, watched", repo, rel)
}

// queue adds path to what repo's next sync copies, and has it made soon.
// sw.mu is held.
func (sw *SiloWatcher) queue(repo, path string) {
	if sw.pending[repo] == nil {
		sw.pending[repo] = make(map[string]bool)
	}
	sw.pending[repo][path] = true
	sw.verbose("%s: added to pending (%d pending)", path, len(sw.pending[repo]))
	if t, ok := sw.debounce[repo]; ok {
		t.Stop()
	}
	sw.debounce[repo] = time.AfterFunc(200*time.Millisecond, func() {
		sw.flushPending(repo)
	})
}

func (sw *SiloWatcher) flushPending(repo string) {
	sw.mu.Lock()
	pending := sw.pending[repo]
	sw.pending[repo] = nil
	capsule := sw.targets[repo]
	sw.mu.Unlock()

	if len(pending) == 0 || capsule == "" {
		return
	}

	sw.verbose("%s: flushing %d pending file(s)", repo, len(pending))

	capsuleDir := filepath.Join(sw.RepoDir(repo), capsule)
	siloDir := sw.SiloWorktree(repo)
	synced := 0
	var failed, added, removed []string

	for path := range pending {
		relPath, check := strings.CutPrefix(path, "CHECK:")
		if check {
			if _, err := os.Stat(filepath.Join(capsuleDir, relPath)); err != nil {
				// File is actually gone. Remove from silo.
				if RemoveSyncedFile(siloDir, relPath) == nil {
					synced++
					removed = append(removed, relPath)
				}
				continue
			}
			// File still exists (was replaced, not deleted). Sync it.
		}
		wrote, err := syncFile(capsuleDir, siloDir, relPath)
		switch {
		case err != nil:
			failed = append(failed, relPath)
			sw.log.Printf("silos: sync error %s/%s: %v", repo, relPath, err)
		case wrote:
			synced++
			added = append(added, relPath)
		}
	}
	// The manifest has what the silo was given, so a file that came this
	// way goes when it's pointed at a capsule without it.
	if err := noteSynced(siloDir, added, removed); err != nil {
		sw.log.Printf("silos: %s: noting what was synced: %v", repo, err)
	}
	var err error
	if len(failed) > 0 {
		sort.Strings(failed)
		err = fmt.Errorf("failed to sync %d file(s): %s", len(failed), strings.Join(failed, ", "))
	}
	if synced == 0 && err == nil {
		return
	}
	sw.synced(repo, SiloSync{At: time.Now(), Files: synced}, err)
	if synced > 0 {
		sw.changed(repo, siloDir)
	}
}

// fullResync gives repo's silo its target's files in full, and watches
// folders made since. Called when a git operation (pull, checkout, etc.)
// is detected via HEAD changes, or a resync is asked for.
func (sw *SiloWatcher) fullResync(repo, reason string) {
	sw.verbose("%s: fullResync triggered by %s", repo, reason)
	sw.mu.Lock()
	capsule := sw.targets[repo]
	// Clear any pending incremental syncs — the full sync covers everything.
	delete(sw.pending, repo)
	if t, ok := sw.debounce[repo]; ok {
		t.Stop()
		delete(sw.debounce, repo)
	}
	// Stamp now so HEAD events arriving during the resync are ignored.
	sw.lastFullSync[repo] = time.Now()
	sw.mu.Unlock()

	if capsule == "" {
		return
	}

	capsuleDir := filepath.Join(sw.RepoDir(repo), capsule)
	siloDir := sw.SiloWorktree(repo)

	count, err := FullSync(capsuleDir, siloDir)
	sw.synced(repo, SiloSync{At: time.Now(), Files: count, Full: true, Why: reason}, err)

	if dirs, err := syncableDirs(capsuleDir, ""); err == nil {
		for _, d := range dirs {
			sw.watchDir(repo, filepath.Join(capsuleDir, d))
		}
		n := sw.watchedCount(repo)
		sw.setState(repo, func(st *SiloState) { st.Dirs = n })
	}

	// Record completion time so we can ignore HEAD events arriving shortly after our own resync.
	sw.mu.Lock()
	sw.lastFullSync[repo] = time.Now()
	sw.mu.Unlock()

	if count > 0 {
		sw.changed(repo, siloDir)
	}
}

// Resync gives repo's silo its target's files in full, or every silo's
// for "". It says so for a repo whose silo isn't kept.
func (sw *SiloWatcher) Resync(repo string) error {
	sw.mu.Lock()
	var repos []string
	for r := range sw.targets {
		if repo == "" || r == repo {
			repos = append(repos, r)
		}
	}
	sw.mu.Unlock()
	if repo != "" && len(repos) == 0 {
		return fmt.Errorf("%s's silo isn't kept: it's pointed nowhere, or at what isn't there", repo)
	}
	for _, r := range repos {
		sw.fullResync(r, "resync")
	}
	return nil
}

// FullResyncAll triggers a full re-sync for every active silo target.
func (sw *SiloWatcher) FullResyncAll() { sw.Resync("") } //nolint:errcheck // every one there is

// reloadTargets reads where ws.local.toml points the silos, and keeps
// each as it says: a silo pointed elsewhere is given its new target's
// files and its hooks are run, unless whoever pointed it has done both
// (ws silo point, which marks the silo), and one no longer named is let go.
func (sw *SiloWatcher) reloadTargets() {
	local, err := config.ReadLocal(sw.Root)
	if err != nil {
		sw.log.Printf("silos: can't read %s: %v", config.LocalFileName, err)
		return
	}
	want := local.Silo
	if want == nil {
		want = map[string]string{}
	}

	sw.mu.Lock()
	had := make(map[string]string, len(sw.targets))
	maps.Copy(had, sw.targets)
	for k, st := range sw.states {
		if _, ok := had[k]; !ok {
			had[k] = st.Target // kept in name only: its target wasn't there
		}
	}
	sw.mu.Unlock()

	changed := false
	for _, repo := range sortedKeys(had) {
		if _, ok := want[repo]; !ok {
			sw.log.Printf("silos: letting %s go", repo)
			sw.removeWatch(repo)
			sw.mu.Lock()
			delete(sw.states, repo)
			delete(sw.lastFullSync, repo)
			sw.mu.Unlock()
			changed = true
		}
	}
	for _, repo := range sortedKeys(want) {
		capsule := want[repo]
		if old, ok := had[repo]; ok && old == capsule {
			continue
		}
		sw.log.Printf("silos: %s pointed at %s", repo, capsule)
		sw.removeWatch(repo)
		sw.keep(repo, capsule, "pointed")
		changed = true
	}

	if changed && sw.OnTargetsChanged != nil {
		sw.OnTargetsChanged(want)
	}
}

// States is where each silo stands, by repo.
func (sw *SiloWatcher) States() []SiloState {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	out := make([]SiloState, 0, len(sw.states))
	for _, repo := range sortedKeys(sw.states) {
		st := *sw.states[repo]
		st.Syncs = append([]SiloSync(nil), st.Syncs...)
		if st.Hook != nil {
			h := *st.Hook
			st.Hook = &h
		}
		out = append(out, st)
	}
	return out
}

// setState changes repo's state, made if it has none.
func (sw *SiloWatcher) setState(repo string, fn func(*SiloState)) SiloState {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.states == nil {
		sw.states = make(map[string]*SiloState)
	}
	st, ok := sw.states[repo]
	if !ok {
		st = &SiloState{Repo: repo}
		sw.states[repo] = st
	}
	fn(st)
	return *st
}

func (sw *SiloWatcher) fail(repo, why string) {
	sw.log.Printf("silos: not keeping %s: %s", repo, why)
	sw.setState(repo, func(st *SiloState) { st.Watching, st.Error = false, why })
}

func (sw *SiloWatcher) synced(repo string, s SiloSync, err error) {
	if err != nil {
		s.Error = err.Error()
		sw.log.Printf("silos: %s: %v", repo, err)
	} else if s.Full {
		sw.log.Printf("silos: %s: %s, %d file(s) changed", repo, s.Why, s.Files)
	} else {
		sw.verbose("%s: synced %d file(s)", repo, s.Files)
	}
	sw.setState(repo, func(st *SiloState) {
		st.Syncs = append(st.Syncs, s)
		if len(st.Syncs) > siloSyncsKept {
			st.Syncs = st.Syncs[len(st.Syncs)-siloSyncsKept:]
		}
	})
}

// runSwitchHooks runs a silo's after_create (ws.local.toml's, ws.toml's,
// else ws.repo.toml's) and after_switch, as ws silo point does.
func (sw *SiloWatcher) runSwitchHooks(repo, siloDir string) {
	cfg, _ := config.ParseRepoConfig(filepath.Join(sw.MainWorktree(repo), config.RepoFileName))
	hook, ok := sw.AfterCreateHooks[repo]
	if !ok && cfg != nil {
		hook = cfg.Capsule.AfterCreate
	}
	if hook != "" {
		sw.runHook(repo, siloDir, "after_create", hook)
	}
	if cfg != nil && cfg.Silo.AfterSwitch != "" {
		sw.runHook(repo, siloDir, "after_switch", cfg.Silo.AfterSwitch)
	}
}

// changed runs repo's after_change hook, if it has one: once at a time,
// with changes made while it runs having it run once more after.
func (sw *SiloWatcher) changed(repo, siloDir string) {
	cfg, err := config.ParseRepoConfig(filepath.Join(sw.MainWorktree(repo), config.RepoFileName))
	if err != nil || cfg == nil || cfg.Silo.AfterChange == "" {
		return
	}
	sw.mu.Lock()
	if sw.hooks == nil {
		sw.hooks = make(map[string]*hookTurn)
	}
	turn := sw.hooks[repo]
	if turn == nil {
		turn = &hookTurn{}
		sw.hooks[repo] = turn
	}
	if turn.running {
		turn.again = true
		sw.mu.Unlock()
		return
	}
	turn.running = true
	sw.mu.Unlock()
	for {
		sw.runHook(repo, siloDir, "after_change", cfg.Silo.AfterChange)
		sw.mu.Lock()
		if !turn.again {
			turn.running = false
			sw.mu.Unlock()
			return
		}
		turn.again = false
		sw.mu.Unlock()
	}
}

// runHook runs one of a silo's hooks in it, and keeps how it went.
func (sw *SiloWatcher) runHook(repo, siloDir, name, command string) {
	var out bytes.Buffer
	err := RunHook(siloDir, command, &out, &out)
	h := &SiloHook{Name: name, At: time.Now(), Output: endOf(out.String(), siloHookOutput)}
	if err != nil {
		h.Error = err.Error()
		sw.log.Printf("silos: %s's %s hook failed: %v", repo, name, err)
	} else {
		sw.verbose("%s: ran %s hook", repo, name)
	}
	sw.setState(repo, func(st *SiloState) { st.Hook = h })
}

// endOf is the end of s, at most n bytes of it, from a line's start.
func endOf(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[len(s)-n:]
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	return s
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
