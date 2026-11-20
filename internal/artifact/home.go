package artifact

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/withworkspace/workspace/internal/store"
)

// An artifact's home in a repo: what's written there when it's promoted
// (WriteHome), and bringing onto this machine the ones that were kept in
// a repo whole, before they all lived here (BringHome).

// WriteHome writes an artifact as its home in a repo has it, in dir (its
// folder there, in a worktree): the version chosen, its page (HomeFile)
// and the files beside it, each at its path, and a record of it
// (a.AtHome). Whatever was in the folder goes: it holds one version, the
// one chosen now.
func WriteHome(dir string, a *Artifact, p Promotion, files Files) error {
	if _, ok := files.Index(); !ok {
		return fmt.Errorf("v%d has no %s", p.Version, IndexFile)
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := files.WriteFolder(dir); err != nil {
		return err
	}
	data, err := Encode(a.AtHome(p))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, RecordFile), data, 0o644)
}

// BringHome brings onto this machine each artifact that was kept in a
// repo whole, from before they all lived here: its folder as the repo's
// ground has it (its record, and its versions' files) is copied into
// .ws/artifacts, and its row is this machine's from then on, its ref the
// same. One that was promoted lives where it was (its Home), at the
// version chosen; one that wasn't is this machine's only, and its old
// folder is left in the repo, yours to remove. One whose folder isn't on
// the ground (discarded there, or never landed) is this machine's too,
// with no files: what's known of it is what the records kept.
//
// ground is where a repo's default branch is checked out, by the repo's
// name ("" for one the workspace hasn't). It's those it brought.
func (s Store) BringHome(ctx context.Context, ground func(repo string) string) ([]Entry, error) {
	db, err := s.records()
	if err != nil {
		return nil, err
	}
	var away []*kept
	if err := db.Read(ctx, func(tx *store.Tx) error {
		all, err := allIn(tx)
		for _, k := range all {
			if k.Away() {
				away = append(away, k)
			}
		}
		return err
	}); err != nil || len(away) == 0 {
		return nil, err
	}
	var out []Entry
	err = db.Write(ctx, func(tx *store.Tx) error {
		for _, was := range away {
			k, err := keptIn(tx, was.ID)
			if err != nil {
				return err
			}
			if k == nil || !k.Away() {
				continue // brought since
			}
			repo, folder := k.Place, k.Folder
			name, err := s.freeName(tx, path.Base(folder))
			if err != nil {
				return err
			}
			var info os.FileInfo
			if g := ground(repo); g != "" && k.Status != Discarded {
				src := filepath.Join(g, filepath.FromSlash(folder))
				if data, err := os.ReadFile(filepath.Join(src, RecordFile)); err == nil {
					a, err := Parse(data)
					if err != nil {
						continue // can't be read: left where it is, to be put right
					}
					if a.Status == Promoted && a.Promoted != nil {
						a.Home = &Home{Repo: repo, Folder: folder, Version: a.Promoted.Version, At: a.Promoted.At}
					}
					dst := filepath.Join(s.Dir(), name)
					if err := copyFiles(src, dst); err != nil {
						os.RemoveAll(dst)
						return fmt.Errorf("bringing %s from %s:%s/: %w", k.ID, repo, folder, err)
					}
					if info, err = writeRecord(dst, a); err != nil {
						os.RemoveAll(dst)
						return err
					}
					k.Artifact = *a
				}
			}
			if _, err := tx.Exec(`UPDATE artifacts SET place = ?, folder = ? WHERE id = ?`, Local, name, k.ID); err != nil {
				return err
			}
			k.Place, k.Folder = Local, name
			if err := saveIn(tx, k, info); err != nil {
				return err
			}
			out = append(out, k.Entry)
		}
		return nil
	})
	return out, err
}

// freeName is a folder's name in .ws/artifacts that nothing has: name, or
// name-2, name-3.
func (s Store) freeName(tx *store.Tx, name string) (string, error) {
	for i := 1; i <= 100; i++ {
		n := name
		if i > 1 {
			n = name + "-" + strconv.Itoa(i)
		}
		if k, err := keptAt(tx, Local, n); err != nil {
			return "", err
		} else if k != nil {
			continue
		}
		taken := false
		for _, dir := range []string{s.Dir(), s.DiscardedDir()} {
			if _, err := os.Lstat(filepath.Join(dir, n)); err == nil {
				taken = true
			}
		}
		if !taken {
			return n, nil
		}
	}
	return "", fmt.Errorf("couldn't name a folder for %s in %s", name, s.Dir())
}

// copyFiles copies a folder's files (not its folders) to to, which
// mustn't be there.
func copyFiles(from, to string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(to); err == nil {
		return errors.New(to + " is there already")
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(to, e.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
