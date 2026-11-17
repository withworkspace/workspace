package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/git"
)

// How a repo's work lands: through a pull request, or (direct) as commits
// pushed straight to its default branch, with no review gate but you.
const (
	LandPR     = "pr"
	LandDirect = "direct"
)

// LandTries is how many times a landing pushes before it gives up: the
// first, and twice more when the default branch moved under it.
const LandTries = 3

// repoFile is what a repo says of itself in its ws.repo.toml, on its
// default branch: how its work lands and how it's checked first, and
// whether it's a space of documents.
func (w *Workspace) repoFile(repo string) config.RepoFileConfig {
	rc, err := config.ParseRepoConfig(filepath.Join(w.MainWorktree(repo), config.RepoFileName))
	if err != nil || rc == nil {
		return config.RepoFileConfig{}
	}
	return *rc
}

// Landing is how work lands in repo: what the workspace says (ws.toml,
// ws.local.toml), else what the repo says of itself (ws.repo.toml on its
// default branch), else through a pull request.
func (w *Workspace) Landing(repo string) string {
	if l := w.Landings[repo]; l != "" {
		return l
	}
	if w.repoFile(repo).Landing == LandDirect {
		return LandDirect
	}
	return LandPR
}

// Direct reports whether repo takes commits straight to its default branch.
func (w *Workspace) Direct(repo string) bool { return w.Landing(repo) == LandDirect }

// VerifyCommand is the command that must pass in a capsule of repo before
// it lands, or "".
func (w *Workspace) VerifyCommand(repo string) string {
	if v := w.Verifies[repo]; v != "" {
		return v
	}
	return w.repoFile(repo).Verify
}

// LandError is a landing that stopped: at which step, why, and what git
// (or the verify command) said. Nothing was pushed.
type LandError struct {
	Step   string // check, fetch, rebase, number, verify, push
	Msg    string
	Output string
}

func (e *LandError) Error() string { return e.Msg }

// Landed is a capsule's commits on its repo's default branch.
type Landed struct {
	Repo, Capsule string
	Branch        string   // the default branch
	Head          string   // the commit it's now at
	Commits       []string // what landed: each commit's subject, oldest first
	Tries         int      // pushes it took
	// Ground is why the ground couldn't be brought up to it, if it couldn't:
	// the commits are public all the same.
	Ground error
	// Settled is what was settled as it was rebased (LandOptions.Settle):
	// an artifact's version that lands under another number, each said
	// once.
	Settled []string
}

// LandOptions are for Land.
type LandOptions struct {
	// Step is told each step as it starts.
	Step func(name string)
	// Quiet stops git asking for a password where there's no terminal to
	// ask in (the web UI).
	Quiet bool
	// Settle settles the clashes a rebase stops at (conflicted: their
	// paths) where it knows how: an artifact's version numbered as one
	// that landed first takes the next number. It says what it did, and
	// false where it can't, and the landing stops, as at any clash.
	Settle func(dir string, conflicted []string) ([]string, bool)
	// Check checks what's been rebased before it's pushed: an error stops
	// the landing, with the capsule put back as it was before the rebase.
	Check func(dir, base string) error
}

// CapsuleConflicts says whether a capsule merges cleanly onto its repo's
// default branch, as its remote has it now (fetched first; as last
// fetched, if that fails): the files that would conflict, none when it
// merges cleanly. A pull request that conflicts from the start is no use
// to anyone, so ws checks before it opens one.
func (w *Workspace) CapsuleConflicts(repo, capsule string) ([]string, error) {
	dir := filepath.Join(w.RepoDir(repo), capsule)
	GitFetchOrigin(dir) //nolint:errcheck // offline: as last fetched
	return GitMergeConflicts(dir, "origin/"+w.DefaultBranch)
}

// CapsuleAhead is how many commits a capsule has that its repo's default
// branch, as last fetched, doesn't.
func (w *Workspace) CapsuleAhead(repo, capsule string) int {
	return GitCommitsSince(filepath.Join(w.RepoDir(repo), capsule), "origin/"+w.DefaultBranch)
}

