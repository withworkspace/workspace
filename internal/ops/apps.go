package ops

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/runs"
)

// Apps are the workspace's own recipes with a page ([run.docs] page = …):
// each a place in the web UI that shows its run in a frame. Nothing runs
// until the page is opened. Opening it starts the run, or joins the one
// that's running, owned by looking at it: the page says it's looked at
// while it's open, and the run is ended runs.LookLinger after nobody has
// it open. Its needs are pointed from the page (PointRun).

// AppEntry is an app as the sidebar has it.
type AppEntry struct {
	Name, Title, Icon string
}

// Apps are the workspace's apps, by name.
func (w *Workspace) Apps() []AppEntry {
	recipes := w.recipes("", "")
	var out []AppEntry
	for _, name := range slices.Sorted(maps.Keys(recipes)) {
		if p := recipes[name].Page; p != nil {
			out = append(out, AppEntry{Name: name, Title: firstNonEmpty(strings.TrimSpace(p.Title), name), Icon: strings.TrimSpace(p.Icon)})
		}
	}
	return out
}

// AppView is an app's page: what it is, what its needs are pointed at, and
// its run, as it stands.
type AppView struct {
	AppEntry
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

// App is the app name, as it stands.
func (w *Workspace) App(name string) (AppView, error) {
	rc, ok := w.recipes("", "")[name]
	if !ok || rc.Page == nil {
		return AppView{}, fmt.Errorf("%w: the workspace has no app %s ([run.%s] with a page, in %s)", ErrNotFound, name, name, config.FileName)
	}
	v := AppView{AppEntry: AppEntry{Name: name, Title: firstNonEmpty(strings.TrimSpace(rc.Page.Title), name), Icon: strings.TrimSpace(rc.Page.Icon)}}
	v.Pointers, _ = w.Pointers(name)
	all, err := w.runStore().List()
	if err != nil {
		return v, err
	}
	for _, r := range all {
		if !r.OfWorkspace() || r.Recipe != name {
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

// OpenApp is the app name opened, by whoever's looking: its run started
// with what it needs, or the one that's running joined, and either marked
// as looked at now. It doesn't wait for the run to be up: the page says
// how it's coming on.
func (w *Workspace) OpenApp(name string, by Actor) (AppView, error) {
	if _, err := w.App(name); err != nil {
		return AppView{}, err
	}
	up, err := w.StartRun(RunAsk{
		Recipe: name, Owner: runs.Owner{Kind: runs.ByLooking, App: name}, By: by.By(),
	})
	if err != nil {
		return AppView{}, err
	}
	w.runStore().LookedRun(up.Run, time.Now())
	return w.App(name)
}

// LookedAtApps marks the runs of the apps open in a page as looked at now.
func (w *Workspace) LookedAtApps(names map[string]bool) error {
	if len(names) == 0 {
		return nil
	}
	return w.runStore().LookedApps(names, time.Now())
}

// RestartApp starts the app's run again, for everyone it was for: what
// it's pointed at is read again, and so is its recipe.
func (w *Workspace) RestartApp(name string, by Actor) (AppView, error) {
	if _, err := w.App(name); err != nil {
		return AppView{}, err
	}
	old, ok := w.runningRecipe("", "", name)
	if !ok {
		return w.OpenApp(name, by)
	}
	if _, err := w.restartRun(old, 0); err != nil {
		return AppView{}, err
	}
	return w.App(name)
}

// StopApp ends the app's run, and what was started for it that nothing
// else needs.
func (w *Workspace) StopApp(name string) error {
	if _, err := w.App(name); err != nil {
		return err
	}
	old, ok := w.runningRecipe("", "", name)
	if !ok {
		return nil
	}
	return w.endRun(old)
}
