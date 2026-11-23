package mission

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/slug"
	"github.com/withworkspace/workspace/internal/store"
)

// ErrNotFound is returned when a mission slug doesn't exist.
var ErrNotFound = errors.New("mission not found")

// now is replaced in tests.
var now = func() time.Time { return time.Now().UTC().Truncate(time.Second) }

const (
	briefFile = "brief.md"
)

// Store reads and writes missions for one workspace.
type Store struct {
	Root string // workspace root
}

// Dir is the directory holding every mission.
func (s Store) Dir() string { return filepath.Join(s.Root, ".ws", "missions") }

func (s Store) missionDir(slug string) string { return filepath.Join(s.Dir(), slug) }

// BriefPath is the path of a mission's brief.
func (s Store) BriefPath(slug string) string { return filepath.Join(s.missionDir(slug), briefFile) }

// Create makes a new mission with a templated brief and a "created" event.
// The slug is derived from the title and made unique: its folder (for its
// brief and files) is made first, then its record.
func (s Store) Create(title string) (*Mission, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("mission title is required")
	}
	if err := os.MkdirAll(s.Dir(), 0o755); err != nil {
		return nil, err
	}
	ensureExcluded(s.Root)

	base := Slugify(title)
	slug := base
	for i := 2; ; i++ {
		err := os.Mkdir(s.missionDir(slug), 0o755)
		if err == nil {
			if err := os.WriteFile(s.BriefPath(slug), []byte(briefTemplate(title)), 0o644); err != nil {
				return nil, err
			}
			t := now()
			// Made with its brief, it's confirmed as it's made; an idea
			// (CreateBriefing) is to brief.
			m := &Mission{Slug: slug, Title: title, Created: t, Confirmed: &t}
			err = s.write(func(tx *store.Tx) error {
				var taken int
				if err := tx.QueryRow(`SELECT count(*) FROM missions WHERE slug = ?`, slug).Scan(&taken); err != nil {
					return err
				}
				if taken > 0 {
					return errTaken
				}
				if err := s.keep(tx, m); err != nil {
					return err
				}
				return s.appendLog(tx, slug, event(Event{Kind: EventCreated, Text: title}))
			})
			if err == nil {
				return m, nil
			}
			if !errors.Is(err, errTaken) {
				return nil, err
			}
			// A record whose folder had gone: leave it, and take the next.
			os.RemoveAll(s.missionDir(slug))
		} else if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

var errTaken = errors.New("taken")

func briefTemplate(title string) string {
	return "# " + title + "\n\n## Goal\n\n\n## Constraints\n\n\n## Acceptance\n\n"
}

// Load reads one mission.
func (s Store) Load(slug string) (*Mission, error) {
	if !validSlug(slug) {
		return nil, notFound(slug)
	}
	var m *Mission
	err := s.read(func(tx *store.Tx) error {
		var err error
		m, err = s.loadIn(tx, slug)
		return err
	})
	return m, err
}

// FilesDir is a mission's folder for private files: mission:path artifacts
// resolve inside it.
func (s Store) FilesDir(slug string) string {
	return filepath.Join(s.missionDir(slug), MissionFilesDir)
}

// List returns every mission, archived ones too, oldest first. Unreadable
// missions are skipped. InPlay is those that aren't archived.
func (s Store) List() ([]*Mission, error) { return s.list(`1`) }

// Update loads a mission, applies fn and saves it, in one transaction, so
// concurrent crew don't lose each other's writes. fn returning an error
// aborts without saving; fn changing nothing saves nothing. fn mustn't do
// anything slow: read what it needs first.
func (s Store) Update(slug string, fn func(*Mission) error) (*Mission, error) {
	var m *Mission
	err := s.write(func(tx *store.Tx) error {
		var err error
		if m, err = s.loadIn(tx, slug); err != nil {
			return err
		}
		was, _ := json.Marshal(m)
		if err := fn(m); err != nil {
			return err
		}
		if now, _ := json.Marshal(m); string(now) == string(was) {
			return nil
		}
		return s.keep(tx, m)
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

// FindByCapsule returns the mission owning repo/name, if any.
func (s Store) FindByCapsule(repo, name string) (*Mission, bool) {
	missions, err := s.List()
	if err != nil {
		return nil, false
	}
	for _, m := range missions {
		if m.HasCapsule(repo, name) {
			return m, true
		}
	}
	return nil, false
}

// ReadBrief returns a mission's brief, or "" if it has none.
func (s Store) ReadBrief(slug string) (string, error) {
	data, err := os.ReadFile(s.BriefPath(slug))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

var (
	slugShape = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

const maxSlugLen = 40

// Slugify turns a title into a directory-safe slug.
func Slugify(title string) string { return slug.Make(title, maxSlugLen, "mission") }

func validSlug(slug string) bool { return slugShape.MatchString(slug) }

// ensureExcluded keeps .ws/ out of the config repo that the workspace root is
// a clone of, by adding it to that repo's info/exclude. Best effort: a root
// that isn't a git repo is left alone.
func ensureExcluded(root string) {
	if IsExcluded(root) {
		return
	}
	path, ok := excludePath(root)
	if !ok {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if data, _ := os.ReadFile(path); len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		f.WriteString("\n")
	}
	f.WriteString(excludeEntry + "\n")
}

const excludeEntry = "/.ws/"

// IsExcluded reports whether .ws/ is ignored by the workspace root's git repo.
// A root that isn't a git repo counts as excluded (nothing to leak into).
func IsExcluded(root string) bool {
	if _, ok := excludePath(root); !ok {
		return true
	}
	return git.Ok(root, "check-ignore", "-q", ".ws/")
}

func excludePath(root string) (string, bool) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return "", false
	}
	p, err := git.Line(root, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return "", false
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return p, true
}

// EnsureExcluded is the exported form of ensureExcluded, for ws doctor --fix.
func EnsureExcluded(root string) { ensureExcluded(root) }

// StartCrew records new crew on a mission and logs it. IDs are c1, c2, … in
// order of launch, so they stay short enough to show in the sidebar.
func (s Store) StartCrew(slug, harness, conversation string) (Crew, error) {
	return s.StartCrewAs(slug, harness, conversation, "")
}

// StartCrewAs is StartCrew with a role (RoleBriefing, or "" for work).
func (s Store) StartCrewAs(slug, harness, conversation, role string) (Crew, error) {
	return s.StartCrewOn(slug, harness, "", conversation, role)
}

// StartCrewOn is StartCrewAs with the model crew were launched on, when
// one was asked for ("" for the one their role gives them).
func (s Store) StartCrewOn(slug, harness, model, conversation, role string) (Crew, error) {
	var crew Crew
	_, err := s.Update(slug, func(m *Mission) error {
		crew = Crew{
			ID:           fmt.Sprintf("c%d", len(m.Crew)+1),
			Harness:      harness,
			Model:        model,
			Conversation: conversation,
			Started:      now(),
			Role:         role,
		}
		if m.Launching != nil {
			// The crew ws was starting: recorded, they hold the mark.
			t := crew.Started
			crew.Starting, m.Launching = &t, nil
		}
		m.Crew = append(m.Crew, crew)
		m.Waiting = nil
		return nil
	})
	if err != nil {
		return Crew{}, err
	}
	text := harness
	if model != "" {
		text += " on " + model
	}
	if conversation != "" {
		text += " " + conversation
	}
	return crew, s.AppendLog(slug, Event{Kind: EventCrewStarted, Crew: crew.ID, Text: text})
}

// EndCrew records that crew stopped, with a short exit summary. The first
// end recorded wins: ending crew that has already ended changes nothing (so
// `ws mission ready` can record "briefed" before the hang-up lands).
func (s Store) EndCrew(slug, id, exit string) error {
	return s.EndCrewStopped(slug, id, exit, false)
}

// EndCrewStopped is EndCrew that also records whether the crew was stopped
// (by you, or its host shutting down) rather than ending by itself.
func (s Store) EndCrewStopped(slug, id, exit string, stopped bool) error {
	return s.endCrew(slug, id, exit, stopped, false)
}

// EndCrewInterrupted records crew whose host shut down under it: stopped,
// and Interrupted, for the next daemon to resume if it was still at its
// work.
func (s Store) EndCrewInterrupted(slug, id, exit string) error {
	return s.endCrew(slug, id, exit, true, true)
}

// How crew ended, as its crew_ended event says (Event.What): stopped by a
// daemon shutting down under it, its session closed by ws after it handed
// its work back, or stopped (by you); "" is ended by itself.
const (
	EndedInterrupted = "interrupted"
	EndedHandedBack  = "handed-back"
	EndedStopped     = "stopped"
)

func (s Store) endCrew(slug, id, exit string, stopped, interrupted bool) error {
	found, already, done := false, false, false
	if _, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if m.Crew[i].ID == id {
				found = true
				if m.Crew[i].Ended != nil {
					already = true
					return nil
				}
				done = m.Crew[i].Status == CrewDone
				t := now()
				m.Crew[i].Ended = &t
				m.Crew[i].started()
				m.Crew[i].Exit = exit
				m.Crew[i].Stopped = stopped
				m.Crew[i].Interrupted = interrupted
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("mission %s has no crew %s", slug, id)
	}
	if already {
		return nil
	}
	how := ""
	switch {
	case interrupted:
		how = EndedInterrupted
	case stopped && done:
		how = EndedHandedBack
	case stopped:
		how = EndedStopped
	}
	return s.AppendLog(slug, Event{Kind: EventCrewEnded, Crew: id, Text: exit, What: how})
}

// SetCrewProcess records where running crew run: its process id and the
// capsule (repo/name) it works in.
func (s Store) SetCrewProcess(slug, id string, pid int, capsule string) error {
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if m.Crew[i].ID == id {
				m.Crew[i].PID = pid
				if capsule != "" {
					m.Crew[i].Capsule = capsule
				}
			}
		}
		return nil
	})
	return err
}

// ResumeCrew reopens ended crew: the same crew, continuing its conversation,
// rather than new crew. It clears the end and logs the resume. Resumed by
// ws or the orchestrator (by), or by you with a message one of them left
// (to carry on, to put something right), they've more to do, so what
// they'd said of themselves is taken back. Resumed by you to talk to,
// only a question they'd asked is: that they'd reported done stands, as
// when you talk to crew still running (TalkedTo).
func (s Store) ResumeCrew(slug, id, by string) (Crew, error) {
	var crew Crew
	found := false
	if _, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if m.Crew[i].ID == id {
				m.Crew[i].Ended = nil
				m.Crew[i].Exit = ""
				m.Crew[i].Stopped = false
				m.Crew[i].Interrupted = false
				m.Crew[i].Conversing = false
				m.Crew[i].PID = 0
				m.Crew[i].Pending = nil // a new session waits on nothing yet
				if by == "you" && (m.Crew[i].ResumeWith == "" || m.Crew[i].ResumeWithBy == "you") {
					m.Crew[i].answered()
				} else {
					m.Crew[i].Status, m.Crew[i].Note, m.Crew[i].StatusAt = "", "", nil
				}
				if m.Launching != nil {
					// The crew ws was starting again: they hold the mark.
					t := now()
					m.Crew[i].Starting, m.Launching = &t, nil
					m.Crew[i].AwaitsReport, m.Crew[i].Handed = false, false
				}
				crew, found = m.Crew[i], true
			}
		}
		m.Waiting = nil
		return nil
	}); err != nil {
		return Crew{}, err
	}
	if !found {
		return Crew{}, fmt.Errorf("mission %s has no crew %s", slug, id)
	}
	return crew, s.AppendLog(slug, Event{Kind: EventCrewResumed, Crew: id, Text: crew.Harness + " " + crew.Conversation})
}

