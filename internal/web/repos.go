package web

import (
	"errors"
	"net/http"

	"github.com/withworkspace/workspace/internal/ops"
)

// The Files page's repos: every one in the workspace, and a repo's ground
// and capsules. Their files are read by the explorer's handlers (tree,
// file, raw) at /api/repos/{repo}/worktrees/{name}/….

func (s *Server) repos(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"repos": s.repoList()})
}

// shareRepo gives a repo of yours to the team (ws repo share): what
// happened, as a sentence, or why it couldn't be.
func (s *Server) shareRepo(w http.ResponseWriter, r *http.Request) {
	done, err := s.App.ShareRepo(r.Context(), ops.AsYou, r.PathValue("repo"))
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": done.String(), "commit": done.Commit})
}

func (s *Server) worktrees(w http.ResponseWriter, r *http.Request) {
	got, err := s.App.Worktrees(r.PathValue("repo"))
	wts := make([]Worktree, 0, len(got))
	for _, wt := range got {
		wts = append(wts, Worktree(wt))
	}
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, "no repo "+r.PathValue("repo"))
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, map[string]any{"worktrees": wts})
	}
}

// repoList is the workspace's repos, as the API serves them.
func (s *Server) repoList() []Repo {
	got := s.App.Repos()
	out := make([]Repo, 0, len(got))
	for _, r := range got {
		out = append(out, Repo(r))
	}
	return out
}
