package cli

import (
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/workspace"
)

// An agent on a document of one of the workspace's kinds is told what it
// is, and to read the kind's guide first; every agent is told the kinds
// there are.
func TestKindNotes(t *testing.T) {
	root := t.TempDir()
	bp, err := workspace.NewKind(workspace.Kind{Name: "bp", Title: "Build proposal", Repo: "records", Pattern: "bp/proposed/{date}-{slug}.md", Guide: ".agents/skills/build-proposals/SKILL.md"})
	if err != nil {
		t.Fatal(err)
	}
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"records"}, AliasMap: map[string]string{"rec": "records"}, Kinds: []*workspace.Kind{bp}}
	guide := filepath.Join(root, ".agents", "skills", "build-proposals", "SKILL.md")

	// With no guide there, it's told the kind, and nothing of a guide.
	got := ops.KindPara(ws, "rec:bp/proposed/2026-10-27-portfolio.md")
	if !strings.Contains(got, "of a kind this workspace has: Build proposal (bp, at records:bp/proposed/{date}-{slug}.md).") || strings.Contains(got, "read it") {
		t.Errorf("no guide there: %q", got)
	}
	os.MkdirAll(filepath.Dir(guide), 0o755)
	os.WriteFile(guide, []byte("# BPs\n"), 0o644)
	if got := ops.KindPara(ws, "records/some-capsule:bp/proposed/2026-10-27-portfolio.md"); !strings.Contains(got, "How one's written is in "+guide+": read it before you answer") {
		t.Errorf("in a capsule, with the guide there: %q", got)
	}
	for _, ref := range []string{"records:README.md", "mission:notes.md", "demo:bp/proposed/2026-10-27-x.md"} {
		if got := ops.KindPara(ws, ref); got != "" {
			t.Errorf("%s is of no kind: %q", ref, got)
		}
	}

	if note := ops.AgentsNote(ws); !strings.Contains(note, "Its documents are of kinds (Build proposal): `ws show kinds`") {
		t.Errorf("what every agent is told: %q", note)
	}
	if note := ops.AgentsNote(&workspace.Workspace{Root: t.TempDir()}); note != "" {
		t.Errorf("a workspace with no guide, skills or kinds: %q", note)
	}
}

// The agents of a page started as a kind are told what it's to be, and
// the guide to read; a page of no kind tells them nothing.
func TestPageKindPara(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	adr, _ := workspace.NewKind(workspace.Kind{Name: "adr", Title: "ADR", Repo: "records", Pattern: "adr/{number}-{slug}.md", Guide: "GUIDE.md"})
	ctx := ops.New(nil, &workspace.Workspace{Root: root, Kinds: []*workspace.Kind{adr}}, nil, ops.Options{})
	os.WriteFile(filepath.Join(root, "GUIDE.md"), []byte("# ADRs\n"), 0o644)
	ps := page.Store{Root: root}
	plain, _ := ps.New([]byte("# Plain\n"))
	asADR, _ := ps.New([]byte("# Cache\n"))
	ps.SetMeta(asADR.ID, page.Meta{Kind: "adr"})

	if got := ctx.PageKindPara(plain.ID); got != "" {
		t.Errorf("a page of no kind: %q", got)
	}
	got := ctx.PageKindPara(asADR.ID)
	if !strings.Contains(got, "one of the kinds of document this workspace has, ADR (adr): once written, it's filed at records:adr/{number}-{slug}.md.") || !strings.Contains(got, "How one's written is in "+filepath.Join(root, "GUIDE.md")) {
		t.Errorf("a page started as an ADR: %q", got)
	}
}

// ws show passes counts what came of each pass's findings, over pages:
// taken, and dismissed by why, and of those judged, how many were right.
func TestPassesView(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ctx := ops.New(nil, &workspace.Workspace{Root: root}, nil, ops.Options{})
	s := conversation.ForWorkspace(root)
	for _, pg := range []string{"p1", "p2"} {
		c, _ := s.OpenPass(pg, "check", time.Now())
		_, fs := conversation.ParseFindings("## question\n> a\nA?\n\n## missing\nB.\n\n## ambiguity\n> c\nC?\n")
		s.Say(c.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "Three.", Findings: fs})
		s.SettleFinding(c.ID, fs[0].Key, conversation.FindingDone, "", time.Now())
		s.SettleFinding(c.ID, fs[1].Key, conversation.FindingDismissed, conversation.DismissWrong, time.Now())
		if pg == "p2" {
			s.SettleFinding(c.ID, fs[2].Key, conversation.FindingDismissed, conversation.DismissMinor, time.Now())
		}
	}

	got := passesView(ctx).render(false, true)

	for _, want := range []string{"- Check (check): 2 runs over 2 pages; 6 findings: 2 taken, 1 not worth it, 2 wrong, 1 open", "Of 5 judged: 3 right (60%), 2 acted on (40%)"} {
		if !strings.Contains(got, want) {
			t.Errorf("ws show passes says nothing of %q:\n%s", want, got)
		}
	}
}
