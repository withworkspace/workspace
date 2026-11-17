package process

import (
	"context"
	"testing"

	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// climateRisk is a process, as a team might write it.
const climateRisk = `Features are explored and then written up as Build Proposals. These are shared
with the team, and once approved can be built. We always self-review code before
opening a pull request.

Bugs are reproduced first, then a regression test is added, then they're fixed,
and a pull request opened. They reach nonprod on merge, but we might need to
hotfix them to pre and prd: that's up to the engineer. Don't close a bug's
mission on merge.

Larger pieces of work need collaboration, and perhaps notes and ADRs written,
before a **Build Proposal** is made.
`

func TestSays(t *testing.T) {
	p := Plain(climateRisk)
	for quote, want := range map[string]bool{
		"Bugs are reproduced first":                          true,
		"bugs are REPRODUCED first":                          true,
		"before a Build Proposal is made":                    true, // the process has it in bold
		"we might need to hotfix them":                       true, // over a line break
		"Don't close a bug's mission":                        true,
		"Don’t close a bug’s mission":                        true, // curly
		"Features are explored … can be built":               true,
		"can be built … Features are explored":               false, // out of order
		"Bugs are fixed first":                               false,
		"…":                                                  false,
		"We always review code after opening a pull request": false,
	} {
		if got := Says(p, quote); got != want {
			t.Errorf("Says(%q) = %v, want %v", quote, got, want)
		}
	}
}

// Records that kept read-backs drop their table once this ws has them,
// and new ones never have it.
func TestReadBacksGone(t *testing.T) {
	root := t.TempDir()
	db, err := store.Own(root)
	if err != nil {
		t.Fatal(err)
	}
	// As a ws from before left them: the table, and the part at layout 1.
	if err := db.Write(context.Background(), func(tx *store.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE process_readbacks (n INTEGER PRIMARY KEY, readback TEXT NOT NULL)`); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE meta SET value = 1 WHERE key = 'layout/process'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db = storetest.Own(t, root)
	var n int
	if err := db.Read(context.Background(), func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'process_readbacks'`).Scan(&n)
	}); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("the read-backs' table is still there")
	}
}
