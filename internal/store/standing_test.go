package store

import (
	"math"
	"slices"
	"testing"
	"time"
)

func TestStands_Worth(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	day := 24 * time.Hour
	for _, c := range []struct {
		name string
		s    Stands
		said time.Time
		want float64
	}{
		{"a kind that says nothing doesn't age", Stands{}, ago(400 * day), 1},
		{"what's kept true doesn't age", Stands{Standing: Lasting}, ago(400 * day), 1},
		{"what's in force doesn't age", Stands{Standing: InForce}, ago(400 * day), 1},
		{"what's passing, said now, is all there", Stands{Standing: Passing}, now, 1},
		{"what's passing, a half-life on, is three quarters there", Stands{Standing: Passing}, ago(halfLife), 0.75},
		{"what's passing, long ago, is at the floor", Stands{Standing: Passing}, ago(1000 * day), ageFloor},
		{"what's settled ages from when it settled", Stands{Standing: Settled, Since: now}, ago(400 * day), 1},
		{"or from what was said after", Stands{Standing: Settled, Since: ago(400 * day)}, ago(halfLife), 0.75},
		{"settled, since when not known, ages from what was said", Stands{Standing: Settled}, ago(halfLife), 0.75},
		{"what's gone is below, even now", Stands{Standing: Gone, Since: now}, ago(400 * day), goneWorth},
		{"and lower as it ages", Stands{Standing: Gone, Since: ago(1000 * day)}, ago(1000 * day), goneWorth * ageFloor},
	} {
		if got := c.s.Worth(c.said, now); math.Abs(got-c.want) > 0.001 {
			t.Errorf("%s: %.3f, want %.3f", c.name, got, c.want)
		}
	}
}

func TestStands_Under(t *testing.T) {
	early, late := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	inForce, passing, lasting := Stands{Standing: InForce}, Stands{Standing: Passing}, Stands{Standing: Lasting}
	settled := func(at time.Time) Stands { return Stands{Standing: Settled, Since: at} }
	gone := func(at time.Time) Stands { return Stands{Standing: Gone, Since: at} }
	for _, c := range []struct {
		name         string
		s, in, wants Stands
	}{
		{"in what's in force, its own", passing, inForce, passing},
		{"in nothing that says, its own", inForce, Stands{}, inForce},
		{"what's in force, in what's settled, is settled with it", inForce, settled(late), settled(late)},
		{"what's passing, in what's settled, is too", passing, settled(late), settled(late)},
		{"settled in settled: the later", settled(early), settled(late), settled(late)},
		{"settled later in settled earlier: its own", settled(late), settled(early), settled(late)},
		{"gone stays gone", gone(early), settled(late), gone(early)},
		{"in what's gone, gone", inForce, gone(late), gone(late)},
		{"what's kept true stays so in what's settled", lasting, settled(late), lasting},
	} {
		if got := c.s.Under(c.in); got != c.wants {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.wants)
		}
	}
}

// An old turn that says one thing, and a decision since that says
// another: asked what was decided, the decision's first, though the turn
// says the words more fully; once its mission's long over, the decision's
// below what's in play; once superseded, below what superseded it.
func TestFind_WeighedByStandingAndAge(t *testing.T) {
	db := own(t)
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	say(t, db, func(tx *Tx) error {
		mission := func(slug string, s Stands) {
			tx.Thing(Origin("mission:"+slug), Thing{Key: Key{Name: slug}, Kind: "mission", Title: slug, State: "in play", Changed: now, Stands: s})
		}
		mission("live", Stands{Standing: InForce})
		mission("over", Stands{Standing: Settled, Since: now.Add(-200 * day)})
		// A talk, its last word two months ago: it says the words the most.
		talk := Origin("conversation:/t1")
		tx.Thing(talk, Thing{Key: Key{Name: "t1"}, Kind: "talk", Title: "Retry the webhook", State: "open", Changed: now.Add(-60 * day), Stands: Stands{Standing: Passing}})
		tx.Words(talk, Place{Thing: Key{Name: "t1"}, At: "1", Kind: "turn", Title: "Retry the webhook", Text: "Retry the webhook forever: retries are cheap, so retry the webhook until it answers.", Said: now.Add(-60 * day)})
		// What else is said, so the words that matter score as they would.
		for i, text := range []string{"The cache is keyed by tenant.", "Tokens expire after an hour.", "Deploys go out on Tuesdays.", "The importer reads CSV.", "Logs are kept for a week.", "Names are sorted by surname.", "The queue is drained at night.", "Errors are sent to the channel."} {
			o := Origin("conversation:/t" + string(rune('2'+i)))
			k := Key{Name: "t" + string(rune('2'+i))}
			tx.Thing(o, Thing{Key: k, Kind: "talk", Title: text, State: "open", Changed: now, Stands: Stands{Standing: Passing}})
			tx.Words(o, Place{Thing: k, At: "1", Kind: "turn", Text: text, Said: now})
		}
		decide := func(slug, id, what string, s Stands, when time.Time) {
			o := Origin("decision:" + slug + "/" + id)
			k := Key{Scope: slug, Name: id}
			tx.Thing(o, Thing{Key: k, Kind: "decision", Title: what, State: "current", Changed: when, Stands: s})
			tx.Words(o, Place{Thing: k, Kind: "decision", Title: what, Text: "Webhook retries.", Said: when})
		}
		decide("live", "d1", "Retry the webhook three times", Stands{Standing: InForce}, now.Add(-10*day))
		decide("live", "d2", "Retry the webhook twice", Stands{Standing: Gone, Since: now.Add(-10 * day)}, now.Add(-30*day))
		decide("over", "d1", "Retry the webhook five times", Stands{Standing: InForce}, now.Add(-300*day))
		return nil
	})
	found, _, err := db.Find(bg, Said{Words: "retry the webhook", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	got := refs(found)
	if want := []string{"live/d1", "/t1"}; !slices.Equal(got[:2], want) {
		t.Errorf("found %q: want %q first, the decision in force over the old talk", got, want)
	}
	at := map[string]int{}
	for i, r := range got {
		at[r] = i
	}
	if at["over/d1"] < at["live/d1"] || at["live/d2"] < at["live/d1"] {
		t.Errorf("found %q: what's of a mission long over, and what's superseded, below what's in force", got)
	}
	for _, f := range found {
		if r := f.Thing.Key.Scope + "/" + f.Thing.Key.Name; r == "over/d1" && f.Stands.Standing != Settled {
			t.Errorf("a decision in a mission that's over stands as it does: %+v", f.Stands)
		}
	}
	// Asked then, the talk was new: it's first, as it says the words most.
	found, _, _ = db.Find(bg, Said{Words: "retry the webhook", Now: now.Add(-60 * day)})
	if got := refs(found); got[0] != "/t1" {
		t.Errorf("asked when it was said, the talk first: %q", got)
	}
}