// ClearInterrupted records that crew a daemon shut down under has been
// looked at, and isn't to be resumed: it had handed its work back, say.
func (s Store) ClearInterrupted(slug, id string) error {
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if m.Crew[i].ID == id {
				m.Crew[i].Interrupted = false
			}
		}
		return nil
	})
	return err
}

// LastResumableCrew is the most recent crew with a conversation id that
// isn't running (per live), if any.
//
// Briefing crew only count while the mission is still being briefed: once
// it is ready, they have done their job and there is nothing to resume.
func (m *Mission) LastResumableCrew(live func(id string) bool) (Crew, bool) {
	for i := len(m.Crew) - 1; i >= 0; i-- {
		c := m.Crew[i]
		if c.Done(m) {
			continue
		}
		if c.Conversation != "" && !live(c.ID) {
			return c, true
		}
	}
	return Crew{}, false
}

// Done reports whether crew has served its purpose on mission m: a briefing
// crew once the mission is no longer being briefed.
func (c Crew) Done(m *Mission) bool {
	return c.Role == RoleBriefing && !m.ToBrief()
}

// HandedBack reports whether crew has handed its work back: it reported
// done (and ws hasn't given it more to do since), or it finished briefing.
// Its session may still be open, and you may be talking to it, but its
// work is handed over: it doesn't make its capsule busy, and whoever
// starts new crew there ends it.
func (c Crew) HandedBack(m *Mission) bool {
	return c.Status == CrewDone || c.Done(m)
}

