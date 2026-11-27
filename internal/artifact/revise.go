package artifact

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// revise writes a new version of the artifact whose folder is dir:
// numbered after its latest, committed to its repo, then named in
// artifact.toml. It's the record, the version's number, and how to take
// the commit away again (for a caller whose row isn't written): nil once
// artifact.toml names it.
func revise(ctx context.Context, dir, id string, r Revision) (*Artifact, int, func(), error) {
	at := r.At.UTC().Truncate(time.Second)
	files, err := checkFiles(r.files())
	if err != nil {
		return nil, 0, nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, RecordFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil, fmt.Errorf("%s's folder is missing (%s): add a new artifact", id, dir)
	}
	if err != nil {
		return nil, 0, nil, err
	}
	a, err := Parse(data) // the file wins
	if err != nil {
		return nil, 0, nil, fmt.Errorf("could not read %s's record (%s): %w", id, filepath.Join(dir, RecordFile), err)
	}
	if a.Status == Discarded {
		return nil, 0, nil, fmt.Errorf("%w: %s was discarded: add a new artifact instead", ErrDiscarded, id)
	}
	// One from when a version was a file in the folder: into its repo
	// first, so the new one has what it's based on to stand on.
	if _, err := convert(ctx, dir, a); err != nil {
		return nil, 0, nil, fmt.Errorf("could not upgrade %s: %w", id, err)
	}
	latest, _ := a.Latest()
	on := r.On
	if on == 0 {
		on = latest.N
	} else if _, ok := a.Version(on); !ok {
		return nil, 0, nil, fmt.Errorf("%s has no v%d to base on: %s", id, on, versionsSaid(*a))
	}
	// The number after the latest, and after any commit no version names
	// (what a write that didn't finish left): a version's number is never
	// used for two.
	repo := Repo(filepath.Join(dir, RepoDir))
	n := latest.N + 1
	for ; repo.Has(ctx, n); n++ {
		if n > latest.N+100 {
			return nil, 0, nil, fmt.Errorf("could not number a new version of %s", id)
		}
	}
	v := r.Version
	v.N, v.BasedOn, v.At, v.Note = n, on, at, CleanNote(v.Note)
	v.Files, v.Size = len(files), files.Size()
	if t := CleanTitle(r.Title); t != "" {
		a.Title = t
	}
	if _, err := repo.CommitVersion(ctx, n, on, files, versionMessage(a.Title, v), at); err != nil {
		return nil, 0, nil, err
	}
	undo := func() { repo.Drop(ctx, n) }
	a.Versions = append(a.Versions, v)
	if _, err := writeRecord(dir, a); err != nil {
		return nil, 0, undo, err
	}
	return a, n, nil, nil
}
