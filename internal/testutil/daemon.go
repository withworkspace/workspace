package testutil

import (
	"errors"
	"io"
	"log"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/nav"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/web"
)

// What the daemon serves of a workspace, for a test to ask as the web UI
// would: no daemon runs in these tests.

// daemonOps is the workspace's operations as the daemon has them. A
// workspace whose config names no file to answer for GitHub gets one with
// nothing on it: a test's repos are on no GitHub, and the real one isn't
// asked about them.
func daemonOps(root string, prs *ops.PRCache, more ...func(*ops.Options)) *ops.Workspace {
	none := func(o *ops.Options) {
		if _, _, gh, err := ops.Load(root); err == nil {
			if _, live := gh.(github.LiveClient); live {
				o.GitHub = &StubClient{}
			}
		}
	}
	return cli.OpenOps(root, prs, append([]func(*ops.Options){none}, more...)...)
}

// OpsAt is the workspace's operations as the daemon has them, on a clock
// that says it's at.
func OpsAt(root string, at time.Time) *ops.Workspace {
	return daemonOps(root, nil, func(o *ops.Options) { o.Now = func() time.Time { return at } })
}

// The web UI's view of the workspace.

func WebCapsule(root string) func(slug, repo, name string) (string, string, error) {
	return daemonOps(root, nil).CapsuleDir
}

func WebWorkspace(root string) func() (web.Workspace, error) {
	return func() (web.Workspace, error) {
		return web.WorkspaceOf(daemonOps(root, ops.NewPRCache()), root, nil)
	}
}

// WebMission is a mission as its page first reads it, in a daemon just
// started: GitHub not yet answered for its pull requests.
func WebMission(root string) func(string) (*web.MissionDetail, error) {
	return func(slug string) (*web.MissionDetail, error) {
		return web.MissionOf(daemonOps(root, ops.NewPRCache()), root, slug)
	}
}

// WebMissionAnswered is a mission as its page reads it again once GitHub,
// asked behind its first read, has answered for its pull requests.
func WebMissionAnswered(root string) func(string) (*web.MissionDetail, error) {
	return func(slug string) (*web.MissionDetail, error) {
		w := daemonOps(root, ops.NewPRCache())
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
			d, err := web.MissionOf(w, root, slug)
			if err != nil || d.GitHub {
				return d, err
			}
			if time.Now().After(deadline) {
				return d, errors.New("GitHub's answer never reached the mission's page")
			}
		}
	}
}

// WebMissionRead is a mission as its page reads it again once where its
// capsules stand, read behind its first read, has been.
func WebMissionRead(root string) func(string) (*web.MissionDetail, error) {
	return func(slug string) (*web.MissionDetail, error) {
		w := daemonOps(root, ops.NewPRCache())
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
			d, err := web.MissionOf(w, root, slug)
			if err != nil || !slices.ContainsFunc(d.Capsules, func(c web.CapsuleDetail) bool { return c.Unread }) {
				return d, err
			}
			if time.Now().After(deadline) {
				return d, errors.New("where the mission's capsules stand never reached its page")
			}
		}
	}
}

func WebArtifact(root string) func(slug, ref string) (*web.ArtifactContent, error) {
	return func(slug, ref string) (*web.ArtifactContent, error) {
		t, err := daemonOps(root, nil).Artifact(slug, ref)
		if err != nil {
			return nil, err
		}
		return web.ArtifactContentOf(t), nil
	}
}

func WebSaveDoc(root string) func(slug string, d web.DocSave) (*web.DocSaved, error) {
	return func(slug string, d web.DocSave) (*web.DocSaved, error) {
		saved, err := daemonOps(root, nil).SaveDocument(slug, d.Ref, d.Base, d.Text, d.Message)
		return web.DocSavedOf(saved), err
	}
}

func WebRepos(root string) func() ([]ops.Repo, error) {
	return func() ([]ops.Repo, error) { return daemonOps(root, nil).Repos(), nil }
}

func WebWorktrees(root string) func(repo string) ([]ops.Worktree, error) {
	return daemonOps(root, nil).Worktrees
}