// InTurnSinceDone reports whether crew who reported done are in a turn
// begun since: on a message sent to them after it (yours, or one that
// waited for their turn to end). Their done stands, but what they're
// doing now isn't in it yet, so ws doesn't pass their step under them: it
// looks again when the turn ends.
func (c Crew) InTurnSinceDone() bool {
	return c.Status == CrewDone && c.Ended == nil && c.Activity == ActivityWorking &&
		c.StatusAt != nil && c.ActivityAt != nil && c.ActivityAt.After(*c.StatusAt)
}

// startingFor is how long crew recorded without a process are taken to be
// starting, rather than lost: a launch records crew, then starts the
// server that runs them, and that server reconciles as it starts.
const startingFor = 2 * time.Minute

// ReconcileCrew closes crew records left open by a server that died without
// recording their end: any open crew whose process isn't alive is ended as
// "lost", and so is crew that never got a process, once it's had time to.
// It returns how many were closed.
func (s Store) ReconcileCrew(alive func(pid int) bool) int {
	missions, err := s.List()
	if err != nil {
		return 0
	}
	closed := 0
	for _, m := range missions {
		for _, c := range m.Crew {
			if c.Ended != nil || (c.PID != 0 && alive(c.PID)) {
				continue
			}
			if c.PID == 0 && now().Sub(c.Started) < startingFor {
				continue // still starting
			}
			if s.EndCrew(m.Slug, c.ID, "lost") == nil {
				closed++
			}
		}
	}
	return closed
}

// maxProvisionalTitle bounds the title a briefing mission starts with (the
// request's first line) until the briefing crew sets a real one.
const maxProvisionalTitle = 80

