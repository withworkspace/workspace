package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/withworkspace/workspace/internal/team"
)

// The Team page: what the people working in the workspace's repos have
// been doing, by day, from what's landed on default branches and from
// pull requests.

// TeamView is GET /api/team.
type TeamView struct {
	*team.Timeline
	// Repos are the repos in it, as ws labels them.
	Repos map[string]TeamRepo `json:"repos"`
	// Stale: git and GitHub haven't been looked at lately (the page's
	// socket looks, and says when there's something new). Error is
	// why they couldn't be.
	Stale bool   `json:"stale,omitempty"`
	Error string `json:"error,omitempty"`
	// Reading: what's asked for hasn't been read yet (nothing has, or not
	// as far back), and is being: the page's told when it has.
	Reading bool `json:"reading,omitempty"`
}

// TeamRepo is how a repo is shown: its name as ws labels it, and its
// colour.
type TeamRepo struct {
	Label string `json:"label"`
	Color string `json:"color,omitempty"`
}

// teamFresh is how long a look answers before it's called stale.
const teamFresh = 5 * time.Minute

func (s *Server) noTeam(w http.ResponseWriter) bool {
	if s.Team == nil {
		jsonError(w, http.StatusNotFound, "the team's events aren't available here")
		return true
	}
	return false
}

// teamView is the timeline from ?since on (today, week, 7d, a date: as
// ws team --since takes it; without one, as far back as is kept). It
// answers from what's kept when it can, so opening the page doesn't wait
// on a fetch: ?refresh looks first.
func (s *Server) teamView(w http.ResponseWriter, r *http.Request) {
	if s.noTeam(w) {
		return
	}
	q := r.URL.Query()
	var since time.Time
	if v := q.Get("since"); v != "" {
		var err error
		if since, err = team.Since(v, s.now()); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	var snap *team.Snapshot
	var err error
	reading := false
	if q.Get("refresh") != "" {
		snap, err = s.Team.Look(true, since)
	} else if snap = s.Team.Peek(); snap == nil || (!since.IsZero() && since.Before(snap.Since)) {
		// Nothing's read till it's asked for, and the asking doesn't wait:
		// it's answered with what there is, the repos are read behind, and
		// the page's told when they have been.
		reading = true
		go func() {
			s.Team.Look(false, since) //nolint:errcheck // the page reads what the look kept, or its error, when told
			if h := s.topics[topicTeam]; h != nil {
				h.nudge()
			}
		}()
		if snap == nil {
			snap = &team.Snapshot{At: s.now(), Since: since}
		}
	}
	if snap == nil {
		msg := "the repos couldn't be read"
		if err != nil {
			msg = err.Error()
		}
		jsonError(w, http.StatusBadGateway, msg)
		return
	}
	out := TeamView{Timeline: s.Team.Timeline(snap, since), Repos: map[string]TeamRepo{}}
	s.Team.Account(out.Timeline, q.Get("since"))
	if err != nil {
		out.Error = err.Error()
	}
	out.Stale = !reading && s.now().Sub(snap.At) > teamFresh
	out.Reading = reading
	for _, repo := range s.App.Repos() {
		out.Repos[repo.Name] = TeamRepo{Label: repo.Label, Color: repo.Color}
	}
	s.Team.NearYou(out.Timeline)
	writeJSON(w, http.StatusOK, out)
}

// teamSummarise is POST /api/team/summarise: have the period's account
// written, from the changes in it that have been read.
func (s *Server) teamSummarise(w http.ResponseWriter, r *http.Request) {
	if s.noTeam(w) {
		return
	}
	var req TeamNarrate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "send the period")
		return
	}
	snap := s.Team.Peek()
	if snap == nil {
		jsonError(w, http.StatusConflict, "nothing's been read yet")
		return
	}
	var since time.Time
	if req.Since != "" {
		var err error
		if since, err = team.Since(req.Since, s.now()); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"started": s.App.Summarise(snap, since, req.Since)})
}

// teamSeen is POST /api/team/seen: you've looked at the page, so what's
// there now isn't new to you next time.
func (s *Server) teamSeen(w http.ResponseWriter, r *http.Request) {
	if s.noTeam(w) {
		return
	}
	if err := s.App.MarkTeamSeen(); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// teamFingerprint says whether there's been a new look. Taking it looks,
// if that hasn't been done lately: an open Team page is what keeps the
// timeline fresh.
// It also has the gists of what's happened lately written, where they
// haven't been: so a gist is written only while someone's there to read
// it.
func (s *Server) teamFingerprint() string {
	snap, _ := s.Team.Look(false, time.Time{})
	s.App.Narrate(snap, nil, time.Time{})
	return s.snapFingerprint(snap)
}

// snapFingerprint changes with each look, and each gist that starts being
// written or is written.
func (s *Server) snapFingerprint(snap *team.Snapshot) string {
	if snap == nil {
		return ""
	}
	return strconv.FormatInt(snap.At.UnixNano(), 16) + "." + strconv.FormatInt(s.Team.NarrativeVersion(), 16)
}

// TeamNarrate is POST /api/team/narrate: have gists written, of one
// strand (again, if it failed) or of every strand in a period that has
// none.
type TeamNarrate struct {
	Strand string `json:"strand"`
	Since  string `json:"since"`
}

func (s *Server) teamNarrate(w http.ResponseWriter, r *http.Request) {
	if s.noTeam(w) {
		return
	}
	var req TeamNarrate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "send what to write: a strand, or a period")
		return
	}
	snap := s.Team.Peek()
	if snap == nil {
		jsonError(w, http.StatusConflict, "nothing's been read yet")
		return
	}
	var strands []string
	since := snap.Since
	if req.Strand != "" {
		strands = []string{req.Strand}
	} else if req.Since != "" {
		var err error
		if since, err = team.Since(req.Since, s.now()); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	n := s.App.Narrate(snap, strands, since)
	writeJSON(w, http.StatusOK, map[string]int{"started": n})
}
