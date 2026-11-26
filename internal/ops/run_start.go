package ops

import (
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Starting a run with what it needs. A recipe says the runs it needs up
// before it ([run.service.web.needs.api]): ws finds each where it's nearest,
// starts the ones that aren't running, waits for them, and hands their
// addresses to the run. A recipe runs once in a worktree, so two runs
// that need the same thing share one run of it, which is kept while
// either does (runs.Store.Sweep counts them).

// RunAsk is a run asked for: a recipe of a worktree, or a command that's
// no recipe's, and who it's for.
type RunAsk struct {
	// Repo, Worktree and Dir are where it's asked for: a recipe that runs
	// in the repo's silo (where = "silo") runs there instead.
	Repo, Worktree, Dir string
	// Recipe names one of the worktree's recipes; Command is run as it's
	// said, when there's no recipe.
	Recipe, Command string
	// Type is which sort of run a command's is: runs.Task for one that
	// ends by itself and is kept for what it came to. A recipe says its
	// own.
	Type runs.Type
	// Check is the check of a mission's step it's run for, if it is.
	Check *runs.CheckOf
	// Name is what the run is called: "" is its recipe's name, or its
	// command's first word.
	Name  string
	Owner runs.Owner
	// TTL is how long it may last: 0 for no limit. A task's time is its
	// Timeout instead.
	TTL time.Duration
	// Timeout is how long a task may take before it's stopped: 0 is its
	// recipe's timeout, or runs.TaskTimeout; below 0, it has no limit.
	Timeout time.Duration
	By      string
	// With says what meets one of the recipe's needs, by the need's name,
	// in place of the nearest: a capsule's name, ground, silo, or an
	// address (https://…), behind which ws starts nothing.
	With map[string]string
	// Mission is the mission it's asked within, whose capsules are the
	// nearest in other repos: "" is the mission whose capsule Worktree is.
	Mission string
	// Wait is how long each run it needs is given to be up before it's
	// started: 0 doesn't wait.
	Wait time.Duration
	// Env is the environment it runs in: nil is this process's.
	Env []string

	// hold is until when it isn't ended for nothing needing it: it's
	// being started for a run that isn't recorded yet.
	hold time.Time
}

// RunUp is a run asked for, as it now stands.
type RunUp struct {
	Run runs.Run
	// Started is false when a run of the recipe was there already: it's
	// then the asker's as well.
	Started bool
	// Needs are what it needs, each as it was met.
	Needs []NeedUp
	// Notes are what's worth saying of how it went, each a sentence.
	Notes []string
}

// NeedUp is one of a run's needs, as it was met.
type NeedUp struct {
	runs.Need
	// Started is whether ws started its run now, for this.
	Started bool
	// Where says how its run was chosen, when that's worth saying
	// ("backend's ground").
	Where string
}

// needWait is how long past its wait a run started for a need is kept
// before the run that needs it has been recorded.
const needWait = 30 * time.Second

var needName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// StartRun starts what's asked for, having seen to what it needs: each
// need's run found or started first, and waited for. A recipe that's
// running in its worktree already isn't started again: that run is
// returned, the asker one more of who it's for, and what it needs is
// started again where it has ended.
func (w *Workspace) StartRun(a RunAsk) (RunUp, error) {
	if a.Recipe == "" && len(a.With) > 0 {
		return RunUp{}, invalid("--with says what meets a recipe's need: a command has none")
	}
	if a.Mission == "" {
		if m, ok := w.Missions.FindByCapsule(a.Repo, a.Worktree); ok {
			a.Mission = m.Slug
		}
	}
	return w.startRun(a, nil)
}

// startRun is StartRun for a, with the recipes that need it above (each as
// repo/worktree/recipe), so one that needs itself is said and not started
// for ever.
func (w *Workspace) startRun(a RunAsk, above []string) (RunUp, error) {
	store := w.runStore()
	now := time.Now()
	if a.Recipe == "" {
		st := runs.Start{
			Name: a.Name, Type: a.Type, Repo: a.Repo, Worktree: a.Worktree, Dir: a.Dir, Command: a.Command,
			Owner: a.Owner, TTL: a.TTL, By: a.By, Env: a.Env, Check: a.Check,
		}
		if a.Type == runs.Task {
			st.Commit, st.Dirty = standsAt(a.Dir)
			st.TTL, _ = taskTime(a, "")
		}
		r, err := store.Start(st, now)
		return RunUp{Run: r, Started: err == nil}, err
	}
	if a.Repo == "" {
		a.Worktree, a.Dir = "", w.Root() // the workspace's own runs run at its root
	}
	rc, ok := w.recipes(a.Repo, a.Worktree)[a.Recipe]
	if !ok {
		if a.Repo == "" {
			return RunUp{}, invalid("the workspace has no recipe %s (%s in %s)", a.Recipe, tablesOf(a.Recipe), config.FileName)
		}
		return RunUp{}, invalid("%s/%s has no recipe %s (%s in its %s)", a.Repo, runs.Label(a.Worktree), a.Recipe, tablesOf(a.Recipe), config.RepoFileName)
	}
	if slices.Contains(w.runTables(a.Repo, a.Worktree).Twice(), a.Recipe) {
		return RunUp{}, invalid("%s is both a service and a task in %s ([run.service.%s] and [run.task.%s]): a recipe's name is one run's, so one of them wants another", a.Recipe, recipeFile(a.Repo), a.Recipe, a.Recipe)
	}
	if strings.TrimSpace(rc.Command) == "" {
		return RunUp{}, invalid("%s in %s has no command", rc.Table(a.Recipe), recipeFile(a.Repo))
	}
	if rc.Where == "silo" && a.Repo == "" {
		return RunUp{}, invalid("%s in %s: where = \"silo\" is for a repo's recipe, and the workspace's run at its root", rc.Table(a.Recipe), config.FileName)
	}
	typ := runs.Service
	if rc.IsTask() {
		typ = runs.Task
		if len(above) > 0 {
			return RunUp{}, invalid("%s is a task, which ends by itself: what a run needs is a service, kept up for it", a.Recipe)
		}
		var err error
		if a.TTL, err = taskTime(a, rc.Timeout); err != nil {
			return RunUp{}, invalid("%s in %s: %v", rc.Table(a.Recipe), recipeFile(a.Repo), err)
		}
	}
	if rc.Where == "silo" && a.Worktree != workspace.SiloDir {
		a.Worktree, a.Dir = workspace.SiloDir, w.WS().SiloWorktree(a.Repo)
		if !dirExists(a.Dir) {
			return RunUp{}, refused("no-silo", "%s runs in %s's silo, and it has none: ws silo point %s <capsule> makes one", a.Recipe, a.Repo, a.Repo)
		}
	}
	key := a.Repo + "/" + runs.Label(a.Worktree) + "/" + a.Recipe
	if a.Repo == "" {
		key = "workspace/" + a.Recipe
	}
	if slices.Contains(above, key) {
		return RunUp{}, invalid("%s needs itself: %s", key, strings.Join(append(above, key), ", which needs "))
	}
	above = append(slices.Clone(above), key)
	for name := range a.With {
		if _, ok := rc.Needs[name]; !ok {
			return RunUp{}, invalid("%s has no need called %s%s", a.Recipe, name, needsSaid(rc))
		}
	}

	if old, ok := w.runningRecipe(a.Repo, a.Worktree, a.Recipe); ok {
		return w.runAgain(a, old, above)
	}

	// Each need's run is found or started, and kept until this one is
	// recorded as needing it.
	var needs []runs.Need
	var ups []NeedUp
	var held []runs.Run
	var notes []string
	hold := func() time.Time { return time.Now().Add(a.Wait + needWait) }
	for _, name := range slices.Sorted(maps.Keys(rc.Needs)) {
		n := rc.Needs[name]
		if !needName.MatchString(name) || name == "port" {
			return RunUp{}, invalid("%s: a need's name is lower case letters, numbers, - and _, and not port, which is the run's own", needTable(rc, a.Recipe, name))
		}
		t, err := w.needTarget(a, name, n)
		if err != nil {
			return RunUp{}, err
		}
		notes = append(notes, t.notes...)
		need := runs.Need{Name: name, Env: strings.TrimSpace(n.Env), Mount: router.MountPath(n.Mount), Strip: n.Strip}
		up := NeedUp{Where: t.how}
		switch {
		case t.recipe == "" && t.url == "":
			// A folder: nothing's started for it.
			need.Repo, need.Worktree, need.Dir = t.repo, t.worktree, t.dir
		case t.url != "":
			need.Addr = t.url
		default:
			for i, h := range held {
				held[i], _ = store.HoldUntil(h, hold())
			}
			sub, err := w.startRun(RunAsk{
				Repo: t.repo, Worktree: t.worktree, Dir: t.dir, Recipe: t.recipe,
				Owner: runs.Owner{Kind: runs.ByRun}, By: a.By, Mission: a.Mission, Wait: a.Wait,
				Env: needEnv(a.Env), hold: hold(),
			}, above)
			if err != nil {
				return RunUp{}, fmt.Errorf("%s, which %s needs: %w", name, a.Recipe, err)
			}
			held = append(held, sub.Run)
			notes = append(notes, sub.Notes...)
			if a.Wait > 0 {
				ready, err := store.Wait(sub.Run, a.Wait)
				if err != nil {
					return RunUp{}, fmt.Errorf("%s, which %s needs: %w", name, a.Recipe, err)
				}
				if !ready {
					notes = append(notes, fmt.Sprintf("%s (%s) is running, but wasn't up on %d in %s: %s is started all the same.", name, sub.Run.Ref(), sub.Run.Port, a.Wait, a.Recipe))
				}
			}
			need.Run, need.Repo, need.Worktree, need.Recipe = sub.Run.Ref(), sub.Run.Repo, sub.Run.Worktree, sub.Run.Recipe
			need.Addr = w.RunAddr(sub.Run)
			up.Started = sub.Started
		}
		up.Need = need
		needs, ups = append(needs, need), append(ups, up)
	}

	command, env := rc.Command, a.Env
	if len(needs) > 0 {
		if env == nil {
			env = os.Environ()
		}
		env = slices.Clone(env)
		for _, n := range needs {
			said := n.Addr
			if n.IsFolder() {
				said = n.Dir
			} else {
				command = strings.ReplaceAll(command, "{"+n.Name+".port}", w.needPort(n))
			}
			command = strings.ReplaceAll(command, "{"+n.Name+"}", said)
			if n.Env != "" {
				env = append(withoutEnv(env, n.Env), n.Env+"="+said)
			}
		}
	}
	name := a.Name
	if name == "" {
		name = a.Recipe
	}
	st := runs.Start{
		Name: name, Repo: a.Repo, Worktree: a.Worktree, Dir: a.Dir,
		Command: command, Recipe: a.Recipe, Ready: rc.Ready, Stop: rc.Stop,
		Owner: a.Owner, TTL: a.TTL, By: a.By, Env: env, Needs: needs, Hold: a.hold, Check: a.Check,
	}
	if typ == runs.Task {
		st.Type = runs.Task
		st.Commit, st.Dirty = standsAt(a.Dir)
		for _, n := range slices.Sorted(maps.Keys(rc.Reports)) {
			st.Reports = append(st.Reports, runs.Report{Name: n, Path: strings.TrimSpace(rc.Reports[n].Path), Format: strings.TrimSpace(rc.Reports[n].Format)})
		}
	}
	r, started, err := store.Ensure(st, now)
	if err != nil {
		return RunUp{}, err
	}
	// It's recorded as needing them now, which is what keeps them.
	for _, h := range held {
		logFailed(store.HoldUntil(h, time.Time{}))
	}
	if !started {
		// Another ws started the recipe at the same moment: that run's
		// needs are the ones it was started with.
		ups = nil
		for _, n := range r.Needs {
			ups = append(ups, NeedUp{Need: n})
		}
	}
	if on, _, _ := w.Names(); !on {
		for _, n := range r.Needs {
			if n.Mount != "" {
				notes = append(notes, fmt.Sprintf("%s answers at %s of %s's own name only where the workspace has names: [router] enabled = true in ws.local.toml, or Settings › Runs.", n.Name, n.Mount, r.Name))
			}
		}
	}
	return RunUp{Run: r, Started: started, Needs: ups, Notes: notes}, nil
}

// taskTime is how long a task asked for may take: what the asker says
// (a.Timeout), else its recipe's timeout as written, else
// runs.TaskTimeout. 0 is no limit.
func taskTime(a RunAsk, written string) (time.Duration, error) {
	switch {
	case a.Timeout < 0:
		return 0, nil
	case a.Timeout > 0:
		return a.Timeout, nil
	}
	if written = strings.TrimSpace(written); written == "" {
		return runs.TaskTimeout, nil
	}
	d, err := time.ParseDuration(written)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("timeout is how long it may take (10m, 1h30m), not %q", written)
	}
	return d, nil
}

