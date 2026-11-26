package ops

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Refs: what there is to name in the workspace, for ws's one grammar
// (internal/ref), and what a name is now. A conversation's turn keeps what
// its names were when it was said; a page's cell is written with them
// linked; ws show reads an address.

// RefsKnown is what there is to name: each repo with its default branch
// (read with trees; otherwise none of their files are found, but
// addresses are read), what else each is called, each one's capsules and
// the branches they're on, the missions, pages, conversations and
// artifacts, and how each plugin's things are named.
func (w *Workspace) RefsKnown(ctx context.Context, trees bool) *ref.Known {
	return w.refsKnown(ctx, trees, false)
}

// namesWait is how long what's written to the records waits for the
// repos' files to be read, where a read is due: every other writer waits
// behind it meanwhile.
var namesWait = 500 * time.Millisecond

// namesKnown is RefsKnown for the records to find what words name as
// they're written (store.Name): asked inside a Write, so it reads the
// records and never writes them (the pages are as last read, not read
// again from their files), and it doesn't wait long for the repos' files
// (namesWait): a repo that's slow to read is as it was last read, or its
// files go unfound this once, which leaves a file's name unlinked until
// what names it is written again.
func (w *Workspace) namesKnown() *ref.Known {
	return w.refsKnown(context.Background(), true, true)
}

func (w *Workspace) refsKnown(ctx context.Context, trees, writing bool) *ref.Known {
	ws := w.WS()
	k := &ref.Known{
		Repos:         map[string]ref.Tree{},
		Aliases:       ws.AliasMap,
		Capsules:      map[string][]string{},
		Branches:      map[string]map[string]string{},
		Root:          ws.Root,
		Org:           ws.Org,
		Remotes:       ws.Remotes,
		DefaultBranch: firstNonEmpty(ws.DefaultBranch, "main"),
	}
	var grounds map[string]find.GroundTree
	switch {
	case trees && writing:
		grounds = w.FindIndex().GroundsWithin(ctx, namesWait)
	case trees:
		grounds, _ = w.FindIndex().Grounds(ctx)
	}
	for _, repo := range ws.RepoNames {
		if g, ok := grounds[repo]; ok {
			k.Repos[repo] = g
		} else {
			k.Repos[repo] = nil
		}
		k.Branches[repo] = map[string]string{}
		if trees, kept := w.caps.worktrees(repo, false); kept {
			for _, t := range trees {
				if t.Ground {
					continue
				}
				k.Capsules[repo] = append(k.Capsules[repo], t.Name)
				if t.Branch != "" && t.Branch != "HEAD" {
					k.Branches[repo][t.Branch] = t.Name
				}
			}
			continue
		}
		capsules, _ := workspace.ListWorktrees(ws.RepoDir(repo))
		k.Capsules[repo] = capsules
		for _, c := range capsules {
			// Read from git's own files (branchOf): what there is to name
			// is asked for on every pin and every address read, and a git
			// apiece was a process for each capsule in the workspace.
			if branch := branchOf(filepath.Join(ws.RepoDir(repo), c)); branch != "" && branch != "HEAD" {
				k.Branches[repo][branch] = c
			}
		}
	}
	k.Missions = append(k.Missions, w.Missions.Slugs()...)
	if writing {
		k.Pages = (page.Store{Root: ws.Root}).IDs()
	} else if ps, err := (page.Store{Root: ws.Root}).List(); err == nil {
		for _, p := range ps {
			k.Pages = append(k.Pages, p.ID)
		}
	}
	k.Talks = conversation.ForWorkspace(ws.Root).IDs()
	k.Artifacts = (artifact.Store{Root: ws.Root}).IDs()
	k.Things = w.thingsNamed()
	return k
}

// thingsNamed is how each plugin's things are named, as the workspace's
// records keep it (plugin.KeepRefs, when a plugin registers), in the
// plugins' order: a plugin that's no longer here names nothing.
func (w *Workspace) thingsNamed() []ref.Named {
	root := w.WS().Root
	if len(plugin.Installed(root)) == 0 {
		return nil // nothing to name, and no records to open for it
	}
	kept, err := plugin.Refs(root)
	if err != nil {
		return nil
	}
	var out []ref.Named
	for _, k := range kept {
		if plugin.IsInstalled(root, k.Plugin) {
			out = append(out, k.Named())
		}
	}
	return out
}

