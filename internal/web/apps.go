package web

import (
	"net/http"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/runs"
)

// Apps: the workspace's own recipes with a page (ops.Apps), each shown in
// a frame on a page of its own. Opening the page starts its run, or joins
// the one that's running, and the page says so again every minute while
// it's open (POST …/open), which is what keeps the run: once nobody has
// it open, it's ended. The page points its needs, and starts it again or
// stops it.

// Apping is what an app's page asks of the workspace: the workspace's
// apps, and a repo's worktree's (its tabs on the repo's page).
type Apping interface {
	AppAt(p ops.AppPlace, name string) (ops.AppView, error)
	OpenAppAt(p ops.AppPlace, name string, by ops.Actor) (ops.AppView, error)
	RestartAppAt(p ops.AppPlace, name string, by ops.Actor) (ops.AppView, error)
	StopAppAt(p ops.AppPlace, name string) error
	PointRun(recipe, need, worktree string, by ops.Actor, wait time.Duration) (ops.Pointed, error)
	RunsAt(p ops.AppPlace) (ops.PlaceRuns, error)
	LookedAtPlace(p ops.AppPlace) error
}

// appPlace is where the request's app is: a repo's worktree, for one
// under /api/repos/{repo}/worktrees/{name}, else the workspace.
func appPlace(r *http.Request) ops.AppPlace {
	return ops.AppPlace{Repo: r.PathValue("repo"), Worktree: r.PathValue("worktree")}
}

// AppDetail is GET /api/apps/{name}: an app, what its needs are pointed
// at, and its run as it stands.
type AppDetail struct {
	AppInfo
	Pointers []AppPointer `json:"pointers"`
	// Run is its run: the one that's running, or the one that ended last.
	Run *RunDetail `json:"run,omitempty"`
	// Ready is whether the run's up: its recipe's ready path answers, or
	// something listens on its port.
	Ready bool `json:"ready"`
	// Log is the end of its output, while it isn't ready.
	Log string `json:"log,omitempty"`
	// Said is what a change to it did, in a sentence.
	Said string `json:"said,omitempty"`
}

// AppPointer is one of an app's needs and what it's pointed at: a
// worktree of Repo, said as a name has it (a capsule's name, ground or
// silo), "" for the nearest (its ground). Choices are what it can be.
type AppPointer struct {
	Need    string   `json:"need"`
	Repo    string   `json:"repo"`
	Folder  bool     `json:"folder,omitempty"`
	Recipe  string   `json:"recipe,omitempty"`
	At      string   `json:"at"`
	Choices []string `json:"choices"`
}

// AppPoint is POST /api/apps/{name}/point: a need pointed at a worktree,
// as AppPointer says it ("" for the nearest).
type AppPoint struct {
	Need string `json:"need"`
	At   string `json:"at"`
}

func (s *Server) appDetail(v ops.AppView) AppDetail {
	d := AppDetail{AppInfo: AppInfo{Name: v.Name, Title: v.Title, Icon: v.Icon}, Pointers: []AppPointer{}, Ready: v.Ready, Log: v.Log}
	for _, p := range v.Pointers {
		ap := AppPointer{Need: p.Need, Repo: p.Repo, Folder: p.Folder, Recipe: p.Recipe, Choices: []string{}}
		if p.At != "" {
			ap.At = runs.Label(p.At)
		}
		for _, c := range p.Choices {
			ap.Choices = append(ap.Choices, runs.Label(c))
		}
		d.Pointers = append(d.Pointers, ap)
	}
	if v.Run != nil {
		r := RunDetailOf(*v.Run, s.Root, time.Now())
		d.Run = &r
	}
	return d
}

func (s *Server) getApp(w http.ResponseWriter, r *http.Request) {
	v, err := s.App.AppAt(appPlace(r), r.PathValue("name"))
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, s.appDetail(v))
}

func (s *Server) openApp(w http.ResponseWriter, r *http.Request) {
	v, err := s.App.OpenAppAt(appPlace(r), r.PathValue("name"), ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, s.appDetail(v))
}

func (s *Server) restartApp(w http.ResponseWriter, r *http.Request) {
	v, err := s.App.RestartAppAt(appPlace(r), r.PathValue("name"), ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, s.appDetail(v))
}

func (s *Server) stopApp(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.App.StopAppAt(appPlace(r), name); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	v, err := s.App.AppAt(appPlace(r), name)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, s.appDetail(v))
}

func (s *Server) pointApp(w http.ResponseWriter, r *http.Request) {
	var b AppPoint
	if !readBody(w, r, &b) {
		return
	}
	if b.Need == "" {
		jsonError(w, http.StatusBadRequest, "name the need to point")
		return
	}
	name := r.PathValue("name")
	if _, err := s.App.AppAt(ops.AtWorkspace, name); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	done, err := s.App.PointRun(name, b.Need, b.At, ops.AsYou, 0)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	v, err := s.App.AppAt(appPlace(r), name)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	d := s.appDetail(v)
	d.Said = done.String()
	writeJSON(w, http.StatusOK, d)
}

// PlaceRunsInfo is GET /api/repos/{repo}/worktrees/{worktree}/runs: a
// worktree's apps (its tabs), recipes and runs, for its repo's page.
type PlaceRunsInfo struct {
	Apps    []AppInfo    `json:"apps"`
	Recipes []RecipeInfo `json:"recipes"`
	Runs    []RunDetail  `json:"runs"`
}

// RecipeInfo is one of a worktree's recipes: its name, its command, and
// its app's, for one with a page.
type RecipeInfo struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	App     *AppInfo `json:"app,omitempty"`
}

func (s *Server) placeRuns(w http.ResponseWriter, r *http.Request) {
	p := appPlace(r)
	v, err := s.App.RunsAt(p)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	out := PlaceRunsInfo{Apps: []AppInfo{}, Recipes: []RecipeInfo{}, Runs: []RunDetail{}}
	for _, a := range v.Apps {
		out.Apps = append(out.Apps, AppInfo{Name: a.Name, Title: a.Title, Icon: a.Icon})
	}
	for _, rc := range v.Recipes {
		ri := RecipeInfo{Name: rc.Name, Command: rc.Command}
		if rc.App != nil {
			ri.App = &AppInfo{Name: rc.App.Name, Title: rc.App.Title, Icon: rc.App.Icon}
		}
		out.Recipes = append(out.Recipes, ri)
	}
	now := time.Now()
	for _, run := range v.Runs {
		out.Runs = append(out.Runs, RunDetailOf(run, s.Root, now))
	}
	writeJSON(w, http.StatusOK, out)
}

// lookedAtPlace is POST …/looked: a worktree's page is open, which keeps
// what was started from it.
func (s *Server) lookedAtPlace(w http.ResponseWriter, r *http.Request) {
	if err := s.App.LookedAtPlace(appPlace(r)); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"looked": true})
}
