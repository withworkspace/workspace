package ops

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
)

// Conversations: threads on a document, talk about the workspace or a
// mission, and what was decided in them. Each write is here, and each one
// that leaves something for its agent to answer has the daemon answer it.
// A document is passed as it reads now (its ref, and its text), for the
// version a thread is opened or answered on.

// now is the time an operation is recorded at.
func (w *Workspace) now() time.Time {
	if w.opts.Now != nil {
		return w.opts.Now()
	}
	return time.Now()
}

// answer has the daemon answer a conversation (kind job.Answer), a thread
// (job.Thread), or draft what a resolved thread decided (job.Draft).
func (w *Workspace) answer(kind string, sc doc.Scope, id string) {
	if w.takes(kind) {
		w.markPlanning(kind, sc, id)
		w.start(job.Job{Kind: kind, Mission: sc.Mission, ID: id})
	}
}

// Conversations are a scope's: a mission's (ErrNotFound if there's no such
// mission), or the workspace's.
func (w *Workspace) Conversations(sc doc.Scope) (conversation.Store, error) {
	if sc.IsWorkspace() {
		return conversation.ForWorkspace(w.Root()), nil
	}
	if _, err := w.load(sc.Mission); err != nil {
		return conversation.Store{}, err
	}
	return conversation.For(w.Root(), sc.Mission), nil
}

// KeepVersion keeps a document's text as it's read to be edited or
// commented on: what an edit made from it is merged with, and what a
// thread opened on it shows changed since.
func (w *Workspace) KeepVersion(sc doc.Scope, text []byte) (string, error) {
	return conversation.For(w.Root(), sc.Mission).KeepVersion(text)
}

// OpenThread opens a thread on a document as it reads now (see
// conversation.OpenOnDoc), and has its agent answer.
func (w *Workspace) OpenThread(sc doc.Scope, ref string, text []byte, t conversation.DocThread, by Actor) (*conversation.Conversation, error) {
	store, err := w.Conversations(sc)
	if err != nil {
		return nil, err
	}
	c, err := store.OpenOnDoc(ref, text, t, by.By(), w.now())
	if err != nil {
		return nil, err
	}
	w.answered(job.Thread, sc, c)
	return c, nil
}

// Reply says something in a thread, on the document as it reads now (text;
// nil if it can't be read, when no version is kept), and with the commit
// that changed it for the thread, if there's one. Its agent answers, if
// what's said needs it.
func (w *Workspace) Reply(sc doc.Scope, id string, text []byte, body, commit string, by Actor) (*conversation.Conversation, error) {
	store, err := w.Conversations(sc)
	if err != nil {
		return nil, err
	}
	t := conversation.Turn{By: by.By(), At: w.now(), Body: body, Commit: commit}
	w.pinTurn(&t)
	if text != nil {
		if t.Version, err = store.KeepVersion(text); err != nil {
			return nil, err
		}
	}
	c, err := store.Say(id, t)
	if err != nil {
		return nil, err
	}
	w.answered(job.Thread, sc, c)
	return c, nil
}

// answered has c's agent answer it, if what was last said needs it.
func (w *Workspace) answered(kind string, sc doc.Scope, c *conversation.Conversation) {
	if c.NeedsAnswer() {
		w.answer(kind, sc, c.ID)
	}
}

// SetThreadStatus resolves or reopens a thread (you only). One resolved
// in a mission has what was decided in it drafted, if anything was said
// worth drafting from: a document outside one has nowhere to record it.
func (w *Workspace) SetThreadStatus(sc doc.Scope, id, status string, by Actor) (*conversation.Conversation, error) {
	store, err := w.Conversations(sc)
	if err != nil {
		return nil, err
	}
	c, err := store.SetStatus(id, by.By(), status, w.now())
	if err != nil {
		return nil, err
	}
	if !sc.IsWorkspace() && c.ShouldDraft() {
		w.answer(job.Draft, sc, c.ID)
	}
	return c, nil
}

