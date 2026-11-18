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
	"unicode"
	"unicode/utf8"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// The things a plugin tells ws of (thing/put): each is a record of
// values, one for each of its kind's fields, kept as told
// (plugin_things). From it ws makes what the rest of ws reads: a thing
// in the graph, called by its title field and standing where its state
// field says, and the words it's found by, from the fields its kind says
// it's found by. ws makes those itself, so it can make them again when
// the kind changes, without the plugin telling it anything twice.
//
// A thing is under its plugin: the record by its plugin's name, and what's
// made from it by an origin of its own (plugin:jira/issue/RAM-123), so
// removing the plugin forgets all of it (Forget).

// ThingKind is what one of a plugin's things is in the graph: what it is
// to its plugin (an issue) is the ref it's named by.
const ThingKind = "thing"

// thingOrigin is what's made of one of a plugin's things, in the graph.
func thingOrigin(name, kind, ref string) store.Origin {
	return store.Origin(string(Origin(name)) + "/" + kind + "/" + ref)
}

// Record is one of a plugin's things as ws keeps it: its kind and name,
// when it changed, and its values by field.
type Record struct {
	Kind, Ref string
	Changed   time.Time
	Fields    map[string]string
}

// How much of a field ws keeps, by its type: a title and a state are a
// line, words are a few, and what's said at length is kept for finding
// it by and for looking at, not as the plugin's copy of it.
const (
	keptTitle    = 300
	keptState    = 60
	keptText     = 2000
	keptMarkdown = 50000
	// keptWords is how much of what a thing's found by is looked through.
	keptWords = 20000
)

// Tidy is a thing's values as ws keeps them: those its kind has a field
// for, each cut to what ws keeps of its type, a title and a state on one
// line. Nothing's kept for a field with nothing in it.
func Tidy(kind protocol.Ref, fields map[string]string) map[string]string {
	out := map[string]string{}
	for _, f := range kind.Fields {
		v := strings.TrimSpace(fields[f.Name])
		switch f.Type {
		case protocol.FieldTitle:
			v = clip(strings.Join(strings.Fields(v), " "), keptTitle)
		case protocol.FieldState:
			v = clip(strings.Join(strings.Fields(v), " "), keptState)
		case protocol.FieldMarkdown:
			v = clip(v, keptMarkdown)
		default:
			v = clip(v, keptText)
		}
		if v != "" {
			out[f.Name] = v
		}
	}
	return out
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

// foundBy is the words a thing is found by: the fields its kind says, in
// order, each as far as it says, or every field but its title when it
// says none. Its title and its name are found besides, whatever's said.
func foundBy(kind protocol.Ref, rec Record) string {
	by := kind.Search
	if len(by) == 0 {
		for _, f := range kind.Fields {
			if f.Type != protocol.FieldTitle {
				by = append(by, protocol.Searched{Field: f.Name})
			}
		}
	}
	var parts []string
	for _, s := range by {
		v := rec.Fields[s.Field]
		if s.Field == protocol.FieldKey {
			v = rec.Ref
		}
		if s.Max > 0 {
			v = clip(v, s.Max)
		}
		if v != "" {
			parts = append(parts, v)
		}
	}
	return clip(strings.Join(parts, "\n\n"), keptWords)
}

// linksOf is what a thing points at, from its fields that name other
// things: each name, and how its field says it points (mentions, where it
// says nothing ws knows). A name is one ws reads without looking further:
// one of a plugin's things, as all holds how they're named, or a pull
// request (demo#12).
func linksOf(kind protocol.Ref, rec Record, all []KeptRef) []store.Link {
	var out []store.Link
	seen := map[string]bool{}
	for _, f := range kind.Fields {
		if f.Type != protocol.FieldRef && f.Type != protocol.FieldRefs {
			continue
		}
		how := store.Mentions
		switch f.Link {
		case protocol.LinkIn, protocol.LinkAbout, protocol.LinkFrom, protocol.LinkOf:
			how = f.Link
		}
		for _, named := range Names(rec.Fields[f.Name]) {
			if named == rec.Ref || seen[how+" "+named] {
				continue
			}
			_, _, pr := ref.ParsePR(named)
			thing := slices.ContainsFunc(all, func(k KeptRef) bool { return k.Named().Names(named) })
			if !pr && !thing {
				continue
			}
			seen[how+" "+named] = true
			out = append(out, store.Link{Src: store.Key{Name: rec.Ref}, Kind: how, Dst: store.Key{Name: named}, At: rec.Changed})
		}
	}
	return out
}

// Names are the names in a field that names other things: one, or
// several separated by spaces, commas or lines.
func Names(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || unicode.IsSpace(r) })
}

