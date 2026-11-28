package outbox

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// Kind is how a kind of write is sent: whoever makes writes of it says
// (Queue.Handle).
type Kind struct {
	// Send makes the write there. An error is it refused, or not known to
	// have been made: either way it's kept, failed, for whoever made it to
	// send again or let go of.
	Send func(Item) error
	// Again says whether sending a write twice is the same as sending it
	// once: one a daemon stopped in the middle of is then sent again. One
	// that isn't (a comment: twice is two) is kept as failed, as it may
	// have been made. nil: none is.
	Again func(Item) bool
	// Made says whether a write a daemon stopped in the middle of, which
	// can't be sent twice, was made there: one that was is done with, one
	// that wasn't is sent, and one it can't say of is kept as failed. It's
	// asked as the lane's sent, not as the daemon starts. nil: it can't be
	// said of any.
	Made func(Item) (bool, error)
	// Hold is how long one of this kind waits after it's made before it's
	// sent, so it can still be taken back (Cancel): an undo pressed in
	// the moment after. Those behind it in its lane wait with it, as a
	// lane's go in the order they were made. A daemon that stops in the
	// meantime leaves it waiting for the next, which holds it what's
	// left of the time.
	Hold time.Duration
	// Halts: one of this kind failing holds those of it waiting behind it
	// in its lane, which fail too, unsent, as they were to be made in
	// order (a stack's reviews, bottom first).
	Halts bool
	// Settled is told once a lane's writes of this kind have been sent:
	// those made (still kept, so what's shown of them stays until this
	// returns) and those that failed. It's where what they changed is
	// read again.
	Settled func(lane string, sent, failed []Item)
}

// atOnce is how many lanes' writes are sent at a time.
const atOnce = 4

// stoppedMidway is why a write a daemon stopped in the middle of, which
// can't be sent twice, is failed.
const stoppedMidway = "Workspace stopped mid-send, so it may have gone through: check before sending again"

// Queue sends what's in a workspace's outbox, in the daemon: each lane's
// writes in the order they were made, one at a time.
type Queue struct {
	Root string
	// Now is the clock (tests set it).
	Now func() time.Time
	// Log is told what couldn't be done.
	Log func(format string, args ...any)

	mu      sync.Mutex
	kinds   map[string]Kind
	running map[string]bool // by lane
	// check is the writes a daemon before this one was in the middle of,
	// whose kind can say whether they were made (Kind.Made).
	check   map[int64]bool
	slots   chan struct{}
	stopped bool
	// stop is closed by Stop: a lane held (Kind.Hold) stops waiting.
	stop chan struct{}
	wg   sync.WaitGroup
}

func (q *Queue) logf(format string, args ...any) {
	if q.Log != nil {
		q.Log(format, args...)
	}
}

// Handle says how writes of a kind are sent. Said before Resume.
func (q *Queue) Handle(kind string, k Kind) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.kinds == nil {
		q.kinds = map[string]Kind{}
	}
	q.kinds[kind] = k
}

// Add keeps a write and starts sending its lane's, if they aren't being
// sent already. op is the write as its kind reads it back (JSON); doing
// is the write in words.
func (q *Queue) Add(lane, kind, doing string, op any) (Item, error) {
	it, err := add(q.Root, lane, kind, doing, op, q.now())
	if err != nil {
		return Item{}, err
	}
	q.kick(lane)
	return it, nil
}

// Retry sends a write that failed again.
func (q *Queue) Retry(id int64) error {
	it, err := Get(q.Root, id)
	if err != nil {
		return err
	}
	moved, err := set(q.Root, id, Waiting, "", true, Failed)
	if err != nil {
		return err
	}
	if !moved {
		return ErrNotFound
	}
	q.kick(it.Lane)
	return nil
}

// Cancel takes back a lane's writes of a kind that are still waiting: not
// sent, and not kept. One being sent goes on. It says which it took back.
func (q *Queue) Cancel(lane, kind string) ([]Item, error) {
	items, err := In(q.Root, lane)
	if err != nil {
		return nil, err
	}
	var out []Item
	for _, it := range items {
		if it.Kind != kind || it.State != Waiting {
			continue
		}
		// Only from waiting: a drain that's begun sending it has moved it on.
		n, err := remove(q.Root, Waiting, it.ID)
		if err != nil {
			return out, err
		}
		if n > 0 {
			out = append(out, it)
		}
	}
	if len(out) > 0 {
		touch(q.Root)
	}
	return out, nil
}

// Drop lets go of a write that failed: it's not made, and not kept.
func (q *Queue) Drop(id int64) error {
	n, err := remove(q.Root, Failed, id)
	if err == nil && n == 0 {
		err = ErrNotFound
	}
	return err
}

