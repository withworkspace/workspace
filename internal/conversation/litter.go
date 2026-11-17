package conversation

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
)

// Litter is what's in the conversations' folders that no record has (ws
// doctor): a run's transcript (w4.run) whose conversation has no row, and
// what a conversation was kept in before the records (its .jsonl, an
// .answering mark, approvals.jsonl, .lock), which nothing reads now. A
// mission's folders are those with a row: one without is the missions'
// litter, whole.
func Litter(ctx context.Context, root string) ([]string, error) {
	s := ForWorkspace(root)
	db, err := s.records()
	if err != nil {
		return nil, err
	}
	rows := map[string]map[string]bool{} // scope: ids
	var slugs []string
	if err := db.Read(ctx, func(tx *store.Tx) error {
		r, err := tx.Query(`SELECT scope, id FROM conversations`)
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var scope, id string
			if err := r.Scan(&scope, &id); err != nil {
				return err
			}
			if rows[scope] == nil {
				rows[scope] = map[string]bool{}
			}
			rows[scope][id] = true
		}
		if err := r.Err(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if slugs, err = (mission.Store{Root: root}).SlugsKept(ctx); err != nil {
		return nil, err
	}
	var out []string
	for _, scope := range append([]string{""}, slugs...) {
		dir := For(root, scope).dir()
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				continue
			}
			id, run := strings.CutSuffix(name, ".run")
			switch {
			case run && !rows[scope][id]:
			case strings.HasSuffix(name, ".jsonl"), strings.HasSuffix(name, ".answering"), name == ".lock":
			default:
				continue
			}
			out = append(out, filepath.Join(dir, name))
		}
	}
	return out, nil
}
