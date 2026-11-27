package web

import (
	"encoding/json"
	"net/http"

	"github.com/withworkspace/workspace/internal/ops"
)

// The workspace's groups (Settings' repos): each with its repos and
// whether it's hidden, for you and for the team; showing or hiding one
// for you; and putting a repo in a group for you. Each change is
// ws.local.toml's.

// RepoGroup is a repo's group in the workspace, as the API serves it.
type RepoGroup struct {
	Name string `json:"name"`
	// Label is what it's called where it's shown (ws.toml's display_name,
	// or its name), and Order where it comes among them: they're served
	// in it, lowest first.
	Label string   `json:"label"`
	Order int      `json:"order,omitempty"`
	Repos []string `json:"repos"`
	// Hidden is as it is for you, TeamHidden as ws.toml says, and Chosen
	// whether Hidden is your choice rather than the team's.
	Hidden     bool `json:"hidden,omitempty"`
	TeamHidden bool `json:"teamHidden,omitempty"`
	Chosen     bool `json:"chosen,omitempty"`
}

// GroupShownSet is a group shown or hidden for you (POST
// /api/groups/{group}/shown): hidden, or null to go back to the team's.
type GroupShownSet struct {
	Hidden *bool `json:"hidden"`
}

// RepoGroupSet is a repo put in a group (POST /api/repos/{repo}/group):
// "" for none, where it can be.
type RepoGroupSet struct {
	Group string `json:"group"`
}

func (s *Server) groups(w http.ResponseWriter, r *http.Request) {
	got := s.App.Groups()
	out := make([]RepoGroup, 0, len(got))
	for _, g := range got {
		out = append(out, RepoGroup(g))
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": out})
}

func (s *Server) showGroup(w http.ResponseWriter, r *http.Request) {
	var req GroupShownSet
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "hidden must be true, false, or null for the team default")
		return
	}
	done, err := s.App.ShowGroup(r.Context(), ops.AsYou, r.PathValue("group"), req.Hidden, false)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	s.told()
	writeJSON(w, http.StatusOK, map[string]string{"message": done.String()})
}

func (s *Server) groupRepo(w http.ResponseWriter, r *http.Request) {
	var req RepoGroupSet
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "missing group: a name, or \"\" for none")
		return
	}
	done, err := s.App.GroupRepo(r.Context(), ops.AsYou, r.PathValue("repo"), req.Group)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	s.told()
	writeJSON(w, http.StatusOK, map[string]string{"message": done.String()})
}

// told is the workspace's page told at once that its config has changed:
// which repos are shown is the overview's.
func (s *Server) told() {
	if h := s.topics[topicWorkspace]; h != nil {
		h.nudge()
	}
}
