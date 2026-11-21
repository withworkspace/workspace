package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/runs"
)

// needsFixture is a mission (cache) with a capsule in frontend, whose web
// recipe needs backend's server and its own db, and whose admin needs the
// server too; backend has its ground alone.
func needsFixture(t *testing.T) (*Workspace, string) {
	t.Helper()
	if _, err := runs.ReadProcs(); err != nil {
		t.Skipf("processes can't be read here: %v", err)
	}
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache")
	front := filepath.Join(w.WS().RepoDir("frontend"), "cache")
	write(t, filepath.Join(front, "ws.repo.toml"), `
[run.service.web]
command = "echo api=$API_URL arg={api} port={api.port} mine={port} > web.txt; sleep 300"

[run.service.web.needs.api]
run = "backend/server"
env = "API_URL"
mount = "api/"

[run.service.web.needs.db]

[run.service.admin]
command = "sleep 300"

[run.service.admin.needs.api]
run = "backend/server"

[run.service.db]
command = "sleep 300"
`)
	write(t, filepath.Join(w.WS().RepoDir("backend"), ".ground", "ws.repo.toml"), "[run.service.server]\ncommand = \"sleep 300\"\n")
	t.Cleanup(func() {
		list, _ := w.runStore().List()
		for _, r := range list {
			w.runStore().End(r)
		}
	})
	return w, front
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runAsk(dir, recipe string) RunAsk {
	return RunAsk{Repo: "frontend", Worktree: "cache", Dir: dir, Recipe: recipe, Owner: runs.Owner{Kind: runs.ByNone}, TTL: time.Hour, By: "you"}
}

func refsOf(w *Workspace) string {
	list, _ := w.Runs()
	var out []string
	for _, r := range list {
		if r.State == runs.Running {
			out = append(out, r.Ref())
		}
	}
	return strings.Join(out, " ")
}

// A recipe's needs are started before it, each where it's nearest, and
// their addresses handed to it; asked for again, nothing's started twice;
// and each is kept until the last run that needs it has gone.
func TestStartRunStartsWhatItNeeds(t *testing.T) {
	w, front := needsFixture(t)
	up, err := w.StartRun(runAsk(front, "web"))
	if err != nil {
		t.Fatal(err)
	}
	if !up.Started || up.Run.Ref() != "web.cache.frontend" || len(up.Needs) != 2 {
		t.Fatalf("up %+v", up)
	}
	api, db := up.Needs[0], up.Needs[1]
	if api.Run != "server.ground.backend" || !api.Started || api.Where != "backend's ground" || api.Mount != "/api" || api.Env != "API_URL" {
		t.Errorf("api %+v", api)
	}
	if db.Run != "db.cache.frontend" || !db.Started || db.Where != "" {
		t.Errorf("db %+v", db)
	}
	if got := refsOf(w); got != "server.ground.backend db.cache.frontend web.cache.frontend" {
		t.Fatalf("running: %s", got)
	}
	list, _ := w.Runs()
	server := list[0]
	if server.Owner.Kind != runs.ByRun || !server.Hold.IsZero() || strings.Join(server.NeededBy, ",") != "web.cache.frontend" {
		t.Errorf("server: owner %s, hold %s, needed by %v", server.Owner.Kind, server.Hold, server.NeededBy)
	}
	// Its address reached the run, in its environment and its command.
	want := fmt.Sprintf("api=%s arg=%s port=%d mine=%d\n", server.Link(), server.Link(), server.Port, up.Run.Port)
	var got []byte
	for range 100 {
		if got, _ = os.ReadFile(filepath.Join(front, "web.txt")); len(got) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if string(got) != want {
		t.Errorf("web was started with %q, want %q", got, want)
	}
	if len(up.Notes) != 1 || !strings.Contains(up.Notes[0], "api answers at /api of web's own name only where the workspace has names") {
		t.Errorf("notes %q", up.Notes)
	}

	// Again: the same run, and nothing more.
	again, err := w.StartRun(runAsk(front, "web"))
	if err != nil || again.Started || again.Run.PID != up.Run.PID || len(again.Needs) != 2 || again.Needs[0].Started {
		t.Fatalf("again %+v, %v", again, err)
	}
	// Another run needing the server shares it.
	admin, err := w.StartRun(runAsk(front, "admin"))
	if err != nil || len(admin.Needs) != 1 || admin.Needs[0].Started || admin.Needs[0].Run != "server.ground.backend" {
		t.Fatalf("admin %+v, %v", admin, err)
	}
	if got := refsOf(w); got != "server.ground.backend admin.cache.frontend db.cache.frontend web.cache.frontend" {
		t.Fatalf("running: %s", got)
	}

	// web ended: its db goes with it, and the server stays for admin.
	if err := w.EndRun("web.cache.frontend"); err != nil {
		t.Fatal(err)
	}
	if got := refsOf(w); got != "server.ground.backend admin.cache.frontend" {
		t.Fatalf("after web: %s", got)
	}
	if err := w.EndRun("admin.cache.frontend"); err != nil {
		t.Fatal(err)
	}
	if got := refsOf(w); got != "" {
		t.Fatalf("after admin: %s", got)
	}
}

// In another repo, the nearest is the mission's capsule there; --with
// says another, or an address with nothing of ws's behind it.
func TestStartRunFindsTheNearest(t *testing.T) {
	w, front := needsFixture(t)
	mine := filepath.Join(w.WS().RepoDir("backend"), "cache-api")
	write(t, filepath.Join(mine, "ws.repo.toml"), "[run.service.server]\ncommand = \"sleep 300\"\n")
	if _, err := w.Attach("cache", "backend", "cache-api", false, AsYou); err != nil {
		t.Fatal(err)
	}
	up, err := w.StartRun(runAsk(front, "admin"))
	if err != nil || up.Needs[0].Run != "server.cache-api.backend" || up.Needs[0].Where != "mission cache's capsule in backend" {
		t.Fatalf("up %+v, %v", up, err)
	}
	w.EndRun(up.Run.Ref())

	a := runAsk(front, "admin")
	a.With = map[string]string{"api": "ground"}
	up, err = w.StartRun(a)
	if err != nil || up.Needs[0].Run != "server.ground.backend" {
		t.Fatalf("with ground: %+v, %v", up, err)
	}
	// Running already, --with changes nothing, and says so.
	a.With = map[string]string{"api": "cache-api"}
	again, err := w.StartRun(a)
	if err != nil || again.Started || len(again.Notes) != 1 || !strings.Contains(again.Notes[0], "--with is for when it's started") {
		t.Fatalf("again: %+v, %v", again, err)
	}
	w.EndRun(up.Run.Ref())

	a.With = map[string]string{"api": "https://staging.example/v1/"}
	up, err = w.StartRun(a)
	if err != nil || up.Needs[0].Run != "" || up.Needs[0].Addr != "https://staging.example/v1" {
		t.Fatalf("with an address: %+v, %v", up, err)
	}
	if got := refsOf(w); got != "admin.cache.frontend" {
		t.Errorf("running: %s, want admin alone", got)
	}
	w.EndRun(up.Run.Ref())

	for with, want := range map[string]string{"nope": "backend has no nope to run it in", "silo": "backend has no silo to run it in"} {
		a.With = map[string]string{"api": with}
		if _, err := w.StartRun(a); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("with %s: %v", with, err)
		}
	}
	a.With = map[string]string{"cache": "ground"}
	if _, err := w.StartRun(a); err == nil || !strings.Contains(err.Error(), "admin has no need called cache (its needs: api)") {
		t.Errorf("a need it hasn't: %v", err)
	}
	if got := refsOf(w); got != "" {
		t.Errorf("left running: %s", got)
	}
}

func TestStartRunSaysWhatsWrongWithANeed(t *testing.T) {
	w, front := needsFixture(t)
	write(t, filepath.Join(front, "ws.repo.toml"), `
[run.service.a]
command = "sleep 300"
[run.service.a.needs.b]

[run.service.b]
command = "sleep 300"
[run.service.b.needs.again]
run = "a"

[run.service.lost]
command = "sleep 300"
[run.service.lost.needs.api]
run = "nowhere/server"

[run.service.norecipe]
command = "sleep 300"
[run.service.norecipe.needs.api]
run = "backend/worker"

[run.service.dies]
command = "sleep 300"
[run.service.dies.needs.flaky]

[run.service.flaky]
command = "echo no database; exit 3"

[run.service.port]
command = "sleep 300"
[run.service.port.needs.port]
run = "b"
`)
	for recipe, want := range map[string]string{
		"a":        "frontend/cache/a needs itself: frontend/cache/a, which needs frontend/cache/b, which needs frontend/cache/a",
		"lost":     "the workspace has no repo nowhere",
		"norecipe": "backend/ground has no recipe worker ([run.service.worker] in its ws.repo.toml)",
		"port":     "a need's name is lower case letters, numbers, - and _, and not port",
		"nope":     "frontend/cache has no recipe nope",
	} {
		if _, err := w.StartRun(runAsk(front, recipe)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", recipe, err, want)
		}
	}
	// A need that ends before it's up stops the run that needs it from
	// starting, and is the run the error is about.
	a := runAsk(front, "dies")
	a.Wait = 5 * time.Second
	_, err := w.StartRun(a)
	var ended *runs.EndedError
	if !errors.As(err, &ended) || ended.Run.Name != "flaky" || ended.Run.Status != 3 || !strings.Contains(err.Error(), "flaky, which dies needs") {
		t.Fatalf("a need that died: %v", err)
	}
	if got := refsOf(w); got != "" {
		t.Errorf("running: %s", got)
	}
	a = runAsk(front, "x")
	a.Recipe, a.Command, a.With = "", "sleep 300", map[string]string{"api": "ground"}
	if _, err := w.StartRun(a); err == nil || !strings.Contains(err.Error(), "a command has none") {
		t.Errorf("--with and a command: %v", err)
	}
}

// What a run needs, ended under it, is started again when the run is
// asked for again.
func TestStartRunAgainStartsWhatEnded(t *testing.T) {
	w, front := needsFixture(t)
	up, err := w.StartRun(runAsk(front, "admin"))
	if err != nil {
		t.Fatal(err)
	}
	list, _ := w.Runs()
	if err := w.runStore().End(list[0]); err != nil || list[0].Name != "server" {
		t.Fatal(err, list[0].Name)
	}
	again, err := w.StartRun(runAsk(front, "admin"))
	if err != nil || again.Started || again.Run.PID != up.Run.PID || !again.Needs[0].Started {
		t.Fatalf("again %+v, %v", again, err)
	}
	if got := refsOf(w); got != "server.ground.backend admin.cache.frontend" {
		t.Errorf("running: %s", got)
	}
}

// From a mission's page, a recipe's needs come with it, and a run that's
// there already is the lookers' as well as whoever started it.
func TestRunRecipeFromThePageSharesARun(t *testing.T) {
	w, front := needsFixture(t)
	old := RunUpWait
	RunUpWait = 100 * time.Millisecond // sleep never listens
	t.Cleanup(func() { RunUpWait = old })
	a := runAsk(front, "admin")
	a.Owner = runs.Owner{Kind: runs.ByPID, PID: os.Getpid()}
	if _, err := w.StartRun(a); err != nil {
		t.Fatal(err)
	}
	r, err := w.RunRecipe("cache", "frontend", "cache", "admin", AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if r.Owner.Kind != runs.ByPID || len(r.Also) != 1 || r.Also[0].Kind != runs.ByLooking || r.Also[0].Mission != "cache" {
		t.Errorf("owners %+v, also %+v", r.Owner, r.Also)
	}
	if got := refsOf(w); got != "server.ground.backend admin.cache.frontend" {
		t.Errorf("running: %s", got)
	}
}
