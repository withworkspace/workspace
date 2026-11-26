package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

func GitClone(url, target string) error {
	return git.Run("", "clone", url, target)
}

func GitCloneBare(url, bareDir string) error {
	if err := git.Run("", "clone", "--bare", url, bareDir); err != nil {
		return err
	}
	if err := git.Run(bareDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return err
	}
	if err := git.Run(bareDir, "config", "push.autoSetupRemote", "true"); err != nil {
		return err
	}
	// Populate remote tracking refs now that the refspec is configured.
	return git.Run(bareDir, "fetch", "origin")
}

func GitFetch(repoDir string) error {
	return git.Run(repoDir, "fetch", "--all", "--prune")
}

func GitFetchOrigin(dir string) error {
	return git.Run(dir, "fetch", "origin")
}

func GitResetHard(dir, ref string) error {
	return git.Run(dir, "reset", "--hard", ref)
}

// GitFFMerge fast-forwards the current branch to the given ref.
func GitFFMerge(dir, ref string) error {
	return git.Run(dir, "merge", "--ff-only", ref)
}

// GitWorktreeAddBranch checks out an existing branch into a new worktree.
func GitWorktreeAddBranch(gitDir, path, branch string) error {
	return git.Run(gitDir, "worktree", "add", path, branch)
}

// GitWorktreeAddDetached creates a worktree with a detached HEAD at the given ref.
func GitWorktreeAddDetached(gitDir, path, ref string) error {
	return git.Run(gitDir, "worktree", "add", "--detach", path, ref)
}

// GitSetUpstream sets the upstream tracking branch for a local branch.
// The remote tracking ref must already exist.
func GitSetUpstream(gitDir, branch, remote string) error {
	return git.Run(gitDir, "branch", "--set-upstream-to="+remote+"/"+branch, branch)
}

// GitConfigBranchUpstream sets the upstream remote and merge ref for a branch
// via git config, without requiring the remote tracking ref to already exist.
func GitConfigBranchUpstream(gitDir, branch, remote string) error {
	if err := git.Run(gitDir, "config", "branch."+branch+".remote", remote); err != nil {
		return err
	}
	return git.Run(gitDir, "config", "branch."+branch+".merge", "refs/heads/"+branch)
}

// GitWorktreeAddNewBranch creates a new branch from base and checks it out into a new worktree.
func GitWorktreeAddNewBranch(gitDir, path, branch, base string) error {
	return git.Run(gitDir, "worktree", "add", path, "-b", branch, base)
}

func GitWorktreeRemove(gitDir, path string) error {
	return git.Run(gitDir, "worktree", "remove", path)
}

func GitWorktreeRemoveForce(gitDir, path string) error {
	return git.Run(gitDir, "worktree", "remove", "--force", path)
}

func GitIsDirty(dir string) bool {
	out, err := git.Combined(dir, "status", "--porcelain")
	return err == nil && strings.TrimSpace(out) != ""
}

