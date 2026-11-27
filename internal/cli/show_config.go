package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/workspace"
)

// ws show config: for an agent asked to help set the workspace up. Where
// it stands (what each file says, each repo's ws.repo.toml and the signs
// in its ground of what's worth configuring), then the guide to every
// setting (config.Guide). It reads files and nothing else: no git, and
// nothing asked of GitHub, so it answers as fast in a workspace of forty
// repos as of two.

// configTops are the tables worth a look when neither ws.toml nor
// ws.local.toml says them, in the order the guide takes them.
var configTops = []string{"missions", "agents", "groups", "documents", "crew", "orchestrator", "conversation", "reviews", "pages", "team", "plugins", "run", "ui"}

// configSigns are the files and folders at a repo's root that say
// something's worth configuring (the guide's Signs), as they're listed.
var configSigns = []string{
	"package.json", "justfile", "Justfile", "Makefile", "Taskfile.yml", "Procfile",
	"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml",
	"go.mod", "Cargo.toml", "pyproject.toml", "Gemfile", "pom.xml", "build.gradle", "build.gradle.kts",
	"pnpm-lock.yaml", "package-lock.json", "yarn.lock", "bun.lockb", "uv.lock", "poetry.lock",
	".env", ".env.example", ".env.sample", ".env.local",
	".storybook/", "docs/", "adr/", "rfcs/",
	"AGENTS.md", "CLAUDE.md", "CONTRIBUTING.md", "CODEOWNERS", ".github/CODEOWNERS", ".github/workflows/", ".github/pull_request_template.md",
}

// configView is ws show config.
func configView(ws *workspace.Workspace) *view {
	v := &view{head: "Setting ws up: where this workspace stands, then the guide to every setting"}
	v.body = append(v.body, "## Where this workspace stands", "")
	team, _ := config.Said(filepath.Join(ws.Root, config.FileName))
	local, _ := config.Said(filepath.Join(ws.Root, config.LocalFileName))
	v.body = append(v.body, fileSays(ws.Root, config.FileName, team), fileSays(ws.Root, config.LocalFileName, local))
	var unset []string
	set := append(config.Tops(team), config.Tops(local)...)
	for _, t := range configTops {
		if !slices.Contains(set, t) {
			unset = append(unset, "["+t+"]")
		}
	}
	if len(unset) > 0 {
		v.body = append(v.body, "- Neither file says: "+strings.Join(unset, ", ")+".")
	}
	v.body = append(v.body, "- At the root: "+rootFiles(ws)+".")
	if keys := ops.SettingKeys(ws.Root); len(keys) > 0 {
		v.body = append(v.body, "- The Settings page has rows for (it writes ws.local.toml): "+strings.Join(keys, ", ")+".")
	}
	v.body = append(v.body, "", fmt.Sprintf("### Repos (%d)", len(ws.RepoNames)), "")
	if len(ws.RepoNames) == 0 {
		v.body = append(v.body, "None yet: [repos.<name>] in ws.toml adds one, or the Add repo button in the web UI.")
	}
	for _, name := range ws.RepoNames {
		v.body = append(v.body, repoStands(ws, name)...)
	}
	v.body = append(v.body, "", strings.TrimRight(config.Guide(), "\n"))
	return v
}

// fileSays is a line on what one of the workspace's files says: its top
// tables, with how many of each that's named by the file ([repos.x]).
func fileSays(root, name string, said []string) string {
	path := filepath.Join(root, name)
	if said == nil {
		if _, err := os.Stat(path); err != nil {
			return fmt.Sprintf("- %s isn't there (%s).", name, path)
		}
	}
	if len(said) == 0 {
		return fmt.Sprintf("- %s (%s) says nothing.", name, path)
	}
	var parts []string
	for _, top := range config.Tops(said) {
		if n := named(said, top); n > 0 {
			parts = append(parts, fmt.Sprintf("%s (%d)", top, n))
		} else {
			parts = append(parts, top)
		}
	}
	return fmt.Sprintf("- %s (%s) says: %s.", name, path, strings.Join(parts, ", "))
}

