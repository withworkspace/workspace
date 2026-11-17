package store

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/words"
)

// say writes things and their words, as stores do.
func say(t testing.TB, db *DB, fn func(tx *Tx) error) {
	t.Helper()
	if err := db.Write(bg, fn); err != nil {
		t.Fatal(err)
	}
}

func refs(found []Found) []string {
	var out []string
	for _, f := range found {
		r := f.Thing.Key.Scope + "/" + f.Thing.Key.Name
		if f.At != "" {
			r += "@" + f.At
		}
		out = append(out, r)
	}
	return out
}

func TestWords_FoundLedToTheBestPlace_AndForgottenWithTheirRecord(t *testing.T) {
	db := own(t)
	at := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	say(t, db, func(tx *Tx) error {
		mission := Origin("mission:cache")
		tx.Thing(mission, Thing{Key: Key{Name: "cache"}, Kind: "mission", Title: "Cache tokens per tenant", State: "in play", Changed: at})
		thread := Origin("conversation:cache/t1")
		tx.Thing(thread, Thing{Key: Key{Scope: "cache", Name: "t1"}, Kind: "thread-ask", Title: "", State: "open", Changed: at})
		for i, body := range []string{"Where do we keep them?", "In the token cache, keyed by tenant and expiring with the token.", "Thanks."} {
			if err := tx.Words(thread, Place{Thing: Key{Scope: "cache", Name: "t1"}, At: fmt.Sprint(i + 1), Kind: "turn", Text: body, Said: at.Add(time.Duration(i) * time.Minute)}); err != nil {
				return err
			}
		}
		d1 := Origin("decision:cache/d1")
		tx.Thing(d1, Thing{Key: Key{Scope: "cache", Name: "d1"}, Kind: "decision", Title: "Key the token cache by tenant", State: "current", Changed: at})
		return tx.Words(d1, Place{Thing: Key{Scope: "cache", Name: "d1"}, Kind: "decision", Title: "Key the token cache by tenant", Text: "Tenants share nothing.", Said: at})
	})
	found, more, err := db.Find(bg, Said{Words: "token cache"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := refs(found), []string{"cache/d1", "cache/t1@2"}; !slices.Equal(got, want) || more != 0 {
		t.Fatalf("found %q (%d more), want %q: the decision titled so first, then the turn that says it", got, more, want)
	}
	if !found[1].Every || found[1].Thing.Kind != "thread-ask" || found[1].Place != "turn" {
		t.Errorf("the thread's turn: %+v", found[1])
	}
	if snip := words.Text(found[1].Snippet, "[", "]"); !strings.Contains(snip, "[token] [cache]") {
		t.Errorf("snippet %q", snip)
	}

	// From a command, through the daemon, the same.
	if got, _, err := borrow(t, db).Find(bg, Said{Words: "token cache"}); err != nil || !slices.Equal(refs(got), refs(found)) {
		t.Errorf("from a command: %q, %v", refs(got), err)
	}

	// Forgetting a record forgets its words.
	say(t, db, func(tx *Tx) error { return tx.Forget("decision:cache/d1") })
	if found, _, _ = db.Find(bg, Said{Words: "token cache"}); !slices.Equal(refs(found), []string{"cache/t1@2"}) {
		t.Errorf("after the decision's forgotten: %q", refs(found))
	}
	var places int
	db.Read(bg, func(tx *Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM places WHERE origin = 'decision:cache/d1'`).Scan(&places)
	})
	if places != 0 {
		t.Errorf("%d places left of what was forgotten", places)
	}
}

func TestWords_Retitle_ForgetAKind(t *testing.T) {
	db := own(t)
	o := Origin("conversation:/w1")
	k := Key{Name: "w1"}
	say(t, db, func(tx *Tx) error {
		tx.Thing(o, Thing{Key: k, Kind: "pass", Title: "w1", State: "open"})
		tx.Words(o, Place{Thing: k, At: "1", Kind: "turn", Text: "The page reads well."})
		tx.Words(o, Place{Thing: k, At: "check:1", Kind: "finding", Text: "Nothing says who approves a refund."})
		return nil
	})
	say(t, db, func(tx *Tx) error { return tx.Retitle(o, "Refund policy") })
	if found, _, _ := db.Find(bg, Said{Words: "refund policy"}); len(found) != 1 || found[0].At != "" {
		t.Errorf("found by its new title, as a whole: %+v", found)
	}
	say(t, db, func(tx *Tx) error { return tx.ForgetWords(o, "finding") })
	if found, _, _ := db.Find(bg, Said{Words: "approves"}); len(found) != 0 {
		t.Errorf("a finding forgotten is still found: %+v", found)
	}
	if found, _, _ := db.Find(bg, Said{Words: "reads"}); len(found) != 1 {
		t.Errorf("the turn went with the findings: %+v", found)
	}
}

func TestFind_WithdrawnLeftOut_ArchivedBelow_NewerFirst_OneMission(t *testing.T) {
	db := own(t)
	at := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	say(t, db, func(tx *Tx) error {
		for _, m := range []struct{ slug, state string }{{"old", "archived"}, {"new", "in play"}} {
			tx.Thing(Origin("mission:"+m.slug), Thing{Key: Key{Name: m.slug}, Kind: "mission", Title: m.slug, State: m.state})
		}
		decide := func(slug, id, state, what string, when time.Time) {
			o := Origin("decision:" + slug + "/" + id)
			k := Key{Scope: slug, Name: id}
			tx.Thing(o, Thing{Key: k, Kind: "decision", Title: what, State: state, Changed: when})
			tx.Words(o, Place{Thing: k, Kind: "decision", Title: what, Said: when})
		}
		decide("old", "d1", "current", "Retry the webhook three times", at)
		decide("new", "d1", "current", "Retry the webhook three times", at.Add(-time.Hour))
		decide("new", "d2", "withdrawn", "Retry the webhook forever", at)
		// A brief's section: the mission is the thing.
		return tx.Words("missionfile:new/brief.md", Place{Thing: Key{Name: "new"}, At: "goal", Kind: "brief", Title: "new", Heading: "Goal", Text: "Webhooks that fail are retried.", Said: at.Add(-2 * time.Hour)})
	})
	found, _, _ := db.Find(bg, Said{Words: "retry the webhook"})
	if got, want := refs(found), []string{"new/d1", "old/d1", "/new@goal"}; !slices.Equal(got, want) {
		t.Errorf("found %q, want %q: an archived mission's below the one in play, the withdrawn left out", got, want)
	}
	if !found[1].Ended || found[0].Ended {
		t.Errorf("which is archived: %+v", found)
	}
	found, _, _ = db.Find(bg, Said{Words: "retry the webhook", Gone: true})
	if !slices.Contains(refs(found), "new/d2") {
		t.Errorf("asked for, the withdrawn is found: %q", refs(found))
	}
	found, _, _ = db.Find(bg, Said{Words: "webhook", Mission: "new"})
	if got, want := refs(found), []string{"new/d1", "/new@goal"}; !slices.Equal(got, want) {
		t.Errorf("in mission new: %q, want %q", got, want)
	}
	if found[1].Thing.Kind != "mission" || found[1].Place != "brief" || found[1].Heading != "Goal" {
		t.Errorf("a brief's section is the mission's: %+v", found[1])
	}

	// Words that say nothing to look for, or what isn't FTS5's own.
	for _, typed := range []string{"", "-- !", `demo#12 "per`, "token-cache"} {
		if _, _, err := db.Find(bg, Said{Words: typed}); err != nil {
			t.Errorf("%q: %v", typed, err)
		}
	}
}
