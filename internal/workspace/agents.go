package workspace

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A workspace's own way of working, for the agents ws starts: its guide
// (AGENTS.md, say) and its skills (a folder each, with a SKILL.md saying
// what it's for and how it's done). Each agent is told where the guide is,
// and how to list the skills, in whichever harness it runs: every harness
// looks in a place of its own (Claude Code in .claude/skills and
// CLAUDE.md, others in .agents/skills and AGENTS.md), and a workspace's
// standards are usually in one of them.

// Where a workspace's guide and skills are when [agents] doesn't say: the
// first of these that's there.
var (
	guideNames = []string{"AGENTS.md", "CLAUDE.md"}
	skillsDirs = []string{filepath.Join(".agents", "skills"), filepath.Join(".claude", "skills")}
)

// inWorkspace is a path as [agents] gives it: relative to the workspace,
// absolute, or from ~.
func (w *Workspace) inWorkspace(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(w.Root, path)
}

// Guide is the workspace's guide for agents: [agents] guide, else
// AGENTS.md or CLAUDE.md at its root. "" with none.
func (w *Workspace) Guide() string {
	if w.AgentsGuide != "" {
		return w.inWorkspace(w.AgentsGuide)
	}
	for _, name := range guideNames {
		if info, err := os.Stat(filepath.Join(w.Root, name)); err == nil && !info.IsDir() {
			return filepath.Join(w.Root, name)
		}
	}
	return ""
}

// ProcessName is where the workspace's process is when [agents] doesn't
// say: beside ws.toml, in the config repo.
const ProcessName = "PROCESS.md"

// Process is where the workspace's process is (how work goes here, in
// prose): [agents] process, else PROCESS.md at its root, whether or not
// it's there.
func (w *Workspace) Process() string {
	if w.AgentsProcess != "" {
		return w.inWorkspace(w.AgentsProcess)
	}
	return filepath.Join(w.Root, ProcessName)
}

// PRGuideName is where the workspace says how its pull requests are
// written when [agents] doesn't say: beside ws.toml, in the config repo.
const PRGuideName = "PR.md"

// PRGuide is where the workspace says how its pull requests are written,
// for the PR writer: [agents] pr, else PR.md at its root, whether or not
// it's there.
func (w *Workspace) PRGuide() string {
	if w.AgentsPR != "" {
		return w.inWorkspace(w.AgentsPR)
	}
	return filepath.Join(w.Root, PRGuideName)
}

// SkillsDir is the folder the workspace's skills are in: [agents] skills,
// else .agents/skills or .claude/skills. "" with none.
func (w *Workspace) SkillsDir() string {
	if w.AgentsSkills != "" {
		return w.inWorkspace(w.AgentsSkills)
	}
	for _, dir := range skillsDirs {
		if info, err := os.Stat(filepath.Join(w.Root, dir)); err == nil && info.IsDir() {
			return filepath.Join(w.Root, dir)
		}
	}
	return ""
}

// Skill is one of the workspace's skills: its name, what it's for (its
// SKILL.md's description), and that file, which says how it's done.
type Skill struct {
	Name  string
	About string
	Path  string
}

// Skills are the workspace's skills, by name: each folder of SkillsDir
// with a SKILL.md.
func (w *Workspace) Skills() []Skill {
	dir := w.SkillsDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Skill
	for _, e := range entries {
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		s := Skill{Name: e.Name(), Path: path}
		name, about := skillFrontMatter(f)
		f.Close()
		if name != "" {
			s.Name = name
		}
		s.About = about
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// skillFrontMatter reads a SKILL.md's name and description from the front
// matter it starts with. A description may run on (folded, or indented
// lines after it).
func skillFrontMatter(f *os.File) (name, about string) {
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return "", ""
	}
	field := ""
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		if key, value, ok := strings.Cut(line, ":"); ok && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			field = strings.TrimSpace(key)
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			if value == ">" || value == "|" || value == ">-" || value == "|-" {
				value = ""
			}
			switch field {
			case "name":
				name = value
			case "description":
				about = value
			}
			continue
		}
		if field == "description" && strings.TrimSpace(line) != "" {
			about = strings.TrimSpace(about + " " + strings.TrimSpace(line))
		}
	}
	return name, strings.Join(strings.Fields(about), " ")
}

// harnessLooks is where a harness looks for a project's skills and guide
// by itself, relative to where it runs.
func harnessLooks(name string) (skills, guide string) {
	if name == "claude" {
		return filepath.Join(".claude", "skills"), "CLAUDE.md"
	}
	return filepath.Join(".agents", "skills"), "AGENTS.md"
}

// sameFile reports whether two paths are one file (a link to it counts).
func sameFile(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	return err == nil && os.SameFile(ai, bi)
}

