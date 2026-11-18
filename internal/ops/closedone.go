package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/mission"
)

// Crew who've reported done are closed: their session is hung up, so an
// agent's process (and its memory) isn't held for as long as the daemon
// runs. Nothing else would close it but new crew starting in its capsule
// (endHandedBack), which a mission across several capsules may never do.
// They're handed back (mission.EndedHandedBack), and resumable: anything
// said to them after (you on their page, a failing check, the
// orchestrator's resume-with) resumes them with it, as Say does. A mission
// that ends or is archived closes all its crew.

// CloseDoneAfter is how long crew who've reported done are left open once
// their turn has ended: what follows a done (its checks, the orchestrator's
// look) starts within it, and may send them back while they're open.
const CloseDoneAfter = 2 * time.Minute

// CloseDone closes the sessions of crew on missions in play who reported
// done and whose turn ended CloseDoneAfter or more before now, while ws
// isn't checking or orchestrating their mission. It says what it closed.
func (w *Workspace) CloseDone(now time.Time) []string {
	if w.opts.Host == nil {
		return nil
	}
	missions, err := w.Missions.InPlay()
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range missions {
		var ids []string
		for _, c := range m.Crew {
			if closable(m, c, now) {
				ids = append(ids, c.ID)
			}
		}
		if said := w.closeCrew(m, ids, "it had reported done and finished its turn (saying anything to it resumes it)"); said != "" {
			out = append(out, m.Slug+": "+said)
		}
	}
	return out
}

// closable is whether crew c can be closed for having reported done, at
// now: work crew, still open, done and waiting since CloseDoneAfter, with
// no work of theirs left running, on a mission ws isn't checking or
// orchestrating (either may send them back).
func closable(m *mission.Mission, c mission.Crew, now time.Time) bool {
	switch {
	case c.Ended != nil, c.Role != "", c.Status != mission.CrewDone:
		return false
	case c.Activity != mission.ActivityWaiting, c.ActivityAt == nil, now.Sub(*c.ActivityAt) < CloseDoneAfter:
		return false
	case c.Pending != nil:
		return false
	}
	return m.Orchestrating == nil && m.Checking == nil && m.Launching == nil
}

// closeEnded closes every crew session still open on mission m, which has
// ended or is being archived: there's nothing left for them to do on it.
func (w *Workspace) closeEnded(m *mission.Mission, why string) {
	var ids []string
	for _, c := range m.Crew {
		if c.Ended == nil {
			ids = append(ids, c.ID)
		}
	}
	w.closeCrew(m, ids, why)
}

// closeCrew hangs up crew ids on mission m, and logs it with why. It says
// whom it closed ("" for none). Crew the daemon doesn't hold are left as
// they are: their process, if there is one, isn't its to end.
func (w *Workspace) closeCrew(m *mission.Mission, ids []string, why string) string {
	if w.opts.Host == nil || len(ids) == 0 {
		return ""
	}
	var closed []string
	for _, id := range ids {
		if err := w.opts.Host.Stop(m.Slug + "/" + id); err == nil {
			closed = append(closed, id)
		}
	}
	if len(closed) == 0 {
		return ""
	}
	who := strings.Join(closed, ", ")
	text := fmt.Sprintf("closed %s's session: %s", who, why)
	w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventNote, Text: text}) //nolint:errcheck // a note
	execlog.Note(w.Root(), "crew "+m.Slug, "%s", text)
	return "closed " + who
}
