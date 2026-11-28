package prompt

import (
	"strings"
	"testing"
)

func TestLessonsPrompt(t *testing.T) {
	golden(t, "lessons", LessonsPrompt(LessonsAsk{
		Mission: "plugin-pages", Title: "Plugin pages as single-page apps", Crew: "c6", Repo: "workspace",
		Said:        "- `just test` fails for crew: TestDaemonRestart refuses WS_CREW. `env -u WS_CREW -u WS_MISSION just test` passes.\n- Commit signing through 1Password failed once (\"failed to fill whole buffer\"); a retry worked.",
		AfterCreate: "go mod download",
		Known:       []KnownLesson{{ID: 3, Repo: "", Title: "1Password signing fails commits", Change: "Keep 1Password unlocked\nwhile crew run.", Crew: 4}},
	}))
	// A handoff kept from before Environment was asked for is read whole.
	whole := LessonsPrompt(LessonsAsk{Mission: "m", Title: "M", Crew: "c1", Said: "## Done\n- x", Whole: true})
	for _, want := range []string{"Most of a handoff is about the work", "the repo (not known)", "Known lessons: none yet."} {
		if !strings.Contains(whole, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(whole, "capsule setup now") {
		t.Error("a repo not known has a capsule setup")
	}
}

func TestReadLessons(t *testing.T) {
	got, err := ReadLessons("Here they are.\n\n```json\n{\"lessons\": [" +
		"{\"same\": 3, \"said\": \" Signing failed once. \", \"change\": \"\"}," +
		"{\"same\": 0, \"said\": \"`just test` fails for crew.\", \"title\": \"Daemon test fails for crew\", \"repo\": \"workspace\", \"fix\": \"Repo\", \"change\": \"Unset WS_CREW in the test.\", \"why\": \"Crew inherit it.\"}," +
		"{\"same\": 0, \"said\": \"a new one with no title\"}," +
		"{\"same\": 0, \"said\": \"\", \"title\": \"nothing said\"}" +
		"]}\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Same != 3 || got[0].Said != "Signing failed once." || got[1].Fix != "repo" || got[1].Title != "Daemon test fails for crew" {
		t.Errorf("lessons = %+v", got)
	}
	if got, err := ReadLessons(`{"lessons": []}`); err != nil || len(got) != 0 {
		t.Errorf("none = %+v, %v", got, err)
	}
	for _, bad := range []string{"I found nothing.", "```json\n{\"lessons\": [oops]}\n```"} {
		if _, err := ReadLessons(bad); err == nil {
			t.Errorf("%q read", bad)
		}
	}
}
