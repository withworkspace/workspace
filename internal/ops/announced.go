package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// What needs you is told to the desktop once: a new need, while the page
// is out of sight. Which needs have been told is kept here, by each
// need's key, so a page that's reloaded, or a second one, doesn't tell
// of the same need again. Only the keys of needs that still hold are
// kept: a need that's gone is forgotten, and one that comes back is news.

type keptAnnounced struct {
	Keys []string `json:"keys"`
}

func announcedPath(root string) string { return filepath.Join(root, ".ws", "cache", "announced.json") }

// One claim at a time: two pages asking at once get one answer each, and
// only one of them is told a need is new.
var announcing sync.Mutex

// Announce claims the needs with these keys as told to the desktop, and
// answers with those that hadn't been: the ones the asker is to tell of.
// holding is every need that holds now, as the asker has them; what's
// kept of needs that aren't among them is dropped.
func Announce(root string, keys, holding []string) ([]string, error) {
	announcing.Lock()
	defer announcing.Unlock()
	var kept keptAnnounced
	if data, err := os.ReadFile(announcedPath(root)); err == nil {
		json.Unmarshal(data, &kept) //nolint:errcheck // one that doesn't read is none
	}
	fresh := []string{}
	next := []string{}
	for _, k := range kept.Keys {
		if slices.Contains(holding, k) || slices.Contains(keys, k) {
			next = append(next, k)
		}
	}
	for _, k := range keys {
		if k == "" || slices.Contains(next, k) {
			continue
		}
		next = append(next, k)
		fresh = append(fresh, k)
	}
	if len(fresh) == 0 && len(next) == len(kept.Keys) {
		return fresh, nil
	}
	data, err := json.Marshal(keptAnnounced{Keys: next})
	if err != nil {
		return nil, err
	}
	path := announcedPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return nil, err
	}
	return fresh, os.Rename(tmp, path)
}
