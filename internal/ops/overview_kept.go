package ops

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// The workspace's view has a row for each mission in play (Overview's
// MissionItem): where its capsules stand, its headline, what's to do. A
// row takes a handful of reads of the records, and the view is asked for
// on every page that opens and whenever anything moves, so each row is
// kept, and built again only when what it's built from has moved:
//
//   - its records: a write that names the mission (mission:<slug>), which
//     its record, its log, its reviews and its conversations all do. The
//     view catches up with which have been named since it was last built,
//     in one read (catchUp).
//   - its folder, its capsules and its repos' pull requests (missionStamp).
//   - whether the orchestrator's on.
//   - keptFor since it was built: a headline says how long things have
//     taken, and the time moves it.

// keptFor is how long a mission's row is kept when nothing it's built from
// has moved.
var keptFor = 30 * time.Second

type keptOverview struct {
	mu sync.Mutex
	// seq is the records' latest change as last caught up with, and moved
	// how many times each topic's been named since the first catch-up.
	seq   int64
	moved map[string]int64
	items map[string]keptRow
	// built counts the rows built: what a test counts.
	built int64
}

type keptRow struct {
	key string
	at  time.Time
	it  MissionItem
}

// catchUp reads which topics the records' writes have named since it last
// did. false where the records can't be read: no row's kept then.
func (k *keptOverview) catchUp(root string) bool {
	db, err := store.Dial(root)
	if err != nil {
		return false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	topics, latest, err := db.Since(context.Background(), k.seq)
	if err != nil {
		return false
	}
	if k.moved == nil { // the first: nothing's kept to be moved
		k.moved, k.items = map[string]int64{}, map[string]keptRow{}
	} else if latest != k.seq {
		for _, t := range topics {
			k.moved[t]++
		}
	}
	k.seq = latest
	return true
}

// keyOf names what a mission's row is built from, as it is now.
func (k *keptOverview) keyOf(slug, stamp string, orchestratorOff bool) string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return fmt.Sprintf("%d %t %s", k.moved["mission:"+slug], orchestratorOff, stamp)
}

// item is a mission's row as kept, if it was built at key and lately.
func (k *keptOverview) item(slug, key string) (MissionItem, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	r, ok := k.items[slug]
	if !ok || r.key != key || time.Since(r.at) >= keptFor {
		return MissionItem{}, false
	}
	return r.it, true
}

func (k *keptOverview) keep(slug, key string, it MissionItem) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.items != nil {
		k.items[slug] = keptRow{key: key, at: time.Now(), it: it}
	}
}

// forgetBut drops the rows of missions no longer in play.
func (k *keptOverview) forgetBut(inPlay map[string]bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for slug := range k.items {
		if !inPlay[slug] {
			delete(k.items, slug)
		}
	}
}
