package ops

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

// Endings: a mission ends landed, answered, handed on or dropped. The
// orchestrator (or crew) suggests how it ends (Conclude), with Z checked,
// each part met or not and how, and you end it (End); a capsule can end
// alone, handed on to another mission or dropped. ws doesn't judge whether
// a part of Z is met: it holds that whoever suggests an ending looked at
// each, and that an ending is what it says (landed is at Z with its work
// merged; handed on names where to). Checking Z isn't asked of you: by
// the time you'd be asked, the work has merged.
//
// One ending is ws's to make itself, unless [missions] end_when_merged is
// off: landed, once nothing else on the mission waits. As the orchestrator
// suggested (endWhenMerged), or, where it had its turn and suggested
// nothing, with the plan done and every pull request merged (endAtZ).
// Merging was yours, and a mission with code is finished once it has
// merged; left for you, finished missions piled up waiting for a click.

// EndAs is how a mission ends, as said: the ending as written, what it
// came to, each part of Z met or not ("<n>: <how>"), and for handed on,
// the missions it goes to.
type EndAs struct {
	Ending     string
	Text       string
	Met, Unmet []string
	To         []string
}

// Concluded is an ending suggested, or a mission ended.
type Concluded struct {
	Slug       string
	Conclusion mission.Conclusion
	Ended      bool
}

func (c Concluded) String() string {
	if c.Ended && c.Conclusion.By == AsWS.By() {
		return "ws ended it, " + c.Conclusion.Summary() + " (every pull request has merged, and nothing else on it waits)"
	}
	if c.Ended {
		return "Ended, " + c.Conclusion.Summary()
	}
	return "Suggested ending it, " + c.Conclusion.Summary() + ": the user ends it."
}

// Conclude suggests how a mission ends, with Z checked, for you to end it
// (or not): the orchestrator's when the plan reaches Z or stops early, and
// crew's or yours. It's refused when the ending isn't what it says, or Z
// isn't checked part by part.
func (w *Workspace) Conclude(slug string, as EndAs, by Actor) (Concluded, error) {
	m, err := w.load(slug)
	if err != nil {
		return Concluded{}, err
	}
	if err := stillOpen(m); err != nil {
		return Concluded{}, err
	}
	c, err := w.conclusion(m, as, by)
	if err != nil {
		return Concluded{}, err
	}
	if err := w.Missions.SetConclude(m.Slug, &c); err != nil {
		return Concluded{}, wrap(err)
	}
	w.youActed(m.Slug, by)
	if by.Kind == Orchestrator {
		if ended, why := w.endWhenMerged(m.Slug); why == "" {
			return ended, nil
		}
	}
	return Concluded{Slug: m.Slug, Conclusion: c}, nil
}

// endWhenMerged ends a mission as the orchestrator suggested, landed, once
// nothing else holds it (notEndingLanded), where ws holds end_when_merged.
// It says why not, when it doesn't: "" is ended.
func (w *Workspace) endWhenMerged(slug string) (Concluded, string) {
	if !w.Config().EndWhenMerged() {
		return Concluded{}, "end_when_merged is off"
	}
	m, err := w.load(slug)
	if err != nil {
		return Concluded{}, err.Error()
	}
	if why := w.notEndingLanded(m); why != "" {
		return Concluded{}, why
	}
	c := *m.Conclude
	c.By, c.At = AsWS.By(), w.now()
	if err := w.Missions.End(m.Slug, c); err != nil {
		return Concluded{}, err.Error()
	}
	return Concluded{Slug: m.Slug, Conclusion: c, Ended: true}, ""
}

