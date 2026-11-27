package ops

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/github"
	wsref "github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/slug"
	"github.com/withworkspace/workspace/internal/workspace"
)

// A change to ws's config: the team's ws.toml, or a repo's ws.repo.toml,
// as the whole of the file as it should read. Both are committed files,
// so a change to one is a pull request, never a write in place: a
// repo's in a capsule lifted for it (config-<slug>), ws.toml's on a
// branch of the workspace's config repo, pushed from a clone of it made
// for the change and removed after, since the root's own checkout is
// reset to its remote by ws sync. A repo with no pull requests has its
// change committed in the capsule, for you to land. ws.local.toml is
// yours and committed nowhere: it isn't changed this way.
//
// A conversation's agent proposes one (ws config edit --propose), and you
// confirm it: ws checks the file as ws would read it when it's proposed,
// and again when it's accepted, and merges in what changed in the file
// meanwhile, as a document's edit is.

// ConfigFile is one of the config files a change can be made to, as it is
// now: its ref (ws.toml, repo:ws.repo.toml), its repo ("" for ws.toml),
// its text ("" where there's none yet) and version (as doc.Version), and
// how a change to it goes in.
type ConfigFile struct {
	Ref, Repo     string
	Text, Version string
	// Lands is conversation.LandsPR or LandsCapsule; Where says it in
	// words ("a pull request on withworkspace/dogfood").
	Lands, Where string
}

// ConfigPR is what a change to a config file made: the pull request
// opened (PR, with its URL), or the capsule it's committed in, to land
// (Capsule, repo/name), and what it says it did.
type ConfigPR struct {
	PR      *github.PR
	Capsule string
	Branch  string
	Message string
}

// Result is what the change made, as a proposal's Result says it: the
// pull request's URL, or the capsule.
func (c ConfigPR) Result() string {
	if c.PR != nil {
		return c.PR.URL
	}
	return c.Capsule
}

// NewFileVersion is the version of a file that isn't there yet.
const NewFileVersion = "new"

// ConfigToEdit is the config file ref names as it is now: ws.toml, or a
// repo's ws.repo.toml (repo:ws.repo.toml, or the repo alone), read from
// its ground.
func (w *Workspace) ConfigToEdit(ref string) (*ConfigFile, error) {
	ref = strings.Trim(strings.TrimSpace(ref), "`")
	switch ref {
	case "":
		return nil, invalid("name the file: ws.toml, or <repo>:ws.repo.toml")
	case config.LocalFileName:
		return nil, refused("yours", "ws.local.toml is yours and committed nowhere, so it isn't changed by a pull request: give the user the lines to paste, or send them to the Settings page where it has a row")
	case config.FileName:
		cr, err := w.configRepo()
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(filepath.Join(w.root, config.FileName))
		if err != nil {
			return nil, err
		}
		return &ConfigFile{Ref: config.FileName, Text: string(data), Version: doc.Version(data), Lands: conversation.LandsPR, Where: "a pull request on " + cr.github.String()}, nil
	}
	repo, file, ok := strings.Cut(ref, ":")
	if !ok {
		file = config.RepoFileName
	}
	if file != config.RepoFileName {
		return nil, invalid("%s isn't one of ws's config files: name ws.toml, or <repo>:ws.repo.toml", ref)
	}
	ws := w.WS()
	name, ok := ws.ResolveAlias(repo)
	if !ok {
		return nil, invalid("no repo %s: ws status lists them", repo)
	}
	ground := ws.MainWorktree(name)
	if _, err := os.Stat(ground); err != nil {
		return nil, refused("not-cloned", "%s isn't cloned here: ws setup clones it", name)
	}
	f := &ConfigFile{Ref: name + ":" + config.RepoFileName, Repo: name, Version: NewFileVersion}
	if data, err := os.ReadFile(filepath.Join(ground, config.RepoFileName)); err == nil {
		f.Text, f.Version = string(data), doc.Version(data)
	}
	if ws.Direct(name) {
		f.Lands, f.Where = conversation.LandsCapsule, "committed in a capsule of "+name+", which takes no pull requests, for you to land"
	} else {
		f.Lands, f.Where = conversation.LandsPR, "a pull request on "+ws.RemoteOf(name).String()
	}
	return f, nil
}

