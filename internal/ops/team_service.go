package ops

import (
	"os"
	"path/filepath"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/team"
	"github.com/withworkspace/workspace/internal/workspace"
)

// KeepTeam gives the workspace the Team page's service (TeamService),
// for its life: the daemon's, which serves the page and writes its gists.
// It's called once, before the workspace is used.
func (w *Workspace) KeepTeam() {
	if w.opts.Team == nil {
		w.opts.Team = w.TeamService()
	}
}

// Team is the Team page's service the workspace keeps (KeepTeam, or
// Options.Team): nil for none.
func (w *Workspace) Team() *team.Service { return w.opts.Team }

// TeamService is the Team page's service for the workspace: each visible
// repo's default branch read from its clone, and GitHub asked through
// ctx's client.
func (w *Workspace) TeamService() *team.Service {
	root := w.WS().Root
	window, _ := team.ParseWindow(w.Config().Team.Window)
	// Where there's a file answering for GitHub, there's no GitHub to link to.
	var web func(repo string) string
	if w.Config().GitHub.Fixture == "" {
		web = func(repo string) string {
			if !w.WS().OnGitHub(repo) {
				return ""
			}
			return w.WS().RemoteOf(repo).URL()
		}
	}
	// An agent writes gists unless [team] narrative says not to, or no
	// harness here can.
	var writer func(team.Ask) (string, error)
	noWriter := ""
	if !w.Config().TeamNarrative() {
		noWriter = "[team] narrative is off"
	} else if _, _, err := Roles(w.Config()).ForTeam(); err != nil {
		noWriter = err.Error()
	} else {
		writer = w.teamWriter()
	}
	var summariser func(team.SummaryAsk) (string, error)
	if writer != nil {
		summariser = w.teamSummariser()
	}
	return &team.Service{
		Root: root, GitHub: w.GitHub(), Window: window, Web: web, Writer: writer, NoWriter: noWriter, Summariser: summariser,
		Repos: func() []team.Repo {
			var out []team.Repo
			for _, name := range w.WS().ListedRepoNames() {
				if info, err := os.Stat(w.WS().BareDir(name)); err == nil && info.IsDir() {
					out = append(out, team.Repo{Name: name, Bare: w.WS().BareDir(name), Branch: w.WS().DefaultBranch, GitHub: w.WS().RemoteOf(name)})
				}
			}
			return out
		},
		// Your capsules, compared with the default branch they left.
		Capsules: func(repo string) []team.Capsule {
			branch := w.WS().DefaultBranch
			if branch == "" {
				branch = "main"
			}
			names, _ := workspace.ListWorktrees(w.WS().RepoDir(repo))
			out := make([]team.Capsule, 0, len(names))
			for _, n := range names {
				dir := filepath.Join(w.WS().RepoDir(repo), n)
				out = append(out, team.Capsule{Repo: repo, Name: n, Dir: dir, Base: "origin/" + branch, Stamp: changesStamp(dir, "origin/"+branch)})
			}
			return out
		},
		Watch: func() []team.Watch {
			var out []team.Watch
			for name, w := range w.Config().Team.Watch {
				out = append(out, team.Watch{Name: name, Title: w.Title, Paths: w.Paths})
			}
			return out
		},
		People: func() map[string]team.PersonConfig {
			out := map[string]team.PersonConfig{}
			for key, p := range w.Config().Team.People {
				out[key] = team.PersonConfig{Name: p.Name, Logins: p.Logins, Emails: p.Emails}
			}
			return out
		},
	}
}

// What an agent writes for the Team page: a change's gist, and what's
// worth knowing of it. It's given the change in its prompt (title,
// description, files, diff) and no tools, replies in a form ws reads, and
// its session isn't kept.

// gistTurnFor is how long writing one gist may take.
const gistTurnFor = 2 * time.Minute

// teamWriter has an agent write a gist. It reads the config each time, so
// changing [team] takes effect on the next one.
func (w *Workspace) teamWriter() func(team.Ask) (string, error) {
	return func(ask team.Ask) (string, error) {
		root := w.WS().Root
		pick, h, err := Roles(w.Config()).ForTeam()
		if err != nil {
			return "", err
		}
		run := harness.ConverseRun{Prompt: team.GistPrompt(ask, w.WS().DisplayNameFor), Dir: root, Model: pick.Model, Alone: true, Once: true}
		rep, err := RunHeadless(w.life(), root, "team gist "+ask.Subject.Strand, crew.Env(root, "", ""), "", gistTurnFor, h, run)
		if err != nil {
			return "", err
		}
		return rep.Output, nil
	}
}

// teamSummariser has an agent write a period's account, as teamWriter
// has one write a gist: no tools, no session kept.
func (w *Workspace) teamSummariser() func(team.SummaryAsk) (string, error) {
	return func(ask team.SummaryAsk) (string, error) {
		root := w.WS().Root
		pick, h, err := Roles(w.Config()).ForTeam()
		if err != nil {
			return "", err
		}
		run := harness.ConverseRun{Prompt: team.SummaryPrompt(ask, w.WS().DisplayNameFor), Dir: root, Model: pick.Model, Alone: true, Once: true}
		rep, err := RunHeadless(w.life(), root, "team account since "+ask.Since.Format("2006-01-02"), crew.Env(root, "", ""), "", gistTurnFor, h, run)
		if err != nil {
			return "", err
		}
		return rep.Output, nil
	}
}
