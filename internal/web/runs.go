package web

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/runs"
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

// resyncSilo is POST /api/silos/{repo}/resync: a silo given its target's
// files in full, by the daemon that keeps it.
func (s *Server) resyncSilo(w http.ResponseWriter, r *http.Request) {
	if s.ResyncSilo == nil {
		jsonError(w, http.StatusNotFound, "nothing keeps the silos here: they're kept by the daemon")
		return
	}
	repo := r.PathValue("repo")
	if err := s.ResyncSilo(repo); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"resynced": repo})
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
// missions someone has open, and from the grounds and capsules open on
// the Files page.
func (s *Server) markLooked() {
	t := time.NewTicker(lookedEvery)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-t.C:
			s.App.LookedAt(runs.Looking{Missions: s.missions.open(), Worktrees: s.files.open(now)})
		}
	}
}

// Runs on the Files page: a ground's or capsule's runs and recipes, which
// the page asks for again every few seconds while it's open (each ask
// says it's open), and "Run" starting one there, lasting while someone
// has it open (ops.RunIn). Stop is the Runs page's (POST /api/runs/stop).

// filesOpen is the grounds and capsules open on the Files page, by
// repo/worktree: when each was last asked for its runs.
type filesOpen struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// saw records that repo/name is open now.
func (f *filesOpen) saw(repo, name string, now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.seen == nil {
		f.seen = map[string]time.Time{}
	}
	f.seen[repo+"/"+name] = now
}

// open is those asked for since the last look but one, forgetting the
// rest: a page shut, or gone to another worktree, stops asking.
func (f *filesOpen) open(now time.Time) map[string]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]bool{}
	for k, at := range f.seen {
		if now.Sub(at) > 2*lookedEvery {
			delete(f.seen, k)
			continue
		}
		out[k] = true
	}
	return out
}

// WorktreeRunsInfo is GET /api/repos/{repo}/worktrees/{name}/runs: what's
// running in a ground or capsule, and the names of its ws.repo.toml's
// recipes that run there.
type WorktreeRunsInfo struct {
	Runs    []RunDetail `json:"runs"`
	Recipes []string    `json:"recipes"`
	// Apps are its recipes with a page: each a tab of the worktree's.
	Apps []AppInfo `json:"apps"`
}

// RunInWorktree is POST /api/repos/{repo}/worktrees/{name}/runs: a recipe
// to run there.
type RunInWorktree struct {
	Recipe string `json:"recipe"`
}

