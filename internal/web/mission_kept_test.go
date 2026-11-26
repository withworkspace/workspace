package web

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// A mission's page that's open is built before it's told, and told only
// when what it's served has changed, with the version it's now at; its
// reads meanwhile are served as kept, built by none of them.
func TestMission_ToldOnlyWhenItsPageChanges(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	ms := mission.Store{Root: root}
	one, _ := ms.Create("One")
	s := startTest(t, root, &fakeWorkspace{}, nil)
	topic := missionTopic + one.Slug
	st := followOver(t, s, topic)

	ms.AppendLog(one.Slug, mission.Event{Kind: mission.EventNote, Text: "hi"})
	if got := st.nextOf(topic, 3*time.Second); got != "changed "+topic {
		t.Fatalf("after a note in its log: %q", got)
	}
	told := st.last.Version
	if told == 0 {
		t.Error("told with no version")
	}

	built := s.views.built[topic]
	for range 3 {
		res := do(t, s, "GET", "/api/missions/"+one.Slug, true, nil)
		if v, _ := strconv.ParseInt(res.Header.Get(versionHeader), 10, 64); v != told {
			t.Errorf("read at version %d, told %d", v, told)
		}
	}
	if n := s.views.built[topic] - built; n != 0 {
		t.Errorf("its reads built it %d times, want none", n)
	}

	// A write that names it and changes nothing it shows: built, and not
	// told.
	db.Write(context.Background(), func(tx *store.Tx) error {
		tx.Changed(topic)
		return nil
	})
	if got := st.nextOf(topic, 700*time.Millisecond); got != "(nothing)" {
		t.Errorf("after a write that changed nothing it shows: %q", got)
	}

	ms.AppendLog(one.Slug, mission.Event{Kind: mission.EventNote, Text: "again"})
	if got := st.nextOf(topic, 3*time.Second); got != "changed "+topic {
		t.Fatalf("after another note: %q", got)
	}
	if st.last.Version <= told {
		t.Errorf("told version %d after %d", st.last.Version, told)
	}
}
