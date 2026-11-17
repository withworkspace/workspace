package store

import (
	"crypto/sha1"
	"database/sql/driver"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// What crosses store.sock: newline-delimited JSON, a request then its
// reply, one at a time on a connection. A connection opens with hello,
// and holds at most one transaction.
//
//	hello     layouts                → layouts, pid (or an error: they differ)
//	begin     write                  → (a write waits for its turn)
//	exec      sql, args              → affected, last
//	query     sql, args              → cols, rows
//	commit, rollback                 → (ends the transaction)
//	check                            → found: what SQLite's check of the file finds wrong
//
// exec and query outside a transaction are one statement each, a write
// taking its turn.
type request struct {
	Op      string         `json:"op"`
	Layouts map[string]int `json:"layouts,omitempty"`
	Write   bool           `json:"write,omitempty"`
	SQL     string         `json:"sql,omitempty"`
	Args    []value        `json:"args,omitempty"`
}

type reply struct {
	Error string `json:"error,omitempty"`
	// Code is SQLite's code for the error, when it's SQLite's.
	Code int `json:"code,omitempty"`

	Layouts  map[string]int `json:"layouts,omitempty"`
	Pid      int            `json:"pid,omitempty"`
	Cols     []string       `json:"cols,omitempty"`
	Rows     [][]value      `json:"rows,omitempty"`
	Affected int64          `json:"affected,omitempty"`
	Last     int64          `json:"last,omitempty"`
	Found    []string       `json:"found,omitempty"`
}

// value is one of the values a statement takes or a row holds (what
// database/sql's drivers deal in), said so its kind survives JSON: null,
// or [kind, what], kind one of i (an integer), f (a real), s (text), x
// (bytes, in base64), b (a bool) or t (a time).
type value struct{ v driver.Value }

func (v value) MarshalJSON() ([]byte, error) {
	switch x := v.v.(type) {
	case nil:
		return []byte("null"), nil
	case int64:
		return json.Marshal([]any{"i", x})
	case float64:
		return json.Marshal([]any{"f", x})
	case string:
		return json.Marshal([]any{"s", x})
	case []byte:
		return json.Marshal([]any{"x", base64.StdEncoding.EncodeToString(x)})
	case bool:
		return json.Marshal([]any{"b", x})
	case time.Time:
		return json.Marshal([]any{"t", x.Format(time.RFC3339Nano)})
	}
	return nil, fmt.Errorf("store: can't send a %T", v.v)
}

func (v *value) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		v.v = nil
		return nil
	}
	var pair [2]json.RawMessage
	if err := json.Unmarshal(data, &pair); err != nil {
		return err
	}
	var kind string
	if err := json.Unmarshal(pair[0], &kind); err != nil {
		return err
	}
	switch kind {
	case "i":
		var x int64
		err := json.Unmarshal(pair[1], &x)
		v.v = x
		return err
	case "f":
		var x float64
		err := json.Unmarshal(pair[1], &x)
		v.v = x
		return err
	case "s":
		var x string
		err := json.Unmarshal(pair[1], &x)
		v.v = x
		return err
	case "x":
		var s string
		if err := json.Unmarshal(pair[1], &s); err != nil {
			return err
		}
		x, err := base64.StdEncoding.DecodeString(s)
		v.v = x
		return err
	case "b":
		var x bool
		err := json.Unmarshal(pair[1], &x)
		v.v = x
		return err
	case "t":
		var s string
		if err := json.Unmarshal(pair[1], &s); err != nil {
			return err
		}
		x, err := time.Parse(time.RFC3339Nano, s)
		v.v = x
		return err
	}
	return fmt.Errorf("store: a value of kind %q", kind)
}

// Path is where a workspace's records are kept.
func Path(root string) string { return filepath.Join(root, ".ws", "ws.db") }

// lockPath is the file the daemon holds locked while it owns the records,
// with its process id in it.
func lockPath(root string) string { return filepath.Join(root, ".ws", "store.lock") }

// maxSocketPath is under the smallest sun_path limit ws runs on (104, on
// macOS), as the daemon's own socket's.
const maxSocketPath = 100

// SocketPath is where the daemon serves the records: beside its own
// socket, or in the temporary folder when the workspace's path is too long
// for one (as daemon.SocketPath has it).
func SocketPath(root string) string {
	p := filepath.Join(root, ".ws", "store.sock")
	if len(p) <= maxSocketPath {
		return p
	}
	sum := sha1.Sum([]byte(root))
	return filepath.Join(os.TempDir(), "ws-store-"+hex.EncodeToString(sum[:6])+".sock")
}
