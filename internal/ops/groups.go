package ops

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/config"
)

// Groups: repos put together (group = "platform"), and which groups are
// hidden, left out of what covers every repo. The team says both in
// ws.toml; you can say otherwise for you, in ws.local.toml: a group shown
// or hidden, and a repo in another group.

// Group is one of the workspace's groups: its repos, whether it's hidden
// for you, what the team says, and whether that's your choice.
type Group struct {
	Name string
	// Label is what it's called where it's shown (ws.toml's display_name,
	// or its name), and Order where it comes among the groups (Groups
	// are in it, lowest first).
	Label string
	Order int
	Repos []string
	// Hidden is as it is for you; TeamHidden is what ws.toml says; Chosen:
	// Hidden is your choice (ws.local.toml), not the team's.
	Hidden, TeamHidden, Chosen bool
}

// Groups are the workspace's groups, in their order (then by name): every
// one a repo is in, or that's hidden or shown by name.
func (w *Workspace) Groups() []Group {
	ws := w.WS()
	seen := map[string]bool{}
	for _, g := range ws.GroupNames() {
		seen[g] = true
	}
	for g := range ws.GroupChoices {
		seen[g] = true
	}
	out := []Group{}
	names := slices.Sorted(maps.Keys(seen))
	slices.SortStableFunc(names, func(a, b string) int { return cmp.Compare(ws.GroupOrders[a], ws.GroupOrders[b]) })
	for _, name := range names {
		_, chosen := ws.GroupChoices[name]
		repos := ws.ReposInGroup(name)
		if repos == nil {
			repos = []string{}
		}
		out = append(out, Group{Name: name, Label: ws.GroupLabel(name), Order: ws.GroupOrders[name], Repos: repos, Hidden: ws.HiddenGroups[name], TeamHidden: ws.TeamHidden[name], Chosen: chosen})
	}
	return out
}

// GroupShown is a group shown or hidden for you, and the repos of it
// being cloned now it's shown.
type GroupShown struct {
	Group   string
	Hidden  bool
	Chosen  bool
	Cloning []string
}

func (g GroupShown) String() string {
	state := "shown"
	if g.Hidden {
		state = "hidden"
	}
	s := fmt.Sprintf("%s is %s for you", g.Group, state)
	if !g.Chosen {
		s = fmt.Sprintf("%s is as the team has it: %s", g.Group, state)
	}
	switch len(g.Cloning) {
	case 0:
	case 1:
		s += ", and " + g.Cloning[0] + " is being cloned"
	default:
		s += ", and " + strings.Join(g.Cloning, ", ") + " are being cloned"
	}
	return s
}

// ShowGroup shows or hides a group for you, as ws group show and hide do,
// or, with nil, takes your choice away so the team's applies (ws group
// reset). A group shown has its repos that aren't cloned cloned behind
// the answer, as the rest of the workspace's are; Wait clones them first.
func (w *Workspace) ShowGroup(ctx context.Context, who Actor, group string, hidden *bool, wait bool) (*GroupShown, error) {
	if !who.IsYou() {
		return nil, refused("not-yours", "which groups are shown is the user's to say")
	}
	if !slices.ContainsFunc(w.Groups(), func(g Group) bool { return g.Name == group }) {
		return nil, fmt.Errorf("%w: there's no group called %s", ErrNotFound, group)
	}
	if err := config.SaveGroupVisibility(w.root, group, hidden); err != nil {
		return nil, fmt.Errorf("saving whether %s is shown: %w", group, err)
	}
	_, ws, _, err := Load(w.root)
	if err != nil {
		return nil, err
	}
	out := &GroupShown{Group: group, Hidden: ws.HiddenGroups[group], Chosen: hidden != nil}
	if out.Hidden {
		return out, nil
	}
	for _, repo := range ws.ReposInGroup(group) {
		if _, err := os.Stat(ws.RepoDir(repo)); os.IsNotExist(err) {
			out.Cloning = append(out.Cloning, repo)
		}
	}
	if wait {
		for _, repo := range out.Cloning {
			if err := w.cloneRepo(repo); err != nil {
				return out, fmt.Errorf("%s is shown, but %s couldn't be cloned: %w", group, repo, err)
			}
		}
		out.Cloning = nil
		return out, nil
	}
	w.cloneBehind(out.Cloning...)
	return out, nil
}

// RepoGrouped is a repo put in a group for you, or back in the team's.
type RepoGrouped struct {
	Repo, Group string
	// Team: it's in the group the team has it in (or none), with nothing
	// of yours over it.
	Team bool
}

func (g RepoGrouped) String() string {
	switch {
	case g.Group == "":
		return g.Repo + " is in no group"
	case g.Team:
		return fmt.Sprintf("%s is in %s, as the team has it", g.Repo, g.Group)
	}
	return fmt.Sprintf("%s is in %s, for you", g.Repo, g.Group)
}

// GroupRepo puts a repo in a group, for you, in ws.local.toml: a repo of
// yours in any (or none), one of the team's in any but the team's own,
// which takes yours away. A team's repo can't be put in no group for
// you, where the team has it in one: ws.local.toml can't say "none" over
// ws.toml's group.
func (w *Workspace) GroupRepo(ctx context.Context, who Actor, name, group string) (*RepoGrouped, error) {
	if !who.IsYou() {
		return nil, refused("not-yours", "which group a repo's in is the user's to say")
	}
	ws := w.WS()
	repo, ok := ws.ResolveAlias(name)
	if !ok {
		return nil, fmt.Errorf("%w: there's no repo called %s", ErrNotFound, name)
	}
	group = strings.TrimSpace(group)
	if group != "" && strings.ContainsAny(group, ", \t") {
		return nil, invalid("%q can't be a group's name: no spaces or commas", group)
	}
	team, overridden := ws.TeamGroups[repo]
	if !overridden && !ws.Yours[repo] {
		team = ws.GroupOf(repo)
	}
	yours := ws.Yours[repo]
	if !yours && group == "" && team != "" {
		return nil, invalid("the team has %s in %s: it can be in another group for you, but not in none", repo, team)
	}
	set := group
	if !yours && group == team {
		set = "" // back as the team has it: yours goes
	}
	err := config.UpdateLocal(w.root, func(l *config.LocalConfig) {
		rc, had := l.Repos[repo]
		if !had && set == "" {
			return
		}
		rc.Group = set
		if l.Repos == nil {
			l.Repos = map[string]config.RepoConfig{}
		}
		l.Repos[repo] = rc
	})
	if err != nil {
		return nil, err
	}
	// A repo of yours whose only key was its group has lost its table with
	// it: its table is what makes it one, so it's written again, bare.
	if yours {
		if local, err := config.ReadLocal(w.root); err == nil {
			if _, ok := local.Repos[repo]; !ok {
				if err := config.AddLocalRepo(w.root, repo, config.RepoConfig{}); err != nil {
					return nil, err
				}
			}
		}
	}
	return &RepoGrouped{Repo: repo, Group: group, Team: !yours && set == ""}, nil
}