// indexThing makes what ws reads of one of name's things from its record,
// as its kind now is: its thing in the graph, what it points at, and the
// words it's found by, in place of what was made of it before. all is
// how every plugin's things are named, for what its fields name. A record
// with nothing for its kind's title makes nothing: kept is false, and
// it's the caller's to forget.
func indexThing(tx *store.Tx, name string, kind protocol.Ref, rec Record, all []KeptRef) (kept bool, err error) {
	fields := Tidy(kind, rec.Fields)
	title := fields[kind.Typed(protocol.FieldTitle)]
	if title == "" {
		return false, nil
	}
	key, origin := store.Key{Name: rec.Ref}, thingOrigin(name, kind.Kind, rec.Ref)
	if err := tx.Forget(origin); err != nil {
		return false, err
	}
	if err := tx.Thing(origin, store.Thing{Key: key, Kind: ThingKind, Title: title, State: fields[kind.Typed(protocol.FieldState)], Changed: rec.Changed}); err != nil {
		return false, err
	}
	tidy := rec
	tidy.Fields = fields
	for _, l := range linksOf(kind, tidy, all) {
		if err := tx.Link(origin, l); err != nil {
			return false, err
		}
	}
	// Its name is among its title's words: it's looked for by its name
	// as often as by what it says.
	return true, tx.Words(origin, store.Place{Thing: key, Kind: ThingKind, Title: rec.Ref + " " + title, Text: foundBy(kind, tidy), Said: rec.Changed})
}

// forgetThing removes one of name's things: its record, and what was
// made of it.
func forgetThing(tx *store.Tx, name, kind, ref string) error {
	if _, err := tx.Exec(`DELETE FROM plugin_things WHERE plugin = ? AND kind = ? AND ref = ?`, name, kind, ref); err != nil {
		return err
	}
	return tx.Forget(thingOrigin(name, kind, ref))
}

