package lessons

import (
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/store/storetest"
)

func day(d int) time.Time { return time.Date(2026, 11, d, 10, 0, 0, 0, time.UTC) }

func read(mission, crew string, d int) Read {
	return Read{Mission: mission, Crew: crew, Repo: "api", HandedOff: day(d), At: day(d).Add(time.Minute)}
}

// A lesson one crew ran into is a note; a second crew makes it a
// recommendation, once. Reading the same crew's handoff again counts
// nothing twice.
func TestKeep_ANoteThenARecommendation(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	first := Found{Said: "`just test` fails for crew: TestDaemonRestart refuses WS_CREW.", Title: "Daemon test fails for crew",
		Fix: FixRepo, Change: "Unset WS_CREW in TestDaemonRestart.", Why: "Crew inherit WS_CREW.", Repo: "api"}
	ch, err := Keep(root, read("cache", "c1", 17), []Found{first})
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 1 || !ch[0].New || ch[0].Recommended || ch[0].Lesson.Crew != 1 || ch[0].Lesson.IsRecommended() {
		t.Fatalf("first = %+v", ch)
	}
	id := ch[0].Lesson.ID

	again, err := Keep(root, read("cache", "c1", 17), []Found{{Same: id, Said: "the same, said again"}})
	if err != nil || len(again) != 0 {
		t.Fatalf("read again = %+v, %v", again, err)
	}

	second := Found{Same: id, Said: "`go test ./...` fails in crew: run it as `env -u WS_CREW`.", Change: "Have the test clear WS_CREW and WS_MISSION."}
	ch, err = Keep(root, read("plugins", "c6", 18), []Found{second})
	if err != nil {
		t.Fatal(err)
	}
	l := ch[0].Lesson
	if !ch[0].Recommended || ch[0].New || l.Crew != 2 || l.Missions != 2 || !l.LastSeen.Equal(day(18)) {
		t.Fatalf("second = %+v", ch[0])
	}
	if l.Change != "Have the test clear WS_CREW and WS_MISSION." || l.Seen[0].Mission != "plugins" {
		t.Errorf("lesson = %+v", l)
	}

	third, _ := Keep(root, read("plugins", "c7", 19), []Found{{Same: id, Said: "again"}})
	if third[0].Recommended {
		t.Errorf("a recommendation already, said again: %+v", third[0])
	}
	if got, _ := ReadTo(root); !got[Key("cache", "c1")].Equal(day(17)) || !got[Key("plugins", "c7")].Equal(day(19)) {
		t.Errorf("read to = %v", got)
	}
}

// A lesson marked fixed that crew run into again is open again, saying
// so; a dismissed one stays dismissed, counting who ran into it.
func TestKeep_FixedAndDismissed(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ch, _ := Keep(root, read("a", "c1", 17), []Found{
		{Said: "deps missing", Title: "Web deps missing in a fresh capsule", Fix: FixSetup, Change: "after_create = \"pnpm install\""},
		{Said: "audit warnings", Title: "npm audit warns in dev deps", Fix: "nonsense"},
	})
	fixed, dismissed := ch[0].Lesson.ID, ch[1].Lesson.ID
	if ch[1].Lesson.Fix != FixDocs {
		t.Errorf("a fix that isn't one: %q", ch[1].Lesson.Fix)
	}
	if err := SetState(root, fixed, Fixed, day(18)); err != nil {
		t.Fatal(err)
	}
	if err := SetState(root, dismissed, Dismissed, day(18)); err != nil {
		t.Fatal(err)
	}
	if err := SetState(root, 99, Fixed, day(18)); err != ErrNoLesson {
		t.Errorf("no lesson: %v", err)
	}
	ch, _ = Keep(root, read("b", "c2", 20), []Found{{Same: fixed, Said: "deps still missing"}, {Same: dismissed, Said: "audit again"}})
	got := map[int64]Change{}
	for _, c := range ch {
		got[c.Lesson.ID] = c
	}
	if c := got[fixed]; !c.Again || c.Lesson.State != Open || c.Lesson.Again == nil || !c.Lesson.Again.Equal(day(20)) {
		t.Errorf("fixed, seen again = %+v", c)
	}
	if c := got[dismissed]; c.Again || c.Lesson.State != Dismissed || c.Lesson.Crew != 2 {
		t.Errorf("dismissed, seen again = %+v", c)
	}
	if err := SetMission(root, fixed, "web-deps-in-after-create"); err != nil {
		t.Fatal(err)
	}
	if l, _ := Get(root, fixed); l.Mission != "web-deps-in-after-create" {
		t.Errorf("mission = %q", l.Mission)
	}
	if n, failed, _ := Reads(root); n != 2 || failed != 0 {
		t.Errorf("reads = %d, %d failed", n, failed)
	}
}

// A read that failed is kept, so it isn't tried again and again.
func TestKeep_AFailedRead(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	r := read("a", "c1", 17)
	r.Err = "the agent's answer had no lessons in it"
	if _, err := Keep(root, r, nil); err != nil {
		t.Fatal(err)
	}
	if n, failed, _ := Reads(root); n != 1 || failed != 1 {
		t.Errorf("reads = %d, %d failed", n, failed)
	}
	before := Stamp(root)
	Keep(root, read("a", "c2", 18), nil)
	if Stamp(root) == before {
		t.Error("the stamp didn't move")
	}
}

// Put keeps a seed's lessons as given.
func TestPut(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	err := Put(root, []Lesson{{Title: "1Password signing fails commits", Fix: FixYou, Change: "Keep 1Password unlocked.",
		Seen: []Sighting{{Mission: "a", Crew: "c1", Said: "failed to fill whole buffer", At: day(17)}, {Mission: "b", Crew: "c3", Said: "again", At: day(20)}}}})
	if err != nil {
		t.Fatal(err)
	}
	list, _ := All(root)
	if len(list) != 1 || !list[0].IsRecommended() || !list[0].Created.Equal(day(17)) || list[0].State != Open || !list[0].Quiet(day(20).Add(QuietAfter+time.Hour)) {
		t.Errorf("lessons = %+v", list)
	}
}
