package artifact

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// revise writes a new version of the artifact whose folder is dir:
// numbered after its latest, its file, then artifact.toml naming it.
// It's the record, the version's number, and the file it made (so a
// caller whose row isn't written can take it away again): "" once
// artifact.toml names it.
func revise(dir, id string, r Revision) (*Artifact, int, string, error) {
	at := r.At.UTC().Truncate(time.Second)
	data, err := os.ReadFile(filepath.Join(dir, RecordFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, "", fmt.Errorf("%s's folder has gone (%s): add a new artifact", id, dir)
	}
	if err != nil {
		return nil, 0, "", err
	}
	a, err := Parse(data) // the file wins
	if err != nil {
		return nil, 0, "", fmt.Errorf("%s's record can't be read (%s): %w", id, filepath.Join(dir, RecordFile), err)
	}
	if a.Status == Discarded {
		return nil, 0, "", fmt.Errorf("%w: %s was discarded, so it takes no new versions: add a new artifact", ErrDiscarded, id)
	}
	latest, _ := a.Latest()
	on := r.On
	if on == 0 {
		on = latest.N
	} else if _, ok := a.Version(on); !ok {
		return nil, 0, "", fmt.Errorf("%s has no v%d to base it on: %s", id, on, versionsSaid(*a))
	}
	// The number after the latest, and after any file no version names
	// (what a write that didn't finish left): a version's number is never
	// used for two files.
	n := latest.N + 1
	var made string
	for ; ; n++ {
		if n > latest.N+100 {
			return nil, 0, "", fmt.Errorf("couldn't number a new version of %s", id)
		}
		p := filepath.Join(dir, VersionFile(n))
		err := writeNew(p, r.HTML)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return nil, 0, "", err
		}
		made = p
		break
	}
	v := r.Version
	v.N, v.BasedOn, v.At, v.Note = n, on, at, CleanNote(v.Note)
	a.Versions = append(a.Versions, v)
	if t := CleanTitle(r.Title); t != "" {
		a.Title = t
	}
	if _, err := writeRecord(dir, a); err != nil {
		return nil, 0, made, err
	}
	return a, n, "", nil
}
