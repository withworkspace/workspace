package cli_test

import (
	"github.com/withworkspace/workspace/internal/doc"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// writePage writes a page's text as the browser does.
func writePage(t *testing.T, ps page.Store, id string, text string) {
	t.Helper()
	was, _, _ := ps.Read(id)
	if _, err := ps.Write(id, doc.Version(was), []byte(text)); err != nil {
		t.Fatal(err)
	}
}

// A file opened from Files is a page tied to it: one page however often
// it's opened. Saved, the page goes to the file as it reads (cells as
// text, questions gone), landed in a repo with no pull requests.
func TestPages_AFileIsEditedAsAPage(t *testing.T) {
	w := docsWorkspace(t, "")
	ref := "records:docs/plan.md"
	ps := page.Store{Root: w.Root}

	id, m, err := cli.OpenFilePage(w.Root, ref)
	if err != nil || m != nil || id == "" {
		t.Fatalf("opening it: %q %+v %v", id, m, err)
	}
	text, p, _ := ps.Read(id)
	if string(text) != "# Plan\n\nKey it per user.\n\n## Rollout\n\nShip on Friday.\n" || p.Source == nil || p.Source.Ref != ref || p.Title != "Plan" {
		t.Fatalf("the page: %q %+v", text, p.Source)
	}
	if again, _, _ := cli.OpenFilePage(w.Root, ref); again != id {
		t.Errorf("opened again: %s, not %s", again, id)
	}
	if tied := cli.PageTied(w.Root, id); tied == nil || tied.Changed || tied.Saves == nil || tied.Saves.How != "land" || tied.In.Path != "docs/plan.md" {
		t.Fatalf("where it stands: %+v", tied)
	}

	writePage(t, ps, id, "# Plan\n\nKey it per tenant.\n\n<!-- ws:cell w3 v1 | a table -->\n| a | b |\n|---|---|\n<!-- /ws:cell -->\n\n<!-- ws:cell w4 v0 ask | why? -->\n<!-- /ws:cell -->\n\n## Rollout\n\nShip on Friday.\n")
	saved, err := cli.SavePageToFile(w.Root, id, "")

	if err != nil || !strings.HasPrefix(saved.Message, "Saved: landed on main") {
		t.Fatalf("saving: %+v %v", saved, err)
	}
	src := w.Sources["records"]
	if got := gitOut(t, src, "show", "main:docs/plan.md"); got != "# Plan\n\nKey it per tenant.\n\n| a | b |\n|---|---|\n\n## Rollout\n\nShip on Friday.\n" {
		t.Errorf("the file on main: %q", got)
	}
	if got := subjects(t, src, "main"); !strings.HasPrefix(got, "Edit docs/plan.md | docs: add the plan") {
		t.Errorf("main has: %s", got)
	}
	if text, _, _ := ps.Read(id); !strings.Contains(string(text), "<!-- ws:cell w3 v1") {
		t.Error("the page lost its cells")
	}
	if tied := cli.PageTied(w.Root, id); tied.Changed {
		t.Error("saved, the page and its file should agree")
	}
}

// When the file changes under the page, saving is refused until what
// changed is brought in: merged with the page's edits and cells. Where
// both change the same lines, both are marked on the page to settle.
func TestPages_WhatChangedInTheFileIsBroughtIn(t *testing.T) {
	w := docsWorkspace(t, "")
	ps := page.Store{Root: w.Root}
	id, _, _ := cli.OpenFilePage(w.Root, "records:docs/plan.md")
	writePage(t, ps, id, "# Plan\n\nKey it per tenant.\n\n<!-- ws:cell w3 v1 | a note -->\nNoted.\n<!-- /ws:cell -->\n\n## Rollout\n\nShip on Friday.\n")

	// Someone lands a change to the rollout, and the ground has it.
	src := w.Sources["records"]
	commitIn(t, src, "docs/plan.md", "# Plan\n\nKey it per user.\n\n## Rollout\n\nShip on Tuesday.\n", "docs: ship on Tuesday")
	groundUp(t, w, "records")

	if tied := cli.PageTied(w.Root, id); !tied.Changed {
		t.Fatalf("the file's changed: %+v", tied)
	}
	if _, err := cli.SavePageToFile(w.Root, id, ""); !cli.IsFileMoved(err) {
		t.Fatalf("saving over a file that's moved on: %v", err)
	}
	got, err := cli.BringIn(w.Root, id)
	if err != nil || !got.Changed || got.Clashes != 0 {
		t.Fatalf("bringing it in: %+v %v", got, err)
	}
	if text, _, _ := ps.Read(id); string(text) != "# Plan\n\nKey it per tenant.\n\n<!-- ws:cell w3 v1 | a note -->\nNoted.\n<!-- /ws:cell -->\n\n## Rollout\n\nShip on Tuesday.\n" {
		t.Errorf("the page, brought up: %q", text)
	}
	if _, err := cli.SavePageToFile(w.Root, id, "docs: key the plan per tenant"); err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, src, "show", "main:docs/plan.md"); got != "# Plan\n\nKey it per tenant.\n\nNoted.\n\n## Rollout\n\nShip on Tuesday.\n" {
		t.Errorf("main has: %q", got)
	}

	// The same line changed on both sides: marked on the page.
	commitIn(t, src, "docs/plan.md", "# Plan\n\nKey it per region.\n\nNoted.\n\n## Rollout\n\nShip on Tuesday.\n", "docs: per region")
	groundUp(t, w, "records")
	writePage(t, ps, id, strings.Replace(string(must(ps.Read(id))), "per tenant", "per team", 1))
	got, _ = cli.BringIn(w.Root, id)
	if text, _, _ := ps.Read(id); got.Clashes != 1 || !strings.Contains(string(text), "<<<<<<< your edit") {
		t.Errorf("a clash: %+v\n%s", got, text)
	}
}

// A page filed as a document is tied to it: saved again, it's an amend.
func TestPages_AFiledPageSavesToItsFile(t *testing.T) {
	w := kindsWorkspace(t)
	ps := page.Store{Root: w.Root}
	p, _ := ps.New([]byte("# Quiet hours\n\nNo noise at night.\n"))
	if _, err := cli.FilePage(w.Root, time.Now())(p.ID, web.PageFile{Kind: "adr"}); err != nil {
		t.Fatal(err)
	}
	_, after, _ := ps.Read(p.ID)
	if after.Source == nil || after.Source.Ref != "records:adr/0008-quiet-hours.md" {
		t.Fatalf("tied to: %+v", after.Source)
	}
	writePage(t, ps, p.ID, "# Quiet hours\n\nNo noise between ten and seven.\n")
	if _, err := cli.SavePageToFile(w.Root, p.ID, "docs: amend adr 0008 hours"); err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, w.Sources["records"], "show", "main:adr/0008-quiet-hours.md"); got != "# Quiet hours\n\nNo noise between ten and seven.\n" {
		t.Errorf("main has: %q", got)
	}
}

// groundUp brings a repo's ground up to its remote's main.
func groundUp(t *testing.T, w *testutil.Workspace, repo string) {
	t.Helper()
	ground := filepath.Join(w.Root, "repos", repo, ".ground")
	testutil.GitCmd(t, ground, "fetch", "origin")
	testutil.GitCmd(t, ground, "merge", "--ff-only", "origin/main")
}

func must(text []byte, _ page.Page, err error) []byte {
	if err != nil {
		panic(err)
	}
	return text
}
