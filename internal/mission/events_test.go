package mission

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/events"
)

// What a mission's log shares with the workspace's events is kept there
// too, with who did it; the rest of its log is its own.
func TestLogSharesEvents(t *testing.T) {
	t.Setenv(EnvCrew, "")
	t.Setenv(EnvRun, "")
	t.Setenv(EnvRole, "")
	s := owned(t)
	m, _ := s.Create("Cache")
	s.AppendLog(m.Slug, Event{Kind: EventNote, Text: "just a note"})
	s.Decide(m.Slug, Decision{What: "Key the cache per tenant.", By: "c1"})
	s.Withdraw(m.Slug, "d1", "you", "Changed our minds.")
	s.AppendLog(m.Slug, Event{Kind: EventPR, What: PREventOpened, Repo: "demo", Capsule: "fix", Crew: "c1", Run: "o3", Text: "demo#12 opened: Fix it"})
	s.AppendLog(m.Slug, Event{Kind: EventPR, What: PREventApproved, Repo: "demo", Capsule: "fix", By: "ws", Text: "demo#12 was approved"})
	s.AppendLog(m.Slug, Event{Kind: EventPR, What: PREventChecksFailed, Repo: "demo", Capsule: "fix", By: "ws", Text: "demo#12's checks are failing"})
	s.AppendLog(m.Slug, Event{Kind: EventPR, What: PREventMerged, Repo: "demo", Capsule: "fix", By: "ws", Text: "demo#12 merged"})

	got, err := events.List(s.Root, events.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var said []string
	for _, e := range got {
		if e.Mission != m.Slug {
			t.Errorf("%s isn't the mission's: %+v", e.Kind, e)
		}
		said = append(said, e.Kind+" "+e.By+" "+e.PR+" "+e.Cause)
	}
	want := []string{
		"mission.started you  ",
		"decision.made crew:c1  ",
		"pr.opened crew:c1 demo#12 o3",
		"pr.checks_failed ws demo#12 ",
		"pr.merged ws demo#12 ",
	}
	if strings.Join(said, "\n") != strings.Join(want, "\n") {
		t.Errorf("kept:\n%s\nwant:\n%s", strings.Join(said, "\n"), strings.Join(want, "\n"))
	}
}

// Crew's own process, or the orchestrator's, is who did what's logged
// from it, unless the line says.
func TestLogEventsByWhoseProcess(t *testing.T) {
	t.Setenv(EnvRun, "")
	s := owned(t)
	t.Setenv(EnvRole, RoleOrchestrator)
	t.Setenv(EnvCrew, "")
	m, _ := s.Create("Cache")
	got, _ := events.List(s.Root, events.Filter{})
	if len(got) != 1 || got[0].By != events.ByOrchestrator {
		t.Errorf("started by the orchestrator: %+v", got)
	}
	t.Setenv(EnvRole, "")
	t.Setenv(EnvCrew, "c4")
	s.AppendLog(m.Slug, Event{Kind: EventCapsuleBurned, Repo: "demo", Capsule: "fix"})
	got, _ = events.List(s.Root, events.Filter{Kinds: []string{events.CapsuleBurned}})
	if len(got) != 1 || got[0].By != "crew:c4" || got[0].Capsule != "fix" {
		t.Errorf("burned by crew: %+v", got)
	}
}

func TestPROf(t *testing.T) {
	for text, want := range map[string]string{
		"demo#12 opened, at step 2: Fix it": "demo#12",
		"demo#3's checks are failing":       "demo#3",
		"demo#7 merged":                     "demo#7",
		"nothing here #":                    "",
	} {
		if got := prOf(text); got != want {
			t.Errorf("prOf(%q) = %q, want %q", text, got, want)
		}
	}
}
