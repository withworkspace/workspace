package ops

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

// StateSet is a mission put into a state by hand, or taken out of one.
type StateSet struct {
	Slug  string
	State mission.State // "" reopened
	// Burned are the capsules (repo/name) archiving burned.
	Burned []string
}

// Label is the state in a word: parked, archived, reopened.
func (s StateSet) Label() string {
	if s.State == "" {
		return "reopened"
	}
	return string(s.State)
}

func (s StateSet) String() string {
	switch s.State {
	case mission.Parked:
		return "Parked."
	case mission.Archived:
		if len(s.Burned) > 0 {
			return "Archived; burned " + strings.Join(s.Burned, ", ") + "."
		}
		return "Archived."
	}
	return "Reopened."
}

// SetState parks or archives a mission, or (state "") takes it out of
// either; or out of how it ended, when it's in neither. Taking it out of
// one in particular is Reopen. Ending it is End. Archiving is Archive,
// burning only what has merged.
func (w *Workspace) SetState(slug string, state mission.State, by Actor) (StateSet, error) {
	switch state {
	case "", mission.Parked:
	case mission.Archived:
		return w.Archive(slug, false, by)
	default:
		return StateSet{}, invalid("a mission is put in %q by what happens to it, not by hand", state)
	}
	m, err := w.load(slug)
	if err != nil {
		return StateSet{}, err
	}
	if state == "" && m.State == "" && m.Ending != nil {
		return w.Unend(m.Slug, by)
	}
	if state == mission.Parked && m.Ending != nil {
		return StateSet{}, refused("ended", "it has ended, %s: there's nothing to park", m.Ending.Ending.Words())
	}
	return w.putIn(m.Slug, state, by)
}

// putIn stores a mission's state, set by hand, and logs it.
func (w *Workspace) putIn(slug string, state mission.State, by Actor) (StateSet, error) {
	if _, err := w.Missions.Update(slug, func(m *mission.Mission) error {
		m.State = state
		return nil
	}); err != nil {
		return StateSet{}, wrap(err)
	}
	out := StateSet{Slug: slug, State: state}
	if err := w.Missions.AppendLog(slug, mission.Event{Kind: mission.EventState, Text: out.Label()}); err != nil {
		return StateSet{}, err
	}
	w.youActed(slug, by)
	return out, nil
}

// Archive puts a mission out of sight, and its capsules out of the repo's
// folder: archived is cleaned up. Nothing else burns a capsule whose pull
// request merged on GitHub, and left, it's a diff against a fork point
// its squash merge left behind for good.
//
// A capsule whose work has all left (whyUnmerged) is burned. When any
// hasn't, archiving is refused, saying which and why, unless burnAll: it
// burns them anyway, their branches kept and what's not committed lost.
// It's refused while crew are at work on the mission, as ending it is.
// A burned capsule's branch is kept by the mission (Burn), so its pull
// request can still be found; reopening the mission doesn't bring the
// capsule back.
func (w *Workspace) Archive(slug string, burnAll bool, by Actor) (StateSet, error) {
	m, err := w.load(slug)
	if err != nil {
		return StateSet{}, err
	}
	for _, c := range m.Crew {
		if c.Ended == nil && c.Status != mission.CrewDone && c.Role != mission.RoleBriefing {
			return StateSet{}, refused("crew-working", "%s is still on it: stop it, or let it report done, before it's archived", c.ID)
		}
	}
	v := BuildMissionViews(w.WS(), w.GitHub(), []*mission.Mission{m})[0]
	var burn []CapsuleView
	var unmerged []string
	for _, c := range v.Capsules {
		if !c.Exists {
			continue
		}
		burn = append(burn, c)
		if why := whyUnmerged(c); why != "" {
			unmerged = append(unmerged, fmt.Sprintf("%s/%s (%s)", c.Repo, c.Name, why))
		}
	}
	if len(unmerged) > 0 && !burnAll {
		which := "1 hasn't merged as it is"
		if len(unmerged) > 1 {
			which = fmt.Sprintf("%d haven't merged as they are", len(unmerged))
		}
		return StateSet{}, refused("unmerged-capsules", "Archiving burns its capsules, and %s: %s. Archive it anyway to burn them: their branches are kept, and what's not committed is lost",
			which, strings.Join(unmerged, ", "))
	}
	// Before their capsules go: a session left open would sit in a folder
	// that's gone.
	w.closeEnded(m, "the mission is archived")
	var burned []string
	for _, c := range burn {
		if _, err := w.Burn(c.Repo, c.Name, BurnOptions{Force: burnAll}); err != nil {
			return StateSet{}, fmt.Errorf("burning %s/%s, so it isn't archived: %w", c.Repo, c.Name, err)
		}
		burned = append(burned, c.Repo+"/"+c.Name)
	}
	out, err := w.putIn(m.Slug, mission.Archived, by)
	out.Burned = burned
	return out, err
}

