package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/words"
)

// What ws keeps of plugins in the workspace's records: how each one's
// things are named (the refs it registered), by the plugin they're from,
// and the things a plugin tells ws of (thing/put), each a thing in the
// graph with what it says, so ws finds and names them without asking. A
// name is found from here, without asking the plugin or reading what it
// last said; and everything kept of a plugin is kept under it, so removing
// the plugin forgets it (Forget).

func init() {
	store.Define("plugin", store.SQL(`
		-- A kind of thing a plugin has, and how one is named, as ws took
		-- it: a prefix another plugin registered first isn't in it.
		CREATE TABLE plugin_refs (
			plugin TEXT NOT NULL, kind TEXT NOT NULL,
			place INTEGER NOT NULL,     -- where it is among the plugin's, as registered
			ref TEXT NOT NULL,          -- protocol.Ref, as JSON
			registered TEXT NOT NULL,   -- when the plugin registered it as it is
			PRIMARY KEY (plugin, kind)
		) WITHOUT ROWID;
	`))
}

// Origin is what ws keeps in its graph for a plugin (its things, and
// what they say) kept under: each thing is a record of its own there
// (plugin:jira/issue/RAM-123), and Forget removes them all.
func Origin(name string) store.Origin { return store.Origin("plugin:" + name) }

// thingOrigin is the record of one of a plugin's things.
func thingOrigin(name, kind, ref string) store.Origin {
	return store.Origin(string(Origin(name)) + "/" + kind + "/" + ref)
}

// ThingKind is what one of a plugin's things is in the graph: what it is
// to its plugin (an issue) is the ref it's named by.
const ThingKind = "thing"

// How much of a thing ws keeps: a title's a line, and what it says is
// for finding it by, not for reading it whole (its plugin shows that).
const (
	thingTitle = 300
	thingState = 60
	thingText  = 20000
)

// KeptRef is a kind of thing a plugin has and how one is named, as ws
// keeps it: which plugin registered it, and when.
type KeptRef struct {
	Plugin string `json:"plugin"`
	protocol.Ref
	Registered time.Time `json:"registered"`
}

// Named is the kind as ws's grammar for names reads it.
func (k KeptRef) Named() ref.Named { return named(k.Plugin, k.Ref) }

func named(plugin string, r protocol.Ref) ref.Named {
	return ref.Named{Plugin: plugin, What: r.Kind, Prefixes: r.Prefixes, Tail: r.Tail, Address: r.Address}
}

