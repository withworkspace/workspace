package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// resumableHarness is crew whose sessions can be resumed, as Claude
// Code's can.
type resumableHarness struct{ fakeHarness }

func (resumableHarness) Name() string { return "fake-resumable" }
func (resumableHarness) Resume(dir, conversation string) (harness.Launch, error) {
	return harness.Launch{Args: []string{"/bin/sh"}, Dir: dir, Conversation: conversation}, nil
}

// interruptedCrew records crew on cache, in capsule, ended as a daemon's
// shutdown ends it, having last reported activity.
func interruptedCrew(t *testing.T, w *Workspace, capsule, activity string) string {
	t.Helper()
	c, err := w.Missions.StartCrewOn("cache", "fake-resumable", "", "session-"+capsule+activity, "")
	if err != nil {
		t.Fatal(err)
	}
	w.Missions.SetCrewProcess("cache", c.ID, 1, "frontend/"+capsule)
	w.Missions.Update("cache", func(m *mission.Mission) error {
		for i := range m.Crew {
			if m.Crew[i].ID == c.ID {
				m.Crew[i].Activity = activity
			}
		}
		return nil
	})
	if err := w.Missions.EndCrewInterrupted("cache", c.ID, "stopped: exit 129"); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

// A restart resumes the crew it shut down under that were still at their
// work, telling those that were mid-turn to carry on, and leaves the rest;
// each is looked at once, and none counts as a dispatch.
func TestResumeInterrupted(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "one", "two", "three")
	host := &fakeHost{}
	w.opts.Host = host
	harness.Register(resumableHarness{})

	working := interruptedCrew(t, w, "one", mission.ActivityWorking)
	waiting := interruptedCrew(t, w, "two", mission.ActivityWaiting)
	handedBack := interruptedCrew(t, w, "three", mission.ActivityWaiting)
	w.Missions.SetCrewStatus("cache", handedBack, mission.CrewDone, "built it")
	// Stopped by you, not by the shutdown: not the restart's to resume.
	you, _ := w.Missions.StartCrewOn("cache", "fake-resumable", "", "session-you", "")
	w.Missions.EndCrewStopped("cache", you.ID, "exit 129", true)

	got := w.ResumeInterrupted()
	var said []string
	for _, r := range got {
		said = append(said, r.String())
	}
	want := []string{
		"resumed cache/" + working + ", which the last daemon shut down under",
		"resumed cache/" + waiting + ", which the last daemon shut down under",
		"didn't resume cache/" + handedBack + ", which the last daemon shut down under: it had handed its work back",
	}
	if strings.Join(said, "\n") != strings.Join(want, "\n") {
		t.Errorf("resumed:\n%s\nwant:\n%s", strings.Join(said, "\n"), strings.Join(want, "\n"))
	}
	if len(host.specs) != 2 {
		t.Fatalf("launched %d crew, want 2", len(host.specs))
	}
	if host.specs[0].CrewID != working || host.specs[0].Prompt != CarryOn {
		t.Errorf("mid-turn crew resumed as %s with %q, want %s told to carry on", host.specs[0].CrewID, host.specs[0].Prompt, working)
	}
	if host.specs[1].CrewID != waiting || host.specs[1].Prompt != "" {
		t.Errorf("waiting crew resumed as %s with %q, want %s with nothing typed in", host.specs[1].CrewID, host.specs[1].Prompt, waiting)
	}
	m, _ := w.Missions.Load("cache")
	for _, c := range m.Crew {
		if c.Interrupted {
			t.Errorf("%s is still interrupted: each is looked at once", c.ID)
		}
	}
	if m.Dispatches != 0 {
		t.Errorf("resuming counted %d dispatches", m.Dispatches)
	}
	if again := w.ResumeInterrupted(); len(again) != 0 {
		t.Errorf("looked at again: %v", again)
	}
}

