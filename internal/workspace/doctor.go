package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/ref"
)

type CheckStatus int

const (
	CheckOK CheckStatus = iota
	CheckWarn
	CheckFail
)

type CheckResult struct {
	Name    string
	Status  CheckStatus
	Detail  string
	Fix     func() error // nil if not auto-fixable
	FixHint string       // shown when Fix is nil but manual action exists
}

type CheckCategory struct {
	Name   string
	Checks []CheckResult
}

// Doctor runs all workspace health checks and returns results grouped by
// category. missions checks what's above the workspace (its missions, and
// the agents that work on them), from the caller that knows of them; nil
// for none.
func (w *Workspace) Doctor(missions func(*Workspace) CheckCategory) []CheckCategory {
	cats := []CheckCategory{
		w.checkRepos(),
		w.checkOrphanedWorktrees(),
		w.checkTools(),
	}
	if siloChecks := w.checkSilos(); len(siloChecks.Checks) > 0 {
		cats = append(cats, siloChecks)
	}
	if missions != nil {
		if missionChecks := missions(w); len(missionChecks.Checks) > 0 {
			cats = append(cats, missionChecks)
		}
	}
	if agentChecks := w.checkAgents(); len(agentChecks.Checks) > 0 {
		cats = append(cats, agentChecks)
	}
	if kindChecks := w.checkKinds(); len(kindChecks.Checks) > 0 {
		cats = append(cats, kindChecks)
	}
	return cats
}

// checkRepos covers the default repos only: repos in hidden groups are
// cloned on first use, so not having them is expected.
func (w *Workspace) checkRepos() CheckCategory {
	var checks []CheckResult
	for _, name := range w.DefaultRepoNames() {
		bareDir := w.BareDir(name)
		result := CheckResult{Name: name, Status: CheckOK}
		if _, err := os.Stat(bareDir); err != nil {
			result.Status = CheckFail
			result.Detail = fmt.Sprintf("not cloned (expected %s)", bareDir)
			repoName := name // capture for closure
			result.Fix = func() error {
				return w.SetupRepo(repoName).Err
			}
		}
		checks = append(checks, result)
	}
	return CheckCategory{Name: "Repos", Checks: checks}
}

func (w *Workspace) checkOrphanedWorktrees() CheckCategory {
	var checks []CheckResult
	for _, name := range w.ListedRepoNames() {
		repoDir := w.RepoDir(name)
		worktrees, err := ListAllWorktrees(repoDir)
		if err != nil {
			checks = append(checks, CheckResult{
				Name:   name,
				Status: CheckWarn,
				Detail: fmt.Sprintf("could not list worktrees: %v", err),
			})
			continue
		}

		knownBranches := gitWorktreeListBranches(w.BareDir(name))
		if knownBranches == nil {
			continue
		}

		for _, wt := range worktrees {
			if !knownBranches[wt] {
				wtPath := filepath.Join(w.RepoDir(name), wt)
				checks = append(checks, CheckResult{
					Name:    fmt.Sprintf("%s/%s", name, wt),
					Status:  CheckWarn,
					Detail:  "directory exists but not registered as a git worktree",
					FixHint: fmt.Sprintf("rm -rf %s", wtPath),
				})
			}
		}
	}

	if len(checks) == 0 {
		checks = append(checks, CheckResult{
			Name:   "No orphaned worktrees",
			Status: CheckOK,
		})
	}
	return CheckCategory{Name: "Worktrees", Checks: checks}
}

// CheckGitHubAuthFunc is the function used to check GitHub auth status.
// Tests can replace this to avoid shelling out to gh.
var CheckGitHubAuthFunc = checkGitHubAuth

func (w *Workspace) checkTools() CheckCategory {
	ghResult := CheckResult{Name: "gh", Status: CheckOK}
	if _, err := exec.LookPath("gh"); err != nil {
		ghResult.Status = CheckFail
		ghResult.Detail = "gh CLI not found on PATH"
		ghResult.FixHint = "install with: brew install gh"
	}

	authResult := CheckGitHubAuthFunc()

	return CheckCategory{Name: "Tools", Checks: []CheckResult{ghResult, authResult}}
}

