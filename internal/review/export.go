package review

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/withworkspace/workspace/internal/store"
)

// Export writes the reviews into dir as the files they were kept in
// before the records: r<n>.json each, and viewed.json, what was marked
// viewed in each capsule. It's how many reviews it wrote.
func (s Store) Export(dir string) (int, error) {
	db, err := s.records()
	if err != nil {
		return 0, err
	}
	reviews := map[string]json.RawMessage{}
	var ids []string
	viewed := map[string]map[string]string{}
	if err := db.Read(context.Background(), func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT id, review FROM reviews WHERE scope = ? ORDER BY n`, s.scope())
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, data string
			if err := rows.Scan(&id, &data); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
			reviews[id] = json.RawMessage(data)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = tx.Query(`SELECT capsule, path, blob FROM viewed WHERE scope = ?`, s.scope())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var capsule, path, blob string
			if err := rows.Scan(&capsule, &path, &blob); err != nil {
				return err
			}
			if viewed[capsule] == nil {
				viewed[capsule] = map[string]string{}
			}
			viewed[capsule][path] = blob
		}
		return rows.Err()
	}); err != nil {
		return 0, err
	}
	if len(ids) == 0 && len(viewed) == 0 {
		return 0, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	write := func(name string, v any) error {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, name), append(data, '\n'), 0o644)
	}
	for _, id := range ids {
		if err := write(id+".json", reviews[id]); err != nil {
			return 0, err
		}
	}
	if len(viewed) > 0 {
		if err := write("viewed.json", viewed); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}
