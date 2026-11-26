package web

import (
	"container/list"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
)

// What's read of a capsule for a page (its tree, its summary, a file, a
// diff) is kept, so a page that's shown again, or told something's
// changed, is answered from memory and not by git.
//
// What can't change is kept by what it's of: a file's diff between two
// commits, its lines at one. What a capsule's commits and refs say (its
// diff against its base) is kept by the capsule's stamp (ops.CapsuleStamp),
// which moves when they do; a file as it is now, by that and the file's
// own size and time. Each is read the first time it's asked for, which is
// when a page shows it, and kept in keptMax bytes between them all, the
// least lately asked for going first.
//
// A capsule's tree and summary are of its working tree too, which no
// stamp sees (a file edited, one made): they're answered at once as they
// were last read, and read again behind the read when the capsule has
// moved or readFloor has passed. A read behind that finds one other than a
// page was told moves Seen, which the workspace's view carries
// (readsSeen), and told is called: the page reads it again. Never read,
// the first ask waits firstWait for it, and after that says it's unread.
type keptReads struct {
	// told is called when a read behind finds something other than a page
	// was told (Seen has moved): the server looks at its workspace view.
	told func()

	mu    sync.Mutex
	soons map[string]*soonRead
	slots chan struct{}
	seen  atomic.Int64

	lru     *list.List // of *keptRead, the latest asked for first
	byKey   map[string]*list.Element
	size    int
	reading map[string]*reading // by key: one read of each at a time
}

// How long a tree or summary is answered as kept before it's read again
// behind the read (when its capsule hasn't moved), how long a first read
// waits before it's said to be unread, how many reads behind run at once,
// how many bytes are kept of what's read when it's asked for, and how long
// a tree or summary nobody asks for is kept.
var (
	readFloor   = 10 * time.Second
	firstWait   = 250 * time.Millisecond
	readSlots   = 4
	keptMax     = 64 << 20
	forgetAfter = 30 * time.Minute
)

// soonRead is a tree or summary: as last read, and when, at what stamp.
type soonRead struct {
	value   any
	sum     uint64 // its JSON's hash
	stamp   string
	at      time.Time
	from    time.Time // when the read that's kept began
	err     error     // the last read's, when nothing's kept
	looking chan struct{}
	// owed: a page was answered while a read was under way (what was
	// kept, or that it's unread), so it's told if the read finds other.
	owed  bool
	asked time.Time
	// dir and read are what it's of and how it's read, for reading it
	// again when its capsule moves (refresh).
	dir  string
	read func() (any, error)
}

type keptRead struct {
	key   string
	value any
	size  int
}

type reading struct {
	done  chan struct{}
	value any
	err   error
}

func (k *keptReads) init() {
	if k.soons == nil {
		k.soons, k.byKey, k.reading = map[string]*soonRead{}, map[string]*list.Element{}, map[string]*reading{}
		k.lru, k.slots = list.New(), make(chan struct{}, readSlots)
	}
}

// Seen moves each time a read behind finds a capsule's tree or summary
// other than a page was told. It starts no process.
func (k *keptReads) Seen() int64 { return k.seen.Load() }

// soon is what read says of dir (key names what it is): as kept, or read
// behind the read as the type says; unread when it's never been read and
// the first read is still under way after firstWait.
func (k *keptReads) soon(key, dir string, read func() (any, error)) (v any, unread bool, err error) {
	stamp := ops.CapsuleStamp(dir)
	now := time.Now()
	k.mu.Lock()
	k.init()
	e := k.soons[key]
	if e == nil {
		k.forget(now)
		e = &soonRead{}
		k.soons[key] = e
	}
	e.asked, e.dir, e.read = now, dir, read
	fresh := e.value != nil && stamp != "" && e.stamp == stamp && now.Sub(e.at) < readFloor
	failed := e.value == nil && e.err != nil && e.stamp == stamp && now.Sub(e.at) < readFloor
	switch {
	case fresh:
		v = e.value
		k.mu.Unlock()
		return v, false, nil
	case failed:
		err = e.err
		k.mu.Unlock()
		return nil, false, err
	}
	if e.looking == nil {
		k.look(e, stamp, read)
	}
	if e.value != nil {
		e.owed = true
		v = e.value
		k.mu.Unlock()
		return v, false, nil
	}
	looking := e.looking
	k.mu.Unlock()
	t := time.NewTimer(firstWait)
	defer t.Stop()
	select {
	case <-looking:
	case <-t.C:
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	switch {
	case e.value != nil:
		return e.value, false, nil
	case e.looking == nil && e.err != nil:
		return nil, false, e.err
	}
	e.owed = true
	return nil, true, nil
}

// refreshFor is how long after a page last asked for a tree or summary
// it's read again when its capsule moves.
var refreshFor = time.Minute

// refresh reads again, behind, each tree and summary a page has asked for
// lately whose capsule has moved since it was read: the daemon's told
// when one has (a commit, something staged), and the next ask is
// answered with it as it is, not as it was. The page showing it is told
// if it's changed.
func (k *keptReads) refresh() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.init()
	now := time.Now()
	for _, e := range k.soons {
		if e.read == nil || e.looking != nil || now.Sub(e.asked) > refreshFor {
			continue
		}
		if stamp := ops.CapsuleStamp(e.dir); stamp != e.stamp {
			e.owed = e.value != nil // a page was shown it as it was
			k.look(e, stamp, e.read)
		}
	}
}