// Resume takes up what a daemon before this one left: a write it was in
// the middle of is sent again if that's the same as once; if its kind can
// say whether it was made, it's asked; and otherwise it's failed, as it
// may have been made. Then every lane with something waiting is sent. Called once, before anything's added: a write being
// sent by this daemon would be taken for one the last left.
func (q *Queue) Resume() {
	all, err := In(q.Root, "")
	if err != nil {
		q.logf("outbox: reading what's waiting: %v", err)
		return
	}
	lanes := map[string]bool{}
	for _, it := range all {
		switch it.State {
		case Sending:
			q.mu.Lock()
			k := q.kinds[it.Kind]
			q.mu.Unlock()
			if k.Again != nil && k.Again(it) {
				if _, err := set(q.Root, it.ID, Waiting, "", false, Sending); err != nil {
					q.logf("outbox: %s: %v", it, err)
				}
				lanes[it.Lane] = true
				continue
			}
			if k.Made != nil {
				q.mu.Lock()
				if q.check == nil {
					q.check = map[int64]bool{}
				}
				q.check[it.ID] = true
				q.mu.Unlock()
				lanes[it.Lane] = true
				continue
			}
			q.logf("outbox: %s was being sent when the daemon before this one went: it's kept, to look at", it)
			if _, err := set(q.Root, it.ID, Failed, stoppedMidway, true, Sending); err != nil {
				q.logf("outbox: %s: %v", it, err)
			}
		case Waiting, Sent:
			lanes[it.Lane] = true
		}
	}
	for lane := range lanes {
		q.kick(lane)
	}
}

// kick starts sending a lane's writes, unless they're being sent.
func (q *Queue) kick(lane string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.stopped || q.running[lane] {
		return
	}
	if q.running == nil {
		q.running = map[string]bool{}
		q.slots = make(chan struct{}, atOnce)
	}
	q.running[lane] = true
	q.wg.Add(1)
	go q.drain(lane)
}

// drain sends a lane's writes until none is waiting.
func (q *Queue) drain(lane string) {
	defer q.wg.Done()
	for {
		// What's waiting is read where kick looks, so a write added as
		// this finds none starts a drain of its own.
		q.mu.Lock()
		items, err := In(q.Root, lane)
		now := q.now()
		var todo []Item
		var due time.Time // when the first held is to be sent
		for _, it := range items {
			// (One of a kind nobody here sends waits for a ws that does.)
			if !(it.State == Waiting || it.State == Sent || it.State == Sending && q.check[it.ID]) || q.kinds[it.Kind].Send == nil {
				continue
			}
			if h := q.kinds[it.Kind].Hold; h > 0 && it.State == Waiting && now.Before(it.Queued.Add(h)) {
				due = it.Queued.Add(h) // and those after it wait with it
				break
			}
			todo = append(todo, it)
		}
		if err == nil && len(todo) == 0 && !due.IsZero() && !q.stopped {
			stop := q.stopping()
			q.mu.Unlock()
			select {
			case <-time.After(due.Sub(now)):
			case <-stop:
			}
			continue
		}
		if err != nil || len(todo) == 0 || q.stopped {
			if err != nil {
				q.logf("outbox: reading %s's: %v", lane, err)
			}
			delete(q.running, lane)
			q.mu.Unlock()
			return
		}
		q.mu.Unlock()
		q.slots <- struct{}{}
		q.send(lane, todo)
		<-q.slots
	}
}

