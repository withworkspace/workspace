package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/plugin"
	wsref "github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/workspace"
)

// What a plugin asks ws that ws show answers: the same view the command
// prints, as data, so a plugin reads what ws knows of a thing (and above
// all what's said of one of its own) without running the command. The
// views are here, so this is: the rest of what a plugin asks is ops'.

// showForPlugin answers what a plugin asks ws show (protocol.MethodShow),
// in the daemon (daemon.Config.Show): the view, from here.
func showForPlugin(ctx *ops.Workspace, _ context.Context, params json.RawMessage) (any, error) {
	var in protocol.ShowParams
	if len(params) > 0 && string(params) != "null" {
		if err := json.Unmarshal(params, &in); err != nil {
			return nil, plugin.InvalidParams(err)
		}
	}
	return shownFor(ctx, in)
}

// shownFor is what ws show shows of what in names, as the protocol has
// it. It differs from the command in two ways, both because it's asked
// in the daemon by a plugin: a mission is the one named and no other
// (the command's is the one it's run in), and one of a plugin's things
// is as ws was last told, since asking the plugin would be asking the
// one that's waiting on this answer.
func shownFor(ctx *ops.Workspace, in protocol.ShowParams) (protocol.Shown, error) {
	in.Ref, in.Mission = strings.TrimSpace(in.Ref), strings.TrimSpace(in.Mission)
	if in.Ref == "" && in.Mission == "" {
		return protocol.Shown{}, plugin.InvalidParams(errors.New("what to show: ref and mission are both empty"))
	}
	slug := in.Mission
	ref := fromAddress(ctx, in.Ref, &slug)
	v, m, err := showView(ctx, slug, ref, in.Full, showAsked{mission: missionNamed, thing: thingTold})
	if err != nil && slug == "" {
		// A mission is said of a thing by its slug alone, so it's asked
		// for by it: nothing else a ref names is a mission's slug.
		if named, merr := missionStore(ctx).Load(ref); merr == nil {
			v, m, err = nil, named, nil
		}
	}
	if err != nil {
		// A ref ws reads nothing by, or reads and finds nothing at: the
		// views say which in words, and have no other way to.
		return protocol.Shown{}, plugin.NotFound(err.Error())
	}
	out := protocol.Shown{Ref: in.Ref, Mission: in.Mission, Body: []string{}, Links: []protocol.ShownLink{}, Said: []protocol.Said{}}
	if v == nil { // the mission itself
		mv := ops.BuildMissionViews(ctx.WS(), nil, []*mission.Mission{m})[0]
		ms := missionStore(ctx)
		brief, _ := ms.ReadBrief(m.Slug)
		events, _ := ms.ReadLog(m.Slug, missionShowLogLimit)
		out.Head = "Mission " + m.Slug + ": " + m.Title
		out.Body = strings.Split(strings.TrimRight(formatMissionLLM(ctx, mv, ms.BriefPath(m.Slug), brief, events), "\n"), "\n")
		out.Path = "/missions/" + url.PathEscape(m.Slug)
		if said, err := ctx.SaidOf(store.Key{Name: m.Slug}); err == nil {
			for _, x := range said {
				out.Said = append(out.Said, pluginSaid(ctx, x))
			}
		}
		return out, nil
	}
	out.Head = v.head
	out.Body = append(out.Body, v.body...)
	for _, l := range v.links {
		out.Links = append(out.Links, protocol.ShownLink{Ref: l.ref, What: l.what})
	}
	for _, s := range v.saids {
		for _, x := range s.said {
			if ops.OnLines(x.Lines, s.line, s.end) { // as the command lists them: what's on the lines asked for
				out.Said = append(out.Said, pluginSaid(ctx, x))
			}
		}
	}
	scope := ""
	if m != nil {
		scope = m.Slug
	}
	out.Path = shownPath(ctx, scope, ref)
	return out, nil
}

// missionNamed is the mission a plugin names, and no other: the daemon
// is in no mission, and in no capsule.
func missionNamed(ctx *ops.Workspace, slug string) (*mission.Mission, error) {
	if slug == "" {
		return nil, errors.New("ws has nothing by that name. One of a mission's own is asked for with its mission")
	}
	m, err := missionStore(ctx).Load(slug)
	if errors.Is(err, mission.ErrNotFound) {
		return nil, errors.New("there's no mission " + slug)
	}
	return m, err
}

