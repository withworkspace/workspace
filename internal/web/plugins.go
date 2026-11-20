package web

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
)

// Plugins in the web UI: each one's view is an app of the plugin's, which
// runs in a page ws serves (the shell), shown in a frame.
// /plugin-frame/<name>/… is the shell; /plugins/<name>/… is passed to the
// plugin, for its app's files and what its app asks it; /api/plugins says
// what each adds to the sidebar, and where it stands. How the app and ws
// talk is the frame protocol: PROTOCOL.md in github.com/withworkspace/ui-lib.

const topicPlugins = "plugins"

// PluginPolicy is the Content-Security-Policy of what a plugin's own HTTP
// answers: its own scripts and styles, inline too, and images from
// anywhere. It's a program the workspace's owner installed.
const PluginPolicy = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; frame-ancestors 'self'"

// ShellPolicy is the Content-Security-Policy of the shell: scripts, styles,
// fonts and connections from this origin alone, and no inline script (the
// shell has none, and the app is a file). Inline style is allowed, for ws's
// tokens and for an app that styles from its script.
const ShellPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'self'"

// shellStyles is the file in the built web UI that the shell's <style>
// holds: ws's tokens, light and dark, under the names a plugin's app knows
// them by (web/vite.config.ts writes it).
const shellStyles = "plugin-shell.css"

// Plugin is one of the workspace's plugins, as GET /api/plugins serves it.
type Plugin struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// State is ready, setup (something it requires isn't set), failed, or
	// stopped (it's started when its view is opened).
	State   plugin.State `json:"state"`
	Error   string       `json:"error,omitempty"`
	Missing []string     `json:"missing,omitempty"`
	Views   []PluginView `json:"views"`
	// Refs are how its things are named, as ws keeps them (plugin.Refs):
	// what the web UI finds its names by, running or not.
	Refs []PluginRef `json:"refs,omitempty"`
	// Actions are what it can do, offered in Find among what there is to
	// do: as it registered when it last started.
	Actions []protocol.Action `json:"actions,omitempty"`
	// Changed counts the times it's said what its view shows is stale: an
	// app showing it is told when it moves.
	Changed uint64 `json:"changed"`
	// Starts counts the times it's been started: a page showing its view
	// loads its app again when it moves.
	Starts uint64 `json:"starts"`
}

// PluginRef is a kind of thing a plugin has that words name, and how one
// is named: one of Prefixes, then its Tail (a number, when it doesn't
// say). Address is where one is outside ws and Path where one opens in
// the plugin's app, {ref} its name in each. Fields are what one is made
// of, and Views how the web UI draws one, by where (card, pane): blocks,
// each showing a field.
type PluginRef struct {
	Kind     string                      `json:"kind"`
	Title    string                      `json:"title,omitempty"`
	Icon     string                      `json:"icon,omitempty"`
	Prefixes []string                    `json:"prefixes"`
	Tail     string                      `json:"tail,omitempty"`
	Address  string                      `json:"address,omitempty"`
	Path     string                      `json:"path,omitempty"`
	Fields   []protocol.Field            `json:"fields,omitempty"`
	Views    map[string][]protocol.Block `json:"views,omitempty"`
	// Actions are what can be done to one of its things, from Find.
	Actions []protocol.Action `json:"actions,omitempty"`
}

// PluginThing is one of a plugin's things: its name, and a value for each
// of its kind's fields it has one for. Told: ws has it from the plugin,
// and this is from its own records; otherwise the plugin was asked just
// now.
type PluginThing struct {
	Key     string            `json:"key"`
	Fields  map[string]string `json:"fields"`
	Changed *time.Time        `json:"changed,omitempty"`
	Told    bool              `json:"told"`
}

// PluginView is an entry a plugin adds to the sidebar.
type PluginView struct {
	Title string `json:"title"`
	Icon  string `json:"icon,omitempty"`
	Under string `json:"under,omitempty"`
	// Path is where in the plugin's app it opens.
	Path  string `json:"path"`
	Badge int    `json:"badge,omitempty"`
}

