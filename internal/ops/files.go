package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The web UI's Files: every repo in the workspace, as ws labels them, and
// the files of its ground or any of its capsules; and where a capsule is,
// for reviewing it and finding in it.

// Repo is one of the workspace's repos, as ws labels and colours it, and
// its capsules.
type Repo struct {
	Name, Label, Color, Group string
	Hidden, Cloned            bool
	// Documents: the repo is a space of documents, whole (not a folder of
	// it): its home is Documents, and Files folds it away.
	Documents bool
	Capsules  []string
	Aliases   []string          // what else it's called (ws.toml's aliases), sorted
	Branches  map[string]string // each capsule's branch
	// Landing is how work lands in it, as the workspace or the repo itself
	// says.
	Landing RepoLanding
	// Yours: it's in ws.local.toml alone, not the team's ws.toml.
	Yours bool
	// GitHub is where it is on GitHub, as owner/name: "" for nowhere.
	GitHub string
}

// RepoLanding is how work lands in a repo: through a pull request, or
// straight to its default branch (config's landing).
type RepoLanding string

const (
	LandsByPR     RepoLanding = "pr"
	LandsDirectly RepoLanding = "direct"
)

// Worktree is a repo's ground or one of its capsules, and where it stands.
type Worktree struct {
	Name          string
	Ground        bool
	Branch        string
	Dirty         bool
	Ahead, Behind int
	// Unread: git hasn't been asked yet whether it's dirty or how far it
	// is from its upstream, and is being; a page's told when it's said.
	Unread bool
}

// Repos is the workspace's repos, in ws's order: each with its display
// name, its colour (its own, or the next of ws's palette, as ws status
// colours it), its group, and its capsules.
func (w *Workspace) Repos() []Repo {
	{
		ws := w.WS()
		out := []Repo{}
		colors := w.repoColors()
		for _, name := range ws.RepoNames {
			r := Repo{Name: name, Label: ws.DisplayNameFor(name), Color: colors[name], Group: ws.GroupOf(name), Hidden: ws.IsHidden(name), Capsules: []string{}, Aliases: []string{}, Landing: RepoLanding(ws.Landing(name))}
			if s, ok := ws.SpaceOf(name); ok && s.Folder == "" {
				r.Documents = true
			}
			for alias, of := range ws.AliasMap {
				if of == name {
					r.Aliases = append(r.Aliases, alias)
				}
			}
			slices.Sort(r.Aliases)
			r.Yours = ws.Yours[name]
			if ws.OnGitHub(name) {
				r.GitHub = ws.RemoteOf(name).String()
			}
			if info, err := os.Stat(ws.MainWorktree(name)); err == nil && info.IsDir() {
				r.Cloned = true
			}
			if capsules, err := workspace.ListWorktrees(ws.RepoDir(name)); err == nil {
				r.Capsules = append(r.Capsules, capsules...)
				r.Branches = map[string]string{}
				for _, c := range capsules {
					if branch := branchOf(filepath.Join(ws.RepoDir(name), c)); branch != "" && branch != "HEAD" {
						r.Branches[c] = branch
					}
				}
			}
			out = append(out, r)
		}
		return out
	}
}

// repoColors is each repo's colour, as CSS: its own, or the next of ws's
// palette, in ws's order of repos, as ws status colours it.
func (w *Workspace) repoColors() map[string]string {
	ws := w.WS()
	out := map[string]string{}
	next := 0
	for _, name := range ws.RepoNames {
		if c, ok := ws.RepoColors[name]; ok {
			out[name] = CSSColor(c)
		}
		if out[name] == "" {
			out[name] = CSSColor(fmt.Sprint(paletteNumbers[next%len(paletteNumbers)]))
			next++
		}
	}
	return out
}

// paletteNumbers are ws's repo palette (repoPalette), as xterm colours.
var paletteNumbers = []int{63, 43, 168, 107, 74, 209}

// CSSColor is a colour as ws's config has it (a hex colour, or an xterm
// 256-colour number) as CSS; "" for one it can't read.
func CSSColor(c string) string {
	c = strings.TrimSpace(c)
	if strings.HasPrefix(c, "#") && (len(c) == 4 || len(c) == 7) {
		return c
	}
	n, err := strconv.Atoi(c)
	if err != nil || n < 0 || n > 255 {
		return ""
	}
	switch {
	case n < 16:
		return []string{"#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0",
			"#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"}[n]
	case n >= 232:
		v := 8 + (n-232)*10
		return fmt.Sprintf("#%02x%02x%02x", v, v, v)
	}
	n -= 16
	level := []int{0, 95, 135, 175, 215, 255}
	return fmt.Sprintf("#%02x%02x%02x", level[n/36], level[(n/6)%6], level[n%6])
}

