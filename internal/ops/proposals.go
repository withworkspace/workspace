package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Proposals: what a conversation's agent proposes (a brief, a mission, a
// decision, crew dispatched, a replan, a review comment, a document), accepted
// (done, as you, with your edits) or declined.

// ConversationStore is where a conversation is kept: the workspace's (w3),
// or a mission's (t3, its slug given).
func ConversationStore(root, slug, id string) (conversation.Store, error) {
	switch {
	case conversation.IsWorkspaceRef(id):
		return conversation.ForWorkspace(root), nil
	case conversation.IsRef(id) && slug != "":
		return conversation.For(root, slug), nil
	}
	return conversation.Store{}, fmt.Errorf("%q isn't a conversation here", id)
}

// LiftForMission lifts a capsule for a mission, joined to it, as ws lift
// does, as you; tests replace it.
var LiftForMission = func(w *Workspace, slug, repo, branch string) error {
	repo, err := w.ResolveRepo(repo)
	if err != nil {
		return err
	}
	if err := w.EnsureCloned(repo); err != nil {
		return err
	}
	c, err := w.Lift(repo, branch, "", CapsuleOptions{Mission: slug, By: AsYou})
	return madeQuietly(c, err)
}

// liftOrJoin gives a mission repo's capsule for branch: the one there,
// when it's lifted already (by you, or by the conversation it came out
// of), joined to it; else one lifted for it. A capsule another mission
// has is refused, not taken. joined says it was there.
func (w *Workspace) liftOrJoin(slug, repo, branch string) (joined bool, err error) {
	name := workspace.CapsuleName(branch)
	if !exists(filepath.Join(w.WS().RepoDir(repo), name)) {
		return false, LiftForMission(w, slug, repo, branch)
	}
	_, err = w.Attach(slug, repo, name, false, AsYou)
	return err == nil, err
}

// DockForMission docks existing work (a branch, or a PR's by its number)
// for a mission, joined to it, as ws dock does, as you; tests replace it.
var DockForMission = func(w *Workspace, slug, repo, ref string) error {
	repo, err := w.ResolveRepo(repo)
	if err != nil {
		return err
	}
	branch := ref
	if n, ok := isPRNumber(ref); ok {
		pr, err := w.GitHub().PRFromNumber(w.WS().RemoteOf(repo).Owner, w.WS().RemoteOf(repo).Name, n)
		if err != nil {
			return err
		}
		branch = pr.HeadRefName
	}
	if err := w.EnsureCloned(repo); err != nil {
		return err
	}
	c, err := w.Dock(repo, branch, CapsuleOptions{Mission: slug, By: AsYou})
	return madeQuietly(c, err)
}

// madeQuietly is a capsule made for a mission with nobody watching: what
// didn't go to plan is an error, as the mission must have it.
func madeQuietly(c Capsule, err error) error {
	switch {
	case err != nil:
		return err
	case c.AttachErr != nil:
		return fmt.Errorf("made %s, but it didn't join the mission: %w", c.Path, c.AttachErr)
	case c.HookErr != nil:
		return fmt.Errorf("made %s, but its after_create hook failed: %w", c.Path, c.HookErr)
	}
	return nil
}

// SettleProposalIn accepts (doing it, as you) or declines a proposal in a
// conversation, and says what it did.
func (w *Workspace) SettleProposalIn(store conversation.Store, id string, n int, accept bool, edits ProposalEdits) (string, error) {
	said, _, err := w.settleProposalAt(store, id, n, accept, edits)
	return said, err
}

// settleProposalAt is settleProposal, saying too the slug of the
// conversation's mission after it: briefing may rename it. Once it's
// settled, its agent's told (tellAgent), even if what accepting it went on
// to do went wrong.
func (w *Workspace) settleProposalAt(store conversation.Store, id string, n int, accept bool, edits ProposalEdits) (string, string, error) {
	c, err := store.Load(id)
	if err != nil {
		return "", store.Slug, err
	}
	p := c.Proposal(n)
	if p == nil {
		return "", store.Slug, fmt.Errorf("%w: %s has no proposal %d", conversation.ErrNotFound, id, n)
	}
	if p.State != conversation.ProposalOpen {
		return "", store.Slug, fmt.Errorf("proposal %d in %s is %s already", n, id, p.State)
	}
	said, slug, err := w.settle(store, c, p, accept, edits)
	w.tellAgent(store, slug, id, n, edits)
	return said, slug, err
}

