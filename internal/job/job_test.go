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

// A mission's orchestrator and its checks take turns: neither replaces the
// other waiting, and they run in the order asked for. So do a
// conversation's thread and its draft.
func TestQueueLanes(t *testing.T) {
	var mu sync.Mutex
	var ran []string
	at, most := 0, 0
	release := make(chan struct{})
	q := &Queue{Run: func(j Job) error {
		mu.Lock()
		ran = append(ran, j.Kind+":"+j.ID)
		at++
		most = max(most, at)
		mu.Unlock()
		if j.ID == "first" {
			<-release
		}
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		at--
		mu.Unlock()
		return nil
	}}
	q.Start(Job{Kind: Check, Mission: "m", ID: "first"})
	time.Sleep(20 * time.Millisecond)
	q.Start(Job{Kind: Orchestrate, Mission: "m", ID: "o1"})
	q.Start(Job{Kind: Check, Mission: "m", ID: "c2"})
	q.Start(Job{Kind: Orchestrate, Mission: "m", ID: "o2"}) // takes o1's place, not c2's
	close(release)
	q.Wait()
	if got := strings.Join(ran, ","); got != "check:first,orchestrate:o2,check:c2" {
		t.Errorf("ran %s", got)
	}
	if most != 1 {
		t.Errorf("%d ran at once on one mission", most)
	}
	for _, k := range []string{Answer, Thread, Draft} {
		if l := (Job{Kind: k, Mission: "m", ID: "w1"}).Lane(); l != "conversation m/w1" {
			t.Errorf("%s's lane = %q", k, l)
		}
	}
	if (Job{Kind: Orchestrate, Mission: "a"}).Lane() == (Job{Kind: Orchestrate, Mission: "b"}).Lane() {
		t.Error("two missions share a lane")
	}
}

// A job's mark is given in the order jobs are queued, so the one that runs
// last on a lane has the mark that was given last.
func TestQueueMarksInOrder(t *testing.T) {
	var mu sync.Mutex
	var marked []string
	release := make(chan struct{})
	var last Job
	q := &Queue{
		Run: func(j Job) error {
			if j.ID == "first" {
				<-release
			}
			mu.Lock()
			last = j
			mu.Unlock()
			return nil
		},
		Queued: func(j Job) Job {
			time.Sleep(time.Millisecond) // a write to the records
			mu.Lock()
			marked = append(marked, j.ID)
			mu.Unlock()
			return j
		},
	}
	q.Start(Job{Kind: Orchestrate, Mission: "m", ID: "first"})
	var wg sync.WaitGroup
	for _, id := range []string{"a", "b", "c", "d"} {
		wg.Add(1)
		go func() { defer wg.Done(); q.Start(Job{Kind: Orchestrate, Mission: "m", ID: id}) }()
	}
	wg.Wait()
	close(release)
	q.Wait()
	if want := marked[len(marked)-1]; last.ID != want {
		t.Errorf("ran %s last, but %s was marked last (%v)", last.ID, want, marked)
	}
}

// Stopped, the queue runs what's running out, drops what's waiting and
// takes nothing more; it waits for what's running only so long.
func TestQueueStop(t *testing.T) {
	var mu sync.Mutex
	var ran []string
	release := make(chan struct{})
	q := &Queue{Run: func(j Job) error {
		mu.Lock()
		ran = append(ran, j.ID)
		mu.Unlock()
		<-release
		return nil
	}}
	q.Start(Job{Kind: Answer, Mission: "m", ID: "a"})
	for !q.Busy() {
		time.Sleep(time.Millisecond)
	}
	for {
		mu.Lock()
		n := len(ran)
		mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	q.Start(Job{Kind: Thread, Mission: "m", ID: "a"}) // waits behind a, in its lane
	if q.Stop(50 * time.Millisecond) {
		t.Fatal("finished, though a job's still running")
	}
	q.Start(Job{Kind: Answer, Mission: "m", ID: "b"}) // taken after: not run
	close(release)
	if !q.Stop(time.Second) {
		t.Fatal("didn't finish once its job did")
	}
	if strings.Join(ran, ",") != "a" {
		t.Errorf("ran %v, want only a", ran)
	}
}
