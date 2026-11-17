package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
)

// A quiet turn ws held while crew's own work ran is handed on saying what
// ran, whether it ended and how long after the turn, and what to make of
// it. No quiet turn says the session is "waiting for input": crew may be
// waiting on work.
func TestPrompt_HeldQuietTurn(t *testing.T) {
	at := time.Date(2026, 11, 12, 18, 21, 35, 0, time.UTC)
	ended := at.Add(6*time.Minute + 50*time.Second)
	crew := func(p *mission.Pending) *mission.Mission {
		return &mission.Mission{Slug: "cache", Title: "Cache", Crew: []mission.Crew{{ID: "c1", Started: at.Add(-time.Hour), Activity: mission.ActivityWaiting, ActivityAt: &at, Pending: p}}}
	}
	work := func(w mission.Work, ended *time.Time) mission.Work { w.Ended = ended; return w }
	server := mission.Work{What: "nohup node server.js", From: "processes", PID: 4242, Wakes: mission.WakeNo}
	suite := mission.Work{What: "just test", From: "harness", Wakes: mission.WakeYes}

	ran := Prompt(Input{Mission: crew(&mission.Pending{At: at, Work: []mission.Work{work(server, &ended)}}), CrewID: "c1", Trigger: "quiet",
		LastReply: "Started the server; I'll check it once it's up.", Now: ended.Add(15 * time.Second)})
	for _, want := range []string{
		"Crew c1 finished a turn 7 minutes ago without reporting done or asking the user anything",
		"so ws held the turn until now",
		"- `nohup node server.js`: ended 7 minutes after the turn did. Nothing wakes c1 for it.",
		"The work has ended and nobody has told c1",
		"Its last message:\n\nStarted the server",
	} {
		if !strings.Contains(ran, want) {
			t.Errorf("work that ended: prompt missing %q:\n%s", want, ran)
		}
	}

	running := Prompt(Input{Mission: crew(&mission.Pending{At: at, Work: []mission.Work{work(suite, nil)}}), CrewID: "c1", Trigger: "quiet", Now: at.Add(mission.HoldMax)})
	for _, want := range []string{
		"- `just test`: still running, 30 minutes after the turn ended. Its harness said it would wake c1 when it ended.",
		"because the work has run 30 minutes or more",
	} {
		if !strings.Contains(running, want) {
			t.Errorf("work that never ends: prompt missing %q:\n%s", want, running)
		}
	}

	woken := Prompt(Input{Mission: crew(&mission.Pending{At: at, Work: []mission.Work{work(suite, &ended)}}), CrewID: "c1", Trigger: "quiet", Now: ended.Add(mission.WakeGrace)})
	if want := "- `just test`: ended 7 minutes after the turn did. Its harness said it would wake c1 when it ended, and it hasn't."; !strings.Contains(woken, want) {
		t.Errorf("not woken: prompt missing %q:\n%s", want, woken)
	}

	quiet := Prompt(Input{Mission: crew(nil), CrewID: "c1", Trigger: "quiet"})
	if !strings.Contains(quiet, "Crew c1 has just finished a turn without reporting done") || strings.Contains(quiet, "held the turn") {
		t.Errorf("a quiet turn nobody held:\n%s", quiet)
	}
	for _, p := range []string{ran, running, woken, quiet} {
		if strings.Contains(p, "waiting for input") {
			t.Errorf("says the session is waiting for input:\n%s", p)
		}
	}
}

func TestMinutes(t *testing.T) {
	for d, want := range map[time.Duration]string{
		10 * time.Second:               "under a minute",
		time.Minute + 20*time.Second:   "a minute",
		6*time.Minute + 50*time.Second: "7 minutes",
		31 * time.Minute:               "31 minutes",
	} {
		if got := minutes(d); got != want {
			t.Errorf("minutes(%s) = %q, want %q", d, got, want)
		}
	}
}
