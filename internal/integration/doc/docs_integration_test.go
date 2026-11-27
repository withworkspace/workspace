package doc_test

import (
	"github.com/withworkspace/workspace/internal/doc"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// A file in a repo is a document outside any mission: ws show names it,
// and the threads on it, which are the workspace's own.
func TestShow_ADocumentOutsideAMissionAndItsThreads(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	ground := filepath.Join(w.Root, "repos", "records", ".ground")
	testutil.CommitIn(t, w.Sources["records"], "adr/0001-one.md", "# One\n\nIt stays one binary.\n", "docs: add adr 0001")
	testutil.GitCmd(t, ground, "fetch", "origin")
	testutil.GitCmd(t, ground, "merge", "--ff-only", "origin/main")

	r := testutil.Run(t, w, nil, "show", "records:adr/0001-one.md", "-f", "llm")
	for _, want := range []string{"Document records:adr/0001-one.md (on the default branch of records)", filepath.Join("repos", "records", ".ground", "adr", "0001-one.md"), "No threads on it."} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("ws show of the file says nothing of %q:\n%s", want, r.Stdout)
		}
	}

	text, _ := os.ReadFile(filepath.Join(ground, "adr", "0001-one.md"))
	store := conversation.ForWorkspace(w.Root)
	v, _ := store.KeepVersion(text)
	c, err := store.OpenThread(conversation.NewThread{Kind: conversation.Ask, By: conversation.You, Body: "Why one?",
		Anchor: conversation.Anchor{Doc: "records:adr/0001-one.md", Path: "adr/0001-one.md", Quote: &conversation.Quote{Exact: "one binary"}, Line: 3, EndLine: 3, Version: v, Excerpt: []string{"It stays one binary."}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	r = testutil.Run(t, w, nil, "show", "records:adr/0001-one.md", "-f", "llm")
	if !strings.Contains(r.Stdout, "1 thread on it, 1 open.") || !strings.Contains(r.Stdout, c.ID+"  a thread on it (ask, open, waiting for an answer): Why one?") {
		t.Errorf("ws show of the file doesn't list its thread:\n%s", r.Stdout)
	}
	r = testutil.Run(t, w, nil, "show", c.ID, "-f", "llm")
	for _, want := range []string{"Thread " + c.ID + " (ask, open, waiting for an answer) on records:adr/0001-one.md", "> one binary", "3  It stays one binary.", "- you, ", "Why one?", "not part of a mission", "records:adr/0001-one.md  the document it's on"} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("ws show of the thread says nothing of %q:\n%s", want, r.Stdout)
		}
	}

	// A file that isn't there, a repo that isn't, a path that leads out.
	for _, ref := range []string{"records:adr/nope.md", "nope:README.md", "records:../../ws.toml"} {
		if r := testutil.RunCommand(t, w.Root, nil, "show", ref); r.Err == nil {
			t.Errorf("ws show %s: no error", ref)
		}
	}
}

// One a mission records is that mission's: its threads are there.
func TestShow_ADocumentAMissionRecordsSaysSo(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	testutil.Run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nWrite a note.")
	t.Setenv(mission.EnvMission, "notes")
	testutil.Run(t, w, nil, "lift", "records", "note")
	dir := testutil.CapsuleDir(w, "records", "note")
	testutil.CommitIn(t, dir, "notes/a.md", "# A\n", "docs: add note a")
	testutil.Run(t, w, nil, "mission", "artifact", "add", filepath.Join(dir, "notes", "a.md"), "--kind", "note")
	t.Setenv(mission.EnvMission, "")

	r := testutil.Run(t, w, nil, "show", "records/note:notes/a.md", "-f", "llm")
	if !strings.Contains(r.Stdout, "Document records/note:notes/a.md (in the capsule records/note)") || !strings.Contains(r.Stdout, `Mission notes ("Notes") records it as records/note:notes/a.md`) {
		t.Errorf("ws show of a mission's document, from outside it:\n%s", r.Stdout)
	}
}

