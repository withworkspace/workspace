package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

const FileName = "ws.toml"
const LocalFileName = "ws.local.toml"

type Config struct {
	Workspace    WorkspaceConfig        `toml:"workspace"`
	Repos        map[string]RepoConfig  `toml:"repos"`
	Groups       map[string]GroupConfig `toml:"groups"`
	Crew         CrewConfig             `toml:"crew"`
	Briefing     AgentConfig            `toml:"briefing"`
	Orchestrator OrchestratorConfig     `toml:"orchestrator"`
	// Conversation is the agent that answers threads on documents.
	Conversation AgentConfig `toml:"conversation"`
	// Judge is the agent that judges a Work step's checks in words, with
	// fresh context: unset, the planner's (briefing's) agent and model.
	Judge    AgentConfig    `toml:"judge"`
	Missions MissionsConfig `toml:"missions"`
	Agents   AgentsConfig   `toml:"agents"`
	Pages    PagesConfig    `toml:"pages"`
	Reviews  ReviewsConfig  `toml:"reviews"`
	// Documents are the kinds of document the workspace has, by name:
	// [documents.bp]. The team's: ws.toml only.
	Documents map[string]DocumentKind `toml:"documents"`
	// Artifacts is where artifacts are kept, and the commits that keep
	// them ([artifacts]). The team's, but for its path, which ws.local.toml
	// can say too (Artifacts.Local).
	Artifacts ArtifactsConfig `toml:"artifacts"`
	Web       WebConfig       `toml:"web"`
	UI        UIConfig        `toml:"ui"`
	Team      TeamConfig      `toml:"team"`
	// Plugins are the plugins the team uses, by name ([plugins]): where
	// each comes from. ws doctor says which aren't installed here.
	Plugins map[string]PluginSource `toml:"plugins"`
	// Plugin is each plugin's config, by its name then the key
	// ([plugin.jira]): the team's in ws.toml, yours over it in
	// ws.local.toml. What the keys are is the plugin's to say.
	Plugin  map[string]map[string]any `toml:"plugin"`
	GitHub  GitHubConfig              `toml:"-"` // from ws.local.toml only
	Boarded map[string][]string       `toml:"-"` // from ws.local.toml [boarded] section
	Git     string                    `toml:"-"` // from ws.local.toml only
	Silo    map[string]string         `toml:"-"` // from ws.local.toml [silo] section
	// Unknown is what ws.toml and ws.local.toml say that ws doesn't read: a
	// key that's misspelt (alias for aliases), or in the wrong file. Each is
	// "ws.toml: repos.api.alias". ws doctor lists them.
	Unknown []string `toml:"-"`
	// PluginOrder is every key under [plugin] in the order the files have
	// them, ws.toml's then ws.local.toml's, each as its path below plugin
	// (jira, queries, bugs, label): a plugin's rows are given to it in
	// the order they were written, which a map doesn't keep.
	PluginOrder [][]string `toml:"-"`
}

type LocalConfig struct {
	Git          string                 `toml:"git"`
	Repos        map[string]RepoConfig  `toml:"repos"`
	Groups       map[string]GroupConfig `toml:"groups,omitempty"`
	Boarded      map[string][]string    `toml:"boarded"`
	Silo         map[string]string      `toml:"silo"`
	Crew         CrewConfig             `toml:"crew,omitempty"`
	Briefing     AgentConfig            `toml:"briefing,omitempty"`
	Orchestrator OrchestratorConfig     `toml:"orchestrator,omitempty"`
	Conversation AgentConfig            `toml:"conversation,omitempty"`
	Judge        AgentConfig            `toml:"judge,omitempty"`
	Missions     MissionsConfig         `toml:"missions,omitempty"`
	Artifacts    LocalArtifactsConfig   `toml:"artifacts,omitempty"`
	Agents       AgentsConfig           `toml:"agents,omitempty"`
	Web          WebConfig              `toml:"web,omitempty"`
	UI           UIConfig               `toml:"ui,omitempty"`
	Team         TeamConfig             `toml:"team,omitempty"`
	Reviews      ReviewsConfig          `toml:"reviews,omitempty"`
	GitHub       GitHubConfig           `toml:"github,omitempty"`
	// Plugin is your config for each plugin, over the team's.
	Plugin map[string]map[string]any `toml:"plugin,omitempty"`
}

// PluginSource is where a plugin the team uses comes from.
type PluginSource struct {
	Source  string `toml:"source"`
	Version string `toml:"version,omitempty"`
}

// AgentsConfig is the workspace's own way of working, for every agent ws
// starts: its guide, the folder its skills are in, and its process. Paths
// are relative to the workspace (or absolute, or from ~). Unset, the guide
// is AGENTS.md (or CLAUDE.md) at the workspace's root, the skills
// .agents/skills (or .claude/skills), where they're there, and the process
// PROCESS.md at the root.
type AgentsConfig struct {
	Guide  string `toml:"guide,omitempty"`
	Skills string `toml:"skills,omitempty"`
	// Process is how work goes here, in prose: the steps work takes, whose
	// turn each is, and when it's the user's. The planner and the
	// orchestrator read it as it's written.
	Process string `toml:"process,omitempty"`
}

