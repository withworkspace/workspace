package store

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// Step changes the file from one layout of a part to the next: tables
// made or altered, or records brought in. Each runs once, in a
// transaction of its own, when the daemon takes the file.
type Step func(tx *Tx) error

// SQL is a step that's statements and nothing else.
func SQL(statements string) Step {
	return func(tx *Tx) error {
		_, err := tx.Exec(statements)
		return err
	}
}

// Retired is a step that did something only workspaces from before it
// needed (brought their files across into the records), and now does
// nothing: kept in its place, as steps are numbered by where they are, so
// a file that had it is at the layout it says.
func Retired(*Tx) error { return nil }

var parts = map[string][]Step{}

// Define says what a part of the file is (its tables, as a package owns
// them), as the steps that make it: its layout is how many there are.
// Steps are only ever added at the end. Called from a package's init.
func Define(name string, steps ...Step) {
	if _, ok := parts[name]; ok {
		panic("store: " + name + " is defined twice")
	}
	if name == "" || strings.ContainsAny(name, "/ ") {
		panic("store: a part's name is a word: " + name)
	}
	parts[name] = steps
}

// layouts is each part's layout as this build of ws has it.
func layouts() map[string]int {
	out := map[string]int{}
	for name, steps := range parts {
		out[name] = len(steps)
	}
	return out
}

func init() {
	// The store's own part: what's kept of the file itself, and what
	// changed, for pages to be told; then the graph (graph.go), what
	// things say (words.go), what you've pinned of them (focus.go), and
	// where things stand in words every kind shares (standing.go).
	Define("store", SQL(`
		CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value) WITHOUT ROWID;
		INSERT OR IGNORE INTO meta (key, value) VALUES ('seq', 0);
		CREATE TABLE changes (topic TEXT PRIMARY KEY, seq INTEGER NOT NULL) WITHOUT ROWID;
		CREATE INDEX changes_seq ON changes (seq);
	`), SQL(graphTables), SQL(wordTables), SQL(focusTables), SQL(standingColumns))
}

// bringForward runs every part's steps the file hasn't had, the store's
// own first, each step in its own transaction. A part the file has more
// of than this ws knows is from a newer ws: nothing is changed.
func bringForward(ctx context.Context, root string, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value) WITHOUT ROWID`); err != nil {
		return err
	}
	have := map[string]int{}
	rows, err := db.QueryContext(ctx, `SELECT substr(key, 8), value FROM meta WHERE key LIKE 'layout/%'`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name string
		var v int
		if err := rows.Scan(&name, &v); err != nil {
			rows.Close()
			return err
		}
		have[name] = v
	}
	rows.Close()
	// Read short, have would say a part is further back than it is, and
	// its steps would be run again.
	if err := rows.Err(); err != nil {
		return err
	}
	names := slices.Sorted(maps.Keys(parts))
	sort.SliceStable(names, func(i, j int) bool { return names[i] == "store" && names[j] != "store" })
	for _, name := range names {
		steps := parts[name]
		if have[name] > len(steps) {
			return fmt.Errorf("the workspace's records (%s) are from a newer ws: its %s is at layout %d, and this ws knows %d", "ws.db", name, have[name], len(steps))
		}
		for i := have[name]; i < len(steps); i++ {
			if err := runStep(ctx, root, db, name, i, steps[i]); err != nil {
				return fmt.Errorf("bringing the records' %s to layout %d: %w", name, i+1, err)
			}
		}
	}
	return nil
}

func runStep(ctx context.Context, root string, db *sql.DB, name string, i int, step Step) error {
	stx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer stx.Rollback()
	// A step names nothing (asked): it runs as the file's taken, before
	// there's a workspace to ask what there is to name.
	tx := &Tx{tx: stx, ctx: ctx, root: root, write: true, asked: true}
	if err := step(tx); err != nil {
		return err
	}
	if err := tx.noteChanges(); err != nil {
		return err
	}
	if _, err := stx.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, "layout/"+name, i+1); err != nil {
		return err
	}
	if err := stx.Commit(); err != nil {
		return err
	}
	tx.committed()
	return nil
}

// differ says how a command's layouts differ from the daemon's, as an
// error for the command, or nil when they're the same.
func differ(daemon, command map[string]int) error {
	var older, newer []string // the daemon's parts, against the command's
	for _, name := range slices.Sorted(maps.Keys(command)) {
		switch d := daemon[name]; {
		case d < command[name]:
			older = append(older, fmt.Sprintf("%s at %d, not %d", name, d, command[name]))
		case d > command[name]:
			newer = append(newer, fmt.Sprintf("%s at %d, not %d", name, d, command[name]))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(daemon)) {
		if _, ok := command[name]; !ok {
			newer = append(newer, fmt.Sprintf("%s, which this ws doesn't know", name))
		}
	}
	switch {
	case len(older) > 0:
		return fmt.Errorf("the workspace's daemon is from an older ws than this command (its records: %s): it restarts onto this ws once nothing's at work, or at once when whoever runs the workspace says so (ws daemon restart --now)", strings.Join(older, "; "))
	case len(newer) > 0:
		return fmt.Errorf("this ws is older than the workspace's daemon (its records: %s): rebuild or update ws", strings.Join(newer, "; "))
	}
	return nil
}