// KeepRefs keeps how a plugin's things are named, as it registered them
// just now, in place of what it registered before. Refused is what ws
// left out, and why: a ref that's wrong in itself, and a prefix that's
// another plugin's already (the first to register one keeps it).
func KeepRefs(root, name string, refs []protocol.Ref) (refused []string, err error) {
	db, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	err = db.Write(context.Background(), func(tx *store.Tx) error {
		refused = nil
		kept, err := refsIn(tx)
		if err != nil {
			return err
		}
		taken := map[string]string{} // prefix → whose
		var was []KeptRef
		for _, k := range kept {
			if k.Plugin == name {
				was = append(was, k)
				continue
			}
			for _, p := range k.Prefixes {
				taken[p] = "the plugin " + k.Plugin + "'s"
			}
		}
		var take []protocol.Ref
		kinds := map[string]bool{}
		for _, r := range refs {
			if problem := r.Problem(); problem != "" {
				refused = append(refused, fmt.Sprintf("its ref %q is left out: %s", r.Kind, problem))
				continue
			}
			if kinds[r.Kind] {
				refused = append(refused, fmt.Sprintf("its ref %q is registered twice: the second is left out", r.Kind))
				continue
			}
			var prefixes []string
			for _, p := range r.Prefixes {
				if whose, ok := taken[p]; ok {
					refused = append(refused, fmt.Sprintf("its ref %q doesn't name by %s: that's %s already", r.Kind, p, whose))
					continue
				}
				taken[p] = "its ref " + r.Kind + "'s"
				prefixes = append(prefixes, p)
			}
			if len(prefixes) == 0 {
				continue
			}
			kinds[r.Kind] = true
			r.Prefixes = prefixes
			take = append(take, r)
		}
		// What it registered as it did before stays as it is, and when.
		same := len(was) == len(take)
		for i := 0; same && i < len(take); i++ {
			a, _ := json.Marshal(was[i].Ref)
			b, _ := json.Marshal(take[i])
			same = string(a) == string(b)
		}
		if same {
			return nil
		}
		if _, err := tx.Exec(`DELETE FROM plugin_refs WHERE plugin = ?`, name); err != nil {
			return err
		}
		now := time.Now().UTC().Format(store.Stamp)
		names := map[string]ref.Named{}
		for i, r := range take {
			data, err := json.Marshal(r)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO plugin_refs (plugin, kind, place, ref, registered) VALUES (?, ?, ?, ?, ?)`, name, r.Kind, i, string(data), now); err != nil {
				return err
			}
			names[r.Kind] = named(name, r)
		}
		// A thing ws kept that the plugin no longer names isn't kept: a
		// kind it's dropped, a prefix it's let go of.
		things, err := thingsUnder(tx, name, "")
		if err != nil {
			return err
		}
		for _, t := range things {
			if n, ok := names[t.kind]; ok && n.Names(t.ref) {
				continue
			}
			if err := tx.Forget(t.origin); err != nil {
				return err
			}
		}
		return nil
	})
	return refused, err
}

// AnswerThings answers what the plugin who tells ws of its things
// (thing/put, thing/remove, thing/list), from the workspace's records. ok
// is false for a method that isn't one of those.
func AnswerThings(root, who, method string, params json.RawMessage) (answer any, ok bool, err error) {
	decode := func(into any) error {
		if len(params) == 0 || string(params) == "null" {
			return nil
		}
		if err := json.Unmarshal(params, into); err != nil {
			return InvalidParams(err)
		}
		return nil
	}
	switch method {
	case protocol.MethodThingPut:
		var in protocol.ThingPutParams
		if err := decode(&in); err != nil {
			return nil, true, err
		}
		if len(in.Things) > protocol.ThingsAtOnce {
			return nil, true, InvalidParams(fmt.Errorf("%d things at once: %d is the most", len(in.Things), protocol.ThingsAtOnce))
		}
		refused, err := PutThings(root, who, in.Things)
		return protocol.ThingPutResult{Refused: refused}, true, err
	case protocol.MethodThingRemove:
		var in protocol.ThingRemoveParams
		if err := decode(&in); err != nil {
			return nil, true, err
		}
		return struct{}{}, true, RemoveThings(root, who, in.Kind, in.Refs)
	case protocol.MethodThingList:
		var in protocol.ThingListParams
		if err := decode(&in); err != nil {
			return nil, true, err
		}
		refs, err := ListThings(root, who, in.Kind)
		return protocol.ThingListResult{Refs: refs}, true, err
	}
	return nil, false, nil
}

// PutThings keeps things a plugin tells ws of (thing/put), each in place
// of what ws had by that name: a row in the graph and what it says, to be
// found by, under the plugin. Refused are those it didn't keep, and why:
// one whose name isn't one of its kind's, as ws keeps how the plugin
// names things; one with no title; one ws has something of its own by the
// name of. One that's as ws has it already isn't written again.
func PutThings(root, name string, things []protocol.Thing) (refused []protocol.ThingRefused, err error) {
	db, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	err = db.Write(context.Background(), func(tx *store.Tx) error {
		refused = nil
		kept, err := refsIn(tx)
		if err != nil {
			return err
		}
		kinds := map[string]ref.Named{}
		for _, k := range kept {
			if k.Plugin == name {
				kinds[k.Kind] = k.Named()
			}
		}
		for _, t := range things {
			no := func(why string) {
				refused = append(refused, protocol.ThingRefused{Kind: t.Kind, Ref: t.Ref, Why: why})
			}
			title := clip(strings.Join(strings.Fields(t.Title), " "), thingTitle)
			n, ok := kinds[t.Kind]
			switch {
			case !ok:
				no("ws keeps no ref of " + name + "'s called " + t.Kind)
				continue
			case !n.Names(t.Ref):
				no("it isn't one of " + t.Kind + "'s names, as ws keeps them")
				continue
			case title == "":
				no("it has no title")
				continue
			}
			key, origin := store.Key{Name: t.Ref}, thingOrigin(name, t.Kind, t.Ref)
			state := clip(strings.Join(strings.Fields(t.State), " "), thingState)
			changed := t.Changed.UTC().Format(store.Stamp)
			text := words.Clean(clip(t.Text, thingText))

			var was, wasTitle, wasState, wasChanged string
			switch err := tx.QueryRow(`SELECT origin, title, state, changed FROM things WHERE scope = '' AND name = ?`, t.Ref).Scan(&was, &wasTitle, &wasState, &wasChanged); {
			case errors.Is(err, sql.ErrNoRows):
			case err != nil:
				return err
			case was == string(origin):
				var body string
				if err := tx.QueryRow(`SELECT words.body FROM words JOIN places p ON p.id = words.rowid WHERE p.origin = ?`, was).Scan(&body); err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				if wasTitle == title && wasState == state && wasChanged == changed && body == text {
					continue // as ws has it
				}
			case !strings.HasPrefix(was, string(Origin(name))+"/"):
				no("ws has something of its own called " + t.Ref)
				continue
			}
			// Its own, under this name: as it was, or as another kind.
			if was != "" {
				if err := tx.Forget(store.Origin(was)); err != nil {
					return err
				}
			}
			if err := tx.Thing(origin, store.Thing{Key: key, Kind: ThingKind, Title: title, State: state, Changed: t.Changed}); err != nil {
				return err
			}
			// Its name is among its title's words: it's looked for by its
			// name as often as by what it says.
			if err := tx.Words(origin, store.Place{Thing: key, Kind: ThingKind, Title: t.Ref + " " + title, Text: text, Said: t.Changed}); err != nil {
				return err
			}
		}
		return nil
	})
	return refused, err
}

// clip is the start of s, up to n bytes of it, whole runes.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// RemoveThings forgets things of a kind a plugin told ws of (thing/remove),
// by name. One ws doesn't have is nothing to forget.
func RemoveThings(root, name, kind string, refs []string) error {
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), func(tx *store.Tx) error {
		for _, r := range refs {
			if err := tx.Forget(thingOrigin(name, kind, r)); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListThings are the things of a kind ws keeps for a plugin (thing/list),
// by name, in order.
func ListThings(root, name, kind string) ([]string, error) {
	db, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	out := []string{}
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		things, err := thingsUnder(tx, name, kind)
		for _, t := range things {
			out = append(out, t.ref)
		}
		return err
	})
	return out, err
}

// keptThing is one of a plugin's things in the graph: the record it's of.
type keptThing struct {
	kind, ref string
	origin    store.Origin
}

// thingsUnder are the things ws keeps for a plugin: of one kind, or of
// every kind (""). A thing's record says its kind and its name.
func thingsUnder(tx *store.Tx, name, kind string) ([]keptThing, error) {
	under := string(Origin(name)) + "/"
	if kind != "" {
		under += kind + "/"
	}
	// Under it: from its slash up to, and not, what follows a slash.
	rows, err := tx.Query(`SELECT origin FROM things WHERE origin >= ? AND origin < ? ORDER BY origin`, under, strings.TrimSuffix(under, "/")+"0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []keptThing
	for rows.Next() {
		var origin string
		if err := rows.Scan(&origin); err != nil {
			return nil, err
		}
		k, r, ok := strings.Cut(strings.TrimPrefix(origin, string(Origin(name))+"/"), "/")
		if ok {
			out = append(out, keptThing{kind: k, ref: r, origin: store.Origin(origin)})
		}
	}
	return out, rows.Err()
}

// Refs are how every plugin's things are named, as ws keeps them: in the
// plugins' order, each one's as it registered them.
func Refs(root string) ([]KeptRef, error) {
	db, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	var out []KeptRef
	err = db.Read(context.Background(), func(tx *store.Tx) (err error) {
		out, err = refsIn(tx)
		return err
	})
	return out, err
}

func refsIn(tx *store.Tx) ([]KeptRef, error) {
	rows, err := tx.Query(`SELECT plugin, ref, registered FROM plugin_refs ORDER BY plugin, place`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []KeptRef
	for rows.Next() {
		var k KeptRef
		var data, at string
		if err := rows.Scan(&k.Plugin, &data, &at); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(data), &k.Ref); err != nil {
			return nil, fmt.Errorf("what's kept of how %s names things: %w", k.Plugin, err)
		}
		k.Registered, _ = time.Parse(store.Stamp, at)
		out = append(out, k)
	}
	return out, rows.Err()
}

// RefsOf are how one plugin's things are named, as ws keeps them.
func RefsOf(root, name string) ([]KeptRef, error) {
	all, err := Refs(root)
	return slices.DeleteFunc(all, func(k KeptRef) bool { return k.Plugin != name }), err
}

// Forget removes what ws keeps of a plugin in the workspace's records:
// how its things are named, and the things it told ws of, with what they
// say (everything in the graph under Origin).
// It's how removing a plugin leaves nothing of it behind: names that were
// its own are words again.
func Forget(root, name string) error {
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), func(tx *store.Tx) error {
		if _, err := tx.Exec(`DELETE FROM plugin_refs WHERE plugin = ?`, name); err != nil {
			return err
		}
		return tx.ForgetUnder(Origin(name))
	})
}
