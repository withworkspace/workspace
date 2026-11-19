package ops

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Runs on a mission's page (internal/runs): what's running in each of its
// capsules, and its repo's recipes, which "Run it" starts. A run started
// from the page lasts while someone has the mission open, and
// runs.LookLinger after.

// RunUpWait is how long starting a recipe from a page waits for it to be up.
var RunUpWait = 15 * time.Second

func (w *Workspace) runStore() runs.Store { return runs.Store{Root: w.Root()} }

// recipes is the [run.*] of a repo's worktree, by name.
func (w *Workspace) recipes(repo, worktree string) map[string]config.RunRecipe {
	cfg, err := config.ParseRepoConfig(filepath.Join(w.WS().RepoDir(repo), worktree, config.RepoFileName))
	if err != nil || cfg == nil {
		return nil
	}
	return cfg.Run
}

// capsuleRuns is the runs in each of v's capsules, and the recipes each
// can run, by repo/name.
func (w *Workspace) capsuleRuns(v MissionView) (map[string][]runs.Run, map[string][]string) {
	byCapsule, recipes := map[string][]runs.Run{}, map[string][]string{}
	all, _ := w.runStore().List()
	for _, c := range v.Capsules {
		key := c.Repo + "/" + c.Name
		for _, r := range all {
			if r.Repo == c.Repo && r.Worktree == c.Name {
				byCapsule[key] = append(byCapsule[key], r)
			}
		}
		if c.Exists {
			for _, n := range slices.Sorted(maps.Keys(w.recipes(c.Repo, c.Name))) {
				if w.recipes(c.Repo, c.Name)[n].Where != "silo" { // the silo's, not the capsule's
					recipes[key] = append(recipes[key], n)
				}
			}
		}
	}
	return byCapsule, recipes
}

// runCapsule is the mission's capsule repo/name, if it's the mission's and
// is there.
func (w *Workspace) runCapsule(slug, repo, capsule string) (string, error) {
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	for _, c := range m.Capsules {
		if c.Repo == repo && c.Name == capsule {
			dir := filepath.Join(w.WS().RepoDir(repo), capsule)
			if !dirExists(dir) {
				return "", refused("capsule-gone", "%s/%s has been burned: there's nowhere to run it", repo, capsule)
			}
			return dir, nil
		}
	}
	return "", fmt.Errorf("%w: %s/%s isn't one of %s's capsules", ErrNotFound, repo, capsule, slug)
}

// RunRecipe starts recipe in one of a mission's capsules, for whoever's
// looking at the mission: it lasts while the mission's page is open, and
// runs.LookLinger after. It waits up to RunUpWait for it to be up; one
// that isn't by then is still running, and returned.
func (w *Workspace) RunRecipe(slug, repo, capsule, recipe string, by Actor) (runs.Run, error) {
	dir, err := w.runCapsule(slug, repo, capsule)
	if err != nil {
		return runs.Run{}, err
	}
	rc, ok := w.recipes(repo, capsule)[recipe]
	if !ok {
		return runs.Run{}, invalid("%s/%s has no recipe %s ([run.%s] in its %s)", repo, capsule, recipe, recipe, config.RepoFileName)
	}
	worktree := capsule
	if rc.Where == "silo" {
		worktree, dir = workspace.SiloDir, w.WS().SiloWorktree(repo)
		if !dirExists(dir) {
			return runs.Run{}, refused("no-silo", "%s runs in %s's silo, and it has none: point one at a capsule first", recipe, repo)
		}
	}
	store := w.runStore()
	r, err := store.Start(runs.Start{
		Name: recipe, Repo: repo, Worktree: worktree, Dir: dir,
		Command: rc.Command, Recipe: recipe, Ready: rc.Ready, Stop: rc.Stop,
		Owner: runs.Owner{Kind: runs.ByLooking, Mission: slug},
		TTL:   runs.DefaultTTL, By: by.By(),
	}, time.Now())
	if err != nil {
		return runs.Run{}, err
	}
	if _, err := store.Wait(r, RunUpWait); err != nil {
		var ended *runs.EndedError
		if errors.As(err, &ended) {
			return ended.Run, refused("run-ended", "%s. Its last words: %s", err, ended.Run.Tail(3))
		}
		return r, err
	}
	return r, nil
}

// StopRun ends a run in one of a mission's capsules, by its ref.
func (w *Workspace) StopRun(slug, ref string) error {
	r, err := w.runStore().Find(ref, "", "")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	if _, err := w.runCapsule(slug, r.Repo, r.Worktree); err != nil && errors.Is(err, ErrNotFound) {
		return err
	}
	return w.runStore().End(r)
}

// LookedAt marks the runs started from these missions' pages as looked at
// now: someone has each open.
func (w *Workspace) LookedAt(slugs map[string]bool) error {
	if len(slugs) == 0 {
		return nil
	}
	return w.runStore().Looked(slugs, time.Now())
}
