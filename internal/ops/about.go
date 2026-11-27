package ops

import (
	"context"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/ref"
)

// What a mission is about: things ws names that aren't its own work (a
// plugin's issue it was started for, a pull request it follows up). The
// mission's record says so, by name, and the graph has it from there: so
// what's about RAM-123 is found from RAM-123, and whoever reads the
// mission is told to read the issue.

// AboutNames is names as a mission keeps what it's about: each as ws
// writes it. One ws reads nothing by is refused: a mission isn't about a
// word.
func (w *Workspace) AboutNames(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	k := w.RefsKnown(context.Background(), false)
	var out []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		r, ok := k.At(n)
		if !ok {
			// One of a plugin's things that ws found in a space is named by
			// its folder (records/sketches/release-flow/).
			r, _, ok = w.ThingNamed(n)
		}
		if !ok {
			return nil, &Invalid{Reason: "nothing called " + n + ": a mission can be about a plugin thing (see ws plugin list) or a pull request (demo#12)"}
		}
		// A file's lines aren't what a mission's about: the file is.
		r.Line, r.EndLine, r.Commit = 0, 0, ""
		if name := r.Name(); !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out, nil
}

// SetAbout changes what a mission is about: add to it, and take from it,
// each by a name ws reads. It's the mission's record that's changed, and
// the graph follows.
func (w *Workspace) SetAbout(slug string, add, remove []string) (*mission.Mission, error) {
	add, err := w.AboutNames(add)
	if err != nil {
		return nil, err
	}
	// What's taken from it is as it was written, or as ws writes it.
	gone := slices.Clone(remove)
	if names, err := w.AboutNames(remove); err == nil {
		gone = append(gone, names...)
	}
	return w.Missions.Update(slug, func(m *mission.Mission) error {
		m.About = slices.DeleteFunc(m.About, func(n string) bool { return slices.Contains(gone, n) })
		for _, n := range add {
			if !slices.Contains(m.About, n) {
				m.About = append(m.About, n)
			}
		}
		return nil
	})
}

// StartAbout makes a mission about something ws names, to brief: asked
// for as the thing's name and what it's called, where ws has been told
// (a plugin's thing), so whoever briefs it starts from what it's about.
func (w *Workspace) StartAbout(name string) (*mission.Mission, error) {
	names, err := w.AboutNames([]string{name})
	if err != nil {
		return nil, err
	}
	request := names[0]
	if r, kept, ok := w.ThingNamed(names[0]); ok && r.Kind == ref.Thing {
		if rec, told, _ := plugin.ThingOf(w.WS().Root, r.Plugin, r.What, r.ID); told {
			if title := rec.Fields[kept.Typed("title")]; title != "" {
				request += ": " + title
			}
		}
	}
	m, err := w.Missions.CreateBriefing(request)
	if err != nil {
		return nil, err
	}
	return w.SetAbout(m.Slug, names, nil)
}