// DocumentKind is a kind of document the workspace has: where one goes and
// what it's called, the commits that add and change one, and the file that
// says how one's written.
type DocumentKind struct {
	// Title is what one's called (Build proposal). Default: the kind's name.
	Title string `toml:"title,omitempty"`
	// Path is where one goes, as repo:path, with {date} (the day it's
	// started, 2026-11-11), {slug} (from its title) and {number} (one above
	// the highest there, to four digits) where it varies:
	// "records:bp/proposed/{date}-{slug}.md". A document is of the kind when
	// its path fits.
	Path string `toml:"path"`
	// Add and Amend are the first line of the commit that adds one, and of
	// one that changes one, with {date}, {slug}, {number} and {title} (its
	// first heading) filled in.
	Add   string `toml:"add,omitempty"`
	Amend string `toml:"amend,omitempty"`
	// Guide is a file saying how one's written (a skill's SKILL.md, a
	// template): every agent that writes into or about one is told to read
	// it first. Relative to the workspace, or repo:path for a file on a
	// repo's default branch.
	Guide string `toml:"guide,omitempty"`
	// Passes are the workspace's own passes ([pages.passes.<name>]) that
	// are this kind's: they're tabs on a page started as one, and on no
	// other page. The built-in passes are on every page.
	Passes []string `toml:"passes,omitempty"`
	// Moves are the steps in one's life ([documents.<name>.moves.<move>]):
	// it goes to another folder, in a commit of its own. Where one is,
	// its path's folder or a move's, is where it stands.
	Moves map[string]DocumentMove `toml:"moves,omitempty"`
}

// ArtifactsConfig is [artifacts] in ws.toml: where artifacts (versioned
// HTML prototypes) are kept, and the first lines of the commits that keep
// them in a repo.
type ArtifactsConfig struct {
	// Path is where new ones go: a folder of a repo, as a kind's path is
	// a file of one, with {date} and {slug} (or {number}) in its last
	// part, ending in /: "records:artifacts/{date}-{slug}/". None, or
	// ".ws": .ws/artifacts/{date}-{slug}/, on this machine.
	Path string `toml:"path,omitempty"`
	// Add, Revise, Promote, Discard, Restore and Clear are the first
	// lines of the commits that keep them in a repo, with {title},
	// {slug}, {date}, {number} and {version} filled in. None: ws's own.
	Add     string `toml:"add,omitempty"`
	Revise  string `toml:"revise,omitempty"`
	Promote string `toml:"promote,omitempty"`
	Discard string `toml:"discard,omitempty"`
	Restore string `toml:"restore,omitempty"`
	Clear   string `toml:"clear,omitempty"`
	// ExpireAfter is how long a live one lasts after its last version: a
	// number of days ("30d"), or "never".
	ExpireAfter string `toml:"expire_after,omitempty"`
	// Guide is a file that says how one's written, shown to a
	// conversation's agent before it proposes one in place of ws's own
	// note: relative to the workspace, or repo:path.
	Guide string `toml:"guide,omitempty"`
	// Local is ws.local.toml's path, if it says one: where your new ones
	// go, whatever the team's says (".ws": on this machine).
	Local string `toml:"-"`
}

// LocalArtifactsConfig is [artifacts] in ws.local.toml: where your new
// artifacts go (".ws" keeps them on this machine).
type LocalArtifactsConfig struct {
	Path string `toml:"path,omitempty"`
}

// DocumentMove is a step in a kind of document's life: where one goes, the
// first line of the commit that moves it, and what that commit's body
// asks (who approved it).
type DocumentMove struct {
	To     string `toml:"to"`
	Commit string `toml:"commit,omitempty"`
	Asks   string `toml:"asks,omitempty"`
}

// PagesConfig is for pages in the web UI.
type PagesConfig struct {
	// Style is a file saying how what agents write into pages should read
	// (a style guide, a skill's instructions): the agents filling cells are
	// told to read it before they write. A path, relative to the workspace
	// (or absolute, or from ~).
	Style string `toml:"style,omitempty"`
	// Passes are the workspace's own passes over a page, by name (a word),
	// beside the built-in ones (check, formal, cold): [pages.passes.security].
	Passes map[string]PassConfig `toml:"passes,omitempty"`
}

// PassConfig is a pass over a page: what it's called, what it's for in a
// line, what its agent is to look for, and the kinds its findings can be.
// File is a file that says how to do it (a checklist), which its agent
// reads first: with one, Looks may be left out. Skill is one of the
// workspace's skills, by name, and Args what's asked of it ("ready to
// share?"): its agent follows the skill for that, and reports in ws's form
// (findings on the page's words) in place of the skill's own. Cold gives
// its agent the page and nothing else.
type PassConfig struct {
	Title string   `toml:"title,omitempty"`
	About string   `toml:"about,omitempty"`
	Looks string   `toml:"looks,omitempty"`
	File  string   `toml:"file,omitempty"`
	Skill string   `toml:"skill,omitempty"`
	Args  string   `toml:"args,omitempty"`
	Kinds []string `toml:"kinds,omitempty"`
	Cold  bool     `toml:"cold,omitempty"`
}

// ReviewsConfig is for reviews in the web UI: a pull request's, or a
// capsule's.
type ReviewsConfig struct {
	// Passes are the workspace's passes over a review, by name (a word):
	// [reviews.passes.security]. ws.local.toml's are added to ws.toml's,
	// and one of the same name replaces it: a pass can be the team's, or
	// your own.
	Passes map[string]ReviewPassConfig `toml:"passes,omitempty"`
	// Guide is how the workspace wants to be walked through a change:
	// [reviews.guide]. Yours, in ws.local.toml, replaces the team's.
	Guide ReviewGuideConfig `toml:"guide,omitempty"`
}

// ReviewGuideConfig is for a review's guide, the pass that says what a
// change does and what order to read it in: what it should attend to
// (Looks), or a file that says (File).
type ReviewGuideConfig struct {
	Looks string `toml:"looks,omitempty"`
	File  string `toml:"file,omitempty"`
	// Auto has it written, without being asked for, for a pull request
	// that needs you.
	Auto bool `toml:"auto,omitempty"`
}

