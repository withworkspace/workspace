// Package doc is what a document is, apart from where it's discussed: its
// version, the versions of it kept to merge with and to show what changed,
// and the edits to it not yet saved (drafts). Each is kept in a Scope: a
// mission's, or the workspace's for a file in no mission.
package doc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrNotFound is a version that isn't kept, or a scope that isn't there.
var ErrNotFound = errors.New("not found")

// Version is a document's version: its text's hash, short.
func Version(text []byte) string {
	sum := sha256.Sum256(text)
	return hex.EncodeToString(sum[:])[:12]
}

// Scope is whose a document's versions and drafts are: a mission's, or
// (the zero Scope) the workspace's, for a file in no mission.
type Scope struct{ Mission string }

// Workspace is the workspace's own scope.
var Workspace = Scope{}

// OfMission is a mission's scope.
func OfMission(slug string) Scope { return Scope{Mission: slug} }

// IsWorkspace is whether it's the workspace's.
func (s Scope) IsWorkspace() bool { return s.Mission == "" }

func (s Scope) String() string {
	if s.IsWorkspace() {
		return "the workspace"
	}
	return "mission " + s.Mission
}

// slugShape is a mission's slug: a folder's name, never a way out of one
// (../..). It's mission's own rule, said again here, as doc is below it.
var slugShape = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Check refuses a scope whose mission can't be one: the slug comes from a
// URL, and is a folder's name on the way to what's kept.
func (s Scope) Check() error {
	if !s.IsWorkspace() && !slugShape.MatchString(s.Mission) {
		return fmt.Errorf("%w: no mission %q", ErrNotFound, s.Mission)
	}
	return nil
}

// missionDir is a mission's folder.
func missionDir(root, slug string) string { return filepath.Join(root, ".ws", "missions", slug) }

// versionsDir is where the scope's versions are kept: beside a mission's
// other files, or the workspace's in .ws/docs/versions.
func (s Scope) versionsDir(root string) string {
	if s.IsWorkspace() {
		return filepath.Join(root, ".ws", "docs", "versions")
	}
	return filepath.Join(missionDir(root, s.Mission), "docs")
}

// draftsDir is where the scope's drafts are kept.
func (s Scope) draftsDir(root string) string {
	if s.IsWorkspace() {
		return filepath.Join(root, ".ws", "docs", "drafts")
	}
	return filepath.Join(missionDir(root, s.Mission), "drafts")
}

// Versions are the versions of a scope's documents kept: each once, by its
// version, so what changed since can be shown and an edit merged.
type Versions struct {
	Root  string
	Scope Scope
}

// Keep keeps a document's text, once, and says its version.
func (v Versions) Keep(text []byte) (string, error) {
	ver := Version(text)
	path := filepath.Join(v.Scope.versionsDir(v.Root), ver+".md")
	if _, err := os.Stat(path); err == nil {
		return ver, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ver, err
	}
	return ver, os.WriteFile(path, text, 0o644)
}

// Text is a document's text at a kept version.
func (v Versions) Text(version string) ([]byte, error) {
	if len(version) != 12 || strings.Trim(version, "0123456789abcdef") != "" {
		return nil, fmt.Errorf("%w: no version %q", ErrNotFound, version)
	}
	data, err := os.ReadFile(filepath.Join(v.Scope.versionsDir(v.Root), version+".md"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: version %s is no longer kept", ErrNotFound, version)
	}
	return data, err
}
