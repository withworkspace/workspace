package web

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"maps"
	"net/http"
	"os"
	"strconv"
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
		jsonError(w, http.StatusNotFound, "silos are not available here")
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
		jsonError(w, http.StatusBadRequest, "missing capsule (repo and name) or recipe")
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
		jsonError(w, http.StatusBadRequest, "missing run ref")
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
			open := s.files.open(now)
			maps.Copy(open, s.filesFollowed.open()) // followed over a socket
			if err := s.App.LookedAt(runs.Looking{Missions: s.missions.open(), Worktrees: open}); err != nil {
				s.Log.Printf("[web] couldn't note what's looked at: %v", err)
			}
			for key := range s.apps.open() {
				if name, p, ok := appOfKey(key); ok {
					if err := s.App.LookedAtApp(p, name); err != nil {
						s.Log.Printf("[web] couldn't note %s looked at: %v", name, err)
					}
				}
			}
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
// running in a ground or capsule, and its ws.repo.toml's recipes that run
// there.
type WorktreeRunsInfo struct {
	Runs    []RunDetail  `json:"runs"`
	Recipes []RecipeInfo `json:"recipes"`
	// Apps are its recipes with a page: each a tab of the worktree's.
	Apps []AppInfo `json:"apps"`
	// Past are the earlier runs of its tasks, kept behind each one's
	// latest (in Runs): with Runs, what's been run there lately.
	Past []RunDetail `json:"past"`
}

// RecipeInfo is one of a place's recipes, as its Runs tab lists it: its
// name, what it's called (its display_name, or its page's title: "" for
// its name), its type and command. A service's Kind is what it's for, an
// app or a tool, and its Render how it's shown; Page, that it's an app
// with a tab of its own on the worktree (in Apps).
type RecipeInfo struct {
	Name    string       `json:"name"`
	Title   string       `json:"title,omitempty"`
	Type    runs.Type    `json:"type"`
	Kind    RecipeKind   `json:"kind,omitempty"`
	Render  RecipeRender `json:"render,omitempty"`
	Command string       `json:"command"`
	Page    bool         `json:"page,omitempty"`
}

// RecipeInfoOf is one of ops' recipes as the wire has it.
func RecipeInfoOf(rc ops.Recipe) RecipeInfo {
	t := runs.Service
	if rc.Task {
		t = runs.Task
	}
	return RecipeInfo{Name: rc.Name, Title: rc.Called, Type: t, Kind: RecipeKind(rc.Kind), Render: RecipeRender(rc.Render), Command: rc.Command, Page: rc.Page}
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
	out := WorktreeRunsInfo{Runs: []RunDetail{}, Recipes: []RecipeInfo{}, Apps: []AppInfo{}, Past: []RunDetail{}}
	for _, rc := range in.Recipes {
		out.Recipes = append(out.Recipes, RecipeInfoOf(rc))
	}
	for _, a := range s.App.AppsAt(ops.AppPlace{Repo: repo, Worktree: name}) {
		out.Apps = append(out.Apps, AppInfoOf(a))
	}
	for _, run := range in.Runs {
		out.Runs = append(out.Runs, RunDetailOf(run, s.Root, now))
	}
	for _, run := range in.Past {
		out.Past = append(out.Past, RunDetailOf(run, s.Root, now))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) runInWorktree(w http.ResponseWriter, r *http.Request) {
	var b RunInWorktree
	if !readBody(w, r, &b) {
		return
	}
	if b.Recipe == "" {
		jsonError(w, http.StatusBadRequest, "missing recipe")
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
	data, err := s.views.runs(s)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// runsInfo is what GET /api/runs serves: every run, read in full (the
// process table too), which the runs' stamp says when to do again.
func (s *Server) runsInfo() (RunsInfo, error) {
	list, err := s.App.Runs()
	if err != nil {
		return RunsInfo{}, err
	}
	now := time.Now()
	out := RunsInfo{Names: namesOf(s.App, s.Root), Runs: []RunDetail{}, Silos: s.siloStates()}
	for _, run := range list {
		out.Runs = append(out.Runs, RunDetailOf(run, s.Root, now))
	}
	return out, nil
}

// siloStates is where the silos the daemon keeps stand.
func (s *Server) siloStates() []ops.SiloState {
	out := []ops.SiloState{}
	if s.Silos != nil {
		out = append(out, s.Silos()...)
	}
	return out
}

// runsStamp names what the Runs page is built from, cheaply: the runs'
// stamp, and where the silos stand.
func (s *Server) runsStamp() string {
	silos, _ := json.Marshal(s.siloStates())
	return s.App.RunsStamp() + " " + string(silos)
}

// runsFingerprint says whether what the Runs page shows has changed.
func (s *Server) runsFingerprint() string {
	data, err := s.views.runs(s)
	if err != nil {
		return "error: " + err.Error()
	}
	h := fnv.New64a()
	h.Write(data)
	return strconv.FormatUint(h.Sum64(), 16)
}

// endRun is POST /api/runs/stop: any run ended, or one that's ended
// cleared away, its log with it.
func (s *Server) endRun(w http.ResponseWriter, r *http.Request) {
	var b RunStop
	if !readBody(w, r, &b) {
		return
	}
	if b.Ref == "" {
		jsonError(w, http.StatusBadRequest, "missing run ref")
		return
	}
	if err := s.App.EndRun(b.Ref); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"stopped": b.Ref})
}

// RunPast is GET /api/runs/past?ref=…: a task's run, and the earlier runs
// of it kept behind it, the latest first.
type RunPast struct {
	Run  RunDetail   `json:"run"`
	Past []RunDetail `json:"past"`
}

func (s *Server) runPast(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		jsonError(w, http.StatusBadRequest, "missing run ref")
		return
	}
	run, past, err := s.App.Run(ref)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	now := time.Now()
	out := RunPast{Run: RunDetailOf(run, s.Root, now), Past: []RunDetail{}}
	for _, p := range past {
		out.Past = append(out.Past, RunDetailOf(p, s.Root, now))
	}
	writeJSON(w, http.StatusOK, out)
}