// ReviewPassConfig is a pass over a review: what a page's pass has (what
// it's called and for, what its agent looks for, or the file or skill
// that says, and the kinds its findings can be), and what a review's adds.
// Blocks are the kinds that make it no go while a finding of one is open
// ("blocking", with none given). Cold gives its agent the diff and the
// description and nothing else: nothing's checked out for it. Repos are
// the repos whose reviews have it: all of them, with none given. Auto has
// it run, without being asked for, over a pull request that needs you.
type ReviewPassConfig struct {
	Title  string   `toml:"title,omitempty"`
	About  string   `toml:"about,omitempty"`
	Looks  string   `toml:"looks,omitempty"`
	File   string   `toml:"file,omitempty"`
	Skill  string   `toml:"skill,omitempty"`
	Args   string   `toml:"args,omitempty"`
	Kinds  []string `toml:"kinds,omitempty"`
	Blocks []string `toml:"blocks,omitempty"`
	Cold   bool     `toml:"cold,omitempty"`
	Auto   bool     `toml:"auto,omitempty"`
	Repos  []string `toml:"repos,omitempty"`
}

// TeamConfig is for the Team page: what the people working in the
// workspace's repos have been doing. ws.local.toml's settings override
// ws.toml's, person by person.
type TeamConfig struct {
	// Window is how far back the first look goes: a number of days or
	// weeks (14d, 2w). Default 14d.
	Window string `toml:"window,omitempty"`
	// People says who someone is where ws can't work it out: the logins
	// and emails that are one person, and their name. [team.people.ana].
	People map[string]TeamPerson `toml:"people,omitempty"`
	// Narrative is whether an agent writes a gist of each change, and
	// what's worth knowing of it (default true). Harness and Model pick
	// the agent: by default the one conversations run on, on its small
	// model.
	Narrative *bool  `toml:"narrative,omitempty"`
	Harness   string `toml:"harness,omitempty"`
	Model     string `toml:"model,omitempty"`
	// Watch are the workspace's own paths whose changes are changes to how
	// things are done, by name: [team.watch.patterns]. They're marked on
	// the Team page beside the built-in ones (guides, skills, CI, lint).
	Watch map[string]TeamWatch `toml:"watch,omitempty"`
}

// TeamWatch is one of [team.watch]: what a change that touches it is
// marked with, and the paths (globs; "repo:" before one for one repo's).
// One named as a built-in replaces it, and with no paths turns it off.
type TeamWatch struct {
	Title string   `toml:"title,omitempty"`
	Paths []string `toml:"paths,omitempty"`
}

// TeamNarrative reports whether gists are written.
func (c *Config) TeamNarrative() bool {
	return c.Team.Narrative == nil || *c.Team.Narrative
}

// TeamPerson is one of [team.people].
type TeamPerson struct {
	Name   string   `toml:"name,omitempty"`
	Logins []string `toml:"logins,omitempty"`
	Emails []string `toml:"emails,omitempty"`
}

// GitHubConfig is for trying ws where there's no GitHub.
type GitHubConfig struct {
	// Fixture is a JSON file that answers for GitHub (github.Fixture): the
	// PRs, stacks and reviews ws would otherwise ask gh for. A path,
	// relative to the workspace. The test workspace sets it.
	Fixture string `toml:"fixture,omitempty"`
}

// WebConfig is the web UI, which the workspace's daemon serves.
type WebConfig struct {
	// Port is where it listens on 127.0.0.1. 0 picks one from the
	// workspace's path, so each workspace keeps its own.
	Port int `toml:"port,omitempty"`
}

// UIConfig is how the web UI looks: the team's in ws.toml, yours in
// ws.local.toml, yours winning.
type UIConfig struct {
	// Accent is the colour the web UI is drawn in, from which everything
	// accent-coloured is derived: a hex colour, #rgb or #rrggbb. Unset,
	// the web UI's own default (in its foundations.css). See
	// workspace-records:docs/customisable-ui-accent.md.
	Accent string `toml:"accent,omitempty"`
	// Sidebar and Menu are where the web UI's places go (Missions,
	// Conversations, a plugin's views…), each a list of their ids in
	// order: Sidebar is the sidebar's, Menu the workspace menu's (from the
	// workspace's name). A place in neither goes at the sidebar's end, as
	// it comes; an id that isn't a place now is passed by. The two are one
	// arrangement: yours, setting either, replaces the team's.
	Sidebar []string `toml:"sidebar,omitempty"`
	Menu    []string `toml:"menu,omitempty"`
}

// ParseAccent is s as an accent: a hex colour, #rgb or #rrggbb, given as
// #rrggbb in lowercase. Anything else is refused, as it's put into the
// web UI's CSS.
func ParseAccent(s string) (string, error) {
	s = strings.TrimSpace(s)
	hex := strings.TrimPrefix(s, "#")
	ok := len(s) == len(hex)+1 && (len(hex) == 3 || len(hex) == 6)
	for _, r := range hex {
		ok = ok && strings.ContainsRune("0123456789abcdefABCDEF", r)
	}
	if !ok {
		return "", fmt.Errorf("%q isn't a hex colour: give it as #rgb or #rrggbb", s)
	}
	hex = strings.ToLower(hex)
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	return "#" + hex, nil
}

// UIAccent is the web UI's accent as set (yours, else the team's), as
// #rrggbb: "" where neither sets one, or what's set isn't a hex colour,
// for the web UI's own default.
func (c *Config) UIAccent() string {
	a, err := ParseAccent(c.UI.Accent)
	if err != nil {
		return ""
	}
	return a
}

