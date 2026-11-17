package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The daemon copies the records the first time it takes them each day,
// keeping three days' copies; a file with nothing in it isn't copied. A
// copy is a whole, sound database.
func TestBackup_OnceADay_TheLastThreeKept(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".ws"), 0o755)
	raw, err := sql.Open("sqlite", Path(root))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	day := time.Date(2026, 11, 12, 9, 0, 0, 0, time.Local)
	if err := backup(bg, root, raw, day); err != nil || len(Copies(root)) != 0 {
		t.Fatalf("an empty file was copied: %v %v", Copies(root), err)
	}
	raw.Exec(`CREATE TABLE t (s TEXT); INSERT INTO t VALUES ('kept')`)
	for i := range 5 {
		if err := backup(bg, root, raw, day.AddDate(0, 0, i)); err != nil {
			t.Fatal(err)
		}
		raw.Exec(`INSERT INTO t VALUES ('later')`)
		if err := backup(bg, root, raw, day.AddDate(0, 0, i)); err != nil { // the same day again
			t.Fatal(err)
		}
	}
	got := Copies(root)
	if strings.Join(got, " ") != "ws-2026-11-14.db ws-2026-11-15.db ws-2026-11-16.db" {
		t.Fatalf("copies = %v", got)
	}
	c, err := sql.Open("sqlite", filepath.Join(BackupDir(root), got[0]))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var n int
	if err := c.QueryRow(`SELECT count(*) FROM t`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("the copy of the 14th has %d rows (%v), not what was there when it was taken", n, err)
	}
}

// Own takes the day's copy before bringing the layout forward; ws doctor's
// check asks the daemon whether the file's sound, from a command too.
func TestOwn_CopiesTheRecords_AndChecksThem(t *testing.T) {
	db := own(t)
	root := db.Root()
	if len(Copies(root)) != 0 {
		t.Fatalf("a new file was copied: %v", Copies(root))
	}
	db.Close()
	db, err := Own(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := Copies(root); len(got) != 1 || got[0] != "ws-"+time.Now().Format("2006-01-02")+".db" {
		t.Fatalf("copies = %v", got)
	}
	c, err := Borrow(root)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if bad, err := c.Check(bg); err != nil || len(bad) != 0 {
		t.Fatalf("check: %v %v", bad, err)
	}
}

// A reader that's searched the words holds what it knew of their index,
// and once the writer's changed it, SQLite's check on that reader finds
// the file damaged when it isn't. The check runs on a connection of its
// own, asked by the daemon or a command.
func TestCheck_AfterWordsAreWritten(t *testing.T) {
	db := own(t)
	say := func(o Origin, text string) {
		t.Helper()
		if err := db.Write(bg, func(tx *Tx) error {
			return tx.Words(o, Place{Thing: Key{Name: "x"}, Kind: "turn", Text: text})
		}); err != nil {
			t.Fatal(err)
		}
	}
	say("a", "caching tokens")
	if _, _, err := db.Find(bg, Said{Words: "tokens"}); err != nil {
		t.Fatal(err)
	}
	say("b", "more tokens")
	if bad, err := db.Check(bg); err != nil || len(bad) != 0 {
		t.Errorf("the daemon's check: %v %v", bad, err)
	}
	c, err := Borrow(db.Root())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if bad, err := c.Check(bg); err != nil || len(bad) != 0 {
		t.Errorf("a command's check: %v %v", bad, err)
	}
}
