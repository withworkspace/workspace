package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
)

func TestSpaceOf_WhatTheWorkspaceSaysThenTheRepoThenRecords(t *testing.T) {
	root := t.TempDir()
	w := &Workspace{Root: root, RepoNames: []string{"api", "docs", "records", "web"}, Records: "records",
		DocumentsSaid: map[string]*config.Documents{"api": {On: true, Folder: "docs/"}}}
	// docs says it's a space of documents in its own ws.repo.toml.
	ground := w.MainWorktree("docs")
	os.MkdirAll(ground, 0o755)
	os.WriteFile(filepath.Join(ground, config.RepoFileName), []byte("documents = true\n"), 0o644)

	got := map[string]Space{}
	for _, s := range w.Spaces() {
		got[s.Repo] = s
	}
	if len(got) != 3 || got["api"].Folder != "docs/" || got["docs"].Folder != "" || got["records"].Repo != "records" {
		t.Fatalf("spaces = %+v", got)
	}
	if _, ok := w.SpaceOf("web"); ok {
		t.Error("web is code")
	}
	if !got["api"].Has("docs/guide.md") || got["api"].Has("README.md") || got["api"].Has("docs/logo.png") {
		t.Error("api's space holds the wrong files")
	}
	// The workspace's word beats the repo's, and records can say no.
	w.DocumentsSaid["docs"] = &config.Documents{On: false}
	w.DocumentsSaid["records"] = &config.Documents{On: false}
	if len(w.Spaces()) != 1 {
		t.Errorf("spaces = %+v", w.Spaces())
	}
}

func TestKind_MovesAreWhereItsDocumentsStand(t *testing.T) {
	k, err := NewKind(Kind{Name: "bp", Repo: "records", Pattern: "bp/proposed/{date}-{slug}.md", Moves: []Move{{Name: "accept", To: "bp/accepted/"}, {Name: "retire", To: "bp/retired"}}})
	if err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{
		"bp/proposed/2026-11-12-cache.md": "proposed",
		"bp/accepted/2026-11-12-cache.md": "accepted",
		"bp/retired/2026-10-01-old.md":    "retired",
		"bp/accepted/notes.md":            "",
		"elsewhere/2026-11-12-cache.md":   "",
	} {
		f, state, ok := k.MatchState(rel)
		if ok != (want != "") || state != want {
			t.Errorf("%s: %q %v, want %q", rel, state, ok, want)
		}
		if ok && f.Date == "" {
			t.Errorf("%s: what its path says: %+v", rel, f)
		}
	}
	if got := k.States(); len(got) != 3 || got[0] != "proposed" || got[2] != "retired" {
		t.Errorf("states = %v", got)
	}
	plain, _ := NewKind(Kind{Name: "adr", Repo: "records", Pattern: "adr/{number}-{slug}.md"})
	if plain.States() != nil {
		t.Errorf("a kind with no moves stands in one place: %v", plain.States())
	}
	if _, err := NewKind(Kind{Name: "x", Repo: "r", Pattern: "x/{slug}.md", Moves: []Move{{Name: "go", To: "../out"}}}); err == nil {
		t.Error("a move out of the repo was taken")
	}
}