// settle accepts or declines an open proposal, saying what it did and the
// slug of the conversation's mission after.
func (w *Workspace) settle(store conversation.Store, c *conversation.Conversation, p *conversation.Proposal, accept bool, edits ProposalEdits) (string, string, error) {
	if !accept {
		_, err := store.SettleProposal(c.ID, p.N, conversation.ProposalDeclined, "", conversation.You, time.Now())
		return "Declined.", store.Slug, err
	}
	if p.Kind == conversation.ProposePlugin {
		said, err := w.acceptPlugin(store, c, *p, edits.Items)
		return said, store.Slug, err
	}
	if p.Kind == conversation.ProposeBrief {
		return w.acceptBrief(store, c, *p, edits)
	}
	if p.Kind == conversation.ProposePlan {
		said, err := w.acceptPlan(store, c, *p, edits)
		return said, store.Slug, err
	}
	said, err := w.acceptOther(store, c, p, edits)
	return said, store.Slug, err
}

func (w *Workspace) acceptOther(store conversation.Store, c *conversation.Conversation, p *conversation.Proposal, edits ProposalEdits) (string, error) {
	p.What, p.Why = firstNonEmpty(edits.What, p.What), firstNonEmpty(edits.Why, p.Why)
	p.Title, p.Brief = firstNonEmpty(edits.Title, p.Title), firstNonEmpty(edits.Brief, p.Brief)
	p.Text, p.Note = firstNonEmpty(edits.Text, p.Text), firstNonEmpty(edits.Note, p.Note)
	switch p.Kind {
	case conversation.ProposeDecision:
		return w.acceptDecision(store, c, *p)
	case conversation.ProposeDispatch:
		return w.acceptDispatch(store, c, *p)
	case conversation.ProposeReplan:
		return w.acceptReplan(store, c, *p)
	case conversation.ProposeComment:
		return w.acceptComment(store, c, *p)
	case conversation.ProposeDocument:
		return w.acceptDocument(store, c, *p)
	case conversation.ProposeMission:
		return w.acceptMission(store, c, *p)
	case conversation.ProposeArtifact:
		return w.acceptArtifact(store, c, *p)
	}
	// A kind ws no longer has (an edit to the process, from when it was
	// read back): it can be declined, not accepted.
	return "", &Refused{Code: "gone", Reason: fmt.Sprintf("proposal %d in %s is %s, which ws doesn't take any more: decline it", p.N, c.ID, firstNonEmpty(p.Kind, "of no kind"))}
}

// acceptArtifact adds a proposed artifact, or a version of one, as you:
// the version says it came from the conversation (its proposal's number,
// and the conversation's title as it is). A version proposed on one
// that's been passed since is still added, based on the one it was
// proposed on. In a mission's conversation, the artifact is recorded on
// the mission too, as crew's are.
func (w *Workspace) acceptArtifact(store conversation.Store, c *conversation.Conversation, p conversation.Proposal) (string, error) {
	from := c.ID
	if !store.Workspace() {
		from = store.Slug + "/" + c.ID
	}
	title := c.Title
	if title == "" && len(c.Turns) > 0 {
		title = clipLine(c.Turns[0].Body, 80)
	}
	add := ArtifactAdd{HTML: []byte(p.HTML), Title: p.Title, Note: p.Note, To: p.To, On: p.On, From: from, FromTitle: title, Proposal: p.N}
	if p.Tree != "" {
		// More than its page: its files, as they were kept when it was
		// proposed.
		var err error
		if add.Files, err = w.Artifacts().ProposedFiles(context.Background(), p.Tree); err != nil {
			return "", invalid("not added: %v", err)
		}
	}
	added, err := w.AddArtifact(add, AsYou)
	if err != nil {
		return "", err
	}
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, added.Ref(), conversation.You, time.Now()); err != nil {
		return "", err
	}
	said := added.String()
	if !store.Workspace() {
		rec := mission.Artifact{Ref: added.Entry.ID, Kind: ArtifactKind, Label: added.Entry.Title}
		if err := w.Missions.AddArtifact(store.Slug, rec); err == nil {
			said += ", and recorded it on " + store.Slug
		}
	}
	return said + ".", nil
}

