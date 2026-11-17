package ops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/git"
)

// The workspace's process: PROCESS.md in the config repo says how work
// goes here, in prose: the steps work takes, whose turn each is, and when
// it's the user's. It's the team's, changed by commit like the rest of
// the config. The planner reads it as it's written to plan a mission, and
// the orchestrator to know what to do when something happens; what ws
// itself holds whatever an agent reads is a guarantee, in ws.toml.

// ProcessView is the workspace's process as ws has it.
type ProcessView struct {
	// File is where it is, and Name that as it reads from the workspace's
	// root (PROCESS.md). Text is what it says, "" with no file.
	File, Name string
	Exists     bool
	Text       string
	// Commit is the config repo's last commit of it; Uncommitted when the
	// file isn't as committed (changed since, or never committed).
	Commit      string
	Uncommitted bool
}

// Process is the workspace's process.
func (w *Workspace) Process() (ProcessView, error) {
	file := w.WS().Process()
	v := ProcessView{File: file, Name: w.relToRoot(file)}
	data, err := os.ReadFile(file)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return v, nil
	case err != nil:
		return v, err
	}
	v.Exists, v.Text = true, string(data)
	if strings.TrimSpace(v.Text) != "" {
		v.Commit, v.Uncommitted = w.processCommit(file)
	}
	return v, nil
}

// ProcessText is the process as it's written, or "" when the workspace
// has none.
func (w *Workspace) ProcessText() string {
	v, err := w.Process()
	if err != nil {
		return ""
	}
	return v.Text
}

// relToRoot is a path as it reads from the workspace's root, if it's in
// it.
func (w *Workspace) relToRoot(path string) string {
	if r, err := filepath.Rel(w.Root(), path); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return path
}

// processCommit is the config repo's last commit of the process, and
// whether the file has changes it doesn't (or isn't in one at all).
func (w *Workspace) processCommit(file string) (string, bool) {
	dir := filepath.Dir(file)
	name := filepath.Base(file)
	commit, err := git.Line(dir, "log", "-1", "--format=%h", "--", name)
	if err != nil || commit == "" {
		return "", true
	}
	changed, err := git.Line(dir, "status", "--porcelain", "--", name)
	return commit, err != nil || changed != ""
}
