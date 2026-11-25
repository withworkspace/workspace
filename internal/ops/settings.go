package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/team"
)

// The Settings page: your settings, the ones in ws.local.toml, each with
// where its value comes from (ws's default, the team's ws.toml, or
// yours), and for each role the agent it ends up with. Changes are
// written into ws.local.toml key by key (config.EditLocal), so the rest
// of the file stays as you wrote it.
//
// Left off, file only: commands ([repos.*] after_create, verify), which a
// page that writes them would let anyone signed in run; paths; and the
// state ws keeps there itself ([boarded], [silo]).

// What the Settings page shows and sends, for web to map to its API.

// SettingsInfo is your settings (in File, ws.local.toml), by group, and
// the keys ws.toml and ws.local.toml have that ws doesn't read.
type SettingsInfo struct {
	File, Team string
	Groups     []SettingsGroup
	Unknown    []string
}

// SettingsGroup is a section of the page: a role's agent (Runs, what it
// ends up running), or a group of keys.
type SettingsGroup struct {
	ID, Title, About string
	Settings         []Setting
	Runs             *RoleRun
}

// Setting is a key: what it is, what it can be, what it is now and where
// that comes from (default, team, yours), and for a model, the harness its
// role runs on.
type Setting struct {
	Section, Key, Label, About, Kind string
	Choices                          []SettingChoice
	Value                            any
	Source                           string
	Team, Yours, Default             any
	Harness                          string
}

// SettingChoice is a value a key can take, and why it can't be used here,
// if it can't (Note).
type SettingChoice struct{ Value, Label, Note string }

// SettingChange is a change to a key: a value, or Clear.
type SettingChange struct {
	Section, Key string
	Value        any
	Clear        bool
}

// RoleRun is what a role ends up running, and why it can't, if it can't.
type RoleRun struct{ Harness, Model, Problem, Note string }

// ErrSettingRefused is a change the Settings page can't make.
var ErrSettingRefused = errors.New("not changed")

// settingDef is one key the page shows and writes.
type settingDef struct {
	section, key, label, about, kind string
	choices                          func() []SettingChoice
	// team and yours are what ws.toml and ws.local.toml say (nil: nothing),
	// and def what it would be with neither setting it, given the rest.
	team  func(c *config.Config) any
	yours func(l *config.LocalConfig) any
	def   func(c config.Config) any
	// set sets it in ws.local.toml (nil: clears it), refusing a value the
	// key can't take.
	set func(l *config.LocalConfig, v any) error
	// harness is a model's: the harness its role runs on, as things are.
	harness func(c *config.Config) string
}

// roleDef is a role's agent: its section's harness and model, the
// harnesses that can play it, and what it ends up running.
type roleDef struct {
	id, title, about string
	names            func() []string
	team             func(c *config.Config) (h, m *string)
	local            func(l *config.LocalConfig) (h, m *string)
	resolve          func(c *config.Config) (harness.Pick, error)
	note             func(c *config.Config) string
}