// GitDirtyCount returns the number of uncommitted files (modified, staged, untracked).
func GitDirtyCount(dir string) int {
	out, err := git.Combined(dir, "status", "--porcelain")
	if err != nil {
		return 0
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

func GitCurrentBranch(dir string) string {
	out, err := git.Combined(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// GitHasUpstream reports whether HEAD's branch has an upstream that's
// there: a capsule's branch tracks origin/<itself>, which isn't there
// until it's first pushed.
func GitHasUpstream(dir string) bool {
	_, err := git.Combined(dir, "rev-parse", "--verify", "--quiet", "@{u}")
	return err == nil
}

// GitMergeConflicts says whether HEAD in dir merges cleanly onto base,
// without touching the worktree (git merge-tree): the files that would
// conflict, none when it merges cleanly.
func GitMergeConflicts(dir, base string) ([]string, error) {
	out, err := git.Output(dir, "merge-tree", "--write-tree", "--name-only", "--no-messages", base, "HEAD")
	switch {
	case err == nil:
		return nil, nil
	case git.ExitCode(err) != 1:
		return nil, err
	}
	// Conflicts: the tree it would write, then a file a line.
	var files []string
	for i, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if i > 0 && strings.TrimSpace(line) != "" {
			files = append(files, strings.TrimSpace(line))
		}
	}
	return files, nil
}

func GitAheadBehind(dir string) (int, int) {
	out, err := git.Combined(dir, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	if err != nil {
		return 0, 0
	}
	var ahead, behind int
	_, _ = fmt.Sscanf(strings.TrimSpace(out), "%d\t%d", &ahead, &behind) // what can't be read is none
	return ahead, behind
}

// GitCommitsSince counts commits reachable from HEAD but not from base.
// Returns 0 if base is unknown or the command fails.
func GitCommitsSince(dir, base string) int {
	out, err := git.Combined(dir, "rev-list", "--count", base+"..HEAD")
	if err != nil {
		return 0
	}
	var n int
	_, _ = fmt.Sscanf(strings.TrimSpace(out), "%d", &n) // what can't be read is none
	return n
}

// GitCommitsBehindRef counts commits on ref that are not reachable from HEAD.
// Returns 0 if ref is unknown or the command fails.
func GitCommitsBehindRef(dir, ref string) int {
	out, err := git.Combined(dir, "rev-list", "--count", "HEAD.."+ref)
	if err != nil {
		return 0
	}
	var n int
	_, _ = fmt.Sscanf(strings.TrimSpace(out), "%d", &n) // what can't be read is none
	return n
}

func RepoCloneURL(org, name, gitProtocol string) string {
	if gitProtocol == "ssh" {
		return fmt.Sprintf("git@github.com:%s/%s.git", org, name)
	}
	return fmt.Sprintf("https://github.com/%s/%s.git", org, name)
}

func ListWorktrees(repoDir string) ([]string, error) {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// ListAllWorktrees returns all worktree directories including .ground.
// .ground is always first when present.
func ListAllWorktrees(repoDir string) ([]string, error) {
	capsules, err := ListWorktrees(repoDir)
	if err != nil {
		return nil, err
	}

	groundDir := filepath.Join(repoDir, GroundDir)
	if info, err := os.Stat(groundDir); err == nil && info.IsDir() {
		return append([]string{GroundDir}, capsules...), nil
	}

	return capsules, nil
}

// DisableWorkspaceGit puts the root's git away (.git-disabled), so nothing
// in the workspace mistakes the root for a repo of its own.
func DisableWorkspaceGit(root string) error {
	gitDir := filepath.Join(root, ".git")
	disabled := filepath.Join(root, ".git-disabled")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		if err := os.Rename(gitDir, disabled); err != nil {
			return fmt.Errorf("putting the workspace's git away: %w", err)
		}
	}
	return nil
}

// EnableWorkspaceGit brings the root's git back out. If it can't, git run
// in the root would find whatever repo is above it, so it's an error.
func EnableWorkspaceGit(root string) error {
	gitDir := filepath.Join(root, ".git")
	disabled := filepath.Join(root, ".git-disabled")
	if info, err := os.Stat(disabled); err == nil && info.IsDir() {
		if err := os.Rename(disabled, gitDir); err != nil {
			return fmt.Errorf("bringing the workspace's git out: %w", err)
		}
	}
	return nil
}

// GitRecentCommits returns the last n commits as one-line summaries.
// If baseBranch is non-empty and differs from the current branch, only
// commits not in baseBranch are shown (i.e. the capsule's own work).
func GitRecentCommits(dir string, n int, baseBranch string) []string {
	rangeSpec := ""
	if baseBranch != "" {
		cur := GitCurrentBranch(dir)
		if cur != "" && cur != baseBranch {
			rangeSpec = baseBranch + "..HEAD"
		}
	}

	args := []string{"log", "--oneline", "-n", fmt.Sprintf("%d", n)}
	if rangeSpec != "" {
		args = append(args, rangeSpec)
	}

	out, err := git.Combined(dir, args...)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// GitDiffStat returns the --stat output for uncommitted changes.
func GitDiffStat(dir string) string {
	out, err := git.Combined(dir, "diff", "--stat")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// GitStashCount returns the number of stash entries.
func GitStashCount(dir string) int {
	out, err := git.Combined(dir, "stash", "list")
	if err != nil || strings.TrimSpace(out) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(out), "\n"))
}

// GitLastCommitDate returns the author date of the most recent commit.
func GitLastCommitDate(dir string) time.Time {
	out, err := git.Combined(dir, "log", "-1", "--format=%aI")
	if err != nil {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, strings.TrimSpace(out))
	return t
}

// GitMergedBranches returns branch names that are fully merged into base.
func GitMergedBranches(dir, base string) []string {
	out, err := git.Combined(dir, "branch", "--merged", base)
	if err != nil {
		return nil
	}
	var branches []string
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		// git branch prefixes: "* " for current, "+ " for worktree-linked
		name := strings.TrimSpace(line)
		name = strings.TrimPrefix(name, "* ")
		name = strings.TrimPrefix(name, "+ ")
		if name != "" {
			branches = append(branches, name)
		}
	}
	return branches
}

// GitRevParse resolves a ref to its commit SHA.
func GitRevParse(dir, ref string) string {
	out, err := git.Combined(dir, "rev-parse", ref)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
