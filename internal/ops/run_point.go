package ops

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Pointing a workspace recipe's needs. A recipe of ws.toml runs once for
// the whole workspace, so what meets its needs isn't where it's asked
// from: each need is pointed at one of its repo's worktrees (a capsule,
// the ground or the silo), and is met by the nearest (the ground) until
// it is. A pointer outlasts the run. Pointing a need of a recipe that's
// running starts it again on what it's pointed at now, for whoever it
// was for.

// Pointer is one of a workspace recipe's needs, and what it can be
// pointed at.
type Pointer struct {
	// Need is the need's name, and Repo the repo whose worktrees meet it.
	Need, Repo string
	// Folder is whether the need is the repo's folder, not one of its
	// recipes' runs (Recipe).
	Folder bool
	Recipe string
	// At is the worktree it's pointed at: "" for the nearest, its ground.
	At string
	// Choices are the worktrees it can be pointed at: .ground, .silo where
	// there is one, then the repo's capsules, by name.
	Choices []string
}

// Pointers are recipe's needs that can be pointed: those met by a repo's
// worktree, in the order of their names.
func (w *Workspace) Pointers(recipe string) ([]Pointer, error) {
	rc, ok := w.recipes("", "")[recipe]
	if !ok {
		return nil, fmt.Errorf("%w: no recipe %s (%s in %s)", ErrNotFound, recipe, tablesOf(recipe), config.FileName)
	}
	points := w.runStore().Points(recipe)
	var out []Pointer
	for _, name := range slices.Sorted(maps.Keys(rc.Needs)) {
		n := rc.Needs[name]
		p := Pointer{Need: name, Folder: n.IsFolder()}
		if n.IsFolder() {
			p.Repo = strings.TrimSpace(n.Repo)
			if r, ok := w.WS().ResolveAlias(p.Repo); ok {
				p.Repo = r
			}
		} else {
			p.Repo, p.Recipe = n.Recipe(name)
			if p.Repo == "" {
				continue // the workspace's own recipe: nothing to point
			}
		}
		p.At = points[name]
		p.Choices = w.worktreesOf(p.Repo, p.Recipe)
		out = append(out, p)
	}
	return out, nil
}

// worktreesOf is repo's worktrees that can meet a need: its ground, its
// silo where it has one, then its capsules, by name. With a recipe, only
// those that have it.
func (w *Workspace) worktreesOf(repo, recipe string) []string {
	var out []string
	has := func(wt string) bool {
		if !dirExists(filepath.Join(w.WS().RepoDir(repo), wt)) {
			return false
		}
		if recipe == "" {
			return true
		}
		_, ok := w.recipes(repo, wt)[recipe]
		return ok
	}
	for _, wt := range []string{workspace.GroundDir, workspace.SiloDir} {
		if has(wt) {
			out = append(out, wt)
		}
	}
	// A capsule is a folder of the repo's with a worktree's .git file in it;
	// the ground, the silo and the bare clone are the dot folders.
	entries, _ := os.ReadDir(w.WS().RepoDir(repo))
	for _, e := range entries {
		c := e.Name()
		if !e.IsDir() || strings.HasPrefix(c, ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(w.WS().RepoDir(repo), c, ".git")); err == nil && has(c) {
			out = append(out, c)
		}
	}
	return out
}

// Pointed is what pointing a need did.
type Pointed struct {
	Pointer Pointer
	// Restarted is the run started again on what it's pointed at now,
	// when it was running: nil when it wasn't.
	Restarted *RunUp
}

func (p Pointed) String() string {
	at := runs.Label(p.Pointer.At)
	if p.Pointer.At == "" {
		at = "the nearest (ground)"
	}
	s := fmt.Sprintf("%s is pointed at %s.", p.Pointer.Need, at)
	if p.Restarted != nil {
		s += fmt.Sprintf(" %s was started again on it.", p.Restarted.Run.Ref())
	}
	return s
}

// PointRun points need of the workspace's recipe at worktree (a capsule's
// name, ground or silo; "" for the nearest again), and starts the recipe
// again where it's running, so it's on it. It's yours to do: crew, the
// orchestrator and a conversation's agent are refused, as moving it
// changes what you're looking at.
func (w *Workspace) PointRun(recipe, need, worktree string, by Actor, wait time.Duration) (Pointed, error) {
	if !by.IsYou() {
		return Pointed{}, refused("not-yours", "pointing %s is yours to do: it changes what's shown to whoever has it open", recipe)
	}
	ptrs, err := w.Pointers(recipe)
	if err != nil {
		return Pointed{}, err
	}
	i := slices.IndexFunc(ptrs, func(p Pointer) bool { return p.Need == need })
	if i < 0 {
		var names []string
		for _, p := range ptrs {
			names = append(names, p.Need)
		}
		if len(names) == 0 {
			return Pointed{}, invalid("%s has no needs that can be pointed", recipe)
		}
		return Pointed{}, invalid("%s has no need %s that can be pointed (has: %s)", recipe, need, strings.Join(names, ", "))
	}
	p := ptrs[i]
	switch worktree = strings.TrimSpace(worktree); worktree {
	case "ground":
		worktree = workspace.GroundDir
	case "silo":
		worktree = workspace.SiloDir
	case "nearest":
		worktree = ""
	}
	if worktree != "" && !slices.Contains(p.Choices, worktree) {
		if !dirExists(filepath.Join(w.WS().RepoDir(p.Repo), worktree)) {
			return Pointed{}, invalid("%s has no %s", p.Repo, runs.Label(worktree))
		}
		return Pointed{}, invalid("%s/%s has no recipe %s (%s in its %s)", p.Repo, runs.Label(worktree), p.Recipe, tablesOf(p.Recipe), config.RepoFileName)
	}
	if err := w.runStore().Point(recipe, need, worktree); err != nil {
		return Pointed{}, err
	}
	p.At = worktree
	out := Pointed{Pointer: p}
	old, ok := w.runningRecipe("", "", recipe)
	if !ok {
		return out, nil
	}
	up, err := w.restartRun(old, wait)
	if err != nil {
		return out, fmt.Errorf("%s is pointed at %s, but could not restart %s: %w", need, runs.Label(worktree), old.Ref(), err)
	}
	out.Restarted = &up
	return out, nil
}

// restartRun ends a running recipe and starts it again, for everyone it
// was for and with the time it had left.
func (w *Workspace) restartRun(old runs.Run, wait time.Duration) (RunUp, error) {
	if err := w.endRun(old); err != nil {
		return RunUp{}, err
	}
	ttl := time.Duration(0)
	if !old.Expires.IsZero() {
		ttl = max(time.Until(old.Expires), time.Minute)
	}
	up, err := w.StartRun(RunAsk{
		Repo: old.Repo, Worktree: old.Worktree, Dir: old.Dir, Recipe: old.Recipe, Name: old.Name,
		Owner: old.Owner, TTL: ttl, By: old.By, Wait: wait,
	})
	if err != nil {
		return up, err
	}
	store := w.runStore()
	r := up.Run
	for _, o := range old.Also {
		if r, err = store.Join(r, o, ttl, time.Now()); err != nil {
			return up, err
		}
	}
	if !old.Looked.IsZero() {
		logFailed(store.LookedRun(r, old.Looked))
	}
	up.Run = r
	if r.IsTask() {
		return up, nil // done when it ends, which nothing here waits for
	}
	var ended *runs.EndedError
	if _, err := store.Wait(r, wait); errors.As(err, &ended) {
		return up, err
	}
	return up, nil
}
