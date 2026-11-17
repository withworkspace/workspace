package artifact

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// An artifact's lifetime, as the store keeps it: promoted (kept for good),
// discarded and put back, and, on this machine, deleted for good 30 days
// after it was discarded; and moved from this machine to the repo the
// workspace keeps them in, promoted. In a repo, each is a commit (ops):
// what's here records what that commit did in the index.

// Restorable reports whether a discarded artifact can be put back: on this
// machine, while its folder's in .discarded; in a repo, from the commit
// history keeps it in.
func (e Entry) Restorable() bool {
	if e.Status != Discarded {
		return false
	}
	if e.InRepo() {
		return e.Discarded != nil && e.Discarded.Commit != ""
	}
	return !e.Erased
}

// Promote marks artifact id, on this machine, promoted: its record says so,
// with the version chosen (0: its latest) and what it went into, and it
// expires no more. One promoted already is promoted again, as said now.
func (s Store) Promote(ctx context.Context, id string, p Promotion) (Entry, error) {
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
		if k.InRepo() {
			return fmt.Errorf("%s is kept in %s, where it's changed in git, not here", id, k.Place)
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

// Restore puts artifact id, discarded on this machine, back: its folder
// out of .ws/artifacts/.discarded, its record as it was before (live, or
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
		if k.InRepo() {
			return fmt.Errorf("%s is kept in %s, where it's restored in git, not here", id, k.Place)
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

// Erase deletes for good the folders discarded on this machine before
// before (30 days ago, for the daemon): their rows stay, saying what they
// were. It's those it deleted.
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
			if k.InRepo() || k.Status != Discarded || k.size < 0 || k.Discarded == nil || !k.Discarded.At.Before(before) {
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

// MarkDiscarded records in the index that artifact id, kept in a repo,
// was discarded there: a commit took its folder away (d.Commit is the one
// before). Its row stays, saying what it was and where history keeps it.
func (s Store) MarkDiscarded(ctx context.Context, id string, d Discard) (Entry, error) {
	d.At = d.At.UTC().Truncate(time.Second)
	d.Reason = strings.TrimSpace(d.Reason)
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
		k.Status, k.Discarded = Discarded, &d
		if err := saveAs(tx, k, -1, k.changed); err != nil {
			return err
		}
		out = k.Entry
		return nil
	})
	return out, err
}

// Moving is an artifact on this machine on its way to a repo (promoted,
// and moved there): its row, and so its ref, already the repo's folder,
// and its folder here set aside, so nothing lists it twice meanwhile.
// Done deletes what was set aside, once it's landed; Undo puts both back.
type Moving struct {
	s                   Store
	id                  string
	name, aside         string // its folder's name here, and where it's set aside
	wasPlace, wasFolder string
}

// movingStamp is the changed of a row being moved to a repo: its folder
// isn't on the ground until it lands, and it isn't gone meanwhile.
const movingStamp = "moving"

// StartMove starts moving artifact id from this machine to folder (its
// path in the repo) in place (the repo's name).
func (s Store) StartMove(ctx context.Context, id, place, folder string) (*Moving, error) {
	db, err := s.records()
	if err != nil {
		return nil, err
	}
	var m *Moving
	err = db.Write(ctx, func(tx *store.Tx) error {
		k, err := keptIn(tx, id)
		if err != nil {
			return err
		}
		if k == nil {
			return notFound(id)
		}
		if k.InRepo() || k.Status == Discarded {
			return fmt.Errorf("%s isn't on this machine to move", id)
		}
		if other, err := keptAt(tx, place, folder); err != nil {
			return err
		} else if other != nil {
			return fmt.Errorf("%s:%s/ is %s's folder already", place, folder, other.ID)
		}
		from := filepath.Join(s.Dir(), k.Folder)
		aside := filepath.Join(s.Dir(), ".moving-"+k.Folder)
		if err := os.Rename(from, aside); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE artifacts SET place = ?, folder = ?, changed = ? WHERE id = ?`, place, folder, movingStamp, id); err != nil {
			os.Rename(aside, from)
			return err
		}
		tx.Changed("workspace", "artifact:"+id)
		m = &Moving{s: s, id: id, name: k.Folder, aside: aside, wasPlace: k.Place, wasFolder: k.Folder}
		return nil
	})
	return m, err
}

// Done finishes the move, once it's landed: what was set aside here is
// deleted.
func (m *Moving) Done() error {
	if err := os.Remove(filepath.Join(m.aside, RecordFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.RemoveAll(m.aside)
}

// Undo puts it back as it was, on this machine: it didn't land.
func (m *Moving) Undo(ctx context.Context) error {
	db, err := m.s.records()
	if err != nil {
		return err
	}
	return db.Write(ctx, func(tx *store.Tx) error {
		if _, err := tx.Exec(`UPDATE artifacts SET place = ?, folder = ? WHERE id = ?`, m.wasPlace, m.wasFolder, m.id); err != nil {
			return err
		}
		tx.Changed("workspace", "artifact:"+m.id)
		return os.Rename(m.aside, filepath.Join(m.s.Dir(), m.name))
	})
}
