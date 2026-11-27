package ops

import (
	"fmt"

	"github.com/withworkspace/workspace/internal/mission"
)

// Crew the daemon shut down under (it restarted, was upgraded or closed)
// are recorded as interrupted (mission.Crew.Interrupted). The next daemon
// resumes those that were still at their work, so a restart doesn't leave
// a step stalled until someone notices; the rest it leaves be. Each is
// looked at once.

// CarryOn is what crew a restart stopped mid-turn are resumed with.
const CarryOn = "ws's daemon shut down while you were working (a restart or an upgrade), which stopped your session mid-turn. " +
	"Carry on from where you were: check what you'd already done (git status, the files you were changing) before you redo anything."

// ResumedCarryOn is what crew that ended mid-turn are resumed with, by
// hand, when nothing else is to be said to them: with nothing typed in,
// they'd sit waiting to be told to go on.
const ResumedCarryOn = "Your session ended while you were working, and you've been resumed. " +
	"Carry on from where you were: check what you'd already done (git status, the files you were changing) before you redo anything."

// midTurn says whether crew were at their work when they ended: not
// waiting for you, nor asking you something.
func midTurn(c mission.Crew) bool {
	return c.Activity != mission.ActivityWaiting && c.Activity != mission.ActivityQuestion
}

// CrewAtWork names the crew a restart of the daemon would stop mid-turn:
// those running that aren't waiting for you. A daemon that's to restart
// when it can waits for them; crew that are waiting it hangs up, and the
// next reopens them as they were.
func (w *Workspace) CrewAtWork() []string {
	missions, err := w.Missions.InPlay()
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range missions {
		for _, c := range m.Crew {
			if c.Ended == nil && midTurn(c) {
				out = append(out, c.ID+" on "+m.Slug+", mid-turn")
			}
		}
	}
	return out
}

// Interrupted is crew a daemon shut down under, as the next one found it:
// resumed, or why not.
type Interrupted struct {
	Slug, ID string
	Resumed  bool
	Why      string // why it wasn't resumed
}

func (i Interrupted) String() string {
	if i.Resumed {
		return fmt.Sprintf("resumed %s/%s, which the last daemon shut down under", i.Slug, i.ID)
	}
	return fmt.Sprintf("didn't resume %s/%s, which the last daemon shut down under: %s", i.Slug, i.ID, i.Why)
}

// ResumeInterrupted resumes the crew the daemon before this one shut down
// under, where they were still at their work: on a mission in play that
// hasn't ended and isn't parked, not handed back, and the latest crew in
// their capsule. Crew that were mid-turn are told to carry on (unless the
// orchestrator left them a message to resume with); crew that were waiting
// for you are reopened as they were. Resuming isn't a new dispatch, so it
// doesn't count against the orchestrator's. It says what it did with each.
func (w *Workspace) ResumeInterrupted() []Interrupted {
	missions, err := w.Missions.InPlay()
	if err != nil {
		return nil
	}
	var out []Interrupted
	for _, m := range missions {
		for i, c := range m.Crew {
			if !c.Interrupted || c.Ended == nil {
				continue
			}
			r := Interrupted{Slug: m.Slug, ID: c.ID}
			if r.Why = notResuming(m, i); r.Why == "" {
				r.Why = w.resumeInterrupted(m, c)
			}
			r.Resumed = r.Why == ""
			if !r.Resumed {
				w.Missions.ClearInterrupted(m.Slug, c.ID) //nolint:errcheck // at worst, looked at again next time
			}
			out = append(out, r)
		}
	}
	return out
}

// notResuming says why crew i of m, interrupted, isn't to be resumed; ""
// when it is.
func notResuming(m *mission.Mission, i int) string {
	c := m.Crew[i]
	switch {
	case m.Ending != nil:
		return "the mission has ended"
	case m.State == mission.Parked:
		return "the mission is parked"
	case c.HandedBack(m):
		return "it had handed its work back"
	}
	if c.Role == "" && c.Capsule != "" {
		for _, later := range m.Crew[i+1:] {
			if later.Role == "" && later.Capsule == c.Capsule {
				return later.ID + " has worked in " + c.Capsule + " since"
			}
		}
	}
	return ""
}

// resumeInterrupted resumes crew c, telling it to carry on if it was
// mid-turn. It says why not, if it couldn't.
func (w *Workspace) resumeInterrupted(m *mission.Mission, c mission.Crew) string {
	midTurn := midTurn(c)
	if midTurn && c.ResumeWith == "" {
		if err := w.Missions.SetResumeWithBy(m.Slug, c.ID, CarryOn, AsWS.By()); err != nil {
			return err.Error()
		}
	}
	if _, err := w.Launch(m.Slug, Dispatch{Resume: c.ID}, AsWS); err != nil {
		if midTurn && c.ResumeWith == "" {
			w.Missions.SetResumeWith(m.Slug, c.ID, "") //nolint:errcheck // a message for a resume that didn't happen
		}
		return err.Error()
	}
	text := "Workspace resumed " + c.ID + ", stopped by a restart while waiting for you"
	if midTurn {
		text = "Workspace resumed " + c.ID + ", stopped by a restart mid-turn, and told them to carry on"
	}
	w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventNote, Text: text}) //nolint:errcheck // the resume is logged already
	return ""
}