// standsAt is the commit the worktree at dir is at, and whether it has
// changes that aren't committed: what a task's result is a result of.
func standsAt(dir string) (commit string, dirty bool) {
	return gitOut(dir, "rev-parse", "HEAD"), gitOut(dir, "status", "--porcelain") != ""
}

// runAgain is a recipe asked for where it's running already: the run is
// the asker's as well, and what it needs is started again where that has
// ended, at the places it was started with.
func (w *Workspace) runAgain(a RunAsk, old runs.Run, above []string) (RunUp, error) {
	store := w.runStore()
	if !a.hold.IsZero() {
		old, _ = store.HoldUntil(old, a.hold)
	}
	r, err := store.Join(old, a.Owner, a.TTL, time.Now())
	if err != nil {
		return RunUp{}, err
	}
	up := RunUp{Run: r}
	if len(a.With) > 0 {
		up.Notes = append(up.Notes, fmt.Sprintf("%s is running already, with what it needs as it was started: --with is for when it's started (ws run stop %s first).", r.Name, r.Ref()))
	}
	for _, n := range r.Needs {
		nu := NeedUp{Need: n}
		if n.Run != "" {
			sub, err := w.startRun(RunAsk{
				Repo: n.Repo, Worktree: n.Worktree, Dir: filepath.Join(w.WS().RepoDir(n.Repo), n.Worktree), Recipe: n.Recipe,
				Owner: runs.Owner{Kind: runs.ByRun}, By: a.By, Mission: a.Mission, Wait: a.Wait, Env: needEnv(a.Env),
			}, above)
			switch {
			case err != nil:
				up.Notes = append(up.Notes, fmt.Sprintf("%s (%s), which %s needs, isn't running and couldn't be started: %v", n.Name, n.Run, r.Name, err))
			case sub.Started:
				nu.Started = true
				up.Notes = append(up.Notes, sub.Notes...)
				if a.Wait > 0 {
					if ready, err := store.Wait(sub.Run, a.Wait); err != nil {
						up.Notes = append(up.Notes, fmt.Sprintf("%s (%s), which %s needs, was started again, and %v.", n.Name, n.Run, r.Name, err))
					} else if !ready {
						up.Notes = append(up.Notes, fmt.Sprintf("%s (%s) was started again, but wasn't up on %d in %s.", n.Name, n.Run, sub.Run.Port, a.Wait))
					}
				}
				if addr := w.RunAddr(sub.Run); addr != n.Addr {
					up.Notes = append(up.Notes, fmt.Sprintf("%s is at %s now, and %s was started with %s: ws run stop %s, then run it again, for it to have the new one.", n.Name, addr, r.Name, n.Addr, r.Ref()))
				}
			}
		}
		up.Needs = append(up.Needs, nu)
	}
	return up, nil
}

