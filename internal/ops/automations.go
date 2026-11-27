package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/report"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/workspace"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/notification"
)

// What your automations do is ws's own operations (internal/automation's
// hooks): a notification in your feed, from the automations source; a
// conversation of the workspace's, started with what it's to look at; the
// refs a text names, as ws finds them. ws show's views are the commands',
// and the daemon gives them (show is left to it).

// AutomationHooks are the operations an automation's builtins are, but
// show.
func (w *Workspace) AutomationHooks() automation.Hooks {
	return automation.Hooks{
		Notify: func(a *automation.Automation, text string, refs []string) error {
			_, err := notification.Add(w.Root(), notification.Notification{
				Source: notification.SourceAutomations, Kind: "automation:" + a.Name, Text: text, Refs: refs, By: events.ByWorkflow(a.Name),
			})
			return err
		},
		Failed: func(a *automation.Automation, why string) error {
			_, err := notification.Add(w.Root(), notification.Notification{
				Source: notification.SourceAutomations, Kind: "automation:" + a.Name, Attention: true,
				Text: "Automation " + a.Name + " could not run: " + why,
			})
			return err
		},
		Converse: func(a *automation.Automation, text string, about, chain []string) (string, error) {
			if len(about) > 0 {
				text += "\n\n(About " + strings.Join(about, ", ") + ".)"
			}
			c, err := w.StartTalkChained("automation:"+a.Name, text, chain)
			if err != nil {
				return "", err
			}
			return c.ID, nil
		},
		RunTask: func(a *automation.Automation, repo, recipe string, chain []string) (string, int, error) {
			return w.runAutomationTask(a.Name, repo, recipe, chain)
		},
		Ask:      w.automationAsk,
		Spent:    w.automationSpent,
		Propose:  w.automationPropose,
		Act:      w.automationAct,
		Proposal: w.automationProposalState,
		Report:   w.runReports,
		Repos:    w.automationRepos,
		Things:   w.automationThings,
		RefsIn: func(text string) [][2]string {
			known := w.namesKnown()
			if known == nil {
				return nil
			}
			var out [][2]string
			seen := map[string]bool{}
			for _, f := range known.Find(text) {
				name := f.Ref.Name()
				if seen[name] {
					continue
				}
				seen[name] = true
				kind := string(f.Ref.Kind)
				if f.Ref.Plugin != "" {
					kind = f.Ref.Plugin + ":" + f.Ref.What
				}
				out = append(out, [2]string{name, kind})
			}
			return out
		},
	}
}

// runAutomationTask starts a repo's task on its ground (its default
// branch, as it stands), for an automation: by workflow:<name>, its
// events carrying chain. A task that's running there already is that one.
func (w *Workspace) runAutomationTask(name, repo, recipe string, chain []string) (string, int, error) {
	if !w.recipes(repo, workspace.GroundDir)[recipe].IsTask() {
		return "", 0, fmt.Errorf("%s has no task %s on its ground", repo, recipe)
	}
	dir, err := w.worktreeOf(repo, workspace.GroundDir)
	if err != nil {
		return "", 0, err
	}
	r, err := w.runUp(RunAsk{
		Repo: repo, Worktree: workspace.GroundDir, Dir: dir, Recipe: recipe,
		Owner: runs.Owner{Kind: runs.ByNone}, TTL: runs.DefaultTTL,
		By: events.ByWorkflow(name), Chain: chain,
	})
	if err != nil {
		return "", 0, err
	}
	return r.Ref(), r.N, nil
}

// runReports is what a task's run's reports say, each by its format
// (internal/report), or why it can't be said.
func (w *Workspace) runReports(ref string, n int) map[string]map[string]any {
	r, err := w.runByRef(fmt.Sprintf("%s#%d", ref, n))
	if err != nil {
		return nil
	}
	out := map[string]map[string]any{}
	for _, rp := range r.Reports {
		s, err := report.Summary(rp.Format, filepath.Join(r.Path, "reports", rp.Name))
		if err != nil {
			s = map[string]any{"error": err.Error()}
		}
		out[rp.Name] = s
	}
	return out
}

