package team

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Changes to how things are done are found by where they are: a repo's
// guides, its skills, its CI and lint settings, the workspace's own
// config, and whatever paths the workspace says to watch. A change that
// touches one says so on its row, whether or not an agent has read it,
// and an agent that does read it is told.

// Watch is some paths whose changes are changes to how things are done.
type Watch struct {
	// Name is the word it's set by ([team.watch.<name>]), and Title what
	// a change that touches it is marked with.
	Name  string
	Title string
	// Paths are globs: * within a folder, ** across them. One that starts
	// "repo:" is only for that repo.
	Paths []string
}

// builtinWatches are watched in every workspace.
var builtinWatches = []Watch{
	{Name: "guides", Title: "Guide", Paths: []string{"**/AGENTS.md", "**/CLAUDE.md", "**/CONTRIBUTING.md", ".cursor/rules/**", ".github/copilot-instructions.md", "**/*-template.md"}},
	{Name: "skills", Title: "Skill", Paths: []string{"**/.claude/skills/**", "**/.agents/skills/**", "**/.cursor/skills/**"}},
	{Name: "ci", Title: "CI", Paths: []string{".github/workflows/**", ".github/actions/**", ".gitlab-ci.yml", "azure-pipelines.yml", ".circleci/**"}},
	{Name: "lint", Title: "Lint", Paths: []string{"**/.eslintrc*", "**/eslint.config.*", "**/ruff.toml", "**/.ruff.toml", "**/.golangci.y*ml", "**/.vale.ini", "**/.vale/**", "**/.editorconfig", "**/biome.json*", "**/.prettierrc*", "**/.pre-commit-config.yaml", "**/lefthook.y*ml"}},
	{Name: "workspace", Title: "Workspace", Paths: []string{"ws.toml", "ws.repo.toml"}},
}

// Watches are the built-in watches with the workspace's own: one of the
// same name replaces a built-in (with no paths, it turns it off).
func Watches(own []Watch) []Watch {
	byName := map[string]bool{}
	for _, w := range own {
		byName[w.Name] = true
	}
	var out []Watch
	for _, w := range builtinWatches {
		if !byName[w.Name] {
			out = append(out, w)
		}
	}
	sort.SliceStable(own, func(i, j int) bool { return own[i].Name < own[j].Name })
	for _, w := range own {
		if len(w.Paths) == 0 {
			continue
		}
		if w.Title == "" {
			w.Title = strings.ToUpper(w.Name[:1]) + w.Name[1:]
		}
		out = append(out, w)
	}
	return out
}

// globs kept as they're compiled.
var (
	globsMu sync.Mutex
	globs   = map[string]*regexp.Regexp{}
)

// glob is a path glob as a pattern: ** is any folders (none too, before a
// slash), * anything within one, ? one character of it.
func glob(pattern string) *regexp.Regexp {
	globsMu.Lock()
	defer globsMu.Unlock()
	if re, ok := globs[pattern]; ok {
		return re
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; {
		case strings.HasPrefix(pattern[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(pattern[i:], "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re := regexp.MustCompile(b.String())
	globs[pattern] = re
	return re
}

// Touch is a watch a change touches, and the files of it that it does.
type Touch struct {
	Name  string   `json:"name"`
	Title string   `json:"title"`
	Files []string `json:"files"`
}

// touched is the watches some files of a repo touch, in the watches'
// order, each with the files that do.
func touched(watches []Watch, repo string, paths []string) []Touch {
	var out []Touch
	for _, w := range watches {
		var files []string
		for _, path := range paths {
			for _, pattern := range w.Paths {
				if only, rest, ok := strings.Cut(pattern, ":"); ok && !strings.Contains(only, "/") {
					if only != repo {
						continue
					}
					pattern = rest
				}
				if glob(pattern).MatchString(path) {
					files = append(files, path)
					break
				}
			}
		}
		if len(files) > 0 {
			out = append(out, Touch{Name: w.Name, Title: w.Title, Files: files})
		}
	}
	return out
}

// touches fills a timeline's rows in with the watches each touches, from
// the files of what's landed (an open pull request's aren't kept).
func (s *Service) touches(t *Timeline) {
	watches := s.watches()
	for _, d := range t.Days {
		for i := range d.Items {
			it := &d.Items[i]
			by := map[string]*Touch{}
			var order []string
			for _, e := range it.Events {
				for _, touch := range touched(watches, e.Repo, e.Paths) {
					have := by[touch.Name]
					if have == nil {
						have = &Touch{Name: touch.Name, Title: touch.Title}
						by[touch.Name] = have
						order = append(order, touch.Name)
					}
					for _, f := range touch.Files {
						seen := false
						for _, g := range have.Files {
							seen = seen || g == f
						}
						if !seen {
							have.Files = append(have.Files, f)
						}
					}
				}
			}
			for _, name := range order {
				it.Touches = append(it.Touches, *by[name])
			}
		}
	}
}

// watches are the watches in force.
func (s *Service) watches() []Watch {
	var own []Watch
	if s.Watch != nil {
		own = s.Watch()
	}
	return Watches(own)
}