// CreateBriefing makes a mission from a free-form request ("fe issue 45",
// "the token refresh race in api"), to brief (an idea), with an empty brief
// for its conversation, or a briefing crew, to write.
func (s Store) CreateBriefing(request string) (*Mission, error) {
	request = strings.TrimSpace(request)
	if request == "" {
		return nil, errors.New("say what the mission is")
	}
	title, _, _ := strings.Cut(request, "\n")
	if len(title) > maxProvisionalTitle {
		title = strings.TrimSpace(title[:maxProvisionalTitle]) + "…"
	}
	m, err := s.Create(title)
	if err != nil {
		return nil, err
	}
	m, err = s.Update(m.Slug, func(m *Mission) error {
		m.Confirmed, m.ConfirmedBy = nil, ""
		m.Request = request
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m, os.WriteFile(s.BriefPath(m.Slug), nil, 0o644)
}

// WriteBrief replaces a mission's brief.
func (s Store) WriteBrief(slug, brief string) error {
	if _, err := s.Load(slug); err != nil {
		return err
	}
	if brief != "" && !strings.HasSuffix(brief, "\n") {
		brief += "\n"
	}
	return os.WriteFile(s.BriefPath(slug), []byte(brief), 0o644)
}

// IsUnwrittenBrief reports whether a brief is empty or still the template
// a mission is created with.
func IsUnwrittenBrief(brief, title string) bool {
	b := strings.TrimSpace(brief)
	return b == "" || b == strings.TrimSpace(briefTemplate(title))
}

// SetTitle changes a mission's title (its slug stays).
func (s Store) SetTitle(slug, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("title is empty")
	}
	if n := utf8.RuneCountInString(title); n > MaxTitle {
		return fmt.Errorf("title is %d characters; keep it to %d or fewer: a terse, plain-English name", n, MaxTitle)
	}
	_, err := s.Update(slug, func(m *Mission) error {
		m.Title = title
		return nil
	})
	return err
}

// ErrConfirmed is confirming a mission's brief twice.
var ErrConfirmed = errors.New("its brief is confirmed already: it's under way")

// Confirm records the mission's brief (and plan) confirmed, by by: the
// go-ahead. It's no longer to brief.
func (s Store) Confirm(slug, by string) error {
	_, err := s.Update(slug, func(m *Mission) error {
		if !m.ToBrief() {
			return fmt.Errorf("mission %s: %w", slug, ErrConfirmed)
		}
		t := now()
		m.Confirmed, m.ConfirmedBy = &t, by
		return nil
	})
	if err != nil {
		return err
	}
	return s.AppendLog(slug, Event{Kind: EventState, Crew: by, Text: "brief confirmed: under way"})
}

func stateOrDerived(s State) string {
	if s == "" {
		return "ready"
	}
	return string(s)
}

// Rename moves a mission to a new slug: its folder, then its record, its
// log and its decisions. Crew running with WS_MISSION set to the old slug
// lose track of it, so the deck only renames at the end of briefing. Its
// conversations are their store's to move (conversation.MoveMission).
func (s Store) Rename(old, slug string) error {
	if old == slug {
		return nil
	}
	if !validSlug(slug) || len(slug) > maxSlugLen {
		return fmt.Errorf("%q isn't a valid slug (lowercase letters, digits and dashes, up to %d)", slug, maxSlugLen)
	}
	if _, err := s.Load(old); err != nil {
		return err
	}
	if _, err := os.Stat(s.missionDir(slug)); err == nil {
		return fmt.Errorf("mission %s already exists", slug)
	}
	if err := os.Rename(s.missionDir(old), s.missionDir(slug)); err != nil {
		return err
	}
	err := s.write(func(tx *store.Tx) error {
		var taken int
		if err := tx.QueryRow(`SELECT count(*) FROM missions WHERE slug = ?`, slug).Scan(&taken); err != nil {
			return err
		}
		if taken > 0 {
			return fmt.Errorf("mission %s already exists", slug)
		}
		all, err := s.decisionsIn(tx, old)
		if err != nil {
			return err
		}
		if err := tx.Forget(missionOrigin(old)); err != nil {
			return err
		}
		for _, d := range all {
			if err := tx.Forget(decisionOrigin(old, d.ID)); err != nil {
				return err
			}
		}
		if err := forgetFiles(tx, old); err != nil { // read again under its new slug, when next asked
			return err
		}
		for _, table := range []string{"missions", "mission_log", "decisions"} {
			if _, err := tx.Exec(`UPDATE `+table+` SET slug = ? WHERE slug = ?`, slug, old); err != nil {
				return err
			}
		}
		m, err := s.loadIn(tx, slug)
		if err != nil {
			return err
		}
		// A turn of the planner's underway was begun under the old slug,
		// and can't record its reply under this one: its mark goes.
		m.Planning = nil
		if err := s.keep(tx, m); err != nil {
			return err
		}
		if err := s.decisionsGraph(tx, slug, all); err != nil {
			return err
		}
		changed(tx, old)
		return s.appendLog(tx, slug, event(Event{Kind: EventState, Text: "renamed from " + old}))
	})
	if err != nil {
		os.Rename(s.missionDir(slug), s.missionDir(old)) // as it was
		return err
	}
	return nil
}

// SetCrewStatus records what crew says about itself (CrewAsking or CrewDone,
// with the question or summary) and logs it.
func (s Store) SetCrewStatus(slug, id, status, note string) error {
	found := false
	if _, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if m.Crew[i].ID == id {
				t := now()
				m.Crew[i].Status, m.Crew[i].Note, m.Crew[i].StatusAt = status, note, &t
				m.Crew[i].started() // it's said something: it's started
				found = true
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("mission %s has no crew %s", slug, id)
	}
	kind := EventCrewAsked
	if status == CrewDone {
		kind = EventCrewDone
	}
	return s.AppendLog(slug, Event{Kind: kind, Crew: id, Text: note})
}

// TalkedTo records that you sent crew a message: when its turn ends it's
// answering you, and a question it asked has been answered. That it
// reported done stands: what you say to crew who've handed their work
// over doesn't take that back, so their step can still be checked and
// passed while you talk to them. Only ws giving them more to do does
// (Nudged, ClearStatus), or their reporting again. It's logged, so the
// mission (and the orchestrator) knows you've redirected it: what you
// said, or "" for what you typed in its terminal, logged once a turn. It
// reports whether that answered a question.
func (s Store) TalkedTo(slug, id, message string) (bool, error) {
	cleared, already := false, false
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if c := &m.Crew[i]; c.ID == id {
				cleared, already = c.answered(), c.Conversing
				c.Conversing = true
			}
		}
		return nil
	})
	if err != nil || (message == "" && already) {
		return cleared, err
	}
	return cleared, s.AppendLog(slug, Event{Kind: EventTold, Crew: id, By: "you", Text: strings.TrimSpace(message)})
}