// automationRepos is the workspace's repos as repos() gives them: each
// with its group and the tasks on its ground, with their reports.
func (w *Workspace) automationRepos() []automation.RepoInfo {
	ws, cfg := w.WS(), w.Config()
	var out []automation.RepoInfo
	for _, name := range ws.RepoNames {
		rc := cfg.Repos[name]
		info := automation.RepoInfo{Name: name, Group: rc.Group, Display: rc.DisplayName, Branch: firstNonEmpty(ws.DefaultBranch, "main"), Tasks: []automation.TaskInfo{}}
		tasks := w.runTables(name, workspace.GroundDir).Task
		for _, t := range slices.Sorted(maps.Keys(tasks)) {
			ti := automation.TaskInfo{Name: t}
			for rn, rp := range tasks[t].Reports {
				if ti.Reports == nil {
					ti.Reports = map[string]string{}
				}
				ti.Reports[rn] = rp.Format
			}
			info.Tasks = append(info.Tasks, ti)
		}
		out = append(out, info)
	}
	return out
}

// automationThings are a plugin's things of a kind (jira:issue), as
// things() gives them.
func (w *Workspace) automationThings(kind string) ([]automation.ThingInfo, error) {
	name, what, ok := strings.Cut(kind, ":")
	if !ok {
		return nil, &Invalid{Reason: kind + " is not a plugin kind (as jira:issue)"}
	}
	list, titled, err := plugin.Things(w.Root(), name, what)
	if err != nil {
		return nil, err
	}
	out := []automation.ThingInfo{}
	for _, r := range list {
		out = append(out, automation.ThingInfo{Ref: r.Ref, Kind: kind, Title: r.Fields[titled], Standing: r.Standing, Changed: r.Changed, Fields: r.Fields})
	}
	return out, nil
}

// acceptAutomation adds a proposed automation, on, from the next event:
// its name is the proposal's result.
func (w *Workspace) acceptAutomation(store conversation.Store, c *conversation.Conversation, p conversation.Proposal) (string, error) {
	if p.Automation == nil {
		return "", &Invalid{Reason: "the proposal has no automation"}
	}
	a, err := (&automation.Engine{Root: w.Root()}).Add(p.Automation.Name, []byte(p.Text))
	if err != nil {
		return "", &Invalid{Reason: "could not add automation: " + err.Error()}
	}
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, a.Name, conversation.You, time.Now()); err != nil {
		return "", err
	}
	return a.Name, nil
}

// The Automations page: each of yours, how it stands and what its runs
// did, turned on or off, taken away, and dry-run again. The same as ws
// automation list, runs, on, off, rm and add --propose say.

// AutomationItem is an automation as the Automations page lists it: what
// it's for (in your words), when it runs and what it may do; whether it's
// on, what it last failed with, or why its file can't be read (broken);
// how its runs stand, and its latest.
type AutomationItem struct {
	Name   string           `json:"name"`
	About  string           `json:"about,omitempty"`
	When   string           `json:"when,omitempty"`
	May    []string         `json:"may"`
	On     bool             `json:"on"`
	Added  time.Time        `json:"added"`
	Failed string           `json:"failed,omitempty"`
	Broken string           `json:"broken,omitempty"`
	Tally  automation.Tally `json:"tally"`
	Last   *AutomationRun   `json:"last,omitempty"`
	// Output is the version of its output that's shown (0 for none), and
	// Scheduled whether it runs by the clock (every()), which can be run
	// now.
	Output    int  `json:"output,omitempty"`
	Scheduled bool `json:"scheduled,omitempty"`
	// Kinds are the plugins' kinds of proposal it may act on (act), each
	// with whether you let it do that without asking (granted).
	Kinds []AutomationKind `json:"kinds,omitempty"`
}