// DeclineSuggestion is you turning down the edit a thread suggests.
func (w *Workspace) DeclineSuggestion(sc doc.Scope, id string, by Actor) (*conversation.Conversation, error) {
	store, err := w.Conversations(sc)
	if err != nil {
		return nil, err
	}
	return store.SettleSuggestion(id, by.By(), conversation.SuggestDeclined, "", w.now())
}

// Approve is you approving a document as you read it (version): refused
// if it's changed since (conversation.ErrStale), or while a Change thread
// on it is open. It says what it did.
func (w *Workspace) Approve(sc doc.Scope, ref string, text []byte, version string, by Actor) (string, error) {
	if version != doc.Version(text) {
		return "", conversation.ErrStale
	}
	store, err := w.Conversations(sc)
	if err != nil {
		return "", err
	}
	_, path, _ := strings.Cut(ref, ":")
	if _, err := store.Approve(ref, path, text, by.By(), w.now()); err != nil {
		return "", err
	}
	if sc.Mission != "" {
		// A Review step looking at it may be done with.
		if passed := w.PassReviewStep(sc.Mission); passed != "" {
			return "Approved " + path + ": " + passed + ".", nil
		}
	}
	return "Approved " + path + ".", nil
}

// Decide records what was decided in a thread on a mission's document, as
// its agent drafted it or as you've put it: the decision is from the
// thread, and about what's named (the document, and the stage it was made
// for). It says what it recorded.
func (w *Workspace) Decide(slug, id, what, why string, about []string, by Actor) (string, error) {
	store, err := w.Conversations(doc.OfMission(slug))
	if err != nil {
		return "", err
	}
	c, err := store.Load(id)
	if err != nil {
		return "", err
	}
	if c.Draft != nil && c.Draft.State != conversation.DraftOpen {
		return "", fmt.Errorf("what was decided in %s is %s already", id, c.Draft.State)
	}
	d, err := w.Missions.Decide(slug, mission.Decision{What: what, Why: why, By: by.By(), From: id, About: about})
	if err != nil {
		return "", err
	}
	if _, err := store.SettleDraft(id, conversation.DraftRecorded, d.ID, by.By(), w.now()); err != nil {
		return "", err
	}
	w.youActed(slug, by)
	return "Recorded " + d.ID + ": " + d.What, nil
}

// SkipDraft is you passing on what a thread's agent drafted.
func (w *Workspace) SkipDraft(sc doc.Scope, id string, by Actor) error {
	store, err := w.Conversations(sc)
	if err != nil {
		return err
	}
	_, err = store.SettleDraft(id, conversation.DraftSkipped, "", by.By(), w.now())
	return err
}

// StartTalk starts a conversation about the workspace or a mission, with
// what you said first, and has its agent answer.
func (w *Workspace) StartTalk(sc doc.Scope, body string, by Actor) (*conversation.Conversation, error) {
	store, err := w.Conversations(sc)
	if err != nil {
		return nil, err
	}
	c, err := store.OpenTalk(by.By(), body, w.now())
	if err != nil {
		return nil, &Invalid{Reason: err.Error()}
	}
	w.answer(job.Answer, sc, c.ID)
	return c, nil
}

// StartTalkVia starts one of the workspace's conversations for you from
// a plugin, with what it has you say first, and has its agent answer:
// what a plugin's app does to hand something on to be talked over.
func (w *Workspace) StartTalkVia(plugin, body string) (*conversation.Conversation, error) {
	store, err := w.Conversations(doc.Workspace)
	if err != nil {
		return nil, err
	}
	// What it names is pinned, as a reply's is: the thing a plugin
	// starts a conversation about says the conversation mentions it at
	// once, not from the agent's first reply.
	first := conversation.Turn{By: AsYou.By(), At: w.now(), Body: body}
	w.pinTurn(&first)
	c, err := store.OpenTalkVia(plugin, first)
	if err != nil {
		return nil, &Invalid{Reason: err.Error()}
	}
	w.answer(job.Answer, doc.Workspace, c.ID)
	return c, nil
}

// SayInTalk is you saying something in a conversation, taking it up again
// if it was put away. Its agent answers.
func (w *Workspace) SayInTalk(sc doc.Scope, id, body string, by Actor) (*conversation.Conversation, error) {
	return w.SayInTalkGiven(sc, id, body, nil, by)
}

