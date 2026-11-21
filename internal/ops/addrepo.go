package ops

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/github"
)

// Adding a repo from GitHub: whose repos you can add (you, and your
// orgs), each one's repos, and adding one, for you alone (ws.local.toml)
// or for the team (ws.toml, committed and pushed), then cloning it.
//
// What GitHub said is kept for the daemon's life and answered at once,
// however old: a look behind the answer asks again when it's older than
// listFresh, and GitHubSeen moves when one finishes, for the page to ask
// again.

// listFresh is how long what GitHub said of owners and their repos is
// answered without looking again.
const listFresh = 10 * time.Minute

// listed is what GitHub last said of something: what, when, why it
// couldn't, and whether it's being asked again now.
type listed[T any] struct {
	val     T
	at      time.Time
	problem string
	looking bool
}

// ghListings is what GitHub last said of the owners and their repos, and
// the repos being cloned since they were added (by name: "" while it goes,
// else why it couldn't be).
type ghListings struct {
	mu      sync.Mutex
	owners  *listed[[]github.Owner]
	repos   map[string]*listed[[]github.OwnedRepo] // by owner, lower case
	cloning map[string]string
	seen    uint64
}

// GitHubOwners is whose repos can be added: you, then your orgs.
type GitHubOwners struct {
	Owners  []github.Owner
	At      time.Time // when GitHub said, zero for never
	Looking bool      // GitHub's being asked again
	Problem string    // why it couldn't be, last it was asked
}

// GitHubRepos is an owner's repos that can be added, newest pushed first,
// each with its name here when the workspace has it already.
type GitHubRepos struct {
	Owner   string
	Repos   []AddableRepo
	At      time.Time
	Looking bool
	Problem string
}

// AddableRepo is one of an owner's repos on GitHub, and its name in the
// workspace if it's here already ("" while it isn't).
type AddableRepo struct {
	github.OwnedRepo
	Here string
}

// GitHubSeen moves each time a look at GitHub finishes, or a repo being
// cloned is: what the page that adds repos follows.
func (w *Workspace) GitHubSeen() string {
	w.gh.mu.Lock()
	defer w.gh.mu.Unlock()
	return strconv.FormatUint(w.gh.seen, 10)
}

func (w *Workspace) lister() (github.ListingClient, bool) {
	lc, ok := w.GitHub().(github.ListingClient)
	return lc, ok
}

// GitHubOwners answers at once with what GitHub last said of whose repos
// you can add, and looks again behind it when that's old, or again is
// asked for.
func (w *Workspace) GitHubOwners(again bool) GitHubOwners {
	lc, ok := w.lister()
	if !ok {
		return GitHubOwners{Owners: []github.Owner{}, Problem: "there's no GitHub to list repos from here"}
	}
	w.gh.mu.Lock()
	if w.gh.owners == nil {
		w.gh.owners = &listed[[]github.Owner]{}
	}
	l := w.gh.owners
	if !l.looking && (again || time.Since(l.at) > listFresh) {
		l.looking = true
		go func() {
			got, err := lc.Owners()
			w.gh.mu.Lock()
			defer w.gh.mu.Unlock()
			l.looking = false
			w.gh.seen++
			if err != nil {
				l.problem = err.Error()
				return
			}
			l.val, l.at, l.problem = got, time.Now(), ""
		}()
	}
	out := GitHubOwners{Owners: append([]github.Owner{}, l.val...), At: l.at, Looking: l.looking, Problem: l.problem}
	w.gh.mu.Unlock()
	return out
}

// GitHubRepos answers at once with what GitHub last said of an owner's
// repos, and looks again behind it as GitHubOwners does. Each one the
// workspace has is said by its name here.
func (w *Workspace) GitHubRepos(owner string, again bool) GitHubRepos {
	owner = strings.TrimSpace(owner)
	out := GitHubRepos{Owner: owner, Repos: []AddableRepo{}}
	lc, ok := w.lister()
	if !ok {
		out.Problem = "there's no GitHub to list repos from here"
		return out
	}
	if !validLogin.MatchString(owner) {
		out.Problem = fmt.Sprintf("%q isn't a GitHub owner's name", owner)
		return out
	}
	key := strings.ToLower(owner)
	w.gh.mu.Lock()
	if w.gh.repos == nil {
		w.gh.repos = map[string]*listed[[]github.OwnedRepo]{}
	}
	l := w.gh.repos[key]
	if l == nil {
		l = &listed[[]github.OwnedRepo]{}
		w.gh.repos[key] = l
	}
	if !l.looking && (again || time.Since(l.at) > listFresh) {
		l.looking = true
		go func() {
			got, err := lc.ReposOf(owner)
			w.gh.mu.Lock()
			defer w.gh.mu.Unlock()
			l.looking = false
			w.gh.seen++
			if err != nil {
				l.problem = err.Error()
				return
			}
			l.val, l.at, l.problem = got, time.Now(), ""
		}()
	}
	repos := append([]github.OwnedRepo{}, l.val...)
	out.At, out.Looking, out.Problem = l.at, l.looking, l.problem
	w.gh.mu.Unlock()
	ws := w.WS()
	for _, r := range repos {
		here, _ := ws.RepoAt(owner, r.Name)
		if here != "" && !ws.OnGitHub(here) {
			here = "" // matched by name alone, with no owner to be sure
		}
		out.Repos = append(out.Repos, AddableRepo{OwnedRepo: r, Here: here})
	}
	return out
}