// acceptDocument adds a proposed document, from its text as accepted (your
// edit, or the agent's), where its kind says, as you: its path and the
// commit's first line are worked out again from that text. A landing that
// stops adds nothing, and leaves the proposal to accept again.
func (w *Workspace) acceptDocument(store conversation.Store, c *conversation.Conversation, p conversation.Proposal) (string, error) {
	from := "conversation " + c.ID
	if !store.Workspace() {
		from += " on mission " + store.Slug
	}
	if p.To != "" {
		return w.acceptAmend(store, c, p, from)
	}
	added, err := w.AddDocument(DocAdd{Kind: p.DocKind, Text: p.Text, Why: fmt.Sprintf("Proposed in %s by its agent, and added by the user.", from)})
	if err != nil {
		return "", err
	}
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, added.Ref, conversation.You, time.Now()); err != nil {
		return "", err
	}
	if !store.Workspace() {
		w.Missions.AppendLogAfter(store.Slug, mission.Event{Kind: mission.EventNote, Text: "added " + added.Ref + " from " + c.ID + ": " + added.Title})
	}
	return "Added " + added.Ref + strings.TrimPrefix(added.Message, "Added"), nil
}

// acceptAmend saves a proposed edit to a document (its text as accepted:
// your edit, or the agent's) where the document is, as an edit saved on its
// page is, as you: landed, in a repo with no pull requests, else committed
// in a capsule. If the document's changed since it was proposed, the edit
// is merged into what's there, from the text it was proposed on: where
// the two change the same lines, nothing's saved, and it's refused, to be
// proposed again from the document as it is. Your draft of the document,
// if you have one open, stays.
func (w *Workspace) acceptAmend(store conversation.Store, c *conversation.Conversation, p conversation.Proposal, from string) (string, error) {
	saved, err := w.AmendDocument(p.To, p.Was, p.Text, fmt.Sprintf("Proposed in %s by its agent, and accepted by the user.", from))
	if err != nil {
		return "", err
	}
	if saved.Conflict != nil {
		return "", &Refused{Code: "changed", Reason: fmt.Sprintf("%s has changed since this was proposed, in lines it changes too (%d %s): nothing's saved. Ask in %s for it again, from the document as it is now", p.To, saved.Conflict.Clashes, plural(saved.Conflict.Clashes, "place"), c.ID)}
	}
	// Where it's saved: a new capsule's, for a repo that takes pull
	// requests; else where it was.
	ref := firstNonEmpty(saved.Ref, p.To)
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, ref, conversation.You, time.Now()); err != nil {
		return "", err
	}
	if !store.Workspace() {
		w.Missions.AppendLogAfter(store.Slug, mission.Event{Kind: mission.EventNote, Text: "amended " + ref + " from " + c.ID + ": " + p.Title})
	}
	if rest, ok := strings.CutPrefix(saved.Message, "Saved"); ok {
		return "Amended " + ref + rest, nil
	}
	return saved.Message, nil // nothing to save: it's as it was
}