var settingRoles = []roleDef{
	{
		id: "crew", title: "Crew", about: "The agents doing the work: dispatched by the orchestrator, or launched from a mission's page.",
		names: harness.CrewNames,
		team:  func(c *config.Config) (*string, *string) { return &c.Crew.Harness, &c.Crew.Model },
		local: func(l *config.LocalConfig) (*string, *string) { return &l.Crew.Harness, &l.Crew.Model },
		resolve: func(c *config.Config) (harness.Pick, error) {
			return Roles(c).ForCrew(), nil
		},
		note: func(c *config.Config) string {
			if env := os.Getenv(harness.EnvDefault); env != "" {
				return fmt.Sprintf("%s=%s overrides this wherever it's set.", harness.EnvDefault, env)
			}
			return ""
		},
	},
	{
		id: "briefing", title: "Briefing", about: "The agent that turns a request into a brief and capsules.",
		names: briefingNames,
		team:  func(c *config.Config) (*string, *string) { return &c.Briefing.Harness, &c.Briefing.Model },
		local: func(l *config.LocalConfig) (*string, *string) { return &l.Briefing.Harness, &l.Briefing.Model },
		resolve: func(c *config.Config) (harness.Pick, error) {
			if c.BriefInTerminal() {
				return Roles(c).ForBriefing(), nil
			}
			p, _, err := Roles(c).ForBriefingConversation()
			return p, err
		},
		note: func(c *config.Config) string {
			if c.Briefing.Harness != "" {
				return ""
			}
			if c.BriefInTerminal() {
				return "It follows crew's harness, as none is set here."
			}
			return "It follows conversations' harness, as none is set here."
		},
	},
	{
		id: "orchestrator", title: "Orchestrator", about: "The headless agent that decides what's next on a mission when crew finish.",
		names: orchestratorNames,
		team:  func(c *config.Config) (*string, *string) { return &c.Orchestrator.Harness, &c.Orchestrator.Model },
		local: func(l *config.LocalConfig) (*string, *string) {
			return &l.Orchestrator.Harness, &l.Orchestrator.Model
		},
		resolve: func(c *config.Config) (harness.Pick, error) {
			p, _, err := Roles(c).ForOrchestrator()
			return p, err
		},
		note: func(c *config.Config) string {
			if !c.OrchestratorEnabled() {
				return "It's off: it doesn't run by itself."
			}
			return ""
		},
	},
	{
		id: "conversation", title: "Conversations", about: "The agent answering conversations, document threads, cells and passes.",
		names: converserNames,
		team:  func(c *config.Config) (*string, *string) { return &c.Conversation.Harness, &c.Conversation.Model },
		local: func(l *config.LocalConfig) (*string, *string) {
			return &l.Conversation.Harness, &l.Conversation.Model
		},
		resolve: func(c *config.Config) (harness.Pick, error) {
			p, _, err := Roles(c).ForConversation()
			return p, err
		},
		note: func(c *config.Config) string {
			if c.Team.Harness == "" {
				return "The Team page's gists follow this too, as [team] names no harness."
			}
			return ""
		},
	},
	{
		id: "judge", title: "Judge", about: "The agent judging a Work step's checks in words, once its crew report done: fresh to the work, read-only.",
		names: converserNames,
		team:  func(c *config.Config) (*string, *string) { return &c.Judge.Harness, &c.Judge.Model },
		local: func(l *config.LocalConfig) (*string, *string) { return &l.Judge.Harness, &l.Judge.Model },
		resolve: func(c *config.Config) (harness.Pick, error) {
			p, _, err := Roles(c).ForJudge()
			return p, err
		},
		note: func(c *config.Config) string {
			if c.Judge.Harness == "" {
				return "It follows the planner's agent and model, as none is set here."
			}
			return ""
		},
	},
	{
		id: "pr_writer", title: "PR writer", about: "The agent writing the title and description of each pull request ws opens, from the step it's for and PR.md: fresh to the work, read-only.",
		names: converserNames,
		team:  func(c *config.Config) (*string, *string) { return &c.PRWriter.Harness, &c.PRWriter.Model },
		local: func(l *config.LocalConfig) (*string, *string) { return &l.PRWriter.Harness, &l.PRWriter.Model },
		resolve: func(c *config.Config) (harness.Pick, error) {
			p, _, err := Roles(c).ForPRWriter()
			return p, err
		},
		note: func(c *config.Config) string {
			if !c.PRWriterEnabled() {
				return "It's off ([pr_writer] enabled = false): ws titles each pull request as the mission is, and opens it at once."
			}
			if c.PRWriter.Harness == "" {
				return "It follows conversations' agent and model, as none is set here."
			}
			return ""
		},
	},
	{
		id: "team", title: "Team", about: "The agent writing the Team page's gist of each change.",
		names: converserNames,
		team:  func(c *config.Config) (*string, *string) { return &c.Team.Harness, &c.Team.Model },
		local: func(l *config.LocalConfig) (*string, *string) { return &l.Team.Harness, &l.Team.Model },
		resolve: func(c *config.Config) (harness.Pick, error) {
			p, _, err := Roles(c).ForTeam()
			return p, err
		},
		note: func(c *config.Config) string {
			if c.Team.Harness == "" {
				return "It follows conversations' harness, on its small model, as none is set here."
			}
			return ""
		},
	},
}

// The harnesses that can play each kind of role.
func orchestratorNames() []string {
	return harness.NamesWhere(func(c harness.Caps) bool { return c.Headless })
}

func converserNames() []string {
	return harness.NamesWhere(func(c harness.Caps) bool { return c.Converse })
}

// briefingNames can brief in a terminal (crew's) or in a conversation.
func briefingNames() []string {
	return harness.NamesWhere(func(c harness.Caps) bool { return c.Converse || c.Crew != harness.CrewNone })
}

// harnessChoices are names as choices, each saying if it can't run here.
func harnessChoices(names []string) []SettingChoice {
	out := make([]SettingChoice, 0, len(names))
	for _, n := range names {
		c := SettingChoice{Value: n}
		if h, err := harness.Get(n); err == nil {
			if err := h.Available(); err != nil {
				c.Note = err.Error()
			}
		}
		out = append(out, c)
	}
	return out
}

