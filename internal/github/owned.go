package github

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	gh "github.com/cli/go-gh/v2"
)

// Owner is someone repos can be added from: you, or an org you're in.
type Owner struct {
	Login string `json:"login"`
	// You: it's you, not an org.
	You bool `json:"you,omitempty"`
}

// OwnedRepo is one of an owner's repos, as GitHub lists it.
type OwnedRepo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PushedAt    string `json:"pushedAt,omitempty"`
	Archived    bool   `json:"archived,omitempty"`
	Fork        bool   `json:"fork,omitempty"`
	Private     bool   `json:"private,omitempty"`
}

// ListingClient is a Client that can say whose repos you can add (you and
// your orgs) and what each has: what adding a repo from GitHub picks from.
type ListingClient interface {
	Owners() ([]Owner, error)
	ReposOf(owner string) ([]OwnedRepo, error)
}

// reposPerPage is GitHub's most a page; maxRepoPages stops an owner with
// thousands from taking minutes (the newest pushed come first).
const (
	reposPerPage = 100
	maxRepoPages = 10
)

func (LiveClient) Owners() ([]Owner, error) { return Owners() }

func (LiveClient) ReposOf(owner string) ([]OwnedRepo, error) { return ReposOf(owner) }

// Owners is you, then the orgs you're in, by login.
func Owners() ([]Owner, error) {
	me, err := Viewer()
	if err != nil {
		return nil, err
	}
	stdOut, stdErr, err := gh.Exec("api", "user/orgs", "--paginate", "-q", ".[].login")
	if err != nil {
		return nil, fmt.Errorf("gh api user/orgs: %s", ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	out := []Owner{{Login: me, You: true}}
	var orgs []string
	for line := range strings.SplitSeq(stdOut.String(), "\n") {
		if l := strings.TrimSpace(line); l != "" {
			orgs = append(orgs, l)
		}
	}
	slices.SortFunc(orgs, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	for _, o := range orgs {
		out = append(out, Owner{Login: o})
	}
	return out, nil
}

const ownedQuery = `query($login: String!, $after: String) {
  repositoryOwner(login: $login) {
    repositories(first: 100, after: $after, orderBy: {field: PUSHED_AT, direction: DESC}) {
      nodes { name owner { login } description pushedAt isArchived isFork isPrivate }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

// ReposOf is an owner's repos, newest pushed first: an org's, or a
// person's own (not those they're only a collaborator on). An org that
// enforces SAML SSO answers as GitHub says, until gh's token is authorised
// for it.
func ReposOf(owner string) ([]OwnedRepo, error) {
	var out []OwnedRepo
	after := ""
	for page := 0; page < maxRepoPages; page++ {
		args := []string{"api", "graphql", "-f", "query=" + ownedQuery, "-F", "login=" + owner}
		if after != "" {
			args = append(args, "-F", "after="+after)
		}
		stdOut, stdErr, err := gh.Exec(args...)
		got, next, perr := parseOwned(stdOut.Bytes(), owner)
		if err != nil || perr != nil {
			if err == nil {
				err = perr
			}
			return out, fmt.Errorf("%s's repos: %s", owner, ghError(stdOut.Bytes(), stdErr.String(), err))
		}
		out = append(out, got...)
		if next == "" {
			break
		}
		after = next
	}
	return out, nil
}

// parseOwned reads a page of ownedQuery's answer: the owner's own repos on
// it, and the cursor of the next page ("" for none).
func parseOwned(data []byte, owner string) ([]OwnedRepo, string, error) {
	var res struct {
		Data *struct {
			RepositoryOwner *struct {
				Repositories struct {
					Nodes []struct {
						Name  string `json:"name"`
						Owner struct {
							Login string `json:"login"`
						} `json:"owner"`
						Description string `json:"description"`
						PushedAt    string `json:"pushedAt"`
						IsArchived  bool   `json:"isArchived"`
						IsFork      bool   `json:"isFork"`
						IsPrivate   bool   `json:"isPrivate"`
					} `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"repositories"`
			} `json:"repositoryOwner"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, "", fmt.Errorf("reading repos: %w", err)
	}
	if res.Data == nil {
		return nil, "", fmt.Errorf("reading repos: no data")
	}
	if res.Data.RepositoryOwner == nil {
		return nil, "", fmt.Errorf("there's no %s on GitHub", owner)
	}
	repos := res.Data.RepositoryOwner.Repositories
	var out []OwnedRepo
	for _, n := range repos.Nodes {
		if !strings.EqualFold(n.Owner.Login, owner) {
			continue // a collaborator's, listed under a person
		}
		out = append(out, OwnedRepo{Name: n.Name, Description: n.Description, PushedAt: n.PushedAt, Archived: n.IsArchived, Fork: n.IsFork, Private: n.IsPrivate})
	}
	next := ""
	if repos.PageInfo.HasNextPage {
		next = repos.PageInfo.EndCursor
	}
	return out, next, nil
}

// Owners is the fixture's viewer, then its owners' logins: the repos the
// fixture lists under owners are what can be added.
func (c FixtureClient) Owners() ([]Owner, error) {
	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	f, err := c.read()
	if err != nil {
		return nil, err
	}
	out := []Owner{{Login: f.Viewer, You: true}}
	var orgs []string
	for login := range f.Owners {
		if !strings.EqualFold(login, f.Viewer) {
			orgs = append(orgs, login)
		}
	}
	slices.Sort(orgs)
	for _, o := range orgs {
		out = append(out, Owner{Login: o})
	}
	return out, nil
}

// ReposOf is what the fixture lists under the owner.
func (c FixtureClient) ReposOf(owner string) ([]OwnedRepo, error) {
	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	f, err := c.read()
	if err != nil {
		return nil, err
	}
	for login, repos := range f.Owners {
		if strings.EqualFold(login, owner) {
			return repos, nil
		}
	}
	return nil, fmt.Errorf("there's no %s on GitHub", owner)
}
