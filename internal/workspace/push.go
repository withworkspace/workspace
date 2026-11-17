package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/git"
)

// PushBranch pushes a capsule's branch to its repo's remote, under the
// same name, and sets it to track it: what a pull request is opened from.
// It's never forced. quiet stops git asking for a password where there's
// no terminal to ask in. It says the branch, and what git said when it
// stopped.
func (w *Workspace) PushBranch(repo, capsule string, quiet bool) (branch, said string, err error) {
	dir := filepath.Join(w.RepoDir(repo), capsule)
	branch = GitCurrentBranch(dir)
	if branch == "" {
		return "", "", fmt.Errorf("%s/%s isn't on a branch, so there's nothing to open a pull request from", repo, capsule)
	}
	if branch == w.DefaultBranch {
		return "", "", fmt.Errorf("%s/%s is on %s, the default branch: a pull request is opened from a branch of its own", repo, capsule, branch)
	}
	var env []string
	if quiet {
		env = []string{"GIT_TERMINAL_PROMPT=0"}
	}
	out, err := git.Cmd{Dir: dir, Args: []string{"push", "--porcelain", "-u", "origin", "HEAD:refs/heads/" + branch}, Env: env}.Combined()
	if err != nil {
		return branch, strings.TrimSpace(out), fmt.Errorf("%s's remote refused the push of %s, and nothing was forced", repo, branch)
	}
	return branch, "", nil
}
