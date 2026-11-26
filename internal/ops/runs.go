package ops

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/router"
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

// recipes is the services and tasks ([run.service.*], [run.task.*]) of a
// repo's worktree, by name; with no repo, the workspace's own, in ws.toml.
func (w *Workspace) recipes(repo, worktree string) map[string]config.RunRecipe {
	return w.runTables(repo, worktree).Recipes()
}

// runTables is the [run] of a repo's worktree as its ws.repo.toml has it,
// or with no repo, of the workspace's ws.toml.
func (w *Workspace) runTables(repo, worktree string) config.Runs {
	if repo == "" {
		return w.Config().Run
	}
	cfg, err := config.ParseRepoConfig(filepath.Join(w.WS().RepoDir(repo), worktree, config.RepoFileName))
	if err != nil || cfg == nil {
		return config.Runs{}
	}
	return cfg.Run
}

// tablesOf is where a recipe called name would be written, for an error
// that there's none: [run.service.web] or [run.task.web].
func tablesOf(name string) string {
	return "[run.service." + name + "] or [run.task." + name + "]"
}

// capsuleRuns is the runs in each of v's capsules, the recipes each can
// run, and those of them that are tasks, by repo/name.
func (w *Workspace) capsuleRuns(v MissionView) (map[string][]runs.Run, map[string][]string, map[string][]string) {
	byCapsule, recipes, tasks := map[string][]runs.Run{}, map[string][]string{}, map[string][]string{}
	all, _ := w.runStore().List()
	for _, c := range v.Capsules {
		key := c.Repo + "/" + c.Name
		for _, r := range all {
			if r.Repo == c.Repo && r.Worktree == c.Name {
				byCapsule[key] = append(byCapsule[key], r)
			}
		}
		if c.Exists {
			all := w.recipes(c.Repo, c.Name)
			for _, n := range slices.Sorted(maps.Keys(all)) {
				if all[n].Where != "silo" { // the silo's, not the capsule's
					recipes[key] = append(recipes[key], n)
					if all[n].IsTask() {
						tasks[key] = append(tasks[key], n)
					}
				}
			}
		}
	}
	return byCapsule, recipes, tasks
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
//
// A task isn't waited for, and isn't the lookers': it's the mission's,
// and runs to its end whether or not anyone stays to watch.
func (w *Workspace) RunRecipe(slug, repo, capsule, recipe string, by Actor) (runs.Run, error) {
	dir, err := w.runCapsule(slug, repo, capsule)
	if err != nil {
		return runs.Run{}, err
	}
	owner := runs.Owner{Kind: runs.ByLooking, Mission: slug}
	if w.recipes(repo, capsule)[recipe].IsTask() {
		owner = runs.Owner{Kind: runs.ByMission, Mission: slug}
	}
	return w.runUp(RunAsk{
		Repo: repo, Worktree: capsule, Dir: dir, Recipe: recipe,
		Owner: owner,
		TTL:   runs.DefaultTTL, By: by.By(), Mission: slug, Wait: RunUpWait,
	})
}

// runUp starts what a page asks for, and waits up to RunUpWait for it to
// be up: one that isn't by then is still running, and returned. One that
// ended as it started is refused, with its last words.
func (w *Workspace) runUp(a RunAsk) (runs.Run, error) {
	up, err := w.StartRun(a)
	var ended *runs.EndedError
	if errors.As(err, &ended) {
		return ended.Run, refused("run-ended", "%s. Its last words: %s", err, ended.Run.Tail(3))
	}
	if err != nil {
		return runs.Run{}, err
	}
	r := up.Run
	if r.IsTask() {
		return r, nil
	}
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

// LookedAt marks the runs started from the pages open in l (missions'
// pages, and grounds and capsules on the Files page) as looked at now.
func (w *Workspace) LookedAt(l runs.Looking) error {
	if len(l.Missions) == 0 && len(l.Worktrees) == 0 {
		return nil
	}
	return w.runStore().Looked(l, time.Now())
}

// Runs on the Files page: what's running in a repo's ground or capsule,
// and its recipes, which "Run" starts there for whoever has it open. A run
// started there lasts while someone has that worktree open on the page,
// and runs.LookLinger after.

// WorktreeRuns are the runs in a repo's ground or capsule, and its
// recipes that run there (a silo's run in the silo, not here), by name.
// Past are the earlier runs of its tasks, kept behind each one's latest
// (in Runs), the latest first: what's been run there lately, with the
// runs that are kept.
type WorktreeRuns struct {
	Runs    []runs.Run
	Recipes []Recipe
	Past    []runs.Run
}

// Recipe is one of a place's recipes, as a page lists it: its name, what
// it's called (its display_name, or its page's title: "" for its name)
// and its command; whether it's a task; a service's Kind (RecipeApp or
// RecipeTool) and Render (RenderInline or RenderTab), "" for a task's;
// and whether it has a page (an app's tab on the worktree).
type Recipe struct {
	Name, Called, Command string
	Task                  bool
	Kind, Render          string
	Page                  bool
}

// What a service is for, and how a page shows it, as Recipe says them:
// its file's words.
const (
	RecipeApp    = config.RunApp
	RecipeTool   = config.RunTool
	RenderInline = config.RenderInline
	RenderTab    = config.RenderTab
)

// recipeOf is name's recipe as a page lists it.
func recipeOf(name string, rc config.RunRecipe) Recipe {
	out := Recipe{Name: name, Called: rc.Called(), Command: rc.Command, Task: rc.IsTask(), Page: rc.Page != nil}
	if !out.Task {
		out.Kind, out.Render = RecipeApp, RenderInline
		if rc.IsTool() {
			out.Kind = RecipeTool
		}
		if rc.InTab() {
			out.Render = RenderTab
		}
	}
	return out
}

// worktreeOf is the folder of repo's ground (workspace.GroundDir) or
// capsule name, if the workspace has it.
func (w *Workspace) worktreeOf(repo, name string) (string, error) {
	ws := w.WS()
	if !slices.Contains(ws.RepoNames, repo) {
		return "", fmt.Errorf("%w: the workspace has no repo %s", ErrNotFound, repo)
	}
	if name != workspace.GroundDir {
		capsules, _ := workspace.ListWorktrees(ws.RepoDir(repo))
		if !slices.Contains(capsules, name) {
			return "", fmt.Errorf("%w: %s has no capsule %s", ErrNotFound, repo, name)
		}
	}
	dir := filepath.Join(ws.RepoDir(repo), name)
	if !dirExists(dir) {
		return "", fmt.Errorf("%w: %s/%s isn't there", ErrNotFound, repo, runs.Label(name))
	}
	return dir, nil
}

// RunsIn is what's running in repo's ground or capsule name, and the
// recipes it can run.
func (w *Workspace) RunsIn(repo, name string) (WorktreeRuns, error) {
	if _, err := w.worktreeOf(repo, name); err != nil {
		return WorktreeRuns{}, err
	}
	out := WorktreeRuns{Runs: []runs.Run{}, Recipes: []Recipe{}, Past: []runs.Run{}}
	store := w.runStore()
	all, _ := store.List()
	for _, r := range all {
		if r.Repo == repo && r.Worktree == name {
			out.Runs = append(out.Runs, r)
			if r.IsTask() && r.Past > 0 {
				out.Past = append(out.Past, store.Past(r)...)
			}
		}
	}
	recipes := w.recipes(repo, name)
	for _, n := range slices.Sorted(maps.Keys(recipes)) {
		if recipes[n].Where != "silo" {
			out.Recipes = append(out.Recipes, recipeOf(n, recipes[n]))
		}
	}
	return out, nil
}

// RunIn starts recipe in repo's ground or capsule name, for whoever has
// it open on the Files page: it lasts while someone does, and
// runs.LookLinger after (or runs.DefaultTTL at most). Otherwise it's as
// RunRecipe: what it needs is started first, one running already is the
// lookers' as well, and it waits up to RunUpWait for it to be up.
//
// A task isn't the lookers': it runs to its end whether or not the page
// stays open (or is stopped at its timeout), as one run from a mission's
// page does.
func (w *Workspace) RunIn(repo, name, recipe string, by Actor) (runs.Run, error) {
	dir, err := w.worktreeOf(repo, name)
	if err != nil {
		return runs.Run{}, err
	}
	owner := runs.Owner{Kind: runs.ByLooking}
	if w.recipes(repo, name)[recipe].IsTask() {
		owner = runs.Owner{Kind: runs.ByNone}
	}
	return w.runUp(RunAsk{
		Repo: repo, Worktree: name, Dir: dir, Recipe: recipe,
		Owner: owner,
		TTL:   runs.DefaultTTL, By: by.By(), Wait: RunUpWait,
	})
}

// Runs is every run in the workspace, wherever it runs and whoever it's
// for: the Runs page's.
func (w *Workspace) Runs() ([]runs.Run, error) { return w.runStore().List() }

// runByRef is the run whose ref is ref, said in full
// (<name>.<worktree>.<repo>, with #n after it for an earlier run of a
// task): a page names a run it's shown, so a name alone isn't guessed at.
func (w *Workspace) runByRef(ref string) (runs.Run, error) {
	all, err := w.runStore().List()
	if err != nil {
		return runs.Run{}, err
	}
	at, n, numbered := strings.Cut(ref, "#")
	for _, r := range all {
		if r.Ref() != at {
			continue
		}
		if !numbered || strconv.Itoa(r.N) == n {
			return r, nil
		}
		for _, p := range w.runStore().Past(r) {
			if strconv.Itoa(p.N) == n {
				return p, nil
			}
		}
	}
	return runs.Run{}, fmt.Errorf("%w: no run %s: it may have ended already", ErrNotFound, ref)
}

// Run is one run by its ref, and, for a task, the earlier runs kept
// behind it, the latest first.
func (w *Workspace) Run(ref string) (runs.Run, []runs.Run, error) {
	r, err := w.runByRef(ref)
	if err != nil {
		return runs.Run{}, nil, err
	}
	if !r.IsTask() || strings.Contains(ref, "#") {
		return r, nil, nil
	}
	return r, w.runStore().Past(r), nil
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
	text, _, _, err := w.RunLogSized(ref, n)
	return text, err
}

// RunLogSized is RunLog, with how much of the log it's as of (its size as
// it was read) and the log's path: what's written after it is read from
// there (runs.ReadFrom).
func (w *Workspace) RunLogSized(ref string, n int) (text string, size int64, path string, err error) {
	r, err := w.runByRef(ref)
	if err != nil {
		return "", 0, "", err
	}
	text, size = r.TailSized(n)
	return text, size, r.LogPath(), nil
}

// RunsStamp names where the workspace's runs stand, cheaply
// (runs.Store.Stamp): what the Runs page shows moves only when it does.
// In the daemon it's what's kept (WatchRuns).
func (w *Workspace) RunsStamp() string {
	if s, ok := w.runsKept.Stamp(); ok {
		return s
	}
	return w.runStore().Stamp()
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
