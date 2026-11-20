package cli

import (
	"errors"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/web"
	"github.com/withworkspace/workspace/internal/workspace"
)

// serverWorkspace is a workspace with the capsules given, by repo, as
// folders (no git): enough for crew to be placed and run in.
func serverWorkspace(t *testing.T, capsules map[string][]string) *workspace.Workspace {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	ws := &workspace.Workspace{
		Root:         root,
		Org:          "acme",
		DisplayNames: map[string]string{"frontend": "Frontend", "backend": "API"},
	}
	for repo, names := range capsules {
		ws.RepoNames = append(ws.RepoNames, repo)
		os.MkdirAll(filepath.Join(ws.RepoDir(repo), workspace.GroundDir), 0o755)
		for _, n := range names {
			os.MkdirAll(filepath.Join(ws.RepoDir(repo), n), 0o755)
		}
	}
	return ws
}

// seedMission makes a mission with a brief and capsules, as ws mission new
// and attach do, and returns its slug.
func seedMission(t *testing.T, ws *workspace.Workspace, title, brief string, capsules ...[2]string) string {
	t.Helper()
	store := mission.Store{Root: ws.Root}
	m, err := store.Create(title)
	if err != nil {
		t.Fatal(err)
	}
	if brief != "" {
		os.WriteFile(store.BriefPath(m.Slug), []byte(brief), 0o644)
	}
	for _, c := range capsules {
		if _, err := ops.New(&config.Config{}, ws, nil, ops.Options{}).Attach(m.Slug, c[0], c[1], false, ops.AsYou); err != nil {
			t.Fatal(err)
		}
	}
	return m.Slug
}

// shellCrew runs crew as a plain shell, with a prompt to wait for.
func shellCrew(t *testing.T) {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("ENV", "")
	t.Setenv("PS1", "$ ")
	t.Setenv(harness.EnvDefault, "shell")
}

// startServer runs the workspace's daemon in this process.
func startServer(t *testing.T, root string) *daemon.Server {
	t.Helper()
	srv := &daemon.Server{Root: root, IdleTimeout: time.Minute}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Shutdown("test over") })
	return srv
}