// thingsOf are the records ws keeps of name's things: of one kind, or of
// every kind (""), in order.
func thingsOf(tx *store.Tx, name, kind string) ([]Record, error) {
	q, args := `SELECT kind, ref, changed, fields FROM plugin_things WHERE plugin = ?`, []any{name}
	if kind != "" {
		q, args = q+` AND kind = ?`, append(args, kind)
	}
	rows, err := tx.Query(q+` ORDER BY kind, ref`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var r Record
		var changed, fields string
		if err := rows.Scan(&r.Kind, &r.Ref, &changed, &fields); err != nil {
			return nil, err
		}
		r.Changed, _ = time.Parse(store.Stamp, changed)
		if err := json.Unmarshal([]byte(fields), &r.Fields); err != nil {
			return nil, fmt.Errorf("what's kept of %s's %s: %w", name, r.Ref, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PutThings keeps things a plugin tells ws of (thing/put), each in place
// of what ws had by that name: its record, and what's made of it. Refused
// are those it didn't keep, and why: one whose name isn't one of its
// kind's, as ws keeps how the plugin names things; one with nothing for
// its kind's title; one ws has something of its own by the name of. One
// that's as ws has it already isn't written again.
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
		kinds := map[string]KeptRef{}
		for _, k := range kept {
			if k.Plugin == name {
				kinds[k.Kind] = k
			}
		}
		for _, t := range things {
			no := func(why string) {
				refused = append(refused, protocol.ThingRefused{Kind: t.Kind, Ref: t.Ref, Why: why})
			}
			kind, ok := kinds[t.Kind]
			if !ok {
				no("ws keeps no ref of " + name + "'s called " + t.Kind)
				continue
			}
			titled := kind.Typed(protocol.FieldTitle)
			fields := Tidy(kind.Ref, t.Fields)
			switch {
			case !kind.Named().Names(t.Ref):
				no("it isn't one of " + t.Kind + "'s names, as ws keeps them")
				continue
			case titled == "":
				no(t.Kind + " has no title field: there's nothing to call one by")
				continue
			case fields[titled] == "":
				no("it has nothing for " + titled + ", its title")
				continue
			}
			// Whose the name is, in the graph, if it's anyone's.
			var whose string
			switch err := tx.QueryRow(`SELECT origin FROM things WHERE scope = '' AND name = ?`, t.Ref).Scan(&whose); {
			case errors.Is(err, sql.ErrNoRows):
			case err != nil:
				return err
			case !strings.HasPrefix(whose, string(Origin(name))+"/"):
				no("ws has something of its own called " + t.Ref)
				continue
			case whose != string(thingOrigin(name, t.Kind, t.Ref)):
				// Its own, as another kind: that one goes.
				was, _, _ := strings.Cut(strings.TrimPrefix(whose, string(Origin(name))+"/"), "/")
				if err := forgetThing(tx, name, was, t.Ref); err != nil {
					return err
				}
			}
			told, err := json.Marshal(fields) // its keys in order: the same told twice is the same
			if err != nil {
				return err
			}
			changed := t.Changed.UTC().Format(store.Stamp)
			var wasChanged, wasFields string
			switch err := tx.QueryRow(`SELECT changed, fields FROM plugin_things WHERE plugin = ? AND kind = ? AND ref = ?`, name, t.Kind, t.Ref).Scan(&wasChanged, &wasFields); {
			case errors.Is(err, sql.ErrNoRows):
			case err != nil:
				return err
			case wasChanged == changed && wasFields == string(told):
				continue // as ws has it
			}
			if _, err := tx.Exec(`INSERT INTO plugin_things (plugin, kind, ref, changed, fields) VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (plugin, kind, ref) DO UPDATE SET changed = excluded.changed, fields = excluded.fields`, name, t.Kind, t.Ref, changed, string(told)); err != nil {
				return err
			}
			if _, err := indexThing(tx, name, kind.Ref, Record{Kind: t.Kind, Ref: t.Ref, Changed: t.Changed, Fields: fields}, kept); err != nil {
				return err
			}
		}
		return nil
	})
	return refused, err
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
			if err := forgetThing(tx, name, kind, r); err != nil {
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
		things, err := thingsOf(tx, name, kind)
		for _, t := range things {
			out = append(out, t.Ref)
		}
		return err
	})
	return out, err
}

// ThingOf is one of a plugin's things as ws keeps it, if the plugin's
// told ws of it: what ws draws it from without asking.
func ThingOf(root, name, kind, ref string) (Record, bool, error) {
	db, err := store.Open(root)
	if err != nil {
		return Record{}, false, err
	}
	rec := Record{Kind: kind, Ref: ref}
	found := false
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		var changed, fields string
		switch err := tx.QueryRow(`SELECT changed, fields FROM plugin_things WHERE plugin = ? AND kind = ? AND ref = ?`, name, kind, ref).Scan(&changed, &fields); {
		case errors.Is(err, sql.ErrNoRows):
			return nil
		case err != nil:
			return err
		}
		found = true
		rec.Changed, _ = time.Parse(store.Stamp, changed)
		return json.Unmarshal([]byte(fields), &rec.Fields)
	})
	return rec, found, err
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