// runningRecipe is the run of recipe that's running in repo's worktree.
func (w *Workspace) runningRecipe(repo, worktree, recipe string) (runs.Run, bool) {
	all, _ := w.runStore().List()
	for _, r := range all {
		if r.State == runs.Running && r.Repo == repo && r.Worktree == worktree && r.Recipe == recipe {
			return r, true
		}
	}
	return runs.Run{}, false
}

// needTarget is where a need is met: a recipe in one of a repo's
// worktrees, or an address outside ws.
type needTarget struct {
	repo, worktree, dir, recipe string
	url                         string
	// how says how the worktree was chosen, when it's another repo's.
	how string
	// notes are what's worth saying of how it was chosen: a pointer to a
	// capsule that's gone.
	notes []string
}

// needTarget is what meets the need name of a's recipe. What a.With says
// for it; else the nearest: the same worktree for a recipe of the same
// repo, and for another repo's, the mission's capsule there, or with
// none, that repo's ground.
//
// A need of a repo's folder (repo = "…") is met the same way, by one of
// that repo's worktrees, with no recipe and nothing started. A workspace
// recipe's needs are met by what they're pointed at (PointRun), where
// nothing's said for them by a.With; one pointed at a capsule that's gone
// is met by the nearest, and a note says so.
func (w *Workspace) needTarget(a RunAsk, name string, n config.RunNeed) (needTarget, error) {
	if err := needSaidRight(a, name, n); err != nil {
		return needTarget{}, err
	}
	repo, recipe := n.Recipe(name)
	what := recipe + " of " + repo
	if n.IsFolder() {
		repo, recipe = strings.TrimSpace(n.Repo), ""
		if r, ok := w.WS().ResolveAlias(repo); ok {
			repo = r
		}
		what = repo + "'s folder"
	} else if repo == "" {
		repo = a.Repo
		what = recipe
	}
	if recipe == "" && !n.IsFolder() {
		return needTarget{}, invalid("%s needs %s, and its run names no recipe (%q)", a.Recipe, name, n.Run)
	}
	with := strings.TrimSpace(a.With[name])
	var notes []string
	if with == "" && a.Repo == "" {
		if p := w.runStore().Points(a.Recipe)[name]; p != "" {
			if dirExists(filepath.Join(w.WS().RepoDir(repo), p)) {
				with = p
			} else {
				notes = append(notes, fmt.Sprintf("%s was pointed at %s, which %s no longer has: it's on the nearest now.", name, runs.Label(p), repo))
			}
		}
	}
	if u, err := url.Parse(with); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		if n.IsFolder() {
			return needTarget{}, invalid("%s is %s: an address can't meet it", name, what)
		}
		return needTarget{url: strings.TrimRight(with, "/"), how: "as you said"}, nil
	}
	if repo == "" {
		// One of the workspace's own recipes: it runs at the root, and
		// nothing else can meet it.
		if with != "" {
			return needTarget{}, invalid("%s is the workspace's own %s: only an address (https://…) can meet it in its place", name, recipe)
		}
		if _, ok := w.recipes("", "")[recipe]; !ok {
			return needTarget{}, invalid("%s needs %s, and the workspace has no recipe %s ([run.service.%s] in %s)", a.Recipe, recipe, recipe, recipe, config.FileName)
		}
		return needTarget{recipe: recipe, dir: w.Root(), notes: notes}, nil
	}
	if !dirExists(w.WS().RepoDir(repo)) {
		return needTarget{}, invalid("%s needs %s, and the workspace has no repo %s", a.Recipe, what, repo)
	}
	t := needTarget{repo: repo, recipe: recipe, notes: notes}
	switch {
	case with == "ground" || with == workspace.GroundDir:
		t.worktree, t.how = workspace.GroundDir, "as you said"
	case with == "silo" || with == workspace.SiloDir:
		t.worktree, t.how = workspace.SiloDir, "as you said"
	case with != "":
		t.worktree, t.how = with, "as you said"
	case repo == a.Repo:
		t.worktree = a.Worktree
	default:
		t.worktree, t.how = w.nearest(a.Mission, repo)
	}
	t.dir = filepath.Join(w.WS().RepoDir(repo), t.worktree)
	if !dirExists(t.dir) {
		if n.IsFolder() {
			return needTarget{}, invalid("%s needs %s, and %s has no %s", a.Recipe, what, repo, runs.Label(t.worktree))
		}
		return needTarget{}, invalid("%s needs %s, and %s has no %s to run it in", a.Recipe, what, repo, runs.Label(t.worktree))
	}
	if n.IsFolder() {
		return t, nil
	}
	if _, ok := w.recipes(repo, t.worktree)[recipe]; !ok {
		return needTarget{}, invalid("%s needs %s, and %s/%s has no recipe %s ([run.service.%s] in its %s)", a.Recipe, what, repo, runs.Label(t.worktree), recipe, recipe, config.RepoFileName)
	}
	return t, nil
}

