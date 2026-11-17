package mission

import (
	"testing"

	"github.com/withworkspace/workspace/internal/store"
)

// What's said in a mission of a document by its stored ref is said of the
// file it is, folders and all: records:adr/0001.md is records/adr/0001.md,
// as that file is named anywhere else.
func TestRefKey(t *testing.T) {
	for s, want := range map[string]store.Key{
		"records:adr/0001.md":       {Name: "records/adr/0001.md"},
		"records/adr/0001.md":       {Name: "records/adr/0001.md"},
		"records:adr/2026/0001.md":  {Name: "records/adr/2026/0001.md"},
		"records:adr/0001.md:12-20": {Name: "records/adr/0001.md:12-20"},
		"demo:README.md":            {Name: "demo/README.md"},
		"records/x:adr/0001.md":     {Name: "records/x:adr/0001.md"},
		"https://example.com/a/b":   {Name: "https://example.com/a/b"},
		"t3":                        {Scope: "cache", Name: "t3"},
		"mission:notes.md":          {Scope: "cache", Name: "mission:notes.md"},
	} {
		if got := refKey("cache", s); got != want {
			t.Errorf("refKey(%q) = %+v, want %+v", s, got, want)
		}
	}
}