// strOrNil is a string setting's value: nil for unset.
func strOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// roleSettings are a role's harness and model.
func roleSettings(r roleDef) []settingDef {
	resolved := func(c config.Config, clear func(c *config.Config)) harness.Pick {
		clear(&c)
		p, _ := r.resolve(&c)
		return p
	}
	return []settingDef{{
		section: r.id, key: "harness", label: "Agent", kind: "harness",
		about:   "The agent CLI it runs in.",
		choices: func() []SettingChoice { return harnessChoices(r.names()) },
		team:    func(c *config.Config) any { h, _ := r.team(c); return strOrNil(*h) },
		yours:   func(l *config.LocalConfig) any { h, _ := r.local(l); return strOrNil(*h) },
		def: func(c config.Config) any {
			return strOrNil(resolved(c, func(c *config.Config) { h, _ := r.team(c); *h = "" }).Harness)
		},
		set: func(l *config.LocalConfig, v any) error {
			h, _ := r.local(l)
			if v == nil {
				*h = ""
				return nil
			}
			s, ok := v.(string)
			if !ok || !slices.Contains(r.names(), s) {
				return fmt.Errorf("%v can't run %s: it's one of %s", v, strings.ToLower(r.title), strings.Join(r.names(), ", "))
			}
			*h = s
			return nil
		},
	}, {
		section: r.id, key: "model", label: "Model", kind: "model",
		about: "In that agent's own naming. Unset, the agent's default for the role.",
		team:  func(c *config.Config) any { _, m := r.team(c); return strOrNil(*m) },
		yours: func(l *config.LocalConfig) any { _, m := r.local(l); return strOrNil(*m) },
		def: func(c config.Config) any {
			return strOrNil(resolved(c, func(c *config.Config) { _, m := r.team(c); *m = "" }).Model)
		},
		set: func(l *config.LocalConfig, v any) error {
			_, m := r.local(l)
			if v == nil {
				*m = ""
				return nil
			}
			s, err := settingText(v, 200)
			if err != nil {
				return err
			}
			*m = s
			return nil
		},
		harness: func(c *config.Config) string { p, _ := r.resolve(c); return p.Harness },
	}}
}

// settingText is a text value: one line, not empty, at most max long.
func settingText(v any, max int) (string, error) {
	s, ok := v.(string)
	s = strings.TrimSpace(s)
	switch {
	case !ok || s == "":
		return "", errors.New("give it a value, or clear it")
	case len(s) > max:
		return "", fmt.Errorf("that's longer than %d characters", max)
	case strings.IndexFunc(s, unicode.IsControl) >= 0:
		return "", errors.New("it's one line")
	}
	return s, nil
}

// choiceSetting is a string setting that's one of choices.
func choiceSetting(section, key, label, about, def string, choices []SettingChoice,
	team func(c *config.Config) *string, local func(l *config.LocalConfig) *string) settingDef {
	return settingDef{
		section: section, key: key, label: label, about: about, kind: "choice",
		choices: func() []SettingChoice { return choices },
		team:    func(c *config.Config) any { return strOrNil(*team(c)) },
		yours:   func(l *config.LocalConfig) any { return strOrNil(*local(l)) },
		def:     func(config.Config) any { return def },
		set: func(l *config.LocalConfig, v any) error {
			if v == nil {
				*local(l) = ""
				return nil
			}
			s, _ := v.(string)
			for _, c := range choices {
				if c.Value == s {
					*local(l) = s
					return nil
				}
			}
			var vals []string
			for _, c := range choices {
				vals = append(vals, c.Value)
			}
			return fmt.Errorf("%v isn't one of %s", v, strings.Join(vals, ", "))
		},
	}
}

// boolSetting is an on/off setting, unset meaning def.
func boolSetting(section, key, label, about string, def bool,
	team func(c *config.Config) **bool, local func(l *config.LocalConfig) **bool) settingDef {
	get := func(p *bool) any {
		if p == nil {
			return nil
		}
		return *p
	}
	return settingDef{
		section: section, key: key, label: label, about: about, kind: "bool",
		team:  func(c *config.Config) any { return get(*team(c)) },
		yours: func(l *config.LocalConfig) any { return get(*local(l)) },
		def:   func(config.Config) any { return def },
		set: func(l *config.LocalConfig, v any) error {
			if v == nil {
				*local(l) = nil
				return nil
			}
			b, ok := v.(bool)
			if !ok {
				return errors.New("it's on or off")
			}
			*local(l) = &b
			return nil
		},
	}
}

