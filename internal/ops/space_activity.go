package ops

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/workspace"
)

// What's happening in a space of documents: the commits on its default
// branch lately and which documents each changed, the capsules with
// changes to its documents and the pull requests open on them, and how
// its ground stands with its remote.

// SpaceActivity is a space's top: what changed lately, what's in flight,
// and how fresh it is.
type SpaceActivity struct {
	Lately   []SpaceCommit
	InFlight []SpaceFlight
	Fresh    SpaceFresh
}

// SpaceCommit is a commit on a space's default branch that changed its
// documents: who, when, what they said, and which.
type SpaceCommit struct {
	SHA, At, By, Subject string
	Docs                 []SpaceDocChange
}

// SpaceDocChange is a document a commit or a capsule changed: its path,
// its title (as it is where it changed), and how (added, changed,
// removed).
type SpaceDocChange struct {
	Path, Title string
	How         DocChange
}

// DocChange is what a commit or a capsule did to a document.
type DocChange string

const (
	DocAdded   DocChange = "added"
	DocChanged DocChange = "changed"
	DocRemoved DocChange = "removed"
)

// SpaceFlight is a capsule of a space's repo with changes to its
// documents: its name and branch, the mission it's in (if it is), the
// pull request open from it (if one is), and what it changes.
type SpaceFlight struct {
	Capsule, Branch, Mission string
	PR                       *SpacePR
	Docs                     []SpaceDocChange
}

// SpacePR is a pull request open from a capsule.
type SpacePR struct {
	Number int
	Title  string
	URL    string
}

// SpaceFresh is how a space's ground stands: when its repo was last
// fetched, and how far its default branch is behind (or ahead of) the
// remote's, as last fetched.
type SpaceFresh struct {
	Fetched       time.Time // zero: never, here
	Behind, Ahead int
	Branch        string
	Dirty         bool // the ground has changes of its own: it can't be pulled
}

// latelyKept is how many of a space's latest commits are said.
const latelyKept = 8

// SpaceActivity is what's happening in a space: ErrNotFound for a repo
// that isn't one.
func (w *Workspace) SpaceActivity(ctx context.Context, repo string) (*SpaceActivity, error) {
	s, err := w.spaceNamed(repo)
	if err != nil {
		return nil, err
	}
	ws := w.WS()
	ground := ws.MainWorktree(s.Repo)
	out := &SpaceActivity{Lately: []SpaceCommit{}, InFlight: []SpaceFlight{}}
	if _, err := os.Stat(ground); err != nil {
		return out, nil // not cloned here
	}
	titles := map[string]string{}
	if db, _, _ := w.Spaces(ctx); db != nil { // read fresh, for its titles
		if docs, err := space.Docs(ctx, db, s.Repo); err == nil {
			for _, d := range docs {
				titles[d.Path] = d.Title
			}
		}
	}
	if out.Lately, err = spaceLately(ctx, ground, s, titles); err != nil {
		return nil, err
	}
	out.InFlight = w.spaceInFlight(ctx, s, titles)
	out.Fresh = w.spaceFresh(ctx, s)
	return out, nil
}

func (w *Workspace) spaceNamed(repo string) (workspace.Space, error) {
	name, ok := w.WS().ResolveAlias(repo)
	if !ok {
		return workspace.Space{}, fmt.Errorf("%w: the workspace has no repo %s", ErrNotFound, repo)
	}
	s, ok := w.WS().SpaceOf(name)
	if !ok {
		return workspace.Space{}, fmt.Errorf("%w: %s isn't a space of documents", ErrNotFound, name)
	}
	return s, nil
}

// spaceLately is the latest commits on the ground that changed the
// space's documents, newest first.
func spaceLately(ctx context.Context, ground string, s workspace.Space, titles map[string]string) ([]SpaceCommit, error) {
	args := []string{"log", "-n", "40", "--no-renames", "--format=%x1e%H%x1f%aI%x1f%an%x1f%s", "--name-status", "HEAD"}
	if s.Folder != "" {
		args = append(args, "--", s.Folder)
	}
	out, err := git.Cmd{Ctx: ctx, Dir: ground, Args: args, ReadOnly: true}.Output()
	if err != nil {
		return nil, err
	}
	commits := []SpaceCommit{}
	for _, rec := range strings.Split(string(out), "\x1e") {
		head, rest, _ := strings.Cut(strings.TrimLeft(rec, "\n"), "\n")
		f := strings.Split(head, "\x1f")
		if len(f) < 4 {
			continue
		}
		c := SpaceCommit{SHA: f[0], At: f[1], By: f[2], Subject: f[3], Docs: []SpaceDocChange{}}
		for _, line := range strings.Split(rest, "\n") {
			status, p, ok := strings.Cut(line, "\t")
			if !ok || !s.Has(p) {
				continue
			}
			c.Docs = append(c.Docs, SpaceDocChange{Path: p, Title: titles[p], How: how(status)})
		}
		if len(c.Docs) > 0 {
			commits = append(commits, c)
		}
		if len(commits) == latelyKept {
			break
		}
	}
	return commits, nil
}

func how(status string) DocChange {
	switch {
	case strings.HasPrefix(status, "A"), status == "??":
		return DocAdded
	case strings.HasPrefix(status, "D"):
		return DocRemoved
	}
	return DocChanged
}

