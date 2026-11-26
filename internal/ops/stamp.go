package ops

import (
	"context"
	"fmt"
	"github.com/withworkspace/workspace/internal/config"
	"hash"
	"hash/fnv"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
)

// What a page shows changes when the records or a file under .ws are
// written, or a capsule moves (a commit, a checkout, what's staged), or
// GitHub says something new. Stamp names all of that as it is now, from
// the records' latest change's number and the files' sizes and times (of
// missions in play: an archived one's folder isn't looked in): no process
// is started and no file is read, so a
// page's change check can take it every few seconds and build the view
// (Overview) only when it's moved. (Not a file watcher: on macOS each
// watched file holds a descriptor, and .ws has hundreds.)
func (w *Workspace) Stamp() string {
	h := fnv.New64a()
	note := func(path string, info fs.FileInfo) {
		fmt.Fprintf(h, "%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
	}
	ws := filepath.Join(w.Root(), ".ws")
	walk := func(dir string, skipMissions bool) {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(ws, path)
			if d.IsDir() {
				if skipDir(rel) || skipMissions && rel == "missions" {
					return filepath.SkipDir
				}
				return nil // a file made or removed in it shows in the walk
			}
			if skipFile(d.Name()) || isArtifactVersion(rel) {
				return nil
			}
			if info, err := d.Info(); err == nil {
				note(rel, info)
			}
			return nil
		})
	}
	// An archived mission's folder isn't looked in: nothing a page shows
	// of it comes from there. (Without the records, every one is.)
	inPlay, known := w.Missions.InPlaySlugs()
	walk(ws, known)
	// ws.toml and ws.local.toml: what the view says of config (its spaces,
	// where the web UI's places go) moves with them.
	fmt.Fprintln(h, configStamp(w.Root()))
	if known {
		for _, slug := range slices.Sorted(maps.Keys(inPlay)) {
			walk(filepath.Join(ws, "missions", slug), false)
		}
	}
	// Each capsule's HEAD, what's staged, and its HEAD's log (a commit,
	// a checkout), as its repo's bare clone keeps them: as the daemon keeps
	// them, or looked at here.
	if kept, ok := w.caps.stamp(); ok {
		fmt.Fprintln(h, kept)
	}
	for _, repo := range w.WS().RepoNames {
		if w.caps.keeping.Load() {
			break
		}
		trees := filepath.Join(w.WS().BareDir(repo), "worktrees")
		entries, _ := os.ReadDir(trees)
		for _, e := range entries {
			for _, f := range []string{"HEAD", "index", filepath.Join("logs", "HEAD")} {
				p := filepath.Join(trees, e.Name(), f)
				if info, err := os.Stat(p); err == nil {
					note(p, info)
				}
			}
		}
		if info, err := os.Stat(w.WS().RepoDir(repo)); err == nil {
			note(repo, info) // a capsule's folder made, or burned
		}
	}
	// GitHub's answer for a repo, different from what was kept (asked
	// behind a read, or by the watcher).
	fmt.Fprintf(h, "prs %d\n", w.opts.PRs.Changes())
	// A capsule found dirty, or clean, since a view last said: nothing git
	// keeps moves when a file's edited (keptGit).
	fmt.Fprint(h, w.git.dirtyMoved())
	// A space found other than the pages were given, behind a read of it
	// (keptSpaces).
	fmt.Fprintf(h, "spaces %s\n", w.SpacesSeen())
	// The records say themselves when they've changed: a write that names
	// a topic moves their latest change's number.
	if db, err := store.Dial(w.Root()); err == nil {
		seq, _ := db.Seq(context.Background())
		fmt.Fprintf(h, "records %d\n", seq)
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// MissionStamp names what a mission's page is built from that the records
// don't say (they tell of their own changes): its folder's files (its
// brief, files/, reviews, a run's output) and its capsules' HEAD, index
// and HEAD's log. A few dozen stats, for a mission someone has open.
//
// In the daemon, nothing's walked or read: the writes in its folder are
// counted as they're made, and its record is as its page last read it
// (keptMissions).
func (w *Workspace) MissionStamp(slug string) string {
	m, ok := w.missions.mission(slug)
	if !ok {
		var err error
		if m, err = w.Missions.Load(slug); err != nil {
			m = nil
		} else {
			w.missions.keep(slug, m)
		}
	}
	return w.missionStamp(slug, m)
}

// missionStamp is MissionStamp of m as it was just read (nil: it couldn't
// be).
func (w *Workspace) missionStamp(slug string, m *mission.Mission) string {
	h := fnv.New64a()
	note := func(path string) {
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(h, "%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
		}
	}
	dir := filepath.Join(w.Missions.Dir(), slug)
	if files, ok := w.missions.filesOf(slug); ok {
		fmt.Fprintln(h, files) // as the watch has counted them
	} else {
		w.walkMission(dir, note)
	}
	return w.capsulesStamp(h, m)
}

// walkMission notes each file in a mission's folder that its page is
// built from.
func (w *Workspace) walkMission(dir string, note func(path string)) {
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			if rel == "docs" || rel == "orchestrator" {
				return filepath.SkipDir // versions, written once; runs, the orchestrator page's
			}
			return nil
		}
		if !skipFile(d.Name()) {
			note(path)
		}
		return nil
	})
}

