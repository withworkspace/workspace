package ops

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/runs"
)

// A task whose recipe names its inputs is cached: asked for again with the
// same inputs as a run of it that passed, that pass is reused (its outputs
// put back) rather than run (runs.Store.Reuse). Its key is worked out as
// it's asked for; a pass is kept by its own wrapper as it ends
// (runs.Store.KeepPass, through ws run cache keep).

// keepWith is the command a cached task's wrapper runs to keep a pass,
// with the run's folder after it: this ws.
var keepWith = func() []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return []string{exe, "run", "cache", "keep"}
}

// afterWith is the command a task's wrapper runs to run its After first,
// with the run's folder after it: this ws.
var afterWith = func() []string {
	k := keepWith()
	if len(k) == 0 {
		return nil
	}
	return []string{k[0], "run", "cache", "after"}
}

// taskCache is a cached recipe's run as it's asked for here: its key and
// the outputs a pass keeps, or why it isn't cached this time.
type taskCache struct {
	key     string
	outputs []string
	why     string
}

// outputsOf is what a task's recipe says it writes: its outputs, and its
// reports' paths.
func outputsOf(rc config.RunRecipe) []string {
	out := slices.Clone(rc.Outputs)
	for _, rp := range reportsOf(rc) {
		out = append(out, rp.Path)
	}
	return out
}

// aftersOf is the tasks the recipe a asks for is run after, each with its
// key where it's cached here. above are the tasks being asked for that
// it's an after of, so one that comes back round is refused rather than
// run for ever.
func (w *Workspace) aftersOf(a RunAsk, rc config.RunRecipe, above []string) ([]runs.After, error) {
	if len(rc.After) == 0 {
		return nil, nil
	}
	above = append(slices.Clone(above), a.Recipe)
	all := w.recipes(a.Repo, a.Worktree)
	var out []runs.After
	for _, name := range rc.After {
		name = strings.TrimSpace(name)
		if slices.Contains(above, name) {
			return nil, invalid("%s comes back round: %s", name, strings.Join(append(above, name), ", which is run after "))
		}
		after, ok := all[name]
		switch {
		case !ok:
			return nil, invalid("%s is run after %s, which is no recipe of %s", a.Recipe, name, recipeFile(a.Repo))
		case !after.IsTask():
			return nil, invalid("%s is run after %s, which is a service: a task is run after tasks, and what it needs up is a need", a.Recipe, name)
		}
		sub := a
		sub.Recipe = name
		theirs, err := w.aftersOf(sub, after, above)
		if err != nil {
			return nil, err
		}
		got := runs.After{Recipe: name, Outputs: outputsOf(after)}
		if after.Cached() {
			got.Key = w.cacheFor(sub, after, theirs).key
		}
		out = append(out, got)
	}
	return out, nil
}

// cacheFor is the recipe's run, asked for as a says, as the cache has it,
// run after afters.
func (w *Workspace) cacheFor(a RunAsk, rc config.RunRecipe, afters []runs.After) taskCache {
	if len(rc.Needs) > 0 {
		return taskCache{why: fmt.Sprintf("%s needs %s up, which its key can't say: it's run, and not cached.", a.Recipe, strings.Join(slices.Sorted(maps.Keys(rc.Needs)), ", "))}
	}
	outputs := outputsOf(rc)
	env := map[string]string{}
	have := a.Env
	if have == nil {
		have = os.Environ()
	}
	for _, name := range rc.Env {
		name = strings.TrimSpace(name)
		env[name] = ""
		for _, kv := range have {
			if k, v, ok := strings.Cut(kv, "="); ok && k == name {
				env[name] = v
			}
		}
	}
	key, err := runs.Key(runs.KeyOf{Repo: a.Repo, Command: rc.Command, Dir: a.Dir, Inputs: rc.Inputs, Outputs: outputs, Env: env, After: afters})
	if err != nil {
		return taskCache{why: fmt.Sprintf("%s isn't cached this time: %v.", a.Recipe, err)}
	}
	return taskCache{key: key, outputs: outputs}
}

// reportsOf is the reports a task's recipe names, in order.
func reportsOf(rc config.RunRecipe) []runs.Report {
	var out []runs.Report
	for _, n := range slices.Sorted(maps.Keys(rc.Reports)) {
		out = append(out, runs.Report{Name: n, Path: strings.TrimSpace(rc.Reports[n].Path), Format: strings.TrimSpace(rc.Reports[n].Format)})
	}
	return out
}