func checkGitHubAuth() CheckResult {
	result := CheckResult{Name: "gh auth", Status: CheckOK}
	out, err := exec.Command("gh", "auth", "status", "--hostname", "github.com").CombinedOutput()
	if err != nil {
		result.Status = CheckFail
		result.Detail = "not authenticated"
		result.FixHint = "run: gh auth login"
		return result
	}
	// Extract username from "Logged in to github.com account <user> ..."
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if _, after, ok := strings.Cut(line, "account "); ok {
			if user, _, ok := strings.Cut(after, " "); ok {
				result.Detail = user
			}
			break
		}
	}
	return result
}

// CheckGitHubReachFunc asks GitHub whether a repo is there for the gh
// that's signed in: nil for yes, else what GitHub said. Tests replace it.
var CheckGitHubReachFunc = reachRepo

func reachRepo(r ref.Remote) error {
	out, err := exec.Command("gh", "api", "repos/"+r.String(), "--jq", ".full_name").CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

// CheckGitHub is whether each owner's repos can be reached on GitHub by
// the gh that's signed in, and whether each clone's origin is where the
// workspace says the repo is. An org that enforces SAML SSO needs gh's
// token authorised for it, which is per org: until then its repos answer
// as though they weren't there. The caller leaves it out where GitHub
// isn't asked (a fixture answers for it).
func (w *Workspace) CheckGitHub() CheckCategory {
	byOwner := map[string][]string{}
	var owners []string
	for _, name := range w.RepoNames {
		r := w.RemoteOf(name)
		if r.Owner == "" {
			continue
		}
		key := strings.ToLower(r.Owner)
		if _, ok := byOwner[key]; !ok {
			owners = append(owners, r.Owner)
		}
		byOwner[key] = append(byOwner[key], name)
	}
	if len(owners) == 0 {
		return CheckCategory{Name: "GitHub"}
	}
	errs := make(map[string]error, len(w.RepoNames))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, names := range byOwner {
		for _, name := range names {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := CheckGitHubReachFunc(w.RemoteOf(name))
				mu.Lock()
				errs[name] = err
				mu.Unlock()
			}()
		}
	}
	wg.Wait()

	var checks []CheckResult
	for _, owner := range owners {
		names := byOwner[strings.ToLower(owner)]
		var missing []string
		var said error
		for _, name := range names {
			if errs[name] != nil {
				missing = append(missing, w.RemoteOf(name).String())
				said = errs[name]
			}
		}
		c := CheckResult{Name: owner, Status: CheckOK, Detail: fmt.Sprintf("%d %s reached", len(names), plural(len(names), "repo"))}
		if len(missing) > 0 {
			c.Status = CheckFail
			c.Detail = fmt.Sprintf("can't reach %s: %s", strings.Join(missing, ", "), firstLine(said.Error()))
			switch {
			case strings.Contains(said.Error(), "SAML"):
				c.FixHint = fmt.Sprintf("authorise gh's token for %s's single sign-on (https://github.com/orgs/%s/sso), then: gh auth refresh", owner, owner)
			case len(missing) == len(names):
				c.FixHint = fmt.Sprintf("check gh can see %s (gh auth status), and that ws.toml's org or github says where these repos are", owner)
			default:
				c.FixHint = "check ws.toml says where these repos are on GitHub: [repos.<name>] github = \"owner/name\""
			}
		}
		checks = append(checks, c)
	}
	return CheckCategory{Name: "GitHub", Checks: append(checks, w.checkOrigins()...)}
}

var githubOrigin = regexp.MustCompile(`^(?:https://github\.com/|git@github\.com:|ssh://git@github\.com/)([^/]+)/([^/]+?)(?:\.git)?/?$`)

