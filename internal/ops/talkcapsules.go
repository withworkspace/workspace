package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/workspace"
)

// A conversation's agent with a sandboxed shell (harness.Sandboxer) runs
// what it likes, and writes where ws opens for it: a folder of its own,
// and the capsules it lifts. Those capsules are its conversation's, not a
// mission's, whose capsules are its plan's: ws keeps which it lifted, and
// puts them away with the conversation.

// LiftForConversation lifts a capsule for the conversation an agent is
// answering: branch, from base ("" for the default branch). It joins no
// mission, even on a mission's Talk tab, and isn't boarded. One the
// conversation put away comes back on its branch, as it was left.
func (w *Workspace) LiftForConversation(slug, id, repo, branch, base string, o CapsuleOptions) (Capsule, error) {
	store, err := ConversationStore(w.Root(), slug, id)
	if err != nil {
		return Capsule{}, err
	}
	c, err := store.Load(id)
	if err != nil {
		return Capsule{}, err
	}
	if why := c.CanLift(); why != "" {
		return Capsule{}, refused("conversation-holds-none", "%s", why)
	}
	o.Mission, o.Aside = "", true
	name := workspace.CapsuleName(branch)
	was := c.Capsule(repo, name)
	var made Capsule
	switch {
	case was != nil && was.Away == nil && exists(filepath.Join(w.WS().RepoDir(repo), name)):
		return Capsule{}, refused("capsule-exists", "you lifted %s/%s already: it's at %s", repo, name, filepath.Join(w.WS().RepoDir(repo), name))
	case was != nil && was.Away != nil && git.Ok(w.WS().BareDir(repo), "rev-parse", "--verify", "--quiet", "refs/heads/"+was.Branch):
		// Put away with the conversation, its branch kept: back as it was.
		branch = was.Branch
		made, err = w.makeCapsule(repo, branch, o, func() (string, error) { return w.WS().CreateDockWorktree(repo, branch) })
	default:
		made, err = w.Lift(repo, branch, base, o)
	}
	if err != nil {
		return made, err
	}
	if _, err := store.Lift(id, conversation.Capsule{Repo: repo, Name: made.Name, Branch: branch, At: w.now()}); err != nil {
		return made, fmt.Errorf("%s/%s is lifted, but %s doesn't have it as its own, so it won't be put away with it: %w", repo, made.Name, id, err)
	}
	return made, nil
}

// putAwayCapsules puts away the capsules a conversation's agent lifted,
// as the conversation is: each is burned, its branch kept, with what
// wasn't committed in it committed there first, so nothing's lost and
// lifting it again brings it back. It says which it couldn't, and why:
// those are left as they are.
func (w *Workspace) putAwayCapsules(store conversation.Store, c *conversation.Conversation) []string {
	var left []string
	for _, cp := range c.Lifted() {
		path := filepath.Join(w.WS().RepoDir(cp.Repo), cp.Name)
		kept := false
		if exists(path) {
			if workspace.GitIsDirty(path) {
				// Not signed: it's ws keeping what was there, not anyone's
				// work signed off, and a signing agent may ask you for it.
				msg := fmt.Sprintf("As it was when %s was put away", c.ID)
				if err := git.Run(path, "add", "-A"); err != nil {
					left = append(left, fmt.Sprintf("%s (what's not committed couldn't be kept: %v)", cp.Ref(), err))
					continue
				}
				if err := git.Run(path, "-c", "commit.gpgsign=false", "commit", "-q", "--no-verify", "-m", msg); err != nil {
					left = append(left, fmt.Sprintf("%s (what's not committed couldn't be kept: %v)", cp.Ref(), err))
					continue
				}
				kept = true
			}
			if _, err := w.Burn(cp.Repo, cp.Name, BurnOptions{}); err != nil {
				left = append(left, fmt.Sprintf("%s (%v)", cp.Ref(), err))
				continue
			}
		}
		if _, err := store.PutCapsuleAway(c.ID, cp.Repo, cp.Name, kept, w.now()); err != nil {
			left = append(left, fmt.Sprintf("%s (it's burned, but %s still has it as lifted: %v)", cp.Ref(), c.ID, err))
		}
	}
	return left
}