// needSaidRight is an error where a need is said in a way ws can't meet:
// a folder is a folder, and not a run too, nor mounted.
func needSaidRight(a RunAsk, name string, n config.RunNeed) error {
	if !n.IsFolder() {
		return nil
	}
	file := recipeFile(a.Repo)
	switch {
	case strings.TrimSpace(n.Run) != "":
		return invalid("%s needs %s, which in %s says both repo, a folder, and run, a run: it's one or the other", a.Recipe, name, file)
	case strings.TrimSpace(n.Mount) != "" || n.Strip:
		return invalid("%s needs %s, which in %s is a folder: there's nothing to mount", a.Recipe, name, file)
	}
	return nil
}

// recipeFile is the file a recipe is said in: the repo's ws.repo.toml,
// or with no repo, the workspace's ws.toml.
func recipeFile(repo string) string {
	if repo == "" {
		return config.FileName
	}
	return config.RepoFileName
}

// nearest is the worktree of repo nearest to a mission's work: the
// mission's capsule there, or with none (or no mission), repo's ground.
func (w *Workspace) nearest(slug, repo string) (worktree, how string) {
	if slug != "" {
		if m, err := w.load(slug); err == nil {
			for _, c := range m.Capsules {
				if c.Repo == repo && dirExists(filepath.Join(w.WS().RepoDir(repo), c.Name)) {
					return c.Name, fmt.Sprintf("mission %s's capsule in %s", slug, repo)
				}
			}
		}
	}
	return workspace.GroundDir, its(repo) + " ground"
}