// ThingNamed is the plugin's thing that s is, whole: a name its plugin
// registered (RAM-123), ws: and that, where its plugin said one is, or
// the folder of one ws found in a space (records:sketches/release-flow/).
// With it is how its kind is named, as ws keeps it. It reads nothing but
// how plugins' things are named and which ws found: a workspace with no
// plugins has none.
func (w *Workspace) ThingNamed(s string) (ref.Ref, plugin.KeptRef, bool) {
	root := w.WS().Root
	if strings.TrimSpace(s) == "" || len(plugin.Installed(root)) == 0 {
		return ref.Ref{}, plugin.KeptRef{}, false
	}
	kept, err := plugin.Refs(root)
	if err != nil {
		return ref.Ref{}, plugin.KeptRef{}, false
	}
	k := &ref.Known{}
	for _, kr := range kept {
		if plugin.IsInstalled(root, kr.Plugin) {
			k.Things = append(k.Things, kr.Named())
		}
	}
	r, ok := k.At(s)
	if !ok || r.Kind != ref.Thing {
		// A folder ws found a thing in (its package) is that thing, of the
		// kind it was kept as: named by the folder, not as the kind names.
		dir, isDir := ref.Parse(strings.TrimPrefix(strings.TrimSpace(s), "ws:"))
		if !isDir || dir.Kind != ref.Dir || dir.InCapsule() {
			return ref.Ref{}, plugin.KeptRef{}, false
		}
		f, found, _ := plugin.FoundNamed(root, dir.Name())
		if !found || !plugin.IsInstalled(root, f.Plugin) {
			return ref.Ref{}, plugin.KeptRef{}, false
		}
		r = ref.Ref{Kind: ref.Thing, Plugin: f.Plugin, What: f.Kind, ID: dir.Name()}
	}
	for _, kr := range kept {
		if kr.Plugin == r.Plugin && kr.Kind == r.What {
			return r, kr, true
		}
	}
	return ref.Ref{}, plugin.KeptRef{}, false
}

// Resolve is each ref in a text as it is now: a file's lines pinned to the
// commit its worktree is at (the ground's, as last read; a capsule's
// head), and a capsule's branch, and whether it's pushed.
func (w *Workspace) Resolve(ctx context.Context, k *ref.Known, text string) []ref.Pinned {
	var out []ref.Pinned
	seen := map[string]bool{}
	for _, f := range k.Find(text) {
		said := text[f.From:f.To]
		if seen[said] {
			continue
		}
		seen[said] = true
		out = append(out, w.pin(ctx, k, said, f.Ref))
	}
	return out
}

// pin is a ref with what ties it to now.
func (w *Workspace) pin(ctx context.Context, k *ref.Known, said string, r ref.Ref) ref.Pinned {
	rv := ref.Pinned{Said: said, Ref: r}
	if r.Kind != ref.File && r.Kind != ref.Dir {
		return rv
	}
	ws := w.WS()
	if !r.InCapsule() {
		if g, ok := k.Repos[r.Repo].(find.GroundTree); ok && r.Kind == ref.File && r.Line > 0 {
			rv.Ref.Commit = g.Head()
		}
		return rv
	}
	dir := filepath.Join(ws.RepoDir(r.Repo), r.Worktree)
	head, branch := w.git.headBranch(dir)
	if branch != "" && branch != "HEAD" {
		rv.Branch = branch
		if out, err := git.Read(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch); err == nil && strings.TrimSpace(string(out)) != "" {
			rv.Pushed = true
		}
	}
	if head != "" {
		rv.Ref.Commit = head
	}
	return rv
}

// pinTurn keeps, on a turn, what the names in it are now: in what it says,
// its notes, and its findings.
func (w *Workspace) pinTurn(t *conversation.Turn) {
	if t.By == conversation.WS {
		return
	}
	parts := []string{t.Body, t.Notes}
	for _, f := range t.Findings {
		parts = append(parts, f.Text, f.Suggest)
	}
	text := strings.Join(parts, "\n")
	if strings.TrimSpace(text) == "" {
		return
	}
	ctx := context.Background()
	t.Refs = w.Resolve(ctx, w.RefsKnown(ctx, true), text)
}

// LinkNames is markdown going into a document with the names in it made
// links (ref.Link): each to where it is on GitHub, a file's lines at the
// commit they were read at, a capsule's file at its branch once pushed,
// and what only ws has as ws:. Where no repo has an owner to address
// GitHub by, it's left as it is.
func (w *Workspace) LinkNames(md string) string {
	ws := w.WS()
	if len(ws.Owners()) == 0 || strings.TrimSpace(md) == "" {
		return md
	}
	ctx := context.Background()
	k := w.RefsKnown(ctx, true)
	return k.Link(md, func(f ref.Found, said string) (string, bool) {
		p := w.pin(ctx, k, said, f.Ref)
		where := ref.Where{Org: k.Org, Remotes: k.Remotes, DefaultBranch: k.DefaultBranch}
		if p.Pushed {
			where.Branch = p.Branch
		}
		return p.Ref.Address(where), true
	})
}
