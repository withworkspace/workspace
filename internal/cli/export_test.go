package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/pulls"
	wsref "github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/web"
)

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
var WatchHeld = watchHeldAt

// WatchPRsLoop is the daemon's watch of the missions' pull requests.
var WatchPRsLoop = watchPRsLoop

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

// The web UI's view of the workspace, for tests.
var (
	WebCapsule = func(root string) func(slug, repo, name string) (string, string, error) {
		return openOps(root, nil).CapsuleDir
	}
	WebWorkspace = func(root string) func() (web.Workspace, error) {
		return func() (web.Workspace, error) { return web.WorkspaceOf(openOps(root, ops.NewPRCache()), root, nil) }
	}
	WebMission = func(root string) func(string, bool) (*web.MissionDetail, error) {
		return func(slug string, gh bool) (*web.MissionDetail, error) {
			return web.MissionOf(openOps(root, ops.NewPRCache()), root, slug, gh)
		}
	}
	WebArtifact = func(root string) func(slug, ref string) (*web.ArtifactContent, error) {
		return func(slug, ref string) (*web.ArtifactContent, error) {
			t, err := openOps(root, nil).Artifact(slug, ref)
			if err != nil {
				return nil, err
			}
			return web.ArtifactContentOf(t), nil
		}
	}
	WebSaveDoc = func(root string) func(slug string, d web.DocSave) (*web.DocSaved, error) {
		return func(slug string, d web.DocSave) (*web.DocSaved, error) {
			saved, err := openOps(root, nil).SaveDocument(slug, d.Ref, d.Base, d.Text, d.Message)
			return web.DocSavedOf(saved), err
		}
	}
	WebRepos = func(root string) func() ([]ops.Repo, error) {
		return func() ([]ops.Repo, error) { return openOps(root, nil).Repos(), nil }
	}
	WebWorktrees    = func(root string) func(repo string) ([]ops.Worktree, error) { return openOps(root, nil).Worktrees }
	WebWorktree     = func(root string) func(repo, name string) (string, string, error) { return openOps(root, nil).Worktree }
	CSSColor        = ops.CSSColor
	WebOrchestrator = func(root string) func() (web.OrchestratorInfo, error) {
		return func() (web.OrchestratorInfo, error) { return web.OrchestratorOf(openOps(root, nil)), nil }
	}
	WebSettings = func(root string) func() (web.SettingsInfo, error) {
		return func() (web.SettingsInfo, error) {
			info, err := openOps(root, nil).Settings()
			return web.SettingsOf(info), err
		}
	}
	WebSetSettings = func(root string) func([]web.SettingChange) (web.SettingsInfo, error) {
		return func(changes []web.SettingChange) (web.SettingsInfo, error) {
			chs := make([]ops.SettingChange, 0, len(changes))
			for _, ch := range changes {
				chs = append(chs, ops.SettingChange(ch))
			}
			info, err := openOps(root, nil).SetSettings(chs)
			return web.SettingsOf(info), err
		}
	}
	WebModels          = ops.Models
	WebOrchestratorRun = func(root string) func(string, string) (*web.OrchestratorRunDetail, error) {
		return func(slug, id string) (*web.OrchestratorRunDetail, error) {
			return web.OrchestratorRunOf(openOps(root, nil), slug, id)
		}
	}
	WebCrewTranscript = func(root string) func(slug, id string) (*web.Transcript, error) {
		return func(slug, id string) (*web.Transcript, error) {
			sess, err := openOps(root, nil).Crew().Session(slug, id)
			if err != nil {
				return nil, err
			}
			return web.LoadTranscript(web.CrewSession(sess))
		}
	}
	WebAct = func(root string) func(string, web.ActRequest) (string, error) {
		return func(slug string, a web.ActRequest) (string, error) { return web.ActOn(openOps(root, nil), slug, a) }
	}
)

