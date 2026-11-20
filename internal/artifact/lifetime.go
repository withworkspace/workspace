package artifact

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// An artifact's lifetime, as the store keeps it: promoted (kept for good),
// with where it lives in a repo once it does; discarded and put back; and
// deleted for good 30 days after it was discarded. What's put in a repo,
// and taken out of it, is ops's: what's here records it.

// Restorable reports whether a discarded artifact can be put back: while
// its folder's in .discarded.
func (e Entry) Restorable() bool { return e.Status == Discarded && !e.Erased }

// Promote marks artifact id promoted: its record says so, with the
// version chosen (0: its latest) and what it went into, and it expires no
// more. One promoted already is promoted again, as said now. home is
// where the version chosen was put in a repo, if it was, and here that it
// was chosen to stay on this machine: neither leaves what its record
// says of where it lives as it is.
func (s Store) Promote(ctx context.Context, id string, p Promotion, home *Home, here bool) (Entry, error) {
	p.At = p.At.UTC().Truncate(time.Second)
	return s.rewrite(ctx, id, func(k *kept, a *Artifact) error {
		if a.Status == Discarded {
			return fmt.Errorf("%w: %s was discarded: restore it first (ws artifact restore %s)", ErrDiscarded, id, id)
		}
		v, err := chosen(id, a, p.Version)
		if err != nil {
			return err
		}
		p.Version = v
		a.Status, a.Promoted = Promoted, &p
		switch {
		case home != nil:
			h := *home
			h.At = h.At.UTC().Truncate(time.Second)
			a.Home, a.Here = &h, false
		case here && a.Home == nil:
			a.Here = true
		}
		return nil
	})
}

// PutHome records where artifact id lives in a repo, as it now is: put
// back there (restored), say, in another commit.
func (s Store) PutHome(ctx context.Context, id string, home Home) (Entry, error) {
	home.At = home.At.UTC().Truncate(time.Second)
	return s.rewrite(ctx, id, func(k *kept, a *Artifact) error {
		a.Home = &home
		return nil
	})
}

// chosen is the version promoting an artifact chooses: n, or its latest
// for 0.
func chosen(id string, a *Artifact, n int) (int, error) {
	if n == 0 {
		latest, ok := a.Latest()
		if !ok {
			return 0, fmt.Errorf("%s has no versions to choose", id)
		}
		return latest.N, nil
	}
	if _, ok := a.Version(n); !ok {
		return 0, fmt.Errorf("%s has no v%d: %s", id, n, versionsSaid(*a))
	}
	return n, nil
}

// rewrite changes artifact id's record on this machine, as change says,
// and its row, in one transaction: the record's read again first (the
// file wins).
func (s Store) rewrite(ctx context.Context, id string, change func(*kept, *Artifact) error) (Entry, error) {
	db, err := s.records()
	if err != nil {
		return Entry{}, err
	}
	var out Entry
	err = db.Write(ctx, func(tx *store.Tx) error {
		k, err := keptIn(tx, id)
		if err != nil {
			return err
		}
		if k == nil {
			return notFound(id)
		}
		if k.Away() {
			return awayError(k.Entry)
		}
		dir := s.Path(k.Entry)
		data, err := os.ReadFile(filepath.Join(dir, RecordFile))
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s's folder has gone (%s)", id, dir)
		}
		if err != nil {
			return err
		}
		a, err := Parse(data)
		if err != nil {
			return fmt.Errorf("%s's record can't be read (%s): %w", id, filepath.Join(dir, RecordFile), err)
		}
		if err := change(k, a); err != nil {
			return err
		}
		info, err := writeRecord(dir, a)
		if err != nil {
			return err
		}
		k.Artifact = *a
		if err := saveIn(tx, k, info); err != nil {
			os.WriteFile(filepath.Join(dir, RecordFile), data, 0o644) // its row still says what it was
			return err
		}
		out = k.Entry
		return nil
	})
	return out, err
}

// Restore puts artifact id, discarded, back: its folder out of
// .ws/artifacts/.discarded, its record as it was before (live, or
// promoted if it had been), saying when it was put back, from when its
// expiry counts again.
func (s Store) Restore(ctx context.Context, id string, r Restoration) (Entry, error) {
	r.At = r.At.UTC().Truncate(time.Second)
	db, err := s.records()
	if err != nil {
		return Entry{}, err
	}
	var out Entry
	err = db.Write(ctx, func(tx *store.Tx) error {
		k, err := keptIn(tx, id)
		if err != nil {
			return err
		}
		if k == nil {
			return notFound(id)
		}
		if k.Away() {
			return awayError(k.Entry)
		}
		if k.Status != Discarded {
			return fmt.Errorf("%s isn't discarded: it's %s", id, k.Status)
		}
		from, to := filepath.Join(s.DiscardedDir(), k.Folder), filepath.Join(s.Dir(), k.Folder)
		was, err := os.ReadFile(filepath.Join(from, RecordFile))
		if errors.Is(err, os.ErrNotExist) {
			when := ""
			if k.Discarded != nil {
				when = fmt.Sprintf(" (it was discarded %s, and a folder's kept %d days)", k.Discarded.At.Local().Format("2006-01-02"), int(KeepDiscarded.Hours()/24))
			}
			return fmt.Errorf("%s's files are gone for good%s: there's nothing to restore. ws show %s says what it was", id, when, id)
		}
		if err != nil {
			return err
		}
		if _, err := os.Lstat(to); err == nil {
			return fmt.Errorf("there's a folder at %s already: move it aside, and restore %s again", to, id)
		}
		a, err := Parse(was)
		if err != nil {
			return fmt.Errorf("%s's record can't be read (%s): %w", id, filepath.Join(from, RecordFile), err)
		}
		a.Status, a.Discarded, a.Restored = Live, nil, &r
		if a.Promoted != nil {
			a.Status = Promoted
		}
		if _, err := writeRecord(from, a); err != nil {
			return err
		}
		undo := func() { os.WriteFile(filepath.Join(from, RecordFile), was, 0o644) }
		if err := os.MkdirAll(s.Dir(), 0o755); err != nil {
			undo()
			return err
		}
		if err := os.Rename(from, to); err != nil {
			undo()
			return err
		}
		info, err := os.Stat(filepath.Join(to, RecordFile))
		if err == nil {
			k.Artifact = *a
			err = saveIn(tx, k, info)
		}
		if err != nil {
			os.Rename(to, from) // its row still says it's discarded
			undo()
			return err
		}
		out = k.Entry
		return nil
	})
	return out, err
}

// Erase deletes for good the folders discarded before before (30 days
// ago, for the daemon): their rows stay, saying what they were. It's
// those it deleted.
func (s Store) Erase(ctx context.Context, before time.Time) ([]Entry, error) {
	db, err := s.records()
	if err != nil {
		return nil, err
	}
	var out []Entry
	err = db.Write(ctx, func(tx *store.Tx) error {
		all, err := allIn(tx)
		if err != nil {
			return err
		}
		for _, k := range all {
			if k.Away() || k.Status != Discarded || k.size < 0 || k.Discarded == nil || !k.Discarded.At.Before(before) {
				continue
			}
			dir := filepath.Join(s.DiscardedDir(), k.Folder)
			// Its record first: a folder with none left is litter, not an
			// artifact.
			if err := os.Remove(filepath.Join(dir, RecordFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			if err := saveIn(tx, k, nil); err != nil {
				return err
			}
			out = append(out, k.Entry)
		}
		return nil
	})
	return out, err
}
