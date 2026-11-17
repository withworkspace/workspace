package job

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// One job runs on a subject at a time; a newer one waiting takes the place
// of the one before; other subjects run alongside; a panic fails its job.
func TestQueue(t *testing.T) {
	var mu sync.Mutex
	var ran []string
	release := make(chan struct{})
	var logs []string
	q := &Queue{
		Run: func(j Job) error {
			mu.Lock()
			ran = append(ran, j.ID)
			mu.Unlock()
			switch j.ID {
			case "first":
				<-release
			case "boom":
				panic("boom")
			}
			return nil
		},
		Queued: func(j Job) Job { j.Why += "!"; return j },
		Log:    func(f string, a ...any) { mu.Lock(); logs = append(logs, f); mu.Unlock() },
	}
	q.Start(Job{Kind: Orchestrate, Mission: "m", ID: "first"})
	time.Sleep(50 * time.Millisecond) // first is running
	q.Start(Job{Kind: Orchestrate, Mission: "m", ID: "second"})
	q.Start(Job{Kind: Orchestrate, Mission: "m", ID: "third"}) // takes second's place
	q.Start(Job{Kind: Answer, Mission: "m", ID: "boom"})       // another subject
	if !q.Busy() {
		t.Error("not busy with a job running")
	}
	close(release)
	q.Wait()
	mu.Lock()
	got := strings.Join(ran, ",")
	mu.Unlock()
	if !strings.HasPrefix(got, "first") || strings.Contains(got, "second") || !strings.Contains(got, "third") || !strings.Contains(got, "boom") {
		t.Errorf("ran %s", got)
	}
	if q.Busy() {
		t.Error("busy with nothing running")
	}
	found := false
	for _, l := range logs {
		found = found || strings.Contains(l, "failed: %v")
	}
	if !found {
		t.Errorf("the panic wasn't logged: %q", logs)
	}
}
