package ops

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/lessons"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/notification"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/spend"
	"github.com/withworkspace/workspace/internal/turn"
)

// Lessons are what crew keep running into where they work (internal/
// lessons), read from their handoffs: the daemon reads each crew's once
// it's handed off (ReadLessons), a few at a time, oldest first, so a
// workspace's handoffs from before are read too. Reading one is a turn of
// the [lessons] agent, headless with no tools, told the handoff's
// Environment section (or, for one from before that was asked for, the
// whole handoff), the repo's capsule setup and the lessons known: it says
// which each thing is, or what a new one would fix. A handoff whose
// Environment says nothing is read with no agent.
//
// A lesson becoming a recommendation (a second crew running into it) is a
// notification. You act on one from the Lessons page or ws lessons: make
// a mission of it, mark it fixed (crew running into it again open it
// again), or dismiss it.

// lessonsTurnFor is how long reading a handoff may take.
const lessonsTurnFor = 3 * time.Minute

// LessonsEach is how many handoffs a read takes, at most: the daemon reads
// every few minutes, so a workspace's handoffs from before come in over a
// while rather than all at once.
const LessonsEach = 10

// lessonsKnown is how many known lessons a read is told of, at most: the
// latest seen, the repo's and this machine's.
const lessonsKnown = 60

// handoffToRead is a crew's handoff not yet read for lessons.
type handoffToRead struct {
	m   *mission.Mission
	c   mission.Crew
	at  time.Time
	key string
}

// handoffsToRead are the handoffs not read for lessons as they were
// handed off, oldest first.
func (w *Workspace) handoffsToRead() ([]handoffToRead, error) {
	read, err := lessons.ReadTo(w.Root())
	if err != nil {
		return nil, err
	}
	list, err := w.Missions.List()
	if err != nil {
		return nil, err
	}
	var out []handoffToRead
	for _, m := range list {
		for _, c := range m.Crew {
			if c.HandedOff == nil {
				continue
			}
			key := lessons.Key(m.Slug, c.ID)
			if was, ok := read[key]; ok && was.Equal(c.HandedOff.UTC().Truncate(0)) {
				continue
			}
			out = append(out, handoffToRead{m: m, c: c, at: *c.HandedOff, key: key})
		}
	}
	slices.SortFunc(out, func(a, b handoffToRead) int { return a.at.Compare(b.at) })
	return out, nil
}

// LessonsRead is what a read of handoffs for lessons did.
type LessonsRead struct {
	Read    int              // handoffs read
	Left    int              // still to read
	Changed []lessons.Change // the lessons it found, as they stand
	Failed  []string         // the handoffs it couldn't read, and why
}

// ReadLessons reads up to n handoffs not yet read for lessons (0 for
// all), as the [lessons] agent finds them: none when [lessons] enabled is
// false. A lesson that becomes a recommendation is told of.
func (w *Workspace) ReadLessons(n int) (LessonsRead, error) {
	var out LessonsRead
	if !w.Config().LessonsEnabled() {
		return out, nil
	}
	todo, err := w.handoffsToRead()
	if err != nil {
		return out, err
	}
	if n > 0 && len(todo) > n {
		out.Left = len(todo) - n
		todo = todo[:n]
	}
	for _, h := range todo {
		changed, err := w.readHandoffLessons(h)
		if errors.Is(err, turn.ErrEnded) {
			out.Left += len(todo) - out.Read
			return out, nil // the daemon's going: the rest are read once it's back
		}
		if err != nil {
			out.Failed = append(out.Failed, h.key+": "+err.Error())
		}
		out.Read++
		out.Changed = append(out.Changed, changed...)
		for _, c := range changed {
			if c.Recommended && c.Lesson.State == lessons.Open {
				w.tellOfLesson(c.Lesson)
			}
		}
	}
	return out, nil
}

// readHandoffLessons reads one handoff for lessons and keeps what it
// found. A handoff that can't be read is kept read, with why, so it isn't
// read again and again; one the daemon went before reading isn't.
func (w *Workspace) readHandoffLessons(h handoffToRead) ([]lessons.Change, error) {
	repo, _, _ := strings.Cut(h.c.Capsule, "/")
	r := lessons.Read{Mission: h.m.Slug, Crew: h.c.ID, Repo: repo, HandedOff: h.at.UTC().Truncate(0), At: w.now()}
	found, err := w.lessonsIn(h, repo)
	if errors.Is(err, turn.ErrEnded) {
		return nil, err
	}
	if err != nil {
		r.Err = oneLine(err.Error())
	}
	changed, kerr := lessons.Keep(w.Root(), r, found)
	if kerr != nil {
		return nil, kerr
	}
	return changed, err
}