// CheckConfig is what's wrong with text as the file f, as ws would read
// it: TOML that doesn't parse, and keys ws doesn't read (misspelt, or in
// the wrong file). nil when ws reads all of it.
func CheckConfig(f *ConfigFile, text string) error {
	dir, err := os.MkdirTemp("", "ws-config-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	name := config.FileName
	if f.Repo != "" {
		name = config.RepoFileName
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	var unknown []string
	if f.Repo == "" {
		cfg, err := config.Parse(path)
		if err != nil {
			return invalid("%v", strings.ReplaceAll(err.Error(), path, name))
		}
		unknown = cfg.Unknown
	} else {
		rc, err := config.ParseRepoConfig(path)
		if err != nil {
			return invalid("%v", strings.ReplaceAll(err.Error(), path, name))
		}
		unknown = rc.Unknown
	}
	if len(unknown) > 0 {
		return invalid("ws doesn't read %s (misspelt, or in the wrong file: ws show config lists every key, and which file each goes in)", strings.Join(unknown, "; "))
	}
	return nil
}

// EditConfig changes the config file ref to text, made from was (the
// file as ConfigToEdit gave it), as a pull request titled title, with body
// as its description and the rest of its commit's message. What changed
// in the file since was is merged in; where the two change the same
// lines, nothing's made, and it's refused.
func (w *Workspace) EditConfig(ref, was, text, title, body string) (*ConfigPR, error) {
	f, err := w.ConfigToEdit(ref)
	if err != nil {
		return nil, err
	}
	title = strings.TrimSpace(oneLine(title))
	if title == "" {
		return nil, invalid("a change to ws's config has a title, for its commit and pull request")
	}
	text = strings.TrimSpace(text) + "\n"
	if err := CheckConfig(f, text); err != nil {
		return nil, err
	}
	branch := "config-" + slug.Make(title, 40, "change")
	if f.Repo != "" {
		return w.editRepoConfig(f, was, text, title, body, branch)
	}
	return w.editTeamConfig(was, text, title, body, branch)
}

// mergedConfig is text, made from was, onto now (the file as it is where
// the change is being made): text itself where now is still was.
func mergedConfig(ref, was, text string, now []byte) ([]byte, error) {
	if string(now) == was || strings.TrimSpace(string(now)) == strings.TrimSpace(was) {
		return []byte(text), nil
	}
	got, clashes, err := mergeText([]byte(was), []byte(text), now)
	if err != nil {
		return nil, err
	}
	if clashes > 0 {
		return nil, refused("changed", "%s changed since this was made from it, in the same lines (%d %s): nothing's made. Make the change again from it as it is (ws config edit --propose %s)", ref, clashes, plural(clashes, "place"), ref)
	}
	return got, nil
}

// editRepoConfig makes a change to a repo's ws.repo.toml in a capsule
// lifted for it from the default branch as it is on the remote: a pull
// request opened from it, or, in a repo with no pull requests, left
// committed there, to land.
func (w *Workspace) editRepoConfig(f *ConfigFile, was, text, title, body, branch string) (*ConfigPR, error) {
	ws := w.WS()
	gh := w.GitHub()
	if f.Lands == conversation.LandsPR && gh == nil {
		return nil, refused("no-github", "could not open a pull request: no GitHub access")
	}
	l, err := w.liftForDefaultBranch(f.Repo, w.freeBranch(f.Repo, branch))
	if err != nil {
		return nil, err
	}
	path := filepath.Join(l.dir, config.RepoFileName)
	now, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		l.undo()
		return nil, err
	}
	out, err := mergedConfig(f.Ref, was, text, now)
	if err != nil {
		l.undo()
		return nil, err
	}
	if bytes.Equal(out, now) {
		l.undo()
		return nil, refused("same", "%s already reads that way on %s: nothing to change", f.Ref, ws.DefaultBranch)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		l.undo()
		return nil, err
	}
	if said, err := git.Combined(l.dir, "add", "--", config.RepoFileName); err != nil {
		l.undo()
		return nil, fmt.Errorf("could not add %s: %v: %s", config.RepoFileName, err, clipString(said, 300))
	}
	if said, err := git.Combined(l.dir, "commit", "-q", "-m", title, "-m", body, "--", config.RepoFileName); err != nil {
		l.undo()
		return nil, fmt.Errorf("could not commit %s: %v: %s", config.RepoFileName, err, clipString(said, 300))
	}
	capsule := f.Repo + "/" + l.capsule
	if f.Lands == conversation.LandsCapsule {
		return &ConfigPR{Capsule: capsule, Branch: l.branch, Message: fmt.Sprintf("Committed in %s, for you to land (ws land): %s takes no pull requests", capsule, f.Repo)}, nil
	}
	// From here what's committed stays: the capsule is where it can be
	// pushed from again, by hand, if GitHub refuses it.
	pushed, said, err := ws.PushBranch(f.Repo, l.capsule, true)
	if err != nil {
		return nil, fmt.Errorf("committed in %s, but %w: %s", capsule, err, clipString(said, 300))
	}
	remote := ws.RemoteOf(f.Repo)
	pr, err := gh.CreatePR(remote.Owner, remote.Name, github.NewPR{Head: pushed, Base: ws.DefaultBranch, Title: title, Body: body})
	if err != nil {
		return nil, fmt.Errorf("pushed %s from %s, but could not open the pull request: %w", pushed, capsule, err)
	}
	w.opts.PRs.Forget(f.Repo)
	return &ConfigPR{PR: pr, Capsule: capsule, Branch: pushed, Message: fmt.Sprintf("Opened %s#%d from %s: %s", f.Repo, pr.Number, capsule, pr.URL)}, nil
}

// configRepo is the workspace's config repo: the root's own git (put
// away between syncs as .git-disabled), and where it is on GitHub.
type configRepoAt struct {
	gitDir, origin string
	github         wsref.Remote
}

func (w *Workspace) configRepo() (*configRepoAt, error) {
	gitDir := ""
	for _, d := range []string{".git", ".git-disabled"} {
		if dirThere(filepath.Join(w.root, d)) {
			gitDir = filepath.Join(w.root, d)
			break
		}
	}
	if gitDir == "" {
		return nil, refused("no-config-repo", "the workspace's config isn't a git repo, so ws.toml can't be changed by a pull request: give the user the lines to put in it")
	}
	origin, err := git.ReadLine(context.Background(), "", "--git-dir="+gitDir, "config", "--get", "remote.origin.url")
	if err != nil || origin == "" {
		return nil, refused("no-config-remote", "the workspace's config repo has no remote, so there's nowhere to open a pull request: give the user the lines to put in ws.toml")
	}
	gh, ok := workspace.GitHubAt(origin)
	if !ok {
		return nil, refused("config-not-github", "the workspace's config repo isn't on GitHub (%s), so ws can't open a pull request for ws.toml: give the user the lines to put in it", origin)
	}
	return &configRepoAt{gitDir: gitDir, origin: origin, github: gh}, nil
}

// editTeamConfig makes a change to ws.toml as a pull request on the
// workspace's config repo, from a branch made off its remote's default
// branch as it is now. It's made in a clone of the config repo of its own,
// not in the root, which ws sync resets to the remote: what the root has
// that isn't pushed stays as it is.
func (w *Workspace) editTeamConfig(was, text, title, body, branch string) (*ConfigPR, error) {
	cr, err := w.configRepo()
	if err != nil {
		return nil, err
	}
	gh := w.GitHub()
	if gh == nil {
		return nil, refused("no-github", "could not open a pull request: no GitHub access")
	}
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "ws-config-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	quiet := []string{"GIT_TERMINAL_PROMPT=0"}
	run := func(args ...string) (string, error) {
		return git.Cmd{Ctx: ctx, Dir: dir, Args: args, Env: quiet}.Combined()
	}
	// From the root's own git, which is quick, then pointed where it
	// fetches from, for its default branch as it is there now.
	if said, err := (git.Cmd{Ctx: ctx, Args: []string{"clone", "-q", "--no-checkout", cr.gitDir, dir}, Env: quiet}).Combined(); err != nil {
		return nil, fmt.Errorf("could not clone the workspace's config repo: %v: %s", err, clipString(said, 300))
	}
	base, err := git.ReadLine(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || base == "" {
		base = w.WS().DefaultBranch
	}
	if said, err := run("remote", "set-url", "origin", cr.origin); err != nil {
		return nil, fmt.Errorf("could not point the config repo's clone at %s: %v: %s", cr.origin, err, clipString(said, 300))
	}
	if said, err := run("fetch", "-q", "origin", base); err != nil {
		return nil, &Upstream{Err: fmt.Errorf("could not fetch the workspace's config repo (%s): %v: %s", cr.github, err, clipString(said, 300))}
	}
	branch = freeRemoteBranch(ctx, dir, branch)
	if said, err := run("checkout", "-q", "-b", branch, "FETCH_HEAD"); err != nil {
		return nil, fmt.Errorf("could not make the branch %s: %v: %s", branch, err, clipString(said, 300))
	}
	path := filepath.Join(dir, config.FileName)
	now, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("the config repo's %s has no %s", base, config.FileName)
	}
	out, err := mergedConfig(config.FileName, was, text, now)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(out, now) {
		return nil, refused("same", "ws.toml already reads that way on %s: nothing to change", base)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return nil, err
	}
	commit := []string{"commit", "-q", "-m", title, "-m", body, "--", config.FileName}
	// As you, as the root commits: its own name and email, where it says.
	for _, k := range []string{"user.name", "user.email"} {
		if v, err := git.ReadLine(ctx, "", "--git-dir="+cr.gitDir, "config", "--get", k); err == nil && v != "" {
			commit = append([]string{"-c", k + "=" + v}, commit...)
		}
	}
	if said, err := run(commit...); err != nil {
		return nil, fmt.Errorf("could not commit ws.toml: %v: %s", err, clipString(said, 300))
	}
	if said, err := run("push", "--porcelain", "origin", "HEAD:refs/heads/"+branch); err != nil {
		return nil, &Upstream{Err: fmt.Errorf("%s refused the push of %s: nothing forced: %s", cr.github, branch, clipString(strings.TrimSpace(said), 300))}
	}
	pr, err := gh.CreatePR(cr.github.Owner, cr.github.Name, github.NewPR{Head: branch, Base: base, Title: title, Body: body})
	if err != nil {
		return nil, fmt.Errorf("pushed %s to %s, but could not open the pull request: %w", branch, cr.github, err)
	}
	return &ConfigPR{PR: pr, Branch: branch, Message: fmt.Sprintf("Opened %s#%d: %s. Once it's merged, ws sync brings it here", cr.github, pr.Number, pr.URL)}, nil
}

// freeRemoteBranch is base, or base-2, base-3…: the first that origin
// hasn't a branch of.
func freeRemoteBranch(ctx context.Context, dir, base string) string {
	heads, _ := git.Read(ctx, dir, "ls-remote", "--heads", "origin")
	taken := func(b string) bool { return bytes.Contains(heads, []byte("refs/heads/"+b+"\n")) }
	branch := base
	for n := 2; taken(branch); n++ {
		branch = fmt.Sprintf("%s-%d", base, n)
	}
	return cmp.Or(branch, base)
}
