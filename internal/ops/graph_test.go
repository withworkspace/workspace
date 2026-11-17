package ops

import (
	"reflect"
	"testing"

	"github.com/withworkspace/workspace/internal/store"
)

// A document's stored ref is the file it is, folders and all:
// records:adr/0001.md is records/adr/0001.md, as that file is named
// anywhere else, and its lines are taken off.
func TestKeysOf(t *testing.T) {
	file := []store.Key{{Name: "records/adr/0001.md"}}
	for _, c := range []struct {
		s         string
		keys      []store.Key
		line, end int
	}{
		{"records:adr/0001.md", file, 0, 0},
		{"records/adr/0001.md", file, 0, 0},
		{"records:adr/0001.md:12-20", file, 12, 20},
		{"records/adr/0001.md:12-20", file, 12, 20},
		{"records:adr/2026/0001.md", []store.Key{{Name: "records/adr/2026/0001.md"}}, 0, 0},
		{"demo:README.md", []store.Key{{Name: "demo/README.md"}}, 0, 0},
		{"records/x:adr/0001.md:3", []store.Key{{Name: "records/x:adr/0001.md"}, {Name: "records/adr/0001.md"}}, 3, 0},
		{"https://example.com/a/b", []store.Key{{Name: "https://example.com/a/b"}}, 0, 0},
		{"t3", []store.Key{{Scope: "cache", Name: "t3"}}, 0, 0},
	} {
		keys, line, end := KeysOf("cache", c.s)
		if !reflect.DeepEqual(keys, c.keys) || line != c.line || end != c.end {
			t.Errorf("KeysOf(%q) = %+v, %d, %d; want %+v, %d, %d", c.s, keys, line, end, c.keys, c.line, c.end)
		}
	}
}