// send sends a lane's waiting writes in turn, tells each kind how its
// went, and then forgets those made.
func (q *Queue) send(lane string, todo []Item) {
	sent, failed := map[string][]Item{}, map[string][]Item{}
	var kinds []string
	note := func(m map[string][]Item, it Item) {
		if len(sent[it.Kind])+len(failed[it.Kind]) == 0 {
			kinds = append(kinds, it.Kind)
		}
		m[it.Kind] = append(m[it.Kind], it)
	}
	// fail keeps a write as failed, with why, and holds those behind it
	// where its kind says to.
	fail := func(k Kind, it Item, why string) {
		it.State, it.Error = Failed, why
		if _, err := set(q.Root, it.ID, Failed, why, true, Sending); err != nil {
			q.logf("outbox: %s: %v", it, err)
		}
		note(failed, it)
		if k.Halts {
			for _, held := range q.hold(lane, it) {
				note(failed, held)
			}
		}
	}
	made := func(it Item) {
		it.State = Sent
		if _, err := set(q.Root, it.ID, Sent, "", true, Sending); err != nil {
			q.logf("outbox: %s: %v", it, err)
		}
		note(sent, it)
	}
	for _, it := range todo {
		if it.State == Sent { // made by a daemon before this one, which didn't live to say
			note(sent, it)
			continue
		}
		q.mu.Lock()
		k, stopped := q.kinds[it.Kind], q.stopped
		q.mu.Unlock()
		if stopped {
			break // it waits for the next daemon
		}
		if it.State == Sending {
			// A daemon before this one was in the middle of it: its kind
			// says whether it got there.
			q.mu.Lock()
			delete(q.check, it.ID)
			q.mu.Unlock()
			was, err := q.wasMade(k, it)
			if err != nil {
				q.logf("outbox: %s was being sent when the daemon before this one went, and whether it got there can't be said (%v): it's kept, to look at", it, err)
				fail(k, it, stoppedMidway)
				continue
			}
			if was {
				made(it)
				continue
			}
			if _, err := set(q.Root, it.ID, Waiting, "", false, Sending); err != nil {
				q.logf("outbox: %s: %v", it, err)
				continue
			}
		}
		err := q.sendOne(k, it)
		if errors.Is(err, errGone) {
			continue
		}
		if err != nil {
			q.logf("outbox: %s failed: %v", it, err)
			fail(k, it, err.Error())
			continue
		}
		made(it)
	}
	var done []int64
	for _, kind := range kinds {
		q.mu.Lock()
		k := q.kinds[kind]
		q.mu.Unlock()
		if k.Settled != nil {
			q.settled(k, lane, sent[kind], failed[kind])
		}
		for _, it := range sent[kind] {
			done = append(done, it.ID)
		}
	}
	if _, err := remove(q.Root, Sent, done...); err != nil {
		q.logf("outbox: forgetting what %s was sent: %v", lane, err)
	}
}

// hold fails the writes of a kind waiting behind one of it that failed, in
// its lane, unsent: they were to be made in order. It's those it held.
func (q *Queue) hold(lane string, behind Item) []Item {
	items, err := In(q.Root, lane)
	if err != nil {
		q.logf("outbox: reading %s's: %v", lane, err)
		return nil
	}
	why := "not sent: " + behind.Doing + ", which came first, failed"
	var out []Item
	for _, it := range items {
		if it.ID <= behind.ID || it.Kind != behind.Kind || it.State != Waiting {
			continue
		}
		if moved, err := set(q.Root, it.ID, Failed, why, true, Waiting); err == nil && moved {
			it.State, it.Error = Failed, why
			out = append(out, it)
		}
	}
	return out
}

// wasMade asks a kind whether a write was made. A panic is that it can't
// be said.
func (q *Queue) wasMade(k Kind, it Item) (was bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			q.logf("outbox: asking whether %s was made: %v\n%s", it, r, debug.Stack())
			err = fmt.Errorf("could not check whether it was sent: %v", r)
		}
	}()
	if k.Made == nil {
		return false, errors.New("cannot tell whether it was sent")
	}
	return k.Made(it)
}

// errGone: a write that was waiting isn't any more.
var errGone = errors.New("it's no longer waiting")

// sendOne marks a write as being sent, and sends it. A panic fails it.
func (q *Queue) sendOne(k Kind, it Item) (err error) {
	moved, err := set(q.Root, it.ID, Sending, "", false, Waiting)
	if err != nil {
		return err
	}
	if !moved {
		return errGone
	}
	defer func() {
		if r := recover(); r != nil {
			q.logf("outbox: sending %s: %v\n%s", it, r, debug.Stack())
			err = fmt.Errorf("could not send: %v", r)
		}
	}()
	return k.Send(it)
}

func (q *Queue) settled(k Kind, lane string, sent, failed []Item) {
	defer func() {
		if r := recover(); r != nil {
			q.logf("outbox: after sending %s's: %v\n%s", lane, r, debug.Stack())
		}
	}()
	k.Settled(lane, sent, failed)
}

// stopping is what Stop closes. q.mu is held.
func (q *Queue) stopping() chan struct{} {
	if q.stop == nil {
		q.stop = make(chan struct{})
	}
	return q.stop
}

func (q *Queue) now() time.Time {
	if q.Now != nil {
		return q.Now()
	}
	return time.Now()
}

// Wait waits until nothing's being sent.
func (q *Queue) Wait() { q.wg.Wait() }

// Stop has the queue send no more: a write being sent is let finish, up to
// grace, and those waiting are the next daemon's. It says whether what
// was being sent finished.
func (q *Queue) Stop(grace time.Duration) bool {
	q.mu.Lock()
	if !q.stopped {
		q.stopped = true
		close(q.stopping())
	}
	q.mu.Unlock()
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(grace):
		return false
	}
}