// Land lands a capsule of a direct repo: its commits are rebased onto the
// default branch as it is on the remote, verified if the repo has a
// command for that, and pushed there; the ground is then brought up to
// them. It's how the atomic-repo way of working is done, in one place:
//
//   - a capsule with uncommitted changes, or nothing to land, is refused;
//   - a push that isn't a fast-forward is fetched, rebased, verified and
//     tried again, twice, and then it stops;
//   - a push refused for any other reason (protection, a hook) stops, with
//     what was said. Nothing is ever forced;
//   - a rebase that clashes stops, with the capsule as it was;
//   - there's never a merge commit.
//
// The capsule is left where it is: burning it is the caller's.
func (w *Workspace) Land(repo, capsule string, o LandOptions) (*Landed, error) {
	step := func(name string) {
		if o.Step != nil {
			o.Step(name)
		}
	}
	stop := func(at, msg, out string) (*Landed, error) {
		return nil, &LandError{Step: at, Msg: msg, Output: strings.TrimSpace(out)}
	}
	branch := w.DefaultBranch
	base := "origin/" + branch
	name := repo + "/" + capsule
	dir := filepath.Join(w.RepoDir(repo), capsule)
	// A rebase never stops to ask for a message.
	env := []string{"GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=true"}
	if o.Quiet {
		env = append(env, "GIT_TERMINAL_PROMPT=0")
	}
	run := func(args ...string) (string, error) {
		return git.Cmd{Dir: dir, Args: args, Env: env}.Combined()
	}

	switch {
	case capsule == GroundDir || capsule == SiloDir:
		return stop("check", "the ground isn't landed: it's the default branch already", "")
	case !w.Direct(repo):
		return stop("check", fmt.Sprintf("%s lands through pull requests. If its commits go straight to %s, say so: landing = \"direct\" under [repos.%s] in ws.toml", repo, branch, repo), "")
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return stop("check", "no capsule "+name, "")
	}
	if GitIsDirty(dir) {
		return stop("check", name+" has changes that aren't committed: commit them (or put them aside) and land again", "")
	}

	step("Fetching " + base)
	if out, err := run("fetch", "origin"); err != nil {
		return stop("fetch", "couldn't fetch "+repo+"'s remote", out)
	}
	if GitCommitsSince(dir, base) == 0 {
		return stop("check", fmt.Sprintf("nothing to land: %s has no commits that aren't on %s", name, branch), "")
	}

	landed := &Landed{Repo: repo, Capsule: capsule, Branch: branch}
	verify := w.VerifyCommand(repo)
	for {
		landed.Tries++
		step("Rebasing onto " + base)
		out, err := run("rebase", base)
		for err != nil && o.Settle != nil {
			clash, _ := run("diff", "--name-only", "--diff-filter=U")
			files := strings.Fields(clash)
			if len(files) == 0 {
				break
			}
			said, ok := o.Settle(dir, files)
			if !ok {
				break
			}
			for _, s := range said {
				if !slices.Contains(landed.Settled, s) {
					landed.Settled = append(landed.Settled, s)
				}
			}
			out, err = run("rebase", "--continue")
		}
		if err != nil {
			clash, _ := run("diff", "--name-only", "--diff-filter=U")
			run("rebase", "--abort")
			files := strings.Join(strings.Fields(clash), ", ")
			if files == "" {
				return stop("rebase", fmt.Sprintf("%s couldn't be rebased onto %s: it's as it was", name, base), out)
			}
			return stop("rebase", fmt.Sprintf("%s clashes with what's on %s since, in %s. It's as it was: rebase it in the capsule (git rebase %s), settle the clash, and land again", name, branch, files, base), out)
		}
		if GitCommitsSince(dir, base) == 0 {
			return stop("check", fmt.Sprintf("nothing to land: what %s changes is on %s already", name, branch), "")
		}
		if taken := w.numberTaken(repo, dir, base); taken != "" {
			return stop("number", taken, "")
		}
		if o.Check != nil {
			if err := o.Check(dir, base); err != nil {
				run("reset", "-q", "--hard", "ORIG_HEAD")
				return stop("number", fmt.Sprintf("%s's artifacts aren't whole once it's rebased onto %s (%v). It's as it was: rebase it in the capsule (git rebase %s), put them right, and land again", name, base, err, base), "")
			}
		}
		if verify != "" {
			step("Verifying: " + verify)
			var out bytes.Buffer
			if err := RunHook(dir, verify, &out, &out); err != nil {
				return stop("verify", fmt.Sprintf("%s doesn't pass %s's check (%s): nothing was pushed", name, repo, verify), tail(out.String(), 40))
			}
		}
		if subjects, err := run("log", "--reverse", "--format=%s", base+"..HEAD"); err == nil {
			landed.Commits = strings.Split(strings.TrimSpace(subjects), "\n")
		}

		step("Pushing to " + branch)
		out, err = run("push", "--porcelain", "origin", "HEAD:refs/heads/"+branch)
		if err == nil {
			break
		}
		if !pushBehind(out) {
			return stop("push", fmt.Sprintf("%s's remote refused the push, and nothing was forced. If %s is protected, or takes pull requests, the repo isn't direct", repo, branch), out)
		}
		if landed.Tries >= LandTries {
			return stop("push", fmt.Sprintf("%s moved %d times while this was landing, and nothing was pushed. Land again when it's quieter", branch, LandTries), out)
		}
		step(branch + " moved: fetching it again")
		if out, err := run("fetch", "origin"); err != nil {
			return stop("fetch", "couldn't fetch "+repo+"'s remote", out)
		}
	}
	landed.Head = GitRevParse(dir, "HEAD")

	// The ground sees the push right away. It's the one write the ground
	// gets, and only ever a fast-forward.
	step("Bringing the ground up to it")
	if err := GitFetch(w.BareDir(repo)); err != nil {
		landed.Ground = err
	} else if err := GitFFMerge(w.MainWorktree(repo), base); err != nil {
		landed.Ground = errors.New("the ground isn't a fast-forward from " + base + ": it's drifted")
	}
	return landed, nil
}

// pushBehind reports whether a push (git push --porcelain) was refused
// because the remote's branch has commits this one hasn't: the one refusal
// worth trying again.
func pushBehind(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "!") && strings.Contains(line, "[rejected]") &&
			(strings.Contains(line, "fetch first") || strings.Contains(line, "non-fast-forward")) {
			return true
		}
	}
	return false
}

// tail is the last n lines of s.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
