package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/store"
)

// What's around a mission is one link out from it either way, nearest
// first: the talk it came from, then what was written in it (its
// handoff), and never the mission itself. From the talk, the mission it
// started is around it.
func TestAround(t *testing.T) {
	w := records(t)
	ctx := context.Background()
	near, err := w.Around(ctx, []store.Key{{Name: "house-sorting-cli"}}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Near{}
	for _, n := range near {
		got[n.Scope+"/"+n.Ref] = n
		if n.Ref == "house-sorting-cli" {
			t.Errorf("the mission is around itself: %+v", n)
		}
	}
	if len(near) == 0 || near[0].Ref != "w4" || near[0].How != store.From || !near[0].Out || near[0].Thing.Kind != "talk" {
		t.Fatalf("first should be the talk it came from: %+v", near)
	}
	// The brief is the mission's own words: not around it, but what it
	// names is (TestAround_Fresh).
	if b, ok := got["house-sorting-cli/brief"]; ok {
		t.Errorf("its brief is around it: %+v", b)
	}
	if h := got["house-sorting-cli/mission:handoffs/c1.md"]; h.Place != "handoff" {
		t.Errorf("its handoff: %+v", h)
	}

	near, err = w.Around(ctx, []store.Key{{Name: "w4"}}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(near) == 0 || near[0].Scope != "house-sorting-cli" || near[0].Thing.Kind != "mission" || near[0].Out {
		t.Errorf("around the talk, the mission it started: %+v", near)
	}

	if near, _ := w.Around(ctx, []store.Key{{Name: "house-sorting-cli"}}, 1, false); len(near) != 1 {
		t.Errorf("a limit of one gave %d", len(near))
	}
}

// A note written into a mission's files, outside ws, is around the
// mission once it's asked fresh: as the records stood, it wasn't. What
// the mission's brief names is around the mission, as the mission's.
func TestAround_Fresh(t *testing.T) {
	w := records(t)
	ctx := context.Background()
	mission := []store.Key{{Name: "house-sorting-cli"}}
	os.WriteFile(filepath.Join(w.Root(), ".ws", "missions", "house-sorting-cli", "files", "notes.md"), []byte("# Notes\n\nQuidditch can wait.\n"), 0o644)
	has := func(fresh bool) bool {
		t.Helper()
		near, err := w.Around(ctx, mission, 0, fresh)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range near {
			if n.Ref == "mission:notes.md" {
				return true
			}
		}
		return false
	}
	if has(false) {
		t.Error("the note was around before it was read")
	}
	if !has(true) {
		t.Error("the note wasn't around once read")
	}

	dir := filepath.Join(w.Root(), ".ws", "missions", "house-sorting-cli")
	brief, _ := os.ReadFile(filepath.Join(dir, "brief.md"))
	os.WriteFile(filepath.Join(dir, "brief.md"), append(brief, "\n## Later\nAs w2 asked.\n"...), 0o644)
	near, err := w.Around(ctx, mission, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	var named *Near
	for i, n := range near {
		if n.Ref == "w2" {
			named = &near[i]
		}
	}
	if named == nil || named.How != store.Mentions || !named.Out {
		t.Errorf("what its brief names: %+v in %+v", named, near)
	}
}
