// Package storetest is the workspace's records for a test: a test binary
// has no daemon to start (store.Start is nil), so a test that reads or
// writes records owns them itself, as the daemon would.
package storetest

import (
	"errors"
	"testing"

	"github.com/withworkspace/workspace/internal/store"
)

// Own takes root's records for the test, let go of when it ends: or, if
// the test owns them already, is them.
func Own(t testing.TB, root string) *store.DB {
	t.Helper()
	db, err := store.Own(root)
	if errors.Is(err, store.ErrOwned) {
		if db, err = store.Open(root); err != nil {
			t.Fatal(err)
		}
		return db
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