// reuse is a's run made from the pass kept under c's key, if there is one:
// ok false where there isn't, or it couldn't be put back (notes say why).
// What it's run after is asked for too, so their outputs are put back:
// each reused where it's kept, else run.
func (w *Workspace) reuse(a RunAsk, rc config.RunRecipe, c taskCache, afters []runs.After) (up RunUp, ok bool, notes []string) {
	store := w.runStore()
	e, found := store.Lookup(c.key)
	if !found {
		return RunUp{}, false, nil
	}
	name := a.Name
	if name == "" {
		name = a.Recipe
	}
	st := runs.Start{
		Name: name, Type: runs.Task, Repo: a.Repo, Worktree: a.Worktree, Dir: a.Dir,
		Command: rc.Command, Recipe: a.Recipe, Stop: rc.Stop, Owner: a.Owner, TTL: a.TTL, By: a.By,
		Reports: reportsOf(rc), Check: a.Check, Chain: a.Chain, Key: c.key, Outputs: c.outputs, After: afters,
	}
	st.Commit, st.Dirty = standsAt(a.Dir)
	r, started, err := store.Reuse(st, e, time.Now())
	if err != nil {
		return RunUp{}, false, []string{fmt.Sprintf("Couldn't reuse %s (%v): it's run.", e.From, err)}
	}
	var ran []string
	for _, af := range afters {
		sub := RunAsk{Repo: a.Repo, Worktree: a.Worktree, Dir: a.Dir, Recipe: af.Recipe, Owner: runs.Owner{Kind: runs.ByNone}, By: a.By, Chain: a.Chain, Env: a.Env}
		up, err := w.startRun(sub, nil)
		switch {
		case err != nil:
			notes = append(notes, fmt.Sprintf("Couldn't ask for %s, which %s is run after, so its outputs weren't put back: %v", af.Recipe, a.Recipe, err))
		case up.Run.Reused == nil:
			notes = append(notes, fmt.Sprintf("%s's pass isn't kept, so it's running (%s) to write its outputs again.", af.Recipe, up.Run.NRef()))
			ran = append(ran, up.Run.NRef())
		default:
			ran = append(ran, up.Run.NRef())
		}
		notes = append(notes, up.Notes...)
	}
	if len(ran) > 0 {
		logFailed(runs.AfterRan(r.Path, ran))
		r.AfterRan = ran
	}
	return RunUp{Run: r, Started: started, Notes: notes}, true, nil
}

// RunAfters runs the tasks the task in dir is run after, one after
// another, each reused where it's cached, waiting for each to end and
// printing what it printed to out: what the task's wrapper runs, before
// its command (ws run cache after). It's 0 once they've all passed, else
// the status of the first that didn't, and the task's command isn't run.
func (w *Workspace) RunAfters(dir string, out io.Writer) int {
	r, err := runs.At(dir)
	if err != nil {
		fmt.Fprintf(out, "ws: %v\n", err)
		return 1
	}
	store := w.runStore()
	var ran []string
	defer func() { logFailed(runs.AfterRan(dir, ran)) }()
	for _, af := range r.After {
		up, err := w.StartRun(RunAsk{
			Repo: r.Repo, Worktree: r.Worktree, Dir: r.Dir, Recipe: af.Recipe,
			Owner: runs.Owner{Kind: runs.ByNone}, By: r.By, Chain: r.Chain,
		})
		if err != nil {
			fmt.Fprintf(out, "ws: couldn't run %s, which %s is run after: %v\n", af.Recipe, r.Name, err)
			return 1
		}
		ran = append(ran, up.Run.NRef())
		fmt.Fprintf(out, "ws: %s first (%s)\n", af.Recipe, up.Run.NRef())
		now, _, err := store.WaitEnd(up.Run, 0, func(s string) { _, _ = io.WriteString(out, s) })
		if err != nil {
			fmt.Fprintf(out, "ws: couldn't follow %s: %v\n", up.Run.NRef(), err)
			return 1
		}
		switch {
		case now.Reused != nil:
			fmt.Fprintf(out, "ws: %s passed, %s\n", now.NRef(), ReusedSaid(now))
		case now.Passed():
			fmt.Fprintf(out, "ws: %s passed\n", now.NRef())
		default:
			fmt.Fprintf(out, "ws: %s %s: %s isn't run\n", now.NRef(), now.Result(), r.Name)
			if now.State == runs.Exited && now.Status != 0 {
				return now.Status
			}
			return 1
		}
	}
	return 0
}

// ReusedSaid is how a reused run is said after what it came to: "reused
// from test.ground.api#12, which took 3m10s".
func ReusedSaid(r runs.Run) string {
	if r.Reused == nil {
		return ""
	}
	return fmt.Sprintf("reused from %s, which took %s", r.Reused.From, runs.Short(r.Reused.Took))
}