// acceptBrief writes a proposed brief (as you edited it, if you did), if
// the brief is still what it was when it was proposed: else it's refused,
// to be proposed again. Its plan (as you edited it, if you did), if it
// comes with one, is written with it, if the plan too is as it was
// (and, under way, it changes only what's ahead). Briefing the mission, it
// also names it, gives it a better slug, lifts and docks its capsules, and
// confirms it, which starts it; under way, a new goal and its plan move
// the mission on. It says what it did, and the mission's slug after.
func (w *Workspace) acceptBrief(store conversation.Store, c *conversation.Conversation, p conversation.Proposal, edits ProposalEdits) (string, string, error) {
	p.Title, p.Brief = firstNonEmpty(edits.Title, p.Title), firstNonEmpty(edits.Brief, p.Brief)
	// The plan it comes with is yours to edit too, as the brief is (on the
	// brief's own page, both are).
	if p.Plan != "" {
		p.Plan = firstNonEmpty(edits.Plan, p.Plan)
	}
	ms := w.Missions
	m, err := ms.Load(store.Slug)
	if err != nil {
		return "", store.Slug, err
	}
	// Looked at here to say so before anything's worked out, and again
	// as they're written (WriteFilesIf), with nothing in between.
	changed := func(file string) error {
		return fmt.Errorf("the %s has changed since this was proposed: ask in %s for it again, from the %s as it is now", file, c.ID, file)
	}
	now, _ := ms.ReadBrief(m.Slug)
	if now != p.Was {
		return "", m.Slug, changed("brief")
	}
	var plan *mission.Plan
	if p.Plan != "" {
		if now, _ := ms.ReadPlan(m.Slug); now != p.PlanWas {
			return "", m.Slug, changed("plan")
		}
		// Its quotes were held to the process when it was proposed.
		if plan, err = mission.ParsePlan(p.Plan, nil); err != nil {
			return "", m.Slug, &Invalid{Reason: err.Error()}
		}
	}
	// Under way, a new goal's plan is a revision: the steps behind the
	// mission stay as they were.
	var rev mission.Revision
	if plan != nil && !m.ToBrief() && m.Plan != nil {
		states, _ := m.StageProgress(now)
		rev = mission.Revise(m.Plan, plan, states)
		if prob := rev.Problem(); prob != "" {
			return "", m.Slug, refused("steps-behind", "%s: ask in %s for it again, changing only what's ahead", prob, c.ID)
		}
	}
	// The brief as it is comes with a plan, for a mission briefed when it
	// was proposed: only the plan is new.
	same := p.Brief == now
	writes := []mission.FileWrite{{Was: p.Was, Now: p.Brief}}
	if plan != nil {
		writes = append(writes, mission.FileWrite{Plan: true, Was: p.PlanWas, Now: plan.Text})
	}
	if err := ms.WriteFilesIf(m.Slug, writes...); err != nil {
		if ch := (*mission.ChangedError)(nil); errors.As(err, &ch) {
			return "", m.Slug, changed(ch.File)
		}
		return "", m.Slug, err
	}
	if !same {
		ms.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Text: "brief written, from " + c.ID})
	}
	said := []string{"Wrote the new brief."}
	if plan != nil {
		ms.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Text: fmt.Sprintf("plan written, from %s: %d %s", c.ID, len(plan.Steps), plural(len(plan.Steps), "step"))})
		said = []string{fmt.Sprintf("Wrote the new brief, and its plan (%d %s).", len(plan.Steps), plural(len(plan.Steps), "step"))}
		if same {
			said = []string{fmt.Sprintf("Wrote its plan (%d %s).", len(plan.Steps), plural(len(plan.Steps), "step"))}
		}
	}
	ms.UserActed(m.Slug)
	// Briefing it, accepting this is the go-ahead: ws is starting it from
	// here, through naming it and lifting its capsules, and that's marked
	// before the proposal stops being yours to confirm.
	slug := m.Slug
	endLaunching := w.launchingOnConfirm(slug)
	defer func() { endLaunching(slug) }()
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, "brief", conversation.You, time.Now()); err != nil {
		return "", m.Slug, err
	}
	if !m.ToBrief() {
		if w := w.StageWarnings(m, p.Brief); len(w) > 0 {
			said = append(said, strings.Join(w, "; ")+".")
		}
		if plan != nil {
			// A new goal and its plan, confirmed: the go-ahead. The mission
			// moves on along the plan as it is now.
			said = append(said, upperFirst(sentence(w.afterNewGoal(m.Slug, plan, rev))))
		}
		return strings.Join(said, " "), m.Slug, nil
	}
	// Briefing it: named, given a better slug, its capsules lifted and
	// docked, and ready for crew. (Its conversations move with it.)
	if p.Title != "" && p.Title != m.Title {
		if err := ms.SetTitle(m.Slug, p.Title); err == nil {
			said = append(said, "Named it “"+p.Title+"”.")
		}
	}
	if p.Slug != "" && p.Slug != slug {
		if err := w.RenameMission(slug, p.Slug); err != nil {
			said = append(said, "Kept its slug ("+err.Error()+").")
		} else {
			slug = p.Slug
			said = append(said, "Its slug is "+slug+" now.")
		}
	}
	var made, failed []string
	for _, cap := range w.proposedCapsules(p) {
		repo, branch, _ := strings.Cut(cap, "/")
		if m.HasCapsule(repo, workspace.CapsuleName(branch)) {
			continue
		}
		joined, err := w.liftOrJoin(slug, repo, branch)
		if err != nil {
			failed = append(failed, cap+" ("+err.Error()+")")
			continue
		}
		if joined {
			made = append(made, "joined "+cap)
		} else {
			made = append(made, "lifted "+cap)
		}
	}
	// A Work step in a repo the mission has no capsule in gets one.
	if plan, err := mission.ParsePlan(p.Plan, nil); err == nil && p.Plan != "" {
		for _, said := range w.liftForPlan(slug, plan, 0) {
			if strings.HasPrefix(said, "lifted ") || strings.HasPrefix(said, "joined ") {
				made = append(made, said)
			} else {
				failed = append(failed, said)
			}
		}
	}
	for _, d := range p.Docks {
		repo, ref, ok := strings.Cut(d, "#")
		if !ok {
			repo, ref, ok = strings.Cut(d, "/")
		}
		if !ok || repo == "" || ref == "" {
			failed = append(failed, d+" (say repo/branch or repo#PR)")
			continue
		}
		if err := DockForMission(w, slug, repo, ref); err != nil {
			failed = append(failed, d+" ("+err.Error()+")")
			continue
		}
		made = append(made, "docked "+d)
	}
	if len(made) > 0 {
		said = append(said, strings.ToUpper(made[0][:1])+strings.Join(made, ", ")[1:]+".")
	}
	if len(failed) > 0 {
		said = append(said, "Couldn't make "+strings.Join(failed, "; ")+": lift or dock it yourself (WS_MISSION="+slug+").")
	}
	if plan != nil {
		if m, err := ms.Load(slug); err == nil {
			if warnings := w.PlanWarnings(m, plan); len(warnings) > 0 {
				said = append(said, strings.Join(warnings, "; ")+".")
			}
		}
	}
	// Confirming it is the go-ahead: it starts.
	started, err := w.ConfirmBrief(slug, AsYou)
	if err != nil {
		return strings.Join(said, " "), slug, err
	}
	said = append(said, upperFirst(sentence(started)))
	return strings.Join(said, " "), slug, nil
}