// runLogLines is how much of a run's output the page is shown.
const runLogLines = 400

// RunLog is GET /api/runs/log?ref=…: the end of a run's output.
type RunLog struct {
	Ref string `json:"ref"`
	Log string `json:"log"`
	// Size is how much of the log it's as of: a page following it is told
	// what's written after (RunLogOp).
	Size int64 `json:"size"`
}

func (s *Server) runLog(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		jsonError(w, http.StatusBadRequest, "missing run ref")
		return
	}
	log, size, _, err := s.App.RunLogSized(ref, runLogLines)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, RunLog{Ref: ref, Log: log, Size: size})
}

// RunLogOp is what's been written to a run's log that a page follows
// (runlog:<ref>), told over the socket so the page needn't read it again:
// the text from From (as much of it as the page has) to Size. A page that
// holds less, or more, reads it whole.
type RunLogOp struct {
	From int64  `json:"from"`
	Size int64  `json:"size"`
	Text string `json:"text"`
}

// runLogMost is the most of a log a change says: more written at once is
// read whole by the page (its last lines).
const runLogMost = 256 << 10

// runLogHub follows runs' logs (runlog:<ref>): each looked at every
// transcriptPoll while it's followed (a stat), and what's been written
// since read from where the last look got to.
func (s *Server) runLogHub() *keyedHub {
	type follow struct {
		path string
		size int64
	}
	var mu sync.Mutex
	paths := map[string]string{}
	h := newKeyedHub(runLogTopic, transcriptPoll, func(ref string) string {
		mu.Lock()
		path := paths[ref]
		mu.Unlock()
		info, err := os.Stat(path)
		if path == "" || err != nil {
			return ""
		}
		return fmt.Sprintf("%d %d", info.Size(), info.ModTime().UnixNano())
	})
	h.pathOf = func(ref string) string {
		mu.Lock()
		defer mu.Unlock()
		return paths[ref]
	}
	h.gone = func(ref string) {
		mu.Lock()
		delete(paths, ref)
		mu.Unlock()
	}
	h.change = func(ref string, was any) (any, any) {
		f, _ := was.(*follow)
		if f == nil {
			_, size, path, err := s.App.RunLogSized(ref, 0)
			if err != nil {
				return nil, &follow{}
			}
			mu.Lock()
			paths[ref] = path
			mu.Unlock()
			return nil, &follow{path: path, size: size}
		}
		if f.path == "" {
			return nil, f
		}
		text, size, more, gone, err := runs.ReadFrom(f.path, f.size, runLogMost)
		switch {
		case err != nil:
			return reread, f // ended and cleared away: the page reads why
		case gone || more:
			f.size = size
			return reread, f
		case size == f.size:
			return nil, f
		}
		op := &RunLogOp{From: f.size, Size: size, Text: text}
		f.size = size
		return op, f
	}
	return h
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
		jsonError(w, http.StatusBadRequest, "missing capsule: repo/name")
		return
	}
	o, err := s.App.WritePR(r.PathValue("slug"), b.Capsule, ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, PRWritten{Title: o.Title, Body: o.Body})
}