// docsWorkspace is landWorkspace with a document on each repo's default
// branch: records lands directly, api takes pull requests.
func docsWorkspace(t *testing.T, verify string) *testutil.Workspace {
	t.Helper()
	w := testutil.LandWorkspace(t, verify)
	for _, repo := range []string{"records", "api"} {
		testutil.CommitIn(t, w.Sources[repo], "docs/plan.md", "# Plan\n\nKey it per user.\n\n## Rollout\n\nShip on Friday.\n", "docs: add the plan")
		ground := filepath.Join(w.Root, "repos", repo, ".ground")
		testutil.GitCmd(t, ground, "fetch", "origin")
		testutil.GitCmd(t, ground, "merge", "--ff-only", "origin/main")
	}
	return w
}

// openDoc reads a document outside a mission as the page does, keeping the
// version it opened to merge an edit with.
func openDoc(t *testing.T, w *testutil.Workspace, ref string) *web.ArtifactContent {
	t.Helper()
	a, err := testutil.WebDoc(w.Root)(ref)
	if err != nil {
		t.Fatal(err)
	}
	conversation.ForWorkspace(w.Root).KeepVersion([]byte(a.Text))
	return a
}

// An edit to a document on the default branch of a repo with no pull
// requests is one commit of yours, landed there; the ground has it, and
// nothing's left behind.
func TestDocs_SavingOnADirectReposDefaultBranchLandsIt(t *testing.T) {
	w := docsWorkspace(t, "")
	ref := "records:docs/plan.md"
	a := openDoc(t, w, ref)
	if !a.Editable || a.Saves == nil || a.Saves.How != "land" || a.Saves.Subject != "Edit docs/plan.md" {
		t.Fatalf("how it saves: %+v, editable %v", a.Saves, a.Editable)
	}
	(doc.Drafts{Root: w.Root}).Save(doc.Draft{Ref: ref, Base: doc.Version([]byte(a.Text)), Text: "draft", At: time.Now()})

	s, err := testutil.SaveFileDoc(w.Root)(web.DocSave{Ref: ref, Base: doc.Version([]byte(a.Text)), Text: strings.Replace(a.Text, "per user", "per tenant", 1), Message: "docs: key the plan per tenant"})

	if err != nil || s.Conflict != nil || !strings.Contains(s.Message, "Saved: landed on main") || s.Ref != ref {
		t.Fatalf("saving: %+v, %v", s, err)
	}
	src := w.Sources["records"]
	if got := testutil.Subjects(t, src, "main"); got != "docs: key the plan per tenant | docs: add the plan | initial" {
		t.Errorf("the remote's main has: %s", got)
	}
	if body := testutil.GitOut(t, src, "log", "-1", "--format=%b", "main"); !strings.Contains(body, "Edited in ws's web UI.") {
		t.Errorf("the commit's body: %q", body)
	}
	ground, _ := os.ReadFile(filepath.Join(w.Root, "repos", "records", ".ground", "docs", "plan.md"))
	if !strings.Contains(string(ground), "Key it per tenant.") {
		t.Errorf("the ground has: %q", ground)
	}
	// No capsule, no branch, no draft left of it.
	entries, _ := os.ReadDir(filepath.Join(w.Root, "repos", "records"))
	for _, e := range entries {
		if e.Name() != ".bare" && e.Name() != ".ground" {
			t.Errorf("left behind: repos/records/%s", e.Name())
		}
	}
	if branches := strings.TrimSpace(testutil.GitOut(t, filepath.Join(w.Root, "repos", "records", ".bare"), "branch", "--list", "edit-*")); branches != "" {
		t.Errorf("a branch was left: %s", branches)
	}
	if d, _ := (doc.Drafts{Root: w.Root}).Get(ref); d != nil {
		t.Errorf("the draft was kept: %+v", d)
	}
	if a := openDoc(t, w, ref); doc.Version([]byte(a.Text)) != s.Version {
		t.Errorf("the document's version after isn't what saving said")
	}
}

