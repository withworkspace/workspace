package ops

import (
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Each mission's row in the workspace's view is kept, and built again
// only when something it's built from has moved: a write that names the
// mission (its record, its log, its conversations), or the time.
func TestOverview_RowsKeptTillTheirMissionMoves(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	w := New(&config.Config{}, &workspace.Workspace{Root: root}, nil, Options{})
	cache, _ := w.Missions.Create("Cache")
	ledger, _ := w.Missions.Create("Ledger")
	built := func() int64 {
		t.Helper()
		v, err := w.Overview()
		if err != nil {
			t.Fatal(err)
		}
		if len(v.Missions) != 2 {
			t.Fatalf("%d missions, want 2", len(v.Missions))
		}
		w.overview.mu.Lock()
		defer w.overview.mu.Unlock()
		return w.overview.built
	}
	if n := built(); n != 2 {
		t.Fatalf("first view: %d rows built, want 2", n)
	}
	if n := built(); n != 2 {
		t.Fatalf("nothing moved: %d rows built in all, want still 2", n)
	}

	w.Missions.AppendLog(cache.Slug, mission.Event{Kind: mission.EventState, Text: "a note"})
	if n := built(); n != 3 {
		t.Fatalf("the cache mission's log written: %d rows built in all, want 3 (only its own again)", n)
	}
	if _, err := conversation.For(root, ledger.Slug).OpenTalk("you", "Which ledger?", time.Now()); err != nil {
		t.Fatal(err)
	}
	if n := built(); n != 4 {
		t.Fatalf("a conversation on the ledger mission: %d rows built in all, want 4", n)
	}

	was := keptFor
	keptFor = 0
	t.Cleanup(func() { keptFor = was })
	if n := built(); n != 6 {
		t.Fatalf("their time up: %d rows built in all, want 6", n)
	}
}
