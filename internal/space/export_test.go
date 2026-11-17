package space

import (
	"testing"

	"github.com/withworkspace/workspace/internal/store"
)

// borrowed is a command's connection to records this test owns: Open in a
// process that isn't the daemon. Here, the test process owns them, so
// it's asked for over the socket by hand.
func borrowed(t *testing.T, root string) *store.DB {
	t.Helper()
	db, err := store.Borrow(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