// Worktrees is a repo's ground and its capsules, each with its branch
// and where it stands.
func (w *Workspace) Worktrees(repo string) ([]Worktree, error) {
	{
		ws := w.WS()
		if !slices.Contains(ws.RepoNames, repo) {
			return nil, ErrNotFound
		}
		names, err := workspace.ListWorktrees(ws.RepoDir(repo))
		if err != nil {
			names = nil
		}
		out := []Worktree{}
		if info, err := os.Stat(ws.MainWorktree(repo)); err == nil && info.IsDir() {
			names = append([]string{workspace.GroundDir}, names...)
		}
		// Answered without waiting on git: its branch from its HEAD, and
		// the rest as last said, asked again behind the read (keptGit.soon).
		for _, name := range names {
			dir := filepath.Join(ws.RepoDir(repo), name)
			dirty, saidDirty := w.git.dirtySoon(dir)
			ab, saidAhead := soon(&w.git, dir, "ahead", func() answer[aheadAndBehind] { return askAheadBehind(dir) })
			out = append(out, Worktree{Name: name, Ground: name == workspace.GroundDir, Branch: branchOf(dir), Dirty: dirty, Ahead: ab.ahead, Behind: ab.behind, Unread: !saidDirty || !saidAhead})
		}
		return out, nil
	}
}

// Worktree is where a repo's ground (".ground") or capsule is, and the
// ref its changes are from: none for the ground, the default branch for a
// capsule. Only the workspace's repos and their worktrees.
func (w *Workspace) Worktree(repo, name string) (string, string, error) {
	{
		ws := w.WS()
		if !slices.Contains(ws.RepoNames, repo) {
			return "", "", ErrNotFound
		}
		if name == workspace.GroundDir {
			dir := ws.MainWorktree(repo)
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				return "", "", fmt.Errorf("%s isn't cloned yet: ws setup %s clones it", repo, repo)
			}
			return dir, "", nil
		}
		capsules, _ := workspace.ListWorktrees(ws.RepoDir(repo))
		if !slices.Contains(capsules, name) {
			return "", "", ErrNotFound
		}
		branch := ws.DefaultBranch
		if branch == "" {
			branch = "main"
		}
		return filepath.Join(ws.RepoDir(repo), name), "origin/" + branch, nil
	}
}

// FindIndex is what the find window looks through on the server: the
// workspace's worktrees (each repo's ground, and its capsules, compared
// with the default branch they left), read as they're asked for. One for
// the workspace's life: refs read each repo's files from it too.
func (w *Workspace) FindIndex() *find.Index {
	w.findOnce.Do(func() { w.findIx = w.newFindIndex() })
	return w.findIx
}

func (w *Workspace) newFindIndex() *find.Index {
	ix := &find.Index{RepoAt: func(owner, name string) (string, bool) { return w.WS().RepoAt(owner, name) }}
	ix.Worktrees = func() ([]find.Worktree, error) {
		ws := w.WS()
		branch := ws.DefaultBranch
		if branch == "" {
			branch = "main"
		}
		out := []find.Worktree{}
		for _, repo := range ws.RepoNames {
			if info, err := os.Stat(ws.MainWorktree(repo)); err == nil && info.IsDir() {
				out = append(out, find.Worktree{Repo: repo, Name: find.Ground, Dir: ws.MainWorktree(repo)})
			}
			capsules, _ := workspace.ListWorktrees(ws.RepoDir(repo))
			for _, name := range capsules {
				out = append(out, find.Worktree{Repo: repo, Name: name, Dir: filepath.Join(ws.RepoDir(repo), name), BaseRef: "origin/" + branch})
			}
		}
		return out, nil
	}
	return ix
}

// CapsuleDir is where a mission's capsule is, to review it, and the ref
// its changes are compared with: origin's default branch, as the
// orchestrator's diffs are. ErrNotFound for a capsule that isn't the
// mission's.
func (w *Workspace) CapsuleDir(slug, repo, name string) (string, string, error) {
	m, err := w.Missions.Load(slug)
	if err != nil || !m.HasCapsule(repo, name) {
		return "", "", ErrNotFound
	}
	ws := w.WS()
	dir := filepath.Join(ws.RepoDir(repo), name)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("capsule %s/%s is missing from %s", repo, name, dir)
	}
	branch := ws.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	return dir, "origin/" + branch, nil
}
