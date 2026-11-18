package web

import (
	"errors"
	"net/http"
)

// Landing a capsule of a repo that has no pull requests: its commits go
// straight to the default branch, when you say. GET
// /api/repos/{repo}/worktrees/{name}/landing is where it stands, and POST
// .../land lands it (ws land).

// Landing is where a capsule stands for landing.
type Landing struct {
	// Direct: its repo takes commits straight to the default branch. When
	// it doesn't, the rest is empty: its work lands through a pull request.
	Direct bool   `json:"direct"`
	Branch string `json:"branch"` // the default branch
	// Commits are what would land: each commit's subject, oldest first.
	Commits []string `json:"commits"`
	// Dirty: it has changes that aren't committed, which don't land.
	Dirty bool `json:"dirty"`
	// Verify is the command that must pass before it's pushed, if the repo
	// has one.
	Verify string `json:"verify,omitempty"`
	// Mission is the mission that owns it, and Approved whether your latest
	// review there approves it as it is now.
	Mission  string `json:"mission,omitempty"`
	Approved bool   `json:"approved"`
	Pending  bool   `json:"pending,omitempty"`
	// Landed is its landing, once it has one (a capsule ws couldn't burn).
	Landed *Landed `json:"landed,omitempty"`
}

// Landed is commits on the default branch.
type Landed struct {
	Branch  string   `json:"branch"`
	Head    string   `json:"head"`
	Commits []string `json:"commits"`
	Mission string   `json:"mission,omitempty"`
	// Warnings are what didn't follow: the ground not brought up to it, the
	// capsule not burned.
	Warnings []string `json:"warnings,omitempty"`
}

// LandResult is what landing did: Landed, or Stopped, with nothing
// pushed.
type LandResult struct {
	Landed  *Landed      `json:"landed,omitempty"`
	Stopped *LandStopped `json:"stopped,omitempty"`
}

// LandStopped is a landing that stopped: at which step (check, fetch,
// rebase, verify, push), why, and what git or the check said.
type LandStopped struct {
	Step    string `json:"step"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

func (s *Server) landing(w http.ResponseWriter, r *http.Request) {
	l, err := s.App.Landing(r.PathValue("repo"), r.PathValue("name"))
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, "the workspace has no "+r.PathValue("repo")+"/"+r.PathValue("name"))
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		out := Landing{Direct: l.Direct, Branch: l.Branch, Commits: l.Commits, Dirty: l.Dirty, Verify: l.Verify, Mission: l.Mission, Approved: l.Approved, Pending: l.Pending}
		if l.Landed != nil {
			landed := Landed(*l.Landed)
			out.Landed = &landed
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func (s *Server) land(w http.ResponseWriter, r *http.Request) {
	res, err := s.App.LandCapsule(r.PathValue("repo"), r.PathValue("name"))
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, "the workspace has no "+r.PathValue("repo")+"/"+r.PathValue("name"))
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		var out LandResult
		if res.Landed != nil {
			landed := Landed(*res.Landed)
			out.Landed = &landed
		}
		if res.Stopped != nil {
			stopped := LandStopped(*res.Stopped)
			out.Stopped = &stopped
		}
		writeJSON(w, http.StatusOK, out)
	}
}
