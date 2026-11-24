package mission

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// A mission's brief and plan are files, which ws writes under a lock of
// the mission's (filesLock), held across processes: a write that's only to
// be made if a file is as it was read (WriteFilesIf) can't have another
// slip in between its look and its write. An editor opened on the file
// takes no lock: what it writes is the file as you left it.

// ErrChanged is a write refused as its file has changed since it was read.
var ErrChanged = errors.New("changed since it was read")

// ChangedError is which file a write's refused for (ErrChanged).
type ChangedError struct{ File string }

func (e *ChangedError) Error() string { return "the " + e.File + " has " + ErrChanged.Error() }
func (e *ChangedError) Unwrap() error { return ErrChanged }

// FileWrite is a mission's brief or plan, as it was read (Was) and as
// it's to be (Now): written only if it's still as it was, and not at all
// if Now is Was (it's only looked at).
type FileWrite struct {
	Plan     bool // the plan; else the brief
	Was, Now string
}

func (f FileWrite) name() string {
	if f.Plan {
		return "plan"
	}
	return "brief"
}

// WriteFilesIf writes a mission's brief and plan, each only if every one
// named is still as it was: else none is, refused with the first that's
// changed (ChangedError).
func (s Store) WriteFilesIf(slug string, writes ...FileWrite) error {
	if _, err := s.Load(slug); err != nil {
		return err
	}
	return s.withFiles(slug, func() error {
		for _, f := range writes {
			read := s.ReadBrief
			if f.Plan {
				read = s.ReadPlan
			}
			now, err := read(slug)
			if err != nil {
				return err
			}
			if now != f.Was {
				return &ChangedError{File: f.name()}
			}
		}
		for _, f := range writes {
			if f.Now == f.Was {
				continue
			}
			var err error
			if f.Plan {
				err = s.writePlan(slug, f.Now)
			} else {
				err = s.writeBrief(slug, f.Now)
			}
			if err != nil {
				return fmt.Errorf("writing the %s: %w", f.name(), err)
			}
		}
		return nil
	})
}

// withFiles runs fn holding the mission's file lock: an flock, so it's
// held against other processes and this one's other goroutines alike.
func (s Store) withFiles(slug string, fn func() error) error {
	// Kept out of the mission's folder, whose every file is the mission's.
	dir := filepath.Join(s.Root, ".ws", "locks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "mission-"+slug), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close() // which lets go of the lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	return fn()
}

func (s Store) writeBrief(slug, brief string) error {
	if brief != "" && !strings.HasSuffix(brief, "\n") {
		brief += "\n"
	}
	return writeFile(s.BriefPath(slug), []byte(brief))
}

func (s Store) writePlan(slug, plan string) error {
	if strings.TrimSpace(plan) == "" {
		if err := os.Remove(s.PlanPath(slug)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if !strings.HasSuffix(plan, "\n") {
		plan += "\n"
	}
	return writeFile(s.PlanPath(slug), []byte(plan))
}
