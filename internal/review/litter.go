package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
)

// Litter is what's in missions' folders of their reviews that no record
// has (ws doctor): a review rendered for crew (files/reviews/r2.md) whose
// review has no row, and the reviews/ folder a mission's reviews were kept
// in before the records, which nothing reads now.
func Litter(ctx context.Context, root string) ([]string, error) {
	ms := mission.Store{Root: root}
	slugs, err := ms.SlugsKept(ctx)
	if err != nil {
		return nil, err
	}
	db, err := For(root, "").records()
	if err != nil {
		return nil, err
	}
	kept := map[string]bool{} // slug/id
	if err := db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT scope, id FROM reviews`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var scope, id string
			if err := rows.Scan(&scope, &id); err != nil {
				return err
			}
			kept[scope+"/"+id] = true
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}
	var out []string
	for _, slug := range slugs {
		if old := filepath.Join(ms.Dir(), slug, "reviews"); isDir(old) {
			out = append(out, old)
		}
		rendered := filepath.Join(ms.FilesDir(slug), "reviews")
		entries, _ := os.ReadDir(rendered)
		for _, e := range entries {
			id, ok := strings.CutSuffix(e.Name(), ".md")
			if ok && idShape.MatchString(id) && !kept[slug+"/"+id] {
				out = append(out, filepath.Join(rendered, e.Name()))
			}
		}
	}
	return out, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