// Crew interrupted on a mission that's parked or has ended, or that later
// crew have taken the capsule from, stay as they are.
func TestResumeInterruptedLeaves(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "one")
	host := &fakeHost{}
	w.opts.Host = host
	harness.Register(resumableHarness{})

	interruptedCrew(t, w, "one", mission.ActivityWorking)
	interruptedCrew(t, w, "one", mission.ActivityWorking)
	w.Missions.Update("cache", func(m *mission.Mission) error { m.State = mission.Parked; return nil })
	got := w.ResumeInterrupted()
	if len(got) != 2 || got[0].Why != "the mission is parked" || len(host.specs) != 0 {
		t.Fatalf("parked: %v, launched %d", got, len(host.specs))
	}

	w2, _ := fixture(t, "## Goal\nCache it.\n", "one")
	w2.opts.Host = host
	first := interruptedCrew(t, w2, "one", mission.ActivityWorking)
	interruptedCrew(t, w2, "one", mission.ActivityWorking)
	got = w2.ResumeInterrupted()
	if len(got) != 2 || got[0].ID != first || got[0].Why != "c2 has worked in frontend/one since" || !got[1].Resumed {
		t.Errorf("superseded: %v", got)
	}

	w3, _ := fixture(t, "## Goal\nCache it.\n", "one")
	w3.opts.Host = host
	gone := interruptedCrew(t, w3, "one", mission.ActivityWorking)
	os.RemoveAll(filepath.Join(w3.WS().RepoDir("frontend"), "one"))
	got = w3.ResumeInterrupted()
	if len(got) != 1 || got[0].ID != gone || got[0].Resumed || !strings.Contains(got[0].Why, "is gone") {
		t.Errorf("capsule gone: %v", got)
	}
	if m, _ := w3.Missions.Load("cache"); m.Crew[0].ResumeWith != "" {
		t.Errorf("a resume that didn't happen left its message: %q", m.Crew[0].ResumeWith)
	}
}

// Crew resumed by hand that ended mid-turn are told to carry on, as a
// restart's are; crew that were waiting for you, or had handed their work
// back, are reopened as they were; a message to resume with is what's said.
func TestResume_ByHandMidTurnCarriesOn(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "one", "two", "three", "four")
	host := &fakeHost{}
	w.opts.Host = host
	harness.Register(resumableHarness{})
	ended := func(capsule, activity string) string {
		id := interruptedCrew(t, w, capsule, activity)
		w.Missions.ClearInterrupted("cache", id)
		return id
	}
	working := ended("one", mission.ActivityWorking)
	waiting := ended("two", mission.ActivityWaiting)
	done := ended("three", mission.ActivityWorking)
	w.Missions.SetCrewStatus("cache", done, mission.CrewDone, "built it")
	told := ended("four", mission.ActivityWorking)
	w.Missions.SetResumeWith("cache", told, "the error path has no test")

	for _, id := range []string{working, waiting, done, told} {
		if _, err := w.Launch("cache", Dispatch{Resume: id}, AsYou); err != nil {
			t.Fatalf("resuming %s: %v", id, err)
		}
	}
	want := map[string]string{working: ResumedCarryOn, waiting: "", done: "", told: "the error path has no test"}
	for _, sp := range host.specs {
		if sp.Prompt != want[sp.CrewID] {
			t.Errorf("%s resumed with %q, want %q", sp.CrewID, sp.Prompt, want[sp.CrewID])
		}
	}
	if len(host.specs) != 4 {
		t.Errorf("launched %d, want 4", len(host.specs))
	}
}

// Crew who were resumed in their capsule after later crew had come and
// gone there, and who then asked you something, are resumed after a
// restart, still asking: the later crew were in the capsule before them,
// not since.
func TestResumeInterrupted_AfterLaterCrewLeftStillAsking(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "one")
	host := &fakeHost{}
	w.opts.Host = host
	harness.Register(resumableHarness{})

	asking := interruptedCrew(t, w, "one", mission.ActivityWaiting)
	later, err := w.Missions.StartCrewOn("cache", "fake-resumable", "", "session-later", "")
	if err != nil {
		t.Fatal(err)
	}
	w.Missions.SetCrewProcess("cache", later.ID, 2, "frontend/one")
	w.Missions.EndCrew("cache", later.ID, "exit 0")
	// Then the first was resumed there, and asked: told apart by time,
	// not by which was recorded first.
	gone := time.Now().Add(-time.Hour)
	asked := time.Now().Add(-time.Minute)
	w.Missions.Update("cache", func(m *mission.Mission) error {
		for i := range m.Crew {
			switch m.Crew[i].ID {
			case later.ID:
				m.Crew[i].Started, m.Crew[i].Ended = gone.Add(-time.Minute), &gone
			case asking:
				m.Crew[i].Started = gone.Add(-time.Hour)
				m.Crew[i].Status, m.Crew[i].Note, m.Crew[i].StatusAt = mission.CrewAsking, "re-run it, or fix the flake here?", &asked
				m.Crew[i].ActivityAt = &asked
			}
		}
		return nil
	})

	got := w.ResumeInterrupted()
	if len(got) != 1 || got[0].ID != asking || !got[0].Resumed {
		t.Fatalf("got %v, want %s resumed", got, asking)
	}
	m, _ := w.Missions.Load("cache")
	for _, c := range m.Crew {
		if c.ID == asking && (c.Status != mission.CrewAsking || c.Note != "re-run it, or fix the flake here?") {
			t.Errorf("resumed %s says %q %q, want its question kept", c.ID, c.Status, c.Note)
		}
	}
}