// answered takes back a question crew asked (CrewAsking), which a message
// sent to them answers, and reports whether there was one. That they
// reported done isn't a message's to take back.
func (c *Crew) answered() bool {
	if c.Status != CrewAsking {
		return false
	}
	c.Status, c.Note, c.StatusAt = "", "", nil
	return true
}

// ClearStatus takes back what crew last said of itself (done, asking):
// it's been given more to do, at its step or the next, so its done isn't
// its latest word any more.
func (s Store) ClearStatus(slug, id string) error {
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if c := &m.Crew[i]; c.ID == id {
				c.Status, c.Note, c.StatusAt = "", "", nil
			}
		}
		return nil
	})
	return err
}

// Nudged records that ws sent crew a message for the orchestrator (by, it
// or ws): when its turn ends, the orchestrator looks again. They've been
// given something to do, so what they'd said of themselves by then (done,
// asking) isn't their latest word any more. What they say after it's sent
// stands, even where they hadn't read it yet (a message waits for a
// turn to end): it's taken back as it's sent, not as it's read.
func (s Store) Nudged(slug, id, message, by string) error {
	if _, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if c := &m.Crew[i]; c.ID == id {
				c.Conversing, c.ResumeWith, c.ResumeWithBy = false, "", ""
				c.Status, c.Note, c.StatusAt = "", "", nil
			}
		}
		m.Waiting = nil
		return nil
	}); err != nil {
		return err
	}
	return s.AppendLog(slug, Event{Kind: EventResumeWith, Crew: id, By: by, Text: "typed in: " + message})
}

// SetWaiting records that the mission waits on something outside it ws
// can't see ("" clears it).
func (s Store) SetWaiting(slug, text, by string) error {
	return s.setProposal(slug, text, by, EventWaiting, func(m *Mission) **Proposal { return &m.Waiting })
}

// AddArtifact records an artifact, once per Ref: adding one already
// recorded updates its kind and label (when given) instead. The caller
// normalises Ref.
func (s Store) AddArtifact(slug string, a Artifact) error {
	a.Ref = strings.TrimSpace(a.Ref)
	if a.Ref == "" {
		return errors.New("artifact reference is empty")
	}
	updated := false
	brief, _ := s.ReadBrief(slug)
	if _, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Artifacts {
			if m.Artifacts[i].Ref == a.Ref {
				if a.Kind != "" {
					m.Artifacts[i].Kind = a.Kind
				}
				if a.Label != "" {
					m.Artifacts[i].Label = a.Label
				}
				updated = true
				return nil
			}
		}
		a.Added = now()
		if a.Stage == "" {
			a.Stage = stageMadeFor(m, brief, a.Crew)
		}
		m.Artifacts = append(m.Artifacts, a)
		return nil
	}); err != nil {
		return err
	}
	text := a.Kind + " " + a.Ref
	if a.Label != "" {
		text = a.Kind + " " + a.Label + " · " + a.Ref
	}
	if updated {
		text += " (updated)"
	}
	return s.AppendLog(slug, Event{Kind: EventArtifact, Crew: a.Crew, Text: text})
}