// What landed there meanwhile is merged with; where the two touch the
// same lines, nothing's saved, and the clash comes back to settle.
func TestDocs_SavingOnADefaultBranchMergesWithWhatLandedSince(t *testing.T) {
	w := docsWorkspace(t, "")
	ref := "records:docs/plan.md"
	a := openDoc(t, w, ref)
	base := doc.Version([]byte(a.Text))
	save := testutil.SaveFileDoc(w.Root)
	src := w.Sources["records"]

	// Someone lands a change to another part of it: both stand.
	testutil.CommitIn(t, src, "docs/plan.md", strings.Replace(a.Text, "on Friday", "on Tuesday", 1), "docs: ship on Tuesday")
	s, err := save(web.DocSave{Ref: ref, Base: base, Text: strings.Replace(a.Text, "per user", "per tenant", 1)})
	if err != nil || s.Conflict != nil || !s.Merged || !strings.Contains(s.Message, "merged with changes on main") {
		t.Fatalf("a merge: %+v, %v", s, err)
	}
	if got := testutil.GitOut(t, src, "show", "main:docs/plan.md"); got != "# Plan\n\nKey it per tenant.\n\n## Rollout\n\nShip on Tuesday.\n" {
		t.Errorf("main has: %q", got)
	}

	// And one to the same line: a clash, nothing landed, nothing left.
	a = openDoc(t, w, ref)
	base = doc.Version([]byte(a.Text))
	testutil.CommitIn(t, src, "docs/plan.md", strings.Replace(a.Text, "per tenant", "per region", 1), "docs: key it per region")
	before := testutil.Subjects(t, src, "main")
	s, err = save(web.DocSave{Ref: ref, Base: base, Text: strings.Replace(a.Text, "per tenant", "per team", 1)})
	if err != nil || s.Conflict == nil || s.Conflict.Clashes != 1 || !strings.Contains(s.Conflict.Text, "<<<<<<< your edit") {
		t.Fatalf("a clash: %+v, %v", s, err)
	}
	if got := testutil.Subjects(t, src, "main"); got != before {
		t.Errorf("something landed: %s", got)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "repos", "records", "edit-plan")); !os.IsNotExist(err) {
		t.Error("the capsule lifted for it was left")
	}
	// Settled, and saved from what's there now.
	s, err = save(web.DocSave{Ref: ref, Base: s.Version, Text: "# Plan\n\nKey it per team.\n\n## Rollout\n\nShip on Tuesday.\n"})
	if err != nil || s.Conflict != nil || !strings.Contains(s.Message, "Saved: landed") {
		t.Fatalf("settled: %+v, %v", s, err)
	}
}

// An edit that doesn't pass the repo's check isn't saved at all.
func TestDocs_SavingWhatFailsTheCheckSavesNothing(t *testing.T) {
	w := docsWorkspace(t, "! grep -q TODO docs/plan.md")
	ref := "records:docs/plan.md"
	a := openDoc(t, w, ref)

	s, err := testutil.SaveFileDoc(w.Root)(web.DocSave{Ref: ref, Base: doc.Version([]byte(a.Text)), Text: a.Text + "\nTODO: say when.\n"})

	if err == nil || !strings.Contains(err.Error(), "not saved") || !strings.Contains(err.Error(), "fails records's check") {
		t.Fatalf("saving: %+v, %v", s, err)
	}
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "docs: add the plan | initial" {
		t.Errorf("something landed: %s", got)
	}
	entries, _ := os.ReadDir(filepath.Join(w.Root, "repos", "records"))
	if len(entries) != 2 {
		t.Errorf("a capsule was left: %v", entries)
	}
}

