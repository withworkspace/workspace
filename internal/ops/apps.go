package ops

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/runs"
)

// Apps are recipes with a page (page = …): each shows its run in a frame.
// A recipe of the workspace's own (ws.toml) is a place in the web UI's
// sidebar; a repo's (ws.repo.toml) is a tab of each of its worktrees, on
// the repo's page, run there. Nothing runs until the app is opened.
// Opening it starts the run, or joins the one that's running, owned by
// looking at it: the page says it's looked at while it's open, and the
// run is ended runs.LookLinger after nobody has it open. A workspace app's
// needs are pointed from its page (PointRun); a worktree's are the
// nearest to it, as any run's are.

// AppPlace is where an app is: the workspace (no repo), or one of a
// repo's worktrees (a capsule's name, .ground or .silo).
type AppPlace struct {
	Repo, Worktree string
}

// AtWorkspace is the workspace's own place.
var AtWorkspace = AppPlace{}

func (p AppPlace) String() string {
	if p.Repo == "" {
		return "the workspace"
	}
	return p.Repo + "/" + runs.Label(p.Worktree)
}

// AppEntry is an app as a place or a tab has it.
type AppEntry struct {
	Name, Title, Icon string
}

func entryOf(name string, p *config.RunPage) AppEntry {
	return AppEntry{Name: name, Title: firstNonEmpty(strings.TrimSpace(p.Title), name), Icon: strings.TrimSpace(p.Icon)}
}

// Apps are the workspace's apps, by name.
func (w *Workspace) Apps() []AppEntry { return w.AppsAt(AtWorkspace) }

// AppsAt are the apps at p, by name. A repo's recipe that runs in its silo
// (where = "silo") isn't a worktree's.
func (w *Workspace) AppsAt(p AppPlace) []AppEntry {
	recipes := w.recipes(p.Repo, p.Worktree)
	var out []AppEntry
	for _, name := range slices.Sorted(maps.Keys(recipes)) {
		rc := recipes[name]
		if rc.Page != nil && (p.Repo == "" || rc.Where != "silo") {
			out = append(out, entryOf(name, rc.Page))
		}
	}
	return out
}

// AppView is an app's page: what it is, what its needs are pointed at (a
// workspace app's), and its run, as it stands.
type AppView struct {
	AppEntry
	Place    AppPlace
	Pointers []Pointer
	// Run is the recipe's run: the one that's running, or with none, the
	// one that ended last, which says why. nil when there's neither.
	Run *runs.Run
	// Ready is whether the run's up: its ready path answers, or with none,
	// something listens on its port.
	Ready bool
	// Log is the end of its output, while it isn't ready.
	Log string
}

// appLogLines is how much of an app's output its page is shown while it
// isn't up.
const appLogLines = 30

// App is the workspace's app name, as it stands.
func (w *Workspace) App(name string) (AppView, error) { return w.AppAt(AtWorkspace, name) }

// AppAt is the recipe name at p, as an app stands. The workspace's has to
// have a page to be one; a worktree's recipe is shown as one either way
// (its repo's page starts any of its recipes from its Runs).
func (w *Workspace) AppAt(p AppPlace, name string) (AppView, error) {
	if p.Repo != "" && !dirExists(filepath.Join(w.WS().RepoDir(p.Repo), p.Worktree)) {
		return AppView{}, fmt.Errorf("%w: %s has no %s", ErrNotFound, p.Repo, runs.Label(p.Worktree))
	}
	rc, ok := w.recipes(p.Repo, p.Worktree)[name]
	switch {
	case p.Repo == "" && (!ok || rc.Page == nil):
		return AppView{}, fmt.Errorf("%w: the workspace has no app %s ([run.%s] with a page, in %s)", ErrNotFound, name, name, config.FileName)
	case !ok:
		return AppView{}, fmt.Errorf("%w: %s has no recipe %s ([run.%s] in its %s)", ErrNotFound, p, name, name, config.RepoFileName)
	}
	v := AppView{AppEntry: AppEntry{Name: name, Title: name}, Place: p}
	if rc.Page != nil {
		v.AppEntry = entryOf(name, rc.Page)
	}
	if p.Repo == "" {
		v.Pointers, _ = w.Pointers(name)
	}
	all, err := w.runStore().List()
	if err != nil {
		return v, err
	}
	for _, r := range all {
		if r.Repo != p.Repo || r.Worktree != p.Worktree || r.Recipe != name {
			continue
		}
		switch {
		case v.Run == nil, r.State == runs.Running && v.Run.State != runs.Running, r.State != runs.Running && v.Run.State != runs.Running && r.Ended.After(v.Run.Ended):
			r := r
			v.Run = &r
		}
	}
	if v.Run != nil {
		v.Ready = v.Run.State == runs.Running && v.Run.Up() && w.reachable(*v.Run)
		if !v.Ready {
			v.Log = v.Run.Tail(appLogLines)
		}
	}
	return v, nil
}

