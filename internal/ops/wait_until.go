package ops

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/plugin"
)

// A Wait step's until (mission.Check.Until) is an event ws hears of: a
// pull request merging, a Jira issue moving to Done, commits landing, a
// run ending. It's met by one ws has kept since the mission reached the
// step, or, at the step's first look, by how things stand already (the
// pull request merged before the mission got there). Nothing's run for it.
//
// The daemon looks at a step as soon as an event it waits on is kept
// (CheckWaitsHeard), not at its next look; and while one waits on a pull
// request, keeps the Reviews list read as often as it would be for an
// open Reviews page, so what moves there is heard.

// untilResult is what a Wait's until comes to now.
func (w *Workspace) untilResult(st mission.StageState, c mission.Check) mission.CheckResult {
	r := mission.CheckResult{Check: c.Text, By: "ws"}
	match, err := events.ParseMatch(c.Until)
	if err != nil {
		r.Result, r.Evidence = mission.CheckCantTell, err.Error()
		return r
	}
	var since time.Time
	first := true
	if watch := st.Watch(); watch != nil {
		since, first = watch.Since, watch.Looks == 0
	}
	list, err := events.List(w.Root(), events.Filter{Since: since, Kinds: []string{match.Kind}})
	if err != nil {
		r.Result, r.Evidence = mission.CheckCantTell, "could not read events: "+err.Error()
		return r
	}
	for _, e := range list {
		if match.Matches(e) {
			r.Result, r.Evidence = mission.CheckMet, heardLine(e)
			return r
		}
	}
	if first {
		if stands, why := w.untilStands(match); stands {
			r.Result, r.Evidence = mission.CheckMet, why
			return r
		}
	}
	r.Result, r.Evidence = mission.CheckUnmet, "not heard since the step began"
	return r
}

// heardLine is an event as the evidence that met an until: its ref, what
// it was, by whom.
func heardLine(e events.Event) string {
	what := strings.Join(e.Refs(), " ")
	if e.Title != "" {
		what += " (" + e.Title + ")"
	}
	return fmt.Sprintf("%s: %s %s, by %s, %s", e.Ref(), e.Kind, what, e.By, e.At.Local().Format("Jan 2 15:04"))
}

// untilStands is whether what match waits on is so already, as ws keeps
// it, and what shows it: a pull request merged (GitHub's asked), a
// plugin's thing with the fields asked. false for what can only be heard
// as it happens.
func (w *Workspace) untilStands(match events.Match) (bool, string) {
	field := func(key string) string {
		for _, f := range match.Fields {
			if f.Key == key {
				return f.Value
			}
		}
		return ""
	}
	switch match.Kind {
	case events.PRMerged, events.PRClosed:
		repo, n, ok := strings.Cut(field("pr"), "#")
		number, err := strconv.Atoi(n)
		if !ok || err != nil || w.GitHub() == nil {
			return false, ""
		}
		remote := w.WS().RemoteOf(repo)
		pr, err := w.GitHub().PRFromNumber(remote.Owner, remote.Name, number)
		if err != nil || pr == nil {
			return false, ""
		}
		merged := strings.EqualFold(pr.State, "MERGED") || pr.MergedAt != ""
		if (match.Kind == events.PRMerged) == merged && (merged || strings.EqualFold(pr.State, "CLOSED")) {
			return true, fmt.Sprintf("%s#%d already %s when the step began", repo, number, strings.ToLower(pr.State))
		}
	case events.ThingChanged:
		kind, ref := field("kind"), field("thing")
		name, kindName, ok := strings.Cut(kind, ":")
		if ref == "" || !ok {
			return false, ""
		}
		rec, found, err := plugin.ThingOf(w.Root(), name, kindName, ref)
		if err != nil || !found {
			return false, ""
		}
		var asked []string
		for _, f := range match.Fields {
			switch f.Key {
			case "kind", "thing", "by", "origin":
				continue
			}
			got, ok := rec.Fields[f.Key]
			if !ok || !strings.EqualFold(got, f.Value) {
				return false, ""
			}
			asked = append(asked, f.Key+" "+got)
		}
		if len(asked) > 0 {
			return true, fmt.Sprintf("%s's %s already when the step began", ref, strings.Join(asked, ", "))
		}
	}
	return false, ""
}

// heardUntils is the untils of a step, read.
func heardUntils(st mission.StageState) []events.Match {
	var out []events.Match
	for _, c := range st.Step.Checks {
		if c.Heard() {
			if m, err := events.ParseMatch(c.Until); err == nil {
				out = append(out, m)
			}
		}
	}
	return out
}

// CheckWaitsHeard looks at each mission in play at a Wait step that
// waits on one of the events kept after the one numbered after, and
// passes those whose checks are now all met. The daemon calls it as
// events are kept. It returns what it did, a line each, and the latest
// event it's looked past.
func (w *Workspace) CheckWaitsHeard(after int64) (said []string, latest int64) {
	list, err := events.List(w.Root(), events.Filter{After: after})
	if err != nil || len(list) == 0 {
		return nil, after
	}
	latest = list[len(list)-1].N
	missions, err := w.Missions.InPlay()
	if err != nil {
		return nil, latest
	}
	for _, m := range missions {
		if !Watched(m) || m.ToBrief() {
			continue
		}
		cur, ok := currentStep(w.Missions, m)
		if !ok || !cur.Step.Watched() || cur.Held() {
			continue
		}
		heard := false
		for _, match := range heardUntils(cur) {
			for _, e := range list {
				heard = heard || match.Matches(e)
			}
		}
		if !heard {
			continue
		}
		if _, met := w.lookAtWait(m, cur); met {
			if s := w.passWatched(m.Slug, cur); s != "" {
				said = append(said, m.Slug+": "+s)
			}
		}
	}
	return said, latest
}

// LatestEvent is the number of the latest event kept: 0 for none.
func (w *Workspace) LatestEvent() int64 {
	list, err := events.List(w.Root(), events.Filter{Limit: 1})
	if err != nil || len(list) == 0 {
		return 0
	}
	return list[0].N
}

// freshenForWaits has the Reviews list read (as often as it's read for an
// open Reviews page) where a mission waits on a pull request's move ws
// hears of only from it.
func (w *Workspace) freshenForWaits(missions []*mission.Mission) {
	svc := w.PRService()
	if svc == nil {
		return
	}
	for _, m := range missions {
		cur, ok := currentStep(w.Missions, m)
		if !ok || cur.Step.Type != mission.StepWait || cur.Held() {
			continue
		}
		for _, match := range heardUntils(cur) {
			if strings.HasPrefix(match.Kind, "pr.") {
				svc.List(false) //nolint:errcheck // a list that can't be read is read again at the next look
				return
			}
		}
	}
}
