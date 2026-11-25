package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// What ws keeps of plugins in the workspace's records: how each one's
// things are named (the refs it registered), by the plugin they're from,
// and the things a plugin tells ws of (things.go). A
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
		-- A thing a plugin told ws of: a value for each of its kind's
		-- fields, as told. What it's called, where it stands and what
		-- it's found by are made from this (things, words), and made
		-- again when its kind's fields or what it's found by change.
		CREATE TABLE plugin_things (
			plugin TEXT NOT NULL, kind TEXT NOT NULL, ref TEXT NOT NULL,
			changed TEXT NOT NULL,      -- when it last changed, as the plugin said
			fields TEXT NOT NULL,       -- its values by field, as JSON
			PRIMARY KEY (plugin, kind, ref)
		) WITHOUT ROWID;
	`), store.SQL(`
		-- A thing ws found in a space, of a kind the plugin says can live
		-- in a repo (found.go): named by its folder, and kept with what
		-- it was read at. '' is one the plugin told ws of.
		ALTER TABLE plugin_things ADD COLUMN found TEXT NOT NULL DEFAULT '';
	`), store.SQL(`
		-- A plugin's own name (sketch-12) for a thing ws put in a repo for
		-- it: the thing's name there. So the plugin's name still leads to
		-- the thing, and is written as the thing's where it goes into a
		-- repo itself.
		CREATE TABLE plugin_aliases (
			plugin TEXT NOT NULL, kind TEXT NOT NULL, ref TEXT NOT NULL,
			name TEXT NOT NULL,         -- the thing's, in its repo: its folder
			PRIMARY KEY (plugin, kind, ref)
		) WITHOUT ROWID;
	`), standingStep)
}

// Origin is what ws keeps in its graph for a plugin (its things, and
// what they say) kept under: each thing is a record of its own there
// (plugin:jira/issue/RAM-123), and Forget removes them all.
func Origin(name string) store.Origin { return store.Origin("plugin:" + name) }

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
		// What ws keeps of its things follows what it registers: one it
		// names no more isn't kept (a kind it's dropped, a prefix it's let
		// go of), and the rest are made again from what they were told as
		// (their kind's fields, or what they're found by, may have moved).
		things, err := thingsOf(tx, name, "")
		if err != nil {
			return err
		}
		all, err := refsIn(tx) // how every plugin's things are named, as it now is
		if err != nil {
			return err
		}
		defs := map[string]protocol.Ref{}
		for _, r := range take {
			defs[r.Kind] = r
		}
		for _, t := range things {
			// One it told ws of is kept while its kind names it; one ws
			// found in a space, while its kind's things can live in one.
			r, ok := defs[t.Kind]
			if t.Found == "" {
				ok = ok && names[t.Kind].Names(t.Ref)
			} else {
				ok = ok && r.Remote != nil
			}
			if ok {
				if kept, err := indexThing(tx, name, r, t, all); err != nil {
					return err
				} else if kept {
					continue
				}
			}
			if err := forgetThing(tx, name, t.Kind, t.Ref); err != nil {
				return err
			}
		}
		return nil
	})
	return refused, err
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
		for _, q := range []string{`DELETE FROM plugin_refs WHERE plugin = ?`, `DELETE FROM plugin_things WHERE plugin = ?`, `DELETE FROM plugin_aliases WHERE plugin = ?`} {
			if _, err := tx.Exec(q, name); err != nil {
				return err
			}
		}
		return tx.ForgetUnder(Origin(name))
	})
}

// standingStep is the part's fourth step: where a plugin's thing stands,
// as the plugin says in words every kind shares, and since when (empty where
// it says nothing); one written before stands in force, as one the plugin
// says nothing of does.
var standingStep = store.SQL(`
	ALTER TABLE plugin_things ADD COLUMN standing TEXT NOT NULL DEFAULT '';
	ALTER TABLE plugin_things ADD COLUMN since TEXT NOT NULL DEFAULT '';
	UPDATE things SET standing = 'open' WHERE kind = 'thing' AND standing = '';
`)