// SayInTalkGiven is SayInTalk from a plugin's page, with what the page
// gave the agent with it: its standing context only if that's changed
// since the agent was last given it.
func (w *Workspace) SayInTalkGiven(sc doc.Scope, id, body string, given *conversation.Given, by Actor) (*conversation.Conversation, error) {
	store, err := w.Conversations(sc)
	if err != nil {
		return nil, err
	}
	t := conversation.Turn{By: by.By(), At: w.now(), Body: body}
	if given != nil {
		was, err := store.Load(id)
		if err != nil {
			return nil, err
		}
		g := *given
		if g.From == "" && was.Anchor.Plugin != "" {
			g.From = w.pluginTitle(was.Anchor.Plugin)
		}
		t.Given = was.Give(g)
	}
	c, err := store.TakeUp(id, t)
	if err != nil {
		return nil, err
	}
	w.answered(job.Answer, sc, c)
	return c, nil
}

// SetTalkStatus is you putting a conversation away, or taking it up again.
// The capsules its agent lifted are put away first (putAwayCapsules); if
// one can't be, the conversation isn't put away either, and why is said,
// so a capsule is never left with nothing saying whose it was.
func (w *Workspace) SetTalkStatus(sc doc.Scope, id, status string, by Actor) (*conversation.Conversation, error) {
	store, err := w.Conversations(sc)
	if err != nil {
		return nil, err
	}
	if status == conversation.Resolved && by.By() == conversation.You {
		if c, err := store.Load(id); err == nil && len(c.Lifted()) > 0 {
			if left := w.putAwayCapsules(store, c); len(left) > 0 {
				return nil, refused("capsules-left", "%s isn't put away: the capsules its agent lifted are put away with it, and %s couldn't be. Put it right there, or burn it yourself (ws burn <repo> <name>), then put the conversation away again",
					id, strings.Join(left, "; "))
			}
		}
	}
	return store.SetStatus(id, by.By(), status, w.now())
}

// SaveDraft keeps what you've written of a document and not saved yet.
func (w *Workspace) SaveDraft(sc doc.Scope, d doc.Draft) error {
	return doc.Drafts{Root: w.Root(), Scope: sc}.Save(d)
}

// DropDraft throws a document's unsaved draft away.
func (w *Workspace) DropDraft(sc doc.Scope, ref string) error {
	return doc.Drafts{Root: w.Root(), Scope: sc}.Drop(ref)
}

// Bigger changes: "rewrite the rollout section" is work, not an edit. You
// send a thread (or every Change thread open on a document) to crew: the
// crew that wrote the document, told if it's running or resumed if it's
// ended, or else fresh crew dispatched into the document's capsule. Crew
// answer in the threads and say when they've changed the document (ws
// thread changed), which hands each back to you.

