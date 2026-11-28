package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/lessons"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/notification"
)

// handedOff starts a crew in frontend/<capsule> on the cache mission and
// has it hand off text.
func handedOff(t *testing.T, w *Workspace, capsule, text string) mission.Crew {
	t.Helper()
	c, err := w.Missions.StartCrew("cache", "claude", "conv-"+capsule)
	if err != nil {
		t.Fatal(err)
	}
	w.Missions.SetCrewProcess("cache", c.ID, 1, "frontend/"+capsule)
	h, err := mission.ParseHandoff(text)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Missions.ReportDone("cache", c.ID, "done", h); err != nil {
		t.Fatal(err)
	}
	return c
}

const handoffBody = "## Done\n- The cache.\n## Learned\n- Lookups happen twice.\n"
const handoffEnd = "## Left\n- Nothing.\n## Read first\n- Nothing.\n"

// Each handoff is read once, oldest first: its Environment section, or the
// whole of one from before that was asked for, told the repo's capsule
// setup and what's known; one whose Environment says nothing needs no
// agent. A second crew running into a lesson makes it a recommendation,
// told of once.
func TestReadLessons(t *testing.T) {
	w, _ := fixture(t, "")
	w.cur.cfg.Lessons.Harness = fake.Name()
	w.cur.cfg.Repos = map[string]config.RepoConfig{"frontend": {AfterCreate: "go mod download"}}
	handedOff(t, w, "cache-a", handoffBody+"## Environment\n- A fresh capsule has no web/node_modules: `pnpm install` in web/ first.\n"+handoffEnd)
	handedOff(t, w, "cache-b", handoffBody+"## Environment\nNothing.\n"+handoffEnd)
	handedOff(t, w, "cache-c", handoffBody+"- `tsc` failed until `pnpm install`, which a new capsule needs.\n"+handoffEnd) // from before

	fake.reply, fake.prompts = "```json\n"+`{"lessons": [{"same": 0, "said": "A fresh capsule has no web/node_modules.", "title": "Web dependencies missing in a fresh capsule", "repo": "frontend", "fix": "setup", "change": "after_create = \"go mod download && (cd web && pnpm install --frozen-lockfile)\"", "why": "after_create installs only Go's."}]}`+"\n```", nil
	got, err := w.ReadLessons(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Read != 1 || got.Left != 2 || len(got.Changed) != 1 || !got.Changed[0].New || got.Changed[0].Recommended {
		t.Fatalf("first read = %+v", got)
	}
	id := got.Changed[0].Lesson.ID
	if len(fake.prompts) != 1 || !strings.Contains(fake.prompts[0], "under \"Environment\"") || !strings.Contains(fake.prompts[0], "web/node_modules: `pnpm install`") ||
		!strings.Contains(fake.prompts[0], `after_create = "go mod download"`) || strings.Contains(fake.prompts[0], "Lookups happen twice") {
		t.Errorf("first prompt:\n%s", fake.prompts)
	}

	fake.reply = `{"lessons": [{"same": 1, "said": "tsc failed until pnpm install.", "change": ""}]}`
	got, err = w.ReadLessons(0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Read != 2 || got.Left != 0 || len(fake.prompts) != 2 {
		t.Fatalf("second read = %+v, %d prompts", got, len(fake.prompts))
	}
	if p := fake.prompts[1]; !strings.Contains(p, "Most of a handoff is about the work") || !strings.Contains(p, "Lookups happen twice") || !strings.Contains(p, "- 1 [frontend, 1 crew] Web dependencies missing") {
		t.Errorf("second prompt (a whole handoff):\n%s", p)
	}
	if len(got.Changed) != 1 || !got.Changed[0].Recommended || got.Changed[0].Lesson.Crew != 2 {
		t.Errorf("second read changed = %+v", got.Changed)
	}
	told, _ := notification.List(w.Root(), notification.Filter{})
	if len(told) != 1 || told[0].Source != notification.SourceLessons || told[0].Refs[0] != "lesson-1" || told[0].Text != "2 crew ran into it: Web dependencies missing in a fresh capsule" {
		t.Errorf("told = %+v", told)
	}

	if again, _ := w.ReadLessons(0); again.Read != 0 {
		t.Errorf("read again = %+v", again)
	}
	info, err := w.Lessons()
	if err != nil {
		t.Fatal(err)
	}
	if !info.On || info.Read != 3 || info.Waiting != 0 || len(info.Lessons) != 1 || info.Lessons[0].ID != id {
		t.Errorf("info = %+v", info)
	}
}

// What the agent says that doesn't fit is put right or left out: a known
// lesson it wasn't told of, a repo the workspace hasn't. An answer that
// isn't JSON is kept as a read that failed, not read again.
func TestReadLessons_WhatDoesntFit(t *testing.T) {
	w, _ := fixture(t, "")
	w.cur.cfg.Lessons.Harness = fake.Name()
	handedOff(t, w, "cache-a", handoffBody+"## Environment\n- Something.\n"+handoffEnd)
	handedOff(t, w, "cache-b", handoffBody+"## Environment\n- Something else.\n"+handoffEnd)
	fake.reply = `{"lessons": [{"same": 7, "said": "not told of it"}, {"same": 9, "said": "nor this", "title": "A new one, then", "repo": "nowhere", "fix": "repo", "change": "x"}]}`
	got, _ := w.ReadLessons(1)
	if len(got.Changed) != 1 || got.Changed[0].Lesson.Title != "A new one, then" || got.Changed[0].Lesson.Repo != "frontend" {
		t.Errorf("changed = %+v", got.Changed)
	}
	fake.reply = "I couldn't find anything worth saying."
	got, _ = w.ReadLessons(0)
	if got.Read != 1 || len(got.Failed) != 1 || !strings.Contains(got.Failed[0], "no JSON") {
		t.Errorf("failed read = %+v", got)
	}
	if info, _ := w.Lessons(); info.Failed != 1 || info.Waiting != 0 {
		t.Errorf("info = %+v", info)
	}
}

// Off, no handoff is read.
func TestReadLessons_Off(t *testing.T) {
	w, _ := fixture(t, "")
	off := false
	w.cur.cfg.Lessons = config.LessonsConfig{Harness: fake.Name(), Enabled: &off}
	handedOff(t, w, "cache-a", handoffBody+"## Environment\n- Something.\n"+handoffEnd)
	fake.prompts = nil
	if got, err := w.ReadLessons(0); err != nil || got.Read != 0 || len(fake.prompts) != 0 {
		t.Errorf("off = %+v, %v, %d prompts", got, err, len(fake.prompts))
	}
	if info, _ := w.Lessons(); info.On || info.Waiting != 1 {
		t.Errorf("info = %+v", info)
	}
}

// A lesson is yours to mark, and to make a mission of: once, which says
// what to change and who ran into it.
func TestLessonActions(t *testing.T) {
	w, _ := fixture(t, "")
	lessons.Put(w.Root(), []lessons.Lesson{{Title: "Daemon test fails for crew", Fix: lessons.FixRepo, Repo: "frontend",
		Change: "Clear WS_CREW in TestDaemonRestart.", Why: "Crew inherit it.",
		Seen: []lessons.Sighting{{Mission: "cache", Crew: "c1", Said: "`just test` refuses as crew."}}}})
	if err := w.SetLessonState(1, lessons.Fixed, Actor{Kind: Crew}); err == nil {
		t.Error("crew marked a lesson fixed")
	}
	var inv *Invalid
	if err := w.SetLessonState(1, "done", AsYou); !errors.As(err, &inv) {
		t.Errorf("a state that isn't one: %v", err)
	}
	if err := w.SetLessonState(2, lessons.Fixed, AsYou); !errors.Is(err, ErrNotFound) {
		t.Errorf("no lesson: %v", err)
	}
	slug, err := w.LessonMission(1, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	m, err := w.Missions.Load(slug)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Daemon test fails for crew (in frontend)", "Clear WS_CREW in TestDaemonRestart.", "Why crew run into it: Crew inherit it.", "One crew ran into it", "- cache, crew c1, "} {
		if !strings.Contains(m.Request, want) {
			t.Errorf("request missing %q:\n%s", want, m.Request)
		}
	}
	if again, _ := w.LessonMission(1, AsYou); again != slug {
		t.Errorf("a second mission: %s", again)
	}
}
