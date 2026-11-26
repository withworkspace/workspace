package ops

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/withworkspace/workspace/internal/workspace"
)

// What the pages are told of capsules is found by looking: the hubs
// that tell them stat what git keeps of each (Stamp, MissionStamp), every
// couple of seconds. So a commit in a capsule reached its mission's page
// a second or two after it was made.
//
// The daemon watches the folders git writes to instead, and says when
// one's written (Seen, as a look behind a read does), so the hubs look
// then. Nothing's read from the events: what moved is still found by the
// hubs' stamps, and their poll goes on, for what a watch can't see or
// couldn't be set.
//
// Watched are each worktree's own git folder (.bare/worktrees/<name>: its
// HEAD, its index, the message of its last commit, so a commit, something
// staged, a checkout, a reset) and each repo's .bare (FETCH_HEAD and
// packed-refs: a fetch). Neither's watched below itself: not a repo's
// objects, nor its refs, which are as many files as it has branches (on
// macOS every file in a watched folder is held open). A capsule's own
// files aren't watched either: one edited and not committed is found as
// it was (keptGit's look, past dirtyFloor).

var (
	// gitSettle is how long after git first writes somewhere the hubs are
	// told: a commit writes several files, and what's looked at should be
	// what it came to.
	gitSettle = 50 * time.Millisecond
	// gitGap is the least time between two tellings: crew at work run git
	// all the time, and each telling is a look at every page that's open.
	gitGap = 250 * time.Millisecond
	// gitFoldersEvery is how often the folders to watch are listed again:
	// a capsule lifted is watched from then, one burned no longer.
	gitFoldersEvery = 5 * time.Second
)

// gitWatchFolders are the folders git writes to when a repo's worktrees or
// its remote's branches move: each repo's own, and each worktree's.
func gitWatchFolders(ws *workspace.Workspace) []string {
	var out []string
	for _, repo := range ws.RepoNames {
		names, _ := workspace.ListAllWorktrees(ws.RepoDir(repo))
		common := ""
		for _, name := range names {
			gitDir, c := gitDirs(filepath.Join(ws.RepoDir(repo), name))
			if gitDir == "" {
				continue
			}
			out = append(out, filepath.Clean(gitDir))
			common = filepath.Clean(c)
		}
		if common != "" && !slices.Contains(out, common) {
			out = append(out, common)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// WatchGit keeps the workspace's worktrees (capsulekeeper.go) until stop
// closes, moved by what git writes in the folders it watches: Seen says
// when what's kept has moved. Where they can't be watched at all (logf
// says why), the worktrees are kept by the keeper's own looks, and the
// hubs' polls.
func (w *Workspace) WatchGit(stop <-chan struct{}, logf func(string, ...any)) {
	// What else the pages are built from that's written to files: the
	// workspace's config, and what's under .ws (Stamp's): a file a
	// command writes there reaches the pages as it's written.
	go watchFolders(stop, func(format string, args ...any) { logf("files: "+format, args...) },
		w.stampFolders, w.stampHeeds, func([]string) { w.tellSeen() }, nil)
	moved := make(chan []string, 64)
	kept := make(chan struct{})
	go func() {
		defer close(kept)
		w.keepCapsules(stop, moved)
	}()
	defer func() { <-kept }() // returns once nothing's kept, as stop says
	w.watchGit(stop, logf, func(dirs []string) {
		select {
		case moved <- dirs:
		case <-stop:
		}
	}, nil)
}

// watchGit watches the folders git writes to, telling moved which were
// written (once settled); watching, if set, is told how many folders are
// watched each time they've been listed.
func (w *Workspace) watchGit(stop <-chan struct{}, logf func(string, ...any), moved func(dirs []string), watching func(int)) {
	watchFolders(stop, func(format string, args ...any) { logf("git: "+format, args...) },
		func() []string { return gitWatchFolders(w.WS()) }, nil, moved, watching)
}

// watchFolders watches the folders folders lists (listed again every
// gitFoldersEvery) until stop closes, telling moved which were written,
// once settled; watching, if set, is told how many are watched each time
// they've been listed. A file written that heed (if set) says isn't worth
// telling is let be.
func watchFolders(stop <-chan struct{}, logf func(string, ...any), folders func() []string, heed func(path string) bool, moved func(dirs []string), watching func(int)) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		logf("its folders can't be watched (%v): pages hear of what's moved at the next look", err)
		return
	}
	defer fw.Close()

	watched := map[string]bool{}
	refused := map[string]bool{} // said once each
	list := func() {
		now := map[string]bool{}
		for _, dir := range folders() {
			now[dir] = true
			if watched[dir] {
				continue
			}
			if err := fw.Add(dir); err != nil {
				if !refused[dir] {
					refused[dir] = true
					logf("%s can't be watched: %v", dir, err)
				}
				continue
			}
			watched[dir] = true
		}
		for dir := range watched {
			if !now[dir] {
				fw.Remove(dir) //nolint:errcheck // gone with its folder
				delete(watched, dir)
			}
		}
		if watching != nil {
			watching(len(watched))
		}
	}
	list()

	again := time.NewTicker(gitFoldersEvery)
	defer again.Stop()
	var due <-chan time.Time
	var last time.Time
	written := map[string]bool{} // the folders written since moved was last told
	for {
		select {
		case <-stop:
			return
		case <-again.C:
			list()
		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			// A file's mode or times alone: nothing git says has moved.
			if ev.Op == fsnotify.Chmod || heed != nil && !heed(ev.Name) {
				continue
			}
			// A folder made is listed at once, not at the next listing: what's
			// written in it next is watched.
			if ev.Has(fsnotify.Create) {
				if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
					list()
				}
			}
			written[watchedFolder(ev.Name, watched)] = true
			if due != nil {
				continue
			}
			wait := gitSettle
			if since := time.Since(last); since < gitGap {
				wait = max(wait, gitGap-since)
			}
			due = time.After(wait)
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			logf("watching its folders: %v", err)
		case <-due:
			due, last = nil, time.Now()
			dirs := slices.Sorted(maps.Keys(written))
			clear(written)
			moved(dirs)
		}
	}
}

// watchedFolder is the watched folder an event's path is in: the folder
// itself when it's what moved (made, or removed).
func watchedFolder(path string, watched map[string]bool) string {
	if watched[path] {
		return path
	}
	return filepath.Dir(path)
}