// MissionsConfig holds workspace-wide settings for missions.
type MissionsConfig struct {
	// Records names the repo where shared proposals, ADRs and notes go.
	// Crew and briefing agents are told to lift a capsule there when a
	// stage produces one.
	Records string `toml:"records,omitempty"`
	// ReviewBeforePR and ReviewBeforeLand are guarantees: rules ws holds
	// for every mission, where it acts, whatever an agent reads. With
	// ReviewBeforePR, ws opens a pull request from a mission's capsule
	// only once your latest review of it approves the commit it's at; with
	// ReviewBeforeLand, ws lands a capsule of a repo with no pull requests
	// only once your latest review approves it as it is. Both off unless
	// set. What the process prefers is its own, applied by the planner and
	// the orchestrator.
	ReviewBeforePR   *bool `toml:"review_before_pr,omitempty"`
	ReviewBeforeLand *bool `toml:"review_before_land,omitempty"`
	// ReviewBeforeReady holds a mission's draft pull request a draft until
	// your latest review approves the commit it's at: ws marks it ready
	// only then. Off unless set.
	ReviewBeforeReady *bool `toml:"review_before_ready,omitempty"`
	// RepliesNeedMe holds a reply drafted to a person on a mission's pull
	// request until you've approved it: ws posts it, as you, only then.
	// A reply to a bot needn't wait. On unless set off.
	RepliesNeedMe *bool `toml:"replies_need_me,omitempty"`
	// EndWhenMerged has ws end a mission itself once the orchestrator
	// suggests it landed (every part of Z met, its work merged or landed)
	// and nothing else on it waits for you or crew: merging was yours, and
	// a mission with code is finished when it has merged. On unless set
	// off; off, the ending suggested waits for you to end it.
	EndWhenMerged *bool `toml:"end_when_merged,omitempty"`
	// MaxRounds is a guarantee too: the most times a Work step's checks
	// come round unmet before the step is yours, whatever the process
	// says. 0 is the default (DefaultMaxRounds).
	MaxRounds int `toml:"max_rounds,omitempty"`
}

// DefaultMaxRounds is how many times a step's checks may come round unmet
// before it's yours, unless configured.
const DefaultMaxRounds = 3

// MaxRounds is the most times a step's checks may come round unmet before
// it's yours.
func (c *Config) MaxRounds() int {
	if c.Missions.MaxRounds > 0 {
		return c.Missions.MaxRounds
	}
	return DefaultMaxRounds
}

// ReviewBeforePR reports whether ws holds your review before it opens a
// mission's pull request.
func (c *Config) ReviewBeforePR() bool {
	return c.Missions.ReviewBeforePR != nil && *c.Missions.ReviewBeforePR
}

// ReviewBeforeReady reports whether ws holds your review before it marks
// a mission's draft pull request ready.
func (c *Config) ReviewBeforeReady() bool {
	return c.Missions.ReviewBeforeReady != nil && *c.Missions.ReviewBeforeReady
}

// RepliesNeedMe reports whether a reply to a person on a mission's pull
// request waits for you (on unless set off).
func (c *Config) RepliesNeedMe() bool {
	return c.Missions.RepliesNeedMe == nil || *c.Missions.RepliesNeedMe
}

// EndWhenMerged reports whether ws ends a mission itself once it's
// suggested landed and nothing else on it waits (on unless set off).
func (c *Config) EndWhenMerged() bool {
	return c.Missions.EndWhenMerged == nil || *c.Missions.EndWhenMerged
}

// ReviewBeforeLand reports whether ws holds your review before it lands
// a mission's capsule in a repo with no pull requests.
func (c *Config) ReviewBeforeLand() bool {
	return c.Missions.ReviewBeforeLand != nil && *c.Missions.ReviewBeforeLand
}

// AgentConfig says which harness (agent CLI: cursor, claude, pi, shell) runs a
// role, [crew] or [briefing], and on which model. Empty means the default:
// see harness.Roles. ws.local.toml settings override ws.toml's.
type AgentConfig struct {
	Harness string `toml:"harness,omitempty"`
	// Model is passed to the harness as is, so it's in that agent's own
	// naming ("opus" for claude, "claude-opus-5-5-high" for cursor,
	// "anthropic/claude-opus-5-5" or "sonnet:high" for pi).
	Model string `toml:"model,omitempty"`
	// In is where [briefing] happens: "web" (the default), a conversation
	// in the web UI whose goal is the brief, or "terminal", a briefing
	// agent in a terminal, as before conversations.
	In string `toml:"in,omitempty"`
}

// merge overrides c with whatever local sets.
func (c *AgentConfig) merge(local AgentConfig) {
	if local.Harness != "" {
		c.Harness = local.Harness
	}
	if local.In != "" {
		c.In = local.In
	}
	if local.Model != "" {
		c.Model = local.Model
	}
}

// CrewConfig is [crew]: the harness and model crew run on, and the others
// you may launch them on instead from a mission's page.
type CrewConfig struct {
	AgentConfig
	// Allowed are the other harnesses and models crew may be launched on
	// from a mission's page, each "harness:model" (claude:opus) or a
	// harness alone (on the model [crew] gives it, if it's crew's harness,
	// else the agent's own). [crew]'s own harness and model are always
	// allowed: they're the default. Empty, crew run on the default only.
	// ws.local.toml's replaces ws.toml's whole, when it sets one.
	Allowed []string `toml:"allowed,omitempty"`
	// TrustFolders, read from ws.local.toml alone, has the host answer
	// Claude Code's trust dialog yes for crew (crew.WatchTrust): for a
	// throwaway workspace, where every repo is new to Claude and nobody's
	// there to answer. Trusting a folder lets its agent run what's in it.
	TrustFolders bool `toml:"trust_folders,omitempty"`
}

// merge overrides c with whatever local sets: allowed is replaced whole.
func (c *CrewConfig) merge(local CrewConfig) {
	c.AgentConfig.merge(local.AgentConfig)
	if local.Allowed != nil {
		c.Allowed = local.Allowed
	}
}

// ParsePick splits a [crew] allowed entry into its harness and model:
// "claude:opus", or "claude" alone for no model. The model is what follows
// the first colon, so it can have colons of its own (pi's "sonnet:high").
func ParsePick(s string) (harness, model string, err error) {
	harness, model, _ = strings.Cut(strings.TrimSpace(s), ":")
	harness, model = strings.TrimSpace(harness), strings.TrimSpace(model)
	if harness == "" || strings.ContainsAny(harness, " \t") {
		return "", "", fmt.Errorf("%q isn't a harness and model: write it harness:model (claude:opus), or a harness alone", s)
	}
	return harness, model, nil
}