// current is what's kept at key, if it was read at stamp, and when that
// read began: nothing's read for it.
func (k *keptReads) current(key, stamp string) (any, time.Time, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.init()
	if e := k.soons[key]; e != nil && e.value != nil && stamp != "" && e.stamp == stamp {
		return e.value, e.from, true
	}
	return nil, time.Time{}, false
}

// peek is a tree or summary as kept, read or not: nothing's read for it.
func (k *keptReads) peek(key string) (any, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.init()
	if e := k.soons[key]; e != nil && e.value != nil {
		e.asked = time.Now()
		return e.value, true
	}
	return nil, false
}

// look reads e behind the read, in one of the slots. k.mu is held.
func (k *keptReads) look(e *soonRead, stamp string, read func() (any, error)) {
	done := make(chan struct{})
	e.looking = done
	go func() {
		k.slots <- struct{}{}
		from := time.Now()
		v, err := read()
		<-k.slots
		var sum uint64
		if err == nil {
			data, _ := json.Marshal(v)
			h := fnv.New64a()
			h.Write(data)
			sum = h.Sum64()
		}
		k.mu.Lock()
		tell := false
		switch {
		case err == nil:
			tell = e.owed && (e.value == nil || sum != e.sum)
			e.value, e.sum, e.err = v, sum, nil
			e.stamp, e.at, e.from = stamp, time.Now(), from
		case e.value == nil:
			// Said again for a while, not read again at every ask.
			tell = e.owed
			e.err, e.stamp, e.at = err, stamp, time.Now()
		}
		// (What was read before stands when a read fails: the next ask
		// reads again.)
		e.owed, e.looking = false, nil
		if tell {
			k.seen.Add(1)
		}
		told := k.told
		k.mu.Unlock()
		close(done)
		if tell && told != nil {
			told()
		}
	}()
}

// forget drops the trees and summaries nobody's asked for lately. k.mu is
// held.
func (k *keptReads) forget(now time.Time) {
	for key, e := range k.soons {
		if e.looking == nil && now.Sub(e.asked) > forgetAfter {
			delete(k.soons, key)
		}
	}
}

// kept is what read says of key, which is all it depends on: as kept, or
// read (once, for whoever's asking) and kept. An error isn't kept.
func (k *keptReads) kept(key string, read func() (any, error)) (any, error) {
	k.mu.Lock()
	k.init()
	if el, ok := k.byKey[key]; ok {
		k.lru.MoveToFront(el)
		v := el.Value.(*keptRead).value
		k.mu.Unlock()
		return v, nil
	}
	if r, ok := k.reading[key]; ok {
		k.mu.Unlock()
		<-r.done
		return r.value, r.err
	}
	r := &reading{done: make(chan struct{})}
	k.reading[key] = r
	k.mu.Unlock()

	r.value, r.err = read()
	size := 0
	if r.err == nil {
		data, _ := json.Marshal(r.value)
		size = len(data) + len(key)
	}
	k.mu.Lock()
	delete(k.reading, key)
	if r.err == nil && size <= keptMax/4 {
		k.byKey[key] = k.lru.PushFront(&keptRead{key: key, value: r.value, size: size})
		k.size += size
		for k.size > keptMax {
			last := k.lru.Back()
			kr := last.Value.(*keptRead)
			k.lru.Remove(last)
			delete(k.byKey, kr.key)
			k.size -= kr.size
		}
	}
	k.mu.Unlock()
	close(r.done)
	return r.value, r.err
}

// keyOf is a key of what a read's of.
func keyOf(parts ...string) string {
	return fmt.Sprintf("%q", parts)
}

// fileStamp names a file in dir as it is now: its size and time, or that
// it's not there.
func fileStamp(dir, path string) string {
	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path)))
	if err != nil {
		return "gone"
	}
	return fmt.Sprintf("%d %d", info.Size(), info.ModTime().UnixNano())
}
