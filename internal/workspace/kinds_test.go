package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustKind(t *testing.T, k Kind) *Kind {
	t.Helper()
	got, err := NewKind(k)
	if err != nil {
		t.Fatalf("NewKind(%+v): %v", k, err)
	}
	return got
}

func TestKind_AnyPathOfItsShapeIsOne(t *testing.T) {
	bp := mustKind(t, Kind{Name: "bp", Title: "Build proposal", Repo: "cr-records", Pattern: "bp/proposed/{date}-{slug}.md"})
	adr := mustKind(t, Kind{Name: "adr", Repo: "cr-records", Pattern: "adr/proposed/{number}-{slug}.md"})

	if f, ok := bp.Match("bp/proposed/2026-10-27-portfolio-bundle-contents.md"); !ok || f.Date != "2026-10-27" || f.Slug != "portfolio-bundle-contents" {
		t.Errorf("a BP: %+v, %v", f, ok)
	}
	if f, ok := adr.Match("adr/proposed/0038-cache-by-request.md"); !ok || f.Number != "0038" || f.Slug != "cache-by-request" {
		t.Errorf("an ADR: %+v, %v", f, ok)
	}
	if adr.Title != "adr" {
		t.Errorf("a kind with no title is called by its name: %q", adr.Title)
	}
	for _, rel := range []string{
		"bp/accepted/2026-10-27-portfolio.md", // another folder
		"bp/proposed/portfolio.md",            // no date
		"bp/proposed/2026-10-27-a/b.md",       // a folder deeper
		"bp/proposed/2026-10-27-portfolio.txt",
		"x/bp/proposed/2026-10-27-portfolio.md",
	} {
		if _, ok := bp.Match(rel); ok {
			t.Errorf("%s isn't a BP's path", rel)
		}
	}
}

func TestNewKind_RefusesAPathThatCantBeOne(t *testing.T) {
	for pattern, says := range map[string]string{
		"":                           "no path",
		"notes/today.md":             "neither {slug} nor {number}",
		"notes/{title}.md":           "{title}",
		"notes/{slug}-{slug}.md":     "twice",
		"notes/{date}-{slug}.txt":    "markdown",
		"/notes/{slug}.md":           "leads out",
		"../other/notes/{slug}.md":   "leads out",
		"notes/{date}/{number}.md":   "",
		"notes/{whatever}-{slug}.md": "",
	} {
		_, err := NewKind(Kind{Name: "note", Repo: "records", Pattern: pattern})
		switch {
		case says == "" && err != nil:
			t.Errorf("%q: %v", pattern, err)
		case says != "" && (err == nil || !strings.Contains(err.Error(), says)):
			t.Errorf("%q: want an error saying %q, got %v", pattern, says, err)
		}
	}
}

func TestKind_SubjectIsItsMessageFilledIn(t *testing.T) {
	bp := mustKind(t, Kind{Name: "bp", Repo: "r", Pattern: "bp/proposed/{date}-{slug}.md", Add: "docs: add bp {date} {slug}", Amend: "docs: amend bp {date} {slug}"})
	f, _ := bp.Match("bp/proposed/2026-10-27-portfolio-bundle.md")
	if got := bp.Subject(false, f); got != "docs: amend bp 2026-10-27 portfolio-bundle" {
		t.Errorf("amend: %q", got)
	}
	if got := bp.Subject(true, f); got != "docs: add bp 2026-10-27 portfolio-bundle" {
		t.Errorf("add: %q", got)
	}
	adr := mustKind(t, Kind{Name: "adr", Repo: "r", Pattern: "adr/{number}-{slug}.md", Add: "docs: add adr {number} {title}"})
	f, _ = adr.Match("adr/0007-x.md")
	f.Title = "Cache by request"
	if got := adr.Subject(true, f); got != "docs: add adr 0007 Cache by request" {
		t.Errorf("with its title: %q", got)
	}
	if got := adr.Subject(false, f); got != "" {
		t.Errorf("a kind with no amend message has no subject for a change: %q", got)
	}
	// What isn't known stays as written.
	if got := (DocFields{Slug: "x"}).Fill("{slug} {title} {other}"); got != "x {title} {other}" {
		t.Errorf("Fill = %q", got)
	}
}

func TestKindOf_ByRepoAndPath(t *testing.T) {
	root := t.TempDir()
	note := mustKind(t, Kind{Name: "note", Repo: "records", Pattern: "notes/{slug}.md", Guide: "records:notes/README.md"})
	adr := mustKind(t, Kind{Name: "adr", Repo: "records", Pattern: "adr/{number}-{slug}.md", Guide: ".agents/skills/adr/SKILL.md"})
	ws := &Workspace{Root: root, RepoNames: []string{"records"}, AliasMap: map[string]string{"rec": "records"}, Kinds: []*Kind{adr, note}}

	if k, f := ws.KindOf("records", "notes/greeting.md"); k != note || f.Slug != "greeting" {
		t.Errorf("a note: %v %+v", k, f)
	}
	if k, _ := ws.KindOf("demo", "notes/greeting.md"); k != nil {
		t.Errorf("another repo's notes folder isn't the kind's: %v", k)
	}
	if k, _ := ws.KindOf("records", "README.md"); k != nil {
		t.Errorf("README.md is of no kind: %v", k)
	}
	if got, want := ws.KindGuide(note), filepath.Join(root, "repos", "records", GroundDir, "notes", "README.md"); got != want {
		t.Errorf("a guide in a repo is read from its ground: %q, want %q", got, want)
	}
	if got, want := ws.KindGuide(adr), filepath.Join(root, ".agents", "skills", "adr", "SKILL.md"); got != want {
		t.Errorf("a guide in the workspace: %q, want %q", got, want)
	}

	cat := ws.checkKinds()
	if len(cat.Checks) != 2 || cat.Checks[0].Status != CheckWarn || !strings.Contains(cat.Checks[0].Detail, "isn't there") {
		t.Errorf("doctor, with neither guide there: %+v", cat.Checks)
	}
	os.MkdirAll(filepath.Join(root, ".agents", "skills", "adr"), 0o755)
	os.WriteFile(ws.KindGuide(adr), []byte("# ADRs\n"), 0o644)
	if cat := ws.checkKinds(); cat.Checks[0].Status != CheckOK || cat.Checks[0].Detail != "records:adr/{number}-{slug}.md" {
		t.Errorf("doctor, with the ADR's guide there: %+v", cat.Checks[0])
	}
}

func TestDocTitle(t *testing.T) {
	for text, want := range map[string]string{
		"# Caching requirements\n\nText.\n":                          "Caching requirements",
		"Intro line.\n\n## **Second** level `first`\n":               "Second level first",
		"---\ntitle: \"From front matter\"\nstatus: x\n---\n# Not\n": "From front matter",
		"---\nstatus: x\n---\n\n# After front matter\n":              "After front matter",
		"```\n# a comment in code\n```\n#tag\n# The heading ##\n":    "The heading",
		"No heading at all.\n":                                       "",
	} {
		if got := DocTitle([]byte(text)); got != want {
			t.Errorf("DocTitle(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestSlug(t *testing.T) {
	for title, want := range map[string]string{
		"Caching Requirements!":     "caching-requirements",
		"  ADR 38: Cache — by req ": "adr-38-cache-by-req",
		"???":                       "untitled",
		strings.Repeat("word ", 20): strings.TrimSuffix(strings.Repeat("word-", 12), "-"),
	} {
		if got := Slug(title); got != want {
			t.Errorf("Slug(%q) = %q, want %q", title, got, want)
		}
	}
}