// OrchestratorConfig controls the orchestrator: a headless agent run when
// crew report done or end, which reads the brief and what happened and
// decides what's next (dispatch crew, flag the user, conclude…).
// ws.local.toml settings override ws.toml's.
type OrchestratorConfig struct {
	// Harness and Model pick the agent that runs it; empty means the
	// crew's harness (if it can run headless) and that harness's own
	// orchestrator model. See harness.Roles.
	Harness string `toml:"harness,omitempty"`
	Model   string `toml:"model,omitempty"`
	Enabled *bool  `toml:"enabled,omitempty"`
	// Dispatch is "auto" (launch dispatched crew at once, the default) or
	// "ask" (queue them for the user to launch from the mission's page).
	Dispatch string `toml:"dispatch,omitempty"`
	// MaxDispatches is how many crew it may dispatch in a row without the
	// user stepping in, before it has to flag them instead.
	MaxDispatches int `toml:"max_dispatches,omitempty"`
}

// DefaultOrchestratorMaxDispatches is how many crew the orchestrator may
// dispatch in a row without the user, unless configured.
const DefaultOrchestratorMaxDispatches = 3

// OrchestratorEnabled reports whether the orchestrator runs automatically
// (default on).
func (c *Config) OrchestratorEnabled() bool {
	return c.Orchestrator.Enabled == nil || *c.Orchestrator.Enabled
}

// BriefInTerminal reports whether missions are briefed by an agent in a
// terminal ([briefing] in = "terminal") rather than in a conversation in
// the web UI.
func (c *Config) BriefInTerminal() bool { return c.Briefing.In == "terminal" }

// OrchestratorAutoDispatch reports whether dispatched crew launch at once
// (default) rather than waiting for the user.
func (c *Config) OrchestratorAutoDispatch() bool { return c.Orchestrator.Dispatch != "ask" }

// OrchestratorMaxDispatches is the limit on dispatches without the user.
func (c *Config) OrchestratorMaxDispatches() int {
	if c.Orchestrator.MaxDispatches > 0 {
		return c.Orchestrator.MaxDispatches
	}
	return DefaultOrchestratorMaxDispatches
}

// GroupConfig controls a repo group. Hidden groups are left out of commands
// that cover every repo unless asked for with --group. Hidden is a pointer so
// ws.local.toml can tell "not set" apart from an explicit false.
type GroupConfig struct {
	Hidden *bool `toml:"hidden,omitempty"`
}

const RepoFileName = "ws.repo.toml"

type SiloRepoConfig struct {
	AfterSwitch string `toml:"after_switch"`
	AfterChange string `toml:"after_change"`
}

type RepoFileConfig struct {
	Capsule CapsuleConfig  `toml:"capsule"`
	Silo    SiloRepoConfig `toml:"silo"`
	// Run is how the repo is run, by name ([run.web]): what `ws run web`
	// starts in the worktree you stand in.
	Run map[string]RunRecipe `toml:"run,omitempty"`
	// Landing and Verify are the repo's own account of how work lands in
	// it (see RepoConfig): a workspace's ws.toml or ws.local.toml says
	// otherwise for itself.
	Landing string `toml:"landing,omitempty"`
	Verify  string `toml:"verify,omitempty"`
	// Documents is the repo's own word on whether it's a space of
	// documents (see RepoConfig.Documents).
	Documents *Documents `toml:"documents,omitempty"`
	// Unknown is what the file says that ws doesn't read (see Config.Unknown).
	Unknown []string `toml:"-"`
}

// RunRecipe is one of a repo's [run.<name>]: something ws run starts
// (internal/runs).
type RunRecipe struct {
	// Command is what's run, in a shell: {port} is the run's port, which
	// $PORT holds too.
	Command string `toml:"command"`
	// Ready is a path asked for over HTTP until it answers anything below
	// 500: then the run's up. "" is up once something listens on its port.
	Ready string `toml:"ready,omitempty"`
	// Stop is run in the same place once the command's process has been
	// ended (docker compose down): "" for nothing.
	Stop string `toml:"stop,omitempty"`
}

type CapsuleConfig struct {
	CopyFromGround []string `toml:"copy_from_ground"`
	AfterCreate    string   `toml:"after_create"`
}

// ParseRepoConfig parses a ws.repo.toml file at the given path.
// Returns nil, nil if the file does not exist. A file is decoded again only
// once it's changed: the web UI asks after every repo's every few seconds.
func ParseRepoConfig(path string) (*RepoFileConfig, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	repoFilesMu.Lock()
	kept, ok := repoFiles[path]
	repoFilesMu.Unlock()
	if ok && kept.mod.Equal(info.ModTime()) && kept.size == info.Size() {
		c := *kept.cfg
		return &c, nil
	}
	var cfg RepoFileConfig
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	cfg.Unknown = unknownKeys("ws.repo.toml", md)
	repoFilesMu.Lock()
	repoFiles[path] = keptRepoFile{mod: info.ModTime(), size: info.Size(), cfg: &cfg}
	repoFilesMu.Unlock()
	c := cfg
	return &c, nil
}

// keptRepoFile is a ws.repo.toml as last decoded, and the file it was.
type keptRepoFile struct {
	mod  time.Time
	size int64
	cfg  *RepoFileConfig
}

var (
	repoFilesMu sync.Mutex
	repoFiles   = map[string]keptRepoFile{}
)

// ReadLocal is what ws.local.toml says, apart from ws.toml: empty where
// there's no such file.
func ReadLocal(root string) (LocalConfig, error) {
	var local LocalConfig
	if _, err := toml.DecodeFile(filepath.Join(root, LocalFileName), &local); err != nil && !os.IsNotExist(err) {
		return LocalConfig{}, fmt.Errorf("parsing %s: %w", LocalFileName, err)
	}
	return local, nil
}

