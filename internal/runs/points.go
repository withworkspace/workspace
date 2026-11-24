package runs

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"syscall"
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

// Point points recipe's need at worktree: "" is the nearest again. It's
// read, changed and written under a lock held across processes, so two
// pointed at once are both kept.
func (s Store) Point(recipe, need, worktree string) error {
	path := s.pointsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lock.Close() // which lets go of it
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
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
	// Written beside it and moved into place: a read never finds it
	// half written.
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+pointsFile+".*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(append(b, '\n'))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}
