package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
)

// Adding a repo from GitHub: whose repos you can add (you, and your
// orgs), an owner's repos, and adding one. Each list answers at once with
// what GitHub last said, and asks again behind it when that's old (or
// ?again=1): the github topic says when it has.

const topicGitHub = "github"

// GitHubOwners is whose repos can be added: you, then your orgs.
type GitHubOwners struct {
	Owners []GitHubOwner `json:"owners"`
	// At is when GitHub said ("" for never), Looking whether it's being
	// asked again, and Problem why it couldn't say, last it was asked.
	At      string `json:"at,omitempty"`
	Looking bool   `json:"looking,omitempty"`
	Problem string `json:"problem,omitempty"`
}

// GitHubOwner is someone repos can be added from: you, or an org.
type GitHubOwner struct {
	Login string `json:"login"`
	You   bool   `json:"you,omitempty"`
}

// GitHubRepos is an owner's repos, newest pushed first.
type GitHubRepos struct {
	Owner   string        `json:"owner"`
	Repos   []AddableRepo `json:"repos"`
	At      string        `json:"at,omitempty"`
	Looking bool          `json:"looking,omitempty"`
	Problem string        `json:"problem,omitempty"`
}

// AddableRepo is one of an owner's repos, and its name here when the
// workspace has it already.
type AddableRepo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PushedAt    string `json:"pushedAt,omitempty"`
	Archived    bool   `json:"archived,omitempty"`
	Fork        bool   `json:"fork,omitempty"`
	Private     bool   `json:"private,omitempty"`
	Here        string `json:"here,omitempty"`
}

// RepoAdd is a repo to add (POST /api/repos): where it is on GitHub, its
// name here if not its name there, and whether it's the team's.
type RepoAdd struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
	As    string `json:"as,omitempty"`
	Team  bool   `json:"team,omitempty"`
}

// RepoAdded is what adding one did, said.
type RepoAdded struct {
	Repo    string `json:"repo"`
	Message string `json:"message"`
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func (s *Server) githubOwners(w http.ResponseWriter, r *http.Request) {
	v := s.App.GitHubOwners(r.URL.Query().Get("again") != "")
	out := GitHubOwners{Owners: make([]GitHubOwner, 0, len(v.Owners)), At: stamp(v.At), Looking: v.Looking, Problem: v.Problem}
	for _, o := range v.Owners {
		out.Owners = append(out.Owners, GitHubOwner(o))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) githubRepos(w http.ResponseWriter, r *http.Request) {
	v := s.App.GitHubRepos(r.URL.Query().Get("owner"), r.URL.Query().Get("again") != "")
	out := GitHubRepos{Owner: v.Owner, Repos: make([]AddableRepo, 0, len(v.Repos)), At: stamp(v.At), Looking: v.Looking, Problem: v.Problem}
	for _, x := range v.Repos {
		out.Repos = append(out.Repos, AddableRepo{Name: x.Name, Description: x.Description, PushedAt: x.PushedAt, Archived: x.Archived, Fork: x.Fork, Private: x.Private, Here: x.Here})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) addRepo(w http.ResponseWriter, r *http.Request) {
	var req RepoAdd
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "say which repo: owner and name")
		return
	}
	done, err := s.App.AddRepo(r.Context(), ops.AsYou, ops.RepoAdd{Owner: req.Owner, Name: req.Name, As: req.As, Team: req.Team})
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	if h := s.topics[topicWorkspace]; h != nil {
		h.nudge() // the config's changed: the overview has the repo now
	}
	writeJSON(w, http.StatusOK, RepoAdded{Repo: done.Repo, Message: done.String()})
}