// unknownKeys is what a decoded file said that nothing read, each as
// "file: a.b.c". A table that's unknown is named once, not key by key.
// Nothing under [plugin] is: what a plugin's keys are is the plugin's to
// say, and a table nested in one ([plugin.jira.queries.bugs]) is held as
// it's written, which the decoder counts as unread.
func unknownKeys(file string, md toml.MetaData) []string {
	var keys []string
	var tables []string
	for _, k := range md.Undecoded() {
		if len(k) > 1 && k[0] == "plugin" {
			continue
		}
		name := k.String()
		if slices.ContainsFunc(tables, func(t string) bool { return strings.HasPrefix(name, t+".") }) {
			continue
		}
		if md.Type(k...) == "Hash" || md.Type(k...) == "ArrayHash" {
			tables = append(tables, name)
		}
		keys = append(keys, file+": "+name)
	}
	return keys
}

type WorkspaceConfig struct {
	Org           string `toml:"org"`
	DefaultBranch string `toml:"default_branch"`
	DisplayName   string `toml:"display_name"`
}

type RepoConfig struct {
	DisplayName string   `toml:"display_name"`
	Aliases     []string `toml:"aliases"`
	Color       string   `toml:"color"`
	AfterCreate string   `toml:"after_create"`
	Group       string   `toml:"group,omitempty"`
	// Landing is how work lands in the repo: "pr" (the default), through a
	// pull request, or "direct", commits straight to the default branch
	// with ws land.
	Landing string `toml:"landing,omitempty"`
	// Verify is a shell command that must pass in a capsule before ws land
	// pushes it (a direct repo's lint or tests). None: nothing's run.
	Verify string `toml:"verify,omitempty"`
	// Documents says the repo is a space of documents, read in the web
	// UI's Documents rather than Files (true), or a folder of it is
	// ("docs/"), or it isn't (false: for the records repo, which is one
	// without saying). Unset, the repo's own ws.repo.toml says.
	Documents *Documents `toml:"documents,omitempty"`
}

// Documents is a repo's documents setting: whether it's a space of
// documents, and if a folder of it is, which.
type Documents struct {
	On     bool
	Folder string // "" for the whole repo; else a folder, ending in /
}

// UnmarshalTOML reads documents = true, false, or "a/folder/".
func (d *Documents) UnmarshalTOML(v any) error {
	switch x := v.(type) {
	case bool:
		*d = Documents{On: x}
		return nil
	case string:
		folder := strings.Trim(strings.TrimPrefix(strings.TrimSpace(x), "./"), "/")
		if folder == "" {
			*d = Documents{On: true}
			return nil
		}
		if slices.Contains(strings.Split(folder, "/"), "..") {
			return fmt.Errorf("documents = %q leads out of the repo", x)
		}
		*d = Documents{On: true, Folder: folder + "/"}
		return nil
	}
	return fmt.Errorf("documents is true, false or a folder (\"docs/\"), not %v", v)
}

// MarshalTOML writes it as it's read.
func (d Documents) MarshalTOML() ([]byte, error) {
	switch {
	case d.Folder != "":
		return []byte(strconv.Quote(d.Folder)), nil
	case d.On:
		return []byte("true"), nil
	}
	return []byte("false"), nil
}

// Landings a repo can have.
const (
	LandingPR     = "pr"
	LandingDirect = "direct"
)

// ValidLanding reports whether s is a landing ("" is the default, pr).
func ValidLanding(s string) bool {
	return s == "" || s == LandingPR || s == LandingDirect
}

func Parse(path string) (*Config, error) {
	var cfg Config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	cfg.Unknown = unknownKeys(filepath.Base(path), md)
	cfg.PluginOrder = pluginOrder(md)
	return &cfg, nil
}

// pluginOrder is the keys a file has under [plugin], in the order it has
// them, each as its path below plugin.
func pluginOrder(md toml.MetaData) [][]string {
	var out [][]string
	for _, k := range md.Keys() {
		if len(k) > 1 && k[0] == "plugin" {
			out = append(out, slices.Clone([]string(k[1:])))
		}
	}
	return out
}

func Discover(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, FileName)
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s not found in any parent directory", FileName)
		}
		dir = parent
	}
}

// Merge returns a new Config with local overrides applied on top of base.
// Only repos that exist in base are considered; unknown repos in local are skipped.
// Aliases are appended; AfterCreate, Color, DisplayName, and Group replace if non-empty.
// Group visibility set locally replaces the base setting; local groups may be
// new, so a repo can be moved into a group the team config never declared.
func Merge(base, local *Config) *Config {
	merged := &Config{
		Workspace:    base.Workspace,
		Crew:         base.Crew,
		Briefing:     base.Briefing,
		Orchestrator: base.Orchestrator,
		Conversation: base.Conversation,
		Judge:        base.Judge,
		Missions:     base.Missions,
		Agents:       base.Agents,
		Pages:        base.Pages,
		Reviews:      mergeReviews(base.Reviews, local.Reviews),
		Documents:    base.Documents,
		Artifacts:    base.Artifacts,
		Web:          base.Web,
		UI:           base.UI,
		Team:         base.Team,
		Plugins:      base.Plugins,
		Plugin:       mergePlugin(base.Plugin, local.Plugin),
		Repos:        make(map[string]RepoConfig, len(base.Repos)),
		Groups:       make(map[string]GroupConfig, len(base.Groups)),
	}
	maps.Copy(merged.Repos, base.Repos)
	maps.Copy(merged.Groups, base.Groups)
	for name, localGroup := range local.Groups {
		if localGroup.Hidden != nil {
			merged.Groups[name] = localGroup
		}
	}
	for name, localRepo := range local.Repos {
		baseRepo, ok := merged.Repos[name]
		if !ok {
			continue
		}
		if localRepo.AfterCreate != "" {
			baseRepo.AfterCreate = localRepo.AfterCreate
		}
		if localRepo.Color != "" {
			baseRepo.Color = localRepo.Color
		}
		if localRepo.DisplayName != "" {
			baseRepo.DisplayName = localRepo.DisplayName
		}
		if localRepo.Group != "" {
			baseRepo.Group = localRepo.Group
		}
		if localRepo.Landing != "" {
			baseRepo.Landing = localRepo.Landing
		}
		if localRepo.Verify != "" {
			baseRepo.Verify = localRepo.Verify
		}
		if localRepo.Documents != nil {
			baseRepo.Documents = localRepo.Documents
		}
		if len(localRepo.Aliases) > 0 {
			baseRepo.Aliases = append(baseRepo.Aliases, localRepo.Aliases...)
		}
		merged.Repos[name] = baseRepo
	}
	return merged
}