// AutomationKind is a plugin's kind of proposal an automation may act on:
// what it's called, whether you let this automation do it without asking
// (your call), and what its plugin advises: that it's safe done with
// nobody looking (unattended), or why it might not be (caution). Missing:
// no plugin here proposes it.
type AutomationKind struct {
	Kind       string `json:"kind"`
	Title      string `json:"title,omitempty"`
	Unattended bool   `json:"unattended,omitempty"`
	Caution    string `json:"caution,omitempty"`
	Granted    bool   `json:"granted,omitempty"`
	Missing    bool   `json:"missing,omitempty"`
}

// automationKinds are a's plugin kinds, as they stand.
func automationKinds(root string, a *automation.Automation) []AutomationKind {
	kinds := a.PluginKinds()
	if len(kinds) == 0 {
		return nil
	}
	grants, _ := automation.Grants(root, a.Name)
	byName := map[string]plugin.Proposable{}
	for _, p := range plugin.Proposables(root) {
		byName[p.Name()] = p
	}
	out := []AutomationKind{}
	for _, k := range kinds {
		ak := AutomationKind{Kind: k, Granted: slices.Contains(grants, k)}
		if p, ok := byName[k]; ok {
			ak.Title, ak.Unattended, ak.Caution = p.Title, p.Proposes.Unattended, p.Proposes.Caution
		} else {
			ak.Missing = true
		}
		out = append(out, ak)
	}
	return out
}

// SetAutomationGrant lets one of yours do a plugin's kind without asking
// (on), or no longer.
func (w *Workspace) SetAutomationGrant(name, kind string, on bool) error {
	if !automation.ValidName(name) {
		return automation.ErrNotFound
	}
	if err := automation.Grant(w.Root(), name, kind, on, w.now()); err != nil {
		if errors.Is(err, automation.ErrNotFound) {
			return err
		}
		return &Invalid{Reason: err.Error()}
	}
	return nil
}

// AutomationRun is one run of an automation: the event it ran on (its
// ref, what it says, when), how it went (ran, failed, unsure, waiting,
// held, running) and why it failed, its per key, the lines it logged,
// and what it did.
type AutomationRun struct {
	N     int64           `json:"n"`
	Event string          `json:"event"`
	Says  string          `json:"says,omitempty"`
	At    time.Time       `json:"at"`
	State string          `json:"state"`
	Error string          `json:"error,omitempty"`
	Key   string          `json:"key,omitempty"`
	Log   []string        `json:"log,omitempty"`
	Did   []AutomationDid `json:"did,omitempty"`
}

// AutomationDid is a step a run took that reached outside it: a
// notification (its text), a conversation started (its id), a wait (what
// for, or the event it got), or a read (show, events, refs_in). Started
// is a step begun and never said done.
type AutomationDid struct {
	What         string `json:"what"`
	Text         string `json:"text,omitempty"`
	Conversation string `json:"conversation,omitempty"`
	Started      bool   `json:"started,omitempty"`
	Waiting      bool   `json:"waiting,omitempty"`
	// Proposal is what a propose or an act made (w12#1), and Settled how it
	// stands now: open, accepted, declined.
	Proposal string `json:"proposal,omitempty"`
	Settled  string `json:"settled,omitempty"`
}

// AutomationDetail is one automation on its page: as listed, its script,
// and its latest runs.
type AutomationDetail struct {
	Automation AutomationItem  `json:"automation"`
	Source     string          `json:"source"`
	Runs       []AutomationRun `json:"runs"`
	// Outline is its script, drawn (nil where it can't be read), with how
	// each step's gone over its runs (Steps) and where they failed.
	Outline  *automation.Outline    `json:"outline,omitempty"`
	Steps    []automation.StepCount `json:"steps"`
	Failures map[string]int         `json:"failures,omitempty"`
}

// automationRunsShown is the most runs an automation's page shows.
const automationRunsShown = 50