// lessonsIn is what the [lessons] agent finds in a handoff: nothing,
// with no agent, where its Environment section says nothing.
func (w *Workspace) lessonsIn(h handoffToRead, repo string) ([]lessons.Found, error) {
	data, err := os.ReadFile(w.Missions.HandoffPath(h.m.Slug, h.c.ID))
	if err != nil {
		return nil, err
	}
	ask := prompt.LessonsAsk{Mission: h.m.Slug, Title: h.m.Title, Crew: h.c.ID, Repo: repo}
	if handoff, err := mission.ParseHandoff(string(data)); err == nil && handoff.Complete() == nil {
		ask.Said = handoff.EnvironmentSays()
		if ask.Said == "" {
			return nil, nil
		}
	} else {
		ask.Said, ask.Whole = string(data), true // from before Environment was asked for
	}
	if rc, ok := w.Config().Repos[repo]; ok {
		ask.AfterCreate = rc.AfterCreate
	}
	known, err := lessons.All(w.Root())
	if err != nil {
		return nil, err
	}
	slices.SortFunc(known, func(a, b lessons.Lesson) int { return b.LastSeen.Compare(a.LastSeen) })
	for _, l := range known {
		if l.Repo != "" && l.Repo != repo {
			continue
		}
		ask.Known = append(ask.Known, prompt.KnownLesson{ID: l.ID, Repo: l.Repo, Title: l.Title, Change: l.Change, Crew: l.Crew})
		if len(ask.Known) == lessonsKnown {
			break
		}
	}
	pick, hn, err := Roles(w.Config()).ForLessons()
	if err != nil {
		return nil, err
	}
	run := harness.ConverseRun{Prompt: prompt.LessonsPrompt(ask), Dir: w.Root(), Model: pick.Model, Alone: true, Once: true}
	rep, err := RunHeadless(w.life(), w.Root(), "lessons "+h.key, spend.For{Mission: h.m.Slug, Kind: spend.Lessons, Subject: h.c.ID},
		crew.Env(w.Root(), "", ""), "", lessonsTurnFor, hn, run)
	if err != nil {
		return nil, err
	}
	said, err := prompt.ReadLessons(rep.Output)
	if err != nil {
		return nil, err
	}
	ids := map[int64]bool{}
	for _, k := range ask.Known {
		ids[k.ID] = true
	}
	out := make([]lessons.Found, 0, len(said))
	for _, f := range said {
		if f.Same != 0 && !ids[f.Same] {
			if f.Title == "" {
				continue // a lesson it wasn't told of, and no new one
			}
			f.Same = 0
		}
		if f.Same == 0 && f.Repo != "" && f.Repo != repo {
			if _, ok := w.Config().Repos[f.Repo]; !ok {
				f.Repo = repo // a repo the workspace hasn't: the crew's
			}
		}
		out = append(out, lessons.Found{Same: f.Same, Said: f.Said, Title: f.Title, Fix: f.Fix, Change: f.Change, Why: f.Why, Repo: f.Repo})
	}
	return out, nil
}

// LessonRecommended is the kind of notification a lesson becoming a
// recommendation is.
const LessonRecommended = "lesson.recommended"

// tellOfLesson tells you a lesson's become a recommendation.
func (w *Workspace) tellOfLesson(l lessons.Lesson) {
	_, _ = notification.Add(w.Root(), notification.Notification{
		Source: notification.SourceLessons, Kind: LessonRecommended, Refs: []string{LessonRef(l.ID)}, By: events.ByWS,
		Text: fmt.Sprintf("%d crew ran into it: %s", l.Crew, l.Title),
	})
}

// LessonRef is how a lesson is named: lesson-3.
func LessonRef(id int64) string { return "lesson-" + strconv.FormatInt(id, 10) }

// LessonOf is the lesson a ref names (lesson-3, or 3).
func LessonOf(ref string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(ref, "lesson-"), 10, 64)
	return id, err == nil && id > 0
}