func Load(startDir string) (*Config, string, error) {
	root, err := Discover(startDir)
	if err != nil {
		return nil, "", err
	}
	cfg, err := Parse(filepath.Join(root, FileName))
	if err != nil {
		return nil, "", err
	}

	localPath := filepath.Join(root, LocalFileName)
	if _, err := os.Stat(localPath); err == nil {
		var local LocalConfig
		md, err := toml.DecodeFile(localPath, &local)
		if err != nil {
			return nil, "", fmt.Errorf("parsing %s: %w", LocalFileName, err)
		}
		unknown := append(cfg.Unknown, unknownKeys(LocalFileName, md)...)

		// Merge repo overrides using existing Merge function
		localCfg := &Config{Repos: local.Repos, Groups: local.Groups, Reviews: local.Reviews, Plugin: local.Plugin}
		order := append(cfg.PluginOrder, pluginOrder(md)...)
		cfg = Merge(cfg, localCfg)
		cfg.Unknown = unknown
		cfg.PluginOrder = order

		// Preserve Boarded from local config
		if local.Boarded != nil {
			cfg.Boarded = local.Boarded
		}

		if local.Silo != nil {
			cfg.Silo = local.Silo
		}

		if local.Git != "" {
			cfg.Git = local.Git
		}
		cfg.Crew.merge(local.Crew)
		if local.Agents.Guide != "" {
			cfg.Agents.Guide = local.Agents.Guide
		}
		if local.Agents.Skills != "" {
			cfg.Agents.Skills = local.Agents.Skills
		}
		if local.Agents.Process != "" {
			cfg.Agents.Process = local.Agents.Process
		}
		if local.Missions.Records != "" {
			cfg.Missions.Records = local.Missions.Records
		}
		cfg.Artifacts.Local = strings.TrimSpace(local.Artifacts.Path)
		if local.Missions.ReviewBeforePR != nil {
			cfg.Missions.ReviewBeforePR = local.Missions.ReviewBeforePR
		}
		if local.Missions.ReviewBeforeLand != nil {
			cfg.Missions.ReviewBeforeLand = local.Missions.ReviewBeforeLand
		}
		if local.Missions.ReviewBeforeReady != nil {
			cfg.Missions.ReviewBeforeReady = local.Missions.ReviewBeforeReady
		}
		if local.Missions.RepliesNeedMe != nil {
			cfg.Missions.RepliesNeedMe = local.Missions.RepliesNeedMe
		}
		if local.Missions.EndWhenMerged != nil {
			cfg.Missions.EndWhenMerged = local.Missions.EndWhenMerged
		}
		if local.Missions.MaxRounds != 0 {
			cfg.Missions.MaxRounds = local.Missions.MaxRounds
		}
		cfg.GitHub = local.GitHub
		if local.Web.Port != 0 {
			cfg.Web.Port = local.Web.Port
		}
		if local.UI.Accent != "" {
			cfg.UI.Accent = local.UI.Accent
		}
		if local.UI.Sidebar != nil || local.UI.Menu != nil {
			cfg.UI.Sidebar, cfg.UI.Menu = local.UI.Sidebar, local.UI.Menu
		}
		if local.Team.Window != "" {
			cfg.Team.Window = local.Team.Window
		}
		if local.Team.Narrative != nil {
			cfg.Team.Narrative = local.Team.Narrative
		}
		if local.Team.Harness != "" {
			cfg.Team.Harness = local.Team.Harness
		}
		if local.Team.Model != "" {
			cfg.Team.Model = local.Team.Model
		}
		for name, watch := range local.Team.Watch {
			if cfg.Team.Watch == nil {
				cfg.Team.Watch = map[string]TeamWatch{}
			}
			cfg.Team.Watch[name] = watch
		}
		for key, person := range local.Team.People {
			if cfg.Team.People == nil {
				cfg.Team.People = map[string]TeamPerson{}
			}
			cfg.Team.People[key] = person
		}
		cfg.Briefing.merge(local.Briefing)
		cfg.Conversation.merge(local.Conversation)
		cfg.Judge.merge(local.Judge)
		if local.Orchestrator.Harness != "" {
			cfg.Orchestrator.Harness = local.Orchestrator.Harness
		}
		if local.Orchestrator.Model != "" {
			cfg.Orchestrator.Model = local.Orchestrator.Model
		}
		if local.Orchestrator.Enabled != nil {
			cfg.Orchestrator.Enabled = local.Orchestrator.Enabled
		}
		if local.Orchestrator.Dispatch != "" {
			cfg.Orchestrator.Dispatch = local.Orchestrator.Dispatch
		}
		if local.Orchestrator.MaxDispatches != 0 {
			cfg.Orchestrator.MaxDispatches = local.Orchestrator.MaxDispatches
		}
	}

	// Callers add to these (ws lift boards the new capsule), so they're never
	// nil: Merge builds a fresh Config, and a ws.local.toml may have neither
	// section.
	if cfg.Boarded == nil {
		cfg.Boarded = make(map[string][]string)
	}
	if cfg.Silo == nil {
		cfg.Silo = make(map[string]string)
	}

	for _, p := range cfg.Crew.Allowed {
		if _, _, err := ParsePick(p); err != nil {
			return nil, "", fmt.Errorf("invalid [crew] allowed: %w", err)
		}
	}
	if d := cfg.Orchestrator.Dispatch; d != "" && d != "auto" && d != "ask" {
		return nil, "", fmt.Errorf("invalid [orchestrator] dispatch %q: must be \"auto\" or \"ask\"", d)
	}
	if cfg.Git != "" && cfg.Git != "ssh" && cfg.Git != "https" {
		return nil, "", fmt.Errorf("invalid git protocol %q in %s: must be \"ssh\" or \"https\"", cfg.Git, LocalFileName)
	}
	if cfg.UI.Accent != "" {
		if _, err := ParseAccent(cfg.UI.Accent); err != nil {
			return nil, "", fmt.Errorf("invalid [ui] accent: %w", err)
		}
	}

	for name := range cfg.Groups {
		if err := validateGroupName(name); err != nil {
			return nil, "", err
		}
	}
	for repo, rc := range cfg.Repos {
		if !ValidLanding(rc.Landing) {
			return nil, "", fmt.Errorf("repo %q: invalid landing %q: must be %q or %q", repo, rc.Landing, LandingPR, LandingDirect)
		}
		if rc.Group == "" {
			continue
		}
		if err := validateGroupName(rc.Group); err != nil {
			return nil, "", fmt.Errorf("repo %q: %w", repo, err)
		}
	}

	return cfg, root, nil
}