// In a repo that takes pull requests, a capsule named for the document is
// lifted with your commit in it, and left to you: nothing's pushed.
func TestDocs_SavingOnAPRReposDefaultBranchLiftsACapsule(t *testing.T) {
	w := docsWorkspace(t, "")
	ref := "api:docs/plan.md"
	a := openDoc(t, w, ref)
	if a.Saves == nil || a.Saves.How != "lift" {
		t.Fatalf("how it saves: %+v", a.Saves)
	}

	s, err := testutil.SaveFileDoc(w.Root)(web.DocSave{Ref: ref, Base: doc.Version([]byte(a.Text)), Text: strings.Replace(a.Text, "per user", "per tenant", 1)})

	if err != nil || s.Ref != "api/edit-plan:docs/plan.md" || !strings.Contains(s.Message, "Saved in a new capsule, api/edit-plan") {
		t.Fatalf("saving: %+v, %v", s, err)
	}
	capsule := testutil.CapsuleDir(w, "api", "edit-plan")
	if got := testutil.Subjects(t, capsule, "HEAD"); got != "Edit docs/plan.md | docs: add the plan | initial" {
		t.Errorf("the capsule has: %s", got)
	}
	if got := testutil.Subjects(t, w.Sources["api"], "main"); got != "docs: add the plan | initial" {
		t.Errorf("something was pushed: %s", got)
	}
	// It's a document in a capsule now: saved there as any is, committed.
	b := openDoc(t, w, s.Ref)
	if b.Saves == nil || b.Saves.How != "capsule" || !strings.Contains(b.Text, "per tenant") {
		t.Fatalf("the capsule's document: %+v", b.Saves)
	}
	s2, err := testutil.SaveFileDoc(w.Root)(web.DocSave{Ref: s.Ref, Base: doc.Version([]byte(b.Text)), Text: strings.Replace(b.Text, "on Friday", "on Tuesday", 1), Message: "Ship on Tuesday"})
	if err != nil || !strings.Contains(s2.Message, "Saved: committed") {
		t.Fatalf("saving in the capsule: %+v, %v", s2, err)
	}
	if got := testutil.Subjects(t, capsule, "HEAD"); !strings.HasPrefix(got, "Ship on Tuesday | Edit docs/plan.md") {
		t.Errorf("the capsule has: %s", got)
	}
	// A second edit of the ground's gets a capsule of its own.
	s3, err := testutil.SaveFileDoc(w.Root)(web.DocSave{Ref: ref, Base: doc.Version([]byte(a.Text)), Text: a.Text + "\nMore.\n"})
	if err != nil || s3.Ref != "api/edit-plan-2:docs/plan.md" {
		t.Fatalf("a second edit: %+v, %v", s3, err)
	}
}

// A document of one of the workspace's kinds is saved with its kind's
// message, and says what it is; one in another repo, of the same shape,
// isn't of the kind.
func TestDocs_AKindsDocumentIsSavedWithItsMessage(t *testing.T) {
	w := docsWorkspace(t, "")
	f, err := os.OpenFile(filepath.Join(w.Root, "ws.toml"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\n[documents.plan]\ntitle = \"Plan\"\npath = \"records:docs/{slug}.md\"\nadd = \"docs: add the {slug}\"\namend = \"docs: amend the {slug} ({title})\"\nguide = \"records:docs/HOW.md\"\n")
	f.Close()
	ref := "records:docs/plan.md"

	a := openDoc(t, w, ref)

	if k := a.DocKind; k == nil || k.Name != "plan" || k.Title != "Plan" || k.Where != "records:docs/{slug}.md" || k.Subject != "docs: amend the plan (Plan)" {
		t.Fatalf("its kind: %+v", a.DocKind)
	}
	if a.Saves.Subject != "docs: amend the plan (Plan)" {
		t.Errorf("a save's message starts as: %q", a.Saves.Subject)
	}
	s, err := testutil.SaveFileDoc(w.Root)(web.DocSave{Ref: ref, Base: doc.Version([]byte(a.Text)), Text: strings.Replace(a.Text, "per user", "per tenant", 1)})
	if err != nil || !strings.Contains(s.Message, "Saved: landed") {
		t.Fatalf("saving: %+v, %v", s, err)
	}
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != "docs: amend the plan (Plan) | docs: add the plan | initial" {
		t.Errorf("the remote's main has: %s", got)
	}

	r := testutil.Run(t, w, nil, "show", ref, "-f", "llm")
	if !strings.Contains(r.Stdout, "Of the kind Plan (plan), one of the workspace's kinds of document") {
		t.Errorf("ws show of it:\n%s", r.Stdout)
	}
	r = testutil.Run(t, w, nil, "show", "kinds", "-f", "llm")
	for _, want := range []string{"kinds of document (1)", "- Plan (plan): records:docs/{slug}.md", `Added as "docs: add the {slug}", changed as "docs: amend the {slug} ({title})"`, "Its guide would be " + filepath.Join(w.Root, "repos", "records", ".ground", "docs", "HOW.md") + ", which isn't there"} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("ws show kinds says nothing of %q:\n%s", want, r.Stdout)
		}
	}

	if b := openDoc(t, w, "api:docs/plan.md"); b.DocKind != nil || b.Saves.Subject != "Edit docs/plan.md" {
		t.Errorf("api's docs/plan.md: %+v, %q", b.DocKind, b.Saves.Subject)
	}
}

