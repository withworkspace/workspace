package doc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Draft is an edit to a document that isn't saved yet: what you've
// written, and the version of the document it started from, to merge with
// whatever's there when it's saved.
//
// It's kept apart from the document's file: a half-made edit left in a
// capsule's working tree would go into crew's next commit as theirs.
type Draft struct {
	Ref  string    `json:"ref"`
	Base string    `json:"base"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// Drafts are a scope's drafts, one per document.
type Drafts struct {
	Root  string
	Scope Scope
}

func (d Drafts) path(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return filepath.Join(d.Scope.draftsDir(d.Root), hex.EncodeToString(sum[:8])+".json")
}

// Get is a document's draft, or nil if there's none.
func (d Drafts) Get(ref string) (*Draft, error) {
	if err := d.Scope.Check(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(d.path(ref))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var dr Draft
	if err := json.Unmarshal(data, &dr); err != nil {
		return nil, fmt.Errorf("the draft of %s: %w", ref, err)
	}
	if dr.Ref != ref {
		return nil, nil
	}
	return &dr, nil
}

// Save keeps a draft, in place of any before it. A mission's needs the
// mission.
func (d Drafts) Save(dr Draft) error {
	if err := d.Scope.Check(); err != nil {
		return err
	}
	if !d.Scope.IsWorkspace() {
		if _, err := os.Stat(missionDir(d.Root, d.Scope.Mission)); err != nil {
			return fmt.Errorf("%w: no mission %q", ErrNotFound, d.Scope.Mission)
		}
	}
	path := d.path(dr.Ref)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(dr, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".draft-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Drop forgets a document's draft, if there is one.
func (d Drafts) Drop(ref string) error {
	if err := d.Scope.Check(); err != nil {
		return err
	}
	if err := os.Remove(d.path(ref)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
