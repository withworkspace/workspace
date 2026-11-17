package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// When it goes wrong: the records are one file that holds everything, so
// the daemon copies it when it first takes it each day, before anything
// brings its layout forward; and ws doctor asks SQLite whether it's
// sound.

// keepCopies is how many days' copies are kept.
const keepCopies = 3

// BackupDir is where the copies of a workspace's records are kept.
func BackupDir(root string) string { return filepath.Join(root, ".ws", "backup") }

// backup copies the records to .ws/backup/ws-<date>.db, if there's
// anything in them and no copy's been taken today, and lets go of all but
// the latest three. A copy that can't be taken doesn't stop the daemon:
// ws doctor says how old the latest is.
func backup(ctx context.Context, root string, db *sql.DB, today time.Time) error {
	var tables int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema`).Scan(&tables); err != nil || tables == 0 {
		return err // a file just made: nothing to keep
	}
	dir := BackupDir(root)
	name := filepath.Join(dir, "ws-"+today.Format("2006-01-02")+".db")
	if _, err := os.Stat(name); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	part := name + ".part"
	os.Remove(part) // left by a copy that didn't finish
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, part); err != nil {
		os.Remove(part)
		return fmt.Errorf("copying the records: %w", err)
	}
	if err := os.Rename(part, name); err != nil {
		return err
	}
	copies := Copies(root)
	for len(copies) > keepCopies {
		os.Remove(filepath.Join(dir, copies[0]))
		copies = copies[1:]
	}
	return nil
}

// Copies are the copies of the records kept, oldest first, by name.
func Copies(root string) []string {
	entries, _ := os.ReadDir(BackupDir(root))
	var out []string
	for _, e := range entries {
		if n := e.Name(); strings.HasPrefix(n, "ws-") && strings.HasSuffix(n, ".db") {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}

// Check is SQLite's own check of the file (integrity_check): nil when it's
// sound, else what it found wrong. The daemon runs it, on a connection of
// its own (owner.check).
func (db *DB) Check(ctx context.Context) ([]string, error) {
	if db.own != nil {
		return db.own.check(ctx)
	}
	conn, err := db.read.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var found []string
	err = conn.Raw(func(dc any) error {
		c, ok := dc.(*rconn)
		if !ok {
			return fmt.Errorf("store: a connection to the daemon isn't one")
		}
		rep, err := c.call(request{Op: "check"})
		found = rep.Found
		return err
	})
	return found, err
}