// acceptDispatch starts the crew proposed, as you, in the workspace's
// server, which sends it its opening prompt.
func (w *Workspace) acceptDispatch(store conversation.Store, c *conversation.Conversation, p conversation.Proposal) (string, error) {
	ms := w.Missions
	m, err := ms.Load(store.Slug)
	if err != nil {
		return "", err
	}
	if m.ToBrief() {
		return "", fmt.Errorf("mission %s is still being briefed", m.Slug)
	}
	text := p.Text
	if len(p.Reads) > 0 {
		list, err := w.ReadFirstList(m, p.Reads, w.Root())
		if err != nil {
			return "", err
		}
		text += "\n\nRead these first:\n" + list
	}
	l, err := w.Launch(m.Slug, Dispatch{Capsule: p.Capsule, Text: text}, AsYou)
	if err != nil {
		return "", err
	}
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, l.ID, conversation.You, time.Now()); err != nil {
		return "", err
	}
	return l.String(), nil
}

// acceptReplan asks the mission's planner what was proposed (as you
// edited it, if you did), as ws mission replan does, as you: its result is
// the planner's conversation. A planner that can't be asked (the mission
// ended since) leaves the proposal to accept or decline.
func (w *Workspace) acceptReplan(store conversation.Store, c *conversation.Conversation, p conversation.Proposal) (string, error) {
	if store.Workspace() {
		return "", refused("not-a-mission", "a replan is a mission's: this conversation is the workspace's")
	}
	a, err := w.AskPlanner(store.Slug, p.Text, AsYou)
	if err != nil {
		return "", err
	}
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, a.Conversation, conversation.You, time.Now()); err != nil {
		return "", err
	}
	return a.String(), nil
}

// acceptDecision records a proposed decision: in the conversation's
// mission, or the mission a workspace conversation became; a workspace
// conversation's is kept with it until it becomes one.
func (w *Workspace) acceptDecision(store conversation.Store, c *conversation.Conversation, p conversation.Proposal) (string, error) {
	slug := store.Slug
	if store.Workspace() {
		slug = c.Became()
	}
	if slug == "" {
		if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, "", conversation.You, time.Now()); err != nil {
			return "", err
		}
		return "Decided: it's kept with " + c.ID + ", and moves to the mission this becomes.", nil
	}
	d, err := w.Missions.Decide(slug, mission.Decision{What: p.What, Why: p.Why, By: conversation.You, From: c.ID})
	if err != nil {
		return "", err
	}
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, slug+":"+d.ID, conversation.You, time.Now()); err != nil {
		return "", err
	}
	return fmt.Sprintf("Recorded %s on %s: %s", d.ID, slug, d.What), nil
}

