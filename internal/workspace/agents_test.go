package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// agentsWorkspace is a workspace whose standards are where other harnesses
// than Claude Code look: AGENTS.md, and two skills in .agents/skills.
func agentsWorkspace(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		full := filepath.Join(root, path)
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("AGENTS.md", "# Guide\n\nHow we work.\n")
	write(".agents/skills/atomic-repo/SKILL.md", "---\nname: atomic-repo\ndescription: How to land changes in a repo that takes commits directly on its default branch. Use only when the user asks.\n---\n\n# Atomic repo\n")
	write(".agents/skills/build-proposals/SKILL.md", "---\nname: build-proposals\ndescription: >\n  Write, review and hydrate Build Proposals.\n  Three checks along the way.\nlicense: MIT\n---\n\n# BPs\n")
	write(".agents/skills/notes/README.md", "not a skill: no SKILL.md\n")
	return &Workspace{Root: root, DefaultBranch: "main", Harnesses: []string{"claude"}}
}

func TestSkills_FoundWhereTheyAreWithWhatEachIsFor(t *testing.T) {
	ws := agentsWorkspace(t)

	if got := ws.Guide(); got != filepath.Join(ws.Root, "AGENTS.md") {
		t.Errorf("Guide() = %q", got)
	}
	skills := ws.Skills()
	if len(skills) != 2 {
		t.Fatalf("Skills() = %+v", skills)
	}
	if s := skills[0]; s.Name != "atomic-repo" || !strings.HasPrefix(s.About, "How to land changes") || s.Path != filepath.Join(ws.Root, ".agents", "skills", "atomic-repo", "SKILL.md") {
		t.Errorf("atomic-repo = %+v", s)
	}
	// A description that runs over lines reads as one.
	if s := skills[1]; s.Name != "build-proposals" || s.About != "Write, review and hydrate Build Proposals. Three checks along the way." {
		t.Errorf("build-proposals = %+v", s)
	}

	// [agents] says otherwise.
	os.MkdirAll(filepath.Join(ws.Root, "how", "lint"), 0o755)
	os.WriteFile(filepath.Join(ws.Root, "how", "lint", "SKILL.md"), []byte("---\nname: lint\n---\n"), 0o644)
	ws.AgentsGuide, ws.AgentsSkills = "docs/GUIDE.md", "how"
	if got := ws.Guide(); got != filepath.Join(ws.Root, "docs", "GUIDE.md") {
		t.Errorf("Guide() = %q", got)
	}
	if skills := ws.Skills(); len(skills) != 1 || skills[0].Name != "lint" || skills[0].About != "" {
		t.Errorf("Skills() = %+v", skills)
	}

	// A workspace with neither has neither, and nothing to check.
	bare := &Workspace{Root: t.TempDir(), Harnesses: []string{"claude"}}
	if bare.Guide() != "" || bare.Skills() != nil || len(bare.checkAgents().Checks) != 0 {
		t.Errorf("a workspace with no guide or skills: %q %v %v", bare.Guide(), bare.Skills(), bare.checkAgents().Checks)
	}
}

func check(cat CheckCategory, name string) *CheckResult {
	for i := range cat.Checks {
		if cat.Checks[i].Name == name {
			return &cat.Checks[i]
		}
	}
	return nil
}

func TestDoctor_AHarnessThatLooksElsewhereIsLinked(t *testing.T) {
	ws := agentsWorkspace(t)

	cat := ws.checkAgents()
	guide, skills := check(cat, "claude reads the guide"), check(cat, "claude sees the skills")
	if guide == nil || guide.Status != CheckWarn || !strings.Contains(guide.Detail, "CLAUDE.md") || guide.Fix == nil {
		t.Fatalf("the guide: %+v", guide)
	}
	if skills == nil || skills.Status != CheckWarn || !strings.Contains(skills.Detail, "atomic-repo, build-proposals") || skills.Fix == nil {
		t.Fatalf("the skills: %+v", skills)
	}
	if err := guide.Fix(); err != nil {
		t.Fatal(err)
	}
	if err := skills.Fix(); err != nil {
		t.Fatal(err)
	}

	// Linked, by paths that hold wherever the workspace is.
	if to, _ := os.Readlink(filepath.Join(ws.Root, "CLAUDE.md")); to != "AGENTS.md" {
		t.Errorf("CLAUDE.md -> %q", to)
	}
	if to, _ := os.Readlink(filepath.Join(ws.Root, ".claude", "skills", "atomic-repo")); to != filepath.Join("..", "..", ".agents", "skills", "atomic-repo") {
		t.Errorf(".claude/skills/atomic-repo -> %q", to)
	}
	if data, err := os.ReadFile(filepath.Join(ws.Root, ".claude", "skills", "build-proposals", "SKILL.md")); err != nil || !strings.Contains(string(data), "build-proposals") {
		t.Errorf("the linked skill doesn't read: %v", err)
	}
	cat = ws.checkAgents()
	for _, c := range cat.Checks {
		if c.Status != CheckOK {
			t.Errorf("after the fix, %s: %s", c.Name, c.Detail)
		}
	}
}

func TestDoctor_AGuideOfItsOwnIsntReplaced(t *testing.T) {
	ws := agentsWorkspace(t)
	// An older guide, as Claude Code reads it: not the workspace's.
	os.WriteFile(filepath.Join(ws.Root, "CLAUDE.md"), []byte("# Older guide\n\nWithout the newer sections.\n"), 0o644)
	// And a skill of the same name, that's something else.
	os.MkdirAll(filepath.Join(ws.Root, ".claude", "skills", "atomic-repo"), 0o755)

	cat := ws.checkAgents()
	guide := check(cat, "claude reads the guide")
	if guide == nil || guide.Status != CheckWarn || guide.Fix != nil || !strings.Contains(guide.Detail, "drift apart") || guide.FixHint == "" {
		t.Fatalf("the guide: %+v", guide)
	}
	if err := check(cat, "claude sees the skills").Fix(); err == nil || !strings.Contains(err.Error(), "is there already") {
		t.Errorf("linking over a skill that's there: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(ws.Root, "CLAUDE.md")); !strings.Contains(string(data), "Older guide") {
		t.Error("CLAUDE.md was replaced")
	}

	// One that only points at the workspace's is fine as it is.
	os.WriteFile(filepath.Join(ws.Root, "CLAUDE.md"), []byte("@AGENTS.md\n"), 0o644)
	if guide := check(ws.checkAgents(), "claude reads the guide"); guide.Status != CheckOK {
		t.Errorf("a CLAUDE.md that imports the guide: %+v", guide)
	}

	// A harness that looks where the standards are has nothing to fix.
	ws.Harnesses = []string{"cursor"}
	for _, c := range ws.checkAgents().Checks {
		if c.Status != CheckOK {
			t.Errorf("cursor, %s: %s", c.Name, c.Detail)
		}
	}
}