// validateGroupName rejects names that can't be passed to --group, which
// takes a comma-separated list.
func validateGroupName(name string) error {
	if name == "" || strings.ContainsAny(name, ", \t") {
		return fmt.Errorf("invalid group name %q: must be non-empty with no commas or spaces", name)
	}
	return nil
}

// IsHidden reports whether the named group is hidden. Groups with no
// setting are visible.
func (c *Config) IsHidden(group string) bool {
	g, ok := c.Groups[group]
	return ok && g.Hidden != nil && *g.Hidden
}

// undecoded is the part of a TOML document that decoding it left unread,
// as the document has it: nil if all of it was read.
func undecoded(doc string, md toml.MetaData) map[string]any {
	keys := md.Undecoded()
	if len(keys) == 0 {
		return nil
	}
	var all map[string]any
	if _, err := toml.Decode(doc, &all); err != nil {
		return nil
	}
	kept := map[string]any{}
	for _, key := range keys {
		// The value at key, and the table in kept to put it in.
		var from any = all
		into := kept
		for i, part := range key {
			table, ok := from.(map[string]any)
			if !ok {
				break // inside an array of tables: its key carried all of it
			}
			from = table[part]
			if i == len(key)-1 {
				if _, ok := into[part]; !ok {
					into[part] = from
				}
				break
			}
			next, ok := into[part].(map[string]any)
			if !ok {
				if _, taken := into[part]; taken {
					break
				}
				next = map[string]any{}
				into[part] = next
			}
			into = next
		}
	}
	return kept
}

// addMissing puts into dst what src has that dst hasn't, table by table.
// What dst says stands.
func addMissing(dst, src map[string]any) {
	for k, v := range src {
		have, ok := dst[k]
		if !ok {
			dst[k] = v
			continue
		}
		if a, ok := have.(map[string]any); ok {
			if b, ok := v.(map[string]any); ok {
				addMissing(a, b)
			}
		}
	}
}

func SaveBoarded(root string, boarded map[string][]string) error {
	return UpdateLocal(root, func(local *LocalConfig) {
		local.Boarded = boarded
	})
}

func SaveSilo(root string, silo map[string]string) error {
	return UpdateLocal(root, func(local *LocalConfig) {
		local.Silo = silo
	})
}

// SaveGroupVisibility records a personal visibility choice for a group in
// ws.local.toml. A nil hidden removes the choice, so the team default applies.
func SaveGroupVisibility(root, group string, hidden *bool) error {
	return UpdateLocal(root, func(local *LocalConfig) {
		if hidden == nil {
			delete(local.Groups, group)
			return
		}
		if local.Groups == nil {
			local.Groups = make(map[string]GroupConfig)
		}
		local.Groups[group] = GroupConfig{Hidden: hidden}
	})
}

// mergeReviews is the team's passes over a review with your own added:
// one of yours named as one of the team's replaces it.
func mergeReviews(base, local ReviewsConfig) ReviewsConfig {
	out := base
	if local.Guide != (ReviewGuideConfig{}) {
		out.Guide = local.Guide
	}
	if len(local.Passes) == 0 {
		return out
	}
	out.Passes = make(map[string]ReviewPassConfig, len(base.Passes)+len(local.Passes))
	maps.Copy(out.Passes, base.Passes)
	maps.Copy(out.Passes, local.Passes)
	return out
}

// mergePlugin is each plugin's config with yours over the team's, key by
// key. A key that's a table in both (a plugin's rows, a row's fields) is
// laid over in turn: your row changes that row and leaves the team's
// others, and your field of a row leaves the row's other fields.
func mergePlugin(base, local map[string]map[string]any) map[string]map[string]any {
	if len(base) == 0 && len(local) == 0 {
		return nil
	}
	out := map[string]map[string]any{}
	for _, layer := range []map[string]map[string]any{base, local} {
		for name, values := range layer {
			if out[name] == nil {
				out[name] = map[string]any{}
			}
			layOver(out[name], values)
		}
	}
	return out
}

// layOver lays over's keys on under, table by table: a value that's a
// table on both sides is laid over key by key, and anything else replaces
// what was there. What's laid is copied, so neither file's own values
// change.
func layOver(under, over map[string]any) {
	for k, v := range over {
		table, ok := v.(map[string]any)
		if !ok {
			under[k] = v
			continue
		}
		was, ok := under[k].(map[string]any)
		if !ok {
			was = map[string]any{}
		}
		laid := map[string]any{}
		layOver(laid, was)
		layOver(laid, table)
		under[k] = laid
	}
}
