package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Ground is a repo's default branch as it's checked out in its .ground:
// the commit it's at, and where to read it. Agents read a repo's code and
// history there (or in a capsule), never from repos/<repo>/ itself, which
// isn't a repository.
type Ground struct {
	Repo, Path, Branch, Commit, Subject string
}

// Grounds is every repo's .ground, in the workspace's order.
func Grounds(ws *workspace.Workspace) []Ground {
	var out []Ground
	for _, name := range ws.RepoNames {
		dir := filepath.Join(ws.RepoDir(name), workspace.GroundDir)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		g := Ground{Repo: name, Path: dir, Branch: workspace.GitCurrentBranch(dir)}
		if g.Branch == "" {
			g.Branch = firstNonEmpty(ws.DefaultBranch, "main")
		}
		if data, err := git.Line(dir, "log", "-1", "--format=%h%x00%s"); err == nil {
			g.Commit, g.Subject, _ = strings.Cut(data, "\x00")
		}
		out = append(out, g)
	}
	return out
}

// String is a .ground in a line, as agents are told it.
func (g Ground) String() string {
	at := ""
	if g.Commit != "" {
		at = " at " + g.Commit
		if g.Subject != "" {
			at += fmt.Sprintf(" (%q)", g.Subject)
		}
	}
	return fmt.Sprintf("%s: %s%s, read-only at %s", g.Repo, g.Branch, at, g.Path)
}
