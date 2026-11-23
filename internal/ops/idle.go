package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

// IdleAfter is how long crew at a step of theirs may sit, their turn over
// with nothing said (no done, no question) and nothing of theirs running
// that ws knows of, before ws asks them where they are.
const IdleAfter = 10 * time.Minute

// idleAsked begins what ws asks idle crew: how it's known, in the log,
// that they've been asked at this step already.
const idleAsked = "Your turn ended"

// NudgeIdle asks the crew of every mission in play who've sat idle since
// before now-IdleAfter (mission.Idle) where they are: once a step each.
// Crew wait on things ws can't see (an agent of their own they started,
// which reports to them and not to ws), and end a turn meaning to come
// back; nothing brings them back. Asked, they look, and carry on, report
// done or ask. Their next turn's end is the orchestrator's to look at, as
// after any message of ws's, so a second silence isn't ws's to ask about
// again. With the orchestrator off, crew who've stopped are yours as they
// are, and nobody's asked. It says what it did.
func (w *Workspace) NudgeIdle(now time.Time) []string {
	if !w.Config().OrchestratorEnabled() || w.opts.Host == nil {
		return nil
	}
	missions, err := w.Missions.InPlay()
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range missions {
		if !Watched(m) {
			continue
		}
		brief, _ := w.Missions.ReadBrief(m.Slug)
		idle := m.Idle(brief, now, IdleAfter)
		if len(idle) == 0 {
			continue
		}
		cur, ok := currentStep(w.Missions, m)
		if !ok {
			continue
		}
		for _, c := range idle {
			if w.idleAskedAt(m, cur, c.ID) {
				continue
			}
			text := fmt.Sprintf("%s %s ago, at step %d, without your reporting done or asking anything, and ws knows of nothing you left running. If you were waiting on something, look at whether it has finished, and carry on. When your part is finished, report done with a handoff (ws mission done); if you can't go on, ask (ws mission ask).",
				idleAsked, now.Sub(*c.ActivityAt).Round(time.Minute), cur.N)
			if _, err := w.Say(m.Slug, c.ID, text, AsWS); err != nil {
				out = append(out, fmt.Sprintf("%s/%s has been idle since %s, and couldn't be asked: %v", m.Slug, c.ID, c.ActivityAt.Format(time.TimeOnly), err))
				continue
			}
			out = append(out, fmt.Sprintf("asked %s/%s, idle at step %d since %s, where it is", m.Slug, c.ID, cur.N, c.ActivityAt.Format(time.TimeOnly)))
		}
	}
	return out
}

// idleAskedAt is whether ws has asked crew id where they are since the
// step the mission's at began: what it typed in is in the log.
func (w *Workspace) idleAskedAt(m *mission.Mission, cur mission.StageState, id string) bool {
	since := time.Time{}
	if cur.Record != nil && cur.Record.Started != nil {
		since = *cur.Record.Started
	}
	events, err := w.Missions.ReadLog(m.Slug, 0)
	if err != nil {
		return true // not known: not asked twice
	}
	for _, e := range events {
		if e.Kind == mission.EventResumeWith && e.Crew == id && e.By == "ws" && !e.Time.Before(since) && strings.Contains(e.Text, idleAsked) {
			return true
		}
	}
	return false
}