// notEndingLanded says what holds a mission from ending landed as the
// orchestrator suggested, cheapest first: "" when nothing does. Whatever
// waits on you (a question, a step of yours, crew queued), crew still at
// it, a Plan whose build is yours to decide, Z changed since, or work not
// yet merged keeps it for you.
func (w *Workspace) notEndingLanded(m *mission.Mission) string {
	c := m.Conclude
	switch {
	case m.Ending != nil:
		return "it has ended"
	case c == nil || c.Ending != mission.EndLanded:
		return "nothing suggests it landed"
	case c.By != AsOrchestrator.By():
		return "the orchestrator didn't suggest it"
	}
	if why := heldOpen(m); why != "" {
		return why
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	if !sameZ(m.Z(brief), c.Z) {
		return "Z has changed since it was suggested"
	}
	return w.notLanded(m)
}

// heldOpen says what on a mission keeps ws from ending it itself: "" when
// nothing does. It's parked, something waits on you (a question, a step of
// yours, crew queued), crew are still at it, or it wrote a Plan whose
// build is yours to decide.
func heldOpen(m *mission.Mission) string {
	switch {
	case m.State == mission.Parked:
		return "it's parked"
	case len(m.Tasks) > 0:
		return "it waits on the user for something"
	case m.Dispatch != nil:
		return "crew are queued for the user to launch"
	}
	for _, cr := range m.Crew {
		if cr.Ended == nil && cr.Status != mission.CrewDone && cr.Role != mission.RoleBriefing {
			return cr.ID + " is still on it"
		}
	}
	for _, a := range m.Artifacts {
		if a.Kind == planKind {
			return "it wrote a Plan, and whether it's built is the user's to say"
		}
	}
	return ""
}

// endAtZ ends a mission landed that nothing suggested an ending for: every
// step of its plan is passed, its work has all merged or landed, and
// nothing else on it waits (heldOpen), where ws holds end_when_merged. Z
// goes unchecked: nobody checked it. The orchestrator has its turn first:
// it isn't ended while a run of its is underway, but the one that began
// at run, which is asking as it finishes. It says why not, when it
// doesn't: "" is ended.
func (w *Workspace) endAtZ(slug string, run time.Time) (Concluded, string) {
	if !w.Config().EndWhenMerged() {
		return Concluded{}, "end_when_merged is off"
	}
	m, err := w.load(slug)
	if err != nil {
		return Concluded{}, err.Error()
	}
	switch {
	case m.Ending != nil:
		return Concluded{}, "it has ended"
	case m.Conclude != nil:
		return Concluded{}, "an ending is suggested"
	case m.Orchestrating != nil && !m.Orchestrating.Since.Equal(run):
		return Concluded{}, "the orchestrator is looking at it"
	}
	if why := heldOpen(m); why != "" {
		return Concluded{}, why
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	if !planDone(m, brief) {
		return Concluded{}, "its plan has steps to go"
	}
	if why := w.notLanded(m); why != "" {
		return Concluded{}, why
	}
	c := mission.Conclusion{Ending: mission.EndLanded, Text: landedUnsaid, Z: mission.Conclusion{}.OnZ(m.Z(brief)), By: AsWS.By(), At: w.now()}
	if err := w.Missions.End(m.Slug, c); err != nil {
		return Concluded{}, err.Error()
	}
	return Concluded{Slug: m.Slug, Conclusion: c, Ended: true}, ""
}

// What a mission came to, where nobody said: its plan, done.
const (
	landedUnsaid   = "Every step of the plan done, and its work merged."
	answeredUnsaid = "Every step of the plan done."
	handedOnUnsaid = "Its work handed on."
)

// planDone reports whether the mission has a plan, with every step of it
// passed.
func planDone(m *mission.Mission, brief string) bool {
	if m.Plan == nil || len(m.Plan.Steps) == 0 {
		return false
	}
	states, _ := m.StageProgress(brief)
	_, stepToGo := mission.CurrentStage(states)
	return !stepToGo
}

// unsuggested is how a mission ends when you end it and nothing suggested
// how: with every step of its plan passed, landed where its work has
// merged or landed, handed on where its capsules all were, answered where
// it never had any. Z goes unchecked. Short of that it's refused: what's
// left is to drop it, or to say how it ends.
func (w *Workspace) unsuggested(m *mission.Mission) (mission.Conclusion, error) {
	brief, _ := w.Missions.ReadBrief(m.Slug)
	if !planDone(m, brief) {
		return mission.Conclusion{}, refused("nothing-suggested", "nothing suggests how it ends, and its plan isn't done: drop it (ws mission end --as dropped), or say how it ends (--as answered, --as handed-on)")
	}
	c := mission.Conclusion{Z: mission.Conclusion{}.OnZ(m.Z(brief))}
	why := w.notLanded(m)
	switch to := m.HandedOnTo(); {
	case why == "":
		c.Ending, c.Text = mission.EndLanded, landedUnsaid
	case len(m.Capsules) == 0 && len(to) > 0:
		c.Ending, c.Text, c.To = mission.EndHandedOn, handedOnUnsaid, to
	case len(m.Capsules) == 0:
		c.Ending, c.Text = mission.EndAnswered, answeredUnsaid
	default:
		return mission.Conclusion{}, refused("not-landed", "%s: it ends landed once its work has merged or landed. To give it up, drop it (ws mission end --as dropped)", why)
	}
	return c, nil
}

// planKind is the document kind of a Plan, for something to be built: a
// mission that wrote one isn't ended until you've said whether it's built.
const planKind = "plan"

// End ends a mission: as its suggested ending (as nil), or as you say.
// With nothing suggested and every step of its plan passed, it ends as
// its work stands (unsuggested). Nothing's asked of you about Z: a
// suggestion made on a Z that's changed since keeps the checks of the
// parts still there. It's yours alone, and refused while crew are still
// at work on it.
func (w *Workspace) End(slug string, as *EndAs, by Actor) (Concluded, error) {
	if !by.IsYou() {
		return Concluded{}, refused("not-yours", "ending a mission is the user's: suggest how it ends, with Z checked, with ws mission conclude")
	}
	m, err := w.load(slug)
	if err != nil {
		return Concluded{}, err
	}
	if err := stillOpen(m); err != nil {
		return Concluded{}, err
	}
	for _, c := range m.Crew {
		if c.Ended == nil && c.Status != mission.CrewDone && c.Role != mission.RoleBriefing {
			return Concluded{}, refused("crew-working", "%s is still on it: stop it, or let it report done, before the mission ends", c.ID)
		}
	}
	var c mission.Conclusion
	switch {
	case as != nil:
		if c, err = w.conclusion(m, *as, by); err != nil {
			return Concluded{}, err
		}
	case m.Conclude == nil || m.Conclude.Ending == "":
		if c, err = w.unsuggested(m); err != nil {
			return Concluded{}, err
		}
	default:
		c = *m.Conclude
		brief, _ := w.Missions.ReadBrief(m.Slug)
		if z := m.Z(brief); !sameZ(z, c.Z) && !(c.Ending == mission.EndDropped && len(c.Z) == 0) {
			c.Z = c.OnZ(z)
		}
	}
	c.By, c.At = by.By(), w.now()
	if err := w.Missions.End(m.Slug, c); err != nil {
		return Concluded{}, wrap(err)
	}
	w.closeEnded(m, "the mission has ended")
	w.youActed(m.Slug, by)
	return Concluded{Slug: m.Slug, Conclusion: c, Ended: true}, nil
}

// Unend takes back a mission's ending (an undo, or reopening it): it's in
// play again, where it was.
func (w *Workspace) Unend(slug string, by Actor) (StateSet, error) {
	if !by.IsYou() {
		return StateSet{}, refused("not-yours", "reopening a mission is the user's")
	}
	m, err := w.load(slug)
	if err != nil {
		return StateSet{}, err
	}
	if m.Ending == nil {
		return StateSet{}, refused("not-ended", "it hasn't ended")
	}
	if err := w.Missions.Unend(m.Slug, by.By()); err != nil {
		return StateSet{}, wrap(err)
	}
	w.youActed(m.Slug, by)
	return StateSet{Slug: m.Slug}, nil
}

// stillOpen refuses ending a mission that has.
func stillOpen(m *mission.Mission) error {
	if e := m.Ending; e != nil {
		return refused("ended", "it has ended already, %s: ws mission reopen takes that back", e.Ending.Words())
	}
	return nil
}

// conclusion is how a mission ends as said, held to what its ending
// needs: Z checked part by part, landed only at Z with its work merged or
// landed, handed on only to missions there are. Checking Z is for whoever
// suggests an ending: dropping it needn't, and neither need you, whose
// ending leaves unchecked the parts you don't speak of, and whose
// dropping needn't say why.
func (w *Workspace) conclusion(m *mission.Mission, as EndAs, by Actor) (mission.Conclusion, error) {
	ending, err := mission.ParseEnding(as.Ending)
	if err != nil {
		return mission.Conclusion{}, invalid("%s", err)
	}
	text := strings.TrimSpace(as.Text)
	if text == "" && !(by.IsYou() && ending == mission.EndDropped) {
		return mission.Conclusion{}, invalid("say what it came to: the finding, what landed, why it stops")
	}
	brief, _ := w.Missions.ReadBrief(m.Slug)
	checks, err := mission.CheckZ(m.Z(brief), as.Met, as.Unmet, ending == mission.EndDropped || by.IsYou())
	if err != nil {
		return mission.Conclusion{}, invalid("%s", err)
	}
	c := mission.Conclusion{Ending: ending, Text: text, Z: checks, By: by.By(), At: w.now()}
	if len(as.To) > 0 && ending != mission.EndHandedOn {
		return mission.Conclusion{}, invalid("--to names the missions its work is handed on to: it's for handed on")
	}
	switch ending {
	case mission.EndLanded:
		for i, z := range checks {
			if z.Checked() && !z.Met {
				return mission.Conclusion{}, refused("not-at-z", "landed is at Z, and part %d of Z isn't met (%s): end it answered, handed on or dropped, saying why it stops before Z", i+1, z.How)
			}
		}
		if why := w.notLanded(m); why != "" {
			return mission.Conclusion{}, refused("not-landed", "%s: it ends landed once its work has merged or landed. A finding with no code ends answered", why)
		}
	case mission.EndHandedOn:
		for _, to := range as.To {
			t, err := w.load(to)
			if err != nil {
				return mission.Conclusion{}, invalid("there's no mission %s to hand it on to: raise it with ws mission new first", to)
			}
			if t.Slug == m.Slug {
				return mission.Conclusion{}, invalid("it can't be handed on to itself")
			}
			if !slices.Contains(c.To, t.Slug) {
				c.To = append(c.To, t.Slug)
			}
		}
		for _, to := range m.HandedOnTo() {
			if !slices.Contains(c.To, to) {
				c.To = append(c.To, to)
			}
		}
		if len(c.To) == 0 {
			return mission.Conclusion{}, invalid("say which missions it's handed on to (--to <mission>), or hand its capsules on first (ws mission hand-on)")
		}
	}
	return c, nil
}

// notLanded says what of the mission's work hasn't merged or landed, as
// far as GitHub says; "" when it all has.
func (w *Workspace) notLanded(m *mission.Mission) string {
	v := BuildMissionViews(w.WS(), w.GitHub(), []*mission.Mission{m})[0]
	facts := map[mission.Capsule]mission.CapsuleFacts{}
	var open []string
	for _, c := range v.Capsules {
		facts[capsuleKey(c.Capsule)] = mission.CapsuleFacts{Exists: c.Exists, PR: c.PRS, Landed: c.Landed != nil}
		switch {
		case c.Landed != nil, c.PRS == mission.PRMerged:
		case c.PRS == mission.PROpen:
			open = append(open, fmt.Sprintf("%s's pull request (%s#%d) is open", c.Repo+"/"+c.Name, c.Repo, c.PR.Number))
		case c.Exists:
			open = append(open, c.Repo+"/"+c.Name+" has neither merged nor landed")
		}
	}
	if len(open) > 0 {
		return strings.Join(open, "; ")
	}
	if !mission.CapsulesLanded(m, func(c mission.Capsule) mission.CapsuleFacts { return facts[capsuleKey(c)] }) {
		return "nothing of it has merged or landed"
	}
	return ""
}

// sameZ reports whether Z as checked is Z as it is.
func sameZ(z []string, checked []mission.ZCheck) bool {
	if len(z) != len(checked) {
		return false
	}
	for i := range z {
		if z[i] != checked[i].Z {
			return false
		}
	}
	return true
}

// CapsuleEnded is a capsule that ended alone: handed on to a mission
// (raised for it, or one there was), or dropped.
type CapsuleEnded struct {
	Slug, Capsule string
	Ending        mission.Ending
	To            string
	Raised        bool // To was raised for it
}

func (c CapsuleEnded) String() string {
	switch {
	case c.Ending == mission.EndDropped:
		return fmt.Sprintf("Dropped %s: it's no longer the mission's (its worktree is kept: ws burn removes it).", c.Capsule)
	case c.Raised:
		return fmt.Sprintf("Handed %s on to %s, raised for it: it waits to be briefed.", c.Capsule, c.To)
	}
	return fmt.Sprintf("Handed %s on to %s.", c.Capsule, c.To)
}

// HandOnCapsule hands one of a mission's capsules on to another mission,
// which takes it: to (one there is), or one raised for it now from what.
// It no longer holds this mission open. Yours, and the orchestrator's (a
// plan may say to hand a failure on); crew raise follow-up work with ws
// mission new.
func (w *Workspace) HandOnCapsule(slug, capsule, what, to string, by Actor) (CapsuleEnded, error) {
	if by.Kind != You && by.Kind != Orchestrator {
		return CapsuleEnded{}, refused("not-yours", "handing a capsule on is the orchestrator's or the user's: raise follow-up work with ws mission new")
	}
	m, c, err := w.endingCapsule(slug, capsule)
	if err != nil {
		return CapsuleEnded{}, err
	}
	what = strings.TrimSpace(what)
	out := CapsuleEnded{Slug: m.Slug, Capsule: c.Repo + "/" + c.Name, Ending: mission.EndHandedOn}
	if to != "" {
		t, err := w.load(to)
		if err != nil {
			return CapsuleEnded{}, invalid("there's no mission %s to hand %s on to", to, out.Capsule)
		}
		if t.Slug == m.Slug {
			return CapsuleEnded{}, invalid("%s is this mission's already", out.Capsule)
		}
		out.To = t.Slug
	} else {
		if what == "" {
			return CapsuleEnded{}, invalid("say what the mission it's handed on to is for: it's raised from that, to be briefed")
		}
		n, err := w.Missions.CreateBriefing(what)
		if err != nil {
			return CapsuleEnded{}, wrap(err)
		}
		logFailed(w.Missions.Update(n.Slug, func(x *mission.Mission) error {
			x.From = m.Slug
			return nil
		}))
		w.Missions.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Text: "raised mission " + n.Slug + ": " + what})
		out.To, out.Raised = n.Slug, true
	}
	if err := w.Missions.EndCapsule(m.Slug, mission.EndedCapsule{Repo: c.Repo, Name: c.Name, Branch: c.Branch, Ending: mission.EndHandedOn, To: out.To, Why: what, By: by.By()}); err != nil {
		return CapsuleEnded{}, wrap(err)
	}
	if _, err := w.Missions.Update(out.To, func(x *mission.Mission) error {
		x.AddCapsuleBranch(c.Repo, c.Name, c.Branch)
		return nil
	}); err != nil {
		return CapsuleEnded{}, wrap(err)
	}
	w.Missions.AppendLogAfter(out.To, mission.Event{Kind: mission.EventCapsuleAdded, Repo: c.Repo, Capsule: c.Name, Text: "handed on from " + m.Slug})
	w.youActed(m.Slug, by)
	return out, nil
}