// waitScreen waits for running crew's screen to show want.
func waitScreen(t *testing.T, srv *daemon.Server, key, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		if f, ok := srv.Screen(key); ok {
			if last = f.Screen; strings.Contains(last, want) {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s's screen never showed %q:\n%s", key, want, last)
}

// launchFromWeb starts crew on the mission as the web UI's Launch does.
func launchFromWeb(t *testing.T, ws *workspace.Workspace, srv *daemon.Server, slug string) {
	t.Helper()
	os.WriteFile(filepath.Join(ws.Root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n\n[repos.frontend]\n"), 0o644)
	if said, err := webActIn(ws.Root)(slug, web.ActRequest{Verb: "launch"}); err != nil {
		t.Fatalf("launch: %q, %v", said, err)
	}
	waitScreen(t, srv, slug+"/c1", "$")
}

// The orchestrator's resume-with reaches crew still running in the server:
// the message is typed straight in, as an opening prompt is.
func TestNudgeCrew_TypesIntoRunningCrew(t *testing.T) {
	shellCrew(t)
	old := crew.PromptSettle
	crew.PromptSettle = 200 * time.Millisecond
	t.Cleanup(func() { crew.PromptSettle = old })
	ws := serverWorkspace(t, map[string][]string{"frontend": {"work"}})
	slug := seedMission(t, ws, "Served", "", [2]string{"frontend", "work"})
	srv := startServer(t, ws.Root)
	launchFromWeb(t, ws, srv, slug)

	// From a command: over the daemon's socket.
	host := daemon.NewHost(ws.Root)
	defer host.Leave()
	if _, err := host.Say(slug+"/c1", "echo nudged-in"); err != nil {
		t.Fatal(err)
	}
	waitScreen(t, srv, slug+"/c1", "\nnudged-in")
	if _, err := host.Say(slug+"/c9", "hello"); !errors.Is(err, crew.ErrNotRunning) {
		t.Errorf("typed into crew that isn't running: %v", err)
	}
}

// The web UI types into crew in the server through the server's own key
// encoder; Enter counts as you talking to it.
func TestWebTerminals_TypeIntoCrew(t *testing.T) {
	shellCrew(t)
	ws := serverWorkspace(t, map[string][]string{"frontend": {"work"}})
	slug := seedMission(t, ws, "Served", "", [2]string{"frontend", "work"})
	srv := startServer(t, ws.Root)
	launchFromWeb(t, ws, srv, slug)

	terms := ops.New(&config.Config{}, ws, nil, ops.Options{Host: srv}).Crew()
	for _, k := range "echo Typed" {
		if err := terms.Key(slug, "c1", ops.TermKey{Key: string(k), Shift: k == 'T'}); err != nil {
			t.Fatal(err)
		}
	}
	terms.Key(slug, "c1", ops.TermKey{Key: "Enter"})
	waitScreen(t, srv, slug+"/c1", "\nTyped")
	if sc, ok := terms.Screen(slug, "c1"); !ok || !strings.Contains(sc.Text, "\nTyped") || sc.Width == 0 {
		t.Errorf("screen = %+v", sc)
	}
	if got, _ := (mission.Store{Root: ws.Root}).Load(slug); !got.Crew[0].Conversing {
		t.Error("Enter from the web UI isn't you talking to the crew")
	}
	if err := terms.Key(slug, "c1", ops.TermKey{Key: "Unidentified"}); err == nil {
		t.Error("typed a key with no name")
	}
}

// From the web UI, crew start in the server: new crew, the queued crew;
// and stop there.
func TestWebAct_LaunchesAndStopsCrew(t *testing.T) {
	shellCrew(t)
	ws := serverWorkspace(t, map[string][]string{"frontend": {"one", "two"}})
	// The web UI loads the workspace from its config, as ws does.
	os.WriteFile(filepath.Join(ws.Root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n\n[repos.frontend]\n"), 0o644)
	slug := seedMission(t, ws, "Served", "", [2]string{"frontend", "one"}, [2]string{"frontend", "two"})
	srv := startServer(t, ws.Root)
	store := mission.Store{Root: ws.Root}
	act := webActIn(ws.Root)
	running := func(id string) bool { _, ok := srv.Screen(slug + "/" + id); return ok }

	if said, err := act(slug, web.ActRequest{Verb: "launch"}); err != nil || !strings.HasPrefix(said, "Started c1 in frontend/") {
		t.Fatalf("launch: %q, %v", said, err)
	}
	if !running("c1") {
		t.Fatal("c1 isn't running in the server")
	}

	store.QueueDispatch(slug, &mission.Dispatch{Text: "write the tests", By: "orchestrator"})
	if said, err := act(slug, web.ActRequest{Verb: "launch-queued"}); err != nil || !strings.HasPrefix(said, "Started c2") {
		t.Fatalf("launch queued: %q, %v", said, err)
	}
	m, _ := store.Load(slug)
	if m.Dispatch != nil || m.Crew[1].Dispatch != "write the tests" || m.Crew[0].Capsule == m.Crew[1].Capsule {
		t.Errorf("after launching the queued crew: dispatch %+v, crew %+v", m.Dispatch, m.Crew)
	}
	if _, err := act(slug, web.ActRequest{Verb: "launch"}); err == nil || !strings.Contains(err.Error(), "every capsule") {
		t.Errorf("launched into a capsule crew are working in: %v", err)
	}

	if _, err := act(slug, web.ActRequest{Verb: "stop", Crew: "c1"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for running("c1") && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if running("c1") {
		t.Fatal("c1 still running after stop")
	}
	if m, _ := store.Load(slug); m.Crew[0].Ended == nil || !m.Crew[0].Stopped {
		t.Errorf("c1's end: %+v", m.Crew[0])
	}
	if _, err := act(slug, web.ActRequest{Verb: "resume", Crew: "c1"}); err == nil || !strings.Contains(err.Error(), "no conversation") {
		t.Errorf("resumed crew with no conversation: %v", err)
	}
	if _, err := act(slug, web.ActRequest{Verb: "brief"}); err == nil {
		t.Error("briefed a mission that's briefed")
	}
}

// pickerHarness starts crew that say the model they were started on.
type pickerHarness struct{}

func (pickerHarness) Name() string     { return "picker" }
func (pickerHarness) Available() error { return nil }
func (pickerHarness) Start(b harness.Brief) (harness.Launch, error) {
	return harness.Launch{Args: []string{"/bin/sh", "-c", `printf 'model=%s\n' "$0"; sleep 5`, firstNonEmpty(b.Model, "none")}, Dir: b.Dir}, nil
}
func (pickerHarness) Resume(string, string) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}

// Launch crew from the web UI starts crew on the default, or on a harness
// and model [crew] allowed lists; anything else is refused. The mission's
// page is told what it may offer.
func TestWebAct_LaunchesOnAPick(t *testing.T) {
	shellCrew(t)
	t.Setenv(harness.EnvDefault, "") // crew's harness is the config's
	harness.Register(pickerHarness{})
	ws := serverWorkspace(t, map[string][]string{"frontend": {"one", "two", "three"}})
	writeConfig := func(allowed string) {
		os.WriteFile(filepath.Join(ws.Root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n\n[repos.frontend]\n\n[crew]\nharness = \"shell\"\n"+allowed), 0o644)
	}
	writeConfig("allowed = [\"picker:big\", \"picker:small\", \"shell\"]\n")
	slug := seedMission(t, ws, "Served", "", [2]string{"frontend", "one"}, [2]string{"frontend", "two"}, [2]string{"frontend", "three"})
	srv := startServer(t, ws.Root)
	store := mission.Store{Root: ws.Root}
	act := webActIn(ws.Root)

	// The page offers the default, then the others (shell alone is the
	// default, so it's said once).
	d, err := web.MissionOf(openOps(ws.Root, nil), ws.Root, slug)
	if err != nil {
		t.Fatal(err)
	}
	if p := d.CrewPicks; p.Default != (web.Pick{Harness: "shell"}) || len(p.Allowed) != 2 || p.Allowed[0] != (web.Pick{Harness: "picker", Model: "big"}) || p.Allowed[1] != (web.Pick{Harness: "picker", Model: "small"}) {
		t.Errorf("crew picks = %+v", p)
	}

	if said, err := act(slug, web.ActRequest{Verb: "launch", Harness: "picker", Model: "small"}); err != nil || !strings.HasPrefix(said, "Started c1") {
		t.Fatalf("launch on a pick: %q, %v", said, err)
	}
	waitScreen(t, srv, slug+"/c1", "model=small")
	if m, _ := store.Load(slug); m.Crew[0].Harness != "picker" || m.Crew[0].Model != "small" || m.Crew[0].Agent() != "picker on small" {
		t.Errorf("c1 = %+v", m.Crew[0])
	}

	for _, p := range []web.ActRequest{
		{Harness: "picker", Model: "huge"},
		{Harness: "picker"},
		{Harness: "claude", Model: "opus"},
		{Model: "small"},
	} {
		p.Verb = "launch"
		_, err := act(slug, p)
		var refused *ops.Refused
		var invalid *ops.Invalid
		if !errors.As(err, &refused) && !errors.As(err, &invalid) {
			t.Errorf("launched on %s:%s: %v", p.Harness, p.Model, err)
		}
	}
	if m, _ := store.Load(slug); len(m.Crew) != 1 {
		t.Errorf("crew started on a pick that isn't allowed: %+v", m.Crew)
	}

	// No pick, or the default's: the default, as before.
	if said, err := act(slug, web.ActRequest{Verb: "launch"}); err != nil || !strings.HasPrefix(said, "Started c2") {
		t.Fatalf("launch: %q, %v", said, err)
	}
	if said, err := act(slug, web.ActRequest{Verb: "launch", Harness: "shell"}); err != nil || !strings.HasPrefix(said, "Started c3") {
		t.Fatalf("launch on the default: %q, %v", said, err)
	}
	if m, _ := store.Load(slug); m.Crew[1].Harness != "shell" || m.Crew[1].Model != "" || m.Crew[1].Agent() != "shell" || m.Crew[2].Harness != "shell" {
		t.Errorf("default crew = %+v", m.Crew[1:])
	}

	// With nothing allowed, there's only the default.
	writeConfig("")
	store.EndCrew(slug, "c1", "exit 0")
	if d, _ := web.MissionOf(openOps(ws.Root, nil), ws.Root, slug); d.CrewPicks.Default.Harness != "shell" || len(d.CrewPicks.Allowed) != 0 {
		t.Errorf("crew picks with none allowed = %+v", d.CrewPicks)
	}
	var refused *ops.Refused
	if _, err := act(slug, web.ActRequest{Verb: "launch", Harness: "picker", Model: "small"}); !errors.As(err, &refused) || refused.Code != "not-allowed" || !strings.Contains(err.Error(), "crew may run on shell") {
		t.Errorf("launched on a pick no longer allowed: %v", err)
	}
}

// Once a stage is accepted, ws moves the plan on: a stage that says to
// check in asks you first (Not yet leaves it, Go on starts the next), and
// otherwise the next stage starts in the capsule it names.
func TestAfterAccept_MovesThePlanOn(t *testing.T) {
	shellCrew(t)
	ws := serverWorkspace(t, map[string][]string{"frontend": {"proposal", "build", "docs"}})
	os.WriteFile(filepath.Join(ws.Root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n\n[repos.frontend]\n"), 0o644)
	slug := seedMission(t, ws, "Staged", "## Stages\n1. Propose it in `frontend/proposal`. Check in with me.\n2. Build it in `frontend/build`.\n3. Document it in `frontend/docs`.\n",
		[2]string{"frontend", "proposal"}, [2]string{"frontend", "build"}, [2]string{"frontend", "docs"})
	srv := startServer(t, ws.Root)
	ctx, err := LoadOps(ws.Root)
	if err != nil {
		t.Fatal(err)
	}
	store := mission.Store{Root: ws.Root}
	accept := func() string {
		t.Helper()
		acc, err := ctx.AcceptStage(slug, ops.AsYou, "good")
		if err != nil {
			t.Fatal(err)
		}
		return acc.Next
	}

	// Stage 1 says to check in: you're asked, with go on and not yet.
	if said := accept(); !strings.Contains(said, "the user is asked before stage 2") {
		t.Fatalf("after stage 1: %s", said)
	}
	m, _ := store.Load(slug)
	if f := m.Asking(); f == nil || f.By != "ws" || f.Step != 2 || strings.Join(f.Options, "|") != "Go on|Not yet" || !strings.Contains(f.Text, "Go on to stage 2: Build it in") {
		t.Fatalf("check-in = %+v", f)
	}
	if a, err := ctx.Answer(slug, ops.AsYou, "Not yet"); err != nil || !strings.Contains(a.Next, "it waits for you") {
		t.Errorf("not yet: %q, %v", a.Next, err)
	}
	if m, _ := store.Load(slug); m.Asking() != nil || len(m.Crew) != 0 {
		t.Errorf("not yet started something: %+v", m)
	}

	store.AskAtStep(slug, mission.TaskApprove, "Go on to stage 2?", 2)
	if a, err := ctx.Answer(slug, ops.AsYou, "Go on"); err != nil || !strings.Contains(a.Next, "started stage 2: c1 in frontend/build") {
		t.Fatalf("go on: %q, %v", a.Next, err)
	}
	m, _ = store.Load(slug)
	if len(m.Crew) != 1 || m.Crew[0].Capsule != "frontend/build" || !strings.Contains(m.Crew[0].Dispatch, "Build it in") {
		t.Errorf("stage 2's crew: %+v", m.Crew)
	}
	if all, _ := store.Decisions(slug); len(all) != 0 {
		t.Errorf("a check-in's go on and not yet aren't decisions: %+v", all)
	}

	// Stage 2 doesn't check in: accepting it starts stage 3 straight away.
	store.SetCrewStatus(slug, "c1", mission.CrewDone, "built")
	srv.Stop(slug + "/c1")
	if said := accept(); !strings.Contains(said, "started stage 3: c2 in frontend/docs") {
		t.Errorf("after stage 2: %s", said)
	}
	store.SetCrewStatus(slug, "c2", mission.CrewDone, "documented")
	if said := accept(); said != "that was the last stage" {
		t.Errorf("after the last stage: %s", said)
	}
}

// recordingHarness runs a plain shell, and records the brief it was given.
type recordingHarness struct{ name string }

var (
	recordedMu     sync.Mutex
	recordedBriefs = map[string][]harness.Brief{}
)

func (h recordingHarness) Name() string     { return h.name }
func (h recordingHarness) Available() error { return nil }
func (h recordingHarness) Start(b harness.Brief) (harness.Launch, error) {
	recordedMu.Lock()
	recordedBriefs[h.name] = append(recordedBriefs[h.name], b)
	recordedMu.Unlock()
	return harness.Launch{Args: []string{"/bin/sh", "-i"}, Dir: b.Dir}, nil
}
func (h recordingHarness) Resume(string, string) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}

func lastBrief(t *testing.T, name string) harness.Brief {
	t.Helper()
	recordedMu.Lock()
	defer recordedMu.Unlock()
	bs := recordedBriefs[name]
	if len(bs) == 0 {
		t.Fatalf("%s never launched", name)
	}
	return bs[len(bs)-1]
}

// "Ask an agent" starts crew on the mission's question itself, in the
// workspace's root, with the crew's harness and model; the question stays
// open for it to answer, and while it works the mission isn't asking you.
func TestWebAct_AskAnAgentToAnswer(t *testing.T) {
	shellCrew(t)
	t.Setenv(harness.EnvDefault, "")
	harness.Register(recordingHarness{"answer-agent"})
	ws := serverWorkspace(t, map[string][]string{"frontend": {"work"}})
	os.WriteFile(filepath.Join(ws.Root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n\n[repos.frontend]\n\n[crew]\nharness = \"answer-agent\"\nmodel = \"small\"\n"), 0o644)
	slug := seedMission(t, ws, "Served", "", [2]string{"frontend", "work"})
	srv := startServer(t, ws.Root)
	store := mission.Store{Root: ws.Root}
	act := webActIn(ws.Root)

	if _, err := act(slug, web.ActRequest{Verb: "ask-agent"}); err == nil {
		t.Error("asked an agent to answer when nothing was asked")
	}
	store.Ask(slug, "Commit and open PRs?", "orchestrator", "Commit both", "Leave them")
	if said, err := act(slug, web.ActRequest{Verb: "ask-agent"}); err != nil || !strings.HasPrefix(said, "Asked c1 to answer it") {
		t.Fatalf("ask an agent: %q, %v", said, err)
	}
	waitScreen(t, srv, slug+"/c1", "$")
	b := lastBrief(t, "answer-agent")
	if b.Role != mission.RoleAnswer || b.Dir != ws.Root || b.Question != "Commit and open PRs?" || strings.Join(b.Options, "|") != "Commit both|Leave them" || b.Model != "small" {
		t.Errorf("brief = %+v", b)
	}
	m, _ := store.Load(slug)
	if m.Asking() == nil {
		t.Fatal("launching the answering agent cleared the question it's answering")
	}
	if c := m.Crew[0]; c.Role != mission.RoleAnswer || c.Capsule != briefingPlace {
		t.Errorf("crew = %+v", c)
	}
	brief, _ := store.ReadBrief(slug)
	v := ops.BuildMissionViewsFrom(ws, nil, []*mission.Mission{m})[0]
	snap := ops.MissionSnapshot(v, brief, nil, nil, mission.DocThreads{}, false)
	snap.Seen = mission.Seen{Running: map[string]bool{"c1": true}}
	if h := mission.Resolve(snap); len(h) == 0 || h[0].Kind == mission.KindFlag {
		t.Errorf("still asking you while an agent answers: %+v", h)
	}
}

// webActIn is a mission page's act, in the workspace at root: its crew
// stopped in the daemon, which ops reaches as it does from a command.
func webActIn(root string) func(string, web.ActRequest) (string, error) {
	return func(slug string, a web.ActRequest) (string, error) { return web.ActOn(openOps(root, nil), slug, a) }
}
