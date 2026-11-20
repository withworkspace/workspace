package ops

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Load reads the workspace's config from dir (or a folder above it), and
// what follows from it: its repos, and where GitHub is asked.
func Load(dir string) (*config.Config, *workspace.Workspace, github.Client, error) {
	cfg, root, err := config.Load(dir)
	if err != nil {
		return nil, nil, nil, err
	}

	names := make([]string, 0, len(cfg.Repos))
	for name := range cfg.Repos {
		names = append(names, name)
	}
	sort.Strings(names)

	// Build alias map, display names, and custom colors
	aliasMap := make(map[string]string)
	displayNames := make(map[string]string)
	repoColors := make(map[string]string)
	afterCreateHooks := make(map[string]string)
	landings := make(map[string]string)
	verifies := make(map[string]string)
	documents := make(map[string]*config.Documents)
	repoGroups := make(map[string]string)
	hiddenGroups := make(map[string]bool)

	for name := range cfg.Groups {
		if cfg.IsHidden(name) {
			hiddenGroups[name] = true
		}
	}

	for name, rc := range cfg.Repos {
		if rc.Group != "" {
			repoGroups[name] = rc.Group
		}
		if rc.DisplayName != "" {
			displayNames[name] = rc.DisplayName
		}
		if rc.Color != "" {
			repoColors[name] = rc.Color
		}
		if rc.AfterCreate != "" {
			afterCreateHooks[name] = rc.AfterCreate
		}
		if rc.Landing != "" {
			landings[name] = rc.Landing
		}
		if rc.Verify != "" {
			verifies[name] = rc.Verify
		}
		if rc.Documents != nil {
			documents[name] = rc.Documents
		}
		for _, alias := range rc.Aliases {
			if _, isCanonical := cfg.Repos[alias]; isCanonical {
				return nil, nil, nil, fmt.Errorf("alias %q for repo %q collides with a canonical repo name", alias, name)
			}
			if existing, ok := aliasMap[alias]; ok {
				return nil, nil, nil, fmt.Errorf("alias %q is used by both %q and %q", alias, existing, name)
			}
			aliasMap[alias] = name
		}
	}

	kinds, err := documentKinds(cfg, aliasMap)
	if err != nil {
		return nil, nil, nil, err
	}

	ws := &workspace.Workspace{
		Root:             root,
		Org:              cfg.Workspace.Org,
		DefaultBranch:    cfg.Workspace.DefaultBranch,
		GitProtocol:      cfg.Git,
		Name:             cfg.Workspace.DisplayName,
		RepoNames:        names,
		RepoGroups:       repoGroups,
		HiddenGroups:     hiddenGroups,
		AliasMap:         aliasMap,
		DisplayNames:     displayNames,
		RepoColors:       repoColors,
		AfterCreateHooks: afterCreateHooks,
		Landings:         landings,
		Verifies:         verifies,
		DocumentsSaid:    documents,
		Records:          canonical(cfg.Missions.Records, aliasMap),
		AgentsGuide:      cfg.Agents.Guide,
		AgentsSkills:     cfg.Agents.Skills,
		AgentsProcess:    cfg.Agents.Process,
		AgentsPR:         cfg.Agents.PR,
		Kinds:            kinds,
		Harnesses:        harnessesInUse(cfg),
		Boarded:          cfg.Boarded,
		Silo:             cfg.Silo,
	}

	var gh github.Client = github.LiveClient{}
	if cfg.GitHub.Fixture != "" { // no GitHub here: a file answers for it
		gh = github.FixtureClient{Path: github.FixturePath(root, cfg.GitHub.Fixture)}
	}
	return cfg, ws, gh, nil
}

// documentKinds are the kinds of document the workspace says it has
// ([documents.<name>]), by name, each in a repo it has (named by its
// canonical name or an alias).
func documentKinds(cfg *config.Config, aliases map[string]string) ([]*workspace.Kind, error) {
	names := slices.Sorted(maps.Keys(cfg.Documents))
	out := make([]*workspace.Kind, 0, len(names))
	for _, name := range names {
		dc := cfg.Documents[name]
		repo, pattern, ok := strings.Cut(dc.Path, ":")
		if !ok {
			return nil, fmt.Errorf("[documents.%s]: path %q doesn't say which repo: it's repo:path", name, dc.Path)
		}
		if canonical, ok := aliases[repo]; ok {
			repo = canonical
		}
		if _, ok := cfg.Repos[repo]; !ok {
			return nil, fmt.Errorf("[documents.%s]: path %q is in %s, which isn't one of the workspace's repos", name, dc.Path, repo)
		}
		for _, p := range dc.Passes {
			if _, ok := cfg.Pages.Passes[p]; !ok && !isBuiltinPass(p) {
				return nil, fmt.Errorf("[documents.%s]: passes names %q, which isn't one of the workspace's passes ([pages.passes.%s])", name, p, p)
			}
		}
		var moves []workspace.Move
		for _, mn := range slices.Sorted(maps.Keys(dc.Moves)) {
			m := dc.Moves[mn]
			moves = append(moves, workspace.Move{Name: mn, To: m.To, Commit: m.Commit, Asks: m.Asks})
		}
		k, err := workspace.NewKind(workspace.Kind{Name: name, Title: dc.Title, Repo: repo, Pattern: pattern, Add: dc.Add, Amend: dc.Amend, Guide: dc.Guide, Passes: dc.Passes, Moves: moves})
		if err != nil {
			return nil, fmt.Errorf("[documents.%s]: %w", name, err)
		}
		out = append(out, k)
	}
	return out, nil
}

// isBuiltinPass reports whether name is one of the passes every page has.
func isBuiltinPass(name string) bool {
	return (page.PassDef{Name: name}).Built()
}

// harnessesInUse are the harnesses the workspace's agents run in: what's
// configured for crew (cursor, by default), briefing, the orchestrator and
// conversations, each once.
func harnessesInUse(cfg *config.Config) []string {
	crew := cfg.Crew.Harness
	if crew == "" {
		crew = harness.DefaultHarness
	}
	var out []string
	for _, h := range []string{crew, cfg.Briefing.Harness, cfg.Orchestrator.Harness, cfg.Conversation.Harness} {
		if h != "" && !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	return out
}

// canonical is a repo's canonical name, for one given by an alias.
func canonical(repo string, aliases map[string]string) string {
	if c, ok := aliases[repo]; ok {
		return c
	}
	return repo
}