// LessonsInfo is the lessons as the Lessons page shows them, and how
// reading handoffs for them stands.
type LessonsInfo struct {
	Lessons []lessons.Lesson `json:"lessons"`
	// On is whether handoffs are read ([lessons] enabled).
	On bool `json:"on"`
	// Read is how many handoffs have been read, Failed how many of them
	// couldn't be, and Waiting how many are still to read.
	Read    int `json:"read"`
	Failed  int `json:"failed"`
	Waiting int `json:"waiting"`
}

// Lessons is every lesson, recommendations first (the most crew), then
// notes, each by when crew last ran into it.
func (w *Workspace) Lessons() (LessonsInfo, error) {
	list, err := lessons.All(w.Root())
	if err != nil {
		return LessonsInfo{}, err
	}
	slices.SortStableFunc(list, func(a, b lessons.Lesson) int {
		if ar, br := a.IsRecommended(), b.IsRecommended(); ar != br {
			if ar {
				return -1
			}
			return 1
		}
		if a.Crew != b.Crew {
			return b.Crew - a.Crew
		}
		return b.LastSeen.Compare(a.LastSeen)
	})
	info := LessonsInfo{Lessons: list, On: w.Config().LessonsEnabled()}
	if info.Read, info.Failed, err = lessons.Reads(w.Root()); err != nil {
		return info, err
	}
	if todo, err := w.handoffsToRead(); err == nil {
		info.Waiting = len(todo)
	}
	return info, nil
}

// LessonsStamp names where the lessons stand, cheaply.
func (w *Workspace) LessonsStamp() string { return lessons.Stamp(w.Root()) }

// SetLessonState marks a lesson open, fixed or dismissed: yours to do.
func (w *Workspace) SetLessonState(id int64, state string, by Actor) error {
	if !by.IsYou() {
		return refused("not-yours", "a lesson is yours to mark fixed or dismiss")
	}
	if state != lessons.Open && state != lessons.Fixed && state != lessons.Dismissed {
		return invalid("a lesson is open, fixed or dismissed, not %q", state)
	}
	err := lessons.SetState(w.Root(), id, state, w.now())
	if errors.Is(err, lessons.ErrNoLesson) {
		return fmt.Errorf("%w: lesson %d", ErrNotFound, id)
	}
	return err
}

// LessonMission makes a mission of a lesson, waiting to be briefed: what
// to change, why, and each crew that ran into it, in their words. A lesson
// with a mission already says it.
func (w *Workspace) LessonMission(id int64, by Actor) (string, error) {
	if !by.IsYou() {
		return "", refused("not-yours", "a mission from a lesson is yours to start")
	}
	l, err := lessons.Get(w.Root(), id)
	if errors.Is(err, lessons.ErrNoLesson) {
		return "", fmt.Errorf("%w: lesson %d", ErrNotFound, id)
	}
	if err != nil {
		return "", err
	}
	if l.Mission != "" {
		if _, err := w.Missions.Load(l.Mission); err == nil {
			return l.Mission, nil
		}
	}
	m, err := w.Missions.CreateBriefing(LessonRequest(l))
	if err != nil {
		return "", err
	}
	if err := lessons.SetMission(w.Root(), id, m.Slug); err != nil {
		return m.Slug, err
	}
	return m.Slug, nil
}

// LessonRequest is the mission a lesson asks for, as a request to brief.
func LessonRequest(l lessons.Lesson) string {
	var b strings.Builder
	b.WriteString(l.Title)
	if l.Repo != "" {
		fmt.Fprintf(&b, " (in %s)", l.Repo)
	}
	b.WriteString("\n\n")
	b.WriteString(l.Change)
	if l.Why != "" {
		b.WriteString("\n\nWhy crew run into it: " + l.Why)
	}
	fmt.Fprintf(&b, "\n\n%s, from their handoffs:\n", crewWord(l.Crew))
	for _, s := range l.Seen {
		fmt.Fprintf(&b, "- %s, crew %s, %s: %s\n", s.Mission, s.Crew, s.At.Local().Format(time.DateOnly), s.Said)
	}
	return strings.TrimRight(b.String(), "\n")
}

func crewWord(n int) string {
	if n == 1 {
		return "One crew ran into it"
	}
	return fmt.Sprintf("%d crew ran into it", n)
}
