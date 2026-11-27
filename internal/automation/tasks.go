package automation

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"go.starlark.net/starlark"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/store"
)

// A script says what it wants done, and ws does it: run_task asks for a
// task (a repo's recipe: its tests, its coverage) and is given a handle
// at once, the task queued; ws starts what's queued as this machine has
// room (MaxTasks at a time, across every automation), on the repo's
// ground (its default branch, as it stands). wait_all and wait_any park
// the run till the tasks it's given (or events) have ended, and give it
// what each came to: its status, and what its reports say
// (internal/report). The fan-out is a loop of run_task, the join a
// wait_all: no thread or promise, and nothing held in memory, so a run
// waiting on an hour's tests outlives a restart.

// MaxTasks is how many tasks automations run at once, on this machine,
// unless the engine says otherwise.
const MaxTasks = 2

// lostAfter is how long a task's left running with no word of its end
// before it's taken to be lost: its slot's given back.
const lostAfter = 12 * time.Hour

// The states of a task asked for.
const (
	TaskQueued  = "queued"
	TaskRunning = "running"
	TaskEnded   = "ended"
)

func init() {
	store.Define("automation-tasks", store.SQL(`
		-- The tasks automations asked for (run_task): queued, then started
		-- as there's room, and what they came to once they've ended.
		CREATE TABLE automation_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,                 -- the automation
			event INTEGER NOT NULL,             -- the event its run's on
			repo TEXT NOT NULL,
			recipe TEXT NOT NULL,
			chain TEXT NOT NULL DEFAULT '',     -- the automations that led to it, as JSON
			state TEXT NOT NULL,                -- queued, running, ended
			ref TEXT NOT NULL DEFAULT '',       -- the run's, once started
			n INTEGER NOT NULL DEFAULT 0,       -- which run of the task
			after INTEGER NOT NULL DEFAULT 0,   -- the latest event when it started
			status TEXT NOT NULL DEFAULT '',    -- how it ended: passed, failed (exit 1)…
			reports TEXT NOT NULL DEFAULT '',   -- what its reports say, as JSON
			queued TEXT NOT NULL,
			started TEXT NOT NULL DEFAULT '',
			ended TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX automation_tasks_state ON automation_tasks (state, id);
	`))
}

// Task is a task an automation asked for, as kept.
type Task struct {
	ID      int64                     `json:"id"`
	Name    string                    `json:"name"`
	Event   int64                     `json:"event"`
	Repo    string                    `json:"repo"`
	Recipe  string                    `json:"recipe"`
	Chain   []string                  `json:"chain,omitempty"`
	State   string                    `json:"state"`
	Ref     string                    `json:"ref,omitempty"`
	N       int                       `json:"n,omitempty"`
	After   int64                     `json:"-"`
	Status  string                    `json:"status,omitempty"`
	Reports map[string]map[string]any `json:"reports,omitempty"`
	Queued  time.Time                 `json:"queued"`
	Started time.Time                 `json:"started,omitzero"`
	Ended   time.Time                 `json:"ended,omitzero"`
}

// result is what a task came to, as a wait gives it a script.
func (t Task) result() map[string]any {
	out := map[string]any{"task": t.ID, "repo": t.Repo, "recipe": t.Recipe, "ref": t.Ref, "n": t.N, "status": t.Status, "passed": t.Status == "passed"}
	reports := map[string]any{}
	for k, v := range t.Reports {
		reports[k] = v
	}
	out["reports"] = reports
	return out
}