// Plugging is which plugins the workspace has: the list the Plugins page
// shows, and adding, updating and removing one.
type Plugging interface {
	Plugins(ctx context.Context) ops.PluginsView
	AddPlugin(ctx context.Context, from string, o ops.PluginAdd) (ops.PluginAdded, plugin.Status, error)
	UpdatePlugin(ctx context.Context, name string, say func(line string)) (ops.PluginUpdated, error)
	RemovePlugin(ctx context.Context, name string, data bool) error
	// One of a plugin's own things put in a repo, and saved there, for
	// you, and where one would go: what ws's own UI over a plugin's page
	// asks (thing-promote, thing-save).
	PromoteThing(ctx context.Context, who, kind, name, to string) (protocol.RemoteSaved, error)
	SaveThing(ctx context.Context, who, kind, name string) (protocol.RemoteSaved, error)
	ThingOffered(who, kind, title string) (ops.ThingOffer, error)
}

// ThingRequest is one of a plugin's own things to put in a repo, or to
// save there: its kind, the plugin's name for it, and, to promote it,
// where you said it's to live (repo:folder/), if you did.
type ThingRequest struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
	To   string `json:"to,omitempty"`
}

// ThingAt is a plugin's thing as it is in its repo: its name in ws (its
// folder), where that is, what it's called, and its main file's version.
type ThingAt struct {
	Name    string `json:"name"`
	Repo    string `json:"repo"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

// ThingSaved is what putting a plugin's thing in a repo, or saving it
// there, did: the frame protocol's ThingSaved, which a plugin's page is
// given as what came of asking.
type ThingSaved struct {
	Thing ThingAt `json:"thing"`
	// Changed is whether it changed anything there, and Merged whether
	// what was there had changed since, and was brought into it.
	Changed bool `json:"changed"`
	Merged  bool `json:"merged,omitempty"`
	// Capsule is the capsule it was left in, where its repo takes pull
	// requests: it's on no default branch yet.
	Capsule string `json:"capsule,omitempty"`
	Said    string `json:"said,omitempty"`
}

// ThingOffered is where a thing would go if it were promoted with
// nowhere said: the folder (records:sketches/release-flow/), its repo as
// ws calls it, and whether a commit lands there straight away.
type ThingOffered struct {
	Where  string `json:"where"`
	Repo   string `json:"repo"`
	Label  string `json:"label"`
	Direct bool   `json:"direct"`
}

func thingSaved(done protocol.RemoteSaved) ThingSaved {
	t := done.Thing
	return ThingSaved{Thing: ThingAt{Name: t.Name, Repo: t.Repo, Path: t.Path, Title: t.Title, Version: t.Version}, Changed: done.Changed, Merged: done.Merged, Capsule: done.Capsule, Said: done.Said}
}

// thingAsked reads which of a plugin's things a request is about.
func (s *Server) thingAsked(w http.ResponseWriter, r *http.Request) (name string, req ThingRequest, ok bool) {
	name = r.PathValue("name")
	if s.Plugins == nil || !plugin.IsInstalled(s.Plugins.Root, name) {
		jsonError(w, http.StatusNotFound, "there's no plugin called "+name)
		return name, req, false
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "which thing isn't JSON: "+err.Error())
		return name, req, false
	}
	if strings.TrimSpace(req.Kind) == "" || strings.TrimSpace(req.Ref) == "" {
		jsonError(w, http.StatusBadRequest, "which thing? Its kind, and the plugin's name for it")
		return name, req, false
	}
	return name, req, true
}

// promoteThing is POST /api/plugins/{name}/things/promote: one of the
// plugin's own things put in a repo, as you.
func (s *Server) promoteThing(w http.ResponseWriter, r *http.Request) {
	name, req, ok := s.thingAsked(w, r)
	if !ok {
		return
	}
	done, err := s.App.PromoteThing(r.Context(), name, req.Kind, req.Ref, req.To)
	if err != nil {
		writeError(w, err, http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, thingSaved(done))
}

// saveThing is POST /api/plugins/{name}/things/save: one of the plugin's
// own things that's in a repo saved there, as you.
func (s *Server) saveThing(w http.ResponseWriter, r *http.Request) {
	name, req, ok := s.thingAsked(w, r)
	if !ok {
		return
	}
	done, err := s.App.SaveThing(r.Context(), name, req.Kind, req.Ref)
	if err != nil {
		writeError(w, err, http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, thingSaved(done))
}

// thingOffered is GET /api/plugins/{name}/things/offer?kind=&title=:
// where one of the plugin's things of that kind, called that, would go.
func (s *Server) thingOffered(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if s.Plugins == nil || !plugin.IsInstalled(s.Plugins.Root, name) {
		jsonError(w, http.StatusNotFound, "there's no plugin called "+name)
		return
	}
	at, err := s.App.ThingOffered(name, r.URL.Query().Get("kind"), r.URL.Query().Get("title"))
	if err != nil {
		writeError(w, err, http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, ThingOffered{Where: at.Where, Repo: at.Repo, Label: at.Label, Direct: at.Direct})
}

// PluginsInstalled is GET /api/plugins/installed: the workspace's plugins
// as the Plugins page lists them, with the team's that aren't here.
type PluginsInstalled struct {
	Plugins []InstalledPlugin `json:"plugins"`
	// Wanted are the team's (ws.toml's [plugins]) that aren't installed
	// here.
	Wanted []WantedPlugin `json:"wanted"`
	// Checks is whether the daemon looks for later releases by itself,
	// and Checked when a look last answered.
	Checks  bool       `json:"checks"`
	Checked *time.Time `json:"checked,omitempty"`
}

// InstalledPlugin is a plugin that's installed: which it is, where it's
// from, where it stands, what it adds, and what's out for it.
type InstalledPlugin struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	// State is ready, setup, failed or stopped, as Plugin's is.
	State   plugin.State `json:"state"`
	Error   string       `json:"error,omitempty"`
	Missing []string     `json:"missing,omitempty"`
	// From is where it's from: a repo (with the release it is, or built
	// here from its source), or a folder on this machine.
	From PluginFrom `json:"from"`
	// What it adds: its views, by title; the word after ws its commands
	// are under; whether it has a guide for agents; and how its things
	// are named (RAM-…).
	Views    []string `json:"views,omitempty"`
	Commands string   `json:"commands,omitempty"`
	Guide    bool     `json:"guide,omitempty"`
	Names    []string `json:"names,omitempty"`
	// Refused is what it registered that ws left out, and why.
	Refused []string `json:"refused,omitempty"`
	// Path is where its first view opens in ws: "" for one with none.
	Path string `json:"path,omitempty"`
	// Latest is its repo's latest release, as last found; Newer, whether
	// that's later than the one installed; and UpdateError why the last
	// look didn't find it.
	Latest      string `json:"latest,omitempty"`
	Newer       bool   `json:"newer"`
	UpdateError string `json:"updateError,omitempty"`
}

// PluginFrom is where an installed plugin came from. Repo is the repo
// it's published at, Tag the release (or the tag its source was built
// at), Built whether it was built here from its source, and At when it
// was fetched. Linked is the folder it's run from (ws plugin add --link).
// None of them: it was copied from a folder.
type PluginFrom struct {
	Repo   string     `json:"repo,omitempty"`
	Tag    string     `json:"tag,omitempty"`
	Built  bool       `json:"built,omitempty"`
	At     *time.Time `json:"at,omitempty"`
	Linked string     `json:"linked,omitempty"`
}

// WantedPlugin is a plugin the team uses that isn't installed here.
type WantedPlugin struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Version string `json:"version,omitempty"`
}

// InstalledPluginOf is an installed plugin as the API serves it.
func InstalledPluginOf(row ops.PluginRow) InstalledPlugin {
	st, reg := row.Status, row.Status.Registration
	p := InstalledPlugin{
		Name: st.Name, Description: reg.Description, Version: st.Version, State: st.State, Error: st.Error, Missing: st.Missing,
		Commands: reg.CommandName(st.Name), Guide: reg.Agent != nil && reg.Agent.Guide != "", Refused: st.Refused,
		Latest: row.Update.Latest, Newer: row.Update.Newer, UpdateError: row.Update.Error,
		From: PluginFrom{Linked: st.Linked},
	}
	if st.Source != nil {
		p.From.Repo, p.From.Tag, p.From.Built = st.Source.Repo, st.Source.Tag, st.Source.File == ""
		p.From.At = timeOrNil(st.Source.At)
	}
	for i, v := range reg.Views {
		p.Views = append(p.Views, v.Title)
		if i == 0 {
			p.Path = "/x/" + st.Name
			if v.Path != "" && v.Path != "/" && plugin.ValidUIPath(v.Path) {
				p.Path += v.Path
			}
		}
	}
	for _, r := range reg.Refs {
		for _, prefix := range r.Prefixes {
			p.Names = append(p.Names, prefix+"…")
		}
	}
	return p
}

// pluginsInstalled is GET /api/plugins/installed.
func (s *Server) pluginsInstalled(w http.ResponseWriter, r *http.Request) {
	view := s.App.Plugins(r.Context())
	out := PluginsInstalled{Plugins: []InstalledPlugin{}, Wanted: []WantedPlugin{}, Checks: view.Checks, Checked: timeOrNil(view.Checked)}
	for _, row := range view.Plugins {
		out.Plugins = append(out.Plugins, InstalledPluginOf(row))
	}
	for _, p := range view.Wanted {
		out.Wanted = append(out.Wanted, WantedPlugin{Name: p.Name, Source: p.Source, Version: p.Version})
	}
	writeJSON(w, http.StatusOK, out)
}

// PluginAddRequest is POST /api/plugins: a plugin to install. From is the
// repo it's published at (owner/repo, with @tag for a release that isn't
// the latest), or a folder on this machine, by its whole path or from ~.
type PluginAddRequest struct {
	From string `json:"from"`
}

// PluginChanged is what adding or updating a plugin did: which plugin,
// the version it is now, what it was (an update's) and whether it was the
// latest already, where it stands, and what was done on the way, in
// lines (a download, a build from source).
type PluginChanged struct {
	Name    string       `json:"name"`
	Version string       `json:"version,omitempty"`
	Was     string       `json:"was,omitempty"`
	Now     string       `json:"now,omitempty"`
	Current bool         `json:"current,omitempty"`
	State   plugin.State `json:"state,omitempty"`
	Missing []string     `json:"missing,omitempty"`
	Said    []string     `json:"said,omitempty"`
}

// pluginFailed answers for a plugin that couldn't be added or updated:
// why, and what building it printed, where it got that far.
func pluginFailed(w http.ResponseWriter, err error, added ops.PluginAdded, said []string) {
	body := map[string]any{"error": err.Error()}
	status := http.StatusInternalServerError
	var refused *ops.Refused
	var inv *ops.Invalid
	var up *ops.Upstream
	switch {
	case errors.Is(err, ops.ErrNotFound):
		status = http.StatusNotFound
	case errors.As(err, &inv):
		status = http.StatusUnprocessableEntity
	case errors.As(err, &refused):
		status, body["code"] = http.StatusConflict, refused.Code
	case errors.As(err, &up):
		status = http.StatusBadGateway
	}
	if out := strings.TrimSpace(added.BuildOutput); out != "" {
		body["output"] = out
	}
	if len(said) > 0 {
		body["said"] = said
	}
	writeJSON(w, status, body)
}

// addPlugin is POST /api/plugins: install a plugin, and start it. It
// answers once the plugin's started (or hasn't): a download, or a build
// from source, takes as long as it takes.
func (s *Server) addPlugin(w http.ResponseWriter, r *http.Request) {
	var req PluginAddRequest
	if !readBody(w, r, &req) {
		return
	}
	var said []string
	added, _, err := s.App.AddPlugin(r.Context(), req.From, ops.PluginAdd{Say: func(line string) { said = append(said, line) }})
	if err != nil {
		pluginFailed(w, err, added, said)
		return
	}
	writeJSON(w, http.StatusOK, PluginChanged{Name: added.Name, Version: added.Version, Now: added.Release, State: added.State, Missing: added.Missing, Said: said})
}

// updatePlugin is POST /api/plugins/{name}/update: fetch its repo's
// latest release, if that's not what's here, and start it afresh.
func (s *Server) updatePlugin(w http.ResponseWriter, r *http.Request) {
	var said []string
	got, err := s.App.UpdatePlugin(r.Context(), r.PathValue("name"), func(line string) { said = append(said, line) })
	if err != nil {
		pluginFailed(w, err, got.Added, said)
		return
	}
	writeJSON(w, http.StatusOK, PluginChanged{Name: got.Name, Version: got.Added.Version, Was: got.Was, Now: got.Now, Current: got.Current, State: got.Added.State, Missing: got.Added.Missing, Said: said})
}

// PluginRemoveRequest is POST /api/plugins/{name}/remove. Data says what
// the plugin kept for itself (its data folder) and its secrets go too.
type PluginRemoveRequest struct {
	Data bool `json:"data"`
}

// removePlugin is POST /api/plugins/{name}/remove: stop a plugin and take
// it out of the workspace.
func (s *Server) removePlugin(w http.ResponseWriter, r *http.Request) {
	var req PluginRemoveRequest
	if !readBody(w, r, &req) {
		return
	}
	name := r.PathValue("name")
	if err := s.App.RemovePlugin(r.Context(), name, req.Data); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"removed": name})
}

func (s *Server) pluginsList(w http.ResponseWriter, r *http.Request) {
	out := []Plugin{}
	if s.Plugins != nil {
		statuses := s.Plugins.Statuses()
		// How each one's things are named, read once for them all: none
		// to read with no plugins.
		refs := map[string][]PluginRef{}
		if len(statuses) > 0 {
			kept, _ := plugin.Refs(s.Plugins.Root)
			for _, k := range kept {
				r := PluginRef{Kind: k.Kind, Title: k.Title, Icon: k.Icon, Prefixes: k.Prefixes, Tail: k.Tail, Address: k.Address, Path: k.Path, Fields: k.Fields, Actions: k.Actions}
				// A view with nothing in it is no view: ws draws its own.
				for where, blocks := range k.Views {
					if len(blocks) > 0 {
						if r.Views == nil {
							r.Views = map[string][]protocol.Block{}
						}
						r.Views[where] = blocks
					}
				}
				refs[k.Plugin] = append(refs[k.Plugin], r)
			}
		}
		for _, st := range statuses {
			p := Plugin{Refs: refs[st.Name], Actions: st.Registration.Actions, Name: st.Name, Description: st.Registration.Description, State: st.State, Error: st.Error, Missing: st.Missing, Views: []PluginView{}, Changed: st.Changed, Starts: st.Starts}
			for _, v := range st.Registration.Views {
				path := v.Path
				if path == "" {
					path = "/"
				}
				p.Views = append(p.Views, PluginView{Title: v.Title, Icon: v.Icon, Under: v.Under, Path: path, Badge: st.Badges[path]})
			}
			out = append(out, p)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// pluginThing is GET /api/plugins/{name}/refs/{kind}/{ref}: one of a
// plugin's things, for the web UI to draw (its card, the pane beside
// Find's list). From what the plugin's told ws of it, where it has: no
// plugin's asked for those. Otherwise the plugin's asked, now. 404 is
// there being none by that name; anything else is the plugin not being
// there to ask.
func (s *Server) pluginThing(w http.ResponseWriter, r *http.Request) {
	name, kind, ref := r.PathValue("name"), r.PathValue("kind"), r.PathValue("ref")
	if s.Plugins == nil || !plugin.IsInstalled(s.Plugins.Root, name) {
		jsonError(w, http.StatusBadGateway, "there's no plugin called "+name)
		return
	}
	// As ws keeps how the plugin names things: the name has to be one.
	kept, _ := plugin.RefsOf(s.Plugins.Root, name)
	i := slices.IndexFunc(kept, func(k plugin.KeptRef) bool { return k.Kind == kind })
	if i >= 0 {
		// One the plugin told ws of, by a name its kind names; or one ws
		// found in a space, by its folder.
		if rec, ok, err := plugin.ThingOf(s.Plugins.Root, name, kind, ref); err == nil && ok && (rec.Found != "" || kept[i].Named().Names(ref)) {
			out := PluginThing{Key: ref, Fields: rec.Fields, Told: true}
			if !rec.Changed.IsZero() {
				out.Changed = &rec.Changed
			}
			writeJSON(w, http.StatusOK, out)
			return
		}
	}
	shown, err := s.Plugins.Show(r.Context(), name, kind, ref, false)
	switch {
	case errors.Is(err, plugin.ErrNoThing):
		jsonError(w, http.StatusNotFound, err.Error())
		return
	case errors.Is(err, plugin.ErrNoShow):
		// It says no more than it's told ws, and it hasn't told ws of this.
		jsonError(w, http.StatusNotFound, name+" hasn't told ws of "+ref)
		return
	case err != nil:
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	// What its kind has a field for, as ws would keep it: its kind as ws
	// has it now, since asking may have been what started the plugin.
	out := PluginThing{Key: ref, Fields: map[string]string{}}
	kept, _ = plugin.RefsOf(s.Plugins.Root, name)
	if i := slices.IndexFunc(kept, func(k plugin.KeptRef) bool { return k.Kind == kind }); i >= 0 {
		out.Fields = plugin.Tidy(kept[i].Ref, shown.Fields)
	}
	if !shown.Changed.IsZero() {
		out.Changed = &shown.Changed
	}
	writeJSON(w, http.StatusOK, out)
}

// PluginActRequest is POST /api/plugins/{name}/actions/{id}: an action
// chosen in Find. Kind and Ref are for one of a kind's, on one of its
// things; neither is for one of the plugin's own. Answers are what's been
// answered so far to what the action asked.
type PluginActRequest struct {
	Kind    string   `json:"kind,omitempty"`
	Ref     string   `json:"ref,omitempty"`
	Answers []string `json:"answers"`
}

// PluginActed is what came of an action: Ask, when it wants something
// answered first and has done nothing yet; else a line to say, and where
// in ws to go.
type PluginActed struct {
	Ask  *protocol.Ask `json:"ask,omitempty"`
	Say  string        `json:"say,omitempty"`
	Open string        `json:"open,omitempty"`
}

// pluginAct is POST /api/plugins/{name}/actions/{id}: run one of a
// plugin's actions, or ask what it asks.
func (s *Server) pluginAct(w http.ResponseWriter, r *http.Request) {
	name, id := r.PathValue("name"), r.PathValue("id")
	if s.Plugins == nil || !plugin.IsInstalled(s.Plugins.Root, name) {
		jsonError(w, http.StatusNotFound, "there's no plugin called "+name)
		return
	}
	var req PluginActRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "what to do isn't JSON: "+err.Error())
		return
	}
	done, err := s.Plugins.Act(r.Context(), name, protocol.ActionRunParams{ID: id, Kind: req.Kind, Ref: req.Ref, Answers: req.Answers})
	switch {
	case errors.Is(err, plugin.ErrNoAction):
		jsonError(w, http.StatusNotFound, err.Error())
		return
	case err != nil:
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	out := PluginActed{Ask: done.Ask, Say: done.Say}
	// Where it opens is a path in the plugin's app: its page in ws.
	if done.Open != "" && plugin.ValidUIPath(done.Open) {
		out.Open = "/x/" + name + done.Open
	}
	writeJSON(w, http.StatusOK, out)
}

// StartRequest is POST /api/missions: a mission to make, about something
// ws names (one of a plugin's things, chosen in Find).
type StartRequest struct {
	About string `json:"about"`
}

// Started is the mission that was made: its slug, and its page.
type Started struct {
	Slug string `json:"slug"`
	Path string `json:"path"`
}

// startMission is POST /api/missions: a mission about something ws names,
// made to brief: whoever briefs it starts from what it's about.
func (s *Server) startMission(w http.ResponseWriter, r *http.Request) {
	var req StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.About) == "" {
		jsonError(w, http.StatusUnprocessableEntity, "say what the mission's about")
		return
	}
	m, err := s.App.StartAbout(req.About)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, Started{Slug: m.Slug, Path: "/missions/" + m.Slug})
}

// pluginHeaders are what every answer under /plugins has, in place of the
// guard's, which would refuse to be framed and run no inline script.
func pluginHeaders(h http.Header) {
	h.Set("Content-Security-Policy", PluginPolicy)
	h.Set("X-Frame-Options", "SAMEORIGIN")
	// What a plugin serves says where it is to its own HTTP, and to nowhere
	// else. The app's pages say it to nobody: the sign-in link's address
	// carries the token.
	h.Set("Referrer-Policy", "same-origin")
}

// shellHeaders are what the shell has: as pluginHeaders, under the shell's
// own policy. It carries the accent and what the plugin registered, so
// it's never kept.
func shellHeaders(h http.Header) {
	h.Set("Content-Security-Policy", ShellPolicy)
	h.Set("X-Frame-Options", "SAMEORIGIN")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
}

// pluginTransports are the connections to plugins' views, by socket.
var pluginTransports sync.Map // socket path → *http.Transport

func pluginTransport(sock string) *http.Transport {
	if t, ok := pluginTransports.Load(sock); ok {
		return t.(*http.Transport)
	}
	t := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
	}
	actual, _ := pluginTransports.LoadOrStore(sock, t)
	return actual.(*http.Transport)
}

// FrameBase is the path a plugin's shell is served under: the address of
// its view in ws, /x/<name>/…, under a prefix of the frame's own.
func FrameBase(name string) string { return "/plugin-frame/" + name }

// shell is the page ws serves for a plugin's view: ws's tokens, an empty
// #root, and the plugin's app. Where the app can't be loaded, it has a
// message of ws's in #root and no app.
type shell struct {
	Title string
	// Script and Style are the app's entry and its stylesheet, as addresses
	// in ws; "" for none.
	Script, Style string
	// Message is HTML for #root, in place of the app.
	Message string
}

// pluginSocket is the socket of a plugin's own HTTP, the plugin started if
// it isn't running. Where it can't be asked, the answer's written, as the
// shell with a message saying why, and ok is false.
func (s *Server) pluginSocket(w http.ResponseWriter, name string) (sock string, ok bool) {
	if s.Plugins == nil || !plugin.IsInstalled(s.Plugins.Root, name) {
		s.shellMessage(w, http.StatusNotFound, "There's no such plugin", "<p><code>ws plugin list</code> says which this workspace has.</p>")
		return "", false
	}
	sock, err := s.Plugins.ViewSocket(name)
	var setup *plugin.SetupError
	switch {
	case errors.As(err, &setup):
		keys := make([]string, len(setup.Missing))
		for i, k := range setup.Missing {
			title := k.Title
			if title == "" {
				title = k.Key
			}
			keys[i] = "<li>" + html.EscapeString(title) + " (<code>" + html.EscapeString(k.Key) + "</code>)</li>"
		}
		s.shellMessage(w, http.StatusServiceUnavailable, name+" needs setting up",
			"<p>It can't be used until these are set:</p><ul>"+strings.Join(keys, "")+"</ul><p>In a terminal: <code>ws plugin config "+html.EscapeString(name)+" --ask</code></p>")
		return "", false
	case err != nil:
		s.shellMessage(w, http.StatusBadGateway, name+" didn't start",
			"<p>"+html.EscapeString(err.Error())+"</p><p>Its log: <code>ws plugin logs "+html.EscapeString(name)+"</code></p>")
		return "", false
	}
	return sock, true
}

// pluginShell is GET /plugin-frame/{name}/{rest...}: the shell, for every
// path under the plugin's name. Which paths the app has is the app's to
// say, once it's loaded: so a link to somewhere deep in it, and a reload,
// load it there.
func (s *Server) pluginShell(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if outsideFrame(w, r, name) {
		return
	}
	if _, ok := s.pluginSocket(w, name); !ok {
		return
	}
	reg := s.Plugins.Status(name).Registration
	switch {
	case reg.UI == nil || reg.UI.Script == "":
		s.shellMessage(w, http.StatusBadGateway, name+" has no app to show",
			"<p>It was built on a framework from before plugin pages were apps: update it.</p>")
		return
	case !plugin.ValidUIPath(reg.UI.Script) || reg.UI.Style != "" && !plugin.ValidUIPath(reg.UI.Style):
		s.shellMessage(w, http.StatusBadGateway, name+"'s app can't be loaded",
			"<p>What it registered as its <code>ui</code> isn't a path in its own HTTP. <code>ws plugin check</code> says what's wrong with it.</p>")
		return
	}
	page := shell{Title: name, Script: plugin.ViewBase(name) + reg.UI.Script}
	if len(reg.Views) > 0 && reg.Views[0].Title != "" {
		page.Title = reg.Views[0].Title
	}
	if reg.UI.Style != "" {
		page.Style = plugin.ViewBase(name) + reg.UI.Style
	}
	s.writeShell(w, http.StatusOK, page)
}

// pluginShellRoot is GET /plugin-frame/{name}: the shell at the app's
// first path.
func (s *Server) pluginShellRoot(w http.ResponseWriter, r *http.Request) {
	if outsideFrame(w, r, r.PathValue("name")) {
		return
	}
	http.Redirect(w, r, FrameBase(r.PathValue("name"))+"/", http.StatusSeeOther)
}

// outsideFrame sends a shell asked for as a page of its own, not in ws's
// frame (a relative link in the app opened in a tab, say), to the same
// path under /x/<name>/, where ws shows it: the bare shell is no use to
// anyone. The browser says which (Sec-Fetch-Dest); one that doesn't say is
// given the shell.
func outsideFrame(w http.ResponseWriter, r *http.Request, name string) bool {
	if r.Header.Get("Sec-Fetch-Dest") != "document" {
		return false
	}
	to := "/x/" + name + "/" + r.PathValue("rest")
	if r.URL.RawQuery != "" {
		to += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
	return true
}

// pluginView is /plugins/{name}/{rest...}: the plugin's own HTTP, asked of
// the plugin, which is started if it isn't running. It's where its app's
// files are, and what its app asks it.
func (s *Server) pluginView(w http.ResponseWriter, r *http.Request) {
	pluginHeaders(w.Header())
	name := r.PathValue("name")
	sock, ok := s.pluginSocket(w, name)
	if !ok {
		return
	}
	base := plugin.ViewBase(name)
	proxy := &httputil.ReverseProxy{
		Transport: pluginTransport(sock),
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme, pr.Out.URL.Host = "http", "plugin"
			pr.Out.URL.Path, pr.Out.URL.RawPath = "/"+r.PathValue("rest"), ""
			pr.Out.Host = r.Host
			// The plugin is given the request, not who's signed in to ws.
			pr.Out.Header.Del("Cookie")
			pr.Out.Header.Del("Authorization")
			pr.Out.Header.Set("X-Forwarded-Prefix", base)
		},
		ModifyResponse: func(res *http.Response) error {
			// ws's policy for what a plugin serves is the one that holds.
			res.Header.Del("Content-Security-Policy")
			res.Header.Del("X-Frame-Options")
			// A redirect to a path of the plugin's own is to it under its base.
			if loc := res.Header.Get("Location"); strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "//") && loc != base && !strings.HasPrefix(loc, base+"/") {
				res.Header.Set("Location", base+loc)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			s.shellMessage(w, http.StatusBadGateway, name+" isn't answering",
				"<p>"+html.EscapeString(err.Error())+"</p><p>Its log: <code>ws plugin logs "+html.EscapeString(name)+"</code></p>")
		},
	}
	proxy.ServeHTTP(w, r)
}

// pluginRoot is /plugins/{name}: the root of the plugin's own HTTP.
func (s *Server) pluginRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, plugin.ViewBase(r.PathValue("name"))+"/", http.StatusSeeOther)
}

// shellMessage is the shell with a message of ws's in place of an app: why
// the plugin's can't be shown. title is text, and body is HTML.
func (s *Server) shellMessage(w http.ResponseWriter, status int, title, body string) {
	s.writeShell(w, status, shell{Title: title, Message: "<h2>" + html.EscapeString(title) + "</h2>" + body})
}

// writeShell writes the shell. The theme is there before anything of the
// plugin's has loaded: /theme.js (the web UI's own, a file, as the policy
// runs no inline script) sets light or dark on <html> as it does for the
// web UI, the accent is on <html> as it is there, and the <style> has every
// token for both, worked out from those two by the browser. So the frame's
// first paint is in ws's colours.
func (s *Server) writeShell(w http.ResponseWriter, status int, p shell) {
	styles, err := fs.ReadFile(s.Assets, shellStyles)
	if err != nil {
		page(w, http.StatusInternalServerError, "No web UI built", "This ws was built without its web UI: run <code>just build</code>.")
		return
	}
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\"")
	if accent := s.App.Accent(); hexColor(accent) {
		b.WriteString(` style="--accent-pick: ` + accent + `"`)
	}
	b.WriteString(">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"color-scheme\" content=\"light dark\">\n<script src=\"/theme.js\"></script>\n")
	b.WriteString("<title>" + html.EscapeString(p.Title) + "</title>\n<style>")
	b.Write(styles)
	b.WriteString("</style>\n")
	if p.Message == "" {
		if p.Style != "" {
			b.WriteString(`<link rel="stylesheet" href="` + html.EscapeString(p.Style) + "\">\n")
		}
		b.WriteString(`<script type="module" src="` + html.EscapeString(p.Script) + "\"></script>\n")
	}
	b.WriteString("</head>\n<body><div id=\"root\">")
	if p.Message != "" {
		b.WriteString(`<div class="ws-shell-message">` + p.Message + "</div>")
	}
	b.WriteString("</div></body>\n</html>\n")
	shellHeaders(w.Header())
	w.WriteHeader(status)
	w.Write([]byte(b.String()))
}