func (s *Server) worktreeRuns(w http.ResponseWriter, r *http.Request) {
	repo, name := r.PathValue("repo"), r.PathValue("name")
	in, err := s.App.RunsIn(repo, name)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	now := time.Now()
	s.files.saw(repo, name, now)
	out := WorktreeRunsInfo{Runs: []RunDetail{}, Recipes: in.Recipes, Apps: []AppInfo{}}
	for _, a := range s.App.AppsAt(ops.AppPlace{Repo: repo, Worktree: name}) {
		out.Apps = append(out.Apps, AppInfo{Name: a.Name, Title: a.Title, Icon: a.Icon})
	}
	for _, run := range in.Runs {
		out.Runs = append(out.Runs, RunDetailOf(run, s.Root, now))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) runInWorktree(w http.ResponseWriter, r *http.Request) {
	var b RunInWorktree
	if !readBody(w, r, &b) {
		return
	}
	if b.Recipe == "" {
		jsonError(w, http.StatusBadRequest, "need the recipe")
		return
	}
	repo, name := r.PathValue("repo"), r.PathValue("name")
	run, err := s.App.RunIn(repo, name, b.Recipe, ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	s.files.saw(repo, name, time.Now())
	writeJSON(w, http.StatusOK, RunDetailOf(run, s.Root, time.Now()))
}

// The Runs page: every run in the workspace, wherever it runs and whoever
// it's for, and the names it's reached by (internal/router).

// RunsInfo is GET /api/runs.
type RunsInfo struct {
	Names NamesInfo   `json:"names"`
	Runs  []RunDetail `json:"runs"`
	// Silos are the repos' silos, as the daemon keeping them has them.
	Silos []ops.SiloState `json:"silos"`
}

// NamesInfo is where the workspace's names stand: whether ws.local.toml
// turns them on (On), the name it asks for (Want) and the one it has
// (Name: another's, given a clash, and "" until its daemon's written its
// routes); the machine's router's port (0: none serving, Fallback: 80
// couldn't be had) and the daemon holding it; and the web UI by its name.
type NamesInfo struct {
	On       bool   `json:"on"`
	Want     string `json:"want"`
	Name     string `json:"name,omitempty"`
	Port     int    `json:"port,omitempty"`
	Fallback bool   `json:"fallback,omitempty"`
	HeldBy   int    `json:"heldBy,omitempty"`
	Web      string `json:"web,omitempty"`
}

// namesOf is where the workspace at root's names stand.
func namesOf(app Running, root string) NamesInfo {
	var n NamesInfo
	n.On, n.Want, _ = app.Names()
	n.Port = router.Serving()
	n.Fallback = n.Port == router.Fallback
	if h, ok := router.WhoIs(n.Port); ok && n.Port > 0 {
		n.HeldBy = h.PID
	}
	if !n.On {
		return n
	}
	if name, ok := router.NameOf(root); ok {
		n.Name = name
		if n.Port > 0 {
			n.Web = router.Link(router.Host("", name), n.Port)
		}
	}
	return n
}

func (s *Server) allRuns(w http.ResponseWriter, r *http.Request) {
	list, err := s.App.Runs()
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	now := time.Now()
	out := RunsInfo{Names: namesOf(s.App, s.Root), Runs: []RunDetail{}, Silos: []ops.SiloState{}}
	if s.Silos != nil {
		out.Silos = append(out.Silos, s.Silos()...)
	}
	for _, run := range list {
		out.Runs = append(out.Runs, RunDetailOf(run, s.Root, now))
	}
	writeJSON(w, http.StatusOK, out)
}

// endRun is POST /api/runs/stop: any run ended, or one that's ended
// cleared away, its log with it.
func (s *Server) endRun(w http.ResponseWriter, r *http.Request) {
	var b RunStop
	if !readBody(w, r, &b) {
		return
	}
	if b.Ref == "" {
		jsonError(w, http.StatusBadRequest, "need the run's ref")
		return
	}
	if err := s.App.EndRun(b.Ref); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"stopped": b.Ref})
}

// runLogLines is how much of a run's output the page is shown.
const runLogLines = 400

// RunLog is GET /api/runs/log?ref=…: the end of a run's output.
type RunLog struct {
	Ref string `json:"ref"`
	Log string `json:"log"`
}

func (s *Server) runLog(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		jsonError(w, http.StatusBadRequest, "need the run's ref")
		return
	}
	log, err := s.App.RunLog(ref, runLogLines)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, RunLog{Ref: ref, Log: log})
}

// PRWrite is POST /api/missions/{slug}/pr-write: a capsule whose pull
// request the PR writer writes, for you to read and change before you
// open it. It answers once it's written (PRWritten), which takes a while.
type PRWrite struct {
	Capsule string `json:"capsule"` // repo/name
}

// PRWritten is what the PR writer wrote.
type PRWritten struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (s *Server) writePR(w http.ResponseWriter, r *http.Request) {
	var b PRWrite
	if !readBody(w, r, &b) {
		return
	}
	if b.Capsule == "" {
		jsonError(w, http.StatusBadRequest, "name the capsule, as repo/name")
		return
	}
	o, err := s.App.WritePR(r.PathValue("slug"), b.Capsule, ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, PRWritten{Title: o.Title, Body: o.Body})
}
