package outbox

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/store/storetest"
)

type op struct {
	Say string `json:"say"`
}

// states are a lane's writes as they stand: "hello waiting".
func states(t *testing.T, root, lane string) string {
	t.Helper()
	items, err := In(root, lane)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, it := range items {
		out = append(out, it.Doing+" "+it.State)
	}
	return strings.Join(out, ", ")
}

// A lane's writes are sent in the order they were made, one at a time,
// and forgotten once their kind has been told; one that's refused is
// kept, with why, to send again or let go of, and doesn't hold up those
// after it.
func TestQueue(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	var mu sync.Mutex
	var sent, told []string
	refuse := "no"
	q := &Queue{Root: root}
	q.Handle("say", Kind{
		Send: func(it Item) error {
			mu.Lock()
			defer mu.Unlock()
			if it.Doing == refuse {
				return errors.New("GitHub said no")
			}
			sent = append(sent, it.Doing)
			return nil
		},
		Settled: func(lane string, ok, failed []Item) {
			mu.Lock()
			defer mu.Unlock()
			// What was made is still kept as this is told.
			told = append(told, fmt.Sprintf("%s: %d sent, %d failed; kept: %s", lane, len(ok), len(failed), states(t, root, lane)))
		},
	})

	hold := make(chan struct{})
	q.Handle("slow", Kind{Send: func(Item) error { <-hold; return nil }})
	if _, err := q.Add("a", "slow", "first", op{}); err != nil {
		t.Fatal(err)
	}
	for _, doing := range []string{"second", "no", "third"} {
		if _, err := q.Add("a", "say", doing, op{Say: doing}); err != nil {
			t.Fatal(err)
		}
	}
	// Another lane's doesn't wait on this one's.
	q.Add("b", "say", "elsewhere", op{})
	q.laneIdle(t, "b")
	if unsent, failed, _ := Standing(root); unsent != 4 || failed != 0 {
		t.Errorf("standing while a's first is being sent: %d unsent, %d failed", unsent, failed)
	}
	close(hold)
	q.Wait()

	mu.Lock()
	if got := strings.Join(sent, " "); got != "elsewhere second third" {
		t.Errorf("sent %q", got)
	}
	if len(told) < 2 || !strings.Contains(strings.Join(told, "\n"), "no failed") || !strings.Contains(strings.Join(told, "\n"), "second sent") {
		t.Errorf("told %q", told)
	}
	mu.Unlock()
	if got := states(t, root, "a"); got != "no failed" {
		t.Errorf("kept after sending: %q", got)
	}
	if unsent, failed, _ := Standing(root); unsent != 0 || failed != 1 {
		t.Errorf("standing: %d unsent, %d failed", unsent, failed)
	}
	items, _ := In(root, "a")
	if items[0].Error != "GitHub said no" {
		t.Errorf("why it failed: %q", items[0].Error)
	}

	// Sent again, once GitHub would take it.
	mu.Lock()
	refuse = ""
	mu.Unlock()
	if err := q.Retry(items[0].ID); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	if got := states(t, root, "a"); got != "" {
		t.Errorf("kept after sending it again: %q", got)
	}
	// Only one that failed is sent again, or let go of.
	if err := q.Retry(items[0].ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("sending again what's gone: %v", err)
	}

	// Let go of.
	mu.Lock()
	refuse = "never"
	mu.Unlock()
	it, _ := q.Add("a", "say", "never", op{})
	q.Wait()
	if err := q.Drop(it.ID); err != nil {
		t.Fatal(err)
	}
	if got := states(t, root, "a"); got != "" {
		t.Errorf("kept after letting go: %q", got)
	}
}

// laneIdle waits for a lane to have nothing left to send.
func (q *Queue) laneIdle(t *testing.T, lane string) {
	t.Helper()
	for i := 0; i < 2000; i++ {
		q.mu.Lock()
		running := q.running[lane]
		q.mu.Unlock()
		if !running {
			return
		}
		sleep()
	}
	t.Fatalf("%s's writes were never sent", lane)
}

// What a daemon before this one left: a write it hadn't sent is sent; one
// it was in the middle of is sent again if twice is the same as once, and
// otherwise kept as failed, as it may have been made; one it had sent and
// not said is said.
func TestResume(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	before := &Queue{Root: root}
	before.stopped = true // it takes writes, and sends none
	var ids []int64
	for _, doing := range []string{"waiting", "resolve", "reply", "made"} {
		it, err := before.Add("a", "say", doing, op{})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, it.ID)
	}
	set(root, ids[1], Sending, "", false)
	set(root, ids[2], Sending, "", false)
	set(root, ids[3], Sent, "", false)

	var mu sync.Mutex
	var sent []string
	var settled int
	q := &Queue{Root: root}
	q.Handle("say", Kind{
		Send: func(it Item) error {
			mu.Lock()
			defer mu.Unlock()
			sent = append(sent, it.Doing)
			return nil
		},
		Again: func(it Item) bool { return it.Doing == "resolve" },
		Settled: func(lane string, ok, failed []Item) {
			mu.Lock()
			defer mu.Unlock()
			settled += len(ok)
		},
	})
	q.Resume()
	q.Wait()
	if got := strings.Join(sent, " "); got != "waiting resolve" {
		t.Errorf("sent %q", got)
	}
	if settled != 3 {
		t.Errorf("%d were said to be made, want 3: the two sent, and the one sent before", settled)
	}
	if got := states(t, root, "a"); got != "reply failed" {
		t.Errorf("kept: %q", got)
	}
	if items, _ := In(root, "a"); !strings.Contains(items[0].Error, "may have been") {
		t.Errorf("why: %q", items[0].Error)
	}

	// A queue that's stopped sends nothing more: what's added waits.
	q.Stop(0)
	q.Add("a", "say", "later", op{})
	q.Wait()
	if got := states(t, root, "a"); got != "reply failed, later waiting" {
		t.Errorf("after stopping: %q", got)
	}
}

func sleep() { time.Sleep(time.Millisecond) }