// sendToCrew hands open threads on one document to crew to change it, and
// says who has them.
func (w *Workspace) sendToCrew(m *mission.Mission, ids []string) (string, error) {
	store := conversation.For(w.WS().Root, m.Slug)
	threads, doc, err := threadsToSend(store, ids)
	if err != nil {
		return "", err
	}
	ids = nil
	for _, c := range threads {
		ids = append(ids, c.ID)
	}
	v := w.missionView(m)
	var av ArtifactView
	found := false
	for _, a := range ResolveArtifacts(w.WS(), w.Missions, v) {
		if a.Ref == doc {
			av, found = a, true
		}
	}
	if !found {
		return "", fmt.Errorf("mission %s has no document %s", m.Slug, doc)
	}
	path := threads[0].Anchor.Path
	which := "thread " + ids[0]
	if len(ids) > 1 {
		which = "threads " + strings.Join(ids, ", ")
	}
	commit := ""
	if av.Parsed.Loc == LocCapsule || av.Parsed.Loc == LocGround {
		commit = " (and commit it)"
	}
	brief := fmt.Sprintf("The user wants %s (%s) changed, in %s. Read each with `ws show %s`: the text it's on, and what's been said. Change the document%s, then say what you changed in each with `ws thread changed <thread> \"…\"`, which hands it back to the user to look at and resolve. If you won't change something, say why with `ws thread reply <thread> \"…\"`.",
		path, doc, which, ids[0], commit)
	mstore := w.Missions
	w.youActed(m.Slug, AsYou)

	// The crew that wrote it, if it can be reached.
	if c, ok := m.CrewByID(av.Crew); ok && c.Role == "" && c.Conversation != "" {
		if c.Ended == nil {
			if _, err := w.Say(m.Slug, c.ID, brief, AsYou); err != nil {
				return "", fmt.Errorf("%s wrote it and is still running, but couldn't be told (%v): tell it in its terminal, or stop it and send again", c.ID, err)
			}
			return fmt.Sprintf("Told %s, which wrote %s and is still running.", c.ID, path), store.Send(ids, c.ID, conversation.You, time.Now())
		}
		for _, cv := range v.Capsules {
			if cv.Repo+"/"+cv.Name != c.Capsule || !cv.Exists {
				continue
			}
			if err := mstore.SetResumeWithBy(m.Slug, c.ID, brief, AsYou.By()); err != nil {
				return "", err
			}
			l, err := w.Launch(m.Slug, Dispatch{Resume: c.ID}, AsYou)
			if err != nil {
				mstore.SetResumeWith(m.Slug, c.ID, "")
				return "", err
			}
			return w.startedFor(m, ids, fmt.Sprintf("Resumed %s, which wrote %s.", l.ID, path), l.ID)
		}
	}

	// Fresh crew, in the document's capsule (or one of its repo's).
	want := ""
	switch av.Parsed.Loc {
	case LocCapsule, LocGround:
		for _, c := range v.Capsules {
			if c.Repo == av.Parsed.Repo && c.Exists && (want == "" || c.Name == av.Parsed.Capsule) {
				want = c.Repo + "/" + c.Name
			}
		}
		if want == "" {
			return "", fmt.Errorf("%s is in %s, and the mission has no capsule of it to change it in: lift one (ws lift %s <branch>) and send it again", path, av.Parsed.Repo, av.Parsed.Repo)
		}
	}
	reads, err := w.ReadFirstList(m, []string{doc}, w.Root())
	if err != nil {
		return "", err
	}
	l, err := w.Launch(m.Slug, Dispatch{Capsule: want, Text: brief + "\n\nRead these first:\n" + reads}, AsYou)
	if err != nil {
		return "", err
	}
	return w.startedFor(m, ids, fmt.Sprintf("Started %s in %s on %s.", l.ID, l.Capsule, which), l.ID)
}

// startedFor hands the threads to crew just started, and says so.
func (w *Workspace) startedFor(m *mission.Mission, ids []string, said, id string) (string, error) {
	if err := conversation.For(w.WS().Root, m.Slug).Send(ids, id, conversation.You, time.Now()); err != nil {
		return "", err
	}
	return said, nil
}

// threadsToSend loads the threads to send: open, on one document, and
// none with crew already. It says which document.
func threadsToSend(store conversation.Store, ids []string) ([]*conversation.Conversation, string, error) {
	var out []*conversation.Conversation
	doc := ""
	for _, id := range ids {
		c, err := store.Load(id)
		if err != nil {
			return nil, "", err
		}
		switch {
		case c.Status != conversation.Open:
			return nil, "", fmt.Errorf("%s is resolved: reopen it to send it", id)
		case c.WithCrew():
			return nil, "", fmt.Errorf("%s is with %s already", id, c.Sent.Crew)
		case doc != "" && c.Anchor.Doc != doc:
			return nil, "", fmt.Errorf("%s is on %s, not %s: send threads on one document at a time", id, c.Anchor.Doc, doc)
		}
		doc = c.Anchor.Doc
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, "", errors.New("say which threads to send")
	}
	return out, doc, nil
}

// openChangesToSend is the Change threads open on a document that crew
// don't have.
func openChangesToSend(store conversation.Store, doc string) ([]string, error) {
	threads, err := store.OnDoc(doc)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, c := range threads {
		if c.Kind == conversation.Change && c.Status == conversation.Open && !c.WithCrew() {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: no change asked for on %s is open and not with crew", conversation.ErrNotFound, doc)
	}
	return ids, nil
}