func queueTask(root string, t Task, at time.Time) (int64, error) {
	var id int64
	chain, _ := json.Marshal(t.Chain)
	err := write(root, func(tx *store.Tx) error {
		res, err := tx.Exec(`INSERT INTO automation_tasks (name, event, repo, recipe, chain, state, queued) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			t.Name, t.Event, t.Repo, t.Recipe, string(chain), TaskQueued, stampOf(at))
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return id, err
}

// Tasks are the tasks asked for in state ("" for any), oldest first, at
// most limit (0 for no limit).
func Tasks(root, state string, limit int) ([]Task, error) {
	var out []Task
	err := read(root, func(tx *store.Tx) error {
		q, args := `SELECT id, name, event, repo, recipe, chain, state, ref, n, after, status, reports, queued, started, ended FROM automation_tasks`, []any{}
		if state != "" {
			q, args = q+` WHERE state = ?`, append(args, state)
		}
		q += ` ORDER BY id`
		if limit > 0 {
			q, args = q+` LIMIT ?`, append(args, limit)
		}
		rows, err := tx.Query(q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t Task
			var chain, reports, queued, started, ended string
			if err := rows.Scan(&t.ID, &t.Name, &t.Event, &t.Repo, &t.Recipe, &chain, &t.State, &t.Ref, &t.N, &t.After, &t.Status, &reports, &queued, &started, &ended); err != nil {
				return err
			}
			_ = json.Unmarshal([]byte(chain), &t.Chain)
			_ = json.Unmarshal([]byte(reports), &t.Reports)
			t.Queued = timeOf(queued)
			if started != "" {
				t.Started = timeOf(started)
			}
			if ended != "" {
				t.Ended = timeOf(ended)
			}
			out = append(out, t)
		}
		return rows.Err()
	})
	return out, err
}

// taskByID is one task asked for.
func taskByID(root string, id int64) (Task, bool) {
	var t Task
	found := false
	_ = read(root, func(tx *store.Tx) error {
		var chain, reports string
		err := tx.QueryRow(`SELECT id, repo, recipe, chain, state, ref, n, status, reports FROM automation_tasks WHERE id = ?`, id).
			Scan(&t.ID, &t.Repo, &t.Recipe, &chain, &t.State, &t.Ref, &t.N, &t.Status, &reports)
		if err != nil {
			return nil
		}
		_ = json.Unmarshal([]byte(chain), &t.Chain)
		_ = json.Unmarshal([]byte(reports), &t.Reports)
		found = true
		return nil
	})
	return t, found
}

func startedTask(root string, id int64, ref string, n int, after int64, at time.Time) error {
	return write(root, func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE automation_tasks SET state = ?, ref = ?, n = ?, after = ?, started = ? WHERE id = ?`, TaskRunning, ref, n, after, stampOf(at), id)
		return err
	})
}