// whyUnmerged says why a capsule's work hasn't all left: "" when its pull
// request merged at the commit it's at, or it landed from there, and
// nothing in it isn't committed.
func whyUnmerged(c CapsuleView) string {
	switch {
	case c.Landed != nil:
		if c.Head != c.Landed.Head {
			return "it has moved since it landed"
		}
	case c.PRS == mission.PRMerged:
		// GitHub says what a merged pull request's head was: a capsule
		// that's moved on from it has commits that didn't merge.
		if c.PR.HeadRefOid == "" {
			return fmt.Sprintf("GitHub didn't say what #%d merged", c.PR.Number)
		}
		if c.Head != c.PR.HeadRefOid {
			return fmt.Sprintf("it has moved since #%d merged", c.PR.Number)
		}
	case c.PRS == mission.PROpen:
		return fmt.Sprintf("#%d is open", c.PR.Number)
	default:
		return "its work hasn't merged or landed"
	}
	if workspace.GitIsDirty(c.Path) {
		return "it has changes not committed"
	}
	return ""
}

// Reopen takes a mission out of the state from: refused when it isn't in
// it (an undo that's come too late).
func (w *Workspace) Reopen(slug string, from mission.State, by Actor) (StateSet, error) {
	m, err := w.load(slug)
	if err != nil {
		return StateSet{}, err
	}
	if m.State != from {
		return StateSet{}, refused("not-"+string(from), "it isn't %s", from)
	}
	return w.SetState(m.Slug, "", by)
}

// BriefSet is a brief written, with what its stages name that the mission
// doesn't have.
type BriefSet struct {
	Slug     string
	Warnings []string
}

func (b BriefSet) String() string {
	return strings.TrimSpace("Brief saved. " + strings.Join(b.Warnings, " "))
}

// SetBrief replaces a mission's brief. The brief is yours: crew may only
// write it while they're briefing the mission.
func (w *Workspace) SetBrief(slug, text string, by Actor) (BriefSet, error) {
	if strings.TrimSpace(text) == "" {
		return BriefSet{}, invalid("the brief is empty")
	}
	m, err := w.load(slug)
	if err != nil {
		return BriefSet{}, err
	}
	if err := mayEdit(m, by); err != nil {
		return BriefSet{}, err
	}
	if err := w.Missions.WriteBrief(m.Slug, text); err != nil {
		return BriefSet{}, err
	}
	w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventNote, Text: "brief written"})
	w.youActed(m.Slug, by)
	return BriefSet{Slug: m.Slug, Warnings: w.StageWarnings(m, text)}, nil
}

// SetTitle renames a mission (its slug stays). Crew may only while they're
// briefing it.
func (w *Workspace) SetTitle(slug, title string, by Actor) error {
	m, err := w.load(slug)
	if err != nil {
		return err
	}
	if err := mayEdit(m, by); err != nil {
		return err
	}
	if err := w.Missions.SetTitle(m.Slug, title); err != nil {
		return &Invalid{Reason: err.Error()}
	}
	w.youActed(m.Slug, by)
	return nil
}

// RenameMission gives a mission a new slug: its folder moves, and its
// conversations in the records go with it.
func (w *Workspace) RenameMission(old, slug string) error {
	if err := w.Missions.Rename(old, slug); err != nil {
		return err
	}
	if old == slug {
		return nil
	}
	if err := conversation.MoveMission(w.Root(), old, slug); err != nil {
		return fmt.Errorf("renamed, but its conversations weren't moved with it: %w", err)
	}
	return nil
}

// mayEdit refuses crew changing a mission's brief or title once it's
// briefed: then it's the user's.
func mayEdit(m *mission.Mission, by Actor) error {
	if by.Kind == Crew && !m.ToBrief() {
		return refused("brief-is-yours", "the brief belongs to the user once a mission is briefed; record findings with ws mission note")
	}
	return nil
}

// Attached is a capsule added to a mission, and the mission it was taken
// from, if one had it.
type Attached struct {
	Slug, Repo, Capsule string
	From                string
}

func (a Attached) String() string {
	if a.From != "" {
		return fmt.Sprintf("Moved %s/%s from %s.", a.Repo, a.Capsule, a.From)
	}
	return fmt.Sprintf("Attached %s/%s.", a.Repo, a.Capsule)
}