// RemoveArtifact forgets an artifact by its Ref. It errors if there's no
// such artifact.
func (s Store) RemoveArtifact(slug, ref, by string) error {
	removed := false
	if _, err := s.Update(slug, func(m *Mission) error {
		for i, a := range m.Artifacts {
			if a.Ref == ref {
				m.Artifacts = append(m.Artifacts[:i], m.Artifacts[i+1:]...)
				removed = true
				return nil
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if !removed {
		return fmt.Errorf("mission %s has no artifact %s", slug, ref)
	}
	return s.AppendLog(slug, Event{Kind: EventArtifact, Crew: by, Text: "removed " + ref})
}

// TranscriptPath is where a crew's output is kept.
func (s Store) TranscriptPath(slug, id string) string {
	return filepath.Join(s.missionDir(slug), "crew-"+id+".log")
}

// ACPRecordPath is where the daemon records an ACP crew's session
// (acp.Event JSON lines), across its runs: its transcript, read by the web
// UI as a harness's session file is.
func (s Store) ACPRecordPath(slug, id string) string {
	return filepath.Join(s.missionDir(slug), "crew-"+id+".acp.jsonl")
}

// SaveTranscript appends a crew run's output to its transcript, under a
// header saying how the run ended (resumed crew keep earlier runs).
func (s Store) SaveTranscript(slug, id, exit, text string) error {
	if !validSlug(slug) {
		return ErrNotFound
	}
	if _, err := os.Stat(s.missionDir(slug)); err != nil {
		return ErrNotFound
	}
	f, err := os.OpenFile(s.TranscriptPath(slug, id), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "=== %s run ended %s (%s) ===\n%s", id, now().Format(time.RFC3339), exit, text)
	return err
}

// The answers ws suggests to a step of yours (AskAtStep).
const (
	CheckInGoOn   = "Go on"
	CheckInNotYet = "Not yet"
)

func (s Store) setProposal(slug, text, by, kind string, field func(*Mission) **Proposal) error {
	text = strings.TrimSpace(text)
	if _, err := s.Update(slug, func(m *Mission) error {
		p := field(m)
		if text == "" {
			*p = nil
			return nil
		}
		*p = &Proposal{Text: text, By: by, At: now()}
		return nil
	}); err != nil {
		return err
	}
	if text == "" {
		return nil
	}
	return s.AppendLog(slug, Event{Kind: kind, Crew: by, Text: text})
}

// SetResumeWith stores a message for the next time crew is resumed ("" clears).
func (s Store) SetResumeWith(slug, id, message string) error {
	return s.SetResumeWithBy(slug, id, message, "")
}

// SetResumeWithBy is SetResumeWith, saying who it's from ("you",
// "orchestrator", "ws"), as the log has it.
func (s Store) SetResumeWithBy(slug, id, message, by string) error {
	found := false
	if _, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if m.Crew[i].ID == id {
				m.Crew[i].ResumeWith = strings.TrimSpace(message)
				m.Crew[i].ResumeWithBy = ""
				if m.Crew[i].ResumeWith != "" {
					m.Crew[i].ResumeWithBy = by
				}
				found = true
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("mission %s has no crew %s", slug, id)
	}
	if message == "" {
		return nil
	}
	return s.AppendLog(slug, Event{Kind: EventResumeWith, Crew: id, By: by, Text: message})
}

// eventKey names a crew's latest state: its run, status, the end of a
// turn it said nothing about, and its end.
func (c Crew) eventKey() string {
	key := c.Started.Format(time.RFC3339)
	if c.StatusAt != nil {
		key += "|" + c.Status + "@" + c.StatusAt.Format(time.RFC3339) + ":" + c.Note
	}
	if c.Status == "" && c.Activity == ActivityWaiting && c.ActivityAt != nil {
		key += "|turn@" + c.ActivityAt.Format(time.RFC3339)
	}
	if c.Ended != nil {
		key += "|ended@" + c.Ended.Format(time.RFC3339)
	}
	return key
}

// MarkOrchestrated records that the orchestrator is handling crew's latest
// event. It reports false if that event was already handled.
func (s Store) MarkOrchestrated(slug, id string) (bool, error) {
	fresh := false
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			c := &m.Crew[i]
			if c.ID != id {
				continue
			}
			key := c.eventKey()
			if c.Orchestrated == key {
				return nil
			}
			c.Orchestrated, fresh = key, true
		}
		return nil
	})
	return fresh, err
}

// StartOrchestrating records that the orchestrator is at work on the
// mission, looking at what, for at most d. It returns the run, to end it
// with.
func (s Store) StartOrchestrating(slug, what string, d time.Duration) (OrchestratorRun, error) {
	t := now()
	r := OrchestratorRun{For: what, Since: t, Until: t.Add(d)}
	_, err := s.Update(slug, func(m *Mission) error {
		m.Orchestrating = &r
		m.Waiting = nil // each run places the ball again
		return nil
	})
	return r, err
}

// EndOrchestrating records that the orchestrator's run has finished,
// unless another has started since.
func (s Store) EndOrchestrating(slug string, r OrchestratorRun) error {
	_, err := s.Update(slug, func(m *Mission) error {
		if o := m.Orchestrating; o != nil && o.Since.Equal(r.Since) {
			m.Orchestrating = nil
		}
		return nil
	})
	return err
}

// StartLaunching marks ws starting crew on a mission, for what ("step 3",
// or ""): from now until the crew are recorded, which take the mark over
// (StartCrewOn, ResumeCrew), or EndLaunching says none were. One underway
// already stands, and is returned: a step reached and the launch it makes
// are one start.
func (s Store) StartLaunching(slug, what string) (Launching, error) {
	l := Launching{What: what, Since: now()}
	_, err := s.Update(slug, func(m *Mission) error {
		if m.Launching != nil {
			l = *m.Launching
			return nil
		}
		m.Launching = &l
		return nil
	})
	return l, err
}

// EndLaunching ends the mark l, if it still stands: no crew were recorded
// under it (the launch was refused, failed, or queued for you instead).
func (s Store) EndLaunching(slug string, l Launching) error {
	_, err := s.Update(slug, func(m *Mission) error {
		if o := m.Launching; o != nil && o.Since.Equal(l.Since) {
			m.Launching = nil
		}
		return nil
	})
	return err
}

// StartMoving marks ws moving the plan on from step n: from now until
// EndMoving. One underway already stands, and is returned: a step passed
// inside another's passing is one move.
func (s Store) StartMoving(slug string, n int) (Moving, error) {
	mv := Moving{Step: n, Since: now()}
	_, err := s.Update(slug, func(m *Mission) error {
		if m.Moving != nil {
			mv = *m.Moving
			return nil
		}
		m.Moving = &mv
		return nil
	})
	return mv, err
}

// EndMoving ends the mark mv, if it still stands.
func (s Store) EndMoving(slug string, mv Moving) error {
	_, err := s.Update(slug, func(m *Mission) error {
		if o := m.Moving; o != nil && o.Since.Equal(mv.Since) && o.Step == mv.Step {
			m.Moving = nil
		}
		return nil
	})
	return err
}

// StartWritingPR marks the PR writer writing p.Capsule's pull request.
// One underway already stands, and is returned with false: it's one pull
// request, written once.
func (s Store) StartWritingPR(slug string, p WritingPR) (WritingPR, bool, error) {
	started := false
	_, err := s.Update(slug, func(m *Mission) error {
		if o, ok := m.WritingPR(p.Capsule); ok {
			p = o
			return nil
		}
		if p.Since.IsZero() {
			p.Since = now()
		}
		m.WritingPRs = append(m.WritingPRs, p)
		started = true
		return nil
	})
	return p, started, err
}

// EndWritingPR clears the PR writer's mark on capsule's pull request:
// it's opened, or it couldn't be.
func (s Store) EndWritingPR(slug, capsule string) error {
	_, err := s.Update(slug, func(m *Mission) error {
		m.WritingPRs = slices.DeleteFunc(m.WritingPRs, func(p WritingPR) bool { return p.Capsule == capsule })
		return nil
	})
	return err
}

// SetCrewHooks records, as ws starts a session for crew, whether it
// installed its hooks in it: with them the crew report for themselves, so
// they're starting until their first report, however long ago ws handed
// over their prompt (CrewHandedPrompt). Without them nothing's reported as
// a session begins, and handing over their prompt is what starts them.
func (s Store) SetCrewHooks(slug, id string, installed bool) error {
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if c := &m.Crew[i]; c.ID == id && c.Starting != nil {
				c.AwaitsReport, c.Handed = installed, false
			}
		}
		return nil
	})
	return err
}

