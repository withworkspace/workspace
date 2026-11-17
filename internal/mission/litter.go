package mission

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/withworkspace/workspace/internal/store"
)

// Before is what a mission's folder held of its record before the records
// were kept in SQLite: nothing reads them now.
var Before = []string{"mission.toml", "log.jsonl", "decisions.jsonl", ".lock"}

// Litter is what's in the missions' folder that no record has (ws
// doctor): a folder no mission's row names, whole, and in a mission's
// folder the files its record was kept in before the records (Before).
func (s Store) Litter(ctx context.Context) ([]string, error) {
	rows := map[string]bool{}
	slugs, err := s.SlugsKept(ctx)
	if err != nil {
		return nil, err
	}
	for _, slug := range slugs {
		rows[slug] = true
	}
	entries, err := os.ReadDir(s.Dir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		dir := filepath.Join(s.Dir(), e.Name())
		if !e.IsDir() {
			continue
		}
		if !rows[e.Name()] {
			out = append(out, dir)
			continue
		}
		for _, name := range Before {
			if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
				out = append(out, filepath.Join(dir, name))
			}
		}
	}
	return out, nil
}

// SlugsKept is every mission the records keep, or why they can't be read
// (where Slugs says none).
func (s Store) SlugsKept(ctx context.Context) ([]string, error) {
	db, err := s.records()
	if err != nil {
		return nil, err
	}
	var slugs []string
	err = db.Read(ctx, func(tx *store.Tx) error {
		slugs, err = column(tx, `SELECT slug FROM missions ORDER BY slug`)
		return err
	})
	return slugs, err
}
