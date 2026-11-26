package ops

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The repos a workspace has: the team's, in ws.toml, and yours, in
// ws.local.toml alone. Yours is cloned and worked in as the team's are;
// sharing one gives it to the team.

// RepoShared is a repo of yours given to the team: the commit that put it
// in ws.toml, and the branch of the workspace's config repo it went to.
type RepoShared struct {
	Repo, Commit, Branch string
}

func (r RepoShared) String() string {
	return fmt.Sprintf("%s is the team's now: in ws.toml, pushed to %s (%s)", r.Repo, r.Branch, r.Commit)
}

// ShareRepo gives a repo of yours to the team: it's written into ws.toml,
// as ws.local.toml says it, committed on the workspace's config repo and
// pushed, and only then taken out of ws.local.toml. ws sync resets the
// config repo to its remote, so a commit that isn't pushed would be lost:
// one the remote refuses is taken back, and the repo stays yours. The
// config repo has to be where its remote is, or behind it with nothing of
// its own, to be committed to.
func (w *Workspace) ShareRepo(ctx context.Context, who Actor, name string) (*RepoShared, error) {
	if !who.IsYou() {
		return nil, refused("not-yours", "giving a repo to the team changes the team's ws.toml: it's the user's to do")
	}
	ws := w.WS()
	repo, ok := ws.ResolveAlias(name)
	if !ok {
		return nil, fmt.Errorf("%w: there's no repo called %s", ErrNotFound, name)
	}
	if !ws.Yours[repo] {
		return nil, invalid("%s is the team's already: it's in ws.toml", repo)
	}
	local, err := config.ReadLocal(w.root)
	if err != nil {
		return nil, err
	}
	rc := local.Repos[repo]

	commit, branch, err := w.teamRepo(ctx, repo, rc, fmt.Sprintf("Given to the team from %s's ws.local.toml.", gitUser(ctx, w.root)), "so it stays yours")
	if err != nil {
		return nil, err
	}
	// ws.toml has it now: what ws.local.toml said of it is the team's
	// whole, so it goes.
	if err := config.DropLocalRepo(w.root, repo); err != nil {
		return nil, fmt.Errorf("%s is in ws.toml (pushed, %s), but couldn't be taken out of ws.local.toml: %w", repo, commit, err)
	}
	return &RepoShared{Repo: repo, Commit: commit, Branch: branch}, nil
}

// teamRepo writes a repo into ws.toml, as rc says it, and commits that on
// the workspace's config repo (Add repo <name>, with why in its body) and
// pushes it: the commit and the branch it went to. ws sync resets the
// config repo to its remote, so a commit that isn't pushed would be lost:
// one the remote refuses is taken back, and the error says what that
// leaves (unpushed). The config repo has to be where its remote is, or
// behind it with nothing of its own, to be committed to.
func (w *Workspace) teamRepo(ctx context.Context, repo string, rc config.RepoConfig, why, unpushed string) (commit, branch string, err error) {
	root := w.root
	hidden := dirThere(filepath.Join(root, ".git-disabled"))
	if !hidden && !dirThere(filepath.Join(root, ".git")) {
		return "", "", refused("no-config-repo", "the workspace's config isn't a git repo, so there's nowhere to give it to the team: add [repos.%s] to ws.toml by hand", repo)
	}
	if hidden {
		// Its git is put away between syncs, so nothing in the workspace
		// mistakes the root for a repo of its own: out for this, and back.
		if err := workspace.EnableWorkspaceGit(root); err != nil {
			return "", "", err
		}
		defer func() {
			// What's given to the team stands if it can't be put away again.
			if err := workspace.DisableWorkspaceGit(root); err != nil {
				log.Printf("%v", err)
			}
		}()
	}

	branch, err = git.ReadLine(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", "", refused("config-detached", "the workspace's config repo isn't on a branch: run ws sync, then try again")
	}
	if dirty, _ := git.ReadLine(ctx, root, "status", "--porcelain", "--", config.FileName); dirty != "" {
		return "", "", refused("config-dirty", "ws.toml has changes that aren't committed: commit or undo them first")
	}
	if _, err := (git.Cmd{Ctx: ctx, Dir: root, Args: []string{"fetch", "--quiet", "origin", branch}}).Output(); err != nil {
		return "", "", &Upstream{Err: fmt.Errorf("fetching the workspace's config: %w", err)}
	}
	upstream := "origin/" + branch
	ahead, _ := git.ReadLine(ctx, root, "rev-list", "--count", upstream+"..HEAD")
	if ahead != "0" {
		return "", "", refused("config-ahead", "the workspace's config repo has commits that aren't on %s: push or drop them first (ws sync resets it to its remote)", upstream)
	}
	// Behind with nothing of its own: brought up to its remote first, as
	// ws sync would.
	if _, err := (git.Cmd{Ctx: ctx, Dir: root, Args: []string{"merge", "--quiet", "--ff-only", upstream}}).Output(); err != nil {
		return "", "", refused("config-behind", "the workspace's config repo can't be brought up to %s: run ws sync, then try again", upstream)
	}
	before, err := git.ReadLine(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}

	if err := config.AddTeamRepo(root, repo, rc); err != nil {
		return "", "", err
	}
	undo := func() {
		(git.Cmd{Ctx: ctx, Dir: root, Args: []string{"reset", "--quiet", before}}).Output()                   //nolint:errcheck // taking it back as far as it goes
		(git.Cmd{Ctx: ctx, Dir: root, Args: []string{"checkout", "--quiet", "--", config.FileName}}).Output() //nolint:errcheck
	}
	msg := fmt.Sprintf("Add repo %s\n\n%s", repo, why)
	if _, err := (git.Cmd{Ctx: ctx, Dir: root, Args: []string{"commit", "--quiet", "-m", msg, "--", config.FileName}}).Output(); err != nil {
		undo()
		return "", "", fmt.Errorf("committing ws.toml: %w", err)
	}
	if _, err := (git.Cmd{Ctx: ctx, Dir: root, Args: []string{"push", "--quiet", "origin", "HEAD:" + branch}}).Output(); err != nil {
		undo()
		return "", "", &Upstream{Err: fmt.Errorf("pushing ws.toml to %s, %s: %w", upstream, unpushed, err)}
	}
	commit, _ = git.ReadLine(ctx, root, "rev-parse", "--short", "HEAD")
	return commit, branch, nil
}

// gitUser is who commits in dir, by name: "you" where git doesn't say.
func gitUser(ctx context.Context, dir string) string {
	if name, err := git.ReadLine(ctx, dir, "config", "user.name"); err == nil && strings.TrimSpace(name) != "" {
		return name
	}
	return "you"
}
