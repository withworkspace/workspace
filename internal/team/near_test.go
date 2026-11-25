package team

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

// nearService is a service with one capsule, api/try, whose changes are
// whatever changed says: asked counts the times git would have been.
type nearService struct {
	*Service
	mu      sync.Mutex
	now     time.Time
	asked   int
	stamp   string
	changed []string
	err     error
	gone    bool
	moved   chan struct{}
}

func newNearService() *nearService {
	n := &nearService{now: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), stamp: "a", changed: []string{"a.go"}, moved: make(chan struct{}, 8)}
	n.Service = &Service{
		Now: func() time.Time {
			n.mu.Lock()
			defer n.mu.Unlock()
			return n.now
		},
		Capsules: func(repo string) []Capsule {
			n.mu.Lock()
			defer n.mu.Unlock()
			if n.gone {
				return nil
			}
			return []Capsule{{Repo: repo, Name: "try", Dir: "/nowhere", Base: "origin/main", Stamp: n.stamp}}
		},
		Moved: func() { n.moved <- struct{}{} },
	}
	n.nearRead = func(Capsule) ([]string, error) {
		n.mu.Lock()
		defer n.mu.Unlock()
		n.asked++
		return slices.Clone(n.changed), n.err
	}
	return n
}

func (n *nearService) set(f func()) {
	n.mu.Lock()
	defer n.mu.Unlock()
	f()
}

// near is what a read says is near the one change there is, which
// touched a.go and b.go in api.
func (n *nearService) near() []Near {
	t := &Timeline{Days: []Day{{Items: []Item{{Events: []Event{{Repo: "api", Paths: []string{"a.go", "b.go"}}}}}}}}
	n.NearYou(t)
	return t.Days[0].Items[0].Near
}

// settle waits for the look behind a read to end.
func (n *nearService) settle(t *testing.T) {
	t.Helper()
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until); time.Sleep(time.Millisecond) {
		n.nearMu.Lock()
		looking := n.nearLooking
		n.nearMu.Unlock()
		if !looking {
			return
		}
	}
	t.Fatal("the look never ended")
}

func (n *nearService) told() bool {
	select {
	case <-n.moved:
		return true
	default:
		return false
	}
}

func TestNearYou_AnswersFromWhatsKept(t *testing.T) {
	n := newNearService()
	// Git is slow: the read doesn't wait for it.
	release := make(chan struct{})
	read := n.nearRead
	n.nearRead = func(c Capsule) ([]string, error) {
		<-release
		return read(c)
	}
	if got := n.near(); len(got) != 0 {
		t.Fatalf("before it's been read: %+v", got)
	}
	version := n.NarrativeVersion()
	close(release)
	n.settle(t)
	if !n.told() || n.NarrativeVersion() == version {
		t.Error("the page that was answered without it wasn't told")
	}
	got := n.near()
	if len(got) != 1 || got[0].Capsule != "api/try" || !slices.Equal(got[0].Files, []string{"a.go"}) {
		t.Errorf("near = %+v", got)
	}
}

func TestNearYou_ReadsACapsuleAgainOnlyWhenItMoves(t *testing.T) {
	n := newNearService()
	n.near()
	n.settle(t)
	n.told()
	step := func(d time.Duration) []Near {
		t.Helper()
		n.set(func() { n.now = n.now.Add(d) })
		n.near()
		n.settle(t)
		return n.near()
	}

	// Looked at again, it hasn't moved: git isn't asked, nobody's told.
	step(nearEvery)
	if n.asked != 1 || n.told() {
		t.Errorf("unmoved: asked %d times", n.asked)
	}
	// It's moved, and has changed b.go too.
	n.set(func() { n.stamp, n.changed = "b", []string{"a.go", "b.go"} })
	if got := step(nearEvery); n.asked != 2 || len(got) != 1 || len(got[0].Files) != 2 || !n.told() {
		t.Errorf("moved: asked %d times, near %+v", n.asked, got)
	}
	// What's edited and not staged moves nothing: it's asked again in time.
	n.set(func() { n.changed = []string{"b.go"} })
	if got := step(nearAge); n.asked != 3 || len(got) != 1 || !slices.Equal(got[0].Files, []string{"b.go"}) || !n.told() {
		t.Errorf("after a while: asked %d times, near %+v", n.asked, got)
	}
	// Git can't say: it stays as it was.
	n.set(func() { n.stamp, n.changed, n.err = "c", nil, errors.New("busy") })
	if got := step(nearEvery); n.asked != 4 || len(got) != 1 || n.told() {
		t.Errorf("unread: asked %d times, near %+v", n.asked, got)
	}
	// The capsule's gone.
	n.set(func() { n.gone = true })
	if got := step(nearEvery); len(got) != 0 || !n.told() {
		t.Errorf("gone: near %+v", got)
	}
}