// settingDefs are the page's settings, by group.
func settingDefs(root string) []struct {
	group    roleDef
	settings []settingDef
} {
	var out []struct {
		group    roleDef
		settings []settingDef
	}
	for _, r := range settingRoles {
		defs := roleSettings(r)
		switch r.id {
		case "crew":
			defs = append(defs, allowedSetting())
		case "briefing":
			defs = append(defs, choiceSetting("briefing", "in", "Where", "Where a mission's briefing happens.", "web",
				[]SettingChoice{{Value: "web", Label: "In a conversation in the web UI"}, {Value: "terminal", Label: "By a briefing agent in a terminal"}},
				func(c *config.Config) *string { return &c.Briefing.In },
				func(l *config.LocalConfig) *string { return &l.Briefing.In }))
		case "orchestrator":
			defs = append(defs,
				boolSetting("orchestrator", "enabled", "Runs by itself", "Whether it runs when crew report done or end. Off, nothing moves a mission on but you.", true,
					func(c *config.Config) **bool { return &c.Orchestrator.Enabled },
					func(l *config.LocalConfig) **bool { return &l.Orchestrator.Enabled }),
				choiceSetting("orchestrator", "dispatch", "Crew it starts", "Whether the crew it dispatches launch at once, or wait for you.", "auto",
					[]SettingChoice{{Value: "auto", Label: "Start at once"}, {Value: "ask", Label: "Wait for you to launch them"}},
					func(c *config.Config) *string { return &c.Orchestrator.Dispatch },
					func(l *config.LocalConfig) *string { return &l.Orchestrator.Dispatch }),
				maxDispatchesSetting())
		case "team":
			defs = append(defs,
				boolSetting("team", "narrative", "Gists", "Whether an agent writes a gist of each change, and what's worth knowing of it. Off, the page is the facts alone.", true,
					func(c *config.Config) **bool { return &c.Team.Narrative },
					func(l *config.LocalConfig) **bool { return &l.Team.Narrative }),
				windowSetting())
		}
		out = append(out, struct {
			group    roleDef
			settings []settingDef
		}{r, defs})
	}
	out = append(out, struct {
		group    roleDef
		settings []settingDef
	}{roleDef{id: "missions", title: "Guarantees", about: "What ws holds on every mission, where it acts, whatever an agent reads. The process's preferences are the planner's and the orchestrator's to apply; these are ws's own."}, []settingDef{
		boolSetting("missions", "review_before_pr", "Your review before a pull request", "ws opens a pull request from a mission's capsule only once your latest review of it approves the commit it's at.", false,
			func(c *config.Config) **bool { return &c.Missions.ReviewBeforePR },
			func(l *config.LocalConfig) **bool { return &l.Missions.ReviewBeforePR }),
		boolSetting("missions", "review_before_land", "Your review before landing", "ws lands a mission's capsule, in a repo with no pull requests, only once your latest review of it approves it as it is.", false,
			func(c *config.Config) **bool { return &c.Missions.ReviewBeforeLand },
			func(l *config.LocalConfig) **bool { return &l.Missions.ReviewBeforeLand }),
		boolSetting("missions", "review_before_ready", "Your review before a draft's ready", "ws marks a mission's draft pull request ready only once your latest review of its capsule approves the commit it's at.", false,
			func(c *config.Config) **bool { return &c.Missions.ReviewBeforeReady },
			func(l *config.LocalConfig) **bool { return &l.Missions.ReviewBeforeReady }),
		boolSetting("missions", "replies_need_me", "Replies to people wait for you", "A reply crew draft to a person on a mission's pull request is posted, as you, only once you approve it. A reply to a bot needn't wait.", true,
			func(c *config.Config) **bool { return &c.Missions.RepliesNeedMe },
			func(l *config.LocalConfig) **bool { return &l.Missions.RepliesNeedMe }),
		boolSetting("missions", "end_when_merged", "Merged missions end themselves", "Once the orchestrator suggests a mission landed (every part of Z met, every pull request merged) and nothing else on it waits for you or crew, ws ends it. Off, the ending suggested waits for you.", true,
			func(c *config.Config) **bool { return &c.Missions.EndWhenMerged },
			func(l *config.LocalConfig) **bool { return &l.Missions.EndWhenMerged }),
		maxRoundsSetting(),
	}})
	out = append(out, struct {
		group    roleDef
		settings []settingDef
	}{roleDef{id: "workspace", title: "Repos", about: "How repos are cloned and fetched."}, []settingDef{
		choiceSetting("", "git", "Git protocol", "How repos are cloned.", "https",
			[]SettingChoice{{Value: "https", Label: "HTTPS"}, {Value: "ssh", Label: "SSH"}},
			func(c *config.Config) *string { return &c.Git },
			func(l *config.LocalConfig) *string { return &l.Git }),
	}})
	out = append(out, struct {
		group    roleDef
		settings []settingDef
	}{roleDef{id: "router", title: "Local names", about: "Names under .localhost for the web UI and what ws runs, served by one router for every workspace on this machine (ws router). Yours alone: it listens on your machine."}, []settingDef{
		routerEnabledSetting(), routerNameSetting(root),
	}})
	out = append(out, struct {
		group    roleDef
		settings []settingDef
	}{roleDef{id: "updates", title: "Updates", about: "Later releases of ws, and of the plugins installed from a repo."}, []settingDef{
		// Yours alone (ws.toml has no [updates]): it's your machine that asks.
		boolSetting("updates", "check", "Look for updates", "Once a day, the daemon asks GitHub for the latest release of ws, and of each plugin installed from a repo. Nothing's installed by looking. Off, it asks only when you say: Check now, here and under Plugins.", true,
			func(*config.Config) **bool { var none *bool; return &none },
			func(l *config.LocalConfig) **bool { return &l.Updates.Check }),
	}})
	out = append(out, struct {
		group    roleDef
		settings []settingDef
	}{roleDef{id: "notifications", title: "Notifications", about: "What happened that you'd want to know, kept as a feed: where each source of it is sent. Yours alone, as the feed is. What needs you is told apart from these, and always listed."}, []settingDef{
		notifySetting("missions", "Missions", "A mission ended by itself, or one was started for you by crew or the orchestrator.",
			func(l *config.LocalConfig) *string { return &l.Notifications.Missions }),
		notifySetting("pulls", "Pull requests", "A pull request this ws opened merged.",
			func(l *config.LocalConfig) *string { return &l.Notifications.Pulls }),
		notifySetting("plugins", "Plugins", "A plugin's update is out.",
			func(l *config.LocalConfig) *string { return &l.Notifications.Plugins }),
		notifySetting("ws", "ws", "ws was upgraded, and the daemon restarted onto it.",
			func(l *config.LocalConfig) *string { return &l.Notifications.WS }),
	}})
	out = append(out, struct {
		group    roleDef
		settings []settingDef
	}{roleDef{id: "ui", title: "Appearance", about: "How the web UI looks."}, []settingDef{
		accentSetting(),
		placesSetting("sidebar", "Sidebar", "The places in the sidebar, in order. A place in neither list goes at the sidebar's end, or the menu's for one whose home is the menu (Runs). Arrange them from Settings, under Appearance (Arrange the sidebar…). Yours, setting either list, replaces the team's arrangement."),
		placesSetting("menu", "In the menu", "The places moved out of the sidebar into the workspace's menu, in order. Arrange them from Settings, under Appearance (Arrange the sidebar…)."),
	}})
	return out
}