// spaceInFlight is the capsules of the space's repo that change its
// documents (against where they branched from the default branch, what's
// uncommitted and untracked too), each with the pull request open from it.
func (w *Workspace) spaceInFlight(ctx context.Context, s workspace.Space, titles map[string]string) []SpaceFlight {
	ws := w.WS()
	capsules, _ := workspace.ListWorktrees(ws.RepoDir(s.Repo))
	var prs PRIndex
	if gh := w.GitHub(); w.opts.PRs != nil && gh != nil {
		prs = w.opts.PRs.Peek(ws, gh, []string{s.Repo})
	}
	branch := firstNonEmpty(ws.DefaultBranch, "main")
	out := []SpaceFlight{}
	for _, name := range capsules {
		dir := filepath.Join(ws.RepoDir(s.Repo), name)
		base, err := git.ReadLine(ctx, dir, "merge-base", "HEAD", "refs/remotes/origin/"+branch)
		if err != nil {
			continue
		}
		changed := map[string]DocChange{}
		if d, err := (git.Cmd{Ctx: ctx, Dir: dir, Args: []string{"diff", "--no-renames", "--name-status", base, "--"}, ReadOnly: true}).Output(); err == nil {
			for _, line := range strings.Split(string(d), "\n") {
				if status, p, ok := strings.Cut(line, "\t"); ok && s.Has(p) {
					changed[p] = how(status)
				}
			}
		}
		if u, err := (git.Cmd{Ctx: ctx, Dir: dir, Args: []string{"ls-files", "--others", "--exclude-standard", "--"}, ReadOnly: true}).Output(); err == nil {
			for _, p := range strings.Split(string(u), "\n") {
				if s.Has(p) {
					changed[p] = "added"
				}
			}
		}
		if len(changed) == 0 {
			continue
		}
		_, b := workspace.GitHeadBranch(dir)
		f := SpaceFlight{Capsule: name, Branch: b, Docs: []SpaceDocChange{}}
		if m, ok := w.Missions.FindByCapsule(s.Repo, name); ok {
			f.Mission = m.Title
		}
		if pr := LookupPR(prs[s.Repo].open, b, name); pr != nil {
			f.PR = &SpacePR{Number: pr.Number, Title: pr.Title, URL: pr.URL}
		}
		for _, p := range slices.Sorted(maps.Keys(changed)) {
			title := titles[p]
			if changed[p] != "removed" {
				if text, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p))); err == nil {
					title = firstNonEmpty(workspace.DocTitle(text), title)
				}
			}
			f.Docs = append(f.Docs, SpaceDocChange{Path: p, Title: title, How: changed[p]})
		}
		out = append(out, f)
	}
	return out
}

// spaceFresh is how the space's ground stands with its remote, as last
// fetched.
func (w *Workspace) spaceFresh(ctx context.Context, s workspace.Space) SpaceFresh {
	ws := w.WS()
	f := SpaceFresh{Branch: firstNonEmpty(ws.DefaultBranch, "main")}
	if info, err := os.Stat(filepath.Join(ws.BareDir(s.Repo), "FETCH_HEAD")); err == nil {
		f.Fetched = info.ModTime()
	}
	ground := ws.MainWorktree(s.Repo)
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

// FetchSpace brings a space's default branch up to date in its repo's
// clone (the ground isn't moved: PullSpace does that), and says how it
// stands then.
func (w *Workspace) FetchSpace(ctx context.Context, repo string) (SpaceFresh, error) {
	s, err := w.spaceNamed(repo)
	if err != nil {
		return SpaceFresh{}, err
	}
	branch := firstNonEmpty(w.WS().DefaultBranch, "main")
	if _, err := (git.Cmd{Ctx: ctx, Dir: w.WS().BareDir(s.Repo), Args: []string{"fetch", "--quiet", "--no-tags", "origin", fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch)}}).Output(); err != nil {
		return w.spaceFresh(ctx, s), &Upstream{Err: err}
	}
	return w.spaceFresh(ctx, s), nil
}

// PullSpace moves a space's ground forward to its remote's default branch,
// as last fetched: a fast-forward of a ground with nothing in it, which
// can't lose anything. A ground with changes of its own, or that's moved
// apart from the remote's, is refused.
func (w *Workspace) PullSpace(ctx context.Context, repo string) (SpaceFresh, error) {
	s, err := w.spaceNamed(repo)
	if err != nil {
		return SpaceFresh{}, err
	}
	f := w.spaceFresh(ctx, s)
	switch {
	case f.Dirty:
		return f, refused("ground-dirty", "%s's ground has changes of its own: it's read-only by rule, so look at what's there before anything moves it", s.Repo)
	case f.Ahead > 0:
		return f, refused("ground-ahead", "%s's ground has %d commits the remote hasn't: it can't be moved forward without losing them", s.Repo, f.Ahead)
	case f.Behind == 0:
		return f, nil
	}
	if err := workspace.GitFFMerge(w.WS().MainWorktree(s.Repo), "refs/remotes/origin/"+f.Branch); err != nil {
		var ge *git.Error
		if errors.As(err, &ge) {
			return f, refused("ground-pull", "moving %s's ground forward: %s", s.Repo, ge.Stderr)
		}
		return f, err
	}
	return w.spaceFresh(ctx, s), nil
}
