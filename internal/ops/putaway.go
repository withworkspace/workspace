package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/notification"
	"github.com/withworkspace/workspace/internal/outbox"
)

// Putting away is what archiving a mission leaves to do: burning its
// capsules, and those its conversations' agents lifted. Each is seconds
// of git and disk (a capsule's folder goes, node_modules and all), which
// a page needn't wait on: in the daemon, archiving answers once it's
// decided, and the burning is done behind, from the outbox, a few
// seconds later (putAwayHold), so the undo the page offers can still
// keep them (Reopen, takeBackPutAway). One that fails is kept, with why, on the mission's
// page to try again or let go of, and you're told.
//
// Resolving a conversation whose agent lifted capsules goes the same way:
// resolved at once, its capsules put away behind. One that can't be is
// left to the conversation, which still holds it, and you're told:
// resolving it again tries again.
//
// Without an outbox (a command's), they're burned there and then.

// putAwayKind is the outbox's kind for it.
const putAwayKind = "put-away"

// putAwayHold is how long a capsule waits after archiving before it's
// burned: as long as the page offers to undo it.
var putAwayHold = 5 * time.Second

// PutAwayFailed is the kind of notification one that failed is.
const PutAwayFailed = "mission.put-away-failed"

// putAway is one thing to put away, as the outbox keeps it: an archived
// mission's capsule, or the capsules one of its conversations lifted; or
// those a conversation (of a mission's, or the workspace's) lifted, as
// it's resolved.
type putAway struct {
	Mission string `json:"mission,omitempty"`
	Repo    string `json:"repo,omitempty"`
	Name    string `json:"name,omitempty"`
	// Force burns it whatever's in it: you agreed to (Archive's burnAll).
	Force bool `json:"force,omitempty"`
	// Conversation is the conversation whose lifted capsules are put away
	// (putAwayCapsules), in place of Repo and Name.
	Conversation string `json:"conversation,omitempty"`
	// Resolved: it's the conversation being resolved, not its mission
	// archived.
	Resolved bool `json:"resolved,omitempty"`
}

func putAwayOf(it outbox.Item) (putAway, bool) {
	var p putAway
	return p, json.Unmarshal(it.Op, &p) == nil && (p.Mission != "" || p.Resolved && p.Conversation != "")
}

// resolvedLane is what a conversation's take turns on: the conversation.
func resolvedLane(sc doc.Scope, id string) string { return "conversation " + sc.Mission + "/" + id }

// putAwayLane is what a mission's take turns on: the mission.
func putAwayLane(slug string) string { return "mission " + slug }

// PutAwayBehind has what archiving leaves to do done from q.
func (w *Workspace) PutAwayBehind(q *outbox.Queue) {
	q.Handle(putAwayKind, outbox.Kind{
		Send: w.sendPutAway,
		// A burned capsule isn't there to burn again.
		Again:   func(outbox.Item) bool { return true },
		Hold:    putAwayHold,
		Settled: w.putAwaySettled,
	})
}

// sendPutAway puts one away: nothing, if the mission's no longer archived
// (it was reopened, or never got there), or the conversation no longer
// resolved.
func (w *Workspace) sendPutAway(it outbox.Item) error {
	p, ok := putAwayOf(it)
	if !ok {
		return errors.New("could not read what was to be put away")
	}
	if p.Resolved {
		store, err := w.Conversations(doc.Scope{Mission: p.Mission})
		if err != nil {
			return err
		}
		c, err := store.Load(p.Conversation)
		if err != nil {
			return err
		}
		if c.Status != conversation.Resolved {
			return nil
		}
		if left := w.putAwayCapsules(store, c); len(left) > 0 {
			return errors.New(strings.Join(left, "; "))
		}
		return nil
	}
	m, err := w.load(p.Mission)
	if err != nil {
		return err
	}
	if m.State != mission.Archived {
		return nil
	}
	if p.Conversation != "" {
		convs := conversation.For(w.Root(), m.Slug)
		c, err := convs.Load(p.Conversation)
		if err != nil {
			return err
		}
		if left := w.putAwayCapsules(convs, c); len(left) > 0 {
			return errors.New("could not put away " + strings.Join(left, "; "))
		}
		return nil
	}
	if !exists(filepath.Join(w.WS().RepoDir(p.Repo), p.Name)) {
		return nil
	}
	_, err = w.Burn(p.Repo, p.Name, BurnOptions{Force: p.Force})
	return err
}

