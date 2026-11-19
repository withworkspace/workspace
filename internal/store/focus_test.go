package store

import (
	"slices"
	"testing"
	"time"
)

func pinned(t *testing.T, db *DB) []string {
	t.Helper()
	var out []string
	if err := db.Read(bg, func(tx *Tx) error {
		pins, err := tx.Pins()
		for _, p := range pins {
			name := p.Key.Name
			if p.Thing != nil {
				name += " (" + p.Thing.Title + ")"
			}
			out = append(out, name)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func write(t *testing.T, db *DB, f func(tx *Tx) error) {
	t.Helper()
	if err := db.Write(bg, f); err != nil {
		t.Fatal(err)
	}
}

func TestPins_KeptInOrder_ArrangedAndUnpinned(t *testing.T) {
	db := own(t)
	at := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	write(t, db, func(tx *Tx) error {
		for _, n := range []string{"cache", "demo#12", "w4"} {
			if err := tx.Pin(Key{Name: n}, at); err != nil {
				return err
			}
		}
		return tx.Pin(Key{Name: "cache"}, at.Add(time.Hour)) // pinned already: stays first
	})
	if got := pinned(t, db); !slices.Equal(got, []string{"cache", "demo#12", "w4"}) {
		t.Fatalf("pins = %v", got)
	}
	write(t, db, func(tx *Tx) error { return tx.ArrangePins([]Key{{Name: "w4"}, {Name: "nope"}, {Name: "cache"}}) })
	if got := pinned(t, db); !slices.Equal(got, []string{"w4", "cache", "demo#12"}) {
		t.Fatalf("arranged = %v", got)
	}
	write(t, db, func(tx *Tx) error {
		if was, err := tx.Unpin(Key{Name: "cache"}); err != nil || !was {
			t.Errorf("unpin cache = %v, %v", was, err)
		}
		if was, err := tx.Unpin(Key{Name: "cache"}); err != nil || was {
			t.Errorf("unpin cache again = %v, %v", was, err)
		}
		return tx.Pin(Key{Name: "p3"}, at)
	})
	if got := pinned(t, db); !slices.Equal(got, []string{"w4", "demo#12", "p3"}) {
		t.Fatalf("after unpin = %v", got)
	}
}

func TestPins_ReadTheGraph_OutliveARewrite_GoWithTheirRecordForGood(t *testing.T) {
	db := own(t)
	at := time.Now()
	thing := func(tx *Tx, origin Origin, name, title string) error {
		return tx.Thing(origin, Thing{Key: Key{Name: name}, Kind: "thing", Title: title, State: "open", Changed: at})
	}
	write(t, db, func(tx *Tx) error {
		if err := thing(tx, "plugin:jira/issue/RAM-1", "RAM-1", "Export fails"); err != nil {
			return err
		}
		if err := thing(tx, "mission:cache", "cache", "Cache the index"); err != nil {
			return err
		}
		for _, n := range []string{"RAM-1", "cache", "demo#12"} {
			if err := tx.Pin(Key{Name: n}, at); err != nil {
				return err
			}
		}
		return nil
	})
	if got := pinned(t, db); !slices.Equal(got, []string{"RAM-1 (Export fails)", "cache (Cache the index)", "demo#12"}) {
		t.Fatalf("pins = %v", got)
	}
	// A record writing its things again forgets them first: its pins stay,
	// and read what it wrote.
	write(t, db, func(tx *Tx) error {
		if err := tx.Forget("mission:cache"); err != nil {
			return err
		}
		return thing(tx, "mission:cache", "cache", "Cache the whole index")
	})
	if got := pinned(t, db); !slices.Equal(got, []string{"RAM-1 (Export fails)", "cache (Cache the whole index)", "demo#12"}) {
		t.Fatalf("after a rewrite = %v", got)
	}
	// The plugin's removed: its things go, and so do their pins.
	since := seq(t, db)
	write(t, db, func(tx *Tx) error { return tx.ForgetUnder("plugin:jira") })
	if got := pinned(t, db); !slices.Equal(got, []string{"cache (Cache the whole index)", "demo#12"}) {
		t.Fatalf("after the plugin's removed = %v", got)
	}
	if topics := changedSince(t, db, since); !slices.Contains(topics, FocusTopic) {
		t.Errorf("unpinning named %v, not %s", topics, FocusTopic)
	}
}

func seq(t *testing.T, db *DB) int64 {
	t.Helper()
	n, err := db.Seq(bg)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func changedSince(t *testing.T, db *DB, since int64) []string {
	t.Helper()
	topics, _, err := db.Since(bg, since)
	if err != nil {
		t.Fatal(err)
	}
	return topics
}