// acceptMission creates a proposed mission: ready with its brief (lifting
// its capsules), or without one, an idea waiting to be briefed. It
// records the conversation it came from, and the decisions kept with the
// conversation move to it.
func (w *Workspace) acceptMission(store conversation.Store, c *conversation.Conversation, p conversation.Proposal) (string, error) {
	ms := w.Missions
	// It's an idea, to brief: with a brief, the one it's proposed with,
	// for its planner to plan from; confirming the plan starts it.
	m, err := ms.CreateBriefing(p.Title)
	if err == nil && p.Brief != "" {
		err = ms.WriteBrief(m.Slug, p.Brief)
	}
	if err != nil {
		return "", err
	}
	// A mission's conversation is named with its mission's slug (cache/t4),
	// and the mission it raised it records it, as crew raising one do.
	ref := c.ID
	if !store.Workspace() {
		ref = store.Slug + "/" + c.ID
	}
	ms.Update(m.Slug, func(mm *mission.Mission) error {
		mm.Conversation = ref
		if !store.Workspace() {
			mm.From = store.Slug
		}
		return nil
	})
	// What it was proposed as being about, which the thing then says of
	// it. A name ws no longer reads (its plugin's gone since) is left
	// out, and said: the mission's made all the same.
	var notAbout string
	if len(p.About) > 0 {
		if _, err := w.SetAbout(m.Slug, p.About, nil); err != nil {
			notAbout = "It isn't recorded as about " + strings.Join(p.About, ", ") + " (" + err.Error() + "): ws mission about -m " + m.Slug + " <name> says so."
		}
	}
	title := firstNonEmpty(c.Title, "“"+clipLine(c.Turns[0].Body, 80)+"”")
	ms.AppendLogAfter(m.Slug, mission.Event{Kind: mission.EventNote, Text: "came from conversation " + ref + ": " + title})
	if !store.Workspace() {
		ms.AppendLogAfter(store.Slug, mission.Event{Kind: mission.EventNote, Text: "raised mission " + m.Slug + " from " + c.ID + ": " + p.Title})
	}
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, m.Slug, conversation.You, time.Now()); err != nil {
		return "", err
	}
	var said []string
	if p.Brief == "" {
		said = append(said, fmt.Sprintf("Saved as an idea: mission %s, waiting to be briefed (its briefing agent starts from %s).", m.Slug, c.ID))
	} else {
		// Its planner plans it from the brief, for you to confirm.
		planning := "brief it on its page"
		if b, err := w.Brief(m.Slug, AsYou); err == nil {
			planning = strings.TrimSuffix(b, ".")
		}
		said = append(said, fmt.Sprintf("Created mission %s with its brief: its planner plans it (%s), and confirming the plan starts it.", m.Slug, planning))
		var lifted, joined, failed []string
		for _, cap := range w.proposedCapsules(p) {
			repo, branch, _ := strings.Cut(cap, "/")
			there, err := w.liftOrJoin(m.Slug, repo, branch)
			switch {
			case err != nil:
				failed = append(failed, cap+" ("+err.Error()+")")
			case there:
				joined = append(joined, cap)
			default:
				lifted = append(lifted, cap)
			}
		}
		if len(lifted) > 0 {
			said = append(said, "Lifted "+strings.Join(lifted, ", ")+".")
		}
		if len(joined) > 0 {
			said = append(said, "Joined "+strings.Join(joined, ", ")+", lifted already.")
		}
		if len(failed) > 0 {
			said = append(said, "Couldn't lift "+strings.Join(failed, "; ")+": lift it with ws lift <repo> <branch> (WS_MISSION="+m.Slug+").")
		}
	}
	// What was decided before it was a mission is the mission's now.
	moved := 0
	for _, d := range c.KeptDecisions() {
		rec, err := ms.Decide(m.Slug, mission.Decision{What: d.What, Why: d.Why, By: conversation.You, From: c.ID})
		if err == nil {
			store.SettleProposal(c.ID, d.N, conversation.ProposalAccepted, m.Slug+":"+rec.ID, conversation.You, time.Now())
			moved++
		}
	}
	if moved > 0 {
		said = append(said, fmt.Sprintf("%d %s from %s moved to it.", moved, plural(moved, "decision"), c.ID))
	}
	if notAbout != "" {
		said = append(said, notAbout)
	}
	return strings.Join(said, " "), nil
}

