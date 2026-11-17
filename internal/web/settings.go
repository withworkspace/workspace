package web

import (
	"errors"
	"net/http"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/ops"
)

// The Settings page: your settings, in ws.local.toml, each with where its
// value comes from, and for each role what actually runs.

// SettingsInfo is GET /api/settings.
type SettingsInfo struct {
	// File is ws.local.toml's path, and Team ws.toml's.
	File   string          `json:"file"`
	Team   string          `json:"team"`
	Groups []SettingsGroup `json:"groups"`
	// Unknown are keys in either file ws doesn't read ("ws.local.toml:
	// crew.harnes"), as ws doctor lists them.
	Unknown []string `json:"unknown,omitempty"`
}

// SettingsGroup is a part of the page: a role's agent, the orchestrator…
type SettingsGroup struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	About    string    `json:"about,omitempty"`
	Settings []Setting `json:"settings"`
	// Runs is, for a role, what actually runs it as things are set.
	Runs *RoleRun `json:"runs,omitempty"`
}

// RoleRun is the agent a role ends up with: its harness and model (""
// for the agent's own default), why it can't run if it can't, and what
// it follows from where it isn't set itself.
type RoleRun struct {
	Harness string `json:"harness"`
	Model   string `json:"model,omitempty"`
	Problem string `json:"problem,omitempty"`
	Note    string `json:"note,omitempty"`
}

// Setting is one key: what it's set to, and where that comes from
// ("default", "team" for ws.toml, "yours" for ws.local.toml), with each
// file's own value where it has one, and what it would be with neither.
type Setting struct {
	Section string `json:"section"` // "" for a key at the top
	Key     string `json:"key"`
	Label   string `json:"label"`
	About   string `json:"about,omitempty"`
	// Kind is harness, model, choice, bool, number, text, picks (a list
	// of harness:model) or color (a hex colour, #rrggbb).
	Kind    string          `json:"kind"`
	Choices []SettingChoice `json:"choices,omitempty"`
	Value   any             `json:"value"`
	Source  string          `json:"source"`
	Team    any             `json:"team,omitempty"`
	Yours   any             `json:"yours,omitempty"`
	Default any             `json:"default,omitempty"`
	// Harness is a model's: the harness it'd run on, whose models to offer.
	Harness string `json:"harness,omitempty"`
}

// SettingChoice is a value a setting can take. Note says why it may not
// work here (a harness that isn't installed).
type SettingChoice struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
	Note  string `json:"note,omitempty"`
}

// SettingChange sets a key in ws.local.toml, or clears it (Clear), so the
// team's value or the default applies again.
type SettingChange struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Value   any    `json:"value,omitempty"`
	Clear   bool   `json:"clear,omitempty"`
}

// ErrSettingRefused is a change that wasn't made: a value the key can't
// take, or a file ws can't edit safely. Its message says which.
var ErrSettingRefused = ops.ErrSettingRefused

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	info, err := s.App.Settings()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, SettingsOf(info))
}

// setSettings makes changes together (a harness and its model), and
// answers the settings as they now are.
func (s *Server) setSettings(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Changes []SettingChange `json:"changes"`
	}
	if !readBody(w, r, &b) {
		return
	}
	if len(b.Changes) == 0 {
		jsonError(w, http.StatusBadRequest, "no changes")
		return
	}
	changes := make([]ops.SettingChange, 0, len(b.Changes))
	for _, ch := range b.Changes {
		changes = append(changes, ops.SettingChange(ch))
	}
	info, err := s.App.SetSettings(changes)
	switch {
	case errors.Is(err, ErrSettingRefused):
		jsonError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		jsonError(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, SettingsOf(info))
	}
}

// models answers the models a harness offers (GET
// /api/settings/models?harness=codex), asked of its CLI.
func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	ms, err := s.App.Models(r.URL.Query().Get("harness"))
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if ms == nil {
		ms = []harness.Model{}
	}
	writeJSON(w, http.StatusOK, ms)
}

// Settings is the Settings page's: your settings, and changing them.
type Settings interface {
	Settings() (ops.SettingsInfo, error)
	SetSettings(changes []ops.SettingChange) (ops.SettingsInfo, error)
	Models(harness string) ([]harness.Model, error)
	// Accent is the web UI's accent as set, #rrggbb, or "" for its own
	// default: the page is served with it (see Server.app).
	Accent() string
}

// SettingsOf is your settings as the API serves them.
func SettingsOf(in ops.SettingsInfo) SettingsInfo {
	out := SettingsInfo{File: in.File, Team: in.Team, Unknown: in.Unknown}
	for _, g := range in.Groups {
		group := SettingsGroup{ID: g.ID, Title: g.Title, About: g.About}
		if g.Runs != nil {
			run := RoleRun(*g.Runs)
			group.Runs = &run
		}
		for _, st := range g.Settings {
			set := Setting{Section: st.Section, Key: st.Key, Label: st.Label, About: st.About, Kind: st.Kind,
				Value: st.Value, Source: st.Source, Team: st.Team, Yours: st.Yours, Default: st.Default, Harness: st.Harness}
			for _, c := range st.Choices {
				set.Choices = append(set.Choices, SettingChoice(c))
			}
			group.Settings = append(group.Settings, set)
		}
		out.Groups = append(out.Groups, group)
	}
	return out
}