// pointsAt reports whether the file at path does nothing but point at the
// guide: a line importing it (@AGENTS.md), with or without a word of why.
func pointsAt(path, guide string) bool {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 2000 {
		return false
	}
	return strings.Contains(string(data), "@"+filepath.Base(guide))
}

// checkAgents says when a harness in use can't see the workspace's skills,
// or reads a guide that isn't the workspace's. ws tells every agent where
// both are, whichever harness it runs in, so these are warnings: what the
// harness finds by itself (its own skill tool, the guide it loads at the
// start) is otherwise something else. Only for a workspace with a guide or
// skills, and harnesses that are in use.
func (w *Workspace) checkAgents() CheckCategory {
	cat := CheckCategory{Name: "Agents"}
	guide, skillsDir, skills := w.Guide(), w.SkillsDir(), w.Skills()
	if guide == "" && len(skills) == 0 {
		return cat
	}
	if guide != "" {
		if _, err := os.Stat(guide); err != nil {
			cat.Checks = append(cat.Checks, CheckResult{Name: "guide", Status: CheckWarn, Detail: fmt.Sprintf("[agents] guide is %s, which isn't there", w.rel(guide))})
			guide = ""
		} else {
			cat.Checks = append(cat.Checks, CheckResult{Name: "guide", Status: CheckOK, Detail: w.rel(guide)})
		}
	}
	if skillsDir != "" {
		cat.Checks = append(cat.Checks, CheckResult{Name: "skills", Status: CheckOK, Detail: fmt.Sprintf("%d in %s", len(skills), w.rel(skillsDir))})
	}
	seen := map[string]bool{}
	for _, h := range w.Harnesses {
		looksSkills, looksGuide := harnessLooks(h)
		if seen[looksSkills] {
			continue // two harnesses that look in the same places: said once
		}
		seen[looksSkills] = true
		if guide != "" {
			theirs := filepath.Join(w.Root, looksGuide)
			c := CheckResult{Name: h + " reads the guide", Status: CheckOK}
			_, err := os.Lstat(theirs)
			switch {
			case sameFile(theirs, guide) || pointsAt(theirs, guide):
			case err != nil:
				c.Status = CheckWarn
				c.Detail = fmt.Sprintf("%s loads %s by itself, and there's none: the workspace's guide is %s", h, looksGuide, w.rel(guide))
				c.Fix = func() error { return linkTo(theirs, guide) }
			default:
				c.Status = CheckWarn
				c.Detail = fmt.Sprintf("%s loads %s by itself, which isn't the workspace's guide (%s): the two can drift apart", h, looksGuide, w.rel(guide))
				c.FixHint = fmt.Sprintf("move what only %s says into %s, and remove it: ws doctor --fix then links it", looksGuide, w.rel(guide))
			}
			cat.Checks = append(cat.Checks, c)
		}
		if len(skills) > 0 {
			theirs := filepath.Join(w.Root, looksSkills)
			var missing []Skill
			for _, s := range skills {
				dir := filepath.Dir(s.Path)
				if !sameFile(filepath.Join(theirs, filepath.Base(dir)), dir) {
					missing = append(missing, s)
				}
			}
			c := CheckResult{Name: h + " sees the skills", Status: CheckOK}
			if len(missing) > 0 {
				names := make([]string, len(missing))
				for i, s := range missing {
					names[i] = s.Name
				}
				c.Status = CheckWarn
				c.Detail = fmt.Sprintf("%s looks for skills in %s, which hasn't %s", h, looksSkills, strings.Join(names, ", "))
				c.Fix = func() error {
					if err := os.MkdirAll(theirs, 0o755); err != nil {
						return err
					}
					for _, s := range missing {
						dir := filepath.Dir(s.Path)
						at := filepath.Join(theirs, filepath.Base(dir))
						if _, err := os.Lstat(at); err == nil {
							return fmt.Errorf("%s is there already, and isn't %s: remove it, or move what it has into the workspace's", w.rel(at), w.rel(dir))
						}
						if err := linkTo(at, dir); err != nil {
							return err
						}
					}
					return nil
				}
			}
			cat.Checks = append(cat.Checks, c)
		}
	}
	return cat
}

// linkTo makes at a link to target, by the path from one to the other, so
// it holds wherever the workspace is checked out.
func linkTo(at, target string) error {
	rel, err := filepath.Rel(filepath.Dir(at), target)
	if err != nil {
		rel = target
	}
	return os.Symlink(rel, at)
}

// rel is a path as it reads from the workspace's root.
func (w *Workspace) rel(path string) string {
	if r, err := filepath.Rel(w.Root, path); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return path
}
