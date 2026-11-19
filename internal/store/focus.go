package store

import (
	"database/sql"
	"time"
)

// Focus: the things you've pinned, in the order you keep them. A pin is a
// thing's key, so anything with a ref can be pinned (a mission, a
// conversation, a pull request, a document, a plugin's thing), and what
// it's called and how it stands are read from the graph when it's shown,
// not copied. A pin may name what the graph hasn't a thing for (a pull
// request ws keeps nothing of, a file): it's shown by its name.
//
// Pins are kept apart from the graph's rows, which a record forgets and
// writes again whenever it changes (Forget): a pin outlives that. It goes
// when it's unpinned, or when what it names is removed for good
// (ForgetUnder: a plugin uninstalled, a space dropped).

// focusTables are the focus's: the store part's fourth step.
const focusTables = `
		CREATE TABLE focus (
			scope TEXT NOT NULL, name TEXT NOT NULL,
			place INTEGER NOT NULL,   -- where it is among the pins, first lowest
			pinned TEXT NOT NULL,     -- when it was pinned
			PRIMARY KEY (scope, name)
		) WITHOUT ROWID;
`

// FocusTopic is the topic a change to the pins names: they're shown
// beside everything, so it's the workspace's.
const FocusTopic = "workspace"

// Pin is a pinned thing, with what the graph says of it (Thing is nil
// where the graph has nothing for its key) and its record (Origin, "" with
// it).
type Pin struct {
	Key    Key
	Pinned time.Time
	Thing  *Thing
	Origin Origin
}

// Pin pins k after the rest. One that's pinned already stays where it is.
func (tx *Tx) Pin(k Key, at time.Time) error {
	res, err := tx.Exec(`INSERT INTO focus (scope, name, place, pinned)
		SELECT ?, ?, coalesce(max(place) + 1, 0), ? FROM focus WHERE true -- (an upsert's SELECT needs a WHERE)
		ON CONFLICT (scope, name) DO NOTHING`, k.Scope, k.Name, at.UTC().Format(Stamp))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		tx.Changed(FocusTopic)
	}
	return nil
}

// Unpin unpins k, and says whether it was pinned.
func (tx *Tx) Unpin(k Key) (bool, error) {
	res, err := tx.Exec(`DELETE FROM focus WHERE scope = ? AND name = ?`, k.Scope, k.Name)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		tx.Changed(FocusTopic)
	}
	return n > 0, nil
}

// ArrangePins puts the pins in keys first, in that order, and the rest
// after them as they were. A key that isn't pinned is passed by.
func (tx *Tx) ArrangePins(keys []Key) error {
	pins, err := tx.Pins()
	if err != nil {
		return err
	}
	pinned := map[Key]bool{}
	for _, p := range pins {
		pinned[p.Key] = true
	}
	var order []Key
	placed := map[Key]bool{}
	for _, k := range keys {
		if pinned[k] && !placed[k] {
			placed[k] = true
			order = append(order, k)
		}
	}
	for _, p := range pins {
		if !placed[p.Key] {
			order = append(order, p.Key)
		}
	}
	for i, k := range order {
		if _, err := tx.Exec(`UPDATE focus SET place = ? WHERE scope = ? AND name = ?`, i, k.Scope, k.Name); err != nil {
			return err
		}
	}
	tx.Changed(FocusTopic)
	return nil
}

// Pins are the pins in order, each with what the graph says of it.
func (tx *Tx) Pins() ([]Pin, error) {
	rows, err := tx.Query(`SELECT f.scope, f.name, f.pinned, t.kind, t.title, t.state, t.changed, t.origin
		FROM focus f LEFT JOIN things t ON t.scope = f.scope AND t.name = f.name
		ORDER BY f.place, f.pinned`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pin
	for rows.Next() {
		var p Pin
		var pinned string
		var kind, title, state, changed, origin sql.NullString
		if err := rows.Scan(&p.Key.Scope, &p.Key.Name, &pinned, &kind, &title, &state, &changed, &origin); err != nil {
			return nil, err
		}
		p.Pinned, _ = time.Parse(Stamp, pinned)
		if kind.Valid {
			t := Thing{Key: p.Key, Kind: kind.String, Title: title.String, State: state.String}
			t.Changed, _ = time.Parse(Stamp, changed.String)
			p.Thing, p.Origin = &t, Origin(origin.String)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
