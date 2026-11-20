package ops

import (
	"context"
	"fmt"
	"hash/fnv"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/artifact"
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
	// a checkout), as its repo's bare clone keeps them.
	for _, repo := range w.WS().RepoNames {
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
	fmt.Fprintf(h, "prs %d\n", w.opts.PRs.Fetched())
	// A capsule found dirty, or clean, since a view last said: nothing git
	// keeps moves when a file's edited (keptGit).
	fmt.Fprint(h, w.git.dirtyMoved())
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
func (w *Workspace) MissionStamp(slug string) string {
	h := fnv.New64a()
	note := func(path string) {
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(h, "%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
		}
	}
	dir := filepath.Join(w.Missions.Dir(), slug)
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
	if m, err := w.Missions.Load(slug); err == nil {
		for _, c := range m.Capsules {
			trees := filepath.Join(w.WS().BareDir(c.Repo), "worktrees", c.Name)
			for _, f := range []string{"HEAD", "index", filepath.Join("logs", "HEAD")} {
				note(filepath.Join(trees, f))
			}
			dir := filepath.Join(w.WS().RepoDir(c.Repo), c.Name)
			note(dir) // made, or burned
			// Its runs: one started or ended, and one that's exited. Not
			// its records, which are written as it's looked at.
			runsDir := filepath.Join(w.runStore().Dir(), c.Repo, c.Name)
			note(runsDir)
			if ents, err := os.ReadDir(runsDir); err == nil {
				for _, e := range ents {
					note(filepath.Join(runsDir, e.Name(), "exit"))
				}
			}
			// Found dirty, or clean, since the page was last told (keptGit).
			fmt.Fprint(h, w.git.dirtyMoved(dir))
		}
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// skipDir is what under .ws no page's view is built from, or is kept by a
// page of its own (Team's, Reviews'): its own change checks follow it.
func skipDir(rel string) bool {
	switch rel {
	case "team", "prs", "bin", "hooks", "converse":
		return true
	case "backup", "swept": // copies of the records; what ws doctor swept aside
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
