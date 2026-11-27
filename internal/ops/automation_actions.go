package ops

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/ref"
)

// Your automations' actions (when = action(...)), as Find and a row's menu
// offer them: those on nothing in particular, or those on a thing, as it
// stands now; and one run, by you, waited on a moment so what it did can
// be said where you ran it.

// AutomationAction is one of your automations' actions, as it's offered:
// its automation, what that's for, and the action (its title, what it's
// on, which of them, and what it asks before it runs).
type AutomationAction struct {
	Automation string `json:"automation"`
	About      string `json:"about,omitempty"`
	automation.Action
}

// AutomationActions are your automations' actions that are on, for
// on: those on nothing in particular for "", else those for the thing it
// names, as it stands. A thing ws can't say the kind of has none.
func (w *Workspace) AutomationActions(on string) ([]AutomationAction, error) {
	kind, fields := "", map[string]string(nil)
	if on = strings.TrimSpace(on); on != "" {
		var ok bool
		if kind, fields, ok = w.thingKindOf(on); !ok {
			return []AutomationAction{}, nil
		}
	}
	states, err := automation.States(w.Root())
	if err != nil {
		return nil, err
	}
	list, _ := automation.List(w.Root())
	out := []AutomationAction{}
	for _, a := range list {
		if !states[a.Name].On {
			continue
		}
		for _, x := range a.Actions() {
			if x.Applies(kind, fields) {
				out = append(out, AutomationAction{Automation: a.Name, About: a.About, Action: x})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}

// AutomationActionRun is an action run, as it went by the time it's said:
// the event it's run on, its run's state (running, still, if it's taking
// a while), what it did, why it failed, and what it logged.
type AutomationActionRun struct {
	Event string          `json:"event"`
	State string          `json:"state"`
	Did   []AutomationDid `json:"did,omitempty"`
	Error string          `json:"error,omitempty"`
	Log   []string        `json:"log,omitempty"`
}

// actionWaits is how long running an action waits to say how it went.
const actionWaits = 8 * time.Second

// RunAutomationAction runs name's action title, by you, on what on names
// ("" for nothing) with input: refused if it's off, isn't for that, or
// input isn't what it asks. The daemon runs it at once; this says how it
// went, as soon as it's ended or parked, or after a few seconds.
func (w *Workspace) RunAutomationAction(name, title, on string, input map[string]any) (AutomationActionRun, error) {
	if !automation.ValidName(name) {
		return AutomationActionRun{}, automation.ErrNotFound
	}
	a, err := automation.Load(w.Root(), name)
	if err != nil {
		return AutomationActionRun{}, automation.ErrNotFound
	}
	if st, _ := automation.States(w.Root()); !st[name].On {
		return AutomationActionRun{}, &Invalid{Reason: name + " is off: turn on to run actions"}
	}
	x, ok := a.ActionNamed(title)
	if !ok {
		return AutomationActionRun{}, &Invalid{Reason: fmt.Sprintf("%s has no action called %q", name, title)}
	}
	kind, fields := "", map[string]string(nil)
	if on = strings.TrimSpace(on); on != "" {
		if kind, fields, ok = w.thingKindOf(on); !ok {
			return AutomationActionRun{}, &Invalid{Reason: "could not tell what " + on + " is"}
		}
	}
	if why := x.WhyNot(kind, fields); why != "" {
		return AutomationActionRun{}, &Invalid{Reason: fmt.Sprintf("%q does not apply: %s", title, why)}
	}
	in, err := x.ReadInput(input)
	if err != nil {
		return AutomationActionRun{}, &Invalid{Reason: err.Error()}
	}
	ev, err := automation.RunAction(w.Root(), name, title, on, in)
	if err != nil {
		return AutomationActionRun{}, err
	}
	out := AutomationActionRun{Event: ev.Ref(), State: automation.Running}
	deadline := time.Now().Add(actionWaits)
	for time.Now().Before(deadline) {
		runs, err := automationRuns(w.Root(), name, 10)
		if err != nil {
			return out, err
		}
		for _, r := range runs {
			if r.Event == ev.Ref() && r.State != automation.Running {
				return AutomationActionRun{Event: r.Event, State: r.State, Did: r.Did, Error: r.Error, Log: r.Log}, nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return out, nil
}

// thingKindOf is the kind of thing a ref names, as an action's on says
// it (pr, mission, conversation, artifact, repo, or a plugin's kind:
// jira:issue), and its fields as they stand: false for one ws can't say.
func (w *Workspace) thingKindOf(name string) (string, map[string]string, bool) {
	if known := w.namesKnown(); known != nil {
		for _, f := range known.Find(name) {
			if f.Ref.Kind != ref.Thing || f.Ref.Name() != name {
				continue
			}
			kind := f.Ref.Plugin + ":" + f.Ref.What
			fields := map[string]string{"ref": name}
			if rec, ok, _ := plugin.ThingOf(w.Root(), f.Ref.Plugin, f.Ref.What, name); ok {
				for k, v := range rec.Fields {
					fields[k] = v
				}
				if rec.Standing != "" {
					fields["standing"] = rec.Standing
				}
			}
			return kind, fields, true
		}
	}
	if slices.Contains(w.WS().RepoNames, name) {
		return "repo", map[string]string{"repo": name, "group": w.Config().Repos[name].Group}, true
	}
	r, ok := ref.Parse(name)
	if !ok {
		return "", nil, false
	}
	switch r.Kind {
	case ref.PR:
		fields := map[string]string{"repo": r.Repo, "number": strconv.Itoa(r.Number)}
		if rec, err := pulls.Load(w.Root(), r.Repo, r.Number); err == nil && rec != nil {
			fields["title"] = rec.Title
			fields["branch"] = rec.Branch
		}
		return "pr", fields, true
	case ref.Mission:
		m, err := w.Missions.Load(r.Slug)
		if err != nil {
			return "", nil, false
		}
		// Where it stands, as kept: parked, archived or how it ended; to
		// brief; else active. Not what its capsules and pull requests would
		// say (review, landed): that's git and GitHub, which a menu
		// doesn't wait on.
		state := string(m.State)
		switch {
		case state != "":
		case m.Ending != nil && m.Ending.Ending != "":
			state = string(m.Ending.Ending)
		case m.ToBrief():
			state = string(mission.Briefing)
		default:
			state = string(mission.Active)
		}
		return "mission", map[string]string{"slug": m.Slug, "title": m.Title, "state": state}, true
	case ref.Talk:
		c, err := conversation.ForWorkspace(w.Root()).Load(r.ID)
		if err != nil {
			return "", nil, false
		}
		return "conversation", map[string]string{"id": c.ID, "title": c.Title, "status": c.Status, "via": c.Via}, true
	case ref.Artifact:
		return "artifact", map[string]string{"id": r.ID}, true
	}
	return "", nil, false
}
