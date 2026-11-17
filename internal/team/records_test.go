package team

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
)

// has reports whether events have one of id.
func has(events []Event, id string) bool {
	for _, e := range events {
		if e.ID == id {
			return true
		}
	}
	return false
}

// What's kept goes back the window: a pull request that hasn't moved
// since goes, with its events. A look asked to go further keeps that for
// the rest of the day, so the looks that keep a page fresh don't cut a
// longer period back; the next day it's the window again.
func TestService_Window(t *testing.T) {
	w := newWorld(t)
	w.prs = []github.PRActivity{
		{PR: github.PR{Number: 5, Title: "fix: old", State: "MERGED", Author: "ben-acme", CreatedAt: "2026-10-19T09:00:00Z", MergedAt: "2026-10-20T09:00:00Z", UpdatedAt: "2026-10-20T09:00:00Z"}},
		{PR: github.PR{Number: 12, Title: "feat: cache tokens", State: "OPEN", Author: "ana-acme", CreatedAt: stamp(8, 9), UpdatedAt: stamp(9, 9)}},
	}
	snap, err := w.svc.Look(false, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if has(snap.Events, "pr:api#5:merged") || !has(snap.Events, "pr:api#12:opened") {
		t.Errorf("the first look kept %q", kinds(snap.Events))
	}
	if _, ok := w.svc.readPRs()["api#5"]; ok {
		t.Error("a pull request that hasn't moved in the window was kept")
	}

	// Further back, it's kept, and stays for the day.
	far := time.Date(2026, 10, 1, 0, 0, 0, 0, utc)
	if snap, err = w.svc.Look(false, far); err != nil || !has(snap.Events, "pr:api#5:merged") {
		t.Fatalf("further back: %v, %v", kinds(snap.Events), err)
	}
	w.now = w.now.Add(3 * time.Hour)
	if snap, err = w.svc.Look(false, time.Time{}); err != nil || !snap.Since.Equal(far) || !has(snap.Events, "pr:api#5:merged") {
		t.Errorf("later that day: since %v, %v", snap.Since, err)
	}

	// The next day, it's the window again, in the records too.
	w.now = at(11, 9)
	if snap, err = w.svc.Look(false, time.Time{}); err != nil || !snap.Since.Equal(w.svc.Start()) || has(snap.Events, "pr:api#5:merged") {
		t.Errorf("the next day: since %v, %q, %v", snap.Since, kinds(snap.Events), err)
	}
	if kept := w.service().Peek(); kept == nil || has(kept.Events, "pr:api#5:merged") || !kept.Since.Equal(w.svc.Start()) {
		t.Errorf("kept: %+v", kept)
	}
}

// The daemon and ws team each hold a service: what one looks at and
// writes, the other reads, and neither loses the other's gists.
func TestService_TwoProcesses(t *testing.T) {
	w := newWorld(t)
	w.prs = []github.PRActivity{{PR: github.PR{Number: 12, Title: "feat: cache tokens", State: "OPEN", Author: "ana-acme", CreatedAt: stamp(9, 9), UpdatedAt: stamp(9, 9)}}}
	daemon, command := w.svc, w.service()
	if _, err := daemon.Look(false, time.Time{}); err != nil {
		t.Fatal(err)
	}
	asked := len(w.asked)

	// The command finds the daemon's look fresh, and doesn't look again.
	if snap, err := command.Look(false, time.Time{}); err != nil || len(w.asked) != asked || !has(snap.Events, "pr:api#12:opened") {
		t.Fatalf("the command looked again: %d asks, %v", len(w.asked), err)
	}
	// Later, the command looks; the daemon has its look without one.
	w.now = w.now.Add(time.Hour)
	w.prs = append(w.prs, github.PRActivity{PR: github.PR{Number: 13, Title: "fix: evict", State: "OPEN", Author: "ben-acme", CreatedAt: stamp(10, 12), UpdatedAt: stamp(10, 12)}})
	if _, err := command.Look(true, time.Time{}); err != nil {
		t.Fatal(err)
	}
	asked = len(w.asked)
	if snap := daemon.Peek(); snap == nil || !has(snap.Events, "pr:api#13:opened") {
		t.Errorf("the daemon didn't read the command's look")
	}
	if _, err := daemon.Look(false, time.Time{}); err != nil || len(w.asked) != asked {
		t.Errorf("the daemon looked again: %d asks", len(w.asked)-asked)
	}

	// Each writes a gist (these can't be read: their branches aren't
	// there, so what's kept is why); neither is lost.
	writer := func(a Ask) (string, error) { return "gist: " + a.Subject.Title + ", done.\nknow: none", nil }
	daemon.Writer, command.Writer = writer, writer
	snap := daemon.Peek()
	daemon.Narrate(snap, []string{"pr:api#12"}, time.Time{})
	command.Narrate(snap, []string{"pr:api#13"}, time.Time{})
	waitNarrated(t, daemon)
	waitNarrated(t, command)
	for _, s := range []*Service{daemon, command, w.service()} {
		g := s.gistsOf([]string{"pr:api#12", "pr:api#13"})
		if !strings.Contains(g["pr:api#12"].Failed, "#12") || !strings.Contains(g["pr:api#13"].Failed, "#13") {
			t.Errorf("gists: %+v", g)
		}
	}
	// Tried again and failing, it counts the tries.
	daemon.Narrate(snap, []string{"pr:api#13"}, time.Time{})
	waitNarrated(t, daemon)
	if g := command.gistsOf([]string{"pr:api#13"}); g["pr:api#13"].Tries != 2 {
		t.Errorf("tries: %+v", g)
	}
}