var (
	validLogin   = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	validGitName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	validKey     = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*$`)
)

// RepoAdd is a repo to add from GitHub: where it is there, its name here
// (its name there, unless As says), and whether it's the team's (in
// ws.toml, committed and pushed) or yours (in ws.local.toml). Wait clones
// it before answering; else it's cloned behind the answer.
type RepoAdd struct {
	Owner, Name string
	As          string
	// Group is the group it's in, if any: one the workspace has, or new.
	Group string
	Team  bool
	Wait  bool
}

// RepoAdded is a repo added: its name here, where it is on GitHub, whose
// it is, the commit that gave it to the team, and whether it's being
// cloned still.
type RepoAdded struct {
	Repo, GitHub   string
	Team           bool
	Commit, Branch string
	Cloning        bool
}

func (r RepoAdded) String() string {
	where := "yours, in ws.local.toml"
	if r.Team {
		where = fmt.Sprintf("the team's, in ws.toml: pushed to %s (%s)", r.Branch, r.Commit)
	}
	s := fmt.Sprintf("%s (%s) is %s", r.Repo, r.GitHub, where)
	if r.Cloning {
		s += ", and it's being cloned"
	}
	return s
}

// AddRepo adds a repo from GitHub to the workspace, and clones it. For
// you it's written into ws.local.toml; for the team, into ws.toml,
// committed on the workspace's config repo and pushed (or taken back, if
// the remote refuses: nothing's added). It's said where it is on GitHub
// (github) unless it's the org's of its name here.
func (w *Workspace) AddRepo(ctx context.Context, who Actor, a RepoAdd) (*RepoAdded, error) {
	if !who.IsYou() {
		return nil, refused("not-yours", "adding a repo to the workspace is the user's to do")
	}
	owner, name := strings.TrimSpace(a.Owner), strings.TrimSuffix(strings.TrimSpace(a.Name), ".git")
	if !validLogin.MatchString(owner) || !validGitName.MatchString(name) {
		return nil, invalid("say where the repo is on GitHub, as owner/name: %q isn't", owner+"/"+name)
	}
	key := strings.TrimSpace(a.As)
	if key == "" {
		key = name
	}
	if !validKey.MatchString(key) {
		return nil, invalid("%q can't be a repo's name here: letters, digits, '.', '_' and '-'", key)
	}
	ws := w.WS()
	if here, ok := ws.RepoAt(owner, name); ok && ws.OnGitHub(here) {
		return nil, invalid("%s/%s is here already, as %s", owner, name, here)
	}
	if _, taken := ws.ResolveAlias(key); taken {
		return nil, invalid("there's a repo called %s here already: give this one another name", key)
	}
	group := strings.TrimSpace(a.Group)
	if strings.ContainsAny(group, ", \t") {
		return nil, invalid("%q can't be a group's name: no spaces or commas", group)
	}
	rc := config.RepoConfig{Group: group}
	if !strings.EqualFold(owner, ws.Org) || name != key {
		rc.GitHub = owner + "/" + name
	}
	out := &RepoAdded{Repo: key, GitHub: owner + "/" + name, Team: a.Team}
	if a.Team {
		commit, branch, err := w.teamRepo(ctx, key, rc, fmt.Sprintf("%s/%s, added by %s.", owner, name, gitUser(ctx, w.root)), "so it isn't added")
		if err != nil {
			return nil, err
		}
		out.Commit, out.Branch = commit, branch
	} else if err := config.AddLocalRepo(w.root, key, rc); err != nil {
		return nil, err
	}
	if a.Wait {
		if err := w.cloneRepo(key); err != nil {
			return out, fmt.Errorf("%s is added, but couldn't be cloned (ws setup tries again): %w", key, err)
		}
		return out, nil
	}
	out.Cloning = true
	w.cloneBehind(key)
	return out, nil
}

// cloneBehind clones repos one after another, behind the answer: each is
// said as cloning until it is, or why it couldn't be, and GitHubSeen
// moves as each ends.
func (w *Workspace) cloneBehind(repos ...string) {
	if len(repos) == 0 {
		return
	}
	w.gh.mu.Lock()
	if w.gh.cloning == nil {
		w.gh.cloning = map[string]string{}
	}
	for _, r := range repos {
		w.gh.cloning[r] = ""
	}
	w.gh.mu.Unlock()
	go func() {
		for _, r := range repos {
			err := w.cloneRepo(r)
			w.gh.mu.Lock()
			w.gh.seen++
			if err != nil {
				w.gh.cloning[r] = err.Error()
			} else {
				delete(w.gh.cloning, r)
			}
			w.gh.mu.Unlock()
		}
	}()
}

// cloneRepo clones a repo just added, with the workspace's config read
// again: what's held may be from before it was.
func (w *Workspace) cloneRepo(repo string) error {
	_, ws, _, err := Load(w.root)
	if err != nil {
		return err
	}
	ws.DefaultBranch = firstNonEmpty(ws.DefaultBranch, "main") // as the rest of ws reads it
	return ws.SetupRepo(repo).Err
}

// cloningOf is whether a repo added is being cloned still, and why it
// couldn't be, if it couldn't.
func (w *Workspace) cloningOf(repo string) (bool, string) {
	w.gh.mu.Lock()
	defer w.gh.mu.Unlock()
	problem, ok := w.gh.cloning[repo]
	return ok && problem == "", problem
}
