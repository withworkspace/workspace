package ops

import (
	"fmt"
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

// taskCache is a cached recipe's run as it's asked for here: its key and
// the outputs a pass keeps, or why it isn't cached this time.
type taskCache struct {
	key     string
	outputs []string
	why     string
}

// cacheFor is the recipe's run, asked for as a says, as the cache has it.
func (w *Workspace) cacheFor(a RunAsk, rc config.RunRecipe) taskCache {
	if len(rc.Needs) > 0 {
		return taskCache{why: fmt.Sprintf("%s needs %s up, which its key can't say: it's run, and not cached.", a.Recipe, strings.Join(slices.Sorted(maps.Keys(rc.Needs)), ", "))}
	}
	outputs := slices.Clone(rc.Outputs)
	for _, rp := range reportsOf(rc) {
		outputs = append(outputs, rp.Path)
	}
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
	key, err := runs.Key(runs.KeyOf{Repo: a.Repo, Command: rc.Command, Dir: a.Dir, Inputs: rc.Inputs, Outputs: outputs, Env: env})
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
func (w *Workspace) reuse(a RunAsk, rc config.RunRecipe, c taskCache) (up RunUp, ok bool, notes []string) {
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
		Reports: reportsOf(rc), Check: a.Check, Chain: a.Chain, Key: c.key, Outputs: c.outputs,
	}
	st.Commit, st.Dirty = standsAt(a.Dir)
	r, started, err := store.Reuse(st, e, time.Now())
	if err != nil {
		return RunUp{}, false, []string{fmt.Sprintf("Couldn't reuse %s (%v): it's run.", e.From, err)}
	}
	return RunUp{Run: r, Started: started}, true, nil
}

// ReusedSaid is how a reused run is said after what it came to: "reused
// from test.ground.api#12, which took 3m10s".
func ReusedSaid(r runs.Run) string {
	if r.Reused == nil {
		return ""
	}
	return fmt.Sprintf("reused from %s, which took %s", r.Reused.From, runs.Short(r.Reused.Took))
}
