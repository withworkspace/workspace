package mission

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/withworkspace/workspace/internal/store"
)

// Export writes a mission's record into dir as the files it was kept in
// before the records: mission.toml, log.jsonl and decisions.jsonl, each
// line as the records hold it. It's how a record is read without ws, or
// handed to someone (ws export).
func (s Store) Export(slug, dir string) error {
	db, err := s.records()
	if err != nil {
		return err
	}
	var m *Mission
	var log, decisions []string
	if err := db.Read(context.Background(), func(tx *store.Tx) error {
		var err error
		if m, err = s.loadIn(tx, slug); err != nil {
			return err
		}
		if log, err = column(tx, `SELECT event FROM mission_log WHERE slug = ? ORDER BY n`, slug); err != nil {
			return err
		}
		decisions, err = column(tx, `SELECT line FROM decisions WHERE slug = ? ORDER BY n`, slug)
		return err
	}); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "mission.toml"), buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := WriteLines(filepath.Join(dir, "log.jsonl"), log); err != nil {
		return err
	}
	return WriteLines(filepath.Join(dir, "decisions.jsonl"), decisions)
}

// column is what a query of one column of text answers, in order.
func column(tx *store.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// WriteLines writes lines to path, one to a line: nothing, for none.
func WriteLines(path string, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
