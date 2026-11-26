package ops

import (
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/withworkspace/workspace/internal/mission"
)

// A mission's page is told when what it's built from has moved
// (MissionStamp), which is asked every time its hub looks. In the daemon
// that's a lookup:
//
//   - its folder's files (its brief, files/, reviews, a bundle) aren't
//     walked: the folder's watched (WatchGit's files watch), and each
//     mission counts the writes in it (keptMissions.files).
//   - its record, for which capsules and repos it has, isn't read: it's
//     the mission as its page was last built from (Mission), or read once.
//     A write to the records that names it has its page built again, which
//     reads it then.
//
// Outside the daemon (nothing's watched) the folder's walked and the
// record's read, as before.

type keptMissions struct {
	// watching: the files watch is on, so a write in a mission's folder
	// is counted.
	watching atomic.Bool

	mu    sync.Mutex
	files map[string]int64            // writes in each mission's folder, by slug
	read  map[string]*mission.Mission // each mission as last read for its page
}

// wrote counts the writes in the folders written (as the files watch says
// them) that are a mission's, but those its page isn't built from (its
// documents' versions, the orchestrator's runs: missionStamp skips them).
func (k *keptMissions) wrote(missionsDir string, dirs []string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, dir := range dirs {
		rel, err := filepath.Rel(missionsDir, dir)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		slug, rest, _ := strings.Cut(rel, string(filepath.Separator))
		if sub, _, _ := strings.Cut(rest, string(filepath.Separator)); sub == "docs" || sub == "orchestrator" {
			continue
		}
		if k.files == nil {
			k.files = map[string]int64{}
		}
		k.files[slug]++
	}
}

// filesOf names the writes in slug's folder, while they're counted.
func (k *keptMissions) filesOf(slug string) (string, bool) {
	if !k.watching.Load() {
		return "", false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	return "files " + strconv.FormatInt(k.files[slug], 10), true
}

// keep notes slug as it was just read for its page.
func (k *keptMissions) keep(slug string, m *mission.Mission) {
	if !k.watching.Load() {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.read == nil {
		k.read = map[string]*mission.Mission{}
	}
	k.read[slug] = m
}

// mission is slug as last read for its page, while that's kept.
func (k *keptMissions) mission(slug string) (*mission.Mission, bool) {
	if !k.watching.Load() {
		return nil, false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	m, ok := k.read[slug]
	return m, ok
}