// DropCapsule drops one of a mission's capsules: its work won't be part of
// the mission, and it no longer holds it open. Its worktree is kept, in no
// mission. Yours alone.
func (w *Workspace) DropCapsule(slug, capsule, why string, by Actor) (CapsuleEnded, error) {
	if !by.IsYou() {
		return CapsuleEnded{}, refused("not-yours", "dropping a capsule is the user's: flag them, or hand it on (ws mission hand-on)")
	}
	why = strings.TrimSpace(why)
	if why == "" {
		return CapsuleEnded{}, invalid("say why it's dropped")
	}
	m, c, err := w.endingCapsule(slug, capsule)
	if err != nil {
		return CapsuleEnded{}, err
	}
	if err := w.Missions.EndCapsule(m.Slug, mission.EndedCapsule{Repo: c.Repo, Name: c.Name, Branch: c.Branch, Ending: mission.EndDropped, Why: why, By: by.By()}); err != nil {
		return CapsuleEnded{}, wrap(err)
	}
	w.youActed(m.Slug, by)
	return CapsuleEnded{Slug: m.Slug, Capsule: c.Repo + "/" + c.Name, Ending: mission.EndDropped}, nil
}

// endingCapsule is the mission and its capsule (repo/name, the repo by
// any of its names) that's to end alone: refused while crew are at work
// in it, or once the mission has ended.
func (w *Workspace) endingCapsule(slug, capsule string) (*mission.Mission, mission.Capsule, error) {
	m, err := w.load(slug)
	if err != nil {
		return nil, mission.Capsule{}, err
	}
	if err := stillOpen(m); err != nil {
		return nil, mission.Capsule{}, err
	}
	repo, name, ok := strings.Cut(strings.Trim(strings.TrimSpace(capsule), "`"), "/")
	if !ok || repo == "" || name == "" {
		return nil, mission.Capsule{}, invalid("name the capsule as repo/name")
	}
	if r, ok := w.WS().ResolveAlias(repo); ok {
		repo = r
	}
	i := slices.IndexFunc(m.Capsules, func(c mission.Capsule) bool { return c.Repo == repo && c.Name == name })
	if i < 0 {
		return nil, mission.Capsule{}, refused("not-attached", "%s/%s isn't one of the mission's capsules", repo, name)
	}
	c := m.Capsules[i]
	for _, cr := range m.Crew {
		if cr.Capsule == repo+"/"+name && cr.Ended == nil && cr.Status != mission.CrewDone {
			return nil, mission.Capsule{}, refused("crew-working", "%s is at work in %s/%s: it ends alone once they've stopped, or reported done", cr.ID, repo, name)
		}
	}
	return m, c, nil
}