// thingTold is one of a plugin's things as ws was last told of it: what
// a plugin that asks is shown, where the command asks the plugin. One ws
// hasn't been told of is still shown, by its name: what's said of it in
// the workspace doesn't wait on that.
func thingTold(root string, r wsref.Ref, kept plugin.KeptRef, full bool) (map[string]string, string, error) {
	rec, told, _ := plugin.ThingOf(root, r.Plugin, r.What, r.ID)
	if !told {
		return nil, "The " + r.Plugin + " plugin hasn't told ws of it.", nil
	}
	return rec.Fields, "As the " + r.Plugin + " plugin last told ws.", nil
}

// pluginSaid is something that points at a view's thing, as the protocol
// has it: where it's read, how it stands, and its page.
func pluginSaid(ctx *ops.Workspace, x ops.Said) protocol.Said {
	s := protocol.Said{Ref: x.Ref, Kind: x.Kind, Title: x.Title, State: x.State, Standing: x.Standing(), How: x.How, Lines: x.Lines, At: x.At}
	switch {
	case x.Scope == "":
	case isPR(x.Scope):
		s.PullRequest = x.Scope
	default:
		s.Mission = x.Scope
	}
	s.Path = saidPath(ctx, x)
	return s
}

// saidPath is the page of something that points at a view's thing, in
// the web UI: "" where it has none of its own.
func saidPath(ctx *ops.Workspace, x ops.Said) string {
	switch {
	case x.Scope == "" && x.Kind == "mission":
		return "/missions/" + url.PathEscape(x.Ref)
	case isPR(x.Scope): // a review's comment, or a talk about the pull request: read on its review
		return prPath(x.Scope)
	case x.Scope != "":
		return missionPath(x.Scope, x.Ref, x.Kind)
	}
	return workspacePath(ctx, x.Ref)
}

// shownPath is the page of what a view shows: one of mission scope's
// own, or with no scope the workspace's.
func shownPath(ctx *ops.Workspace, scope, ref string) string {
	if scope != "" {
		kind := ""
		switch {
		case mission.IsDecisionRef(ref):
			kind = "decision"
		case reviewShape.MatchString(ref), commentShape.MatchString(ref):
			kind = "review"
		case ref == mission.BriefRef:
			kind = "brief"
		case strings.HasPrefix(ref, "mission:"):
			kind = "file"
		}
		return missionPath(scope, ref, kind)
	}
	return workspacePath(ctx, ref)
}

// missionPath is where one of a mission's own is read: a conversation
// has its page, its brief has its tab, a handoff or another of its files
// is read as an artifact, a review is the mission's work, and the rest
// are on the mission's.
func missionPath(slug, ref, kind string) string {
	at := "/missions/" + url.PathEscape(slug)
	switch {
	case kind == "comment" || kind == "review":
		return at + "?tab=work"
	case kind == "brief":
		return at + "?tab=brief"
	case kind == mission.HandoffKind || kind == "file":
		return at + "/artifact?" + url.Values{"ref": {ref}}.Encode()
	case kind == "decision":
		return at
	case conversation.IsRef(ref):
		return at + "/conversations/" + url.PathEscape(ref)
	}
	return at
}

// workspacePath is where one of the workspace's is read: a conversation,
// an artifact, a page, a pull request's review, one of a plugin's things
// in its plugin's app (or where it is outside ws), a document in its
// space.
func workspacePath(ctx *ops.Workspace, ref string) string {
	switch {
	case conversation.IsWorkspaceRef(ref):
		return "/conversations/" + url.PathEscape(ref)
	case artifact.IsRef(ref):
		return "/artifacts/" + url.PathEscape(ref)
	case page.IsRef(ref):
		return "/pages/" + url.PathEscape(ref)
	case isPR(ref):
		return prPath(ref)
	}
	if r, kept, ok := ctx.ThingNamed(ref); ok {
		if kept.Path != "" {
			return "/x/" + kept.Plugin + strings.Replace(kept.Path, wsref.Placeholder, url.PathEscape(r.ID), 1)
		}
		return r.URL
	}
	if isFileKey(ref) {
		if r, ok := wsref.Parse(ref); ok && (r.Worktree == "" || r.Worktree == workspace.GroundDir) {
			if _, space := ctx.WS().SpaceOf(r.Repo); space {
				return "/documents/" + url.PathEscape(r.Repo) + "/" + escapePath(r.Path)
			}
		}
	}
	return ""
}

// prPath is a pull request's review in the web UI.
func prPath(ref string) string {
	repo, number, ok := wsref.ParsePR(ref)
	if !ok {
		return ""
	}
	return "/reviews/" + url.PathEscape(repo) + "/" + strconv.Itoa(number)
}

// escapePath escapes a file's path a segment at a time, keeping its
// slashes.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}
