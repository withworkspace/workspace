package ops

import (
	"fmt"
	"strings"
	"sync"
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
		if said := w.closeCrew(m, ids, "reported done, turn finished (send a message to resume)"); said != "" {
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

// closings is the missions whose crew are being closed behind an answer.
type closings struct {
	mu sync.Mutex
	of map[string]chan struct{}
}

// closeEndedBehind is closeEnded behind the answer, where there's a daemon
// to do it in (an ACP crew's stop waits for it, seconds each): what needs
// them closed first (a capsule burned) waits for it (closedFor). A command
// closes them there and then.
func (w *Workspace) closeEndedBehind(m *mission.Mission, why string) {
	if w.opts.Outbox == nil {
		w.closeEnded(m, why)
		return
	}
	done := make(chan struct{})
	w.closing.mu.Lock()
	if w.closing.of == nil {
		w.closing.of = map[string]chan struct{}{}
	}
	was := w.closing.of[m.Slug]
	w.closing.of[m.Slug] = done
	w.closing.mu.Unlock()
	go func() {
		defer close(done)
		if was != nil {
			<-was
		}
		w.closeEnded(m, why)
		w.closing.mu.Lock()
		if w.closing.of[m.Slug] == done {
			delete(w.closing.of, m.Slug)
		}
		w.closing.mu.Unlock()
	}()
}

// closedFor waits until a mission's crew being closed behind an answer
// are.
func (w *Workspace) closedFor(slug string) {
	w.closing.mu.Lock()
	done := w.closing.of[slug]
	w.closing.mu.Unlock()
	if done != nil {
		<-done
	}
}

// closeEnded closes every crew session still open on mission m, which has
// ended or is being archived: there's nothing left for them to do on it.
func (w *Workspace) closeEnded(m *mission.Mission, why string) {
	// What they spent last, read before they go.
	if _, err := w.readCrewSpend(m); err != nil {
		execlog.Note(w.Root(), "crew "+m.Slug, "couldn't read what its crew spent: %v", err)
	}
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