// notifySetting is where one source of notifications is sent
// ([notifications] <source>). It's yours alone: ws.toml has no
// [notifications].
func notifySetting(key, label, about string, local func(l *config.LocalConfig) *string) settingDef {
	return choiceSetting("notifications", key, label, about, config.NotifyFeed,
		[]SettingChoice{
			{Value: config.NotifyFeed, Label: "Feed"},
			{Value: config.NotifyDesktop, Label: "Feed and desktop"},
			{Value: config.NotifyOff, Label: "Off"},
		},
		func(*config.Config) *string { var none string; return &none }, local)
}

// routerEnabledSetting is [router] enabled: whether the daemon gives the
// workspace and its runs names. It's yours alone (ws.toml has no
// [router]), and the daemon reads it again every few seconds, so it needs
// no restart.
func routerEnabledSetting() settingDef {
	return settingDef{
		section: "router", key: "enabled", label: "Names for the web UI and runs", kind: "bool",
		about: "The web UI at http://<name>.localhost, and each run at <run>.<capsule>.<repo>.<name>.localhost, as well as at 127.0.0.1 and its port. Browsers send any .localhost to this machine by themselves.",
		team:  func(*config.Config) any { return nil },
		yours: func(l *config.LocalConfig) any {
			if l.Router.Enabled {
				return true
			}
			return nil
		},
		def: func(config.Config) any { return false },
		set: func(l *config.LocalConfig, v any) error {
			if v == nil {
				l.Router.Enabled = false
				return nil
			}
			b, ok := v.(bool)
			if !ok {
				return errors.New("it's on or off")
			}
			l.Router.Enabled = b
			return nil
		},
	}
}