// CrewHandedPrompt records that ws has done its part of starting crew:
// it's handed them their prompt, or had none to hand them. Crew whose
// session has no hooks of ws's are started by that: it's the last ws
// knows of how they begin. Crew with them are starting until their first
// report (CrewHook), which a session held up before its first turn
// (Claude Code asking whether to trust the folder) hasn't made.
func (s Store) CrewHandedPrompt(slug, id string) error {
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if c := &m.Crew[i]; c.ID == id && c.Starting != nil {
				c.Handed = true
				c.settleStart()
			}
		}
		return nil
	})
	return err
}

// started ends a crew's start: they're no longer starting.
func (c *Crew) started() { c.Starting, c.AwaitsReport, c.Handed = nil, false, false }

// settleStart ends a crew's start once it has nothing left to wait for:
// ws has handed over their prompt, and a session that reports has.
func (c *Crew) settleStart() {
	if c.Handed && !c.AwaitsReport {
		c.started()
	}
}

// StartPlanning marks a turn of the planner's underway in its conversation
// in, as ws asks for it. One underway there already stands: what's said
// while it answers is answered in the same turn.
func (s Store) StartPlanning(slug, in string) (PlannerTurn, error) {
	t := PlannerTurn{In: in, Since: now()}
	_, err := s.Update(slug, func(m *Mission) error {
		if m.Planning != nil && m.Planning.In == in {
			t = *m.Planning
			return nil
		}
		m.Planning = &t
		return nil
	})
	return t, err
}

// EndPlanning ends the planner's turn in in: its reply's recorded, or the
// run that was to have it is over without one.
func (s Store) EndPlanning(slug, in string) error {
	_, err := s.Update(slug, func(m *Mission) error {
		if m.Planning != nil && m.Planning.In == in {
			m.Planning = nil
		}
		return nil
	})
	return err
}

// ErrDispatchLimit is returned when the orchestrator has dispatched as many
// crew in a row as it may without the user.
var ErrDispatchLimit = errors.New("dispatch limit reached")

// CountDispatch counts one more crew dispatched without the user, refusing
// past max.
func (s Store) CountDispatch(slug string, max int) error {
	_, err := s.Update(slug, func(m *Mission) error {
		if m.Dispatches >= max {
			return fmt.Errorf("%w: %d crew dispatched without the user", ErrDispatchLimit, m.Dispatches)
		}
		m.Dispatches++
		return nil
	})
	return err
}