func endedTask(root string, id int64, status string, reports map[string]map[string]any, at time.Time) error {
	b := []byte("")
	if len(reports) > 0 {
		b, _ = json.Marshal(reports)
	}
	return write(root, func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE automation_tasks SET state = ?, status = ?, reports = ?, ended = ? WHERE id = ?`, TaskEnded, status, string(b), stampOf(at), id)
		return err
	})
}

// latestN is the number of the latest event kept.
func latestN(root string) int64 {
	var n int64
	_ = read(root, func(tx *store.Tx) error {
		var err error
		n, err = latestEvent(tx)
		return err
	})
	return n
}

// tasksPass moves the tasks asked for along: one whose run has ended is
// ended, with what its reports say; one lost is ended so; and what's
// queued is started as there's room. It writes only what's moved.
func (e *Engine) tasksPass() []string {
	running, err := Tasks(e.Root, TaskRunning, 0)
	if err != nil {
		return nil
	}
	queued, _ := Tasks(e.Root, TaskQueued, 0)
	if len(running) == 0 && len(queued) == 0 {
		return nil
	}
	var said []string
	now := e.now()
	busy := 0
	for _, t := range running {
		ended, ok := e.runEnded(t)
		switch {
		case ok:
			var reports map[string]map[string]any
			if e.Hooks.Report != nil {
				reports = e.Hooks.Report(t.Ref, t.N)
			}
			_ = endedTask(e.Root, t.ID, ended.Status, reports, now)
			said = append(said, fmt.Sprintf("%s's task %s on %s: %s", t.Name, t.Recipe, t.Repo, ended.Status))
		case now.Sub(t.Started) > lostAfter:
			_ = endedTask(e.Root, t.ID, "lost: its run never said it ended", nil, now)
		default:
			busy++
		}
	}
	most := e.MaxTasks
	if most <= 0 {
		most = MaxTasks
	}
	for _, t := range queued {
		if busy >= most {
			break
		}
		if e.Hooks.RunTask == nil {
			_ = endedTask(e.Root, t.ID, "not run: there's nothing here to run it", nil, now)
			continue
		}
		after := latestN(e.Root)
		ref, n, err := e.Hooks.RunTask(&Automation{Name: t.Name}, t.Repo, t.Recipe, t.Chain)
		if err != nil {
			_ = endedTask(e.Root, t.ID, "not run: "+err.Error(), nil, now)
			continue
		}
		_ = startedTask(e.Root, t.ID, ref, n, after, now)
		busy++
		said = append(said, fmt.Sprintf("%s started %s on %s (%s#%d)", t.Name, t.Recipe, t.Repo, ref, n))
	}
	return said
}

// runEnded is the run.ended of t's run, if it's been kept.
func (e *Engine) runEnded(t Task) (events.Event, bool) {
	list, err := events.List(e.Root, events.Filter{After: t.After, Kinds: []string{events.RunEnded}})
	if err != nil {
		return events.Event{}, false
	}
	for _, ev := range list {
		if ev.Run == t.Ref && (ev.RunN == t.N || t.N == 0) {
			return ev, true
		}
	}
	return events.Event{}, false
}

// taskHandle is a task asked for, as a script holds it: what a wait
// takes.
type taskHandle struct {
	id           int64
	repo, recipe string
}

func (h taskHandle) String() string        { return fmt.Sprintf("task(%s on %s)", h.recipe, h.repo) }
func (h taskHandle) Type() string          { return "task" }
func (h taskHandle) Freeze()               {}
func (h taskHandle) Truth() starlark.Bool  { return true }
func (h taskHandle) Hash() (uint32, error) { return starlark.MakeInt64(h.id).Hash() }
func (h taskHandle) AttrNames() []string   { return []string{"repo", "recipe"} }
func (h taskHandle) Attr(name string) (starlark.Value, error) {
	switch name {
	case "repo":
		return starlark.String(h.repo), nil
	case "recipe":
		return starlark.String(h.recipe), nil
	}
	return nil, nil
}

// runTask is run_task(recipe, repo = …): a task asked for, queued; its
// handle at once.
func (r *run) runTask(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if !r.a.Allows(MayRun) {
		return nil, fmt.Errorf("run_task: it may not (may = [\"run\"] says it may)")
	}
	var recipe, repo string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "recipe", &recipe, "repo", &repo); err != nil {
		return nil, err
	}
	if r.e.Hooks.Repos != nil {
		if err := hasTask(r.e.Hooks.Repos(), repo, recipe); err != nil {
			return nil, fmt.Errorf("run_task: %w", err)
		}
	}
	h := taskHandle{repo: repo, recipe: recipe}
	ask := func() (string, error) {
		id, err := queueTask(r.e.Root, Task{Name: r.a.Name, Event: r.ev.N, Repo: repo, Recipe: recipe, Chain: ChainOf(r.a.Name, r.ev, r.batch)}, r.e.now())
		return fmt.Sprint(id), err
	}
	if r.dry != nil {
		// A dry run runs nothing: the task's said, and a wait on it is
		// given what it'd have come to, unknown.
		ask = func() (string, error) {
			id := -int64(len(r.dry.tasks) + 1)
			r.dry.tasks[id] = taskHandle{id: id, repo: repo, recipe: recipe}
			r.dry.did = append(r.dry.did, fmt.Sprintf("run %s on %s", recipe, repo))
			return fmt.Sprint(id), nil
		}
	}
	got, err := r.step("run_task", ask)
	if err != nil {
		return nil, err
	}
	if h.id, err = strconv.ParseInt(got, 10, 64); err != nil {
		return nil, fmt.Errorf("run_task: %q isn't a task's number: %w", got, err)
	}
	return h, nil
}

// hasTask is whether repo has a task called recipe, as its config says.
func hasTask(repos []RepoInfo, repo, recipe string) error {
	for _, rp := range repos {
		if rp.Name != repo {
			continue
		}
		for _, t := range rp.Tasks {
			if t.Name == recipe {
				return nil
			}
		}
		var names []string
		for _, t := range rp.Tasks {
			names = append(names, t.Name)
		}
		if len(names) == 0 {
			return fmt.Errorf("%s has no tasks ([run.task.<name>] in its ws.repo.toml)", repo)
		}
		return fmt.Errorf("%s has no task %s: it has %s", repo, recipe, strings.Join(names, ", "))
	}
	return fmt.Errorf("the workspace has no repo %s", repo)
}

// RepoInfo is a repo as repos() gives it a script: its name, group and
// what it's called, its default branch, and its tasks with their reports.
type RepoInfo struct {
	Name    string     `json:"name"`
	Group   string     `json:"group,omitempty"`
	Display string     `json:"display,omitempty"`
	Branch  string     `json:"branch,omitempty"`
	Tasks   []TaskInfo `json:"tasks"`
}

// TaskInfo is a repo's task recipe: its name, and its reports' names and
// formats.
type TaskInfo struct {
	Name    string            `json:"name"`
	Reports map[string]string `json:"reports,omitempty"`
}

// repos is repos(group = …, name = …): the workspace's repos, those of a
// group, or whose names a pattern matches. Each is .name, .group,
// .display, .branch and .tasks (the names of its task recipes), with
// .reports, each task's reports by name and format.
func (r *run) repos(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var group, name string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "group?", &group, "name?", &name); err != nil {
		return nil, err
	}
	got, err := r.read("repos", func() (any, error) {
		var out []map[string]any
		if r.e.Hooks.Repos == nil {
			return out, nil
		}
		for _, rp := range r.e.Hooks.Repos() {
			if group != "" && !like(group, rp.Group) || name != "" && !like(name, rp.Name) {
				continue
			}
			tasks, reports := []any{}, map[string]any{}
			for _, t := range rp.Tasks {
				tasks = append(tasks, t.Name)
				rs := map[string]any{}
				for k, v := range t.Reports {
					rs[k] = v
				}
				reports[t.Name] = rs
			}
			out = append(out, map[string]any{"name": rp.Name, "group": rp.Group, "display": rp.Display, "branch": rp.Branch, "tasks": tasks, "reports": reports})
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	if got == nil {
		return starlark.NewList(nil), nil
	}
	return valueOf(got), nil
}

func like(pattern, s string) bool {
	ok, _ := path.Match(strings.ToLower(pattern), strings.ToLower(s))
	return ok
}

// waitItem is one of what a wait_all or wait_any waits on: an event, or a
// task asked for.
type waitItem struct {
	Match string `json:"match,omitempty"`
	Task  int64  `json:"task,omitempty"`
	// Ask is an ask (ask's own wait), Proposal a proposal (propose's
	// handle: w12#1).
	Ask      int64  `json:"ask,omitempty"`
	Proposal string `json:"proposal,omitempty"`
}

// lookups are how a wait finds what it waits on, but events: a task asked
// for, an ask, a proposal. A nil one finds none.
type lookups struct {
	task     func(int64) (Task, bool)
	ask      func(int64) (AskInfo, bool)
	proposal func(string) (state, result string, ok bool)
}

// lookups are e's, as kept.
func (e *Engine) lookups() lookups {
	return lookups{
		task:     func(id int64) (Task, bool) { return taskByID(e.Root, id) },
		ask:      func(id int64) (AskInfo, bool) { return askByID(e.Root, id) },
		proposal: e.Hooks.Proposal,
	}
}

// waitMany is wait_all(items, timeout = …) and wait_any(…): the run
// parked till every one of them has come (all), or the first has (any),
// or its time's up. wait_all gives a list, each what it came to (an
// event, a task's result) or None; wait_any the first, or None.
func (r *run) waitMany(any bool) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var list starlark.Value
		timeout := "1d"
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "items", &list, "timeout?", &timeout); err != nil {
			return nil, err
		}
		iterable, ok := list.(starlark.Iterable)
		if !ok {
			return nil, fmt.Errorf("%s takes a list of tasks (run_task's), proposals (propose's) and event()s", b.Name())
		}
		var items []waitItem
		iter := iterable.Iterate()
		var x starlark.Value
		for iter.Next(&x) {
			switch v := x.(type) {
			case taskHandle:
				items = append(items, waitItem{Task: v.id})
			case proposalHandle:
				items = append(items, waitItem{Proposal: v.ref})
			case matchValue:
				items = append(items, waitItem{Match: v.m.String()})
			default:
				iter.Done()
				return nil, fmt.Errorf("%s: a %s isn't something to wait on: a task (run_task's), a proposal (propose's) or an event()", b.Name(), x.Type())
			}
		}
		iter.Done()
		d, err := lengthOf(timeout)
		if err != nil {
			return nil, fmt.Errorf("%s: timeout: %v", b.Name(), err)
		}
		if d > maxWait {
			return nil, fmt.Errorf("%s: timeout: at most %s", b.Name(), shortDuration(maxWait))
		}
		if len(items) == 0 {
			if any {
				return starlark.None, nil
			}
			return starlark.NewList(nil), nil
		}
		after, due := r.ev.N, r.e.now().Add(d)
		if r.dry != nil {
			due = r.ev.At.Add(d)
		}
		return r.parkOn(waitFor{Items: items, Any: any, After: after, Due: due})
	}
}

// parkOn is a wait's step: what it came to, once it's done; else it's
// kept as waiting, and the run's parked.
func (r *run) parkOn(w waitFor) (starlark.Value, error) {
	r.steps++
	n := r.steps
	state, result, err := r.stepState(n)
	switch {
	case err != nil:
		return nil, err
	case state == "done":
		return waitValue(w, result)
	case state == "waiting":
		return nil, errParked
	}
	kept, _ := json.Marshal(w)
	if err := r.keepStep(n, "wait", "waiting", string(kept)); err != nil {
		return nil, err
	}
	return nil, errParked
}

// waitValue is what a wait came to (result, as kept), as the script's
// given it.
func waitValue(w waitFor, result string) (starlark.Value, error) {
	if len(w.Items) == 0 || w.Any {
		return itemValue(json.RawMessage(result))
	}
	var each []json.RawMessage
	if err := json.Unmarshal([]byte(result), &each); err != nil {
		return nil, err
	}
	vs := make([]starlark.Value, len(each))
	for i, raw := range each {
		v, err := itemValue(raw)
		if err != nil {
			return nil, err
		}
		vs[i] = v
	}
	return starlark.NewList(vs), nil
}

// itemValue is one thing a wait came to: None, a task's result, or an
// event.
func itemValue(raw json.RawMessage) (starlark.Value, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return starlark.None, nil
	}
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	if _, ok := probe["task"]; ok {
		return valueOf(probe), nil
	}
	if _, ok := probe["proposal"]; ok {
		return valueOf(probe), nil
	}
	var ev events.Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		return nil, err
	}
	return eventValue(ev), nil
}

// waitCame is what w has come to so far, as kept (a JSON value): done
// when it's met (every item, or one for any) or its time's up by now;
// "" when it waits still. ours are events not to be met by; look finds
// what else it waits on.
func waitCame(w waitFor, list []events.Event, now time.Time, ours func(events.Event) bool, look lookups) (string, bool) {
	if len(w.Items) == 0 {
		m, err := events.ParseMatch(w.Match)
		if err != nil {
			return "null", true
		}
		for _, ev := range list {
			if ev.N > w.After && m.Matches(ev) && !ours(ev) && !ev.At.After(w.Due) {
				b, _ := json.Marshal(ev)
				return string(b), true
			}
		}
		if now.Before(w.Due) {
			return "", false
		}
		return "null", true
	}
	came := make([]json.RawMessage, len(w.Items))
	all := true
	for i, it := range w.Items {
		switch {
		case it.Task != 0:
			if look.task == nil {
				break
			}
			if t, ok := look.task(it.Task); ok && t.State == TaskEnded {
				b, _ := json.Marshal(t.result())
				came[i] = b
			}
		case it.Ask != 0:
			if look.ask == nil {
				break
			}
			if a, ok := look.ask(it.Ask); ok && a.State == TaskEnded {
				came[i] = json.RawMessage(a.outcomeOf())
			}
		case it.Proposal != "":
			if look.proposal == nil {
				break
			}
			if state, result, ok := look.proposal(it.Proposal); ok && state != ProposalOpen {
				b, _ := json.Marshal(proposalCame(it.Proposal, state, result))
				came[i] = b
			}
		default:
			m, err := events.ParseMatch(it.Match)
			if err != nil {
				break
			}
			for _, ev := range list {
				if ev.N > w.After && m.Matches(ev) && !ours(ev) && !ev.At.After(w.Due) {
					b, _ := json.Marshal(ev)
					came[i] = b
					break
				}
			}
		}
		if came[i] == nil {
			all = false
		} else if w.Any {
			return string(came[i]), true
		}
	}
	if all {
		b, _ := json.Marshal(came)
		return string(b), true
	}
	if now.Before(w.Due) {
		return "", false
	}
	if w.Any {
		return "null", true
	}
	for i := range came {
		if came[i] == nil {
			came[i] = json.RawMessage("null")
		}
	}
	b, _ := json.Marshal(came)
	return string(b), true
}

// ThingInfo is one of a plugin's things as things() gives it a script:
// its ref and kind (jira:issue), its title, where it stands, when it
// last changed (as its plugin says: an issue's updated), and its fields.
type ThingInfo struct {
	Ref      string            `json:"ref"`
	Kind     string            `json:"kind"`
	Title    string            `json:"title,omitempty"`
	Standing string            `json:"standing,omitempty"`
	Changed  time.Time         `json:"changed"`
	Fields   map[string]string `json:"fields"`
}

// thingsBuiltin is things(kind, unchanged_for = "5d", field = value or
// [values], …): the things of a plugin's kind ws keeps, those whose
// fields are as asked (a value, a pattern, or a list of either), and,
// with unchanged_for, those that haven't changed in so long. Each is
// .ref, .kind, .title, .standing, .changed, .idle_days (how many days
// since it changed) and .fields.
func (r *run) thingsBuiltin(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(`things takes its kind first, as things("jira:issue", status = ["In Review", "Blocked"], unchanged_for = "5d")`)
	}
	kind, ok := starlark.AsString(args[0])
	if !ok || !strings.Contains(kind, ":") {
		return nil, fmt.Errorf(`things: its kind is a plugin's, as "jira:issue"`)
	}
	var idle time.Duration
	want := map[string][]string{}
	for _, kv := range kwargs {
		key, _ := starlark.AsString(kv[0])
		if key == "unchanged_for" {
			s, _ := starlark.AsString(kv[1])
			d, err := lengthOf(s)
			if err != nil {
				return nil, fmt.Errorf("things: unchanged_for: %v", err)
			}
			idle = d
			continue
		}
		switch v := kv[1].(type) {
		case starlark.String:
			want[key] = []string{string(v)}
		case *starlark.List, starlark.Tuple:
			iter := starlark.Iterate(v)
			var x starlark.Value
			for iter.Next(&x) {
				s, ok := starlark.AsString(x)
				if !ok {
					iter.Done()
					return nil, fmt.Errorf("things: %s's values are strings", key)
				}
				want[key] = append(want[key], s)
			}
			iter.Done()
		default:
			return nil, fmt.Errorf("things: %s is a %s: a string, or a list of them", key, kv[1].Type())
		}
	}
	got, err := r.read("things", func() (any, error) {
		out := []map[string]any{}
		if r.e.Hooks.Things == nil {
			return out, nil
		}
		list, err := r.e.Hooks.Things(kind)
		if err != nil {
			return nil, err
		}
		now := r.e.now()
		for _, t := range list {
			if idle > 0 && (t.Changed.IsZero() || now.Sub(t.Changed) < idle) {
				continue
			}
			if !fieldsAre(t, want) {
				continue
			}
			fields := map[string]any{}
			for k, v := range t.Fields {
				fields[k] = v
			}
			days := 0.0
			if !t.Changed.IsZero() {
				days = float64(int(now.Sub(t.Changed).Hours()/24*10)) / 10
			}
			out = append(out, map[string]any{"ref": t.Ref, "kind": t.Kind, "title": t.Title, "standing": t.Standing, "changed": t.Changed.UTC().Format(time.RFC3339), "idle_days": days, "fields": fields})
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	if got == nil {
		return starlark.NewList(nil), nil
	}
	return valueOf(got), nil
}

// fieldsAre is whether t's fields are as want asks: each any of its
// values (a pattern), its case aside.
func fieldsAre(t ThingInfo, want map[string][]string) bool {
	for key, values := range want {
		have := ""
		for k, v := range t.Fields {
			if strings.EqualFold(k, key) {
				have = v
			}
		}
		matched := false
		for _, v := range values {
			if like(v, have) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
