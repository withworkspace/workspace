package testutil

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

var shared struct {
	sync.Mutex
	dir  string            // where they're kept: made with the first, removed by Main
	n    int               // how many have been started
	made map[string]string // what's made, by name -> its folder
}

// Shared is a folder that the tests of this binary share, for what's slow
// to make and the same for each of them: the first to ask for name has it
// made, by make, in a new folder, and the rest are given that one. What's
// in it is read or copied, never changed. Main removes it when the tests
// are over.
func Shared(t *testing.T, name string, make func(dir string)) string {
	t.Helper()
	if !inMain {
		t.Fatal("testutil.Shared: this package's TestMain isn't testutil.Main, which removes what its tests share")
	}
	shared.Lock()
	defer shared.Unlock()
	if dir, ok := shared.made[name]; ok {
		return dir
	}
	if shared.dir == "" {
		dir, err := os.MkdirTemp("", "ws-test-shared-")
		if err != nil {
			t.Fatal(err)
		}
		// As git writes a path, for what looks for one in what git wrote.
		if dir, err = filepath.EvalSymlinks(dir); err != nil {
			t.Fatal(err)
		}
		shared.dir, shared.made = dir, map[string]string{}
	}
	shared.n++
	dir := filepath.Join(shared.dir, strconv.Itoa(shared.n))
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	make(dir) // a make that fails the test leaves nothing to be given to the next
	shared.made[name] = dir
	return dir
}

// removeShared removes what the tests shared, when they're over.
func removeShared() {
	shared.Lock()
	defer shared.Unlock()
	if shared.dir != "" {
		os.RemoveAll(shared.dir)
	}
	shared.dir, shared.made = "", nil
}