// ReleaseDispatch gives back a count taken for a dispatch that then failed
// to launch, so failures don't use up the orchestrator's allowance.
func (s Store) ReleaseDispatch(slug string) error {
	_, err := s.Update(slug, func(m *Mission) error {
		if m.Dispatches > 0 {
			m.Dispatches--
		}
		return nil
	})
	return err
}

// UserActed records that the user stepped in (launched crew, dismissed a
// flag): the orchestrator's dispatch count starts again.
func (s Store) UserActed(slug string) error {
	_, err := s.Update(slug, func(m *Mission) error {
		m.Dispatches, m.Waiting = 0, nil
		return nil
	})
	return err
}

// ErrDispatchQueued is returned when queueing a dispatch while another one
// is still waiting for the user: one would silently replace the other.
var ErrDispatchQueued = errors.New("a dispatch is already queued")

// QueueDispatch stores a dispatch for the user to launch (nil clears it). It
// refuses to replace one that's still waiting.
func (s Store) QueueDispatch(slug string, d *Dispatch) error {
	if _, err := s.Update(slug, func(m *Mission) error {
		if d != nil && m.Dispatch != nil {
			return fmt.Errorf("%w on %s, waiting for the user: %s", ErrDispatchQueued, slug, m.Dispatch.Text)
		}
		if d != nil && d.At.IsZero() {
			d.At = now()
		}
		m.Dispatch = d
		return nil
	}); err != nil {
		return err
	}
	if d == nil {
		return nil
	}
	return s.AppendLog(slug, Event{Kind: EventDispatchQueued, Crew: d.By, Text: d.Text})
}

// SetCrewDispatch records what crew was dispatched to do.
func (s Store) SetCrewDispatch(slug, id, by, text string) error {
	if _, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			if m.Crew[i].ID == id {
				m.Crew[i].Dispatch = strings.TrimSpace(text)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return s.AppendLog(slug, Event{Kind: EventDispatch, Crew: id, Text: "by " + by + ": " + text})
}

// Hook is what an agent reported through one of its hooks.
type Hook struct {
	// Prompted: the agent was sent a message (by you, or by ws for you). It's
	// working, and a question it asked has been answered. That it reported
	// done stands (TalkedTo says why).
	Prompted bool
	// WakeUp, with Prompted: the prompt is its harness waking it (a
	// background task of its own ended), not a message. It's working, but
	// nobody answered what it asked: that stays too.
	WakeUp bool
	// Activity, when set, replaces the crew's (ActivityWaiting at the end of
	// its turn, ActivityPermission when it asks for one).
	Activity string
	// Work, at the end of its turn (ActivityWaiting), is what it left
	// running, as its harness said (crew.LeftRunning): none when there's
	// none, or the harness can't tell.
	Work   []Work
	Reply  string // its turn's final message, when known
	Notice string // a notification it raised, or the question it asked
	// Options are the answers a question (ActivityQuestion) offered.
	Options []string
	// Conversation is its session id, recorded if the crew has none yet.
	Conversation string
}

// lastReplyMax keeps a crew's last reply to what's worth showing.
const lastReplyMax = 1500

// CrewHook records what crew reported through their hooks. It's how the deck
// knows, rather than guesses, whether crew are working or waiting for you.
func (s Store) CrewHook(slug, id string, h Hook) error {
	_, err := s.Update(slug, func(m *Mission) error {
		for i := range m.Crew {
			c := &m.Crew[i]
			if c.ID != id {
				continue
			}
			t := now()
			// A question it asked with its question tool waits like a
			// permission prompt, and the notification that follows says
			// only that: the question says more.
			aboutQuestion := c.Activity == ActivityQuestion && h.Activity == ActivityPermission
			if h.Prompted || h.Activity != "" {
				c.started() // its first activity: it's started
			} else {
				// Anything else its hooks report (its session's start,
				// first of all) says the session's up: it's started if ws
				// has handed over its prompt, and once it has if not.
				c.AwaitsReport = false
				c.settleStart()
			}
			switch {
			case h.Prompted:
				c.Activity, c.ActivityAt, c.Notice, c.Options = ActivityWorking, &t, "", nil
				c.Pending = nil
				if !h.WakeUp {
					c.answered()
				}
			case aboutQuestion:
			case h.Activity == ActivityQuestion:
				c.Activity, c.ActivityAt, c.Notice, c.Options = h.Activity, &t, h.Notice, h.Options
			case h.Activity != "":
				c.Activity, c.ActivityAt = h.Activity, &t
				c.Options = nil
				if h.Activity == ActivityWaiting {
					c.Pending = pendingAt(h.Work, t)
				}
			}
			if r := strings.TrimSpace(h.Reply); r != "" {
				if rs := []rune(r); len(rs) > lastReplyMax {
					r = string(rs[:lastReplyMax]) + "…"
				}
				c.LastReply = r
			}
			if h.Notice != "" && !aboutQuestion {
				c.Notice = h.Notice
			}
			if h.Conversation != "" && c.Conversation == "" {
				c.Conversation = h.Conversation
			}
			return nil
		}
		return fmt.Errorf("mission %s has no crew %s", slug, id)
	})
	return err
}