// routerNameSetting is [router] name: the workspace's name under
// .localhost; unset, its folder's.
func routerNameSetting(root string) settingDef {
	return settingDef{
		section: "router", key: "name", label: "Name", kind: "text",
		about: "What the workspace's names end in: <name>.localhost. Where another workspace already has it, this one's gets four letters of its own after it while that one's daemon runs.",
		team:  func(*config.Config) any { return nil },
		yours: func(l *config.LocalConfig) any { return strOrNil(l.Router.Name) },
		def:   func(config.Config) any { return router.NameFor(root, "") },
		set: func(l *config.LocalConfig, v any) error {
			if v == nil {
				l.Router.Name = ""
				return nil
			}
			s, err := settingText(v, 63)
			if err != nil {
				return err
			}
			switch l := router.Label(s); {
			case l == "":
				return errors.New("lower case letters, numbers and dashes")
			case l != s:
				return fmt.Errorf("lower case letters, numbers and dashes only (%s would do)", l)
			}
			l.Router.Name = s
			return nil
		},
	}
}

// accentSetting is [ui] accent: the colour the web UI is drawn in. Its
// default is the web UI's own (in foundations.css, so the violet is written
// once), so it has none here.
func accentSetting() settingDef {
	get := func(s string) any {
		if a, err := config.ParseAccent(s); err == nil {
			return a
		}
		return strOrNil(s)
	}
	return settingDef{
		section: "ui", key: "accent", label: "Accent", kind: "color",
		about: "The colour the web UI is drawn in: buttons, links and selection, the wordmark and the glow behind each page. A hex colour (#rgb or #rrggbb). Unset, ws's own violet.",
		team:  func(c *config.Config) any { return get(c.UI.Accent) },
		yours: func(l *config.LocalConfig) any { return get(l.UI.Accent) },
		def:   func(config.Config) any { return nil },
		set: func(l *config.LocalConfig, v any) error {
			if v == nil {
				l.UI.Accent = ""
				return nil
			}
			s, ok := v.(string)
			if !ok {
				return errors.New("it's a hex colour, #rgb or #rrggbb")
			}
			a, err := config.ParseAccent(s)
			if err != nil {
				return err
			}
			l.UI.Accent = a
			return nil
		},
	}
}

// placesSetting is [ui] sidebar or [ui] menu: where the web UI's places
// go, as their ids in order. What a place's id is is the web UI's to say
// (a plugin's view's is its plugin's name and path), so any id is kept:
// one that isn't a place now is passed by. Clearing either clears both,
// as they're one arrangement (config.UIConfig), and the team's stands.
func placesSetting(key, label, about string) settingDef {
	field := func(u *config.UIConfig) *[]string {
		if key == "menu" {
			return &u.Menu
		}
		return &u.Sidebar
	}
	get := func(u config.UIConfig) any {
		if u.Sidebar == nil && u.Menu == nil {
			return nil
		}
		list := *field(&u)
		if list == nil {
			list = []string{}
		}
		return list
	}
	return settingDef{
		section: "ui", key: key, label: label, kind: "places", about: about,
		team:  func(c *config.Config) any { return get(c.UI) },
		yours: func(l *config.LocalConfig) any { return get(l.UI) },
		def:   func(config.Config) any { return nil },
		set: func(l *config.LocalConfig, v any) error {
			if v == nil {
				l.UI.Sidebar, l.UI.Menu = nil, nil
				return nil
			}
			list, ok := v.([]any)
			switch {
			case !ok:
				return errors.New("it's a list of places")
			case len(list) > 200:
				return errors.New("that's more than 200")
			}
			out := make([]string, 0, len(list))
			for _, x := range list {
				id, err := settingText(x, 200)
				if err != nil {
					return err
				}
				if slices.Contains(out, id) {
					return fmt.Errorf("%s is in it twice", id)
				}
				out = append(out, id)
			}
			if len(out) == 0 {
				out = nil
			}
			*field(&l.UI) = out
			return nil
		},
	}
}

// Accent is the web UI's accent as set (yours, else the team's), as
// #rrggbb, for the page to have before it first paints: "" for none, and
// the web UI's own default applies.
func (w *Workspace) Accent() string { return w.Config().UIAccent() }

