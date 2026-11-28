package web

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/lessons"
	"github.com/withworkspace/workspace/internal/ops"
)

// The Lessons page lists the lessons, recommendations first; one's marked
// fixed, and made a mission of, once. Its followers are told when one
// moves, without asking again.
func TestLessons_Page(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	at := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	if err := lessons.Put(s.Root, []lessons.Lesson{
		{Title: "Seen once", Fix: lessons.FixDocs, Change: "Say it in AGENTS.md.", Seen: []lessons.Sighting{{Mission: "a", Crew: "c1", Said: "x", At: at}}},
		{Title: "Web deps missing", Fix: lessons.FixSetup, Repo: "demo", Change: "after_create = \"pnpm install\"",
			Seen: []lessons.Sighting{{Mission: "a", Crew: "c2", Said: "no node_modules", At: at}, {Mission: "b", Crew: "c1", Said: "tsc not found", At: at.Add(time.Hour)}}},
	}); err != nil {
		t.Fatal(err)
	}

	conn, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, conn)
	st.ask(SocketAsk{Op: SocketFollow, Topic: "lessons"})
	if got := st.next(3 * time.Second); got != "following lessons" {
		t.Fatalf("following: %q", got)
	}

	var info ops.LessonsInfo
	asks(t, s, "GET", "/api/lessons", nil, &info)
	if len(info.Lessons) != 2 || info.Lessons[0].Title != "Web deps missing" || !info.Lessons[0].IsRecommended() || info.Lessons[0].Missions != 2 {
		t.Fatalf("lessons = %+v", info.Lessons)
	}
	id := strconv.FormatInt(info.Lessons[0].ID, 10)

	if res := asks(t, s, "POST", "/api/lessons/"+id+"/state", LessonState{State: lessons.Fixed}, nil); res.StatusCode != http.StatusOK {
		t.Fatalf("fixed: %d", res.StatusCode)
	}
	if got := st.next(3 * time.Second); got != "changed lessons" {
		t.Fatalf("marked fixed: %q", got)
	}
	if res := asks(t, s, "POST", "/api/lessons/"+id+"/state", LessonState{State: "done"}, nil); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a state that isn't one: %d", res.StatusCode)
	}
	if res := asks(t, s, "POST", "/api/lessons/9/state", LessonState{State: lessons.Fixed}, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("no lesson: %d", res.StatusCode)
	}
	if res := asks(t, s, "POST", "/api/lessons/nope/mission", nil, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("not a lesson: %d", res.StatusCode)
	}

	var made, again LessonMissionMade
	asks(t, s, "POST", "/api/lessons/lesson-"+id+"/mission", nil, &made)
	asks(t, s, "POST", "/api/lessons/"+id+"/mission", nil, &again)
	if made.Mission == "" || again.Mission != made.Mission {
		t.Errorf("missions = %q, then %q", made.Mission, again.Mission)
	}
	asks(t, s, "GET", "/api/lessons", nil, &info)
	if l := info.Lessons[0]; l.State != lessons.Fixed || l.Mission != made.Mission {
		t.Errorf("after = %+v", l)
	}
}