// Attach adds a capsule, with the branch it's on, to a mission. One that
// belongs to another mission is refused, unless move: then it's taken from
// that one.
func (w *Workspace) Attach(slug, repo, capsule string, move bool, by Actor) (Attached, error) {
	if capsule == workspace.GroundDir {
		return Attached{}, invalid("%s can't belong to a mission", workspace.GroundDir)
	}
	m, err := w.load(slug)
	if err != nil {
		return Attached{}, err
	}
	out := Attached{Slug: m.Slug, Repo: repo, Capsule: capsule}
	if owner, ok := w.Missions.FindByCapsule(repo, capsule); ok && owner.Slug != m.Slug {
		if !move {
			return Attached{}, refused("capsule-taken", "%s/%s already belongs to mission %s (use --move)", repo, capsule, owner.Slug)
		}
		if _, err := w.Missions.Update(owner.Slug, func(o *mission.Mission) error {
			o.RemoveCapsule(repo, capsule)
			return nil
		}); err != nil {
			return Attached{}, wrap(err)
		}
		w.Missions.AppendLog(owner.Slug, mission.Event{Kind: mission.EventCapsuleRemove, Repo: repo, Capsule: capsule, Text: "moved to " + m.Slug})
		out.From = owner.Slug
	}
	branch := workspace.GitCurrentBranch(filepath.Join(w.WS().RepoDir(repo), capsule))
	added := false
	if _, err := w.Missions.Update(m.Slug, func(m *mission.Mission) error {
		added = m.AddCapsuleBranch(repo, capsule, branch)
		return nil
	}); err != nil {
		return Attached{}, wrap(err)
	}
	if added {
		if err := w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventCapsuleAdded, Repo: repo, Capsule: capsule}); err != nil {
			return Attached{}, err
		}
		w.youActed(m.Slug, by)
	}
	return out, nil
}

// Detach takes a capsule off a mission. The capsule itself is kept.
func (w *Workspace) Detach(slug, repo, capsule string, by Actor) error {
	m, err := w.load(slug)
	if err != nil {
		return err
	}
	removed := false
	if _, err := w.Missions.Update(m.Slug, func(m *mission.Mission) error {
		removed = m.RemoveCapsule(repo, capsule)
		return nil
	}); err != nil {
		return wrap(err)
	}
	if !removed {
		return refused("not-attached", "%s/%s is not part of mission %s", repo, capsule, m.Slug)
	}
	w.youActed(m.Slug, by)
	return w.Missions.AppendLog(m.Slug, mission.Event{Kind: mission.EventCapsuleRemove, Repo: repo, Capsule: capsule})
}

// BriefsInWeb is whether missions are briefed in a conversation in the web
// UI: unless [briefing] in = "terminal", or no agent here can hold a
// conversation.
func (w *Workspace) BriefsInWeb() bool {
	cfg := w.Config()
	if cfg.BriefInTerminal() {
		return false
	}
	_, _, err := Roles(cfg).ForBriefingConversation()
	return err == nil
}

// Brief has a mission briefed: one in briefing in a conversation in the
// web UI (its briefing conversation, started if there's none, its agent
// answering), as BriefsInWeb says, or else by briefing crew. It says what
// it did.
func (w *Workspace) Brief(slug string, by Actor) (string, error) {
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	if !m.ToBrief() || !w.BriefsInWeb() {
		l, err := w.Launch(slug, Dispatch{Role: mission.RoleBriefing}, by)
		if err != nil {
			return "", err
		}
		return l.String(), nil
	}
	store := conversation.For(w.Root(), m.Slug)
	c := store.Briefing()
	if c == nil {
		if c, err = store.OpenBriefing(firstNonEmpty(m.Request, m.Title), w.now()); err != nil {
			return "", err
		}
		w.answer(job.Answer, doc.OfMission(m.Slug), c.ID)
	}
	return "Briefing in " + c.ID + ".", nil
}

// StopCrew hangs up a mission's running crew, in the daemon that holds it.
func (w *Workspace) StopCrew(slug, id string, by Actor) (string, error) {
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	c, ok := m.CrewByID(id)
	if !ok || c.Ended != nil {
		return "", fmt.Errorf("%s isn't running", firstNonEmpty(id, "that crew"))
	}
	if w.opts.Host == nil {
		return "", errors.New("crew can't be stopped from here")
	}
	if err := w.opts.Host.Stop(slug + "/" + c.ID); err != nil {
		return "", err
	}
	return "Stopping " + c.ID + ".", nil
}
