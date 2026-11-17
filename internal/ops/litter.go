package ops

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/team"
)

// Litter is a file or folder under .ws that no record has: Path is under
// .ws, What says whose.
type Litter struct{ Path, What string }

// Litter is what's under .ws that no record has, each store's own.
func (w *Workspace) Litter(ctx context.Context) ([]Litter, error) {
	root := w.Root()
	var out []Litter
	add := func(what string, paths []string, err error) error {
		for _, p := range paths {
			rel, _ := filepath.Rel(filepath.Join(root, ".ws"), p)
			out = append(out, Litter{Path: filepath.ToSlash(rel), What: what})
		}
		return err
	}
	paths, err := w.Missions.Litter(ctx)
	if err := add("missions'", paths, err); err != nil {
		return nil, err
	}
	paths, err = conversation.Litter(ctx, root)
	if err := add("conversations'", paths, err); err != nil {
		return nil, err
	}
	paths, err = review.Litter(ctx, root)
	if err := add("reviews'", paths, err); err != nil {
		return nil, err
	}
	paths, err = page.Store{Root: root}.Litter(ctx)
	if err := add("pages'", paths, err); err != nil {
		return nil, err
	}
	paths, err = w.Artifacts().Litter(ctx)
	if err := add("artifacts'", paths, err); err != nil {
		return nil, err
	}
	add("pull requests', from before the records", pulls.Litter(root), nil)
	add("Team's, from before the records", team.Litter(root), nil)
	if before := filepath.Join(root, ".ws", "before-db"); exists(before) {
		add("brought into the records once, and kept", []string{before}, nil)
	}
	return out, nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// SweptDir is where swept litter goes: .ws/swept/<date>/, each where it
// was under .ws.
func SweptDir(root string, at time.Time) string {
	return filepath.Join(root, ".ws", "swept", at.Format("2006-01-02"))
}

// Sweep moves litter aside, to SweptDir, each where it was under .ws: not
// deleted, so what was swept can be put back. It's how many it moved.
func (w *Workspace) Sweep(litter []Litter) (int, error) {
	to := SweptDir(w.Root(), w.now())
	n := 0
	for _, l := range litter {
		from := filepath.Join(w.Root(), ".ws", filepath.FromSlash(l.Path))
		dst := filepath.Join(to, filepath.FromSlash(l.Path))
		if exists(dst) {
			dst += "." + w.now().Format("150405")
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return n, err
		}
		if err := os.Rename(from, dst); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
