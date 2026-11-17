package ops

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/review"
)

// Exported is what writing a mission out as files wrote.
type Exported struct {
	Slug, Dir                     string
	Files, Conversations, Reviews int // files copied from its folder; records written as files
}

// ExportMission writes a mission into dir as the folder it was before the
// records were kept in SQLite: its own folder's files as they are (the
// brief, files/, crew's output, runs' transcripts, its document versions),
// and beside them its record, log, decisions, conversations and reviews as
// the files they were kept in. It's how a mission is read without ws, or
// handed to someone. dir mustn't hold anything yet.
func (w *Workspace) ExportMission(slug, dir string) (*Exported, error) {
	if _, err := w.Missions.Load(slug); err != nil {
		return nil, err
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return nil, &Invalid{Reason: dir + " has something in it already: export into a new folder"}
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	out := &Exported{Slug: slug, Dir: dir}
	from := filepath.Join(w.Missions.Dir(), slug)
	if err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && p == from {
			return filepath.SkipDir // a mission with no folder: its records alone
		}
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		if err := copyFile(p, filepath.Join(dir, rel)); err != nil {
			return fmt.Errorf("copying %s: %w", rel, err)
		}
		out.Files++
		return nil
	}); err != nil {
		return nil, err
	}
	if err := w.Missions.Export(slug, dir); err != nil {
		return nil, fmt.Errorf("writing its record: %w", err)
	}
	var err error
	if out.Conversations, err = conversation.For(w.Root(), slug).Export(filepath.Join(dir, "conversations")); err != nil {
		return nil, fmt.Errorf("writing its conversations: %w", err)
	}
	if out.Reviews, err = review.For(w.Root(), slug).Export(filepath.Join(dir, "reviews")); err != nil {
		return nil, fmt.Errorf("writing its reviews: %w", err)
	}
	return out, nil
}

// copyFile copies a file to to, making its folder.
func copyFile(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