// The Reviews page's, for tests: asking about a pull request, opening it
// here, and tidying up the capsules ws docked.
func AskPR(ctx *ops.Workspace, repo string, number int, q web.PRQuestion) (string, error) {
	c, err := ctx.AskPR(repo, number, ops.PRQuestion(q))
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

func OpenHere(ctx *ops.Workspace, repo string, number int) (string, error) {
	return ctx.DockPR(repo, number)
}

func TidyDocked(ctx *ops.Workspace) { ctx.TidyDocked() }

func ReviewsService(ctx *ops.Workspace) *pulls.Service { return ctx.ReviewsService() }

// UseLifter lifts capsules for proposed missions with fn instead of a
// ws lift of its own, and returns a cleanup.
func UseLifter(fn func(root, slug, repo, branch string) error) func() {
	old := ops.LiftForMission
	ops.LiftForMission = func(w *ops.Workspace, slug, repo, branch string) error { return fn(w.Root(), slug, repo, branch) }
	return func() { ops.LiftForMission = old }
}

// UseDocker docks existing work for briefed missions with fn instead of a
// ws dock of its own, and returns a cleanup.
func UseDocker(fn func(root, slug, repo, ref string) error) func() {
	old := ops.DockForMission
	ops.DockForMission = func(w *ops.Workspace, slug, repo, ref string) error { return fn(w.Root(), slug, repo, ref) }
	return func() { ops.DockForMission = old }
}

// WebDoc is the web UI's read of a document outside any mission.
func WebDoc(root string) func(ref string) (*web.ArtifactContent, error) {
	return func(ref string) (*web.ArtifactContent, error) {
		t, err := openOps(root, nil).Doc(ref)
		if err != nil {
			return nil, err
		}
		return web.ArtifactContentOf(t), nil
	}
}

// opsAt is the workspace's operations on a clock that says it's at.
func opsAt(root string, at time.Time) *ops.Workspace {
	o := opsOptions(root)
	o.Now = func() time.Time { return at }
	return ops.Open(root, o)
}

// FilePage is the web UI's filing of a page as a document of a kind, on
// the day at.
func FilePage(root string, at time.Time) func(id string, r web.PageFile) (*web.PageFiling, error) {
	return func(id string, r web.PageFile) (*web.PageFiling, error) {
		f, err := opsAt(root, at).FilePage(id, ops.PageFile(r))
		if err != nil {
			return nil, err
		}
		out := web.PageFiling(*f)
		return &out, nil
	}
}

// OpenFilePage, PageTied, SavePageToFile and BringIn are the web UI's for
// a page tied to a file.
func OpenFilePage(root, ref string) (string, *web.DocMission, error) {
	id, m, err := openOps(root, nil).OpenFile(ref)
	if m == nil {
		return id, nil, err
	}
	dm := web.DocMission(*m)
	return id, &dm, err
}

func PageTied(root, id string) *web.PageTied { return web.PageTiedOf(openOps(root, nil).PageTied(id)) }

func SavePageToFile(root, id, message string) (*web.DocSaved, error) {
	saved, err := openOps(root, nil).SavePageToFile(id, message)
	return web.DocSavedOf(saved), err
}

func BringIn(root, id string) (*web.PageBroughtIn, error) {
	got, err := openOps(root, nil).BringIn(id)
	if got == nil {
		return nil, err
	}
	out := web.PageBroughtIn(*got)
	return &out, err
}

// IsFileMoved reports whether a page's save was refused because its file
// has changed since.
func IsFileMoved(err error) bool { return errors.Is(err, web.ErrChangedSince) }

// SaveFileDoc saves a file in a repo, outside any mission, as saving a page
// to its file does.
func SaveFileDoc(root string) func(d web.DocSave) (*web.DocSaved, error) {
	return func(d web.DocSave) (*web.DocSaved, error) {
		saved, err := openOps(root, nil).SaveWorkspaceDoc(d.Ref, d.Base, d.Text, d.Message, "Edited in ws's web UI.")
		return web.DocSavedOf(saved), err
	}
}

// Ops is the workspace's operations as the daemon has them, on the day at.
func Ops(root string, at time.Time) *ops.Workspace { return opsAt(root, at) }

// OpsAt is the workspace's operations on a clock that says it's at.
var OpsAt = opsAt

// SweepArtifacts is the daemon's sweep of the artifacts on this machine,
// run once: what it logged.
func SweepArtifacts(root string) []string {
	var said []string
	sweepArtifacts(root, func(f string, a ...any) { said = append(said, fmt.Sprintf(f, a...)) })
	return said
}

// EvalMade is evalMade, for a test to own the records of the workspace an
// eval's run is made in.
var EvalMade = &evalMade

// AnswerPlugin is what the daemon answers a plugin's requests with, in
// the workspace at root.
func AnswerPlugin(root string) func(context.Context, string, string, json.RawMessage) (any, error) {
	return answerPlugin(openOps(root, nil))
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

// RouteFile is routeFile, for the tests.
var RouteFile = routeFile
