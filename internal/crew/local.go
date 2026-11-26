package crew

import (
	"errors"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/termpane"
)

// LeaveGrace is how long crew get to exit after SIGHUP when an in-process
// host stops them.
const LeaveGrace = 3 * time.Second

// LocalHost runs crew as children of this process, so they stop when it
// exits. The workspace's daemon is the normal host; this one is for tests.
// It records ends exactly as the server does, and types the opening prompt
// in as the server does.
type LocalHost struct {
	Store   mission.Store
	OnEnded EndHook
	// TrustFolders, as daemon.Server's: whether to answer Claude Code's
	// trust dialog yes for crew. Nil leaves it to you.
	TrustFolders func() bool

	mu      sync.Mutex
	terms   []*localTerm
	byKey   map[string]*localTerm
	ending  sync.WaitGroup
	leaving atomic.Bool
}

// localTerm is a pane whose Done (and Exited) wait for its end to be
// recorded, so nobody sees a crew exit before the mission says so.
type localTerm struct {
	*termpane.Pane
	stopped atomic.Bool
	done    chan struct{}
	copies  atomic.Uint64 // copies already taken
}

func (t *localTerm) TakeCopied() (string, bool) {
	n, text := t.Pane.Copied()
	if t.copies.Swap(n) == n {
		return "", false
	}
	return text, true
}

func (t *localTerm) Done() <-chan struct{} { return t.done }

func (t *localTerm) Exited() bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

func (t *localTerm) Close(grace time.Duration) {
	t.stopped.Store(true)
	t.Pane.Close(grace)
}

func (h *LocalHost) Launch(s Spec) (int, error) {
	if s.ACP != nil {
		return 0, errors.New("crew over ACP need the workspace's daemon to hold them")
	}
	cmd := exec.Command(s.Args[0], s.Args[1:]...)
	cmd.Dir = s.Dir
	cmd.Env = s.Env
	p, err := termpane.Start(cmd, s.Width, s.Height)
	if err != nil {
		return 0, err
	}
	t := &localTerm{Pane: p, done: make(chan struct{})}
	h.mu.Lock()
	h.terms = append(h.terms, t)
	if h.byKey == nil {
		h.byKey = map[string]*localTerm{}
	}
	h.byKey[s.Key()] = t
	h.mu.Unlock()
	h.ending.Go(func() {
		defer close(t.done)
		<-p.Done()
		exit, leaving := p.ExitSummary(), h.leaving.Load()
		if leaving {
			exit = "stopped: " + exit
		}
		if err := Finish(h.Store, s, exit, p.Transcript(), HowEnded(t.stopped.Load(), leaving), h.OnEnded); err != nil {
			execlog.Note(h.Store.Root, "crew "+s.Slug+"/"+s.CrewID, "couldn't record its end: %v", err)
		}
	})
	// The prompt's typer and the trust dialog's watch end with the crew,
	// and Leave waits for them as it does for the end's record: neither
	// writes to a mission that's been left.
	if s.Prompt != "" {
		h.ending.Go(func() { TypePrompt(h.Store.Root, s.Key(), t, s.Prompt) })
	}
	h.ending.Go(func() { WatchTrust(h.Store, h.Store.Root, s, t, h.TrustFolders) })
	return p.Pid(), nil
}

// Stop stops crew this host is running, by key.
func (h *LocalHost) Stop(key string) error {
	t, ok := h.Term(key)
	if !ok || t.Exited() {
		return ErrNotRunning
	}
	t.Close(LeaveGrace)
	return nil
}

// Say types a message into crew this host is running.
func (h *LocalHost) Say(key, text string) (string, error) {
	t, ok := h.Term(key)
	if !ok || t.Exited() {
		return "", ErrNotRunning
	}
	slug, id, _ := strings.Cut(key, "/")
	return TermRun{S: Spec{Slug: slug, CrewID: id}, T: t, Root: h.Store.Root}.Say(text)
}

// Term is crew's terminal, by key, while this host has it.
func (h *LocalHost) Term(key string) (Term, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.byKey[key]
	return t, ok
}

// Leave stops every crew still running and waits until all their ends are
// recorded.
func (h *LocalHost) Leave() {
	h.leaving.Store(true)
	h.mu.Lock()
	terms := append([]*localTerm(nil), h.terms...)
	h.mu.Unlock()
	var wg sync.WaitGroup
	for _, t := range terms {
		wg.Go(func() {
			t.Pane.Close(LeaveGrace)
		})
	}
	wg.Wait()
	h.ending.Wait()
}

// PaneTerm is a pane in this process, as crew's Term: what a host types an
// opening prompt into.
func PaneTerm(p *termpane.Pane) Term { return &paneTerm{Pane: p} }

// paneTerm is a pane as a Term, ended when its process is.
type paneTerm struct {
	*termpane.Pane
	copies atomic.Uint64
}

func (t *paneTerm) TakeCopied() (string, bool) {
	n, text := t.Pane.Copied()
	if t.copies.Swap(n) == n {
		return "", false
	}
	return text, true
}
