package events

import (
	"context"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/store/storetest"
)

func TestAppendAndList(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	now := time.Now()
	for _, e := range []Event{
		{Kind: CapsuleLanded, By: ByYou, Repo: "demo", Capsule: "fix", Mission: "m1", At: now.Add(-2 * time.Hour)},
		{Kind: PROpened, By: ByCrew("c1"), Repo: "demo", Capsule: "fix", PR: "demo#3", Mission: "m1", At: now.Add(-time.Hour)},
		{Kind: ArtifactPromoted, Artifact: "a2", At: now},
	} {
		if _, err := Emit(root, e); err != nil {
			t.Fatal(err)
		}
	}

	all, err := List(root, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Kind != CapsuleLanded || all[2].Kind != ArtifactPromoted {
		t.Fatalf("all, oldest first: %+v", all)
	}
	if all[0].N != 1 || all[2].Ref() != "e3" {
		t.Errorf("numbered: %d, %s", all[0].N, all[2].Ref())
	}
	if all[2].By != ByYou {
		t.Errorf("By defaults to you: %q", all[2].By)
	}

	got, _ := List(root, Filter{Kinds: []string{PROpened, ArtifactPromoted}})
	if len(got) != 2 || got[0].Kind != PROpened {
		t.Errorf("by kind: %+v", got)
	}
	got, _ = List(root, Filter{Since: now.Add(-90 * time.Minute)})
	if len(got) != 2 {
		t.Errorf("since: %+v", got)
	}
	got, _ = List(root, Filter{After: 2})
	if len(got) != 1 || got[0].N != 3 {
		t.Errorf("after: %+v", got)
	}
	got, _ = List(root, Filter{Limit: 2})
	if len(got) != 2 || got[0].N != 2 || got[1].N != 3 {
		t.Errorf("limit keeps the latest, oldest first: %+v", got)
	}
	got, _ = List(root, Filter{Mission: "m1"})
	if len(got) != 2 {
		t.Errorf("a mission's: %+v", got)
	}
	if refs := all[1].Refs(); len(refs) != 3 || refs[0] != "demo/fix" || refs[1] != "demo#3" || refs[2] != "m1" {
		t.Errorf("refs: %v", refs)
	}
}

func TestOldEventsGo(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	if _, err := Emit(root, Event{Kind: DocAdded, At: time.Now().Add(-Keep - time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := Emit(root, Event{Kind: DocAmended}); err != nil {
		t.Fatal(err)
	}
	got, _ := List(root, Filter{})
	if len(got) != 1 || got[0].Kind != DocAmended {
		t.Fatalf("what's older than Keep goes: %+v", got)
	}
}

func TestUnknownKindRefused(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	if _, err := Emit(root, Event{Kind: "capsule.exploded"}); err == nil {
		t.Fatal("an unknown kind is refused")
	}
}

func TestAppendTellsPages(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	seq, _ := db.Seq(context.Background())
	if _, err := Emit(root, Event{Kind: MissionStarted, Mission: "m1"}); err != nil {
		t.Fatal(err)
	}
	topics, _, err := db.Since(context.Background(), seq)
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 1 || topics[0] != Topic {
		t.Fatalf("topics: %v", topics)
	}
}