// checkOrigins is each clone whose origin is on GitHub somewhere other
// than where the workspace says the repo is: a repo given its github after
// it was cloned, still fetching from where it was.
func (w *Workspace) checkOrigins() []CheckResult {
	var out []CheckResult
	for _, name := range w.RepoNames {
		want := w.RemoteOf(name)
		bare := w.BareDir(name)
		if want.Owner == "" || !dirExists(bare) {
			continue
		}
		origin, err := git.ReadLine(context.Background(), bare, "config", "--get", "remote.origin.url")
		if err != nil {
			continue
		}
		m := githubOrigin.FindStringSubmatch(origin)
		if m == nil || want.Is(m[1], m[2]) {
			continue // not GitHub's (a test's, a mirror), or where it should be
		}
		url := RepoCloneURL(want.Owner, want.Name, w.GitProtocol)
		out = append(out, CheckResult{
			Name:   name + " origin",
			Status: CheckWarn,
			Detail: fmt.Sprintf("fetches from %s/%s, but the workspace says it's %s", m[1], m[2], want),
			Fix: func() error {
				_, err := git.ReadLine(context.Background(), bare, "remote", "set-url", "origin", url)
				return err
			},
		})
	}
	return out
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func (w *Workspace) checkSilos() CheckCategory {
	var checks []CheckResult

	for repo, target := range w.Silo {
		// Check target capsule exists
		targetDir := filepath.Join(w.RepoDir(repo), target)
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			checks = append(checks, CheckResult{
				Name:    fmt.Sprintf("%s/%s", repo, target),
				Status:  CheckWarn,
				Detail:  "silo target does not exist",
				FixHint: fmt.Sprintf("ws silo point %s .ground", repo),
			})
		}

		// Check .silo/ directory exists
		siloDir := filepath.Join(w.RepoDir(repo), SiloDir)
		if _, err := os.Stat(siloDir); os.IsNotExist(err) {
			checks = append(checks, CheckResult{
				Name:    fmt.Sprintf("%s/.silo", repo),
				Status:  CheckWarn,
				Detail:  "silo configured but .silo/ directory missing",
				FixHint: fmt.Sprintf("ws silo point %s %s", repo, target),
			})
		}
	}

	// Check for orphaned .silo/ directories
	for _, name := range w.RepoNames {
		siloDir := filepath.Join(w.RepoDir(name), SiloDir)
		if _, err := os.Stat(siloDir); err == nil {
			if _, ok := w.Silo[name]; !ok {
				checks = append(checks, CheckResult{
					Name:    fmt.Sprintf("%s/.silo", name),
					Status:  CheckWarn,
					Detail:  ".silo/ directory exists but no silo configured",
					FixHint: fmt.Sprintf("ws silo stop %s", name),
				})
			}
		}
	}

	// The lock an older ws's silo watch took: the daemon keeps the silos
	// now, so one still running syncs them as well, and one gone left it.
	lockPath := filepath.Join(w.Root, ".silo.lock")
	if _, err := os.Stat(lockPath); err == nil {
		if pid, ok := lockHolder(lockPath); ok {
			checks = append(checks, CheckResult{
				Name:    ".silo.lock",
				Status:  CheckWarn,
				Detail:  fmt.Sprintf("a ws silo watch from an older ws is still running (pid %d), syncing the silos as the daemon does", pid),
				FixHint: fmt.Sprintf("stop it (kill %d): the daemon keeps them", pid),
			})
		} else {
			lp := lockPath
			checks = append(checks, CheckResult{
				Name:   ".silo.lock",
				Status: CheckWarn,
				Detail: "left by an older ws's silo watch: the daemon keeps the silos now",
				Fix: func() error {
					return os.Remove(lp)
				},
			})
		}
	}

	return CheckCategory{Name: "Silos", Checks: checks}
}

// gitWorktreeListBranches uses `git worktree list` to get registered worktree paths,
// then extracts the directory name (which corresponds to the worktree folder name).
func gitWorktreeListBranches(mainWT string) map[string]bool {
	out, err := git.Combined(mainWT, "worktree", "list", "--porcelain")
	if err != nil {
		return nil
	}
	result := make(map[string]bool)
	for line := range strings.SplitSeq(out, "\n") {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			result[filepath.Base(path)] = true
		}
	}
	return result
}

// lockHolder is the process an older ws's .silo.lock names, if it's alive.
func lockHolder(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	proc, err := os.FindProcess(pid)
	return pid, err == nil && proc.Signal(syscall.Signal(0)) == nil
}
