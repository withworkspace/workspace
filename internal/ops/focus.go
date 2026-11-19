package ops

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// Focus: what you've pinned, of anything ws names, to keep beside
// everything (the web UI's sidebar). It's yours: crew and the
// orchestrator don't pin. A pin stands while what it names does: a mission
// that's ended, a conversation that's resolved, an artifact that's
// discarded leaves your focus by itself, as it isn't anything to be
// focused on any more. (The pin's kept, so a mission brought back, or a
// conversation reopened, is pinned still.)

// FocusItem is a pin, as it's shown: its name (as ws writes it, and as
// ws show takes it), and what the graph says of it, where it says: what
// it is, what it's called, and how it stands. Kind is "" for a name the
// graph has no thing for (a pull request ws keeps nothing of, a file).
type FocusItem struct {
	Name               string
	Kind, Title, State string
}

// Focus is your pins that stand, in the order you keep them.
func (w *Workspace) Focus() ([]FocusItem, error) {
	db, err := w.Records()
	if err != nil {
		return nil, err
	}
	var pins []store.Pin
	if err := db.Read(context.Background(), func(tx *store.Tx) error {
		pins, err = tx.Pins()
		return err
	}); err != nil {
		return nil, err
	}
	out := []FocusItem{}
	for _, p := range pins {
		it := FocusItem{Name: p.Key.Name}
		if t := p.Thing; t != nil {
			if settled(t) {
				continue
			}
			it.Kind, it.Title, it.State = t.Kind, t.Title, t.State
		}
		out = append(out, it)
	}
	return out, nil
}

// settled is whether a thing has come to an end, by what the graph says
// of it: there's nothing in it to focus on.
func settled(t *store.Thing) bool {
	switch t.Kind {
	case "mission":
		s := mission.State(t.State)
		return s.Ended() || s == mission.Archived
	case artifact.ThingKind:
		return t.State == string(artifact.Discarded)
	}
	// A conversation's thing is what it is (a talk, a thread…): what
	// resolves is one.
	return t.State == conversation.Resolved
}

// Pinned is what pinning did.
type Pinned struct {
	Name    string
	Already bool // it was pinned before
}

func (p Pinned) String() string {
	if p.Already {
		return p.Name + " is pinned already."
	}
	return "Pinned " + p.Name + "."
}

// Unpinned is what unpinning did.
type Unpinned struct {
	Name string
	Was  bool // it was pinned
}

func (u Unpinned) String() string {
	if !u.Was {
		return u.Name + " wasn't pinned."
	}
	return "Unpinned " + u.Name + "."
}

// pinsAreYours refuses anyone else: what's in your focus is yours to say.
func pinsAreYours(a Actor) error {
	if a.Kind != You {
		return &Refused{Code: "not-yours", Reason: "only you pin things: your focus is yours to keep"}
	}
	return nil
}

// pinName is a name as a pin keeps it: as ws writes it, the file rather
// than its lines, the artifact rather than one version of it. What ws
// names that isn't in the graph is refused unless it's a pull request, a
// file or a folder, which needn't be.
func (w *Workspace) pinName(name string) (string, error) {
	name = strings.TrimSpace(name)
	r, ok := w.RefsKnown(context.Background(), false).At(name)
	if !ok {
		r, ok = ref.Parse(name) // a file's, by its form: what's known here has no trees
	}
	if !ok {
		return "", &Invalid{Reason: "ws names nothing " + name + ": pin a mission, a conversation, a pull request (demo#12), a page, an artifact, a document or a plugin's thing"}
	}
	r.Line, r.EndLine, r.Commit, r.Anchor = 0, 0, "", ""
	if r.Kind == ref.Artifact {
		r.Version = 0
	}
	name = r.Name()
	switch r.Kind {
	case ref.PR, ref.File, ref.Dir:
		return name, nil
	}
	db, err := w.Records()
	if err != nil {
		return "", err
	}
	var there bool
	if err := db.Read(context.Background(), func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM things WHERE scope = '' AND name = ?)`, name).Scan(&there)
	}); err != nil {
		return "", err
	}
	if !there {
		return "", &Invalid{Reason: fmt.Sprintf("ws has nothing called %s", name)}
	}
	return name, nil
}

// Pin pins what name names, after your other pins.
func (w *Workspace) Pin(a Actor, name string) (Pinned, error) {
	if err := pinsAreYours(a); err != nil {
		return Pinned{}, err
	}
	name, err := w.pinName(name)
	if err != nil {
		return Pinned{}, err
	}
	db, err := w.Records()
	if err != nil {
		return Pinned{}, err
	}
	out := Pinned{Name: name}
	err = db.Write(context.Background(), func(tx *store.Tx) error {
		pins, err := tx.Pins()
		if err != nil {
			return err
		}
		out.Already = slices.ContainsFunc(pins, func(p store.Pin) bool { return p.Key == store.Key{Name: name} })
		return tx.Pin(store.Key{Name: name}, time.Now())
	})
	return out, err
}

// Unpin unpins what name names: by the name it was pinned by, or as ws
// writes it, so what's no longer there can still be unpinned.
func (w *Workspace) Unpin(a Actor, name string) (Unpinned, error) {
	if err := pinsAreYours(a); err != nil {
		return Unpinned{}, err
	}
	names := []string{strings.TrimSpace(name)}
	if n, err := w.pinName(name); err == nil && n != names[0] {
		names = append(names, n)
	}
	db, err := w.Records()
	if err != nil {
		return Unpinned{}, err
	}
	out := Unpinned{Name: names[len(names)-1]}
	err = db.Write(context.Background(), func(tx *store.Tx) error {
		for _, n := range names {
			was, err := tx.Unpin(store.Key{Name: n})
			if err != nil {
				return err
			}
			if was {
				out.Was, out.Name = true, n
			}
		}
		return nil
	})
	return out, err
}

// ArrangeFocus puts your pins in the order names gives (those it leaves
// out keep their order after them).
func (w *Workspace) ArrangeFocus(a Actor, names []string) error {
	if err := pinsAreYours(a); err != nil {
		return err
	}
	keys := make([]store.Key, len(names))
	for i, n := range names {
		keys[i] = store.Key{Name: strings.TrimSpace(n)}
	}
	db, err := w.Records()
	if err != nil {
		return err
	}
	return db.Write(context.Background(), func(tx *store.Tx) error { return tx.ArrangePins(keys) })
}
