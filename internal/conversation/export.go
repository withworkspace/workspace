package conversation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
)

// Export writes the scope's conversations into dir as the files they were
// kept in before the records: <id>.jsonl each, its lines as the records
// hold them, and approvals.jsonl. A run's transcript is a file already,
// and isn't written here. It's how many it wrote.
func (s Store) Export(dir string) (int, error) {
	db, err := s.records()
	if err != nil {
		return 0, err
	}
	lines := map[string][]string{}
	var ids []string
	var approvals []string
	if err := db.Read(context.Background(), func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT id, line FROM conversation_lines WHERE scope = ? ORDER BY id, n`, s.Slug)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, line string
			if err := rows.Scan(&id, &line); err != nil {
				rows.Close()
				return err
			}
			if _, ok := lines[id]; !ok {
				ids = append(ids, id)
			}
			lines[id] = append(lines[id], line)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = tx.Query(`SELECT doc, version, by, at FROM approvals WHERE scope = ? ORDER BY n`, s.Slug)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a Approval
			var at string
			if err := rows.Scan(&a.Doc, &a.Version, &a.By, &at); err != nil {
				return err
			}
			a.At, _ = time.Parse(time.RFC3339Nano, at)
			data, err := json.Marshal(a)
			if err != nil {
				return err
			}
			approvals = append(approvals, string(data))
		}
		return rows.Err()
	}); err != nil {
		return 0, err
	}
	if len(ids) == 0 && len(approvals) == 0 {
		return 0, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := mission.WriteLines(filepath.Join(dir, id+".jsonl"), lines[id]); err != nil {
			return 0, err
		}
	}
	return len(ids), mission.WriteLines(filepath.Join(dir, "approvals.jsonl"), approvals)
}