// its is a name as what something belongs to: backend's, records'.
func its(name string) string {
	if strings.HasSuffix(name, "s") {
		return name + "'"
	}
	return name + "'s"
}

// RunAddr is where a run is reached, as it's handed to a run that needs
// it: by its name where the workspace has names, else at 127.0.0.1 and
// its port, with no trailing slash, so a path can follow it.
func (w *Workspace) RunAddr(r runs.Run) string {
	return strings.TrimRight(router.RunAddress(w.Root(), r), "/")
}

// needPort is the port a need answers on, on this machine for a run of
// ws's, as {name.port} says it: "" where an address outside ws names none.
func (w *Workspace) needPort(n runs.Need) string {
	if n.Run != "" {
		all, _ := w.runStore().List()
		for _, r := range all {
			if r.Ref() == n.Run {
				return strconv.Itoa(r.Port)
			}
		}
		return ""
	}
	if u, err := url.Parse(n.Addr); err == nil {
		return u.Port()
	}
	return ""
}

// needTable is the table of one of a recipe's needs, as its file has it:
// [run.service.web.needs.api].
func needTable(rc config.RunRecipe, recipe, need string) string {
	return strings.TrimSuffix(rc.Table(recipe), "]") + ".needs." + need + "]"
}

// needsSaid is a recipe's needs, to follow an error about one it hasn't.
func needsSaid(rc config.RunRecipe) string {
	if len(rc.Needs) == 0 {
		return ": it needs nothing"
	}
	return " (its needs: " + strings.Join(slices.Sorted(maps.Keys(rc.Needs)), ", ") + ")"
}

// needEnv is the environment of a run started for another's need: the
// asker's, less whose work the asker is (a mission, its crew, a
// conversation). It's every run's that needs it, not theirs.
func needEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	for _, k := range []string{mission.EnvMission, mission.EnvCrew, mission.EnvRole, mission.EnvRun, conversation.EnvConversation} {
		env = withoutEnv(env, k)
	}
	return env
}

func withoutEnv(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if k, _, _ := strings.Cut(kv, "="); k != key {
			out = append(out, kv)
		}
	}
	return out
}