// allowedSetting is [crew] allowed: the other harnesses and models crew
// may be launched on from a mission's page, as a list.
func allowedSetting() settingDef {
	get := func(list []string) any {
		if list == nil {
			return nil
		}
		return list
	}
	return settingDef{
		section: "crew", key: "allowed", label: "Also allowed", kind: "picks",
		about: "Other agents and models you may launch crew on from a mission's page, each harness:model (claude:sonnet), or an agent alone. Crew's own is always allowed. Yours replaces the team's list.",
		team:  func(c *config.Config) any { return get(c.Crew.Allowed) },
		yours: func(l *config.LocalConfig) any { return get(l.Crew.Allowed) },
		def:   func(config.Config) any { return nil },
		set: func(l *config.LocalConfig, v any) error {
			if v == nil {
				l.Crew.Allowed = nil
				return nil
			}
			list, ok := v.([]any)
			switch {
			case !ok:
				return errors.New("it's a list of harness:model")
			case len(list) == 0:
				return errors.New("give it an agent and model, or clear it")
			case len(list) > 20:
				return errors.New("that's more than 20")
			}
			out := make([]string, 0, len(list))
			for _, x := range list {
				s, err := settingText(x, 200)
				if err != nil {
					return err
				}
				h, m, err := config.ParsePick(s)
				if err != nil {
					return err
				}
				if names := harness.CrewNames(); !slices.Contains(names, h) {
					return fmt.Errorf("%s can't run crew: it's one of %s", h, strings.Join(names, ", "))
				}
				out = append(out, harness.Pick{Harness: h, Model: m}.String())
			}
			l.Crew.Allowed = out
			return nil
		},
	}
}

func maxDispatchesSetting() settingDef {
	get := func(n int) any {
		if n == 0 {
			return nil
		}
		return n
	}
	return settingDef{
		section: "orchestrator", key: "max_dispatches", label: "Crew in a row", kind: "number",
		about: "How many crew it may dispatch in a row without you stepping in, before it has to ask you instead.",
		team:  func(c *config.Config) any { return get(c.Orchestrator.MaxDispatches) },
		yours: func(l *config.LocalConfig) any { return get(l.Orchestrator.MaxDispatches) },
		def:   func(config.Config) any { return config.DefaultOrchestratorMaxDispatches },
		set: func(l *config.LocalConfig, v any) error {
			if v == nil {
				l.Orchestrator.MaxDispatches = 0
				return nil
			}
			f, ok := v.(float64)
			if !ok || f != float64(int(f)) || f < 1 || f > 50 {
				return errors.New("a whole number from 1 to 50 (to stop it dispatching by itself, turn it off, or have crew wait for you)")
			}
			l.Orchestrator.MaxDispatches = int(f)
			return nil
		},
	}
}

func maxRoundsSetting() settingDef {
	get := func(n int) any {
		if n == 0 {
			return nil
		}
		return n
	}
	return settingDef{
		section: "missions", key: "max_rounds", label: "Rounds before it's yours", kind: "number",
		about: "The most times a step's checks come round unmet before the step is yours, whatever the process says.",
		team:  func(c *config.Config) any { return get(c.Missions.MaxRounds) },
		yours: func(l *config.LocalConfig) any { return get(l.Missions.MaxRounds) },
		def:   func(config.Config) any { return config.DefaultMaxRounds },
		set: func(l *config.LocalConfig, v any) error {
			if v == nil {
				l.Missions.MaxRounds = 0
				return nil
			}
			f, ok := v.(float64)
			if !ok || f != float64(int(f)) || f < 1 || f > 20 {
				return errors.New("a whole number from 1 to 20")
			}
			l.Missions.MaxRounds = int(f)
			return nil
		},
	}
}

func windowSetting() settingDef {
	return settingDef{
		section: "team", key: "window", label: "How far back", kind: "text",
		about: "How far back the Team page's first look goes: days or weeks (14d, 2w).",
		team:  func(c *config.Config) any { return strOrNil(c.Team.Window) },
		yours: func(l *config.LocalConfig) any { return strOrNil(l.Team.Window) },
		def:   func(config.Config) any { return "14d" },
		set: func(l *config.LocalConfig, v any) error {
			if v == nil {
				l.Team.Window = ""
				return nil
			}
			s, err := settingText(v, 10)
			if err == nil {
				_, err = team.ParseWindow(s)
			}
			if err != nil {
				return err
			}
			l.Team.Window = s
			return nil
		},
	}
}

// Settings are your settings, each with where its value comes from, and
// for each role the agent it ends up with.
func (w *Workspace) Settings() (SettingsInfo, error) {
	root := w.Root()
	{
		cfg, _, err := config.Load(root)
		if err != nil {
			return SettingsInfo{}, err
		}
		team, local, err := config.Layers(root)
		if err != nil {
			return SettingsInfo{}, err
		}
		info := SettingsInfo{
			File: filepath.Join(root, config.LocalFileName), Team: filepath.Join(root, config.FileName),
			Unknown: cfg.Unknown,
		}
		for _, g := range settingDefs(root) {
			group := SettingsGroup{ID: g.group.id, Title: g.group.title, About: g.group.about}
			for _, d := range g.settings {
				s := Setting{Section: d.section, Key: d.key, Label: d.label, About: d.about, Kind: d.kind,
					Team: d.team(team), Yours: d.yours(&local), Default: d.def(*cfg)}
				if d.choices != nil {
					s.Choices = d.choices()
				}
				switch {
				case s.Yours != nil:
					s.Value, s.Source = s.Yours, "yours"
				case s.Team != nil:
					s.Value, s.Source = s.Team, "team"
				default:
					s.Value, s.Source = s.Default, "default"
				}
				if d.harness != nil {
					s.Harness = d.harness(cfg)
				}
				group.Settings = append(group.Settings, s)
			}
			if g.group.resolve != nil {
				group.Runs = roleRun(g.group, cfg)
			}
			info.Groups = append(info.Groups, group)
		}
		return info, nil
	}
}