// Automations is yours, by name.
func (w *Workspace) Automations() ([]AutomationItem, error) {
	root := w.Root()
	list, bad := automation.List(root)
	states, err := automation.States(root)
	if err != nil {
		return nil, err
	}
	tallies, err := automation.Tallies(root)
	if err != nil {
		return nil, err
	}
	out := []AutomationItem{}
	for _, a := range list {
		out = append(out, automationItem(root, a, states[a.Name], tallies[a.Name]))
	}
	for name, err := range bad {
		st := states[name]
		out = append(out, AutomationItem{Name: name, May: []string{}, On: st.On, Added: st.Added, Broken: err.Error(), Tally: tallies[name]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func automationItem(root string, a *automation.Automation, st automation.State, t automation.Tally) AutomationItem {
	item := AutomationItem{Name: a.Name, About: a.About, When: a.WhenSaid(), May: append([]string{}, a.May...), On: st.On, Added: st.Added, Failed: st.Failed, Tally: t,
		Output: automation.LatestOutput(root, a.Name), Scheduled: len(a.Schedules()) > 0, Kinds: automationKinds(root, a)}
	if runs, _ := automationRuns(root, a.Name, 1); len(runs) > 0 {
		item.Last = &runs[0]
	}
	return item
}

// Automation is one of yours, with its script and latest runs.
func (w *Workspace) Automation(name string) (AutomationDetail, error) {
	root := w.Root()
	if !automation.ValidName(name) {
		return AutomationDetail{}, automation.ErrNotFound
	}
	src, err := os.ReadFile(automation.Path(root, name))
	if err != nil {
		return AutomationDetail{}, automation.ErrNotFound
	}
	states, err := automation.States(root)
	if err != nil {
		return AutomationDetail{}, err
	}
	tallies, err := automation.Tallies(root)
	if err != nil {
		return AutomationDetail{}, err
	}
	st := states[name]
	var item AutomationItem
	var outline *automation.Outline
	if a, err := automation.Load(root, name); err != nil {
		item = AutomationItem{Name: name, May: []string{}, On: st.On, Added: st.Added, Broken: err.Error(), Tally: tallies[name]}
	} else {
		item = automationItem(root, a, st, tallies[name])
		if o, err := automation.OutlineOf(a); err == nil {
			outline = &o
		}
	}
	runs, err := automationRuns(root, name, automationRunsShown)
	if err != nil {
		return AutomationDetail{}, err
	}
	steps, failures, err := automation.Counts(root, name)
	if err != nil {
		return AutomationDetail{}, err
	}
	return AutomationDetail{Automation: item, Source: string(src), Runs: runs, Outline: outline, Steps: steps, Failures: failures}, nil
}

// automationRuns is name's latest runs, at most limit, each with the event
// it ran on and what it did.
func automationRuns(root, name string, limit int) ([]AutomationRun, error) {
	runs, err := automation.Runs(root, name, limit)
	if err != nil {
		return nil, err
	}
	out := []AutomationRun{}
	if len(runs) == 0 {
		return out, nil
	}
	var ns []int64
	for _, r := range runs {
		ns = append(ns, r.Event)
	}
	evs := map[int64]events.Event{}
	for _, ev := range automation.EventsNumbered(root, ns) {
		evs[ev.N] = ev
	}
	steps, err := automation.StepsOf(root, name, ns)
	if err != nil {
		return nil, err
	}
	for _, r := range runs {
		ar := AutomationRun{N: r.N, Event: "e" + strconv.FormatInt(r.Event, 10), At: r.At, State: r.State, Error: r.Error, Key: r.Key}
		if ev, ok := evs[r.Event]; ok {
			ar.Event, ar.Says = ev.Ref(), ev.Says()
		}
		if r.Log != "" {
			ar.Log = strings.Split(r.Log, "\n")
		}
		for _, st := range steps[r.Event] {
			d := didOf(st)
			if (d.What == "propose" || d.What == "act") && !d.Started {
				d.Proposal = st.Result
				d.Text = firstNonEmpty(proposalSays(root, st.Result), d.Text)
				d.Settled = proposalState(root, st.Result)
			}
			ar.Did = append(ar.Did, d)
		}
		out = append(out, ar)
	}
	return out, nil
}

// didOf is a step as a run's page says it.
func didOf(st automation.Step) AutomationDid {
	d := AutomationDid{What: st.Builtin, Started: st.State == "started", Waiting: st.State == "waiting"}
	switch st.Builtin {
	case "notify":
		d.Text = st.Result
	case "converse":
		d.Conversation = st.Result
	case "propose", "act":
		d.Text = st.Result
		d.Conversation, _, _ = strings.Cut(st.Result, "#")
	case "ask":
		if d.Waiting {
			d.Text = "for an agent's answer"
			break
		}
		d.Text = answeredWith(st.Result)
	case "wait":
		if d.Waiting {
			var w struct {
				Match string `json:"match"`
				Items []struct {
					Match    string `json:"match"`
					Task     int64  `json:"task"`
					Proposal string `json:"proposal"`
				} `json:"items"`
				Due time.Time `json:"due"`
			}
			if json.Unmarshal([]byte(st.Result), &w) == nil {
				what := w.Match
				if what == "" {
					var each []string
					for _, it := range w.Items {
						switch {
						case it.Proposal != "":
							each = append(each, "your answer to "+it.Proposal)
						case it.Task != 0:
							each = append(each, "a task")
						default:
							each = append(each, it.Match)
						}
					}
					what = strings.Join(each, ", ")
				}
				d.Text = "for " + what + ", till " + w.Due.Local().Format("Jan 2 15:04")
			}
			break
		}
		d.Text = waitedFor(json.RawMessage(st.Result))
	}
	return d
}

// answeredWith is what an ask came to, in words: its answer, short, or
// why there's none.
func answeredWith(result string) string {
	var o struct {
		Answer json.RawMessage `json:"answer"`
		Why    string          `json:"why"`
	}
	if strings.TrimSpace(result) == "null" {
		return "no answer came in time"
	}
	if json.Unmarshal([]byte(result), &o) != nil {
		return ""
	}
	if s := strings.TrimSpace(string(o.Answer)); s == "" || s == "null" {
		return "no answer: " + o.Why
	}
	var words string
	if json.Unmarshal(o.Answer, &words) == nil {
		return clipLine(words, 300)
	}
	var fields map[string]any
	if json.Unmarshal(o.Answer, &fields) == nil {
		var said []string
		for _, k := range slices.Sorted(maps.Keys(fields)) {
			said = append(said, fmt.Sprintf("%s: %v", k, fields[k]))
		}
		return clipLine(strings.Join(said, " · "), 300)
	}
	return clipLine(string(o.Answer), 300)
}

// proposalState is how a proposal an automation made stands (open,
// accepted, declined…), by its ref: "" where it's gone.
func proposalState(root, ref string) string {
	id, nth, _ := strings.Cut(ref, "#")
	n, err := strconv.Atoi(nth)
	if err != nil {
		return ""
	}
	c, err := conversation.ForWorkspace(root).Load(id)
	if err != nil || n < 1 || n > len(c.Proposals) {
		return ""
	}
	return c.Proposals[n-1].State
}

// proposalSays is what a proposal an automation made is, in a line ("a
// mission: Build Queues"), by its ref (w12#1): "" where it's gone.
func proposalSays(root, ref string) string {
	id, nth, _ := strings.Cut(ref, "#")
	n, err := strconv.Atoi(nth)
	if err != nil {
		return ""
	}
	c, err := conversation.ForWorkspace(root).Load(id)
	if err != nil || n < 1 || n > len(c.Proposals) {
		return ""
	}
	p := c.Proposals[n-1]
	// A plugin's is its own title ("The note on THING-3, rewritten").
	return firstNonEmpty(p.Title, p.Says())
}

// waitedFor is what a wait came to, in words: an event (e12: what it
// says), a task (demo's coverage: passed), its time up, or each of a
// wait_all's.
func waitedFor(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return "timed out"
	}
	if strings.HasPrefix(s, "[") {
		var each []json.RawMessage
		if json.Unmarshal(raw, &each) != nil {
			return ""
		}
		var said []string
		for _, one := range each {
			said = append(said, waitedFor(one))
		}
		return strings.Join(said, "; ")
	}
	var task struct {
		Task   int64  `json:"task"`
		Repo   string `json:"repo"`
		Recipe string `json:"recipe"`
		Status string `json:"status"`
	}
	if json.Unmarshal(raw, &task) == nil && task.Task != 0 {
		return task.Repo + "'s " + task.Recipe + ": " + task.Status
	}
	var prop struct {
		Proposal string `json:"proposal"`
		State    string `json:"state"`
		Result   string `json:"result"`
	}
	if json.Unmarshal(raw, &prop) == nil && prop.Proposal != "" {
		said := "you " + prop.State + " " + prop.Proposal
		if prop.Result != "" {
			said += " (" + prop.Result + ")"
		}
		return said
	}
	var ev events.Event
	if json.Unmarshal(raw, &ev) == nil {
		return ev.Ref() + ": " + ev.Says()
	}
	return ""
}

// SetAutomationOn turns one of yours on (from the next event) or off.
func (w *Workspace) SetAutomationOn(name string, on bool) error {
	return automation.SetOn(w.Root(), name, on)
}

// RemoveAutomation takes one of yours away: its file, and its runs.
func (w *Workspace) RemoveAutomation(name string) error {
	if !automation.ValidName(name) {
		return automation.ErrNotFound
	}
	return (&automation.Engine{Root: w.Root()}).Remove(name)
}

// DryRunAutomation is what one of yours would have done over the last 30
// days, done again now: show is ws show's, where there's one.
func (w *Workspace) DryRunAutomation(name string, show func(ref string) (map[string]any, error)) (automation.DryRun, error) {
	if !automation.ValidName(name) {
		return automation.DryRun{}, automation.ErrNotFound
	}
	a, err := automation.Load(w.Root(), name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return automation.DryRun{}, automation.ErrNotFound
		}
		return automation.DryRun{}, &Invalid{Reason: err.Error()}
	}
	hooks := w.AutomationHooks()
	hooks.Show = show
	return (&automation.Engine{Root: w.Root(), Hooks: hooks}).DryRun(a, automationDryRunBack)
}

// automationDryRunBack is how far back a dry run looks.
const automationDryRunBack = 30 * 24 * time.Hour

// RunAutomationNow runs one of yours that's on a schedule now, as if its
// time had come.
func (w *Workspace) RunAutomationNow(name string) (events.Event, error) {
	if !automation.ValidName(name) {
		return events.Event{}, automation.ErrNotFound
	}
	ev, err := automation.RunNow(w.Root(), name)
	if errors.Is(err, automation.ErrNoSchedule) {
		return ev, &Invalid{Reason: err.Error()}
	}
	return ev, err
}

// AutomationOutput is a version of what one of yours shows (0 for the
// latest), and every version kept, the latest first, without their
// bodies.
func (w *Workspace) AutomationOutput(name string, version int) (automation.Output, []automation.Output, error) {
	if !automation.ValidName(name) {
		return automation.Output{}, nil, automation.ErrNotFound
	}
	shown, all, err := automation.Outputs(w.Root(), name, version)
	if errors.Is(err, automation.ErrNoOutput) {
		return shown, all, ErrNotFound
	}
	if all == nil {
		all = []automation.Output{}
	}
	return shown, all, err
}

// AutomationsStamp names where your automations stand, cheaply.
func (w *Workspace) AutomationsStamp() string {
	return automation.Stamp(w.Root())
}