// putAwaySettled tells you of each that failed: on the mission's log, and
// in your notifications, as the mission's out of sight.
func (w *Workspace) putAwaySettled(_ string, _, failed []outbox.Item) {
	for _, it := range failed {
		p, ok := putAwayOf(it)
		if !ok {
			continue
		}
		if p.Resolved {
			// The conversation still holds what it couldn't put away, and
			// resolving it again tries again: nothing's kept here.
			var refs []string
			if p.Mission != "" {
				refs = []string{p.Mission}
			}
			w.notify(notification.Notification{Source: notification.SourceMissions, Kind: PutAwayFailed, Refs: refs, By: events.ByWS, Attention: true,
				Text: fmt.Sprintf("%s resolved, but its capsules weren't put away: %s. Resolve it again to try again", p.Conversation, it.Error)})
			if err := w.opts.Outbox.Drop(it.ID); err != nil {
				execlog.Note(w.Root(), "put away "+p.Conversation, "couldn't let go of what failed: %v", err)
			}
			continue
		}
		text := fmt.Sprintf("Archived, but %s failed: %s", it.Doing, it.Error)
		if err := w.Missions.AppendLog(p.Mission, mission.Event{Kind: mission.EventNote, Text: text}); err != nil {
			execlog.Note(w.Root(), "put away "+p.Mission, "%q isn't on its log: %v", text, err)
		}
		w.notify(notification.Notification{Source: notification.SourceMissions, Kind: PutAwayFailed, Refs: []string{p.Mission},
			By: events.ByWS, Attention: true, Text: fmt.Sprintf("Mission %s archived, but %s failed", p.Mission, it.Doing)})
	}
}

// archiveBehind is Archive with an outbox: the mission archived now, and
// burn (its capsules) and its conversations' lifted capsules put away
// behind. It says what will be burned.
func (w *Workspace) archiveBehind(q *outbox.Queue, m *mission.Mission, burn []CapsuleView, force bool, by Actor) (StateSet, error) {
	// Before their capsules go: a session left open would sit in a folder
	// that's gone.
	w.closeEnded(m, "mission archived")
	lane := putAwayLane(m.Slug)
	var burning []string
	// Queued before it's archived: should archiving fail, those queued do
	// nothing (sendPutAway), where the other way round a mission would be
	// archived with nothing to burn its capsules.
	for _, c := range burn {
		ref := c.Repo + "/" + c.Name
		if _, err := q.Add(lane, putAwayKind, "burning "+ref, putAway{Mission: m.Slug, Repo: c.Repo, Name: c.Name, Force: force}); err != nil {
			return StateSet{}, fmt.Errorf("not archived: could not queue burning %s: %w", ref, err)
		}
		burning = append(burning, ref)
	}
	convs := conversation.For(w.Root(), m.Slug)
	if talks, err := convs.Talks(); err == nil {
		for _, c := range talks {
			if len(c.Lifted()) == 0 {
				continue
			}
			if _, err := q.Add(lane, putAwayKind, "putting away the capsules "+c.ID+" lifted", putAway{Mission: m.Slug, Conversation: c.ID}); err != nil {
				return StateSet{}, fmt.Errorf("not archived: could not queue putting away %s's capsules: %w", c.ID, err)
			}
		}
	}
	out, err := w.putIn(m.Slug, mission.Archived, by)
	out.Burning = burning
	return out, err
}