// proposedCapsules are the capsules a proposed mission is lifted with:
// those it names, and those its brief's stages or its plan's steps name in
// a repo of the workspace, once each.
func (w *Workspace) proposedCapsules(p conversation.Proposal) []string {
	var out []string
	add := func(c string) {
		repo, branch, ok := strings.Cut(c, "/")
		if !ok || branch == "" {
			return
		}
		r, ok := w.WS().ResolveAlias(repo)
		if !ok {
			return
		}
		c = r + "/" + branch
		if slices.Contains(out, c) {
			return
		}
		out = append(out, c)
	}
	for _, c := range p.Capsules {
		add(c)
	}
	for _, st := range mission.ParseStages(p.Brief) {
		for _, c := range st.NamedCapsules() {
			add(c)
		}
	}
	if plan, err := mission.ParsePlan(p.Plan, nil); err == nil && p.Plan != "" {
		for _, st := range plan.Stages() {
			for _, c := range st.NamedCapsules() {
				add(c)
			}
		}
	}
	return out
}

// SettleProposal accepts (doing it, as you, with your edits) or declines
// what a conversation's agent proposed, saying what it did, and the slug
// of its mission after (briefing may rename it).
func (w *Workspace) SettleProposal(slug, id string, n int, accept bool, e ProposalEdits) (string, string, error) {
	store, err := ConversationStore(w.Root(), slug, id)
	if err != nil {
		return "", slug, err
	}
	return w.settleProposalAt(store, id, n, accept, e)
}

// ResolveRepo is the repo arg names: its name, an alias, or a pattern
// that matches one repo. Anything ambiguous is an error naming the
// candidates: it never asks, as what asks is an agent or a page.
func (w *Workspace) ResolveRepo(arg string) (string, error) {
	ws := w.WS()
	if arg == "" || arg == "." {
		return "", fmt.Errorf("repo name required")
	}
	if canonical, ok := ws.ResolveAlias(arg); ok {
		return canonical, nil
	}
	matches := ws.FuzzyMatchRepos(arg)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("unknown repo %q (not in ws.toml)", arg)
	}
	return "", fmt.Errorf("ambiguous repo %q: matches %s", arg, strings.Join(matches, ", "))
}

// EnsureCloned clones a repo the first time it's needed (a repo in a
// hidden group isn't cloned by setup), quietly: its after_create hook's
// output goes to the process log.
func (w *Workspace) EnsureCloned(repo string) error {
	ws := w.WS()
	if _, err := os.Stat(ws.RepoDir(repo)); err == nil {
		return nil
	}
	if err := ws.EnsureReposDir(); err != nil {
		return err
	}
	r := ws.SetupRepo(repo)
	if r.Err != nil || !r.Cloned {
		// SetupRepo makes the repo's folder before cloning: one left empty
		// would look cloned next time.
		os.Remove(ws.RepoDir(repo))
		if r.Err != nil {
			return fmt.Errorf("cloning %s: %w", repo, r.Err)
		}
		return fmt.Errorf("could not clone %s", repo)
	}
	if hook, ok := ws.AfterCreateHooks[repo]; ok {
		var out strings.Builder
		if err := workspace.RunHook(ws.MainWorktree(repo), hook, &out, &out); err != nil {
			execlog.Note(w.Root(), "ws", "%s's after_create hook failed: %v\n%s", repo, err, clipString(out.String(), 2000))
		}
	}
	return nil
}

// isPRNumber is s as a pull request's number.
func isPRNumber(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// afterNewGoal moves a mission under way on once you've confirmed a new
// goal and its plan: past the planner's step it was at, or else the
// orchestrator looks at the plan as it is now. It says what it did.
func (w *Workspace) afterNewGoal(slug string, plan *mission.Plan, rev mission.Revision) string {
	for _, l := range w.liftForPlan(slug, plan, rev.Fixed) {
		w.Missions.AppendLogAfter(slug, mission.Event{Kind: mission.EventNote, Text: l})
	}
	m, err := w.Missions.Load(slug)
	if err != nil {
		return ""
	}
	if cur, ok := currentStep(w.Missions, m); ok && cur.Step.Who == mission.WhoPlanner {
		return w.passPlannerStep(slug, cur.N, "the user confirmed the new goal and its plan")
	}
	if w.Notify(slug, mission.OrchestratorEvent{Trigger: "plan"}) {
		return "the orchestrator looks at the plan as it is now"
	}
	return "it goes on along the plan as it is now"
}