// capsulesStamp adds what m's page shows of its capsules and its repos'
// pull requests to h, and is the stamp it comes to.
func (w *Workspace) capsulesStamp(h hash.Hash64, m *mission.Mission) string {
	note := func(path string) {
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(h, "%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
		}
	}
	if m != nil {
		// What GitHub says of its repos' pull requests, when it's said
		// something different (asked behind the page's read: PRCache).
		for _, repo := range MissionRepos([]*mission.Mission{m}) {
			fmt.Fprintf(h, "prs %s %d\n", repo, w.opts.PRs.Changes(repo))
		}
		for _, c := range m.Capsules {
			dir := filepath.Join(w.WS().RepoDir(c.Repo), c.Name)
			if kept, ok := w.caps.factsOf(dir); ok {
				fmt.Fprintln(h, dir, kept) // as the daemon keeps it
			} else {
				trees := filepath.Join(w.WS().BareDir(c.Repo), "worktrees", c.Name)
				for _, f := range []string{"HEAD", "index", filepath.Join("logs", "HEAD")} {
					note(filepath.Join(trees, f))
				}
				note(dir) // made, or burned
			}
			// Its runs: one started, up, ended (its process gone too) or
			// cleared away (runs.Store.StampIn).
			fmt.Fprintln(h, w.runsStampIn(c.Repo, c.Name))
			// Found dirty, or clean, since the page was last told (keptGit).
			fmt.Fprint(h, w.git.dirtyMoved(dir))
		}
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// ConversationStamp names what a conversation's page shows that its
// records don't say (they tell of their own changes): what its agent's
// written of its steps as it answers, in its run file. A stat, for a
// conversation someone has open.
func (w *Workspace) ConversationStamp(scope, id string) string {
	path := conversation.For(w.Root(), scope).RunFile(id)
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d %d", info.Size(), info.ModTime().UnixNano())
}

// stampFolders are the folders Stamp looks in: the workspace's own (its
// config), .ws, and every folder under it Stamp walks.
func (w *Workspace) stampFolders() []string {
	root := w.Root()
	ws := filepath.Join(root, ".ws")
	out := []string{root}
	inPlay, known := w.Missions.InPlaySlugs()
	add := func(dir string, skipMissions bool) {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(ws, path)
			if rel != "." && (skipDir(rel) || skipMissions && rel == "missions") {
				return filepath.SkipDir
			}
			out = append(out, path)
			return nil
		})
	}
	add(ws, known)
	if known {
		// The missions' own folder, not below it: a mission's folder made
		// is watched from then.
		out = append(out, filepath.Join(ws, "missions"))
		for _, slug := range slices.Sorted(maps.Keys(inPlay)) {
			add(filepath.Join(ws, "missions", slug), false)
		}
	}
	return out
}

// stampHeeds is whether a file written at path moves what Stamp says: in
// the workspace's own folder only its config does, and under .ws what
// Stamp doesn't skip.
func (w *Workspace) stampHeeds(path string) bool {
	root := w.Root()
	if filepath.Dir(path) == root {
		name := filepath.Base(path)
		return name == config.FileName || name == config.LocalFileName
	}
	rel, err := filepath.Rel(filepath.Join(root, ".ws"), path)
	return err == nil && !skipFile(filepath.Base(path)) && !isArtifactVersion(rel)
}

// skipDir is what under .ws no page's view is built from, or is kept by a
// page of its own (Team's, Reviews'): its own change checks follow it.
func skipDir(rel string) bool {
	switch rel {
	case "team", "prs", "bin", "hooks", "converse":
		return true
	case "backup", "swept", "locks": // copies of the records; what ws doctor swept aside; locks held while files are written
		return true
	case "cache", "runs":
		// What's kept to be quick (code nav's facts: thousands of files,
		// touched as they're read) and what ws run's processes write as
		// they go (their output): neither is in the view, and walking
		// them had it built again every tick while anything ran.
		return true
	case filepath.Join("artifacts", ".discarded"): // what's left of a discarded one is its row's
		return true
	}
	// Versions are written once and never shown changed.
	return rel == filepath.Join("docs", "versions") || strings.HasSuffix(rel, string(filepath.Separator)+"docs") && strings.HasPrefix(rel, "missions"+string(filepath.Separator))
}

// isArtifactVersion is whether a file under .ws (rel) is an artifact's
// version: written once, before the artifact.toml that names it, which
// moves when it's written.
func isArtifactVersion(rel string) bool {
	dir, name := filepath.Split(rel)
	if filepath.Dir(filepath.Clean(dir)) != "artifacts" {
		return false
	}
	_, ok := artifact.IsVersionFile(name)
	return ok
}

// skipFile is a file under .ws no page's view is built from: logs of
// processes and of crew's output, which are written all the time, and the
// daemon's own.
func skipFile(name string) bool {
	switch name {
	case "exec.log", "daemon.log", "web.json", "daemon.sock", "deck.sock", "deck-server.log":
		return true
	case "ws.db", "ws.db-wal", "ws.db-shm", "store.sock", "store.lock":
		// The records say what changed in them themselves (store.Since).
		return true
	}
	return strings.HasPrefix(name, "crew-") && (strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".acp.jsonl"))
}
