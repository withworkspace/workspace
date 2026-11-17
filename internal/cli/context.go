package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// override is the workspace commands run in, when a test says
// (testutil.RunCommand), in place of loading the one they're run in.
var override *ops.Workspace

// SetOverride has commands run in w.
func SetOverride(w *ops.Workspace) { override = w }

// ClearOverride has commands load the workspace they're run in again.
func ClearOverride() { override = nil }

// ShellPWD is the directory the shell wrapper was standing in when it invoked
// us. The wrapper sets it for commands that step the shell out of the way
// before running (see `workspace shell-init`), so cwd-based resolution still
// reflects where the user actually is.
const ShellPWD = "WS_ORIGIN_PWD"

// userDir returns the directory to resolve relative to: the shell's own
// directory when the wrapper reported one, otherwise our working directory.
func userDir() (string, error) {
	if dir := os.Getenv(ShellPWD); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}
	return os.Getwd()
}

// userDirOr is userDir, or the working directory as "." when there's
// none to say: where a path you typed is read from.
func userDirOr() string {
	if dir, err := userDir(); err == nil {
		return dir
	}
	return "."
}

// loadOps is the workspace you're in (where the shell wrapper was
// standing, else the working directory), loaded for a command's run.
func loadOps() (*ops.Workspace, error) {
	if override != nil {
		return override, nil
	}
	cwd, err := userDir()
	if err != nil {
		return nil, err
	}
	return LoadOps(cwd)
}

// LoadOps is the workspace dir is in, loaded for a command's run: its
// config, repos and GitHub, with what cli gives ops (opsOptions), and
// more.
func LoadOps(dir string, more ...func(*ops.Options)) (*ops.Workspace, error) {
	cfg, ws, gh, err := ops.Load(dir)
	if err != nil {
		return nil, err
	}
	o := opsOptions(ws.Root)
	for _, f := range more {
		f(&o)
	}
	return ops.New(cfg, ws, gh, o), nil
}

// resolveRepo figures out which repo the user means.
// "." means infer from cwd (error if not in a repo).
// "" means try cwd, fall back to interactive picker.
// Anything else is a repo name lookup.
func resolveRepo(w *ops.Workspace, arg string) (string, error) {
	if arg != "" && arg != "." {
		// Exact name or alias match
		if canonical, ok := w.WS().ResolveAlias(arg); ok {
			return canonical, nil
		}
		// Fuzzy match
		matches := w.WS().FuzzyMatchRepos(arg)
		switch len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			return "", fmt.Errorf("unknown repo %q (not in ws.toml)", arg)
		default:
			return ui.PickRepo(matches, w.WS().DisplayNames)
		}
	}

	cwd, _ := userDir()
	if repo, _, ok := workspace.DetectRepo(w.WS().Root, cwd); ok {
		if _, exists := w.Config().Repos[repo]; exists {
			return repo, nil
		}
	}

	if arg == "." {
		return "", fmt.Errorf("not inside a known repo directory")
	}

	return ui.PickRepo(w.WS().DefaultRepoNames(), w.WS().DisplayNames)
}

// resolveCapsule figures out which capsule the user means within a repo.
// Does exact match, then fuzzy match. Single fuzzy match is used directly;
// multiple matches prompt an interactive picker; zero matches return an error.
func resolveCapsule(w *ops.Workspace, repo, arg string) (string, error) {
	capsules, err := workspace.ListWorktrees(w.WS().RepoDir(repo))
	if err != nil {
		return "", fmt.Errorf("listing capsules for %s: %w", repo, err)
	}

	// Exact match
	for _, name := range capsules {
		if name == arg {
			return name, nil
		}
	}

	// Fuzzy match
	var matches []string
	for _, name := range capsules {
		if workspace.FuzzyMatch(arg, name) {
			matches = append(matches, name)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("no capsule matching %q in %s", arg, repo)
	default:
		return ui.PickWorktree(matches)
	}
}

// resolveRepoStrict is resolveRepo for non-interactive callers such as agents.
// It accepts a canonical name, an alias, or a fuzzy pattern with exactly one
// match. Anything ambiguous is an error naming the candidates; it never opens
// a picker and never infers from cwd.
func resolveRepoStrict(w *ops.Workspace, arg string) (string, error) {
	if arg == "" || arg == "." {
		return "", fmt.Errorf("repo name required")
	}
	if canonical, ok := w.WS().ResolveAlias(arg); ok {
		return canonical, nil
	}
	matches := w.WS().FuzzyMatchRepos(arg)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("unknown repo %q (not in ws.toml)", arg)
	default:
		return "", fmt.Errorf("ambiguous repo %q: matches %s", arg, strings.Join(matches, ", "))
	}
}

// resolveWorktreeStrict resolves a worktree name within a repo without any
// interactive fallback. An existing directory name (including .ground and
// .silo) matches exactly; otherwise a fuzzy pattern must match exactly one
// capsule.
func resolveWorktreeStrict(w *ops.Workspace, repo, arg string) (string, error) {
	if arg == "" {
		return "", fmt.Errorf("worktree name required")
	}
	if arg == "." || arg == ".." || strings.ContainsAny(arg, `/\`) {
		return "", fmt.Errorf("invalid worktree name %q", arg)
	}
	if info, err := os.Stat(filepath.Join(w.WS().RepoDir(repo), arg)); err == nil && info.IsDir() {
		return arg, nil
	}
	worktrees, err := workspace.ListAllWorktrees(w.WS().RepoDir(repo))
	if err != nil {
		return "", fmt.Errorf("listing worktrees for %s: %w", repo, err)
	}
	var matches []string
	for _, name := range worktrees {
		if workspace.FuzzyMatch(arg, name) {
			matches = append(matches, name)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("no worktree matching %q in %s", arg, repo)
	default:
		return "", fmt.Errorf("ambiguous worktree %q in %s: matches %s", arg, repo, strings.Join(matches, ", "))
	}
}