// roleRun is what a role ends up running, and why it can't if it can't.
func roleRun(r roleDef, cfg *config.Config) *RoleRun {
	p, err := r.resolve(cfg)
	run := &RoleRun{Harness: p.Harness, Model: p.Model, Note: r.note(cfg)}
	switch {
	case err != nil:
		run.Problem = err.Error()
	default:
		if h, err := harness.Get(p.Harness); err != nil {
			run.Problem = err.Error()
		} else if err := h.Available(); err != nil {
			run.Problem = err.Error()
		}
	}
	return run
}

// SetSettings checks changes on what ws.local.toml says now, then writes
// them into it together, key by key: refused (ErrSettingRefused) for a key
// the page doesn't change, or a value it can't take. It answers the
// settings as they are then.
func (w *Workspace) SetSettings(changes []SettingChange) (SettingsInfo, error) {
	root := w.Root()
	{
		defs := map[string]settingDef{}
		for _, g := range settingDefs(root) {
			for _, d := range g.settings {
				defs[d.section+"."+d.key] = d
			}
		}
		apply := func(l *config.LocalConfig) error {
			for _, ch := range changes {
				d, ok := defs[ch.Section+"."+ch.Key]
				if !ok {
					return fmt.Errorf("%w: %s isn't a setting this page changes", ErrSettingRefused, settingName(ch.Section, ch.Key))
				}
				v := ch.Value
				if ch.Clear {
					v = nil
				} else if v == nil {
					return fmt.Errorf("%w: %s: give it a value, or clear it", ErrSettingRefused, settingName(ch.Section, ch.Key))
				}
				if err := d.set(l, v); err != nil {
					return fmt.Errorf("%w: %s: %v", ErrSettingRefused, settingName(ch.Section, ch.Key), err)
				}
			}
			return nil
		}
		// Checked first on a copy, as EditLocal's change can't fail.
		_, local, err := config.Layers(root)
		if err != nil {
			return SettingsInfo{}, err
		}
		if err := apply(&local); err != nil {
			return SettingsInfo{}, err
		}
		err = config.EditLocal(root, func(l *config.LocalConfig) { apply(l) }) //nolint:errcheck // checked above
		if errors.Is(err, config.ErrHandEdit) {
			return SettingsInfo{}, fmt.Errorf("%w: %v", ErrSettingRefused, err)
		}
		if err != nil {
			return SettingsInfo{}, err
		}
		return w.Settings()
	}
}

// settingName is a key as a person writes it: [section] key.
func settingName(section, key string) string {
	if section == "" {
		return key
	}
	return "[" + section + "] " + key
}

// modelsCache keeps what each harness's CLI said of its models a while:
// asking takes a second or so, and the page asks whenever a harness is
// picked.
var modelsCache = struct {
	sync.Mutex
	got map[string]modelsGot
}{got: map[string]modelsGot{}}

type modelsGot struct {
	at     time.Time
	models []harness.Model
	err    error
}

// Models are the models a harness offers, or none for one that can't say
// (the shell); what it says is kept for a while.
func (w *Workspace) Models(name string) ([]harness.Model, error) { return Models(name) }

// Models are the models the harness name offers.
func Models(name string) ([]harness.Model, error) {
	h, err := harness.Get(name)
	if err != nil {
		return nil, err
	}
	ml, ok := h.(harness.ModelLister)
	if !ok {
		return nil, nil
	}
	modelsCache.Lock()
	got, ok := modelsCache.got[name]
	modelsCache.Unlock()
	keep := 10 * time.Minute
	if got.err != nil {
		keep = 30 * time.Second
	}
	if ok && time.Since(got.at) < keep {
		return got.models, got.err
	}
	models, err := ml.Models(context.Background())
	modelsCache.Lock()
	modelsCache.got[name] = modelsGot{at: time.Now(), models: models, err: err}
	modelsCache.Unlock()
	return models, err
}