func WebWorktree(root string) func(repo, name string) (string, string, error) {
	return daemonOps(root, nil).Worktree
}

func WebOrchestrator(root string) func() (web.OrchestratorInfo, error) {
	return func() (web.OrchestratorInfo, error) { return web.OrchestratorOf(daemonOps(root, nil)), nil }
}

func WebSettings(root string) func() (web.SettingsInfo, error) {
	return func() (web.SettingsInfo, error) {
		info, err := daemonOps(root, nil).Settings()
		return web.SettingsOf(info), err
	}
}

func WebSetSettings(root string) func([]web.SettingChange) (web.SettingsInfo, error) {
	return func(changes []web.SettingChange) (web.SettingsInfo, error) {
		chs := make([]ops.SettingChange, 0, len(changes))
		for _, ch := range changes {
			chs = append(chs, ops.SettingChange(ch))
		}
		info, err := daemonOps(root, nil).SetSettings(chs)
		return web.SettingsOf(info), err
	}
}

func WebOrchestratorRun(root string) func(string, string) (*web.OrchestratorRunDetail, error) {
	return func(slug, id string) (*web.OrchestratorRunDetail, error) {
		return web.OrchestratorRunOf(daemonOps(root, nil), slug, id)
	}
}

func WebCrewTranscript(root string) func(slug, id string) (*web.Transcript, error) {
	return func(slug, id string) (*web.Transcript, error) {
		sess, err := daemonOps(root, nil).Crew().Session(slug, id)
		if err != nil {
			return nil, err
		}
		return web.LoadTranscript(web.CrewSession(sess))
	}
}

func WebAct(root string) func(string, web.ActRequest) (string, error) {
	return func(slug string, a web.ActRequest) (string, error) {
		return web.ActOn(daemonOps(root, nil), slug, a)
	}
}

// WebDoc is the web UI's read of a document outside any mission.
func WebDoc(root string) func(ref string) (*web.ArtifactContent, error) {
	return func(ref string) (*web.ArtifactContent, error) {
		t, err := daemonOps(root, nil).Doc(ref)
		if err != nil {
			return nil, err
		}
		return web.ArtifactContentOf(t), nil
	}
}

// The Reviews page's: asking about a pull request, opening it here, and
// tidying up the capsules ws docked.
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

// FilePage is the web UI's filing of a page as a document of a kind, on
// the day at.
func FilePage(root string, at time.Time) func(id string, r web.PageFile) (*web.PageFiling, error) {
	return func(id string, r web.PageFile) (*web.PageFiling, error) {
		f, err := OpsAt(root, at).FilePage(id, ops.PageFile(r))
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
	id, m, err := daemonOps(root, nil).OpenFile(ref)
	if m == nil {
		return id, nil, err
	}
	dm := web.DocMission(*m)
	return id, &dm, err
}

func PageTied(root, id string) *web.PageTied {
	return web.PageTiedOf(daemonOps(root, nil).PageTied(id))
}

func SavePageToFile(root, id, message string) (*web.DocSaved, error) {
	saved, err := daemonOps(root, nil).SavePageToFile(id, message)
	return web.DocSavedOf(saved), err
}

func BringIn(root, id string) (*web.PageBroughtIn, error) {
	got, err := daemonOps(root, nil).BringIn(id)
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
		saved, err := daemonOps(root, nil).SaveWorkspaceDoc(d.Ref, d.Base, d.Text, d.Message, "Edited in ws's web UI.")
		return web.DocSavedOf(saved), err
	}
}

// WebServer is the web UI as the daemon serves it of the workspace at
// root (as daemon.newWeb makes it, but for plugins, silos and the
// router), started on a port of its own and closed when t ends. GitHub is
// a stub with nothing on it, where the workspace names none.
func WebServer(t testing.TB, root string) *web.Server {
	t.Helper()
	app := daemonOps(root, ops.NewPRCache())
	s := &web.Server{
		Root: root, App: app, Log: log.New(io.Discard, "", 0), Version: "test",
		Reviews: app.PRService(), Team: app.Team(), Find: app.FindIndex(),
		Nav: nav.New(filepath.Join(root, ".ws", "cache", "nav")),
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}
