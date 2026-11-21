package runs

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
)

// Pointers say what meets a workspace recipe's needs, in place of the
// nearest: for each recipe, by the need's name, a worktree of the need's
// repo (a capsule's name, .ground or .silo). They're kept beside the
// workspace's runs, not in a run's folder, so they outlast the run: a
// recipe started again is started on what it was pointed at.

const pointsFile = "points.json"

func (s Store) pointsPath() string {
	return filepath.Join(s.Dir(), workspacePlace[0], pointsFile)
}

func (s Store) readPoints() (map[string]map[string]string, error) {
	b, err := os.ReadFile(s.pointsPath())
	if errors.Is(err, os.ErrNotExist) {
		return map[string]map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]string{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Points is what recipe's needs are pointed at, by need: those not in it
// are met by the nearest.
func (s Store) Points(recipe string) map[string]string {
	all, err := s.readPoints()
	if err != nil {
		return map[string]string{}
	}
	out := maps.Clone(all[recipe])
	if out == nil {
		out = map[string]string{}
	}
	return out
}

// Point points recipe's need at worktree: "" is the nearest again.
func (s Store) Point(recipe, need, worktree string) error {
	all, err := s.readPoints()
	if err != nil {
		all = map[string]map[string]string{} // unreadable: what's said now is what's kept
	}
	if worktree == "" {
		delete(all[recipe], need)
		if len(all[recipe]) == 0 {
			delete(all, recipe)
		}
	} else {
		if all[recipe] == nil {
			all[recipe] = map[string]string{}
		}
		all[recipe][need] = worktree
	}
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	path := s.pointsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
