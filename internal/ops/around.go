package ops

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
)

// What's around a thing: what's one link from it in the graph, either
// way. It's what Find shows before anything's typed, beside where you are:
// the mission's brief and handoffs, the talk it came from, the issue it's
// about, whatever names it. Asked as the records stand, it answers at
// once, for a window that's opening; asked fresh, it first reads again
// what's edited outside ws, as SaidOf does (a brief just written isn't in
// the graph until it's read), and the window shows that when it comes.

// Near is a thing one link from the one asked about: how it's linked (How:
// about, from, in, mentions, lands…) and which way (Out: the thing asked
// about points at it; otherwise it points at the thing).
type Near struct {
	Found
	How string
	Out bool
	At  time.Time
}

// passing are the kinds of link that say little of either end worth
// going to from the other: who made a thing, the stage it's for.
var passing = map[string]bool{store.By: true, store.For: true}

// written are the kinds of thing someone wrote (a person, or crew for
// one): what's read first of what names a thing.
var written = map[string]bool{
	"brief": true, "handoff": true, "file": true, "decision": true, "document": true,
	"talk": true, "cell": true, "page": true, "pr": true, "doc": true,
	"thread-ask": true, "thread-change": true, "thread-note": true, "comment": true,
}

// tier is how near a link is: what the thing is about, where it came
// from, where it lands, then what someone wrote of it, then the rest.
func tier(n Near) int {
	switch {
	case n.How == store.About || n.How == store.From || n.How == store.Lands:
		return 0
	case written[n.Thing.Kind]:
		return 1
	}
	return 2
}

// Around is what's one link from any of keys, nearest first (see tier),
// what still stands before what doesn't, then latest first; each thing
// once, by its nearest link, and none of keys among them. A link to
// something with no thing (a repo by its name) isn't followed: there's
// nothing to draw. At most limit (20 for none). Fresh, what's edited
// outside ws is read again first where it's moved: what can't be read
// just now is as it was last read.
func (w *Workspace) Around(ctx context.Context, keys []store.Key, limit int, fresh bool) ([]Near, error) {
	db, err := w.Records()
	if err != nil {
		return nil, err
	}
	if fresh {
		w.Missions.Reread(ctx)
		w.pages().Reread(ctx)
		w.Spaces(ctx)
	}
	asked := map[store.Key]bool{}
	for _, k := range keys {
		asked[k] = true
	}
	var out []Near
	var found []Found // out's, for whereOf and nameThings to say where each is
	err = db.Read(ctx, func(tx *store.Tx) error {
		type end struct {
			key store.Key
			l   store.Link
			out bool
		}
		own, err := ownText(tx, keys)
		if err != nil {
			return err
		}
		for _, k := range own {
			if k.Name == mission.BriefRef {
				asked[k] = true // the mission's own words: its page is the brief's
			}
		}
		var ends []end
		follow := func(k store.Key, from bool) error {
			links, err := tx.Links(k, from)
			if err != nil {
				return err
			}
			for _, l := range links {
				other := l.Src
				if from {
					other = l.Dst
				}
				if passing[l.Kind] || asked[other] {
					continue
				}
				ends = append(ends, end{other, l, from})
			}
			return nil
		}
		for _, k := range keys {
			for _, from := range []bool{true, false} {
				if err := follow(k, from); err != nil {
					return err
				}
			}
		}
		// What a mission's own text points at, the mission does: what its
		// brief names is what the mission's about.
		for _, k := range own {
			if err := follow(k, true); err != nil {
				return err
			}
		}
		others := make([]store.Key, 0, len(ends))
		for _, e := range ends {
			others = append(others, e.key)
		}
		things, err := tx.ThingsAt(others...)
		if err != nil {
			return err
		}
		for _, e := range ends {
			t, ok := things[e.key]
			if !ok {
				continue
			}
			f := named(store.Found{Thing: t, Place: fileOfMission(t.Kind), Ended: t.State == "archived"})
			out = append(out, Near{Found: f, How: e.l.Kind, Out: e.out, At: e.l.At})
		}
		slices.SortStableFunc(out, func(a, b Near) int {
			if c := cmp.Compare(tier(a), tier(b)); c != 0 {
				return c
			}
			as, bs := Said{State: a.Thing.State}.Standing(), Said{State: b.Thing.State}.Standing()
			switch {
			case as && !bs:
				return -1
			case bs && !as:
				return 1
			}
			return b.At.Compare(a.At)
		})
		seen := map[store.Key]bool{}
		out = slices.DeleteFunc(out, func(n Near) bool {
			if seen[n.Thing.Key] {
				return true
			}
			seen[n.Thing.Key] = true
			return false
		})
		if len(out) > cmp.Or(limit, 20) {
			out = out[:cmp.Or(limit, 20)]
		}
		found = make([]Found, len(out))
		for i := range out {
			found[i] = out[i].Found
		}
		return whereOf(tx, found)
	})
	if err != nil {
		return nil, err
	}
	w.nameThings(found)
	for i := range out {
		out[i].Found = found[i]
	}
	return out, nil
}

// ownText is the text written in any of keys that's its own (a mission's
// brief, its handoffs and its notes): in it, and read as its words are.
func ownText(tx *store.Tx, keys []store.Key) ([]store.Key, error) {
	var in []store.Key
	for _, k := range keys {
		links, err := tx.Links(k, false)
		if err != nil {
			return nil, err
		}
		for _, l := range links {
			if l.Kind == store.In && l.Src.Scope == k.Name {
				in = append(in, l.Src)
			}
		}
	}
	things, err := tx.ThingsAt(in...)
	if err != nil {
		return nil, err
	}
	var out []store.Key
	for _, k := range in {
		if fileOfMission(things[k].Kind) != "" {
			out = append(out, k)
		}
	}
	return out, nil
}

// fileOfMission is where a thing is led to, when it's a mission's file: what
// the find window goes to for one (its brief, a handoff, a file).
func fileOfMission(kind string) string {
	switch kind {
	case "brief", "handoff", "file":
		return kind
	}
	return ""
}