// sandboxFor readies a conversation's run for a sandboxed shell, where
// its agent's harness has one and the conversation is one that talks
// (prompt.Sandboxed): where it may write, and whether it may lift. It's
// what the agent's told of it, or nil for a run that only reads.
func (w *Workspace) sandboxFor(store conversation.Store, c *conversation.Conversation, h harness.Converser, r *harness.ConverseRun) *prompt.Sandbox {
	if !harness.Sandboxes(h) || !prompt.Sandboxed(c) {
		return nil
	}
	own, err := w.ownFolder(store, c.ID)
	if err != nil {
		return nil // nowhere of its own to write: it reads, as before
	}
	out := &prompt.Sandbox{Own: own, NoLift: c.CanLift()}
	r.Sandbox, r.Lift = true, out.NoLift == ""
	r.Write = []string{own}
	for _, cp := range c.Capsules {
		path := resolved(filepath.Join(w.WS().RepoDir(cp.Repo), cp.Name))
		if cp.Away != nil || !exists(path) {
			if cp.Away != nil {
				out.Away = append(out.Away, prompt.SandboxCapsule{Repo: cp.Repo, Name: cp.Name, Branch: cp.Branch})
			}
			continue
		}
		out.Capsules = append(out.Capsules, prompt.SandboxCapsule{Repo: cp.Repo, Name: cp.Name, Branch: cp.Branch, Path: path})
		r.Write = append(r.Write, path)
		if admin, err := git.Line(path, "rev-parse", "--absolute-git-dir"); err == nil && admin != "" {
			r.Beside = append(r.Beside, resolved(admin))
		}
	}
	// A capsule it lifts as it goes is written in at once: so each repo's
	// folder has room for one, and git's folder for it is open where
	// every capsule's commits go.
	for _, repo := range w.WS().RepoNames {
		dir := resolved(w.WS().RepoDir(repo))
		if !exists(dir) {
			continue
		}
		r.Room = append(r.Room, dir)
		beside, room := gitFolders(resolved(w.WS().BareDir(repo)))
		r.Beside, r.Room = append(r.Beside, beside...), append(r.Room, room...)
	}
	r.Beside = append(r.Beside, toolCaches()...)
	return out
}

// ConversationFolders are the folders the tools of a conversation's agent
// may write files in, as they stand: its own, and the capsules it's
// lifted that are there. None for a conversation that isn't one.
func (w *Workspace) ConversationFolders(slug, id string) []string {
	store, err := ConversationStore(w.Root(), slug, id)
	if err != nil {
		return nil
	}
	c, err := store.Load(id)
	if err != nil {
		return nil
	}
	var out []string
	if own, err := w.ownFolder(store, id); err == nil {
		out = append(out, own)
	}
	for _, cp := range c.Lifted() {
		if path := resolved(filepath.Join(w.WS().RepoDir(cp.Repo), cp.Name)); exists(path) {
			out = append(out, path)
		}
	}
	return out
}

// ownFolder is the folder a conversation's agent writes what it likes in:
// in the system's temp folder, out of the workspace, made if it isn't
// there. It's kept as long as the system keeps it.
func (w *Workspace) ownFolder(store conversation.Store, id string) (string, error) {
	sum := sha256.Sum256([]byte(w.Root()))
	scope := store.Slug
	if scope == "" {
		scope = "workspace"
	}
	dir := filepath.Join(os.TempDir(), "ws-"+hex.EncodeToString(sum[:])[:8], scope, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return resolved(dir), nil
}

// resolved is path as it resolves, links followed: how a sandbox knows it.
func resolved(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	return path
}

// gitFolders are what of a repo's git folder a sandboxed shell writes when
// it works in a capsule: open, what every capsule of the repo shares
// (the objects, the refs); room, the folder each capsule's own is kept
// in (its index, its HEAD), where one for a capsule lifted since is made,
// and those there now stay closed unless they're the conversation's. Not
// the rest: its config and hooks, which would run for whoever next used
// the repo. A commit made in the wrong capsule fails, its own folder
// closed; but a branch can be moved from any of them, which is loose.
func gitFolders(common string) (open, room []string) {
	for _, name := range []string{"objects", "refs", "logs", "packed-refs", "lfs", "modules"} {
		if p := filepath.Join(common, name); exists(p) {
			open = append(open, p)
		}
	}
	if p := filepath.Join(common, "worktrees"); exists(p) {
		room = append(room, p)
	}
	return open, room
}

// toolCaches are where tools keep what they've built and fetched, of
// those that are there: a build or a test run writes to them, and is
// slow, or fails, where it can't.
func toolCaches() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dirs := []string{".cache", "go/pkg", ".npm", ".bun/install", ".cargo/registry", ".cargo/git", ".gradle", ".m2", ".pnpm-store", ".yarn", "Library/pnpm", ".local/share/pnpm"}
	var out []string
	if cache, err := os.UserCacheDir(); err == nil && exists(cache) {
		out = append(out, resolved(cache))
	}
	for _, d := range dirs {
		p := resolved(filepath.Join(home, filepath.FromSlash(d)))
		if exists(p) && !containsPath(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// containsPath is whether p is one of paths, or inside one.
func containsPath(paths []string, p string) bool {
	for _, q := range paths {
		if p == q || strings.HasPrefix(p, q+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