// A kind that can't be one stops ws, saying which and why.
func TestDocs_AKindThatCantBeOneIsRefused(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	f, _ := os.OpenFile(filepath.Join(w.Root, "ws.toml"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("\n[documents.note]\npath = \"nowhere:notes/{slug}.md\"\n")
	f.Close()

	_, err := cli.LoadOps(w.Root)

	if err == nil || !strings.Contains(err.Error(), "[documents.note]") || !strings.Contains(err.Error(), "not a workspace repo") {
		t.Errorf("loading the workspace: %v", err)
	}
}

// The workspace's guide and skills are found by any agent: ws show skills
// lists them, a mission's view has them, and crew are told where they are.
func TestShow_TheWorkspacesGuideAndSkills(t *testing.T) {
	w := testutil.LandWorkspace(t, "")
	os.WriteFile(filepath.Join(w.Root, "AGENTS.md"), []byte("# Guide\n"), 0o644)
	skill := filepath.Join(w.Root, ".agents", "skills", "atomic-repo", "SKILL.md")
	os.MkdirAll(filepath.Dir(skill), 0o755)
	os.WriteFile(skill, []byte("---\nname: atomic-repo\ndescription: How to land changes in a repo with no pull requests.\n---\n"), 0o644)

	r := testutil.Run(t, w, nil, "show", "skills", "-f", "llm")
	for _, want := range []string{"The workspace's skills (1)", "guide for agents: " + filepath.Join(w.Root, "AGENTS.md"), "- atomic-repo: How to land changes in a repo with no pull requests.", skill} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("ws show skills says nothing of %q:\n%s", want, r.Stdout)
		}
	}

	testutil.Run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nWrite a note.")
	r = testutil.Run(t, w, nil, "show", "-m", "notes", "-f", "llm")
	if !strings.Contains(r.Stdout, "Guide for agents: "+filepath.Join(w.Root, "AGENTS.md")) || !strings.Contains(r.Stdout, "  atomic-repo: How to land changes in a repo with no pull requests. — "+skill) {
		t.Errorf("the mission's view doesn't have them:\n%s", r.Stdout)
	}

	// And ws doctor says Claude Code, which is in use, sees neither.
	os.WriteFile(filepath.Join(w.Root, "ws.local.toml"), []byte("[crew]\nharness = \"claude\"\n"), 0o644)
	d := testutil.RunCommand(t, w.Root, nil, "doctor")
	if !strings.Contains(d.Stderr+d.Stdout, "claude sees the skills") || !strings.Contains(d.Stderr+d.Stdout, "atomic-repo") {
		t.Errorf("ws doctor:\n%s%s", d.Stdout, d.Stderr)
	}
	testutil.RunCommand(t, w.Root, nil, "doctor", "--fix")
	if _, err := os.Stat(filepath.Join(w.Root, ".claude", "skills", "atomic-repo", "SKILL.md")); err != nil {
		t.Errorf("ws doctor --fix didn't link the skill: %v", err)
	}
	if to, _ := os.Readlink(filepath.Join(w.Root, "CLAUDE.md")); to != "AGENTS.md" {
		t.Errorf("ws doctor --fix: CLAUDE.md -> %q", to)
	}
}
