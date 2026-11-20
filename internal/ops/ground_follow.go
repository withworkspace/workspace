package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Each repo's ground (.ground) is its default branch as the remote has
// it, read-only by rule: what capsules are lifted from, what refs and
// documents are read from. The daemon keeps every ground up to date by
// itself (FollowGrounds), so none sits days behind a branch that's moved
// on GitHub.

// GroundFollowed is what following a repo's ground did.
type GroundFollowed struct {
	Repo string
	// Fetched is whether the repo's remote was fetched: only when its
	// branch had moved there, or GitHub couldn't say where it is.
	Fetched bool
	// Moved is how many commits the ground moved forward, to At.
	Moved int
	At    string
	// Held is why a ground that's behind wasn't moved: a refusal coded
	// ground-dirty, ground-ahead, ground-branch or ground-pull.
	Held *Refused
	Err  error
}

// FollowGrounds brings each repo's ground up to its remote's default
// branch: it asks GitHub where every repo's branch is in one request,
// fetches only those that have moved from what was last fetched (each,
// when GitHub can't say), then fast-forwards each ground that's behind
// and has nothing of its own. A repo that isn't cloned here is passed by.
func (w *Workspace) FollowGrounds(ctx context.Context) []GroundFollowed {
	ws := w.WS()
	branch := firstNonEmpty(ws.DefaultBranch, "main")
	var repos []string
	for _, repo := range ws.RepoNames {
		if dirThere(ws.MainWorktree(repo)) && dirThere(ws.BareDir(repo)) {
			repos = append(repos, repo)
		}
	}
	if len(repos) == 0 {
		return nil
	}
	var heads map[string]string // nil: GitHub can't say
	if hc, ok := w.GitHub().(github.HeadsClient); ok && ws.Org != "" {
		heads, _ = hc.Heads(ws.Org, repos, branch) // what answered, even beside an error
	}
	out := make([]GroundFollowed, 0, len(repos))
	for _, repo := range repos {
		if ctx.Err() != nil {
			break
		}
		f := GroundFollowed{Repo: repo}
		if remote, ok := heads[repo]; !ok || remote != w.fetchedHead(ctx, repo, branch) {
			f.Fetched = true
			if f.Err = w.fetchGround(ctx, repo); f.Err != nil {
				out = append(out, f)
				continue
			}
		}
		before := w.groundFresh(ctx, repo)
		if before.Behind == 0 && before.Ahead == 0 {
			out = append(out, f)
			continue
		}
		after, _, err := w.pullGround(ctx, repo)
		var r *Refused
		switch {
		case errors.As(err, &r):
			f.Held = r
		case err != nil:
			f.Err = err
		default:
			f.Moved = before.Behind - after.Behind
			f.At, _ = git.ReadLine(ctx, ws.MainWorktree(repo), "rev-parse", "--short", "HEAD")
		}
		out = append(out, f)
	}
	return out
}

// fetchedHead is the commit the repo's branch was at on its remote, as
// last fetched: "" for never.
func (w *Workspace) fetchedHead(ctx context.Context, repo, branch string) string {
	oid, err := git.ReadLine(ctx, w.WS().BareDir(repo), "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	if err != nil {
		return ""
	}
	return oid
}

// fetchGround brings the repo's default branch up to date in its clone,
// and nothing else: not its tags, not its other branches. The ground
// isn't moved (pullGround does that).
func (w *Workspace) fetchGround(ctx context.Context, repo string) error {
	branch := firstNonEmpty(w.WS().DefaultBranch, "main")
	if _, err := (git.Cmd{Ctx: ctx, Dir: w.WS().BareDir(repo), Args: []string{"fetch", "--quiet", "--no-tags", "origin", fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch)}}).Output(); err != nil {
		return &Upstream{Err: err}
	}
	return nil
}

// groundFresh is how the repo's ground stands with its remote, as last
// fetched.
func (w *Workspace) groundFresh(ctx context.Context, repo string) SpaceFresh {
	ws := w.WS()
	f := SpaceFresh{Branch: firstNonEmpty(ws.DefaultBranch, "main")}
	if info, err := os.Stat(filepath.Join(ws.BareDir(repo), "FETCH_HEAD")); err == nil {
		f.Fetched = info.ModTime()
	}
	ground := ws.MainWorktree(repo)
	if counts, err := git.ReadLine(ctx, ground, "rev-list", "--left-right", "--count", "HEAD...refs/remotes/origin/"+f.Branch); err == nil {
		if a, b, ok := strings.Cut(counts, "\t"); ok {
			f.Ahead, _ = strconv.Atoi(a)
			f.Behind, _ = strconv.Atoi(b)
		}
	}
	if st, err := (git.Cmd{Ctx: ctx, Dir: ground, Args: []string{"status", "--porcelain", "--untracked-files=no"}, ReadOnly: true}).Output(); err == nil {
		f.Dirty = strings.TrimSpace(string(st)) != ""
	}
	return f
}

// pullGround moves the repo's ground forward to its remote's default
// branch, as last fetched: a fast-forward of a ground with nothing in it,
// which can't lose anything, and whether it moved. A ground with changes
// of its own, that's moved apart from the remote's, or that's on another
// branch, is refused.
func (w *Workspace) pullGround(ctx context.Context, repo string) (SpaceFresh, bool, error) {
	f := w.groundFresh(ctx, repo)
	ground := w.WS().MainWorktree(repo)
	switch {
	case f.Dirty:
		return f, false, refused("ground-dirty", "%s's ground has changes of its own: it's read-only by rule, so look at what's there before anything moves it", repo)
	case f.Ahead > 0:
		return f, false, refused("ground-ahead", "%s's ground has %d commits the remote hasn't: it can't be moved forward without losing them", repo, f.Ahead)
	case f.Behind == 0:
		return f, false, nil
	}
	if on, _ := git.ReadLine(ctx, ground, "symbolic-ref", "--quiet", "--short", "HEAD"); on != f.Branch {
		return f, false, refused("ground-branch", "%s's ground isn't on %s: look at why before anything moves it", repo, f.Branch)
	}
	if err := workspace.GitFFMerge(ground, "refs/remotes/origin/"+f.Branch); err != nil {
		var ge *git.Error
		if errors.As(err, &ge) {
			return f, false, refused("ground-pull", "moving %s's ground forward: %s", repo, ge.Stderr)
		}
		return f, false, err
	}
	return w.groundFresh(ctx, repo), true, nil
}

func dirThere(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
