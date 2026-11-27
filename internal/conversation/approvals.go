package conversation

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
)

// A document is approved when you're happy with it: at the version it is
// then, so what's changed since shows. It's refused while a Change thread
// on it is open. Approvals are kept with the conversations, a row each in
// approvals, and said in the mission's log.

// Approval is a document approved, at a version.
type Approval struct {
	Doc     string    `json:"doc"` // the artifact's ref
	Version string    `json:"version"`
	By      string    `json:"by"`
	At      time.Time `json:"at"`
}

// keepApproval writes an approval's row.
func keepApproval(tx *store.Tx, scope string, a Approval) error {
	_, err := tx.Exec(`INSERT INTO approvals (scope, doc, version, by, at) VALUES (?, ?, ?, ?, ?)`, scope, a.Doc, a.Version, a.By, a.At.UTC().Format(time.RFC3339Nano))
	return err
}

// ErrChangesOpen is the answer for approving a document with Change
// threads open on it.
var ErrChangesOpen = errors.New("change requests still open")

// OpenChanges is the Change threads open on a document, oldest first.
func OpenChanges(threads []*Conversation, doc string) []string {
	var out []string
	for _, c := range threads {
		if c.Anchor.Doc == doc && c.Kind == Change && c.Status == Open {
			out = append(out, c.ID)
		}
	}
	return out
}

// Approve records you approving a document as it is now (text). It's
// refused while Change threads on it are open.
func (s Store) Approve(doc, path string, text []byte, by string, at time.Time) (Approval, error) {
	if by != You {
		return Approval{}, errors.New("only the user approves a document")
	}
	// The version's file is written before the row that names it.
	v, err := s.KeepVersion(text)
	if err != nil {
		return Approval{}, err
	}
	a := Approval{Doc: doc, Version: v, By: by, At: at}
	err = s.write(func(tx *store.Tx) error {
		threads, err := s.loadWhere(tx, `c.doc = ?`, doc)
		if err != nil {
			return err
		}
		if open := OpenChanges(threads, doc); len(open) > 0 {
			return fmt.Errorf("%w: %s %s for a change: resolve %s first, or decline the suggestion",
				ErrChangesOpen, strings.Join(open, ", "), map[bool]string{true: "asks", false: "ask"}[len(open) == 1], map[bool]string{true: "it", false: "them"}[len(open) == 1])
		}
		if err := keepApproval(tx, s.Slug, a); err != nil {
			return err
		}
		tx.Changed("workspace")
		if !s.Workspace() {
			tx.Changed("mission:" + s.Slug)
		}
		return nil
	})
	if err != nil {
		return Approval{}, err
	}
	name := path
	if name == "" {
		name = doc
	}
	return a, s.Missions.AppendLog(s.Slug, mission.Event{Time: at, Kind: mission.EventThread, Text: "approved " + name + " (version " + a.Version + ")"})
}

// Approvals is each document's latest approval, by its ref.
func (s Store) Approvals() (map[string]Approval, error) {
	out := map[string]Approval{}
	err := s.read(func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT doc, version, by, at FROM approvals WHERE scope = ? ORDER BY n`, s.Slug)
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
			out[a.Doc] = a
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Approved is a document's latest approval, or nil.
func (s Store) Approved(doc string) (*Approval, error) {
	all, err := s.Approvals()
	if err != nil {
		return nil, err
	}
	if a, ok := all[doc]; ok {
		return &a, nil
	}
	return nil, nil
}