// named is how many tables of top the file names itself ([repos.web],
// [repos.api]: 2), for a top that's keyed by name; 0 for one that isn't.
func named(said []string, top string) int {
	switch top {
	case "repos", "groups", "documents", "things", "plugins", "plugin", "boarded", "silo":
	default:
		return 0
	}
	var names []string
	for _, k := range said {
		rest, ok := strings.CutPrefix(k, top+".")
		if !ok {
			continue
		}
		n, _, _ := strings.Cut(rest, ".")
		if !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	return len(names)
}

// rootFiles says which of the files the workspace's agents read are at
// its root: its guide, skills, process and PR guide.
func rootFiles(ws *workspace.Workspace) string {
	var have, missing []string
	note := func(what, path string) {
		if _, err := os.Stat(path); path != "" && err == nil {
			have = append(have, what+" "+rel(ws.Root, path))
		} else {
			missing = append(missing, what)
		}
	}
	note("guide", ws.Guide())
	note("skills", ws.SkillsDir())
	note("process", ws.Process())
	note("PR guide", ws.PRGuide())
	out := "has " + strings.Join(have, ", ")
	if len(have) == 0 {
		out = "has none of the files agents read"
	}
	if len(missing) > 0 {
		out += "; no " + strings.Join(missing, ", ")
	}
	return out
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return path
}

// repoStands is a repo's lines: its ground, what the workspace says of
// it, its ws.repo.toml, and the signs at its root.
func repoStands(ws *workspace.Workspace, name string) []string {
	ground := ws.MainWorktree(name)
	var about []string
	if g := ws.GroupOf(name); g != "" {
		about = append(about, "group "+g)
	}
	if l := ws.Landings[name]; l != "" {
		about = append(about, "landing "+l)
	}
	if ws.Yours[name] {
		about = append(about, "yours alone")
	}
	head := "- " + name
	if len(about) > 0 {
		head += " (" + strings.Join(about, ", ") + ")"
	}
	if _, err := os.Stat(ground); err != nil {
		return []string{head + ": not cloned here, so there's nothing to read."}
	}
	lines := []string{head + ": " + rel(ws.Root, ground)}
	repoFile := filepath.Join(ground, config.RepoFileName)
	said, err := config.Said(repoFile)
	switch {
	case err != nil:
		lines = append(lines, "  Its ws.repo.toml can't be read: "+err.Error())
	case said == nil:
		lines = append(lines, "  No ws.repo.toml.")
	default:
		line := "  ws.repo.toml says: " + strings.Join(config.Tops(said), ", ")
		if rc, err := config.ParseRepoConfig(repoFile); err == nil && rc != nil {
			if names := sortedKeys(rc.Run.Service); len(names) > 0 {
				line += "; services " + strings.Join(names, ", ")
			}
			if names := sortedKeys(rc.Run.Task); len(names) > 0 {
				line += "; tasks " + strings.Join(names, ", ")
			}
		}
		lines = append(lines, line+".")
	}
	if signs := repoSigns(ground); len(signs) > 0 {
		lines = append(lines, "  Signs: "+strings.Join(signs, ", ")+".")
	}
	return lines
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// repoSigns are configSigns that are at a repo's root, package.json with
// its scripts' names.
func repoSigns(ground string) []string {
	var out []string
	for _, s := range configSigns {
		info, err := os.Stat(filepath.Join(ground, strings.TrimSuffix(s, "/")))
		if err != nil || info.IsDir() != strings.HasSuffix(s, "/") {
			continue
		}
		if s == "package.json" {
			if scripts := packageScripts(filepath.Join(ground, s)); len(scripts) > 0 {
				s += " (scripts: " + strings.Join(scripts, ", ") + ")"
			}
		}
		out = append(out, s)
	}
	return out
}

// maxScripts is the most of a package.json's scripts named.
const maxScripts = 16

// packageScripts are the names of a package.json's scripts, sorted, at
// most maxScripts and then how many more.
func packageScripts(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	names := sortedKeys(pkg.Scripts)
	if len(names) > maxScripts {
		names = append(names[:maxScripts], fmt.Sprintf("%d more", len(names)-maxScripts))
	}
	return names
}
