package ops

import (
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

// WatchGit watches the folders git writes to until stop closes, and says
// on Seen when one's been written. It returns at once where they can't
// be watched at all (logf says why): the hubs' poll is what there is then.
func (w *Workspace) WatchGit(stop <-chan struct{}, logf func(string, ...any)) {
	w.watchGit(stop, logf, w.tellSeen, nil)
}

// watchGit is WatchGit, telling moved; watching, if set, is told how many
// folders are watched each time they've been listed.
func (w *Workspace) watchGit(stop <-chan struct{}, logf func(string, ...any), moved func(), watching func(int)) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		logf("git: its folders can't be watched (%v): pages hear of a capsule that's moved at the next look", err)
		return
	}
	defer fw.Close()

	watched := map[string]bool{}
	refused := map[string]bool{} // said once each
	list := func() {
		now := map[string]bool{}
		for _, dir := range gitWatchFolders(w.WS()) {
			now[dir] = true
			if watched[dir] {
				continue
			}
			if err := fw.Add(dir); err != nil {
				if !refused[dir] {
					refused[dir] = true
					logf("git: %s can't be watched: %v", dir, err)
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
			if ev.Op == fsnotify.Chmod || due != nil {
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
			logf("git: watching its folders: %v", err)
		case <-due:
			due, last = nil, time.Now()
			moved()
		}
	}
}
