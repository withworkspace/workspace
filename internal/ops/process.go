package ops

import (
	"context"
	"errors"
	"fmt"
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
// has none. Only the file is read: what's written into a prompt doesn't
// ask git where it's committed.
func (w *Workspace) ProcessText() string {
	data, err := os.ReadFile(w.WS().Process())
	if err != nil {
		return ""
	}
	return string(data)
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
// whether the file has changes it doesn't (or isn't in one at all). It's
// kept until the repo moves (a commit, something staged) or the file's
// written: the page reads it each time it's opened.
func (w *Workspace) processCommit(file string) (string, bool) {
	dir, name := filepath.Dir(file), filepath.Base(file)
	ask := func() answer[processAt] {
		commit, err := git.Line(dir, "log", "-1", "--format=%h", "--", name)
		if err != nil || commit == "" {
			return asked(processAt{uncommitted: true}, err)
		}
		// Read only: a status that refreshes the index moves the repo,
		// and what's kept would be asked for again.
		changed, err := git.ReadLine(context.Background(), dir, "status", "--porcelain", "--", name)
		return asked(processAt{commit, err != nil || changed != ""}, err)
	}
	info, err := os.Stat(file)
	if gitDir, _ := gitDirs(dir); gitDir == "" || err != nil {
		p := ask().is // not at its repo's root: nothing says when it's moved
		return p.commit, p.uncommitted
	}
	p := kept(&w.git, dir, fmt.Sprintf("process %s %d %d", name, info.Size(), info.ModTime().UnixNano()), ask)
	return p.commit, p.uncommitted
}

// processAt is what git said of the process's file.
type processAt struct {
	commit      string
	uncommitted bool
}