// resolveBehind is resolving a conversation with an outbox: resolved now,
// the capsules its agent lifted put away behind.
func (w *Workspace) resolveBehind(q *outbox.Queue, sc doc.Scope, store conversation.Store, id string, by Actor) (*conversation.Conversation, error) {
	if _, err := q.Add(resolvedLane(sc, id), putAwayKind, "putting away the capsules "+id+" lifted",
		putAway{Mission: sc.Mission, Conversation: id, Resolved: true}); err != nil {
		return nil, fmt.Errorf("%s not resolved: could not queue putting away its capsules: %w", id, err)
	}
	return store.SetStatus(id, by.By(), conversation.Resolved, w.now())
}

// takeBackPutAway takes back what archiving a mission had still to put away,
// as it's reopened: the capsules it hadn't burned yet are kept.
func (w *Workspace) takeBackPutAway(slug string) []string {
	q := w.opts.Outbox
	if q == nil {
		return nil
	}
	gone, err := q.Cancel(putAwayLane(slug), putAwayKind)
	if err != nil {
		execlog.Note(w.Root(), "put away "+slug, "couldn't take back what was to be put away: %v", err)
	}
	var kept []string
	for _, it := range gone {
		if p, ok := putAwayOf(it); ok && p.Repo != "" {
			kept = append(kept, p.Repo+"/"+p.Name)
		}
	}
	return kept
}

// PutAwayItem is something archiving a mission left to do, as its page
// shows it: waiting, being done, or failed (with why).
type PutAwayItem struct {
	ID    int64  `json:"id"`
	Doing string `json:"doing"`
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

// PuttingAway is what archiving a mission has still to do, or couldn't.
func (w *Workspace) PuttingAway(slug string) []PutAwayItem { return PuttingAway(w.Root(), slug) }

// PuttingAway is what archiving a mission has still to do, or couldn't,
// in the workspace at root: as the daemon's outbox keeps it.
func PuttingAway(root, slug string) []PutAwayItem {
	items, _ := outbox.In(root, putAwayLane(slug))
	var out []PutAwayItem
	for _, it := range items {
		if it.Kind == putAwayKind && it.State != outbox.Sent {
			out = append(out, PutAwayItem{ID: it.ID, Doing: it.Doing, State: it.State, Error: it.Error})
		}
	}
	return out
}

// RetryPutAway tries again what archiving a mission couldn't do; DropPutAway
// lets go of it, leaving the capsule as it is.
func (w *Workspace) RetryPutAway(slug string, id int64, by Actor) (string, error) {
	it, err := w.putAwayItem(slug, id)
	if err != nil {
		return "", err
	}
	if err := w.opts.Outbox.Retry(id); err != nil {
		return "", wrapPutAway(err)
	}
	w.youActed(slug, by)
	return "Trying again: " + it.Doing + ".", nil
}

func (w *Workspace) DropPutAway(slug string, id int64, by Actor) (string, error) {
	it, err := w.putAwayItem(slug, id)
	if err != nil {
		return "", err
	}
	if err := w.opts.Outbox.Drop(id); err != nil {
		return "", wrapPutAway(err)
	}
	w.youActed(slug, by)
	return "Left as it is: " + strings.TrimPrefix(it.Doing, "burning ") + ".", nil
}

func (w *Workspace) putAwayItem(slug string, id int64) (outbox.Item, error) {
	if w.opts.Outbox == nil {
		return outbox.Item{}, refused("no-daemon", "nothing's put away behind here: only the daemon does that")
	}
	it, err := outbox.Get(w.Root(), id)
	if err != nil || it.Kind != putAwayKind || it.Lane != putAwayLane(slug) {
		return outbox.Item{}, refused("not-found", "%s has nothing like that left to put away", slug)
	}
	return it, nil
}

func wrapPutAway(err error) error {
	if errors.Is(err, outbox.ErrNotFound) {
		return refused("not-failed", "that's not failed: only one that failed is tried again or let go of")
	}
	return err
}
