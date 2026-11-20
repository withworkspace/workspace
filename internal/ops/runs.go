package ops

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/runs"
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
// runs.LookLinger after. What it needs is started first (StartRun), and
// one that's running already is the lookers' as well. It waits up to
// RunUpWait for it to be up; one that isn't by then is still running, and
// returned.
func (w *Workspace) RunRecipe(slug, repo, capsule, recipe string, by Actor) (runs.Run, error) {
	dir, err := w.runCapsule(slug, repo, capsule)
	if err != nil {
		return runs.Run{}, err
	}
	up, err := w.StartRun(RunAsk{
		Repo: repo, Worktree: capsule, Dir: dir, Recipe: recipe,
		Owner: runs.Owner{Kind: runs.ByLooking, Mission: slug},
		TTL:   runs.DefaultTTL, By: by.By(), Mission: slug, Wait: RunUpWait,
	})
	var ended *runs.EndedError
	if errors.As(err, &ended) {
		return ended.Run, refused("run-ended", "%s. Its last words: %s", err, ended.Run.Tail(3))
	}
	if err != nil {
		return runs.Run{}, err
	}
	r := up.Run
	if _, err := w.runStore().Wait(r, RunUpWait); err != nil {
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
	return w.endRun(r)
}

// endRun ends r, and then whatever ws had started for it that nothing
// needs now, without waiting for the daemon's next look.
func (w *Workspace) endRun(r runs.Run) error {
	err := w.runStore().End(r)
	if len(r.Needs) > 0 {
		w.runStore().Sweep(nil, time.Now())
	}
	return err
}

// LookedAt marks the runs started from these missions' pages as looked at
// now: someone has each open.
func (w *Workspace) LookedAt(slugs map[string]bool) error {
	if len(slugs) == 0 {
		return nil
	}
	return w.runStore().Looked(slugs, time.Now())
}

// Runs is every run in the workspace, wherever it runs and whoever it's
// for: the Runs page's.
func (w *Workspace) Runs() ([]runs.Run, error) { return w.runStore().List() }

// runByRef is the run whose ref is ref, said in full
// (<name>.<worktree>.<repo>): a page names a run it's shown, so a name
// alone isn't guessed at.
func (w *Workspace) runByRef(ref string) (runs.Run, error) {
	all, err := w.runStore().List()
	if err != nil {
		return runs.Run{}, err
	}
	for _, r := range all {
		if r.Ref() == ref {
			return r, nil
		}
	}
	return runs.Run{}, fmt.Errorf("%w: no run %s: it may have ended already", ErrNotFound, ref)
}

// EndRun ends any run, by its ref, as ws run stop does: the Runs page's
// Stop.
func (w *Workspace) EndRun(ref string) error {
	r, err := w.runByRef(ref)
	if err != nil {
		return err
	}
	return w.endRun(r)
}

// RunLog is the last n lines of a run's output.
func (w *Workspace) RunLog(ref string, n int) (string, error) {
	r, err := w.runByRef(ref)
	if err != nil {
		return "", err
	}
	return r.Tail(n), nil
}

// Names is whether ws.local.toml's [router] gives the workspace names
// under .localhost, and the name it asks for (its folder's, unset), as the
// file says now: the daemon reads it again every few seconds, so a change
// on the Settings page is soon what's served.
func (w *Workspace) Names() (on bool, want string, err error) {
	local, err := config.ReadLocal(w.Root())
	if err != nil {
		return false, "", err
	}
	return local.Router.Enabled, router.NameFor(w.Root(), local.Router.Name), nil
}