// reachable is whether r is reached where the web UI will send you for it:
// at 127.0.0.1 and its port, or by its name once the router has its route.
func (w *Workspace) reachable(r runs.Run) bool {
	if router.RunAddress(w.Root(), r) == r.Link() {
		return true
	}
	return router.Published(w.Root(), router.RunKey(r.Name, r.Worktree, r.Repo), r.Port)
}

// OpenApp is the workspace's app name opened (OpenAppAt).
func (w *Workspace) OpenApp(name string, by Actor) (AppView, error) {
	return w.OpenAppAt(AtWorkspace, name, by)
}

// OpenAppAt is the app name at p opened, by whoever's looking: its run
// started with what it needs, or the one that's running joined, and either
// marked as looked at now. It doesn't wait for the run to be up: the page
// says how it's coming on.
func (w *Workspace) OpenAppAt(p AppPlace, name string, by Actor) (AppView, error) {
	if _, err := w.AppAt(p, name); err != nil {
		return AppView{}, err
	}
	ask := RunAsk{Repo: p.Repo, Worktree: p.Worktree, Recipe: name, Owner: runs.Owner{Kind: runs.ByLooking, App: name}, By: by.By()}
	if p.Repo != "" {
		ask.Dir = filepath.Join(w.WS().RepoDir(p.Repo), p.Worktree)
	}
	up, err := w.StartRun(ask)
	if err != nil {
		return AppView{}, err
	}
	w.runStore().LookedRun(up.Run, time.Now())
	return w.AppAt(p, name)
}

// LookedAtApps marks the runs of the apps open in a page as looked at now.
func (w *Workspace) LookedAtApps(names map[string]bool) error {
	if len(names) == 0 {
		return nil
	}
	return w.runStore().LookedApps(names, time.Now())
}

// LookedAtPlace marks the runs started from p's page (owned by looking at one
// of its apps or recipes) as looked at now: someone has it open.
func (w *Workspace) LookedAtPlace(p AppPlace) error {
	return w.runStore().LookedAt(p.Repo, p.Worktree, time.Now())
}

// RestartApp starts the workspace's app name again (RestartAppAt).
func (w *Workspace) RestartApp(name string, by Actor) (AppView, error) {
	return w.RestartAppAt(AtWorkspace, name, by)
}

// RestartAppAt starts the app's run again, for everyone it was for: what
// it's pointed at is read again, and so is its recipe.
func (w *Workspace) RestartAppAt(p AppPlace, name string, by Actor) (AppView, error) {
	if _, err := w.AppAt(p, name); err != nil {
		return AppView{}, err
	}
	old, ok := w.runningRecipe(p.Repo, p.Worktree, name)
	if !ok {
		return w.OpenAppAt(p, name, by)
	}
	if _, err := w.restartRun(old, 0); err != nil {
		return AppView{}, err
	}
	return w.AppAt(p, name)
}

// StopApp ends the workspace's app name's run (StopAppAt).
func (w *Workspace) StopApp(name string) error { return w.StopAppAt(AtWorkspace, name) }

// StopAppAt ends the app's run, and what was started for it that nothing
// else needs.
func (w *Workspace) StopAppAt(p AppPlace, name string) error {
	if _, err := w.AppAt(p, name); err != nil {
		return err
	}
	old, ok := w.runningRecipe(p.Repo, p.Worktree, name)
	if !ok {
		return nil
	}
	return w.endRun(old)
}

// RecipeAt is one of a worktree's recipes, as its repo's page lists them.
type RecipeAt struct {
	Name, Command string
	// App is its page's, for a recipe that has one.
	App *AppEntry
}

// PlaceRuns is a worktree's recipes and runs, as its repo's page has them:
// its apps (the recipes with a page) as tabs, and in Runs, every recipe it
// has and every run in it.
type PlaceRuns struct {
	Apps    []AppEntry
	Recipes []RecipeAt
	Runs    []runs.Run
}

// RunsAt is p's recipes and runs.
func (w *Workspace) RunsAt(p AppPlace) (PlaceRuns, error) {
	if p.Repo != "" && !dirExists(filepath.Join(w.WS().RepoDir(p.Repo), p.Worktree)) {
		return PlaceRuns{}, fmt.Errorf("%w: %s has no %s", ErrNotFound, p.Repo, runs.Label(p.Worktree))
	}
	out := PlaceRuns{Apps: w.AppsAt(p), Recipes: []RecipeAt{}, Runs: []runs.Run{}}
	recipes := w.recipes(p.Repo, p.Worktree)
	for _, name := range slices.Sorted(maps.Keys(recipes)) {
		rc := recipes[name]
		if p.Repo != "" && rc.Where == "silo" {
			continue // the silo's, not this worktree's
		}
		r := RecipeAt{Name: name, Command: rc.Command}
		if rc.Page != nil {
			e := entryOf(name, rc.Page)
			r.App = &e
		}
		out.Recipes = append(out.Recipes, r)
	}
	all, err := w.runStore().List()
	if err != nil {
		return out, err
	}
	for _, r := range all {
		if r.Repo == p.Repo && r.Worktree == p.Worktree {
			out.Runs = append(out.Runs, r)
		}
	}
	return out, nil
}
