package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
)

// Runs on a mission's page: "Run it" starts one of a capsule's recipes,
// lasting while someone has the mission open (ops.RunRecipe), and Stop
// ends a run. Every lookedEvery, the runs started from the missions open
// in a page are marked as looked at.

// RunStart is POST /api/missions/{slug}/runs: a capsule's recipe to run.
type RunStart struct {
	Repo    string `json:"repo"`
	Capsule string `json:"capsule"`
	Recipe  string `json:"recipe"`
}

// SiloPoint is POST /api/silos/{repo}/point: the capsule to point a repo's
// silo at, from the page ws's router sends you to.
type SiloPoint struct {
	To string `json:"to"`
}

func (s *Server) pointSilo(w http.ResponseWriter, r *http.Request) {
	var b SiloPoint
	if !readBody(w, r, &b) {
		return
	}
	var said strings.Builder
	if err := s.App.PointSilo(r.PathValue("repo"), b.To, &said); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"said": said.String()})
}

// RunStop is POST /api/missions/{slug}/runs/stop: a run to end.
type RunStop struct {
	Ref string `json:"ref"`
}

func (s *Server) startRun(w http.ResponseWriter, r *http.Request) {
	var b RunStart
	if !readBody(w, r, &b) {
		return
	}
	if b.Repo == "" || b.Capsule == "" || b.Recipe == "" {
		jsonError(w, http.StatusBadRequest, "need the capsule's repo and name, and the recipe")
		return
	}
	run, err := s.App.RunRecipe(r.PathValue("slug"), b.Repo, b.Capsule, b.Recipe, ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, RunDetailOf(run, s.Root, time.Now()))
}

func (s *Server) stopRun(w http.ResponseWriter, r *http.Request) {
	var b RunStop
	if !readBody(w, r, &b) {
		return
	}
	if b.Ref == "" {
		jsonError(w, http.StatusBadRequest, "need the run's ref")
		return
	}
	if err := s.App.StopRun(r.PathValue("slug"), b.Ref); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"stopped": b.Ref})
}

// lookedEvery is how often the runs of missions open in a page are marked
// as looked at: well inside runs.LookLinger.
const lookedEvery = time.Minute

// markLooked marks, until the server's done, the runs started from the
// missions someone has open.
func (s *Server) markLooked() {
	t := time.NewTicker(lookedEvery)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
			if open := s.missions.open(); len(open) > 0 {
				s.App.LookedAt(open)
			}
		}
	}
}
