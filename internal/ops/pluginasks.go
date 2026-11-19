package ops

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/workspace"
)

// AnswerPlugin answers what a plugin asks ws (plugin.Host's Answer): the
// workspace's missions and repos, a conversation started for the person,
// and what it tells ws of its own things,
// in the protocol's types, which are what ws promises a plugin, not its
// own. who is the plugin that asked.
func (w *Workspace) AnswerPlugin(ctx context.Context, who, method string, params json.RawMessage) (any, error) {
	switch method {
	case protocol.MethodMissionList:
		var in protocol.MissionListParams
		if err := decodeParams(params, &in); err != nil {
			return nil, err
		}
		return w.pluginMissions(in.Archived)
	case protocol.MethodMissionShow:
		var in protocol.MissionShowParams
		if err := decodeParams(params, &in); err != nil {
			return nil, err
		}
		return w.pluginMission(in.Slug)
	case protocol.MethodRepoList:
		return w.pluginRepos()
	case protocol.MethodConversationStart:
		var in protocol.ConversationStartParams
		if err := decodeParams(params, &in); err != nil {
			return nil, err
		}
		return w.pluginStartsTalk(who, in.Message)
	}
	// What a plugin tells ws of its things: kept in the workspace's
	// records under the plugin that told it, so ws finds and names them
	// without asking.
	if answer, ok, err := plugin.AnswerThings(w.WS().Root, who, method, params); ok {
		return answer, err
	}
	return nil, plugin.NoMethod(method)
}

// pluginStartsTalk starts one of the workspace's conversations for the
// person from plugin who, with what it has them say first: the
// conversation, and its page.
func (w *Workspace) pluginStartsTalk(who, message string) (protocol.ConversationStarted, error) {
	c, err := w.StartTalkVia(who, message)
	var invalid *Invalid
	if errors.As(err, &invalid) {
		return protocol.ConversationStarted{}, plugin.InvalidParams(err)
	}
	if err != nil {
		return protocol.ConversationStarted{}, err
	}
	return protocol.ConversationStarted{ID: c.ID, Path: "/conversations/" + c.ID}, nil
}

// decodeParams reads a request's params; none is the zero value.
func decodeParams(params json.RawMessage, into any) error {
	if len(params) == 0 || string(params) == "null" {
		return nil
	}
	if err := json.Unmarshal(params, into); err != nil {
		return plugin.InvalidParams(err)
	}
	return nil
}

// pluginMissions are the missions as their lists show them: those in
// play, then with archived, those archived.
func (w *Workspace) pluginMissions(archived bool) (protocol.MissionListResult, error) {
	ov, err := w.Overview()
	if err != nil {
		return protocol.MissionListResult{}, err
	}
	out := protocol.MissionListResult{Missions: []protocol.Mission{}}
	for _, it := range ov.Missions {
		if it.State == mission.Archived && !archived {
			continue
		}
		out.Missions = append(out.Missions, pluginMission(it.MissionView, it.Updated, it.Headline, it.At))
	}
	return out, nil
}

// pluginMission is a mission as its page shows it, with its brief and
// plan.
func (w *Workspace) pluginMission(slug string) (protocol.MissionDetail, error) {
	if slug == "" {
		return protocol.MissionDetail{}, plugin.InvalidParams(errors.New("which mission: slug is empty"))
	}
	d, err := w.Mission(slug, true)
	if errors.Is(err, ErrNotFound) {
		return protocol.MissionDetail{}, plugin.NotFound("there's no mission " + slug)
	}
	if err != nil {
		return protocol.MissionDetail{}, err
	}
	var h mission.Headline
	if len(d.Headlines) > 0 {
		h = d.Headlines[0]
	}
	return protocol.MissionDetail{
		Mission: pluginMission(d.MissionView, MissionUpdated(w.Missions, slug), h, d.At),
		Brief:   d.Brief,
		Plan:    d.Plan,
	}, nil
}

// pluginMission is a mission's view as the protocol has it.
func pluginMission(v MissionView, updated time.Time, h mission.Headline, at string) protocol.Mission {
	m := protocol.Mission{
		Slug:     v.M.Slug,
		Title:    v.M.Title,
		State:    string(v.State),
		Created:  v.M.Created,
		Updated:  updated,
		At:       at,
		Capsules: []protocol.MissionCapsule{},
		Path:     "/missions/" + v.M.Slug,
		About:    v.M.About,
	}
	if h.Court != "" {
		m.Headline = &protocol.Headline{Court: string(h.Court), Text: h.Text}
	}
	for _, c := range v.Capsules {
		pc := protocol.MissionCapsule{Repo: c.Repo, Name: c.Name, Branch: c.Branch, Landed: c.Landed != nil}
		if c.Exists {
			pc.Path = c.Path
		}
		if c.PR != nil {
			pc.PullRequest = &protocol.PullRequest{Number: c.PR.Number, Title: c.PR.Title, URL: c.PR.URL, State: prState(c)}
		}
		m.Capsules = append(m.Capsules, pc)
	}
	return m
}

// prState is a capsule's pull request's state: open, merged or closed.
func prState(c CapsuleView) string {
	switch c.PRS {
	case mission.PROpen:
		return "open"
	case mission.PRMerged:
		return "merged"
	}
	return strings.ToLower(c.PR.State)
}

// pluginRepos are the workspace's repos, in ws's order, with every
// capsule (the ground first) and the mission in play working in each.
func (w *Workspace) pluginRepos() (protocol.RepoListResult, error) {
	ws := w.WS()
	inPlay, err := w.Missions.InPlay()
	if err != nil {
		return protocol.RepoListResult{}, err
	}
	missionOf := map[string]string{} // repo/capsule → slug
	for _, m := range inPlay {
		for _, c := range m.Capsules {
			missionOf[c.Repo+"/"+c.Name] = m.Slug
		}
	}
	out := protocol.RepoListResult{Repos: []protocol.Repo{}}
	for _, name := range ws.RepoNames {
		dir := ws.RepoDir(name)
		r := protocol.Repo{Name: name, Title: ws.DisplayNameFor(name), Group: ws.GroupOf(name), Path: dir, Capsules: []protocol.Capsule{}}
		for alias, of := range ws.AliasMap {
			if of == name {
				r.Aliases = append(r.Aliases, alias)
			}
		}
		slices.Sort(r.Aliases)
		if info, err := os.Stat(ws.MainWorktree(name)); err == nil && info.IsDir() {
			r.Cloned = true
		}
		names, _ := workspace.ListAllWorktrees(dir)
		for _, c := range names {
			path := filepath.Join(dir, c)
			pc := protocol.Capsule{Name: c, Path: path, Ground: c == workspace.GroundDir, Mission: missionOf[name+"/"+c]}
			if _, branch := workspace.GitHeadBranch(path); branch != "HEAD" {
				pc.Branch = branch
			}
			r.Capsules = append(r.Capsules, pc)
		}
		out.Repos = append(out.Repos, r)
	}
	return out, nil
}
