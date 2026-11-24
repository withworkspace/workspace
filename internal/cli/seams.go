package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/daemon"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	wsref "github.com/withworkspace/workspace/internal/ref"
)

// What a test of ws's commands reaches of cli's own. Those tests are in
// other packages (internal/integration, through testutil), so what they
// stand in for, and what of the daemon's they run by hand, is exported
// here. ws itself uses none of it.

// NoDaemon is for a test binary, which has no daemon to hand work to (one
// started from it would be the test binary run again): the orchestrator
// isn't run, and a step's checks run there and then, as the server would
// run them, as does the PR writer.
func NoDaemon() {
	orchestrateSpawner = func(root, slug, crewID, trigger string) {}
	checkSpawner = func(root, slug, crewID string) {
		if ctx, err := LoadOps(root); err == nil {
			ctx.RunChecks(slug, crewID)
		}
	}
	prWriterSpawner = func(root, slug, capsule string) {
		if ctx, err := LoadOps(root); err == nil {
			ctx.OpenWrittenPR(slug, capsule)
		}
	}
}

// OpenOps is the workspace's operations as the daemon has them.
var OpenOps = openOps

// UseLocalDispatchHost runs crew as children of the test instead of in
// the workspace's daemon, and returns a cleanup that stops them. The host
// types their opening prompts in, as the server does.
func UseLocalDispatchHost(root string) func() {
	old := hostFor
	h := &crew.LocalHost{Store: mission.Store{Root: root}}
	hostFor = func(string) crew.Host { return h }
	return func() {
		hostFor = old
		h.Leave()
	}
}

// WatchHeld looks at turns held on crew's own work once, at now, as the
// daemon does each tick, with alive standing in for the process table and
// callDone for the daemon's ACP sessions.
func WatchHeld(root string, now time.Time, alive func(pid int) bool, callDone func(slug, crewID, call string) bool) {
	if w, err := LoadOps(root); err == nil {
		daemon.WatchHeld(w, now, alive, callDone)
	}
}

// WatchPRsLoop is the daemon's watch of the missions' pull requests, until
// stop closes: every ops.WatchPRsEvery, and when a page's fetch brings news.
func WatchPRsLoop(root string, prs *ops.PRCache, stop <-chan struct{}) {
	t := time.NewTicker(ops.WatchPRsEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		case <-prs.News():
		}
		daemon.WatchPRs(openOps(root, prs), prs)
	}
}

// UseProcs stands procs in for the user's processes, which ws's hook
// command reads for the process check, and returns a cleanup.
func UseProcs(procs harness.Procs) func() {
	old := readProcs
	readProcs = func() (harness.Procs, error) { return procs, nil }
	return func() { readProcs = old }
}

// CaptureTalkAnswers records the conversations ws would have an agent
// answer, as slug/id, instead of starting one.
func CaptureTalkAnswers() (*[]string, func()) {
	old := talkSpawner
	var asked []string
	talkSpawner = func(root, slug, id string) { asked = append(asked, slug+"/"+id) }
	return &asked, func() { talkSpawner = old }
}

// CaptureOrchestratorRuns records the orchestrator runs ws would spawn, as
// slug/crew/trigger, instead of spawning them.
func CaptureOrchestratorRuns() (*[]string, func()) {
	old := orchestrateSpawner
	var runs []string
	orchestrateSpawner = func(root, slug, crewID, trigger string) {
		runs = append(runs, slug+"/"+crewID+"/"+trigger)
	}
	return &runs, func() { orchestrateSpawner = old }
}

// SweepArtifacts is the daemon's sweep of the artifacts on this machine,
// run once: what it logged.
func SweepArtifacts(root string) []string {
	var said []string
	w, err := LoadOps(root)
	if err != nil {
		return []string{"artifacts: couldn't sweep: " + err.Error()}
	}
	daemon.SweepArtifacts(w, func(f string, a ...any) { said = append(said, fmt.Sprintf(f, a...)) })
	return said
}

// EvalMade is evalMade, for a test to own the records of the workspace an
// eval's run is made in.
var EvalMade = &evalMade

// AnswerPlugin is what the daemon answers a plugin's requests with, in
// the workspace at root.
func AnswerPlugin(root string) func(context.Context, string, string, json.RawMessage) (any, error) {
	w := openOps(root, nil)
	return func(c context.Context, who, method string, params json.RawMessage) (any, error) {
		if method == protocol.MethodShow {
			return showForPlugin(w, c, params)
		}
		return w.AnswerPlugin(c, who, method, params)
	}
}

// UseThings has ask stand in for the workspace's plugins when ws show
// asks one what a thing is, and returns a cleanup: no daemon runs one in
// these tests.
func UseThings(ask func(r wsref.Ref, full bool) (plugin.Shown, error)) func() {
	old := askPlugin
	askPlugin = func(_ string, r wsref.Ref, full bool) (plugin.Shown, error) { return ask(r, full) }
	return func() { askPlugin = old }
}

// NoRunsWatcher has ws run start no daemon, for the test's length.
func NoRunsWatcher(t interface{ Cleanup(func()) }) {
	old := ensureRunsWatched
	ensureRunsWatched = func(string) error { return nil }
	t.Cleanup(func() { ensureRunsWatched = old })
}

// RouteFile is the route file the daemon writes for the workspace at root
// (the daemon's router loop).
var RouteFile = daemon.RouteFile
